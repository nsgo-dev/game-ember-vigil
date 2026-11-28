package main

import (
	"time"

	"github.com/nsgo-dev/game/packages/game/placeholder"
	"github.com/nsgo-dev/game/packages/game/sound"
	"github.com/nsgo-dev/nsgo/audio"
	"github.com/nsgo-dev/nsgo/core"
)

// Mixer bus names and storage prefix.
const (
	audioMixerKey   = "embervigil.audio"
	audioGroupMusic = "music"
	audioGroupSFX   = "sfx"
	audioGroupUI    = "ui"
)

// soundCue identifies a logical audio cue triggered by run events or UI actions.
type soundCue uint8

const (
	cueNone soundCue = iota
	cueShoot
	cueHit
	cueKill
	cueEmber
	cueLevelUp
	cueBossSpawn
	cueBossPhase
	cueVictory
	cueDefeat
	cueKeeperHurt
	cueUIClick
	cueUISelect
	cueChest
	cueEvolve
	cueCount
)

// String returns a human-readable name for diagnostic messages and tests.
func (c soundCue) String() string {
	switch c {
	case cueShoot:
		return "shoot"
	case cueHit:
		return "hit"
	case cueKill:
		return "kill"
	case cueEmber:
		return "ember"
	case cueLevelUp:
		return "level_up"
	case cueBossSpawn:
		return "boss_spawn"
	case cueBossPhase:
		return "boss_phase"
	case cueVictory:
		return "victory"
	case cueDefeat:
		return "defeat"
	case cueKeeperHurt:
		return "keeper_hurt"
	case cueUIClick:
		return "ui_click"
	case cueUISelect:
		return "ui_select"
	case cueChest:
		return "chest"
	case cueEvolve:
		return "evolve"
	default:
		return "none"
	}
}

// cueSpec configures how a cue maps to the CC0 placeholder sound pack and its bus.
type cueSpec struct {
	asset    string
	group    string
	limit    int
	volume   float32
	rate     float32
	cooldown time.Duration
}

// cueSpecs defines every sound cue using the nsgo-dev/game CC0 placeholder pack
// ("Placeholder art and sound: NSGo Game contributors, CC0").
var cueSpecs = [cueCount]cueSpec{
	cueShoot:      {asset: "shoot", group: audioGroupSFX, limit: 4, volume: 0.42, rate: 1.05, cooldown: 55 * time.Millisecond},
	cueHit:        {asset: "hit", group: audioGroupSFX, limit: 4, volume: 0.46, rate: 1.15, cooldown: 45 * time.Millisecond},
	cueKill:       {asset: "explosion", group: audioGroupSFX, limit: 3, volume: 0.42, rate: 1.35, cooldown: 60 * time.Millisecond},
	cueEmber:      {asset: "coin", group: audioGroupSFX, limit: 4, volume: 0.44, rate: 1.10, cooldown: 50 * time.Millisecond},
	cueLevelUp:    {asset: "powerup", group: audioGroupUI, limit: 2, volume: 0.78, rate: 1.00, cooldown: 150 * time.Millisecond},
	cueBossSpawn:  {asset: "explosion", group: audioGroupSFX, limit: 2, volume: 0.88, rate: 0.62, cooldown: 250 * time.Millisecond},
	cueBossPhase:  {asset: "explosion", group: audioGroupSFX, limit: 2, volume: 0.78, rate: 0.78, cooldown: 200 * time.Millisecond},
	cueVictory:    {asset: "powerup", group: audioGroupUI, limit: 1, volume: 0.85, rate: 1.15, cooldown: 250 * time.Millisecond},
	cueDefeat:     {asset: "hit", group: audioGroupUI, limit: 1, volume: 0.85, rate: 0.55, cooldown: 250 * time.Millisecond},
	cueKeeperHurt: {asset: "hit", group: audioGroupSFX, limit: 2, volume: 0.82, rate: 0.72, cooldown: 120 * time.Millisecond},
	cueUIClick:    {asset: "click", group: audioGroupUI, limit: 3, volume: 0.55, rate: 1.00, cooldown: 35 * time.Millisecond},
	cueUISelect:   {asset: "select", group: audioGroupUI, limit: 3, volume: 0.68, rate: 1.00, cooldown: 45 * time.Millisecond},
	cueChest:      {asset: "coin", group: audioGroupUI, limit: 1, volume: 0.9, rate: 0.7, cooldown: 250 * time.Millisecond},
	cueEvolve:     {asset: "powerup", group: audioGroupUI, limit: 1, volume: 0.95, rate: 0.8, cooldown: 250 * time.Millisecond},
}

