package main

import (
	"fmt"
	"math"
	"time"

	"github.com/nsgo-dev/game/packages/game"
	"github.com/nsgo-dev/game/packages/game/input"
	"github.com/nsgo-dev/game/packages/game/quality"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/desktop"
	evstrings "me.oneo/embervigil/strings"
)

// rate is how many times a second the game steps its run, whatever the display's refresh rate, and hudRate how often the native HUD reads the run.
const (
	rate       = 60
	hudRate    = 100 * time.Millisecond
	volumeStep = 0.1
)

// Virtual touch stick geometry in view points, kept clear of the screen's safe area.
const (
	stickRadius     = 48
	stickMargin     = 24
	stickDeadZone   = 0.14
	stickZoneWidth  = 220
	stickZoneHeight = 200
)

// The actions game.yaml binds, which the shell reads by name.
const (
	actionLeft    = "left"
	actionRight   = "right"
	actionUp      = "up"
	actionDown    = "down"
	actionPause   = "pause"
	actionConfirm = "confirm"
	actionCard1   = "card_1"
	actionCard2   = "card_2"
	actionCard3   = "card_3"
	axisMoveX     = "move_x"
	axisMoveY     = "move_y"
)

// phase is which screen the shell shows.
type phase uint8

const (
	phaseTitle phase = iota
	phaseArena
	phasePaused
	phaseSettings
)

// Menu item counts for directional keyboard/gamepad traversal.
const (
	titleMenuCount       = 3 // Start, Settings, Quit
	pauseMenuCount       = 3 // Resume, Settings, Quit to Title
	confirmQuitMenuCount = 2 // Confirm Quit, Cancel
	settingsMenuCount    = 8 // Master, Music, SFX, Shake, Quality, Language, Apply, Back
	resultsMenuCount     = 2 // Play Again, Title
)

// emberVigil is the game's shell: it owns the window, the fixed-step loop, input and the screens, and hands the run its steps.
type emberVigil struct {
	cx      *core.Scope
	storage core.Storage
	loop    *game.Loop
	actions *input.Actions
	pad     *input.VirtualPad
	stick   *input.VirtualStick
	art     *art
	scene   *arenaRenderer
	audio   *audioController
	quality *quality.Selector
	run     *Run

	// seed is the run's seed, which the arena's ground decoration is drawn from, so it stays put for the whole run.
	seed uint64
	// alpha is how far the last step's frame falls between the run's last two steps, which Draw blends by.
	alpha float64

	// phase is the screen that shows; frame changes on every draw so the canvas draws again.
	phase *core.Signal[phase]
	frame *core.Signal[uint64]
	// hud is what the native views show, published at hudRate rather than every step; hudClock is how long since the last time.
	hud       *core.Signal[hudState]
	hudClock  time.Duration
	cardFocus int

	// Persisted settings and lifetime records, plus reactive menu state.
	settings       *core.Signal[Settings]
	records        *core.Signal[Records]
	savedSettings  Settings
	settingsReturn phase
	confirmQuit    *core.Signal[bool]
	menuFocus      *core.Signal[int]

	// Per-run record guard so a finished run updates persisted records at most once.
	runRecorded   bool
	lastNewRecord bool

	// size is the arena canvas's size when it last drew, which input maps into virtual pixels.
	size      core.Size
	stickSize core.Size
	stickSafe core.Edges
	touchUsed bool
}

// Root is the game's root component.
func Root(cx *core.Scope) core.Node {
	return newEmberVigil(cx).view()
}

