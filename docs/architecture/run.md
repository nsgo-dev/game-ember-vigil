# The run model

`run.go` simulates one run without NSGo: no views, devices, sound, storage or wall clock. The shell steps it at 60 Hz with one `Controls` value and reads back the step's events and a snapshot. Tests step it the same way, so every rule is testable headlessly and a seed plus a list of controls replays a run.

Today the model runs the complete twelve-minute run: the keeper, all six weapons (Ember Bolt, Lantern Halo, Cinder Trail, Bell Toll and the unlockable Spark Chain and Ash Censer) and their evolutions, all eight passives with four weapon and four passive slots, all six regular enemies (Ash Mite, Moth, Husk, Thornling, Wisp, Shade), the Gloomwarden elite with healing caches and chests, surges, and the three-phase Hollow King boss at 12:00.

## Time

- `Step` expects exactly one 60 Hz step. The loop owns the fixed rate; the run never scales by a display frame.
- The run clock starts at zero and drives waves, the boss and results. Pausing stops the clock because the shell stops stepping.
- Positions are `float64` world units. Only `draw.go` converts them to the 480×270 canvas and rounds for the pixel grid.

## Determinism

- `NewRun(seed, tier)` derives separate streams for spawns, offers and drops. The shell draws the ground's decor and its particles from its own streams on the same seed, so drawing never changes a run.
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
5. Apply weapon effects: bolts, the halo, trail patches, bell pulses, spark leaps and censers.
6. Kill, drop embers, pull them toward the keeper, collect XP.
7. Fill the XP bar; when it passes the threshold, stop and prepare three cards.
8. Advance the clock; report victory or defeat.

The run stops between steps while cards are open. `Choose` applies one card, recomputes derived stats and resumes. The shell never edits run state directly.

## Combat and collision

- A uniform grid of 32-unit cells, rebuilt each step, answers circle queries for shots, the halo, pulses and pickup. Enemy separation uses the same grid.
- Enemy contact has a short invulnerability window. Each lingering weapon keeps a per-target cooldown so a patch or halo cannot tick every step; Cinder Trail and an Ash Censer's fire are both ground fire and share one, so they do not stack on a foe.
- Knockback moves enemies along the hit direction; the boss resists it.
- The arena has no blocking obstacles in the first version. Clamping to the world bounds is the only wall; the keeper, embers and heal caches clamp to the field inside the fog band, while enemies and shots roam the whole arena.
- Damage, health, speeds and caps come from `data/*.yaml`; the code holds no balance numbers beyond defaults for a missing file.

## Waves and the boss

The director reads a timeline of spans keyed by seconds: each span names the enemy mix, spawn interval, maximum alive, health and damage multipliers. Surges and elite spawns are entries on the same timeline, so the whole run is one data file.

Spawns appear on a ring a little outside the view, clamped inside the arena (often in the fog), and never inside the view. A global cap protects frame time; when it is reached, spawns wait instead of dropping the timeline.

The timeline is tuned so pressure holds the whole run and movement decides it: early waves hit harder instead of crowding the keeper in, later spans scale faster than a full build, and `bot_test.go` checks that a bot dodging every step wins most seeds while the same bot steering three times a second loses most.

At 12:00 the director stops normal spawns, clears space and spawns the Hollow King. Its three phases and attacks are data, timed by the run clock and its own action queue.

## Upgrades

`upgrades.go` owns four weapon slots and four passive slots. The starter Ember Bolt fills one weapon slot at the start. The offer pool is every owned weapon and passive below level five plus every new weapon or passive that still fits a slot. A weapon whose `unlock` names an entry in `data/progression.yaml` stays out of the pool until the shell passes that id to `UnlockWeapons` before the first step; a replay must pass the same ids.

Three distinct cards are drawn from the pool on the offer stream. If fewer than three remain, the rest are healing cards, up to the limit the run's tier sets. `Redraw` spends one of the run's redraws on a new hand from the same stream, putting the cards it replaces after the rest of the pool so they return only when nothing else is left. Applying a card raises a level and recomputes one derived stat block: damage, cooldown, area, move speed, max health, regeneration, pickup radius and XP gain. Weapons read the block every step instead of caching a modified copy.

## Finds

`data/run.yaml`'s `finds` roll, on their own `finds` stream so the other streams never shift, for a small potion and a magnet to turn up `near` to `far` from the keeper: each kind every `every` seconds with its `chance`, while fewer than its `most` lie on the ground. A Gloomwarden, marked `magnet` in `data/enemies.yaml`, always drops one too. A potion heals `heal` of maximum health; a magnet sets every ember within `pull` of the keeper flying in and reports `EventMagnet`. Both share the heal caches' slice as pickups of another kind.

## Spark Chain and Ash Censer

