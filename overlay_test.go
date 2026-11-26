package main

import (
	"strings"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/gamepad"
	evstrings "me.oneo/embervigil/strings"
)

// testOffers is a level-up of a weapon, a passive and the heal.
func testOffers() hudState {
	st := testHUDState()
	st.phase = PhaseLevelUp
	st.offers[0] = hudCard{id: "ember-bolt", name: "Ember Bolt", kind: offerWeapon, description: "Bolts fly faster and pierce one more enemy.", level: 3, maxLevel: 5}
	st.offers[1] = hudCard{id: "wick", name: "Wick", kind: offerPassive, description: "Weapons fire more often.", level: 1, maxLevel: 5}
	st.offers[2] = hudCard{id: "ember-heal", name: "Ember Heal", kind: offerHeal, description: "Restore some health.", heal: true}
	return st
}

// settled is overlay motion long after a deal and a panel's opening.
func settled(focus int) *overlayMotion {
	var m overlayMotion
	for range 120 {
		m.step(stepSeconds, focus)
	}
	return &m
}

func testPainter(d *core.Drawing, size core.Size, kit *hudArt, m *overlayMotion, lang evstrings.Lang) *overlayPainter {
	return &overlayPainter{d: d, size: size, art: kit, icons: newArt(), m: m, tr: evstrings.For(lang), lang: string(lang)}
}

func TestOverlayMotionDealsLiftsAndBurnsOnce(t *testing.T) {
	var m overlayMotion
	m.dealCards()
	m.step(stepSeconds, 1)
	if m.dealt(0) <= 0 || m.dealt(0) >= 1 || m.dealt(2) != 0 {
		t.Fatalf("one step into a deal the cards are at %v, %v, %v; want the first flying and the last waiting", m.dealt(0), m.dealt(1), m.dealt(2))
	}
	for range 60 {
		m.step(stepSeconds, 1)
	}
	if m.dealt(2) != 1 || m.lift[1] < 0.99 || m.lift[0] > 0.01 {
		t.Fatalf("after a second the deal is at %v and the lifts %v, want dealt and only card 1 lifted", m.dealt(2), m.lift)
	}
	if !m.burnAway(2) || m.burnAway(0) {
		t.Fatal("burnAway took no card, or a second card while one burns")
	}
	if _, ok := m.burnt(); ok {
		t.Fatal("a card burnt at once")
	}
	for range int(burnTime*rate) + 1 {
		m.step(stepSeconds, 1)
	}
	if i, ok := m.burnt(); !ok || i != 2 {
		t.Fatalf("after the burn burnt() = %d, %v, want card 2", i, ok)
	}
	if _, ok := m.burnt(); ok {
		t.Fatal("a burnt card was handed over twice")
	}
}

func TestOverlayLayoutsFitTheSafeArea(t *testing.T) {
	cases := []struct {
		name string
		size core.Size
		safe core.Edges
	}{
		{"desktop", core.Size{Width: 1280, Height: 720}, core.Edges{}},
		{"window", core.Size{Width: 960, Height: 540}, core.Edges{}},
		{"phone", core.Size{Width: 844, Height: 390}, core.Edges{Top: 12, Right: 44, Bottom: 21, Left: 44}},
		{"small phone", core.Size{Width: 667, Height: 375}, core.Edges{Bottom: 21}},
	}
	for _, c := range cases {
		inside := func(what string, r rect) {
			t.Helper()
			const eps = 0.5
			if r.x < float32(c.safe.Left)-eps || r.y < float32(c.safe.Top)-eps ||
				r.x+r.w > float32(c.size.Width-c.safe.Right)+eps || r.y+r.h > float32(c.size.Height-c.safe.Bottom)+eps {
				t.Fatalf("%s: %s %+v leaves the safe area of %v %+v", c.name, what, r, c.size, c.safe)
			}
		}
		cards := layoutCards(c.size, c.safe, offerCount)
		inside("heading", cards.heading)
		for i, r := range cards.cards {
			inside("card", r)
			// A lifted card still clears the heading.
			if r.y-cardLiftPt*cards.scale < cards.heading.y+cards.heading.h-0.5 {
				t.Fatalf("%s: lifted card %d overlaps the heading", c.name, i)
			}
			if r.w < 120 {
				t.Fatalf("%s: card %d is %v points wide, too narrow to read", c.name, i, r.w)
			}
			if i > 0 && r.x < cards.cards[i-1].x+cards.cards[i-1].w {
				t.Fatalf("%s: card %d overlaps card %d", c.name, i, i-1)
			}
		}
		for _, l := range []panelLayout{layoutPause(c.size, c.safe, false), layoutPause(c.size, c.safe, true), layoutResults(c.size, c.safe)} {
			inside("panel", l.panel)
			for _, b := range l.buttons {
				inside("button", b)
				if b.h < buttonSide-0.5 {
					t.Fatalf("%s: a painted button is %v points tall, under %v", c.name, b.h, buttonSide)
				}
			}
		}
	}
}

