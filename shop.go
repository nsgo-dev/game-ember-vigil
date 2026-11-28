package main

import (
	"math"
	"strconv"

	"github.com/nsgo-dev/nsgo/core"
	evstrings "me.oneo/embervigil/strings"
)

// The shop's pieces in the HUD kit, which assets/hud/README.md describes.
const (
	hudShopRow        = "shop_row"
	hudShopRowFocused = "shop_row_focused"
	hudEmberEmblem    = "ember_emblem"
)

// shopRowInsets are the shop rows' nine-slice insets in the kit's pixels: top, right, bottom and left. The left inset holds the icon well.
var shopRowInsets = [4]float32{32, 40, 32, 136}

// The shop panel's geometry in points at a scale of 1: two columns of rows the kit's 1× height, the icon well's centre in a row, and the text sizes.
const (
	shopColumns   = 2
	shopRowW      = 320
	shopRowRight  = 26
	shopRowH      = 64
	shopRowGap    = 8
	shopWellX     = 35
	shopIcon      = 32
	shopTextX     = 74
	shopNamePt    = 14
	shopLinePt    = 11
	shopCostPt    = 13
	shopPip       = 10
	shopBalanceH  = 24
	shopNoteH     = 14
	shopEmblemPt  = 20
	shopBalancePt = 20
)

// shopEntry is what one shop row says: the item's name and effect, its level of the most, its bonus now and the cost of its next level, which maxed is set instead of.
type shopEntry struct {
	id           string
	name, effect string
	level, most  int
	now          string
	cost         int
	maxed        bool
	affordable   bool
}

// shopEntries is what the shop says of each item, given the progression.
func shopEntries(tr evstrings.Table, shop *shopDef, p Progression) []shopEntry {
	out := make([]shopEntry, len(shop.Items))
	for i := range shop.Items {
		it := &shop.Items[i]
		c := tr.ShopItem(it.ID, it.Name, it.Description)
		level := p.shopLevel(shop, it.ID)
		e := shopEntry{id: it.ID, name: c.Name, effect: c.Description, level: level, most: shop.maxLevel(), now: tr.ShopNow(shopBonus(it, level))}
		if cost, ok := p.shopCost(shop, it.ID); ok {
			e.cost, e.affordable = cost, cost <= p.Embers
		} else {
			e.maxed = true
		}
		out[i] = e
	}
	return out
}

// shopBonus writes what level levels of it add, such as "+6%", "-4%", "+20" or, for redraws, "+2", and "0" or "0%" before the first level.
func shopBonus(it *shopItemDef, level int) string {
	v := it.Amount * float64(level)
	unit := ""
	if it.Stat == statDamage || it.Stat == statCooldown {
		v, unit = v*100, "%"
	}
	sign := "+"
	switch {
	case level == 0:
		sign = ""
	case it.Stat == statCooldown:
		sign = "-"
	}
	return sign + strconv.FormatFloat(math.Round(v*100)/100, 'f', -1, 64) + unit
}

// shopLabel is what assistive technology reads of an entry: its name, effect, level, bonus now and next cost.
func shopLabel(tr evstrings.Table, e shopEntry) string {
	next := tr.ShopMax
	if !e.maxed {
		next = tr.ShopCost(e.cost)
	}
	return e.name + ", " + e.effect + ", " + tr.Level(e.level) + "/" + strconv.Itoa(e.most) + ", " + e.now + ", " + next
}

// shopLayout places the shop panel: its heading, the balance, the rows in two columns, the note on the tiers and Back.
type shopLayout struct {
	scale          float32
	panel, heading rect
	balance        rect
	rows           []rect
	note           rect
	back           rect
}