`weapons.go` holds the two weapons a run unlocks. Spark Chain strikes the nearest living foe in `range` and leaps to the nearest foe it has not struck within `jumpRange`, striking the level's `count` foes in all; the hits land in the step it casts, and the snapshot's `Arcs` keep each leap for `duration` so the arena can draw it. Ash Censer picks, among the foes in `range`, the one whose burst would catch the most foes, and flies `flight` seconds to where it stood; the snapshot's `Censers` show it in flight. It then bursts with the level's damage, knockback and radius and leaves a fire patch `fireRadius` of the burst wide that burns `fireDamage` of it each `tick`. A censer carries the stats it was thrown with.

## Chests and evolution

An enemy marked `chest` in `data/enemies.yaml` (the Gloomwarden) drops a chest inside the field. Walking over one opens it and moves the run to `PhaseChest`, which holds the world still for the chest's `hold` in `data/run.yaml` while the shell plays the flourish; the run clock stops as it does for cards. The chest evolves the first weapon, in data order, that is at its last level, held with the passive its `evolution` names and not yet evolved, but only after the shell calls `EnableEvolution`, which it does once the first win has unlocked `evolution`; a replay must call it the same way. Otherwise the hold ends in a free level-up card that spends no XP. Every chest pays its embers.

An evolution keeps the weapon's last level and adds its own behaviour from `data/weapons.yaml`: Starfall Volley splits each bolt into small shots on hit, Sun Ring burns a whole band at the halo's orbit, Wildfire Path drops patches to the sides and merges wild patches that touch while the circle around both stays within `maxRadius`, Dawn Bell stuns struck foes (never the Hollow King) and heals the keeper, Ember Storm forks from every foe the spark strikes to `fork` more nearby foes that leap no further (at most `maxChainHits` a cast), and Pyre Censer, once it lands, pulls foes (never the Hollow King) and embers within `pullRadius` toward its centre for `pull` seconds before it bursts. `Evolutions()` lists the recipes a run made; the shell records each one in the collection as `EventEvolve` reports it.

The balance bot plays tier 1 with and without evolution; `bot_test.go` holds the plain bot within 2 to 5 wins on seeds 1–10, the evolved bot between that and 7, and evolved fights at least 10% shorter on the seeds both won. Its evolved bot also unlocks Spark Chain and Ash Censer, which were tuned to add choice rather than power, as [the design](../meta-progression.md) records.

## The ember shop

`data/progression.yaml` lists the shop's items, each adding its `amount` per level to a stat the passives use or to the redraws, and the cost of each level. `SetShop` hands the run the levels the progression holds before the first step; a replay must pass the same ones. The levels add to the passives in `recomputeStats`, after which the keeper starts at full health, and give `Redraws` in the snapshot. A tier's `shop` rule in `data/tiers.yaml` could multiply the bonuses, rounding redraws down; no tier uses it, as players should keep what they bought. Buying is `buyShopItem` in `save.go`, a pure rule on the progression; the shell saves after each sale.

## Payout and unlocks

A finished run reports what it earned without touching storage. `Payout()` counts whole minutes survived, whole hundreds of kills, chests opened and a win at the rates in `data/progression.yaml`, multiplied by 1 + `tierBonus` for each step the run's Long Night tier is above 1. `Unlocks()` lists the unlocks whose every condition (kills, elites defeated, a win) the run met: `evolution` for a win, `spark-chain` for 5000 kills and `ash-censer` for seven elites, which only a tier with an added elite holds. The shell adds both to the progression save and names only the unlocks that are new.

## Long Night tiers

`data/tiers.yaml` lists the rule each tier above 1 adds; tier 1 is the base run. `NewRun` folds the rules of tiers 2 to its tier into one `tierRules` once, so the step reads plain multipliers and lists instead of asking which tier it is, and a tier's numbers never live in code. The fold merges a tier's elites into the waves' in time order, multiplies the light (the ember pull), the sight (the snapshot's `Sight`, the spawn ring and the view the director stays outside), regeneration, foe speed, health and damage, keeps the fewest healing cards an offer may hold, and adds the seconds the Hollow King comes early. Shadow zones and the Hollow King's volley are the only state a tier adds; both live in `tiers.go`. An offer that runs out of upgrades on a tier that limits healing cards is shorter than three, so the shell counts the cards it shows.

The rule numbers and the foe health and damage multipliers are set with the balance bot; [the design](../meta-progression.md) records the win rates they give.

## Events

Each step returns a bounded list of events: `Shot`, `Hit`, `Kill`, `Ember`, `LevelUp`, `BossSpawn`, `BossPhase`, `Victory`, `Defeat`, `Chest`, `Evolve` and `Magnet`. The shell maps them to sounds, flashes, shake and screen changes. Events carry ids and positions, never pointers into run slices.

## Invariants

- No NSGo UI, input, audio or storage import in the run or its data loaders.
- Steady-state steps allocate nothing except the offer slice when a level opens.
- A cap on enemies, shots, embers and particles holds under every wave.
- Every data file loads with its version checked, unknown fields refused and a canonical round trip.
- A test runs the bot for a full twelve minutes and asserts a stable event count for a fixed seed.
