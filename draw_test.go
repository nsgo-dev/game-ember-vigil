package main

import (
	"bytes"
	"image"
	"math"
	"testing"

	"github.com/nsgo-dev/nsgo/core"
)

// newTestArena prepares an arena renderer on a run's arena, as the shell does when it starts one.
func newTestArena(t *testing.T, r *Run, seed uint64) *arenaRenderer {
	t.Helper()
	a := newArenaRenderer(newArt())
	a.startRun(seed)
	s := r.Snapshot()
	a.sync(s, nil, 0)
	if a.ground == nil {
		t.Fatal("the arena painted no ground")
	}
	return a
}

// countSprites returns how many sprite frames a drawing holds, whether the batch drew them together or one by one.
func countSprites(ops []core.DrawOp) int {
	n := 0
	for _, op := range ops {
		switch {
		case op.Kind == core.DrawAtlas && op.Atlas != nil:
			n += len(*op.Atlas)
		case op.Kind == core.DrawImage && op.Image != nil && op.Image.Pixels != nil && !isGround(op.Image):
			n++
		}
	}
	return n
}

// isGround reports whether an image is the arena's whole ground rather than a sprite's atlas.
func isGround(img *core.ImageSource) bool {
	if img.Pixels == nil {
		return false
	}
	b := img.Pixels.Bounds()
	return b.Dx() > 1000 || b.Dy() > 1000
}

// TestArtPacksEveryPictureInsideItsAtlas checks that every picture has a frame inside the atlas image and that no two frames touch.
func TestArtPacksEveryPictureInsideItsAtlas(t *testing.T) {
	a := newArt()
	img := a.atlas.Image.Pixels
	if img == nil {
		t.Fatal("the atlas has no image")
	}
	if len(a.atlas.Frames) != len(pictures) {
		t.Fatalf("the atlas holds %d frames for %d pictures", len(a.atlas.Frames), len(pictures))
	}
	for _, p := range pictures {
		f, ok := a.atlas.Frames[p.name]
		if !ok {
			t.Fatalf("the atlas has no frame for %q", p.name)
		}
		if f.W <= 0 || f.H <= 0 || f.Width != f.W || f.Height != f.H {
			t.Fatalf("frame %q = %+v, want the picture's whole size", p.name, f)
		}
		if f.PivotX != f.W/2 || f.PivotY != f.H/2 {
			t.Fatalf("frame %q pivots at (%v, %v), want its centre", p.name, f.PivotX, f.PivotY)
		}
		if f.X < 0 || f.Y < 0 || f.X+f.W > float32(img.Bounds().Dx()) || f.Y+f.H > float32(img.Bounds().Dy()) {
			t.Fatalf("frame %q = %+v lies outside the %v atlas", p.name, f, img.Bounds())
		}
		for _, q := range pictures {
			if q.name == p.name {
				continue
			}
			o := a.atlas.Frames[q.name]
			if f.X < o.X+o.W && o.X < f.X+f.W && f.Y < o.Y+o.H && o.Y < f.Y+f.H {
				t.Fatalf("frames %q and %q overlap", p.name, q.name)
			}
		}
	}
}

// TestAPicturePaintsItsPalette checks that a picture's rows reach the atlas in the palette's colours.
func TestAPicturePaintsItsPalette(t *testing.T) {
	a := newArt()
	f := a.frames.ember
	img := a.atlas.Image.Pixels
	c := img.RGBAAt(int(f.X)+int(f.W)/2, int(f.Y)+int(f.H)/2)
	want := palette[palFlame]
	if r, g, b, _ := want.Components(); c.R != uint8(r*255+0.5) || c.G != uint8(g*255+0.5) || c.B != uint8(b*255+0.5) {
		t.Fatalf("the ember's middle pixel is %v, want the flame colour %v", c, want)
	}
	if _, _, _, alpha := img.RGBAAt(int(f.X), int(f.Y)).RGBA(); alpha != 0 {
		t.Fatalf("the ember's corner pixel is opaque, want it clear")
	}
}

// TestGroundIsStablePerSeed checks that one seed paints one ground and another seed paints another.
func TestGroundIsStablePerSeed(t *testing.T) {
	a := newArt()
	first := a.newGround(99, 640, 400)
	again := a.newGround(99, 640, 400)
	other := a.newGround(100, 640, 400)
	if first.Bounds() != image.Rect(0, 0, 640, 400) {
		t.Fatalf("the ground is %v, want 640x400", first.Bounds())
	}
	if !bytes.Equal(first.Pix, again.Pix) {
		t.Fatal("one seed painted two different grounds")
	}
	if bytes.Equal(first.Pix, other.Pix) {
		t.Fatal("two seeds painted the same ground")
	}
}

