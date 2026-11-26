package main

import (
	"strconv"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
)

// Colors of the native screens, which read as the arena's palette.
var (
	barColor     = core.RGB(0x0b0d17)
	textColor    = core.RGB(0xf8fafc)
	dimColor     = core.RGB(0x8a93a8)
	panelColor   = core.RGBA(0x111827, 0.92)
	cardColor    = core.RGBA(0x1a2234, 0.96)
	cardBorder   = core.RGBA(0x3b4a66, 0.9)
	buttonColor  = core.RGB(0xc2410c)
	healthColor  = core.RGB(0xd0453a)
	xpColor      = core.RGB(0xffb347)
	flameColor   = core.RGB(0xff7a1a)
	passiveColor = core.RGB(0x5b4a7f)
	healColor    = core.RGB(0x3f6b4a)
	pipEmpty     = core.RGBA(0x8a93a8, 0.3)
)

// buttonSide is the shortest side of a tappable button in points, and menuButtonWidth keeps a menu's buttons alike.
const (
	buttonSide      = 44
	menuButtonWidth = 160
	cardWidth       = 176
	cardMinHeight   = 186
	cardIconSize    = 36
	maxHUDWeapons   = 4
)

// The HUD's sizes in points.
const (
	meterWidth  = 120
	meterHeight = 8
	hudGap      = 6
)

// hudSlot is one equipped weapon shown on the HUD.
type hudSlot struct {
	id    string
	name  string
	level int
}

// hudCard is one level-up card shown in the level-up modal.
type hudCard struct {
	id          string
	name        string
	kind        string
	description string
	level       int
	maxLevel    int
	heal        bool
}

// hudState is what the native HUD shows, which the shell publishes at hudRate instead of reading the run every frame.
type hudState struct {
	phase Phase
	// time is the run's clock as minutes and seconds.
	time          string
	health        int
	maxHealth     int
	xp            int
	xpNeeded      int
	level         int
	kills         int
	cardFocus     int
	bossActive    bool
	bossName      string
	bossHealth    int
	bossMaxHealth int
	bossPhase     int
	newRecord     bool
	weapons       [maxHUDWeapons]hudSlot
	// offers are the cards a level-up offers, empty when none is open.
	offers [offerCount]hudCard
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
			g.show(func() bool { return g.inRun(PhaseLevelUp) }, func() core.Node { return dynamic(g.levelUp) }),
			g.show(func() bool { return g.inRun(PhaseVictory) || g.inRun(PhaseDefeat) }, func() core.Node { return dynamic(g.resultsOverlay) }),
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

// hudView is the native bar over the arena: the run's clock, health and XP, its level, kills and weapons, and the pause button. Touches elsewhere reach the arena.
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

	topChildren := []core.Node{
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
				g.weaponSlots(st.weapons),
			},
		},
	}
	if st.bossActive {
		bossFrac := float64(0)
		if st.bossMaxHealth > 0 {
			bossFrac = float64(st.bossHealth) / float64(st.bossMaxHealth)
		}
		phaseTag := "Phase I"
		switch st.bossPhase {
		case 2:
			phaseTag = "Phase II"
		case 3:
			phaseTag = "Phase III"
		}
		topChildren = append(topChildren, core.View{
			TestID: "hud-boss",
			Style: core.Style{
				Direction:    core.Column,
				AlignItems:   core.AlignCenter,
				Gap:          4,
				Background:   panelColor,
				CornerRadius: 8,
				Padding:      core.Symmetric(6, 12),
			},
			Children: []core.Node{
				core.View{
					Style: core.Style{Direction: core.Row, Gap: 8, AlignItems: core.AlignCenter},
					Children: []core.Node{
						label("hud-boss-name", st.bossName, 13, textColor),
						label("hud-boss-phase", phaseTag, 11, xpColor),
					},
				},
				wideMeter("hud-boss-health", 220, 10, bossFrac, healthColor),
			},
		})
	}
	topChildren = append(topChildren, button("Pause", buttonSide, g.pause))

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
		Children: topChildren,
	}
}

// weaponSlots shows the keeper's equipped weapons, keeping the starter slot identifiable as hud-weapon / hud-weapon-name.
func (g *emberVigil) weaponSlots(slots [maxHUDWeapons]hudSlot) core.Node {
	var children []core.Node
	for i, slot := range slots {
		if slot.name == "" {
			continue
		}
		boxID, nameID := "hud-weapon-"+strconv.Itoa(i), "hud-weapon-name-"+strconv.Itoa(i)
		if i == 0 {
			boxID, nameID = "hud-weapon", "hud-weapon-name"
		}
		children = append(children, core.View{
			TestID: boxID,
			Style: core.Style{
				Direction:    core.Row,
				AlignItems:   core.AlignCenter,
				Gap:          hudGap,
				Background:   panelColor,
				CornerRadius: 6,
				Padding:      core.Symmetric(4, 8),
			},
			Children: []core.Node{
				core.View{Style: core.Style{Width: core.Pt(10), Height: core.Pt(10), CornerRadius: 5, Background: cardAccent(slot.id, offerWeapon)}},
				label(nameID, slot.name, 12, textColor),
			},
		})
	}
	if len(children) == 0 {
		return core.View{}
	}
	return core.View{
		Style:    core.Style{Direction: core.Row, Gap: hudGap, Wrap: core.Wrap},
		Children: children,
	}
}

