package main

import (
	"embed"
	"math"
	"runtime"

	"github.com/nsgo-dev/nsgo/core"
	evstrings "me.oneo/embervigil/strings"
)

// embeddedTitle holds the title kit; the whole folder is embedded so the game still builds, and draws its still procedural cover, without the PNGs.
//
//go:embed assets/title
var embeddedTitle embed.FS

// The title kit's pieces, which assets/title/README.md describes: the cover's layers back to front, the glow and embers, the wordmarks and the button states.
const (
	titleBG          = "bg"
	titleMist        = "mist_layer"
	titleGlow        = "lantern_glow"
	titleKeeper      = "keeper_fg"
	titleCanopy      = "canopy_frame"
	titleLogoEN      = "logo_en"
	titleLogoZhCN    = "logo_zh_cn"
	titleButton      = "button_normal"
	titleButtonFocus = "button_focused"
	titleButtonPress = "button_pressed"
)

// titleEmberNames are the ember variants, which the cover cycles through.
var titleEmberNames = [...]string{"ember_1", "ember_2", "ember_3"}

// The cover's geometry in the kit's 1920×1080 pixels, from assets/title/README.md: the lantern's flame, and where embers rise from.
const (
	titleArtW, titleArtH       = 1920, 1080
	lanternX, lanternY         = 834, 402
	keeperTop                  = 210
	knollLeft, knollRight      = 600, 1350
	knollTop, knollBottom      = 780, 920
	titleGlowBack              = 560
	titleGlowFront             = 300
	titleEmberSide             = 64
	titleDrift                 = 32
	titleZoom                  = 1.04
	titleKeeperSway            = 5
	titleEmberRise, titleSway  = 420, 22
	titleEmberMin, titleEmberK = 0.25, 0.4
)

// The menu's layout in points at a 1280×720 window: the buttons, the records above them and the wordmark at the top.
const (
	titleButtonW, titleButtonH = 240, 60
	titleButtonGap             = 16
	titleButtonInsetPt         = 22
	titleLogoW, titleLogoH     = 512, 128
	titleRecordW               = 600
	titleRecordLineH           = 18
	titleRecordGap             = 10
	titleRecordFontPt          = 13
	titleLogoMinH              = 40
	titleHeadingFontPt         = 44
	titleButtonFontPt          = 18
)

// buttonInsets are the title buttons' nine-slice insets in the kit's pixels: top, right, bottom and left.
var buttonInsets = [4]float32{36, 44, 36, 44}

// How the cover moves, in seconds and hertz: the menu's fade in after a short wait, and the drift, mist, sway and flicker rates.
const (
	titleFadeDelay = 0.3
	titleFadeTime  = 0.9
	titleDriftHz   = 0.07
	titleMistHz    = 0.09
	titleSwayTime  = 5
	titleEmbers    = 40
)

// quitShown reports whether the title offers Quit: not on a phone or tablet, where apps are not quit, nor on the web, where quitting cannot close the tab.
func quitShown(idiom core.Idiom, goos string) bool {
	return idiom != core.IdiomPhone && idiom != core.IdiomTablet && goos != "js"
}

// titleItem is one button of the title menu.
type titleItem struct {
	id, label string
	press     func()
}

// titleItems are the title menu's buttons in focus order: Start, the Long Night tier once more than one is open, Shop, Collection, Settings and, where the app can quit, Quit.
func (g *emberVigil) titleItems(tr evstrings.Table) []titleItem {
	items := []titleItem{{"title-start", tr.Start, g.start}}
	if g.progress.openTier(mustGameData().tiers.maxTier()) > 1 {
		items = append(items, titleItem{"title-tier", tr.Tier(g.tier.Get()), func() { g.cycleTier(1) }})
	}
	items = append(items, titleItem{"title-shop", tr.Shop, g.openShop}, titleItem{"title-collection", tr.Collection, g.openCollection}, titleItem{"title-settings", tr.Settings, g.openSettings})
	if quitShown(g.cx.Env().Idiom(), runtime.GOOS) {
		items = append(items, titleItem{"title-quit", tr.Quit, g.quitApp})
	}
	return items
}

// titleLayout places the cover, the wordmark, the records and the menu in a canvas of one size and safe area.
type titleLayout struct {
	// cover is where the kit's 1920×1080 art lands, covering the canvas around its centre, and k its points per kit pixel.
	cover rect
	k     float32
	// scale is the wordmark's and the records' scale, and buttonScale the buttons', which never drops a button under a thumb.
	scale, buttonScale float32
	logo               rect
	recordsHeading     rect
	records            [4]rect
	buttons            []rect
}

