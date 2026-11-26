package main

import (
	"encoding/json"
	"math"

	"github.com/nsgo-dev/nsgo/core"
	evstrings "me.oneo/embervigil/strings"
)

// Storage keys and schema versions for the two persisted save slots.
const (
	settingsStorageKey = "settings"
	recordsStorageKey  = "records"

	settingsVersion = 1
	recordsVersion  = 1
)

// Quality is the graphics quality tier stored in settings and consumed by the renderer.
type Quality string

const (
	QualityAuto   Quality = "auto"
	QualityLow    Quality = "low"
	QualityMedium Quality = "medium"
	QualityHigh   Quality = "high"
)

// qualityTiers lists the quality options in display order.
var qualityTiers = []Quality{QualityAuto, QualityLow, QualityMedium, QualityHigh}

// Settings holds player preferences persisted across sessions.
type Settings struct {
	Version      int            `json:"version"`
	MasterVolume float64        `json:"masterVolume"`
	MusicVolume  float64        `json:"musicVolume"`
	SFXVolume    float64        `json:"sfxVolume"`
	ScreenShake  bool           `json:"screenShake"`
	Quality      Quality        `json:"quality"`
	Language     evstrings.Lang `json:"language"`
}

// Records holds lifetime best stats persisted across sessions.
type Records struct {
	Version   int     `json:"version"`
	HasRecord bool    `json:"hasRecord"`
	BestTime  float64 `json:"bestTime"`
	BestKills int     `json:"bestKills"`
	BestLevel int     `json:"bestLevel"`
	Wins      int     `json:"wins"`
}

// defaultSettings returns the initial preferences before any user edits are saved.
func defaultSettings() Settings {
	return Settings{
		Version:      settingsVersion,
		MasterVolume: 0.8,
		MusicVolume:  0.7,
		SFXVolume:    0.8,
		ScreenShake:  true,
		Quality:      QualityAuto,
		Language:     evstrings.EN,
	}
}

// defaultRecords returns an empty lifetime record set at the current schema version.
func defaultRecords() Records {
	return Records{Version: recordsVersion}
}

// normalizeSettings clamps volumes and coerces unknown quality or language values to valid defaults.
func normalizeSettings(s Settings) Settings {
	s.Version = settingsVersion
	s.MasterVolume = clampVolume(s.MasterVolume)
	s.MusicVolume = clampVolume(s.MusicVolume)
	s.SFXVolume = clampVolume(s.SFXVolume)
	switch s.Quality {
	case QualityAuto, QualityLow, QualityMedium, QualityHigh:
	default:
		s.Quality = QualityAuto
	}
	s.Language = evstrings.NormalizeLang(string(s.Language))
	return s
}

// clampVolume keeps a volume slider in [0, 1] and rounds to two decimal places to avoid float drift.
func clampVolume(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	v = math.Min(1, math.Max(0, v))
	return math.Round(v*100) / 100
}

// normalizeRecords ensures non-negative stats and a current schema version.
func normalizeRecords(r Records) Records {
	r.Version = recordsVersion
	if math.IsNaN(r.BestTime) || math.IsInf(r.BestTime, 0) || r.BestTime < 0 {
		r.BestTime = 0
	}
	if r.BestKills < 0 {
		r.BestKills = 0
	}
	if r.BestLevel < 0 {
		r.BestLevel = 0
	}
	if r.Wins < 0 {
		r.Wins = 0
	}
	if r.BestTime > 0 || r.BestKills > 0 || r.BestLevel > 0 || r.Wins > 0 {
		r.HasRecord = true
	}
	return r
}

// loadSettings reads, validates and migrates the settings slot from storage, falling back to defaults when missing or corrupt.
func loadSettings(storage core.Storage) Settings {
	raw, ok := storage.GetItem(settingsStorageKey)
	if !ok || raw == "" {
		return defaultSettings()
	}
	s, migrated, ok := decodeSettings([]byte(raw))
	if !ok {
		return defaultSettings()
	}
	if migrated {
		_ = saveSettings(storage, s)
	}
	return s
}

// decodeSettings parses a settings payload, migrating older schemas (version 0) and rejecting unsupported future versions.
func decodeSettings(data []byte) (Settings, bool, bool) {
	var envelope struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return Settings{}, false, false
	}
	if envelope.Version < 0 || envelope.Version > settingsVersion {
		return Settings{}, false, false
	}

	// Start from defaults so omitted fields in older payloads keep sensible values.
	s := defaultSettings()
	if err := json.Unmarshal(data, &s); err != nil {
		return Settings{}, false, false
	}
	migrated := envelope.Version < settingsVersion
	return normalizeSettings(s), migrated, true
}

// saveSettings writes normalized settings to storage under the versioned settings slot.
func saveSettings(storage core.Storage, s Settings) error {
	s = normalizeSettings(s)
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return storage.SetItem(settingsStorageKey, string(data))
}

// loadRecords reads, validates and migrates the records slot from storage, falling back to empty records when missing or corrupt.
func loadRecords(storage core.Storage) Records {
	raw, ok := storage.GetItem(recordsStorageKey)
	if !ok || raw == "" {
		return defaultRecords()
	}
	r, migrated, ok := decodeRecords([]byte(raw))
	if !ok {
		return defaultRecords()
	}
	if migrated {
		_ = saveRecords(storage, r)
	}
	return r
}

// decodeRecords parses a records payload, migrating older schemas (version 0) and rejecting unsupported future versions.
func decodeRecords(data []byte) (Records, bool, bool) {
	var envelope struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return Records{}, false, false
	}
	if envelope.Version < 0 || envelope.Version > recordsVersion {
		return Records{}, false, false
	}

	var r Records
	if err := json.Unmarshal(data, &r); err != nil {
		return Records{}, false, false
	}
	migrated := envelope.Version < recordsVersion
	return normalizeRecords(r), migrated, true
}

// saveRecords writes normalized records to storage under the versioned records slot.
func saveRecords(storage core.Storage, r Records) error {
	r = normalizeRecords(r)
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	return storage.SetItem(recordsStorageKey, string(data))
}

// applyRunRecords folds a finished run snapshot into prev and reports whether any personal best was set.
func applyRunRecords(prev Records, s Snapshot) (Records, bool) {
	next := normalizeRecords(prev)
	isNew := !next.HasRecord ||
		s.Time > next.BestTime ||
		s.Kills > next.BestKills ||
		s.Level > next.BestLevel ||
		(s.Phase == PhaseVictory && next.Wins == 0)

	if !next.HasRecord || s.Time > next.BestTime {
		next.BestTime = s.Time
	}
	if !next.HasRecord || s.Kills > next.BestKills {
		next.BestKills = s.Kills
	}
	if !next.HasRecord || s.Level > next.BestLevel {
		next.BestLevel = s.Level
	}
	if s.Phase == PhaseVictory {
		next.Wins++
	}
	next.HasRecord = true
	return next, isNew
}
