package main

import (
	"math"

	"github.com/nsgo-dev/game/packages/game/random"
)

// offerCount is how many cards a level-up offers.
const offerCount = 3

// stats is the one stat block a run reads each step: what the tuning and the cards taken make of the keeper and the starter weapon.
type stats struct {
	damage    float64
	cooldown  float64
	moveSpeed float64
	maxHealth float64
}

// offer is one card a level-up offers.
type offer struct {
	name     string
	stat     string
	level    int
	maxLevel int
	// upgrade is the card's position in data/run.yaml's pool, and -1 for the healing card.
	upgrade int
	heal    bool
}

// recomputeStats derives the stat block from the tuning and the levels taken, so no weapon or view caches a modified copy.
func (r *Run) recomputeStats() {
	w := r.data.run.Weapon
	next := stats{
		damage:    w.Damage,
		cooldown:  w.Cooldown,
		moveSpeed: r.data.run.Keeper.Speed,
		maxHealth: r.data.run.Keeper.Health,
	}
	for i, level := range r.upgradeLevels {
		if level == 0 {
			continue
		}
		u := &r.data.run.Upgrades[i]
		amount := u.Amount * float64(level)
		switch u.Stat {
		case statDamage:
			next.damage += amount
		case statCooldown:
			next.cooldown -= amount
		case statMoveSpeed:
			next.moveSpeed += amount
		case statMaxHealth:
			next.maxHealth += amount
		}
	}
	next.cooldown = math.Max(next.cooldown, w.CooldownMin)
	r.stats = next
}

// drawOffers takes three distinct cards from the pool on the offer stream, filling what is missing with healing cards.
func (r *Run) drawOffers() []offer {
	pool := r.offerPool()
	random.Shuffle(r.offers, pool)
	out := make([]offer, 0, offerCount)
	out = append(out, pool[:min(offerCount, len(pool))]...)
	for len(out) < offerCount {
		out = append(out, offer{name: r.data.run.Healing.Name, upgrade: -1, heal: true})
	}
	return out
}

// offerPool is every card still below its maximum level, in data order.
func (r *Run) offerPool() []offer {
	var pool []offer
	for i := range r.data.run.Upgrades {
		u := &r.data.run.Upgrades[i]
		if r.upgradeLevels[i] >= u.MaxLevel {
			continue
		}
		pool = append(pool, offer{
			name:     u.Name,
			stat:     u.Stat,
			level:    r.upgradeLevels[i] + 1,
			maxLevel: u.MaxLevel,
			upgrade:  i,
		})
	}
	return pool
}
