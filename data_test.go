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
	for _, name := range []string{fileRun, fileEnemies, fileWaves} {
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
		case fileEnemies:
			decoded, err = parseEnemyFile(name, data)
		case fileWaves:
			decoded, err = parseWaveFile(name, data)
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

// TestEmbeddedDataLoads checks that the files the app embeds load and that the cross-references the waves need are resolved.
func TestEmbeddedDataLoads(t *testing.T) {
	d, err := loadGameData(dataFS)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.run.Upgrades) == 0 || len(d.enemies.Enemies) == 0 || len(d.waves.Spans) == 0 {
		t.Fatalf("the embedded data is empty: %+v", d)
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
	enemyYAML := readData(t, fileEnemies)
	waveYAML := readData(t, fileWaves)
	replace := func(data []byte, old, new string) []byte {
		if !bytes.Contains(data, []byte(old)) {
			t.Fatalf("%q is not in the file to replace", old)
		}
		return bytes.Replace(data, []byte(old), []byte(new), 1)
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
		{"run with an unknown upgrade stat", func() error {
			_, err := parseRunFile(fileRun, replace(runYAML, "stat: damage", "stat: reach"))
			return err
		}},
		{"run with a duplicate upgrade", func() error {
			_, err := parseRunFile(fileRun, replace(runYAML, "name: Oil", "name: Wick"))
			return err
		}},
		{"run with a cooldown floor above the cooldown", func() error {
			_, err := parseRunFile(fileRun, replace(runYAML, "cooldownMin: 0.15", "cooldownMin: 1"))
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
			_, err := parseWaveFile(fileWaves, replace(waveYAML, "from: 75", "from: 20"))
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
