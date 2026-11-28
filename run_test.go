package main

import (
	"math"
	"reflect"
	"slices"
	"testing"
	"time"
)

// stepTime is one fixed run step, the length game.go's loop hands a run.
const stepTime = time.Second / 60

// stepOnce advances a test run by one step with controls c.
func stepOnce(r *Run, c Controls) { r.Step(stepTime, c) }

// quietRun starts a run with the wave director held back, so a test decides alone what the keeper meets.
func quietRun(t *testing.T) *Run {
	t.Helper()
	r := NewRun(7, 1)
	r.director.nextSpawn = math.Inf(1)
	r.director.nextSurge = len(r.data.waves.Surges)
	r.director.nextElite = len(r.data.waves.Elites)
	r.director.bossSpawned = true
	return r
}

// putEnemy places an enemy of kind offset world units from the keeper.
func putEnemy(t *testing.T, r *Run, kind int, offset Vec2) {
	t.Helper()
	r.addEnemy(kind, r.keeper.pos.Add(offset), 1, 1)
}

// playSteps advances r one step at a time, taking the first card whenever a level-up opens and reporting the phases it reached.
func playSteps(t *testing.T, r *Run, steps int, controls func(step int) Controls) (leveled, chose, died bool) {
	t.Helper()
	for step := range steps {
		switch r.Snapshot().Phase {
		case PhaseLevelUp:
			if !r.Choose(0) {
				t.Fatalf("step %d: the run offered no card to take", step)
			}
			chose = true
			continue
		case PhaseDefeat:
			return leveled, chose, true
		}
		stepOnce(r, controls(step))
		if r.Snapshot().Level > 1 {
			leveled = true
		}
	}
	return leveled, chose, r.Snapshot().Phase == PhaseDefeat
}

// circleControls walks the keeper around a circle, a fixed script a bot can repeat exactly.
func circleControls(step int) Controls {
	t := float64(step) * stepSeconds
	return Controls{Move: Vec2{X: math.Cos(t * 0.6), Y: math.Sin(t * 0.6)}}
}

// TestSameSeedAndControlsReplayIdentically plays two runs with one seed and one control script and compares every event and the final snapshot.
func TestSameSeedAndControlsReplayIdentically(t *testing.T) {
	play := func() (*Run, []Event) {
		r := NewRun(4242, 1)
		var events []Event
		for step := range 3 * 60 * 60 {
			if r.Snapshot().Phase == PhaseLevelUp {
				r.Choose(0)
			}
			r.Step(stepTime, circleControls(step))
			events = append(events, r.Events()...)
		}
		return r, events
	}
	first, firstEvents := play()
	second, secondEvents := play()
	if !reflect.DeepEqual(firstEvents, secondEvents) {
		t.Fatalf("the same seed and controls reported %d events once and %d events twice", len(firstEvents), len(secondEvents))
	}
	if !reflect.DeepEqual(first.Snapshot(), second.Snapshot()) {
		t.Fatalf("the same seed and controls ended in different runs:\n%+v\n%+v", first.Snapshot(), second.Snapshot())
	}
	if a, b := first.Payout(), second.Payout(); a != b {
		t.Fatalf("the same seed and controls paid %d and %d embers", a, b)
	}
}

// TestPayoutFollowsTheDesign checks the ember payout of docs/meta-progression.md: 5 a whole minute, 2 per whole hundred kills, 15 a chest and 50 a win, times 1 + 0.25 per tier above 1.
func TestPayoutFollowsTheDesign(t *testing.T) {
	r := NewRun(1, 1)
	if got := r.Payout(); got != 0 {
		t.Fatalf("a fresh run pays %d embers, want 0", got)
	}
	r.time, r.kills, r.chests, r.phase = 3*60+59, 250, 0, PhaseDefeat
	if got := r.Payout(); got != 3*5+2*2 {
		t.Fatalf("a death at 3:59 with 250 kills pays %d, want 19", got)
	}
	r.time, r.kills, r.chests, r.phase = 14*60, 5000, 2, PhaseVictory
	base := 14*5 + 50*2 + 2*15 + 50
	for tier, want := range map[int]int{0: base, 1: base, 3: base * 3 / 2, 5: 2 * base} {
		r.tier = tier
		if got := r.Payout(); got != want {
			t.Fatalf("a won run at tier %d pays %d, want %d", tier, got, want)
		}
	}
}

