package main

import (
	"image"
	"math"
	"time"

	"github.com/nsgo-dev/game/packages/game/lights2d"
	"github.com/nsgo-dev/game/packages/game/random"
	"github.com/nsgo-dev/game/packages/game/sprite"
	"github.com/nsgo-dev/nsgo/core"
)

// The arena's look: how fast its camera catches the keeper, how dark the moonlit twilight is, how far the keeper sees through the fog of war, and how much the frame may shake.
const (
	cameraEase      = 8.0
	ambientDarkness = 0.26
	visionClear     = 170.0
	visionSolid     = 290.0
	shakePixels     = 3.0
)

// The arena's effect limits: how long flashes, puffs and flourishes live, and how many particles and glow lights one frame may hold, so a crowd never costs the frame budget.
const (
	flashSeconds          = 0.1
	puffSeconds           = 0.45
	sparkSeconds          = 0.3
	trailSeconds          = 0.35
	levelFlashSeconds     = 0.55
	damageVignetteSeconds = 0.45
	maxParticles          = 240
	maxGlowLights         = 24
	emberTrail            = 0.03
)

// viewMargin is how far outside the canvas a sprite may sit and still draw, which keeps a body half in view whole.
const viewMargin = 40

// arenaColor is what the canvas shows behind the ground and the twilight overlay.
var arenaColor = core.RGB(0x1b2233)

// ambientTint is the moonlit twilight tint laid over the arena before point lights brighten it.
var ambientTint = core.RGBA(0x0c1222, ambientDarkness)

// hitColor is the warm bright colour a struck enemy flashes.
var hitColor = core.RGB(0xfff1d6)

// The enemy kinds the arena draws, which art.frames holds a picture for each of.
const (
	kindMite = iota
	kindMoth
	kindHusk
	kindThornling
	kindWisp
	kindShade
	kindGloomwarden
	kindHollowKing
)

// enemyKind maps a snapshot's enemy name to the picture the arena draws for it.
func enemyKind(name string) int {
	switch name {
	case "Moth":
		return kindMoth
	case "Husk":
		return kindHusk
	case "Thornling":
		return kindThornling
	case "Wisp":
		return kindWisp
	case "Shade":
		return kindShade
	case "Gloomwarden":
		return kindGloomwarden
	case "Hollow King":
		return kindHollowKing
	default:
		return kindMite
	}
}

// camera is the arena's window onto the world: the world point under the canvas's top-left corner, which never leaves the arena, where it was on the step before, which a frame blends from as it does the actors, and the size of the world the window shows, which follows the window's shape.
type camera struct {
	x, y       float64
	wasX, wasY float64
	w, h       float64
}

// size returns the world area the camera shows, the run's view until a frame has told it the window's.
func (c *camera) size() (float64, float64) {
	if c.w <= 0 || c.h <= 0 {
		return viewWidth, viewHeight
	}
	return c.w, c.h
}

// resize shows w by h of the world from now on, keeping the window's centre where it was so a resize does not jolt the frame.
func (c *camera) resize(w, h float64, arena Vec2) {
	oldW, oldH := c.size()
	if w <= 0 || h <= 0 || (w == oldW && h == oldH) {
		return
	}
	dx, dy := (oldW-w)/2, (oldH-h)/2
	c.w, c.h = w, h
	c.x += dx
	c.y += dy
	c.wasX += dx
	c.wasY += dy
	c.clamp(arena)
	c.wasX = clampLength(c.wasX, 0, arena.X-w)
	c.wasY = clampLength(c.wasY, 0, arena.Y-h)
}

// follow eases c toward centering target, at a rate that does not depend on the frame's length, and clamps it to the arena.
func (c *camera) follow(target, arena Vec2, dt float64) {
	c.wasX, c.wasY = c.x, c.y
	k := 1 - math.Exp(-cameraEase*dt)
	w, h := c.size()
	c.x += (target.X - w/2 - c.x) * k
	c.y += (target.Y - h/2 - c.y) * k
	c.clamp(arena)
}

// snap puts c on target at once, which a run's first step and a retry do instead of sliding in from the last run's place.
func (c *camera) snap(target, arena Vec2) {
	w, h := c.size()
	c.x = target.X - w/2
	c.y = target.Y - h/2
	c.clamp(arena)
	c.wasX, c.wasY = c.x, c.y
}

// at returns the camera's window alpha of the way from the last step's to this step's.
func (c *camera) at(alpha float64) (float64, float64) {
	return c.wasX + (c.x-c.wasX)*alpha, c.wasY + (c.y-c.wasY)*alpha
}

// clamp keeps the camera's window inside the arena; an arena smaller than the view pins it to the corner.
func (c *camera) clamp(arena Vec2) {
	w, h := c.size()
	c.x = clampLength(c.x, 0, arena.X-w)
	c.y = clampLength(c.y, 0, arena.Y-h)
}

// actor is one drawn thing: what kind it is, where it is now and where it was on the step before, which the loop's alpha blends between.
type actor struct {
	id          uint64
	kind        int
	elite       bool
	radius      float64
	hp          float64
	maxHP       float64
	pos         Vec2
	was         Vec2
	facingRight bool
	angle       float32
}

