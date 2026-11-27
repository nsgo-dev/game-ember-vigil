package main

import (
	"testing"

	"github.com/nsgo-dev/nsgo/core"
)

// TestSettingsLayoutFitsTheSafeArea checks that the settings panel, its rows and their controls stay inside the safe area at desktop and phone sizes, with rows that never overlap, in two columns on a phone held sideways.
func TestSettingsLayoutFitsTheSafeArea(t *testing.T) {
	for _, c := range []struct {
		size core.Size
		safe core.Edges
		cols int
	}{
		{core.Size{Width: 1280, Height: 720}, core.Edges{}, 1},
		{core.Size{Width: 844, Height: 390}, core.Edges{Left: 47, Right: 47, Bottom: 21}, 2},
		{core.Size{Width: 390, Height: 844}, core.Edges{Top: 47, Bottom: 34}, 1},
	} {
		l := layoutSettings(c.size, c.safe)
		area := overlayArea(c.size, c.safe)
		inside := func(r, in rect) bool {
			return r.x >= in.x-0.5 && r.y >= in.y-0.5 && r.x+r.w <= in.x+in.w+0.5 && r.y+r.h <= in.y+in.h+0.5
		}
		if !inside(l.panel, area) {
			t.Errorf("%v: the panel %v leaves the safe area %v", c.size, l.panel, area)
		}
		cols := map[float32]bool{}
		for i, it := range l.items {
			cols[it.row.x] = true
			for _, ctl := range it.controls {
				if !inside(ctl, it.row) {
					t.Errorf("%v: row %d's control %v leaves its row %v", c.size, i, ctl, it.row)
				}
			}
			for j := range i {
				o := l.items[j].row
				if it.row.x < o.x+o.w && o.x < it.row.x+it.row.w && it.row.y < o.y+o.h && o.y < it.row.y+it.row.h {
					t.Errorf("%v: rows %d and %d overlap", c.size, j, i)
				}
			}
		}
		if len(cols) != c.cols {
			t.Errorf("%v: the rows lie in %d columns, want %d", c.size, len(cols), c.cols)
		}
		for _, b := range l.buttons {
			if !inside(b, l.panel) {
				t.Errorf("%v: Apply or Back %v leaves the panel", c.size, b)
			}
		}
	}
}
