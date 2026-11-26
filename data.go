package main

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"sync"

	"go.yaml.in/yaml/v3"
)

// dataFS holds the tuning files under data/, so a designer edits the balance without touching Go.
//
//go:embed data/*.yaml
var dataFS embed.FS

// The schema version each data file must declare, which its loader accepts and its encoder writes.
const (
	runVersion     = 1
	weaponVersion  = 1
	upgradeVersion = 1
	enemyVersion   = 1
	waveVersion    = 1
	fileRun        = "data/run.yaml"
	fileWeapons    = "data/weapons.yaml"
	fileUpgrades   = "data/upgrades.yaml"
	fileEnemies    = "data/enemies.yaml"
	fileWaves      = "data/waves.yaml"
)

// Weapon kinds as data/weapons.yaml names them.
const (
	weaponBolt  = "bolt"
	weaponHalo  = "halo"
	weaponTrail = "trail"
	weaponBell  = "bell"
)

// Passive stats as data/upgrades.yaml names them.
const (
	statDamage    = "damage"
	statCooldown  = "cooldown"
	statArea      = "area"
	statMoveSpeed = "moveSpeed"
	statMaxHealth = "maxHealth"
	statRegen     = "regen"
	statPickup    = "pickup"
	statXPGain    = "xpGain"
)

// A kind of enemy movement, as an enemy's motion field names it.
const (
	// motionChase walks straight at the keeper.
	motionChase = "chase"
	// motionDrift walks at the keeper while swaying across the line between them.
	motionDrift = "drift"
)

// gameData is every tuning file a run reads, loaded and cross-checked once at startup.
type gameData struct {
	run           runFile
	weapons       weaponFile
	upgrades      upgradeFile
	enemies       enemyFile
	waves         waveFile
	starterWeapon int
}

// runFile is data/run.yaml: the arena, the keeper, the embers and the level curve.
type runFile struct {
	Version int       `yaml:"version"`
	Arena   arenaDef  `yaml:"arena"`
	Keeper  keeperDef `yaml:"keeper"`
	Embers  emberDef  `yaml:"embers"`
	Level   levelDef  `yaml:"level"`
}

// arenaDef is the run's world bounds, in world units, which are virtual pixels at zoom one.
type arenaDef struct {
	Width  float64 `yaml:"width"`
	Height float64 `yaml:"height"`
}

// keeperDef is the keeper's size, speed and health before any upgrade.
type keeperDef struct {
	Radius       float64 `yaml:"radius"`
	Speed        float64 `yaml:"speed"`
	Health       float64 `yaml:"health"`
	Invulnerable float64 `yaml:"invulnerable"`
}

// emberDef is what an enemy leaves behind and how it reaches the keeper.
type emberDef struct {
	Radius       float64 `yaml:"radius"`
	MagnetRadius float64 `yaml:"magnetRadius"`
	MagnetSpeed  float64 `yaml:"magnetSpeed"`
	Value        int     `yaml:"value"`
}

// levelDef is how much XP each level needs: First for level 2, then Step more for each level after.
type levelDef struct {
	First float64 `yaml:"first"`
	Step  float64 `yaml:"step"`
}

// weaponFile is data/weapons.yaml: the starter weapon name, knockback decay and every weapon's levels.
type weaponFile struct {
	Version       int         `yaml:"version"`
	Starter       string      `yaml:"starter"`
	KnockbackDrag float64     `yaml:"knockbackDrag"`
	Weapons       []weaponDef `yaml:"weapons"`
}

// weaponDef is one weapon's identity, kind-specific geometry and per-level scaling.
type weaponDef struct {
	ID          string           `yaml:"id"`
	Name        string           `yaml:"name"`
	Kind        string           `yaml:"kind"`
	Description string           `yaml:"description"`
	CooldownMin float64          `yaml:"cooldownMin"`
	Range       float64          `yaml:"range,omitempty"`
	ShotSpeed   float64          `yaml:"shotSpeed,omitempty"`
	ShotRadius  float64          `yaml:"shotRadius,omitempty"`
	ShotLife    float64          `yaml:"shotLife,omitempty"`
	Spread      float64          `yaml:"spread,omitempty"`
	OrbRadius   float64          `yaml:"orbRadius,omitempty"`
	OrbitSpeed  float64          `yaml:"orbitSpeed,omitempty"`
	Duration    float64          `yaml:"duration,omitempty"`
	Levels      []weaponLevelDef `yaml:"levels"`
}

