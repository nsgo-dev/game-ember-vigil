package main

import (
	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
)

// Colors of the native screens, which read as the arena's palette on a placeholder canvas.
var (
	barColor    = core.RGB(0x0b0d17)
	textColor   = core.RGB(0xf8fafc)
	panelColor  = core.RGBA(0x111827, 0.92)
	buttonColor = core.RGB(0xc2410c)
)

// buttonSide is the shortest side of a tappable button in points, and menuButtonWidth keeps a menu's buttons alike.
const (
	buttonSide      = 44
	menuButtonWidth = 160
)

// view is the game's whole screen: the arena canvas, and over it the native views of the current screen.
func (g *emberVigil) view() core.Node {
	arena := core.View{
		TestID: "arena-view",
		Style:  fill(),
		Children: []core.Node{
			core.Canvas{
				TestID: "arena",
				Label:  Project.Title() + " arena",
				Style:  fill(),
				Draw:   g.draw,
			},
		},
	}
	g.actions.Attach(&arena)

	return core.View{
		Style: core.Style{FlexGrow: 1, FlexShrink: 1, Background: barColor},
		Children: []core.Node{
			arena,
			g.when(phaseArena, g.hud),
			g.when(phaseTitle, g.title),
			g.when(phasePaused, g.paused),
			g.when(phaseSettings, g.settings),
		},
	}
}

// when shows the views screen makes while the shell is in phase p.
func (g *emberVigil) when(p phase, screen func() core.Node) core.Node {
	return core.Show{
		When: func() bool { return g.phase.Get() == p },
		Then: func(*core.Scope) core.Node { return screen() },
	}
}

// hud is the pause button over the top-right corner of the arena; touches elsewhere reach the arena.
func (g *emberVigil) hud() core.Node {
	return core.View{
		TestID:        "hud",
		PointerEvents: core.PointerEventsBoxNone,
		Style: core.Style{
			Position:       core.Absolute,
			Top:            core.Pt(0),
			Left:           core.Pt(0),
			Right:          core.Pt(0),
			Direction:      core.Row,
			JustifyContent: core.JustifyEnd,
			Padding:        core.All(12),
			PaddingEnv:     core.EnvAll(core.EnvSafeArea),
		},
		Children: []core.Node{button("Pause", buttonSide, g.pause)},
	}
}

// title is the first screen: start a run, open the settings, or leave the app.
func (g *emberVigil) title() core.Node {
	return panel("title",
		heading(Project.Title()),
		button("Start", menuButtonWidth, g.start),
		button("Settings", menuButtonWidth, g.openSettings),
		button("Quit", menuButtonWidth, g.quitApp),
	)
}

// paused is the overlay that stops a run: continue it, or leave it for the title screen.
func (g *emberVigil) paused() core.Node {
	return panel("pause",
		heading("Paused"),
		button("Resume", menuButtonWidth, g.resume),
		button("Quit", menuButtonWidth, g.quitRun),
	)
}

// settings is a placeholder until the settings screen edits saved settings.
func (g *emberVigil) settings() core.Node {
	return panel("settings",
		heading("Settings"),
		button("Back", menuButtonWidth, g.closeSettings),
	)
}

// panel centers a card of children over the arena, padded clear of the safe area, and takes the presses that land on it.
func panel(id string, children ...core.Node) core.Node {
	return core.View{
		TestID: id,
		Style: core.Style{
			Position:       core.Absolute,
			Top:            core.Pt(0),
			Right:          core.Pt(0),
			Bottom:         core.Pt(0),
			Left:           core.Pt(0),
			AlignItems:     core.AlignCenter,
			JustifyContent: core.JustifyCenter,
			Padding:        core.All(16),
			PaddingEnv:     core.EnvAll(core.EnvSafeArea),
			Background:     core.RGBA(0x020617, 0.55),
		},
		Children: []core.Node{
			core.View{
				Style: core.Style{
					Background:   panelColor,
					CornerRadius: 14,
					Padding:      core.Symmetric(20, 28),
					AlignItems:   core.AlignCenter,
					Gap:          12,
				},
				Children: children,
			},
		},
	}
}

// heading is a screen's title.
func heading(text string) core.Node {
	return core.Text{Content: text, Style: core.Style{Color: textColor, FontSize: 26, FontWeight: core.FontWeightBold}}
}

// button is a native button called label that runs onPress when pressed by pointer, keyboard or screen reader.
func button(label string, minWidth core.Points, onPress func()) core.Node {
	return base.Button{
		Label:   label,
		OnPress: func(core.PressEvent) { onPress() },
		Style: core.Style{
			Background:     buttonColor,
			CornerRadius:   10,
			Padding:        core.Symmetric(10, 16),
			MinWidth:       core.Pt(minWidth),
			MinHeight:      core.Pt(buttonSide),
			AlignItems:     core.AlignCenter,
			JustifyContent: core.JustifyCenter,
			Active:         &core.Style{Opacity: core.Alpha(0.8)},
		},
		Children: []core.Node{
			core.Text{Content: label, Style: core.Style{Color: textColor, FontSize: 15, FontWeight: core.FontWeightBold}},
		},
	}
}

// fill is the style of a child that covers its whole parent.
func fill() core.Style {
	return core.Style{
		Position: core.Absolute,
		Top:      core.Pt(0),
		Right:    core.Pt(0),
		Bottom:   core.Pt(0),
		Left:     core.Pt(0),
	}
}
