# Ember Vigil HUD Art Kit

Hand-painted 2D HUD assets for Ember Vigil, matching the Cinderwood Grove palette, wrought-iron/tarnished-bronze materials, and lantern-lit night atmosphere of `assets/sprites/`, `assets/environment/`, and `assets/Title.png`.

All PNGs are transparent RGBA assets authored at **2× resolution** relative to their target display size on a 1280×720 window (divide pixel dimensions and insets by 2 for 1× logical points).

## Asset Inventory

### Meters (Frame + Croppable Fill)

Each meter pair shares identical canvas dimensions so both layers align at `(0, 0)`.

| File | Size (2× px) | Display (1× pt) | Draw Order | Fill Crop Direction & Active Range (2× px) |
| --- | --- | --- | --- | --- |
| `health_frame.png` | 112×208 | 56×104 | Top (over fill) | — (transparent glass panes; iron mullions overlay the oil) |
| `health_fill.png` | 112×208 | 56×104 | Bottom (under frame) | **Bottom-to-top**, active glass chamber `y: 77..155` (`h = 78`) |
| `xp_frame.png` | 420×48 | 210×24 | Bottom (under fill) | — |
| `xp_fill.png` | 420×48 | 210×24 | Top (over frame) | **Left-to-right**, active trough `x: 20..400` (`w = 380`) |
| `boss_frame.png` | 680×168 | 340×84 | Bottom (under fill) | — (no baked text; shared by Hollow King and Gloomwarden) |
| `boss_fill.png` | 680×168 | 340×84 | Top (over frame) | **Left-to-right**, active trough `x: 84..593` (`w = 509`) |

### Plaques, Badges, Slots & Controls

| File | Size (2× px) | Display (1× pt) | Notes |
| --- | --- | --- | --- |
| `timer_plaque.png` | 240×104 | 120×52 | Carved dark wood & wrought-iron plaque for centered `MM:SS` digits |
| `level_badge.png` | 96×96 | 48×48 | Diamond bronze badge for centered level digits |
| `kill_emblem.png` | 80×80 | 40×40 | Antlered-skull emblem placed left of `×<kills>` |
| `slot_empty.png` | 96×96 | 48×48 | Empty dark iron weapon/passive slot (fits 64×64 `assets/icons/` centered) |
| `slot_filled.png` | 96×96 | 48×48 | Equipped tarnished-bronze slot (fits 64×64 `assets/icons/` centered) |
| `slot_maxed.png` | 96×96 | 48×48 | Max-level gilded slot (fits 64×64 `assets/icons/` centered) |
| `pip_lit.png` | 24×24 | 12×12 | Lit glowing amber diamond level pip |
| `pip_unlit.png` | 24×24 | 12×12 | Unlit dark iron diamond level pip |
| `pause_normal.png` | 104×104 | 52×52 | Octagonal pause glyph button (normal state; verified readable at phone thumb size, no repaint needed) |
| `pause_pressed.png` | 104×104 | 52×52 | Octagonal pause glyph button (pressed state, pixel-aligned with normal; verified readable at phone thumb size) |

### Touch Controls (Mobile Virtual Stick)

Authored at **2× resolution** matching the HUD's wrought-iron, tarnished-bronze, and lantern-amber palette. `stick_knob_idle.png` and `stick_knob_active.png` are pixel-aligned so pressing/dragging never shifts the knob silhouette.

| File | Size (2× px) | Display (1× pt) | Notes |
| --- | --- | --- | --- |
| `stick_base.png` | 256×256 | 128×128 | Circular wrought-iron & tarnished-bronze compass ring with four cardinal diamond markers and a semi-translucent dark-iron interior disc (~32% alpha) so the arena floor stays visible beneath |
| `stick_knob_idle.png` | 112×112 | 56×56 | Riveted dark-iron & tarnished-bronze circular medallion with a softly muted amber center gem (resting state) |
| `stick_knob_active.png` | 112×112 | 56×56 | Pixel-aligned active/dragging state with an ignited incandescent golden-amber core and warm gilded bezel highlights |

