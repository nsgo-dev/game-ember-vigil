# Ember Vigil requirements

Ember Vigil is a survivors-like 2D game: the last keeper of the First Ember walks an ash-choked forest for twelve minutes while shadow creatures try to snuff the light. It is built in Go with NSGo and `nsgo-dev/game`.

## Goals

- Ship one complete, replayable twelve-minute run that a new player can finish and a skilled player can beat reliably.
- Make the lantern's light the game's identity: it is the keeper's weapon, pickup range and hope at once.
- Prove the NSGo game layer can carry a real action game at 60 fps with hundreds of actors.

## Players

- People who play Vampire Survivors, Brotato or arcade survival games in short sessions.
- Desktop players with a keyboard or a gamepad, and web players.
- English and Simplified Chinese readers.

## First version

The first version is one playable run on one open arena:

- One keeper and one map, the Cinderwood Grove, roughly 1920×1280 world units. Decor is drawn but never blocks movement.
- Four weapons, five levels each: Ember Bolt (auto-aimed shots), Lantern Halo (orbiting embers), Cinder Trail (burning ground) and Bell Toll (a knockback pulse).
- Eight passives, five levels each: Wick (damage), Oil (cooldown), Reach (area), Step (move speed), Warmth (max health), Mending (regeneration), Gathering (pickup radius) and Glow (XP gain).
- Six enemy kinds: Ash Mite, Moth, Husk, Thornling, Wisp and Shade, plus the Gloomwarden elite and the Hollow King boss.
- Embers drop XP. Each level pauses the run and offers three upgrade cards. Four weapon slots and four passive slots fit in a run; a full build falls back to healing.
- Twelve minutes per run. Enemy mixes, surges, elite spawns and stats scale with the clock. The Hollow King appears at 12:00.
- Defeat the boss to win; reach zero health to lose. Results show the outcome, elapsed time, kills, level and whether a record was set.
- Title, run, level-up, pause, results and settings screens.
- Settings keep master, music and SFX volume, screen shake, quality and language. Records keep best time, most kills, highest level and wins.
- Keyboard, gamepad and touch input. English and Simplified Chinese strings.
- Procedural pixel art in one original palette, a lit darkness layer, particles and screen shake. Audio starts from the game layer's CC0 placeholder pack.

The target is 60 fixed updates a second at 480×270 virtual pixels, holding 60 fps with 200 enemies and effects on the user's Mac.

## Out of scope

The first version does not include:

- Other keepers, maps, weapon sets or weapon evolutions.
- Permanent meta upgrades, unlocks, shops or currencies.
- Multiplayer, accounts, cloud saves, leaderboards or telemetry.
- Android and iOS store builds. The touch layout and builds are prepared, but store packaging comes later.
- Steam, consoles, achievements, modding or replay files.
- Hand-painted sprite sheets or custom music. The icon and title key art are the only image assets.

## Constraints

- Go 1.27, NSGo v0.3.4 and the `nsgo-dev/game` version pinned in `go.mod`; no cgo.
- All tuning lives in versioned YAML under `data/`, refuses unknown fields and is canonical on save.
- Gameplay art is generated in code, so the repository stays text except for the icon and title art.
- The same seed and controls replay the same run on every platform.
- Every change keeps `go tool nsgo test ./...` and `go tool nsgo check -target macos,windows,linux,web` passing.
- No CI is added unless the user asks for it.

## Assumptions

- The working title is Ember Vigil, the app ID `me.oneo.embervigil` and the module `me.oneo/embervigil`. The repository stays `oneo-me/test-game-2` until the user renames it.
- Desktop comes first; touch input and the web build ship with the first version, mobile store builds later.
- Audio uses the CC0 placeholder pack until a custom set exists.
- The icon and title key art are drawn by 彩叶 after the palette is on screen.

## Acceptance

The first version is done when a player can:

- Start from the title, play, level up, pause, win or lose, and play again.
- Feel every weapon and passive change the run, and see all six enemies, the elite and the boss.
- Change volume, shake, quality and language, and find the settings and records after a restart.
- Play with a keyboard, a gamepad or touch, in English or Simplified Chinese.
- Run the game on macOS, Windows, Linux and the web from the documented commands.

It is also done when the tests and cross-target checks pass, the same seed replays the same run, the game holds its frame budget, and no CI workflow was added.
