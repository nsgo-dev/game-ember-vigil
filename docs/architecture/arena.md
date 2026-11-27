# The arena

The arena draws one run into a canvas that fills the window: 480×270 virtual pixels on its shorter side, widened on its longer one so a window of any shape shows more of the arena instead of bars. It draws its HUD, level-up cards, pause and results over it at the canvas's real size. It reads a run only through `Snapshot`, `Events` and a copied seed, so nothing in drawing can change a run or its replay.

The palette and the procedural fallback atlas are generated at startup from `art.go`'s rows of palette indices, so one colour list governs the ground, particles, fallback actors and native screens. Stylized 2D sprite sheets under `assets/sprites/` are embedded via `go:embed`, sliced from uniform grids into `sprite.Frame` and `sprite.Animation` clips, and packed into a single padded `sheetAtlas` (2 px transparent gaps prevent bilinear edge bleeding). Upgrade card icons under `assets/icons/` are embedded and decoded once at startup for the HUD's slots and the level-up cards; any missing sheet or icon falls back cleanly to the procedural atlas or glyph badge without crashing.

## The frame

A displayed frame draws, in order: the ground crop (`SetImageSmoothing(true)` when hand-painted tiles are loaded, `false` for procedural fallback), Cinder Trail burning ground patches (`cinder_trail.png` batched smoothly or procedural arcs), boss telegraphs, the smooth stylized sprite batch (`sheetAtlas` with `SetImageSmoothing(true)`), the crisp procedural batch (fallback actors and particles with `SetImageSmoothing(false)`), Bell Toll shockwave pulses (`bell_toll.png` mapped across its four expansion frames or procedural rings), the twilight tint with the glows over it, and the fog of war's crop over everything. Outside the virtual canvas, the shell then draws the touch stick, in the kit's `stick_base` and idle or active knob (flat circles without them), and the HUD. Each batch shares one atlas, so a crowd costs one draw call per atlas; a struck enemy carries a `sprite.Effect` of its own and splits the batch for the tenth of a second it flashes.

Animations advance on fixed steps only while `Phase == PhasePlaying`, freezing cleanly when paused or on the level-up modal. The keeper switches between `idle` and `walk` clips based on movement, two-row enemies (`hollow_king.png`, `gloomwarden.png`) switch from row 0 movement to row 1 attack/telegraph clips when telegraphing or striking at close range, enemies derive deterministic per-ID phase offsets so crowds do not march in lockstep, horizontal `FlipX` respects each sheet's default facing (every sheet defaults right), and projectiles rotate along their travel direction.

Positions come from the shell's copy of the last two steps: every frame places an actor, its glow and the camera between where they were and where they are by the loop's alpha, so movement stays smooth on displays faster than the fixed step, however many frames fall between two steps. Positions are not rounded to virtual pixels: the canvas is scaled up, and rounding the camera and an actor apart made the followed keeper flicker a pixel. While the shell's updates leave the run unstepped, as a paused run's do, frames hold the last step's places instead of blending toward a step that is not coming.

The ground is the exception: the whole arena (2400×1760 pixels) painted once when a run starts, drawn each frame as the camera's crop of it. It comes from a cosmetic stream on the run's seed, so a seed always paints the same forest floor, and painting it never touches the run. `art.go` embeds `assets/environment/*.png`, blends seamless `ground_soil.png` and `ground_moss.png` through smooth two-octave value noise, scatters proportionally scaled transparent forest props (`grass_tuft`, `fern`, `moss_stone`, `mushrooms`, `stump`, `fallen_branch`), and frames all four outer edges with overlapping `tree_canopy.png` (purely visual, never blocking movement), falling back cleanly to procedural moonlit soil when environment assets are absent.

## Camera

The camera shows as much world as the window's virtual size; a resize keeps its centre. The run's spawn check still counts the 480×270 view, so the keeper's sight in the fog stays short of the spawn ring and a wider window never shows a monster appear. The camera eases toward the keeper on each step at a rate independent of frame length, blends between steps like an actor, then clamps to the arena, which is the only wall the arena has. An arena smaller than the view stays pinned to its corner. Screen shake moves the same clamped window, so a violent frame still cannot show outside the arena.

## Light

`lights2d` lays a light moonlit twilight tint over the canvas and screen-blends glows for shots, patches, pickups and embers; the keeper carries no light of its own. The active quality preset caps glow light count and controls shadow casting.