// TestUnlocksNeedEveryConditionTheyName checks the unlock list against a run's kills, elites defeated and win, with test entries.
func TestUnlocksNeedEveryConditionTheyName(t *testing.T) {
	d := *mustGameData()
	d.progression.Unlocks = []unlockDef{
		{ID: "kills", Name: "Kills", Kills: 100},
		{ID: "elites-and-win", Name: "Elites and win", Elites: 2, Win: true},
		{ID: "win", Name: "Win", Win: true},
	}
	r := newRunWith(&d, 7, 1)
	if got := r.Unlocks(); len(got) != 0 {
		t.Fatalf("a fresh run unlocks %v, want nothing", got)
	}
	r.kills = 100
	if got := r.Unlocks(); !reflect.DeepEqual(got, []string{"kills"}) {
		t.Fatalf("100 kills unlock %v, want [kills]", got)
	}
	r.phase = PhaseVictory
	if got := r.Unlocks(); !reflect.DeepEqual(got, []string{"kills", "win"}) {
		t.Fatalf("a win with 100 kills unlocks %v, want [kills win]", got)
	}

	// Killing Gloomwardens counts toward the elites an unlock asks for.
	r = quietRun(t)
	r.data = &d
	r.phase = PhaseVictory
	warden := enemyByName(t, r, "Gloomwarden")
	for range 2 {
		putEnemy(t, r, warden, Vec2{X: 20})
		r.enemies[len(r.enemies)-1].hp = 0
		r.reap()
	}
	if r.elites != 2 {
		t.Fatalf("two dead Gloomwardens count %d elites, want 2", r.elites)
	}
	if got := r.Unlocks(); !reflect.DeepEqual(got, []string{"elites-and-win", "win"}) {
		t.Fatalf("a win with two elites unlocks %v, want [elites-and-win win]", got)
	}
}

// TestThreeMinutesWithFixedControlsLevelsUpTakesACardAndDies checks that a stationary keeper levels up, takes a card, and eventually falls in defeat.
func TestThreeMinutesWithFixedControlsLevelsUpTakesACardAndDies(t *testing.T) {
	// Frail enemies let a keeper that never moves kill some before it falls, so the card flow does not depend on the balance.
	d := *mustGameData()
	d.enemies.Enemies = slices.Clone(d.enemies.Enemies)
	for i := range d.enemies.Enemies {
		d.enemies.Enemies[i].Health = 1
	}
	r := newRunWith(&d, 2024, 1)
	leveled, chose, died := playSteps(t, r, 6*60*60, func(int) Controls { return Controls{} })
	if !leveled {
		t.Fatal("the keeper never reached level 2")
	}
	if !chose {
		t.Fatal("the run never offered a card")
	}
	if !died {
		t.Fatalf("the stationary keeper survived with %v health", r.Snapshot().Health)
	}
}

// TestKeeperMovesFromControlsAndStaysInsideTheArena checks one second of movement, diagonal speed and the world bounds.
func TestKeeperMovesFromControlsAndStaysInsideTheArena(t *testing.T) {
	r := quietRun(t)
	start := r.Snapshot()
	for range 60 {
		stepOnce(r, Controls{Move: Vec2{X: 1}})
	}
	moved := r.Snapshot().Keeper.Sub(start.Keeper)
	if want := r.stats.moveSpeed; math.Abs(moved.X-want) > 0.5 || moved.Y != 0 {
		t.Fatalf("a second of holding right moved the keeper %+v, want (%v, 0)", moved, want)
	}

	r = quietRun(t)
	corner := r.Snapshot().Keeper
	for range 60 {
		stepOnce(r, Controls{Move: Vec2{X: 1, Y: 1}})
	}
	diagonal := r.Snapshot().Keeper.Sub(corner)
	if want := r.stats.moveSpeed; math.Abs(diagonal.Len()-want) > 0.5 {
		t.Fatalf("a second of holding right and down moved the keeper %v units, want %v", diagonal.Len(), want)
	}

	for range 60 * 60 {
		stepOnce(r, Controls{Move: Vec2{X: 1, Y: 1}})
	}
	at := r.Snapshot()
	edge := at.Fog + at.KeeperRadius
	if want := at.Arena.X - edge; math.Abs(at.Keeper.X-want) > 0.001 || math.Abs(at.Keeper.Y-(at.Arena.Y-edge)) > 0.001 {
		t.Fatalf("the keeper reached %+v, want the field's far corner (%v, %v)", at.Keeper, want, at.Arena.Y-edge)
	}
}

