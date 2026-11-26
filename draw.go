package main

import "github.com/nsgo-dev/nsgo/core"

// The placeholder palette and sizes the arena draws with, until the generated atlas and the real palette arrive.
var (
	arenaColor  = core.RGB(0x0b0d17)
	gridColor   = core.RGBA(0x27303f, 0.55)
	keeperColor = core.RGB(0xff7a1a)
	emberColor  = core.RGB(0xffd166)
)

// gridStep is the distance between the placeholder grid's lines in virtual pixels, and keeperWidth and keeperHeight size the keeper's placeholder.
const (
	gridStep     = 24
	keeperWidth  = 10
	keeperHeight = 14
)

// drawArena draws the empty arena in virtual pixels: a placeholder grid and the keeper's placeholder at its centre.
func drawArena(d *core.Drawing, _ core.Size) {
	w, h := virtualWidth, virtualHeight
	d.SetFillColor(arenaColor).FillRect(0, 0, w, h)

	d.SetStrokeColor(gridColor).SetLineWidth(1).BeginPath()
	for x := float32(gridStep); x < w; x += gridStep {
		d.MoveTo(x, 0).LineTo(x, h)
	}
	for y := float32(gridStep); y < h; y += gridStep {
		d.MoveTo(0, y).LineTo(w, y)
	}
	d.Stroke()

	x, y := w/2-keeperWidth/2, h/2-keeperHeight/2
	d.SetFillColor(keeperColor).FillRect(x, y, keeperWidth, keeperHeight)
	d.SetFillColor(emberColor).FillRect(x+keeperWidth/2-2, y+3, 4, 4)
}
