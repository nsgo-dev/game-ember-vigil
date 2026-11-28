package main

import (
	"math"
	"slices"

	"github.com/nsgo-dev/nsgo/core"
	evstrings "me.oneo/embervigil/strings"
)

// The collection's slot frames in the HUD kit, which assets/hud/README.md describes.
const (
	hudCollectionLocked   = "collection_locked"
	hudCollectionUnlocked = "collection_unlocked"
)

// The collection panel's geometry in points at a scale of 1.
const (
	collectionPanelW  = 520
	collectionRowH    = 60
	collectionRowGap  = 6
	collectionSlot    = 48
	collectionIcon    = 32
	collectionNamePt  = 15
	collectionLinePt  = 11.5
	collectionMadePt  = 12
	collectionTextGap = 12
)

// recipe is one evolution the collection lists: its weapon and passive and, in English, what data/weapons.yaml calls them.
type recipe struct {
	id, weaponID, passiveID string
	name, description, hint string
	weaponName, passiveName string
}

// recipes lists every evolution in data/weapons.yaml, in its order.
func recipes(d *gameData) []recipe {
	var out []recipe
	for i := range d.weapons.Weapons {
		w := &d.weapons.Weapons[i]
		evo, p := w.Evolution, d.evolutionPassive[i]
		if evo == nil || p < 0 {
			continue
		}
		passive := &d.upgrades.Passives[p]
		out = append(out, recipe{
			id: evo.ID, weaponID: w.ID, passiveID: passive.ID,
			name: evo.Name, description: evo.Description, hint: evo.Hint,
			weaponName: w.Name, passiveName: passive.Name,
		})
	}
	return out
}

// collectionEntry is what one collection row says: a made recipe's name, weapon and passive, and description, or "???" and the hint of one never made.
type collectionEntry struct {
	id         string
	made       bool
	name, line string
	detail     string
}

// collectionEntries is what the collection says of each recipe, given the ids the progression has collected.
func collectionEntries(tr evstrings.Table, list []recipe, collected []string) []collectionEntry {
	out := make([]collectionEntry, len(list))
	for i, r := range list {
		c := tr.Evolution(r.id, r.name, r.description, r.hint)
		e := collectionEntry{id: r.id, made: slices.Contains(collected, r.id)}
		if e.made {
			weapon := tr.Card(r.weaponID, r.weaponName, "").Name
			passive := tr.Card(r.passiveID, r.passiveName, "").Name
			e.name, e.line, e.detail = c.Name, weapon+tr.RecipeSeparator+passive, c.Description
		} else {
			e.name, e.line = tr.CollectionUnknown, c.Hint
		}
		out[i] = e
	}
	return out
}

// madeCount is how many entries are made.
func madeCount(entries []collectionEntry) int {
	n := 0
	for _, e := range entries {
		if e.made {
			n++
		}
	}
	return n
}

// collectionRow is one row of the collection: the whole row, its slot, and its name and two lines of text.
type collectionRow struct {
	row, slot, name, line, detail rect
}

// collectionLayout places the collection panel: its heading, how many recipes are made, a row for each and Back.
type collectionLayout struct {
	scale          float32
	panel, heading rect
	made           rect
	rows           []collectionRow
	back           rect
}

// layoutCollection lays n recipe rows in one column under the heading, Back below them.
func layoutCollection(size core.Size, safe core.Edges, n int) collectionLayout {
	area := overlayArea(size, safe)
	rowsH := float32(n)*collectionRowH + float32(max(0, n-1))*collectionRowGap
	h := float32(overlayPad+32+4+16+12) + rowsH + float32(14+overlayButtonH+overlayPad)
	u := overlayScale(size, area, collectionPanelW, h)
	l := collectionLayout{scale: u, panel: centred(area, collectionPanelW*u, h*u)}
	x, w := l.panel.x+overlayPad*u, l.panel.w-2*overlayPad*u
	y := l.panel.y + overlayPad*u
	l.heading = rect{l.panel.x, y, l.panel.w, 32 * u}
	y += 36 * u
	l.made = rect{x, y, w, 16 * u}
	y += 28 * u
	for range n {
		row := rect{x, y, w, collectionRowH * u}
		slot := rect{x + 6*u, y + (collectionRowH-collectionSlot)/2*u, collectionSlot * u, collectionSlot * u}
		tx := slot.x + slot.w + collectionTextGap*u
		tw := row.x + row.w - 8*u - tx
		l.rows = append(l.rows, collectionRow{
			row:    row,
			slot:   slot,
			name:   rect{tx, y + 6*u, tw, 18 * u},
			line:   rect{tx, y + 25*u, tw, 15 * u},
			detail: rect{tx, y + 41*u, tw, 15 * u},
		})
		y += (collectionRowH + collectionRowGap) * u
	}
	y += (14 - collectionRowGap) * u
	l.back = buttonRow(l.panel, y, 1, u)[0]
	return l
}

