package main

import (
	"math"
	"strconv"
	"strings"
	"unicode"

	"github.com/nsgo-dev/nsgo/core"
	evstrings "me.oneo/embervigil/strings"
)

// The overlays' kit pieces: the nine-slice panel that frames pause, results and the painted buttons, and the two card frames.
const (
	hudPanel       = "panel_frame"
	hudCardCommon  = "card_common"
	hudCardFocused = "card_focused"
)

// The nine-slice insets in the kit's pixels, from assets/hud/README.md: top, right, bottom and left.
var panelInsets = [4]float32{44, 44, 44, 44}

// Card geometry in points at a card scale of 1. A card keeps the kit's 304×400 shape, because its crests stretch badly; its content sits inside the frame's insets.
const (
	cardBaseW            = 176
	cardKitW, cardKitH   = 304, 400
	cardInsetTop         = 60
	cardInsetSide        = 44
	cardInsetBottom      = 56
	cardGapPt            = 16
	cardLiftPt           = 14
	cardRedrawGap        = 14
	overlayMargin        = 16
	overlayHeadingPt     = 52
	overlayButtonW       = 176
	overlayButtonH       = 50
	overlayButtonGap     = 10
	overlayPad           = 24
	overlayMinScale      = 0.9
	overlayMaxScale      = 1.4
	cardMaxScaleOfHUD    = 1.25
	pausePanelW          = 300
	confirmPanelW        = 400
	resultsPanelW        = 440
	resultsStatW         = 120
	resultsStatH         = 48
	resultsCaptionH      = 16
	resultsBestLineH     = 14
	resultsLineH         = 16
	overlayHeadingFontPt = 26
)

// How the overlays move, in seconds: a card's flight in and the stagger between cards, the chosen card's burn, how fast a focused card lifts, and a panel's fade in.
const (
	dealTime    = 0.42
	dealStagger = 0.09
	burnTime    = 0.55
	liftRate    = 14
	panelInTime = 0.22
)

// Overlay colours over the HUD's.
var (
	overlayScrim = core.RGB(0x05040a)
	overlayDim   = core.RGB(0xb8a98a)
	overlayEmber = core.RGB(0xff9a3c)
)

// overlayMotion is how the level-up cards and the panels move: the deal, the focused card's lift, the chosen card's burn and a panel's fade. It runs on the shell's fixed steps and never touches the run.
type overlayMotion struct {
	deal     float64
	lift     [offerCount]float64
	burning  bool
	burnCard int
	burn     float64
	panel    float64
	clock    float64
}

// dealCards starts a new hand: every card flies in again from below.
func (m *overlayMotion) dealCards() {
	m.deal, m.lift, m.burning, m.burn = 0, [offerCount]float64{}, false, 0
}

// openPanel fades a pause or results panel in.
func (m *overlayMotion) openPanel() { m.panel = 0 }

// burnAway starts burning card index, unless a card already burns.
func (m *overlayMotion) burnAway(index int) bool {
	if m.burning {
		return false
	}
	m.burning, m.burnCard, m.burn = true, index, 0
	return true
}

// burnt reports a card that has finished burning and forgets it, so the shell applies the choice once.
func (m *overlayMotion) burnt() (int, bool) {
	if !m.burning || m.burn < 1 {
		return 0, false
	}
	m.burning = false
	return m.burnCard, true
}

// step moves the overlays dt seconds on, lifting the card at focus.
func (m *overlayMotion) step(dt float64, focus int) {
	m.deal += dt
	m.panel += dt
	m.clock += dt
	k := 1 - math.Exp(-liftRate*dt)
	for i := range m.lift {
		want := 0.0
		if i == focus {
			want = 1
		}
		m.lift[i] += (want - m.lift[i]) * k
	}
	if m.burning {
		m.burn = min(1, m.burn+dt/burnTime)
	}
}

// dealt is how far card i has flown in, from 0 to 1.
func (m *overlayMotion) dealt(i int) float64 {
	return max(0, min(1, (m.deal-float64(i)*dealStagger)/dealTime))
}

// panelIn is how far a panel has faded in, from 0 to 1.
func (m *overlayMotion) panelIn() float64 { return max(0, min(1, m.panel/panelInTime)) }

func easeOut(t float64) float64 { return 1 - (1-t)*(1-t)*(1-t) }

// easeOutBack overshoots a little before it settles, which reads as a card landing.
func easeOutBack(t float64) float64 {
	const c = 1.4
	u := t - 1
	return 1 + (c+1)*u*u*u + c*u*u
}