// TestCameraFollowsAndStaysInsideTheArena checks that the camera eases after the keeper and never shows outside the arena, even when the keeper walks into a corner.
func TestCameraFollowsAndStaysInsideTheArena(t *testing.T) {
	arena := Vec2{X: 1920, Y: 1280}
	var c camera
	c.snap(Vec2{X: 960, Y: 640}, arena)
	if c.x != 960-viewWidth/2 || c.y != 640-viewHeight/2 {
		t.Fatalf("the camera starts at (%v, %v), want the keeper centered", c.x, c.y)
	}

	// Following a keeper a hundred world units to the right eases the camera's centre toward it, not at once.
	before, want := c.x, c.x+100
	c.follow(Vec2{X: 960 + 100, Y: 640}, arena, stepSeconds)
	if c.x <= before || c.x >= want {
		t.Fatalf("the camera moved to %v after one step, want it eased past %v but short of %v", c.x, before, want)
	}
	for range 600 {
		c.follow(Vec2{X: 960 + 100, Y: 640}, arena, stepSeconds)
	}
	if c.x < want-1 {
		t.Fatalf("the camera stopped at %v, want it near %v", c.x, want)
	}

	for range 300 {
		c.follow(Vec2{X: 0, Y: 0}, arena, stepSeconds)
	}
	if c.x != 0 || c.y != 0 {
		t.Fatalf("the camera reached (%v, %v) in the top-left corner, want (0, 0)", c.x, c.y)
	}
	for range 300 {
		c.follow(arena, arena, stepSeconds)
	}
	if c.x != arena.X-viewWidth || c.y != arena.Y-viewHeight {
		t.Fatalf("the camera reached (%v, %v) in the bottom-right corner, want (%v, %v)", c.x, c.y, arena.X-viewWidth, arena.Y-viewHeight)
	}

	for i := range 500 {
		target := Vec2{X: float64(i%97) * 21, Y: float64(i%53) * 25}
		c.follow(target, arena, stepSeconds)
		if c.x < 0 || c.x > arena.X-viewWidth || c.y < 0 || c.y > arena.Y-viewHeight {
			t.Fatalf("the camera left the arena at (%v, %v) chasing %v", c.x, c.y, target)
		}
	}

	small := Vec2{X: 100, Y: 80}
	c.snap(small, small)
	if c.x != 0 || c.y != 0 {
		t.Fatalf("the camera sits at (%v, %v) over an arena smaller than the view, want (0, 0)", c.x, c.y)
	}
}

// TestShakeKeepsTheViewInsideTheArena checks that the screen's shake never shows outside the arena either.
func TestShakeKeepsTheViewInsideTheArena(t *testing.T) {
	a := newArenaRenderer(newArt())
	a.arena = Vec2{X: 1920, Y: 1280}
	a.fx.shake = 1
	a.fx.shakeX, a.fx.shakeY = -6, -6
	a.cam.snap(Vec2{X: 0, Y: 0}, a.arena)
	if x, y := a.origin(); x != 0 || y != 0 {
		t.Fatalf("the shaken view starts at (%v, %v), want the arena's corner", x, y)
	}
	// At the far edge the shake may pull the view in, never push it past the arena.
	a.cam.snap(a.arena, a.arena)
	right := float32(a.arena.X - viewWidth)
	if x, _ := a.origin(); x < 0 || x > right {
		t.Fatalf("the shaken view starts at x %v, outside the arena's 0 to %v", x, right)
	}
	a.fx.shakeX, a.fx.shakeY = 6, 6
	if x, _ := a.origin(); x != right {
		t.Fatalf("the shaken view starts at x %v, want it clamped to %v", x, right)
	}
}

