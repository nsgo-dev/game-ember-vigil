package main

import (
	"math"
	"time"

	"github.com/nsgo-dev/game/packages/game/random"
)

// A run's structural limits: one fixed step, the grid its queries use, and caps no wave or weapon may pass.
const (
	stepSeconds = 1.0 / 60.0
	gridCell    = 32.0
	maxEnemies  = 400
	maxShots    = 256
	maxEmbers   = 512
	maxEvents   = 256
)

// Vec2 is a position, a direction or a velocity in world units, whose X grows right and Y grows down.
type Vec2 struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Add returns v + o.
func (v Vec2) Add(o Vec2) Vec2 { return Vec2{v.X + o.X, v.Y + o.Y} }

// Sub returns v - o.
func (v Vec2) Sub(o Vec2) Vec2 { return Vec2{v.X - o.X, v.Y - o.Y} }

// Scale returns v stretched by s.
func (v Vec2) Scale(s float64) Vec2 { return Vec2{v.X * s, v.Y * s} }

// LenSq returns the square of v's length, which compares distances without a square root.
func (v Vec2) LenSq() float64 { return v.X*v.X + v.Y*v.Y }

// Len returns v's length.
func (v Vec2) Len() float64 { return math.Sqrt(v.LenSq()) }

// Normalized returns v as a unit vector, or the zero vector when v is too short to point anywhere.
func (v Vec2) Normalized() Vec2 {
	l := v.Len()
	if l < 1e-9 {
		return Vec2{}
	}
	return Vec2{v.X / l, v.Y / l}
}

// Controls is one step's input: where the bound axes point the keeper, each in [-1, 1].
type Controls struct {
	Move Vec2
}

// Phase is what a run is doing: playing, waiting for a level-up card, or over.
type Phase uint8

const (
	// PhasePlaying advances with every step.
	PhasePlaying Phase = iota
	// PhaseLevelUp holds the run still until Choose takes a card.
	PhaseLevelUp
	// PhaseDefeat is a finished run the shell shows a result for.
	PhaseDefeat
)

// EventKind is what an event reports.
type EventKind uint8

const (
	// EventShot is a shot leaving the keeper.
	EventShot EventKind = iota
	// EventHit is a shot or a contact taking health off a target.
	EventHit
	// EventKill is an enemy dying.
	EventKill
	// EventEmber is an ember reaching the keeper and giving XP.
	EventEmber
	// EventLevelUp is the run stopping for a card.
	EventLevelUp
	// EventDefeat is the keeper running out of health.
	EventDefeat
)

// Event is one thing a step did, which the shell turns into a sound, a flash or a screen. It names entities by id and positions in world units, never by pointers into the run.
type Event struct {
	Kind  EventKind `json:"kind"`
	ID    uint64    `json:"id,omitempty"`
	Pos   Vec2      `json:"pos"`
	Value float64   `json:"value,omitempty"`
}

// keeper is the player's ember keeper.
type keeper struct {
	pos    Vec2
	health float64
	// invuln is how many seconds of contact still pass without damage.
	invuln float64
}

// enemy is one living creature chasing the keeper.
type enemy struct {
	id              uint64
	kind            int
	pos             Vec2
	hp              float64
	maxHP           float64
	damage          float64
	radius          float64
	speed           float64
	motion          string
	driftAmplitude  float64
	driftPeriod     float64
	driftPhase      float64
	knockbackResist float64
	knockback       Vec2
	embers          int
}

// shot is one Ember Bolt in flight.
type shot struct {
	id        uint64
	pos       Vec2
	vel       Vec2
	damage    float64
	radius    float64
	knockback float64
	life      float64
}

// ember is XP an enemy left on the ground, which flies to the keeper once it is close enough.
type ember struct {
	pos       Vec2
	value     int
	attracted bool
}