// overlayArea is the canvas inside the safe area and the overlays' margin.
func overlayArea(size core.Size, safe core.Edges) rect {
	x := float32(safe.Left) + overlayMargin
	y := float32(safe.Top) + overlayMargin
	return rect{x, y, max(0, float32(size.Width-safe.Right)-overlayMargin-x), max(0, float32(size.Height-safe.Bottom)-overlayMargin-y)}
}

// overlayScale is the panels' scale: the HUD's, but never so small a button drops under a thumb's width.
func overlayScale(size core.Size, area rect, needW, needH float32) float32 {
	s := min(float32(size.Width)/hudRefWidth, float32(size.Height)/hudRefHeight)
	u := max(overlayMinScale, min(overlayMaxScale, s*1.1))
	if needW > 0 && needH > 0 {
		u = min(u, area.w/needW, area.h/needH)
	}
	return max(0.5, u)
}

// cardLayout places the level-up heading, its cards and, while redraws remain, the Redraw button under them.
type cardLayout struct {
	scale   float32
	heading rect
	cards   [offerCount]rect
	redraw  rect
}

// layoutCards sets n cards in a row under their heading, and the Redraw button below them when redraw is set, centred in the safe area and as large as fits, up to a little more than the HUD's scale.
func layoutCards(size core.Size, safe core.Edges, n int, redraw bool) cardLayout {
	area := overlayArea(size, safe)
	n = max(1, min(offerCount, n))
	cardH := float32(cardBaseW * cardKitH / cardKitW)
	needW := float32(n)*cardBaseW + float32(n-1)*cardGapPt
	needH := overlayHeadingPt + cardLiftPt + cardH
	if redraw {
		needH += cardRedrawGap + overlayButtonH
	}
	s := min(float32(size.Width)/hudRefWidth, float32(size.Height)/hudRefHeight)
	cs := min(area.w/needW, area.h/needH, max(1, cardMaxScaleOfHUD*s))
	cs = max(0.5, cs)
	w, h := cardBaseW*cs, cardH*cs
	rowW := float32(n)*w + float32(n-1)*cardGapPt*cs
	blockH := needH * cs
	top := area.y + (area.h-blockH)/2
	left := area.x + (area.w-rowW)/2
	l := cardLayout{scale: cs, heading: rect{area.x, top, area.w, overlayHeadingPt * cs}}
	for i := range n {
		l.cards[i] = rect{left + float32(i)*(w+cardGapPt*cs), top + (overlayHeadingPt+cardLiftPt)*cs, w, h}
	}
	if redraw {
		// The button scales with the cards to fit; its native button still grows to a thumb's size.
		bw := overlayButtonW * cs
		l.redraw = rect{area.x + (area.w-bw)/2, l.cards[0].y + h + cardRedrawGap*cs, bw, overlayButtonH * cs}
	}
	return l
}

// panelLayout places a pause or results panel, its heading and its buttons.
type panelLayout struct {
	scale   float32
	panel   rect
	heading rect
	buttons []rect
	// body is the confirmation's text; the results' rows are outcome, stats, record, embers, unlocks and best records.
	body, outcome, record, embers, unlocked, best rect
	stats                                         [3]rect
}

// layoutPause places the pause panel's three stacked buttons, or the quit confirmation's two side by side.
func layoutPause(size core.Size, safe core.Edges, confirm bool) panelLayout {
	area := overlayArea(size, safe)
	if confirm {
		h := float32(overlayPad + 30 + 8 + 32 + 14 + overlayButtonH + overlayPad)
		u := overlayScale(size, area, confirmPanelW, h)
		l := panelLayout{scale: u, panel: centred(area, confirmPanelW*u, h*u)}
		y := l.panel.y + overlayPad*u
		l.heading = rect{l.panel.x, y, l.panel.w, 30 * u}
		y += 38 * u
		l.body = rect{l.panel.x + overlayPad*u, y, l.panel.w - 2*overlayPad*u, 32 * u}
		y += 46 * u
		l.buttons = buttonRow(l.panel, y, 2, u)
		return l
	}
	h := float32(overlayPad + 32 + 14 + 3*overlayButtonH + 2*overlayButtonGap + overlayPad)
	u := overlayScale(size, area, pausePanelW, h)
	l := panelLayout{scale: u, panel: centred(area, pausePanelW*u, h*u)}
	y := l.panel.y + overlayPad*u
	l.heading = rect{l.panel.x, y, l.panel.w, 32 * u}
	y += 46 * u
	bw := min(l.panel.w-2*overlayPad*u, (overlayButtonW+48)*u)
	for range 3 {
		l.buttons = append(l.buttons, rect{l.panel.x + (l.panel.w-bw)/2, y, bw, overlayButtonH * u})
		y += (overlayButtonH + overlayButtonGap) * u
	}
	return l
}

