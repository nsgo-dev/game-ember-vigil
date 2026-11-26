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
	stepOnce(r, Controls{})
	controls := Controls{Move: Vec2{X: 0.5, Y: 0.5}}
	if allocs := testing.AllocsPerRun(120, func() { r.Step(stepTime, controls) }); allocs != 0 {
		t.Fatalf("a step allocated %v times, want none", allocs)
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
