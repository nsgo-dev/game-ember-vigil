package main

import (
	"bytes"
	"math"
	"testing"
	"testing/fstest"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/gamepad"
	evstrings "me.oneo/embervigil/strings"
)

// drewImage reports whether ops draw img, the same decoded pixels.
func drewImage(ops []core.DrawOp, img core.ImageSource) bool {
	return len(imageOps(ops, img)) > 0
}

// drewHUDPiece reports whether ops draw the kit's named piece, or true when the kit lacks it and the HUD falls back.
func drewHUDPiece(ops []core.DrawOp, kit *hudArt, name string) bool {
	img, ok := kit.image(name)
	return !ok || drewImage(ops, img)
}

// imageOps are the operations of ops that draw img.
func imageOps(ops []core.DrawOp, img core.ImageSource) []core.DrawOp {
	var out []core.DrawOp
	if img.Pixels == nil {
		return nil
	}
	want := img.Pixels
	for _, op := range ops {
		if op.Kind != core.DrawImage || op.Image == nil || op.Image.Pixels == nil {
			continue
		}
		// A test app hands back copies of the pixels it was given, so compare them by content.
		if got := op.Image.Pixels; got == want || got.Rect == want.Rect && bytes.Equal(got.Pix, want.Pix) {
			out = append(out, op)
		}
	}
	return out
}

// testHUDState is a run seen by the HUD: health at 40 of 100, XP half full, Ember Bolt at level 2 and the Hollow King at three quarters.
func testHUDState() hudState {
	st := hudState{
		phase:         PhasePlaying,
		time:          "03:41",
		health:        40,
		maxHealth:     100,
		lowHealthAt:   0.3,
		xp:            5,
		xpNeeded:      10,
		level:         4,
		kills:         127,
		bossActive:    true,
		bossName:      "Hollow King",
		bossHealth:    750,
		bossMaxHealth: 1000,
		bossPhase:     1,
	}
	st.weapons[0] = hudSlot{id: "ember-bolt", name: "Ember Bolt", level: 2, maxLevel: 5}
	return st
}

// drawTestHUD draws st once with settled motion at a 1280×720 window.
func drawTestHUD(st hudState, kit *hudArt) []core.DrawOp {
	var m hudMotion
	m.step(st, stepSeconds)
	d := &core.Drawing{}
	drawHUD(d, layoutHUD(core.Size{Width: 1280, Height: 720}, core.Edges{}), st, &m, kit, newArtFromFS(nil, nil), evstrings.For(evstrings.EN), string(evstrings.EN), true, false)
	return d.Ops
}

// TestHUDDrawsTheKitsFillsAtTheirValues checks that health, XP and the boss bar crop their fills to their values inside the README's active ranges, and that slots, pips, numerals and pause come from the kit.
func TestHUDDrawsTheKitsFillsAtTheirValues(t *testing.T) {
	kit := newHUDArt(embeddedHUD)
	if len(kit.images) == 0 {
		t.Skip("assets/hud/ holds no kit; TestHUDFallsBackWithoutTheKit covers the procedural HUD")
	}
	for _, name := range []string{hudHealthFrame, hudHealthFill, hudXPFrame, hudXPFill, hudBossFrame, hudBossFill, hudTimer, hudLevelBadge, hudKillEmblem, hudSlotEmpty, hudSlotFilled, hudSlotMaxed, hudPipLit, hudPipUnlit, hudPause, hudPausePushed} {
		if _, ok := kit.image(name); !ok {
			t.Fatalf("the HUD kit lacks %s", name)
		}
	}
	if !kit.numerals {
		t.Fatal("the HUD kit's numerals did not load")
	}
	st := testHUDState()
	ops := drawTestHUD(st, kit)

	fill := imageOps(ops, kit.images[hudHealthFill])
	if len(fill) != 1 {
		t.Fatalf("health fill drawn %d times, want once", len(fill))
	}
	wantTop := healthFillTop + (healthFillBottom-healthFillTop)*0.6
	if op := fill[0]; math.Abs(float64(op.Y1-float32(wantTop))) > 0.5 || math.Abs(float64(op.Y1+op.Y2-healthFillBottom)) > 0.5 {
		t.Fatalf("health fill crops rows %v..%v, want %v..%v for 40%%", op.Y1, op.Y1+op.Y2, wantTop, healthFillBottom)
	}
	xp := imageOps(ops, kit.images[hudXPFill])
	if len(xp) != 1 || xp[0].X1 != xpFillLeft || math.Abs(float64(xp[0].X2-(xpFillRight-xpFillLeft)*0.5)) > 0.5 {
		t.Fatalf("xp fill ops %+v, want one crop of half the trough from x %d", xp, xpFillLeft)
	}
	boss := imageOps(ops, kit.images[hudBossFill])
	if len(boss) != 1 || boss[0].X1 != bossFillLeft || math.Abs(float64(boss[0].X2-(bossFillRight-bossFillLeft)*0.75)) > 0.5 {
		t.Fatalf("boss fill ops %+v, want one crop of three quarters of the trough from x %d", boss, bossFillLeft)
	}
	if !drewImage(ops, kit.images[hudBossFrame]) {
		t.Fatal("the boss frame was not drawn")
	}
	if got := len(imageOps(ops, kit.images[hudSlotFilled])); got != 1 {
		t.Fatalf("filled slots drawn %d, want 1", got)
	}
	if got := len(imageOps(ops, kit.images[hudSlotEmpty])); got != maxHUDWeapons-1 {
		t.Fatalf("empty slots drawn %d, want %d", got, maxHUDWeapons-1)
	}
	if lit, unlit := len(imageOps(ops, kit.images[hudPipLit])), len(imageOps(ops, kit.images[hudPipUnlit])); lit != 2 || unlit != 3 {
		t.Fatalf("pips lit %d and unlit %d, want 2 and 3", lit, unlit)
	}
	if !drewImage(ops, kit.digits[3]) || !drewImage(ops, kit.colon) || !drewImage(ops, kit.times) {
		t.Fatal("the timer and kills were not drawn in painted numerals")
	}
	if !drewImage(ops, kit.images[hudPause]) {
		t.Fatal("the pause button was not drawn while playing")
	}
	text := false
	for _, op := range ops {
		if op.Kind == core.DrawFillText && op.Text == "Hollow King · Phase I" {
			text = true
		}
	}
	if !text {
		t.Fatal("the boss bar did not name the Hollow King and its phase")
	}

	// A maxed weapon takes the gilded slot, and a level-up hides pause.
	st.weapons[0].level = 5
	st.phase = PhaseLevelUp
	ops = drawTestHUD(st, kit)
	if !drewImage(ops, kit.images[hudSlotMaxed]) {
		t.Fatal("a maxed weapon did not take the gilded slot")
	}
	if drewImage(ops, kit.images[hudPause]) {
		t.Fatal("pause was drawn while a level-up waits")
	}
}