// layoutShop lays n rows two to a line under the heading and the balance, the note and Back below them.
func layoutShop(size core.Size, safe core.Edges, n int) shopLayout {
	area := overlayArea(size, safe)
	lines := (n + shopColumns - 1) / shopColumns
	rowsH := float32(lines)*shopRowH + float32(max(0, lines-1))*shopRowGap
	panelW := float32(2*overlayPad + shopColumns*shopRowW + (shopColumns-1)*12)
	h := float32(overlayPad+32+4+shopBalanceH+10) + rowsH + float32(8+shopNoteH+14+overlayButtonH+overlayPad)
	u := overlayScale(size, area, panelW, h)
	l := shopLayout{scale: u, panel: centred(area, panelW*u, h*u)}
	x := l.panel.x + overlayPad*u
	y := l.panel.y + overlayPad*u
	l.heading = rect{l.panel.x, y, l.panel.w, 32 * u}
	y += 36 * u
	l.balance = rect{x, y, l.panel.w - 2*overlayPad*u, shopBalanceH * u}
	y += (shopBalanceH + 10) * u
	for i := range n {
		col, line := i%shopColumns, i/shopColumns
		l.rows = append(l.rows, rect{x + float32(col)*(shopRowW+12)*u, y + float32(line)*(shopRowH+shopRowGap)*u, shopRowW * u, shopRowH * u})
	}
	y += rowsH*u + 8*u
	l.note = rect{x, y, l.panel.w - 2*overlayPad*u, shopNoteH * u}
	y += (shopNoteH + 14) * u
	l.back = buttonRow(l.panel, y, 1, u)[0]
	return l
}

// shopFocusMove moves the shop's focus over n rows two to a line and Back after them: up and down by line, past the last line to Back and around, left and right across the line.
func shopFocusMove(focus, n, dx, dy int) int {
	back := n
	switch {
	case dy > 0 && focus == back:
		return 0
	case dy > 0 && focus+shopColumns < n:
		return focus + shopColumns
	case dy > 0:
		return back
	case dy < 0 && focus == back:
		return (n - 1) / shopColumns * shopColumns
	case dy < 0 && focus >= shopColumns:
		return focus - shopColumns
	case dy < 0:
		return back
	case dx != 0 && focus < back:
		next := focus - focus%shopColumns + (focus%shopColumns+dx+shopColumns)%shopColumns
		if next >= n {
			return focus - focus%shopColumns
		}
		return next
	}
	return focus
}

// drawShop draws the shop over the title's cover: the balance, each item's row with its icon, name, effect, bonus now, level pips and next cost, the note on the tiers and Back.
func drawShop(p *overlayPainter, l shopLayout, entries []shopEntry, embers, focus int) {
	tr := p.tr
	u := l.scale
	p.d.Save()
	p.d.SetImageSmoothing(true)
	p.fadeIn(l.panel)
	p.panel(l.panel, u)
	p.heading(tr.ShopTitle, l.heading, u)
	p.balance(l.balance, embers, u)
	for i, e := range entries {
		if i < len(l.rows) {
			p.shopRow(l.rows[i], e, focus == i, u)
		}
	}
	cx, cy := l.note.center()
	p.text(fitText(p.d, tr.ShopHalved, p.font(shopLinePt*u, false), l.note.w), cx, cy, shopLinePt*u, false, overlayDim, core.TextAlignCenter)
	p.button("shop-back", tr.Back, l.back, u, buttonPlain, focus == len(entries))
	p.d.Restore()
}

// balance draws the ember balance centred in r: the kit's ember emblem and the painted numerals after a ×.
func (p *overlayPainter) balance(r rect, embers int, u float32) {
	cx, cy := r.center()
	side := shopEmblemPt * u
	if img, ok := p.art.image(hudEmberEmblem); ok {
		img.drawIn(p.d, cx-side-4*u, cy-side/2, side, side)
	} else {
		p.d.SetFillColor(overlayEmber).BeginPath().Ellipse(cx-side/2-4*u, cy, side/2.4, side/2.4).Fill()
	}
	hp := hudPainter{d: p.d, art: p.art}
	hp.drawNumber(strconv.Itoa(embers), cx, cy, shopBalancePt*u, core.TextAlignLeft, true)
}