// levelUp is the three-card level-up modal: each card shows an icon, name, description and level pips, and can be chosen by tap, 1/2/3 or a gamepad button.
func (g *emberVigil) levelUp() core.Node {
	st := g.hud.Get()
	var cards []core.Node
	for i, c := range st.offers {
		if c.name == "" {
			continue
		}
		idx := i
		focused := i == st.cardFocus
		cards = append(cards, g.upgradeCard(idx, c, focused))
	}
	return panel("level-up",
		heading("Level "+strconv.Itoa(st.level)),
		core.View{
			Style: core.Style{
				Direction:      core.Row,
				AlignItems:     core.AlignStretch,
				JustifyContent: core.JustifyCenter,
				Gap:            12,
			},
			Children: cards,
		},
	)
}

// upgradeCard renders one selectable level-up card button.
func (g *emberVigil) upgradeCard(index int, c hudCard, focused bool) core.Node {
	border := cardBorder
	if focused {
		border = xpColor
	}
	kindLabel := "PASSIVE"
	switch c.kind {
	case offerWeapon:
		kindLabel = "WEAPON"
	case offerHeal:
		kindLabel = "HEAL"
	}

	children := []core.Node{
		core.View{
			Style: core.Style{
				Direction:      core.Row,
				AlignItems:     core.AlignCenter,
				JustifyContent: core.JustifySpaceBetween,
				Width:          core.Pct(100),
			},
			Children: []core.Node{
				core.Text{Content: kindLabel, Style: core.Style{Color: cardAccent(c.id, c.kind), FontSize: 10, FontWeight: core.FontWeightBold}},
				core.View{
					Style: core.Style{
						Background:     panelColor,
						CornerRadius:   4,
						Padding:        core.Symmetric(2, 6),
						AlignItems:     core.AlignCenter,
						JustifyContent: core.JustifyCenter,
					},
					Children: []core.Node{
						core.Text{Content: strconv.Itoa(index + 1), Style: core.Style{Color: dimColor, FontSize: 10, FontWeight: core.FontWeightBold}},
					},
				},
			},
		},
		cardIcon(c.id, c.kind),
		core.Text{
			TestID:  "level-up-card-name-" + strconv.Itoa(index),
			Content: c.name,
			Style:   core.Style{Color: textColor, FontSize: 15, FontWeight: core.FontWeightBold, TextAlign: core.TextAlignCenter},
		},
		core.Text{
			TestID:  "level-up-card-desc-" + strconv.Itoa(index),
			Content: c.description,
			Style:   core.Style{Color: dimColor, FontSize: 12, TextAlign: core.TextAlignCenter},
		},
	}
	if c.maxLevel > 0 {
		children = append(children, levelPips("level-up-card-pips-"+strconv.Itoa(index), c.level, c.maxLevel))
	}

	return base.Button{
		TestID:  "level-up-card-" + strconv.Itoa(index),
		Label:   c.name,
		OnPress: func(core.PressEvent) { g.choose(index) },
		Style: core.Style{
			Width:          core.Pt(cardWidth),
			MinWidth:       core.Pt(buttonSide),
			MinHeight:      core.Pt(cardMinHeight),
			Background:     cardColor,
			BorderWidth:    2,
			BorderColor:    border,
			CornerRadius:   12,
			Padding:        core.All(12),
			Direction:      core.Column,
			AlignItems:     core.AlignCenter,
			JustifyContent: core.JustifySpaceBetween,
			Gap:            8,
			Active:         &core.Style{Opacity: core.Alpha(0.85)},
		},
		Children: children,
	}
}

// cardIconImage is the hook where assets/icons/<id>.png will be returned once #13 and #15 embed stylized card icons.
func cardIconImage(id string) (core.ImageSource, bool) {
	return core.ImageSource{}, false
}

// cardIcon draws the card's embedded icon when available, or a procedural glyph badge as fallback.
func cardIcon(id, kind string) core.Node {
	if img, ok := cardIconImage(id); ok {
		return core.Image{
			Source: img,
			Style:  core.Style{Width: core.Pt(cardIconSize), Height: core.Pt(cardIconSize)},
		}
	}
	return core.View{
		Style: core.Style{
			Width:          core.Pt(cardIconSize),
			Height:         core.Pt(cardIconSize),
			CornerRadius:   10,
			Background:     cardAccent(id, kind),
			AlignItems:     core.AlignCenter,
			JustifyContent: core.JustifyCenter,
		},
		Children: []core.Node{
			core.Text{
				Content: cardGlyph(id),
				Style:   core.Style{Color: textColor, FontSize: 15, FontWeight: core.FontWeightBold},
			},
		},
	}
}

