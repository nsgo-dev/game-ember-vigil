# Ember Vigil architecture

Ember Vigil is an NSGo app with one fixed-step game loop. A run is simulated headlessly at 60 Hz and drawn, with its HUD and its level-up, pause and results overlays, into one canvas, which also draws the animated title and settings; invisible native NSGo views lie over the canvas's buttons. YAML files under `data/` hold the balance; the shell owns screens, input and saves.

## Code map

| Path | Responsibility |
| --- | --- |
| `main.go`, `project.go` | NSGo entry point and the parsed `game.yaml` |
| `game.go` | Root component: loop, screens, input translation, pause, save and sound hooks |
| `run.go` | One run: keeper, enemies, shots, embers, chests, evolved weapons, contact, XP and the snapshot |
| `weapons.go` | The run's unlockable weapons: Spark Chain's leaps and Ash Censer's flight, pull and burst |
| `upgrades.go` | The upgrade pool, three-card offers and the derived stat block |
| `waves.go` | The wave director: timeline, spawn ring, caps and the view it stays outside |
| `tiers.go` | Long Night tiers: their rules folded up to a run's tier, and the shadow zones dead elites leave |
| `data.go`, `data/` | Tuning and progression YAML and its loaders: version checked, unknown fields refused, canonical round trip |
| `art.go`, `draw.go`, `quality.go` | Palette, embedded sprite-sheet atlases with procedural fallback; world, lighting, effects and quality presets |
| `hud.go` | The canvas HUD: the `assets/hud/` kit with procedural fallback, its layout, motion and pause hit area |
| `title.go` | The canvas title: the `assets/title/` kit with a still procedural fallback, its layout, cover motion and menu |
| `overlay.go` | The canvas level-up cards, pause and results: layout, deal, lift and burn motion, and their painting |
| `settings.go` | The canvas settings panel: its layout, painting and the controls over it |
| `collection.go`, `chest.go` | The canvas evolution collection reached from the title, and the banner naming what an opened chest gave |
| `shop.go` | The canvas ember shop reached from the title: its rows, focus and the native buttons that buy |
| `screens.go` | The invisible native buttons over the canvas title and overlays |
| `audio.go`, `sfx.go`, `save.go`, `strings/` | Mixer and cues, and the software mixer Apple targets play effects through; settings, records, progression and the shop's buy rule; English and Chinese strings |
| `stats*.go` | Anonymous usage statistics sent to Umami while the settings allow it |
| `*_test.go` | Headless shell, run and project tests |

## Boundaries

- `run.go` knows nothing about NSGo views, devices, sound, storage or wall-clock time. It takes one step of controls and a fixed `dt`, and returns a snapshot and the step's events.
- The shell turns input into controls and events into sounds, screen changes and saves. A rule never lives in a view, and a view never mutates a run.
- Tuning lives in `data/*.yaml`, versioned and canonical, with unknown fields refused. Constants in code only name things no designer would tune.
- `art.go` and `hud.go` load embedded stylized 2D sprite sheets (`assets/sprites/`), arena environment tiles and props (`assets/environment/`), card icons (`assets/icons/`), the HUD kit (`assets/hud/`) and the title kit (`assets/title/`), falling back to the procedural atlas and ground when any asset is absent, and paints the ground once per run from its seed.
- One canvas draws the title or the world and, at the canvas's real size, the HUD and the in-run overlays. Invisible native views above it take the presses. Both read run snapshots through a signal instead of polling the simulation.
- A seed plus a list of controls replays a run. The map, spawns, offers and drops all derive from the run's streams.
- Pin `nsgo-dev/game` and `nsgo-dev/nsgo`; upgrade both in a change of its own after the tests pass. No cgo and no hand-written C.

## Core flows

**Frame.** A display frame runs zero or more fixed updates, then one `Draw` with the interpolation alpha. Each update steps the run unless a screen pauses it. The arena canvas draws the ground, one sprite batch, the glows and the fog of war; [the arena](docs/architecture/arena.md) owns that order. The HUD signal updates at a lower rate so the view tree does not re-commit every frame; the canvas HUD eases toward it on every step.

**Run.** Move the keeper, spawn off-screen, chase, fire automatically, kill, drop embers, pull them in, pick up the potions and magnets that turn up nearby, fill the XP bar, pause on three cards, burn the chosen one away, apply it, resume. A Gloomwarden drops a chest; opening it holds the run briefly, then evolves a weapon or offers a free card, and the shell saves a new recipe to the collection at once. At 12:00 the Hollow King arrives; defeat or death ends the run, pays its embers and opens the results. A run plays one Long Night tier, chosen on the title among those its wins have opened, and every rule up to it, and starts with the shop's bonuses folded into its stats.

**Upgrade.** `upgrades.go` takes the eligible weapons and passives that are not maxed and draws three distinct cards from the run's stream. The shell shows the cards and returns the choice, or asks for a redraw while the run has one; the run applies it and recomputes the derived stats.

## Data and state

`game.yaml` holds the window, the 480×270 virtual screen (expanded to fill the window) and the input map. `data/*.yaml` holds the balance; `data/progression.yaml` holds the ember payout, the unlock conditions and the shop, and `data/tiers.yaml` the rule each Long Night tier adds. Three save slots live in `core.Storage`: settings (volumes, language, quality, shake, statistics, frame rate readout), records (best time, kills, level, wins) and progression (ember balance, shop levels, highest Long Night tier won, unlocks, evolution collection), beside a random install ID for the statistics. A save without a progression slot starts it empty. When a run ends, the run reports its payout and the unlocks it met, and the shell adds them to the progression and saves it once; a recipe a run makes, and a shop level bought, is saved at once. A run evolves weapons only when the shell enables it, after the evolution unlock, and offers a weapon `data/weapons.yaml` locks only once the shell names its unlock. A run is not saved: closing the app ends it. There is no account or cloud state; the only network traffic is the usage statistics, a pageview on launch and on each run, an event when a run ends and a heartbeat each minute of play, which the settings turn off and builds of `nsgo dev`, tests and local web servers never send.

## Verification

Headless tests play a run from its own API: replays, movement, contact, kills, embers, level cards, defeat, chests, each weapon and evolution, and three minutes of waves. `nsgotest` tests play the shell from the title into the arena, through a pause and back, press the canvas pause button, take a level-up card by tap, key, hover and gamepad, hide Quit on phones, evolve a weapon after a first win and read the collection after a restart, offer the weapons a run unlocked after a restart, buy in the shop by tap, key and gamepad and redraw cards in the next run, and read the HUD through its state and the canvas's label and the defeat overlay. Canvas tests check a frame's ground and sprites, the camera's clamp, a seed's ground and the HUD's fills, slots, boss bar, fallback and layout, and a benchmark keeps two hundred enemies and the HUD inside the frame budget. `go tool nsgo check -target macos,windows,linux,web` covers builds. Frame pacing, gamepad feel, sound and the read of the fog need a manual run.

## Subjects

| Document | When to read |
| --- | --- |
| [The run model](docs/architecture/run.md) | Changing simulation, spawning, collision or upgrades |
| [The arena](docs/architecture/arena.md) | Changing the canvas, the camera, the light, the fog, the effects, the HUD, the touch controls or the title |
