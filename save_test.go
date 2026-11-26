package main

import (
	"encoding/json"
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
	evstrings "me.oneo/embervigil/strings"
)

func storageSnapshot(s core.Storage) map[string]string {
	out := map[string]string{}
	for _, k := range s.Keys() {
		if v, ok := s.GetItem(k); ok {
			out[k] = v
		}
	}
	return out
}

func TestSettingsMigrationAndFallback(t *testing.T) {
	// Legacy version-0 payload migrates to v1 and clamps out-of-range values.
	legacy := `{"masterVolume":1.5,"musicVolume":-0.2,"sfxVolume":0.4,"screenShake":false,"quality":"ultra","language":"zh"}`
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return core.View{}
	}, nsgotest.Storage(map[string]string{settingsStorageKey: legacy}))

	got := loadSettings(app.Storage())
	if got.Version != settingsVersion {
		t.Fatalf("migrated settings version = %d, want %d", got.Version, settingsVersion)
	}
	if got.MasterVolume != 1 || got.MusicVolume != 0 || got.SFXVolume != 0.4 {
		t.Fatalf("migrated volumes = (%v, %v, %v), want (1, 0, 0.4)", got.MasterVolume, got.MusicVolume, got.SFXVolume)
	}
	if got.ScreenShake {
		t.Fatal("migrated ScreenShake = true, want false")
	}
	if got.Quality != QualityAuto {
		t.Fatalf("unknown quality normalized to %q, want %q", got.Quality, QualityAuto)
	}
	if got.Language != evstrings.ZhCN {
		t.Fatalf("migrated language = %q, want %q", got.Language, evstrings.ZhCN)
	}
	// Migrated payload is persisted back with version=1.
	raw, ok := app.Storage().GetItem(settingsStorageKey)
	if !ok {
		t.Fatal("migrated settings missing from storage")
	}
	var persisted Settings
	if err := json.Unmarshal([]byte(raw), &persisted); err != nil {
		t.Fatalf("unmarshal persisted settings: %v", err)
	}
	if persisted.Version != settingsVersion {
		t.Fatalf("persisted migrated version = %d, want %d", persisted.Version, settingsVersion)
	}

	// Corrupt and future-version payloads fall back to defaults.
	for _, bad := range []string{`{not json`, `{"version":99,"masterVolume":0.1}`, `{"version":-1}`} {
		if err := app.Storage().SetItem(settingsStorageKey, bad); err != nil {
			t.Fatalf("set bad settings: %v", err)
		}
		if def := loadSettings(app.Storage()); def != defaultSettings() {
			t.Fatalf("loadSettings(%s) = %+v, want default %+v", bad, def, defaultSettings())
		}
	}
}

func TestRecordsMigrationAndApplyRunRecords(t *testing.T) {
	legacy := `{"bestTime":125.5,"bestKills":42,"bestLevel":7,"wins":1}`
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return core.View{}
	}, nsgotest.Storage(map[string]string{recordsStorageKey: legacy}))

	got := loadRecords(app.Storage())
	if got.Version != recordsVersion || !got.HasRecord {
		t.Fatalf("migrated records = %+v, want version %d and HasRecord=true", got, recordsVersion)
	}
	if got.BestTime != 125.5 || got.BestKills != 42 || got.BestLevel != 7 || got.Wins != 1 {
		t.Fatalf("migrated records stats = %+v", got)
	}

	// Corrupt and future-version payloads fall back to empty records.
	for _, bad := range []string{`{bad`, `{"version":99,"bestKills":100}`} {
		if err := app.Storage().SetItem(recordsStorageKey, bad); err != nil {
			t.Fatalf("set bad records: %v", err)
		}
		if empty := loadRecords(app.Storage()); empty != defaultRecords() {
			t.Fatalf("loadRecords(%s) = %+v, want default %+v", bad, empty, defaultRecords())
		}
	}

	// First run always sets a record.
	rec, isNew := applyRunRecords(defaultRecords(), Snapshot{Phase: PhaseDefeat, Time: 90, Kills: 15, Level: 4})
	if !isNew || !rec.HasRecord || rec.BestTime != 90 || rec.BestKills != 15 || rec.BestLevel != 4 || rec.Wins != 0 {
		t.Fatalf("first run apply = %+v, isNew=%v", rec, isNew)
	}

	// Worse run does not set a new record.
	rec2, isNew2 := applyRunRecords(rec, Snapshot{Phase: PhaseDefeat, Time: 30, Kills: 5, Level: 2})
	if isNew2 {
		t.Fatalf("worse run reported isNew=true: %+v", rec2)
	}
	if rec2 != rec {
		t.Fatalf("worse run mutated records: got %+v, want %+v", rec2, rec)
	}

	// Victory increments wins and marks new record on first win.
	rec3, isNew3 := applyRunRecords(rec2, Snapshot{Phase: PhaseVictory, Time: 725, Kills: 300, Level: 18})
	if !isNew3 || rec3.Wins != 1 || rec3.BestTime != 725 || rec3.BestKills != 300 || rec3.BestLevel != 18 {
		t.Fatalf("victory apply = %+v, isNew=%v", rec3, isNew3)
	}
}
