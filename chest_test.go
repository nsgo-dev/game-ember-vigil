package main

import (
	"math"
	"slices"
	"testing"
)

// putChest lays a chest offset world units from the keeper.
func putChest(r *Run, offset Vec2) {
	r.chestDrops = append(r.chestDrops, chest{id: r.takeID(), pos: r.keeper.pos.Add(offset)})
}

// openChest lays a chest under the keeper and steps once, which opens it.
func openChest(t *testing.T, r *Run) {
	t.Helper()
	putChest(r, Vec2{})
	stepOnce(r, Controls{})
	if r.Snapshot().Phase != PhaseChest || !hasEvent(r.Events(), EventChest) {
		t.Fatalf("a chest under the keeper left the run in phase %v with events %v", r.Snapshot().Phase, r.Events())
	}
}

// holdUntil steps through a chest's hold and returns the phase that follows it.
func holdUntil(t *testing.T, r *Run) Phase {
	t.Helper()
	for range int(r.data.run.Chest.EvolveHold*60) + 2 {
		if p := r.Snapshot().Phase; p != PhaseChest {
			return p
		}
		stepOnce(r, Controls{})
	}
	t.Fatal("the chest's hold never ended")
	return PhaseChest
}

// evolve gives the keeper weapon name at its last level, its paired passive and its evolution.
func evolve(t *testing.T, r *Run, name string) int {
	t.Helper()
	i := weaponByName(t, r, name)
	r.weaponLevels[i] = len(r.data.weapons.Weapons[i].Levels)
	r.passiveLevels[r.data.evolutionPassive[i]] = 1
	r.recomputeStats()
	r.evolved[i] = true
	return i
}

// TestAGloomwardenDropsAChestThatGivesAFreeCard checks that a Gloomwarden leaves a chest, that walking over it holds the run and then offers a card without spending XP, and that it pays its embers.
func TestAGloomwardenDropsAChestThatGivesAFreeCard(t *testing.T) {
	r := quietRun(t)
	r.weaponLevels[r.data.starterWeapon] = 0
	putEnemy(t, r, enemyByName(t, r, "Gloomwarden"), Vec2{X: 40})
	r.enemies[0].hp = 0
	stepOnce(r, Controls{})
	if got := len(r.Snapshot().Chests); got != 1 {
		t.Fatalf("a dead Gloomwarden left %d chests, want 1", got)
	}
	// Its embers wait elsewhere, so the chest's card is the only one the step can bring.
	r.embers = r.embers[:0]
	before := r.Payout()
	level, xp := r.level, r.xp
	r.keeper.pos = r.chestDrops[0].pos
	stepOnce(r, Controls{})
	snap := r.Snapshot()
	if snap.Phase != PhaseChest || !snap.Opening.Active || snap.Opening.Evolved != "" || len(snap.Chests) != 0 {
		t.Fatalf("opening the chest showed %+v in phase %v with %d chests left", snap.Opening, snap.Phase, len(snap.Chests))
	}
	if got := holdUntil(t, r); got != PhaseLevelUp {
		t.Fatalf("a chest that evolved nothing ended its hold in phase %v, want a level-up", got)
	}
	if len(r.Snapshot().Offers) != offerCount || r.level != level || r.xp != xp {
		t.Fatalf("the free card offered %d cards at level %d with %v XP, want %d cards at level %d with %v XP", len(r.Snapshot().Offers), r.level, r.xp, offerCount, level, xp)
	}
	if got := r.Payout() - before; got != r.data.progression.Payout.PerChest {
		t.Fatalf("the chest paid %d embers, want %d", got, r.data.progression.Payout.PerChest)
	}
	if !r.Choose(0) || r.Snapshot().Phase != PhasePlaying {
		t.Fatal("taking the free card did not resume the run")
	}
}

