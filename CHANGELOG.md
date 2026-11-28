# Changelog

## [Unreleased]

- Start a run from the title and pause it in an empty arena (#1, #10 by @oneo-me).
- Simulate a run headlessly: the keeper, three enemies, Ember Bolt, embers and level-up cards (#2, #11 by @oneo-me).
- Draw the arena with a followed camera, a lantern-lit night and a native HUD, with prompts to take a card or start again after defeat (#3, #12 by @oneo-me).
- Add all four weapons, eight passives with four-slot caps, weapon visuals and the three-card level-up modal with keyboard, gamepad and tap selection (#4, #17 by @oneo-me).
- Complete the twelve-minute wave timeline with Thornling, Wisp, Shade, the Gloomwarden elite, the three-phase Hollow King boss, boss telegraphs and health bar, and the victory/defeat results screen (#5, #18 by @oneo-me).
- Integrate animated stylized 2D sprite sheets with idle/walk transitions, facing flips, projectile rotation and embedded upgrade card icons, keeping procedural art as a clean fallback (#15, #21 by @oneo-me).
- Add the title screen with embedded key art and best records, persistent versioned settings and records in `core.Storage`, pause settings and quit confirmation, and live English and Simplified Chinese localization (#6, #22 by @oneo-me).
- Add the sound mixer, CC0 placeholder music and SFX cues, brighter moonlit arena lighting and forest floor readability, level-up flourish, damage vignette, and adaptive low/medium/high quality presets (#7, #23 by @oneo-me).
- Integrate the hand-painted Cinderwood Grove arena tiles, forest props and border canopy with procedural fallback, and wire the remaining two-row boss/elite (`hollow_king.png`, `gloomwarden.png`) and weapon (`lantern_halo.png`, `cinder_trail.png`, `bell_toll.png`) sprite sheets (#25, #27 by @oneo-me).
- Strengthen monsters, waves and the Hollow King, upgrade Gloomwarden into a recurring mini-boss with radial bone bursts, ground aura and floating health bar, and add 360-degree encircling minion surge waves (#29 by @oneo-me).
- Add the bottom-left virtual touch stick and phone safe-area layout, enable the bundled app icon, verify web and cross-platform builds (macOS, Windows, Linux) and add a gameplay screenshot to the README (#8, #28 by @oneo-me).
- Face the keeper the way it walks, replace its lantern light with a fog of war past its sight and around a wider arena that it cannot enter, and field two and a half times as many monsters with about ten ninths of the health (#34 by @oneo-me).
- Draw the in-run HUD on the arena canvas from the hand-painted kit, with easing fills, a hit flash, a flickering lantern at low health, a level-up glow, popping weapon slots, a Gloomwarden bar and a canvas pause button (#31, #35 by @oneo-me).
- Keep every stage of a run demanding but winnable with sharp movement: fewer, harder-hitting monsters early, steeper scaling after six minutes and a tougher Hollow King (#37 by @oneo-me).
- Draw the level-up cards, pause menu and results on the arena canvas in the painted HUD style, with cards that deal in, lift and glow when focused and burn away when chosen (#32, #36 by @oneo-me).
- Keep the camera and the world gliding with the keeper on displays faster than the run's steps, instead of jittering as it moves (#40 by @oneo-me).
- Send anonymous usage statistics to Umami, with a settings switch to turn them off (#46 by @oneo-me).
- Fill the whole window with the arena, showing more of it on wider or taller windows and seeing farther through the fog, instead of leaving bars (#42 by @oneo-me).
- Publish the web version to GitHub Pages on every push to `main` (#44 by @oneo-me).
- Redraw the title with an animated painted cover and a painted menu centred at the bottom, without Quit on phones, tablets and the web, draw settings in the painted HUD style, and paint the touch stick and a thumb-sized pause button (#39, #43 by @oneo-me).
- Make the run harder after 4:30 and give the Hollow King more health, so a good run is no longer a sure win (#48 by @oneo-me).
- Pay embers at the end of every run, won or lost, show them and the new balance on the results, and keep the balance and unlocks across restarts (#49, #56 by @oneo-me).
- Add Long Night tiers 1 to 5, each adding one rule on top of the ones below, opened by winning the tier before and chosen on the title (#50, #57 by @oneo-me).
- Drop a chest from every Gloomwarden that, once a first win unlocks evolution, evolves a maxed weapon held with its paired passive into Starfall Volley, Sun Ring, Wildfire Path or Dawn Bell, and otherwise gives a free card, with a Collection on the title listing the recipes found (#52, #58 by @oneo-me).
- Add an ember shop on the title selling six small permanent bonuses, among them redraws of the level-up cards by tap, R or the gamepad, counting in full on every Long Night tier (#51, #59 by @oneo-me).
- Add Spark Chain, unlocked by 5000 kills in one run, and Ash Censer, unlocked by seven Gloomwardens in one run, which evolve into Ember Storm with Wick and Pyre Censer with Gathering (#53, #60 by @oneo-me).
- Make a win take both skill and luck: tougher foes from 2:00 on, and much tougher foes on every Long Night tier, so even a full shop rarely clears tier 5 (#62 by @oneo-me).
- Turn up small potions and ember magnets on the ground now and then, drop a magnet from every Gloomwarden, and offset them with foes that hit a little harder (#63 by @oneo-me).
- Show the frame rate and the longest recent frame in the corner when the settings turn it on (#63 by @oneo-me).
- Hold 120 frames a second on a ProMotion iPhone in a crowded fight, instead of dropping to about 80 with stutters, by mixing sound effects on Apple platforms in the game and drawing fewer images a frame (by @oneo-me).
