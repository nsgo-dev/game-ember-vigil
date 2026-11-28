package main

import (
	"testing"
	"time"
)

// TestFPSMeterAveragesAWindow checks that the readout waits for a whole window, then names its rate and longest frame.
func TestFPSMeterAveragesAWindow(t *testing.T) {
	var m fpsMeter
	m.observe(0)
	for range 14 {
		m.observe(time.Second / 30)
	}
	if m.text != "" {
		t.Fatalf("readout %q before a whole window", m.text)
	}
	m.observe(50 * time.Millisecond)
	if want := "29 FPS · 50 ms"; m.text != want {
		t.Fatalf("readout %q, want %q", m.text, want)
	}
}
