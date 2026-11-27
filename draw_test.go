package main

import (
	"bytes"
	"image"
	"math"
	"testing"
	"testing/fstest"

	"github.com/nsgo-dev/game/packages/game/quality"
	"github.com/nsgo-dev/nsgo/core"
	evstrings "me.oneo/embervigil/strings"
)

// runView is the visible world of a 16:9 window: the run's own view.
var runView = core.Size{Width: viewWidth, Height: viewHeight}

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
		case op.Kind == core.DrawImage && op.Image != nil && op.Image.Pixels != nil && !isGround(op.Image) && !coversView(op):
			n++
		}
	}
	return n
}

// isGround reports whether an image is the arena's whole ground rather than a sprite's atlas.
// coversView reports whether a drawn image fills the whole view, as the fog of war's crop does, which no sprite does.
func coversView(op core.DrawOp) bool {
	return op.X == 0 && op.Y == 0 && op.W == viewWidth && op.H == viewHeight
}

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
	a.draw(d, 0.5, runView)

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

	// The frame also lays the fog of war's crop over the world.
	fog := 0
	for _, op := range d.Ops {
		if op.Kind == core.DrawImage && op.Image != nil && op.Image.Pixels == a.fog {
			fog++
		}
	}
	if a.fog == nil || fog != 1 {
		t.Fatalf("the frame drew %d fog crops, want 1", fog)
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
	a.draw(d, 0, runView)
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
		a.stepEffects(nil, false, stepSeconds)
	}
	if len(a.fx.flash) != 0 {
		t.Fatalf("%d flashes outlived the flash's length", len(a.fx.flash))
	}
}

