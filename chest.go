package main

import (
	"math"

	"github.com/nsgo-dev/nsgo/core"
)

// The chest banner's size in points at a scale of 1, and the share of the hold it takes to rise in.
const (
	chestBannerW    = 320
	chestBannerH    = 64
	chestBannerTop  = 0.3
	chestBannerRise = 0.25
)

// drawChestBanner names what an opened chest gave while the run holds still for it: the evolved weapon, or a free card. It rises in, glows and fades as the hold ends.
func drawChestBanner(p *overlayPainter, size core.Size, safe core.Edges, o OpeningView) {
	if !o.Active {
		return
	}
	area := overlayArea(size, safe)
	u := overlayScale(size, area, chestBannerW, chestBannerH)
	w, h := chestBannerW*u, chestBannerH*u
	in := float32(easeOut(min(1, o.Progress/chestBannerRise)))
	out := float32(min(1, (1-o.Progress)/chestBannerRise))
	alpha := min(in, out)
	r := rect{area.x + (area.w-w)/2, area.y + area.h*chestBannerTop - h/2 + (1-in)*12*u, w, h}
	tr := p.tr
	p.d.Save()
	p.d.SetImageSmoothing(true)
	p.d.SetAlpha(alpha)
	glow := 0.6 + 0.4*float32(math.Sin(o.Progress*math.Pi*4))
	p.d.SetShadow(core.RGBA(0xffd166, 0.8*glow), 0, 0, 24*u)
	p.panel(r, 0.6*u)
	p.d.SetShadow(core.RGBA(0, 0), 0, 0, 0)
	cx := r.x + r.w/2
	if o.Evolved != "" {
		name := tr.Evolution(o.Evolved, o.Name, "", "").Name
		p.text(tr.Evolved, cx, r.y+20*u, 11*u, true, overlayEmber, core.TextAlignCenter)
		p.text(fitText(p.d, name, p.font(20*u, true), r.w-24*u), cx, r.y+42*u, 20*u, true, core.RGB(0xffe4a8), core.TextAlignCenter)
	} else {
		p.text(tr.FreeCard, cx, r.y+r.h/2, 18*u, true, hudIvory, core.TextAlignCenter)
	}
	p.d.Restore()
}
