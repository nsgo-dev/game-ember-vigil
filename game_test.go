package main

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nsgo-dev/game/packages/game/screen"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
	evstrings "me.oneo/embervigil/strings"
)

// TestMain gives the platform the main thread under nsgo test -target.
func TestMain(m *testing.M) { nsgotest.Main(m) }

// frame is one display frame, which at 60 Hz runs one fixed step.
const frame = 16667 * time.Microsecond

// startEmberVigil starts the shell and hands back its instance, so a test reads the phase and the run it steps.
func startEmberVigil(t *testing.T, opts ...nsgotest.Option) (*nsgotest.App, *emberVigil) {
	t.Helper()
	var g *emberVigil
	allOpts := append([]nsgotest.Option{nsgotest.Size(960, 540), nsgotest.ManualFrames()}, opts...)
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		g = newEmberVigil(cx)
		return g.view()
	}, allOpts...)
	app.Advance(3 * frame)
	if g == nil {
		t.Fatal("the shell was not created")
	}
	return app, g
}

// press presses the view the test names and runs a frame, so the shell reads the press.
func press(t *testing.T, app *nsgotest.App, selector string) {
	t.Helper()
	app.Press(selector)
	app.Advance(frame)
}

// burnFrames is how many frames a chosen card takes to burn away before the run applies it.
const burnFrames = int(burnTime*rate) + 2

// chooseCard presses a level-up card and runs frames until it has burnt away and the run took it.
func chooseCard(t *testing.T, app *nsgotest.App, selector string) {
	t.Helper()
	app.Press(selector)
	app.Advance(time.Duration(burnFrames) * frame)
}

// pauseCenter is the middle of the pause button the HUD draws on the arena canvas.
func pauseCenter(g *emberVigil) core.Point {
	x, y := layoutHUD(g.canvasSize(), g.cx.Env().SafeArea()).pauseHit.center()
	return core.Point{X: core.Points(x), Y: core.Points(y)}
}

// pressPause clicks the canvas pause button and runs a frame.
func pressPause(t *testing.T, app *nsgotest.App, g *emberVigil) {
	t.Helper()
	app.PressAt("id:arena-view", pauseCenter(g))
	app.Advance(frame)
}

// arenaLabel is what the arena canvas tells assistive technology, the HUD's values included.
func arenaLabel(app *nsgotest.App) string {
	return app.Find("id:arena").Props.Label
}

// wantLabel fails t unless the arena's label holds every part.
func wantLabel(t *testing.T, app *nsgotest.App, parts ...string) {
	t.Helper()
	label := arenaLabel(app)
	for _, part := range parts {
		if !strings.Contains(label, part) {
			t.Fatalf("the arena's label %q lacks %q", label, part)
		}
	}
}

func TestTitleArenaPauseAndQuitFlow(t *testing.T) {
	app, g := startEmberVigil(t)
	app.Find("id:title")

	press(t, app, "role:button Start")
	if g.phase.Get() != phaseArena {
		t.Fatalf("phase after Start = %v, want phaseArena", g.phase.Get())
	}
	if ops := app.Find("id:arena").Ops; len(ops) == 0 {
		t.Fatal("the arena canvas drew nothing")
	}
	if g.run == nil {
		t.Fatal("Start began no run")
	}

	before := g.run.Snapshot().Time
	app.Advance(10 * frame)
	if g.run.Snapshot().Time <= before {
		t.Fatalf("the run lasted %v seconds after ten frames, want more than %v", g.run.Snapshot().Time, before)
	}

	pressPause(t, app, g)
	app.Find("id:pause")
	app.Find("label:Paused")
	paused := g.run.Snapshot().Time
	app.Advance(20 * frame)
	if g.run.Snapshot().Time != paused {
		t.Fatalf("the run lasted %v seconds while paused, want %v", g.run.Snapshot().Time, paused)
	}

	press(t, app, "role:button Resume")
	if g.phase.Get() != phaseArena {
		t.Fatalf("phase after Resume = %v, want phaseArena", g.phase.Get())
	}
	app.Advance(10 * frame)
	if g.run.Snapshot().Time <= paused {
		t.Fatalf("the run lasted %v seconds after resuming, want more than %v", g.run.Snapshot().Time, paused)
	}

	pressPause(t, app, g)
	press(t, app, "id:pause-quit")
	app.Find("id:pause-confirm-quit")
	// Cancel keeps the run paused.
	press(t, app, "id:pause-confirm-quit-no")
	if g.phase.Get() != phasePaused {
		t.Fatalf("phase after Cancel = %v, want phasePaused", g.phase.Get())
	}
	if g.run == nil {
		t.Fatal("Cancel dropped the run")
	}
	// Confirm Quit ends the run and returns to title.
	press(t, app, "id:pause-quit")
	press(t, app, "id:pause-confirm-quit-yes")
	if g.phase.Get() != phaseTitle {
		t.Fatalf("phase after Confirm Quit = %v, want phaseTitle", g.phase.Get())
	}
	if g.run != nil {
		t.Fatal("the run survived Quit")
	}
	app.Find("id:title")
}

func TestEscapePausesAndResumesTheRun(t *testing.T) {
	app, g := startEmberVigil(t)
	press(t, app, "role:button Start")

	app.KeyDown("escape")
	app.Advance(2 * frame)
	app.KeyUp("escape")
	app.Advance(frame)
	if g.phase.Get() != phasePaused {
		t.Fatalf("phase after Escape = %v, want phasePaused", g.phase.Get())
	}

	app.KeyDown("escape")
	app.Advance(2 * frame)
	app.KeyUp("escape")
	app.Advance(frame)
	if g.phase.Get() != phaseArena {
		t.Fatalf("phase after the second Escape = %v, want phaseArena", g.phase.Get())
	}
}

func TestConfirmStartsARun(t *testing.T) {
	app, g := startEmberVigil(t)
	app.Find("id:title")

	app.KeyDown("enter")
	app.Advance(2 * frame)
	app.KeyUp("enter")
	app.Advance(time.Duration(burnFrames) * frame)
	if g.phase.Get() != phaseArena {
		t.Fatalf("phase after Enter = %v, want phaseArena", g.phase.Get())
	}
}

