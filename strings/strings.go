// Package strings holds Ember Vigil's English and Simplified Chinese interface tables.
package strings

import "strconv"

// Lang identifies a supported interface language.
type Lang string

const (
	EN   Lang = "en"
	ZhCN Lang = "zh-CN"
)

// Languages lists supported languages in display order.
var Languages = []Lang{EN, ZhCN}

// NormalizeLang coerces a language tag to a supported Lang, defaulting to English.
func NormalizeLang(tag string) Lang {
	switch tag {
	case string(ZhCN), "zh", "zh_CN", "zh-Hans":
		return ZhCN
	default:
		return EN
	}
}

// CardCopy holds the localized display name and description of a weapon, passive or heal offer.
type CardCopy struct {
	Name        string
	Description string
}

// Table holds every player-facing string for one language.
type Table struct {
	// Title & app
	GameTitle     string
	TitleArtLabel string
	ArenaLabel    string
	Start         string
	Settings      string
	Quit          string
	BestRecords   string
	NoRecordsYet  string
	BestTimeFmt   string
	BestKillsFmt  string
	BestLevelFmt  string
	WinsFmt       string

	// HUD
	LevelPrefix  string
	KillsPrefix  string
	HealthPrefix string
	Pause        string
	BossName     string
	EliteName    string
	BossPhase1   string
	BossPhase2   string
	BossPhase3   string

	// Level-up
	KindWeapon  string
	KindPassive string
	KindHeal    string
	Cards       map[string]CardCopy

	// Pause & quit confirmation
	Paused           string
	Resume           string
	QuitToTitle      string
	ConfirmQuitTitle string
	ConfirmQuitBody  string
	ConfirmQuitYes   string
	Cancel           string

	// Settings
	SettingsTitle  string
	MasterVolume   string
	MusicVolume    string
	SFXVolume      string
	ScreenShake    string
	On             string
	Off            string
	Quality        string
	QualityAuto    string
	QualityLow     string
	QualityMedium  string
	QualityHigh    string
	Language       string
	LanguageEN     string
	LanguageZhCN   string
	Stats          string
	DecreasePrefix string
	IncreasePrefix string
	Apply          string
	Back           string

	// Results
	VictoryBadge string
	VictoryTitle string
	DefeatBadge  string
	DefeatTitle  string
	TimePrefix   string
	NewRecord    string
	NoNewRecord  string
	PlayAgain    string
	TitleButton  string
}

// For returns the localized string table for lang, falling back to English.
func For(lang Lang) Table {
	if NormalizeLang(string(lang)) == ZhCN {
		return zhCNTable
	}
	return enTable
}

// Level formats a level readout such as "Level 3" or "等级 3".
func (t Table) Level(n int) string { return t.LevelPrefix + strconv.Itoa(n) }

// Kills formats a kill count readout such as "Kills 12" or "击杀 12".
func (t Table) Kills(n int) string { return t.KillsPrefix + strconv.Itoa(n) }

// Health writes the keeper's health out of its maximum.
func (t Table) Health(n, maxHealth int) string {
	return t.HealthPrefix + strconv.Itoa(n) + "/" + strconv.Itoa(maxHealth)
}

// Time formats a duration readout such as "Time 04:12" or "时间 04:12".
func (t Table) Time(clock string) string { return t.TimePrefix + clock }

// BestTime formats the best survival time record line.
func (t Table) BestTime(clock string) string { return t.BestTimeFmt + clock }

// BestKills formats the highest kills record line.
func (t Table) BestKills(n int) string { return t.BestKillsFmt + strconv.Itoa(n) }

// BestLevel formats the highest level record line.
func (t Table) BestLevel(n int) string { return t.BestLevelFmt + strconv.Itoa(n) }

// Wins formats the total wins record line.
func (t Table) Wins(n int) string { return t.WinsFmt + strconv.Itoa(n) }

// BossPhaseLabel returns the localized phase tag for boss phases 1-3.
func (t Table) BossPhaseLabel(phase int) string {
	switch phase {
	case 2:
		return t.BossPhase2
	case 3:
		return t.BossPhase3
	default:
		return t.BossPhase1
	}
}

// BossDisplayName returns the localized boss name, falling back to fallback if unknown.
func (t Table) BossDisplayName(fallback string) string {
	if t.BossName != "" {
		return t.BossName
	}
	return fallback
}

// EliteDisplayName returns the localized Gloomwarden name, falling back to fallback if unknown.
func (t Table) EliteDisplayName(fallback string) string {
	if t.EliteName != "" {
		return t.EliteName
	}
	return fallback
}

// Card returns the localized name and description for an upgrade or weapon id, falling back to the raw strings.
func (t Table) Card(id, fallbackName, fallbackDesc string) CardCopy {
	if c, ok := t.Cards[id]; ok {
		return c
	}
	return CardCopy{Name: fallbackName, Description: fallbackDesc}
}

// DecreaseLabel builds an accessible label for decreasing a setting value.
func (t Table) DecreaseLabel(setting string) string { return t.DecreasePrefix + setting }

// IncreaseLabel builds an accessible label for increasing a setting value.
func (t Table) IncreaseLabel(setting string) string { return t.IncreasePrefix + setting }
