package main

import (
	"bytes"
	"slices"
	"testing"

	"github.com/nsgo-dev/game/packages/game/placeholder"
)

// TestRunEventsMapToExpectedCues checks that every run EventKind and keeper damage
// maps to the expected audio cue and deduplicates high-frequency bursts per step.
func TestRunEventsMapToExpectedCues(t *testing.T) {
	cases := []struct {
		name          string
		events        []Event
		keeperDamaged bool
		want          []soundCue
	}{
		{
			name:   "shot",
			events: []Event{{Kind: EventShot}},
			want:   []soundCue{cueShoot},
		},
		{
			name:   "enemy hit",
			events: []Event{{Kind: EventHit, ID: 1}},
			want:   []soundCue{cueHit},
		},
		{
			name:          "keeper contact hit only",
			events:        []Event{{Kind: EventHit, ID: 1}},
			keeperDamaged: true,
			want:          []soundCue{cueKeeperHurt},
		},
		{
			name:          "keeper hurt plus enemy hit in same step",
			events:        []Event{{Kind: EventHit, ID: 1}, {Kind: EventHit, ID: 2}},
			keeperDamaged: true,
			want:          []soundCue{cueKeeperHurt, cueHit},
		},
		{
			name:   "kill and ember",
			events: []Event{{Kind: EventKill}, {Kind: EventEmber}},
			want:   []soundCue{cueKill, cueEmber},
		},
		{
			name:   "level up",
			events: []Event{{Kind: EventLevelUp}},
			want:   []soundCue{cueLevelUp},
		},
		{
			name:   "boss spawn and phase",
			events: []Event{{Kind: EventBossSpawn}, {Kind: EventBossPhase}},
			want:   []soundCue{cueBossSpawn, cueBossPhase},
		},
		{
			name:   "victory and defeat",
			events: []Event{{Kind: EventVictory}, {Kind: EventDefeat}},
			want:   []soundCue{cueVictory, cueDefeat},
		},
		{
			name: "burst deduplicates repeated events in one step",
			events: []Event{
				{Kind: EventShot}, {Kind: EventShot}, {Kind: EventShot},
				{Kind: EventHit}, {Kind: EventHit}, {Kind: EventKill}, {Kind: EventKill},
			},
			want: []soundCue{cueShoot, cueHit, cueKill},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mapRunEventsToCues(tc.events, tc.keeperDamaged)
			if !slices.Equal(got, tc.want) {
				t.Fatalf("mapRunEventsToCues() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestPlaceholderAudioPackLoadsAllCues verifies every configured cue and the looping
// ambient track resolve to valid RIFF WAV payloads in the CC0 placeholder pack.
func TestPlaceholderAudioPackLoadsAllCues(t *testing.T) {
	music, err := placeholder.Sound("music_loop")
	if err != nil {
		t.Fatalf("placeholder music_loop failed: %v", err)
	}
	if !bytes.HasPrefix(music.Data, []byte("RIFF")) {
		t.Fatalf("music_loop does not start with RIFF WAV header")
	}

	for c := cueNone + 1; c < cueCount; c++ {
		spec := cueSpecs[c]
		if spec.asset == "" {
			t.Fatalf("cue %v has empty placeholder asset name", c)
		}
		src, err := placeholder.Sound(spec.asset)
		if err != nil {
			t.Fatalf("cue %v asset %q failed: %v", c, spec.asset, err)
		}
		if !bytes.HasPrefix(src.Data, []byte("RIFF")) {
			t.Fatalf("cue %v asset %q is not a WAV payload", c, spec.asset)
		}
	}
}

// TestAudioMixerSyncsWithSettingsAndDegradesGracefully checks that the shell's audio
// controller binds master/music/SFX volumes to settings, keeps UI pause-exempt, and
// handles gameplay cues safely even when no audio output device is present.
func TestAudioMixerSyncsWithSettingsAndDegradesGracefully(t *testing.T) {
	app, g := startEmberVigil(t)
	if g.audio == nil || g.audio.mixer == nil {
		t.Fatal("shell did not initialize audio controller and mixer")
	}
	if !g.audio.mixer.PauseExempt(audioGroupUI) {
		t.Fatal("UI audio bus should be pause-exempt")
	}
	if g.audio.mixer.PauseExempt(audioGroupSFX) || g.audio.mixer.PauseExempt(audioGroupMusic) {
		t.Fatal("gameplay SFX and music buses should pause when the game is paused")
	}

	want := Settings{
		MasterVolume: 0.6,
		MusicVolume:  0.4,
		SFXVolume:    0.9,
		ScreenShake:  true,
		Quality:      QualityAuto,
	}
	g.audio.applySettings(want)
	if got := g.audio.mixer.Master(); got != float32(want.MasterVolume) {
		t.Fatalf("mixer master = %v, want %v", got, want.MasterVolume)
	}
	if got := g.audio.mixer.Volume(audioGroupMusic); got != float32(want.MusicVolume) {
		t.Fatalf("mixer music = %v, want %v", got, want.MusicVolume)
	}
	if got := g.audio.mixer.Volume(audioGroupSFX); got != float32(want.SFXVolume) {
		t.Fatalf("mixer sfx = %v, want %v", got, want.SFXVolume)
	}
	if got := g.audio.mixer.Volume(audioGroupUI); got != float32(want.SFXVolume) {
		t.Fatalf("mixer ui = %v, want %v", got, want.SFXVolume)
	}

	// Start, pause and resume a run: mixer pause state follows the arena pause overlay
	// and never panics when audio hardware is absent.
	press(t, app, "role:button Start")
	if g.audio.mixer.Paused() {
		t.Fatal("mixer should not be paused during active arena play")
	}
	g.audio.playRunEvents([]Event{{Kind: EventShot}, {Kind: EventKill}, {Kind: EventEmber}}, true)

	press(t, app, "role:button Pause")
	if !g.audio.mixer.Paused() {
		t.Fatal("mixer should pause gameplay audio when paused")
	}
	press(t, app, "role:button Resume")
	if g.audio.mixer.Paused() {
		t.Fatal("mixer should resume gameplay audio when resumed")
	}
}