// drawCollection draws the collection over the title's cover: each recipe's slot, lit with its icon once made or engraved "???" until then, its words, and Back.
func drawCollection(p *overlayPainter, l collectionLayout, entries []collectionEntry) {
	tr := p.tr
	u := l.scale
	p.d.Save()
	p.d.SetImageSmoothing(true)
	p.fadeIn(l.panel)
	p.panel(l.panel, u)
	p.heading(tr.CollectionTitle, l.heading, u)
	cx, cy := l.made.center()
	p.text(tr.CollectionMade(madeCount(entries), len(entries)), cx, cy, collectionMadePt*u, true, overlayDim, core.TextAlignCenter)
	for i, e := range entries {
		if i >= len(l.rows) {
			break
		}
		r := l.rows[i]
		p.d.SetFillColor(core.RGBA(0x0b0a0e, 0.35)).BeginPath().RoundRect(r.row.x, r.row.y, r.row.w, r.row.h, 8*u).Fill()
		p.collectionSlot(r.slot, e, u)
		nameColor := hudIvory
		if !e.made {
			nameColor = overlayDim
		}
		_, ny := r.name.center()
		p.text(fitText(p.d, e.name, p.font(collectionNamePt*u, true), r.name.w), r.name.x, ny, collectionNamePt*u, true, nameColor, core.TextAlignLeft)
		lineColor := overlayEmber
		if !e.made {
			lineColor = overlayDim
		}
		_, ly := r.line.center()
		p.text(fitText(p.d, e.line, p.font(collectionLinePt*u, !e.made), r.line.w), r.line.x, ly, collectionLinePt*u, !e.made, lineColor, core.TextAlignLeft)
		if e.detail != "" {
			_, dy := r.detail.center()
			p.text(fitText(p.d, e.detail, p.font(collectionLinePt*u, false), r.detail.w), r.detail.x, dy, collectionLinePt*u, false, overlayDim, core.TextAlignLeft)
		}
	}
	p.button("collection-back", tr.Back, l.back, u, buttonPlain, true)
	p.d.Restore()
}

// collectionSlot draws a recipe's slot: the kit's lit frame with the evolution's icon once made, its "???" frame until then, or painted stand-ins without the kit.
func (p *overlayPainter) collectionSlot(r rect, e collectionEntry, u float32) {
	frame := hudCollectionLocked
	if e.made {
		frame = hudCollectionUnlocked
	}
	if img, ok := p.art.image(frame); ok {
		img.drawIn(p.d, r.x, r.y, r.w, r.h)
	} else {
		border := hudBronze
		if e.made {
			border = hudGold
		}
		p.d.SetFillColor(hudIron).BeginPath().RoundRect(r.x, r.y, r.w, r.h, 6*u).Fill()
		p.d.SetStrokeColor(border).SetLineWidth(2*u).BeginPath().RoundRect(r.x+u, r.y+u, r.w-2*u, r.h-2*u, 6*u).Stroke()
		if !e.made {
			cx, cy := r.center()
			p.text(p.tr.CollectionUnknown, cx, cy, 14*u, true, overlayDim, core.TextAlignCenter)
		}
	}
	if !e.made {
		return
	}
	icon := collectionIcon * r.w / collectionSlot
	cx, cy := r.center()
	if img, ok := p.icons.cardIcon(e.id); ok {
		img.drawIn(p.d, cx-icon/2, cy-icon/2, icon, icon)
		return
	}
	pulse := 0.85 + 0.15*float32(math.Sin(p.m.clock*3))
	p.d.SetFillColor(core.RGBA(0xffd166, pulse)).BeginPath().Ellipse(cx, cy, icon/2.5, icon/2.5).Fill()
}

// collectionView is the native layer over the canvas's collection: the heading, the count and each recipe's words for assistive technology, and an invisible Back button.
func (g *emberVigil) collectionView() core.Node {
	tr := g.tr()
	entries := collectionEntries(tr, recipes(mustGameData()), g.progress.Collection)
	l := layoutCollection(g.viewSize.Get(), g.cx.Env().SafeArea(), len(entries))
	children := []core.Node{
		infoView("collection-heading", tr.CollectionTitle, l.heading),
		infoView("collection-made", tr.CollectionMade(madeCount(entries), len(entries)), l.made),
	}
	for i, e := range entries {
		label := e.name + ", " + e.line
		if e.detail != "" {
			label += ", " + e.detail
		}
		children = append(children, infoView("collection-recipe-"+e.id, label, l.rows[i].row))
	}
	children = append(children, g.hitButton("collection-back", tr.Back, "", l.back, g.closeCollection, func() { g.hoverMenu(0) }))
	return overlayLayer("collection", tr.CollectionTitle, children...)
}
