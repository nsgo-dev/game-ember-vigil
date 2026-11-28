package main

import (
	"math"
	"slices"
	"strconv"
	"strings"
	"testing"

	evstrings "me.oneo/embervigil/strings"
)

// The unlocks in data/progression.yaml that open the two later weapons.
const (
	unlockSparkChain = "spark-chain"
	unlockAshCenser  = "ash-censer"
)

// offeredIDs lists the ids of every card r's offer pool holds now.
func offeredIDs(r *Run) []string {
	var ids []string
	for _, o := range r.offerPool() {
		ids = append(ids, o.id)
	}
	return ids
}

// TestLockedWeaponsWaitForTheirUnlocks checks that a run never offers Spark Chain or Ash Censer until the shell names their unlocks, which a run meets with 3000 kills and three Gloomwardens.
func TestLockedWeaponsWaitForTheirUnlocks(t *testing.T) {
	r := NewRun(5, 1)
	for _, id := range []string{unlockSparkChain, unlockAshCenser} {
		if slices.Contains(offeredIDs(r), id) {
			t.Fatalf("a run offers %q before it is unlocked", id)
		}
	}
	for range 50 {
		for _, o := range r.drawOffers() {
			if o.id == unlockSparkChain || o.id == unlockAshCenser {
				t.Fatalf("an offer dealt the locked %q", o.id)
			}
		}
	}
	r.UnlockWeapons([]string{unlockSparkChain})
	if ids := offeredIDs(r); !slices.Contains(ids, unlockSparkChain) || slices.Contains(ids, unlockAshCenser) {
		t.Fatalf("with Spark Chain unlocked the pool holds %v", ids)
	}
	r.UnlockWeapons([]string{unlockAshCenser})
	if ids := offeredIDs(r); !slices.Contains(ids, unlockSparkChain) || !slices.Contains(ids, unlockAshCenser) {
		t.Fatalf("with both unlocked the pool holds %v", ids)
	}

	r = quietRun(t)
	r.kills = 4999
	if slices.Contains(r.Unlocks(), unlockSparkChain) {
		t.Fatal("4999 kills unlocked Spark Chain")
	}
	r.kills = 5000
	if !slices.Contains(r.Unlocks(), unlockSparkChain) {
		t.Fatal("5000 kills did not unlock Spark Chain")
	}
	warden := enemyByName(t, r, "Gloomwarden")
	for k := range 7 {
		if slices.Contains(r.Unlocks(), unlockAshCenser) {
			t.Fatalf("%d Gloomwardens unlocked Ash Censer", k)
		}
		putEnemy(t, r, warden, Vec2{X: 20})
		r.enemies[len(r.enemies)-1].hp = 0
		r.reap()
	}
	if !slices.Contains(r.Unlocks(), unlockAshCenser) {
		t.Fatal("seven Gloomwardens did not unlock Ash Censer")
	}
}

// TestWeaponDescriptionsMatchTheirNumbers checks that Spark Chain's card names its jumps at the first and last level in every language.
func TestWeaponDescriptionsMatchTheirNumbers(t *testing.T) {
	r := NewRun(1, 1)
	w := &r.data.weapons.Weapons[weaponByName(t, r, "Spark Chain")]
	first, last := strconv.Itoa(w.Levels[0].Count), strconv.Itoa(w.Levels[len(w.Levels)-1].Count)
	for _, lang := range evstrings.Languages {
		d := evstrings.For(lang).Cards[w.ID].Description
		if i := strings.Index(d, first); i < 0 || !strings.Contains(d[i+1:], last) {
			t.Fatalf("%s: Spark Chain reads %q, want %s jumps rising to %s", lang, d, first, last)
		}
	}
	evo := w.Evolution
	if evo.Fork != 1 || !strings.Contains(evo.Description, "one more") {
		t.Fatalf("Ember Storm forks to %d foes but reads %q", evo.Fork, evo.Description)
	}
}

// chainRun is a quiet run holding only Spark Chain at level, with a row of Husks every gap units to the keeper's right.
func chainRun(t *testing.T, level, husks int, gap float64) (*Run, int) {
	t.Helper()
	r := quietRun(t)
	r.weaponLevels[r.data.starterWeapon] = 0
	chain := weaponByName(t, r, "Spark Chain")
	r.weaponLevels[chain] = level
	husk := enemyByName(t, r, "Husk")
	for k := range husks {
		putEnemy(t, r, husk, Vec2{X: 30 + gap*float64(k)})
	}
	return r, chain
}