**Touch Control Prompts (Reproducibility):**
- `stick_base.png`: Hand-painted 2D dark fantasy circular double-beveled wrought-iron and tarnished-bronze virtual joystick compass ring with four symmetric pointed bronze diamond cardinal markers and fine inner rim notches, combined with a clean semi-translucent dark-iron interior disc.
- `stick_knob_idle.png`: Hand-painted 2D circular riveted dark wrought-iron and tarnished-bronze joystick thumb medallion with recessed dark iron inner ring and softly muted warm amber faceted ember gem in the center.
- `stick_knob_active.png`: Pixel-aligned edit of `stick_knob_idle.png` with an ignited incandescent golden-amber fire gem core and warm golden lantern-fire bezel and rivet highlights.

### Nine-Slice Panels & Cards

| File | Size (2× px) | Nine-Slice Insets (2× px, T / R / B / L) | Notes |
| --- | --- | --- | --- |
| `panel_frame.png` | 256×256 | `44 / 44 / 44 / 44` | Pause, level-up, settings, and results modal backdrop; uniform straight iron borders |
| `card_common.png` | 304×400 | `60 / 44 / 56 / 44` | Upgrade card frame (unfocused); top/bottom center crests stretch best vertically only or at fixed width |
| `card_focused.png` | 304×400 | `60 / 44 / 56 / 44` | Upgrade card frame (focused/selected); pixel-aligned with `card_common.png` |

### Painted Numerals

Unified carved ivory-gold fantasy serif numerals with dark outlines for readability over both dark forest edges and warm lantern light. All glyphs share a consistent baseline and cap height (`56 px` inside a `64 px` canvas).

| Files | Size (2× px) | Display (1× pt) | Usage |
| --- | --- | --- | --- |
| `digit_0.png` – `digit_9.png` | 48×64 | 24×32 | Monospaced digits for timer, kills, and level |
| `digit_colon.png` | 28×64 | 14×32 | `:` separator for timer (`MM:SS`) |
| `digit_times.png` | 48×64 | 24×32 | `×` multiplier prefix for kill counter |

## Progression Art Kit

Art for the progression in [docs/meta-progression.md](../../docs/meta-progression.md), wired as each feature lands, with a procedural fallback kept for every file. The chest, the evolved forms' icons and sheets and the collection slots are drawn; the new weapons, shop and tier pieces still wait. Icons and sprite sheets are 1× like the rest of `assets/icons/` and `assets/sprites/`; the screen pieces here are 2× like the HUD kit above.

### Card Icons (`assets/icons/`, 64×64)

| File | Use |
| --- | --- |
| `spark_chain.png`, `ash_censer.png` | The two new weapons |
| `starfall_volley.png`, `sun_ring.png`, `wildfire_path.png`, `dawn_bell.png`, `ember_storm.png`, `pyre_censer.png` | Evolved forms, each repainted from its base weapon's icon (Ember Bolt, Lantern Halo, Cinder Trail, Bell Toll, Spark Chain, Ash Censer) |
| `banked_fire.png`, `lamp_oil.png`, `hearth.png`, `light_foot.png`, `lure.png`, `redraw.png` | Shop items: brazier, oil can, fireplace, slippers, lure charm and reshuffled cards, kept apart from the Oil, Warmth, Step and Gathering passives |

### Sprite Sheets (`assets/sprites/`, one row of 4 frames unless noted)

| File | Size | Frame | Use |
| --- | --- | --- | --- |
| `spark_chain.png` | 128×32 | 32×32 | The spark in flight, crackling |
| `spark_arc.png` | 256×64 | 64×64 | The jump arc, drawn left to right across the frame; stretch and rotate it between two foes; frame 4 fades |
| `ash_censer.png` | 128×32 | 32×32 | The censer tumbling in flight (a quarter turn per frame) |
| `ash_censer_burst.png` | 256×64 | 64×64 | The burst, then the small fire it leaves (frames 3–4 loop for the fire) |
| `starfall_volley.png` | 128×32 | 32×32 | A split shot; the shot before the split keeps `bolt.png` |
| `sun_ring.png` | 256×64 | 64×64 | The continuous burning ring, rotating |
| `wildfire_path.png` | 256×64 | 64×64 | A wider fire patch whose flames spread sideways, flickering |
| `dawn_bell.png` | 256×64 | 64×64 | The pale-gold stunning pulse, expanding and fading |
| `ember_storm.png` | 256×64 | 64×64 | The forking arc, drawn like `spark_arc.png`, splitting in two at mid-length |
| `pyre_censer.png` | 256×64 | 64×64 | The pull (frames 1–3 spiral inward), then the burst (frame 4) |
| `chest.png` | 128×64 | 32×32, 2 rows | Row 0: closed chest with a pulsing seam glow (loop); row 1: opening, frame 4 the empty open chest |

