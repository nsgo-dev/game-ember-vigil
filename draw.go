package main

import (
	"image"
	"math"

	"github.com/nsgo-dev/game/packages/game/lights2d"
	"github.com/nsgo-dev/game/packages/game/random"
	"github.com/nsgo-dev/game/packages/game/sprite"
	"github.com/nsgo-dev/nsgo/core"
)

// The arena's look: how fast its camera catches the keeper, how far the lantern reaches into the dark, and how much the frame may shake.
const (
	cameraEase      = 8.0
	lanternRadius   = 110
	lanternFalloff  = 2.0
	ambientDarkness = 0.62
	shakePixels     = 3.0
)

// The arena's effect limits: how long a hit flashes, how long a puff lives, and how many particles and glow lights one frame may hold, so a crowd never costs the frame budget.
const (
	flashSeconds  = 0.1
	puffSeconds   = 0.45
	sparkSeconds  = 0.3
	trailSeconds  = 0.35
	maxParticles  = 240
	maxGlowLights = 24
	emberTrail    = 0.03
)

// viewMargin is how far outside the canvas a sprite may sit and still draw, which keeps a body half in view whole.
const viewMargin = 40

// arenaColor is what the canvas shows behind the ground and the darkness.
var arenaColor = core.RGB(0x0b0d17)

// hitColor is the colour a struck enemy flashes.
var hitColor = core.RGB(0xffd9a0)

// The enemy kinds the arena draws, which art.frames holds a picture for each of.
const (
	kindMite = iota
	kindMoth
	kindHusk
)

// enemyKind maps a snapshot's enemy name to the picture the arena draws for it.
func enemyKind(name string) int {
	switch name {
	case "Moth":
		return kindMoth
	case "Husk":
		return kindHusk
	default:
		return kindMite
	}
}

// camera is the arena's window onto the world: the world point under the canvas's top-left corner, which never leaves the arena.
type camera struct {
	x, y float64
}

// follow eases c toward centering target, at a rate that does not depend on the frame's length, and clamps it to the arena.
func (c *camera) follow(target, arena Vec2, dt float64) {
	k := 1 - math.Exp(-cameraEase*dt)
	c.x += (target.X - viewWidth/2 - c.x) * k
	c.y += (target.Y - viewHeight/2 - c.y) * k
	c.clamp(arena)
}

// snap puts c on target at once, which a run's first step and a retry do instead of sliding in from the last run's place.
func (c *camera) snap(target, arena Vec2) {
	c.x = target.X - viewWidth/2
	c.y = target.Y - viewHeight/2
	c.clamp(arena)
}

// clamp keeps the camera's window inside the arena; an arena smaller than the view pins it to the corner.
func (c *camera) clamp(arena Vec2) {
	c.x = clampLength(c.x, 0, arena.X-viewWidth)
	c.y = clampLength(c.y, 0, arena.Y-viewHeight)
}

// actor is one drawn thing: what kind it is, where it is now and where it was on the step before, which the loop's alpha blends between.
type actor struct {
	id     uint64
	kind   int
	radius float64
	pos    Vec2
	was    Vec2
}

// particle is one spark, puff or trail: where it drifts from, how, and how long it has left.
type particle struct {
	pos, vel  Vec2
	life, max float64
	kind      int
}

// The particle kinds, which each draw one picture.
const (
	particleSpark = iota
	particlePuff
	particleTrail
)

// effects are the arena's short-lived looks: hit flashes, death puffs, ember trails and the screen's shake. Every list is capped, so a crowd cannot cost a frame.
type effects struct {
	flash     map[uint64]float64
	particles []particle
	// shake is how hard the frame shakes, from 0 to 1, and shakeX and shakeY where it moves the camera this frame.
	shake          float64
	shakeX, shakeY float64
	time           float64
}