// Run is one run of the game, simulated headlessly: no views, devices, sound or storage, and no wall clock. The shell steps it one fixed step at a time and draws what its snapshot and events say.
type Run struct {
	data *gameData

	// streams keep spawns, offers and drops apart, so a change to one never shifts the others.
	spawns *random.Stream
	offers *random.Stream
	drops  *random.Stream

	phase Phase
	time  float64

	keeper         keeper
	stats          stats
	weaponCooldown float64

	enemies []enemy
	shots   []shot
	embers  []ember

	director director
	grid     grid

	// upgradeLevels holds each upgrade's level, in the order data/run.yaml lists the pool.
	upgradeLevels []int
	offersNow     []offer

	xp     float64
	level  int
	kills  int
	nextID uint64

	events []Event
	snap   Snapshot

	// maxRadius is the widest enemy's radius, which bounds a nearby query.
	maxRadius float64
}

// NewRun starts a run at 00:00 from the embedded tuning and seed.
func NewRun(seed uint64) *Run { return newRunWith(mustGameData(), seed) }

// newRunWith starts a run from d, which tests may hold themselves.
func newRunWith(d *gameData, seed uint64) *Run {
	streams := random.NewStreams(seed)
	r := &Run{
		data:          d,
		spawns:        streams.Stream("spawns"),
		offers:        streams.Stream("offers"),
		drops:         streams.Stream("drops"),
		phase:         PhasePlaying,
		level:         1,
		upgradeLevels: make([]int, len(d.run.Upgrades)),
		offersNow:     make([]offer, 0, offerCount),
		events:        make([]Event, 0, maxEvents),
		enemies:       make([]enemy, 0, maxEnemies),
		shots:         make([]shot, 0, maxShots),
		embers:        make([]ember, 0, maxEmbers),
	}
	r.keeper = keeper{pos: Vec2{d.run.Arena.Width / 2, d.run.Arena.Height / 2}, health: d.run.Keeper.Health}
	r.recomputeStats()
	r.keeper.health = r.stats.maxHealth
	r.director = newDirector(&d.waves)
	r.grid = newGrid(gridCell, d.run.Arena.Width, d.run.Arena.Height)
	for i := range d.enemies.Enemies {
		r.maxRadius = math.Max(r.maxRadius, d.enemies.Enemies[i].Radius)
	}
	return r
}

// Step advances the run by exactly one 60 Hz step. dt is the loop's step length: a run never scales with it, and a step that is not positive does nothing, so a display frame cannot change the simulation. While a level-up waits for a Choose the run holds still.
func (r *Run) Step(dt time.Duration, controls Controls) {
	r.events = r.events[:0]
	if dt <= 0 || r.phase != PhasePlaying {
		return
	}

	// Move the keeper, then tick its timers.
	move := controls.Move
	if !finite(move.X) || !finite(move.Y) {
		move = Vec2{}
	}
	if l2 := move.LenSq(); l2 > 1 {
		move = move.Scale(1 / math.Sqrt(l2))
	}
	r.keeper.pos = r.clamp(r.keeper.pos.Add(move.Scale(r.stats.moveSpeed*stepSeconds)), r.data.run.Keeper.Radius)
	r.keeper.invuln = math.Max(0, r.keeper.invuln-stepSeconds)
	r.weaponCooldown -= stepSeconds

	// Spawn from the timeline, then move the world.
	r.director.step(r)
	r.grid.build(r.enemies)
	for i := range r.enemies {
		r.moveEnemy(&r.enemies[i])
	}
	r.grid.build(r.enemies)
	r.separate()
	r.grid.build(r.enemies)
	r.moveShots()

	// Hit, fire, bury the dead and gather what they left.
	r.contact()
	r.fire()
	r.reap()
	r.gatherEmbers()

	if r.keeper.health <= 0 {
		r.defeat()
		return
	}
	r.fillLevel()
	r.time += stepSeconds
}

