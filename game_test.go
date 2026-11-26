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

	before := g.run.elapsed
	app.Advance(10 * frame)
	if g.run.elapsed <= before {
		t.Fatalf("the run lasted %v after ten frames, want more than %v", g.run.elapsed, before)
	}

	press(t, app, "role:button Pause")
	app.Find("id:pause")
	app.Find("text:Paused")
	paused := g.run.elapsed
	app.Advance(20 * frame)
	if g.run.elapsed != paused {
		t.Fatalf("the run lasted %v while paused, want %v", g.run.elapsed, paused)
	}

	press(t, app, "role:button Resume")
	if g.phase.Get() != phaseArena {
		t.Fatalf("phase after Resume = %v, want phaseArena", g.phase.Get())
	}
	app.Advance(10 * frame)
	if g.run.elapsed <= paused {
		t.Fatalf("the run lasted %v after resuming, want more than %v", g.run.elapsed, paused)
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
	app, _ := startEmberVigil(t)

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
}
