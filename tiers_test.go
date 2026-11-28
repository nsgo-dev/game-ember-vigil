package main

import (
	"bytes"
	"io/fs"
	"math"
	"testing"

	"github.com/nsgo-dev/nsgo/core"
)

// TestTierFileIsCanonicalAndRefusesBrokenTiers checks that data/tiers.yaml matches its embedded copy and encodes back to its own bytes, and that the loader refuses tiers a run cannot play.
func TestTierFileIsCanonicalAndRefusesBrokenTiers(t *testing.T) {
	data := readData(t, fileTiers)
	embedded, err := fs.ReadFile(dataFS, fileTiers)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(embedded, data) {
		t.Fatalf("%s on disk and its embedded copy differ", fileTiers)
	}
	f, err := parseTierFile(fileTiers, data)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := encodeYAML(f)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encoded, data) {
		t.Fatalf("%s is not canonical, want:\n%s", fileTiers, encoded)
	}

	replace := func(old, new string) []byte {
		if !bytes.Contains(data, []byte(old)) {
			t.Fatalf("%q is not in the file to replace", old)
		}
		return bytes.Replace(data, []byte(old), []byte(new), 1)
	}
	for name, broken := range map[string][]byte{
		"another version":       replace("version: 1", "version: 2"),
		"an unknown field":      append(append([]byte(nil), data...), []byte("unknown: true\n")...),
		"tiers out of order":    replace("tier: 3", "tier: 4"),
		"a tier with no rule":   []byte("version: 1\ntiers:\n  - tier: 2\n"),
		"sight past the base":   replace("sight: 0.8", "sight: 1.2"),
		"no cards at all":       replace("healCards: 1", "healCards: 3"),
		"a shadow without life": replace("life: 14", "life: 0"),
		"a volley of no bones":  replace("count: 13", "count: 0"),
	} {
		if _, err := parseTierFile(fileTiers, broken); err == nil {
			t.Errorf("the loader accepted tiers with %s", name)
		}
	}

	d := *mustGameData()
	d.tiers.Tiers = append([]tierDef(nil), d.tiers.Tiers...)
	d.tiers.Tiers[0].Elites = []eliteSpawnDef{{At: 1, Enemy: "Nobody", Health: 1, Damage: 1}}
	if err := d.tiers.check(&d); err == nil {
		t.Error("a tier's elite naming no enemy was accepted")
	}
	d.tiers.Tiers[0].Elites = nil
	d.tiers.Tiers[3].BossEarlier = d.waves.Boss.At
	if err := d.tiers.check(&d); err == nil {
		t.Error("a Hollow King arriving at the start was accepted")
	}
}

// TestTiersStackTheirRules checks that a run at each tier plays every rule up to it and none above it, and that a tier the data does not define plays the nearest one.
func TestTiersStackTheirRules(t *testing.T) {
	d := mustGameData()
	base := len(d.waves.Elites)
	for tier, want := range map[int]struct {
		elites               int
		shadow, volley       bool
		light, sight, regen  float64
		healCards            int
		speed, earlier, harm float64
	}{
		1: {elites: base, light: 1, sight: 1, regen: 1, speed: 1, harm: 1},
		2: {elites: base + 1, shadow: true, light: 1, sight: 1, regen: 1, speed: 1, harm: 1},
		3: {elites: base + 1, shadow: true, light: 0.9, sight: 0.8, regen: 1, speed: 1, harm: 1},
		4: {elites: base + 1, shadow: true, light: 0.9, sight: 0.8, regen: 0.5, healCards: 1, speed: 1, harm: 1},
		5: {elites: base + 1, shadow: true, volley: true, light: 0.9, sight: 0.8, regen: 0.5, healCards: 1, speed: 1.1, earlier: 60, harm: 1.15},
	} {
		r := NewRun(1, tier)
		got := r.rules
		if r.Tier() != tier || len(got.elites) != want.elites || (got.shadow.Life > 0) != want.shadow || (got.volley.Count > 0) != want.volley ||
			got.light != want.light || got.sight != want.sight || got.regen != want.regen || got.healCards != want.healCards ||
			math.Abs(got.enemySpeed-want.speed) > 1e-9 || got.bossEarlier != want.earlier || math.Abs(got.damage-want.harm) > 1e-9 {
			t.Errorf("tier %d plays %+v, want %+v", tier, got, want)
		}
		for i := 1; i < len(got.elites); i++ {
			if got.elites[i].At < got.elites[i-1].At {
				t.Errorf("tier %d's elites are out of time order: %+v", tier, got.elites)
			}
		}
	}
	if NewRun(1, 0).Tier() != 1 || NewRun(1, 9).Tier() != 5 {
		t.Fatal("a tier outside 1 to 5 did not play the nearest one")
	}
	if len(d.waves.Elites) != base || len(NewRun(1, 1).rules.elites) != base {
		t.Fatal("a tier's elites leaked into the waves every run reads")
	}
}