// weaponLevelDef is one level of a weapon.
type weaponLevelDef struct {
	Damage    float64 `yaml:"damage"`
	Cooldown  float64 `yaml:"cooldown"`
	Knockback float64 `yaml:"knockback,omitempty"`
	Count     int     `yaml:"count,omitempty"`
	Pierce    int     `yaml:"pierce,omitempty"`
	Radius    float64 `yaml:"radius,omitempty"`
	Duration  float64 `yaml:"duration,omitempty"`
	Tick      float64 `yaml:"tick,omitempty"`
}

// upgradeFile is data/upgrades.yaml: slot caps, passives and the fallback healing card.
type upgradeFile struct {
	Version      int          `yaml:"version"`
	WeaponSlots  int          `yaml:"weaponSlots"`
	PassiveSlots int          `yaml:"passiveSlots"`
	Passives     []passiveDef `yaml:"passives"`
	Healing      healDef      `yaml:"healing"`
}

// passiveDef is one passive card in the upgrade pool: a stat it raises by Amount at each level, up to MaxLevel.
type passiveDef struct {
	ID          string  `yaml:"id"`
	Name        string  `yaml:"name"`
	Stat        string  `yaml:"stat"`
	Description string  `yaml:"description"`
	Amount      float64 `yaml:"amount"`
	MaxLevel    int     `yaml:"maxLevel"`
}

// healDef is the card that fills an offer when fewer than three upgrades remain.
type healDef struct {
	ID          string  `yaml:"id"`
	Name        string  `yaml:"name"`
	Description string  `yaml:"description"`
	Fraction    float64 `yaml:"fraction"`
}

// enemyFile is data/enemies.yaml: every enemy kind the waves send.
type enemyFile struct {
	Version int        `yaml:"version"`
	Enemies []enemyDef `yaml:"enemies"`
}

// enemyDef is a kind of enemy: how it moves, how hard it hits and what it leaves behind.
type enemyDef struct {
	Name            string   `yaml:"name"`
	Motion          string   `yaml:"motion"`
	Health          float64  `yaml:"health"`
	Speed           float64  `yaml:"speed"`
	Radius          float64  `yaml:"radius"`
	Damage          float64  `yaml:"damage"`
	KnockbackResist float64  `yaml:"knockbackResist"`
	Embers          int      `yaml:"embers"`
	Drift           driftDef `yaml:"drift,omitempty"`
}

// driftDef is how a drifting enemy sways: Amplitude is the fraction of its speed across its course, and Period is how long one sway takes, in seconds.
type driftDef struct {
	Amplitude float64 `yaml:"amplitude"`
	Period    float64 `yaml:"period"`
}

// waveFile is data/waves.yaml: the spawn timeline, the ring spawns appear on and the global living cap.
type waveFile struct {
	Version  int       `yaml:"version"`
	MaxAlive int       `yaml:"maxAlive"`
	Ring     ringDef   `yaml:"ring"`
	Spans    []spanDef `yaml:"spans"`
}

// ringDef is how far from the keeper, in world units, a spawn may appear.
type ringDef struct {
	Min float64 `yaml:"min"`
	Max float64 `yaml:"max"`
}

// spanDef is one stretch of the timeline, from its From second until the next span starts.
type spanDef struct {
	From     float64  `yaml:"from"`
	Every    float64  `yaml:"every"`
	MaxAlive int      `yaml:"maxAlive"`
	Health   float64  `yaml:"health"`
	Damage   float64  `yaml:"damage"`
	Enemies  []mixDef `yaml:"enemies"`

	// weights is Enemies' weights in order, which the spawn stream draws from and loadGameData fills in.
	weights []float64
}

// mixDef is how strongly a span favours one enemy kind.
type mixDef struct {
	Name   string  `yaml:"name"`
	Weight float64 `yaml:"weight"`
}

