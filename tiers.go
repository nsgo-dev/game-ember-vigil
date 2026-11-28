package main

import (
	"errors"
	"fmt"
	"math"
	"slices"
)

// tierVersion is the schema version data/tiers.yaml must declare.
const (
	tierVersion = 1
	fileTiers   = "data/tiers.yaml"
)

// maxShadows caps the shadow zones dead elites leave at once.
const maxShadows = 16

// tierFile is data/tiers.yaml: the rule each Long Night tier above 1 adds on top of the tiers below it. Tier 1 is the base run.
type tierFile struct {
	Version int       `yaml:"version"`
	Tiers   []tierDef `yaml:"tiers"`
}

// tierDef is one tier's rule. A tier sets only the fields of its own rule; a run at tier N applies every tier from 2 to N.
type tierDef struct {
	Tier int `yaml:"tier"`
	// Elites are spawned on top of data/waves.yaml's, and Shadow is what each dead elite leaves behind.
	Elites []eliteSpawnDef `yaml:"elites,omitempty"`
	Shadow shadowDef       `yaml:"shadow,omitempty"`
	// Light multiplies how far the lantern draws embers and caches in. Sight multiplies how far the keeper sees through the fog, and the spawn ring and view the director keeps outside, so foes still appear unseen.
	Light float64 `yaml:"light,omitempty"`
	Sight float64 `yaml:"sight,omitempty"`
	// HealCards is the most healing cards one level-up offers; Regen multiplies regeneration.
	HealCards int     `yaml:"healCards,omitempty"`
	Regen     float64 `yaml:"regen,omitempty"`
	// Shop multiplies every bonus the ember shop's items give.
	Shop float64 `yaml:"shop,omitempty"`
	// EnemySpeed multiplies every foe's speed, BossEarlier brings the Hollow King that many seconds sooner, and Volley is the attack he adds.
	EnemySpeed  float64   `yaml:"enemySpeed,omitempty"`
	BossEarlier float64   `yaml:"bossEarlier,omitempty"`
	Volley      volleyDef `yaml:"volley,omitempty"`
	// Health and Damage multiply every foe's health and damage, the Hollow King's too, where a tier's rule alone leaves the run too easy.
	Health float64 `yaml:"health,omitempty"`
	Damage float64 `yaml:"damage,omitempty"`
}

// shadowDef is a dark zone a dead elite leaves: it takes Damage off the keeper every Tick seconds the keeper stands in it, for Life seconds.
type shadowDef struct {
	Radius float64 `yaml:"radius"`
	Damage float64 `yaml:"damage"`
	Tick   float64 `yaml:"tick"`
	Life   float64 `yaml:"life"`
}

// volleyDef is the Hollow King's added attack: after Windup seconds he throws Count bones of his phase, Speed times as fast, in a fan Spread radians wide at the keeper.
type volleyDef struct {
	Windup float64 `yaml:"windup"`
	Count  int     `yaml:"count"`
	Spread float64 `yaml:"spread"`
	Speed  float64 `yaml:"speed"`
}

// tierRules is what a run at one tier plays: every rule of the tiers up to it, folded together. The zero value with the multipliers at 1 is the base run.
type tierRules struct {
	elites      []eliteSpawnDef
	shadow      shadowDef
	light       float64
	sight       float64
	healCards   int
	regen       float64
	shop        float64
	enemySpeed  float64
	bossEarlier float64
	volley      volleyDef
	health      float64
	damage      float64
}

// maxTier is the highest Long Night tier the data defines.
func (f *tierFile) maxTier() int { return 1 + len(f.Tiers) }

// clampTier keeps tier within the tiers the data defines.
func (f *tierFile) clampTier(tier int) int { return max(1, min(tier, f.maxTier())) }