// TestTheArenaAxesMoveTheKeeper checks that the bound move_x and move_y actions reach the run's controls while the arena is on screen.
func TestTheArenaAxesMoveTheKeeper(t *testing.T) {
	app, g := startEmberVigil(t)
	press(t, app, "role:button Start")
	before := g.run.Snapshot().Keeper

	app.KeyCodeDown("KeyD")
	app.Advance(30 * frame)
	app.KeyCodeUp("KeyD")
	app.Advance(frame)

	after := g.run.Snapshot().Keeper
	if after.X <= before.X {
		t.Fatalf("the keeper stayed at x %v while D was held, want it to move right from %v", after.X, before.X)
	}
	if after.Y != before.Y {
		t.Fatalf("the keeper moved to y %v while D was held, want %v", after.Y, before.Y)
	}
}

// TestTheRunViewMatchesTheVirtualCanvas checks that the spawn ring and the camera count the same view, from the run and from game.yaml.
func TestTheRunViewMatchesTheVirtualCanvas(t *testing.T) {
	if viewWidth != virtualWidth || viewHeight != virtualHeight {
		t.Fatalf("the run assumes a %vx%v view, but game.yaml declares %vx%v", viewWidth, viewHeight, virtualWidth, virtualHeight)
	}
}

func TestSettingsPlaceholderReturnsToTheTitle(t *testing.T) {
	app, g := startEmberVigil(t)

	press(t, app, "role:button Settings")
	if g.phase.Get() != phaseSettings {
		t.Fatalf("phase after Settings = %v, want phaseSettings", g.phase.Get())
	}
	app.Find("id:settings")

	press(t, app, "role:button Back")
	if g.phase.Get() != phaseTitle {
		t.Fatalf("phase after Back = %v, want phaseTitle", g.phase.Get())
	}
	app.Find("id:title")
}

// TestTitleQuitQuitsTheApp checks that a desktop title quits the app from its Quit button.
func TestTitleQuitQuitsTheApp(t *testing.T) {
	app, _ := startEmberVigil(t, nsgotest.Idiom(core.IdiomDesktop))

	press(t, app, "role:button Quit")
	if app.Quits != 1 {
		t.Fatalf("the app was asked to quit %d times, want 1", app.Quits)
	}
}

// TestViewportFillsTheWindow checks that the window scales the 480x270 canvas without stretching or bars, showing more of the world along its longer side.
func TestViewportFillsTheWindow(t *testing.T) {
	if viewport.Width != 480 || viewport.Height != 270 {
		t.Fatalf("virtual canvas = %vx%v, want 480x270", viewport.Width, viewport.Height)
	}
	if viewport.Mode != screen.ModeExpand {
		t.Fatalf("viewport mode = %v, want screen.ModeExpand", viewport.Mode)
	}
	if !viewport.PixelSnap {
		t.Fatal("viewport does not snap pixels")
	}

	for _, window := range []core.Size{
		{Width: 960, Height: 540},
		{Width: 1200, Height: 540},
		{Width: 480, Height: 800},
		{Width: 360, Height: 240},
	} {
		place := viewport.Layout(window)
		if place.ScaleX != place.ScaleY {
			t.Fatalf("window %v scales x by %v and y by %v", window, place.ScaleX, place.ScaleY)
		}
		if place.VirtualSize.Width < 480-0.01 || place.VirtualSize.Height < 270-0.01 {
			t.Fatalf("window %v shows a %v virtual canvas, less than 480x270", window, place.VirtualSize)
		}
		if place.Rect.Size != window || len(place.Bars) > 0 {
			t.Fatalf("window %v holds a %v canvas with %d bars, want it filled", window, place.Rect, len(place.Bars))
		}
	}
}

// TestArenaSurvivesAResize checks that the app keeps drawing the arena when the window changes shape.
func TestArenaSurvivesAResize(t *testing.T) {
	app, _ := startEmberVigil(t)
	press(t, app, "role:button Start")

	for _, size := range []core.Size{{Width: 1200, Height: 540}, {Width: 480, Height: 800}, {Width: 960, Height: 540}} {
		app.Resize(size.Width, size.Height)
		app.Advance(2 * frame)
		if ops := app.Find("id:arena").Ops; len(ops) == 0 {
			t.Fatalf("the arena drew nothing at %v", size)
		}
	}
}

// TestButtonsAreTouchFriendly checks that every button on every screen takes at least buttonSide points and stays inside the window.
func TestButtonsAreTouchFriendly(t *testing.T) {
	const windowWidth, windowHeight = 960, 540
	app, g := startEmberVigil(t)

	check := func(screen string) {
		t.Helper()
		buttons := app.Query("role:button")
		if len(buttons) == 0 {
			t.Fatalf("the %s screen shows no button", screen)
		}
		for _, b := range buttons {
			bounds := b.Bounds()
			if bounds.Size.Width < buttonSide || bounds.Size.Height < buttonSide {
				t.Fatalf("a %s screen button is %v points, smaller than %v", screen, bounds.Size, buttonSide)
			}
			if bounds.Origin.X < 0 || bounds.Origin.Y < 0 ||
				bounds.Origin.X+bounds.Size.Width > windowWidth || bounds.Origin.Y+bounds.Size.Height > windowHeight {
				t.Fatalf("a %s screen button lies outside the window: %v", screen, bounds)
			}
		}
	}

	check("title")
	press(t, app, "role:button Settings")
	check("settings")
	press(t, app, "role:button Back")
	press(t, app, "role:button Start")
	// The arena has no native button: its pause is drawn on the canvas, which TestHUDLayoutStaysInsideTheSafeAreaAndOffTheStick sizes.
	pressPause(t, app, g)
	check("pause")
	press(t, app, "id:pause-quit")
	check("pause-confirm-quit")
	press(t, app, "id:pause-confirm-quit-no")
	press(t, app, "id:pause-resume")
	g.run.xp = g.run.xpNeeded()
	app.Advance(2 * frame)
	check("level-up")
}

