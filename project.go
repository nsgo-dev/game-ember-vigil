package main

import (
	_ "embed"

	"github.com/nsgo-dev/game/packages/game/project"
	"github.com/nsgo-dev/game/packages/game/screen"
)

// gameYAML is Ember Vigil's project file, which holds its settings: its name, id and version, its window, its virtual resolution and input map, and its export presets. nsgo.yaml repeats some of them; after editing game.yaml, run go run github.com/nsgo-dev/game/packages/game/cmd/gameproject sync to update it.
//
//go:embed game.yaml
var gameYAML []byte

// Project is Ember Vigil's parsed project file.
var Project = project.MustParse(gameYAML)

// viewport maps the window onto the virtual canvas game.yaml declares.
var viewport = mustViewport()

// virtualWidth and virtualHeight are the virtual canvas's size in virtual pixels.
var virtualWidth, virtualHeight = viewport.Width, viewport.Height

// mustViewport is the virtual canvas game.yaml declares, which the game cannot run without.
func mustViewport() screen.Viewport {
	vp, ok := Project.Viewport()
	if !ok {
		panic("embervigil: game.yaml declares no display")
	}
	return vp
}