// rules folds the rules of tiers 2 to tier into one, and merges the added elites into waves' in time order.
func (f *tierFile) rules(tier int, waves *waveFile) tierRules {
	r := tierRules{light: 1, sight: 1, regen: 1, shop: 1, enemySpeed: 1, health: 1, damage: 1, elites: waves.Elites}
	added := false
	for i := range f.Tiers[:f.clampTier(tier)-1] {
		t := &f.Tiers[i]
		if len(t.Elites) > 0 {
			if !added {
				r.elites = slices.Clone(r.elites)
				added = true
			}
			r.elites = append(r.elites, t.Elites...)
		}
		if t.Shadow != (shadowDef{}) {
			r.shadow = t.Shadow
		}
		if t.Light > 0 {
			r.light *= t.Light
		}
		if t.Sight > 0 {
			r.sight *= t.Sight
		}
		if t.HealCards > 0 && (r.healCards == 0 || t.HealCards < r.healCards) {
			r.healCards = t.HealCards
		}
		if t.Regen > 0 {
			r.regen *= t.Regen
		}
		if t.Shop > 0 {
			r.shop *= t.Shop
		}
		if t.EnemySpeed > 0 {
			r.enemySpeed *= t.EnemySpeed
		}
		r.bossEarlier += t.BossEarlier
		if t.Volley != (volleyDef{}) {
			r.volley = t.Volley
		}
		if t.Health > 0 {
			r.health *= t.Health
		}
		if t.Damage > 0 {
			r.damage *= t.Damage
		}
	}
	// A stable sort keeps the waves' elites ahead of a tier's at the same second.
	slices.SortStableFunc(r.elites, func(a, b eliteSpawnDef) int {
		switch {
		case a.At < b.At:
			return -1
		case a.At > b.At:
			return 1
		}
		return 0
	})
	return r
}

// parseTierFile decodes and validates data/tiers.yaml.
func parseTierFile(file string, data []byte) (*tierFile, error) {
	var f tierFile
	if err := decodeStrict(file, data, &f); err != nil {
		return nil, err
	}
	if err := f.validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	return &f, nil
}

func (f *tierFile) validate() error {
	if f.Version != tierVersion {
		return fmt.Errorf("version %d, want %d", f.Version, tierVersion)
	}
	for i := range f.Tiers {
		t := &f.Tiers[i]
		if t.Tier != i+2 {
			return fmt.Errorf("tier %d is listed where tier %d belongs", t.Tier, i+2)
		}
		if err := t.validate(); err != nil {
			return fmt.Errorf("tier %d: %w", t.Tier, err)
		}
	}
	return nil
}

func (t *tierDef) validate() error {
	ratio := func(v float64) bool { return v == 0 || positive(v) }
	switch {
	case len(t.Elites) == 0 && t.Shadow == (shadowDef{}) && t.Light == 0 && t.Sight == 0 && t.HealCards == 0 && t.Regen == 0 && t.Shop == 0 &&
		t.EnemySpeed == 0 && t.BossEarlier == 0 && t.Volley == (volleyDef{}):
		return errors.New("adds no rule")
	case !ratio(t.Light) || t.Light > 1:
		return fmt.Errorf("light %v is not in (0, 1]", t.Light)
	case !ratio(t.Sight) || t.Sight > 1:
		return fmt.Errorf("sight %v is not in (0, 1]", t.Sight)
	case t.HealCards < 0 || t.HealCards >= offerCount:
		return fmt.Errorf("healCards %d is not from 0 to %d", t.HealCards, offerCount-1)
	case !ratio(t.Regen):
		return fmt.Errorf("regen %v is negative", t.Regen)
	case !ratio(t.Shop) || t.Shop > 1:
		return fmt.Errorf("shop %v is not in (0, 1]", t.Shop)
	case !ratio(t.EnemySpeed):
		return fmt.Errorf("enemySpeed %v is negative", t.EnemySpeed)
	case !finite(t.BossEarlier) || t.BossEarlier < 0:
		return fmt.Errorf("bossEarlier %v is negative", t.BossEarlier)
	case !ratio(t.Health) || !ratio(t.Damage):
		return fmt.Errorf("health %v or damage %v is negative", t.Health, t.Damage)
	}
	for i := range t.Elites {
		el := &t.Elites[i]
		if err := el.validate(); err != nil {
			return fmt.Errorf("elite %d: %w", i, err)
		}
		if i > 0 && el.At < t.Elites[i-1].At {
			return fmt.Errorf("elite %d at %v is before elite %d at %v", i, el.At, i-1, t.Elites[i-1].At)
		}
	}
	if s := t.Shadow; s != (shadowDef{}) && (!positive(s.Radius) || !positive(s.Damage) || !positive(s.Tick) || !positive(s.Life)) {
		return fmt.Errorf("shadow %+v needs a radius, damage, tick and life above 0", s)
	}
	if v := t.Volley; v != (volleyDef{}) && (!positive(v.Windup) || v.Count < 1 || !finite(v.Spread) || v.Spread < 0 || v.Spread >= 2*math.Pi || !positive(v.Speed)) {
		return fmt.Errorf("volley %+v needs a windup and speed above 0, a count of at least 1 and a spread under a full turn", v)
	}
	return nil
}