// TestTheHUDFollowsTheRun checks that the canvas HUD shows the run's clock, health, level, kills and starter weapon, keeps up with them and tells assistive technology.
func TestTheHUDFollowsTheRun(t *testing.T) {
	app, g := startEmberVigil(t)
	press(t, app, "role:button Start")

	st := g.hud.Get()
	if st.time != "00:00" || st.level != 1 || st.kills != 0 || st.health != st.maxHealth || st.maxHealth <= 0 {
		t.Fatalf("the HUD starts at %+v, want 00:00, level 1, no kills and full health", st)
	}
	if got := st.weapons[0]; got.id != "ember-bolt" || got.level != 1 || got.maxLevel <= 1 {
		t.Fatalf("the HUD's first slot is %+v, want Ember Bolt at level 1 of several", got)
	}
	wantLabel(t, app, "00:00", "Level 1", "Kills 0", "Health 100/100")
	if !drewHUDPiece(app.Find("id:arena").Ops, g.hudArt, hudHealthFrame) {
		t.Fatal("the arena canvas did not draw the HUD's lantern")
	}

	app.Advance(70 * frame)
	want := clockText(g.run.Snapshot().Time)
	if got := g.hud.Get().time; got != want || got == "00:00" {
		t.Fatalf("the HUD's clock reads %q after a second, want %q", got, want)
	}
	wantLabel(t, app, want)
}

// TestALevelUpCardIsChosenFromThePrompt checks that a level-up shows three rich cards (name, description, pips), pauses the run, and resumes after a tap.
func TestALevelUpCardIsChosenFromThePrompt(t *testing.T) {
	app, g := startEmberVigil(t)
	press(t, app, "role:button Start")

	// Exactly the XP for the next level, which the run spends on three cards on its next step.
	g.run.xp = g.run.xpNeeded()
	app.Advance(2 * frame)
	if p := g.run.Snapshot().Phase; p != PhaseLevelUp {
		t.Fatalf("the run's phase with a level's XP is %v, want PhaseLevelUp", p)
	}
	app.Find("id:level-up")
	offers := g.run.Snapshot().Offers
	if len(offers) != offerCount {
		t.Fatalf("the level-up offered %d cards, want %d", len(offers), offerCount)
	}
	for i, o := range offers {
		app.Find("role:button " + o.Name)
		card := app.Find("id:level-up-card-" + string(rune('0'+i)))
		if card.Props.Label != o.Name || card.Props.Hint != o.Description {
			t.Fatalf("card %d reads %q / %q, want %q / %q", i, card.Props.Label, card.Props.Hint, o.Name, o.Description)
		}
		if img, ok := g.art.cardIcon(o.ID); ok {
			app.Advance(30 * frame)
			if !drewImage(app.Find("id:arena").Ops, img) {
				t.Fatalf("the canvas did not draw card %d's icon", i)
			}
		}
	}
	if !drewHUDPiece(app.Find("id:arena").Ops, g.hudArt, hudCardFocused) || !drewHUDPiece(app.Find("id:arena").Ops, g.hudArt, hudCardCommon) {
		t.Fatal("the canvas did not draw the focused and the other card frames")
	}

	// Run stays paused while the modal is open.
	pausedAt := g.run.Snapshot().Time
	app.Advance(10 * frame)
	if g.run.Snapshot().Time != pausedAt {
		t.Fatalf("the run advanced from %v to %v while level-up cards were open", pausedAt, g.run.Snapshot().Time)
	}

	app.Press("role:button " + offers[0].Name)
	app.Advance(frame)
	if p := g.run.Snapshot().Phase; p != PhaseLevelUp || !g.overlay.burning {
		t.Fatalf("the run's phase while the chosen card burns is %v, want PhaseLevelUp", p)
	}
	app.Advance(time.Duration(burnFrames) * frame)
	if p := g.run.Snapshot().Phase; p != PhasePlaying {
		t.Fatalf("the run's phase after choosing a card is %v, want PhasePlaying", p)
	}
	if lvl := g.run.Snapshot().Level; lvl != 2 {
		t.Fatalf("the run's level after its first card is %d, want 2", lvl)
	}
	if app.Exists("id:level-up") {
		t.Fatal("the level-up prompt stayed after a card was chosen")
	}
	before := g.run.Snapshot().Time
	app.Advance(10 * frame)
	if g.run.Snapshot().Time <= before {
		t.Fatalf("the run lasted %v seconds after the card, want more than %v", g.run.Snapshot().Time, before)
	}
}

// TestLevelUpCardChosenWithNumberKeyAndConfirm checks choosing a level-up card with 1/2/3 keys and with directional focus + confirm.
func TestLevelUpCardChosenWithNumberKeyAndConfirm(t *testing.T) {
	app, g := startEmberVigil(t)
	press(t, app, "role:button Start")

	// Choose the second card using Digit2.
	g.run.xp = g.run.xpNeeded()
	app.Advance(2 * frame)
	app.Find("id:level-up")
	app.KeyCodeDown("Digit2")
	app.Advance(2 * frame)
	app.KeyCodeUp("Digit2")
	app.Advance(time.Duration(burnFrames) * frame)
	if p := g.run.Snapshot().Phase; p != PhasePlaying {
		t.Fatalf("phase after pressing 2 = %v, want PhasePlaying", p)
	}
	if app.Exists("id:level-up") {
		t.Fatal("level-up modal stayed open after pressing 2")
	}

	// Open another level-up, move focus right and press Enter to confirm.
	g.run.xp = g.run.xpNeeded()
	app.Advance(2 * frame)
	app.Find("id:level-up")
	app.KeyCodeDown("KeyD")
	app.Advance(2 * frame)
	app.KeyCodeUp("KeyD")
	app.Advance(frame)
	if g.cardFocus != 1 {
		t.Fatalf("cardFocus after KeyD = %d, want 1", g.cardFocus)
	}
	app.KeyDown("enter")
	app.Advance(2 * frame)
	app.KeyUp("enter")
	app.Advance(time.Duration(burnFrames) * frame)
	if p := g.run.Snapshot().Phase; p != PhasePlaying {
		t.Fatalf("phase after Enter = %v, want PhasePlaying", p)
	}
}