// loadGameData reads, decodes and cross-checks the tuning files.
func loadGameData(fsys fs.FS) (*gameData, error) {
	runBytes, err := fs.ReadFile(fsys, fileRun)
	if err != nil {
		return nil, err
	}
	run, err := parseRunFile(fileRun, runBytes)
	if err != nil {
		return nil, err
	}
	weaponBytes, err := fs.ReadFile(fsys, fileWeapons)
	if err != nil {
		return nil, err
	}
	weapons, err := parseWeaponFile(fileWeapons, weaponBytes)
	if err != nil {
		return nil, err
	}
	upgradeBytes, err := fs.ReadFile(fsys, fileUpgrades)
	if err != nil {
		return nil, err
	}
	upgrades, err := parseUpgradeFile(fileUpgrades, upgradeBytes)
	if err != nil {
		return nil, err
	}
	enemyBytes, err := fs.ReadFile(fsys, fileEnemies)
	if err != nil {
		return nil, err
	}
	enemies, err := parseEnemyFile(fileEnemies, enemyBytes)
	if err != nil {
		return nil, err
	}
	waveBytes, err := fs.ReadFile(fsys, fileWaves)
	if err != nil {
		return nil, err
	}
	waves, err := parseWaveFile(fileWaves, waveBytes)
	if err != nil {
		return nil, err
	}
	d := &gameData{run: *run, weapons: *weapons, upgrades: *upgrades, enemies: *enemies, waves: *waves}
	if err := d.check(); err != nil {
		return nil, err
	}
	return d, nil
}

// check resolves the names the files reference across each other, which a single file cannot check alone.
func (d *gameData) check() error {
	if len(d.weapons.Weapons) > 0 {
		starter, ok := d.weaponIndex(d.weapons.Starter)
		if !ok {
			return fmt.Errorf("%s: starter weapon %q not found", fileWeapons, d.weapons.Starter)
		}
		d.starterWeapon = starter
	}
	for si := range d.waves.Spans {
		span := &d.waves.Spans[si]
		span.weights = make([]float64, len(span.Enemies))
		for mi := range span.Enemies {
			mix := &span.Enemies[mi]
			if _, ok := d.enemyIndex(mix.Name); !ok {
				return fmt.Errorf("%s: span %d: no enemy %q", fileWaves, si, mix.Name)
			}
			span.weights[mi] = mix.Weight
		}
	}
	return nil
}

// weaponIndex returns the position of the weapon called name in the weapons file.
func (d *gameData) weaponIndex(name string) (int, bool) {
	for i := range d.weapons.Weapons {
		if d.weapons.Weapons[i].Name == name {
			return i, true
		}
	}
	return 0, false
}

// enemyIndex returns the position of the enemy kind called name in the enemies file.
func (d *gameData) enemyIndex(name string) (int, bool) {
	for i := range d.enemies.Enemies {
		if d.enemies.Enemies[i].Name == name {
			return i, true
		}
	}
	return 0, false
}

// parseRunFile decodes and validates data/run.yaml, refusing unknown fields, trailing documents and values a run cannot use.
func parseRunFile(file string, data []byte) (*runFile, error) {
	var f runFile
	if err := decodeStrict(file, data, &f); err != nil {
		return nil, err
	}
	if err := f.validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	return &f, nil
}

// parseWeaponFile decodes and validates data/weapons.yaml.
func parseWeaponFile(file string, data []byte) (*weaponFile, error) {
	var f weaponFile
	if err := decodeStrict(file, data, &f); err != nil {
		return nil, err
	}
	if err := f.validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	return &f, nil
}

// parseUpgradeFile decodes and validates data/upgrades.yaml.
func parseUpgradeFile(file string, data []byte) (*upgradeFile, error) {
	var f upgradeFile
	if err := decodeStrict(file, data, &f); err != nil {
		return nil, err
	}
	if err := f.validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	return &f, nil
}

// parseEnemyFile decodes and validates data/enemies.yaml.
func parseEnemyFile(file string, data []byte) (*enemyFile, error) {
	var f enemyFile
	if err := decodeStrict(file, data, &f); err != nil {
		return nil, err
	}
	if err := f.validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	return &f, nil
}