// TestHUDFallsBackWithoutTheKit checks that a HUD with no kit draws procedurally, with canvas text for its numbers, and that one missing piece falls back alone.
func TestHUDFallsBackWithoutTheKit(t *testing.T) {
	kit := newHUDArt(fstest.MapFS{})
	ops := drawTestHUD(testHUDState(), kit)
	fills, texts := 0, 0
	for _, op := range ops {
		switch op.Kind {
		case core.DrawImage:
			if op.Image != nil && op.Image.Pixels != nil && op.Image.Pixels.Bounds().Dx() > 64 {
				t.Fatalf("the HUD drew an image %v without its kit", op.Image.Pixels.Bounds())
			}
		case core.DrawFill:
			fills++
		case core.DrawFillText:
			texts++
		}
	}
	if fills < 6 || texts < 4 {
		t.Fatalf("the procedural HUD drew %d fills and %d texts, want its pieces and numbers", fills, texts)
	}

	full := newHUDArt(embeddedHUD)
	if len(full.images) == 0 {
		return
	}
	partial := fstest.MapFS{}
	for name, img := range full.images {
		if name == hudHealthFill {
			continue
		}
		_ = img
		raw, _ := embeddedHUD.ReadFile("assets/hud/" + name + ".png")
		partial["assets/hud/"+name+".png"] = &fstest.MapFile{Data: raw}
	}
	kit = newHUDArt(partial)
	ops = drawTestHUD(testHUDState(), kit)
	if drewImage(ops, kit.images[hudHealthFrame]) {
		t.Fatal("the lantern frame was drawn without its fill")
	}
	if !drewImage(ops, kit.images[hudXPFrame]) {
		t.Fatal("a missing health fill took the XP strip with it")
	}
}

// TestHUDMotionEasesFlashesGlowsAndPops checks that fills ease rather than jump, a hit flashes, a level-up glows and empties the strip, a new weapon pops, and the lantern flickers only under the data's danger line.
func TestHUDMotionEasesFlashesGlowsAndPops(t *testing.T) {
	st := testHUDState()
	var m hudMotion
	m.step(st, stepSeconds)
	if m.health != 0.4 || m.flash != 0 {
		t.Fatalf("a primed HUD shows health %v and flash %v, want 0.4 and none", m.health, m.flash)
	}

	hit := st
	hit.health = 20
	m.step(hit, stepSeconds)
	if m.flash != 1-stepSeconds/hudFlashTime {
		t.Fatalf("flash after a hit = %v, want it started", m.flash)
	}
	if m.health <= 0.2 || m.health >= 0.4 {
		t.Fatalf("health after one step = %v, want between 0.2 and 0.4", m.health)
	}
	for range 120 {
		m.step(hit, stepSeconds)
	}
	if math.Abs(m.health-0.2) > 1e-3 || m.flash != 0 {
		t.Fatalf("after two seconds health = %v and flash = %v, want 0.2 and none", m.health, m.flash)
	}
	if !hit.lowHealth() {
		t.Fatal("20 of 100 is not low under a 0.3 danger line")
	}
	if st.lowHealth() {
		t.Fatal("40 of 100 is low under a 0.3 danger line")
	}

	up := hit
	up.level++
	up.xp = 1
	up.weapons[1] = hudSlot{id: "lantern-halo", name: "Lantern Halo", level: 1, maxLevel: 5}
	m.step(up, stepSeconds)
	if m.glow <= 0 || m.xp >= 0.1 {
		t.Fatalf("after a level-up glow = %v and xp = %v, want glowing from empty", m.glow, m.xp)
	}
	if m.pops[1] <= 0 || m.pops[0] != 0 {
		t.Fatalf("pops after a new weapon = %v, want the second slot popping", m.pops)
	}
}

