package main

import (
	"bytes"
	_ "embed"
	"os"
	"testing"

	"github.com/nsgo-dev/game/packages/game/project"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

//go:embed nsgo.yaml
var nsgoYAML []byte

// TestProjectFileIsValidAndCanonical checks that game.yaml is in the form gameproject fmt writes and that nsgo.yaml agrees with it, as gameproject check and sync -check do. It reads both from the test binary, so it runs on every nsgo test target.
func TestProjectFileIsValidAndCanonical(t *testing.T) {
	if err := Project.CheckNSGo(nsgoYAML); err != nil {
		t.Fatal(err)
	}
	if canonical, err := project.Encode(Project); err != nil || !bytes.Equal(canonical, gameYAML) {
		t.Fatalf("game.yaml is not canonical (%v); run gameproject fmt:\n%s", err, canonical)
	}
}

// TestAutoloadsLoad checks that the scenes and resources game.yaml autoloads exist and that the scenes are valid, as gameproject check does. It reads them from the project directory, so it is skipped where the test does not run in it, as on a phone, a simulator or the web under nsgo test -target.
func TestAutoloadsLoad(t *testing.T) {
	if _, err := os.Stat(project.FileName); err != nil {
		t.Skipf("the project directory is not readable here: %v", err)
	}
	if _, err := Project.LoadAutoloads(os.DirFS(".")); err != nil {
		t.Fatal(err)
	}
}

// TestRootTitlesTheWindowFromTheProject checks that the window carries the game's name.
func TestRootTitlesTheWindowFromTheProject(t *testing.T) {
	app := nsgotest.Start(t, Root)
	if app.Title != Project.Title() {
		t.Fatalf("window title = %q, want %q", app.Title, Project.Title())
	}
}