// TestResultsScreenForDefeatAndVictory checks the results screen on defeat and victory, boss HUD bar, Play Again and Title actions.
func TestResultsScreenForDefeatAndVictory(t *testing.T) {
	app, g := startEmberVigil(t)
	press(t, app, "role:button Start")

	first := g.run
	g.run.keeper.health = 0
	app.Advance(8 * frame)
	if p := g.run.Snapshot().Phase; p != PhaseDefeat {
		t.Fatalf("the run's phase with no health is %v, want PhaseDefeat", p)
	}
	app.Find("id:results")
	if got := app.Find("id:results-outcome").Props.Label; got != "DEFEAT  ·  Long Night 1" {
		t.Fatalf("defeat outcome = %q, want DEFEAT at Long Night 1", got)
	}
	app.Find("id:results-time")
	app.Find("id:results-kills")
	app.Find("id:results-level")
	app.Find("id:results-record")

	press(t, app, "role:button Play Again")
	if g.run == first {
		t.Fatal("Play Again kept the finished run")
	}
	if p := g.run.Snapshot().Phase; p != PhasePlaying {
		t.Fatalf("the retried run's phase is %v, want PhasePlaying", p)
	}
	if app.Exists("id:results") {
		t.Fatal("the results screen stayed after Play Again")
	}
	wantLabel(t, app, "00:00")

	// Spawn Hollow King and confirm boss health bar appears in the HUD.
	g.run.spawnBoss()
	app.Advance(8 * frame)
	if st := g.hud.Get(); !st.bossActive || st.bossElite || st.bossName != "Hollow King" || st.bossMaxHealth <= 0 {
		t.Fatalf("boss HUD = %+v, want the Hollow King's bar", st)
	}
	wantLabel(t, app, "Hollow King · Phase I")
	if !drewHUDPiece(app.Find("id:arena").Ops, g.hudArt, hudBossFrame) {
		t.Fatal("the arena canvas did not draw the boss bar")
	}

	// Defeat the Hollow King and confirm Victory results screen and Title action.
	if be := g.run.bossEnemy(); be != nil {
		be.hp = 0
	}
	app.Advance(8 * frame)
	if p := g.run.Snapshot().Phase; p != PhaseVictory {
		t.Fatalf("phase after killing boss = %v, want PhaseVictory", p)
	}
	app.Find("id:results")
	if got := app.Find("id:results-outcome").Props.Label; got != "VICTORY  ·  Long Night 1" {
		t.Fatalf("victory outcome = %q, want VICTORY at Long Night 1", got)
	}

	press(t, app, "role:button Title")
	if g.phase.Get() != phaseTitle {
		t.Fatalf("the phase after Title is %v, want phaseTitle", g.phase.Get())
	}
	if g.run != nil {
		t.Fatal("the run survived Title")
	}
	app.Find("id:title")
}

// TestTheArenaCanvasDrawsTheWorld checks that the shell's canvas draws the run's ground and its sprites.
func TestTheArenaCanvasDrawsTheWorld(t *testing.T) {
	app, g := startEmberVigil(t)
	press(t, app, "role:button Start")

	g.run.addEnemy(0, g.run.keeper.pos.Add(Vec2{X: 40}), 1, 1)
	app.Advance(2 * frame)
	ops := app.Find("id:arena").Ops
	if sprites := countSprites(ops); sprites < 2 {
		t.Fatalf("the canvas drew %d sprites, want at least the keeper and its enemy", sprites)
	}
	ground := false
	for _, op := range ops {
		if op.Kind == core.DrawImage && op.Image != nil && isGround(op.Image) {
			ground = true
		}
	}
	if !ground {
		t.Fatal("the canvas drew no ground")
	}
}

// TestTitleArtAndFallback checks that the title shows the animated cover from the kit, and the still procedural cover when a layer is missing.
func TestTitleArtAndFallback(t *testing.T) {
	app, g := startEmberVigil(t)
	if !g.titleArt.animated() {
		t.Fatal("the embedded title kit does not animate the cover")
	}
	app.Find("id:title-art")
	app.Find("id:title-records")

	appFallback := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		g := newEmberVigil(cx)
		g.titleArt = newTitleArt(nil)
		return g.view()
	}, nsgotest.Size(960, 540), nsgotest.ManualFrames())
	appFallback.Advance(3 * frame)
	appFallback.Find("id:title-art-fallback")
	press(t, appFallback, "id:title-start")
}

// TestTitleHidesQuitOnPhones checks that a phone's title offers only Start, Shop, Collection and Settings, and that the keyboard's focus wraps over those four.
func TestTitleHidesQuitOnPhones(t *testing.T) {
	app, g := startEmberVigil(t, nsgotest.Idiom(core.IdiomPhone))
	app.Find("id:title-start")
	app.Find("id:title-shop")
	app.Find("id:title-collection")
	app.Find("id:title-settings")
	if n := len(app.Query("id:title-quit")); n != 0 {
		t.Fatalf("a phone's title shows %d Quit buttons, want none", n)
	}
	for range 4 {
		g.moveFocus(1, len(g.titleItems(g.tr())))
	}
	if f := g.menuFocus.Get(); f != 0 {
		t.Fatalf("focus after four steps over four buttons = %d, want 0", f)
	}
	g.confirm()
	if g.phase.Get() != phaseArena {
		t.Fatalf("confirming the first button gave phase %v, want phaseArena", g.phase.Get())
	}
	for _, c := range []struct {
		idiom core.Idiom
		goos  string
		want  bool
	}{
		{core.IdiomDesktop, "darwin", true},
		{core.IdiomPhone, "android", false},
		{core.IdiomTablet, "ios", false},
		{core.IdiomDesktop, "js", false},
	} {
		if got := quitShown(c.idiom, c.goos); got != c.want {
			t.Errorf("quitShown(%v, %q) = %v, want %v", c.idiom, c.goos, got, c.want)
		}
	}
}