// check resolves the enemies a tier's elites name and keeps the Hollow King's earlier arrival after the run starts.
func (f *tierFile) check(d *gameData) error {
	earlier := 0.0
	for i := range f.Tiers {
		t := &f.Tiers[i]
		for j := range t.Elites {
			if _, ok := d.enemyIndex(t.Elites[j].Enemy); !ok {
				return fmt.Errorf("%s: tier %d: elite %d: no enemy %q", fileTiers, t.Tier, j, t.Elites[j].Enemy)
			}
		}
		earlier += t.BossEarlier
		if earlier >= d.waves.Boss.At {
			return fmt.Errorf("%s: tier %d brings the Hollow King %v seconds early, at or before the run starts", fileTiers, t.Tier, earlier)
		}
	}
	return nil
}

// shadow is a dark zone a dead elite left behind.
type shadow struct {
	id   uint64
	pos  Vec2
	life float64
}

// ShadowView is one shadow zone as a snapshot shows it.
type ShadowView struct {
	ID     uint64  `json:"id"`
	Pos    Vec2    `json:"pos"`
	Radius float64 `json:"radius"`
	Life   float64 `json:"life"`
	Max    float64 `json:"max"`
}

// leaveShadow puts a shadow zone where an elite died, when the run's tier makes elites leave one.
func (r *Run) leaveShadow(pos Vec2) {
	if r.rules.shadow.Life <= 0 || len(r.shadows) >= maxShadows {
		return
	}
	r.shadows = append(r.shadows, shadow{id: r.takeID(), pos: pos, life: r.rules.shadow.Life})
}

// stepShadows ages the shadow zones and, once every tick the keeper stands in one, takes its damage off the keeper.
func (r *Run) stepShadows() {
	s := r.rules.shadow
	inside := false
	for i := 0; i < len(r.shadows); {
		sh := &r.shadows[i]
		sh.life -= stepSeconds
		if sh.life <= 0 {
			last := len(r.shadows) - 1
			r.shadows[i] = r.shadows[last]
			r.shadows = r.shadows[:last]
			continue
		}
		reach := s.Radius + r.data.run.Keeper.Radius
		if !inside && sh.pos.Sub(r.keeper.pos).LenSq() <= reach*reach {
			inside = true
			// A hair of slack keeps float drift from adding a step to a tick.
			if r.shadowTick <= 1e-9 {
				r.keeper.health -= s.Damage
				r.shadowTick = s.Tick
				r.emit(Event{Kind: EventHit, ID: sh.id, Pos: sh.pos, Value: s.Damage})
			}
		}
		i++
	}
	if !inside {
		r.shadowTick = 0
	} else {
		r.shadowTick -= stepSeconds
	}
}
