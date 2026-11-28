package main

import (
	"encoding/json"
	"math"
	"slices"

	"github.com/nsgo-dev/nsgo/core"
	evstrings "me.oneo/embervigil/strings"
)

// Storage keys and schema versions for the persisted save slots.
const (
	settingsStorageKey    = "settings"
	recordsStorageKey     = "records"
	progressionStorageKey = "progression"

	settingsVersion        = 1
	recordsVersion         = 1
	progressionSaveVersion = 1
)

// Quality is the graphics quality tier stored in settings and consumed by the renderer.
type Quality string

const (
	QualityAuto   Quality = "auto"
	QualityLow    Quality = "low"
	QualityMedium Quality = "medium"
	QualityHigh   Quality = "high"
)

// qualityTiers lists the quality options in display order.
var qualityTiers = []Quality{QualityAuto, QualityLow, QualityMedium, QualityHigh}

// Settings holds player preferences persisted across sessions.
type Settings struct {
	Version      int            `json:"version"`
	MasterVolume float64        `json:"masterVolume"`
	MusicVolume  float64        `json:"musicVolume"`
	SFXVolume    float64        `json:"sfxVolume"`
	ScreenShake  bool           `json:"screenShake"`
	Quality      Quality        `json:"quality"`
	Language     evstrings.Lang `json:"language"`
	// Stats is whether the game reports its use to Umami.
	Stats bool `json:"stats"`
}

// Records holds lifetime best stats persisted across sessions.
type Records struct {
	Version   int     `json:"version"`
	HasRecord bool    `json:"hasRecord"`
	BestTime  float64 `json:"bestTime"`
	BestKills int     `json:"bestKills"`
	BestLevel int     `json:"bestLevel"`
	Wins      int     `json:"wins"`
}

// Progression is what the player has earned and unlocked across runs.
type Progression struct {
	Version int `json:"version"`
	// Embers is the balance the shop spends.
	Embers int `json:"embers"`
	// Shop holds each bought shop item's level by item id.
	Shop map[string]int `json:"shop,omitempty"`
	// TierWon is the highest Long Night tier won, 0 before the first win.
	TierWon int `json:"tierWon"`
	// Unlocked and Collection are the ids of unlocked content and of evolution recipes made, in the order they were earned.
	Unlocked   []string `json:"unlocked,omitempty"`
	Collection []string `json:"collection,omitempty"`
}

// openTier is the highest Long Night tier the player may choose, of the maxTier there are: the one after the highest won. Tier 1 is always open.
func (p Progression) openTier(maxTier int) int { return max(1, min(maxTier, p.TierWon+1)) }

// has reports whether id is unlocked.
func (p Progression) has(id string) bool { return slices.Contains(p.Unlocked, id) }

// defaultSettings returns the initial preferences before any user edits are saved.
func defaultSettings() Settings {
	return Settings{
		Version:      settingsVersion,
		MasterVolume: 0.8,
		MusicVolume:  0.7,
		SFXVolume:    0.8,
		ScreenShake:  true,
		Quality:      QualityAuto,
		Language:     evstrings.EN,
		Stats:        true,
	}
}

// defaultRecords returns an empty lifetime record set at the current schema version.
func defaultRecords() Records {
	return Records{Version: recordsVersion}
}

// defaultProgression returns an empty progression at the current schema version.
func defaultProgression() Progression {
	return Progression{Version: progressionSaveVersion}
}

// normalizeProgression drops negative amounts, empty and repeated ids, and stamps the current schema version.
func normalizeProgression(p Progression) Progression {
	p.Version = progressionSaveVersion
	p.Embers = max(0, p.Embers)
	p.TierWon = max(0, p.TierWon)
	var shop map[string]int
	for id, level := range p.Shop {
		if id != "" && level > 0 {
			if shop == nil {
				shop = map[string]int{}
			}
			shop[id] = level
		}
	}
	p.Shop = shop
	p.Unlocked = uniqueIDs(p.Unlocked)
	p.Collection = uniqueIDs(p.Collection)
	return p
}

