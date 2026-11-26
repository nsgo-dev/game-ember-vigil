# The arena

The arena draws one run into the 480×270 canvas and shows a native HUD, level-up prompt and defeat overlay above it. It reads a run only through `Snapshot`, `Events` and a copied seed, so nothing in drawing can change a run or its replay.

The palette and the procedural fallback atlas are generated at startup from `art.go`'s rows of palette indices, so one colour list governs the ground, particles, fallback actors and native screens. Stylized 2D sprite sheets under `assets/sprites/` are embedded via `go:embed`, sliced from uniform grids into `sprite.Frame` and `sprite.Animation` clips, and packed into a single padded `sheetAtlas` (2 px transparent gaps prevent bilinear edge bleeding). Upgrade card icons under `assets/icons/` are embedded and decoded once at startup for the level-up overlay; any missing sheet or icon falls back cleanly to the procedural atlas or glyph badge without crashing.

## The frame

A displayed frame draws, in order: the ground crop (`SetImageSmoothing(false)`), ground patches and boss telegraphs, the smooth stylized sprite batch (`sheetAtlas` with `SetImageSmoothing(true)`), the crisp procedural batch (fallback actors and particles with `SetImageSmoothing(false)`), shockwave pulses, and the darkness with the keeper's light over it. Each batch shares one atlas, so a crowd costs one draw call per atlas; a struck enemy carries a `sprite.Effect` of its own and splits the batch for the tenth of a second it flashes.

Animations advance on fixed steps only while `Phase == PhasePlaying`, freezing cleanly when paused or on the level-up modal. The keeper switches between `idle` and `walk` clips based on movement, enemies derive deterministic per-ID phase offsets so crowds do not march in lockstep, horizontal `FlipX` respects each sheet's default facing (`keeper.png` defaults left; enemy sheets default right), and projectiles rotate along their travel direction.

Positions come from the shell's copy of the last two steps: the sprite batch places an actor between where it was and where it is by the loop's alpha, so movement stays smooth on displays faster than the fixed step. A frame with no step behind it, such as a paused run's, holds the last step's places instead of blending toward nothing.

The ground is the exception: 1920×1280 pixels painted once when a run starts, drawn each frame as the camera's crop of it. It comes from a cosmetic stream on the run's seed, so a seed always paints the same forest floor, and painting it never touches the run. Today it paints procedural soil and decor; once `assets/environment/` lands, `art.go` will tile the hand-painted forest ground and scatter its props and edge canopy onto the same seeded canvas, keeping procedural ground as fallback.

## Camera

The camera eases toward the keeper at a rate independent of frame length, then clamps to the arena, which is the only wall the arena has. An arena smaller than the view stays pinned to its corner. Screen shake moves the same clamped window, so a violent frame still cannot show outside the arena.

## Light

`lights2d` lays a light moonlit twilight tint over the canvas and screen-blends a wide, flickering warm lantern on the keeper plus glows for shots, patches, pickups and embers. The active quality preset caps glow light count and controls shadow casting.

## Effects

Hit flashes, hit sparks, death puffs, ember trails, the level-up flourish, the crimson damage vignette and restrained screen shake all run on the cosmetic stream, never on the run's streams. Every list is capped by the active quality preset (`low`, `medium`, `high`, or adaptive `auto`), and screen shake strictly follows the settings toggle. Ember trails are decoration only: they leave where an ember already is and cannot move it.

## HUD

The native views read one `hudState` signal that the shell publishes at `hudRate`, not every step, because a view tree that re-commits sixty times a second costs more than the HUD tells anyone. The signal carries the values a player reads, and the views subscribe to it through `core.Dynamic`.

The level-up prompt and the defeat overlay are keyed to the run's phase in that signal: the run itself stops while a level-up waits and stays over once the keeper dies, so the shell needs no separate screen for either. Later milestones replace both views without touching the run.
