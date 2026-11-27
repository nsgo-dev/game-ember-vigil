package main

import (
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

// TestStatsSettingDefaultsOnAndPersists checks that statistics start on, stay on for saves made before the setting existed, and that turning them off in the settings persists once applied.
func TestStatsSettingDefaultsOnAndPersists(t *testing.T) {
	if s, _, ok := decodeSettings([]byte(`{"version":1,"masterVolume":0.5}`)); !ok || !s.Stats {
		t.Fatalf("a save without the setting decodes as %+v, want statistics on", s)
	}
	app, g := startEmberVigil(t)
	if g.stats != nil {
		t.Fatal("a test binary reports its use")
	}
	press(t, app, "id:title-settings")
	press(t, app, "id:settings-stats")
	if g.savedSettings.Stats != true {
		t.Fatal("toggling statistics took effect before Apply")
	}
	press(t, app, "id:settings-apply")
	_, g2 := startEmberVigil(t, nsgotest.Storage(storageSnapshot(app.Storage())))
	if g2.settings.Get().Stats {
		t.Fatal("reloaded Stats = true, want false")
	}
}

// TestStatsSendsOnlyWhenOn checks that events carry the page they belong to and that none is queued while the settings turn statistics off.
func TestStatsSendsOnlyWhenOn(t *testing.T) {
	on := true
	nsgotest.Start(t, func(cx *core.Scope) core.Node {
		s := &usageStats{cx: cx, on: func() bool { return on }, host: "example.org", referrer: "https://ref", tag: "app", id: "abc", queue: make(chan statsPayload, 4)}
		s.page("/run", "Run")
		s.event("victory")
		on = false
		s.event("defeat")
		if len(s.queue) != 2 {
			t.Fatalf("queued %d events, want 2", len(s.queue))
		}
		first, second := <-s.queue, <-s.queue
		if first.URL != "/run" || first.Name != "" || first.Referrer != "https://ref" || first.Website != statsWebsite {
			t.Errorf("the pageview is %+v", first)
		}
		if second.URL != "/run" || second.Name != "victory" || second.Referrer != "" {
			t.Errorf("the event is %+v", second)
		}
		return core.View{}
	})
}

// TestInstallIDIsKept checks that an install's ID is made once and kept.
func TestInstallIDIsKept(t *testing.T) {
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node { return core.View{} })
	a, b := installID(app.Storage()), installID(app.Storage())
	if len(a) != 32 || a != b {
		t.Fatalf("install IDs %q and %q, want one 32-digit ID", a, b)
	}
}