// mapRunEventsToCues translates one simulation step's events (plus whether the
// keeper took damage this step) into deduplicated sound cues.
func mapRunEventsToCues(events []Event, keeperDamaged bool) []soundCue {
	if len(events) == 0 && !keeperDamaged {
		return nil
	}
	var seen [cueCount]bool
	out := make([]soundCue, 0, 6)
	add := func(c soundCue) {
		if c == cueNone || c >= cueCount || seen[c] {
			return
		}
		seen[c] = true
		out = append(out, c)
	}
	if keeperDamaged {
		add(cueKeeperHurt)
	}
	for _, e := range events {
		switch e.Kind {
		case EventShot:
			add(cueShoot)
		case EventHit:
			// Only emit enemy-hit cue when the event is not solely keeper contact damage.
			if !keeperDamaged || len(events) > 1 {
				add(cueHit)
			}
		case EventKill:
			add(cueKill)
		case EventEmber, EventMagnet:
			add(cueEmber)
		case EventLevelUp:
			add(cueLevelUp)
		case EventBossSpawn:
			add(cueBossSpawn)
		case EventBossPhase:
			add(cueBossPhase)
		case EventVictory:
			add(cueVictory)
		case EventDefeat:
			add(cueDefeat)
		case EventChest:
			add(cueChest)
		case EventEvolve:
			add(cueEvolve)
		}
	}
	return out
}

// audioController owns the game's sound.Mixer, music player and preloaded cue effects.
// On platforms or headless tests without an audio device, it degrades gracefully to a no-op.
type audioController struct {
	cx           *core.Scope
	mixer        *sound.Mixer
	music        *sound.Music
	musicSource  audio.Source
	musicReady   bool
	musicStarted bool
	available    bool

	sounds   map[string]*audio.Sound
	effects  [cueCount]*sound.Effect
	lastPlay [cueCount]time.Time

	// sfx mixes the effects into one PCM output where native playbacks cost the frame (usePCMEffects), from clips decoded per cue; nil elsewhere.
	sfx    *sfxMixer
	clips  [cueCount]*pcmClip
	paused bool
}

// newAudioController builds the mixer, wires bus defaults and asynchronously preloads
// the CC0 placeholder cues when the platform can play audio.
func newAudioController(cx *core.Scope, initial Settings) *audioController {
	m := sound.NewMixer(cx, audioMixerKey)
	m.SetPauseExempt(audioGroupUI, true)
	mu := m.Music(audioGroupMusic)
	mu.Loop = true

	ac := &audioController{
		cx:        cx,
		mixer:     m,
		music:     mu,
		available: cx.Env().CanPlayAudio(),
		sounds:    map[string]*audio.Sound{},
	}
	ac.applySettings(initial)

	if src, err := placeholder.Sound("music_loop"); err == nil {
		ac.musicSource = src
		ac.musicReady = true
	}

	if ac.available && usePCMEffects() {
		ac.sfx = newSFXMixer(cx)
	}
	if ac.sfx != nil {
		ac.decodeCues()
		cx.Effect(func() { ac.sfx.setMuted(cx.Env().Lifecycle() == core.LifecycleBackground) })
	} else if ac.available {
		ac.preloadCues()
	}

	cx.Cleanup(func() {
		ac.music.Stop(0)
		for _, s := range ac.sounds {
			if s != nil {
				s.Close()
			}
		}
	})
	return ac
}

