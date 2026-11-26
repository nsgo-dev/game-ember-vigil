# The arena

The arena draws one run into the 480×270 canvas and shows a native HUD, level-up prompt and defeat overlay above it. It reads a run only through `Snapshot`, `Events` and a copied seed, so nothing in drawing can change a run or its replay.

The palette and the fallback atlas are generated at startup from `art.go`'s rows of palette indices, so one colour list governs the ground, the creatures and the native screens. Today the renderer draws that procedural atlas; once the stylized 2D sprite sheets land under `assets/sprites/`, `art.go` will load them into `sprite.Atlas` and drive actor frames through `sprite.Animation`, keeping the procedural atlas as fallback.

## The frame

A displayed frame draws, in order: the ground crop, one sprite batch of everything alive, the particles, and the darkness with the keeper's light over it. The batch is shared by all sprites, so a crowd costs one draw call; a struck enemy carries a `sprite.Effect` of its own and splits the batch for the tenth of a second it flashes.

Positions come from the shell's copy of the last two steps: the sprite batch places an actor between where it was and where it is by the loop's alpha, so movement stays smooth on displays faster than the fixed step. A frame with no step behind it, such as a paused run's, holds the last step's places instead of blending toward nothing.

The ground is the exception: 1920×1280 pixels painted once when a run starts, drawn each frame as the camera's crop of it. It comes from a cosmetic stream on the run's seed, so a seed always paints the same forest floor, and painting it never touches the run.

## Camera

The camera eases toward the keeper at a rate independent of frame length, then clamps to the arena, which is the only wall the arena has. An arena smaller than the view stays pinned to its corner. Screen shake moves the same clamped window, so a violent frame still cannot show outside the arena.

## Light

`lights2d` lays an ambient darkness over the whole canvas and adds a flickering lantern on the keeper plus additive glows for shots and embers. This version has no occluders, so the scene disables shadows: nothing should pay for visibility polygons until something casts a shadow.

## Effects

Hit flashes, death puffs, ember trails and the screen's shake all run on the cosmetic stream, never on the run's streams. Every list is capped, so a burst of kills cannot grow a frame without a limit. Ember trails are decoration only: they leave where an ember already is and cannot move it.

## HUD

The native views read one `hudState` signal that the shell publishes at `hudRate`, not every step, because a view tree that re-commits sixty times a second costs more than the HUD tells anyone. The signal carries the values a player reads, and the views subscribe to it through `core.Dynamic`.

The level-up prompt and the defeat overlay are keyed to the run's phase in that signal: the run itself stops while a level-up waits and stays over once the keeper dies, so the shell needs no separate screen for either. Later milestones replace both views without touching the run.