// shopRow draws one item's row in the kit's row frame, lit when focused, or an iron plate without the kit.
func (p *overlayPainter) shopRow(r rect, e shopEntry, focused bool, u float32) {
	if e.id != "" && "shop-item-"+e.id == p.pressed {
		r.y += 2 * u
	}
	frame := hudShopRow
	if focused {
		frame = hudShopRowFocused
	}
	if img, ok := p.art.image(frame); ok {
		// The kit is 2×, so its 136-pixel left inset spans 68 points at a scale of 1, keeping the well round.
		p.nineSlice(img, shopRowInsets, r, shopRowInsets[3]/2*u)
	} else {
		border := hudBronze
		if focused {
			border = hudGold
		}
		p.d.SetFillColor(core.RGBA(0x15120e, 0.96)).BeginPath().RoundRect(r.x, r.y, r.w, r.h, 8*u).Fill()
		p.d.SetStrokeColor(border).SetLineWidth(2*u).BeginPath().RoundRect(r.x+u, r.y+u, r.w-2*u, r.h-2*u, 8*u).Stroke()
	}
	wx, wy := r.x+shopWellX*u, r.y+r.h/2
	icon := shopIcon * u
	if img, ok := p.icons.cardIcon(e.id); ok {
		img.drawIn(p.d, wx-icon/2, wy-icon/2, icon, icon)
	} else {
		p.d.SetFillColor(overlayEmber).BeginPath().Ellipse(wx, wy, icon/3, icon/3).Fill()
	}

	// The next cost, or MAX, sits right of the name; the effect and the pips take the whole width under them.
	right := r.x + r.w - shopRowRight*u
	nameY := r.y + 15*u
	costW := float32(0)
	// Text is measured and drawn from its left, so it ends at right.
	if e.maxed {
		costW = p.d.MeasureText(p.tr.ShopMax, p.font(shopCostPt*u, true))
		p.text(p.tr.ShopMax, right-costW, nameY, shopCostPt*u, true, hudGold, core.TextAlignLeft)
	} else {
		color := hudIvory
		if !e.affordable {
			color = overlayDim
		}
		cost := strconv.Itoa(e.cost)
		side := 14 * u
		textW := p.d.MeasureText(cost, p.font(shopCostPt*u, true))
		costW = textW + side + 3*u
		p.text(cost, right-textW, nameY, shopCostPt*u, true, color, core.TextAlignLeft)
		if img, ok := p.art.image(hudEmberEmblem); ok {
			img.drawIn(p.d, right-costW, nameY-side/2, side, side)
		}
	}
	tx := r.x + shopTextX*u
	tw := right - tx
	nameColor := hudIvory
	if focused {
		nameColor = core.RGB(0xffe4a8)
	}
	p.text(fitText(p.d, e.name, p.font(shopNamePt*u, true), tw-costW-6*u), tx, nameY, shopNamePt*u, true, nameColor, core.TextAlignLeft)
	p.text(fitText(p.d, e.effect, p.font(shopLinePt*u, false), tw), tx, r.y+31*u, shopLinePt*u, false, overlayDim, core.TextAlignLeft)
	hp := hudPainter{d: p.d, art: p.art, l: hudLayout{scale: u}}
	// drawPips centres the pips on the rectangle's bottom edge.
	pips := rect{tx, r.y + 42*u, shopPip * u * float32(e.most), shopPip * u}
	hp.drawPips(pips, e.level, e.most)
	nowX := pips.x + pips.w + 8*u
	p.text(fitText(p.d, e.now, p.font(shopLinePt*u, true), max(0, tx+tw-nowX)), nowX, r.y+52*u, shopLinePt*u, true, overlayEmber, core.TextAlignLeft)
}

// shopView is the native layer over the canvas's shop: the heading, the balance and the note for assistive technology, an invisible button over each item that buys it and Back.
func (g *emberVigil) shopView() core.Node {
	tr := g.tr()
	shop := &mustGameData().progression.Shop
	_ = g.shopVersion.Get()
	entries := shopEntries(tr, shop, g.progress)
	l := layoutShop(g.viewSize.Get(), g.cx.Env().SafeArea(), len(entries))
	children := []core.Node{
		infoView("shop-heading", tr.ShopTitle, l.heading),
		infoView("shop-balance", tr.EmberBalance(g.progress.Embers), l.balance),
	}
	for i, e := range entries {
		idx, id := i, e.id
		children = append(children, g.hitButton("shop-item-"+id, e.name, shopLabel(tr, e), l.rows[i], func() { g.buy(id) }, func() { g.hoverMenu(idx) }))
	}
	children = append(children,
		infoView("shop-note", tr.ShopHalved, l.note),
		g.hitButton("shop-back", tr.Back, "", l.back, g.closeShop, func() { g.hoverMenu(len(entries)) }),
	)
	return overlayLayer("shop", tr.ShopTitle, children...)
}