// TestDeadElitesLeaveShadowsThatHurt checks tier 2's rule: an elite that dies leaves a shadow zone that hurts the keeper once a tick while it stands in it and fades after its life, and tier 1 leaves none.
func TestDeadElitesLeaveShadowsThatHurt(t *testing.T) {
	for _, tier := range []int{1, 2} {
		r := NewRun(3, tier)
		eliteKind := -1
		for i := range r.data.enemies.Enemies {
			if r.data.enemies.Enemies[i].Elite {
				eliteKind = i
			}
		}
		r.addEnemy(eliteKind, r.keeper.pos.Add(Vec2{X: 10}), 1, 1)
		r.enemies[len(r.enemies)-1].hp = 0
		r.reap()
		if tier == 1 {
			if len(r.shadows) != 0 {
				t.Fatalf("a dead elite at tier 1 left %d shadows", len(r.shadows))
			}
			continue
		}
		if len(r.shadows) != 1 || len(r.Snapshot().Shadows) != 1 {
			t.Fatalf("a dead elite at tier 2 left %d shadows, want 1", len(r.shadows))
		}
		s := r.rules.shadow
		before := r.keeper.health
		steps := int(math.Round(s.Tick / stepSeconds))
		for range steps {
			r.stepShadows()
		}
		if got := before - r.keeper.health; math.Abs(got-s.Damage) > 1e-9 {
			t.Fatalf("a tick in the shadow took %v health, want %v", got, s.Damage)
		}
		r.stepShadows()
		if got := before - r.keeper.health; math.Abs(got-2*s.Damage) > 1e-9 {
			t.Fatalf("the next tick took %v health in all, want %v", got, 2*s.Damage)
		}
		r.keeper.pos = r.keeper.pos.Add(Vec2{X: 3 * s.Radius})
		r.keeper.health = before
		for range int(s.Life/stepSeconds) + 1 {
			r.stepShadows()
		}
		if r.keeper.health != before || len(r.shadows) != 0 {
			t.Fatalf("outside the shadow the keeper lost %v health, and %d shadows remain after their life", before-r.keeper.health, len(r.shadows))
		}
	}
}

// TestShortSightSpawnsCloserButUnseen checks tier 3's rule: the snapshot carries the shorter sight, embers are drawn from nearer, and the director still spawns outside the shrunk view.
func TestShortSightSpawnsCloserButUnseen(t *testing.T) {
	base, short := NewRun(5, 1), NewRun(5, 3)
	if s := short.Snapshot().Sight; s != short.rules.sight || s >= 1 || base.Snapshot().Sight != 1 {
		t.Fatalf("snapshot sight is %v at tier 3 and %v at tier 1", s, base.Snapshot().Sight)
	}
	if short.stats.pickupRadius >= base.stats.pickupRadius {
		t.Fatalf("tier 3 draws embers from %v, not nearer than tier 1's %v", short.stats.pickupRadius, base.stats.pickupRadius)
	}
	ring := short.data.waves.Ring
	for range 200 {
		p, ok := short.spawnPos(4)
		if !ok {
			continue
		}
		d := p.Sub(short.keeper.pos).Len()
		if d > ring.Max*short.rules.sight+1e-9 || !offscreenWithin(short.keeper.pos, p, short.rules.sight) {
			t.Fatalf("tier 3 spawned at %v, %v away, outside its ring or inside its view", p, d)
		}
	}
}