// TestFrameDrawsTheRunAndItsGround checks that a frame of a run draws the ground crop and a sprite for every actor in sight.
func TestFrameDrawsTheRunAndItsGround(t *testing.T) {
	r := quietRun(t)
	r.addEnemy(0, r.keeper.pos.Add(Vec2{X: 40}), 1, 1)
	r.addEnemy(1, r.keeper.pos.Add(Vec2{X: -40}), 1, 1)
	r.addEnemy(2, r.keeper.pos.Add(Vec2{Y: 40}), 1, 1)
	a := newTestArena(t, r, 7)

	stepOnce(r, Controls{})
	keeper, enemies := r.keeper.pos, len(r.enemies)
	a.sync(r.Snapshot(), r.Events(), stepSeconds)
	d := &core.Drawing{}
	a.draw(d, 0.5)

	ground := 0
	for _, op := range d.Ops {
		if op.Kind == core.DrawImage && op.Image != nil && isGround(op.Image) {
			ground++
			if op.X2 != viewWidth || op.Y2 != viewHeight || op.W != viewWidth || op.H != viewHeight {
				t.Fatalf("the ground crop draws %vx%v into %vx%v, want the whole view", op.X2, op.Y2, op.W, op.H)
			}
		}
	}
	if ground != 1 {
		t.Fatalf("the frame drew %d ground crops, want 1", ground)
	}
	if got := countSprites(d.Ops); got < 4 {
		t.Fatalf("the frame drew %d sprites, want the keeper and its three enemies", got)
	}
	if r.keeper.pos != keeper || len(r.enemies) != enemies {
		t.Fatal("drawing changed the run")
	}

	// The frame also lays the darkness and the lantern's glow over the world.
	lights := 0
	for _, op := range d.Ops {
		if op.Kind == core.DrawFillGradient {
			lights++
		}
	}
	if lights == 0 {
		t.Fatal("the frame drew no light gradient over the darkness")
	}
}

// TestFrameCullsWhatIsOffScreen checks that an actor outside the view costs no sprite.
func TestFrameCullsWhatIsOffScreen(t *testing.T) {
	r := quietRun(t)
	r.addEnemy(0, r.keeper.pos.Add(Vec2{X: 600}), 1, 1)
	a := newTestArena(t, r, 1)
	stepOnce(r, Controls{})
	a.sync(r.Snapshot(), r.Events(), stepSeconds)

	d := &core.Drawing{}
	a.draw(d, 0)
	// One sprite for the keeper, and none for the enemy 600 world units away.
	if got := countSprites(d.Ops); got != 1 {
		t.Fatalf("the frame drew %d sprites, want only the keeper", got)
	}
}

// TestFramesBetweenStepsBlend checks that a frame between two steps places an actor between where it was and where it is.
func TestFramesBetweenStepsBlend(t *testing.T) {
	a := newArenaRenderer(newArt())
	a.alpha = 0.5
	s := a.add(a.art.frames.ember, actor{pos: Vec2{X: 100, Y: 50}, was: Vec2{X: 90, Y: 50}}, 0, 0, 1, 1)
	if s == nil {
		t.Fatal("an ember in the middle of the view drew nothing")
	}
	if s.X != 95 || s.Y != 50 {
		t.Fatalf("the ember drew at (%v, %v), want (95, 50)", s.X, s.Y)
	}
}

// TestEffectsStayWithinTheirCaps checks that a crowd of kills cannot grow the particles or the flashes past what a frame can draw.
func TestEffectsStayWithinTheirCaps(t *testing.T) {
	a := newArenaRenderer(newArt())
	a.arena = Vec2{X: 1920, Y: 1280}
	events := make([]Event, 0, 800)
	for i := range 400 {
		events = append(events, Event{Kind: EventKill, ID: uint64(i), Pos: Vec2{X: 100, Y: 100}})
		events = append(events, Event{Kind: EventHit, ID: uint64(i), Pos: Vec2{X: 100, Y: 100}})
	}
	a.sync(Snapshot{Arena: a.arena, Keeper: Vec2{X: 500, Y: 500}}, events, stepSeconds)
	if len(a.fx.particles) > maxParticles {
		t.Fatalf("the effects hold %d particles, above the cap of %d", len(a.fx.particles), maxParticles)
	}
	if len(a.fx.particles) == 0 {
		t.Fatal("four hundred kills left no puff")
	}
	// The flashes only hold the enemies struck this step, and fade.
	for range 20 {
		a.stepEffects(nil, stepSeconds)
	}
	if len(a.fx.flash) != 0 {
		t.Fatalf("%d flashes outlived the flash's length", len(a.fx.flash))
	}
}

