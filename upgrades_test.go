package main

import (
	"math"
	"testing"
)

// TestOfferPoolLeavesOutMaxedCards checks that a full build is offered healing cards instead of upgrades it cannot take.
func TestOfferPoolLeavesOutMaxedCards(t *testing.T) {
	r := quietRun(t)
	for i := range r.weaponLevels {
		r.weaponLevels[i] = len(r.data.weapons.Weapons[i].Levels)
	}
	for i := range r.data.upgrades.PassiveSlots {
		r.passiveLevels[i] = r.data.upgrades.Passives[i].MaxLevel
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
		if !o.Heal || o.Name != r.data.upgrades.Healing.Name {
			t.Fatalf("a full build was offered %+v, want healing cards", o)
		}
	}
}

// TestOfferPoolRespectsSlotCaps checks that once four passive slots are filled, unowned passives are excluded while owned ones can still level up.
func TestOfferPoolRespectsSlotCaps(t *testing.T) {
	r := quietRun(t)
	for i := range r.data.upgrades.PassiveSlots {
		r.passiveLevels[i] = 1
	}
	pool := r.offerPool()
	for _, o := range pool {
		if o.kind == offerPassive && r.passiveLevels[o.index] == 0 {
			t.Fatalf("offer pool included unowned passive %q when passive slots are full", o.name)
		}
	}
	if r.ownedPassives() != r.data.upgrades.PassiveSlots {
		t.Fatalf("owned passives = %d, want %d", r.ownedPassives(), r.data.upgrades.PassiveSlots)
	}
}

// TestRunCanReachFullBuildAndOffersAllEightPassives verifies a run can acquire 4 weapons + 4 passives without exceeding slots or repeating cards, and that every passive appears in offers.
func TestRunCanReachFullBuildAndOffersAllEightPassives(t *testing.T) {
	seenPassives := map[string]bool{}
	for seed := uint64(1); seed <= 25; seed++ {
		r := NewRun(seed)
		r.director.nextSpawn = math.Inf(1)
		for range 40 {
			r.xp = r.xpNeeded()
			stepOnce(r, Controls{})
			offers := r.Snapshot().Offers
			if len(offers) != offerCount {
				t.Fatalf("seed %d: offered %d cards, want %d", seed, len(offers), offerCount)
			}
			nonHealSeen := map[string]bool{}
			for _, o := range offers {
				if o.Kind == offerPassive {
					seenPassives[o.Name] = true
				}
				if !o.Heal {
					if nonHealSeen[o.Name] {
						t.Fatalf("seed %d: offered %q twice in %+v", seed, o.Name, offers)
					}
					nonHealSeen[o.Name] = true
				}
			}
			if !r.Choose(0) {
				t.Fatalf("seed %d: Choose(0) failed", seed)
			}
			if r.ownedWeapons() > r.data.upgrades.WeaponSlots {
				t.Fatalf("seed %d: owned weapons %d exceeds cap %d", seed, r.ownedWeapons(), r.data.upgrades.WeaponSlots)
			}
			if r.ownedPassives() > r.data.upgrades.PassiveSlots {
				t.Fatalf("seed %d: owned passives %d exceeds cap %d", seed, r.ownedPassives(), r.data.upgrades.PassiveSlots)
			}
		}
		if seed == 1 {
			if r.ownedWeapons() != 4 || r.ownedPassives() != 4 {
				t.Fatalf("after 40 levels owned %d weapons and %d passives, want 4 and 4", r.ownedWeapons(), r.ownedPassives())
			}
		}
	}
	for _, p := range mustGameData().upgrades.Passives {
		if !seenPassives[p.Name] {
			t.Fatalf("passive %q was never offered across 25 seeds", p.Name)
		}
	}
}