// parseWaveFile decodes and validates data/waves.yaml.
func parseWaveFile(file string, data []byte) (*waveFile, error) {
	var f waveFile
	if err := decodeStrict(file, data, &f); err != nil {
		return nil, err
	}
	if err := f.validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	return &f, nil
}

// decodeStrict decodes one YAML document into v, refusing fields v does not declare and any document after the first.
func decodeStrict(file string, data []byte, v any) error {
	if len(bytes.TrimSpace(data)) == 0 {
		return fmt.Errorf("%s: empty file", file)
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("%s: decode: %w", file, err)
	}
	var extra any
	switch err := dec.Decode(&extra); {
	case err == nil:
		return fmt.Errorf("%s: unexpected trailing YAML document", file)
	case !errors.Is(err, io.EOF):
		return fmt.Errorf("%s: decode: %w", file, err)
	}
	return nil
}

// encodeYAML writes v with 2-space indents and its fields in declaration order, so the same value always encodes to the same bytes.
func encodeYAML(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(v); err != nil {
		_ = enc.Close()
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (r *runFile) validate() error {
	switch {
	case r.Version != runVersion:
		return fmt.Errorf("version %d, want %d", r.Version, runVersion)
	case !positive(r.Arena.Width) || !positive(r.Arena.Height):
		return fmt.Errorf("arena %vx%v is not above 0", r.Arena.Width, r.Arena.Height)
	case !positive(r.Keeper.Radius):
		return fmt.Errorf("keeper radius %v is not above 0", r.Keeper.Radius)
	case !positive(r.Keeper.Speed):
		return fmt.Errorf("keeper speed %v is not above 0", r.Keeper.Speed)
	case !positive(r.Keeper.Health):
		return fmt.Errorf("keeper health %v is not above 0", r.Keeper.Health)
	case !finite(r.Keeper.Invulnerable) || r.Keeper.Invulnerable < 0:
		return fmt.Errorf("keeper invulnerable %v is negative", r.Keeper.Invulnerable)
	}
	e := r.Embers
	switch {
	case !positive(e.Radius):
		return fmt.Errorf("embers radius %v is not above 0", e.Radius)
	case !positive(e.MagnetRadius):
		return fmt.Errorf("embers magnetRadius %v is not above 0", e.MagnetRadius)
	case !positive(e.MagnetSpeed):
		return fmt.Errorf("embers magnetSpeed %v is not above 0", e.MagnetSpeed)
	case e.Value < 1:
		return fmt.Errorf("embers value %d is not at least 1", e.Value)
	case !positive(r.Level.First):
		return fmt.Errorf("level first %v is not above 0", r.Level.First)
	case !finite(r.Level.Step) || r.Level.Step < 0:
		return fmt.Errorf("level step %v is negative", r.Level.Step)
	}
	return nil
}

func (f *weaponFile) validate() error {
	switch {
	case f.Version != weaponVersion:
		return fmt.Errorf("version %d, want %d", f.Version, weaponVersion)
	case f.Starter == "":
		return errors.New("starter weapon is empty")
	case !positive(f.KnockbackDrag):
		return fmt.Errorf("knockbackDrag %v is not above 0", f.KnockbackDrag)
	case len(f.Weapons) == 0:
		return errors.New("no weapons")
	}
	foundStarter := false
	for i := range f.Weapons {
		w := &f.Weapons[i]
		if err := w.validate(); err != nil {
			return err
		}
		if w.Name == f.Starter {
			foundStarter = true
		}
		for _, other := range f.Weapons[:i] {
			if other.ID == w.ID {
				return fmt.Errorf("weapon id %q is defined twice", w.ID)
			}
			if other.Name == w.Name {
				return fmt.Errorf("weapon %q is defined twice", w.Name)
			}
		}
	}
	if !foundStarter {
		return fmt.Errorf("starter weapon %q is not defined", f.Starter)
	}
	return nil
}

func (w *weaponDef) validate() error {
	if w.ID == "" {
		return errors.New("a weapon has no id")
	}
	if w.Name == "" {
		return fmt.Errorf("weapon %q has no name", w.ID)
	}
	fail := func(format string, args ...any) error {
		return fmt.Errorf("weapon %q: %s", w.Name, fmt.Sprintf(format, args...))
	}
	switch {
	case w.Description == "":
		return fail("has no description")
	case !positive(w.CooldownMin):
		return fail("cooldownMin %v is not above 0", w.CooldownMin)
	case len(w.Levels) == 0:
		return fail("has no levels")
	}
	switch w.Kind {
	case weaponBolt:
		switch {
		case !positive(w.Range):
			return fail("range %v is not above 0", w.Range)
		case !positive(w.ShotSpeed):
			return fail("shotSpeed %v is not above 0", w.ShotSpeed)
		case !positive(w.ShotRadius):
			return fail("shotRadius %v is not above 0", w.ShotRadius)
		case !positive(w.ShotLife):
			return fail("shotLife %v is not above 0", w.ShotLife)
		case !finite(w.Spread) || w.Spread < 0:
			return fail("spread %v is negative", w.Spread)
		case w.OrbRadius != 0 || w.OrbitSpeed != 0 || w.Duration != 0:
			return fail("sets fields for another weapon kind")
		}
	case weaponHalo:
		switch {
		case !positive(w.OrbRadius):
			return fail("orbRadius %v is not above 0", w.OrbRadius)
		case !positive(w.OrbitSpeed):
			return fail("orbitSpeed %v is not above 0", w.OrbitSpeed)
		case w.Range != 0 || w.ShotSpeed != 0 || w.ShotRadius != 0 || w.ShotLife != 0 || w.Spread != 0 || w.Duration != 0:
			return fail("sets fields for another weapon kind")
		}
	case weaponTrail:
		if w.Range != 0 || w.ShotSpeed != 0 || w.ShotRadius != 0 || w.ShotLife != 0 || w.Spread != 0 || w.OrbRadius != 0 || w.OrbitSpeed != 0 || w.Duration != 0 {
			return fail("sets fields for another weapon kind")
		}
	case weaponBell:
		switch {
		case !positive(w.Duration):
			return fail("duration %v is not above 0", w.Duration)
		case w.Range != 0 || w.ShotSpeed != 0 || w.ShotRadius != 0 || w.ShotLife != 0 || w.Spread != 0 || w.OrbRadius != 0 || w.OrbitSpeed != 0:
			return fail("sets fields for another weapon kind")
		}
	default:
		return fail("kind %q is not %s, %s, %s or %s", w.Kind, weaponBolt, weaponHalo, weaponTrail, weaponBell)
	}
	for i := range w.Levels {
		if err := w.Levels[i].validate(w.Kind, w.CooldownMin); err != nil {
			return fmt.Errorf("weapon %q: level %d: %w", w.Name, i+1, err)
		}
	}
	return nil
}

func (l *weaponLevelDef) validate(kind string, cooldownMin float64) error {
	switch {
	case !positive(l.Damage):
		return fmt.Errorf("damage %v is not above 0", l.Damage)
	case !positive(l.Cooldown):
		return fmt.Errorf("cooldown %v is not above 0", l.Cooldown)
	case cooldownMin > l.Cooldown:
		return fmt.Errorf("cooldown %v is below cooldownMin %v", l.Cooldown, cooldownMin)
	case !finite(l.Knockback) || l.Knockback < 0:
		return fmt.Errorf("knockback %v is negative", l.Knockback)
	}
	switch kind {
	case weaponBolt:
		if l.Count < 1 {
			return fmt.Errorf("count %d is not at least 1", l.Count)
		}
		if l.Pierce < 0 || l.Pierce >= maxShotHits {
			return fmt.Errorf("pierce %d is not in [0, %d)", l.Pierce, maxShotHits)
		}
		if l.Radius != 0 || l.Duration != 0 || l.Tick != 0 {
			return errors.New("sets fields for another weapon kind")
		}
	case weaponHalo:
		if l.Count < 1 {
			return fmt.Errorf("count %d is not at least 1", l.Count)
		}
		if !positive(l.Radius) {
			return fmt.Errorf("radius %v is not above 0", l.Radius)
		}
		if l.Pierce != 0 || l.Duration != 0 || l.Tick != 0 {
			return errors.New("sets fields for another weapon kind")
		}
	case weaponTrail:
		if !positive(l.Radius) {
			return fmt.Errorf("radius %v is not above 0", l.Radius)
		}
		if !positive(l.Duration) {
			return fmt.Errorf("duration %v is not above 0", l.Duration)
		}
		if !positive(l.Tick) {
			return fmt.Errorf("tick %v is not above 0", l.Tick)
		}
		if l.Count != 0 || l.Pierce != 0 {
			return errors.New("sets fields for another weapon kind")
		}
	case weaponBell:
		if !positive(l.Radius) {
			return fmt.Errorf("radius %v is not above 0", l.Radius)
		}
		if l.Count != 0 || l.Pierce != 0 || l.Duration != 0 || l.Tick != 0 {
			return errors.New("sets fields for another weapon kind")
		}
	}
	return nil
}

func (f *upgradeFile) validate() error {
	switch {
	case f.Version != upgradeVersion:
		return fmt.Errorf("version %d, want %d", f.Version, upgradeVersion)
	case f.WeaponSlots < 1:
		return fmt.Errorf("weaponSlots %d is not at least 1", f.WeaponSlots)
	case f.PassiveSlots < 1:
		return fmt.Errorf("passiveSlots %d is not at least 1", f.PassiveSlots)
	case len(f.Passives) == 0:
		return errors.New("no passives")
	}
	for i := range f.Passives {
		p := &f.Passives[i]
		if p.ID == "" {
			return fmt.Errorf("passive %d has no id", i)
		}
		if p.Name == "" {
			return fmt.Errorf("passive %q has no name", p.ID)
		}
		if p.Description == "" {
			return fmt.Errorf("passive %q has no description", p.Name)
		}
		switch p.Stat {
		case statDamage, statCooldown, statArea, statMoveSpeed, statMaxHealth, statRegen, statPickup, statXPGain:
		default:
			return fmt.Errorf("passive %q: unknown stat %q", p.Name, p.Stat)
		}
		if !positive(p.Amount) {
			return fmt.Errorf("passive %q: amount %v is not above 0", p.Name, p.Amount)
		}
		if p.MaxLevel < 1 {
			return fmt.Errorf("passive %q: maxLevel %d is not at least 1", p.Name, p.MaxLevel)
		}
		if p.Stat == statCooldown && p.Amount*float64(p.MaxLevel) >= 1 {
			return fmt.Errorf("passive %q: cooldown reduction reaches 100%%", p.Name)
		}
		for _, other := range f.Passives[:i] {
			if other.ID == p.ID {
				return fmt.Errorf("passive id %q is defined twice", p.ID)
			}
			if other.Name == p.Name {
				return fmt.Errorf("passive %q is defined twice", p.Name)
			}
		}
	}
	switch {
	case f.Healing.ID == "":
		return errors.New("healing card has no id")
	case f.Healing.Name == "":
		return errors.New("healing card has no name")
	case f.Healing.Description == "":
		return errors.New("healing card has no description")
	case !finite(f.Healing.Fraction) || f.Healing.Fraction <= 0 || f.Healing.Fraction > 1:
		return fmt.Errorf("healing fraction %v is not in (0, 1]", f.Healing.Fraction)
	}
	return nil
}

func (f *enemyFile) validate() error {
	if f.Version != enemyVersion {
		return fmt.Errorf("version %d, want %d", f.Version, enemyVersion)
	}
	if len(f.Enemies) == 0 {
		return errors.New("no enemies")
	}
	for i := range f.Enemies {
		e := &f.Enemies[i]
		if err := e.validate(); err != nil {
			return err
		}
		for _, other := range f.Enemies[:i] {
			if other.Name == e.Name {
				return fmt.Errorf("enemy %q is defined twice", e.Name)
			}
		}
	}
	return nil
}

func (e *enemyDef) validate() error {
	if e.Name == "" {
		return errors.New("an enemy has no name")
	}
	fail := func(format string, args ...any) error {
		return fmt.Errorf("enemy %q: %s", e.Name, fmt.Sprintf(format, args...))
	}
	switch {
	case e.Motion != motionChase && e.Motion != motionDrift:
		return fail("motion %q is not %s or %s", e.Motion, motionChase, motionDrift)
	case !positive(e.Health):
		return fail("health %v is not above 0", e.Health)
	case !positive(e.Speed):
		return fail("speed %v is not above 0", e.Speed)
	case !positive(e.Radius):
		return fail("radius %v is not above 0", e.Radius)
	case !finite(e.Damage) || e.Damage < 0:
		return fail("damage %v is negative", e.Damage)
	case !finite(e.KnockbackResist) || e.KnockbackResist < 0 || e.KnockbackResist > 1:
		return fail("knockbackResist %v is not in [0, 1]", e.KnockbackResist)
	case e.Embers < 0:
		return fail("embers %d is negative", e.Embers)
	}
	switch e.Motion {
	case motionDrift:
		if !finite(e.Drift.Amplitude) || e.Drift.Amplitude < 0 {
			return fail("drift amplitude %v is negative", e.Drift.Amplitude)
		}
		if !positive(e.Drift.Period) {
			return fail("a drifting enemy needs a drift period above 0")
		}
	case motionChase:
		if e.Drift != (driftDef{}) {
			return fail("a chasing enemy has drift values")
		}
	}
	return nil
}

func (f *waveFile) validate() error {
	switch {
	case f.Version != waveVersion:
		return fmt.Errorf("version %d, want %d", f.Version, waveVersion)
	case f.MaxAlive < 1 || f.MaxAlive > maxEnemies:
		return fmt.Errorf("maxAlive %d is not from 1 to %d", f.MaxAlive, maxEnemies)
	case !positive(f.Ring.Min):
		return fmt.Errorf("ring min %v is not above 0", f.Ring.Min)
	case !positive(f.Ring.Max) || f.Ring.Max < f.Ring.Min:
		return fmt.Errorf("ring max %v is not at least min %v", f.Ring.Max, f.Ring.Min)
	case len(f.Spans) == 0:
		return errors.New("no spans")
	}
	for i := range f.Spans {
		s := &f.Spans[i]
		if err := s.validate(); err != nil {
			return fmt.Errorf("span %d: %w", i, err)
		}
		if i == 0 && s.From != 0 {
			return fmt.Errorf("span 0 starts at %v, want 0", s.From)
		}
		if i > 0 && s.From <= f.Spans[i-1].From {
			return fmt.Errorf("span %d starts at %v, not after span %d at %v", i, s.From, i-1, f.Spans[i-1].From)
		}
		if s.MaxAlive > f.MaxAlive {
			return fmt.Errorf("span %d: maxAlive %d is above the file's %d", i, s.MaxAlive, f.MaxAlive)
		}
	}
	return nil
}

func (s *spanDef) validate() error {
	switch {
	case !finite(s.From) || s.From < 0:
		return fmt.Errorf("from %v is negative", s.From)
	case !positive(s.Every):
		return fmt.Errorf("every %v is not above 0", s.Every)
	case s.MaxAlive < 1:
		return fmt.Errorf("maxAlive %d is not at least 1", s.MaxAlive)
	case !positive(s.Health):
		return fmt.Errorf("health %v is not above 0", s.Health)
	case !positive(s.Damage):
		return fmt.Errorf("damage %v is not above 0", s.Damage)
	case len(s.Enemies) == 0:
		return errors.New("no enemies")
	}
	for i := range s.Enemies {
		m := &s.Enemies[i]
		if m.Name == "" {
			return fmt.Errorf("mix %d has no enemy name", i)
		}
		if !positive(m.Weight) {
			return fmt.Errorf("%s: weight %v is not above 0", m.Name, m.Weight)
		}
	}
	return nil
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

func positive(v float64) bool { return finite(v) && v > 0 }

// mustGameData is the tuning the embedded files hold, which the app cannot run without.
var mustGameData = sync.OnceValue(func() *gameData {
	d, err := loadGameData(dataFS)
	if err != nil {
		panic("embervigil: " + err.Error())
	}
	return d
})