// TestTitleLayoutKeepsTheMenuAtTheBottomCentre checks the menu sits centred along the bottom of the safe area at desktop and phone sizes, with every button a thumb's size.
func TestTitleLayoutKeepsTheMenuAtTheBottomCentre(t *testing.T) {
	for _, c := range []struct {
		size core.Size
		safe core.Edges
		n    int
	}{
		{core.Size{Width: 1280, Height: 720}, core.Edges{}, 3},
		{core.Size{Width: 844, Height: 390}, core.Edges{Left: 47, Right: 47, Bottom: 21}, 2},
		{core.Size{Width: 390, Height: 844}, core.Edges{Top: 47, Bottom: 34}, 2},
	} {
		l := layoutTitle(c.size, c.safe, c.n)
		first, last := l.buttons[0], l.buttons[len(l.buttons)-1]
		left, right := first.x-float32(c.safe.Left), float32(c.size.Width-c.safe.Right)-(last.x+last.w)
		if d := left - right; d > 1 || d < -1 {
			t.Errorf("%v: the menu is off centre by %v", c.size, d)
		}
		if bottom := float32(c.size.Height - c.safe.Bottom); last.y+last.h > bottom || last.y+last.h < bottom-overlayMargin-1 {
			t.Errorf("%v: the menu ends at %v, not at the safe area's bottom %v", c.size, last.y+last.h, bottom)
		}
		for _, b := range l.buttons {
			if b.h < buttonSide || b.w < buttonSide {
				t.Errorf("%v: a button is %vx%v", c.size, b.w, b.h)
			}
		}
		if l.recordsHeading.y < l.logo.y+l.logo.h {
			t.Errorf("%v: the records overlap the wordmark", c.size)
		}
	}
}

// TestSettingsPersistAndReload checks that edited settings apply immediately, persist to core.Storage and reload on restart.
func TestSettingsPersistAndReload(t *testing.T) {
	app, g := startEmberVigil(t)
	press(t, app, "id:title-settings")
	app.Find("id:settings")

	press(t, app, "id:settings-master-dec")
	press(t, app, "id:settings-music-inc")
	press(t, app, "id:settings-sfx-dec")
	press(t, app, "id:settings-shake")
	if g.scene.shakeEnabled {
		t.Fatal("toggling screen shake off did not immediately disable renderer shake")
	}
	press(t, app, "id:settings-quality-high")
	press(t, app, "id:settings-lang-zh-CN")
	if got := app.Find("id:settings-heading").Props.Label; got != "设置" {
		t.Fatalf("settings heading after switching to zh-CN = %q, want 设置", got)
	}
	press(t, app, "id:settings-apply")
	app.Find("id:title")
	if got := app.Find("id:title-heading").Props.Label; got != "余烬守望" {
		t.Fatalf("title heading after applying zh-CN = %q, want 余烬守望", got)
	}

	persisted := storageSnapshot(app.Storage())
	app2, g2 := startEmberVigil(t, nsgotest.Storage(persisted))
	got := g2.settings.Get()
	if got.MasterVolume != 0.7 || got.MusicVolume != 0.8 || got.SFXVolume != 0.7 {
		t.Fatalf("reloaded volumes = (%v, %v, %v), want (0.7, 0.8, 0.7)", got.MasterVolume, got.MusicVolume, got.SFXVolume)
	}
	if got.ScreenShake {
		t.Fatal("reloaded ScreenShake = true, want false")
	}
	if g2.scene.shakeEnabled {
		t.Fatal("reloaded renderer shakeEnabled = true, want false")
	}
	if got.Quality != QualityHigh {
		t.Fatalf("reloaded Quality = %q, want %q", got.Quality, QualityHigh)
	}
	if got.Language != evstrings.ZhCN {
		t.Fatalf("reloaded Language = %q, want %q", got.Language, evstrings.ZhCN)
	}
	if heading := app2.Find("id:title-heading").Props.Label; heading != "余烬守望" {
		t.Fatalf("reloaded title heading = %q, want 余烬守望", heading)
	}
}

// TestSettingsBackDiscardsUnsavedEdits checks that Back reverts unsaved edits and leaves persisted settings untouched.
func TestSettingsBackDiscardsUnsavedEdits(t *testing.T) {
	app, g := startEmberVigil(t)
	press(t, app, "id:title-settings")
	press(t, app, "id:settings-shake")
	press(t, app, "id:settings-lang-zh-CN")
	press(t, app, "id:settings-back")

	if g.phase.Get() != phaseTitle {
		t.Fatalf("phase after Back = %v, want phaseTitle", g.phase.Get())
	}
	if !g.settings.Get().ScreenShake || !g.scene.shakeEnabled {
		t.Fatal("Back did not restore ScreenShake=true")
	}
	if g.settings.Get().Language != evstrings.EN {
		t.Fatalf("Back did not restore Language=en, got %q", g.settings.Get().Language)
	}
}

// TestPauseSettingsRoundTripKeepsClockFrozen checks that opening Settings from Pause and returning keeps the run paused and its clock frozen.
func TestPauseSettingsRoundTripKeepsClockFrozen(t *testing.T) {
	app, g := startEmberVigil(t)
	press(t, app, "id:title-start")
	app.Advance(10 * frame)

	pressPause(t, app, g)
	if g.phase.Get() != phasePaused {
		t.Fatalf("phase after Pause = %v, want phasePaused", g.phase.Get())
	}
	frozenAt := g.run.Snapshot().Time

	press(t, app, "id:pause-settings")
	if g.phase.Get() != phaseSettings {
		t.Fatalf("phase after pause Settings = %v, want phaseSettings", g.phase.Get())
	}
	app.Advance(20 * frame)
	if g.run.Snapshot().Time != frozenAt {
		t.Fatalf("run clock advanced while in Settings from %v to %v", frozenAt, g.run.Snapshot().Time)
	}

	press(t, app, "id:settings-shake")
	press(t, app, "id:settings-apply")
	if g.phase.Get() != phasePaused {
		t.Fatalf("phase after Apply from pause Settings = %v, want phasePaused", g.phase.Get())
	}
	app.Advance(10 * frame)
	if g.run.Snapshot().Time != frozenAt {
		t.Fatalf("run clock advanced after returning to Pause from %v to %v", frozenAt, g.run.Snapshot().Time)
	}
	if g.scene.shakeEnabled {
		t.Fatal("shake setting applied from Pause did not stick")
	}
}