// newEmberVigil starts the shell on the title screen, loading persisted settings and records from storage.
func newEmberVigil(cx *core.Scope) *emberVigil {
	storage := cx.Storage()
	saved := loadSettings(storage)
	recs := loadRecords(storage)

	desktop.SetTitle(cx, evstrings.For(saved.Language).GameTitle)
	cx.RequestOrientation(core.OrientationLandscape)

	g := &emberVigil{
		cx:             cx,
		storage:        storage,
		actions:        input.NewActions(),
		art:            newArt(),
		phase:          core.NewSignal(cx, phaseTitle),
		frame:          core.NewSignal(cx, uint64(0)),
		hud:            core.NewSignal(cx, hudState{}),
		settings:       core.NewSignal(cx, saved),
		records:        core.NewSignal(cx, recs),
		savedSettings:  saved,
		settingsReturn: phaseTitle,
		confirmQuit:    core.NewSignal(cx, false),
		menuFocus:      core.NewSignal(cx, 0),
	}
	g.scene = newArenaRenderer(g.art)
	g.audio = newAudioController(cx, saved)
	if sel, err := quality.NewSelector(emberQualityConfig(), quality.DeviceFromEnv(cx.Env())); err == nil {
		g.quality = sel
		g.quality.OnChange = func(l quality.Level, p quality.Preset) {
			g.scene.setQuality(qualityProfileFor(l, p))
		}
	}
	g.applyLiveSettings(saved)

	// Touches and clicks arrive in the view's points; game.yaml's regions are in virtual pixels.
	g.actions.MapPoint = func(p core.Point) (core.Point, bool) { return viewport.ViewToGame(g.size, p) }
	Project.BindInput(g.actions)
	g.pad = input.NewVirtualPad()
	g.stick = g.pad.AddStick(&input.VirtualStick{
		Name:      "stick",
		AxisXName: axisMoveX,
		AxisYName: axisMoveY,
		Radius:    stickRadius,
		DeadZone:  stickDeadZone,
		Floating:  true,
	})
	g.actions.BindVirtualPad(g.pad)
	initialSize := cx.Env().Window()
	if initialSize.Width <= 0 || initialSize.Height <= 0 {
		initialSize = core.Size{Width: 960, Height: 540}
	}
	g.updateStickLayout(initialSize, cx.Env().SafeArea())
	g.actions.WatchKeys(cx)
	g.actions.WatchGamepads(cx)

	g.loop = &game.Loop{
		Rate:   rate,
		Update: g.update,
		FrameTime: func(dt time.Duration) {
			if g.quality != nil && g.phase.GetUntracked() == phaseArena && g.run != nil && g.run.Snapshot().Phase == PhasePlaying {
				g.quality.Observe(dt)
			}
		},
		Draw: func(alpha float64) {
			g.alpha = alpha
			g.frame.Update(func(n uint64) uint64 { return n + 1 })
		},
	}
	g.loop.Run(cx)
	return g
}

// applyLiveSettings pushes volume, screen shake, quality preset and window title updates to the shell's subsystems.
func (g *emberVigil) applyLiveSettings(s Settings) {
	s = normalizeSettings(s)
	g.scene.setShakeEnabled(s.ScreenShake)
	if g.audio != nil {
		g.audio.applySettings(s)
	}
	if g.quality != nil {
		_ = g.quality.SetSetting(string(s.Quality))
		g.scene.setQuality(qualityProfileFor(g.quality.Level(), g.quality.Preset()))
	}
	desktop.SetTitle(g.cx, evstrings.For(s.Language).GameTitle)
}

// tr returns the active language's string table, subscribing the caller to language changes.
func (g *emberVigil) tr() evstrings.Table {
	return evstrings.For(g.settings.Get().Language)
}