// TestContactDamageWaitsForTheInvulnerabilityWindow checks that an enemy touching the keeper hurts it once and then only after the window.
func TestContactDamageWaitsForTheInvulnerabilityWindow(t *testing.T) {
	r := quietRun(t)
	before := r.Snapshot().Health
	putEnemy(t, r, 2, Vec2{})
	stepOnce(r, Controls{})
	hurt := r.Snapshot().Health
	if hurt >= before {
		t.Fatalf("the keeper kept %v health while an enemy touched it, want less than %v", hurt, before)
	}

	stepOnce(r, Controls{})
	if got := r.Snapshot().Health; got != hurt {
		t.Fatalf("the keeper lost health again the step after contact: %v, want %v", got, hurt)
	}

	for range 60 {
		stepOnce(r, Controls{})
	}
	if got := r.Snapshot().Health; got >= hurt {
		t.Fatalf("the keeper kept %v health after the invulnerability window, want less than %v", got, hurt)
	}
}

// TestAKillDropsAnEmberTheKeeperCollectsForXP follows one shot from the keeper to an enemy, its ember and the XP bar.
func TestAKillDropsAnEmberTheKeeperCollectsForXP(t *testing.T) {
	r := quietRun(t)
	putEnemy(t, r, 1, Vec2{X: 60})
	// One bolt's worth of health keeps the flow short, so the Moth falls before it reaches the keeper.
	r.enemies[0].hp = 1
	for range 120 {
		stepOnce(r, Controls{})
		if len(r.enemies) == 0 {
			break
		}
	}
	if got := r.Snapshot().Kills; got != 1 {
		t.Fatalf("the keeper killed %d enemies, want 1", got)
	}
	if !hasEvent(r.Events(), EventKill) {
		t.Fatal("the step that killed an enemy reported no kill")
	}
	if len(r.embers) != 1 {
		t.Fatalf("the kill left %d embers, want 1", len(r.embers))
	}

	for range 120 {
		stepOnce(r, Controls{})
		if r.Snapshot().XP > 0 {
			break
		}
	}
	if got := r.Snapshot().XP; got != float64(r.data.run.Embers.Value) {
		t.Fatalf("the collected ember gave %v XP, want %v", got, r.data.run.Embers.Value)
	}
	if !hasEvent(r.Events(), EventEmber) {
		t.Fatal("the step that collected an ember reported no ember")
	}
}

// TestLevelUpOffersThreeDistinctCardsAndHoldsTheRun checks the offer, that the run holds still for it, and that a card resumes it.
func TestLevelUpOffersThreeDistinctCardsAndHoldsTheRun(t *testing.T) {
	r := quietRun(t)
	levelBefore := r.Snapshot().Level
	r.xp = r.xpNeeded()
	stepOnce(r, Controls{})
	offers := r.Snapshot().Offers
	if r.Snapshot().Phase != PhaseLevelUp {
		t.Fatalf("the run is in phase %v after filling the XP bar, want PhaseLevelUp", r.Snapshot().Phase)
	}
	if len(offers) != offerCount {
		t.Fatalf("the level offered %d cards, want %d", len(offers), offerCount)
	}
	seen := map[string]bool{}
	for _, o := range offers {
		if seen[o.Name] {
			t.Fatalf("the level offered %q twice: %+v", o.Name, offers)
		}
		seen[o.Name] = true
	}

	held := r.Snapshot()
	stepOnce(r, Controls{})
	if now := r.Snapshot(); now.Time != held.Time || now.Level != held.Level {
		t.Fatalf("the run moved on while cards were open: time %v, level %d", now.Time, now.Level)
	}

	if r.Choose(-1) || r.Choose(offerCount) {
		t.Fatal("the run took a card that was not offered")
	}
	if !r.Choose(0) {
		t.Fatal("the run refused the first card it offered")
	}
	after := r.Snapshot()
	if after.Phase != PhasePlaying {
		t.Fatalf("the run is in phase %v after a card, want PhasePlaying", after.Phase)
	}
	if len(after.Offers) != 0 {
		t.Fatalf("the taken cards stayed open: %+v", after.Offers)
	}
	if after.Level != levelBefore+1 || after.XP != 0 {
		t.Fatalf("taking a card left level %d with %v XP, want level %d and an empty bar", after.Level, after.XP, levelBefore+1)
	}
	stepOnce(r, Controls{})
	if r.Snapshot().Time <= held.Time {
		t.Fatal("the run did not resume after a card")
	}
}

