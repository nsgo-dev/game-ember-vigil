package main

import (
	"math"
	"strconv"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	evstrings "me.oneo/embervigil/strings"
)

// Colors of the native screens, which read as the arena's palette.
var (
	barColor       = core.RGB(0x0b0d17)
	textColor      = core.RGB(0xf8fafc)
	dimColor       = core.RGB(0x8a93a8)
	panelColor     = core.RGBA(0x111827, 0.92)
	titleScrim     = core.RGBA(0x0b0d17, 0.78)
	cardColor      = core.RGBA(0x1a2234, 0.96)
	cardBorder     = core.RGBA(0x3b4a66, 0.9)
	buttonColor    = core.RGB(0xc2410c)
	secondaryColor = core.RGB(0x27314a)
	healthColor    = core.RGB(0xd0453a)
	xpColor        = core.RGB(0xffb347)
	flameColor     = core.RGB(0xff7a1a)
	passiveColor   = core.RGB(0x5b4a7f)
	healColor      = core.RGB(0x3f6b4a)
	pipEmpty       = core.RGBA(0x8a93a8, 0.3)
)

// buttonSide is the shortest side of a tappable button in points, and menuButtonWidth keeps a menu's buttons alike.
const (
	buttonSide      = 44
	menuButtonWidth = 176
	maxHUDWeapons   = 4
)