// update samples input once and steps the run while the arena shows.
func (g *emberVigil) update(dt time.Duration) {
	g.actions.Step()
	if g.actions.JustPressed(actionPause) {
		g.togglePause()
		return
	}
	if g.phase.GetUntracked() == phaseArena && g.run != nil && g.run.Snapshot().Phase == PhaseLevelUp {
		switch {
		case g.actions.JustPressed(actionCard1):
			g.choose(0)
			return
		case g.actions.JustPressed(actionCard2):
			g.choose(1)
			return
		case g.actions.JustPressed(actionCard3):
			g.choose(2)
			return
		case g.actions.JustPressed(actionLeft):
			g.cardFocus = (g.cardFocus + offerCount - 1) % offerCount
			g.audio.playCue(cueUIClick)
			g.publishHUD()
		case g.actions.JustPressed(actionRight):
			g.cardFocus = (g.cardFocus + 1) % offerCount
			g.audio.playCue(cueUIClick)
			g.publishHUD()
		case g.actions.JustPressed(actionConfirm):
			g.choose(g.cardFocus)
			return
		}
	} else {
		g.handleMenuNavigation()
	}

	if g.phase.GetUntracked() == phaseArena && g.run != nil {
		prev := g.run.Snapshot()
		g.run.Step(dt, Controls{Move: Vec2{X: g.actions.Axis(axisMoveX), Y: g.actions.Axis(axisMoveY)}})
		snap := g.run.Snapshot()
		events := g.run.Events()
		keeperDamaged := snap.Health < prev.Health-1e-6
		g.scene.sync(snap, events, dt.Seconds())
		g.audio.playRunEvents(events, keeperDamaged)
		if snap.Phase != prev.Phase {
			if snap.Phase == PhaseLevelUp {
				g.cardFocus = 0
			}
			if snap.Phase == PhaseVictory || snap.Phase == PhaseDefeat {
				g.recordResult(snap)
				g.menuFocus.Set(0)
			}
			g.hudClock = 0
			g.publishHUD()
			return
		}
		if g.hudClock += dt; g.hudClock >= hudRate {
			g.hudClock = 0
			g.publishHUD()
		}
	}
}

// handleMenuNavigation handles directional focus and confirm input on title, pause, settings and results screens.
func (g *emberVigil) handleMenuNavigation() {
	p := g.phase.GetUntracked()
	switch p {
	case phaseTitle:
		g.stepVerticalFocus(titleMenuCount)
		if g.actions.JustPressed(actionConfirm) {
			g.confirm()
		}
	case phasePaused:
		if g.confirmQuit.GetUntracked() {
			if g.actions.JustPressed(actionLeft) || g.actions.JustPressed(actionUp) {
				g.moveFocus(-1, confirmQuitMenuCount)
			} else if g.actions.JustPressed(actionRight) || g.actions.JustPressed(actionDown) {
				g.moveFocus(1, confirmQuitMenuCount)
			}
		} else {
			g.stepVerticalFocus(pauseMenuCount)
		}
		if g.actions.JustPressed(actionConfirm) {
			g.confirm()
		}
	case phaseSettings:
		g.stepVerticalFocus(settingsMenuCount)
		if g.actions.JustPressed(actionLeft) {
			g.adjustFocusedSetting(-1)
		} else if g.actions.JustPressed(actionRight) {
			g.adjustFocusedSetting(1)
		} else if g.actions.JustPressed(actionConfirm) {
			g.confirm()
		}
	case phaseArena:
		if g.run != nil {
			if rp := g.run.Snapshot().Phase; rp == PhaseVictory || rp == PhaseDefeat {
				if g.actions.JustPressed(actionLeft) || g.actions.JustPressed(actionUp) {
					g.moveFocus(-1, resultsMenuCount)
				} else if g.actions.JustPressed(actionRight) || g.actions.JustPressed(actionDown) {
					g.moveFocus(1, resultsMenuCount)
				}
				if g.actions.JustPressed(actionConfirm) {
					g.confirm()
				}
			}
		}
	}
}

// stepVerticalFocus moves menuFocus up or down within count items.
func (g *emberVigil) stepVerticalFocus(count int) {
	if g.actions.JustPressed(actionUp) {
		g.moveFocus(-1, count)
	} else if g.actions.JustPressed(actionDown) {
		g.moveFocus(1, count)
	}
}