// TestLanguageSwitchUpdatesAllScreens checks that switching to Simplified Chinese translates title, settings, HUD, boss bar, level-up cards, pause, quit confirmation and results.
func TestLanguageSwitchUpdatesAllScreens(t *testing.T) {
	app, g := startEmberVigil(t, nsgotest.Idiom(core.IdiomDesktop))
	press(t, app, "id:title-settings")
	press(t, app, "id:settings-lang-zh-CN")
	press(t, app, "id:settings-apply")

	// Title screen
	if got := app.Find("id:title-heading").Props.Label; got != "余烬守望" {
		t.Fatalf("zh-CN title = %q, want 余烬守望", got)
	}
	app.Find("role:button 开始")
	app.Find("role:button 设置")
	app.Find("role:button 退出")

	// Arena HUD + Boss bar
	press(t, app, "id:title-start")
	wantLabel(t, app, "余烬守望竞技场", "等级 1", "击杀 0", "生命 100/100")

	// Pause + quit confirmation
	pressPause(t, app, g)
	app.Find("label:已暂停")
	app.Find("role:button 继续")
	press(t, app, "id:pause-quit")
	app.Find("label:确定返回标题？")
	press(t, app, "id:pause-confirm-quit-no")
	press(t, app, "id:pause-resume")

	// Level-up cards
	g.run.xp = g.run.xpNeeded()
	app.Advance(2 * frame)
	offers := g.run.Snapshot().Offers
	zh := evstrings.For(evstrings.ZhCN)
	for i, o := range offers {
		want := zh.Card(o.ID, o.Name, o.Description)
		card := app.Find("id:level-up-card-" + string(rune('0'+i)))
		if card.Props.Label != want.Name || card.Props.Hint != want.Description {
			t.Fatalf("zh-CN card %d reads %q / %q, want %q / %q", i, card.Props.Label, card.Props.Hint, want.Name, want.Description)
		}
	}
	chooseCard(t, app, "id:level-up-card-0")

	// Boss HUD + Victory results
	g.run.spawnBoss()
	app.Advance(8 * frame)
	wantLabel(t, app, "空洞之王 · 第一阶段")
	if be := g.run.bossEnemy(); be != nil {
		be.hp = 0
	}
	app.Advance(8 * frame)
	if got := app.Find("id:results-outcome").Props.Label; got != "胜利  ·  长夜 1" {
		t.Fatalf("zh-CN victory outcome = %q, want 胜利 at 长夜 1", got)
	}
	if got := app.Find("id:results-record").Props.Label; got != "刷新记录！" {
		t.Fatalf("zh-CN new record = %q, want 刷新记录！", got)
	}
}

// TestRecordsUpdateOncePerRunAndPersist checks that each finished run updates saved records once, persists across restart, and distinguishes new vs non-new records.
func TestRecordsUpdateOncePerRunAndPersist(t *testing.T) {
	app, g := startEmberVigil(t)
	press(t, app, "id:title-start")
	g.run.spawnBoss()
	g.run.kills = 25
	g.run.level = 5
	app.Advance(60 * frame)
	if be := g.run.bossEnemy(); be != nil {
		be.hp = 0
	}
	app.Advance(8 * frame)
	if got := app.Find("id:results-record").Props.Label; got != "New record" {
		t.Fatalf("first run record label = %q, want New record", got)
	}

	en := evstrings.For(evstrings.EN)
	won := g.run.Payout()
	if won < 50 {
		t.Fatalf("a won run pays %d embers, want at least the 50 for a win", won)
	}
	if got, want := app.Find("id:results-embers").Props.Label, en.EmbersEarned(won)+"  ·  "+en.EmberBalance(won); got != want {
		t.Fatalf("results embers = %q, want %q", got, want)
	}

	// Extra frames and repeated recordResult calls must not double-count wins or embers.
	app.Advance(20 * frame)
	g.recordResult(g.run.Snapshot())
	if wins := g.records.Get().Wins; wins != 1 {
		t.Fatalf("wins after one victory = %d, want 1", wins)
	}
	if g.progress.Embers != won || g.progress.TierWon != 1 {
		t.Fatalf("progression after one victory = %+v, want %d embers and tier 1 won", g.progress, won)
	}

	// Restart app from persisted storage and verify title shows saved records.
	wantKills := g.run.Snapshot().Kills
	persisted := storageSnapshot(app.Storage())
	app2, g2 := startEmberVigil(t, nsgotest.Storage(persisted))
	if got := app2.Find("id:title-record-wins").Props.Label; got != "Wins 1" {
		t.Fatalf("reloaded title wins = %q, want Wins 1", got)
	}
	if got := app2.Find("id:title-record-kills").Props.Label; got != evstrings.For(evstrings.EN).BestKills(wantKills) {
		t.Fatalf("reloaded title kills = %q, want %q", got, evstrings.For(evstrings.EN).BestKills(wantKills))
	}

	// A worse second run must show "No new record" and keep existing records.
	press(t, app2, "id:title-start")
	g2.run.keeper.health = 0
	app2.Advance(8 * frame)
	if got := app2.Find("id:results-record").Props.Label; got != "No new record" {
		t.Fatalf("worse run record label = %q, want No new record", got)
	}
	if rec := g2.records.Get(); rec.BestKills != wantKills || rec.BestLevel != 5 || rec.Wins != 1 {
		t.Fatalf("worse run altered records: %+v", rec)
	}

	// The balance survived the restart, and the lost run added its own payout to it.
	lost := g2.run.Payout()
	if g2.progress.Embers != won+lost || g2.progress.TierWon != 1 {
		t.Fatalf("progression after a restart and a lost run = %+v, want %d embers and tier 1 won", g2.progress, won+lost)
	}
	if got, want := app2.Find("id:results-embers").Props.Label, en.EmbersEarned(lost)+"  ·  "+en.EmberBalance(won+lost); got != want {
		t.Fatalf("lost run results embers = %q, want %q", got, want)
	}
	if got := loadProgression(app2.Storage()); got.Embers != won+lost {
		t.Fatalf("saved balance = %d, want %d", got.Embers, won+lost)
	}
}

