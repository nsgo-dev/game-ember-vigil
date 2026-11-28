package main

import (
	"encoding/binary"
	"errors"
	"math"
	"runtime"
	"sync/atomic"

	"github.com/nsgo-dev/nsgo/audio"
	"github.com/nsgo-dev/nsgo/core"
)

// The software mixer plays the short effects on Apple platforms through one PCM output instead of a native playback each.
// NSGo's Apple sounds start, stop and retire an AVAudioPlayerNode on the main thread for every playback, and a crowd's
// hits, kills, shots and embers, about thirty a second, held a 120 Hz iPhone near 78 frames a second while a muted run
// held 120. Here a cue is a command in a ring the audio thread drains, so playing one costs the frame nothing.
const (
	// sfxRate is the output's sample rate; the clips are resampled to it as they play.
	sfxRate = 44100
	// sfxVoices is how many effects sound at once; a new one past it takes the oldest's voice.
	sfxVoices = 24
	// sfxQueue is the ring of commands from the engine to the audio thread, a power of two.
	sfxQueue = 64
)

// usePCMEffects reports whether the effects go through the software mixer: on Apple platforms, whose native sounds cost the
// main thread per playback. Elsewhere they stay native, where a playback costs no frame and the web needs its gesture rules.
func usePCMEffects() bool { return runtime.GOOS == "darwin" || runtime.GOOS == "ios" }

// pcmClip is a sound decoded to mono samples in [-1, 1] at its own sample rate.
type pcmClip struct {
	samples []float32
	rate    float64
}

// decodeWAV decodes a 16-bit PCM WAV file, mixing its channels down to mono.
func decodeWAV(data []byte) (*pcmClip, error) {
	if len(data) < 12 || string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return nil, errors.New("not a WAVE file")
	}
	var channels, bits int
	var rate float64
	for p := 12; p+8 <= len(data); {
		id, size := string(data[p:p+4]), int(binary.LittleEndian.Uint32(data[p+4:p+8]))
		body := data[p+8 : min(len(data), p+8+size)]
		switch id {
		case "fmt ":
			if len(body) < 16 || binary.LittleEndian.Uint16(body[0:2]) != 1 {
				return nil, errors.New("not PCM")
			}
			channels = int(binary.LittleEndian.Uint16(body[2:4]))
			rate = float64(binary.LittleEndian.Uint32(body[4:8]))
			bits = int(binary.LittleEndian.Uint16(body[14:16]))
		case "data":
			if channels < 1 || bits != 16 || rate <= 0 {
				return nil, errors.New("not 16-bit PCM")
			}
			frames := len(body) / (2 * channels)
			c := &pcmClip{samples: make([]float32, frames), rate: rate}
			for i := range frames {
				var sum float32
				for ch := range channels {
					o := 2 * (i*channels + ch)
					sum += float32(int16(binary.LittleEndian.Uint16(body[o:o+2]))) / 32768
				}
				c.samples[i] = sum / float32(channels)
			}
			return c, nil
		}
		p += 8 + size + size&1
	}
	return nil, errors.New("no data chunk")
}

// sfxCommand asks the audio thread to play a clip.
type sfxCommand struct {
	clip     *pcmClip
	cue      soundCue
	limit    int
	volume   float32
	step     float64
	pausable bool
}

// sfxVoice is one clip sounding; only the audio thread touches it.
type sfxVoice struct {
	clip     *pcmClip
	cue      soundCue
	pos      float64
	step     float64
	volume   float32
	pausable bool
	started  uint64
}

// sfxMixer mixes the effects into one PCM output.
type sfxMixer struct {
	out     *audio.Output
	running bool

	ring       [sfxQueue]sfxCommand
	head, tail atomic.Uint32
	// paused holds the gameplay effects silent where they are, and muted silences every effect, as the background does.
	paused, muted atomic.Bool

	voices [sfxVoices]sfxVoice
	plays  uint64
}

// newSFXMixer opens the PCM output, paused until the first effect, or returns nil where the target has none.
func newSFXMixer(cx *core.Scope) *sfxMixer {
	m := &sfxMixer{}
	out, err := audio.NewOutput(cx, audio.OutputConfig{
		SampleRate:  sfxRate,
		Channels:    1,
		Render:      m.render,
		OnInterrupt: func(error) { m.running = false },
	})
	if err != nil {
		return nil
	}
	m.out = out
	return m
}

// play queues a playback of clip at volume and rate, at most limit of cue at once; a full ring drops it.
func (m *sfxMixer) play(clip *pcmClip, cue soundCue, limit int, volume, rate float32, pausable bool) {
	if clip == nil || len(clip.samples) == 0 || volume <= 0 {
		return
	}
	t := m.tail.Load()
	if t-m.head.Load() >= sfxQueue {
		return
	}
	m.ring[t%sfxQueue] = sfxCommand{clip: clip, cue: cue, limit: limit, volume: volume, step: clip.rate / sfxRate * float64(rate), pausable: pausable}
	m.tail.Store(t + 1)
	if !m.running {
		m.running = m.out.Start() == nil
	}
}

// setPaused holds the gameplay effects, or lets them play on.
func (m *sfxMixer) setPaused(paused bool) { m.paused.Store(paused) }

// setMuted silences every effect, or lets them sound again.
func (m *sfxMixer) setMuted(muted bool) { m.muted.Store(muted) }

// render runs on the audio thread: it takes the queued effects into voices and mixes them into buf.
func (m *sfxMixer) render(buf []float32) int {
	for h := m.head.Load(); h != m.tail.Load(); h++ {
		m.start(&m.ring[h%sfxQueue])
		m.head.Store(h + 1)
	}
	clear(buf)
	paused, muted := m.paused.Load(), m.muted.Load()
	for i := range m.voices {
		v := &m.voices[i]
		if v.clip == nil || paused && v.pausable {
			continue
		}
		s := v.clip.samples
		last := len(s) - 1
		for j := range buf {
			k := int(v.pos)
			if k >= last {
				v.clip = nil
				break
			}
			if !muted {
				f := float32(v.pos - float64(k))
				buf[j] += (s[k] + (s[k+1]-s[k])*f) * v.volume
			}
			v.pos += v.step
		}
	}
	for j, x := range buf {
		buf[j] = min(max(x, -1), 1)
	}
	return len(buf)
}

// start puts c in a voice: the oldest of its cue's once it has limit sounding, else a free one, else the oldest of all.
func (m *sfxMixer) start(c *sfxCommand) {
	var free, oldest, oldestCue *sfxVoice
	same := 0
	for i := range m.voices {
		v := &m.voices[i]
		if v.clip == nil {
			if free == nil {
				free = v
			}
			continue
		}
		if oldest == nil || v.started < oldest.started {
			oldest = v
		}
		if v.cue == c.cue {
			same++
			if oldestCue == nil || v.started < oldestCue.started {
				oldestCue = v
			}
		}
	}
	v := free
	switch {
	case c.limit > 0 && same >= c.limit:
		v = oldestCue
	case v == nil:
		v = oldest
	}
	m.plays++
	*v = sfxVoice{clip: c.clip, cue: c.cue, step: c.step, volume: c.volume, pausable: c.pausable, started: m.plays}
	if math.IsNaN(v.step) || v.step <= 0 {
		v.clip = nil
	}
}
