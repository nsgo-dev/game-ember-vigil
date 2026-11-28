package main

import (
	"math"
	"strconv"

	"github.com/nsgo-dev/nsgo/core"
	evstrings "me.oneo/embervigil/strings"
)

// The settings panel's geometry in points at a scale of 1: a row's height, its label's width, the controls' width and the gaps.
const (
	settingsRowH        = 44
	settingsRowGap      = 6
	settingsLabelW      = 116
	settingsControlW    = 280
	settingsColumnGap   = 28
	settingsStepW       = 44
	settingsValueW      = 60
	settingsToggleW     = 104
	settingsOptionGap   = 4
	settingsLabelFontPt = 14
	settingsRowCount    = 8
)

// settingsRow is one row of the settings panel: its whole width, its label, and its controls left to right.
type settingsRow struct {
	row, label rect
	controls   []rect
	// value is a volume's readout, between its two steps.
	value rect
}

// settingsLayout places the settings panel: its heading, the volumes, shake, quality, language, statistics and frame rate rows, and Apply and Back.
type settingsLayout struct {
	scale          float32
	panel, heading rect
	rows           [settingsRowCount]rect
	items          [settingsRowCount]settingsRow
	buttons        []rect
}

// layoutSettings lays the rows in one column, or in two where that makes them larger, as on a phone held sideways.
func layoutSettings(size core.Size, safe core.Edges) settingsLayout {
	area := overlayArea(size, safe)
	colW := float32(settingsLabelW + 12 + settingsControlW)
	rowsH := func(n int) float32 { return float32(n)*settingsRowH + float32(n-1)*settingsRowGap }
	frame := float32(overlayPad + 32 + 12 + 14 + overlayButtonH + overlayPad)
	oneW, oneH := colW+2*overlayPad, frame+rowsH(settingsRowCount)
	perTwo := (settingsRowCount + 1) / 2
	twoW, twoH := 2*colW+settingsColumnGap+2*overlayPad, frame+rowsH(perTwo)
	u1 := overlayScale(size, area, oneW, oneH)
	u2 := overlayScale(size, area, twoW, twoH)
	perCol, u, w, h := settingsRowCount, u1, oneW, oneH
	if u2 > u1 {
		perCol, u, w, h = perTwo, u2, twoW, twoH
	}
	l := settingsLayout{scale: u, panel: centred(area, w*u, h*u)}
	y := l.panel.y + overlayPad*u
	l.heading = rect{l.panel.x, y, l.panel.w, 32 * u}
	y += 44 * u
	for i := range settingsRowCount {
		col, n := i/perCol, i%perCol
		x := l.panel.x + overlayPad*u + float32(col)*(colW+settingsColumnGap)*u
		r := rect{x, y + float32(n)*(settingsRowH+settingsRowGap)*u, colW * u, settingsRowH * u}
		l.rows[i] = r
		l.items[i] = settingsRowAt(r, i, u)
	}
	y += (rowsH(perCol) + 14) * u
	l.buttons = buttonRow(l.panel, y, 2, u)
	return l
}

// settingsRowAt places row i's label and controls in r: three volumes' steps, the shake, statistics and frame rate toggles, the four quality tiers and the two languages.
func settingsRowAt(r rect, i int, u float32) settingsRow {
	s := settingsRow{row: r, label: rect{r.x + 8*u, r.y, settingsLabelW * u, r.h}}
	x := r.x + r.w - settingsControlW*u
	inner := rect{x, r.y + 3*u, settingsControlW * u, r.h - 6*u}
	options := func(n int) {
		ow := (inner.w - float32(n-1)*settingsOptionGap*u) / float32(n)
		for j := range n {
			s.controls = append(s.controls, rect{inner.x + float32(j)*(ow+settingsOptionGap*u), inner.y, ow, inner.h})
		}
	}
	switch {
	case i < 3:
		step := settingsStepW * u
		s.controls = []rect{{inner.x, inner.y, step, inner.h}, {inner.x + step + settingsValueW*u, inner.y, step, inner.h}}
		s.value = rect{inner.x + step, inner.y, settingsValueW * u, inner.h}
	case i == 3 || i >= 6:
		s.controls = []rect{{inner.x, inner.y, settingsToggleW * u, inner.h}}
	case i == 4:
		options(len(qualityTiers))
	case i == 5:
		options(len(evstrings.Languages))
	}
	return s
}

// settingsControl is one control of the settings panel: what it says, what it is called for assistive technology, whether it is chosen, and what it does.
type settingsControl struct {
	id, text, label string
	selected        bool
	press           func()
}

