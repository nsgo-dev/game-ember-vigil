# Meta progression design

Status: structure agreed with the user; the ember payout and the Long Night tiers are built, and the open questions below remain for the rest.

## Why

A run was too easy and left nothing behind: the balance bot won 9 of 10 seeds with 34–78% minimum health, every build ended up with all four weapons, and the Hollow King fell in under a minute. Players had no reason to start another run.

## Agreed

- Add progression that persists between runs, so each run earns something and the next run starts with new goals. This lifts the former non-goal of permanent upgrades, unlocks, shops and currencies in [requirements.md](requirements.md).
- Structure is a mix, led by unlocks and difficulty:
  - **Unlocks.** In-run challenges unlock new content: weapons and evolution first, passives and keepers in later batches.
  - **Long Night tiers.** Each win opens the next tier, and each tier makes the run harder. This is the long-term goal and keeps permanent power from flattening the game.
  - **Ember shop.** Each run pays embers for a small shop of permanent bonuses with a low total cap (for example at most +15% damage), mainly to help weaker players; higher tiers weaken those bonuses.
- Each Long Night tier adds one fixed rule on top of the tiers below it, so every tier brings a new kind of pressure, not only bigger numbers. The player picks a single tier number. Self-chosen rule mixes may come later, above tier 5. Built in `data/tiers.yaml`:

  | Tier | Adds | Numbers |
  | --- | --- | --- |
  | 1 | The base run, made harder first (see below) | — |
  | 2 | One more elite per run; a dead elite leaves a shadow zone behind | A Gloomwarden at 6:50; a zone of radius 48 for 14 s, 12 damage every 0.5 s inside |
  | 3 | A smaller lantern light and a shorter view | Embers drawn from 90% as far; sight, spawn ring and spawn view at 80% |
  | 4 | Fewer healing cards and half regeneration | At most one healing card per level-up; regeneration ×0.5 |
  | 5 | Faster enemies; the Hollow King arrives a minute early with one more attack | Foes ×1.1 speed and ×1.15 damage; the King at 11:00, adding a 13-bone fan 1 rad wide at ×1.4 bone speed |

  Winning a tier opens the next, saved as the highest tier won; tier 1 is always open. The title offers a tier button once a second tier is open, defaulting to the highest; a win that opens a tier picks it for Play Again. The HUD and the results name the tier.
- Over seeds 1–100 the sharp bot wins 67%, 53%, 43%, 26% and 22% from tier 1 to tier 5 (over 200 seeds 67%, 53%, 46%, 27% and 25%), and `bot_test.go` holds the fall. The rules alone barely moved the bot: the early Hollow King skips the last minute's hardest waves and clears the field, and faster foes die sooner on the halo and the trail, so tier 5 also raises foe damage. Tier 3's view alone changes nothing for a bot that reads every foe, which is why its light also shortens the ember pull.

- The first unlocks are two new weapons and weapon evolution. A weapon at max level, held with its paired passive, evolves into a stronger form when the keeper opens a chest dropped by an elite. Evolution itself is unlocked, and each recipe joins a collection the first time it is made in a run. New keepers wait for a later batch.
- The two new weapons fill the play styles the current four miss (single shots, orbit, ground fire, knockback pulse):

  | Weapon | Play | Unlocked by |
  | --- | --- | --- |
  | Spark Chain | A spark that jumps between nearby foes, 3 jumps at level 1 to 6 at level 5 | Killing 3000 foes in one run |
  | Ash Censer | A censer lobbed at the densest crowd, bursting and leaving a small fire | Defeating 3 Gloomwardens in one run |

- Evolution recipes (weapon at max level + paired passive + an opened chest):

  | Weapon | Passive | Evolves into | Change |
  | --- | --- | --- | --- |
  | Ember Bolt | Oil | Starfall Volley | Each shot splits into three small shots on hit |
  | Lantern Halo | Reach | Sun Ring | The orbs join into one continuous burning ring |
  | Cinder Trail | Step | Wildfire Path | Fire spreads sideways and touching patches merge |
  | Bell Toll | Warmth | Dawn Bell | The pulse briefly stuns foes and heals the keeper a little |
  | Spark Chain | Wick | Ember Storm | Every jump forks |
  | Ash Censer | Gathering | Pyre Censer | Each burst first pulls foes and embers to its centre |

- Gloomwardens drop the chests. A chest evolves an eligible weapon; with none, it grants one free upgrade and some embers. Chests evolve nothing until the player's first win.
- The collection shows an unmade recipe as "???" with a one-line hint (for example "The halo longs for a wider light") so players discover recipes themselves.
- Mending and Glow pair with no recipe; they stay the choice of a safe or a fast-levelling build.
- Embers earned by a run, then multiplied by 1 + 0.25 per Long Night tier above 1 (×2 at tier 5):

  | Source | Embers |
  | --- | --- |
  | Survival | 5 a minute (60 for a full run) |
  | Kills | 2 per 100 kills (about 100 for a won run) |
  | Chests | 15 each |
  | Win | 50 |

  A tier-1 win earns about 270, a death at 3:00 about 20.
- The shop has six items of five levels each, costing 50, 100, 150, 200 and 250 (about 4500 in all), so a player fills it in about 25 runs, around the time they reach tier 5. Tiers 4 and 5 halve every shop bonus. Item names differ from the passives so the two are never confused.

  | Item | Per level | At max |
  | --- | --- | --- |
  | Banked Fire | +3% damage | +15% |
  | Lamp Oil | −2% cooldown | −10% |
  | Hearth | +10 max health | +50 |
  | Light Foot | +3 move speed | +15 |
  | Lure | +5 pickup radius | +25 |
  | Redraw | One more redraw of the level-up cards per run (new mechanic) | 5 per run |

- Tier 1 is the base run, made harder in `data/waves.yaml` before any tier exists: enemy damage from 4:30 on rises by 13–20% and the Hollow King's health from 20000 to 32000. Over 30 seeds the sharp bot now wins 67% with a median minimum health of about 36% among its wins, and the boss fight lasts about 70 seconds; the bot test holds its wins on seeds 1–10 between 6 and 8.
- First version scope: the ember currency and shop, Long Night tiers 1–5, two new weapons, and six evolutions (one for each weapon).

## Open questions

- Exact numbers of the new weapons and the evolutions.
- Screens: where the shop and the collection live. The results show the embers a run earned, the new balance and what it unlocked.