// TestFrameDrawsNewWeapons checks that Lantern Halo orbs, Cinder Trail patches and Bell Toll pulses render onto the canvas without mutating the run.
func TestFrameDrawsNewWeapons(t *testing.T) {
	r := quietRun(t)
	for i := range r.weaponLevels {
		r.weaponLevels[i] = 2
	}
	a := newTestArena(t, r, 9)
	stepOnce(r, Controls{})
	a.sync(r.Snapshot(), r.Events(), stepSeconds)

	d := &core.Drawing{}
	a.draw(d, 0.5)

	// Keeper + 3 halo orbs = at least 4 sprites.
	if got := countSprites(d.Ops); got < 4 {
		t.Fatalf("frame drew %d sprites, want keeper plus Lantern Halo orbs", got)
	}
	hasStroke := false
	hasArc := false
	for _, op := range d.Ops {
		if op.Kind == core.DrawStroke {
			hasStroke = true
		}
		if op.Kind == core.DrawArc {
			hasArc = true
		}
	}
	if !hasArc {
		t.Fatal("frame drew no arcs for Cinder Trail patches or Bell Toll pulses")
	}
	if !hasStroke {
		t.Fatal("frame drew no stroke ring for Bell Toll pulse")
	}
}

// TestFrameDrawsNewEnemiesPickupsAndBossTelegraph checks that late enemies, enemy shots, heal caches and boss telegraphs render cleanly.
func TestFrameDrawsNewEnemiesPickupsAndBossTelegraph(t *testing.T) {
	r := quietRun(t)
	r.weaponLevels[r.data.starterWeapon] = 0
	for i := range r.data.enemies.Enemies {
		r.addEnemy(i, r.keeper.pos.Add(Vec2{X: float64(30 + i*14), Y: 20}), 1, 1)
	}
	r.addEnemyShot(r.keeper.pos.Add(Vec2{X: -40}), Vec2{X: 10}, 5, 4, 2, enemyShotThorn)
	r.addEnemyShot(r.keeper.pos.Add(Vec2{Y: -40}), Vec2{Y: 10}, 5, 5, 2, enemyShotBone)
	r.pickups = append(r.pickups, pickup{id: r.takeID(), pos: r.keeper.pos.Add(Vec2{X: -24, Y: 24}), healFraction: 0.3})
	a := newTestArena(t, r, 13)

	stepOnce(r, Controls{})
	a.sync(r.Snapshot(), r.Events(), stepSeconds)
	d := &core.Drawing{}
	a.draw(d, 0.5)
	if got := countSprites(d.Ops); got < 11 {
		t.Fatalf("frame drew %d sprites, want keeper + 7 enemies + 2 enemy shots + 1 heal cache", got)
	}

	// Render a Hollow King charge telegraph.
	r.spawnBoss()
	r.boss.state = bossChargeWindup
	r.boss.timer = 0.4
	r.boss.timerMax = 0.8
	r.boss.chargeDir = Vec2{X: 1}
	a.sync(r.Snapshot(), r.Events(), stepSeconds)
	d.Ops = d.Ops[:0]
	a.draw(d, 0.5)
	hasLine := false
	for _, op := range d.Ops {
		if op.Kind == core.DrawLineTo {
			hasLine = true
			break
		}
	}
	if !hasLine {
		t.Fatal("boss charge telegraph drew no line")
	}
}

// BenchmarkArenaFrame measures a whole frame of a crowded arena: step two hundred enemies that surround the keeper, copy them for drawing, and draw them with their effects.
func BenchmarkArenaFrame(b *testing.B) {
	r := NewRun(11)
	r.director.nextSpawn = math.Inf(1)
	for i := range maxEnemies / 2 {
		angle := float64(i) * 2 * math.Pi / float64(maxEnemies/2)
		ring := 40 + float64(i%9)*24
		pos := r.keeper.pos.Add(Vec2{X: math.Cos(angle), Y: math.Sin(angle)}.Scale(ring))
		r.addEnemy(i%3, pos, 20, 1)
	}
	a := newArenaRenderer(newArt())
	a.startRun(11)
	r.Step(stepTime, Controls{})
	a.sync(r.Snapshot(), r.Events(), stepSeconds)
	d := &core.Drawing{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Step(stepTime, Controls{})
		a.sync(r.Snapshot(), r.Events(), stepSeconds)
		d.Ops = d.Ops[:0]
		a.draw(d, 0.5)
	}
}