// layoutTitle centres the menu's n buttons in a row at the bottom of the safe area, the records just above them and the wordmark at the top.
func layoutTitle(size core.Size, safe core.Edges, n int) titleLayout {
	w, h := float32(size.Width), float32(size.Height)
	k := max(w/titleArtW, h/titleArtH)
	l := titleLayout{k: k, cover: rect{(w - titleArtW*k) / 2, (h - titleArtH*k) / 2, titleArtW * k, titleArtH * k}}
	area := overlayArea(size, safe)
	u := max(hudMinScale, min(hudMaxScale, min(w/hudRefWidth, h/hudRefHeight)))
	bu := max(u, buttonSide/float32(titleButtonH))
	l.scale, l.buttonScale = u, bu

	n = max(1, n)
	gap := titleButtonGap * bu
	bw := min(titleButtonW*bu, (area.w-float32(n-1)*gap)/float32(n))
	bh := titleButtonH * bu
	x := area.x + (area.w-float32(n)*bw-float32(n-1)*gap)/2
	y := area.y + area.h - bh
	for i := range n {
		l.buttons = append(l.buttons, rect{x + float32(i)*(bw+gap), y, bw, bh})
	}

	ru := max(u, 0.85)
	lineH := titleRecordLineH * ru
	recW := min(area.w, titleRecordW*ru)
	recY := y - titleRecordGap*bu - lineH
	for i := range l.records {
		l.records[i] = rect{area.x + (area.w-recW)/2 + float32(i)*recW/4, recY, recW / 4, lineH}
	}
	l.recordsHeading = rect{area.x + (area.w-recW)/2, recY - lineH, recW, lineH}

	// The wordmark keeps to the sky above the keeper's hood, shrinking on wide, short screens.
	lh := min(area.w*titleLogoH/titleLogoW, titleLogoH*u, max(titleLogoMinH, l.cover.y+keeperTop*k-area.y))
	lw := lh * titleLogoW / titleLogoH
	l.logo = rect{area.x + (area.w-lw)/2, area.y, lw, lh}
	return l
}

// titleMotion is how the cover moves: its clock, which runs only while the title shows, and so the menu's fade in.
type titleMotion struct{ clock float64 }

func (m *titleMotion) step(dt float64) { m.clock += dt }

// fade is how far the wordmark, the records and the menu have faded in, from 0 to 1.
func (m *titleMotion) fade() float64 {
	return max(0, min(1, (m.clock-titleFadeDelay)/titleFadeTime))
}

// titleView is what one frame of the title shows.
type titleView struct {
	l     titleLayout
	items []titleItem
	focus int
	tr    evstrings.Table
	rec   Records
	// embers is how many embers rise, which the quality preset sets; still draws the cover without motion, as reduced motion asks.
	embers int
	still  bool
	// under is whether settings covers the title, which then shows only its cover.
	under bool
}

// drawTitle draws the title on the arena canvas: the cover, then the wordmark, the records and the painted menu fading in over it.
func drawTitle(p *overlayPainter, kit *titleArt, m *titleMotion, v titleView) {
	d := p.d
	d.Save()
	d.SetImageSmoothing(true)
	t := m.clock
	fade := float32(m.fade())
	if v.still {
		t, fade = 0, 1
	}
	if kit.animated() {
		drawCover(d, kit, v.l, t, v.embers, v.still)
	} else {
		drawCoverFallback(d, v.l)
	}
	if v.under {
		// Settings covers the title, so only the still cover shows beneath it.
		d.Restore()
		return
	}
	d.SetAlpha(fade)
	drawTitleShade(d, v.l)
	drawTitleLogo(p, kit, v.l)
	drawTitleRecords(p, v)
	for i, it := range v.items {
		if i < len(v.l.buttons) {
			drawTitleButton(p, kit, it.id, it.label, v.l.buttons[i], v.l.buttonScale, i == v.focus, fade)
		}
	}
	d.Restore()
}

