package main

import "math"

// viewWidth and viewHeight are the world area the arena shows at once, in world units: the 480x270 canvas at zoom one. A spawn ring outside them keeps an enemy from appearing in sight.
const (
	viewWidth  = 480
	viewHeight = 270
)

// spawnTries is how many ring points the director draws in one step before it waits for the next, so a keeper in a corner does not stall a spawn on an impossible point.
const spawnTries = 8

// director is the wave director: it reads data/waves.yaml at the run clock, picks enemies from the span's mix, triggers surges and elites, and hands off to the Hollow King at 12:00.
type director struct {
	waves       *waveFile
	nextSpawn   float64
	nextSurge   int
	nextElite   int
	bossSpawned bool
}

// newDirector starts the director at the timeline's beginning.
func newDirector(w *waveFile) director { return director{waves: w} }

// spanAt returns the span the clock is in, or nil before the timeline starts.
func (d *director) spanAt(seconds float64) *spanDef {
	for i := len(d.waves.Spans) - 1; i >= 0; i-- {
		if seconds >= d.waves.Spans[i].From {
			return &d.waves.Spans[i]
		}
	}
	return nil
}

// step spawns what the timeline owes by now. Normal spawns pause once the Hollow King arrives; before then, scheduled elites, surges and regular span spawns run in order.
func (d *director) step(r *Run) {
	if d.bossSpawned || r.boss.active {
		return
	}
	if d.waves.Boss.At > 0 && r.time >= d.waves.Boss.At {
		d.bossSpawned = true
		r.spawnBoss()
		return
	}

	// Scheduled elites always appear on time (up to maxEnemies), independent of the span's soft alive cap.
	for d.nextElite < len(d.waves.Elites) && r.time >= d.waves.Elites[d.nextElite].At {
		el := &d.waves.Elites[d.nextElite]
		d.nextElite++
		if kind, ok := r.data.enemyIndex(el.Enemy); ok {
			def := &r.data.enemies.Enemies[kind]
			pos, ok := r.spawnPos(def.Radius)
			if !ok {
				pos = r.fallbackRingPos(def.Radius)
			}
			r.addEnemy(kind, pos, el.Health, el.Damage)
		}
	}

	// Scheduled surges spawn a burst or full 360-degree encircling wave around the ring, respecting the global alive cap.
	for d.nextSurge < len(d.waves.Surges) && r.time >= d.waves.Surges[d.nextSurge].At {
		s := &d.waves.Surges[d.nextSurge]
		d.nextSurge++
		if kind, ok := r.data.enemyIndex(s.Enemy); ok {
			def := &r.data.enemies.Enemies[kind]
			if s.Surround {
				r.spawnSurround(kind, def.Radius, s.Count, s.Health, s.Damage)
				continue
			}
			for range s.Count {
				if len(r.enemies) >= d.waves.MaxAlive {
					break
				}
				if pos, ok := r.spawnPos(def.Radius); ok {
					r.addEnemy(kind, pos, s.Health, s.Damage)
				}
			}
		}
	}

	span := d.spanAt(r.time)
	if span == nil {
		return
	}
	cap := min(span.MaxAlive, d.waves.MaxAlive)
	for r.time >= d.nextSpawn {
		if len(r.enemies) >= cap {
			return
		}
		mix := r.spawns.WeightedIndex(span.weights)
		if mix < 0 {
			return
		}
		kind, ok := r.data.enemyIndex(span.Enemies[mix].Name)
		if !ok {
			return
		}
		def := &r.data.enemies.Enemies[kind]
		if pos, ok := r.spawnPos(def.Radius); ok {
			r.addEnemy(kind, pos, span.Health, span.Damage)
		}
		d.nextSpawn += span.Every
	}
}

// spawnPos draws a point on the ring around the keeper, clamped inside the arena and off the view, or reports that no attempt this step landed there.
func (r *Run) spawnPos(radius float64) (Vec2, bool) {
	ring := r.data.waves.Ring
	for range spawnTries {
		dir := Vec2{r.spawns.Range(-1, 1), r.spawns.Range(-1, 1)}
		if l2 := dir.LenSq(); l2 < 1e-6 || l2 > 1 {
			continue
		}
		p := r.clamp(r.keeper.pos.Add(dir.Normalized().Scale(r.spawns.Range(ring.Min, ring.Max))), radius)
		if offscreen(r.keeper.pos, p) {
			return p, true
		}
	}
	return Vec2{}, false
}

