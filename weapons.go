package main

import "math"

// stepChain casts Spark Chain once its cooldown has run out: the spark strikes the nearest foe in range and leaps on to the nearest foe it has not struck within its jump range, striking count foes in all. Ember Storm also forks from each foe struck to its nearest unstruck neighbours, and the forks leap no further.
func (r *Run) stepChain(index int, w *weaponDef, lvl *weaponLevelDef, evo *evolutionDef) {
	if r.weaponCooldowns[index] > 0 {
		return
	}
	next, ok := r.nearestFoe(r.keeper.pos, w.Range, nil)
	if !ok {
		return
	}
	damage := lvl.Damage * r.stats.damageMul
	fork := 0
	if evo != nil {
		damage *= evo.Damage
		fork = evo.Fork
	}
	jump := w.JumpRange * r.stats.areaMul
	var struck [maxChainHits]uint64
	n := 0
	strike := func(j int, from Vec2) Vec2 {
		e := &r.enemies[j]
		r.hitEnemy(j, damage, lvl.Knockback, e.pos.Sub(from), Vec2{X: 1})
		struck[n] = e.id
		n++
		r.addArc(from, e.pos, w.Duration, evo != nil)
		return e.pos
	}
	r.emit(Event{Kind: EventShot, ID: r.takeID(), Pos: r.keeper.pos})
	from := r.keeper.pos
	for k := range lvl.Count {
		at := strike(next, from)
		for range fork {
			f, ok := r.nearestFoe(at, jump, struck[:n])
			if !ok {
				break
			}
			strike(f, at)
		}
		if k == lvl.Count-1 {
			break
		}
		if next, ok = r.nearestFoe(at, jump, struck[:n]); !ok {
			break
		}
		from = at
	}
	r.weaponCooldowns[index] = r.weaponCooldown(w, lvl.Cooldown)
}

// addArc keeps a spark's leap for drawing, dropping it when the arcs are full.
func (r *Run) addArc(from, to Vec2, life float64, storm bool) {
	if len(r.arcs) < maxArcs {
		r.arcs = append(r.arcs, arc{id: r.takeID(), from: from, to: to, life: life, max: life, storm: storm})
	}
}

// tickArcs ages the spark's leaps and removes spent ones.
func (r *Run) tickArcs() {
	for i := 0; i < len(r.arcs); {
		a := &r.arcs[i]
		a.life -= stepSeconds
		if a.life <= 0 {
			last := len(r.arcs) - 1
			r.arcs[i] = r.arcs[last]
			r.arcs = r.arcs[:last]
			continue
		}
		i++
	}
}

// nearestFoe returns the index of the closest living enemy within radius of center whose id skip does not hold, breaking ties by its position in the slice.
func (r *Run) nearestFoe(center Vec2, radius float64, skip []uint64) (int, bool) {
	best := -1
	var bestDist float64
	r.grid.nearby(center, radius, func(i int) bool {
		e := &r.enemies[i]
		if e.hp <= 0 {
			return true
		}
		d2 := e.pos.Sub(center).LenSq()
		if d2 > radius*radius || (best >= 0 && (d2 > bestDist || (d2 == bestDist && i > best))) {
			return true
		}
		for _, id := range skip {
			if id == e.id {
				return true
			}
		}
		best, bestDist = i, d2
		return true
	})
	return best, best >= 0
}

// throwCenser lobs an Ash Censer at the densest crowd in range once its cooldown has run out, carrying the burst and the fire it will leave.
func (r *Run) throwCenser(index int, w *weaponDef, lvl *weaponLevelDef, evo *evolutionDef) {
	if r.weaponCooldowns[index] > 0 || len(r.censers) >= maxCensers {
		return
	}
	radius := lvl.Radius * r.stats.areaMul
	target, ok := r.densestFoe(r.keeper.pos, w.Range, radius)
	if !ok {
		return
	}
	damage := lvl.Damage * r.stats.damageMul
	c := censer{
		id:         r.takeID(),
		from:       r.keeper.pos,
		to:         target,
		flight:     w.Flight,
		left:       w.Flight,
		radius:     radius,
		knockback:  lvl.Knockback,
		fireRadius: radius * w.FireRadius,
		fireLife:   lvl.Duration,
		fireTick:   lvl.Tick * r.stats.cooldownMul,
	}
	if evo != nil {
		damage *= evo.Damage
		c.pull = evo.Pull
		c.pullRadius = evo.PullRadius * r.stats.areaMul
		c.pullSpeed = evo.PullSpeed
	}
	c.damage = damage
	c.fireDamage = damage * w.FireDamage
	r.censers = append(r.censers, c)
	r.emit(Event{Kind: EventShot, ID: c.id, Pos: r.keeper.pos})
	r.weaponCooldowns[index] = r.weaponCooldown(w, lvl.Cooldown)
}