// TestScreenShakeToggleStopsRendererShake checks that disabling screen shake clears shake immediately and ignores subsequent shake events.
func TestScreenShakeToggleStopsRendererShake(t *testing.T) {
	r := newArenaRenderer(newArt())
	r.stepEffects([]Event{{Kind: EventDefeat}}, false, 0.016)
	if r.fx.shake <= 0 {
		t.Fatalf("shake after EventDefeat with shake enabled = %v, want > 0", r.fx.shake)
	}
	r.setShakeEnabled(false)
	if r.fx.shake != 0 || r.fx.shakeX != 0 || r.fx.shakeY != 0 {
		t.Fatalf("setShakeEnabled(false) did not clear shake: %+v", r.fx)
	}
	r.stepEffects([]Event{{Kind: EventDefeat}, {Kind: EventBossSpawn}}, false, 0.016)
	if r.fx.shake != 0 || r.fx.shakeX != 0 || r.fx.shakeY != 0 {
		t.Fatalf("shake occurred while disabled: %+v", r.fx)
	}
}

// TestVirtualStickMovesKeeperAndRespectsSafeArea checks that the bottom-left virtual stick stays clear of phone safe areas, renders feedback and drives move_x/move_y.
func TestVirtualStickMovesKeeperAndRespectsSafeArea(t *testing.T) {
	const width, height = 844, 390
	safe := core.Edges{Top: 12, Right: 44, Bottom: 21, Left: 44}
	env := core.Environment{
		Idiom:    core.IdiomPhone,
		Pointer:  core.PointerCoarse,
		SafeArea: safe,
	}
	app, g := startEmberVigil(t, nsgotest.Size(width, height), nsgotest.Environment(env))
	press(t, app, "id:title-start")

	if !g.showTouchStick() {
		t.Fatal("showTouchStick() = false on a phone with coarse pointer, want true")
	}
	base := g.stick.BasePosition()
	stickR := core.Points(g.stick.Radius)
	if base.X-stickR < safe.Left || base.Y+stickR > height-safe.Bottom {
		t.Fatalf("stick resting circle (center=%v, r=%v) intrudes into safe area %+v", base, stickR, safe)
	}

	before := g.run.Snapshot().Keeper
	finger := app.Touch("id:arena-view", base)
	app.Advance(2 * frame)
	if !g.stick.Active() {
		t.Fatal("virtual stick did not activate on touch inside its bottom-left zone")
	}

	finger.Move(core.Point{X: base.X + stickR, Y: base.Y - stickR*0.5})
	app.Advance(30 * frame)

	if ax := g.actions.Axis(axisMoveX); ax <= 0.5 {
		t.Fatalf("move_x while dragging stick right = %v, want > 0.5", ax)
	}
	if ay := g.actions.Axis(axisMoveY); ay >= -0.2 {
		t.Fatalf("move_y while dragging stick up = %v, want < -0.2", ay)
	}
	after := g.run.Snapshot().Keeper
	if after.X <= before.X || after.Y >= before.Y {
		t.Fatalf("keeper moved from %+v to %+v while dragging stick up-right", before, after)
	}

	// Verify the virtual stick drew its base and knob on the arena canvas.
	ops := app.Find("id:arena").Ops
	fillCount := 0
	for _, op := range ops {
		if op.Kind == core.DrawFill {
			fillCount++
		}
	}
	if fillCount < 2 {
		t.Fatalf("arena canvas drew %d fill ops with active stick, want at least 2 for stick base and knob", fillCount)
	}

	finger.Lift()
	app.Advance(3 * frame)
	if g.stick.Active() {
		t.Fatal("virtual stick remained active after finger lifted")
	}
	if ax, ay := g.actions.Axis(axisMoveX), g.actions.Axis(axisMoveY); ax != 0 || ay != 0 {
		t.Fatalf("move axes after releasing stick = (%v, %v), want (0, 0)", ax, ay)
	}
}

// TestTouchLayoutAndSafeAreaOnPhoneLandscape checks that HUD pause, level-up cards and menu buttons stay >= 44pt and clear of phone safe areas.
func TestTouchLayoutAndSafeAreaOnPhoneLandscape(t *testing.T) {
	const width, height = 844, 390
	safe := core.Edges{Top: 12, Right: 44, Bottom: 21, Left: 44}
	env := core.Environment{
		Idiom:    core.IdiomPhone,
		Pointer:  core.PointerCoarse,
		SafeArea: safe,
	}
	app, g := startEmberVigil(t, nsgotest.Size(width, height), nsgotest.Environment(env))

	checkSafe := func(screen string) {
		t.Helper()
		buttons := app.Query("role:button")
		if len(buttons) == 0 {
			t.Fatalf("the %s screen shows no button", screen)
		}
		for _, b := range buttons {
			bounds := b.Bounds()
			if bounds.Size.Width < buttonSide || bounds.Size.Height < buttonSide {
				t.Fatalf("%s button %q is %v, smaller than %vpt", screen, b.Text, bounds.Size, buttonSide)
			}
			if bounds.Origin.X < safe.Left-0.5 || bounds.Origin.Y < safe.Top-0.5 ||
				bounds.Origin.X+bounds.Size.Width > width-safe.Right+0.5 ||
				bounds.Origin.Y+bounds.Size.Height > height-safe.Bottom+0.5 {
				t.Fatalf("%s button %q bounds %v intrude outside phone safe area %+v on %vx%v", screen, b.Text, bounds, safe, width, height)
			}
		}
	}

	checkSafe("title")
	press(t, app, "id:title-start")

	// The canvas pause button sits top-right inside the safe area, takes a thumb, and stays off the stick's zone.
	l := layoutHUD(core.Size{Width: width, Height: height}, safe)
	hit := l.pauseHit
	if hit.x < width/2 || hit.w < buttonSide || hit.h < buttonSide ||
		hit.x+hit.w > width-float32(safe.Right) || hit.y < float32(safe.Top) {
		t.Fatalf("pause hit area %+v, want >= %vpt in the top-right of the safe area %+v", hit, buttonSide, safe)
	}
	if zone := g.stick.Zone; float32(zone.Origin.Y) < l.bottom {
		t.Fatalf("stick zone %+v starts above the HUD's bottom %v", zone, l.bottom)
	}
	finger := app.Touch("id:arena-view", pauseCenter(g))
	app.Advance(frame)
	if g.stick.Active() {
		t.Fatal("a touch on the pause button woke the stick")
	}
	finger.Lift()
	app.Advance(frame)
	if g.phase.Get() != phasePaused {
		t.Fatalf("phase after tapping the canvas pause = %v, want phasePaused", g.phase.Get())
	}
	checkSafe("pause")
	press(t, app, "id:pause-quit")
	checkSafe("pause-confirm-quit")
	press(t, app, "id:pause-confirm-quit-no")
	press(t, app, "id:pause-resume")

	// Trigger level-up and check all three cards fit within the safe area and respond to tap.
	g.run.xp = g.run.xpNeeded()
	app.Advance(2 * frame)
	checkSafe("level-up")
	chooseCard(t, app, "id:level-up-card-0")
	if g.run.Snapshot().Phase != PhasePlaying {
		t.Fatalf("phase after tapping level-up card on phone = %v, want PhasePlaying", g.run.Snapshot().Phase)
	}

	// Trigger defeat and check results buttons stay inside safe area.
	g.run.keeper.health = 0
	app.Advance(8 * frame)
	checkSafe("results")
}