// Choose takes one of the three cards a level-up offered, applies it and resumes the run.
func (r *Run) Choose(index int) bool {
	if r.phase != PhaseLevelUp || index < 0 || index >= len(r.offersNow) {
		return false
	}
	o := r.offersNow[index]
	beforeMax := r.stats.maxHealth
	if o.heal {
		r.keeper.health = math.Min(r.stats.maxHealth, r.keeper.health+r.stats.maxHealth*r.data.run.Healing.Fraction)
	} else {
		r.upgradeLevels[o.upgrade]++
		r.recomputeStats()
		// Health a card adds is health the keeper gains at once, so a bigger heart is not a weaker card.
		r.keeper.health = math.Min(r.stats.maxHealth, r.keeper.health+r.stats.maxHealth-beforeMax)
	}
	r.offersNow = r.offersNow[:0]
	r.phase = PhasePlaying
	return true
}

// Events returns what the last step did, which the shell maps to sounds, flashes and screens. The list is valid until the next Step.
func (r *Run) Events() []Event { return r.events }

// emit appends e to the step's events, dropping it when the step already reported maxEvents.
func (r *Run) emit(e Event) {
	if len(r.events) < maxEvents {
		r.events = append(r.events, e)
	}
}

// moveEnemy walks e toward the keeper, swaying across its course when it drifts, and lets its knockback fade.
func (r *Run) moveEnemy(e *enemy) {
	to := r.keeper.pos.Sub(e.pos)
	dir := to.Normalized()
	vel := dir.Scale(e.speed)
	if e.motion == motionDrift && e.driftPeriod > 0 {
		sway := math.Sin(2*math.Pi*(r.time/e.driftPeriod)+e.driftPhase) * e.driftAmplitude * e.speed
		vel = vel.Add(Vec2{-dir.Y, dir.X}.Scale(sway))
	}
	vel = vel.Add(e.knockback)
	e.pos = r.clamp(e.pos.Add(vel.Scale(stepSeconds)), e.radius)
	drag := math.Exp(-r.data.run.Weapon.KnockbackDrag * stepSeconds)
	e.knockback = e.knockback.Scale(drag)
}

// separate pushes enemies that overlap each other apart, so a crowd spreads around the keeper instead of stacking on one point.
func (r *Run) separate() {
	for i := range r.enemies {
		a := &r.enemies[i]
		r.grid.nearby(a.pos, a.radius+r.maxRadius, func(j int) bool {
			if j <= i {
				return true
			}
			b := &r.enemies[j]
			reach := a.radius + b.radius
			d := b.pos.Sub(a.pos)
			d2 := d.LenSq()
			if d2 >= reach*reach {
				return true
			}
			var push Vec2
			if dist := math.Sqrt(d2); dist > 1e-9 {
				push = d.Scale((reach - dist) / dist)
			} else {
				push = Vec2{X: reach}
			}
			a.pos = a.pos.Sub(push.Scale(0.5))
			b.pos = b.pos.Add(push.Scale(0.5))
			return true
		})
		a.pos = r.clamp(a.pos, a.radius)
	}
}

// moveShots flies each shot on, hits the first enemy it reaches and drops it when it is spent.
func (r *Run) moveShots() {
	for i := 0; i < len(r.shots); {
		s := &r.shots[i]
		s.pos = s.pos.Add(s.vel.Scale(stepSeconds))
		s.life -= stepSeconds
		hit := false
		r.grid.nearby(s.pos, s.radius+r.maxRadius, func(j int) bool {
			e := &r.enemies[j]
			reach := s.radius + e.radius
			if e.pos.Sub(s.pos).LenSq() > reach*reach {
				return true
			}
			r.hitEnemy(j, s)
			hit = true
			return false
		})
		spent := hit || s.life <= 0 || !r.inside(s.pos, s.radius)
		if !spent {
			i++
			continue
		}
		last := len(r.shots) - 1
		r.shots[i] = r.shots[last]
		r.shots = r.shots[:last]
	}
}

// hitEnemy takes s's damage off enemy j and pushes it along the shot.
func (r *Run) hitEnemy(j int, s *shot) {
	e := &r.enemies[j]
	e.hp -= s.damage
	push := e.pos.Sub(s.pos).Normalized()
	if push == (Vec2{}) {
		push = s.vel.Normalized()
	}
	e.knockback = e.knockback.Add(push.Scale(s.knockback * (1 - e.knockbackResist)))
	r.emit(Event{Kind: EventHit, ID: e.id, Pos: e.pos, Value: s.damage})
}

