package main

import (
	"math"
	"strconv"
	"time"

	"github.com/nsgo-dev/nsgo/core"
)

// fpsWindow is how long the frame rate readout averages over, so it steadies instead of flickering every frame.
const fpsWindow = 500 * time.Millisecond

// fpsMeter counts display frames over fpsWindow and keeps the window's rate and its longest frame, which a stutter shows in even when the average holds.
type fpsMeter struct {
	frames  int
	elapsed time.Duration
	longest time.Duration
	// text is the last window's readout, built once a window so drawing allocates nothing.
	text string
}

// observe adds one display frame of length dt; a frame that adds no time, as one after a pause, is left out.
func (m *fpsMeter) observe(dt time.Duration) {
	if dt <= 0 {
		return
	}
	m.frames++
	m.elapsed += dt
	m.longest = max(m.longest, dt)
	if m.elapsed < fpsWindow {
		return
	}
	fps := int(math.Round(float64(m.frames) / m.elapsed.Seconds()))
	m.text = strconv.Itoa(fps) + " FPS · " + strconv.Itoa(int(math.Round(float64(m.longest)/float64(time.Millisecond)))) + " ms"
	m.frames, m.elapsed, m.longest = 0, 0, 0
}

// drawFPS writes the frame rate readout at the top-right of the safe area, under the pause button.
func drawFPS(p *overlayPainter, size core.Size, safe core.Edges, m *fpsMeter) {
	if m.text == "" {
		return
	}
	l := layoutHUD(size, safe)
	pt := 11 * l.scale
	p.d.Save()
	p.text(m.text, l.pause.x+l.pause.w, l.pause.y+l.pause.h+pt, pt, true, hudIvory, core.TextAlignRight)
	p.d.Restore()
}