// layoutResults places the results panel: outcome, heading, the time, kills and level, the record line, the embers line, the unlocks line when the run unlocked anything, the best records and two buttons.
func layoutResults(size core.Size, safe core.Edges, unlocked bool) panelLayout {
	area := overlayArea(size, safe)
	unlockH := float32(0)
	if unlocked {
		unlockH = resultsLineH + 4
	}
	h := float32(overlayPad+14+4+30+8+resultsStatH+resultsCaptionH+10+16+4+resultsLineH+4+2*resultsBestLineH+14+overlayButtonH+overlayPad) + unlockH
	u := overlayScale(size, area, resultsPanelW, h)
	l := panelLayout{scale: u, panel: centred(area, resultsPanelW*u, h*u)}
	x, w := l.panel.x+overlayPad*u, l.panel.w-2*overlayPad*u
	y := l.panel.y + overlayPad*u
	l.outcome = rect{x, y, w, 14 * u}
	y += 18 * u
	l.heading = rect{x, y, w, 30 * u}
	y += 38 * u
	statsW := 3 * resultsStatW * u
	for i := range l.stats {
		l.stats[i] = rect{l.panel.x + (l.panel.w-statsW)/2 + float32(i)*resultsStatW*u, y, resultsStatW * u, (resultsStatH + resultsCaptionH) * u}
	}
	y += (resultsStatH + resultsCaptionH + 10) * u
	l.record = rect{x, y, w, 16 * u}
	y += 20 * u
	l.embers = rect{x, y, w, resultsLineH * u}
	y += (resultsLineH + 4) * u
	if unlocked {
		l.unlocked = rect{x, y, w, resultsLineH * u}
		y += (resultsLineH + 4) * u
	}
	l.best = rect{x, y, w, 2 * resultsBestLineH * u}
	y += (2*resultsBestLineH + 14) * u
	l.buttons = buttonRow(l.panel, y, 2, u)
	return l
}

// buttonRow lays n buttons side by side, centred in panel at y, narrowing them when the panel is narrow.
func buttonRow(panel rect, y float32, n int, u float32) []rect {
	gap := 12 * u
	bw := min(overlayButtonW*u, (panel.w-2*overlayPad*u-float32(n-1)*gap)/float32(n))
	x := panel.x + (panel.w-float32(n)*bw-float32(n-1)*gap)/2
	out := make([]rect, n)
	for i := range out {
		out[i] = rect{x + float32(i)*(bw+gap), y, bw, overlayButtonH * u}
	}
	return out
}

func centred(area rect, w, h float32) rect {
	return rect{area.x + (area.w-w)/2, area.y + (area.h-h)/2, w, h}
}

// overlayPainter draws the level-up cards, the pause panel and the results on the arena canvas, in the HUD's kit.
type overlayPainter struct {
	d     *core.Drawing
	size  core.Size
	art   *hudArt
	icons *art
	m     *overlayMotion
	tr    evstrings.Table
	lang  string
	// pressed is the TestID of the overlay button a press holds down.
	pressed string
}

// font is the canvas font of the shell's language at size points.
func (p *overlayPainter) font(size float32, bold bool) core.Font {
	f := core.Font{Size: core.Points(size), Language: p.lang}
	if bold {
		f.Weight = core.FontWeightBold
	}
	return f
}

// text draws s with a dark outline so it reads over the painted wood and iron.
func (p *overlayPainter) text(s string, x, y, size float32, bold bool, color core.Color, align core.TextAlign) {
	f := p.font(size, bold)
	p.d.SetTextAlign(align).SetTextBaseline(core.BaselineMiddle)
	p.d.SetStrokeColor(hudOutline).SetLineWidth(max(2, size/6)).StrokeText(s, x, y, f)
	p.d.SetFillColor(color).FillText(s, x, y, f)
}

// lines draws text wrapped to r's width, lineH points apart from r's top, at most as many lines as r holds.
func (p *overlayPainter) lines(s string, r rect, size, lineH float32, color core.Color) {
	f := p.font(size, false)
	n := max(1, int(r.h/lineH))
	cx := r.x + r.w/2
	for i, line := range wrapText(p.d, s, f, r.w, n) {
		p.text(line, cx, r.y+lineH*(float32(i)+0.5), size, false, color, core.TextAlignCenter)
	}
}

// scrim darkens the arena under an overlay.
func (p *overlayPainter) scrim(alpha float32) {
	p.d.Save()
	p.d.SetAlpha(0.6*alpha).SetFillColor(overlayScrim).FillRect(0, 0, float32(p.size.Width), float32(p.size.Height))
	p.d.Restore()
}

