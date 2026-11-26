package main

import "math"

// viewWidth and viewHeight are the world area the arena shows at once, in world units: the 480x270 canvas at zoom one. A spawn ring outside them keeps an enemy from appearing in sight.
const (
	viewWidth  = 480
	viewHeight = 270
)

// spawnTries is how many ring points the director draws in one step before it waits for the next, so a keeper in a corner does not stall a spawn on an impossible point.
const spawnTries = 8

// director is the wave director: it reads data/waves.yaml at the run clock, picks enemies from the span's mix and spawns them on a ring around the keeper.
type director struct {
	waves *waveFile
	// nextSpawn is when the next spawn is due, in seconds on the run clock.
	nextSpawn float64
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

// step spawns what the timeline owes by now. When the span's cap is full the due spawn waits instead of being skipped, and once room frees the director catches up without passing the cap.
func (d *director) step(r *Run) {
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
	r.enemies = append(r.enemies, enemy{
		id:              r.takeID(),
		kind:            kind,
		pos:             pos,
		hp:              health,
		maxHP:           health,
		damage:          def.Damage * damageMul,
		radius:          def.Radius,
		speed:           def.Speed,
		motion:          def.Motion,
		driftAmplitude:  def.Drift.Amplitude,
		driftPeriod:     def.Drift.Period,
		driftPhase:      r.spawns.Range(0, 2*math.Pi),
		knockbackResist: def.KnockbackResist,
		embers:          def.Embers,
	})
}