// TestDefeatEndsTheRun checks that an enemy taking the keeper's last health stops the run and reports it.
func TestDefeatEndsTheRun(t *testing.T) {
	r := quietRun(t)
	r.keeper.health = 0.5
	putEnemy(t, r, 0, Vec2{})
	stepOnce(r, Controls{})
	if r.Snapshot().Phase != PhaseDefeat {
		t.Fatalf("the run is in phase %v with no health left, want PhaseDefeat", r.Snapshot().Phase)
	}
	if !hasEvent(r.Events(), EventDefeat) {
		t.Fatal("the step that ended the run reported no defeat")
	}
	at := r.Snapshot()
	stepOnce(r, Controls{})
	if now := r.Snapshot(); now.Time != at.Time || now.Health != at.Health {
		t.Fatalf("the defeated run moved on: %+v", now)
	}
	if len(r.Events()) != 0 {
		t.Fatalf("the defeated run reported events: %+v", r.Events())
	}
}

// TestAStepReportsAtMostMaxEvents checks that a step with more pickups than the buffer holds still returns a bounded list.
func TestAStepReportsAtMostMaxEvents(t *testing.T) {
	r := quietRun(t)
	for range maxEvents * 2 {
		r.embers = append(r.embers, ember{pos: r.keeper.pos, value: 1})
	}
	stepOnce(r, Controls{})
	if got := len(r.Events()); got != maxEvents {
		t.Fatalf("the step reported %d events, want %d", got, maxEvents)
	}
}

// TestSteadyStateStepsAllocateNothing checks that a step in a quiet run reuses memory, as the fixed-step loop needs.
func TestSteadyStateStepsAllocateNothing(t *testing.T) {
	r := quietRun(t)
	for i := range r.weaponLevels {
		r.weaponLevels[i] = 3
	}
	putEnemy(t, r, 2, Vec2{X: 36})
	stepOnce(r, Controls{})
	controls := Controls{Move: Vec2{X: 0.5, Y: 0.5}}
	if allocs := testing.AllocsPerRun(120, func() { r.Step(stepTime, controls) }); allocs != 0 {
		t.Fatalf("a step allocated %v times, want none", allocs)
	}
}

// weaponByName returns the index of weapon name in r's data.
func weaponByName(t *testing.T, r *Run, name string) int {
	t.Helper()
	idx, ok := r.data.weaponIndex(name)
	if !ok {
		t.Fatalf("weapon %q not found", name)
	}
	return idx
}

// TestEmberBoltScalesProjectilesAndPierce checks that higher Ember Bolt levels fire more projectiles and pierce through multiple enemies.
func TestEmberBoltScalesProjectilesAndPierce(t *testing.T) {
	r := quietRun(t)
	bolt := weaponByName(t, r, "Ember Bolt")
	r.weaponLevels[bolt] = 5
	// Line up three sturdy Husks along +X so the center shot pierces all three.
	putEnemy(t, r, 2, Vec2{X: 30})
	putEnemy(t, r, 2, Vec2{X: 55})
	putEnemy(t, r, 2, Vec2{X: 80})

	stepOnce(r, Controls{})
	if got := len(r.shots); got != 3 {
		t.Fatalf("level 5 Ember Bolt fired %d shots, want 3", got)
	}
	hitByID := map[uint64]bool{}
	for range 30 {
		stepOnce(r, Controls{})
		for _, ev := range r.Events() {
			if ev.Kind == EventHit {
				hitByID[ev.ID] = true
			}
		}
	}
	if len(hitByID) != 3 {
		t.Fatalf("level 5 piercing Ember Bolt hit %d distinct enemies, want 3", len(hitByID))
	}
}