// contact takes the first touching enemy's damage off the keeper, once per invulnerability window.
func (r *Run) contact() {
	if r.keeper.invuln > 0 {
		return
	}
	for i := range r.enemies {
		e := &r.enemies[i]
		reach := r.data.run.Keeper.Radius + e.radius
		if e.pos.Sub(r.keeper.pos).LenSq() > reach*reach {
			continue
		}
		r.keeper.health -= e.damage
		r.keeper.invuln = r.data.run.Keeper.Invulnerable
		r.emit(Event{Kind: EventHit, ID: e.id, Pos: e.pos, Value: e.damage})
		return
	}
}

// fire shoots Ember Bolt at the nearest enemy in range once its cooldown has run out.
func (r *Run) fire() {
	if r.weaponCooldown > 0 || len(r.shots) >= maxShots {
		return
	}
	target, ok := r.nearestEnemy(r.keeper.pos, r.data.run.Weapon.Range)
	if !ok {
		return
	}
	dir := target.Sub(r.keeper.pos).Normalized()
	if dir == (Vec2{}) {
		dir = Vec2{X: 1}
	}
	w := r.data.run.Weapon
	r.shots = append(r.shots, shot{
		id:        r.takeID(),
		pos:       r.keeper.pos,
		vel:       dir.Scale(w.ShotSpeed),
		damage:    r.stats.damage,
		radius:    w.ShotRadius,
		knockback: w.Knockback,
		life:      w.ShotLife,
	})
	r.weaponCooldown = r.stats.cooldown
	r.emit(Event{Kind: EventShot, ID: r.shots[len(r.shots)-1].id, Pos: r.keeper.pos})
}

// nearestEnemy returns the position of the closest enemy within radius of center, breaking ties by its position in the slice, so the same seed aims the same way.
func (r *Run) nearestEnemy(center Vec2, radius float64) (Vec2, bool) {
	best := -1
	var bestDist float64
	r.grid.nearby(center, radius, func(i int) bool {
		d2 := r.enemies[i].pos.Sub(center).LenSq()
		if d2 > radius*radius {
			return true
		}
		if best < 0 || d2 < bestDist || (d2 == bestDist && i < best) {
			best, bestDist = i, d2
		}
		return true
	})
	if best < 0 {
		return Vec2{}, false
	}
	return r.enemies[best].pos, true
}

// reap removes the enemies whose health is gone, counts them and leaves their embers where they fell.
func (r *Run) reap() {
	for i := 0; i < len(r.enemies); {
		e := &r.enemies[i]
		if e.hp > 0 {
			i++
			continue
		}
		r.kills++
		r.emit(Event{Kind: EventKill, ID: e.id, Pos: e.pos})
		r.dropEmbers(e)
		last := len(r.enemies) - 1
		r.enemies[i] = r.enemies[last]
		r.enemies = r.enemies[:last]
	}
}

// dropEmbers scatters what an enemy leaves around where it died.
func (r *Run) dropEmbers(e *enemy) {
	em := r.data.run.Embers
	for range e.embers {
		if len(r.embers) >= maxEmbers {
			return
		}
		offset := Vec2{r.drops.Range(-1, 1), r.drops.Range(-1, 1)}.Scale(em.Radius * 2)
		r.embers = append(r.embers, ember{pos: e.pos.Add(offset), value: em.Value})
	}
}

// gatherEmbers pulls the embers near the keeper in and gives their XP up when they reach it.
func (r *Run) gatherEmbers() {
	em := r.data.run.Embers
	for i := 0; i < len(r.embers); {
		e := &r.embers[i]
		to := r.keeper.pos.Sub(e.pos)
		dist := to.Len()
		if dist <= em.MagnetRadius {
			e.attracted = true
		}
		if e.attracted {
			if dist <= r.data.run.Keeper.Radius+em.Radius {
				r.xp += float64(e.value)
				r.emit(Event{Kind: EventEmber, Pos: e.pos, Value: float64(e.value)})
				last := len(r.embers) - 1
				r.embers[i] = r.embers[last]
				r.embers = r.embers[:last]
				continue
			}
			e.pos = e.pos.Add(to.Scale(math.Min(dist, em.MagnetSpeed*stepSeconds) / dist))
		}
		i++
	}
}

