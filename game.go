package main

import (
	"fmt"
	"math"
	"time"

	"github.com/nsgo-dev/game/packages/game"
	"github.com/nsgo-dev/game/packages/game/input"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/desktop"
)

// rate is how many times a second the game steps its run, whatever the display's refresh rate, and hudRate how often the native HUD reads the run.
const (
	rate    = 60
	hudRate = 100 * time.Millisecond
)

// The actions game.yaml binds, which the shell reads by name.
const (
	actionLeft    = "left"
	actionRight   = "right"
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

// emberVigil is the game's shell: it owns the window, the fixed-step loop, input and the screens, and hands the run its steps.
type emberVigil struct {
	cx      *core.Scope
	loop    *game.Loop
	actions *input.Actions
	art     *art
	scene   *arenaRenderer
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

	// size is the arena canvas's size when it last drew, which input maps into virtual pixels.
	size core.Size
}

// Root is the game's root component.
func Root(cx *core.Scope) core.Node {
	return newEmberVigil(cx).view()
}

// newEmberVigil starts the shell on the title screen.
func newEmberVigil(cx *core.Scope) *emberVigil {
	desktop.SetTitle(cx, Project.Title())
	cx.RequestOrientation(core.OrientationLandscape)

	g := &emberVigil{
		cx:      cx,
		actions: input.NewActions(),
		art:     newArt(),
		phase:   core.NewSignal(cx, phaseTitle),
		frame:   core.NewSignal(cx, uint64(0)),
		hud:     core.NewSignal(cx, hudState{}),
	}
	g.scene = newArenaRenderer(g.art)

	// Touches and clicks arrive in the view's points; game.yaml's regions are in virtual pixels.
	g.actions.MapPoint = func(p core.Point) (core.Point, bool) { return viewport.ViewToGame(g.size, p) }
	Project.BindInput(g.actions)
	g.actions.WatchKeys(cx)
	g.actions.WatchGamepads(cx)

	g.loop = &game.Loop{
		Rate:   rate,
		Update: g.update,
		Draw: func(alpha float64) {
			g.alpha = alpha
			g.frame.Update(func(n uint64) uint64 { return n + 1 })
		},
	}
	g.loop.Run(cx)
	return g
}

// update samples input once and steps the run while the arena shows.
func (g *emberVigil) update(dt time.Duration) {
	g.actions.Step()
	if g.actions.JustPressed(actionPause) {
		g.togglePause()
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
			g.publishHUD()
		case g.actions.JustPressed(actionRight):
			g.cardFocus = (g.cardFocus + 1) % offerCount
			g.publishHUD()
		case g.actions.JustPressed(actionConfirm):
			g.choose(g.cardFocus)
			return
		}
	} else if g.actions.JustPressed(actionConfirm) {
		g.confirm()
	}
	if g.phase.GetUntracked() == phaseArena && g.run != nil {
		prevPhase := g.run.Snapshot().Phase
		g.run.Step(dt, Controls{Move: Vec2{X: g.actions.Axis(axisMoveX), Y: g.actions.Axis(axisMoveY)}})
		snap := g.run.Snapshot()
		g.scene.sync(snap, g.run.Events(), dt.Seconds())
		if snap.Phase != prevPhase {
			if snap.Phase == PhaseLevelUp {
				g.cardFocus = 0
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

// publishHUD tells the native HUD what the run shows now: its clock, health, XP, level, kills, build and cards, or nothing when no run is on.
func (g *emberVigil) publishHUD() {
	if g.run == nil {
		g.hud.Set(hudState{})
		return
	}
	s := g.run.Snapshot()
	st := hudState{
		phase:     s.Phase,
		time:      clockText(s.Time),
		health:    max(0, int(math.Ceil(s.Health))),
		maxHealth: int(math.Round(s.MaxHealth)),
		xp:        int(math.Round(s.XP)),
		xpNeeded:  int(math.Ceil(s.XPNeeded)),
		level:     s.Level,
		kills:     s.Kills,
		cardFocus: g.cardFocus,
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

// setPhase shows p's screen and drops held input, so a press on a menu does not reach the arena.
func (g *emberVigil) setPhase(p phase) {
	g.phase.Set(p)
	g.actions.Reset()
}

// start begins a run from the wall clock's seed, which the run itself never reads, and keeps that seed for the arena's ground.
func (g *emberVigil) start() {
	g.seed = uint64(time.Now().UnixNano())
	g.run = NewRun(g.seed)
	g.scene.startRun(g.seed)
	g.hudClock = 0
	g.cardFocus = 0
	g.setPhase(phaseArena)
	g.publishHUD()
}

// choose takes card index of a level-up and resumes the run behind it.
func (g *emberVigil) choose(index int) {
	if g.run != nil && g.run.Choose(index) {
		g.cardFocus = 0
		g.actions.Reset()
		g.publishHUD()
	}
}

// pause stops a run that is being played under the pause overlay, leaving one that is over or waiting for a card as it is.
func (g *emberVigil) pause() {
	if g.phase.GetUntracked() == phaseArena && g.run != nil && g.run.Snapshot().Phase == PhasePlaying {
		g.setPhase(phasePaused)
	}
}

// resume returns from the pause overlay to the run it stopped.
func (g *emberVigil) resume() {
	if g.phase.GetUntracked() == phasePaused {
		g.setPhase(phaseArena)
	}
}

// togglePause pauses a run that is being played and resumes a paused one.
func (g *emberVigil) togglePause() {
	switch g.phase.GetUntracked() {
	case phaseArena:
		g.pause()
	case phasePaused:
		g.resume()
	}
}

// quitRun ends the run and returns to the title screen.
func (g *emberVigil) quitRun() {
	g.run = nil
	g.setPhase(phaseTitle)
	g.publishHUD()
}

// openSettings shows the settings screen, a placeholder until it has settings to edit.
func (g *emberVigil) openSettings() {
	if g.phase.GetUntracked() == phaseTitle {
		g.setPhase(phaseSettings)
	}
}

// closeSettings returns from the settings screen to the title.
func (g *emberVigil) closeSettings() {
	if g.phase.GetUntracked() == phaseSettings {
		g.setPhase(phaseTitle)
	}
}

// quitApp ends the app.
func (g *emberVigil) quitApp() { g.cx.Quit() }

// confirm presses the screen's default button.
func (g *emberVigil) confirm() {
	switch g.phase.GetUntracked() {
	case phaseTitle:
		g.start()
	case phasePaused:
		g.resume()
	case phaseSettings:
		g.closeSettings()
	}
}

// draw draws the arena through the viewport, which keeps the virtual canvas's shape in any window.
func (g *emberVigil) draw(d *core.Drawing, size core.Size) {
	_ = g.frame.Get()
	g.size = size
	alpha := g.alpha
	viewport.Draw(d, size, func(d *core.Drawing, _ core.Size) { g.scene.draw(d, alpha) })
}
