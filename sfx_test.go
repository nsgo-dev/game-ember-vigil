package main

import (
	"testing"

	"github.com/nsgo-dev/game/packages/game/placeholder"
)

// TestEveryCueDecodesForTheSoftwareMixer checks that each cue's placeholder WAV decodes to mono samples in range.
func TestEveryCueDecodesForTheSoftwareMixer(t *testing.T) {
	for c := cueNone + 1; c < cueCount; c++ {
		src, err := placeholder.Sound(cueSpecs[c].asset)
		if err != nil {
			t.Fatalf("%s: %v", c, err)
		}
		clip, err := decodeWAV(src.Data)
		if err != nil {
			t.Fatalf("%s: %v", c, err)
		}
		if clip.rate <= 0 || len(clip.samples) < int(clip.rate/50) {
			t.Fatalf("%s decoded %d samples at %v Hz", c, len(clip.samples), clip.rate)
		}
		for _, s := range clip.samples {
			if s < -1 || s > 1 {
				t.Fatalf("%s has a sample of %v", c, s)
			}
		}
	}
	if _, err := decodeWAV([]byte("not a wave")); err == nil {
		t.Fatal("decoded something that is not a WAVE file")
	}
}

// TestTheSoftwareMixerPlaysLimitsAndPauses checks that a queued effect sounds, that a cue past its limit takes its own oldest voice, that a pause holds gameplay effects but not the UI's, and that a clip ends.
func TestTheSoftwareMixerPlaysLimitsAndPauses(t *testing.T) {
	m := &sfxMixer{running: true}
	clip := &pcmClip{samples: make([]float32, 100), rate: sfxRate}
	for i := range clip.samples {
		clip.samples[i] = 0.5
	}
	sounding := func() int {
		n := 0
		for _, v := range m.voices {
			if v.clip != nil {
				n++
			}
		}
		return n
	}
	buf := make([]float32, 10)

	m.play(clip, cueHit, 2, 1, 1, true)
	m.render(buf)
	if buf[0] < 0.49 || buf[0] > 0.51 {
		t.Fatalf("one effect at full volume mixes to %v, want 0.5", buf[0])
	}
	m.play(clip, cueHit, 2, 1, 1, true)
	m.play(clip, cueHit, 2, 1, 1, true)
	m.render(buf)
	if got := sounding(); got != 2 {
		t.Fatalf("a cue limited to 2 sounds %d voices", got)
	}
	if buf[0] != 1 {
		t.Fatalf("two effects of 0.5 mix to %v, want 1 at most", buf[0])
	}

	m.play(clip, cueUIClick, 1, 1, 1, false)
	m.setPaused(true)
	m.render(buf)
	if buf[0] < 0.49 || buf[0] > 0.51 {
		t.Fatalf("a pause leaves %v, want only the UI's 0.5", buf[0])
	}
	m.setPaused(false)
	m.setMuted(true)
	m.render(buf)
	if buf[0] != 0 {
		t.Fatalf("a muted mixer plays %v", buf[0])
	}
	m.setMuted(false)
	for range 20 {
		m.render(buf)
	}
	if got := sounding(); got != 0 || buf[0] != 0 {
		t.Fatalf("%d voices still sound after their clips ended", got)
	}
}
