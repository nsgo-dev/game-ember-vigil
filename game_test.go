package main

import (
	"testing"
	"time"

	"github.com/nsgo-dev/game/packages/game/screen"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

// TestMain gives the platform the main thread under nsgo test -target.
func TestMain(m *testing.M) { nsgotest.Main(m) }

// frame is one display frame, which at 60 Hz runs one fixed step.
const frame = 16667 * time.Microsecond

// startEmberVigil starts the shell and hands back its instance, so a test reads the phase and the run it steps.
func startEmberVigil(t *testing.T) (*nsgotest.App, *emberVigil) {
	t.Helper()
	var g *emberVigil
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		g = newEmberVigil(cx)
		return g.view()
	}, nsgotest.Size(960, 540), nsgotest.ManualFrames())
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

	press(t, app, "role:button Pause")
	app.Find("id:pause")
	app.Find("text:Paused")
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

	press(t, app, "role:button Pause")
	press(t, app, "role:button Quit")
	if g.phase.Get() != phaseTitle {
		t.Fatalf("phase after Quit = %v, want phaseTitle", g.phase.Get())
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
	app.Advance(frame)
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

func TestTitleQuitQuitsTheApp(t *testing.T) {
	app, _ := startEmberVigil(t)

	press(t, app, "role:button Quit")
	if app.Quits != 1 {
		t.Fatalf("the app was asked to quit %d times, want 1", app.Quits)
	}
}

// TestViewportKeepsTheVirtualCanvasShape checks that the window scales the 480x270 canvas without stretching it, whatever its aspect ratio.
func TestViewportKeepsTheVirtualCanvasShape(t *testing.T) {
	if viewport.Width != 480 || viewport.Height != 270 {
		t.Fatalf("virtual canvas = %vx%v, want 480x270", viewport.Width, viewport.Height)
	}
	if viewport.Mode != screen.ModeFit {
		t.Fatalf("viewport mode = %v, want screen.ModeFit", viewport.Mode)
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
		if place.VirtualSize.Width != 480 || place.VirtualSize.Height != 270 {
			t.Fatalf("window %v shows a %v virtual canvas, want 480x270", window, place.VirtualSize)
		}
		if place.Rect.Size.Width > window.Width+0.01 || place.Rect.Size.Height > window.Height+0.01 {
			t.Fatalf("window %v holds a %v canvas, which does not fit", window, place.Rect)
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
	check("arena")
	g.run.xp = g.run.xpNeeded()
	app.Advance(2 * frame)
	check("level-up")
}

// TestTheHUDFollowsTheRun checks that the native HUD shows the run's clock, level, kills and starter weapon, and keeps up with them.
func TestTheHUDFollowsTheRun(t *testing.T) {
	app, g := startEmberVigil(t)
	press(t, app, "role:button Start")

	if got := app.Find("id:hud-timer").Text; got != "00:00" {
		t.Fatalf("the HUD's clock reads %q at the start, want 00:00", got)
	}
	if got := app.Find("id:hud-level").Text; got != "Level 1" {
		t.Fatalf("the HUD's level reads %q, want Level 1", got)
	}
	if got := app.Find("id:hud-kills").Text; got != "Kills 0" {
		t.Fatalf("the HUD's kills read %q, want Kills 0", got)
	}
	if got := app.Find("id:hud-weapon-name").Text; got != "Ember Bolt" {
		t.Fatalf("the HUD's weapon slot reads %q, want Ember Bolt", got)
	}
	app.Find("id:hud-health")
	app.Find("id:hud-xp")

	app.Advance(70 * frame)
	want := clockText(g.run.Snapshot().Time)
	if got := app.Find("id:hud-timer").Text; got != want || got == "00:00" {
		t.Fatalf("the HUD's clock reads %q after a second, want %q", got, want)
	}
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
		if got := app.Find("id:level-up-card-name-" + string(rune('0'+i))).Text; got != o.Name {
			t.Fatalf("card %d name = %q, want %q", i, got, o.Name)
		}
		if got := app.Find("id:level-up-card-desc-" + string(rune('0'+i))).Text; got != o.Description {
			t.Fatalf("card %d description = %q, want %q", i, got, o.Description)
		}
		if o.MaxLevel > 0 {
			app.Find("id:level-up-card-pips-" + string(rune('0'+i)))
		}
	}

	// Run stays paused while the modal is open.
	pausedAt := g.run.Snapshot().Time
	app.Advance(10 * frame)
	if g.run.Snapshot().Time != pausedAt {
		t.Fatalf("the run advanced from %v to %v while level-up cards were open", pausedAt, g.run.Snapshot().Time)
	}

	press(t, app, "role:button "+offers[0].Name)
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
	app.Advance(frame)
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
	app.Advance(frame)
	if p := g.run.Snapshot().Phase; p != PhasePlaying {
		t.Fatalf("phase after Enter = %v, want PhasePlaying", p)
	}
}

// TestDefeatShowsTheOverlay checks that a dead keeper shows the minimal defeat overlay, whose Retry starts a fresh run and whose Title returns to the title screen.
func TestDefeatShowsTheOverlay(t *testing.T) {
	app, g := startEmberVigil(t)
	press(t, app, "role:button Start")

	first := g.run
	g.run.keeper.health = 0
	app.Advance(8 * frame)
	if p := g.run.Snapshot().Phase; p != PhaseDefeat {
		t.Fatalf("the run's phase with no health is %v, want PhaseDefeat", p)
	}
	app.Find("id:defeat")
	var texts []string
	texts = app.Texts()
	for _, want := range []string{"Time ", "Kills ", "Level "} {
		found := false
		for _, got := range texts {
			if len(got) >= len(want) && got[:len(want)] == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("the defeat overlay shows no %q line: %v", want, texts)
		}
	}

	press(t, app, "role:button Retry")
	if g.run == first {
		t.Fatal("Retry kept the defeated run")
	}
	if p := g.run.Snapshot().Phase; p != PhasePlaying {
		t.Fatalf("the retried run's phase is %v, want PhasePlaying", p)
	}
	if app.Exists("id:defeat") {
		t.Fatal("the defeat overlay stayed after a retry")
	}
	app.Find("id:hud-timer")

	g.run.keeper.health = 0
	app.Advance(8 * frame)
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
