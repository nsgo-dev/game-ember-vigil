# Meta progression design

Status: structure agreed with the user; the ember payout, the Long Night tiers, chests with evolution, the ember shop and the two new weapons are built, and the open questions below remain for the rest.

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
  | 2 | One more elite per run; a dead elite leaves a shadow zone behind | A Gloomwarden at 6:50; a zone of radius 48 for 14 s, 12 damage every 0.5 s inside; shop bonuses ×0.8 |
  | 3 | A smaller lantern light and a shorter view | Embers drawn from 90% as far; sight, spawn ring and spawn view at 80%; shop bonuses ×0.75 |
  | 4 | Fewer healing cards, half regeneration and half shop bonuses | At most one healing card per level-up; regeneration and shop bonuses ×0.5 |
  | 5 | Faster enemies; the Hollow King arrives a minute early with one more attack | Foes ×1.1 speed and ×1.15 damage; the King at 11:00, adding a 13-bone fan 1 rad wide at ×1.4 bone speed; shop bonuses ×0.67 |

  Winning a tier opens the next, saved as the highest tier won; tier 1 is always open. The title offers a tier button once a second tier is open, defaulting to the highest; a win that opens a tier picks it for Play Again. The HUD and the results name the tier.
- Over seeds 1–100 the sharp bot wins 67%, 53%, 43%, 26% and 22% from tier 1 to tier 5 (over 200 seeds 67%, 53%, 46%, 27% and 25%), and `bot_test.go` holds the fall. With chests, which the bot opens unless a shadow zone covers them, and the harder waves they brought, seeds 1–100 give 73%, 65%, 49%, 32% and 28%. The rules alone barely moved the bot: the early Hollow King skips the last minute's hardest waves and clears the field, and faster foes die sooner on the halo and the trail, so tier 5 also raises foe damage. Tier 3's view alone changes nothing for a bot that reads every foe, which is why its light also shortens the ember pull.

- The first unlocks are two new weapons and weapon evolution. A weapon at max level, held with its paired passive, evolves into a stronger form when the keeper opens a chest dropped by an elite. Evolution itself is unlocked, and each recipe joins a collection the first time it is made in a run. New keepers wait for a later batch.
- The two new weapons fill the play styles the current four miss (single shots, orbit, ground fire, knockback pulse):

  | Weapon | Play | Unlocked by |
  | --- | --- | --- |
  | Spark Chain | A spark that jumps between nearby foes, 3 jumps at level 1 to 6 at level 5 | Killing 5000 foes in one run |
  | Ash Censer | A censer lobbed at the densest crowd, bursting and leaving a small fire | Defeating 7 Gloomwardens in one run, which needs tier 2's extra elite |

  The first conditions, 3000 kills and 3 Gloomwardens, were met by about 80% of a new player's first tier-1 runs, won or lost, so the unlocks were no goal. A run's kills top out near 5200 and tier 1 has six Gloomwardens: 5000 kills now takes a strong run (about half of the bot's tier-1 runs), and the seventh Gloomwarden makes Ash Censer a reward for reaching tier 2.

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
- The shop has six items of five levels each, costing 100, 150, 250, 350 and 450 (7800 in all), so a player who pays about 330 a run fills it in about 25 runs, around the time they reach tier 5. Every tier above 1 weakens the bonuses (×0.8, 0.6, 0.3 and 0.2 from tier 2 to 5, redraws rounded down). Item names differ from the passives so the two are never confused.

  | Item | Per level | At max |
  | --- | --- | --- |
  | Banked Fire | +3% damage | +15% |
  | Lamp Oil | −2% cooldown | −10% |
  | Hearth | +6 max health | +30 |
  | Light Foot | +2 move speed | +10 |
  | Lure | +2 pickup radius | +10 |
  | Redraw | One more redraw of the level-up cards per run (new mechanic) | 5 per run |

- Tier 1 is the base run, made harder in `data/waves.yaml` before any tier exists: enemy damage from 4:30 on rises by 13–20% and the Hollow King's health from 20000 to 32000. Over 30 seeds the sharp bot now wins 67% with a median minimum health of about 36% among its wins, and the boss fight lasts about 70 seconds; the bot test holds its wins on seeds 1–10 between 6 and 8. Chests then gave the bot a free card from each Gloomwarden, so enemy damage from 2:00 on rose by another 30% and from 7:30 on by 15% more; over 120 seeds the sharp bot wins about 73%, and about 82% with evolution unlocked, which shortens the Hollow King fight by about 15%. The evolutions' numbers live in `data/weapons.yaml`.
- Spark Chain and Ash Censer, also in `data/weapons.yaml`, are tuned to widen the choice, not the power: over seeds 1–100 the evolved bot with them wins 84% at tier 1 and 42% at tier 5, against 82% and 38% without them. Each alone barely moves tier 1, but together they lifted tier 5 from 38% to 56% before Spark Chain lost about a fifth of its damage and Ember Storm its extra damage fell from ×1.25 to ×1.2.
- The shop was first built at 50 to 250 a level (4500 in all), +10 health, +3 speed and +5 pickup a level, halved only at tiers 4 and 5. Over seeds 1–100 a full shop then took the evolved bot from 84%, 80%, 62%, 46% and 42% at tiers 1–5 to 100% at tiers 1–3, so tiers stopped mattering, and at about 300 embers a run it filled in about 15 runs. Lure alone lifted tier 3 from 60% to 98%: a wider pull levels the keeper faster. The bot's runs turn on small margins, so even a fifth of the bonuses still moves it: with the current shop, a full shop gives 100%, 93%, 94%, 80% and 68%. Its real aim is the weaker player: the slow bot wins none of 40 tier-1 runs without the shop and 13 with it full. A person plays with more margin than the bot; check the shop's pull in a manual run before tuning further.
- First version scope: the ember currency and shop, Long Night tiers 1–5, two new weapons, and six evolutions (one for each weapon).

## Open questions

- Screens: the shop and the collection open from the title. The results show the embers a run earned, the new balance and what it unlocked.