func TestOverlaysDrawTheKitAndFallBack(t *testing.T) {
	size := core.Size{Width: 1280, Height: 720}
	kit := newHUDArt(embeddedHUD)
	st := testOffers()
	for _, lang := range []evstrings.Lang{evstrings.EN, evstrings.ZhCN} {
		d := &core.Drawing{}
		drawCards(testPainter(d, size, kit, settled(0), lang), layoutCards(size, core.Edges{}, offerCount), st, 0)
		for _, name := range []string{hudCardFocused, hudCardCommon, hudPipLit, hudPipUnlit} {
			if !drewHUDPiece(d.Ops, kit, name) {
				t.Fatalf("%s: the cards did not draw %s", lang, name)
			}
		}
		if img, ok := newArt().cardIcon("ember-bolt"); ok && !drewImage(d.Ops, img) {
			t.Fatalf("%s: the cards did not draw Ember Bolt's icon", lang)
		}
		tr := evstrings.For(lang)
		if !drewText(d.Ops, tr.Card("wick", "Wick", "").Name) {
			t.Fatalf("%s: the cards did not write the passive's name", lang)
		}

		d = &core.Drawing{}
		drawPause(testPainter(d, size, kit, settled(0), lang), layoutPause(size, core.Edges{}, false), false, 1)
		if !drewHUDPiece(d.Ops, kit, hudPanel) || !drewText(d.Ops, tr.Paused) || !drewText(d.Ops, tr.Resume) {
			t.Fatalf("%s: the pause panel lacks its frame, title or buttons", lang)
		}

		d = &core.Drawing{}
		st := testHUDState()
		st.phase = PhaseVictory
		txt := resultsTextOf(tr, st, Records{})
		drawResults(testPainter(d, size, kit, settled(0), lang), layoutResults(size, core.Edges{}), st, txt, 0)
		if !drewHUDPiece(d.Ops, kit, hudPanel) || !drewText(d.Ops, tr.VictoryTitle) || !drewText(d.Ops, tr.PlayAgain) {
			t.Fatalf("%s: the results lack their frame, title or buttons", lang)
		}
		if kit.numeralsReady() && (!drewImage(d.Ops, kit.digits[3]) || !drewImage(d.Ops, kit.colon) || !drewImage(d.Ops, kit.times)) {
			t.Fatalf("%s: the results did not write the time and kills in the painted numerals", lang)
		}
	}

	// Without the kit every overlay still draws its frames and words.
	empty := newHUDArt(nil)
	d := &core.Drawing{}
	drawCards(testPainter(d, size, empty, settled(0), evstrings.EN), layoutCards(size, core.Edges{}, offerCount), st, 0)
	drawPause(testPainter(d, size, empty, settled(0), evstrings.EN), layoutPause(size, core.Edges{}, true), true, 0)
	st.phase = PhaseDefeat
	drawResults(testPainter(d, size, empty, settled(0), evstrings.EN), layoutResults(size, core.Edges{}), st, resultsTextOf(evstrings.For(evstrings.EN), st, Records{}), 0)
	for _, want := range []string{"Ember Bolt", "Return to Title?", "DEFEAT", "Play Again", "127"} {
		if !drewText(d.Ops, want) {
			t.Fatalf("without the kit the overlays did not write %q", want)
		}
	}
}

// drewText reports whether ops fill text that holds want.
func drewText(ops []core.DrawOp, want string) bool {
	for _, op := range ops {
		if op.Kind == core.DrawFillText && strings.Contains(op.Text, want) {
			return true
		}
	}
	return false
}

func TestWrapTextBreaksWordsAndCJKAndEllipsizes(t *testing.T) {
	d := &core.Drawing{}
	f := core.Font{Size: 12}
	en := wrapText(d, "Bolts fly faster and pierce one more enemy every time.", f, 80, 0)
	if len(en) < 2 {
		t.Fatalf("an English description wrapped to %q, want several lines", en)
	}
	for _, l := range en {
		if d.MeasureText(l, f) > 80 && !strings.Contains(l, " ") {
			continue // a single word wider than the line stays whole
		}
		if d.MeasureText(l, f) > 80 {
			t.Fatalf("line %q is wider than 80 points", l)
		}
	}
	zh := wrapText(d, "武器的攻击间隔缩短，余烬弹飞得更快并多穿透一个敌人。", f, 60, 2)
	if len(zh) != 2 || !strings.Contains(zh[1], "…") {
		t.Fatalf("a Chinese description cut to two lines reads %q, want two lines ending in an ellipsis", zh)
	}
}