// uniqueIDs returns ids without empty or repeated entries, keeping the first of each in order.
func uniqueIDs(ids []string) []string {
	var out []string
	for _, id := range ids {
		if id != "" && !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}

// normalizeSettings clamps volumes and coerces unknown quality or language values to valid defaults.
func normalizeSettings(s Settings) Settings {
	s.Version = settingsVersion
	s.MasterVolume = clampVolume(s.MasterVolume)
	s.MusicVolume = clampVolume(s.MusicVolume)
	s.SFXVolume = clampVolume(s.SFXVolume)
	switch s.Quality {
	case QualityAuto, QualityLow, QualityMedium, QualityHigh:
	default:
		s.Quality = QualityAuto
	}
	s.Language = evstrings.NormalizeLang(string(s.Language))
	return s
}

// clampVolume keeps a volume slider in [0, 1] and rounds to two decimal places to avoid float drift.
func clampVolume(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	v = math.Min(1, math.Max(0, v))
	return math.Round(v*100) / 100
}

// normalizeRecords ensures non-negative stats and a current schema version.
func normalizeRecords(r Records) Records {
	r.Version = recordsVersion
	if math.IsNaN(r.BestTime) || math.IsInf(r.BestTime, 0) || r.BestTime < 0 {
		r.BestTime = 0
	}
	if r.BestKills < 0 {
		r.BestKills = 0
	}
	if r.BestLevel < 0 {
		r.BestLevel = 0
	}
	if r.Wins < 0 {
		r.Wins = 0
	}
	if r.BestTime > 0 || r.BestKills > 0 || r.BestLevel > 0 || r.Wins > 0 {
		r.HasRecord = true
	}
	return r
}

// loadSettings reads, validates and migrates the settings slot from storage, falling back to defaults when missing or corrupt.
func loadSettings(storage core.Storage) Settings {
	raw, ok := storage.GetItem(settingsStorageKey)
	if !ok || raw == "" {
		return defaultSettings()
	}
	s, migrated, ok := decodeSettings([]byte(raw))
	if !ok {
		return defaultSettings()
	}
	if migrated {
		_ = saveSettings(storage, s)
	}
	return s
}

// decodeSettings parses a settings payload, migrating older schemas (version 0) and rejecting unsupported future versions.
func decodeSettings(data []byte) (Settings, bool, bool) {
	var envelope struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return Settings{}, false, false
	}
	if envelope.Version < 0 || envelope.Version > settingsVersion {
		return Settings{}, false, false
	}

	// Start from defaults so omitted fields in older payloads keep sensible values.
	s := defaultSettings()
	if err := json.Unmarshal(data, &s); err != nil {
		return Settings{}, false, false
	}
	migrated := envelope.Version < settingsVersion
	return normalizeSettings(s), migrated, true
}

// saveSettings writes normalized settings to storage under the versioned settings slot.
func saveSettings(storage core.Storage, s Settings) error {
	s = normalizeSettings(s)
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return storage.SetItem(settingsStorageKey, string(data))
}

// loadRecords reads, validates and migrates the records slot from storage, falling back to empty records when missing or corrupt.
func loadRecords(storage core.Storage) Records {
	raw, ok := storage.GetItem(recordsStorageKey)
	if !ok || raw == "" {
		return defaultRecords()
	}
	r, migrated, ok := decodeRecords([]byte(raw))
	if !ok {
		return defaultRecords()
	}
	if migrated {
		_ = saveRecords(storage, r)
	}
	return r
}

// decodeRecords parses a records payload, migrating older schemas (version 0) and rejecting unsupported future versions.
func decodeRecords(data []byte) (Records, bool, bool) {
	var envelope struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return Records{}, false, false
	}
	if envelope.Version < 0 || envelope.Version > recordsVersion {
		return Records{}, false, false
	}

	var r Records
	if err := json.Unmarshal(data, &r); err != nil {
		return Records{}, false, false
	}
	migrated := envelope.Version < recordsVersion
	return normalizeRecords(r), migrated, true
}

// saveRecords writes normalized records to storage under the versioned records slot.
func saveRecords(storage core.Storage, r Records) error {
	r = normalizeRecords(r)
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	return storage.SetItem(recordsStorageKey, string(data))
}