// arenaRenderer draws the run the shell steps: the ground painted once per run, one sprite batch for everything alive, the effects above them and the lantern's light over it all. It copies each snapshot, since a run reuses its buffers, and never changes the run.
type arenaRenderer struct {
	art   *art
	arena Vec2
	seed  uint64

	// ground is the arena painted once from the run's seed, and groundSource what the canvas draws its camera's crop from.
	ground       *image.RGBA
	groundSource core.ImageSource

	cam     camera
	alpha   float64
	started bool
	// seenKeeper is whether the keeper's last position is the previous step's, which the first step of a run has no need of.
	seenKeeper bool
	// invuln is how much of the keeper's invulnerability is left, which the keeper blinks through.
	invuln float64

	keeper  actor
	enemies []actor
	shots   []actor
	embers  []actor
	halo    []actor
	patches []PatchView
	pulses  []PulseView

	// before holds where each id was last step so a frame between steps can blend; spare is the map the next step fills, swapped in when it is done.
	before, spare map[uint64]Vec2

	fx        effects
	cosmetics *random.Stream

	// fresh is whether a step arrived since the last frame, which a paused run does not do: a frame between steps blends, a frame without a step holds still.
	fresh bool

	// batch, pool, effects, lights and the lantern are reused every frame, so drawing a steady frame allocates nothing.
	batch    sprite.Batch
	pool     []*sprite.Sprite
	used     int
	flash    []sprite.Effect
	flashing int
	scene    lights2d.Scene
	lights   []*lights2d.Light
	glows    []*lights2d.Light
	lantern  *lights2d.Light
}

// newArenaRenderer prepares the arena's art, which every run of the app shares.
func newArenaRenderer(a *art) *arenaRenderer {
	r := &arenaRenderer{
		art:       a,
		before:    map[uint64]Vec2{},
		spare:     map[uint64]Vec2{},
		cosmetics: random.NewStreams(0).Stream("cosmetics"),
		fx:        effects{flash: map[uint64]float64{}},
		lantern:   lights2d.NewLight(0, 0, lanternRadius, core.RGB(0xffb347)),
	}
	r.lantern.Falloff = lanternFalloff
	r.lantern.Intensity = 0.5
	for range maxGlowLights {
		r.glows = append(r.glows, lights2d.NewLight(0, 0, 0, core.RGB(0xffffff)))
	}
	return r
}

// startRun readies the arena for a run with seed: it forgets the last run's actors and ground, and starts the cosmetic stream the effects and the ground decoration draw from. The ground is painted on the first step, when the run reports its arena.
func (r *arenaRenderer) startRun(seed uint64) {
	r.seed = seed
	r.ground = nil
	r.groundSource = core.ImageSource{}
	r.keeper = actor{}
	r.enemies = r.enemies[:0]
	r.shots = r.shots[:0]
	r.embers = r.embers[:0]
	r.halo = r.halo[:0]
	r.patches = r.patches[:0]
	r.pulses = r.pulses[:0]
	clear(r.before)
	clear(r.spare)
	r.fx = effects{flash: map[uint64]float64{}}
	r.alpha = 0
	r.fresh = false
	r.started, r.seenKeeper = false, false
	r.invuln = 0
	r.cosmetics = random.NewStreams(seed).Stream("cosmetics")
}

// paintGround paints the arena's ground from the run's seed, which every frame then draws its camera's crop of.
func (r *arenaRenderer) paintGround() {
	if r.arena.X <= 0 || r.arena.Y <= 0 {
		return
	}
	r.ground = r.art.newGround(r.seed, int(r.arena.X), int(r.arena.Y))
	r.groundSource = core.ImageSource{Pixels: r.ground, Version: 1}
}

// sync copies a step's snapshot and events for drawing: where everything is this step, where it was last, and the effects the step caused.
func (r *arenaRenderer) sync(s Snapshot, events []Event, dt float64) {
	if s.Arena != r.arena {
		r.arena = s.Arena
		r.paintGround()
	}
	if !r.started {
		r.cam.snap(s.Keeper, r.arena)
		r.started = true
	}
	r.cam.follow(s.Keeper, r.arena, dt)
	r.invuln = s.Invulnerable
	clear(r.spare)
	r.trackKeeper(s)
	r.trackEnemies(s)
	r.trackShots(s)
	r.before, r.spare = r.spare, r.before
	r.trackEmbers(s)
	r.trackHalo(s)
	r.patches = append(r.patches[:0], s.Patches...)
	r.pulses = append(r.pulses[:0], s.Pulses...)
	r.stepEffects(events, dt)
	r.emitTrails(dt)
	r.fresh = true
}