// moveFocus wraps menuFocus by delta within [0, count).
func (g *emberVigil) moveFocus(delta, count int) {
	if count <= 0 {
		return
	}
	cur := g.menuFocus.GetUntracked()
	next := (cur + delta%count + count) % count
	if next != cur {
		g.audio.playCue(cueUIClick)
	}
	g.menuFocus.Set(next)
}

// adjustFocusedSetting modifies the currently focused setting row by delta (-1 or +1).
func (g *emberVigil) adjustFocusedSetting(delta int) {
	switch g.menuFocus.GetUntracked() {
	case 0:
		g.adjustMasterVolume(float64(delta) * volumeStep)
	case 1:
		g.adjustMusicVolume(float64(delta) * volumeStep)
	case 2:
		g.adjustSFXVolume(float64(delta) * volumeStep)
	case 3:
		g.toggleShake()
	case 4:
		g.cycleQuality(delta)
	case 5:
		g.cycleLanguage(delta)
	case 6:
		g.menuFocus.Set(7)
	case 7:
		g.menuFocus.Set(6)
	}
}

// publishHUD tells the native HUD what the run shows now: its clock, health, XP, level, kills, build and cards, or nothing when no run is on.
func (g *emberVigil) publishHUD() {
	if g.run == nil {
		g.hud.Set(hudState{})
		return
	}
	s := g.run.Snapshot()
	st := hudState{
		phase:         s.Phase,
		time:          clockText(s.Time),
		health:        max(0, int(math.Ceil(s.Health))),
		maxHealth:     int(math.Round(s.MaxHealth)),
		xp:            int(math.Round(s.XP)),
		xpNeeded:      int(math.Ceil(s.XPNeeded)),
		level:         s.Level,
		kills:         s.Kills,
		cardFocus:     g.cardFocus,
		bossActive:    s.Boss.Active,
		bossName:      s.Boss.Name,
		bossHealth:    max(0, int(math.Ceil(s.Boss.Health))),
		bossMaxHealth: int(math.Round(s.Boss.MaxHealth)),
		bossPhase:     s.Boss.Phase,
		newRecord:     g.lastNewRecord,
	}
	for i := range st.weapons {
		if i < len(s.Weapons) {
			st.weapons[i] = hudSlot{id: s.Weapons[i].ID, name: s.Weapons[i].Name, level: s.Weapons[i].Level}
		}
	}
	for i := range st.offers {
		if i < len(s.Offers) {
			o := s.Offers[i]
			st.offers[i] = hudCard{
				id:          o.ID,
				name:        o.Name,
				kind:        o.Kind,
				description: o.Description,
				level:       o.Level,
				maxLevel:    o.MaxLevel,
				heal:        o.Heal,
			}
		}
	}
	g.hud.Set(st)
}

// clockText writes a run's time as the minutes and seconds it has lasted.
func clockText(seconds float64) string {
	t := int(math.Max(0, seconds))
	return fmt.Sprintf("%02d:%02d", t/60, t%60)
}

// setPhase shows p's screen, resets menu focus and drops held input so a press on a menu does not reach the arena.
func (g *emberVigil) setPhase(p phase) {
	g.phase.Set(p)
	g.menuFocus.Set(0)
	g.actions.Reset()
}

// recordResult persists lifetime records once when a run ends in victory or defeat.
func (g *emberVigil) recordResult(s Snapshot) {
	if g.runRecorded {
		return
	}
	g.runRecorded = true
	next, isNew := applyRunRecords(g.records.GetUntracked(), s)
	_ = saveRecords(g.storage, next)
	g.records.Set(next)
	g.lastNewRecord = isNew
}

