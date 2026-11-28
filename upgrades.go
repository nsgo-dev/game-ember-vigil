package main

import (
	"cmp"
	"math"
	"slices"

	"github.com/nsgo-dev/game/packages/game/random"
)

// offerCount is how many cards a level-up offers.
const offerCount = 3

// Offer kinds as cards and snapshots name them.
const (
	offerWeapon  = "weapon"
	offerPassive = "passive"
	offerHeal    = "heal"
)

// stats is the one stat block a run reads each step: what the tuning and the passives taken make of the keeper and every weapon.
type stats struct {
	damageMul    float64
	cooldownMul  float64
	areaMul      float64
	moveSpeed    float64
	maxHealth    float64
	regen        float64
	pickupRadius float64
	xpMul        float64
}

// offer is one card a level-up offers.
type offer struct {
	id          string
	name        string
	kind        string
	stat        string
	description string
	level       int
	maxLevel    int
	// index is the card's position in Weapons or Passives, and -1 for the healing card.
	index int
	heal  bool
}

// recomputeStats derives the stat block from the tuning and the passive levels taken, so no weapon or view caches a modified copy.
func (r *Run) recomputeStats() {
	next := stats{
		damageMul:    1,
		cooldownMul:  1,
		areaMul:      1,
		moveSpeed:    r.data.run.Keeper.Speed,
		maxHealth:    r.data.run.Keeper.Health,
		regen:        0,
		pickupRadius: r.data.run.Embers.MagnetRadius,
		xpMul:        1,
	}
	for i, level := range r.passiveLevels {
		if level == 0 {
			continue
		}
		p := &r.data.upgrades.Passives[i]
		amount := p.Amount * float64(level)
		switch p.Stat {
		case statDamage:
			next.damageMul += amount
		case statCooldown:
			next.cooldownMul -= amount
		case statArea:
			next.areaMul += amount
		case statMoveSpeed:
			next.moveSpeed += amount
		case statMaxHealth:
			next.maxHealth += amount
		case statRegen:
			next.regen += amount
		case statPickup:
			next.pickupRadius += amount
		case statXPGain:
			next.xpMul += amount
		}
	}
	// The shop's bonuses add to the passives', weakened by the run's Long Night tier.
	shop := &r.data.progression.Shop
	for i, level := range r.shopLevels {
		it := &shop.Items[i]
		amount := it.Amount * float64(level) * r.rules.shop
		switch it.Stat {
		case statDamage:
			next.damageMul += amount
		case statCooldown:
			next.cooldownMul -= amount
		case statMoveSpeed:
			next.moveSpeed += amount
		case statMaxHealth:
			next.maxHealth += amount
		case statPickup:
			next.pickupRadius += amount
		}
	}
	next.regen *= r.rules.regen
	next.pickupRadius *= r.rules.light
	next.cooldownMul = math.Max(next.cooldownMul, 0.2)
	r.stats = next
}

// ownedWeapons returns how many weapon slots the keeper currently fills.
func (r *Run) ownedWeapons() int {
	n := 0
	for _, level := range r.weaponLevels {
		if level > 0 {
			n++
		}
	}
	return n
}

// ownedPassives returns how many passive slots the keeper currently fills.
func (r *Run) ownedPassives() int {
	n := 0
	for _, level := range r.passiveLevels {
		if level > 0 {
			n++
		}
	}
	return n
}

// drawOffers takes three distinct cards from the pool on the offer stream, filling what is missing with healing cards. A redraw passes the cards it replaces, which come only once the rest of the pool is used up.
func (r *Run) drawOffers(replaced ...offer) []offer {
	pool := r.offerPool()
	random.Shuffle(r.offers, pool)
	if len(replaced) > 0 {
		slices.SortStableFunc(pool, func(a, b offer) int {
			return cmp.Compare(offered(replaced, a.id), offered(replaced, b.id))
		})
	}
	out := make([]offer, 0, offerCount)
	out = append(out, pool[:min(offerCount, len(pool))]...)
	h := r.data.upgrades.Healing
	// A tier that offers fewer healing cards leaves an offer short, but never empty.
	want := offerCount
	if limit := r.rules.healCards; limit > 0 {
		want = max(1, min(offerCount, len(out)+limit))
	}
	for len(out) < want {
		out = append(out, offer{
			id:          h.ID,
			name:        h.Name,
			kind:        offerHeal,
			description: h.Description,
			index:       -1,
			heal:        true,
		})
	}
	return out
}

// offered is 1 when cards holds the card id and 0 otherwise, which sorts the cards a redraw replaces last.
func offered(cards []offer, id string) int {
	for i := range cards {
		if cards[i].id == id {
			return 1
		}
	}
	return 0
}

// offerPool is every eligible weapon and passive still below its maximum level and fitting within the slot caps, in data order; a locked weapon is never eligible.
func (r *Run) offerPool() []offer {
	canAddWeapon := r.ownedWeapons() < r.data.upgrades.WeaponSlots
	canAddPassive := r.ownedPassives() < r.data.upgrades.PassiveSlots
	pool := make([]offer, 0, len(r.data.weapons.Weapons)+len(r.data.upgrades.Passives))
	for i := range r.data.weapons.Weapons {
		w := &r.data.weapons.Weapons[i]
		lvl := r.weaponLevels[i]
		if lvl >= len(w.Levels) || (lvl == 0 && (!canAddWeapon || r.locked[i])) {
			continue
		}
		pool = append(pool, offer{
			id:          w.ID,
			name:        w.Name,
			kind:        offerWeapon,
			description: w.Description,
			level:       lvl + 1,
			maxLevel:    len(w.Levels),
			index:       i,
		})
	}
	for i := range r.data.upgrades.Passives {
		p := &r.data.upgrades.Passives[i]
		lvl := r.passiveLevels[i]
		if lvl >= p.MaxLevel || (lvl == 0 && !canAddPassive) {
			continue
		}
		pool = append(pool, offer{
			id:          p.ID,
			name:        p.Name,
			kind:        offerPassive,
			stat:        p.Stat,
			description: p.Description,
			level:       lvl + 1,
			maxLevel:    p.MaxLevel,
			index:       i,
		})
	}
	return pool
}
