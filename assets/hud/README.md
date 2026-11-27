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
