# The run model

`run.go` simulates one run without NSGo: no views, devices, sound, storage or wall clock. The shell steps it at 60 Hz with one `Controls` value and reads back the step's events and a snapshot. Tests step it the same way, so every rule is testable headlessly and a seed plus a list of controls replays a run.

Today the model runs the complete twelve-minute run: the keeper, all four weapons (Ember Bolt, Lantern Halo, Cinder Trail, Bell Toll), all eight passives with four weapon and four passive slots, all six regular enemies (Ash Mite, Moth, Husk, Thornling, Wisp, Shade), the Gloomwarden elite with healing caches, surges, and the three-phase Hollow King boss at 12:00.

## Time

- `Step` expects exactly one 60 Hz step. The loop owns the fixed rate; the run never scales by a display frame.
- The run clock starts at zero and drives waves, the boss and results. Pausing stops the clock because the shell stops stepping.
- Positions are `float64` world units. Only `draw.go` converts them to the 480×270 canvas and rounds for the pixel grid.

## Determinism

- `NewRun(seed)` derives separate streams for spawns, offers and drops. The shell draws the ground's decor and its particles from its own streams on the same seed, so drawing never changes a run.
- Every iteration is over a slice, never a map. Ties break by index.
- The run never reads the system random source, `time.Now` or map order. Cosmetic randomness uses its own stream and never changes gameplay.
- A seed and a control list produce the same events on every platform; tests compare two runs step by step.

## Entities

The run keeps packed slices for the keeper, enemies, shots, embers, ground patches, pulses and the boss. Removing an entry swaps in the last one. No reference to a slice index survives a step, so nothing aliases a recycled slot.

A generic ECS would fit, but the game has a fixed set of shapes and a hot loop. Typed slices, a uniform grid and a small event buffer keep the step allocation-free and easy to inspect; `game/entities` remains an option for a later, more varied game.

## Update order

1. Read controls, move the keeper, clamp it to the field inside the fog.
2. Tick cooldowns, invulnerability and status timers.
3. Let the wave director spawn; update the boss phase and spawns.
4. Move enemies and shots; resolve enemy separation, contact and hits.
5. Apply weapon effects: bolts, the halo, trail patches and bell pulses.
6. Kill, drop embers, pull them toward the keeper, collect XP.
7. Fill the XP bar; when it passes the threshold, stop and prepare three cards.
8. Advance the clock; report victory or defeat.

The run stops between steps while cards are open. `Choose` applies one card, recomputes derived stats and resumes. The shell never edits run state directly.

## Combat and collision

- A uniform grid of 32-unit cells, rebuilt each step, answers circle queries for shots, the halo, pulses and pickup. Enemy separation uses the same grid.
- Enemy contact has a short invulnerability window. Each weapon keeps its own per-target cooldown so a lingering patch or halo cannot tick every step.
- Knockback moves enemies along the hit direction; the boss resists it.
- The arena has no blocking obstacles in the first version. Clamping to the world bounds is the only wall; the keeper, embers and heal caches clamp to the field inside the fog band, while enemies and shots roam the whole arena.
- Damage, health, speeds and caps come from `data/*.yaml`; the code holds no balance numbers beyond defaults for a missing file.

## Waves and the boss

The director reads a timeline of spans keyed by seconds: each span names the enemy mix, spawn interval, maximum alive, health and damage multipliers. Surges and elite spawns are entries on the same timeline, so the whole run is one data file.

Spawns appear on a ring a little outside the view, clamped inside the arena (often in the fog), and never inside the view. A global cap protects frame time; when it is reached, spawns wait instead of dropping the timeline.

The timeline is tuned so pressure holds the whole run and movement decides it: early waves hit harder instead of crowding the keeper in, later spans scale faster than a full build, and `bot_test.go` checks that a bot dodging every step wins most seeds while the same bot steering three times a second loses most.

At 12:00 the director stops normal spawns, clears space and spawns the Hollow King. Its three phases and attacks are data, timed by the run clock and its own action queue.

## Upgrades

`upgrades.go` owns four weapon slots and four passive slots. The starter Ember Bolt fills one weapon slot at the start. The offer pool is every owned weapon and passive below level five plus every new weapon or passive that still fits a slot.

Three distinct cards are drawn from the pool on the offer stream. If fewer than three remain, the rest are healing cards. There is no reroll in the first version. Applying a card raises a level and recomputes one derived stat block: damage, cooldown, area, move speed, max health, regeneration, pickup radius and XP gain. Weapons read the block every step instead of caching a modified copy.

## Payout and unlocks

A finished run reports what it earned without touching storage. `Payout(tier)` counts whole minutes survived, whole hundreds of kills, chests opened and a win at the rates in `data/progression.yaml`, multiplied by 1 + `tierBonus` for each Long Night tier above 1; until tiers exist the shell passes tier 1. `Unlocks()` lists the unlocks whose every condition (kills, elites defeated, a win) the run met. The shell adds both to the progression save and names only the unlocks that are new.

## Events

Each step returns a bounded list of events: `Shot`, `Hit`, `Kill`, `Ember`, `LevelUp`, `BossSpawn`, `BossPhase`, `Victory` and `Defeat`. The shell maps them to sounds, flashes, shake and screen changes. Events carry ids and positions, never pointers into run slices.

## Invariants

- No NSGo UI, input, audio or storage import in the run or its data loaders.
- Steady-state steps allocate nothing except the offer slice when a level opens.
- A cap on enemies, shots, embers and particles holds under every wave.
- Every data file loads with its version checked, unknown fields refused and a canonical round trip.
- A test runs the bot for a full twelve minutes and asserts a stable event count for a fixed seed.