// TestLaterWeaponsUnlockAndAreOfferedAfterARestart checks that a run of 5000 kills and seven Gloomwardens unlocks Spark Chain and Ash Censer, names them on the results and saves them, and that runs after a restart offer them.
func TestLaterWeaponsUnlockAndAreOfferedAfterARestart(t *testing.T) {
	app, g := startEmberVigil(t)
	press(t, app, "id:title-start")
	if ids := offeredIDs(g.run); slices.Contains(ids, unlockSparkChain) || slices.Contains(ids, unlockAshCenser) {
		t.Fatalf("a first run offers %v", ids)
	}
	g.run.kills, g.run.elites = 5000, 7
	g.run.keeper.health = 0
	app.Advance(8 * frame)
	if !g.progress.has(unlockSparkChain) || !g.progress.has(unlockAshCenser) {
		t.Fatalf("the run did not unlock both weapons: %+v", g.progress)
	}
	en := evstrings.For(evstrings.EN)
	if got, want := app.Find("id:results-unlocked").Props.Label, en.UnlockedList([]string{"Spark Chain", "Ash Censer"}); got != want {
		t.Fatalf("results unlocked = %q, want %q", got, want)
	}

	app2, g2 := startEmberVigil(t, nsgotest.Storage(storageSnapshot(app.Storage())))
	press(t, app2, "id:title-start")
	if ids := offeredIDs(g2.run); !slices.Contains(ids, unlockSparkChain) || !slices.Contains(ids, unlockAshCenser) {
		t.Fatalf("a run after a restart offers %v", ids)
	}
}

// TestEvolutionUnlocksByAWinAndTheCollectionPersists checks that evolution waits for the first win, that a chest then evolves a met recipe, shows it in the HUD and saves it to the collection at once, and that the collection lists it, and the rest as "???" with a hint, after a restart.
func TestEvolutionUnlocksByAWinAndTheCollectionPersists(t *testing.T) {
	app, g := startEmberVigil(t)
	press(t, app, "id:title-start")
	if g.run.evolution {
		t.Fatal("a run before the first win may evolve weapons")
	}
	g.run.spawnBoss()
	app.Advance(frame)
	g.run.bossEnemy().hp = 0
	app.Advance(8 * frame)
	if !g.progress.has(unlockEvolution) {
		t.Fatalf("the first win did not unlock evolution: %+v", g.progress)
	}
	en := evstrings.For(evstrings.EN)
	if got, want := app.Find("id:results-unlocked").Props.Label, en.UnlockedList([]string{"Long Night 2", en.Unlocks[unlockEvolution]}); got != want {
		t.Fatalf("results unlocked = %q, want %q", got, want)
	}

	press(t, app, "id:results-play-again")
	r := g.run
	if !r.evolution {
		t.Fatal("a run after the first win may not evolve weapons")
	}
	bolt := weaponByName(t, r, "Ember Bolt")
	r.weaponLevels[bolt] = len(r.data.weapons.Weapons[bolt].Levels)
	r.passiveLevels[r.data.evolutionPassive[bolt]] = 1
	r.recomputeStats()
	putChest(r, Vec2{})
	app.Advance(frame)
	if !slices.Equal(g.progress.Collection, []string{"starfall-volley"}) || !slices.Equal(loadProgression(app.Storage()).Collection, []string{"starfall-volley"}) {
		t.Fatalf("evolving did not save the recipe at once: %v", g.progress.Collection)
	}
	app.Advance(time.Duration(r.data.run.Chest.EvolveHold*rate+2) * frame)
	if p := r.Snapshot().Phase; p != PhasePlaying {
		t.Fatalf("after the evolving chest's hold the run is in phase %v, want playing", p)
	}
	if st := g.hud.Get(); st.weapons[0].id != "starfall-volley" || !st.weapons[0].evolved {
		t.Fatalf("the HUD slot shows %+v, want the evolved Starfall Volley", st.weapons[0])
	}

	app2, g2 := startEmberVigil(t, nsgotest.Storage(storageSnapshot(app.Storage())))
	press(t, app2, "id:title-collection")
	if g2.phase.Get() != phaseCollection {
		t.Fatalf("Collection opened phase %v", g2.phase.Get())
	}
	app2.Find("id:collection-heading")
	if got, want := app2.Find("id:collection-made").Props.Label, en.CollectionMade(1, len(recipes(mustGameData()))); got != want {
		t.Fatalf("collection count = %q, want %q", got, want)
	}
	if got := app2.Find("id:collection-recipe-starfall-volley").Props.Label; !strings.Contains(got, "Starfall Volley") || !strings.Contains(got, "Ember Bolt + Oil") {
		t.Fatalf("a made recipe reads %q", got)
	}
	if got := app2.Find("id:collection-recipe-sun-ring").Props.Label; got != "???, The halo longs for a wider light." {
		t.Fatalf("an unmade recipe reads %q", got)
	}
	press(t, app2, "id:collection-back")
	if g2.phase.Get() != phaseTitle {
		t.Fatalf("Back from the collection gave phase %v, want the title", g2.phase.Get())
	}
}