// nineSlice draws the kit image img into r, keeping its insets (in the kit's pixels) at inset points each and stretching the middle.
func (p *overlayPainter) nineSlice(img kitImage, insets [4]float32, r rect, inset float32) {
	p.nineSliceFrom(img.src, img.x, img.y, img.w, img.h, insets, r, inset)
}

// nineSliceFrom is nineSlice of the part ox, oy, iw, ih of img, such as a frame packed in an atlas.
func (p *overlayPainter) nineSliceFrom(img core.ImageSource, ox, oy, iw, ih float32, insets [4]float32, r rect, inset float32) {
	k := inset / max(insets[0], insets[1], insets[2], insets[3])
	top, right, bottom, left := insets[0]*k, insets[1]*k, insets[2]*k, insets[3]*k
	if top+bottom > r.h {
		f := r.h / (top + bottom)
		top, bottom = top*f, bottom*f
	}
	if left+right > r.w {
		f := r.w / (left + right)
		left, right = left*f, right*f
	}
	sx := [4]float32{ox, ox + insets[3], ox + iw - insets[1], ox + iw}
	sy := [4]float32{oy, oy + insets[0], oy + ih - insets[2], oy + ih}
	dx := [4]float32{r.x, r.x + left, r.x + r.w - right, r.x + r.w}
	dy := [4]float32{r.y, r.y + top, r.y + r.h - bottom, r.y + r.h}
	for row := range 3 {
		for col := range 3 {
			if dx[col+1] <= dx[col] || dy[row+1] <= dy[row] {
				continue
			}
			p.d.DrawImageFrom(img, sx[col], sy[row], sx[col+1]-sx[col], sy[row+1]-sy[row], dx[col], dy[row], dx[col+1]-dx[col], dy[row+1]-dy[row])
		}
	}
}

// panel draws the kit's iron panel into r, or an iron plate when the kit lacks it.
func (p *overlayPainter) panel(r rect, u float32) {
	if img, ok := p.art.image(hudPanel); ok {
		p.nineSlice(img, panelInsets, r, 22*u)
		return
	}
	p.d.SetFillColor(core.RGBA(0x15120e, 0.96)).BeginPath().RoundRect(r.x, r.y, r.w, r.h, 12*u).Fill()
	p.d.SetStrokeColor(hudBronze).SetLineWidth(3*u).BeginPath().RoundRect(r.x+2*u, r.y+2*u, r.w-4*u, r.h-4*u, 10*u).Stroke()
}

// button kinds, which tint a painted button's plate.
const (
	buttonPlain = iota
	buttonEmber
	buttonDanger
)

// button draws a painted button: a small iron panel tinted by kind, its label, a glow when focused and sunk while pressed.
func (p *overlayPainter) button(id, label string, r rect, u float32, kind int, focused bool) {
	if id == p.pressed {
		r.y += 2 * u
	}
	p.d.Save()
	if focused {
		pulse := 0.75 + 0.25*float32(math.Sin(p.m.clock*5))
		p.d.SetShadow(core.RGBA(0xffb347, 0.85*pulse), 0, 0, 14*u)
	}
	p.panel(r, 0.55*u)
	p.d.Restore()
	inner := rect{r.x + 8*u, r.y + 7*u, r.w - 16*u, r.h - 14*u}
	tint := core.RGBA(0, 0)
	switch kind {
	case buttonEmber:
		tint = core.RGBA(0xc2410c, 0.55)
	case buttonDanger:
		tint = core.RGBA(0xa3341a, 0.5)
	}
	p.d.SetFillColor(tint).BeginPath().RoundRect(inner.x, inner.y, inner.w, inner.h, 4*u).Fill()
	if focused {
		p.d.SetStrokeColor(hudGold).SetLineWidth(2*u).BeginPath().RoundRect(inner.x-2*u, inner.y-2*u, inner.w+4*u, inner.h+4*u, 6*u).Stroke()
	}
	if id == p.pressed {
		p.d.SetFillColor(core.RGBA(0, 0.25)).BeginPath().RoundRect(inner.x, inner.y, inner.w, inner.h, 4*u).Fill()
	}
	color := hudIvory
	if focused {
		color = core.RGB(0xffe4a8)
	}
	cx, cy := r.center()
	p.text(fitText(p.d, label, p.font(15*u, true), inner.w), cx, cy, 15*u, true, color, core.TextAlignCenter)
}