// hudSlot is one equipped weapon shown on the HUD.
type hudSlot struct {
	id       string
	name     string
	level    int
	maxLevel int
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

// hudState is what the HUD and the native overlays show, which the shell publishes at hudRate instead of reading the run every frame.
type hudState struct {
	phase Phase
	// time is the run's clock as minutes and seconds.
	time      string
	health    int
	maxHealth int
	// lowHealthAt is the health fraction at or under which the lantern flickers, from the run's data.
	lowHealthAt float64
	xp          int
	xpNeeded    int
	level       int
	kills       int
	cardFocus   int
	bossActive  bool
	// bossElite says the bar shows a Gloomwarden rather than the Hollow King.
	bossElite     bool
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
				TestID:    "arena",
				LabelFunc: func() string { return hudLabel(g.tr(), g.hud.Get()) },
				Style:     fill(),
				Draw:      g.draw,
			},
		},
	}
	arena.OnTouch = g.onArenaTouch
	arena.OnPressDown = g.onArenaPressDown
	arena.OnPressUp = g.onArenaPressUp
	arena.OnLayout = func(b core.Bounds) {
		if b.Size != g.viewSize.GetUntracked() {
			g.viewSize.Set(b.Size)
		}
	}
	g.actions.Attach(&arena)

	return core.View{
		Style: core.Style{FlexGrow: 1, FlexShrink: 1, Background: barColor},
		Children: []core.Node{
			arena,
			g.show(func() bool { return g.inRun(PhaseLevelUp) }, func() core.Node { return dynamic(g.levelUp) }),
			g.show(func() bool { return g.inRun(PhaseVictory) || g.inRun(PhaseDefeat) }, func() core.Node { return dynamic(g.resultsOverlay) }),
			g.when(phaseTitle, func() core.Node { return dynamic(g.title) }),
			g.when(phasePaused, func() core.Node { return dynamic(g.paused) }),
			g.when(phaseSettings, func() core.Node { return dynamic(g.settingsView) }),
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

// levelUp is the native layer over the canvas's level-up cards: one invisible button over each card, which a tap, a click or assistive technology presses and the mouse focuses.
func (g *emberVigil) levelUp() core.Node {
	st := g.hud.Get()
	tr := g.tr()
	n := 0
	for _, c := range st.offers {
		if c.name != "" {
			n++
		}
	}
	l := layoutCards(g.viewSize.Get(), g.cx.Env().SafeArea(), n)
	var cards []core.Node
	for i, c := range st.offers {
		if c.name == "" {
			continue
		}
		idx := i
		copyText := tr.Card(c.id, c.name, c.description)
		cards = append(cards, g.hitButton("level-up-card-"+strconv.Itoa(i), copyText.Name, copyText.Description, l.cards[i],
			func() { g.choose(idx) }, func() { g.focusCard(idx) }))
	}
	return overlayLayer("level-up", tr.Level(st.level), cards...)
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

// resultsOverlay is the native layer over the canvas's results: the outcome, the run's time, kills and level and the records for assistive technology, and the Play Again and Title buttons.
func (g *emberVigil) resultsOverlay() core.Node {
	st := g.hud.Get()
	tr := g.tr()
	txt := resultsTextOf(tr, st, g.records.Get())
	l := layoutResults(g.viewSize.Get(), g.cx.Env().SafeArea())
	return overlayLayer("results", txt.title,
		infoView("results-outcome", txt.outcome, l.outcome),
		infoView("results-time", txt.time, l.stats[0]),
		infoView("results-kills", txt.kills, l.stats[1]),
		infoView("results-level", txt.level, l.stats[2]),
		infoView("results-record", txt.record, l.record),
		infoView("results-best-records", txt.best, l.best),
		g.hitButton("results-play-again", tr.PlayAgain, "", l.buttons[0], g.start, func() { g.hoverMenu(0) }),
		g.hitButton("results-title", tr.TitleButton, "", l.buttons[1], g.quitRun, func() { g.hoverMenu(1) }),
	)
}

// title is the first screen: full-bleed title key art (or dark ash-and-ember fallback) with title, best records and menu buttons in the left third.
func (g *emberVigil) title() core.Node {
	tr := g.tr()
	rec := g.records.Get()
	focus := g.menuFocus.Get()

	var backdrop core.Node
	if img, ok := g.art.titleKeyArt(); ok {
		backdrop = core.Image{
			TestID:     "title-art",
			Label:      tr.TitleArtLabel,
			Source:     img,
			ResizeMode: core.Cover,
			Style:      fill(),
		}
	} else {
		backdrop = core.View{
			TestID: "title-art-fallback",
			Style: core.Style{
				Position:   core.Absolute,
				Top:        core.Pt(0),
				Right:      core.Pt(0),
				Bottom:     core.Pt(0),
				Left:       core.Pt(0),
				Background: barColor,
			},
		}
	}

	var recordChildren []core.Node
	if rec.HasRecord {
		recordChildren = []core.Node{
			label("title-records-heading", tr.BestRecords, 12, xpColor),
			label("title-record-time", tr.BestTime(clockText(rec.BestTime)), 12, textColor),
			label("title-record-kills", tr.BestKills(rec.BestKills), 12, textColor),
			label("title-record-level", tr.BestLevel(rec.BestLevel), 12, textColor),
			label("title-record-wins", tr.Wins(rec.Wins), 12, dimColor),
		}
	} else {
		recordChildren = []core.Node{
			label("title-records-heading", tr.BestRecords, 12, xpColor),
			label("title-records-empty", tr.NoRecordsYet, 12, dimColor),
		}
	}

	return core.View{
		TestID: "title",
		Label:  tr.GameTitle,
		Style: core.Style{
			Position:   core.Absolute,
			Top:        core.Pt(0),
			Right:      core.Pt(0),
			Bottom:     core.Pt(0),
			Left:       core.Pt(0),
			Direction:  core.Row,
			AlignItems: core.AlignStretch,
			Background: barColor,
		},
		Children: []core.Node{
			backdrop,
			core.View{
				Style: core.Style{
					Width:          core.Pct(36),
					MinWidth:       core.Pt(240),
					MaxWidth:       core.Pt(380),
					Height:         core.Pct(100),
					Background:     titleScrim,
					Padding:        core.Symmetric(14, 20),
					PaddingEnv:     core.EnvAll(core.EnvSafeArea),
					Direction:      core.Column,
					AlignItems:     core.AlignStart,
					JustifyContent: core.JustifyCenter,
					Gap:            10,
				},
				Children: []core.Node{
					core.Text{
						TestID:  "title-heading",
						Content: tr.GameTitle,
						Style:   core.Style{Color: textColor, FontSize: 28, FontWeight: core.FontWeightBold},
					},
					core.View{
						TestID: "title-records",
						Style: core.Style{
							Direction:    core.Column,
							AlignItems:   core.AlignStart,
							Gap:          3,
							Background:   cardColor,
							BorderWidth:  1,
							BorderColor:  cardBorder,
							CornerRadius: 10,
							Padding:      core.Symmetric(10, 14),
							MinWidth:     core.Pt(menuButtonWidth),
						},
						Children: recordChildren,
					},
					actionButton("title-start", tr.Start, menuButtonWidth, buttonColor, focus == 0, g.start),
					actionButton("title-settings", tr.Settings, menuButtonWidth, secondaryColor, focus == 1, g.openSettings),
					actionButton("title-quit", tr.Quit, menuButtonWidth, secondaryColor, focus == 2, g.quitApp),
				},
			},
		},
	}
}

// paused is the native layer over the canvas's pause panel: resume, open settings, or quit to the title screen with confirmation.
func (g *emberVigil) paused() core.Node {
	tr := g.tr()
	size, safe := g.viewSize.Get(), g.cx.Env().SafeArea()
	if g.confirmQuit.Get() {
		l := layoutPause(size, safe, true)
		return overlayLayer("pause", tr.Paused,
			core.View{
				TestID: "pause-confirm-quit",
				Label:  tr.ConfirmQuitTitle,
				Hint:   tr.ConfirmQuitBody,
				Style:  fill(),
				Children: []core.Node{
					g.hitButton("pause-confirm-quit-yes", tr.ConfirmQuitYes, tr.ConfirmQuitBody, l.buttons[0], g.quitRun, func() { g.hoverMenu(0) }),
					g.hitButton("pause-confirm-quit-no", tr.Cancel, "", l.buttons[1], g.cancelQuitRun, func() { g.hoverMenu(1) }),
				},
			},
		)
	}
	l := layoutPause(size, safe, false)
	return overlayLayer("pause", tr.Paused,
		g.hitButton("pause-resume", tr.Resume, "", l.buttons[0], g.resume, func() { g.hoverMenu(0) }),
		g.hitButton("pause-settings", tr.Settings, "", l.buttons[1], g.openSettings, func() { g.hoverMenu(1) }),
		g.hitButton("pause-quit", tr.QuitToTitle, "", l.buttons[2], g.requestQuitRun, func() { g.hoverMenu(2) }),
	)
}

// settingsView edits audio volumes, screen shake, graphics quality and language, with Apply and Back actions.
func (g *emberVigil) settingsView() core.Node {
	cfg := g.settings.Get()
	tr := g.tr()
	focus := g.menuFocus.Get()

	shakeLabel := tr.Off
	shakeBg := secondaryColor
	if cfg.ScreenShake {
		shakeLabel = tr.On
		shakeBg = buttonColor
	}

	qualityButtons := make([]core.Node, 0, len(qualityTiers))
	for _, q := range qualityTiers {
		tier := q
		selected := cfg.Quality == tier
		qualityButtons = append(qualityButtons, optionButton(
			"settings-quality-"+string(tier),
			qualityLabel(tr, tier),
			selected,
			func() { g.setQuality(tier) },
		))
	}

	langButtons := []core.Node{
		optionButton("settings-lang-en", tr.LanguageEN, cfg.Language == evstrings.EN, func() { g.setLanguage(evstrings.EN) }),
		optionButton("settings-lang-zh-CN", tr.LanguageZhCN, cfg.Language == evstrings.ZhCN, func() { g.setLanguage(evstrings.ZhCN) }),
	}

	return core.View{
		TestID: "settings",
		Label:  tr.SettingsTitle,
		Style: core.Style{
			Position:       core.Absolute,
			Top:            core.Pt(0),
			Right:          core.Pt(0),
			Bottom:         core.Pt(0),
			Left:           core.Pt(0),
			AlignItems:     core.AlignCenter,
			JustifyContent: core.JustifyCenter,
			Padding:        core.All(12),
			PaddingEnv:     core.EnvAll(core.EnvSafeArea),
			Background:     core.RGBA(0x020617, 0.65),
		},
		Children: []core.Node{
			core.Scroll{
				Style: core.Style{
					MaxWidth:  core.Pct(100),
					MaxHeight: core.Pct(100),
				},
				Children: []core.Node{
					core.View{
						Style: core.Style{
							Background:   panelColor,
							CornerRadius: 14,
							Padding:      core.Symmetric(14, 20),
							AlignItems:   core.AlignStretch,
							Gap:          6,
							MinWidth:     core.Pt(460),
							MaxWidth:     core.Pt(520),
						},
						Children: []core.Node{
							core.Text{
								TestID:  "settings-heading",
								Content: tr.SettingsTitle,
								Style:   core.Style{Color: textColor, FontSize: 22, FontWeight: core.FontWeightBold, TextAlign: core.TextAlignCenter},
							},
							g.volumeRow("master", tr.MasterVolume, cfg.MasterVolume, focus == 0, tr, g.adjustMasterVolume),
							g.volumeRow("music", tr.MusicVolume, cfg.MusicVolume, focus == 1, tr, g.adjustMusicVolume),
							g.volumeRow("sfx", tr.SFXVolume, cfg.SFXVolume, focus == 2, tr, g.adjustSFXVolume),
							settingsRow(tr.ScreenShake, focus == 3,
								actionButton("settings-shake", shakeLabel, 100, shakeBg, false, g.toggleShake),
							),
							settingsRow(tr.Quality, focus == 4,
								core.View{
									Style:    core.Style{Direction: core.Row, Gap: 6},
									Children: qualityButtons,
								},
							),
							settingsRow(tr.Language, focus == 5,
								core.View{
									Style:    core.Style{Direction: core.Row, Gap: 8},
									Children: langButtons,
								},
							),
							core.View{
								Style: core.Style{
									Direction:      core.Row,
									JustifyContent: core.JustifyCenter,
									Gap:            12,
									Margin:         core.Edges{Top: 6},
								},
								Children: []core.Node{
									actionButton("settings-apply", tr.Apply, menuButtonWidth, buttonColor, focus == 6, g.applySettings),
									actionButton("settings-back", tr.Back, menuButtonWidth, secondaryColor, focus == 7, g.closeSettings),
								},
							},
						},
					},
				},
			},
		},
	}
}

// volumeRow renders one volume setting with - / + buttons and a percentage readout.
func (g *emberVigil) volumeRow(key, name string, value float64, focused bool, tr evstrings.Table, adjust func(float64)) core.Node {
	pct := strconv.Itoa(int(math.Round(value*100))) + "%"
	return settingsRow(name, focused,
		core.View{
			Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8},
			Children: []core.Node{
				stepButton("settings-"+key+"-dec", "-", tr.DecreaseLabel(name), func() { adjust(-volumeStep) }),
				core.Text{
					TestID:  "settings-" + key + "-value",
					Content: pct,
					Style:   core.Style{Color: textColor, FontSize: 14, FontWeight: core.FontWeightBold, Width: core.Pt(48), TextAlign: core.TextAlignCenter},
				},
				stepButton("settings-"+key+"-inc", "+", tr.IncreaseLabel(name), func() { adjust(volumeStep) }),
			},
		},
	)
}

// settingsRow pairs a setting label on the left with its control on the right.
func settingsRow(name string, focused bool, control core.Node) core.Node {
	border := core.RGBA(0, 0)
	if focused {
		border = xpColor
	}
	return core.View{
		Style: core.Style{
			Direction:      core.Row,
			AlignItems:     core.AlignCenter,
			JustifyContent: core.JustifySpaceBetween,
			Gap:            12,
			Padding:        core.Symmetric(2, 6),
			CornerRadius:   8,
			BorderWidth:    2,
			BorderColor:    border,
		},
		Children: []core.Node{
			core.Text{
				Content: name,
				Style:   core.Style{Color: textColor, FontSize: 14, FontWeight: core.FontWeightBold, MinWidth: core.Pt(124)},
			},
			control,
		},
	}
}

// qualityLabel returns the localized name of a quality tier.
func qualityLabel(tr evstrings.Table, q Quality) string {
	switch q {
	case QualityLow:
		return tr.QualityLow
	case QualityMedium:
		return tr.QualityMedium
	case QualityHigh:
		return tr.QualityHigh
	default:
		return tr.QualityAuto
	}
}

// overlayLayer covers the arena with the native layer of a screen the canvas draws, which takes the presses that land on it.
func overlayLayer(id, accessibleLabel string, children ...core.Node) core.Node {
	return core.View{TestID: id, Label: accessibleLabel, Style: fill(), Children: children}
}

// at is the style of a view placed over r of the canvas.
func at(r rect) core.Style {
	return core.Style{
		Position: core.Absolute,
		Left:     core.Pt(core.Points(r.x)),
		Top:      core.Pt(core.Points(r.y)),
		Width:    core.Pt(core.Points(r.w)),
		Height:   core.Pt(core.Points(r.h)),
	}
}

// infoView tells assistive technology what the canvas draws over r.
func infoView(id, text string, r rect) core.Node {
	return core.View{TestID: id, Label: text, Style: at(r)}
}

// hitButton is an invisible button over a button or card the canvas draws, at least buttonSide each way: it takes presses from the mouse, touch and assistive technology, holds the painted button down while pressed, and focuses it when the mouse comes over it. The keyboard and gamepad reach it through the shell's focus instead of Tab.
func (g *emberVigil) hitButton(id, text, hint string, r rect, onPress, onHover func()) core.Node {
	return core.View{
		Style:        at(r.grow(buttonSide)),
		OnMouseEnter: func(core.MouseEvent) { onHover() },
		Children: []core.Node{
			base.Button{
				TestID:      id,
				Label:       text,
				Hint:        hint,
				NoFocus:     true,
				OnPressDown: func(core.PressEvent) { g.pressed = id },
				OnPressUp: func(core.PressEvent) {
					if g.pressed == id {
						g.pressed = ""
					}
				},
				OnPress: func(core.PressEvent) {
					g.pressed = ""
					onPress()
				},
				Style: core.Style{Width: core.Pct(100), Height: core.Pct(100)},
			},
		},
	}
}

// label is a small HUD text, which tests find by id.
func label(id, text string, size core.Points, color core.Color) core.Node {
	return core.Text{TestID: id, Content: text, Style: core.Style{Color: color, FontSize: size, FontWeight: core.FontWeightBold}}
}

// actionButton is a touch-friendly native button with an accessible label, TestID and optional focus ring.
func actionButton(id, text string, minWidth core.Points, bg core.Color, focused bool, onPress func()) core.Node {
	border := core.RGBA(0, 0)
	if focused {
		border = xpColor
	}
	return base.Button{
		TestID:  id,
		Label:   text,
		OnPress: func(core.PressEvent) { onPress() },
		Style: core.Style{
			Background:     bg,
			BorderWidth:    2,
			BorderColor:    border,
			CornerRadius:   10,
			Padding:        core.Symmetric(8, 16),
			MinWidth:       core.Pt(minWidth),
			MinHeight:      core.Pt(buttonSide),
			AlignItems:     core.AlignCenter,
			JustifyContent: core.JustifyCenter,
			Active:         &core.Style{Opacity: core.Alpha(0.8)},
		},
		Children: []core.Node{
			core.Text{Content: text, Style: core.Style{Color: textColor, FontSize: 15, FontWeight: core.FontWeightBold}},
		},
	}
}

// stepButton is a square touch-friendly stepper button used for volume adjustments.
func stepButton(id, glyph, accessibleLabel string, onPress func()) core.Node {
	return base.Button{
		TestID:  id,
		Label:   accessibleLabel,
		OnPress: func(core.PressEvent) { onPress() },
		Style: core.Style{
			Background:     secondaryColor,
			CornerRadius:   8,
			MinWidth:       core.Pt(buttonSide),
			MinHeight:      core.Pt(buttonSide),
			AlignItems:     core.AlignCenter,
			JustifyContent: core.JustifyCenter,
			Active:         &core.Style{Opacity: core.Alpha(0.8)},
		},
		Children: []core.Node{
			core.Text{Content: glyph, Style: core.Style{Color: textColor, FontSize: 16, FontWeight: core.FontWeightBold}},
		},
	}
}

// optionButton is a selectable choice pill for quality tiers and languages.
func optionButton(id, text string, selected bool, onPress func()) core.Node {
	bg := secondaryColor
	border := cardBorder
	if selected {
		bg = buttonColor
		border = xpColor
	}
	return base.Button{
		TestID:   id,
		Label:    text,
		Selected: func() bool { return selected },
		OnPress:  func(core.PressEvent) { onPress() },
		Style: core.Style{
			Background:     bg,
			BorderWidth:    2,
			BorderColor:    border,
			CornerRadius:   8,
			Padding:        core.Symmetric(6, 12),
			MinWidth:       core.Pt(64),
			MinHeight:      core.Pt(buttonSide),
			AlignItems:     core.AlignCenter,
			JustifyContent: core.JustifyCenter,
			Active:         &core.Style{Opacity: core.Alpha(0.8)},
		},
		Children: []core.Node{
			core.Text{Content: text, Style: core.Style{Color: textColor, FontSize: 13, FontWeight: core.FontWeightBold}},
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