func TestCardsTakeHoverAndGamepadAndBurnBeforeTheRunResumes(t *testing.T) {
	app, g := startEmberVigil(t)
	press(t, app, "role:button Start")
	g.run.xp = g.run.xpNeeded()
	app.Advance(2 * frame)
	if p := g.run.Snapshot().Phase; p != PhaseLevelUp {
		t.Fatalf("phase = %v, want PhaseLevelUp", p)
	}

	app.Hover("id:level-up-card-2")
	app.Advance(frame)
	if g.cardFocus != 2 {
		t.Fatalf("hovering card 2 focused card %d", g.cardFocus)
	}
	app.ConnectGamepad(0, "Test Pad")
	tap := func(b gamepad.Button) {
		app.SetGamepadButton(0, b, 1)
		app.Advance(2 * frame)
		app.SetGamepadButton(0, b, 0)
		app.Advance(frame)
	}
	tap(gamepad.ButtonDPadLeft)
	if g.cardFocus != 1 {
		t.Fatalf("the d-pad moved focus to card %d, want 1", g.cardFocus)
	}
	tap(gamepad.ButtonSouth)
	if !g.overlay.burning || g.overlay.burnCard != 1 {
		t.Fatalf("the gamepad's confirm burns card %d (burning %v), want card 1", g.overlay.burnCard, g.overlay.burning)
	}
	// A second choice while the first burns changes nothing, and the run waits.
	app.Press("id:level-up-card-0")
	app.Advance(frame)
	if g.overlay.burnCard != 1 || g.run.Snapshot().Phase != PhaseLevelUp {
		t.Fatalf("a press during the burn moved it to card %d in phase %v", g.overlay.burnCard, g.run.Snapshot().Phase)
	}
	app.Advance(time.Duration(burnFrames) * frame)
	if p := g.run.Snapshot().Phase; p != PhasePlaying || g.run.Snapshot().Level != 2 {
		t.Fatalf("after the burn the run is in %v at level %d, want playing at level 2", p, g.run.Snapshot().Level)
	}
}

func TestPauseAndResultsTakeHoverKeyboardAndGamepad(t *testing.T) {
	app, g := startEmberVigil(t)
	press(t, app, "role:button Start")
	app.ConnectGamepad(0, "Test Pad")
	tap := func(b gamepad.Button) {
		app.SetGamepadButton(0, b, 1)
		app.Advance(2 * frame)
		app.SetGamepadButton(0, b, 0)
		app.Advance(frame)
	}

	tap(gamepad.ButtonStart)
	if g.phase.Get() != phasePaused {
		t.Fatalf("start moved to %v, want paused", g.phase.Get())
	}
	if !drewHUDPiece(app.Find("id:arena").Ops, g.hudArt, hudPanel) {
		t.Fatal("the canvas did not draw the pause panel")
	}
	app.Hover("id:pause-quit")
	app.Advance(frame)
	if got := g.menuFocus.Get(); got != 2 {
		t.Fatalf("hovering Quit focused item %d, want 2", got)
	}
	tap(gamepad.ButtonSouth)
	if !g.confirmQuit.Get() {
		t.Fatal("the gamepad's confirm on Quit did not ask to confirm")
	}
	tap(gamepad.ButtonDPadRight)
	tap(gamepad.ButtonSouth)
	if g.confirmQuit.Get() || g.phase.Get() != phasePaused {
		t.Fatal("Cancel by gamepad did not return to the pause menu")
	}
	tap(gamepad.ButtonSouth)
	if g.phase.Get() != phaseArena {
		t.Fatalf("Resume by gamepad moved to %v, want the arena", g.phase.Get())
	}

	g.run.keeper.health = 0
	app.Advance(8 * frame)
	if !drewHUDPiece(app.Find("id:arena").Ops, g.hudArt, hudPanel) {
		t.Fatal("the canvas did not draw the results panel")
	}
	tap(gamepad.ButtonDPadRight)
	tap(gamepad.ButtonSouth)
	if g.phase.Get() != phaseTitle {
		t.Fatalf("Title by gamepad moved to %v, want the title", g.phase.Get())
	}
}