// TestFrameDrawsNewWeapons checks that Lantern Halo orbs, Cinder Trail patches and Bell Toll pulses
// render via their stylized sprite sheets when embedded and fall back cleanly to procedural shapes when absent.
func TestFrameDrawsNewWeapons(t *testing.T) {
	r := quietRun(t)
	for i := range r.weaponLevels {
		r.weaponLevels[i] = 2
	}
	a := newTestArena(t, r, 9)
	stepOnce(r, Controls{})
	a.sync(r.Snapshot(), r.Events(), stepSeconds)

	d := &core.Drawing{}
	a.draw(d, 0.5, runView)

	// Keeper + 3 halo orbs + Cinder Trail patch + Bell Toll pulse sprites.
	if got := countSprites(d.Ops); got < 6 {
		t.Fatalf("frame drew %d sprites, want keeper, Lantern Halo orbs, Cinder Trail patch and Bell Toll pulse", got)
	}
	if len(a.patchSprites) == 0 {
		t.Fatal("Cinder Trail patch did not render via cinder_trail sprite sheet")
	}
	if len(a.pulseSprites) == 0 {
		t.Fatal("Bell Toll pulse did not render via bell_toll sprite sheet")
	}

	// Verify procedural fallback draws arcs and stroke rings when sprite sheets are absent.
	fb := newArenaRenderer(newArtFromFS(fstest.MapFS{}, fstest.MapFS{}))
	fb.startRun(9)
	fb.sync(r.Snapshot(), r.Events(), stepSeconds)
	d.Ops = d.Ops[:0]
	fb.draw(d, 0.5, runView)
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
		t.Fatal("fallback frame drew no arcs for Cinder Trail patches or Bell Toll pulses")
	}
	if !hasStroke {
		t.Fatal("fallback frame drew no stroke ring for Bell Toll pulse")
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
	a.draw(d, 0.5, runView)
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
	a.draw(d, 0.5, runView)
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

// TestEmbeddedSpriteSheetsSliceCleanly checks that all 14 embedded sprite sheets decode, slice into the expected frame counts and cell sizes, pack without overlap, and preserve facing and action-row metadata.
func TestEmbeddedSpriteSheetsSliceCleanly(t *testing.T) {
	a := newArt()
	if a.sheetAtlas == nil || a.sheetAtlas.Image.Pixels == nil {
		t.Fatal("embedded sprite sheets produced no sheetAtlas")
	}
	b := a.sheetAtlas.Image.Pixels.Bounds()

	checks := []struct {
		name             string
		actor            sheetActor
		wantIdleFrames   int
		wantMoveFrames   int
		wantActionFrames int
		wantCellW        float32
		wantCellH        float32
		wantFacesLeft    bool
	}{
		{"keeper", a.sheets.keeper, 4, 4, 0, 64, 64, false},
		{"mite", a.sheets.mite, 4, 4, 0, 64, 64, false},
		{"moth", a.sheets.moth, 4, 4, 0, 64, 64, false},
		{"husk", a.sheets.husk, 4, 4, 0, 64, 64, false},
		{"thornling", a.sheets.thornling, 4, 4, 0, 64, 64, false},
		{"wisp", a.sheets.wisp, 4, 4, 0, 64, 64, false},
		{"shade", a.sheets.shade, 4, 4, 0, 64, 64, false},
		{"gloomwarden", a.sheets.gloomwarden, 4, 4, 4, 96, 96, false},
		{"hollow_king", a.sheets.hollowking, 4, 4, 4, 128, 128, false},
		{"ember", a.sheets.ember, 4, 4, 0, 32, 32, false},
		{"bolt", a.sheets.bolt, 4, 4, 0, 32, 32, false},
		{"lantern_halo", a.sheets.halo, 4, 4, 0, 64, 64, false},
		{"cinder_trail", a.sheets.cinderTrail, 4, 4, 0, 64, 64, false},
		{"bell_toll", a.sheets.bellToll, 4, 4, 0, 64, 64, false},
	}
	for _, tc := range checks {
		if !tc.actor.loaded {
			t.Fatalf("sheet %q was not loaded", tc.name)
		}
		if len(tc.actor.idle.Frames) != tc.wantIdleFrames {
			t.Fatalf("sheet %q idle has %d frames, want %d", tc.name, len(tc.actor.idle.Frames), tc.wantIdleFrames)
		}
		if len(tc.actor.move.Frames) != tc.wantMoveFrames {
			t.Fatalf("sheet %q move has %d frames, want %d", tc.name, len(tc.actor.move.Frames), tc.wantMoveFrames)
		}
		if len(tc.actor.action.Frames) != tc.wantActionFrames {
			t.Fatalf("sheet %q action has %d frames, want %d", tc.name, len(tc.actor.action.Frames), tc.wantActionFrames)
		}
		if tc.actor.defaultFacesLeft != tc.wantFacesLeft {
			t.Fatalf("sheet %q defaultFacesLeft = %v, want %v", tc.name, tc.actor.defaultFacesLeft, tc.wantFacesLeft)
		}
		if tc.actor.scale <= 0 {
			t.Fatalf("sheet %q scale = %v, want positive", tc.name, tc.actor.scale)
		}
		for _, af := range tc.actor.move.Frames {
			f := af.Frame
			if f.W != tc.wantCellW || f.H != tc.wantCellH {
				t.Fatalf("sheet %q frame %q size = %vx%v, want %vx%v", tc.name, f.Name, f.W, f.H, tc.wantCellW, tc.wantCellH)
			}
			if f.PivotX != f.W/2 || f.PivotY != f.H/2 {
				t.Fatalf("sheet %q frame %q pivot = (%v,%v), want center", tc.name, f.Name, f.PivotX, f.PivotY)
			}
		}
		for _, af := range tc.actor.action.Frames {
			f := af.Frame
			if f.W != tc.wantCellW || f.H != tc.wantCellH {
				t.Fatalf("sheet %q action frame %q size = %vx%v, want %vx%v", tc.name, f.Name, f.W, f.H, tc.wantCellW, tc.wantCellH)
			}
		}
	}

	// Verify two-row sheets use distinct rows for idle/walk and move/action clips.
	if a.sheets.keeper.idle.Frames[0].Frame.Name == a.sheets.keeper.move.Frames[0].Frame.Name {
		t.Fatal("keeper idle and walk clips share the same first frame")
	}
	if a.sheets.gloomwarden.move.Frames[0].Frame.Name == a.sheets.gloomwarden.action.Frames[0].Frame.Name {
		t.Fatal("gloomwarden move and action clips share the same first frame")
	}
	if a.sheets.hollowking.move.Frames[0].Frame.Name == a.sheets.hollowking.action.Frames[0].Frame.Name {
		t.Fatal("hollow_king move and action clips share the same first frame")
	}

	// Verify no two packed frames in sheetAtlas overlap and all fit inside the atlas bounds.
	all := make([]string, 0, len(a.sheetAtlas.Frames))
	for name, f := range a.sheetAtlas.Frames {
		if f.X < 0 || f.Y < 0 || f.X+f.W > float32(b.Dx()) || f.Y+f.H > float32(b.Dy()) {
			t.Fatalf("sheet frame %q = %+v lies outside %v", name, f, b)
		}
		all = append(all, name)
	}
	for i := 0; i < len(all); i++ {
		f := a.sheetAtlas.Frames[all[i]]
		for j := i + 1; j < len(all); j++ {
			o := a.sheetAtlas.Frames[all[j]]
			if f.X < o.X+o.W && o.X < f.X+f.W && f.Y < o.Y+o.H && o.Y < f.Y+f.H {
				t.Fatalf("sheet frames %q and %q overlap", all[i], all[j])
			}
		}
	}
}

// TestEmbeddedEnvironmentArtAndFallback verifies all 9 embedded environment assets decode with expected
// dimensions, seeded painted ground rendering stays deterministic and draws smoothly, and missing/corrupt
// environment assets fall back cleanly to procedural ground.
func TestEmbeddedEnvironmentArtAndFallback(t *testing.T) {
	a := newArt()
	if !a.hasPaintedGround() {
		t.Fatal("embedded environment assets did not enable painted ground")
	}
	envChecks := []struct {
		name string
		img  *image.RGBA
		w, h int
	}{
		{"ground_soil.png", a.env.soil, 256, 256},
		{"ground_moss.png", a.env.moss, 256, 256},
		{"grass_tuft.png", a.env.grassTuft, 64, 64},
		{"fern.png", a.env.fern, 64, 64},
		{"moss_stone.png", a.env.mossStone, 64, 64},
		{"mushrooms.png", a.env.mushrooms, 64, 64},
		{"stump.png", a.env.stump, 96, 96},
		{"fallen_branch.png", a.env.fallenBranch, 96, 64},
		{"tree_canopy.png", a.env.treeCanopy, 128, 128},
	}
	for _, tc := range envChecks {
		if tc.img == nil {
			t.Fatalf("environment asset %q was not loaded", tc.name)
		}
		b := tc.img.Bounds()
		if b.Dx() != tc.w || b.Dy() != tc.h {
			t.Fatalf("environment asset %q = %dx%d, want %dx%d", tc.name, b.Dx(), b.Dy(), tc.w, tc.h)
		}
	}

	// Painted ground must be deterministic per seed and distinct across seeds.
	paintedA := a.newGround(77, 640, 400)
	paintedB := a.newGround(77, 640, 400)
	paintedOther := a.newGround(78, 640, 400)
	if !bytes.Equal(paintedA.Pix, paintedB.Pix) {
		t.Fatal("painted ground differed for identical seed")
	}
	if bytes.Equal(paintedA.Pix, paintedOther.Pix) {
		t.Fatal("painted ground matched across different seeds")
	}

	// Missing environment FS falls back cleanly to procedural ground and stays deterministic.
	fallback := newArtFromAllFS(fstest.MapFS{}, fstest.MapFS{}, fstest.MapFS{})
	if fallback.hasPaintedGround() {
		t.Fatal("empty environment FS reported painted ground as loaded")
	}
	procA := fallback.newGround(77, 640, 400)
	procB := fallback.newGround(77, 640, 400)
	if !bytes.Equal(procA.Pix, procB.Pix) {
		t.Fatal("procedural fallback ground differed for identical seed")
	}
	if bytes.Equal(paintedA.Pix, procA.Pix) {
		t.Fatal("painted ground was identical to procedural fallback")
	}

	// Corrupt soil tile falls back cleanly without panicking.
	corrupt := newArtFromAllFS(fstest.MapFS{}, fstest.MapFS{}, fstest.MapFS{
		"assets/environment/ground_soil.png": &fstest.MapFile{Data: []byte("bad png")},
	})
	if corrupt.hasPaintedGround() {
		t.Fatal("corrupt ground_soil.png reported painted ground as loaded")
	}
	if g := corrupt.newGround(5, 320, 200); g.Bounds() != image.Rect(0, 0, 320, 200) {
		t.Fatalf("corrupt fallback ground bounds = %v, want 320x200", g.Bounds())
	}

	// Painted ground crop draws with smooth image sampling.
	r := quietRun(t)
	ar := newTestArena(t, r, 7)
	d := &core.Drawing{}
	ar.draw(d, 1, runView)
	groundSmooth := false
	for _, op := range d.Ops {
		if op.Kind == core.DrawImage && op.Image != nil && isGround(op.Image) && op.Sampling == core.ImageSamplingSmooth {
			groundSmooth = true
			break
		}
	}
	if !groundSmooth {
		t.Fatal("painted ground crop did not draw with smooth image sampling")
	}
}

// TestBossTelegraphAndGloomwardenActionRows verifies Hollow King switches to row 1 while telegraphing
// and Gloomwarden switches to row 1 at close melee range.
func TestBossTelegraphAndGloomwardenActionRows(t *testing.T) {
	a := newArenaRenderer(newArt())
	a.startRun(31)
	arena := Vec2{X: 1920, Y: 1280}
	keeper := Vec2{X: 960, Y: 640}

	// Hollow King hovering without telegraph -> row 0; Gloomwarden far away -> row 0.
	a.sync(Snapshot{
		Phase:        PhasePlaying,
		Arena:        arena,
		Keeper:       keeper,
		KeeperRadius: 8,
		Enemies: []EnemyView{
			{ID: 1, Kind: "Hollow King", Boss: true, Pos: keeper.Add(Vec2{X: 80}), Radius: 22},
			{ID: 2, Kind: "Gloomwarden", Elite: true, Pos: keeper.Add(Vec2{X: -120}), Radius: 15},
		},
		Boss: BossView{Active: true, ID: 1, Name: "Hollow King", Pos: keeper.Add(Vec2{X: 80}), Radius: 22},
	}, nil, stepSeconds)

	d := &core.Drawing{}
	a.draw(d, 1, runView)
	bossFrame, wardenFrame := "", ""
	for _, s := range a.sheetSprites {
		if s.Z != 2 {
			continue
		}
		if len(s.Frame.Name) >= len("hollow_king/") && s.Frame.Name[:len("hollow_king/")] == "hollow_king/" {
			bossFrame = s.Frame.Name
		}
		if len(s.Frame.Name) >= len("gloomwarden/") && s.Frame.Name[:len("gloomwarden/")] == "gloomwarden/" {
			wardenFrame = s.Frame.Name
		}
	}
	if bossFrame == "" || bossFrame[:len("hollow_king/0/")] != "hollow_king/0/" {
		t.Fatalf("Hollow King without telegraph used frame %q, want row 0", bossFrame)
	}
	if wardenFrame == "" || wardenFrame[:len("gloomwarden/0/")] != "gloomwarden/0/" {
		t.Fatalf("distant Gloomwarden used frame %q, want row 0", wardenFrame)
	}

	// Hollow King telegraphing -> row 1; Gloomwarden in close melee range -> row 1.
	a.sync(Snapshot{
		Phase:        PhasePlaying,
		Arena:        arena,
		Keeper:       keeper,
		KeeperRadius: 8,
		Enemies: []EnemyView{
			{ID: 1, Kind: "Hollow King", Boss: true, Pos: keeper.Add(Vec2{X: 80}), Radius: 22},
			{ID: 2, Kind: "Gloomwarden", Elite: true, Pos: keeper.Add(Vec2{X: -30}), Radius: 15},
		},
		Boss: BossView{
			Active: true, ID: 1, Name: "Hollow King", Pos: keeper.Add(Vec2{X: 80}), Radius: 22,
			Telegraph: "ring", TelegraphProgress: 0.5, TelegraphLength: 68,
		},
	}, nil, stepSeconds)
	d.Ops = d.Ops[:0]
	a.draw(d, 1, runView)
	bossFrame, wardenFrame = "", ""
	for _, s := range a.sheetSprites {
		if s.Z != 2 {
			continue
		}
		if len(s.Frame.Name) >= len("hollow_king/") && s.Frame.Name[:len("hollow_king/")] == "hollow_king/" {
			bossFrame = s.Frame.Name
		}
		if len(s.Frame.Name) >= len("gloomwarden/") && s.Frame.Name[:len("gloomwarden/")] == "gloomwarden/" {
			wardenFrame = s.Frame.Name
		}
	}
	if bossFrame == "" || bossFrame[:len("hollow_king/1/")] != "hollow_king/1/" {
		t.Fatalf("Hollow King with telegraph used frame %q, want row 1", bossFrame)
	}
	if wardenFrame == "" || wardenFrame[:len("gloomwarden/1/")] != "gloomwarden/1/" {
		t.Fatalf("close-range Gloomwarden used frame %q, want row 1", wardenFrame)
	}
}

// TestAnimationAdvancesAndFreezesWhenPaused checks idle/walk transitions, animation progression, per-id phase offsets, and freezing when paused.
func TestAnimationAdvancesAndFreezesWhenPaused(t *testing.T) {
	a := newArenaRenderer(newArt())
	a.startRun(42)
	arena := Vec2{X: 1920, Y: 1280}
	pos := Vec2{X: 960, Y: 640}

	// Initial idle step.
	a.sync(Snapshot{Phase: PhasePlaying, Arena: arena, Keeper: pos}, nil, stepSeconds)
	idleStart := a.keeperAnim.Frame().Name
	if a.keeperAnimMoving {
		t.Fatal("stationary keeper switched to walk animation")
	}

	// Advance enough fixed steps for idle to step to the next frame.
	for range 15 {
		a.sync(Snapshot{Phase: PhasePlaying, Arena: arena, Keeper: pos}, nil, stepSeconds)
	}
	if got := a.keeperAnim.Frame().Name; got == idleStart {
		t.Fatalf("keeper idle stayed on %q after 15 steps", got)
	}

	// Start moving: keeper should switch to walk clip and advance frames.
	pos.X += 4
	a.sync(Snapshot{Phase: PhasePlaying, Arena: arena, Keeper: pos}, nil, stepSeconds)
	if !a.keeperAnimMoving {
		t.Fatal("moving keeper did not switch to walk animation")
	}
	walkStart := a.keeperAnim.Frame().Name
	for range 10 {
		pos.X += 2
		a.sync(Snapshot{Phase: PhasePlaying, Arena: arena, Keeper: pos}, nil, stepSeconds)
	}
	if got := a.keeperAnim.Frame().Name; got == walkStart {
		t.Fatalf("keeper walk stayed on %q after 10 moving steps", got)
	}

	// Pause via PhaseLevelUp: animTime and keeper frame must freeze.
	frozenTime := a.animTime
	frozenFrame := a.keeperAnim.Frame().Name
	for range 20 {
		a.sync(Snapshot{Phase: PhaseLevelUp, Arena: arena, Keeper: pos}, nil, stepSeconds)
	}
	if a.animTime != frozenTime || a.keeperAnim.Frame().Name != frozenFrame {
		t.Fatalf("animation advanced while paused: time %v->%v, frame %q->%q", frozenTime, a.animTime, frozenFrame, a.keeperAnim.Frame().Name)
	}

	// Verify per-id phase offsets spread enemies across frames rather than locking them together.
	seen := map[string]bool{}
	for id := uint64(1); id <= 32; id++ {
		fr := sampleAnimation(a.art.sheets.mite, id, a.animTime)
		seen[fr.Name] = true
	}
	if len(seen) < 2 {
		t.Fatalf("32 enemy IDs sampled only %d distinct frame(s), want phase-offset variety", len(seen))
	}
}

// TestSpriteFacingFlipsAndBoltRotation checks horizontal flip rules (including keeper.png facing left by default), enemy flips, bolt rotation, and image smoothing toggles.
func TestSpriteFacingFlipsAndBoltRotation(t *testing.T) {
	a := newArenaRenderer(newArt())
	a.startRun(7)
	arena := Vec2{X: 1920, Y: 1280}
	keeperPos := Vec2{X: 960, Y: 640}
	mitePos := Vec2{X: 1000, Y: 640}
	boltPos := Vec2{X: 980, Y: 640}

	// Step 1: place actors.
	a.sync(Snapshot{
		Phase:   PhasePlaying,
		Arena:   arena,
		Keeper:  keeperPos,
		Enemies: []EnemyView{{ID: 10, Kind: "Ash Mite", Pos: mitePos, Radius: 7}},
		Shots:   []ShotView{{ID: 20, Pos: boltPos, Radius: 4}},
	}, nil, stepSeconds)

	// Step 2: keeper moves right (+X -> FlipX=true since keeper.png defaults left),
	// mite moves left (-X -> FlipX=true since mite.png defaults right),
	// bolt flies straight down (+Y -> Rotation = pi/2).
	keeperPos.X += 3
	mitePos.X -= 2
	boltPos.Y += 5
	a.sync(Snapshot{
		Phase:   PhasePlaying,
		Arena:   arena,
		Keeper:  keeperPos,
		Enemies: []EnemyView{{ID: 10, Kind: "Ash Mite", Pos: mitePos, Radius: 7}},
		Shots:   []ShotView{{ID: 20, Pos: boltPos, Radius: 4}},
	}, nil, stepSeconds)

	d := &core.Drawing{}
	a.draw(d, 1, runView)

	var keeperFlip, miteFlip bool
	var boltRot float32
	foundKeeper, foundMite, foundBolt := false, false, false
	for _, s := range a.sheetSprites {
		switch s.Z {
		case 4:
			keeperFlip, foundKeeper = s.FlipX, true
		case 2:
			miteFlip, foundMite = s.FlipX, true
		case 3:
			boltRot, foundBolt = s.Rotation, true
		}
	}
	if !foundKeeper || !foundMite || !foundBolt {
		t.Fatalf("missing sheet sprites: keeper=%v mite=%v bolt=%v", foundKeeper, foundMite, foundBolt)
	}
	if keeperFlip {
		t.Fatal("keeper moving right had FlipX=true, want FlipX=false because keeper.png defaults right")
	}
	if !miteFlip {
		t.Fatal("mite moving left had FlipX=false, want FlipX=true because mite.png defaults right")
	}
	if math.Abs(float64(boltRot)-math.Pi/2) > 1e-4 {
		t.Fatalf("bolt flying down had Rotation=%v, want pi/2", boltRot)
	}

	// Step 3: reverse horizontal directions -> keeper moves left (FlipX=true), mite moves right (FlipX=false).
	keeperPos.X -= 3
	mitePos.X += 2
	a.sync(Snapshot{
		Phase:   PhasePlaying,
		Arena:   arena,
		Keeper:  keeperPos,
		Enemies: []EnemyView{{ID: 10, Kind: "Ash Mite", Pos: mitePos, Radius: 7}},
	}, nil, stepSeconds)
	d.Ops = d.Ops[:0]
	a.draw(d, 1, runView)
	for _, s := range a.sheetSprites {
		if s.Z == 4 && !s.FlipX {
			t.Fatal("keeper moving left had FlipX=false, want FlipX=true")
		}
		if s.Z == 2 && s.FlipX {
			t.Fatal("mite moving right had FlipX=true, want FlipX=false")
		}
	}

	// Confirm the stylized sheet batch rendered with smooth image sampling.
	smoothed := false
	for _, op := range d.Ops {
		if (op.Kind == core.DrawAtlas || op.Kind == core.DrawImage) && op.Image != nil && !isGround(op.Image) && op.Sampling == core.ImageSamplingSmooth {
			smoothed = true
			break
		}
	}
	if !smoothed {
		t.Fatal("draw did not use smooth image sampling for stylized sprite sheets")
	}
}

// TestMissingSpritesAndIconsFallBackCleanly checks that missing or corrupt sprite sheets and icons fall back to the procedural atlas and glyph badges without panicking.
func TestMissingSpritesAndIconsFallBackCleanly(t *testing.T) {
	empty := fstest.MapFS{}
	fallbackArt := newArtFromFS(empty, empty)
	if fallbackArt.sheetAtlas != nil {
		t.Fatal("empty FS produced a non-nil sheetAtlas")
	}
	if fallbackArt.sheets.keeper.loaded || fallbackArt.sheets.mite.loaded {
		t.Fatal("empty FS marked sprite sheets as loaded")
	}
	if _, ok := fallbackArt.cardIcon("ember-bolt"); ok {
		t.Fatal("empty FS returned a card icon")
	}

	// Render a full frame using only the procedural fallback atlas.
	r := quietRun(t)
	r.addEnemy(0, r.keeper.pos.Add(Vec2{X: 30}), 1, 1)
	r.addEnemy(1, r.keeper.pos.Add(Vec2{X: -30}), 1, 1)
	a := newArenaRenderer(fallbackArt)
	a.startRun(5)
	stepOnce(r, Controls{})
	a.sync(r.Snapshot(), r.Events(), stepSeconds)
	d := &core.Drawing{}
	a.draw(d, 1, runView)
	if got := countSprites(d.Ops); got < 3 {
		t.Fatalf("fallback frame drew %d sprites, want at least keeper and two enemies", got)
	}
	if len(a.sheetSprites) != 0 {
		t.Fatalf("fallback frame populated %d sheetSprites, want 0", len(a.sheetSprites))
	}

	// Corrupt PNG bytes for one sheet must be ignored without panicking while valid sheets still load.
	keeperBytes, err := embeddedSprites.ReadFile("assets/sprites/keeper.png")
	if err != nil {
		t.Fatalf("reading embedded keeper.png: %v", err)
	}
	partial := fstest.MapFS{
		"assets/sprites/keeper.png": &fstest.MapFile{Data: keeperBytes},
		"assets/sprites/mite.png":   &fstest.MapFile{Data: []byte("not a png")},
	}
	partArt := newArtFromFS(partial, empty)
	if !partArt.sheets.keeper.loaded {
		t.Fatal("valid keeper.png failed to load alongside corrupt mite.png")
	}
	if partArt.sheets.mite.loaded {
		t.Fatal("corrupt mite.png was marked loaded")
	}
}

// TestUpgradeCardIconsLoadAndFallback checks that all 12 embedded card icons load as 64x64 images and the heal fallback card returns false.
func TestUpgradeCardIconsLoadAndFallback(t *testing.T) {
	a := newArt()
	expected := []string{
		"ember-bolt", "lantern-halo", "cinder-trail", "bell-toll",
		"wick", "oil", "reach", "step", "warmth", "mending", "gathering", "glow",
	}
	for _, id := range expected {
		src, ok := a.cardIcon(id)
		if !ok {
			t.Fatalf("missing embedded card icon for %q", id)
		}
		if src.Pixels == nil {
			t.Fatalf("card icon %q has nil Pixels", id)
		}
		b := src.Pixels.Bounds()
		if b.Dx() != 64 || b.Dy() != 64 {
			t.Fatalf("card icon %q size = %dx%d, want 64x64", id, b.Dx(), b.Dy())
		}
	}
	if _, ok := a.cardIcon("ember-heal"); ok {
		t.Fatal("ember-heal unexpectedly matched an embedded icon")
	}
	if _, ok := a.cardIcon("unknown-card"); ok {
		t.Fatal("unknown-card unexpectedly matched an embedded icon")
	}
}

// TestLightingReadabilityAndGroundContrast verifies the moonlit twilight lighting parameters
// and procedural forest floor brightness so the arena stays readable across the entire viewport.
func TestLightingReadabilityAndGroundContrast(t *testing.T) {
	if ambientDarkness < 0.20 || ambientDarkness > 0.32 {
		t.Fatalf("ambientDarkness = %v, want in [0.20, 0.32] for moonlit readability", ambientDarkness)
	}

	// Ground base soil should have clearly visible luminance (not near-black).
	r, g, b, _ := groundSoilBase.Components()
	luma := 0.2126*r + 0.7152*g + 0.0722*b
	if luma < 0.16 {
		t.Fatalf("groundSoilBase luminance = %.3f, want >= 0.16 so ground reads clearly", luma)
	}
}

// TestDamageVignetteLevelFlourishAndShakeSetting checks keeper damage vignette rendering,
// level-up flourish sparks/ring, and strict adherence to the screen shake toggle.
func TestDamageVignetteLevelFlourishAndShakeSetting(t *testing.T) {
	a := newArenaRenderer(newArt())
	a.startRun(17)
	arena := Vec2{X: 1920, Y: 1280}
	keeper := Vec2{X: 960, Y: 640}

	// Initial step establishes baseline health.
	a.sync(Snapshot{Phase: PhasePlaying, Arena: arena, Keeper: keeper, Health: 100, MaxHealth: 100}, nil, stepSeconds)
	if a.fx.damageVignette != 0 {
		t.Fatalf("initial damageVignette = %v, want 0", a.fx.damageVignette)
	}

	// Taking damage triggers damage vignette and shake.
	a.sync(Snapshot{Phase: PhasePlaying, Arena: arena, Keeper: keeper, Health: 85, MaxHealth: 100}, []Event{{Kind: EventHit, ID: 1}}, stepSeconds)
	if a.fx.damageVignette < 0.99 {
		t.Fatalf("damageVignette after keeper damage = %v, want 1.0", a.fx.damageVignette)
	}
	if a.fx.shake <= 0 {
		t.Fatal("keeper damage did not trigger screen shake when enabled")
	}

	d := &core.Drawing{}
	a.draw(d, 1, runView)
	gradients := 0
	for _, op := range d.Ops {
		if op.Kind == core.DrawFillGradient {
			gradients++
		}
	}
	// At least the damage vignette's radial gradient.
	if gradients < 1 {
		t.Fatalf("drew %d radial gradients on damaged frame, want >= 1 (damage vignette)", gradients)
	}

	// Level-up triggers expanding flourish ring and golden sparks.
	a.sync(Snapshot{Phase: PhaseLevelUp, Arena: arena, Keeper: keeper, Health: 85, MaxHealth: 100}, []Event{{Kind: EventLevelUp}}, stepSeconds)
	if a.fx.levelFlash <= 0 {
		t.Fatal("EventLevelUp did not start levelFlash flourish")
	}
	if len(a.fx.particles) == 0 {
		t.Fatal("EventLevelUp emitted no golden spark particles")
	}

	// Disabling screen shake clears and suppresses all shake impulses.
	a.setShakeEnabled(false)
	a.sync(Snapshot{Phase: PhasePlaying, Arena: arena, Keeper: keeper, Health: 70, MaxHealth: 100}, []Event{{Kind: EventBossSpawn, Pos: keeper}}, stepSeconds)
	if a.fx.shake != 0 || a.fx.shakeX != 0 || a.fx.shakeY != 0 {
		t.Fatalf("shake stayed active while disabled: shake=%v (%v,%v)", a.fx.shake, a.fx.shakeX, a.fx.shakeY)
	}

	// Vignette fades cleanly to zero over time.
	for range 60 {
		a.stepEffects(nil, false, stepSeconds)
	}
	if a.fx.damageVignette != 0 {
		t.Fatalf("damageVignette = %v after 1s, want 0", a.fx.damageVignette)
	}
}

// TestQualityLimitsParticlesAndGlowLights verifies Low quality enforces tighter particle
// and glow-light caps than High quality.
func TestQualityLimitsParticlesAndGlowLights(t *testing.T) {
	cfg := emberQualityConfig()
	low := qualityProfileFor(quality.Low, cfg.Presets.For(quality.Low))
	high := qualityProfileFor(quality.High, cfg.Presets.For(quality.High))

	a := newArenaRenderer(newArt())
	a.startRun(23)
	a.setQuality(low)

	arena := Vec2{X: 1920, Y: 1280}
	keeper := Vec2{X: 960, Y: 640}
	embers := make([]EmberView, 40)
	events := make([]Event, 120)
	for i := range embers {
		embers[i] = EmberView{Pos: keeper.Add(Vec2{X: float64(i%8) * 12, Y: float64(i/8) * 12})}
	}
	for i := range events {
		events[i] = Event{Kind: EventKill, ID: uint64(i + 1), Pos: keeper}
	}
	a.sync(Snapshot{Phase: PhasePlaying, Arena: arena, Keeper: keeper, Embers: embers}, events, stepSeconds)
	if len(a.fx.particles) > low.maxParticles {
		t.Fatalf("low quality kept %d particles, above cap %d", len(a.fx.particles), low.maxParticles)
	}

	d := &core.Drawing{}
	a.draw(d, 1, runView)
	if got := len(a.lights); got > low.maxGlowLights {
		t.Fatalf("low quality emitted %d lights, above %d glows", got, low.maxGlowLights)
	}

	a.setQuality(high)
	d.Ops = d.Ops[:0]
	a.draw(d, 1, runView)
	if got := len(a.lights); got <= low.maxGlowLights {
		t.Fatalf("high quality emitted %d lights, want more than low cap (%d)", got, low.maxGlowLights)
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
		a.draw(d, 0.5, runView)
	}
}

// BenchmarkWorstWave200Enemies measures the heaviest late-game frame: 200 enemies across all
// kinds plus the Gloomwarden elite and Hollow King boss with an active telegraph, all four
// weapons at level 5, enemy projectiles, embers, heal caches, hit flashes and full High quality.
func BenchmarkWorstWave200Enemies(b *testing.B) {
	r := NewRun(29)
	r.director.nextSpawn = math.Inf(1)
	for i := range r.weaponLevels {
		r.weaponLevels[i] = 5
	}
	r.recomputeStats()

	enemyKinds := len(r.data.enemies.Enemies)
	for i := range 199 {
		angle := float64(i) * 2 * math.Pi / 199
		ring := 48 + float64(i%10)*20
		pos := r.keeper.pos.Add(Vec2{X: math.Cos(angle), Y: math.Sin(angle)}.Scale(ring))
		r.addEnemy(i%enemyKinds, pos, 1e6, 1)
	}
	r.spawnBoss()
	if boss := r.bossEnemy(); boss != nil {
		boss.hp = 1e9
		boss.maxHP = 1e9
	}
	r.boss.state = bossRingWindup
	r.boss.timer = 0.5
	r.boss.timerMax = 1.0
	for i := range 24 {
		ang := float64(i) * 2 * math.Pi / 24
		r.addEnemyShot(r.keeper.pos.Add(Vec2{X: math.Cos(ang) * 90, Y: math.Sin(ang) * 90}), Vec2{X: -math.Cos(ang) * 20, Y: -math.Sin(ang) * 20}, 6, 4, 5, uint8(i%2))
	}
	for i := range 48 {
		ang := float64(i) * 2 * math.Pi / 48
		r.embers = append(r.embers, ember{pos: r.keeper.pos.Add(Vec2{X: math.Cos(ang) * 70, Y: math.Sin(ang) * 70}), value: 1})
	}

	a := newArenaRenderer(newArt())
	a.startRun(29)
	a.setQuality(defaultQualityProfile())
	r.Step(stepTime, Controls{Move: Vec2{X: 1, Y: 0}})
	a.sync(r.Snapshot(), r.Events(), stepSeconds)
	a.fx.damageVignette = 0.7

	// The frame includes the HUD at a 1280×720 window, with the boss bar up.
	kit, l := newHUDArt(embeddedHUD), layoutHUD(core.Size{Width: 1280, Height: 720}, core.Edges{})
	var motion hudMotion
	tr := evstrings.For(evstrings.EN)
	st := hudStateOf(r.Snapshot(), 0, false)

	d := &core.Drawing{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.keeper.health = r.stats.maxHealth
		r.xp = 0
		r.phase = PhasePlaying
		r.Step(stepTime, Controls{Move: Vec2{X: math.Cos(float64(i) * 0.05), Y: math.Sin(float64(i) * 0.05)}})
		a.sync(r.Snapshot(), r.Events(), stepSeconds)
		d.Ops = d.Ops[:0]
		a.draw(d, 0.5, runView)
		motion.step(st, stepSeconds)
		drawHUD(d, l, st, &motion, kit, a.art, tr, string(evstrings.EN), true, false)
	}
}

// TestFogOfWarCoversTheArenaEdgesAndLeavesTheFieldClear checks that the fog mask is clear inside the field and solid at the arena's border, and that a seed paints the same fog.
func TestFogOfWarCoversTheArenaEdgesAndLeavesTheFieldClear(t *testing.T) {
	const w, h, fog = 2400, 1760, 240
	m := newFogMask(3, w, h, fog)
	alpha := func(x, y int) uint8 { return m.Pix[m.PixOffset(x/fogScale, y/fogScale)+3] }
	for _, p := range [][2]int{{w / 2, h / 2}, {fog + 8, h / 2}, {w / 2, fog + 8}, {w - fog - 8, h - fog - 8}} {
		if a := alpha(p[0], p[1]); a > 40 {
			t.Fatalf("fog at field point %v has alpha %d, want nearly clear", p, a)
		}
	}
	for _, p := range [][2]int{{0, 0}, {20, h / 2}, {w / 2, h - 20}, {w - 20, 20}} {
		if a := alpha(p[0], p[1]); a != 255 {
			t.Fatalf("fog at border point %v has alpha %d, want solid", p, a)
		}
	}
	if again := newFogMask(3, w, h, fog); string(again.Pix) != string(m.Pix) {
		t.Fatal("the same seed painted a different fog")
	}
}

// TestFogOfWarDarkensPastTheKeepersSight checks that a frame lays a radial fog centred on the keeper, clear within its sight and solid beyond.
func TestFogOfWarDarkensPastTheKeepersSight(t *testing.T) {
	if visionClear <= 0 || visionClear >= visionSolid {
		t.Fatalf("sight clears to %v and turns solid at %v, want a clear area inside a fading rim", visionClear, visionSolid)
	}
	r := quietRun(t)
	a := newTestArena(t, r, 5)
	stepOnce(r, Controls{})
	a.sync(r.Snapshot(), r.Events(), stepSeconds)
	d := &core.Drawing{}
	a.draw(d, 1, runView)
	fog := 0
	for _, op := range d.Ops {
		if op.Kind == core.DrawFillGradient {
			fog++
		}
	}
	if fog == 0 {
		t.Fatal("the frame laid no fog past the keeper's sight")
	}
}

// TestTheCameraBlendsBetweenSteps checks that a frame between two steps places the camera between them, as it does the actors, so the world and the keeper glide together.
func TestTheCameraBlendsBetweenSteps(t *testing.T) {
	a := newArenaRenderer(newArt())
	a.arena = Vec2{X: 2000, Y: 2000}
	a.cam.snap(Vec2{X: 1000, Y: 1000}, a.arena)
	a.cam.follow(Vec2{X: 1100, Y: 1000}, a.arena, stepSeconds)
	was, now := a.cam.wasX, a.cam.x
	if now <= was {
		t.Fatalf("the camera stayed at x %v following a keeper to its right, from %v", now, was)
	}
	a.alpha = 0.5
	if x, _ := a.origin(); math.Abs(float64(x)-(was+now)/2) > 1e-3 {
		t.Fatalf("the camera's window starts at x %v halfway between steps, want %v", x, (was+now)/2)
	}
}

// TestAWiderWindowShowsMoreOfTheArena checks that a window wider than 16:9 widens the ground crop instead of leaving bars, keeps the keeper centred, and keeps the keeper's sight short of the spawn ring.
func TestAWiderWindowShowsMoreOfTheArena(t *testing.T) {
	r := quietRun(t)
	a := newTestArena(t, r, 7)
	stepOnce(r, Controls{})
	a.sync(r.Snapshot(), r.Events(), stepSeconds)
	a.draw(&core.Drawing{}, 1, runView)
	before, _ := a.origin()

	wide := core.Size{Width: 630, Height: viewHeight}
	d := &core.Drawing{}
	a.draw(d, 1, wide)
	after, _ := a.origin()
	if got, want := float64(before-after), (630.0-viewWidth)/2; math.Abs(got-want) > 1e-3 {
		t.Fatalf("widening the window moved the view's left edge by %v, want %v so the keeper stays centred", got, want)
	}
	cropped := false
	for _, op := range d.Ops {
		if op.Kind == core.DrawImage && op.Image != nil && isGround(op.Image) {
			cropped = op.W == 630 && op.H == viewHeight
		}
	}
	if !cropped {
		t.Fatal("the wide frame did not draw a 630x270 ground crop")
	}
	if visionSolid >= r.data.waves.Ring.Min {
		t.Fatalf("the keeper sees %v far, past the spawn ring's %v, so enemies would appear in sight", visionSolid, r.data.waves.Ring.Min)
	}
}

// TestFramesWithoutAStepKeepBlendingUnlessHeld checks that a display faster than the steps blends every frame, and that a held run holds its last step.
func TestFramesWithoutAStepKeepBlendingUnlessHeld(t *testing.T) {
	r := quietRun(t)
	a := newArenaRenderer(newArt())
	a.startRun(7)
	for range 2 {
		r.Step(stepTime, Controls{Move: Vec2{X: 1}})
		a.sync(r.Snapshot(), r.Events(), stepSeconds)
	}
	d := &core.Drawing{}
	a.draw(d, 0.25, runView)
	a.draw(d, 0.75, runView)
	if a.alpha != 0.75 {
		t.Fatalf("a second frame between the same steps drew at alpha %v, want 0.75", a.alpha)
	}
	a.hold()
	a.draw(d, 0.25, runView)
	if a.alpha != 1 {
		t.Fatalf("a held run drew at alpha %v, want its last step's places (1)", a.alpha)
	}
}