// drawCover draws the kit's layers back to front at time t: the background breathing, the mist drifting, the lantern's glow flickering behind and over the keeper, embers rising and the canopy framing it all, each layer drifting by its own parallax.
func drawCover(d *core.Drawing, kit *titleArt, l titleLayout, t float64, embers int, still bool) {
	k := l.k
	drift := float32(math.Sin(2 * math.Pi * titleDriftHz * t))
	layer := func(name string, parallax, dx, dy, zoom float32) {
		r := l.cover
		x0 := r.x - r.w*(zoom-1)/2 + (drift*titleDrift*parallax+dx)*k
		y0 := r.y - r.h*(zoom-1)/2 + dy*k
		kz := k * zoom
		for _, p := range kit.pieces[name] {
			p.draw(d, x0+p.x*kz, y0+p.y*kz, p.w*kz, p.h*kz)
		}
	}
	breathe := float32(0.01 * (1 + math.Sin(2*math.Pi*t/titleSwayTime/2)))
	layer(titleBG, 0.15, 0, 0, titleZoom+breathe)

	mist := float32(math.Sin(2*math.Pi*titleMistHz*t + 1))
	d.SetAlpha(0.8 + 0.15*float32(math.Sin(t*0.7)))
	layer(titleMist, 0.35, mist*titleDrift, 0, titleZoom)
	d.SetAlpha(1)

	sway := float32(math.Sin(2 * math.Pi * t / titleSwayTime))
	keeperDX, keeperDY := drift*titleDrift*0.6, sway*titleKeeperSway
	flicker := float32(0.5 + 0.3*math.Sin(t*9) + 0.2*math.Sin(t*23+1.3))
	if still {
		flicker = 0.7
	}
	// The glow and embers follow the keeper's lantern, which the zoom moves out from the centre too.
	zx := l.cover.x - l.cover.w*(titleZoom-1)/2
	zy := l.cover.y - l.cover.h*(titleZoom-1)/2
	gx := zx + (lanternX*titleZoom+keeperDX)*k
	gy := zy + (lanternY*titleZoom+keeperDY)*k
	glow, _ := kit.piece(titleGlow)
	side := titleGlowBack * k * (0.94 + 0.12*flicker)
	d.SetAlpha(0.75 + 0.25*flicker)
	glow.draw(d, gx-side/2, gy-side/2, side, side)
	d.SetAlpha(1)

	layer(titleKeeper, 0.6, 0, keeperDY, titleZoom)

	d.Save()
	d.SetComposite(core.CompositeLighter)
	front := titleGlowFront * k * (0.9 + 0.2*flicker)
	d.SetAlpha(0.35 + 0.25*flicker)
	glow.draw(d, gx-front/2, gy-front/2, front, front)
	if !still {
		for i := range embers {
			drawEmber(d, kit, i, t, zx+keeperDX*k, zy+keeperDY*k, k*titleZoom)
		}
	}
	d.Restore()

	layer(titleCanopy, 1, sway*6, 0, titleZoom)
}

// drawEmber draws ember i at time t, rising from the lantern or the knoll and fading out as it climbs. Each ember's path follows from its index, so the cover holds no particle state; ox, oy and k place the kit's pixels.
func drawEmber(d *core.Drawing, kit *titleArt, i int, t float64, ox, oy, k float32) {
	r := func(n int) float64 { return emberHash(i*7 + n) }
	period := 4 + 3*r(1)
	phase := math.Mod(t/period+r(2), 1)
	var x0, y0 float64
	if r(3) < 0.35 {
		x0, y0 = lanternX+(r(4)-0.5)*60, lanternY+(r(5)-0.5)*30
	} else {
		x0 = knollLeft + (knollRight-knollLeft)*r(4)
		y0 = knollTop + (knollBottom-knollTop)*r(5)
	}
	y := y0 - phase*titleEmberRise*(0.7+0.6*r(6))
	x := x0 + math.Sin(t*1.3+r(7)*2*math.Pi)*titleSway
	side := float32(titleEmberSide*(titleEmberMin+titleEmberK*r(8))) * k
	d.SetAlpha(float32(math.Sin(phase * math.Pi)))
	kit.embers[i%len(kit.embers)].draw(d, ox+float32(x)*k-side/2, oy+float32(y)*k-side/2, side, side)
}

// emberHash is a fixed pseudo-random number in [0, 1) for n.
func emberHash(n int) float64 {
	x := uint32(n)*0x9e3779b9 + 0x7f4a7c15
	x ^= x >> 16
	x *= 0x85ebca6b
	x ^= x >> 13
	x *= 0xc2b2ae35
	x ^= x >> 16
	return float64(x) / (1 << 32)
}

// drawCoverFallback draws the still procedural cover: a night sky over dark ground and the lantern's warm light where the keeper would stand.
func drawCoverFallback(d *core.Drawing, l titleLayout) {
	c := l.cover
	sky := core.CanvasLinear(0, c.y, 0, c.y+c.h,
		core.GradientStop{Offset: 0, Color: core.RGB(0x0b0d17)},
		core.GradientStop{Offset: 0.7, Color: core.RGB(0x14121a)},
		core.GradientStop{Offset: 1, Color: core.RGB(0x1a120c)},
	)
	d.SetFillGradient(sky).FillRect(c.x, c.y, c.w, c.h)
	gx, gy := c.x+lanternX*l.k, c.y+lanternY*l.k
	glow := core.CanvasRadial(gx, gy, 0, gx, gy, titleGlowBack*l.k,
		core.GradientStop{Offset: 0, Color: core.RGBA(0xffb347, 0.45)},
		core.GradientStop{Offset: 1, Color: core.RGBA(0xffb347, 0)},
	)
	d.SetFillGradient(glow).FillRect(c.x, c.y, c.w, c.h)
}

