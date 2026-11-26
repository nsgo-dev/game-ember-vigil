# Roadmap

The first version ships in three milestones. The developer closes one issue before starting the next; the art issue can run beside them.

## M1: Playable slice

| Issue | Delivers | Depends on |
| --- | --- | --- |
| [#1](https://github.com/oneo-me/test-game-2/issues/1) | Project skeleton: fixed-step loop, title, arena shell and pause | — |
| [#2](https://github.com/oneo-me/test-game-2/issues/2) | Headless run model: keeper, enemies, combat and level-ups | #1 |
| [#3](https://github.com/oneo-me/test-game-2/issues/3) | Arena rendering, camera, lantern lighting and HUD | #1, #2 |
| [#4](https://github.com/oneo-me/test-game-2/issues/4) | Four weapons, eight passives and level-up cards | #2, #3 |

## M2: Complete run

| Issue | Delivers | Depends on |
| --- | --- | --- |
| [#5](https://github.com/oneo-me/test-game-2/issues/5) | Wave director, the Hollow King and results | #2, #3, #4 |
| [#6](https://github.com/oneo-me/test-game-2/issues/6) | Menus, settings, localization and saved records | #1, #3, #5 |

## M3: Ship

| Issue | Delivers | Depends on |
| --- | --- | --- |
| [#7](https://github.com/oneo-me/test-game-2/issues/7) | Audio, game feel and quality presets | #3, #4, #5 |
| [#8](https://github.com/oneo-me/test-game-2/issues/8) | Touch layout, web build and release packaging | #5, #6, #7, #9 |
| [#9](https://github.com/oneo-me/test-game-2/issues/9) | App icon and title key art | — |

## Later

- More keepers, maps and weapons, and weapon evolutions.
- Permanent unlocks and a meta-progression loop.
- Leaderboards, accounts and cloud saves.
- Android and iOS store builds, achievements and modding.

The first version's scope and non-goals are in [requirements.md](requirements.md).