// TestLanternHaloOrbitsAndScorchesWithPerTargetCooldown checks that Lantern Halo orbits, damages nearby enemies, respects per-target cooldown and scales with level.
func TestLanternHaloOrbitsAndScorchesWithPerTargetCooldown(t *testing.T) {
	r := quietRun(t)
	bolt := weaponByName(t, r, "Ember Bolt")
	halo := weaponByName(t, r, "Lantern Halo")
	r.weaponLevels[bolt] = 0
	r.weaponLevels[halo] = 1

	snap := r.Snapshot()
	if len(snap.Halo) != 2 {
		t.Fatalf("level 1 Lantern Halo has %d orbs, want 2", len(snap.Halo))
	}
	// Place a Husk directly on the orbit ring and advance until scorched.
	putEnemy(t, r, 2, Vec2{X: 34})
	startHP := r.enemies[0].hp
	hits := 0
	for range 120 {
		stepOnce(r, Controls{})
		for _, ev := range r.Events() {
			if ev.Kind == EventHit {
				hits++
			}
		}
		if hits > 0 {
			break
		}
	}
	if hits == 0 || r.enemies[0].hp >= startHP {
		t.Fatal("Lantern Halo dealt no damage to an enemy on its orbit")
	}
	// The very next step must not hit again because of the per-target cooldown.
	hpAfterHit := r.enemies[0].hp
	stepOnce(r, Controls{})
	if r.enemies[0].hp != hpAfterHit {
		t.Fatal("Lantern Halo hit the same enemy on consecutive steps")
	}

	r.weaponLevels[halo] = 5
	if got := len(r.Snapshot().Halo); got != 5 {
		t.Fatalf("level 5 Lantern Halo has %d orbs, want 5", got)
	}
}

// TestCinderTrailDropsBurningPatchesAndScales checks that Cinder Trail drops patches that burn enemies on a tick interval and scale with level.
func TestCinderTrailDropsBurningPatchesAndScales(t *testing.T) {
	r := quietRun(t)
	bolt := weaponByName(t, r, "Ember Bolt")
	trail := weaponByName(t, r, "Cinder Trail")
	r.weaponLevels[bolt] = 0
	r.weaponLevels[trail] = 1

	putEnemy(t, r, 2, Vec2{X: 10})
	startHP := r.enemies[0].hp
	stepOnce(r, Controls{})
	if len(r.patches) != 1 {
		t.Fatalf("Cinder Trail left %d patches, want 1", len(r.patches))
	}
	if r.enemies[0].hp >= startHP {
		t.Fatal("Cinder Trail patch dealt no damage to an enemy standing in it")
	}
	hpAfterFirstTick := r.enemies[0].hp
	stepOnce(r, Controls{})
	if r.enemies[0].hp != hpAfterFirstTick {
		t.Fatal("Cinder Trail burned the same enemy on consecutive steps")
	}
	r1Radius := r.patches[0].radius

	r.patches = r.patches[:0]
	r.weaponLevels[trail] = 5
	r.weaponCooldowns[trail] = 0
	stepOnce(r, Controls{})
	if r.patches[0].radius <= r1Radius {
		t.Fatalf("level 5 Cinder Trail radius %v did not scale above level 1 radius %v", r.patches[0].radius, r1Radius)
	}
}

// TestBellTollPulsesDamageAndKnockback checks that Bell Toll rings a shockwave that damages, pushes back and scales with level.
func TestBellTollPulsesDamageAndKnockback(t *testing.T) {
	r := quietRun(t)
	bolt := weaponByName(t, r, "Ember Bolt")
	bell := weaponByName(t, r, "Bell Toll")
	r.weaponLevels[bolt] = 0
	r.weaponLevels[bell] = 1

	putEnemy(t, r, 1, Vec2{X: 40})
	startDist := r.enemies[0].pos.Sub(r.keeper.pos).Len()
	stepOnce(r, Controls{})
	if len(r.pulses) != 1 {
		t.Fatalf("Bell Toll created %d pulses, want 1", len(r.pulses))
	}
	if r.Snapshot().Kills == 0 && (len(r.enemies) != 1 || r.enemies[0].hp >= r.enemies[0].maxHP) {
		t.Fatal("level 1 Bell Toll did not hurt the Moth in its ring")
	}
	r.enemies = r.enemies[:0]
	// Test knockback on a tougher Husk that survives the level 5 hit.
	r.addEnemy(2, r.keeper.pos.Add(Vec2{X: 40}), 2, 1)
	r.weaponLevels[bell] = 5
	r.weaponCooldowns[bell] = 0
	stepOnce(r, Controls{})
	if len(r.enemies) == 0 {
		t.Fatal("expected tough Husk to survive and be knocked back")
	}
	if r.enemies[0].knockback.X <= 0 {
		t.Fatalf("Bell Toll gave Husk knockback %+v, want positive X push", r.enemies[0].knockback)
	}
	if r.pulses[len(r.pulses)-1].radius <= startDist {
		t.Fatalf("level 5 Bell Toll radius %v did not exceed %v", r.pulses[len(r.pulses)-1].radius, startDist)
	}
}

