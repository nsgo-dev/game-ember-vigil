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
	dangerColor    = core.RGB(0xa3341a)
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
	cardWidth       = 176
	cardMinHeight   = 186
	cardIconSize    = 36
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

// levelUp is the three-card level-up modal: each card shows an icon, name, description and level pips, and can be chosen by tap, 1/2/3 or a gamepad button.
func (g *emberVigil) levelUp() core.Node {
	st := g.hud.Get()
	tr := g.tr()
	var cards []core.Node
	for i, c := range st.offers {
		if c.name == "" {
			continue
		}
		idx := i
		focused := i == st.cardFocus
		cards = append(cards, g.upgradeCard(idx, c, focused, tr))
	}
	return panel("level-up", tr.Level(st.level),
		heading(tr.Level(st.level)),
		core.View{
			Style: core.Style{
				Direction:      core.Row,
				AlignItems:     core.AlignStretch,
				JustifyContent: core.JustifyCenter,
				Gap:            12,
				Width:          core.Pct(100),
				MaxWidth:       core.Pt(cardWidth*offerCount + 24),
			},
			Children: cards,
		},
	)
}

// upgradeCard renders one selectable level-up card button.
func (g *emberVigil) upgradeCard(index int, c hudCard, focused bool, tr evstrings.Table) core.Node {
	border := cardBorder
	if focused {
		border = xpColor
	}
	kindLabel := tr.KindPassive
	switch c.kind {
	case offerWeapon:
		kindLabel = tr.KindWeapon
	case offerHeal:
		kindLabel = tr.KindHeal
	}
	copyText := tr.Card(c.id, c.name, c.description)

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
		g.cardIcon(c.id, c.kind),
		core.Text{
			TestID:  "level-up-card-name-" + strconv.Itoa(index),
			Content: copyText.Name,
			Style:   core.Style{Color: textColor, FontSize: 15, FontWeight: core.FontWeightBold, TextAlign: core.TextAlignCenter},
		},
		core.Text{
			TestID:  "level-up-card-desc-" + strconv.Itoa(index),
			Content: copyText.Description,
			Style:   core.Style{Color: dimColor, FontSize: 12, TextAlign: core.TextAlignCenter},
		},
	}
	if c.maxLevel > 0 {
		children = append(children, levelPips("level-up-card-pips-"+strconv.Itoa(index), c.level, c.maxLevel))
	}

	return base.Button{
		TestID:  "level-up-card-" + strconv.Itoa(index),
		Label:   copyText.Name,
		OnPress: func(core.PressEvent) { g.choose(index) },
		Style: core.Style{
			FlexGrow:       1,
			FlexShrink:     1,
			MinWidth:       core.Pt(132),
			MaxWidth:       core.Pt(cardWidth),
			MinHeight:      core.Pt(156),
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

// cardIcon draws the card's embedded PNG icon when available, or a procedural glyph badge as fallback.
func (g *emberVigil) cardIcon(id, kind string) core.Node {
	if img, ok := g.art.cardIcon(id); ok {
		return core.Image{
			TestID:         "card-icon-" + id,
			Source:         img,
			ElementsHidden: true,
			Style:          core.Style{Width: core.Pt(cardIconSize), Height: core.Pt(cardIconSize)},
		}
	}
	return core.View{
		TestID: "card-icon-fallback-" + id,
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
	rec := g.records.Get()
	tr := g.tr()
	focus := g.menuFocus.Get()

	badgeText, badgeColor, titleText := tr.DefeatBadge, healthColor, tr.DefeatTitle
	if st.phase == PhaseVictory {
		badgeText, badgeColor, titleText = tr.VictoryBadge, xpColor, tr.VictoryTitle
	}
	recordText, recordColor := tr.NoNewRecord, dimColor
	if st.newRecord {
		recordText, recordColor = tr.NewRecord, xpColor
	}

	return panel("results", titleText,
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
				label("results-time", tr.Time(st.time), 15, textColor),
				label("results-kills", tr.Kills(st.kills), 15, textColor),
				label("results-level", tr.Level(st.level), 15, textColor),
				label("results-record", recordText, 13, recordColor),
			},
		},
		core.View{
			TestID: "results-best-records",
			Style: core.Style{
				Direction:      core.Row,
				AlignItems:     core.AlignCenter,
				JustifyContent: core.JustifyCenter,
				Gap:            12,
				Wrap:           core.Wrap,
			},
			Children: []core.Node{
				label("results-best-time", tr.BestTime(clockText(rec.BestTime)), 11, dimColor),
				label("results-best-kills", tr.BestKills(rec.BestKills), 11, dimColor),
				label("results-best-level", tr.BestLevel(rec.BestLevel), 11, dimColor),
				label("results-wins", tr.Wins(rec.Wins), 11, dimColor),
			},
		},
		core.View{
			Style: core.Style{Direction: core.Row, Gap: 12},
			Children: []core.Node{
				actionButton("results-play-again", tr.PlayAgain, menuButtonWidth, buttonColor, focus == 0, g.start),
				actionButton("results-title", tr.TitleButton, menuButtonWidth, secondaryColor, focus == 1, g.quitRun),
			},
		},
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

// paused is the overlay that stops a run: resume, open settings, or quit to the title screen with confirmation.
func (g *emberVigil) paused() core.Node {
	tr := g.tr()
	focus := g.menuFocus.Get()

	if g.confirmQuit.Get() {
		return panel("pause", tr.ConfirmQuitTitle,
			core.View{
				TestID: "pause-confirm-quit",
				Style:  core.Style{Direction: core.Column, AlignItems: core.AlignCenter, Gap: 12},
				Children: []core.Node{
					heading(tr.ConfirmQuitTitle),
					core.Text{
						TestID:  "pause-confirm-quit-body",
						Content: tr.ConfirmQuitBody,
						Style:   core.Style{Color: dimColor, FontSize: 13, TextAlign: core.TextAlignCenter},
					},
					core.View{
						Style: core.Style{Direction: core.Row, Gap: 12},
						Children: []core.Node{
							actionButton("pause-confirm-quit-yes", tr.ConfirmQuitYes, menuButtonWidth, dangerColor, focus == 0, g.quitRun),
							actionButton("pause-confirm-quit-no", tr.Cancel, menuButtonWidth, secondaryColor, focus == 1, g.cancelQuitRun),
						},
					},
				},
			},
		)
	}

	return panel("pause", tr.Paused,
		heading(tr.Paused),
		actionButton("pause-resume", tr.Resume, menuButtonWidth, buttonColor, focus == 0, g.resume),
		actionButton("pause-settings", tr.Settings, menuButtonWidth, secondaryColor, focus == 1, g.openSettings),
		actionButton("pause-quit", tr.QuitToTitle, menuButtonWidth, dangerColor, focus == 2, g.requestQuitRun),
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

// panel centers a card of children over the arena, padded clear of the safe area, and takes the presses that land on it.
func panel(id, accessibleLabel string, children ...core.Node) core.Node {
	return core.View{
		TestID: id,
		Label:  accessibleLabel,
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
					MaxWidth:     core.Pct(100),
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
