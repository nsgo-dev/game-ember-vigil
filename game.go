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

// rate is how many times a second the game steps its run, whatever the display's refresh rate, and hudRate how often the HUD reads the run.
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
	actionRedraw  = "redraw"
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
	phaseCollection
	phaseShop
)

// Menu item counts for directional keyboard/gamepad traversal.
const (
	pauseMenuCount       = 3                    // Resume, Settings, Quit to Title
	confirmQuitMenuCount = 2                    // Confirm Quit, Cancel
	settingsMenuCount    = settingsRowCount + 2 // the rows, Apply and Back
	resultsMenuCount     = 2                    // Play Again, Title
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
	hudArt  *hudArt
	// titleArt is the title's kit, and titleMotion the cover's clock.
	titleArt    *titleArt
	titleMotion titleMotion
	scene       *arenaRenderer
	audio       *audioController
	quality     *quality.Selector
	run         *Run

	// seed is the run's seed, which the arena's ground decoration is drawn from, so it stays put for the whole run.
	seed uint64
	// alpha is how far the last step's frame falls between the run's last two steps, which Draw blends by.
	alpha float64

	// phase is the screen that shows; frame changes on every draw so the canvas draws again.
	phase *core.Signal[phase]
	frame *core.Signal[uint64]
	// hud is what the HUD and the native views show, published at hudRate rather than every step; hudClock is how long since the last time.
	hud       *core.Signal[hudState]
	hudClock  time.Duration
	cardFocus int
	// hudMotion eases the canvas HUD toward hud between publications; pausePushed is whether a press or touch holds the canvas pause button.
	hudMotion   hudMotion
	pausePushed bool
	// overlay moves the canvas's cards and panels; pressed is the TestID of the overlay button a press holds down; viewSize is the arena's size, which the native layer over the overlays is placed by.
	overlay  overlayMotion
	pressed  string
	viewSize *core.Signal[core.Size]

	// Persisted settings and lifetime records, plus reactive menu state.
	settings *core.Signal[Settings]
	records  *core.Signal[Records]
	// progress is the saved progression; it changes as a run ends, before the HUD publishes the results it shows on, and as the shop sells, which bumps shopVersion so the shop's native layer reads it again.
	progress    Progression
	shopVersion *core.Signal[int]
	// tier is the Long Night tier the next run plays, which the title chooses among the open ones.
	tier           *core.Signal[int]
	savedSettings  Settings
	settingsReturn phase
	confirmQuit    *core.Signal[bool]
	menuFocus      *core.Signal[int]
	// stats reports the game's use while the saved settings allow it.
	stats *usageStats
	// fps measures the display's frame rate, which the canvas shows while the settings ask for it.
	fps fpsMeter

	// Per-run record guard so a finished run updates persisted records at most once.
	runRecorded   bool
	lastNewRecord bool
	lastReward    runReward

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
	prog := loadProgression(storage)

	desktop.SetTitle(cx, evstrings.For(saved.Language).GameTitle)
	cx.RequestOrientation(core.OrientationLandscape)

	g := &emberVigil{
		cx:             cx,
		storage:        storage,
		actions:        input.NewActions(),
		art:            newArt(),
		hudArt:         newHUDArt(embeddedHUD),
		titleArt:       newTitleArt(embeddedTitle),
		phase:          core.NewSignal(cx, phaseTitle),
		frame:          core.NewSignal(cx, uint64(0)),
		hud:            core.NewSignal(cx, hudState{}),
		settings:       core.NewSignal(cx, saved),
		records:        core.NewSignal(cx, recs),
		progress:       prog,
		shopVersion:    core.NewSignal(cx, 0),
		tier:           core.NewSignal(cx, prog.openTier(mustGameData().tiers.maxTier())),
		savedSettings:  saved,
		settingsReturn: phaseTitle,
		confirmQuit:    core.NewSignal(cx, false),
		menuFocus:      core.NewSignal(cx, 0),
	}
	g.scene = newArenaRenderer(g.art)
	g.viewSize = core.NewSignal(cx, cx.Env().Window())
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
	// The stick is laid out in the view's points, so the pad keeps them instead of taking the actions' virtual mapping.
	g.pad.MapPoint = func(p core.Point) (core.Point, bool) { return p, true }
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
			g.fps.observe(dt)
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
	g.stats = newStats(cx, func() bool { return g.savedSettings.Stats })
	g.stats.page("/", "Title")
	g.stats.beat(func() bool { return g.run != nil && g.phase.GetUntracked() == phaseArena })
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
	// Until the run steps below, frames hold its last step instead of blending toward one that is not coming.
	g.scene.hold()
	if g.phase.GetUntracked() == phaseArena {
		g.hudMotion.step(g.hud.GetUntracked(), dt.Seconds())
	}
	g.overlay.step(dt.Seconds(), g.cardFocus)
	// The cover moves only while the title shows, and stays still when the system asks for less motion.
	if g.phase.GetUntracked() == phaseTitle && !g.cx.Env().ReduceMotion() {
		g.titleMotion.step(dt.Seconds())
	}
	if index, ok := g.overlay.burnt(); ok {
		g.commitChoice(index)
	}
	if g.actions.JustPressed(actionPause) {
		g.togglePause()
		return
	}
	if g.phase.GetUntracked() == phaseArena && g.run != nil && g.run.Snapshot().Phase == PhaseLevelUp {
		switch {
		case g.overlay.burning:
			// A chosen card is burning away; the next card waits for it.
		case g.actions.JustPressed(actionCard1):
			g.choose(0)
			return
		case g.actions.JustPressed(actionCard2):
			g.choose(1)
			return
		case g.actions.JustPressed(actionCard3):
			g.choose(2)
			return
		case g.actions.JustPressed(actionRedraw):
			g.redraw()
		case g.actions.JustPressed(actionLeft):
			n := max(1, len(g.run.Snapshot().Offers))
			g.focusCard((g.cardFocus + n - 1) % n)
		case g.actions.JustPressed(actionRight):
			g.focusCard((g.cardFocus + 1) % max(1, len(g.run.Snapshot().Offers)))
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
		if hasEvent(events, EventEvolve) {
			g.recordEvolutions()
		}
		if snap.Phase != prev.Phase {
			if snap.Phase == PhaseLevelUp {
				g.cardFocus = 0
				g.overlay.dealCards()
			}
			if snap.Phase == PhaseVictory || snap.Phase == PhaseDefeat {
				g.recordResult(snap)
				g.menuFocus.Set(0)
				g.overlay.openPanel()
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
		// The menu is a row, so either axis moves along it, wrapping over the buttons shown.
		n := len(g.titleItems(g.tr()))
		if g.actions.JustPressed(actionLeft) || g.actions.JustPressed(actionUp) {
			g.moveFocus(-1, n)
		} else if g.actions.JustPressed(actionRight) || g.actions.JustPressed(actionDown) {
			g.moveFocus(1, n)
		}
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
	case phaseCollection:
		if g.actions.JustPressed(actionConfirm) {
			g.closeCollection()
		}
	case phaseShop:
		n, focus := len(mustGameData().progression.Shop.Items), g.menuFocus.GetUntracked()
		next := focus
		switch {
		case g.actions.JustPressed(actionUp):
			next = shopFocusMove(focus, n, 0, -1)
		case g.actions.JustPressed(actionDown):
			next = shopFocusMove(focus, n, 0, 1)
		case g.actions.JustPressed(actionLeft):
			next = shopFocusMove(focus, n, -1, 0)
		case g.actions.JustPressed(actionRight):
			next = shopFocusMove(focus, n, 1, 0)
		case g.actions.JustPressed(actionConfirm):
			g.confirm()
		}
		if next != focus {
			g.hoverMenu(next)
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
		g.toggleStats()
	case 7:
		g.toggleFPS()
	case settingsRowCount:
		g.menuFocus.Set(settingsRowCount + 1)
	case settingsRowCount + 1:
		g.menuFocus.Set(settingsRowCount)
	}
}

// publishHUD tells the HUD what the run shows now: its clock, health, XP, level, kills, build and cards, or nothing when no run is on.
func (g *emberVigil) publishHUD() {
	if g.run == nil {
		g.hud.Set(hudState{})
		return
	}
	g.hud.Set(hudStateOf(g.run.Snapshot(), g.cardFocus, g.lastNewRecord))
}

// hudStateOf is what the HUD shows of a snapshot, with the shell's focused card and whether the run set a record.
func hudStateOf(s Snapshot, cardFocus int, newRecord bool) hudState {
	st := hudState{
		phase:         s.Phase,
		time:          clockText(s.Time),
		health:        max(0, int(math.Ceil(s.Health))),
		maxHealth:     int(math.Round(s.MaxHealth)),
		lowHealthAt:   s.LowHealth,
		xp:            int(math.Round(s.XP)),
		xpNeeded:      int(math.Ceil(s.XPNeeded)),
		level:         s.Level,
		kills:         s.Kills,
		cardFocus:     cardFocus,
		bossActive:    s.Boss.Active,
		bossName:      s.Boss.Name,
		bossHealth:    max(0, int(math.Ceil(s.Boss.Health))),
		bossMaxHealth: int(math.Round(s.Boss.MaxHealth)),
		bossPhase:     s.Boss.Phase,
		newRecord:     newRecord,
		tier:          s.Tier,
		redraws:       s.Redraws,
	}
	for i := range st.weapons {
		if i < len(s.Weapons) {
			w := s.Weapons[i]
			st.weapons[i] = hudSlot{id: w.ID, name: w.Name, level: w.Level, maxLevel: w.MaxLevel, evolved: w.Evolved}
		}
	}
	// Between the Hollow King's appearances the bar follows the first Gloomwarden on the field.
	if !st.bossActive {
		for _, e := range s.Enemies {
			if e.Elite && e.Health > 0 {
				st.bossActive, st.bossElite, st.bossName = true, true, e.Kind
				st.bossHealth = int(math.Ceil(e.Health))
				st.bossMaxHealth = int(math.Round(e.MaxHealth))
				break
			}
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
	return st
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

// recordResult persists lifetime records, the run's embers and its unlocks once when a run ends in victory or defeat.
func (g *emberVigil) recordResult(s Snapshot) {
	if g.runRecorded {
		return
	}
	g.runRecorded = true
	prog, reward := applyRunProgression(g.progress, s.Phase == PhaseVictory, g.run.Tier(), mustGameData().tiers.maxTier(), g.run.Payout(), g.run.Unlocks())
	// A win that opens the next tier makes it the next run's, so Play Again climbs.
	if reward.tierOpened > 0 {
		g.tier.Set(reward.tierOpened)
	}
	_ = saveProgression(g.storage, prog)
	g.lastReward = reward
	g.progress = prog
	next, isNew := applyRunRecords(g.records.GetUntracked(), s)
	_ = saveRecords(g.storage, next)
	g.records.Set(next)
	g.lastNewRecord = isNew
	if s.Phase == PhaseVictory {
		g.stats.event("victory")
	} else {
		g.stats.event("defeat")
	}
}

// recordEvolutions adds the run's evolutions to the collection and saves it at once, so a recipe made stays made even if the run is left.
func (g *emberVigil) recordEvolutions() {
	prog, added := addEvolutions(g.progress, g.run.Evolutions())
	if len(added) > 0 {
		g.progress = prog
		_ = saveProgression(g.storage, prog)
	}
}

// hasEvent reports whether events holds one of kind.
func hasEvent(events []Event, kind EventKind) bool {
	for _, e := range events {
		if e.Kind == kind {
			return true
		}
	}
	return false
}

// start begins a run from the wall clock's seed, which the run itself never reads, and keeps that seed for the arena's ground.
func (g *emberVigil) start() {
	g.seed = uint64(time.Now().UnixNano())
	g.run = NewRun(g.seed, g.tier.GetUntracked())
	g.run.SetShop(g.progress.Shop)
	if g.progress.has(unlockEvolution) {
		g.run.EnableEvolution()
	}
	g.run.UnlockWeapons(g.progress.Unlocked)
	g.scene.startRun(g.seed)
	g.applyLiveSettings(g.settings.GetUntracked())
	g.hudClock = 0
	g.hudMotion = hudMotion{}
	g.pausePushed = false
	g.overlay = overlayMotion{}
	g.pressed = ""
	g.cardFocus = 0
	g.runRecorded = false
	g.lastNewRecord = false
	g.lastReward = runReward{}
	g.confirmQuit.Set(false)
	g.setPhase(phaseArena)
	g.audio.setPaused(false)
	g.audio.playCue(cueUISelect)
	g.audio.ensureMusic()
	g.publishHUD()
	g.stats.page("/run", "Run")
}

// choose takes card index of a level-up: the card burns away, and then the run applies it and resumes.
func (g *emberVigil) choose(index int) {
	if g.run == nil || g.run.Snapshot().Phase != PhaseLevelUp || index < 0 || index >= len(g.run.Snapshot().Offers) {
		return
	}
	if g.overlay.burnAway(index) {
		g.cardFocus = index
		g.actions.Reset()
		g.audio.playCue(cueUISelect)
	}
}

// commitChoice hands the run the card that has burnt away and resumes it.
func (g *emberVigil) commitChoice(index int) {
	if g.run != nil && g.run.Choose(index) {
		g.cardFocus = 0
		g.actions.Reset()
		g.publishHUD()
	}
}

// redraw deals the level-up a new hand while the run has redraws left, unless a chosen card is already burning.
func (g *emberVigil) redraw() {
	if g.run == nil || g.overlay.burning || !g.run.Redraw() {
		return
	}
	g.cardFocus = 0
	g.overlay.dealCards()
	g.actions.Reset()
	g.audio.playCue(cueUISelect)
	g.publishHUD()
}

// focusCard moves the level-up's focus to card index.
func (g *emberVigil) focusCard(index int) {
	if g.overlay.burning || index == g.cardFocus {
		return
	}
	g.cardFocus = index
	g.audio.playCue(cueUIClick)
	g.publishHUD()
}

// hoverMenu moves a painted menu's focus to the button the mouse came over.
func (g *emberVigil) hoverMenu(index int) {
	if g.menuFocus.GetUntracked() != index {
		g.audio.playCue(cueUIClick)
		g.menuFocus.Set(index)
	}
}

// pause stops a run that is being played under the pause overlay, leaving one that is over or waiting for a card as it is.
func (g *emberVigil) pause() {
	if g.phase.GetUntracked() == phaseArena && g.run != nil && g.run.Snapshot().Phase == PhasePlaying {
		g.confirmQuit.Set(false)
		g.setPhase(phasePaused)
		g.overlay.openPanel()
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
	case phaseCollection:
		g.closeCollection()
	case phaseShop:
		g.closeShop()
	}
}

// openCollection opens the collection from the title.
func (g *emberVigil) openCollection() {
	if g.phase.GetUntracked() != phaseTitle {
		return
	}
	g.overlay.openPanel()
	g.setPhase(phaseCollection)
	g.audio.playCue(cueUISelect)
}

// closeCollection returns from the collection to the title.
func (g *emberVigil) closeCollection() {
	if g.phase.GetUntracked() != phaseCollection {
		return
	}
	g.audio.playCue(cueUIClick)
	g.setPhase(phaseTitle)
}

// openShop opens the ember shop from the title.
func (g *emberVigil) openShop() {
	if g.phase.GetUntracked() != phaseTitle {
		return
	}
	g.overlay.openPanel()
	g.setPhase(phaseShop)
	g.audio.playCue(cueUISelect)
}

// closeShop returns from the shop to the title.
func (g *emberVigil) closeShop() {
	if g.phase.GetUntracked() != phaseShop {
		return
	}
	g.audio.playCue(cueUIClick)
	g.setPhase(phaseTitle)
}

// buy buys the next level of shop item id and saves the progression at once, so the next run starts with it; an item maxed or too dear only clicks.
func (g *emberVigil) buy(id string) {
	if g.phase.GetUntracked() != phaseShop {
		return
	}
	shop := &mustGameData().progression.Shop
	if i := shop.item(id); i >= 0 && g.menuFocus.GetUntracked() != i {
		g.menuFocus.Set(i)
	}
	prog, ok := buyShopItem(g.progress, shop, id)
	if !ok {
		g.audio.playCue(cueUIClick)
		return
	}
	g.progress = prog
	_ = saveProgression(g.storage, prog)
	g.shopVersion.Update(func(n int) int { return n + 1 })
	g.audio.playCue(cueUISelect)
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
	g.titleMotion = titleMotion{}
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
	g.overlay.openPanel()
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

func (g *emberVigil) toggleStats() {
	g.updateDraft(func(s *Settings) { s.Stats = !s.Stats })
}

func (g *emberVigil) toggleFPS() {
	g.updateDraft(func(s *Settings) { s.ShowFPS = !s.ShowFPS })
}

func (g *emberVigil) setQuality(q Quality) {
	g.updateDraft(func(s *Settings) { s.Quality = q })
}

// cycleTier moves the next run's Long Night tier by delta, wrapping over the open tiers.
func (g *emberVigil) cycleTier(delta int) {
	open := g.progress.openTier(mustGameData().tiers.maxTier())
	next := ((g.tier.GetUntracked()-1+delta)%open+open)%open + 1
	g.tier.Set(next)
	g.audio.playCue(cueUIClick)
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
		items := g.titleItems(g.tr())
		items[max(0, min(len(items)-1, focus))].press()
	case phaseShop:
		shop := &mustGameData().progression.Shop
		if focus < len(shop.Items) {
			g.buy(shop.Items[focus].ID)
		} else {
			g.closeShop()
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
			g.toggleStats()
		case 7:
			g.toggleFPS()
		case settingsRowCount:
			g.applySettings()
		case settingsRowCount + 1:
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
	size := g.canvasSize()
	if g.touchPause(ev, size) {
		return
	}
	g.updateStickLayout(size, g.cx.Env().SafeArea())
	g.pad.OnTouch(ev)
}

// canvasSize is the arena canvas's size when it last drew, or the window's before it has.
func (g *emberVigil) canvasSize() core.Size {
	if g.size.Width > 0 && g.size.Height > 0 {
		return g.size
	}
	return g.cx.Env().Window()
}

// pauseHit reports whether p, in the canvas's points, lands on the HUD's pause button while a run is being played.
func (g *emberVigil) pauseHit(p core.Point) bool {
	if g.phase.GetUntracked() != phaseArena || g.run == nil || g.run.Snapshot().Phase != PhasePlaying {
		return false
	}
	return layoutHUD(g.canvasSize(), g.cx.Env().SafeArea()).pauseHit.contains(p)
}

// touchPause presses the canvas pause button with a finger, and reports whether the touch was the button's rather than the stick's.
func (g *emberVigil) touchPause(ev core.TouchEvent, size core.Size) bool {
	hit := layoutHUD(size, g.cx.Env().SafeArea()).pauseHit
	for _, t := range ev.Touches {
		if !t.Changed {
			continue
		}
		switch ev.Phase {
		case core.TouchStart:
			if hit.contains(t.Position) {
				g.pausePushed = true
				return true
			}
		case core.TouchEnd:
			if g.pausePushed && hit.contains(t.Position) {
				g.pausePushed = false
				g.pause()
				return true
			}
		case core.TouchCancel:
			g.pausePushed = false
		}
	}
	return false
}

// onArenaPressDown and onArenaPressUp press the canvas pause button with the mouse: down holds it, up inside it pauses.
func (g *emberVigil) onArenaPressDown(ev core.PressEvent) {
	g.pausePushed = g.pauseHit(ev.Position)
}

func (g *emberVigil) onArenaPressUp(ev core.PressEvent) {
	if g.pausePushed && g.pauseHit(ev.Position) {
		g.pause()
	}
	g.pausePushed = false
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
	// The stick's zone starts below the HUD, so a thumb on it never lands on a HUD piece.
	zoneTop := max(core.Points(layoutHUD(size, safe).bottom)+stickMargin, size.Height-safe.Bottom-zoneHeight)
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

// draw draws the arena through the viewport, which keeps the virtual canvas's shape in any window, then the HUD over the lit arena at the canvas's real size.
func (g *emberVigil) draw(d *core.Drawing, size core.Size) {
	_ = g.frame.Get()
	g.size = size
	alpha := g.alpha
	safe := g.cx.Env().SafeArea()
	ph := g.phase.GetUntracked()
	lang := g.settings.GetUntracked().Language
	tr := evstrings.For(lang)
	p := &overlayPainter{d: d, size: size, art: g.hudArt, icons: g.art, m: &g.overlay, tr: tr, lang: string(lang), pressed: g.pressed}
	if ph == phaseTitle || ph == phaseCollection || ph == phaseShop || (ph == phaseSettings && g.settingsReturn == phaseTitle) {
		g.drawTitle(p, size, safe, ph != phaseTitle)
	} else {
		g.drawArena(d, p, size, safe, alpha)
	}
	if ph == phaseSettings {
		tr, cfg := p.tr, g.settings.GetUntracked()
		var rows [settingsRowCount][]settingsControl
		for i := range rows {
			rows[i] = g.settingsControls(tr, cfg, i)
		}
		drawSettings(p, layoutSettings(size, safe), rows, cfg, g.menuFocus.GetUntracked())
	}
	if ph == phaseCollection {
		entries := collectionEntries(p.tr, recipes(mustGameData()), g.progress.Collection)
		drawCollection(p, layoutCollection(size, safe, len(entries)), entries)
	}
	if ph == phaseShop {
		shop := &mustGameData().progression.Shop
		drawShop(p, layoutShop(size, safe, len(shop.Items)), shopEntries(p.tr, shop, g.progress), g.progress.Embers, g.menuFocus.GetUntracked())
	}
	if g.settings.GetUntracked().ShowFPS {
		drawFPS(p, size, safe, &g.fps)
	}
}

// drawArena draws the run through the viewport, the touch stick, the HUD and the in-run overlays.
func (g *emberVigil) drawArena(d *core.Drawing, p *overlayPainter, size core.Size, safe core.Edges, alpha float64) {
	ph := g.phase.GetUntracked()
	tr, lang := p.tr, p.lang
	viewport.Draw(d, size, func(d *core.Drawing, view core.Size) { g.scene.draw(d, alpha, view) })
	g.updateStickLayout(size, safe)
	if g.showTouchStick() {
		drawStick(d, g.stick, g.hudArt, g.pad)
	}
	if g.run == nil {
		return
	}
	st := g.hud.GetUntracked()
	drawHUD(d, layoutHUD(size, safe), st, &g.hudMotion, g.hudArt, g.art, tr, lang, ph == phaseArena, g.pausePushed)
	drawChestBanner(p, size, safe, g.run.Snapshot().Opening)
	switch {
	case ph == phasePaused:
		confirm := g.confirmQuit.GetUntracked()
		drawPause(p, layoutPause(size, safe, confirm), confirm, g.menuFocus.GetUntracked())
	case ph != phaseArena:
	case st.phase == PhaseLevelUp:
		drawCards(p, layoutCards(size, safe, len(g.run.Snapshot().Offers), st.redraws > 0), st, g.cardFocus)
	case st.phase == PhaseVictory || st.phase == PhaseDefeat:
		txt := resultsTextOf(tr, st, g.records.GetUntracked(), g.lastReward, g.progress.Embers)
		drawResults(p, layoutResults(size, safe, txt.unlocked != ""), st, txt, g.menuFocus.GetUntracked())
	}
}

// drawTitle draws the title on the canvas, its menu focused as the shell's; under settings it shows only the cover.
func (g *emberVigil) drawTitle(p *overlayPainter, size core.Size, safe core.Edges, under bool) {
	items := g.titleItems(p.tr)
	embers := titleEmbers
	if g.quality != nil {
		embers = g.quality.Preset().Particles(titleEmbers)
	}
	v := titleView{
		l:      layoutTitle(size, safe, len(items)),
		items:  items,
		focus:  g.menuFocus.GetUntracked(),
		tr:     p.tr,
		rec:    g.records.GetUntracked(),
		embers: embers,
		still:  g.cx.Env().ReduceMotion(),
	}
	v.under = under
	drawTitle(p, g.titleArt, &g.titleMotion, v)
}

// Stick piece sizes in points for a stick of stickRadius, from assets/hud/README.md.
const (
	stickBasePt = 128
	stickKnobPt = 56
)

// drawStick draws the virtual stick in the HUD kit's base and knob, idle or held, or the pad's flat circles when the kit lacks them.
func drawStick(d *core.Drawing, s *input.VirtualStick, kit *hudArt, pad *input.VirtualPad) {
	knobName := hudStickKnob
	if s.Active() {
		knobName = hudStickKnobActive
	}
	base, okBase := kit.image(hudStickBase)
	knob, okKnob := kit.image(knobName)
	if !okBase || !okKnob {
		pad.Draw(d, emberStickStyle())
		return
	}
	k := s.Radius / stickRadius
	b, n := s.BasePosition(), s.KnobPosition()
	bs, ks := stickBasePt*k, stickKnobPt*k
	d.Save()
	d.SetImageSmoothing(true)
	base.drawIn(d, float32(b.X)-bs/2, float32(b.Y)-bs/2, bs, bs)
	knob.drawIn(d, float32(n.X)-ks/2, float32(n.Y)-ks/2, ks, ks)
	d.Restore()
}