// preloadCues loads each unique WAV asset from the CC0 placeholder pack once and
// binds every cue referencing that asset to a pooled sound.Effect.
func (a *audioController) preloadCues() {
	byAsset := map[string][]soundCue{}
	for c := cueNone + 1; c < cueCount; c++ {
		spec := cueSpecs[c]
		if spec.asset == "" {
			continue
		}
		byAsset[spec.asset] = append(byAsset[spec.asset], c)
	}
	for asset, cues := range byAsset {
		src, err := placeholder.Sound(asset)
		if err != nil {
			continue
		}
		assetName := asset
		boundCues := cues
		audio.LoadSound(a.cx, src, func(s *audio.Sound, err error) {
			if err != nil || s == nil {
				return
			}
			a.sounds[assetName] = s
			for _, c := range boundCues {
				spec := cueSpecs[c]
				a.effects[c] = a.mixer.Effect(s, spec.group, spec.limit)
			}
		})
	}
}

// decodeCues decodes each cue's WAV from the CC0 placeholder pack once for the software mixer.
func (a *audioController) decodeCues() {
	byAsset := map[string]*pcmClip{}
	for c := cueNone + 1; c < cueCount; c++ {
		asset := cueSpecs[c].asset
		clip, ok := byAsset[asset]
		if !ok {
			if src, err := placeholder.Sound(asset); err == nil {
				clip, _ = decodeWAV(src.Data)
			}
			byAsset[asset] = clip
		}
		a.clips[c] = clip
	}
}

// applySettings synchronizes master, music and SFX (including UI) bus volumes immediately.
func (a *audioController) applySettings(s Settings) {
	if a == nil || a.mixer == nil {
		return
	}
	s = normalizeSettings(s)
	_ = a.mixer.SetMaster(float32(s.MasterVolume))
	_ = a.mixer.SetVolume(audioGroupMusic, float32(s.MusicVolume))
	_ = a.mixer.SetVolume(audioGroupSFX, float32(s.SFXVolume))
	_ = a.mixer.SetVolume(audioGroupUI, float32(s.SFXVolume))
}

// setPaused pauses or resumes gameplay SFX and music while keeping UI cues active.
func (a *audioController) setPaused(paused bool) {
	if a == nil || a.mixer == nil {
		return
	}
	a.mixer.SetPaused(paused)
	a.paused = paused
	if a.sfx != nil {
		a.sfx.setPaused(paused)
	}
}

// ensureMusic starts the looping ambient background track on the first user-initiated
// transition into gameplay, fading it in gently. Safe when audio is unavailable.
func (a *audioController) ensureMusic() {
	if a == nil || !a.available || !a.musicReady || a.musicStarted {
		return
	}
	a.musicStarted = true
	a.music.Play(a.musicSource, 800*time.Millisecond)
}

// stopMusic fades out the looping ambient track when returning to the title screen.
func (a *audioController) stopMusic() {
	if a == nil || !a.musicStarted {
		return
	}
	a.musicStarted = false
	a.music.Stop(400 * time.Millisecond)
}

// playCue plays one cue if loaded and not inside its rate-limit cooldown.
func (a *audioController) playCue(c soundCue) {
	if a == nil || !a.available || c <= cueNone || c >= cueCount {
		return
	}
	eff := a.effects[c]
	if eff == nil && a.clips[c] == nil {
		return
	}
	spec := cueSpecs[c]
	now := a.cx.Now()
	if spec.cooldown > 0 && !a.lastPlay[c].IsZero() && now.Sub(a.lastPlay[c]) < spec.cooldown {
		return
	}
	a.lastPlay[c] = now
	if a.sfx != nil {
		// Gameplay effects wait out a pause; the UI's keep playing under the pause menu, as the mixer's exempt group does.
		pausable := spec.group != audioGroupUI
		if pausable && a.paused {
			return
		}
		gain := core.Untracked(a.cx, func() float32 { return a.mixer.Master() * a.mixer.Volume(spec.group) })
		a.sfx.play(a.clips[c], c, spec.limit, spec.volume*gain, spec.rate, pausable)
		return
	}
	eff.Play(audio.Voice{
		Volume: spec.volume,
		Rate:   spec.rate,
	})
}

// playRunEvents dispatches all sound cues mapped from one simulation step.
func (a *audioController) playRunEvents(events []Event, keeperDamaged bool) {
	if a == nil || !a.available {
		return
	}
	for _, c := range mapRunEventsToCues(events, keeperDamaged) {
		a.playCue(c)
	}
}