// TestFewerHealingCardsAndHalfRegen checks tier 4's rule: an offer holds at most one healing card, so a spent pool offers one card, and regeneration halves.
func TestFewerHealingCardsAndHalfRegen(t *testing.T) {
	for _, tier := range []int{3, 4} {
		r := NewRun(8, tier)
		for i := range r.weaponLevels {
			r.weaponLevels[i] = len(r.data.weapons.Weapons[i].Levels)
		}
		for i := range r.passiveLevels {
			r.passiveLevels[i] = r.data.upgrades.Passives[i].MaxLevel
		}
		r.recomputeStats()
		offers := r.drawOffers()
		want := offerCount
		if tier == 4 {
			want = 1
		}
		if len(offers) != want || !offers[0].heal {
			t.Fatalf("a spent pool at tier %d offers %d cards, want %d healing cards", tier, len(offers), want)
		}
		r.passiveLevels = make([]int, len(r.passiveLevels))
		r.recomputeStats()
		if got := r.drawOffers(); len(got) != offerCount {
			t.Fatalf("a full pool at tier %d offers %d cards, want %d", tier, len(got), offerCount)
		}
	}
	mending := -1
	for i, p := range mustGameData().upgrades.Passives {
		if p.Stat == statRegen {
			mending = i
		}
	}
	regen := func(tier int) float64 {
		r := NewRun(8, tier)
		r.passiveLevels[mending] = 2
		r.recomputeStats()
		return r.stats.regen
	}
	if a, b := regen(3), regen(4); math.Abs(b-a/2) > 1e-9 {
		t.Fatalf("regeneration is %v at tier 4 and %v at tier 3, want half", b, a)
	}
}

// TestHollowKingComesEarlyWithAVolley checks tier 5's rule: faster foes, the Hollow King a minute early, and a fourth attack in his cycle that throws a fan of bones at the keeper.
func TestHollowKingComesEarlyWithAVolley(t *testing.T) {
	r := NewRun(13, 5)
	early := r.data.waves.Boss.At - r.rules.bossEarlier
	if early != r.data.waves.Boss.At-60 || r.director.bossAt != early {
		t.Fatalf("the Hollow King comes at %v, want a minute before %v", r.director.bossAt, r.data.waves.Boss.At)
	}
	r.addEnemy(0, r.keeper.pos.Add(Vec2{X: 400}), 1, 1)
	if got, want := r.enemies[0].speed, r.data.enemies.Enemies[0].Speed*r.rules.enemySpeed; got != want || got <= r.data.enemies.Enemies[0].Speed {
		t.Fatalf("a tier 5 Ash Mite moves at %v, want %v", got, want)
	}
	r.enemies = r.enemies[:0]
	r.time = early
	r.director.step(r)
	if !r.boss.active {
		t.Fatalf("the Hollow King has not come at %v", early)
	}
	if be := r.bossEnemy(); be.maxHP != r.data.waves.Boss.Health*r.rules.health || be.damage != r.data.waves.Boss.Damage*r.rules.damage {
		t.Fatalf("the tier 5 Hollow King has %v health and %v damage", be.maxHP, be.damage)
	}
	seen := false
	for range 60 * 60 {
		r.keeper.health = r.stats.maxHealth
		before := len(r.enemyShots)
		r.stepBoss()
		if r.boss.state == bossVolleyWindup && r.Snapshot().Boss.Telegraph != "volley" {
			t.Fatal("the volley's wind-up shows no telegraph")
		}
		if added := len(r.enemyShots) - before; added == r.rules.volley.Count && r.boss.nextAttack == 0 {
			seen = true
			break
		}
	}
	if !seen {
		t.Fatal("the tier 5 Hollow King never threw his volley")
	}
	if b := NewRun(13, 4); b.director.bossAt != b.data.waves.Boss.At {
		t.Fatal("the Hollow King comes early below tier 5")
	}
}

// TestArenaDrawsShadowsAndTheVolley checks that a frame paints a shadow zone near the keeper and the Hollow King's volley wind-up, each of which adds to what a frame draws.
func TestArenaDrawsShadowsAndTheVolley(t *testing.T) {
	frameOps := func(r *Run) int {
		a := newArenaRenderer(newArt())
		a.startRun(7)
		a.sync(r.Snapshot(), nil, stepSeconds)
		d := &core.Drawing{}
		a.draw(d, 1, runView)
		return len(d.Ops)
	}
	r := NewRun(7, 5)
	bare := frameOps(r)
	r.leaveShadow(r.keeper.pos.Add(Vec2{X: 20}))
	shaded := frameOps(r)
	if shaded <= bare {
		t.Fatalf("a frame with a shadow zone drew %d ops, no more than %d without", shaded, bare)
	}
	r.spawnBoss()
	r.boss.state, r.boss.timer, r.boss.timerMax, r.boss.chargeDir = bossChase, 0, 1, Vec2{X: 1}
	still := frameOps(r)
	r.boss.state = bossVolleyWindup
	if r.Snapshot().Boss.Telegraph != "volley" {
		t.Fatal("the volley's wind-up shows no telegraph")
	}
	if got := frameOps(r); got <= still {
		t.Fatalf("a frame with the volley's wind-up drew %d ops, no more than %d without", got, still)
	}
}