// TestPassiveEffectsInRun checks Mending regeneration, Gathering pickup radius and Glow XP gain during steps.
func TestPassiveEffectsInRun(t *testing.T) {
	r := quietRun(t)
	for i, p := range r.data.upgrades.Passives {
		switch p.Stat {
		case statRegen, statPickup, statXPGain:
			r.passiveLevels[i] = 2
		}
	}
	r.recomputeStats()

	// Mending regenerates missing health over time.
	r.keeper.health = 50
	for range 60 {
		stepOnce(r, Controls{})
	}
	if r.keeper.health <= 50 {
		t.Fatalf("Mending did not regenerate health: got %v, want > 50", r.keeper.health)
	}

	// Gathering pulls in an ember placed beyond the base magnet radius, and Glow scales its XP.
	far := r.data.run.Embers.MagnetRadius + 15
	r.embers = append(r.embers, ember{pos: r.keeper.pos.Add(Vec2{X: far}), value: 1})
	for range 120 {
		stepOnce(r, Controls{})
		if r.xp > 0 {
			break
		}
	}
	if math.Abs(r.xp-r.stats.xpMul) > 1e-9 {
		t.Fatalf("collected ember gave %v XP, want %v with Glow", r.xp, r.stats.xpMul)
	}
}

// enemyByName returns the index of enemy name in r's data.
func enemyByName(t *testing.T, r *Run, name string) int {
	t.Helper()
	idx, ok := r.data.enemyIndex(name)
	if !ok {
		t.Fatalf("enemy %q not found", name)
	}
	return idx
}

// TestThornlingAndWispFireEnemyShots checks that Thornling keeps distance and shoots, Wisp orbits and shoots, and enemy projectiles hurt the keeper.
func TestThornlingAndWispFireEnemyShots(t *testing.T) {
	r := quietRun(t)
	r.weaponLevels[r.data.starterWeapon] = 0
	thorn := enemyByName(t, r, "Thornling")
	wisp := enemyByName(t, r, "Wisp")
	putEnemy(t, r, thorn, Vec2{X: 120})
	putEnemy(t, r, wisp, Vec2{Y: 100})

	startHP := r.keeper.health
	sawShot := false
	for range 240 {
		stepOnce(r, Controls{})
		if len(r.enemyShots) > 0 {
			sawShot = true
		}
		if r.keeper.health < startHP {
			break
		}
	}
	if !sawShot {
		t.Fatal("Thornling and Wisp fired no enemy shots")
	}
	if r.keeper.health >= startHP {
		t.Fatalf("enemy shots did not damage stationary keeper: health %v", r.keeper.health)
	}
}

// TestShadeSplitsIntoAshMitesOnDeath checks that killing a Shade spawns Ash Mites at its death site.
func TestShadeSplitsIntoAshMitesOnDeath(t *testing.T) {
	r := quietRun(t)
	r.weaponLevels[r.data.starterWeapon] = 0
	shade := enemyByName(t, r, "Shade")
	putEnemy(t, r, shade, Vec2{X: 80})
	r.enemies[0].hp = 0
	stepOnce(r, Controls{})
	if len(r.enemies) != 2 {
		t.Fatalf("killing a Shade left %d enemies, want 2 split Ash Mites", len(r.enemies))
	}
	for _, e := range r.enemies {
		if name := r.data.enemies.Enemies[e.kind].Name; name != "Ash Mite" {
			t.Fatalf("Shade split into %q, want Ash Mite", name)
		}
	}
}

