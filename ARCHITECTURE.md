# Ember Vigil architecture

Ember Vigil is an NSGo app with one fixed-step game loop. A run is simulated headlessly at 60 Hz and drawn, with its HUD and its level-up, pause and results overlays, into one canvas; native NSGo views show the title and settings and lie, invisible, over the canvas's buttons. YAML files under `data/` hold the balance; the shell owns screens, input and saves.

## Code map

| Path | Responsibility |
| --- | --- |
| `main.go`, `project.go` | NSGo entry point and the parsed `game.yaml` |
| `game.go` | Root component: loop, screens, input translation, pause, save and sound hooks |
| `run.go` | One run: keeper, enemies, shots, embers, contact, XP and the snapshot |
| `upgrades.go` | The upgrade pool, three-card offers and the derived stat block |
| `waves.go` | The wave director: timeline, spawn ring, caps and the view it stays outside |
| `data.go`, `data/` | Tuning YAML and its loaders: version checked, unknown fields refused, canonical round trip |
| `art.go`, `draw.go`, `quality.go` | Palette, embedded sprite-sheet atlases with procedural fallback; world, lighting, effects and quality presets |
| `hud.go` | The canvas HUD: the `assets/hud/` kit with procedural fallback, its layout, motion and pause hit area |
| `overlay.go` | The canvas level-up cards, pause and results: layout, deal, lift and burn motion, and their painting |
| `screens.go` | Native title and settings, and the invisible buttons over the canvas overlays |
| `audio.go`, `save.go`, `strings/` | Mixer and cues; records and settings; English and Chinese strings |
| `*_test.go` | Headless shell, run and project tests |

## Boundaries

- `run.go` knows nothing about NSGo views, devices, sound, storage or wall-clock time. It takes one step of controls and a fixed `dt`, and returns a snapshot and the step's events.
- The shell turns input into controls and events into sounds, screen changes and saves. A rule never lives in a view, and a view never mutates a run.
- Tuning lives in `data/*.yaml`, versioned and canonical, with unknown fields refused. Constants in code only name things no designer would tune.
- `art.go` and `hud.go` load embedded stylized 2D sprite sheets (`assets/sprites/`), arena environment tiles and props (`assets/environment/`), card icons (`assets/icons/`) and the HUD kit (`assets/hud/`), falling back to the procedural atlas and ground when any asset is absent, and paints the ground once per run from its seed.
- One canvas draws the world and, at the canvas's real size, the HUD and the in-run overlays. The title and settings are native views above it. Both read run snapshots through a signal instead of polling the simulation.
- A seed plus a list of controls replays a run. The map, spawns, offers and drops all derive from the run's streams.
- Pin `nsgo-dev/game` and `nsgo-dev/nsgo`; upgrade both in a change of its own after the tests pass. No cgo and no hand-written C.

## Core flows

**Frame.** A display frame runs zero or more fixed updates, then one `Draw` with the interpolation alpha. Each update steps the run unless a screen pauses it. The arena canvas draws the ground, one sprite batch, the glows and the fog of war; [the arena](docs/architecture/arena.md) owns that order. The HUD signal updates at a lower rate so the view tree does not re-commit every frame; the canvas HUD eases toward it on every step.

**Run.** Move the keeper, spawn off-screen, chase, fire automatically, kill, drop embers, pull them in, fill the XP bar, pause on three cards, burn the chosen one away, apply it, resume. At 12:00 the Hollow King arrives; defeat or death ends the run and opens the results.

**Upgrade.** `upgrades.go` takes the eligible weapons and passives that are not maxed and draws three distinct cards from the run's stream. The shell shows the cards and returns the choice; the run applies it and recomputes the derived stats.

## Data and state

`game.yaml` holds the window, the 480×270 virtual screen and the input map. `data/*.yaml` holds the balance. Two save slots live in `core.Storage`: settings (volumes, language, quality, shake) and records (best time, kills, level, wins). A run is not saved: closing the app ends it. There is no account, cloud or network state.

## Verification

Headless tests play a run from its own API: replays, movement, contact, kills, embers, level cards, defeat and three minutes of waves. `nsgotest` tests play the shell from the title into the arena, through a pause and back, press the canvas pause button, take a level-up card by tap, key, hover and gamepad, and read the HUD through its state and the canvas's label and the defeat overlay. Canvas tests check a frame's ground and sprites, the camera's clamp, a seed's ground and the HUD's fills, slots, boss bar, fallback and layout, and a benchmark keeps two hundred enemies and the HUD inside the frame budget. `go tool nsgo check -target macos,windows,linux,web` covers builds. Frame pacing, gamepad feel, sound and the read of the fog need a manual run.

## Subjects

| Document | When to read |
| --- | --- |
| [The run model](docs/architecture/run.md) | Changing simulation, spawning, collision or upgrades |
| [The arena](docs/architecture/arena.md) | Changing the canvas, the camera, the light, the fog, the effects or the HUD |