// struckFoes counts the foes below full health.
func struckFoes(r *Run) int {
	n := 0
	for i := range r.enemies {
		if r.enemies[i].hp < r.enemies[i].maxHP {
			n++
		}
	}
	return n
}

// TestSparkChainLeapsBetweenFoes checks that the spark strikes as many foes as its level's jumps, each once, leaping only within its jump range, and shows each leap.
func TestSparkChainLeapsBetweenFoes(t *testing.T) {
	r, chain := chainRun(t, 1, 8, 40)
	w := &r.data.weapons.Weapons[chain]
	stepOnce(r, Controls{})
	if got, want := struckFoes(r), w.Levels[0].Count; got != want {
		t.Fatalf("a level 1 spark struck %d foes, want %d", got, want)
	}
	snap := r.Snapshot()
	if len(snap.Arcs) != w.Levels[0].Count || snap.Arcs[0].From != r.keeper.pos {
		t.Fatalf("the spark shows %d leaps, the first from %v; want %d from the keeper", len(snap.Arcs), snap.Arcs[0].From, w.Levels[0].Count)
	}
	for range int(w.Duration*60) + 1 {
		stepOnce(r, Controls{})
	}
	if n := len(r.Snapshot().Arcs); n != 0 {
		t.Fatalf("%d leaps outlived their %vs", n, w.Duration)
	}

	r, chain = chainRun(t, 5, 8, 40)
	stepOnce(r, Controls{})
	if got, want := struckFoes(r), r.data.weapons.Weapons[chain].Levels[4].Count; got != want {
		t.Fatalf("a level 5 spark struck %d foes, want %d", got, want)
	}

	// Foes further apart than the jump range stop the spark at the first.
	r, chain = chainRun(t, 5, 3, r.data.weapons.Weapons[chain].JumpRange*1.5)
	stepOnce(r, Controls{})
	if got := struckFoes(r); got != 1 {
		t.Fatalf("a spark leapt past its jump range to %d foes, want 1", got)
	}
}

// TestEmberStormForksEveryJump checks that the evolved spark strikes one more foe beside every foe it leaps to.
func TestEmberStormForksEveryJump(t *testing.T) {
	r := quietRun(t)
	r.weaponLevels[r.data.starterWeapon] = 0
	chain := evolve(t, r, "Spark Chain")
	w := &r.data.weapons.Weapons[chain]
	husk := enemyByName(t, r, "Husk")
	for k := range 12 {
		putEnemy(t, r, husk, Vec2{X: 30 + 30*float64(k/2), Y: 20 * float64(k%2)})
	}
	stepOnce(r, Controls{})
	want := w.Levels[len(w.Levels)-1].Count * (1 + w.Evolution.Fork)
	if got := struckFoes(r); got != want {
		t.Fatalf("Ember Storm struck %d foes, want %d", got, want)
	}
	for _, a := range r.Snapshot().Arcs {
		if !a.Storm {
			t.Fatal("an Ember Storm leap does not show as one")
		}
	}
}

// censerRun is a quiet run holding only Ash Censer at its last level, evolved when pyre is set.
func censerRun(t *testing.T, pyre bool) (*Run, int) {
	t.Helper()
	r := quietRun(t)
	r.weaponLevels[r.data.starterWeapon] = 0
	if pyre {
		return r, evolve(t, r, "Ash Censer")
	}
	i := weaponByName(t, r, "Ash Censer")
	r.weaponLevels[i] = len(r.data.weapons.Weapons[i].Levels)
	return r, i
}

// putTough places a Husk offset from the keeper that no censer burst kills, so a test can read where it went and what it lost.
func putTough(t *testing.T, r *Run, offset Vec2) {
	t.Helper()
	r.addEnemy(enemyByName(t, r, "Husk"), r.keeper.pos.Add(offset), 50, 1)
}

// stepUntilBurst steps r until a censer bursts and returns how many steps it took.
func stepUntilBurst(t *testing.T, r *Run) int {
	t.Helper()
	for step := range 5 * 60 {
		stepOnce(r, Controls{})
		for _, p := range r.Snapshot().Pulses {
			if p.Censer {
				return step + 1
			}
		}
	}
	t.Fatal("the censer never burst")
	return 0
}

