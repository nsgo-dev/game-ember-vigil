package main

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"testing"
)

// readData reads a tuning file from the repository, skipping where the test does not run in it, as on a phone or under nsgo test -target.
func readData(t *testing.T, name string) []byte {
	t.Helper()
	data, err := fs.ReadFile(os.DirFS("."), name)
	if errors.Is(err, fs.ErrNotExist) {
		t.Skipf("the repository is not readable here: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// TestDataFilesLoadAndAreCanonical checks that each tuning file shares its bytes with the embedded copy and that parsing and encoding it changes nothing, as a designer's editor and the loader must agree.
func TestDataFilesLoadAndAreCanonical(t *testing.T) {
	for _, name := range []string{fileRun, fileWeapons, fileUpgrades, fileEnemies, fileWaves, fileProgression} {
		data := readData(t, name)
		embedded, err := fs.ReadFile(dataFS, name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !bytes.Equal(embedded, data) {
			t.Fatalf("%s on disk and its embedded copy differ", name)
		}
		var decoded any
		switch name {
		case fileRun:
			decoded, err = parseRunFile(name, data)
		case fileWeapons:
			decoded, err = parseWeaponFile(name, data)
		case fileUpgrades:
			decoded, err = parseUpgradeFile(name, data)
		case fileEnemies:
			decoded, err = parseEnemyFile(name, data)
		case fileWaves:
			decoded, err = parseWaveFile(name, data)
		case fileProgression:
			decoded, err = parseProgressionFile(name, data)
		}
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := encodeYAML(decoded)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !bytes.Equal(encoded, data) {
			t.Fatalf("%s is not canonical, want:\n%s", name, encoded)
		}
	}
}

// TestEmbeddedDataLoads checks that the files the app embeds load and that the cross-references are resolved.
func TestEmbeddedDataLoads(t *testing.T) {
	d, err := loadGameData(dataFS)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.weapons.Weapons) != 4 {
		t.Fatalf("embedded weapons = %d, want 4", len(d.weapons.Weapons))
	}
	for _, w := range d.weapons.Weapons {
		if len(w.Levels) != 5 {
			t.Fatalf("weapon %q has %d levels, want 5", w.Name, len(w.Levels))
		}
	}
	if len(d.upgrades.Passives) != 8 {
		t.Fatalf("embedded passives = %d, want 8", len(d.upgrades.Passives))
	}
	for _, p := range d.upgrades.Passives {
		if p.MaxLevel != 5 {
			t.Fatalf("passive %q has maxLevel %d, want 5", p.Name, p.MaxLevel)
		}
	}
	if d.upgrades.WeaponSlots != 4 || d.upgrades.PassiveSlots != 4 {
		t.Fatalf("slots = %d weapons / %d passives, want 4 / 4", d.upgrades.WeaponSlots, d.upgrades.PassiveSlots)
	}
	if d.weapons.Weapons[d.starterWeapon].Name != d.weapons.Starter {
		t.Fatalf("starter weapon index %d points to %q, want %q", d.starterWeapon, d.weapons.Weapons[d.starterWeapon].Name, d.weapons.Starter)
	}
	if len(d.enemies.Enemies) != 7 {
		t.Fatalf("embedded enemies = %d, want 7 (6 regular + Gloomwarden elite)", len(d.enemies.Enemies))
	}
	if len(d.waves.Spans) == 0 || len(d.waves.Surges) == 0 || len(d.waves.Elites) == 0 {
		t.Fatalf("the embedded waves are incomplete: %+v", d.waves)
	}
	if d.waves.Boss.At != 720 || len(d.waves.Boss.Phases) != 3 {
		t.Fatalf("boss = at %v with %d phases, want at 720 with 3 phases", d.waves.Boss.At, len(d.waves.Boss.Phases))
	}
	for i := range d.waves.Spans {
		span := &d.waves.Spans[i]
		if len(span.weights) != len(span.Enemies) {
			t.Fatalf("span %d has %d weights for %d enemies", i, len(span.weights), len(span.Enemies))
		}
		for _, mix := range span.Enemies {
			if _, ok := d.enemyIndex(mix.Name); !ok {
				t.Fatalf("span %d names the unknown enemy %q", i, mix.Name)
			}
		}
	}
}

// TestDataLoadersRefuseBrokenFiles checks the version, unknown-field, trailing-document, value and cross-reference checks every file passes.
func TestDataLoadersRefuseBrokenFiles(t *testing.T) {
	runYAML := readData(t, fileRun)
	weaponYAML := readData(t, fileWeapons)
	upgradeYAML := readData(t, fileUpgrades)
	enemyYAML := readData(t, fileEnemies)
	waveYAML := readData(t, fileWaves)
	progressionYAML := readData(t, fileProgression)
	replace := func(data []byte, old, new string) []byte {
		if !bytes.Contains(data, []byte(old)) {
			t.Fatalf("%q is not in the file to replace", old)
		}
		return bytes.Replace(data, []byte(old), []byte(new), 1)
	}
	withUnlock := func(entry string) []byte {
		return replace(progressionYAML, "unlocks: []", "unlocks:\n"+entry)
	}

	tests := []struct {
		name string
		load func() error
	}{
		{"run with another version", func() error {
			_, err := parseRunFile(fileRun, replace(runYAML, "version: 1", "version: 2"))
			return err
		}},
		{"run with an unknown field", func() error {
			_, err := parseRunFile(fileRun, append(append([]byte(nil), runYAML...), []byte("\nunknown: true\n")...))
			return err
		}},
		{"run with a trailing document", func() error {
			_, err := parseRunFile(fileRun, append(append([]byte(nil), runYAML...), []byte("\n---\nversion: 1\n")...))
			return err
		}},
		{"empty run file", func() error {
			_, err := parseRunFile(fileRun, nil)
			return err
		}},
		{"weapons with another version", func() error {
			_, err := parseWeaponFile(fileWeapons, replace(weaponYAML, "version: 1", "version: 2"))
			return err
		}},
		{"weapons with an unknown field", func() error {
			_, err := parseWeaponFile(fileWeapons, append(append([]byte(nil), weaponYAML...), []byte("\nunknown: true\n")...))
			return err
		}},
		{"weapons with unknown kind", func() error {
			_, err := parseWeaponFile(fileWeapons, replace(weaponYAML, "kind: bolt", "kind: beam"))
			return err
		}},
		{"weapons with missing starter", func() error {
			_, err := parseWeaponFile(fileWeapons, replace(weaponYAML, "starter: Ember Bolt", "starter: Missing"))
			return err
		}},
		{"weapons with cooldown floor above cooldown", func() error {
			_, err := parseWeaponFile(fileWeapons, replace(weaponYAML, "cooldownMin: 0.15", "cooldownMin: 1"))
			return err
		}},
		{"weapons with duplicate name", func() error {
			_, err := parseWeaponFile(fileWeapons, replace(weaponYAML, "name: Lantern Halo", "name: Ember Bolt"))
			return err
		}},
		{"upgrades with another version", func() error {
			_, err := parseUpgradeFile(fileUpgrades, replace(upgradeYAML, "version: 1", "version: 2"))
			return err
		}},
		{"upgrades with an unknown field", func() error {
			_, err := parseUpgradeFile(fileUpgrades, append(append([]byte(nil), upgradeYAML...), []byte("\nunknown: true\n")...))
			return err
		}},
		{"upgrades with an unknown stat", func() error {
			_, err := parseUpgradeFile(fileUpgrades, replace(upgradeYAML, "stat: damage", "stat: luck"))
			return err
		}},
		{"upgrades with a duplicate passive", func() error {
			_, err := parseUpgradeFile(fileUpgrades, replace(upgradeYAML, "name: Oil", "name: Wick"))
			return err
		}},
		{"upgrades with invalid healing fraction", func() error {
			_, err := parseUpgradeFile(fileUpgrades, replace(upgradeYAML, "fraction: 0.35", "fraction: 1.5"))
			return err
		}},
		{"enemies with another version", func() error {
			_, err := parseEnemyFile(fileEnemies, replace(enemyYAML, "version: 1", "version: 3"))
			return err
		}},
		{"enemies with a duplicate name", func() error {
			_, err := parseEnemyFile(fileEnemies, replace(enemyYAML, "name: Moth", "name: Ash Mite"))
			return err
		}},
		{"waves with another version", func() error {
			_, err := parseWaveFile(fileWaves, replace(waveYAML, "version: 1", "version: 2"))
			return err
		}},
		{"waves out of order", func() error {
			_, err := parseWaveFile(fileWaves, replace(waveYAML, "from: 120", "from: 20"))
			return err
		}},
		{"waves with invalid boss phase threshold", func() error {
			_, err := parseWaveFile(fileWaves, replace(waveYAML, "threshold: 0.66", "threshold: 1.2"))
			return err
		}},
		{"waves naming a missing surge enemy", func() error {
			waves, err := parseWaveFile(fileWaves, replace(waveYAML, "enemy: Moth", "enemy: Ghost"))
			if err != nil {
				return err
			}
			enemies, err := parseEnemyFile(fileEnemies, enemyYAML)
			if err != nil {
				return err
			}
			return (&gameData{enemies: *enemies, waves: *waves}).check()
		}},
		{"progression with another version", func() error {
			_, err := parseProgressionFile(fileProgression, replace(progressionYAML, "version: 1", "version: 2"))
			return err
		}},
		{"progression with an unknown field", func() error {
			_, err := parseProgressionFile(fileProgression, append(append([]byte(nil), progressionYAML...), []byte("\nunknown: true\n")...))
			return err
		}},
		{"progression paying a negative amount", func() error {
			_, err := parseProgressionFile(fileProgression, replace(progressionYAML, "perChest: 15", "perChest: -15"))
			return err
		}},
		{"progression with a negative tier bonus", func() error {
			_, err := parseProgressionFile(fileProgression, replace(progressionYAML, "tierBonus: 0.25", "tierBonus: -0.25"))
			return err
		}},
		{"an unlock without a condition", func() error {
			_, err := parseProgressionFile(fileProgression, withUnlock("  - id: a\n    name: A\n"))
			return err
		}},
		{"an unlock without a name", func() error {
			_, err := parseProgressionFile(fileProgression, withUnlock("  - id: a\n    name: \"\"\n    win: true\n"))
			return err
		}},
		{"an unlock defined twice", func() error {
			_, err := parseProgressionFile(fileProgression, withUnlock("  - id: a\n    name: A\n    win: true\n  - id: a\n    name: B\n    kills: 1\n"))
			return err
		}},
		{"enemies with unknown motion", func() error {
			_, err := parseEnemyFile(fileEnemies, replace(enemyYAML, "motion: ranged", "motion: teleport"))
			return err
		}},
		{"waves naming a missing enemy", func() error {
			waves, err := parseWaveFile(fileWaves, replace(waveYAML, "name: Moth", "name: Mote"))
			if err != nil {
				return err
			}
			enemies, err := parseEnemyFile(fileEnemies, enemyYAML)
			if err != nil {
				return err
			}
			d := &gameData{enemies: *enemies, waves: *waves}
			if err := d.check(); err != nil {
				return err
			}
			return errors.New("the unknown enemy was accepted")
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.load(); err == nil {
				t.Fatal("the loader accepted a file it cannot use")
			}
		})
	}
}