// heading draws a screen's title centred in r.
func (p *overlayPainter) heading(s string, r rect, u float32) {
	cx, cy := r.center()
	p.d.Save()
	p.d.SetShadow(core.RGBA(0xff9a3c, 0.45), 0, 0, 10*u)
	p.text(fitText(p.d, s, p.font(overlayHeadingFontPt*u, true), r.w), cx, cy, overlayHeadingFontPt*u, true, hudIvory, core.TextAlignCenter)
	p.d.Restore()
}

// fadeIn scales a panel up into place and fades it in, returning how far in it is.
func (p *overlayPainter) fadeIn(r rect) float32 {
	e := float32(easeOut(p.m.panelIn()))
	p.scrim(e)
	cx, cy := r.center()
	s := 0.92 + 0.08*e
	p.d.Translate(cx, cy).Scale(s, s).Translate(-cx, -cy)
	p.d.SetAlpha(e)
	return e
}

// drawPause draws the pause panel, or the quit confirmation, with the focused button glowing.
func drawPause(p *overlayPainter, l panelLayout, confirm bool, focus int) {
	tr := p.tr
	u := l.scale
	p.d.Save()
	p.d.SetImageSmoothing(true)
	p.fadeIn(l.panel)
	p.panel(l.panel, u)
	if confirm {
		p.heading(tr.ConfirmQuitTitle, l.heading, u)
		p.lines(tr.ConfirmQuitBody, l.body, 13*u, 16*u, overlayDim)
		p.button("pause-confirm-quit-yes", tr.ConfirmQuitYes, l.buttons[0], u, buttonDanger, focus == 0)
		p.button("pause-confirm-quit-no", tr.Cancel, l.buttons[1], u, buttonPlain, focus == 1)
	} else {
		p.heading(tr.Paused, l.heading, u)
		p.button("pause-resume", tr.Resume, l.buttons[0], u, buttonEmber, focus == 0)
		p.button("pause-settings", tr.Settings, l.buttons[1], u, buttonPlain, focus == 1)
		p.button("pause-quit", tr.QuitToTitle, l.buttons[2], u, buttonDanger, focus == 2)
	}
	p.d.Restore()
}

// resultsText is what the results panel says, which the canvas draws and the native layer reads out.
type resultsText struct {
	outcome, title, time, kills, level, record, embers, unlocked, best string
	victory, newRecord                                                 bool
}

// resultsTextOf is what the results say of a run whose hud is st, with the records, the run's reward and the ember balance after it.
func resultsTextOf(tr evstrings.Table, st hudState, rec Records, reward runReward, balance int) resultsText {
	r := resultsText{outcome: tr.DefeatBadge, title: tr.DefeatTitle, record: tr.NoNewRecord}
	if st.phase == PhaseVictory {
		r.victory, r.outcome, r.title = true, tr.VictoryBadge, tr.VictoryTitle
	}
	if st.tier > 0 {
		r.outcome += "  ·  " + tr.Tier(st.tier)
	}
	if st.newRecord {
		r.newRecord, r.record = true, tr.NewRecord
	}
	r.time, r.kills, r.level = tr.Time(st.time), tr.Kills(st.kills), tr.Level(st.level)
	r.embers = tr.EmbersEarned(reward.embers) + "  ·  " + tr.EmberBalance(balance)
	if len(reward.unlocks) > 0 || reward.tierOpened > 0 {
		names := make([]string, 0, len(reward.unlocks)+1)
		if reward.tierOpened > 0 {
			names = append(names, tr.Tier(reward.tierOpened))
		}
		for _, id := range reward.unlocks {
			names = append(names, unlockName(tr, id))
		}
		r.unlocked = tr.UnlockedList(names)
	}
	r.best = strings.Join([]string{tr.BestTime(clockText(rec.BestTime)), tr.BestKills(rec.BestKills), tr.BestLevel(rec.BestLevel), tr.Wins(rec.Wins)}, "  ·  ")
	return r
}

// unlockName is the localized name of the unlock id, falling back to the name data/progression.yaml gives it.
func unlockName(tr evstrings.Table, id string) string {
	if name, ok := tr.Unlocks[id]; ok {
		return name
	}
	for _, u := range mustGameData().progression.Unlocks {
		if u.ID == id {
			return u.Name
		}
	}
	return id
}