// trackKeeper keeps the keeper's place this step and the step before.
func (r *arenaRenderer) trackKeeper(s Snapshot) {
	was := r.keeper.pos
	if !r.seenKeeper {
		was, r.seenKeeper = s.Keeper, true
	}
	r.keeper = actor{pos: s.Keeper, was: was, radius: s.KeeperRadius}
}

// trackEnemies copies the snapshot's enemies, reading where each was last step by its id.
func (r *arenaRenderer) trackEnemies(s Snapshot) {
	r.enemies = growActors(r.enemies, len(s.Enemies))
	for i, e := range s.Enemies {
		r.enemies[i] = r.tracked(r.enemies[i], e.ID, enemyKind(e.Kind), e.Radius, e.Pos)
	}
}

// trackShots copies the snapshot's shots, blending them by id as enemies are.
func (r *arenaRenderer) trackShots(s Snapshot) {
	r.shots = growActors(r.shots, len(s.Shots))
	for i, sh := range s.Shots {
		r.shots[i] = r.tracked(r.shots[i], sh.ID, 0, sh.Radius, sh.Pos)
	}
}

// trackEmbers copies the snapshot's embers, which have no id: they blend from the previous step only while the same number of them is alive.
func (r *arenaRenderer) trackEmbers(s Snapshot) {
	same := len(r.embers) == len(s.Embers)
	r.embers = growActors(r.embers, len(s.Embers))
	for i, e := range s.Embers {
		was := e.Pos
		if same {
			was = r.embers[i].pos
		}
		r.embers[i] = actor{pos: e.Pos, was: was}
	}
}

// trackHalo copies Lantern Halo's orbiting embers, blending between steps while the orb count stays the same.
func (r *arenaRenderer) trackHalo(s Snapshot) {
	same := len(r.halo) == len(s.Halo)
	r.halo = growActors(r.halo, len(s.Halo))
	for i, orb := range s.Halo {
		was := orb.Pos
		if same {
			was = r.halo[i].pos
		}
		r.halo[i] = actor{pos: orb.Pos, was: was, radius: orb.Radius}
	}
}

// tracked places a into the actor list at id's position, remembering where it was last step for the frame's blend.
func (r *arenaRenderer) tracked(a actor, id uint64, kind int, radius float64, pos Vec2) actor {
	was, seen := r.before[id]
	if !seen {
		was = pos
	}
	r.spare[id] = pos
	a.id, a.kind, a.radius, a.pos, a.was = id, kind, radius, pos, was
	return a
}

// growActors returns a shortened to n, growing it to hold n when it is smaller.
func growActors(a []actor, n int) []actor {
	if cap(a) < n {
		return make([]actor, n)
	}
	return a[:n]
}

// stepEffects moves the particles, fades the flashes and shakes the frame, then adds what this step's events caused.
func (r *arenaRenderer) stepEffects(events []Event, dt float64) {
	fx := &r.fx
	fx.time += dt
	for i := 0; i < len(fx.particles); {
		p := &fx.particles[i]
		p.life -= dt
		if p.life <= 0 {
			last := len(fx.particles) - 1
			fx.particles[i] = fx.particles[last]
			fx.particles = fx.particles[:last]
			continue
		}
		p.pos = p.pos.Add(p.vel.Scale(dt))
		p.vel = p.vel.Scale(math.Exp(-3 * dt))
		i++
	}
	for id, left := range fx.flash {
		if left -= dt; left <= 0 {
			delete(fx.flash, id)
		} else {
			fx.flash[id] = left
		}
	}
	fx.shake = math.Max(0, fx.shake-2.5*dt)
	amp := shakePixels * fx.shake * fx.shake
	fx.shakeX = amp * math.Sin(fx.time*97)
	fx.shakeY = amp * math.Cos(fx.time*131)

	for _, e := range events {
		switch e.Kind {
		case EventHit:
			fx.flash[e.ID] = flashSeconds
		case EventKill:
			r.burst(e.Pos, particlePuff, 4, 22, puffSeconds)
			fx.shake = math.Min(1, fx.shake+0.12)
		case EventEmber:
			r.burst(e.Pos, particleSpark, 2, 30, sparkSeconds)
		case EventDefeat:
			fx.shake = 1
		}
	}
}