// drawTitleShade darkens the bottom of the cover from the records down, so they and the menu read over the knoll.
func drawTitleShade(d *core.Drawing, l titleLayout) {
	top := l.recordsHeading.y - 3*l.recordsHeading.h
	bottom := l.cover.y + l.cover.h
	if len(l.buttons) > 0 {
		bottom = max(bottom, l.buttons[0].y+l.buttons[0].h)
	}
	shade := core.CanvasLinear(0, top, 0, bottom,
		core.GradientStop{Offset: 0, Color: core.RGBA(0x05040a, 0)},
		core.GradientStop{Offset: 0.4, Color: core.RGBA(0x05040a, 0.55)},
		core.GradientStop{Offset: 1, Color: core.RGBA(0x05040a, 0.8)},
	)
	d.SetFillGradient(shade).FillRect(l.cover.x, top, l.cover.w, bottom-top)
}

// drawTitleLogo draws the language's painted wordmark, or the game's name in canvas text when the kit lacks it.
func drawTitleLogo(p *overlayPainter, kit *titleArt, l titleLayout) {
	name := titleLogoEN
	if p.lang == string(evstrings.ZhCN) {
		name = titleLogoZhCN
	}
	if logo, ok := kit.piece(name); ok {
		logo.draw(p.d, l.logo.x, l.logo.y, l.logo.w, l.logo.h)
		return
	}
	cx, cy := l.logo.center()
	size := titleHeadingFontPt * l.scale
	p.d.Save()
	p.d.SetShadow(core.RGBA(0xff9a3c, 0.5), 0, 0, 12*l.scale)
	p.text(fitText(p.d, p.tr.GameTitle, p.font(size, true), l.logo.w), cx, cy, size, true, hudIvory, core.TextAlignCenter)
	p.d.Restore()
}

// drawTitleRecords draws the best records as one quiet line above the menu, under a small heading.
func drawTitleRecords(p *overlayPainter, v titleView) {
	l := v.l
	size := titleRecordFontPt * max(l.scale, 0.85)
	cx, cy := l.recordsHeading.center()
	p.text(v.tr.BestRecords, cx, cy, size, true, hudGold, core.TextAlignCenter)
	parts := titleRecordTexts(v.tr, v.rec)
	if len(parts) == 1 {
		r := l.records[0]
		row := rect{r.x, r.y, r.w * 4, r.h}
		cx, cy := row.center()
		p.text(parts[0], cx, cy, size, false, overlayDim, core.TextAlignCenter)
		return
	}
	for i, s := range parts {
		r := l.records[i]
		cx, cy := r.center()
		p.text(fitText(p.d, s, p.font(size, false), r.w-4), cx, cy, size, false, overlayDim, core.TextAlignCenter)
	}
}

// titleRecordTexts are the record line's parts: the best time, kills, level and wins, or one line saying there are none yet.
func titleRecordTexts(tr evstrings.Table, rec Records) []string {
	if !rec.HasRecord {
		return []string{tr.NoRecordsYet}
	}
	return []string{tr.BestTime(clockText(rec.BestTime)), tr.BestKills(rec.BestKills), tr.BestLevel(rec.BestLevel), tr.Wins(rec.Wins)}
}

// drawTitleButton draws a title button in the kit's nine-slice frame for its state, normal, focused or pressed, with its label; without the frames it draws the overlays' painted button.
func drawTitleButton(p *overlayPainter, kit *titleArt, id, label string, r rect, u float32, focused bool, fade float32) {
	pressed := id == p.pressed
	name := titleButton
	switch {
	case pressed:
		name = titleButtonPress
	case focused:
		name = titleButtonFocus
	}
	frame, ok := kit.piece(name)
	if !ok {
		kind := buttonPlain
		if id == "title-start" {
			kind = buttonEmber
		}
		p.button(id, label, r, u, kind, focused)
		return
	}
	p.nineSliceFrom(frame.img, frame.sx, frame.sy, frame.sw, frame.sh, buttonInsets, r, titleButtonInsetPt*u)
	if focused {
		// A pulse of the focused frame over itself, as a blurred shadow costs too much on every frame.
		p.d.Save()
		p.d.SetComposite(core.CompositeLighter).SetAlpha(0.25 * (0.5 + 0.5*float32(math.Sin(p.m.clock*5))) * fade)
		p.nineSliceFrom(frame.img, frame.sx, frame.sy, frame.sw, frame.sh, buttonInsets, r, titleButtonInsetPt*u)
		p.d.Restore()
	}
	color := hudIvory
	if focused {
		color = core.RGB(0xffe4a8)
	}
	cx, cy := r.center()
	if pressed {
		cy += 1 * u
	}
	inner := r.w - 2*titleButtonInsetPt*u
	p.text(fitText(p.d, label, p.font(titleButtonFontPt*u, true), inner), cx, cy, titleButtonFontPt*u, true, color, core.TextAlignCenter)
}