// start begins a run from the wall clock's seed, which the run itself never reads, and keeps that seed for the arena's ground.
func (g *emberVigil) start() {
	g.seed = uint64(time.Now().UnixNano())
	g.run = NewRun(g.seed)
	g.scene.startRun(g.seed)
	g.applyLiveSettings(g.settings.GetUntracked())
	g.hudClock = 0
	g.cardFocus = 0
	g.runRecorded = false
	g.lastNewRecord = false
	g.confirmQuit.Set(false)
	g.setPhase(phaseArena)
	g.audio.setPaused(false)
	g.audio.playCue(cueUISelect)
	g.audio.ensureMusic()
	g.publishHUD()
}

// choose takes card index of a level-up and resumes the run behind it.
func (g *emberVigil) choose(index int) {
	if g.run != nil && g.run.Choose(index) {
		g.cardFocus = 0
		g.actions.Reset()
		g.audio.playCue(cueUISelect)
		g.publishHUD()
	}
}

// pause stops a run that is being played under the pause overlay, leaving one that is over or waiting for a card as it is.
func (g *emberVigil) pause() {
	if g.phase.GetUntracked() == phaseArena && g.run != nil && g.run.Snapshot().Phase == PhasePlaying {
		g.confirmQuit.Set(false)
		g.setPhase(phasePaused)
		g.audio.setPaused(true)
		g.audio.playCue(cueUIClick)
	}
}

// resume returns from the pause overlay to the run it stopped.
func (g *emberVigil) resume() {
	if g.phase.GetUntracked() == phasePaused {
		g.confirmQuit.Set(false)
		g.setPhase(phaseArena)
		g.audio.setPaused(false)
		g.audio.playCue(cueUISelect)
	}
}

// togglePause pauses a playing run, closes a confirmation or settings overlay, or resumes a paused run.
func (g *emberVigil) togglePause() {
	switch g.phase.GetUntracked() {
	case phaseArena:
		g.pause()
	case phasePaused:
		if g.confirmQuit.GetUntracked() {
			g.cancelQuitRun()
		} else {
			g.resume()
		}
	case phaseSettings:
		g.closeSettings()
	}
}

// requestQuitRun asks for confirmation before abandoning an active run.
func (g *emberVigil) requestQuitRun() {
	if g.phase.GetUntracked() == phasePaused {
		g.confirmQuit.Set(true)
		g.menuFocus.Set(0)
		g.audio.playCue(cueUIClick)
	}
}

// cancelQuitRun dismisses the quit confirmation and keeps the run paused.
func (g *emberVigil) cancelQuitRun() {
	if g.phase.GetUntracked() == phasePaused {
		g.confirmQuit.Set(false)
		g.menuFocus.Set(0)
		g.audio.playCue(cueUIClick)
	}
}

// quitRun ends the run and returns to the title screen.
func (g *emberVigil) quitRun() {
	g.confirmQuit.Set(false)
	g.run = nil
	g.runRecorded = false
	g.setPhase(phaseTitle)
	g.audio.setPaused(false)
	g.audio.stopMusic()
	g.audio.playCue(cueUIClick)
	g.publishHUD()
}

// openSettings opens the settings screen from either the title or pause screen, remembering where to return.
func (g *emberVigil) openSettings() {
	cur := g.phase.GetUntracked()
	if cur != phaseTitle && cur != phasePaused {
		return
	}
	g.settingsReturn = cur
	g.savedSettings = g.settings.GetUntracked()
	g.confirmQuit.Set(false)
	g.setPhase(phaseSettings)
	g.audio.playCue(cueUISelect)
}

// updateDraft mutates the live settings draft and immediately applies audio, quality, shake and window title updates.
func (g *emberVigil) updateDraft(edit func(*Settings)) {
	cur := g.settings.GetUntracked()
	edit(&cur)
	cur = normalizeSettings(cur)
	g.settings.Set(cur)
	g.applyLiveSettings(cur)
	g.audio.playCue(cueUIClick)
}

func (g *emberVigil) adjustMasterVolume(delta float64) {
	g.updateDraft(func(s *Settings) { s.MasterVolume = clampVolume(s.MasterVolume + delta) })
}