// TestEachPassiveRaisesItsStat checks that every passive in data/upgrades.yaml changes only the stat it names, by its amount per level.
func TestEachPassiveRaisesItsStat(t *testing.T) {
	for i := range mustGameData().upgrades.Passives {
		r := quietRun(t)
		p := &r.data.upgrades.Passives[i]
		before := r.stats
		r.phase = PhaseLevelUp
		r.offersNow = []offer{{
			id: p.ID, name: p.Name, kind: offerPassive, stat: p.Stat,
			description: p.Description, level: 1, maxLevel: p.MaxLevel, index: i,
		}}
		if !r.Choose(0) {
			t.Fatalf("the run refused %q", p.Name)
		}
		after := r.stats
		switch p.Stat {
		case statDamage:
			if math.Abs(after.damageMul-(before.damageMul+p.Amount)) > 1e-9 || after.cooldownMul != before.cooldownMul {
				t.Fatalf("%q changed stats to %+v, want damageMul %v", p.Name, after, before.damageMul+p.Amount)
			}
		case statCooldown:
			if math.Abs(after.cooldownMul-(before.cooldownMul-p.Amount)) > 1e-9 || after.damageMul != before.damageMul {
				t.Fatalf("%q changed stats to %+v, want cooldownMul %v", p.Name, after, before.cooldownMul-p.Amount)
			}
		case statArea:
			if math.Abs(after.areaMul-(before.areaMul+p.Amount)) > 1e-9 || after.damageMul != before.damageMul {
				t.Fatalf("%q changed stats to %+v, want areaMul %v", p.Name, after, before.areaMul+p.Amount)
			}
		case statMoveSpeed:
			if math.Abs(after.moveSpeed-(before.moveSpeed+p.Amount)) > 1e-9 || after.damageMul != before.damageMul {
				t.Fatalf("%q changed stats to %+v, want moveSpeed %v", p.Name, after, before.moveSpeed+p.Amount)
			}
		case statMaxHealth:
			if math.Abs(after.maxHealth-(before.maxHealth+p.Amount)) > 1e-9 || after.damageMul != before.damageMul {
				t.Fatalf("%q changed stats to %+v, want maxHealth %v", p.Name, after, before.maxHealth+p.Amount)
			}
			if r.keeper.health <= before.maxHealth {
				t.Fatalf("%q left the keeper at %v health, want the added health at once", p.Name, r.keeper.health)
			}
		case statRegen:
			if math.Abs(after.regen-(before.regen+p.Amount)) > 1e-9 || after.damageMul != before.damageMul {
				t.Fatalf("%q changed stats to %+v, want regen %v", p.Name, after, before.regen+p.Amount)
			}
		case statPickup:
			if math.Abs(after.pickupRadius-(before.pickupRadius+p.Amount)) > 1e-9 || after.damageMul != before.damageMul {
				t.Fatalf("%q changed stats to %+v, want pickupRadius %v", p.Name, after, before.pickupRadius+p.Amount)
			}
		case statXPGain:
			if math.Abs(after.xpMul-(before.xpMul+p.Amount)) > 1e-9 || after.damageMul != before.damageMul {
				t.Fatalf("%q changed stats to %+v, want xpMul %v", p.Name, after, before.xpMul+p.Amount)
			}
		default:
			t.Fatalf("%q has the unknown stat %q", p.Name, p.Stat)
		}
	}
}

// TestCooldownStaysAboveItsFloor checks that a maxed cooldown passive cannot make any weapon fire faster than its data floor allows.
func TestCooldownStaysAboveItsFloor(t *testing.T) {
	r := quietRun(t)
	for i := range r.data.upgrades.Passives {
		if r.data.upgrades.Passives[i].Stat == statCooldown {
			r.passiveLevels[i] = r.data.upgrades.Passives[i].MaxLevel
		}
	}
	r.recomputeStats()
	for i := range r.data.weapons.Weapons {
		w := &r.data.weapons.Weapons[i]
		for _, lvl := range w.Levels {
			if cd := r.weaponCooldown(w, lvl.Cooldown); cd < w.CooldownMin-1e-9 {
				t.Fatalf("weapon %q cooldown %v fell below floor %v", w.Name, cd, w.CooldownMin)
			}
		}
	}
}

// TestHealingCardRestoresAFractionOfMaxHealth checks the card that fills a pool short of three upgrades.
func TestHealingCardRestoresAFractionOfMaxHealth(t *testing.T) {
	r := quietRun(t)
	r.keeper.health = 1
	r.phase = PhaseLevelUp
	h := r.data.upgrades.Healing
	r.offersNow = []offer{{id: h.ID, name: h.Name, kind: offerHeal, description: h.Description, index: -1, heal: true}}
	if !r.Choose(0) {
		t.Fatal("the run refused the healing card")
	}
	want := math.Min(r.stats.maxHealth, 1+r.stats.maxHealth*h.Fraction)
	if math.Abs(r.keeper.health-want) > 1e-9 {
		t.Fatalf("the healing card left %v health, want %v", r.keeper.health, want)
	}
}

// TestChooseOnlyTakesOfferedCards checks that a choice outside the offer changes nothing.
func TestChooseOnlyTakesOfferedCards(t *testing.T) {
	r := quietRun(t)
	r.xp = r.xpNeeded()
	stepOnce(r, Controls{})
	weapons := append([]int(nil), r.weaponLevels...)
	passives := append([]int(nil), r.passiveLevels...)
	for _, index := range []int{-1, offerCount, 99} {
		if r.Choose(index) {
			t.Fatalf("the run took card %d, which was not offered", index)
		}
	}
	if r.Snapshot().Phase != PhaseLevelUp {
		t.Fatalf("a refused choice resumed the run")
	}
	for i := range weapons {
		if r.weaponLevels[i] != weapons[i] {
			t.Fatalf("a refused choice raised weapon %d", i)
		}
	}
	for i := range passives {
		if r.passiveLevels[i] != passives[i] {
			t.Fatalf("a refused choice raised passive %d", i)
		}
	}
}
