# Roadmap

The first version ships in three completed milestones (M1–M3); all issues below are delivered on `main` as of #8.

## M1: Playable slice (done)

| Issue | Delivers | Depends on |
| --- | --- | --- |
| [#1](https://github.com/oneo-me/test-game-2/issues/1) | Project skeleton: fixed-step loop, title, arena shell and pause | — |
| [#2](https://github.com/oneo-me/test-game-2/issues/2) | Headless run model: keeper, enemies, combat and level-ups | #1 |
| [#3](https://github.com/oneo-me/test-game-2/issues/3) | Arena rendering, camera, lantern lighting and HUD | #1, #2 |
| [#4](https://github.com/oneo-me/test-game-2/issues/4) | Four weapons, eight passives and level-up cards | #2, #3 |

## M2: Complete run (done)

| Issue | Delivers | Depends on |
| --- | --- | --- |
| [#5](https://github.com/oneo-me/test-game-2/issues/5) | Wave director, the Hollow King and results | #2, #3, #4 |
| [#6](https://github.com/oneo-me/test-game-2/issues/6) | Menus, settings, localization and saved records | #1, #3, #5 |
| [#15](https://github.com/oneo-me/test-game-2/issues/15) | Animated sprite sheets and card icons integration | #3, #13 |

## M3: Ship (done)

| Issue | Delivers | Depends on |
| --- | --- | --- |
| [#7](https://github.com/oneo-me/test-game-2/issues/7) | Audio, game feel and quality presets | #3, #4, #5 |
| [#8](https://github.com/oneo-me/test-game-2/issues/8) | Touch layout, web build and release packaging | #5, #6, #7, #9, #14, #15, #24, #25 |
| [#9](https://github.com/oneo-me/test-game-2/issues/9) | App icon and title key art | — |
| [#13](https://github.com/oneo-me/test-game-2/issues/13) | Core actor sprite sheets and upgrade card icons | #9 |
| [#14](https://github.com/oneo-me/test-game-2/issues/14) | Late-wave enemies, elite and Hollow King sprite sheets | #13 |
| [#24](https://github.com/oneo-me/test-game-2/issues/24) | Arena environment ground tiles and forest props | #9 |
| [#25](https://github.com/oneo-me/test-game-2/issues/25) | Arena environment art and late-wave sprite wiring | #14, #15, #24 |

## Later

- More keepers, maps and weapons, and weapon evolutions.
- Permanent unlocks and a meta-progression loop.
- Leaderboards, accounts and cloud saves.
- Android and iOS store builds, achievements and modding.

The first version's scope and non-goals are in [requirements.md](requirements.md).