// emitTrails now and then leaves a trail behind an ember on the ground, purely for the eye: the cosmetic stream decides, never the run.
func (r *arenaRenderer) emitTrails(dt float64) {
	if dt <= 0 {
		return
	}
	for i := range r.embers {
		if len(r.fx.particles) >= maxParticles {
			return
		}
		if !r.cosmetics.Chance(emberTrail) {
			continue
		}
		r.fx.particles = append(r.fx.particles, particle{
			pos:  r.embers[i].pos,
			vel:  Vec2{},
			life: trailSeconds,
			max:  trailSeconds,
			kind: particleTrail,
		})
	}
}

// burst scatters n particles of kind from pos, each in a direction the cosmetic stream picks.
func (r *arenaRenderer) burst(pos Vec2, kind, n int, speed, life float64) {
	for range n {
		if len(r.fx.particles) >= maxParticles {
			return
		}
		dir := Vec2{r.cosmetics.Range(-1, 1), r.cosmetics.Range(-1, 1)}.Normalized()
		r.fx.particles = append(r.fx.particles, particle{
			pos:  pos,
			vel:  dir.Scale(speed * r.cosmetics.Range(0.4, 1)),
			life: life * r.cosmetics.Range(0.6, 1),
			max:  life,
			kind: kind,
		})
	}
}

// draw paints one frame of the run at alpha, the loop's distance between the last step and the next. A frame with no step behind it, such as a paused run's, holds the last step's places instead of drifting between them.
func (r *arenaRenderer) draw(d *core.Drawing, alpha float64) {
	if !r.fresh {
		alpha = 1
	}
	r.fresh = false
	r.alpha = alpha
	w, h := float32(viewWidth), float32(viewHeight)
	d.SetImageSmoothing(false)
	d.SetFillColor(arenaColor).FillRect(0, 0, w, h)
	if r.ground == nil {
		return
	}
	ox, oy := r.origin()
	d.DrawImageFrom(r.groundSource, ox, oy, w, h, 0, 0, w, h)
	r.drawPatches(d, ox, oy)

	r.used, r.flashing = 0, 0
	r.addEmbers(ox, oy)
	r.addEnemies(ox, oy)
	r.addShots(ox, oy)
	r.addHalo(ox, oy)
	r.addKeeper(ox, oy)
	r.addParticles(ox, oy)
	r.batch = sprite.Batch{Atlas: r.art.atlas, Sprites: r.pool[:r.used]}
	r.batch.Draw(d)
	r.drawPulses(d, ox, oy)

	r.drawLight(d, w, h)
}

// origin returns the whole virtual pixels the frame's world is drawn from, with the camera's shake added and kept inside the arena.
func (r *arenaRenderer) origin() (float32, float32) {
	x := clampLength(math.Round(r.cam.x+r.fx.shakeX), 0, r.arena.X-viewWidth)
	y := clampLength(math.Round(r.cam.y+r.fx.shakeY), 0, r.arena.Y-viewHeight)
	return float32(x), float32(y)
}

// toScreen returns a world position in canvas pixels, rounded so the pixel art stays crisp.
func toScreen(p Vec2, ox, oy float32) (float32, float32) {
	return float32(math.Round(p.X)) - ox, float32(math.Round(p.Y)) - oy
}

// inView reports whether a canvas position is close enough to the canvas to be worth drawing.
func inView(x, y float32) bool {
	return x >= -viewMargin && x <= viewWidth+viewMargin && y >= -viewMargin && y <= viewHeight+viewMargin
}

// addEmbers adds the embers waiting on the ground.
func (r *arenaRenderer) addEmbers(ox, oy float32) {
	for i := range r.embers {
		r.add(r.art.frames.ember, r.embers[i], ox, oy, 1, 1)
	}
}

// addEnemies adds the living enemies, with a struck one flashing toward hitColor, and a moth bobbing on the wing.
func (r *arenaRenderer) addEnemies(ox, oy float32) {
	f := r.art.frames
	for i := range r.enemies {
		e := &r.enemies[i]
		frame := f.mite
		switch e.kind {
		case kindMoth:
			frame = f.moth
		case kindHusk:
			frame = f.husk
		}
		s := r.add(frame, *e, ox, oy, 1, 2)
		if s == nil {
			continue
		}
		if e.kind == kindMoth {
			s.Y += float32(math.Round(math.Sin(r.fx.time*6 + float64(e.id))))
		}
		if left, ok := r.fx.flash[e.id]; ok {
			s.Effect = r.flasher(left)
		}
	}
}