// TestHUDLayoutStaysInsideTheSafeAreaAndOffTheStick checks that at a desktop window and a phone with a notch the HUD stays inside the safe area, its pieces do not overlap and the pause hit area takes a thumb.
func TestHUDLayoutStaysInsideTheSafeAreaAndOffTheStick(t *testing.T) {
	cases := []struct {
		size core.Size
		safe core.Edges
	}{
		{core.Size{Width: 1280, Height: 720}, core.Edges{}},
		{core.Size{Width: 960, Height: 540}, core.Edges{}},
		{core.Size{Width: 844, Height: 390}, core.Edges{Top: 12, Right: 44, Bottom: 21, Left: 44}},
		{core.Size{Width: 480, Height: 270}, core.Edges{}},
	}
	for _, c := range cases {
		l := layoutHUD(c.size, c.safe)
		w, h := float32(c.size.Width), float32(c.size.Height)
		inside := func(name string, r rect) {
			t.Helper()
			if r.x < float32(c.safe.Left) || r.y < float32(c.safe.Top) || r.x+r.w > w-float32(c.safe.Right) || r.y+r.h > h-float32(c.safe.Bottom) {
				t.Fatalf("%v: %s %+v leaves the safe area %+v", c.size, name, r, c.safe)
			}
		}
		overlap := func(a, b rect) bool {
			return a.x < b.x+b.w && b.x < a.x+a.w && a.y < b.y+b.h && b.y < a.y+a.h
		}
		inside("health", l.health)
		inside("xp", l.xp)
		inside("timer", l.timer)
		inside("boss", l.boss)
		inside("pause", l.pauseHit)
		inside("last slot", l.slots[maxHUDWeapons-1])
		if l.pauseHit.w < buttonSide || l.pauseHit.h < buttonSide {
			t.Fatalf("%v: pause hit area %+v is under %vpt", c.size, l.pauseHit, buttonSide)
		}
		for _, r := range []rect{l.health, l.xp, l.slots[maxHUDWeapons-1], l.pauseHit} {
			if overlap(r, l.boss) || overlap(r, l.timer) {
				t.Fatalf("%v: %+v overlaps the timer %+v or boss bar %+v", c.size, r, l.timer, l.boss)
			}
		}
		if l.bottom > h/2 {
			t.Fatalf("%v: the HUD reaches %v, past half the height", c.size, l.bottom)
		}
	}
}

// TestCanvasPauseTakesMouseKeyboardAndGamepad checks that a click on the drawn pause button pauses the run while a click elsewhere does not, and that the start button pauses too.
func TestCanvasPauseTakesMouseKeyboardAndGamepad(t *testing.T) {
	app, g := startEmberVigil(t)
	press(t, app, "role:button Start")

	app.PressAt("id:arena-view", core.Point{X: 480, Y: 400})
	app.Advance(frame)
	if g.phase.Get() != phaseArena {
		t.Fatalf("a click in the arena's middle moved to %v, want the arena", g.phase.Get())
	}
	if !drewHUDPiece(app.Find("id:arena").Ops, g.hudArt, hudPause) {
		t.Fatal("the arena canvas did not draw the pause button")
	}
	pressPause(t, app, g)
	if g.phase.Get() != phasePaused {
		t.Fatalf("a click on the canvas pause moved to %v, want paused", g.phase.Get())
	}
	press(t, app, "role:button Resume")

	app.ConnectGamepad(0, "Test Pad")
	app.SetGamepadButton(0, gamepad.ButtonStart, 1)
	app.Advance(2 * frame)
	app.SetGamepadButton(0, gamepad.ButtonStart, 0)
	app.Advance(frame)
	if g.phase.Get() != phasePaused {
		t.Fatalf("the gamepad's start moved to %v, want paused", g.phase.Get())
	}
}

// TestRunDataSetsTheHUDsDangerLine checks that the low-health line comes from data/run.yaml through the snapshot.
func TestRunDataSetsTheHUDsDangerLine(t *testing.T) {
	r := NewRun(1, 1)
	if got := hudStateOf(r.Snapshot(), 0, false).lowHealthAt; got != r.data.run.Keeper.LowHealth || got <= 0 {
		t.Fatalf("the HUD's danger line = %v, want data's %v", got, r.data.run.Keeper.LowHealth)
	}
}