// fillLevel spends the XP bar on a new level and stops the run for three cards.
func (r *Run) fillLevel() {
	need := r.xpNeeded()
	if r.xp < need {
		return
	}
	r.xp -= need
	r.level++
	r.offersNow = r.drawOffers()
	r.phase = PhaseLevelUp
	r.emit(Event{Kind: EventLevelUp, Value: float64(r.level)})
}

// defeat ends the run when the keeper's health is gone.
func (r *Run) defeat() {
	if r.phase == PhaseDefeat {
		return
	}
	r.phase = PhaseDefeat
	r.offersNow = r.offersNow[:0]
	r.emit(Event{Kind: EventDefeat, Pos: r.keeper.pos})
}

// xpNeeded is how much XP the next level costs.
func (r *Run) xpNeeded() float64 {
	return r.data.run.Level.First + r.data.run.Level.Step*float64(r.level-1)
}

// takeID gives the next entity id, which events use instead of a slice index that a removal would reuse.
func (r *Run) takeID() uint64 {
	id := r.nextID
	r.nextID++
	return id
}

// clamp keeps a position at least radius inside the arena, the only wall a run has.
func (r *Run) clamp(p Vec2, radius float64) Vec2 {
	return Vec2{
		X: math.Min(math.Max(p.X, radius), r.data.run.Arena.Width-radius),
		Y: math.Min(math.Max(p.Y, radius), r.data.run.Arena.Height-radius),
	}
}

// inside reports whether a circle at p fits inside the arena.
func (r *Run) inside(p Vec2, radius float64) bool {
	return p.X >= radius && p.Y >= radius && p.X <= r.data.run.Arena.Width-radius && p.Y <= r.data.run.Arena.Height-radius
}

// Snapshot is a read-only view of the run for the shell to draw, which the next Step invalidates: its slices reuse the run's buffers.
type Snapshot struct {
	Phase        Phase       `json:"phase"`
	Time         float64     `json:"time"`
	Arena        Vec2        `json:"arena"`
	Keeper       Vec2        `json:"keeper"`
	KeeperRadius float64     `json:"keeperRadius"`
	Health       float64     `json:"health"`
	MaxHealth    float64     `json:"maxHealth"`
	Invulnerable float64     `json:"invulnerable"`
	Level        int         `json:"level"`
	XP           float64     `json:"xp"`
	XPNeeded     float64     `json:"xpNeeded"`
	Kills        int         `json:"kills"`
	Enemies      []EnemyView `json:"enemies"`
	Shots        []ShotView  `json:"shots"`
	Embers       []EmberView `json:"embers"`
	Offers       []OfferView `json:"offers"`
}

// EnemyView is one enemy as a snapshot shows it.
type EnemyView struct {
	ID        uint64  `json:"id"`
	Kind      string  `json:"kind"`
	Pos       Vec2    `json:"pos"`
	Radius    float64 `json:"radius"`
	Health    float64 `json:"health"`
	MaxHealth float64 `json:"maxHealth"`
}

// ShotView is one shot as a snapshot shows it.
type ShotView struct {
	ID     uint64  `json:"id"`
	Pos    Vec2    `json:"pos"`
	Radius float64 `json:"radius"`
}

// EmberView is one ember as a snapshot shows it.
type EmberView struct {
	Pos   Vec2 `json:"pos"`
	Value int  `json:"value"`
}

// OfferView is one level-up card as a snapshot shows it.
type OfferView struct {
	Name     string `json:"name"`
	Stat     string `json:"stat,omitempty"`
	Level    int    `json:"level,omitempty"`
	MaxLevel int    `json:"maxLevel,omitempty"`
	Heal     bool   `json:"heal,omitempty"`
}