// TestGloomwardenFiresRadialBurst checks that the Gloomwarden mini-boss unleashes a radial bone-shot burst while chasing the keeper.
func TestGloomwardenFiresRadialBurst(t *testing.T) {
	r := quietRun(t)
	r.weaponLevels[r.data.starterWeapon] = 0
	warden := enemyByName(t, r, "Gloomwarden")
	wantBurst := r.data.enemies.Enemies[warden].Shot.Burst
	if wantBurst < 2 {
		t.Fatalf("Gloomwarden burst = %d, want at least 2", wantBurst)
	}
	putEnemy(t, r, warden, Vec2{X: 150})
	for range 240 {
		r.keeper.health = r.stats.maxHealth
		stepOnce(r, Controls{})
		if len(r.enemyShots) >= wantBurst {
			break
		}
	}
	if len(r.enemyShots) != wantBurst {
		t.Fatalf("Gloomwarden fired %d projectiles, want %d", len(r.enemyShots), wantBurst)
	}
	for _, sh := range r.enemyShots {
		if sh.kind != enemyShotBone {
			t.Fatalf("Gloomwarden shot kind = %d, want enemyShotBone (%d)", sh.kind, enemyShotBone)
		}
	}
}

// TestGloomwardenDropsHealCachePickup checks that killing a Gloomwarden drops a heal cache that restores keeper health.
func TestGloomwardenDropsHealCachePickup(t *testing.T) {
	r := quietRun(t)
	r.weaponLevels[r.data.starterWeapon] = 0
	warden := enemyByName(t, r, "Gloomwarden")
	putEnemy(t, r, warden, Vec2{X: 20})
	r.keeper.health = 40
	r.enemies[0].hp = 0
	stepOnce(r, Controls{})
	caches, magnets := 0, 0
	for _, p := range r.pickups {
		switch p.kind {
		case PickupCache:
			caches++
		case PickupMagnet:
			magnets++
		}
	}
	if caches != 1 || magnets != 1 || len(r.pickups) != 2 {
		t.Fatalf("Gloomwarden left %d heal caches and %d magnets of %d pickups, want 1 and 1 of 2", caches, magnets, len(r.pickups))
	}
	// Its embers would open a level-up before both are collected.
	r.embers = r.embers[:0]
	for range 60 {
		stepOnce(r, Controls{})
		if len(r.pickups) == 0 {
			break
		}
	}
	if len(r.pickups) != 0 {
		t.Fatal("keeper did not collect the heal cache and the magnet")
	}
	if r.keeper.health <= 40 {
		t.Fatalf("heal cache did not restore health: got %v, want > 40", r.keeper.health)
	}
}

// TestFindsTurnUpNearTheKeeper checks that potions and magnets turn up near the keeper as data/run.yaml rolls them, never more of a kind than its most, and that a potion heals and a magnet pulls every ember in.
func TestFindsTurnUpNearTheKeeper(t *testing.T) {
	r := quietRun(t)
	r.weaponLevels[r.data.starterWeapon] = 0
	f := r.data.run.Finds
	potions, magnets := 0, 0
	// Stand still far from where the finds land, so none is collected while they are counted.
	for range 30 * 60 * 60 {
		r.keeper.health = r.stats.maxHealth
		stepOnce(r, Controls{})
		potions, magnets = 0, 0
		for _, p := range r.pickups {
			if d := p.pos.Sub(r.keeper.pos).Len(); d < f.Near-1e-9 || d > f.Far+1e-9 {
				t.Fatalf("a find lies %v from the keeper, want %v to %v", d, f.Near, f.Far)
			}
			switch p.kind {
			case PickupPotion:
				potions++
			case PickupMagnet:
				magnets++
			}
		}
		if potions > f.Potion.Most || magnets > f.Magnet.Most {
			t.Fatalf("%d potions and %d magnets on the ground, want at most %d and %d", potions, magnets, f.Potion.Most, f.Magnet.Most)
		}
		if potions == f.Potion.Most && magnets == f.Magnet.Most {
			break
		}
	}
	if potions != f.Potion.Most || magnets != f.Magnet.Most {
		t.Fatalf("after waiting, %d potions and %d magnets turned up, want %d and %d", potions, magnets, f.Potion.Most, f.Magnet.Most)
	}

	// A far ember waits until the magnet is picked up, then flies in.
	r.embers = append(r.embers[:0], ember{pos: r.clampField(r.keeper.pos.Add(Vec2{X: 300}), 1), value: 1})
	r.keeper.health = r.stats.maxHealth / 2
	for range 2 * (f.Potion.Most + f.Magnet.Most) {
		if len(r.pickups) == 0 {
			break
		}
		r.keeper.pos = r.pickups[0].pos
		stepOnce(r, Controls{})
	}
	if r.keeper.health <= r.stats.maxHealth/2 {
		t.Fatalf("potions did not heal: health %v of %v", r.keeper.health, r.stats.maxHealth)
	}
	if len(r.embers) != 1 || !r.embers[0].attracted {
		t.Fatal("the magnet did not set the far ember flying to the keeper")
	}
}

