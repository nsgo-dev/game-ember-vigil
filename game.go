package main

import (
	"time"

	"github.com/nsgo-dev/game/packages/game"
	"github.com/nsgo-dev/game/packages/game/input"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/desktop"
)

// rate is how many times a second the game steps its run, whatever the display's refresh rate.
const rate = 60

// The actions game.yaml binds, which the shell reads by name.
const (
	actionPause   = "pause"
	actionConfirm = "confirm"
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
	run     *run

	// phase is the screen that shows; frame changes on every draw so the canvas draws again.
	phase *core.Signal[phase]
	frame *core.Signal[uint64]

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
		phase:   core.NewSignal(cx, phaseTitle),
		frame:   core.NewSignal(cx, uint64(0)),
	}

	// Touches and clicks arrive in the view's points; game.yaml's regions are in virtual pixels.
	g.actions.MapPoint = func(p core.Point) (core.Point, bool) { return viewport.ViewToGame(g.size, p) }
	Project.BindInput(g.actions)
	g.actions.WatchKeys(cx)
	g.actions.WatchGamepads(cx)

	g.loop = &game.Loop{
		Rate:   rate,
		Update: g.update,
		Draw: func(float64) {
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
	if g.actions.JustPressed(actionConfirm) {
		g.confirm()
	}
	if g.phase.GetUntracked() == phaseArena {
		g.run.step(dt)
	}
}

// setPhase shows p's screen and drops held input, so a press on a menu does not reach the arena.
func (g *emberVigil) setPhase(p phase) {
	g.phase.Set(p)
	g.actions.Reset()
}

// start begins a run in an empty arena.
func (g *emberVigil) start() {
	g.run = newRun()
	g.setPhase(phaseArena)
}

// pause stops the run under the pause overlay.
func (g *emberVigil) pause() {
	if g.phase.GetUntracked() == phaseArena {
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
	viewport.Draw(d, size, drawArena)
}