func (g *emberVigil) adjustMusicVolume(delta float64) {
	g.updateDraft(func(s *Settings) { s.MusicVolume = clampVolume(s.MusicVolume + delta) })
}

func (g *emberVigil) adjustSFXVolume(delta float64) {
	g.updateDraft(func(s *Settings) { s.SFXVolume = clampVolume(s.SFXVolume + delta) })
}

func (g *emberVigil) toggleShake() {
	g.updateDraft(func(s *Settings) { s.ScreenShake = !s.ScreenShake })
}

func (g *emberVigil) setQuality(q Quality) {
	g.updateDraft(func(s *Settings) { s.Quality = q })
}

func (g *emberVigil) cycleQuality(delta int) {
	cur := g.settings.GetUntracked().Quality
	idx := 0
	for i, q := range qualityTiers {
		if q == cur {
			idx = i
			break
		}
	}
	next := (idx + delta%len(qualityTiers) + len(qualityTiers)) % len(qualityTiers)
	g.setQuality(qualityTiers[next])
}

func (g *emberVigil) setLanguage(lang evstrings.Lang) {
	g.updateDraft(func(s *Settings) { s.Language = lang })
}

func (g *emberVigil) cycleLanguage(delta int) {
	cur := g.settings.GetUntracked().Language
	idx := 0
	for i, l := range evstrings.Languages {
		if l == cur {
			idx = i
			break
		}
	}
	next := (idx + delta%len(evstrings.Languages) + len(evstrings.Languages)) % len(evstrings.Languages)
	g.setLanguage(evstrings.Languages[next])
}

// applySettings persists the current settings draft to storage and returns to the previous screen.
func (g *emberVigil) applySettings() {
	if g.phase.GetUntracked() != phaseSettings {
		return
	}
	cur := normalizeSettings(g.settings.GetUntracked())
	_ = saveSettings(g.storage, cur)
	g.savedSettings = cur
	g.settings.Set(cur)
	g.applyLiveSettings(cur)
	g.audio.playCue(cueUISelect)
	g.setPhase(g.settingsReturn)
}

// closeSettings discards unsaved edits and returns to the screen that opened settings without resuming a paused run.
func (g *emberVigil) closeSettings() {
	if g.phase.GetUntracked() != phaseSettings {
		return
	}
	g.settings.Set(g.savedSettings)
	g.applyLiveSettings(g.savedSettings)
	g.audio.playCue(cueUIClick)
	g.setPhase(g.settingsReturn)
}

// quitApp ends the app.
func (g *emberVigil) quitApp() { g.cx.Quit() }

// confirm activates the currently focused control on the active screen.
func (g *emberVigil) confirm() {
	focus := g.menuFocus.GetUntracked()
	switch g.phase.GetUntracked() {
	case phaseTitle:
		switch focus {
		case 1:
			g.openSettings()
		case 2:
			g.quitApp()
		default:
			g.start()
		}
	case phaseArena:
		if g.run != nil {
			if p := g.run.Snapshot().Phase; p == PhaseVictory || p == PhaseDefeat {
				if focus == 1 {
					g.quitRun()
				} else {
					g.start()
				}
			}
		}
	case phasePaused:
		if g.confirmQuit.GetUntracked() {
			if focus == 1 {
				g.cancelQuitRun()
			} else {
				g.quitRun()
			}
			return
		}
		switch focus {
		case 1:
			g.openSettings()
		case 2:
			g.requestQuitRun()
		default:
			g.resume()
		}
	case phaseSettings:
		switch focus {
		case 3:
			g.toggleShake()
		case 4:
			g.cycleQuality(1)
		case 5:
			g.cycleLanguage(1)
		case 6:
			g.applySettings()
		case 7:
			g.closeSettings()
		default:
			g.applySettings()
		}
	}
}