// addShots adds the Ember Bolts in flight above the enemies they are about to hit.
func (r *arenaRenderer) addShots(ox, oy float32) {
	for i := range r.shots {
		r.add(r.art.frames.bolt, r.shots[i], ox, oy, 1, 3)
	}
}

// addHalo adds Lantern Halo's orbiting embers around the keeper.
func (r *arenaRenderer) addHalo(ox, oy float32) {
	for i := range r.halo {
		r.add(r.art.frames.halo, r.halo[i], ox, oy, 1, 3)
	}
}

// drawPatches paints Cinder Trail's burning ground patches over the forest floor before sprites draw.
func (r *arenaRenderer) drawPatches(d *core.Drawing, ox, oy float32) {
	for i := range r.patches {
		p := &r.patches[i]
		x, y := toScreen(p.Pos, ox, oy)
		rad := float32(p.Radius)
		if x+rad < 0 || x-rad > viewWidth || y+rad < 0 || y-rad > viewHeight {
			continue
		}
		fade := float32(math.Min(1, p.Life/0.5))
		flicker := float32(1 + 0.06*math.Sin(r.fx.time*14+float64(p.ID)))
		d.SetFillColor(core.RGBA(0xa3341a, 0.42*fade)).
			BeginPath().Arc(x, y, rad*flicker, 0, 2*math.Pi, false).Fill()
		d.SetFillColor(core.RGBA(0xff7a1a, 0.55*fade)).
			BeginPath().Arc(x, y, rad*0.58*flicker, 0, 2*math.Pi, false).Fill()
	}
}

// drawPulses paints Bell Toll's expanding shockwave rings over the sprites.
func (r *arenaRenderer) drawPulses(d *core.Drawing, ox, oy float32) {
	for i := range r.pulses {
		p := &r.pulses[i]
		x, y := toScreen(p.Pos, ox, oy)
		t := float32(1 - p.Life/max(p.Max, 0.001))
		rad := float32(p.Radius) * (0.2 + 0.8*t)
		if x+rad < 0 || x-rad > viewWidth || y+rad < 0 || y-rad > viewHeight {
			continue
		}
		alpha := float32(math.Max(0, 1-float64(t)))
		d.SetFillColor(core.RGBA(0xffb347, 0.16*alpha)).
			BeginPath().Arc(x, y, rad, 0, 2*math.Pi, false).Fill()
		d.SetStrokeColor(core.RGBA(0xffd166, 0.85*alpha)).
			SetLineWidth(2).
			BeginPath().Arc(x, y, rad, 0, 2*math.Pi, false).Stroke()
	}
}

// addKeeper adds the keeper above its enemies, blinking off while contact leaves it invulnerable.
func (r *arenaRenderer) addKeeper(ox, oy float32) {
	s := r.add(r.art.frames.keeper, r.keeper, ox, oy, 1, 4)
	if s != nil && r.invuln > 0 && math.Mod(r.fx.time*16, 1) < 0.4 {
		s.Alpha = 0.5
	}
}

// addParticles adds the sparks, puffs and trails, fading as they age.
func (r *arenaRenderer) addParticles(ox, oy float32) {
	f := r.art.frames
	for i := range r.fx.particles {
		p := &r.fx.particles[i]
		frame := f.spark
		switch p.kind {
		case particlePuff:
			frame = f.puff
		case particleTrail:
			frame = f.trail
		}
		left := float32(math.Max(0, p.life/max(p.max, 0.001)))
		x, y := toScreen(p.pos, ox, oy)
		if !inView(x, y) {
			continue
		}
		s := r.next(frame)
		s.X, s.Y, s.Alpha, s.Z = x, y, left, 5
	}
}

