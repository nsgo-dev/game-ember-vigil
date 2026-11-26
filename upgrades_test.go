package main

import (
	"math"
	"testing"
)

// TestOfferPoolLeavesOutMaxedCards checks that a full build is offered healing cards instead of upgrades it cannot take.
func TestOfferPoolLeavesOutMaxedCards(t *testing.T) {
	r := quietRun(t)
	for i := range r.upgradeLevels {
		r.upgradeLevels[i] = r.data.run.Upgrades[i].MaxLevel
	}
	if pool := r.offerPool(); len(pool) != 0 {
		t.Fatalf("a full build still offers %+v", pool)
	}
	r.xp = r.xpNeeded()
	stepOnce(r, Controls{})
	offers := r.Snapshot().Offers
	if len(offers) != offerCount {
		t.Fatalf("a full build was offered %d cards, want %d", len(offers), offerCount)
	}
	for _, o := range offers {
		if !o.Heal || o.Name != r.data.run.Healing.Name {
			t.Fatalf("a full build was offered %+v, want healing cards", o)
		}
	}
}

// TestEachCardRaisesItsStat checks that every card in the pool changes only the stat it names, by its amount per level.
func TestEachCardRaisesItsStat(t *testing.T) {
	for i := range mustGameData().run.Upgrades {
		r := quietRun(t)
		u := &r.data.run.Upgrades[i]
		before := r.stats
		r.phase = PhaseLevelUp
		r.offersNow = []offer{{name: u.Name, stat: u.Stat, level: 1, maxLevel: u.MaxLevel, upgrade: i}}
		if !r.Choose(0) {
			t.Fatalf("the run refused %q", u.Name)
		}
		after := r.stats
		switch u.Stat {
		case statDamage:
			if math.Abs(after.damage-(before.damage+u.Amount)) > 1e-9 || after.cooldown != before.cooldown {
				t.Fatalf("%q changed the stats to %+v, want damage %v", u.Name, after, before.damage+u.Amount)
			}
		case statCooldown:
			if math.Abs(after.cooldown-(before.cooldown-u.Amount)) > 1e-9 || after.damage != before.damage {
				t.Fatalf("%q changed the stats to %+v, want cooldown %v", u.Name, after, before.cooldown-u.Amount)
			}
		case statMoveSpeed:
			if math.Abs(after.moveSpeed-(before.moveSpeed+u.Amount)) > 1e-9 || after.damage != before.damage {
				t.Fatalf("%q changed the stats to %+v, want move speed %v", u.Name, after, before.moveSpeed+u.Amount)
			}
		case statMaxHealth:
			if math.Abs(after.maxHealth-(before.maxHealth+u.Amount)) > 1e-9 || after.damage != before.damage {
				t.Fatalf("%q changed the stats to %+v, want max health %v", u.Name, after, before.maxHealth+u.Amount)
			}
			if r.keeper.health <= before.maxHealth {
				t.Fatalf("%q left the keeper at %v health, want the added health at once", u.Name, r.keeper.health)
			}
		default:
			t.Fatalf("%q has the unknown stat %q", u.Name, u.Stat)
		}
	}
}

// TestCooldownStaysAboveItsFloor checks that a maxed cooldown card cannot make the weapon fire faster than the data allows.
func TestCooldownStaysAboveItsFloor(t *testing.T) {
	r := quietRun(t)
	for i := range r.data.run.Upgrades {
		if r.data.run.Upgrades[i].Stat == statCooldown {
			r.upgradeLevels[i] = r.data.run.Upgrades[i].MaxLevel
		}
	}
	r.recomputeStats()
	if want := r.data.run.Weapon.CooldownMin; math.Abs(r.stats.cooldown-want) > 1e-9 {
		t.Fatalf("a maxed cooldown card left %v seconds between shots, want the floor %v", r.stats.cooldown, want)
	}
}

// TestHealingCardRestoresAFractionOfMaxHealth checks the card that fills a pool short of three upgrades.
func TestHealingCardRestoresAFractionOfMaxHealth(t *testing.T) {
	r := quietRun(t)
	r.keeper.health = 1
	r.phase = PhaseLevelUp
	r.offersNow = []offer{{name: r.data.run.Healing.Name, upgrade: -1, heal: true}}
	if !r.Choose(0) {
		t.Fatal("the run refused the healing card")
	}
	want := math.Min(r.stats.maxHealth, 1+r.stats.maxHealth*r.data.run.Healing.Fraction)
	if math.Abs(r.keeper.health-want) > 1e-9 {
		t.Fatalf("the healing card left %v health, want %v", r.keeper.health, want)
	}
}

// TestChooseOnlyTakesOfferedCards checks that a choice outside the offer changes nothing.
func TestChooseOnlyTakesOfferedCards(t *testing.T) {
	r := quietRun(t)
	r.xp = r.xpNeeded()
	stepOnce(r, Controls{})
	levels := append([]int(nil), r.upgradeLevels...)
	for _, index := range []int{-1, offerCount, 99} {
		if r.Choose(index) {
			t.Fatalf("the run took card %d, which was not offered", index)
		}
	}
	if r.Snapshot().Phase != PhaseLevelUp {
		t.Fatalf("a refused choice resumed the run")
	}
	for i := range levels {
		if r.upgradeLevels[i] != levels[i] {
			t.Fatalf("a refused choice raised upgrade %d", i)
		}
	}
}