// densestFoe returns where the living enemy within reach of center stands whose burst of radius would catch the most foes, breaking ties by its position in the slice.
func (r *Run) densestFoe(center Vec2, reach, radius float64) (Vec2, bool) {
	best, bestCount := -1, 0
	r.grid.nearby(center, reach, func(i int) bool {
		e := &r.enemies[i]
		if e.hp <= 0 || e.pos.Sub(center).LenSq() > reach*reach {
			return true
		}
		count := 0
		r.grid.nearby(e.pos, radius+r.maxRadius, func(j int) bool {
			o := &r.enemies[j]
			within := radius + o.radius
			if o.hp > 0 && o.pos.Sub(e.pos).LenSq() <= within*within {
				count++
			}
			return true
		})
		if count > bestCount || (count == bestCount && i < best) {
			best, bestCount = i, count
		}
		return true
	})
	if best < 0 {
		return Vec2{}, false
	}
	return r.enemies[best].pos, true
}

// stepCensers flies each censer on; one that lands bursts at once, or, as a Pyre Censer, pulls foes and embers in for its pull first.
func (r *Run) stepCensers() {
	pulled := false
	for i := 0; i < len(r.censers); {
		c := &r.censers[i]
		if !c.landed {
			c.left -= stepSeconds
			if c.left > 0 {
				i++
				continue
			}
			c.landed = true
			c.pulling = c.pull
		}
		if c.pulling > 0 {
			c.pulling -= stepSeconds
			r.pullTo(c.to, c.pullRadius, c.pullSpeed*stepSeconds)
			pulled = true
			if c.pulling > 0 {
				i++
				continue
			}
		}
		if pulled {
			// The pull moved foes across cells, so the burst reads a fresh grid.
			r.grid.build(r.enemies)
			pulled = false
		}
		r.burstCenser(c)
		last := len(r.censers) - 1
		r.censers[i] = r.censers[last]
		r.censers = r.censers[:last]
	}
	if pulled {
		r.grid.build(r.enemies)
	}
}

// pullTo draws every foe but the Hollow King, as far as its knockback resistance lets it, and every ember within radius of center up to step closer to it.
func (r *Run) pullTo(center Vec2, radius, step float64) {
	r.grid.nearby(center, radius+r.maxRadius, func(j int) bool {
		e := &r.enemies[j]
		to := center.Sub(e.pos)
		d := to.Len()
		if e.isBoss || d > radius+e.radius || d < 1e-9 {
			return true
		}
		e.pos = r.clamp(e.pos.Add(to.Scale(math.Min(d, step*(1-e.knockbackResist))/d)), e.radius)
		return true
	})
	for i := range r.embers {
		em := &r.embers[i]
		to := center.Sub(em.pos)
		if d := to.Len(); d <= radius && d > 1e-9 {
			em.pos = r.clampField(em.pos.Add(to.Scale(math.Min(d, step)/d)), r.data.run.Embers.Radius)
		}
	}
}

// burstCenser hits and pushes back every foe within the censer's burst and leaves its fire where it landed.
func (r *Run) burstCenser(c *censer) {
	r.grid.nearby(c.to, c.radius+r.maxRadius, func(j int) bool {
		e := &r.enemies[j]
		reach := c.radius + e.radius
		d := e.pos.Sub(c.to)
		if d.LenSq() > reach*reach {
			return true
		}
		r.hitEnemy(j, c.damage, c.knockback, d, Vec2{X: 1})
		return true
	})
	if len(r.patches) < maxPatches {
		r.patches = append(r.patches, patch{
			id:     r.takeID(),
			pos:    c.to,
			radius: c.fireRadius,
			damage: c.fireDamage,
			tick:   c.fireTick,
			life:   c.fireLife,
			max:    c.fireLife,
			censer: true,
		})
	}
	if len(r.pulses) < maxPulses {
		r.pulses = append(r.pulses, pulse{id: c.id, pos: c.to, radius: c.radius, life: censerBurstLife, max: censerBurstLife, censer: true, pyre: c.pull > 0})
	}
	r.emit(Event{Kind: EventShot, ID: c.id, Pos: c.to, Value: c.radius})
}
