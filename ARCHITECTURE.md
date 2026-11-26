# Ember Vigil architecture

Ember Vigil is an NSGo app with one fixed-step game loop. A run is simulated headlessly at 60 Hz, drawn into one canvas, and shown with native NSGo views for the HUD and menus. YAML files under `data/` hold the balance; the shell owns screens, input and saves.

## Code map

| Path | Responsibility |
| --- | --- |
| `main.go`, `project.go` | NSGo entry point and the parsed `game.yaml` |
| `game.go` | Root component: loop, screens, input translation, pause, save and sound hooks |
| `run.go` | One run: keeper, enemies, shots, embers, contact, XP and the snapshot |
| `upgrades.go` | The upgrade pool, three-card offers and the derived stat block |
| `waves.go` | The wave director: timeline, spawn ring, caps and the view it stays outside |
| `data.go`, `data/` | Tuning YAML and its loaders: version checked, unknown fields refused, canonical round trip |
| `art.go`, `draw.go` | Palette and procedural pixel atlas; world, lighting and effects |
| `screens.go` | Native HUD, title, pause, level-up, results and settings views |
| `audio.go`, `save.go`, `strings/` | Mixer and cues; records and settings; English and Chinese strings |
| `*_test.go` | Headless shell, run and project tests |

## Boundaries

- `run.go` knows nothing about NSGo views, devices, sound, storage or wall-clock time. It takes one step of controls and a fixed `dt`, and returns a snapshot and the step's events.
- The shell turns input into controls and events into sounds, screen changes and saves. A rule never lives in a view, and a view never mutates a run.
- Tuning lives in `data/*.yaml`, versioned and canonical, with unknown fields refused. Constants in code only name things no designer would tune.
- `art.go` builds one atlas from the palette at startup, so the repository carries no sprite sheets.
- One canvas draws the world. The HUD and menus are native views above it and read run snapshots through signals instead of polling the simulation.
- A seed plus a list of controls replays a run. The map, spawns, offers and drops all derive from the run's streams.
- Pin `nsgo-dev/game` and `nsgo-dev/nsgo`; upgrade both in a change of its own after the tests pass. No cgo and no hand-written C.

## Core flows

**Frame.** A display frame runs zero or more fixed updates, then one `Draw` with the interpolation alpha. Each update steps the run unless a screen pauses it. HUD signals update at a lower rate so the view tree does not re-commit every frame.

**Run.** Move the keeper, spawn off-screen, chase, fire automatically, kill, drop embers, pull them in, fill the XP bar, pause on three cards, apply one, resume. At 12:00 the Hollow King arrives; defeat or death ends the run and opens the results.

**Upgrade.** `upgrades.go` takes the eligible weapons and passives that are not maxed and draws three distinct cards from the run's stream. The shell shows the cards and returns the choice; the run applies it and recomputes the derived stats.

## Data and state

`game.yaml` holds the window, the 480×270 virtual screen and the input map. `data/*.yaml` holds the balance. Two save slots live in `core.Storage`: settings (volumes, language, quality, shake) and records (best time, kills, level, wins). A run is not saved: closing the app ends it. There is no account, cloud or network state.

## Verification

Headless tests play a run from its own API: replays, movement, contact, kills, embers, level cards, defeat and three minutes of waves. `nsgotest` tests play the shell from the title into the arena, through a pause and back. `go tool nsgo check -target macos,windows,linux,web` covers builds. Frame pacing, gamepad feel, sound and the read of darkness need a manual run.

## Subjects

| Document | When to read |
| --- | --- |
| [The run model](docs/architecture/run.md) | Changing simulation, spawning, collision or upgrades |