## Fog of war

The run's arena keeps a fog band along every edge (`arena.fog` in `data/run.yaml`) that the keeper cannot enter. The renderer paints a quarter-resolution mask of it with the ground, from the same seed: clear over the field, a ragged, cloudy rim just past the field's edge and solid darkness beyond, so the arena's border is never seen. Each frame draws the camera's crop of the mask, scaled up smoothly, above everything in the world, then a radial fog centred on the keeper: clear within its sight, solid past it. Together they hide enemies until they come close.

## Effects

Hit flashes, hit sparks, death puffs, ember trails, the level-up flourish, the crimson damage vignette and restrained screen shake all run on the cosmetic stream, never on the run's streams. Every list is capped by the active quality preset (`low`, `medium`, `high`, or adaptive `auto`), and screen shake strictly follows the settings toggle. Ember trails are decoration only: they leave where an ember already is and cannot move it.

## HUD

The HUD is drawn on the arena canvas after the light and fog, in the canvas's own points rather than the 480×270 virtual pixels, so the kit in `assets/hud/` (painted at 2×, sized and sliced as its README says) stays crisp at any window size. `layoutHUD` scales it with the window, clamped, inside the safe area: health, XP, level, kills and weapon slots top-left, the clock and the boss bar top-centre, pause top-right. The boss bar narrows before it reaches either side, and the touch stick's zone starts below the HUD. Each missing kit piece falls back to a procedural shape, and missing numerals fall back to canvas text.

The shell publishes one `hudState` signal at `hudRate`, not every step, because a view tree that re-commits sixty times a second costs more than the HUD tells anyone. The canvas reads it untracked and `hudMotion` moves toward it on the shell's fixed steps: the fills ease, a hit flashes the lantern, health at or under `keeper.lowHealth` in `data/run.yaml` makes it flicker, a level-up makes the XP strip glow, and a new weapon pops into its slot. None of this touches the run. The bar follows the Hollow King, or the first Gloomwarden between his appearances. Its caption is canvas text in the shell's language. The canvas's accessible label carries the clock, health, level, kills and boss.

Pause is drawn and hit-tested on the canvas in its points, never smaller than `buttonSide`: a click (press down, then up inside), or a touch that never reaches the stick. Escape and the gamepad's start button still go through the `pause` action.

## Overlays

The level-up cards, the pause menu with its quit confirmation, and the victory and defeat results are drawn on the canvas after the HUD, from the same kit: the card frames at their own shape (their crests stretch badly), and `panel_frame` as a nine-slice for panels and buttons. The results write the time, kills and level in the painted numerals. Each piece falls back to a procedural plate, and the words are canvas text in the shell's language, wrapped and cut to fit. Settings stays native.

The cards and the results are keyed to the run's phase in the HUD signal; pause is the shell's. `overlayMotion` runs on the fixed steps: cards deal in staggered with a small overshoot, the focused card lifts and glows, and a chosen card burns from its foot up. The shell hands `Run.Choose` the card only when the burn ends, so the run waits in its level-up phase and a replay never sees the burn.

Over each painted button or card lies an invisible native button of at least `buttonSide`, placed from the same layout as the drawing. It takes clicks, taps and assistive technology, carries the card's name and description, and focuses its card or item when the mouse comes over it. The keyboard and gamepad move the shell's own focus instead of Tab.

## Title

The title is drawn on the same canvas, instead of the arena, while it shows or while settings opened from it covers it. `layoutTitle` cover-crops the kit's 1920×1080 layers around their centre, puts the wordmark in the sky above the keeper, and centres the menu in a row at the bottom of the safe area with the best records just above it, over a shade that keeps them readable on the knoll. The menu is Start and Settings, and Quit only where an app can quit: not on a phone or tablet idiom, nor on the web, where it cannot close the tab. Focus moves along the row with either axis and wraps over the buttons shown.

`titleMotion` runs on the fixed steps only while the title shows, and not at all when the system asks for less motion. The layers drift by their own parallax, the lantern's glow flickers, embers rise along paths that follow from their index, so the cover keeps no particle state, and the wordmark, records and menu fade in. The quality preset scales the embers. Without every layer the title draws a still procedural cover. The buttons are `assets/title/` nine-slice frames in normal, focused and pressed states, or the overlays' plate without them, with the same invisible native buttons over them as the overlays.