// drawResults draws the results panel: the outcome, the run's time, kills and level in the painted numerals, the record line, the embers and unlocks, the best records and two buttons.
func drawResults(p *overlayPainter, l panelLayout, st hudState, txt resultsText, focus int) {
	tr := p.tr
	u := l.scale
	p.d.Save()
	p.d.SetImageSmoothing(true)
	p.fadeIn(l.panel)
	p.panel(l.panel, u)

	outcomeColor := hudBossRed
	if txt.victory {
		outcomeColor = hudGold
	}
	cx, cy := l.outcome.center()
	p.text(txt.outcome, cx, cy, 12*u, true, outcomeColor, core.TextAlignCenter)
	p.heading(txt.title, l.heading, u)

	hp := hudPainter{d: p.d, art: p.art}
	numH := 28 * u
	captions := [3]string{strings.TrimSpace(tr.TimePrefix), strings.TrimSpace(tr.KillsPrefix), strings.TrimSpace(tr.LevelPrefix)}
	for i, r := range l.stats {
		mx := r.x + r.w/2
		my := r.y + resultsStatH*u/2
		switch i {
		case 0:
			hp.drawNumber(st.time, mx, my, numH, core.TextAlignCenter, false)
		case 1:
			side := 30 * u
			if img, ok := p.art.image(hudKillEmblem); ok {
				img.drawIn(p.d, mx-side-6*u, my-side/2, side, side)
			}
			hp.drawNumber(strconv.Itoa(st.kills), mx-4*u, my, numH*0.85, core.TextAlignLeft, true)
		case 2:
			side := 44 * u
			if img, ok := p.art.image(hudLevelBadge); ok {
				img.drawIn(p.d, mx-side/2, my-side/2, side, side)
			} else {
				p.d.SetFillColor(hudBronze).BeginPath().MoveTo(mx, my-side/2).LineTo(mx+side/2, my).LineTo(mx, my+side/2).LineTo(mx-side/2, my).ClosePath().Fill()
			}
			hp.drawNumber(strconv.Itoa(st.level), mx, my, 20*u, core.TextAlignCenter, false)
		}
		p.text(captions[i], mx, r.y+(resultsStatH+resultsCaptionH/2)*u, 11*u, true, overlayDim, core.TextAlignCenter)
	}

	recordColor := overlayDim
	if txt.newRecord {
		recordColor = hudGold
	}
	cx, cy = l.record.center()
	p.text(txt.record, cx, cy, 13*u, true, recordColor, core.TextAlignCenter)
	cx, cy = l.embers.center()
	p.text(txt.embers, cx, cy, 12*u, true, hudGold, core.TextAlignCenter)
	if txt.unlocked != "" {
		p.lines(txt.unlocked, l.unlocked, 12*u, resultsLineH*u, hudGold)
	}
	p.lines(txt.best, l.best, 11*u, resultsBestLineH*u, overlayDim)

	p.button("results-play-again", tr.PlayAgain, l.buttons[0], u, buttonEmber, focus == 0)
	p.button("results-title", tr.TitleButton, l.buttons[1], u, buttonPlain, focus == 1)
	p.d.Restore()
}

// drawCards draws the level-up: its heading and the offered cards dealing in, the focused one lifted and glowing, and a chosen one burning away.
func drawCards(p *overlayPainter, l cardLayout, st hudState, focus int) {
	m := p.m
	burn := float32(0)
	if m.burning {
		burn = float32(m.burn)
	}
	p.d.Save()
	p.d.SetImageSmoothing(true)
	p.scrim(float32(easeOut(m.dealt(0))) * (1 - burn))
	p.d.Save()
	p.d.SetAlpha(float32(m.dealt(0)) * (1 - burn))
	p.heading(p.tr.Level(st.level), l.heading, l.scale)
	if st.redraws > 0 && l.redraw.w > 0 {
		p.button("level-up-redraw", p.tr.Redraw(st.redraws), l.redraw, l.scale, buttonPlain, false)
	}
	p.d.Restore()
	n := 0
	for i, c := range st.offers {
		if c.name == "" {
			continue
		}
		n++
		chosen := m.burning && i == m.burnCard
		if chosen {
			continue // drawn last, over its neighbours
		}
		p.card(l, i, c, focus == i && !m.burning, 0, burn)
	}
	if m.burning && m.burnCard < len(st.offers) && st.offers[m.burnCard].name != "" {
		p.card(l, m.burnCard, st.offers[m.burnCard], true, burn, 0)
	}
	p.d.Restore()
}

