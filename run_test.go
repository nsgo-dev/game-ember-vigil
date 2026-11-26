package main

import (
	"math"
	"reflect"
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
	r := NewRun(7)
	r.director.nextSpawn = math.Inf(1)
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
		r := NewRun(4242)
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
}

// TestThreeMinutesWithFixedControlsLevelsUpTakesACardAndDies is the acceptance test for the headless model: a bot plays three minutes of the test timeline, takes a card and loses the run.
func TestThreeMinutesWithFixedControlsLevelsUpTakesACardAndDies(t *testing.T) {
	r := NewRun(2024)
	leveled, chose, died := playSteps(t, r, 3*60*60, circleControls)
	if !leveled {
		t.Fatal("the keeper never reached level 2 in three minutes")
	}
	if !chose {
		t.Fatal("the run never offered a card")
	}
	if !died {
		t.Fatalf("the keeper survived three minutes with %v health", r.Snapshot().Health)
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
	if want := at.Arena.X - at.KeeperRadius; math.Abs(at.Keeper.X-want) > 0.001 || math.Abs(at.Keeper.Y-(at.Arena.Y-at.KeeperRadius)) > 0.001 {
		t.Fatalf("the keeper reached %+v, want the arena's far corner (%v, %v)", at.Keeper, want, at.Arena.Y-at.KeeperRadius)
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
	putEnemy(t, r, 1, Vec2{X: 30})
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

	putEnemy(t, r, 0, Vec2{X: 40})
	startDist := r.enemies[0].pos.Sub(r.keeper.pos).Len()
	stepOnce(r, Controls{})
	if len(r.pulses) != 1 {
		t.Fatalf("Bell Toll created %d pulses, want 1", len(r.pulses))
	}
	if r.Snapshot().Kills != 1 {
		// Ash Mite has 10 HP, Bell Toll lv1 deals 12 damage -> killed immediately.
		t.Fatalf("level 1 Bell Toll kills = %d, want 1", r.Snapshot().Kills)
	}
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

// hasEvent reports whether events holds one of kind.
func hasEvent(events []Event, kind EventKind) bool {
	for _, e := range events {
		if e.Kind == kind {
			return true
		}
	}
	return false
}