// settingsControls are row i's controls, in the order layoutSettings places them.
func (g *emberVigil) settingsControls(tr evstrings.Table, cfg Settings, i int) []settingsControl {
	volume := func(key, name string, adjust func(float64)) []settingsControl {
		return []settingsControl{
			{id: "settings-" + key + "-dec", text: "-", label: tr.DecreaseLabel(name), press: func() { adjust(-volumeStep) }},
			{id: "settings-" + key + "-inc", text: "+", label: tr.IncreaseLabel(name), press: func() { adjust(volumeStep) }},
		}
	}
	switch i {
	case 0:
		return volume("master", tr.MasterVolume, g.adjustMasterVolume)
	case 1:
		return volume("music", tr.MusicVolume, g.adjustMusicVolume)
	case 2:
		return volume("sfx", tr.SFXVolume, g.adjustSFXVolume)
	case 3:
		text := tr.Off
		if cfg.ScreenShake {
			text = tr.On
		}
		return []settingsControl{{id: "settings-shake", text: text, label: tr.ScreenShake + ", " + text, selected: cfg.ScreenShake, press: g.toggleShake}}
	case 4:
		out := make([]settingsControl, 0, len(qualityTiers))
		for _, q := range qualityTiers {
			tier := q
			name := qualityLabel(tr, tier)
			out = append(out, settingsControl{id: "settings-quality-" + string(tier), text: name, label: name, selected: cfg.Quality == tier, press: func() { g.setQuality(tier) }})
		}
		return out
	case 6:
		text := tr.Off
		if cfg.Stats {
			text = tr.On
		}
		return []settingsControl{{id: "settings-stats", text: text, label: tr.Stats + ", " + text, selected: cfg.Stats, press: g.toggleStats}}
	case 7:
		text := tr.Off
		if cfg.ShowFPS {
			text = tr.On
		}
		return []settingsControl{{id: "settings-fps", text: text, label: tr.ShowFPS + ", " + text, selected: cfg.ShowFPS, press: g.toggleFPS}}
	default:
		return []settingsControl{
			{id: "settings-lang-en", text: tr.LanguageEN, label: tr.LanguageEN, selected: cfg.Language == evstrings.EN, press: func() { g.setLanguage(evstrings.EN) }},
			{id: "settings-lang-zh-CN", text: tr.LanguageZhCN, label: tr.LanguageZhCN, selected: cfg.Language == evstrings.ZhCN, press: func() { g.setLanguage(evstrings.ZhCN) }},
		}
	}
}

// settingsRowName is row i's label.
func settingsRowName(tr evstrings.Table, i int) string {
	return [settingsRowCount]string{tr.MasterVolume, tr.MusicVolume, tr.SFXVolume, tr.ScreenShake, tr.Quality, tr.Language, tr.Stats, tr.ShowFPS}[i]
}

// volumeText is a volume's readout as a percentage.
func volumeText(v float64) string { return strconv.Itoa(int(math.Round(v*100))) + "%" }

// settingsVolume is row i's volume, for the three volume rows.
func settingsVolume(cfg Settings, i int) float64 {
	return [3]float64{cfg.MasterVolume, cfg.MusicVolume, cfg.SFXVolume}[i]
}

// drawSettings draws the settings panel in the HUD's kit: each row's label and painted controls, the chosen ones lit, the focused row framed in gold, and Apply and Back.
func drawSettings(p *overlayPainter, l settingsLayout, rows [settingsRowCount][]settingsControl, cfg Settings, focus int) {
	tr := p.tr
	u := l.scale
	p.d.Save()
	p.d.SetImageSmoothing(true)
	p.fadeIn(l.panel)
	p.panel(l.panel, u)
	p.heading(tr.SettingsTitle, l.heading, u)
	for i, r := range l.items {
		if i == focus {
			pulse := 0.7 + 0.3*float32(math.Sin(p.m.clock*5))
			p.d.SetStrokeColor(core.RGBA(0xe0b25a, pulse)).SetLineWidth(2*u).BeginPath().RoundRect(r.row.x, r.row.y, r.row.w, r.row.h, 8*u).Stroke()
		}
		color := hudIvory
		if i == focus {
			color = core.RGB(0xffe4a8)
		}
		_, cy := r.label.center()
		p.text(fitText(p.d, settingsRowName(tr, i), p.font(settingsLabelFontPt*u, true), r.label.w), r.label.x, cy, settingsLabelFontPt*u, true, color, core.TextAlignLeft)
		for j, c := range rows[i] {
			if j >= len(r.controls) {
				break
			}
			kind := buttonPlain
			if c.selected {
				kind = buttonEmber
			}
			// The quality tiers share their row four ways, so their words are smaller.
			bu := u * 0.85
			if i == 4 {
				bu = u * 0.72
			}
			p.button(c.id, c.text, r.controls[j], bu, kind, false)
		}
		if i < 3 {
			cx, cy := r.value.center()
			p.text(volumeText(settingsVolume(cfg, i)), cx, cy, 15*u, true, hudIvory, core.TextAlignCenter)
		}
	}
	p.button("settings-apply", tr.Apply, l.buttons[0], u, buttonEmber, focus == settingsRowCount)
	p.button("settings-back", tr.Back, l.buttons[1], u, buttonPlain, focus == settingsRowCount+1)
	p.d.Restore()
}

// settingsView is the native layer over the canvas's settings panel: the heading and each row's name and volume for assistive technology, and an invisible button over each painted control, which focuses its row when the mouse comes over it.
func (g *emberVigil) settingsView() core.Node {
	tr := g.tr()
	cfg := g.settings.Get()
	l := layoutSettings(g.viewSize.Get(), g.cx.Env().SafeArea())
	children := []core.Node{infoView("settings-heading", tr.SettingsTitle, l.heading)}
	for i, r := range l.items {
		row := i
		hover := func() { g.hoverMenu(row) }
		children = append(children, infoView("settings-row-"+strconv.Itoa(i), settingsRowName(tr, i), r.label))
		for j, c := range g.settingsControls(tr, cfg, i) {
			children = append(children, g.hitOption(c.id, c.label, r.controls[j], c.selected, c.press, hover))
		}
		if i < 3 {
			key := [3]string{"master", "music", "sfx"}[i]
			children = append(children, infoView("settings-"+key+"-value", settingsRowName(tr, i)+" "+volumeText(settingsVolume(cfg, i)), r.value))
		}
	}
	children = append(children,
		g.hitButton("settings-apply", tr.Apply, "", l.buttons[0], g.applySettings, func() { g.hoverMenu(settingsRowCount) }),
		g.hitButton("settings-back", tr.Back, "", l.buttons[1], g.closeSettings, func() { g.hoverMenu(settingsRowCount + 1) }),
	)
	return overlayLayer("settings", tr.SettingsTitle, children...)
}