// TestAChestEvolvesOnlyOnceEvolutionIsUnlockedAndARecipeIsMet checks that a chest evolves nothing before the first win or without the paired passive, and evolves a maxed weapon held with it after.
func TestAChestEvolvesOnlyOnceEvolutionIsUnlockedAndARecipeIsMet(t *testing.T) {
	r := quietRun(t)
	bolt := weaponByName(t, r, "Ember Bolt")
	r.weaponLevels[bolt] = len(r.data.weapons.Weapons[bolt].Levels)
	oil := r.data.evolutionPassive[bolt]
	r.passiveLevels[oil] = 1
	r.recomputeStats()

	openChest(t, r)
	if holdUntil(t, r) != PhaseLevelUp || len(r.Evolutions()) != 0 {
		t.Fatal("a chest evolved a weapon before evolution was unlocked")
	}
	r.Choose(0)

	r.EnableEvolution()
	r.passiveLevels[oil] = 0
	openChest(t, r)
	if holdUntil(t, r) != PhaseLevelUp || len(r.Evolutions()) != 0 {
		t.Fatal("a chest evolved Ember Bolt without Oil")
	}
	r.Choose(0)

	r.passiveLevels[oil] = 1
	putChest(r, Vec2{})
	stepOnce(r, Controls{})
	if !hasEvent(r.Events(), EventEvolve) || r.Snapshot().Opening.Evolved != "starfall-volley" {
		t.Fatalf("a chest did not evolve a maxed Ember Bolt held with Oil: %+v", r.Snapshot().Opening)
	}
	if got := holdUntil(t, r); got != PhasePlaying {
		t.Fatalf("an evolving chest ended its hold in phase %v, want playing", got)
	}
	if !slices.Equal(r.Evolutions(), []string{"starfall-volley"}) {
		t.Fatalf("the run made %v, want [starfall-volley]", r.Evolutions())
	}
	slot := r.Snapshot().Weapons[0]
	if slot.ID != "starfall-volley" || slot.Name != "Starfall Volley" || !slot.Evolved {
		t.Fatalf("the HUD slot shows %+v, want the evolved Starfall Volley", slot)
	}

	// With every recipe made, the next chest falls back to the free card.
	openChest(t, r)
	if holdUntil(t, r) != PhaseLevelUp || len(r.Evolutions()) != 1 {
		t.Fatal("a chest with nothing left to evolve did not give its free card")
	}
}

// TestStarfallVolleySplitsOnHit checks that an evolved bolt breaks into its small shots where it strikes, and that they spare the foe it struck.
func TestStarfallVolleySplitsOnHit(t *testing.T) {
	r := quietRun(t)
	evo := r.data.weapons.Weapons[evolve(t, r, "Ember Bolt")].Evolution
	putEnemy(t, r, enemyByName(t, r, "Husk"), Vec2{X: 30})
	target := r.enemies[0].id
	for range 30 {
		stepOnce(r, Controls{})
		small := 0
		for _, s := range r.Snapshot().Shots {
			if s.Small {
				small++
			}
		}
		if small == 0 {
			continue
		}
		if small%evo.Split != 0 {
			t.Fatalf("a hit split into %d small shots, want a multiple of %d", small, evo.Split)
		}
		for i := range r.shots {
			if r.shots[i].small && (!r.shots[i].alreadyHit(target) || r.shots[i].split != 0) {
				t.Fatal("a small shot may strike the foe it split from or split again")
			}
		}
		return
	}
	t.Fatal("Starfall Volley never split")
}

// TestSunRingBurnsTheWholeOrbit checks that the evolved halo draws one ring instead of orbs and burns every foe on its orbit in one step.
func TestSunRingBurnsTheWholeOrbit(t *testing.T) {
	r := quietRun(t)
	r.weaponLevels[r.data.starterWeapon] = 0
	halo := evolve(t, r, "Lantern Halo")
	w := &r.data.weapons.Weapons[halo]
	orbit := w.Levels[len(w.Levels)-1].Radius * r.stats.areaMul
	snap := r.Snapshot()
	if len(snap.Halo) != 0 || !snap.Ring.Active || math.Abs(snap.Ring.Radius-orbit) > 1e-9 {
		t.Fatalf("Sun Ring shows %d orbs and ring %+v, want no orbs and a ring at %v", len(snap.Halo), snap.Ring, orbit)
	}
	husk := enemyByName(t, r, "Husk")
	for k := range 4 {
		a := float64(k) * math.Pi / 2
		putEnemy(t, r, husk, Vec2{X: math.Cos(a) * orbit, Y: math.Sin(a) * orbit})
	}
	stepOnce(r, Controls{})
	for i := range r.enemies {
		if r.enemies[i].hp >= r.enemies[i].maxHP {
			t.Fatalf("Sun Ring spared the foe at quarter %d of its orbit", i)
		}
	}
}