// TestAshCenserBurstsOnTheDensestCrowd checks that the censer flies to the crowd rather than the nearest foe, bursts there after its flight and leaves a fire that burns.
func TestAshCenserBurstsOnTheDensestCrowd(t *testing.T) {
	r, censer := censerRun(t, false)
	w := &r.data.weapons.Weapons[censer]
	putTough(t, r, Vec2{X: 50})
	for k := range 4 {
		putTough(t, r, Vec2{X: -120 + 8*float64(k%2), Y: 8 * float64(k/2)})
	}
	stepOnce(r, Controls{})
	cs := r.Snapshot().Censers
	if len(cs) != 1 || cs[0].Progress > 0.1 || cs[0].Pos.Sub(r.keeper.pos).Len() > 10 {
		t.Fatalf("the censer shows %+v, want one leaving the keeper", cs)
	}
	if steps := stepUntilBurst(t, r) + 1; math.Abs(float64(steps)-w.Flight*60) > 1 {
		t.Fatalf("the censer burst after %d steps, want its %vs flight", steps, w.Flight)
	}
	if r.enemies[0].hp < r.enemies[0].maxHP {
		t.Fatal("the burst struck the lone foe nearer the keeper")
	}
	for i := 1; i < len(r.enemies); i++ {
		if r.enemies[i].hp >= r.enemies[i].maxHP {
			t.Fatalf("the burst spared crowd foe %d", i)
		}
	}
	fire := -1
	for i := range r.patches {
		if r.patches[i].censer {
			fire = i
		}
	}
	if fire < 0 {
		t.Fatal("the burst left no fire")
	}
	if p := r.patches[fire]; math.Abs(p.radius-w.Levels[4].Radius*w.FireRadius) > 1e-9 {
		t.Fatalf("the fire is %v wide, want %v", p.radius, w.Levels[4].Radius*w.FireRadius)
	}
}

// TestPyreCenserPullsFoesAndEmbersFirst checks that the evolved censer draws foes and embers toward where it lands before it bursts.
func TestPyreCenserPullsFoesAndEmbersFirst(t *testing.T) {
	r, censer := censerRun(t, true)
	evo := r.data.weapons.Weapons[censer].Evolution
	for k := range 4 {
		putTough(t, r, Vec2{X: -120 + 8*float64(k%2), Y: 8 * float64(k/2)})
	}
	stray := len(r.enemies)
	putTough(t, r, Vec2{X: -120, Y: 60})
	r.embers = append(r.embers, ember{pos: r.keeper.pos.Add(Vec2{X: -120, Y: -60}), value: 1})
	stepOnce(r, Controls{})
	to := r.censers[0].to
	strayDist := r.enemies[stray].pos.Sub(to).Len()
	emberDist := r.embers[0].pos.Sub(to).Len()
	pulled := false
	for range 5 * 60 {
		stepOnce(r, Controls{})
		if len(r.censers) > 0 && r.censers[0].landed && r.censers[0].pulling > 0 {
			pulled = true
			if len(r.Snapshot().Censers) != 1 || r.Snapshot().Censers[0].Pulling <= 0 {
				t.Fatal("a pulling Pyre Censer does not show its pull")
			}
		}
		if len(r.censers) == 0 {
			break
		}
	}
	if !pulled {
		t.Fatalf("Pyre Censer burst without pulling for %vs", evo.Pull)
	}
	if d := r.enemies[stray].pos.Sub(to).Len(); d >= strayDist-1 {
		t.Fatalf("a foe %v from the censer stayed %v away", strayDist, d)
	}
	if d := r.embers[0].pos.Sub(to).Len(); d >= emberDist-1 {
		t.Fatalf("an ember %v from the censer stayed %v away", emberDist, d)
	}
}

// TestNewWeaponsReplayIdentically plays a run with both later weapons unlocked twice from one seed and one bot and compares where it ended.
func TestNewWeaponsReplayIdentically(t *testing.T) {
	r1, res1 := playBot(9, 1, 6, 1, true)
	r2, res2 := playBot(9, 1, 6, 1, true)
	held := false
	for _, s := range r1.Snapshot().Weapons {
		held = held || s.ID == "spark-chain" || s.ID == "ash-censer" || s.ID == "ember-storm" || s.ID == "pyre-censer"
	}
	if !held {
		t.Fatalf("the veteran bot took neither later weapon: %+v", r1.Snapshot().Weapons)
	}
	if res1.eventCount != res2.eventCount || r1.Payout() != r2.Payout() || r1.Snapshot().Time != r2.Snapshot().Time {
		t.Fatalf("seed 9 with the later weapons did not replay identically: events %d vs %d", res1.eventCount, res2.eventCount)
	}
}
