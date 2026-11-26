package main

import (
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
	press(t, app, "role:button Pause")
	check("pause")
	press(t, app, "id:pause-quit")
	check("pause-confirm-quit")
	press(t, app, "id:pause-confirm-quit-no")
	press(t, app, "id:pause-resume")
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
		if o.Heal {
			app.Find("id:card-icon-fallback-" + o.ID)
		} else {
			app.Find("id:card-icon-" + o.ID)
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
	if got := app.Find("id:results-outcome").Text; got != "DEFEAT" {
		t.Fatalf("defeat outcome = %q, want DEFEAT", got)
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
	app.Find("id:hud-timer")

	// Spawn Hollow King and confirm boss health bar appears in the HUD.
	g.run.spawnBoss()
	app.Advance(8 * frame)
	app.Find("id:hud-boss")
	if got := app.Find("id:hud-boss-name").Text; got != "Hollow King" {
		t.Fatalf("boss HUD name = %q, want Hollow King", got)
	}
	app.Find("id:hud-boss-health")

	// Defeat the Hollow King and confirm Victory results screen and Title action.
	if be := g.run.bossEnemy(); be != nil {
		be.hp = 0
	}
	app.Advance(8 * frame)
	if p := g.run.Snapshot().Phase; p != PhaseVictory {
		t.Fatalf("phase after killing boss = %v, want PhaseVictory", p)
	}
	app.Find("id:results")
	if got := app.Find("id:results-outcome").Text; got != "VICTORY" {
		t.Fatalf("victory outcome = %q, want VICTORY", got)
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

// TestTitleArtAndFallback checks that the title displays embedded Title.png key art and cleanly falls back when absent.
func TestTitleArtAndFallback(t *testing.T) {
	app, _ := startEmberVigil(t)
	app.Find("id:title-art")
	app.Find("id:title-records")

	appFallback := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		g := newEmberVigil(cx)
		g.art.hasTitleArt = false
		return g.view()
	}, nsgotest.Size(960, 540), nsgotest.ManualFrames())
	appFallback.Advance(3 * frame)
	appFallback.Find("id:title-art-fallback")
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
	if got := app.Find("id:settings-heading").Text; got != "设置" {
		t.Fatalf("settings heading after switching to zh-CN = %q, want 设置", got)
	}
	press(t, app, "id:settings-apply")
	app.Find("id:title")
	if got := app.Find("id:title-heading").Text; got != "余烬守望" {
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
	if heading := app2.Find("id:title-heading").Text; heading != "余烬守望" {
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

	press(t, app, "id:hud-pause")
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
	app, g := startEmberVigil(t)
	press(t, app, "id:title-settings")
	press(t, app, "id:settings-lang-zh-CN")
	press(t, app, "id:settings-apply")

	// Title screen
	if got := app.Find("id:title-heading").Text; got != "余烬守望" {
		t.Fatalf("zh-CN title = %q, want 余烬守望", got)
	}
	app.Find("role:button 开始")
	app.Find("role:button 设置")
	app.Find("role:button 退出")

	// Arena HUD + Boss bar
	press(t, app, "id:title-start")
	if got := app.Find("id:hud-level").Text; got != "等级 1" {
		t.Fatalf("zh-CN HUD level = %q, want 等级 1", got)
	}
	if got := app.Find("id:hud-kills").Text; got != "击杀 0" {
		t.Fatalf("zh-CN HUD kills = %q, want 击杀 0", got)
	}
	if got := app.Find("id:hud-weapon-name").Text; got != "余烬箭" {
		t.Fatalf("zh-CN HUD weapon = %q, want 余烬箭", got)
	}

	// Pause + quit confirmation
	press(t, app, "id:hud-pause")
	app.Find("text:已暂停")
	app.Find("role:button 继续")
	press(t, app, "id:pause-quit")
	app.Find("text:确定返回标题？")
	press(t, app, "id:pause-confirm-quit-no")
	press(t, app, "id:pause-resume")

	// Level-up cards
	g.run.xp = g.run.xpNeeded()
	app.Advance(2 * frame)
	offers := g.run.Snapshot().Offers
	zh := evstrings.For(evstrings.ZhCN)
	for i, o := range offers {
		want := zh.Card(o.ID, o.Name, o.Description)
		if got := app.Find("id:level-up-card-name-" + string(rune('0'+i))).Text; got != want.Name {
			t.Fatalf("zh-CN card %d name = %q, want %q", i, got, want.Name)
		}
		if got := app.Find("id:level-up-card-desc-" + string(rune('0'+i))).Text; got != want.Description {
			t.Fatalf("zh-CN card %d desc = %q, want %q", i, got, want.Description)
		}
	}
	press(t, app, "id:level-up-card-0")

	// Boss HUD + Victory results
	g.run.spawnBoss()
	app.Advance(8 * frame)
	if got := app.Find("id:hud-boss-name").Text; got != "空洞之王" {
		t.Fatalf("zh-CN boss name = %q, want 空洞之王", got)
	}
	if got := app.Find("id:hud-boss-phase").Text; got != "第一阶段" {
		t.Fatalf("zh-CN boss phase = %q, want 第一阶段", got)
	}
	if be := g.run.bossEnemy(); be != nil {
		be.hp = 0
	}
	app.Advance(8 * frame)
	if got := app.Find("id:results-outcome").Text; got != "胜利" {
		t.Fatalf("zh-CN victory outcome = %q, want 胜利", got)
	}
	if got := app.Find("id:results-record").Text; got != "刷新记录！" {
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
	if got := app.Find("id:results-record").Text; got != "New record" {
		t.Fatalf("first run record label = %q, want New record", got)
	}

	// Extra frames and repeated recordResult calls must not double-count wins.
	app.Advance(20 * frame)
	g.recordResult(g.run.Snapshot())
	if wins := g.records.Get().Wins; wins != 1 {
		t.Fatalf("wins after one victory = %d, want 1", wins)
	}

	// Restart app from persisted storage and verify title shows saved records.
	wantKills := g.run.Snapshot().Kills
	persisted := storageSnapshot(app.Storage())
	app2, g2 := startEmberVigil(t, nsgotest.Storage(persisted))
	if got := app2.Find("id:title-record-wins").Text; got != "Wins 1" {
		t.Fatalf("reloaded title wins = %q, want Wins 1", got)
	}
	if got := app2.Find("id:title-record-kills").Text; got != evstrings.For(evstrings.EN).BestKills(wantKills) {
		t.Fatalf("reloaded title kills = %q, want %q", got, evstrings.For(evstrings.EN).BestKills(wantKills))
	}

	// A worse second run must show "No new record" and keep existing records.
	press(t, app2, "id:title-start")
	g2.run.keeper.health = 0
	app2.Advance(8 * frame)
	if got := app2.Find("id:results-record").Text; got != "No new record" {
		t.Fatalf("worse run record label = %q, want No new record", got)
	}
	if rec := g2.records.Get(); rec.BestKills != wantKills || rec.BestLevel != 5 || rec.Wins != 1 {
		t.Fatalf("worse run altered records: %+v", rec)
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
