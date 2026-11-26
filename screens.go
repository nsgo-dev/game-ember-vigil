package main

import (
	"strconv"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
)

// Colors of the native screens, which read as the arena's palette.
var (
	barColor    = core.RGB(0x0b0d17)
	textColor   = core.RGB(0xf8fafc)
	dimColor    = core.RGB(0x8a93a8)
	panelColor  = core.RGBA(0x111827, 0.92)
	buttonColor = core.RGB(0xc2410c)
	healthColor = core.RGB(0xd0453a)
	xpColor     = core.RGB(0xffb347)
	flameColor  = core.RGB(0xff7a1a)
)

// buttonSide is the shortest side of a tappable button in points, and menuButtonWidth keeps a menu's buttons alike.
const (
	buttonSide      = 44
	menuButtonWidth = 160
)

// The HUD's sizes in points.
const (
	meterWidth  = 120
	meterHeight = 8
	hudGap      = 6
)

// hudState is what the native HUD shows, which the shell publishes at hudRate instead of reading the run every frame.
type hudState struct {
	phase Phase
	// time is the run's clock as minutes and seconds.
	time      string
	health    int
	maxHealth int
	xp        int
	xpNeeded  int
	level     int
	kills     int
	// offers are the names of the cards a level-up offers, empty when none is open.
	offers [offerCount]string
}

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
			g.when(phaseArena, func() core.Node { return dynamic(g.hudView) }),
			g.show(func() bool { return g.inRun(PhaseLevelUp) }, g.levelUp),
			g.show(func() bool { return g.inRun(PhaseDefeat) }, g.defeatOverlay),
			g.when(phaseTitle, g.title),
			g.when(phasePaused, g.paused),
			g.when(phaseSettings, g.settings),
		},
	}
}

// inRun reports whether the arena shows a run in phase p, which a level-up or a defeat shows over it.
func (g *emberVigil) inRun(p Phase) bool {
	return g.phase.Get() == phaseArena && g.hud.Get().phase == p
}

// when shows the views screen makes while the shell is in phase p.
func (g *emberVigil) when(p phase, screen func() core.Node) core.Node {
	return g.show(func() bool { return g.phase.Get() == p }, screen)
}

// dynamic builds its views again whenever what build reads changes, which a Show's branch alone would not do.
func dynamic(build func() core.Node) core.Node {
	return core.Dynamic(func(*core.Scope) core.Node { return build() })
}

// show shows the views screen makes while when reports true.
func (g *emberVigil) show(when func() bool, screen func() core.Node) core.Node {
	return core.Show{
		When: when,
		Then: func(*core.Scope) core.Node { return screen() },
	}
}

// hudView is the native bar over the arena: the run's clock, health and XP, its level, kills and weapon, and the pause button. Touches elsewhere reach the arena.
func (g *emberVigil) hudView() core.Node {
	st := g.hud.Get()
	health := float64(1)
	if st.maxHealth > 0 {
		health = float64(st.health) / float64(st.maxHealth)
	}
	xp := float64(0)
	if st.xpNeeded > 0 {
		xp = float64(st.xp) / float64(st.xpNeeded)
	}

	return core.View{
		TestID:        "hud",
		PointerEvents: core.PointerEventsBoxNone,
		Style: core.Style{
			Position:       core.Absolute,
			Top:            core.Pt(0),
			Left:           core.Pt(0),
			Right:          core.Pt(0),
			Direction:      core.Row,
			AlignItems:     core.AlignStart,
			JustifyContent: core.JustifySpaceBetween,
			Padding:        core.All(12),
			PaddingEnv:     core.EnvAll(core.EnvSafeArea),
		},
		Children: []core.Node{
			core.View{
				Style: core.Style{Direction: core.Column, AlignItems: core.AlignStart, Gap: hudGap},
				Children: []core.Node{
					core.Text{TestID: "hud-timer", Content: st.time, Style: core.Style{Color: textColor, FontSize: 18, FontWeight: core.FontWeightBold}},
					meter("hud-health", health, healthColor),
					meter("hud-xp", xp, xpColor),
					core.View{
						Style: core.Style{Direction: core.Row, Gap: 12},
						Children: []core.Node{
							label("hud-level", "Level "+strconv.Itoa(st.level), 12, textColor),
							label("hud-kills", "Kills "+strconv.Itoa(st.kills), 12, dimColor),
						},
					},
					g.weaponSlot(),
				},
			},
			button("Pause", buttonSide, g.pause),
		},
	}
}

// weaponSlot is the starter weapon's slot, which this milestone fills with Ember Bolt and no more.
func (g *emberVigil) weaponSlot() core.Node {
	return core.View{
		TestID: "hud-weapon",
		Style: core.Style{
			Direction:    core.Row,
			AlignItems:   core.AlignCenter,
			Gap:          hudGap,
			Background:   panelColor,
			CornerRadius: 6,
			Padding:      core.Symmetric(4, 8),
		},
		Children: []core.Node{
			core.View{Style: core.Style{Width: core.Pt(10), Height: core.Pt(10), CornerRadius: 5, Background: flameColor}},
			label("hud-weapon-name", "Ember Bolt", 12, textColor),
		},
	}
}

// levelUp is the minimal level-up prompt until the full cards arrive: the three offers as buttons, which take a card and resume the run.
func (g *emberVigil) levelUp() core.Node {
	st := g.hud.Get()
	children := []core.Node{heading("Level " + strconv.Itoa(st.level))}
	for i, name := range st.offers {
		if name == "" {
			continue
		}
		children = append(children, button(name, menuButtonWidth, func() { g.choose(i) }))
	}
	return panel("level-up", children...)
}

// defeatOverlay is the minimal end of a run until the full results screen arrives: what the run came to, and the two ways on.
func (g *emberVigil) defeatOverlay() core.Node {
	st := g.hud.Get()
	return panel("defeat",
		heading("The ember goes out"),
		line("Time "+st.time),
		line("Kills "+strconv.Itoa(st.kills)),
		line("Level "+strconv.Itoa(st.level)),
		button("Retry", menuButtonWidth, g.start),
		button("Title", menuButtonWidth, g.quitRun),
	)
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

// line is one quiet line of a screen, such as a run's time.
func line(text string) core.Node {
	return core.Text{Content: text, Style: core.Style{Color: dimColor, FontSize: 15}}
}

// label is a small HUD text, which tests find by id.
func label(id, text string, size core.Points, color core.Color) core.Node {
	return core.Text{TestID: id, Content: text, Style: core.Style{Color: color, FontSize: size, FontWeight: core.FontWeightBold}}
}

// meter is a bar that fills by fraction, from 0 to 1.
func meter(id string, fraction float64, color core.Color) core.Node {
	f := float32(min(max(fraction, 0), 1)) * 100
	return core.View{
		TestID: id,
		Style: core.Style{
			Width:        core.Pt(meterWidth),
			Height:       core.Pt(meterHeight),
			CornerRadius: 4,
			Overflow:     core.Hidden,
			Background:   core.RGBA(0x020617, 0.7),
		},
		Children: []core.Node{
			core.View{Style: core.Style{Width: core.Pct(f), Height: core.Pt(meterHeight), Background: color}},
		},
	}
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