// add adds one sprite of frame at a's blended position with the given alpha and depth, and returns it, or nil when it is off the canvas.
func (r *arenaRenderer) add(frame sprite.Frame, a actor, ox, oy float32, alpha float32, z int) *sprite.Sprite {
	p := Vec2{a.was.X + (a.pos.X-a.was.X)*r.alpha, a.was.Y + (a.pos.Y-a.was.Y)*r.alpha}
	x, y := toScreen(p, ox, oy)
	if !inView(x, y) {
		return nil
	}
	s := r.next(frame)
	s.X, s.Y, s.Alpha, s.Z = x, y, alpha, z
	return s
}

// next returns the frame's next sprite, reusing the pool so a steady frame allocates no sprite.
func (r *arenaRenderer) next(f sprite.Frame) *sprite.Sprite {
	if r.used == len(r.pool) {
		r.pool = append(r.pool, new(sprite.Sprite))
	}
	s := r.pool[r.used]
	r.used++
	*s = sprite.NewSprite(f)
	return s
}

// flasher returns a flash effect for an enemy struck left seconds ago, from the renderer's pool so a frame allocates none.
func (r *arenaRenderer) flasher(left float64) *sprite.Effect {
	if r.flashing == len(r.flash) {
		r.flash = append(r.flash, sprite.Effect{})
	}
	e := &r.flash[r.flashing]
	r.flashing++
	e.Flash = sprite.Flash{Color: hitColor, Strength: float32(left / flashSeconds)}
	return e
}

// drawLight lays the darkness over the frame and adds the lantern's light and the glow of shots and embers on top of it.
func (r *arenaRenderer) drawLight(d *core.Drawing, w, h float32) {
	ox, oy := r.origin()
	r.lights = append(r.lights[:0], r.lantern)
	lx, ly := toScreen(r.keeper.pos, ox, oy)
	flicker := 2.5*math.Sin(r.fx.time*9) + 1.2*math.Sin(r.fx.time*23.7)
	r.lantern.X, r.lantern.Y = lx, ly
	r.lantern.Radius = lanternRadius + float32(flicker)

	glow := 0
	for i := range r.halo {
		if glow == len(r.glows) {
			break
		}
		x, y := toScreen(r.halo[i].pos, ox, oy)
		if !inView(x, y) {
			continue
		}
		l := r.glows[glow]
		glow++
		*l = lights2d.Light{X: x, Y: y, Radius: 20, Color: core.RGBA(0xffb347, 0.75), Intensity: 0.45, Falloff: 2, Enabled: true}
		r.lights = append(r.lights, l)
	}
	for i := range r.patches {
		if glow == len(r.glows) {
			break
		}
		x, y := toScreen(r.patches[i].Pos, ox, oy)
		if !inView(x, y) {
			continue
		}
		l := r.glows[glow]
		glow++
		*l = lights2d.Light{X: x, Y: y, Radius: float32(r.patches[i].Radius * 1.4), Color: core.RGBA(0xff7a1a, 0.6), Intensity: 0.35, Falloff: 2, Enabled: true}
		r.lights = append(r.lights, l)
	}
	for i := range r.shots {
		if glow == len(r.glows) {
			break
		}
		x, y := toScreen(r.shots[i].pos, ox, oy)
		if !inView(x, y) {
			continue
		}
		l := r.glows[glow]
		glow++
		*l = lights2d.Light{X: x, Y: y, Radius: 16, Color: core.RGBA(0xffb347, 0.7), Intensity: 0.4, Falloff: 2, Enabled: true}
		r.lights = append(r.lights, l)
	}
	for i := range r.embers {
		if glow == len(r.glows) {
			break
		}
		x, y := toScreen(r.embers[i].pos, ox, oy)
		if !inView(x, y) {
			continue
		}
		l := r.glows[glow]
		glow++
		*l = lights2d.Light{X: x, Y: y, Radius: 12, Color: core.RGBA(0xffd166, 0.55), Intensity: 0.35, Falloff: 2, Enabled: true}
		r.lights = append(r.lights, l)
	}

	r.scene = lights2d.Scene{
		Ambient:        core.RGBA(0x05060c, ambientDarkness),
		Lights:         r.lights,
		DisableShadows: true,
	}
	r.scene.Draw(d, core.Bounds{Size: core.Size{Width: core.Points(w), Height: core.Points(h)}})
}
