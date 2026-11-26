package main

import (
	"testing"
	"time"

	"github.com/nsgo-dev/game/packages/game/quality"
	"github.com/nsgo-dev/nsgo/core"
)

// TestQualityPresetsScaleBudgets checks that Low < Medium < High across particles,
// glow lights and shake scaling, and toggles decorative effects and shadows properly.
func TestQualityPresetsScaleBudgets(t *testing.T) {
	cfg := emberQualityConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("emberQualityConfig invalid: %v", err)
	}

	low := qualityProfileFor(quality.Low, cfg.Presets.For(quality.Low))
	med := qualityProfileFor(quality.Medium, cfg.Presets.For(quality.Medium))
	high := qualityProfileFor(quality.High, cfg.Presets.For(quality.High))

	if !(low.maxParticles < med.maxParticles && med.maxParticles < high.maxParticles) {
		t.Fatalf("maxParticles not strictly ordered: low=%d med=%d high=%d", low.maxParticles, med.maxParticles, high.maxParticles)
	}
	if !(low.maxGlowLights < med.maxGlowLights && med.maxGlowLights < high.maxGlowLights) {
		t.Fatalf("maxGlowLights not strictly ordered: low=%d med=%d high=%d", low.maxGlowLights, med.maxGlowLights, high.maxGlowLights)
	}
	if !(low.shakeScale < med.shakeScale && med.shakeScale < high.shakeScale) {
		t.Fatalf("shakeScale not strictly ordered: low=%v med=%v high=%v", low.shakeScale, med.shakeScale, high.shakeScale)
	}
	if low.emberTrails || low.hitSparks || low.shadows {
		t.Fatalf("low preset should disable trails, hit sparks and shadows: %+v", low)
	}
	if !med.emberTrails || !med.hitSparks || med.shadows {
		t.Fatalf("medium preset should enable trails and hit sparks without shadows: %+v", med)
	}
	if !high.emberTrails || !high.hitSparks || !high.shadows {
		t.Fatalf("high preset should enable trails, hit sparks and shadows: %+v", high)
	}
}

// TestQualityAutoSelectsByDeviceAndFrameTimes verifies device-class startup tiers,
// automatic step-down on slow frames, step-up on fast frames, and manual override.
func TestQualityAutoSelectsByDeviceAndFrameTimes(t *testing.T) {
	cfg := emberQualityConfig()

	// Capable desktop starts High; web starts Medium; Android starts Low.
	desktopSel, err := quality.NewSelector(cfg, quality.Device{OS: core.OSMacOS, CPUs: 10})
	if err != nil {
		t.Fatalf("NewSelector(desktop): %v", err)
	}
	if desktopSel.Level() != quality.High {
		t.Fatalf("desktop start level = %v, want high", desktopSel.Level())
	}

	webSel, err := quality.NewSelector(cfg, quality.Device{OS: core.OSWeb, CPUs: 4})
	if err != nil {
		t.Fatalf("NewSelector(web): %v", err)
	}
	if webSel.Level() != quality.Medium {
		t.Fatalf("web start level = %v, want medium", webSel.Level())
	}

	androidSel, err := quality.NewSelector(cfg, quality.Device{OS: core.OSAndroid, CPUs: 8})
	if err != nil {
		t.Fatalf("NewSelector(android): %v", err)
	}
	if androidSel.Level() != quality.Low {
		t.Fatalf("android start level = %v, want low", androidSel.Level())
	}

	// Slow frames (> 16.67ms target) step desktop down from High to Medium.
	steppedDown := false
	for range 200 {
		if desktopSel.Observe(28 * time.Millisecond) {
			steppedDown = true
			break
		}
	}
	if !steppedDown || desktopSel.Level() != quality.Medium {
		t.Fatalf("slow frames left desktop at %v (changed=%v), want medium", desktopSel.Level(), steppedDown)
	}

	// Manual override locks the tier and ignores slow frame times.
	if err := desktopSel.SetSetting(string(QualityHigh)); err != nil {
		t.Fatalf("SetSetting(high): %v", err)
	}
	if desktopSel.Auto() || desktopSel.Level() != quality.High {
		t.Fatalf("manual high override failed: auto=%v level=%v", desktopSel.Auto(), desktopSel.Level())
	}
	for range 200 {
		if desktopSel.Observe(40 * time.Millisecond) {
			t.Fatal("manual override changed level on slow frames")
		}
	}
	if desktopSel.Level() != quality.High {
		t.Fatalf("manual high level drifted to %v", desktopSel.Level())
	}

	// Fast frames (< 60% of 16.67ms) step a Low-starting device up to Medium in auto mode.
	steppedUp := false
	for range 600 {
		if androidSel.Observe(5 * time.Millisecond) {
			steppedUp = true
			break
		}
	}
	if !steppedUp || androidSel.Level() != quality.Medium {
		t.Fatalf("fast frames left android at %v (changed=%v), want medium", androidSel.Level(), steppedUp)
	}
}

// TestQualityShellOverrideAppliesToRenderer checks that changing quality in settings
// updates the arena renderer immediately and reverts cleanly when cancelling settings.
func TestQualityShellOverrideAppliesToRenderer(t *testing.T) {
	app, g := startEmberVigil(t)
	press(t, app, "role:button Settings")

	g.setQuality(QualityLow)
	if g.scene.quality.level != quality.Low {
		t.Fatalf("renderer quality after setQuality(low) = %v, want low", g.scene.quality.level)
	}

	g.setQuality(QualityHigh)
	if g.scene.quality.level != quality.High {
		t.Fatalf("renderer quality after setQuality(high) = %v, want high", g.scene.quality.level)
	}

	// Cancel settings reverts to the saved QualityAuto setting.
	g.closeSettings()
	if !g.quality.Auto() {
		t.Fatal("closeSettings did not restore QualityAuto on selector")
	}
}