### Screen Pieces (`assets/hud/`, 2×)

| File | Size (2× px) | Display (1× pt) | Notes |
| --- | --- | --- | --- |
| `shop_row.png` | 640×128 | 320×64 | Shop item row; the left end holds an icon well centred near `(70, 64)` that fits a 64×64 icon. Nine-slice insets `32 / 40 / 32 / 136` (T / R / B / L); stretch horizontally only, as the well spans the top and bottom insets |
| `shop_row_focused.png` | 640×128 | 320×64 | Focused state, pixel-aligned with `shop_row.png`, same insets |
| `collection_locked.png` | 96×96 | 48×48 | Collection slot of an unmade recipe, with the "???" engraved; show the hint line beside it |
| `collection_unlocked.png` | 96×96 | 48×48 | Collection slot of a made recipe; fits a 64×64 icon centred, like `slot_*.png` |
| `tier_badge_1.png` – `tier_badge_5.png` | 96×96 | 48×48 | Long Night tier badges, sharing one diamond body bottom-aligned; lit gems grow from 1 to 4 and tier 5 adds a crimson flame crown. The centre plate is empty for a `digit_*.png` numeral |
| `ember_emblem.png` | 80×80 | 40×40 | Ember currency emblem, placed left of `×<embers>` like `kill_emblem.png` |

### Prompts (Reproducibility)

Generated with `agiox image edit --quality high` on flat `#FF00FF`, using the existing icons, sheets and HUD pieces as style references (an evolved icon or effect also took its base icon or sheet), then cut to the alpha bounds, magenta-tinted smoke greyed, scaled with premultiplied alpha and, for sheets, cut into 4 equal cells scaled together so frames stay aligned. Each prompt asked for the house style (hand-painted 2D dark fantasy, tarnished bronze, wrought iron, charred wood, ember-orange and lantern-amber light, crisp outlines) plus:

- **Weapon icons**: a pierced bronze thurible censer with glowing coals and ash smoke; star-shaped ember sparks linked by fiery arcs and iron chain links.
- **Evolution icons**: the base icon grown — the fire arrow bursting into three star shots; the lantern inside one unbroken ring of sun-fire; the cinder trail spreading into merging side patches; the gilded bell with a dawn-ray crown and a light shockwave; one central spark with four arcs that each fork; the censer white-hot inside an inward vortex of fire and ash.
- **Shop icons**: an iron brazier of banked coals with a poker; a tin oil can dripping one golden drop; a stone hearth with a kettle; winged-buckle slippers over a wind streak; a hooked lure charm drawing embers in; three blank fanned cards inside a looping fire arrow.
- **Sheets**: 4 equal cells on one row, the subject at the same place in each; a crackling spark; a zigzag fire-lightning arc across the cell; the censer turning a quarter per frame; the burst shrinking to coals; a small falling-star shot; a rotating ring of sun-fire; a wide spreading fire patch; a pale-gold expanding pulse with star glints; an arc forking at mid-length; an inward fire-and-ash spiral that bursts; a bronze-banded charred chest with an ember lock, idle then opening with a column of light.
- **Screen pieces**: a riveted iron row with a left icon well and divider, then its lantern-lit focused edit; a gilded slot and a dim iron slot engraved "???"; a diamond badge with 1–4 ember gems on its points, growing hotter, and at tier 5 a crimson flame crown; a beaded bronze coin with antler flourishes and a glowing ember gem.
