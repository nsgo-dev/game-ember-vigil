package main

import (
	"strconv"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	evstrings "me.oneo/embervigil/strings"
)

// Colors of the window behind the arena and of the procedural card and HUD accents.
var (
	barColor     = core.RGB(0x0b0d17)
	healthColor  = core.RGB(0xd0453a)
	xpColor      = core.RGB(0xffb347)
	flameColor   = core.RGB(0xff7a1a)
	passiveColor = core.RGB(0x5b4a7f)
	healColor    = core.RGB(0x3f6b4a)
	pipEmpty     = core.RGBA(0x8a93a8, 0.3)
)

// buttonSide is the shortest side of a tappable button in points.
const (
	buttonSide    = 44
	maxHUDWeapons = 4
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
	// tier is the Long Night tier the run plays.
	tier    int
	weapons [maxHUDWeapons]hudSlot
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

// resultsOverlay is the native layer over the canvas's results: the outcome, the run's time, kills and level, its embers and unlocks and the records for assistive technology, and the Play Again and Title buttons.
func (g *emberVigil) resultsOverlay() core.Node {
	st := g.hud.Get()
	tr := g.tr()
	txt := resultsTextOf(tr, st, g.records.Get(), g.lastReward, g.progress.Embers)
	l := layoutResults(g.viewSize.Get(), g.cx.Env().SafeArea(), txt.unlocked != "")
	children := []core.Node{
		infoView("results-outcome", txt.outcome, l.outcome),
		infoView("results-time", txt.time, l.stats[0]),
		infoView("results-kills", txt.kills, l.stats[1]),
		infoView("results-level", txt.level, l.stats[2]),
		infoView("results-record", txt.record, l.record),
		infoView("results-embers", txt.embers, l.embers),
	}
	if txt.unlocked != "" {
		children = append(children, infoView("results-unlocked", txt.unlocked, l.unlocked))
	}
	children = append(children,
		infoView("results-best-records", txt.best, l.best),
		g.hitButton("results-play-again", tr.PlayAgain, "", l.buttons[0], g.start, func() { g.hoverMenu(0) }),
		g.hitButton("results-title", tr.TitleButton, "", l.buttons[1], g.quitRun, func() { g.hoverMenu(1) }),
	)
	return overlayLayer("results", txt.title, children...)
}

// title is the native layer over the canvas's title: what the cover, the wordmark and the records show for assistive technology, and one invisible button over each painted menu button.
func (g *emberVigil) title() core.Node {
	tr := g.tr()
	items := g.titleItems(tr)
	l := layoutTitle(g.viewSize.Get(), g.cx.Env().SafeArea(), len(items))
	art := "title-art"
	if !g.titleArt.animated() {
		art = "title-art-fallback"
	}
	records := titleRecordTexts(tr, g.records.Get())
	recordIDs := []string{"title-record-time", "title-record-kills", "title-record-level", "title-record-wins"}
	if len(records) == 1 {
		recordIDs = []string{"title-records-empty"}
	}
	heading := l.recordsHeading
	recordViews := []core.Node{infoView("title-records-heading", tr.BestRecords, rect{0, 0, heading.w, heading.h})}
	for i, text := range records {
		r := l.records[i]
		if len(records) == 1 {
			r.w *= 4
		}
		recordViews = append(recordViews, infoView(recordIDs[i], text, rect{r.x - heading.x, r.y - heading.y, r.w, r.h}))
	}
	children := []core.Node{
		core.View{TestID: art, Label: tr.TitleArtLabel, Style: fill()},
		infoView("title-heading", tr.GameTitle, l.logo),
		core.View{TestID: "title-records", Style: at(rect{heading.x, heading.y, heading.w, heading.h * 2}), Children: recordViews},
	}
	for i, it := range items {
		idx := i
		children = append(children, g.hitButton(it.id, it.label, "", l.buttons[i], it.press, func() { g.hoverMenu(idx) }))
	}
	return overlayLayer("title", tr.GameTitle, children...)
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

// hitOption is hitButton for one of a setting's choices, which tells assistive technology whether it is chosen.
func (g *emberVigil) hitOption(id, text string, r rect, selected bool, onPress, onHover func()) core.Node {
	return g.hitControl(id, text, "", r, func() bool { return selected }, onPress, onHover)
}

// hitButton is an invisible button over a button or card the canvas draws, at least buttonSide each way: it takes presses from the mouse, touch and assistive technology, holds the painted button down while pressed, and focuses it when the mouse comes over it. The keyboard and gamepad reach it through the shell's focus instead of Tab.
func (g *emberVigil) hitButton(id, text, hint string, r rect, onPress, onHover func()) core.Node {
	return g.hitControl(id, text, hint, r, nil, onPress, onHover)
}

func (g *emberVigil) hitControl(id, text, hint string, r rect, selected func() bool, onPress, onHover func()) core.Node {
	return core.View{
		Style:        at(r.grow(buttonSide)),
		OnMouseEnter: func(core.MouseEvent) { onHover() },
		Children: []core.Node{
			base.Button{
				TestID:      id,
				Label:       text,
				Hint:        hint,
				NoFocus:     true,
				Selected:    selected,
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