// cardAccent picks a palette color for a card's kind and id.
func cardAccent(id, kind string) core.Color {
	switch id {
	case "ember-bolt":
		return flameColor
	case "lantern-halo":
		return xpColor
	case "cinder-trail":
		return healthColor
	case "bell-toll":
		return core.RGB(0x8a93a8)
	}
	switch kind {
	case offerWeapon:
		return flameColor
	case offerHeal:
		return healColor
	default:
		return passiveColor
	}
}

// cardGlyph returns a short fallback symbol for a card until stylized icon art lands.
func cardGlyph(id string) string {
	switch id {
	case "ember-bolt":
		return "EB"
	case "lantern-halo":
		return "LH"
	case "cinder-trail":
		return "CT"
	case "bell-toll":
		return "BT"
	case "wick":
		return "WK"
	case "oil":
		return "OL"
	case "reach":
		return "RC"
	case "step":
		return "ST"
	case "warmth":
		return "WM"
	case "mending":
		return "MD"
	case "gathering":
		return "GT"
	case "glow":
		return "GL"
	case "ember-heal":
		return "+"
	default:
		return "*"
	}
}

// levelPips draws maxLevel dots showing owned levels, the incoming level and remaining levels.
func levelPips(id string, nextLevel, maxLevel int) core.Node {
	pips := make([]core.Node, 0, maxLevel)
	for i := range maxLevel {
		color := pipEmpty
		switch {
		case i < nextLevel-1:
			color = flameColor
		case i == nextLevel-1:
			color = xpColor
		}
		pips = append(pips, core.View{
			Style: core.Style{
				Width:        core.Pt(8),
				Height:       core.Pt(8),
				CornerRadius: 4,
				Background:   color,
			},
		})
	}
	return core.View{
		TestID:   id,
		Style:    core.Style{Direction: core.Row, Gap: 5, AlignItems: core.AlignCenter},
		Children: pips,
	}
}

// resultsOverlay is the end-of-run screen for victory and defeat: outcome, time, kills, level, record status and actions.
func (g *emberVigil) resultsOverlay() core.Node {
	st := g.hud.Get()
	badgeText, badgeColor, titleText := "DEFEAT", healthColor, "The ember goes out"
	if st.phase == PhaseVictory {
		badgeText, badgeColor, titleText = "VICTORY", xpColor, "Dawn breaks"
	}
	recordText, recordColor := "No new record", dimColor
	if st.newRecord {
		recordText, recordColor = "New record", xpColor
	}
	return panel("results",
		label("results-outcome", badgeText, 12, badgeColor),
		heading(titleText),
		core.View{
			Style: core.Style{
				Direction:    core.Column,
				AlignItems:   core.AlignCenter,
				Gap:          6,
				Background:   cardColor,
				CornerRadius: 10,
				Padding:      core.Symmetric(12, 24),
			},
			Children: []core.Node{
				label("results-time", "Time "+st.time, 15, textColor),
				label("results-kills", "Kills "+strconv.Itoa(st.kills), 15, textColor),
				label("results-level", "Level "+strconv.Itoa(st.level), 15, textColor),
				label("results-record", recordText, 13, recordColor),
			},
		},
		core.View{
			Style: core.Style{Direction: core.Row, Gap: 12},
			Children: []core.Node{
				button("Play Again", menuButtonWidth, g.start),
				button("Title", menuButtonWidth, g.quitRun),
			},
		},
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

// label is a small HUD text, which tests find by id.
func label(id, text string, size core.Points, color core.Color) core.Node {
	return core.Text{TestID: id, Content: text, Style: core.Style{Color: color, FontSize: size, FontWeight: core.FontWeightBold}}
}

// meter is a bar that fills by fraction, from 0 to 1.
func meter(id string, fraction float64, color core.Color) core.Node {
	return wideMeter(id, meterWidth, meterHeight, fraction, color)
}

// wideMeter is a bar of explicit size that fills by fraction, from 0 to 1.
func wideMeter(id string, width, height core.Points, fraction float64, color core.Color) core.Node {
	f := float32(min(max(fraction, 0), 1)) * 100
	return core.View{
		TestID: id,
		Style: core.Style{
			Width:        core.Pt(width),
			Height:       core.Pt(height),
			CornerRadius: 4,
			Overflow:     core.Hidden,
			Background:   core.RGBA(0x020617, 0.7),
		},
		Children: []core.Node{
			core.View{Style: core.Style{Width: core.Pct(f), Height: core.Pt(height), Background: color}},
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
