# Meta progression design

Status: structure agreed with the user; the ember payout, the Long Night tiers, chests with evolution, the ember shop and the two new weapons are built, and the open questions below remain for the rest.

## Why

A run was too easy and left nothing behind: the balance bot won 9 of 10 seeds with 34–78% minimum health, every build ended up with all four weapons, and the Hollow King fell in under a minute. Players had no reason to start another run.

## Agreed

- Add progression that persists between runs, so each run earns something and the next run starts with new goals. This lifts the former non-goal of permanent upgrades, unlocks, shops and currencies in [requirements.md](requirements.md).
- Structure is a mix, led by unlocks and difficulty:
  - **Unlocks.** In-run challenges unlock new content: weapons and evolution first, passives and keepers in later batches.
  - **Long Night tiers.** Each win opens the next tier, and each tier makes the run harder. This is the long-term goal and keeps permanent power from flattening the game.
  - **Ember shop.** Each run pays embers for a small shop of permanent bonuses with a low total cap (for example at most +15% damage), mainly to help weaker players. Bonuses count in full on every tier; the tiers are made harder instead.
- Each Long Night tier adds one fixed rule on top of the tiers below it, so every tier brings a new kind of pressure, not only bigger numbers. The player picks a single tier number. Self-chosen rule mixes may come later, above tier 5. Built in `data/tiers.yaml`:

  | Tier | Adds | Numbers |
  | --- | --- | --- |
  | 1 | The base run | See [Difficulty](#difficulty) |
  | 2 | One more elite per run; a dead elite leaves a shadow zone behind | A Gloomwarden at 6:50; a zone of radius 48 for 14 s, 12 damage every 0.5 s inside; foes ×1.45 health and ×1.2 damage |
  | 3 | A smaller lantern light and a shorter view | Embers drawn from 90% as far; sight, spawn ring and spawn view at 80%; foes ×1.2 health and ×1.1 damage |
  | 4 | Fewer healing cards and half regeneration | At most one healing card per level-up; regeneration ×0.5; foes ×1.15 health and ×1.05 damage |
  | 5 | Faster enemies; the Hollow King arrives a minute early with one more attack | Foes ×1.1 speed, ×1.15 health and ×1.15 damage; the King at 11:00, adding a 13-bone fan 1 rad wide at ×1.4 bone speed |

  Winning a tier opens the next, saved as the highest tier won; tier 1 is always open. The title offers a tier button once a second tier is open, defaulting to the highest; a win that opens a tier picks it for Play Again. The HUD and the results name the tier.
- The multipliers stack, so tier 5's foes have ×2.3 health and ×1.6 damage. The tier rules alone barely moved the bot (the early Hollow King skips the last minute's hardest waves, and faster foes die sooner on the halo and the trail), which is why every tier also thickens its foes. Win rates are in [Difficulty](#difficulty).

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
- The shop has six items of five levels each, costing 100, 150, 250, 350 and 450 (7800 in all), so a player who pays about 330 a run fills it in about 25 runs, around the time they reach tier 5. The bonuses count in full on every tier: the user chose to make the tiers harder rather than weaken what players bought. Item names differ from the passives so the two are never confused.

  | Item | Per level | At max |
  | --- | --- | --- |
  | Banked Fire | +3% damage | +15% |
  | Lamp Oil | −2% cooldown | −10% |
  | Hearth | +10 max health | +50 |
  | Light Foot | +3 move speed | +15 |
  | Lure | +5 pickup radius | +25 |
  | Redraw | One more redraw of the level-up cards per run (new mechanic) | 5 per run |

- Spark Chain and Ash Censer, in `data/weapons.yaml` like the evolutions, are tuned to widen the choice, not the power: together they lifted the evolved bot's tier 5 from 38% to 56% until Spark Chain lost about a fifth of its damage and Ember Storm's extra damage fell from ×1.25 to ×1.2.
- First version scope: the ember currency and shop, Long Night tiers 1–5, two new weapons, and six evolutions (one for each weapon).

## Difficulty

The user wants a win to take both skill and luck: a skilled player still loses on a bad seed. The sharpest balance bot stands for the skilled player and the seed (spawns, cards and drops) for luck; the bot reacting three times a second stands for a weaker player.

- Tier 1 is set for a skilled player with nothing bought: from 2:00 on, foes have ×1.25 the health and ×1.2 the damage of the first version's late run, and the Hollow King has 32000 health.
- The potions and magnets that turn up on the ground lift every player, the magnets most as they level the keeper faster; every span's foe damage in `data/waves.yaml` rose ×1.12 to offset them. Only the full-power row below was measured again after.
- The shop is the largest source of power. On its own a full shop lifts that player's tier 1 from 37% to 96%, against 58% for evolution and both new weapons together; Lure is the strongest item, as a wider pull levels the keeper faster. The tiers are tuned for a player who has bought everything.

Over seeds 1–100:

| Player | Tier 1 | Tier 2 | Tier 3 | Tier 4 | Tier 5 |
| --- | --- | --- | --- | --- | --- |
| Skilled, nothing unlocked or bought | 37% | | | | |
| Skilled, evolution and both weapons | 58% | 6% | | | 0% |
| Skilled, nothing unlocked, full shop | 96% | | 48% | | |
| Skilled, everything unlocked and bought | 100% | 84% | 60% | 38% | 13% |
| Weaker, nothing bought | 0% | | | | |
| Weaker, everything unlocked and bought | 80% | | | | 0% |

So a new player climbs by buying, and the top tiers stay a test of skill and luck even with everything bought. `bot_test.go` holds tier 1 at 2 to 5 wins in 10 for the plain bot, and the full-power bot's win rate falling with every tier, from at least 90% to 5–25% at tier 5. On the higher tiers most losses come early, between 2:00 and 3:00, before the build is strong; a manual run should check that this feels fair.

## Open questions

- Screens: the shop and the collection open from the title. The results show the embers a run earned, the new balance and what it unlocked.