// TestWildfirePathSpreadsSidewaysAndMerges checks that the evolved trail's patches reach past the keeper's sides and merge instead of piling up.
func TestWildfirePathSpreadsSidewaysAndMerges(t *testing.T) {
	r := quietRun(t)
	r.weaponLevels[r.data.starterWeapon] = 0
	trail := evolve(t, r, "Cinder Trail")
	w := &r.data.weapons.Weapons[trail]
	radius := w.Levels[len(w.Levels)-1].Radius * r.stats.areaMul
	stepOnce(r, Controls{Move: Vec2{X: 1}})
	if len(r.patches) != 1 || !r.patches[0].wild {
		t.Fatalf("one drop laid %d patches, want its three merged into one wild patch", len(r.patches))
	}
	p := r.patches[0]
	if reach := math.Abs(p.pos.Y-r.keeper.pos.Y) + p.radius; reach < radius+w.Evolution.Spread*r.stats.areaMul {
		t.Fatalf("Wildfire Path reaches %v to the side, want at least %v", reach, radius+w.Evolution.Spread)
	}
	drops := 0
	for range 10 * 60 {
		before := r.weaponCooldowns[trail]
		stepOnce(r, Controls{Move: Vec2{X: math.Cos(float64(drops)), Y: 1}})
		if r.weaponCooldowns[trail] > before {
			drops++
		}
		for i := range r.patches {
			if r.patches[i].radius > w.Evolution.MaxRadius*r.stats.areaMul+1e-9 {
				t.Fatalf("a merged patch grew to %v, past maxRadius", r.patches[i].radius)
			}
		}
	}
	if len(r.patches) >= 3*drops/4 {
		t.Fatalf("%d drops left %d patches; touching patches should merge", drops, len(r.patches))
	}
}

// TestDawnBellStunsAndHeals checks that the evolved bell holds the foes it strikes still and heals the keeper a little.
func TestDawnBellStunsAndHeals(t *testing.T) {
	r := quietRun(t)
	r.weaponLevels[r.data.starterWeapon] = 0
	bell := evolve(t, r, "Bell Toll")
	evo := r.data.weapons.Weapons[bell].Evolution
	r.keeper.health = 50
	putEnemy(t, r, enemyByName(t, r, "Husk"), Vec2{X: 30})
	stepOnce(r, Controls{})
	if got := r.keeper.health; math.Abs(got-50-evo.Heal) > 1e-9 {
		t.Fatalf("Dawn Bell left the keeper at %v health, want %v", got, 50+evo.Heal)
	}
	if !r.Snapshot().Enemies[0].Stunned || !r.Snapshot().Pulses[0].Dawn {
		t.Fatal("Dawn Bell did not stun the foe it struck or show its pulse")
	}
	dist := r.enemies[0].pos.Sub(r.keeper.pos).Len()
	for range int(evo.Stun*60) - 1 {
		stepOnce(r, Controls{})
		d := r.enemies[0].pos.Sub(r.keeper.pos).Len()
		if d < dist-1e-9 {
			t.Fatal("a stunned foe walked toward the keeper")
		}
		dist = d
	}
}

// TestEvolvedRunsReplayIdentically plays a run with evolution unlocked twice from one seed and one bot and compares what it made and where it ended.
func TestEvolvedRunsReplayIdentically(t *testing.T) {
	r1, res1 := playBot(3, 1, 16, 1, true)
	r2, res2 := playBot(3, 1, 16, 1, true)
	if len(res1.evolved) == 0 {
		t.Fatal("the evolved bot made no evolution")
	}
	if res1.eventCount != res2.eventCount || !slices.Equal(res1.evolved, res2.evolved) || r1.Payout() != r2.Payout() {
		t.Fatalf("seed 3 with evolution did not replay identically: events %d vs %d, evolutions %v vs %v", res1.eventCount, res2.eventCount, res1.evolved, res2.evolved)
	}
}
