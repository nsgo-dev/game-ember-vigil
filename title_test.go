package main

import (
	"testing"

	"github.com/nsgo-dev/game/packages/game/input"
	"github.com/nsgo-dev/nsgo/core"
	evstrings "me.oneo/embervigil/strings"
)

// drawTestTitle draws the title once at a 1280×720 window, t seconds after it opened, with embers embers rising.
func drawTestTitle(kit *titleArt, t float64, embers int, focus int) []core.DrawOp {
	d := &core.Drawing{}
	size := core.Size{Width: 1280, Height: 720}
	tr := evstrings.For(evstrings.EN)
	p := &overlayPainter{d: d, size: size, art: newHUDArt(embeddedHUD), icons: newArtFromFS(nil, nil), m: &overlayMotion{}, tr: tr, lang: string(evstrings.EN)}
	items := []titleItem{{"title-start", tr.Start, nil}, {"title-settings", tr.Settings, nil}, {"title-quit", tr.Quit, nil}}
	m := titleMotion{clock: t}
	drawTitle(p, kit, &m, titleView{l: layoutTitle(size, core.Edges{}, len(items)), items: items, focus: focus, tr: tr, embers: embers})
	return d.Ops
}

// TestTitleDrawsTheKitsLayersAndButtons checks that the cover draws every layer, moves between frames, scales its embers by the count it is given, and frames the buttons in the kit's states.
func TestTitleDrawsTheKitsLayersAndButtons(t *testing.T) {
	kit := newTitleArt(embeddedTitle)
	if !kit.animated() {
		t.Skip("assets/title/ holds no full kit; TestTitleFallsBackToAStillCover covers the procedural cover")
	}
	ops := drawTestTitle(kit, 3, titleEmbers, 1)
	for _, name := range []string{titleBG, titleMist, titleGlow, titleKeeper, titleCanopy, titleLogoEN, titleButton, titleButtonFocus} {
		img, _ := kit.image(name)
		if !drewImage(ops, img) {
			t.Errorf("the title does not draw %s", name)
		}
	}
	// Start and Quit are normal, Settings focused: two normal frames of nine slices each and one focused.
	normal, _ := kit.image(titleButton)
	if n := len(imageOps(ops, normal)); n != 2*9 {
		t.Errorf("the title draws %d slices of the normal frame, want %d", n, 2*9)
	}

	keeper, _ := kit.image(titleKeeper)
	a, b := imageOps(ops, keeper), imageOps(drawTestTitle(kit, 4.2, titleEmbers, 1), keeper)
	if len(a) != 1 || len(b) != 1 || a[0].X == b[0].X && a[0].Y == b[0].Y {
		t.Error("the keeper does not move between frames")
	}

	count := func(embers int) int {
		ops := drawTestTitle(kit, 3, embers, 0)
		n := 0
		for _, img := range kit.embers {
			n += len(imageOps(ops, img))
		}
		return n
	}
	if all, few := count(titleEmbers), count(titleEmbers/4); all != titleEmbers || few != titleEmbers/4 {
		t.Errorf("the title draws %d and %d embers, want %d and %d", all, few, titleEmbers, titleEmbers/4)
	}
}

// TestTitleFallsBackToAStillCover checks that without the kit the title draws a still procedural cover and the overlays' painted buttons, the same at any time.
func TestTitleFallsBackToAStillCover(t *testing.T) {
	kit := newTitleArt(nil)
	if kit.animated() {
		t.Fatal("an empty kit animates the cover")
	}
	a, b := drawTestTitle(kit, 1, titleEmbers, 0), drawTestTitle(kit, 2, titleEmbers, 0)
	if len(a) == 0 {
		t.Fatal("the fallback title draws nothing")
	}
	for _, op := range a {
		if op.Kind == core.DrawImage && op.Image != nil && op.Image.Pixels != nil {
			if img, ok := newHUDArt(embeddedHUD).image(hudPanel); !ok || op.Image.Pixels.Rect != img.Pixels.Rect {
				t.Fatalf("the fallback title draws an image that is not the HUD's panel: %v", op.Image.Pixels.Rect)
			}
		}
	}
	if len(a) != len(b) {
		t.Errorf("the fallback cover changes between frames: %d and %d ops", len(a), len(b))
	}
}

// TestStickDrawsTheKitsPiecesAndFallsBack checks that the stick draws the kit's base and idle knob, the active knob while held, and the pad's circles without the kit.
func TestStickDrawsTheKitsPiecesAndFallsBack(t *testing.T) {
	kit := newHUDArt(embeddedHUD)
	pad := input.NewVirtualPad()
	pad.MapPoint = func(p core.Point) (core.Point, bool) { return p, true }
	stick := pad.AddStick(&input.VirtualStick{Name: "stick", Radius: stickRadius, Center: core.Point{X: 100, Y: 600}, Zone: core.Bounds{Size: core.Size{Width: 300, Height: 720}}, Floating: true})

	draw := func(kit *hudArt) []core.DrawOp {
		d := &core.Drawing{}
		drawStick(d, stick, kit, pad)
		return d.Ops
	}
	base, okBase := kit.image(hudStickBase)
	idle, okIdle := kit.image(hudStickKnob)
	active, okActive := kit.image(hudStickKnobActive)
	if okBase && okIdle && okActive {
		ops := draw(kit)
		if !drewImage(ops, base) || !drewImage(ops, idle) || drewImage(ops, active) {
			t.Error("an idle stick does not draw the base and the idle knob")
		}
		if b := imageOps(ops, base); len(b) == 1 && (b[0].W != stickBasePt || b[0].X != 100-stickBasePt/2) {
			t.Errorf("the base lands at %v, %vpt wide", b[0].X, b[0].W)
		}
		pad.OnTouch(core.TouchEvent{Phase: core.TouchStart, Touches: []core.Touch{{ID: 1, Position: core.Point{X: 120, Y: 580}, Changed: true}}})
		pad.OnTouch(core.TouchEvent{Phase: core.TouchMove, Touches: []core.Touch{{ID: 1, Position: core.Point{X: 150, Y: 580}, Changed: true}}})
		if ops := draw(kit); !drewImage(ops, active) || drewImage(ops, idle) {
			t.Error("a held stick does not draw the active knob")
		}
	}
	ops := draw(newHUDArt(nil))
	for _, op := range ops {
		if op.Kind == core.DrawImage {
			t.Fatal("the fallback stick draws an image")
		}
	}
	if len(ops) == 0 {
		t.Fatal("the fallback stick draws nothing")
	}
}

// TestPauseButtonStaysThumbSized checks that the HUD's pause button never draws smaller than buttonSide, even on a small phone.
func TestPauseButtonStaysThumbSized(t *testing.T) {
	for _, size := range []core.Size{{Width: 1280, Height: 720}, {Width: 844, Height: 390}, {Width: 667, Height: 375}} {
		l := layoutHUD(size, core.Edges{Left: 44, Right: 44, Bottom: 21})
		if l.pause.w < buttonSide || l.pause.h < buttonSide {
			t.Errorf("%v: the pause button is %vx%v", size, l.pause.w, l.pause.h)
		}
		if l.pause.x+l.pause.w > float32(size.Width)-44 {
			t.Errorf("%v: the pause button leaves the safe area", size)
		}
	}
}