// card draws offer i. burn is how far it has burnt, from the bottom up; fade is how far a card that was not chosen has dropped away.
func (p *overlayPainter) card(l cardLayout, i int, c hudCard, focused bool, burn, fade float32) {
	r := l.cards[i]
	u := l.scale
	t := p.m.dealt(i)
	if t <= 0 {
		return
	}
	e := float32(easeOutBack(t))
	lift := float32(p.m.lift[i])
	if burn > 0 {
		lift = 1
	}
	dy := (1-e)*(r.h*0.8+40*u) - lift*cardLiftPt*u + fade*24*u
	rot := (1 - e) * float32(i-1) * 0.25
	cx, cy := r.center()

	p.d.Save()
	p.d.Translate(cx, cy+dy).Rotate(rot).Translate(-cx, -cy)
	p.d.SetAlpha(min(1, float32(t)*2.5) * (1 - fade))
	if burn > 0 {
		// The card burns from its foot up: what is above the burn line remains.
		line := r.y + r.h - (r.h+8*u)*burn
		p.d.BeginPath().Rect(r.x-20*u, r.y-20*u, r.w+40*u, line-r.y+20*u).Clip()
	}
	p.cardFace(r, c, i, focused)
	if burn > 0 {
		p.d.SetComposite(core.CompositeLighter).SetAlpha(0.5 * (1 - burn))
		p.d.SetFillColor(overlayEmber).FillRect(r.x, r.y, r.w, r.h)
	}
	p.d.Restore()
	if burn > 0 && burn < 1 {
		p.burnEdge(r, u, burn, cx, cy+dy, rot)
	}
}

// cardFace draws a card's frame and content into r.
func (p *overlayPainter) cardFace(r rect, c hudCard, i int, focused bool) {
	u := r.w / cardBaseW
	frameName := hudCardCommon
	if focused {
		frameName = hudCardFocused
	}
	p.d.Save()
	if focused {
		pulse := 0.7 + 0.3*float32(math.Sin(p.m.clock*4))
		p.d.SetShadow(core.RGBA(0xffb347, 0.9*pulse), 0, 0, 22*u)
	}
	if img, ok := p.art.image(frameName); ok {
		img.drawIn(p.d, r.x, r.y, r.w, r.h)
	} else {
		border := hudBronze
		if focused {
			border = hudGold
		}
		p.d.SetFillColor(core.RGBA(0x1c1610, 0.97)).BeginPath().RoundRect(r.x, r.y, r.w, r.h, 12*u).Fill()
		p.d.SetShadow(core.RGBA(0, 0), 0, 0, 0)
		p.d.SetStrokeColor(border).SetLineWidth(3*u).BeginPath().RoundRect(r.x+2*u, r.y+2*u, r.w-4*u, r.h-4*u, 10*u).Stroke()
	}
	p.d.Restore()

	px := r.w / cardKitW
	in := rect{r.x + cardInsetSide*px, r.y + cardInsetTop*px, r.w - 2*cardInsetSide*px, r.h - (cardInsetTop+cardInsetBottom)*px}
	mid := in.x + in.w/2
	y := in.y

	kind := p.tr.KindPassive
	switch c.kind {
	case offerWeapon:
		kind = p.tr.KindWeapon
	case offerHeal:
		kind = p.tr.KindHeal
	}
	p.text(kind, mid, y+6*u, 10*u, true, cardAccent(c.id, c.kind), core.TextAlignCenter)
	hp := hudPainter{d: p.d, art: p.art}
	hp.drawNumber(strconv.Itoa(i+1), in.x+4*u, y+6*u, 14*u, core.TextAlignLeft, false)
	y += 16 * u

	icon := 48 * u
	if img, ok := p.icons.cardIcon(c.id); ok {
		img.drawIn(p.d, mid-icon/2, y, icon, icon)
	} else {
		p.d.SetFillColor(cardAccent(c.id, c.kind)).BeginPath().RoundRect(mid-icon/2, y, icon, icon, 10*u).Fill()
		p.text(cardGlyph(c.id), mid, y+icon/2, 16*u, true, hudIvory, core.TextAlignCenter)
	}
	y += icon + 6*u

	copyText := p.tr.Card(c.id, c.name, c.description)
	nameColor := hudIvory
	if focused {
		nameColor = core.RGB(0xffe4a8)
	}
	nameFont := p.font(15*u, true)
	names := wrapText(p.d, copyText.Name, nameFont, in.w, 2)
	for _, line := range names {
		p.text(line, mid, y+9*u, 15*u, true, nameColor, core.TextAlignCenter)
		y += 18 * u
	}
	y += 4 * u

	pipsY := in.y + in.h - 8*u
	if c.maxLevel > 0 {
		p.cardPips(mid, pipsY, u, c.level, c.maxLevel)
	}
	descBottom := pipsY - 10*u
	if c.maxLevel <= 0 {
		descBottom = in.y + in.h
	}
	p.lines(copyText.Description, rect{in.x, y, in.w, max(0, descBottom-y)}, 11.5*u, 14*u, overlayDim)
}