// onArenaTouch routes touches on the arena to the bottom-left virtual stick while a run is actively playing.
func (g *emberVigil) onArenaTouch(ev core.TouchEvent) {
	g.touchUsed = true
	if g.phase.GetUntracked() != phaseArena || g.run == nil || g.run.Snapshot().Phase != PhasePlaying {
		return
	}
	size := g.size
	if size.Width <= 0 || size.Height <= 0 {
		size = g.cx.Env().Window()
	}
	g.updateStickLayout(size, g.cx.Env().SafeArea())
	g.pad.OnTouch(ev)
}

// updateStickLayout keeps the virtual stick's resting center and activation zone inside the bottom-left safe area.
func (g *emberVigil) updateStickLayout(size core.Size, safe core.Edges) {
	if g.stick == nil || size.Width <= 0 || size.Height <= 0 {
		return
	}
	if size == g.stickSize && safe == g.stickSafe {
		return
	}
	radius := core.Points(stickRadius)
	if maxR := size.Height * 0.2; maxR > 0 && maxR < radius {
		radius = max(core.Points(32), maxR)
	}
	left := safe.Left + stickMargin
	bottom := safe.Bottom + stickMargin
	center := core.Point{
		X: left + radius,
		Y: size.Height - bottom - radius,
	}
	zoneWidth := min(size.Width*0.45, core.Points(stickZoneWidth))
	zoneHeight := min(size.Height-safe.Top-safe.Bottom, core.Points(stickZoneHeight))
	if zoneHeight < radius*2+stickMargin {
		zoneHeight = min(size.Height-safe.Bottom, radius*2+stickMargin)
	}
	zoneTop := max(safe.Top+buttonSide+12, size.Height-safe.Bottom-zoneHeight)
	if zoneTop >= size.Height-safe.Bottom {
		zoneTop = max(safe.Top, size.Height-safe.Bottom-radius*2)
	}
	g.stick.Radius = float32(radius)
	g.stick.Zone = core.Bounds{
		Origin: core.Point{X: safe.Left, Y: zoneTop},
		Size:   core.Size{Width: zoneWidth, Height: max(0, size.Height-safe.Bottom-zoneTop)},
	}
	if !g.stick.Active() {
		g.stick.Center = center
	}
	g.stickSize = size
	g.stickSafe = safe
}

// showTouchStick reports whether the bottom-left virtual stick should be drawn over the arena.
func (g *emberVigil) showTouchStick() bool {
	if g.phase.GetUntracked() != phaseArena || g.run == nil || g.run.Snapshot().Phase != PhasePlaying {
		return false
	}
	if g.touchUsed || (g.stick != nil && g.stick.Active()) {
		return true
	}
	env := g.cx.Env()
	idiom := env.Idiom()
	return idiom == core.IdiomPhone || idiom == core.IdiomTablet || env.Pointer() == core.PointerCoarse
}

// emberStickStyle styles the virtual stick in Ember Vigil's ash-and-ember palette with clear active feedback.
func emberStickStyle() input.VirtualStyle {
	return input.VirtualStyle{
		BaseFill:        core.RGBA(0x111827, 0.42),
		BaseStroke:      core.RGBA(0xf8fafc, 0.48),
		BaseStrokeWidth: 2,
		KnobFill:        core.RGBA(0xff7a1a, 0.68),
		KnobActiveFill:  core.RGBA(0xffb347, 0.92),
		KnobRadiusRatio: 0.42,
	}
}

// draw draws the arena through the viewport, which keeps the virtual canvas's shape in any window.
func (g *emberVigil) draw(d *core.Drawing, size core.Size) {
	_ = g.frame.Get()
	g.size = size
	alpha := g.alpha
	viewport.Draw(d, size, func(d *core.Drawing, _ core.Size) { g.scene.draw(d, alpha) })
	g.updateStickLayout(size, g.cx.Env().SafeArea())
	if g.showTouchStick() {
		g.pad.Draw(d, emberStickStyle())
	}
}