// applyRunRecords folds a finished run snapshot into prev and reports whether any personal best was set.
func applyRunRecords(prev Records, s Snapshot) (Records, bool) {
	next := normalizeRecords(prev)
	isNew := !next.HasRecord ||
		s.Time > next.BestTime ||
		s.Kills > next.BestKills ||
		s.Level > next.BestLevel ||
		(s.Phase == PhaseVictory && next.Wins == 0)

	if !next.HasRecord || s.Time > next.BestTime {
		next.BestTime = s.Time
	}
	if !next.HasRecord || s.Kills > next.BestKills {
		next.BestKills = s.Kills
	}
	if !next.HasRecord || s.Level > next.BestLevel {
		next.BestLevel = s.Level
	}
	if s.Phase == PhaseVictory {
		next.Wins++
	}
	next.HasRecord = true
	return next, isNew
}

// loadProgression reads and validates the progression slot from storage; a save without one, or a corrupt or future one, starts empty.
func loadProgression(storage core.Storage) Progression {
	raw, ok := storage.GetItem(progressionStorageKey)
	if !ok || raw == "" {
		return defaultProgression()
	}
	var envelope struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil || envelope.Version < 1 || envelope.Version > progressionSaveVersion {
		return defaultProgression()
	}
	var p Progression
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return defaultProgression()
	}
	return normalizeProgression(p)
}

// saveProgression writes normalized progression to storage under the versioned progression slot.
func saveProgression(storage core.Storage, p Progression) error {
	data, err := json.Marshal(normalizeProgression(p))
	if err != nil {
		return err
	}
	return storage.SetItem(progressionStorageKey, string(data))
}

// runReward is what a finished run added to the progression: its embers, the ids it newly unlocked and the Long Night tier its win opened, 0 when none.
type runReward struct {
	embers     int
	unlocks    []string
	tierOpened int
}

// applyRunProgression adds a finished run's embers and unlocks to prev, raising the highest tier won when the run won at tier, and reports what it newly gave. maxTier is the highest tier there is, which a win cannot open past.
func applyRunProgression(prev Progression, won bool, tier, maxTier, embers int, unlocks []string) (Progression, runReward) {
	next := normalizeProgression(prev)
	reward := runReward{embers: max(0, embers)}
	next.Embers += reward.embers
	if won && tier > next.TierWon {
		next.TierWon = tier
		if tier < maxTier {
			reward.tierOpened = tier + 1
		}
	}
	for _, id := range unlocks {
		if id != "" && !next.has(id) {
			next.Unlocked = append(next.Unlocked, id)
			reward.unlocks = append(reward.unlocks, id)
		}
	}
	return next, reward
}

// addEvolutions records in prev's collection the evolution ids it does not hold yet, in order, and returns them.
func addEvolutions(prev Progression, ids []string) (Progression, []string) {
	next := normalizeProgression(prev)
	var added []string
	for _, id := range ids {
		if id != "" && !slices.Contains(next.Collection, id) {
			next.Collection = append(next.Collection, id)
			added = append(added, id)
		}
	}
	return next, added
}

// shopLevel is the level the progression holds of shop item id, at most the shop's highest.
func (p Progression) shopLevel(shop *shopDef, id string) int {
	return max(0, min(shop.maxLevel(), p.Shop[id]))
}

// shopCost is what the next level of item id costs, and false when the item is unknown or at its last level.
func (p Progression) shopCost(shop *shopDef, id string) (int, bool) {
	level := p.shopLevel(shop, id)
	if shop.item(id) < 0 || level >= shop.maxLevel() {
		return 0, false
	}
	return shop.Costs[level], true
}

// buyShopItem spends the next level's cost of item id from prev's embers and raises its level, reporting false, with prev unchanged, when the item is unknown, at its last level or more than the balance.
func buyShopItem(prev Progression, shop *shopDef, id string) (Progression, bool) {
	next := normalizeProgression(prev)
	cost, ok := next.shopCost(shop, id)
	if !ok || cost > next.Embers {
		return next, false
	}
	next.Embers -= cost
	shopLevels := make(map[string]int, len(next.Shop)+1)
	for k, v := range next.Shop {
		shopLevels[k] = v
	}
	shopLevels[id] = next.shopLevel(shop, id) + 1
	next.Shop = shopLevels
	return next, true
}