// cardPips lays a card's level pips centred on (cx, cy): the levels held lit, the level the card brings pulsing, the rest dark.
func (p *overlayPainter) cardPips(cx, cy, u float32, next, maxLevel int) {
	side := min(12*u, 120*u/float32(maxLevel))
	x := cx - side*float32(maxLevel)/2
	lit, okLit := p.art.image(hudPipLit)
	unlit, okUnlit := p.art.image(hudPipUnlit)
	for i := range maxLevel {
		px := x + float32(i)*side
		on := i < next
		p.d.Save()
		if i == next-1 {
			pulse := 0.5 + 0.5*float32(math.Sin(p.m.clock*6))
			p.d.SetShadow(core.RGBA(0xffb347, pulse), 0, 0, 8*u)
		}
		switch {
		case on && okLit:
			lit.drawIn(p.d, px, cy-side/2, side, side)
		case !on && okUnlit:
			unlit.drawIn(p.d, px, cy-side/2, side, side)
		default:
			c := pipEmpty
			if on {
				c = hudGold
			}
			h := side / 2
			p.d.SetFillColor(c).BeginPath().MoveTo(px+h, cy-h).LineTo(px+side, cy).LineTo(px+h, cy+h).LineTo(px, cy).ClosePath().Fill()
		}
		p.d.Restore()
	}
}

// burnEdge draws the ragged, glowing edge where a card burns and the embers it sheds, in the card's own turned frame.
func (p *overlayPainter) burnEdge(r rect, u, burn, cx, cy, rot float32) {
	ccx, ccy := r.center()
	line := r.y + r.h - (r.h+8*u)*burn
	p.d.Save()
	p.d.Translate(cx, cy).Rotate(rot).Translate(-ccx, -ccy)
	p.d.SetComposite(core.CompositeLighter)
	p.d.SetShadow(core.RGBA(0xff7a1a, 0.9), 0, 0, 12*u)
	p.d.SetStrokeColor(core.RGBA(0xffc06a, 0.95)).SetLineWidth(3 * u).BeginPath()
	const teeth = 12
	for k := 0; k <= teeth; k++ {
		x := r.x + r.w*float32(k)/teeth
		y := line + 4*u*float32(math.Sin(float64(k)*2.3+float64(burn)*9))
		if k == 0 {
			p.d.MoveTo(x, y)
		} else {
			p.d.LineTo(x, y)
		}
	}
	p.d.Stroke()
	// Embers rise from the edge, each on a path of its own.
	for k := range 9 {
		f := float64(k)
		x := r.x + r.w*float32(math.Mod(f*0.37+0.11, 1))
		rise := float32(math.Mod(float64(burn)*2.2+f*0.13, 1))
		y := line - rise*40*u
		a := 0.9 * (1 - rise)
		p.d.SetFillColor(core.RGBA(0xffb347, a)).BeginPath().Ellipse(x+3*u*float32(math.Sin(f+float64(burn)*12)), y, 2*u, 2*u).Fill()
	}
	p.d.Restore()
}

// wrapText breaks s into at most maxLines lines no wider than width, at spaces or between CJK characters, ending a cut line with an ellipsis.
func wrapText(d *core.Drawing, s string, font core.Font, width float32, maxLines int) []string {
	var tokens []string
	var word []rune
	flush := func() {
		if len(word) > 0 {
			tokens = append(tokens, string(word))
			word = word[:0]
		}
	}
	for _, r := range s {
		switch {
		case r == ' ':
			word = append(word, r)
			flush()
		case r > 0x2E80 || unicode.Is(unicode.Han, r):
			flush()
			tokens = append(tokens, string(r))
		default:
			word = append(word, r)
		}
	}
	flush()

	var lines []string
	cur := ""
	for _, t := range tokens {
		next := cur + t
		if cur != "" && d.MeasureText(strings.TrimRight(next, " "), font) > width {
			lines = append(lines, strings.TrimRight(cur, " "))
			cur = strings.TrimLeft(t, " ")
			continue
		}
		cur = next
	}
	if cur = strings.TrimRight(cur, " "); cur != "" {
		lines = append(lines, cur)
	}
	if maxLines > 0 && len(lines) > maxLines {
		lines = lines[:maxLines]
		lines[maxLines-1] = fitText(d, lines[maxLines-1]+"…", font, width)
	}
	return lines
}

// fitText cuts s with an ellipsis until it is no wider than width.
func fitText(d *core.Drawing, s string, font core.Font, width float32) string {
	if d.MeasureText(s, font) <= width {
		return s
	}
	runes := []rune(strings.TrimSuffix(s, "…"))
	for len(runes) > 0 {
		runes = runes[:len(runes)-1]
		t := strings.TrimRight(string(runes), " ") + "…"
		if d.MeasureText(t, font) <= width {
			return t
		}
	}
	return "…"
}