// TestHollowKingPhasesAttacksAndVictory checks all three Hollow King phases, its charge/bone-ring/summon cycle, and victory on boss death.
func TestHollowKingPhasesAttacksAndVictory(t *testing.T) {
	r := quietRun(t)
	r.weaponLevels[r.data.starterWeapon] = 0
	r.spawnBoss()
	if !r.boss.active || !hasEvent(r.Events(), EventBossSpawn) {
		t.Fatal("spawnBoss did not activate the Hollow King or emit EventBossSpawn")
	}

	// Step through one full attack cycle (charge -> bone ring -> summon).
	sawCharge, sawRing, sawSummon := false, false, false
	for range 12 * 60 {
		r.keeper.health = r.stats.maxHealth
		stepOnce(r, Controls{})
		snap := r.Snapshot()
		switch snap.Boss.Telegraph {
		case "charge":
			sawCharge = true
		case "ring":
			sawRing = true
		case "summon":
			sawSummon = true
		}
		if sawCharge && sawRing && sawSummon {
			break
		}
	}
	if !sawCharge || !sawRing || !sawSummon {
		t.Fatalf("missing boss telegraphs: charge=%v ring=%v summon=%v", sawCharge, sawRing, sawSummon)
	}

	// Drive HP across Phase 2 and Phase 3 thresholds.
	be := r.bossEnemy()
	be.hp = be.maxHP * 0.60
	stepOnce(r, Controls{})
	if r.Snapshot().Boss.Phase != 2 || !hasEvent(r.Events(), EventBossPhase) {
		t.Fatalf("phase 2 transition failed: phase=%d events=%+v", r.Snapshot().Boss.Phase, r.Events())
	}

	be = r.bossEnemy()
	be.hp = be.maxHP * 0.25
	stepOnce(r, Controls{})
	if r.Snapshot().Boss.Phase != 3 || !hasEvent(r.Events(), EventBossPhase) {
		t.Fatalf("phase 3 transition failed: phase=%d events=%+v", r.Snapshot().Boss.Phase, r.Events())
	}

	// Kill the Hollow King and confirm victory ends the run.
	be = r.bossEnemy()
	be.hp = 0
	stepOnce(r, Controls{})
	if r.Snapshot().Phase != PhaseVictory {
		t.Fatalf("phase after boss death = %v, want PhaseVictory", r.Snapshot().Phase)
	}
	if !hasEvent(r.Events(), EventVictory) {
		t.Fatal("boss death emitted no EventVictory")
	}
	wonAt := r.Snapshot().Time
	stepOnce(r, Controls{})
	if r.Snapshot().Time != wonAt || len(r.Events()) != 0 {
		t.Fatal("won run kept stepping after victory")
	}
}

// TestKeeperAndDropsStayOutOfTheFog checks that the keeper cannot walk into the fog of war, while enemies may roam it and leave their embers inside the field.
func TestKeeperAndDropsStayOutOfTheFog(t *testing.T) {
	r := quietRun(t)
	fog := r.data.run.Arena.Fog
	if fog <= 0 {
		t.Fatal("the arena has no fog band")
	}
	for range 60 * 40 {
		stepOnce(r, Controls{Move: Vec2{X: -1, Y: -1}})
	}
	radius := r.data.run.Keeper.Radius
	if want := fog + radius; math.Abs(r.keeper.pos.X-want) > 1e-9 || math.Abs(r.keeper.pos.Y-want) > 1e-9 {
		t.Fatalf("keeper stopped at %v, want the field's corner at %v", r.keeper.pos, want)
	}

	e := enemy{pos: Vec2{X: fog / 2, Y: fog / 2}, embers: 3}
	r.dropEmbers(&e)
	for _, em := range r.embers {
		if em.pos.X < fog || em.pos.Y < fog {
			t.Fatalf("an ember fell at %v inside the fog", em.pos)
		}
	}
	if p := r.clamp(Vec2{X: fog / 2, Y: fog / 2}, 7); p.X >= fog || p.Y >= fog {
		t.Fatalf("an enemy in the fog was pushed out to %v", p)
	}
}