// prevState remembers an actor's position, horizontal facing and travel angle from the previous step.
type prevState struct {
	pos         Vec2
	facingRight bool
	angle       float32
	hasAngle    bool
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

// effects are the arena's short-lived looks: hit flashes, death puffs, ember trails, level-up flourish, damage vignette and the screen's shake. Every list is capped, so a crowd cannot cost a frame.
type effects struct {
	flash     map[uint64]float64
	particles []particle
	// shake is how hard the frame shakes, from 0 to 1, and shakeX and shakeY where it moves the camera this frame.
	shake          float64
	shakeX, shakeY float64
	// damageVignette is the crimson screen-edge flash intensity [0, 1] when the keeper takes damage.
	damageVignette float64
	// levelFlash counts down the expanding golden level-up ring around the keeper.
	levelFlash float64
	time       float64
}

// arenaRenderer draws the run the shell steps: the ground painted once per run, one sprite batch for everything alive, the effects and glows above them and the fog of war over it all. It copies each snapshot, since a run reuses its buffers, and never changes the run.
type arenaRenderer struct {
	art   *art
	arena Vec2
	seed  uint64

	// ground is the arena painted once from the run's seed, and groundSource what the canvas draws its camera's crop from.
	ground       *image.RGBA
	groundSource core.ImageSource
	// fog is the fog of war's coarse mask over the arena's edges, painted with the ground, and fogMargin the band it covers.
	fog       *image.RGBA
	fogSource core.ImageSource
	fogMargin float64

	cam     camera
	alpha   float64
	started bool
	// seenKeeper is whether the keeper's last position is the previous step's, which the first step of a run has no need of.
	seenKeeper bool
	// invuln is how much of the keeper's invulnerability is left, which the keeper blinks through.
	invuln float64
	// lastHealth and maxHealth track keeper HP across steps for damage vignette and low-health cues.
	lastHealth float64
	maxHealth  float64
	seenHealth bool

	keeper     actor
	enemies    []actor
	shots      []actor
	enemyShots []actor
	embers     []actor
	pickups    []actor
	chests     []actor
	halo       []actor
	patches    []PatchView
	pulses     []PulseView
	shadows    []ShadowView
	arcs       []ArcView
	censers    []CenserView
	boss       BossView
	// sight is how far the keeper sees through the fog, as a fraction of the base run's, which a Long Night tier shortens.
	sight float64
	// opening is the chest the run holds still for, ring Sun Ring's burning ring, and stunned the stunned enemies' indices in enemies.
	opening OpeningView
	ring    RingView
	stunned []int

	// before holds where each id was last step so a frame between steps can blend; spare is the map the next step fills, swapped in when it is done.
	before, spare map[uint64]prevState

	// Animation playback state driven by fixed steps and frozen while paused or not playing.
	animTime          time.Duration
	keeperAnim        sprite.Animation
	keeperMoving      bool
	keeperAnimMoving  bool
	keeperFacingRight bool

	fx           effects
	shakeEnabled bool
	quality      qualityProfile
	cosmetics    *random.Stream

	// holding is whether the shell's last update left the run unstepped, as a paused run's does: a frame then holds the last step's places instead of blending toward a step that is not coming.
	holding bool

	// batch, pool, effects and lights are reused every frame, so drawing a steady frame allocates nothing.
	batch        sprite.Batch
	sheetBatch   sprite.Batch
	patchBatch   sprite.Batch
	pulseBatch   sprite.Batch
	ringBatch    sprite.Batch
	ringSprites  []*sprite.Sprite
	sparkBatch   sprite.Batch
	sparkSprites []*sprite.Sprite
	sheetSprites []*sprite.Sprite
	patchSprites []*sprite.Sprite
	pulseSprites []*sprite.Sprite
	pixelSprites []*sprite.Sprite
	pool         []*sprite.Sprite
	used         int
	flash        []sprite.Effect
	flashing     int
	scene        lights2d.Scene
	lights       []*lights2d.Light
	glows        []*lights2d.Light
}

// newArenaRenderer prepares the arena's art, which every run of the app shares.
func newArenaRenderer(a *art) *arenaRenderer {
	r := &arenaRenderer{
		art:               a,
		before:            map[uint64]prevState{},
		spare:             map[uint64]prevState{},
		keeperFacingRight: true,
		shakeEnabled:      true,
		quality:           defaultQualityProfile(),
		cosmetics:         random.NewStreams(0).Stream("cosmetics"),
		fx:                effects{flash: map[uint64]float64{}},
	}
	if a != nil && a.sheets.keeper.loaded {
		r.keeperAnim = a.sheets.keeper.idle
	}
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
	r.fog = nil
	r.fogSource = core.ImageSource{}
	r.keeper = actor{}
	r.enemies = r.enemies[:0]
	r.shots = r.shots[:0]
	r.enemyShots = r.enemyShots[:0]
	r.embers = r.embers[:0]
	r.pickups = r.pickups[:0]
	r.chests = r.chests[:0]
	r.opening = OpeningView{}
	r.ring = RingView{}
	r.stunned = r.stunned[:0]
	r.halo = r.halo[:0]
	r.patches = r.patches[:0]
	r.pulses = r.pulses[:0]
	r.shadows = r.shadows[:0]
	r.sight = 1
	r.boss = BossView{}
	r.sheetSprites = r.sheetSprites[:0]
	r.patchSprites = r.patchSprites[:0]
	r.pulseSprites = r.pulseSprites[:0]
	r.pixelSprites = r.pixelSprites[:0]
	clear(r.before)
	clear(r.spare)
	r.animTime = 0
	r.keeperMoving = false
	r.keeperAnimMoving = false
	r.keeperFacingRight = true
	if r.art != nil && r.art.sheets.keeper.loaded {
		r.keeperAnim = r.art.sheets.keeper.idle
		r.keeperAnim.Reset()
	} else {
		r.keeperAnim = sprite.Animation{}
	}
	r.fx = effects{flash: map[uint64]float64{}}
	r.alpha = 0
	r.holding = false
	r.started, r.seenKeeper = false, false
	r.invuln = 0
	r.lastHealth = 0
	r.maxHealth = 0
	r.seenHealth = false
	r.cosmetics = random.NewStreams(seed).Stream("cosmetics")
}

// paintGround paints the arena's ground from the run's seed, which every frame then draws its camera's crop of.
func (r *arenaRenderer) paintGround() {
	if r.arena.X <= 0 || r.arena.Y <= 0 {
		return
	}
	r.ground = r.art.newGround(r.seed, int(r.arena.X), int(r.arena.Y))
	r.groundSource = core.ImageSource{Pixels: r.ground, Version: 1}
	r.fog = nil
	r.fogSource = core.ImageSource{}
	if r.fogMargin > 0 {
		r.fog = newFogMask(r.seed, int(r.arena.X), int(r.arena.Y), r.fogMargin)
		r.fogSource = core.ImageSource{Pixels: r.fog, Version: 1}
	}
}

// drawFog lays the fog of war over the frame: the crop of the arena edges' coarse mask, scaled up smoothly, and the
// darkness past the keeper's sight, so whatever roams beyond either stays hidden until it comes close.
func (r *arenaRenderer) drawFog(d *core.Drawing, ox, oy, w, h float32) {
	if r.fog != nil {
		d.SetImageSmoothing(true)
		d.DrawImageFrom(r.fogSource, ox/fogScale, oy/fogScale, w/fogScale, h/fogScale, 0, 0, w, h)
		d.SetImageSmoothing(false)
	}
	if !r.seenKeeper {
		return
	}
	k := r.keeper
	x := float32(k.was.X+(k.pos.X-k.was.X)*r.alpha) - ox
	y := float32(k.was.Y+(k.pos.Y-k.was.Y)*r.alpha) - oy
	// The gradient runs past the farthest corner, so its last stop covers the whole frame however a canvas pads it.
	outer := float32(math.Hypot(float64(w), float64(h)))
	at := func(v float64) float32 { return float32(v) / outer }
	sight := r.sight
	if sight <= 0 {
		sight = 1
	}
	clearAt, solidAt := visionClear*sight, visionSolid*sight
	grad := core.CanvasRadial(x, y, 0, x, y, outer,
		core.GradientStop{Offset: at(clearAt), Color: core.RGBA(fogRGB, 0)},
		core.GradientStop{Offset: at((clearAt + solidAt) / 2), Color: core.RGBA(fogRGB, 0.62)},
		core.GradientStop{Offset: at(solidAt), Color: fogColor},
		core.GradientStop{Offset: 1, Color: fogColor},
	)
	d.Save()
	d.SetFillGradient(grad)
	d.FillRect(0, 0, w, h)
	d.Restore()
}

// sync copies a step's snapshot and events for drawing: where everything is this step, where it was last, and the effects the step caused.
func (r *arenaRenderer) sync(s Snapshot, events []Event, dt float64) {
	if r.ground == nil || s.Arena != r.arena || s.Fog != r.fogMargin {
		r.arena, r.fogMargin = s.Arena, s.Fog
		r.paintGround()
	}
	if !r.started {
		r.cam.snap(s.Keeper, r.arena)
		r.started = true
	}
	r.cam.follow(s.Keeper, r.arena, dt)
	r.invuln = s.Invulnerable
	keeperDamaged := r.seenHealth && s.Health < r.lastHealth-1e-6
	r.lastHealth = s.Health
	r.maxHealth = s.MaxHealth
	r.seenHealth = true
	clear(r.spare)
	r.trackKeeper(s)
	r.trackEnemies(s)
	r.trackShots(s)
	r.trackEnemyShots(s)
	r.trackPickups(s)
	r.trackChests(s)
	r.before, r.spare = r.spare, r.before
	r.trackEmbers(s)
	r.trackHalo(s)
	r.patches = append(r.patches[:0], s.Patches...)
	r.pulses = append(r.pulses[:0], s.Pulses...)
	r.arcs = append(r.arcs[:0], s.Arcs...)
	r.censers = append(r.censers[:0], s.Censers...)
	r.shadows = append(r.shadows[:0], s.Shadows...)
	r.sight = s.Sight
	r.boss = s.Boss
	r.opening = s.Opening
	r.ring = s.Ring
	r.stunned = r.stunned[:0]
	for i := range s.Enemies {
		if s.Enemies[i].Stunned {
			r.stunned = append(r.stunned, i)
		}
	}
	if s.Phase == PhasePlaying && dt > 0 {
		stepDur := time.Duration(math.Round(dt * float64(time.Second)))
		r.animTime += stepDur
		r.stepKeeperAnimation(stepDur)
	}
	r.stepEffects(events, keeperDamaged, dt)
	r.emitTrails(dt)
	r.holding = false
}

// hold tells the renderer the shell updated without stepping the run, so frames until the next step hold still.
func (r *arenaRenderer) hold() {
	r.holding = true
}

// stepKeeperAnimation switches between idle and walk clips and advances the keeper's animation by dt.
func (r *arenaRenderer) stepKeeperAnimation(dt time.Duration) {
	sa := r.art.sheets.keeper
	if !sa.loaded {
		return
	}
	if len(r.keeperAnim.Frames) == 0 || r.keeperMoving != r.keeperAnimMoving {
		if r.keeperMoving {
			r.keeperAnim = sa.move
		} else {
			r.keeperAnim = sa.idle
		}
		r.keeperAnim.Reset()
		r.keeperAnimMoving = r.keeperMoving
	}
	r.keeperAnim.Update(dt)
}

// trackKeeper keeps the keeper's place this step and the step before, updating movement and horizontal facing.
func (r *arenaRenderer) trackKeeper(s Snapshot) {
	was := r.keeper.pos
	if !r.seenKeeper {
		was, r.seenKeeper = s.Keeper, true
	}
	if s.Phase == PhasePlaying {
		dx := s.Keeper.X - was.X
		dy := s.Keeper.Y - was.Y
		r.keeperMoving = dx*dx+dy*dy > 1e-4
		if dx > 1e-3 {
			r.keeperFacingRight = true
		} else if dx < -1e-3 {
			r.keeperFacingRight = false
		}
	}
	r.keeper = actor{pos: s.Keeper, was: was, radius: s.KeeperRadius, facingRight: r.keeperFacingRight}
}

// trackEnemies copies the snapshot's enemies, reading where each was last step by its id.
func (r *arenaRenderer) trackEnemies(s Snapshot) {
	r.enemies = growActors(r.enemies, len(s.Enemies))
	for i, e := range s.Enemies {
		defaultFacingRight := s.Keeper.X >= e.Pos.X
		a := r.tracked(r.enemies[i], e.ID, enemyKind(e.Kind), e.Radius, e.Pos, defaultFacingRight)
		a.elite = e.Elite
		a.hp = e.Health
		a.maxHP = e.MaxHealth
		r.enemies[i] = a
	}
}

// trackShots copies the snapshot's shots, blending them by id as enemies are.
func (r *arenaRenderer) trackShots(s Snapshot) {
	r.shots = growActors(r.shots, len(s.Shots))
	for i, sh := range s.Shots {
		// A small Starfall Volley shot draws as kind shotSmall.
		kind := 0
		if sh.Small {
			kind = shotSmall
		}
		r.shots[i] = r.tracked(r.shots[i], sh.ID, kind, sh.Radius, sh.Pos, true)
	}
}

// trackEnemyShots copies enemy projectiles, blending them by id.
func (r *arenaRenderer) trackEnemyShots(s Snapshot) {
	r.enemyShots = growActors(r.enemyShots, len(s.EnemyShots))
	for i, sh := range s.EnemyShots {
		r.enemyShots[i] = r.tracked(r.enemyShots[i], sh.ID, int(sh.Kind), sh.Radius, sh.Pos, true)
	}
}

// trackChests copies the chests waiting on the ground, which stay where they fell.
func (r *arenaRenderer) trackChests(s Snapshot) {
	r.chests = growActors(r.chests, len(s.Chests))
	for i, c := range s.Chests {
		r.chests[i] = r.tracked(r.chests[i], c.ID, 0, 0, c.Pos, true)
	}
}

// trackPickups copies elite healing caches, blending them by id.
func (r *arenaRenderer) trackPickups(s Snapshot) {
	r.pickups = growActors(r.pickups, len(s.Pickups))
	for i, p := range s.Pickups {
		r.pickups[i] = r.tracked(r.pickups[i], p.ID, 0, 6, p.Pos, true)
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

// tracked places a into the actor list at id's position, remembering where it was last step for blending, facing and travel angle.
func (r *arenaRenderer) tracked(a actor, id uint64, kind int, radius float64, pos Vec2, defaultFacingRight bool) actor {
	prev, seen := r.before[id]
	was := pos
	facingRight := defaultFacingRight
	var angle float32
	hasAngle := false
	if seen {
		was = prev.pos
		facingRight = prev.facingRight
		angle = prev.angle
		hasAngle = prev.hasAngle
		dx := pos.X - was.X
		dy := pos.Y - was.Y
		if dx > 0.05 {
			facingRight = true
		} else if dx < -0.05 {
			facingRight = false
		}
		if dx*dx+dy*dy > 1e-6 {
			angle = float32(math.Atan2(dy, dx))
			hasAngle = true
		}
	}
	r.spare[id] = prevState{pos: pos, facingRight: facingRight, angle: angle, hasAngle: hasAngle}
	a.id, a.kind, a.radius, a.pos, a.was, a.facingRight, a.angle = id, kind, radius, pos, was, facingRight, angle
	return a
}

// growActors returns a shortened to n, growing it to hold n when it is smaller.
func growActors(a []actor, n int) []actor {
	if cap(a) < n {
		return make([]actor, n)
	}
	return a[:n]
}

// setShakeEnabled turns camera shake on or off immediately, clearing any active shake when disabled.
func (r *arenaRenderer) setShakeEnabled(enabled bool) {
	r.shakeEnabled = enabled
	if !enabled {
		r.fx.shake = 0
		r.fx.shakeX = 0
		r.fx.shakeY = 0
	}
}

// setQuality applies a resolved quality profile immediately, trimming active particles if over the new cap.
func (r *arenaRenderer) setQuality(p qualityProfile) {
	if p.maxParticles <= 0 {
		p = defaultQualityProfile()
	}
	r.quality = p
	if len(r.fx.particles) > p.maxParticles {
		r.fx.particles = r.fx.particles[:p.maxParticles]
	}
}

// stepEffects moves the particles, fades the flashes, level-up flourish and damage vignette, shakes the frame, then adds what this step's events caused.
func (r *arenaRenderer) stepEffects(events []Event, keeperDamaged bool, dt float64) {
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
	if fx.damageVignette > 0 && dt > 0 {
		fx.damageVignette = math.Max(0, fx.damageVignette-dt/damageVignetteSeconds)
	}
	if fx.levelFlash > 0 && dt > 0 {
		fx.levelFlash = math.Max(0, fx.levelFlash-dt)
	}
	if r.shakeEnabled && r.quality.shakeScale > 0 {
		fx.shake = math.Max(0, fx.shake-2.5*dt)
		amp := shakePixels * r.quality.shakeScale * fx.shake * fx.shake
		fx.shakeX = amp * math.Sin(fx.time*97)
		fx.shakeY = amp * math.Cos(fx.time*131)
	} else {
		fx.shake = 0
		fx.shakeX = 0
		fx.shakeY = 0
	}

	if keeperDamaged {
		fx.damageVignette = 1.0
		if r.shakeEnabled {
			fx.shake = math.Min(1, fx.shake+0.45)
		}
	}

	hitSparks := 0
	for _, e := range events {
		switch e.Kind {
		case EventHit:
			fx.flash[e.ID] = flashSeconds
			if r.quality.hitSparks && hitSparks < 6 {
				r.burst(e.Pos, particleSpark, 1, 36, sparkSeconds*0.7)
				hitSparks++
			}
		case EventKill:
			r.burst(e.Pos, particlePuff, 4, 22, puffSeconds)
			if r.shakeEnabled {
				fx.shake = math.Min(1, fx.shake+0.12)
			}
		case EventEmber:
			r.burst(e.Pos, particleSpark, 2, 30, sparkSeconds)
		case EventLevelUp:
			fx.levelFlash = levelFlashSeconds
			r.burst(r.keeper.pos, particleSpark, 16, 62, sparkSeconds*1.4)
			if r.shakeEnabled {
				fx.shake = math.Min(1, fx.shake+0.25)
			}
		case EventBossSpawn:
			r.burst(e.Pos, particlePuff, 14, 48, puffSeconds*1.3)
			r.burst(e.Pos, particleSpark, 10, 56, sparkSeconds*1.4)
			if r.shakeEnabled {
				fx.shake = 0.9
			}
		case EventBossPhase:
			r.burst(e.Pos, particleSpark, 12, 60, sparkSeconds*1.3)
			if r.shakeEnabled {
				fx.shake = math.Min(1, fx.shake+0.65)
			}
		case EventVictory:
			r.burst(e.Pos, particleSpark, 24, 70, sparkSeconds*1.6)
			if r.shakeEnabled {
				fx.shake = 0.85
			}
		case EventDefeat:
			fx.damageVignette = 1.0
			if r.shakeEnabled {
				fx.shake = 1
			}
		case EventChest:
			r.burst(e.Pos, particleSpark, 14, 54, sparkSeconds*1.4)
			if r.shakeEnabled {
				fx.shake = math.Min(1, fx.shake+0.3)
			}
		case EventEvolve:
			fx.levelFlash = levelFlashSeconds
			r.burst(e.Pos, particleSpark, 28, 80, sparkSeconds*1.8)
			r.burst(r.keeper.pos, particleSpark, 16, 62, sparkSeconds*1.6)
			if r.shakeEnabled {
				fx.shake = math.Min(1, fx.shake+0.6)
			}
		}
	}
}

// emitTrails now and then leaves a trail behind an ember on the ground, purely for the eye: the cosmetic stream decides, never the run.
func (r *arenaRenderer) emitTrails(dt float64) {
	if dt <= 0 || !r.quality.emberTrails {
		return
	}
	capParticles := r.quality.maxParticles
	for i := range r.embers {
		if len(r.fx.particles) >= capParticles {
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

// burst scatters n particles of kind from pos, scaled by the active quality preset, each in a direction the cosmetic stream picks.
func (r *arenaRenderer) burst(pos Vec2, kind, n int, speed, life float64) {
	scaled := max(1, int(math.Round(float64(n)*r.quality.particleScale)))
	capParticles := r.quality.maxParticles
	for range scaled {
		if len(r.fx.particles) >= capParticles {
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

// draw paints one frame of the run at alpha, the loop's distance between the last step and the next. Every frame blends while the run steps, however many frames fall between two steps; a held run, such as a paused one, shows the last step's places instead of drifting between them.
func (r *arenaRenderer) draw(d *core.Drawing, alpha float64, view core.Size) {
	if r.holding {
		alpha = 1
	}
	r.alpha = alpha
	r.cam.resize(float64(view.Width), float64(view.Height), r.arena)
	vw, vh := r.cam.size()
	w, h := float32(vw), float32(vh)
	d.SetImageSmoothing(false)
	d.SetFillColor(arenaColor).FillRect(0, 0, w, h)
	if r.ground == nil {
		return
	}
	ox, oy := r.origin()
	d.SetImageSmoothing(r.art.hasPaintedGround())
	d.DrawImageFrom(r.groundSource, ox, oy, w, h, 0, 0, w, h)
	r.used, r.flashing = 0, 0
	r.patchSprites = r.patchSprites[:0]
	r.pulseSprites = r.pulseSprites[:0]
	r.drawPatches(d, ox, oy)
	d.SetImageSmoothing(false)
	r.drawShadows(d, ox, oy)
	r.drawEliteAuras(d, ox, oy)
	r.drawBossTelegraph(d, ox, oy)

	r.sheetSprites = r.sheetSprites[:0]
	r.pixelSprites = r.pixelSprites[:0]
	r.addEmbers(ox, oy)
	r.addPickups(ox, oy)
	r.addChests(ox, oy)
	r.addEnemies(ox, oy)
	r.addShots(ox, oy)
	r.addEnemyShots(ox, oy)
	r.addHalo(ox, oy)
	r.addKeeper(ox, oy)
	r.addParticles(ox, oy)
	if r.art.sheetAtlas != nil && len(r.sheetSprites) > 0 {
		d.SetImageSmoothing(true)
		r.sheetBatch.Atlas = r.art.sheetAtlas
		r.sheetBatch.Sprites = r.sheetSprites
		r.sheetBatch.Draw(d)
	}
	if len(r.pixelSprites) > 0 {
		d.SetImageSmoothing(false)
		r.batch.Atlas = r.art.atlas
		r.batch.Sprites = r.pixelSprites
		r.batch.Draw(d)
	}
	r.drawRing(d, ox, oy)
	r.drawPulses(d, ox, oy)
	r.drawSparks(d, ox, oy)
	d.SetImageSmoothing(false)
	r.drawStuns(d, ox, oy)
	r.drawEliteBars(d, ox, oy)
	r.drawLevelFlourish(d, ox, oy)
	r.drawChestFlourish(d, ox, oy)

	r.drawLight(d, w, h)
	r.drawFog(d, ox, oy, w, h)
	r.drawDamageVignette(d, w, h)
}

// spriteFlipX reports whether a sprite facing right (facingRight=true) or left (facingRight=false)
// needs horizontal mirroring given whether its source sheet defaults to facing left.
func spriteFlipX(facingRight, defaultFacesLeft bool) bool {
	if defaultFacesLeft {
		return facingRight
	}
	return !facingRight
}

// phaseOffset derives a deterministic animation start offset from an actor id so crowds do not animate in lockstep.
func phaseOffset(id uint64, round time.Duration) time.Duration {
	if round <= 0 {
		return 0
	}
	h := id * 0x9e3779b97f4a7c15
	h ^= h >> 30
	return time.Duration(h % uint64(round))
}

// sampleClip returns anim's frame at animTime plus a deterministic per-id phase offset, using a stack value for zero allocations.
func sampleClip(anim sprite.Animation, round time.Duration, id uint64, animTime time.Duration) sprite.Frame {
	if t := animTime + phaseOffset(id, round); t > 0 {
		anim.Update(t)
	}
	return anim.Frame()
}

// sampleAnimation returns sa's normal movement frame at animTime plus a deterministic per-id phase offset, using a stack value for zero allocations.
func sampleAnimation(sa sheetActor, id uint64, animTime time.Duration) sprite.Frame {
	return sampleClip(sa.move, sa.round, id, animTime)
}

// origin returns the world point the frame is drawn from: the camera blended by the frame's alpha, with its shake added, kept inside the arena. It is not rounded to whole virtual pixels, which a scaled canvas would show as the world stepping a few screen pixels at a time while the keeper glides.
func (r *arenaRenderer) origin() (float32, float32) {
	cx, cy := r.cam.at(r.alpha)
	w, h := r.cam.size()
	x := clampLength(cx+r.fx.shakeX, 0, r.arena.X-w)
	y := clampLength(cy+r.fx.shakeY, 0, r.arena.Y-h)
	return float32(x), float32(y)
}

// toScreen returns a world position in canvas pixels. It is not rounded: rounding a position and the origin apart would make an actor the camera follows flicker a pixel back and forth.
func toScreen(p Vec2, ox, oy float32) (float32, float32) {
	return float32(p.X) - ox, float32(p.Y) - oy
}

// inView reports whether a canvas position is close enough to the canvas to be worth drawing.
func (r *arenaRenderer) inView(x, y float32) bool {
	vw, vh := r.cam.size()
	w, h := float32(vw), float32(vh)
	return x >= -viewMargin && x <= w+viewMargin && y >= -viewMargin && y <= h+viewMargin
}

// addEmbers adds the embers waiting on the ground.
func (r *arenaRenderer) addEmbers(ox, oy float32) {
	sa := r.art.sheets.ember
	for i := range r.embers {
		if sa.loaded {
			frame := sampleAnimation(sa, uint64(i+1), r.animTime)
			r.addSheet(frame, sa.scale, r.embers[i], ox, oy, 1, 1)
		} else {
			r.add(r.art.frames.ember, r.embers[i], ox, oy, 1, 1)
		}
	}
}

// addPickups adds elite healing caches waiting on the ground.
func (r *arenaRenderer) addPickups(ox, oy float32) {
	sa := r.art.sheets.cache
	for i := range r.pickups {
		var s *sprite.Sprite
		if sa.loaded {
			frame := sampleAnimation(sa, r.pickups[i].id, r.animTime)
			s = r.addSheet(frame, sa.scale, r.pickups[i], ox, oy, 1, 1)
		} else {
			s = r.add(r.art.frames.cache, r.pickups[i], ox, oy, 1, 1)
		}
		if s != nil {
			s.Y += float32(math.Round(math.Sin(r.fx.time*5 + float64(r.pickups[i].id))))
		}
	}
}

// addChests adds the chests waiting on the ground, glowing, and the one being opened, which plays its opening as the hold runs.
func (r *arenaRenderer) addChests(ox, oy float32) {
	sa := r.art.sheets.chest
	for i := range r.chests {
		var s *sprite.Sprite
		if sa.loaded {
			s = r.addSheet(sampleAnimation(sa, r.chests[i].id, r.animTime), sa.scale, r.chests[i], ox, oy, 1, 1)
		} else {
			s = r.add(r.art.frames.chest, r.chests[i], ox, oy, 1, 1)
		}
		if s != nil {
			s.Y += float32(math.Round(math.Sin(r.fx.time*4+float64(r.chests[i].id)) * 0.8))
		}
	}
	if !r.opening.Active {
		return
	}
	at := actor{id: r.opening.ID, pos: r.opening.Pos, was: r.opening.Pos}
	if sa.loaded && len(sa.action.Frames) > 0 {
		frames := sa.action.Frames
		idx := min(int(r.opening.Progress*2*float64(len(frames))), len(frames)-1)
		r.addSheet(frames[idx].Frame, sa.scale, at, ox, oy, 1, 1)
		return
	}
	r.add(r.art.frames.chest, at, ox, oy, float32(max(0.2, 1-r.opening.Progress)), 1)
}

// enemySheet returns the stylized sheetActor and procedural fallback frame for an enemy kind.
func (r *arenaRenderer) enemyVisual(kind int) (sheetActor, sprite.Frame) {
	s, f := &r.art.sheets, &r.art.frames
	switch kind {
	case kindMoth:
		return s.moth, f.moth
	case kindHusk:
		return s.husk, f.husk
	case kindThornling:
		return s.thornling, f.thornling
	case kindWisp:
		return s.wisp, f.wisp
	case kindShade:
		return s.shade, f.shade
	case kindGloomwarden:
		return s.gloomwarden, f.gloomwarden
	case kindHollowKing:
		return s.hollowking, f.hollowking
	default:
		return s.mite, f.mite
	}
}

// addEnemies adds the living enemies, with a struck one flashing toward hitColor, horizontal facing flips,
// action/telegraph clip selection on 2-row bosses/elites, and flying foes bobbing.
func (r *arenaRenderer) addEnemies(ox, oy float32) {
	for i := range r.enemies {
		e := &r.enemies[i]
		sa, fallback := r.enemyVisual(e.kind)
		var s *sprite.Sprite
		if sa.loaded {
			frame := r.enemyFrame(sa, e)
			s = r.addSheet(frame, sa.scale, *e, ox, oy, 1, 2)
			if s != nil {
				s.FlipX = spriteFlipX(e.facingRight, sa.defaultFacesLeft)
			}
		} else {
			s = r.add(fallback, *e, ox, oy, 1, 2)
			if s != nil {
				s.FlipX = !e.facingRight
			}
		}
		if s == nil {
			continue
		}
		if e.kind == kindMoth || e.kind == kindWisp {
			s.Y += float32(math.Round(math.Sin(r.fx.time*6 + float64(e.id))))
		}
		if left, ok := r.fx.flash[e.id]; ok {
			s.Effect = r.flasher(left)
		}
	}
}

// enemyFrame picks the animation frame for e, switching 2-row bosses and elites to their row-1 action clip
// while telegraphing (Hollow King) or winding up a close-range strike (Gloomwarden).
func (r *arenaRenderer) enemyFrame(sa sheetActor, e *actor) sprite.Frame {
	if len(sa.action.Frames) > 0 {
		switch e.kind {
		case kindHollowKing:
			if r.boss.Active && r.boss.Telegraph != "" {
				return sampleClip(sa.action, sa.actionRound, e.id, r.animTime)
			}
		case kindGloomwarden:
			reach := e.radius + r.keeper.radius + 26
			if e.pos.Sub(r.keeper.pos).LenSq() <= reach*reach {
				return sampleClip(sa.action, sa.actionRound, e.id, r.animTime)
			}
		}
	}
	return sampleAnimation(sa, e.id, r.animTime)
}

// shotSmall is the drawn kind of a small Starfall Volley shot.
const shotSmall = 1

// addShots adds the Ember Bolts in flight above the enemies they are about to hit, rotated along their travel direction, and Starfall Volley's small shots as falling stars.
func (r *arenaRenderer) addShots(ox, oy float32) {
	for i := range r.shots {
		sa, fallback := r.art.sheets.bolt, r.art.frames.bolt
		if r.shots[i].kind == shotSmall {
			sa, fallback = r.art.sheets.starfall, r.art.frames.spark
		}
		var s *sprite.Sprite
		if sa.loaded {
			frame := sampleAnimation(sa, r.shots[i].id, r.animTime)
			s = r.addSheet(frame, sa.scale, r.shots[i], ox, oy, 1, 3)
		} else {
			s = r.add(fallback, r.shots[i], ox, oy, 1, 3)
		}
		if s != nil {
			s.Rotation = r.shots[i].angle
		}
	}
}

// addEnemyShots adds Thornling, Wisp and Hollow King projectiles in flight.
func (r *arenaRenderer) addEnemyShots(ox, oy float32) {
	f := r.art.frames
	for i := range r.enemyShots {
		sa, frame := r.art.sheets.thorn, f.thorn
		if r.enemyShots[i].kind == int(enemyShotBone) {
			sa, frame = r.art.sheets.bone, f.bone
		}
		var s *sprite.Sprite
		if sa.loaded {
			s = r.addSheet(sampleAnimation(sa, r.enemyShots[i].id, r.animTime), sa.scale, r.enemyShots[i], ox, oy, 1, 3)
		} else {
			s = r.add(frame, r.enemyShots[i], ox, oy, 1, 3)
		}
		if s != nil {
			s.Rotation = r.enemyShots[i].angle
		}
	}
}

// drawEliteAuras paints a pulsing crimson ground ring under each living mini-boss/elite so it stands out in a crowd.
func (r *arenaRenderer) drawEliteAuras(d *core.Drawing, ox, oy float32) {
	pulse := float32(0.75 + 0.25*math.Sin(r.fx.time*5))
	for i := range r.enemies {
		e := &r.enemies[i]
		if !e.elite {
			continue
		}
		p := Vec2{e.was.X + (e.pos.X-e.was.X)*r.alpha, e.was.Y + (e.pos.Y-e.was.Y)*r.alpha}
		x, y := toScreen(p, ox, oy)
		if !r.inView(x, y) {
			continue
		}
		rad := float32(e.radius + 4)
		d.SetFillColor(core.RGBA(0xa3341a, 0.14*pulse)).
			BeginPath().Arc(x, y, rad, 0, 2*math.Pi, false).Fill()
		d.SetStrokeColor(core.RGBA(0xe65239, 0.55*pulse)).
			SetLineWidth(1.5).
			BeginPath().Arc(x, y, rad, 0, 2*math.Pi, false).Stroke()
	}
}

// drawEliteBars paints a compact floating health bar above each living mini-boss/elite on the canvas.
func (r *arenaRenderer) drawEliteBars(d *core.Drawing, ox, oy float32) {
	const (
		barW = float32(30)
		barH = float32(3)
	)
	for i := range r.enemies {
		e := &r.enemies[i]
		if !e.elite || e.maxHP <= 0 {
			continue
		}
		p := Vec2{e.was.X + (e.pos.X-e.was.X)*r.alpha, e.was.Y + (e.pos.Y-e.was.Y)*r.alpha}
		x, y := toScreen(p, ox, oy)
		if !r.inView(x, y) {
			continue
		}
		frac := float32(min(max(e.hp/e.maxHP, 0), 1))
		bx := x - barW/2
		by := y - float32(e.radius) - 9
		d.SetFillColor(core.RGBA(0x0b0a0e, 0.85)).FillRect(bx-1, by-1, barW+2, barH+2)
		if frac > 0 {
			d.SetFillColor(core.RGB(0xd93829)).FillRect(bx, by, barW*frac, barH)
		}
	}
}

// drawBossTelegraph paints the Hollow King's wind-up warning on the ground before sprites draw.
func (r *arenaRenderer) drawBossTelegraph(d *core.Drawing, ox, oy float32) {
	b := r.boss
	if !b.Active || b.Telegraph == "" {
		return
	}
	x, y := toScreen(b.Pos, ox, oy)
	p := float32(min(max(b.TelegraphProgress, 0), 1))
	pulse := float32(0.5 + 0.45*p)
	flash := float32(0.85 + 0.15*math.Sin(r.fx.time*26))
	switch b.Telegraph {
	case "charge":
		end := b.Pos.Add(b.TelegraphDir.Scale(b.TelegraphLength))
		ex, ey := toScreen(end, ox, oy)
		d.SetStrokeColor(core.RGBA(0xa3341a, (0.35+0.38*p)*flash)).
			SetLineWidth(float32(b.Radius*1.5)).
			BeginPath().MoveTo(x, y).LineTo(ex, ey).Stroke()
		d.SetStrokeColor(core.RGBA(0xffd166, pulse*flash)).
			SetLineWidth(2.5).
			BeginPath().MoveTo(x, y).LineTo(ex, ey).Stroke()
		d.SetStrokeColor(core.RGBA(0xfff1d6, 0.7*p)).
			SetLineWidth(1.5).
			BeginPath().Arc(x, y, float32(b.Radius)*(1.15+0.25*p), 0, 2*math.Pi, false).Stroke()
	case "ring":
		outer := float32(b.TelegraphLength)
		rad := outer * (0.35 + 0.65*p)
		d.SetFillColor(core.RGBA(0xa3341a, 0.16*pulse)).
			BeginPath().Arc(x, y, rad, 0, 2*math.Pi, false).Fill()
		d.SetStrokeColor(core.RGBA(0xa3341a, 0.45*pulse)).
			SetLineWidth(1.5).
			BeginPath().Arc(x, y, outer, 0, 2*math.Pi, false).Stroke()
		d.SetStrokeColor(core.RGBA(0xfff1d6, pulse*flash)).
			SetLineWidth(2.2).
			BeginPath().Arc(x, y, rad, 0, 2*math.Pi, false).Stroke()
	case "volley":
		// A fan opening toward the keeper, as wide as the bones will fly.
		rad := float32(b.TelegraphLength) * (0.45 + 0.55*p)
		aim := math.Atan2(b.TelegraphDir.Y, b.TelegraphDir.X)
		half := b.TelegraphSpread / 2
		from, to := float32(aim-half), float32(aim+half)
		d.SetFillColor(core.RGBA(0xa3341a, 0.22*pulse)).
			BeginPath().MoveTo(x, y).Arc(x, y, rad, from, to, false).ClosePath().Fill()
		d.SetStrokeColor(core.RGBA(0xfff1d6, pulse*flash)).
			SetLineWidth(2.2).
			BeginPath().MoveTo(x, y).Arc(x, y, rad, from, to, false).ClosePath().Stroke()
	case "summon":
		rad := float32(b.TelegraphLength)
		d.SetFillColor(core.RGBA(0x5b4a7f, 0.24*pulse)).
			BeginPath().Arc(x, y, rad, 0, 2*math.Pi, false).Fill()
		d.SetStrokeColor(core.RGBA(0xffb347, pulse*flash)).
			SetLineWidth(2.2).
			BeginPath().Arc(x, y, rad, 0, 2*math.Pi, false).Stroke()
		d.SetStrokeColor(core.RGBA(0xffd166, 0.65*p)).
			SetLineWidth(1.5).
			BeginPath().Arc(x, y, rad*(0.3+0.7*p), 0, 2*math.Pi, false).Stroke()
	}
}

// addHalo adds Lantern Halo's orbiting embers around the keeper.
func (r *arenaRenderer) addHalo(ox, oy float32) {
	sa := r.art.sheets.halo
	for i := range r.halo {
		if sa.loaded {
			frame := sampleAnimation(sa, uint64(i+1), r.animTime)
			r.addSheet(frame, sa.scale, r.halo[i], ox, oy, 1, 3)
		} else {
			r.add(r.art.frames.halo, r.halo[i], ox, oy, 1, 3)
		}
	}
}

// drawShadows paints the shadow zones dead elites leave on a Long Night tier: a dark pool with a violet rim that fades as it runs out.
func (r *arenaRenderer) drawShadows(d *core.Drawing, ox, oy float32) {
	cw, ch := r.cam.size()
	vw, vh := float32(cw), float32(ch)
	for i := range r.shadows {
		s := &r.shadows[i]
		x, y := toScreen(s.Pos, ox, oy)
		rad := float32(s.Radius)
		if x+rad < 0 || x-rad > vw || y+rad < 0 || y-rad > vh {
			continue
		}
		fade := float32(math.Min(1, s.Life/0.8))
		swell := float32(1 + 0.04*math.Sin(r.fx.time*3+float64(s.ID)))
		d.SetFillColor(core.RGBA(0x120c1f, 0.62*fade)).
			BeginPath().Arc(x, y, rad*swell, 0, 2*math.Pi, false).Fill()
		d.SetFillColor(core.RGBA(0x2b1d45, 0.5*fade)).
			BeginPath().Arc(x, y, rad*0.62*swell, 0, 2*math.Pi, false).Fill()
		d.SetStrokeColor(core.RGBA(0x8a6cc4, 0.55*fade)).
			SetLineWidth(1.5).
			BeginPath().Arc(x, y, rad*swell, 0, 2*math.Pi, false).Stroke()
	}
}

// drawPatches paints Cinder Trail's burning ground patches over the forest floor before sprites draw,
// using the animated cinder_trail sprite sheet when loaded and falling back to procedural arcs otherwise.
func (r *arenaRenderer) drawPatches(d *core.Drawing, ox, oy float32) {
	cw, ch := r.cam.size()
	vw, vh := float32(cw), float32(ch)
	if len(r.patches) == 0 {
		return
	}
	sa := r.art.sheets.cinderTrail
	if sa.loaded && r.art.sheetAtlas != nil {
		r.patchSprites = r.patchSprites[:0]
		for i := range r.patches {
			p := &r.patches[i]
			x, y := float32(p.Pos.X)-ox, float32(p.Pos.Y)-oy
			rad := float32(p.Radius)
			if x+rad < 0 || x-rad > vw || y+rad < 0 || y-rad > vh {
				continue
			}
			sheet := sa
			if p.Wild && r.art.sheets.wildfire.loaded {
				sheet = r.art.sheets.wildfire
			}
			frame := sampleAnimation(sheet, p.ID, r.animTime)
			// The fire fills about the whole cinder frame, but only censerFireShare of a censer burst's.
			fill := float32(2.15)
			if burst := r.art.sheets.censerBurst; p.Censer && burst.loaded && len(burst.move.Frames) == 4 {
				frame = censerFire(burst, p.ID, r.animTime)
				fill = 2 / censerFireShare
			}
			fade := float32(math.Min(1, p.Life/0.5))
			flicker := float32(1 + 0.05*math.Sin(r.fx.time*14+float64(p.ID)))
			scale := (rad * fill * flicker) / max(frame.W, 1)
			s := r.next(frame)
			s.X, s.Y, s.ScaleX, s.ScaleY, s.Alpha, s.Z = x, y, scale, scale, fade, 0
			r.patchSprites = append(r.patchSprites, s)
		}
		if len(r.patchSprites) > 0 {
			d.SetImageSmoothing(true)
			r.patchBatch.Atlas = r.art.sheetAtlas
			r.patchBatch.Sprites = r.patchSprites
			r.patchBatch.Draw(d)
		}
		return
	}

	d.SetImageSmoothing(false)
	for i := range r.patches {
		p := &r.patches[i]
		x, y := toScreen(p.Pos, ox, oy)
		rad := float32(p.Radius)
		if x+rad < 0 || x-rad > vw || y+rad < 0 || y-rad > vh {
			continue
		}
		fade := float32(math.Min(1, p.Life/0.5))
		flicker := float32(1 + 0.06*math.Sin(r.fx.time*14+float64(p.ID)))
		outer, inner := uint32(0xa3341a), uint32(0xff7a1a)
		switch {
		case p.Wild:
			outer, inner = 0xff7a1a, 0xffd166
		case p.Censer:
			outer, inner = 0x5a3a2e, 0xd9541e
		}
		d.SetFillColor(core.RGBA(outer, 0.42*fade)).
			BeginPath().Arc(x, y, rad*flicker, 0, 2*math.Pi, false).Fill()
		d.SetFillColor(core.RGBA(inner, 0.55*fade)).
			BeginPath().Arc(x, y, rad*0.58*flicker, 0, 2*math.Pi, false).Fill()
	}
}

// drawPulses paints Bell Toll's expanding shockwave rings over the sprites,
// mapping pulse progress across bell_toll's 4 frames when loaded and falling back to procedural rings otherwise.
func (r *arenaRenderer) drawPulses(d *core.Drawing, ox, oy float32) {
	cw, ch := r.cam.size()
	vw, vh := float32(cw), float32(ch)
	if len(r.pulses) == 0 {
		return
	}
	sa := r.art.sheets.bellToll
	if sa.loaded && r.art.sheetAtlas != nil && len(sa.move.Frames) > 0 {
		r.pulseSprites = r.pulseSprites[:0]
		for i := range r.pulses {
			p := &r.pulses[i]
			frames := sa.move.Frames
			if dawn := r.art.sheets.dawnBell; p.Dawn && dawn.loaded && len(dawn.move.Frames) > 0 {
				frames = dawn.move.Frames
			}
			x, y := float32(p.Pos.X)-ox, float32(p.Pos.Y)-oy
			t := float32(min(max(1-p.Life/max(p.Max, 0.001), 0), 1))
			rad := float32(p.Radius) * (0.45 + 0.65*t)
			fill := float32(2)
			if p.Censer {
				// A censer's burst is at its full size at once and only flares a little.
				frames, fill = r.censerBurstFrames(p.Pyre), 2/censerBurstShare
				rad = float32(p.Radius) * (0.85 + 0.25*t)
			}
			if len(frames) == 0 || x+rad < 0 || x-rad > vw || y+rad < 0 || y-rad > vh {
				continue
			}
			idx := min(int(t*float32(len(frames))), len(frames)-1)
			frame := frames[idx].Frame
			alpha := float32(math.Max(0, 1-float64(t)*0.85))
			scale := (rad * fill) / max(frame.W, 1)
			s := r.next(frame)
			s.X, s.Y, s.ScaleX, s.ScaleY, s.Alpha, s.Z = x, y, scale, scale, alpha, 6
			r.pulseSprites = append(r.pulseSprites, s)
		}
		if len(r.pulseSprites) > 0 {
			d.SetImageSmoothing(true)
			r.pulseBatch.Atlas = r.art.sheetAtlas
			r.pulseBatch.Sprites = r.pulseSprites
			r.pulseBatch.Draw(d)
		}
		return
	}

	d.SetImageSmoothing(false)
	for i := range r.pulses {
		p := &r.pulses[i]
		x, y := toScreen(p.Pos, ox, oy)
		t := float32(1 - p.Life/max(p.Max, 0.001))
		rad := float32(p.Radius) * (0.2 + 0.8*t)
		if x+rad < 0 || x-rad > vw || y+rad < 0 || y-rad > vh {
			continue
		}
		alpha := float32(math.Max(0, 1-float64(t)))
		fill, stroke := uint32(0xffb347), uint32(0xffd166)
		switch {
		case p.Dawn:
			fill, stroke = 0xfff1d6, 0xfff1d6
		case p.Censer:
			rad = float32(p.Radius)
			d.SetFillColor(core.RGBA(0xd9541e, 0.4*alpha)).BeginPath().Arc(x, y, rad, 0, 2*math.Pi, false).Fill()
			fill, stroke = 0xffb347, 0xff7a1a
		}
		d.SetFillColor(core.RGBA(fill, 0.16*alpha)).
			BeginPath().Arc(x, y, rad, 0, 2*math.Pi, false).Fill()
		d.SetStrokeColor(core.RGBA(stroke, 0.85*alpha)).
			SetLineWidth(2).
			BeginPath().Arc(x, y, rad, 0, 2*math.Pi, false).Stroke()
	}
}

// sunRingBand is where sun_ring.png's burning band runs, as a share of the frame's half width.
const sunRingBand = 0.66

// drawRing paints Sun Ring's burning ring around the keeper over the sprites, turning with the halo's angle: the sun_ring sheet when loaded, a glowing band otherwise.
func (r *arenaRenderer) drawRing(d *core.Drawing, ox, oy float32) {
	g := r.ring
	if !g.Active {
		return
	}
	x, y := toScreen(r.blend(r.keeper), ox, oy)
	rad, width := float32(g.Radius), float32(g.Width)
	if sa := r.art.sheets.sunRing; sa.loaded && r.art.sheetAtlas != nil {
		frame := sampleAnimation(sa, 1, r.animTime)
		scale := 2 * rad / sunRingBand / max(frame.W, 1)
		s := r.next(frame)
		s.X, s.Y, s.ScaleX, s.ScaleY, s.Rotation, s.Alpha, s.Z = x, y, scale, scale, float32(g.Angle), 0.95, 6
		r.ringSprites = append(r.ringSprites[:0], s)
		d.SetImageSmoothing(true)
		r.ringBatch.Atlas = r.art.sheetAtlas
		r.ringBatch.Sprites = r.ringSprites
		r.ringBatch.Draw(d)
		return
	}
	flicker := float32(0.85 + 0.15*math.Sin(r.fx.time*11))
	d.SetStrokeColor(core.RGBA(0xff7a1a, 0.45*flicker)).SetLineWidth(width).
		BeginPath().Arc(x, y, rad, 0, 2*math.Pi, false).Stroke()
	d.SetStrokeColor(core.RGBA(0xffd166, 0.85*flicker)).SetLineWidth(max(1.5, width/3)).
		BeginPath().Arc(x, y, rad, 0, 2*math.Pi, false).Stroke()
}

// drawStuns rings each foe Dawn Bell holds still with a small pale-gold halo above its head.
func (r *arenaRenderer) drawStuns(d *core.Drawing, ox, oy float32) {
	for _, i := range r.stunned {
		if i >= len(r.enemies) {
			continue
		}
		e := &r.enemies[i]
		x, y := toScreen(r.blend(*e), ox, oy)
		if !r.inView(x, y) {
			continue
		}
		top := y - float32(e.radius) - 3
		spin := float32(r.fx.time * 6)
		d.SetStrokeColor(core.RGBA(0xfff1d6, 0.85)).SetLineWidth(1.2).
			BeginPath().Ellipse(x, top, 5, 2).Stroke()
		d.SetFillColor(core.RGB(0xffd166)).
			BeginPath().Arc(x+5*float32(math.Cos(float64(spin))), top+2*float32(math.Sin(float64(spin))), 1.2, 0, 2*math.Pi, false).Fill()
	}
}

// drawChestFlourish paints the column of light over an opening chest, brighter and wider when it evolves a weapon, and a ring that spreads from it.
func (r *arenaRenderer) drawChestFlourish(d *core.Drawing, ox, oy float32) {
	o := r.opening
	if !o.Active {
		return
	}
	x, y := toScreen(o.Pos, ox, oy)
	t := float32(o.Progress)
	fade := min(1, (1-t)*4, t*8)
	w := float32(10)
	color := uint32(0xffb347)
	if o.Evolved != "" {
		w, color = 18, 0xffd166
	}
	grad := core.CanvasLinear(x, y-90, x, y,
		core.GradientStop{Offset: 0, Color: core.RGBA(color, 0)},
		core.GradientStop{Offset: 1, Color: core.RGBA(color, 0.55*fade)},
	)
	d.Save()
	d.SetFillGradient(grad)
	d.FillRect(x-w/2, y-90, w, 90)
	d.Restore()
	rad := 6 + 60*t
	d.SetStrokeColor(core.RGBA(0xfff1d6, 0.8*fade)).SetLineWidth(2).
		BeginPath().Arc(x, y, rad, 0, 2*math.Pi, false).Stroke()
}

// addKeeper adds the keeper above its enemies, blinking off while contact leaves it invulnerable.
func (r *arenaRenderer) addKeeper(ox, oy float32) {
	var s *sprite.Sprite
	if sa := r.art.sheets.keeper; sa.loaded {
		frame := r.keeperAnim.Frame()
		if frame.W == 0 {
			frame = sa.idle.Frame()
		}
		s = r.addSheet(frame, sa.scale, r.keeper, ox, oy, 1, 4)
		if s != nil {
			s.FlipX = spriteFlipX(r.keeperFacingRight, sa.defaultFacesLeft)
		}
	} else {
		s = r.add(r.art.frames.keeper, r.keeper, ox, oy, 1, 4)
		if s != nil {
			s.FlipX = !r.keeperFacingRight
		}
	}
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
		if !r.inView(x, y) {
			continue
		}
		s := r.next(frame)
		s.X, s.Y, s.Alpha, s.Z = x, y, left, 5
		r.pixelSprites = append(r.pixelSprites, s)
	}
}

// blend returns where a is this frame: alpha of the way from where it was on the last step to where it is.
func (r *arenaRenderer) blend(a actor) Vec2 {
	return Vec2{a.was.X + (a.pos.X-a.was.X)*r.alpha, a.was.Y + (a.pos.Y-a.was.Y)*r.alpha}
}

// add adds one procedural-atlas sprite of frame at a's blended position with the given alpha and depth, and returns it, or nil when it is off the canvas.
func (r *arenaRenderer) add(frame sprite.Frame, a actor, ox, oy float32, alpha float32, z int) *sprite.Sprite {
	p := Vec2{a.was.X + (a.pos.X-a.was.X)*r.alpha, a.was.Y + (a.pos.Y-a.was.Y)*r.alpha}
	x, y := toScreen(p, ox, oy)
	if !r.inView(x, y) {
		return nil
	}
	s := r.next(frame)
	s.X, s.Y, s.Alpha, s.Z = x, y, alpha, z
	r.pixelSprites = append(r.pixelSprites, s)
	return s
}

// addSheet adds one stylized sprite-sheet frame scaled to its logical world footprint into the smooth sheet batch.
func (r *arenaRenderer) addSheet(frame sprite.Frame, scale float32, a actor, ox, oy float32, alpha float32, z int) *sprite.Sprite {
	p := Vec2{a.was.X + (a.pos.X-a.was.X)*r.alpha, a.was.Y + (a.pos.Y-a.was.Y)*r.alpha}
	x, y := float32(p.X)-ox, float32(p.Y)-oy
	if !r.inView(x, y) {
		return nil
	}
	s := r.next(frame)
	s.X, s.Y, s.ScaleX, s.ScaleY, s.Alpha, s.Z = x, y, scale, scale, alpha, z
	r.sheetSprites = append(r.sheetSprites, s)
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

// drawLevelFlourish paints an expanding golden ring around the keeper on level-up.
func (r *arenaRenderer) drawLevelFlourish(d *core.Drawing, ox, oy float32) {
	if r.fx.levelFlash <= 0 {
		return
	}
	t := float32(1 - r.fx.levelFlash/levelFlashSeconds)
	alpha := float32(math.Max(0, 1-float64(t)))
	x, y := toScreen(r.blend(r.keeper), ox, oy)
	rad := 14 + 54*t
	d.SetFillColor(core.RGBA(0xffd166, 0.18*alpha)).
		BeginPath().Arc(x, y, rad, 0, 2*math.Pi, false).Fill()
	d.SetStrokeColor(core.RGBA(0xfff1d6, 0.9*alpha)).
		SetLineWidth(2.5).
		BeginPath().Arc(x, y, rad, 0, 2*math.Pi, false).Stroke()
}

// drawDamageVignette paints a crimson screen-edge vignette when the keeper takes damage
// (and a subtle low-health pulse when health drops to 25% or below).
func (r *arenaRenderer) drawDamageVignette(d *core.Drawing, w, h float32) {
	alpha := float32(0.46 * r.fx.damageVignette)
	if r.quality.preset.Effects && r.maxHealth > 0 && r.lastHealth > 0 && r.lastHealth/r.maxHealth <= 0.25 {
		pulse := float32(0.10 + 0.08*math.Sin(r.fx.time*6))
		if pulse > alpha {
			alpha = pulse
		}
	}
	if alpha <= 0.005 {
		return
	}
	cx, cy := w/2, h/2
	inner := float32(math.Min(float64(w), float64(h))) * 0.34
	outer := float32(math.Hypot(float64(cx), float64(cy)))
	grad := core.CanvasRadial(cx, cy, inner, cx, cy, outer,
		core.GradientStop{Offset: 0, Color: core.RGBA(0xb81d24, 0)},
		core.GradientStop{Offset: 0.65, Color: core.RGBA(0xb81d24, alpha*0.42)},
		core.GradientStop{Offset: 1, Color: core.RGBA(0x8c1018, alpha)},
	)
	d.Save()
	d.SetFillGradient(grad)
	d.FillRect(0, 0, w, h)
	d.Restore()
}

// drawLight lays the moonlit twilight tint over the frame and adds the glow of shots and embers on top of it.
func (r *arenaRenderer) drawLight(d *core.Drawing, w, h float32) {
	ox, oy := r.origin()
	r.lights = r.lights[:0]

	glowLimit := min(r.quality.maxGlowLights, len(r.glows))
	glow := 0
	// Chests glow first, so a crowd's lights never hide one.
	for i := range r.chests {
		if glow >= glowLimit {
			break
		}
		x, y := toScreen(r.blend(r.chests[i]), ox, oy)
		if !r.inView(x, y) {
			continue
		}
		l := r.glows[glow]
		glow++
		*l = lights2d.Light{X: x, Y: y, Radius: 34, Color: core.RGBA(0xffd166, 0.85), Intensity: 0.6, Falloff: 1.5, Enabled: true}
		r.lights = append(r.lights, l)
	}
	if r.ring.Active && glow < glowLimit {
		x, y := toScreen(r.blend(r.keeper), ox, oy)
		l := r.glows[glow]
		glow++
		*l = lights2d.Light{X: x, Y: y, Radius: float32(r.ring.Radius * 1.5), Color: core.RGBA(0xffb347, 0.7), Intensity: 0.45, Falloff: 1.4, Enabled: true}
		r.lights = append(r.lights, l)
	}
	for i := range r.halo {
		if glow >= glowLimit {
			break
		}
		x, y := toScreen(r.blend(r.halo[i]), ox, oy)
		if !r.inView(x, y) {
			continue
		}
		l := r.glows[glow]
		glow++
		*l = lights2d.Light{X: x, Y: y, Radius: 24, Color: core.RGBA(0xffb347, 0.78), Intensity: 0.5, Falloff: 1.6, Enabled: true}
		r.lights = append(r.lights, l)
	}
	for i := range r.patches {
		if glow >= glowLimit {
			break
		}
		x, y := toScreen(r.patches[i].Pos, ox, oy)
		if !r.inView(x, y) {
			continue
		}
		l := r.glows[glow]
		glow++
		*l = lights2d.Light{X: x, Y: y, Radius: float32(r.patches[i].Radius * 1.45), Color: core.RGBA(0xff7a1a, 0.65), Intensity: 0.42, Falloff: 1.6, Enabled: true}
		r.lights = append(r.lights, l)
	}
	for i := range r.shots {
		if glow >= glowLimit {
			break
		}
		x, y := toScreen(r.blend(r.shots[i]), ox, oy)
		if !r.inView(x, y) {
			continue
		}
		l := r.glows[glow]
		glow++
		*l = lights2d.Light{X: x, Y: y, Radius: 18, Color: core.RGBA(0xffb347, 0.75), Intensity: 0.45, Falloff: 1.6, Enabled: true}
		r.lights = append(r.lights, l)
	}
	for i := range r.pickups {
		if glow >= glowLimit {
			break
		}
		x, y := toScreen(r.blend(r.pickups[i]), ox, oy)
		if !r.inView(x, y) {
			continue
		}
		l := r.glows[glow]
		glow++
		*l = lights2d.Light{X: x, Y: y, Radius: 28, Color: core.RGBA(0x52a868, 0.82), Intensity: 0.55, Falloff: 1.5, Enabled: true}
		r.lights = append(r.lights, l)
	}
	for i := range r.enemyShots {
		if glow >= glowLimit {
			break
		}
		x, y := toScreen(r.blend(r.enemyShots[i]), ox, oy)
		if !r.inView(x, y) {
			continue
		}
		l := r.glows[glow]
		glow++
		c := core.RGBA(0xd6482a, 0.75)
		if r.enemyShots[i].kind == int(enemyShotBone) {
			c = core.RGBA(0xd7dce8, 0.7)
		}
		*l = lights2d.Light{X: x, Y: y, Radius: 16, Color: c, Intensity: 0.42, Falloff: 1.6, Enabled: true}
		r.lights = append(r.lights, l)
	}
	for i := range r.embers {
		if glow >= glowLimit {
			break
		}
		x, y := toScreen(r.blend(r.embers[i]), ox, oy)
		if !r.inView(x, y) {
			continue
		}
		l := r.glows[glow]
		glow++
		*l = lights2d.Light{X: x, Y: y, Radius: 14, Color: core.RGBA(0xffd166, 0.6), Intensity: 0.38, Falloff: 1.7, Enabled: true}
		r.lights = append(r.lights, l)
	}

	r.scene = lights2d.Scene{
		Ambient:        ambientTint,
		Lights:         r.lights,
		LightComposite: core.CompositeScreen,
		DisableShadows: !r.quality.shadows,
	}
	r.scene.Draw(d, core.Bounds{Size: core.Size{Width: core.Points(w), Height: core.Points(h)}})
}

// The shares of an ash_censer_burst frame's width its burst and its fire fill.
const (
	censerBurstShare = 0.9
	censerFireShare  = 0.6
)

// censerBurstFrames returns the frames a censer's burst plays across: a Pyre Censer's flare, else the Ash Censer's burst, or none without the art.
func (r *arenaRenderer) censerBurstFrames(pyre bool) []sprite.AnimationFrame {
	if p := r.art.sheets.pyre; pyre && p.loaded && len(p.move.Frames) == 4 {
		return p.move.Frames[3:]
	}
	if b := r.art.sheets.censerBurst; b.loaded && len(b.move.Frames) == 4 {
		return b.move.Frames[:2]
	}
	return nil
}

// censerFire returns the frame a censer's fire shows at animTime: ash_censer_burst's last two frames in turn, offset by id.
func censerFire(burst sheetActor, id uint64, animTime time.Duration) sprite.Frame {
	each := max(burst.round/4, time.Millisecond)
	k := int((animTime+phaseOffset(id, 2*each))/each) % 2
	return burst.move.Frames[2+k].Frame
}

// censerLift is how high an Ash Censer arcs over the ground at the top of its flight, in world units.
const censerLift = 30

// drawSparks paints Spark Chain's leaps and Ash Censers in flight or pulling over the sprites: the sheets when loaded, strokes and circles otherwise.
func (r *arenaRenderer) drawSparks(d *core.Drawing, ox, oy float32) {
	if len(r.arcs) == 0 && len(r.censers) == 0 {
		return
	}
	sheets := &r.art.sheets
	if r.art.sheetAtlas == nil || !sheets.sparkArc.loaded || !sheets.censer.loaded {
		r.drawSparkStrokes(d, ox, oy)
		return
	}
	r.sparkSprites = r.sparkSprites[:0]
	for i := range r.arcs {
		a := &r.arcs[i]
		sheet := sheets.sparkArc
		if a.Storm && sheets.emberStorm.loaded {
			sheet = sheets.emberStorm
		}
		t := float32(min(max(1-a.Life/max(a.Max, 0.001), 0), 1))
		frames := sheet.move.Frames
		frame := frames[min(int(t*float32(len(frames))), len(frames)-1)].Frame
		fx, fy := toScreen(a.From, ox, oy)
		tx, ty := toScreen(a.To, ox, oy)
		length := float32(math.Hypot(float64(tx-fx), float64(ty-fy)))
		s := r.next(frame)
		s.X, s.Y = (fx+tx)/2, (fy+ty)/2
		s.ScaleX = length / max(frame.W*0.9, 1)
		s.ScaleY = sheet.scale
		s.Rotation = float32(math.Atan2(float64(ty-fy), float64(tx-fx)))
		s.Alpha, s.Z = 1-0.6*t, 7
		r.sparkSprites = append(r.sparkSprites, s)
		if sp := sheets.spark; sp.loaded {
			head := r.next(sampleAnimation(sp, a.ID, r.animTime))
			head.X, head.Y, head.ScaleX, head.ScaleY, head.Alpha, head.Z = tx, ty, sp.scale, sp.scale, 1-t, 8
			r.sparkSprites = append(r.sparkSprites, head)
		}
	}
	for i := range r.censers {
		c := &r.censers[i]
		x, y := toScreen(c.Pos, ox, oy)
		if p := sheets.pyre; c.Pulling > 0 && p.loaded && len(p.move.Frames) == 4 {
			frame := p.move.Frames[min(int(c.Pulling*3), 2)].Frame
			// The swirl is drawn inside the pull's reach and half see-through, so the foes it draws in stay readable.
			scale := float32(1.6*c.Radius) / max(frame.W*censerBurstShare, 1)
			s := r.next(frame)
			s.X, s.Y, s.ScaleX, s.ScaleY, s.Alpha, s.Z = x, y, scale, scale, 0.55, 6
			s.Rotation = -float32(c.Pulling) * 2 * math.Pi
			r.sparkSprites = append(r.sparkSprites, s)
			continue
		}
		if c.Progress >= 1 {
			continue
		}
		lift := censerLift * float32(math.Sin(math.Pi*c.Progress))
		s := r.next(sampleAnimation(sheets.censer, c.ID, r.animTime))
		s.X, s.Y, s.ScaleX, s.ScaleY, s.Alpha, s.Z = x, y-lift, sheets.censer.scale, sheets.censer.scale, 1, 8
		r.sparkSprites = append(r.sparkSprites, s)
	}
	if len(r.sparkSprites) > 0 {
		d.SetImageSmoothing(true)
		r.sparkBatch.Atlas = r.art.sheetAtlas
		r.sparkBatch.Sprites = r.sparkSprites
		r.sparkBatch.Draw(d)
	}
}

// drawSparkStrokes is drawSparks without the sheets: each leap a jagged golden line, each censer a dark bead over its shadow, and each pull a closing ring.
func (r *arenaRenderer) drawSparkStrokes(d *core.Drawing, ox, oy float32) {
	d.SetImageSmoothing(false)
	for i := range r.arcs {
		a := &r.arcs[i]
		alpha := float32(min(max(a.Life/max(a.Max, 0.001), 0), 1))
		fx, fy := toScreen(a.From, ox, oy)
		tx, ty := toScreen(a.To, ox, oy)
		nx, ny := -(ty - fy), tx-fx
		if l := float32(math.Hypot(float64(nx), float64(ny))); l > 0 {
			nx, ny = nx/l, ny/l
		}
		color := uint32(0xffd166)
		if a.Storm {
			color = 0xff7a1a
		}
		d.SetStrokeColor(core.RGBA(color, 0.9*alpha)).SetLineWidth(1.5).BeginPath().MoveTo(fx, fy)
		// Two kinks, bent to either side by the arc's id, make a stroke read as a spark.
		bend := float32(3 + a.ID%4)
		d.LineTo(fx+(tx-fx)/3+nx*bend, fy+(ty-fy)/3+ny*bend)
		d.LineTo(fx+2*(tx-fx)/3-nx*bend, fy+2*(ty-fy)/3-ny*bend)
		d.LineTo(tx, ty).Stroke()
	}
	for i := range r.censers {
		c := &r.censers[i]
		x, y := toScreen(c.Pos, ox, oy)
		if c.Pulling > 0 {
			rad := float32(c.Radius) * (1 - float32(c.Pulling)*0.7)
			d.SetStrokeColor(core.RGBA(0xff7a1a, 0.7)).SetLineWidth(2).BeginPath().Arc(x, y, rad, 0, 2*math.Pi, false).Stroke()
			continue
		}
		if c.Progress >= 1 {
			continue
		}
		lift := censerLift * float32(math.Sin(math.Pi*c.Progress))
		d.SetFillColor(core.RGBA(0x000000, 0.3)).BeginPath().Ellipse(x, y, 4, 2).Fill()
		d.SetFillColor(core.RGB(0x5a3a2e)).BeginPath().Arc(x, y-lift, 4, 0, 2*math.Pi, false).Fill()
		d.SetFillColor(core.RGB(0xff7a1a)).BeginPath().Arc(x, y-lift, 1.8, 0, 2*math.Pi, false).Fill()
	}
}