// spawnSurround places count enemies evenly around a full 360-degree ring outside the view, enclosing the keeper from every direction.
func (r *Run) spawnSurround(kind int, radius float64, count int, healthMul, damageMul float64) {
	if count <= 0 {
		return
	}
	ring := r.data.waves.Ring
	baseAngle := r.spawns.Range(0, 2*math.Pi)
	step := 2 * math.Pi / float64(count)
	for i := range count {
		if len(r.enemies) >= r.data.waves.MaxAlive {
			break
		}
		ang := baseAngle + float64(i)*step
		dist := r.spawns.Range(ring.Min, ring.Min+(ring.Max-ring.Min)*0.45)
		dir := Vec2{X: math.Cos(ang), Y: math.Sin(ang)}
		pos := r.clamp(r.keeper.pos.Add(dir.Scale(dist)), radius)
		r.addEnemy(kind, pos, healthMul, damageMul)
	}
}

// fallbackRingPos places a guaranteed spawn (elite or boss) toward the arena center when random ring attempts fall inside the view.
func (r *Run) fallbackRingPos(radius float64) Vec2 {
	center := Vec2{X: r.data.run.Arena.Width / 2, Y: r.data.run.Arena.Height / 2}
	dir := center.Sub(r.keeper.pos).Normalized()
	if dir == (Vec2{}) {
		dir = Vec2{X: 1}
	}
	return r.clamp(r.keeper.pos.Add(dir.Scale(r.data.waves.Ring.Min)), radius)
}

// offscreen reports whether p lies outside the view the arena shows around center.
func offscreen(center, p Vec2) bool {
	d := p.Sub(center)
	return math.Abs(d.X) > viewWidth/2 || math.Abs(d.Y) > viewHeight/2
}

// addEnemy puts one enemy of kind on the field from the span's multipliers.
func (r *Run) addEnemy(kind int, pos Vec2, healthMul, damageMul float64) {
	if len(r.enemies) >= maxEnemies {
		return
	}
	def := &r.data.enemies.Enemies[kind]
	health := def.Health * healthMul
	orbitDir := 1.0
	if def.Motion == motionOrbit && r.spawns.Range(-1, 1) < 0 {
		orbitDir = -1.0
	}
	splitKind := -1
	if def.Split.Count > 0 && def.Split.Enemy != "" {
		if idx, ok := r.data.enemyIndex(def.Split.Enemy); ok {
			splitKind = idx
		}
	}
	r.enemies = append(r.enemies, enemy{
		id:              r.takeID(),
		kind:            kind,
		elite:           def.Elite,
		pos:             pos,
		hp:              health,
		maxHP:           health,
		healthMul:       healthMul,
		damageMul:       damageMul,
		damage:          def.Damage * damageMul,
		radius:          def.Radius,
		speed:           def.Speed,
		motion:          def.Motion,
		driftAmplitude:  def.Drift.Amplitude,
		driftPeriod:     def.Drift.Period,
		driftPhase:      r.spawns.Range(0, 2*math.Pi),
		keepDistance:    def.Ranged.KeepDistance,
		orbitRadius:     def.Orbit.Radius,
		orbitDir:        orbitDir,
		shotRange:       def.Shot.Range,
		shotPeriod:      def.Shot.Cooldown,
		shotCooldown:    def.Shot.Cooldown * 0.5,
		shotSpeed:       def.Shot.Speed,
		shotRadius:      def.Shot.Radius,
		shotLife:        def.Shot.Life,
		shotDamage:      def.Shot.Damage * damageMul,
		shotBurst:       def.Shot.Burst,
		splitKind:       splitKind,
		splitCount:      def.Split.Count,
		healDrop:        def.HealDrop,
		knockbackResist: def.KnockbackResist,
		embers:          def.Embers,
	})
}