// Snapshot builds the run's view for this step.
func (r *Run) Snapshot() Snapshot {
	s := &r.snap
	s.Phase = r.phase
	s.Time = r.time
	s.Arena = Vec2{r.data.run.Arena.Width, r.data.run.Arena.Height}
	s.Keeper = r.keeper.pos
	s.KeeperRadius = r.data.run.Keeper.Radius
	s.Health = r.keeper.health
	s.MaxHealth = r.stats.maxHealth
	s.Invulnerable = r.keeper.invuln
	s.Level = r.level
	s.XP = r.xp
	s.XPNeeded = r.xpNeeded()
	s.Kills = r.kills

	s.Enemies = s.Enemies[:0]
	for i := range r.enemies {
		e := &r.enemies[i]
		s.Enemies = append(s.Enemies, EnemyView{
			ID:        e.id,
			Kind:      r.data.enemies.Enemies[e.kind].Name,
			Pos:       e.pos,
			Radius:    e.radius,
			Health:    e.hp,
			MaxHealth: e.maxHP,
		})
	}
	s.Shots = s.Shots[:0]
	for i := range r.shots {
		sh := &r.shots[i]
		s.Shots = append(s.Shots, ShotView{ID: sh.id, Pos: sh.pos, Radius: sh.radius})
	}
	s.Embers = s.Embers[:0]
	for i := range r.embers {
		s.Embers = append(s.Embers, EmberView{Pos: r.embers[i].pos, Value: r.embers[i].value})
	}
	s.Offers = s.Offers[:0]
	for i := range r.offersNow {
		o := &r.offersNow[i]
		s.Offers = append(s.Offers, OfferView{Name: o.name, Stat: o.stat, Level: o.level, MaxLevel: o.maxLevel, Heal: o.heal})
	}
	return *s
}

// grid is the uniform grid a run rebuilds each step, so shot hits and enemy separation look at a few cells instead of every enemy.
type grid struct {
	cell float64
	cols int
	rows int
	head []int32
	next []int32
}

// newGrid starts an empty grid covering width by height.
func newGrid(cell, width, height float64) grid {
	cols := int(math.Ceil(width/cell)) + 1
	rows := int(math.Ceil(height/cell)) + 1
	return grid{cell: cell, cols: cols, rows: rows, head: make([]int32, cols*rows), next: make([]int32, maxEnemies)}
}

// build puts every enemy in its cell, reusing the previous step's memory.
func (g *grid) build(enemies []enemy) {
	for i := range g.head {
		g.head[i] = 0
	}
	if cap(g.next) < len(enemies) {
		g.next = make([]int32, len(enemies))
	} else {
		g.next = g.next[:len(enemies)]
	}
	for i := range enemies {
		c := g.cellIndex(enemies[i].pos)
		g.next[i] = g.head[c]
		g.head[c] = int32(i + 1)
	}
}

// cellIndex returns the cell holding p, clamped to the grid's edge.
func (g *grid) cellIndex(p Vec2) int {
	col := int(math.Floor(p.X / g.cell))
	row := int(math.Floor(p.Y / g.cell))
	col = min(max(col, 0), g.cols-1)
	row = min(max(row, 0), g.rows-1)
	return row*g.cols + col
}

// nearby visits the enemies in every cell that a circle at p of radius reach touches, stopping when visit returns false. The order is the grid's, not the slice's.
func (g *grid) nearby(p Vec2, reach float64, visit func(i int) bool) {
	minCol := max(int(math.Floor((p.X-reach)/g.cell)), 0)
	maxCol := min(int(math.Floor((p.X+reach)/g.cell)), g.cols-1)
	minRow := max(int(math.Floor((p.Y-reach)/g.cell)), 0)
	maxRow := min(int(math.Floor((p.Y+reach)/g.cell)), g.rows-1)
	for row := minRow; row <= maxRow; row++ {
		for col := minCol; col <= maxCol; col++ {
			for e := g.head[row*g.cols+col]; e != 0; e = g.next[e-1] {
				if !visit(int(e - 1)) {
					return
				}
			}
		}
	}
}
