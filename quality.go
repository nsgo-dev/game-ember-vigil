package main

import (
	"github.com/nsgo-dev/game/packages/game/quality"
)

// qualityProfile translates a quality.Level and quality.Preset into concrete arena
// rendering budgets: particle cap, glow light count, effect toggles, shadows and shake scale.
type qualityProfile struct {
	level         quality.Level
	preset        quality.Preset
	maxParticles  int
	maxGlowLights int
	particleScale float64
	emberTrails   bool
	hitSparks     bool
	shadows       bool
	shakeScale    float64
}

// emberQualityConfig returns Ember Vigil's quality configuration targeting 60 fps.
func emberQualityConfig() quality.Config {
	cfg := quality.DefaultConfig()
	cfg.Adapt.TargetFPS = rate
	cfg.Adapt.WindowFrames = 90
	cfg.Adapt.UpgradeWindows = 4
	return cfg
}

// qualityProfileFor builds the renderer profile for a resolved quality level and preset.
func qualityProfileFor(level quality.Level, preset quality.Preset) qualityProfile {
	p := qualityProfile{
		level:         level,
		preset:        preset,
		maxParticles:  preset.Particles(maxParticles),
		particleScale: float64(preset.ParticleScale),
		emberTrails:   preset.Effects,
		hitSparks:     preset.Effects,
		shadows:       preset.Shadows,
	}
	switch level {
	case quality.Low:
		p.maxGlowLights = 6
		p.shakeScale = 0.4
	case quality.High:
		p.maxGlowLights = maxGlowLights
		p.shakeScale = 1.0
	default:
		p.maxGlowLights = 14
		p.shakeScale = 0.75
	}
	if p.maxParticles <= 0 {
		p.maxParticles = maxParticles
	}
	if p.particleScale <= 0 || p.particleScale > 1 {
		p.particleScale = 1
	}
	return p
}

// defaultQualityProfile returns the high-quality profile used by standalone renderer tests.
func defaultQualityProfile() qualityProfile {
	return qualityProfileFor(quality.High, quality.DefaultPreset(quality.High))
}
