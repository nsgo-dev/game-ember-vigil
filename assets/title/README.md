# Ember Vigil Title Screen Art Kit

Hand-painted 2D layered title key art, painted title wordmarks, and nine-slice menu buttons for Ember Vigil, matching the Cinderwood Grove palette, wrought-iron/tarnished-bronze materials, and lantern-lit night atmosphere of `assets/sprites/`, `assets/environment/`, and `assets/hud/`.

Key art layers share a **1920×1080** (16:9) coordinate space and are composed around the centre so both desktop 16:9 (e.g. 1280×720) and landscape phone aspect ratios (e.g. 844×390, ~2.16:1) keep the Keeper, lantern, title area, and bottom menu visible when cover-cropped around the centre. UI pieces (`logo_*`, `button_*`) are authored at **2× resolution** relative to a 1280×720 window (divide pixel dimensions and insets by 2 for 1× logical points).

`assets/Title.png` (1920×1080) is the flattened static composite of the new centred key art for fallback or static display.

## Asset Inventory

### Title Key Art Layers (Back-to-Front Draw Order)

All full-frame layers share identical `1920×1080` dimensions and align at `(0, 0)`.

| Draw Order | File | Size (px) | Format | Role & Intended Motion |
| --- | --- | --- | --- | --- |
| 1 (bottom) | `bg.png` | 1920×1080 | Opaque RGB | Nocturnal conifer forest clearing, mist valley, distant antlered silhouettes. Subtle slow breathing zoom (`1.00`–`1.02`) or minimal camera drift (`0.15×` parallax). |
| 2 | `mist_layer.png` | 1920×1080 | Transparent RGBA | Soft curling slate-blue mist band across mid-lower clearing, feathered to 0 at all edges. Slow horizontal sinusoidal drift (`±24`–`40 px` at `0.06`–`0.10 Hz`, `0.35×` parallax) and gentle alpha pulse (`0.65`–`0.95`). |
| 3 | `lantern_glow.png` (back) | 512×512 | Transparent RGBA | Warm golden-amber radial bloom centred on the lantern flame at **`(834, 402)`** in `1920×1080` space (normalised `(0.4344, 0.3722)`). Draw scaled ~`560×560` behind `keeper_fg.png`; modulate scale (`0.94`–`1.06`) and opacity (`0.75`–`1.00`) with a subtle multi-frequency flicker. |
| 4 | `keeper_fg.png` | 1920×1080 | Transparent RGBA | Centred hooded Keeper holding the raised lantern & wooden staff on a mossy rocky knoll with ferns and mushrooms (`bbox: 564..1375, 210..900`). Anchored at `0.6×` parallax with subtle idle vertical sway (`±2`–`3 px`, slow breathing period). |
| 5 | `lantern_glow.png` (front) | 512×512 | Transparent RGBA | Optional secondary tighter bloom (~`300×300` at `0.45`–`0.60` alpha, additive or normal blend) centred at `(834, 402)` over `keeper_fg.png` for warm flame flicker. |
| 6 | `ember_1.png`, `ember_2.png`, `ember_3.png` | 64×64 each | Transparent RGBA | 3 rising spark/cinder variants (teardrop spark, diamond cinder, drifting flame wisp). Spawn near the lantern `(834, 402)` and along the lower knoll (`y: 780..920`); drift upward with gentle horizontal sine sway, varying scale (`0.25×`–`0.65×`), and fade out near the upper clearing. |
| 7 (top art) | `canopy_frame.png` | 1920×1080 | Transparent RGBA | Framing mossy tree trunks, warm rim-lit boughs, and bottom-corner ferns along the left and right edges (centre 65% transparent). Strongest foreground parallax (`1.0×`) and gentle wind sway. |

### Safe-Crop Framing Notes

- **16:9 desktop (1280×720)**: Full `1920×1080` frame visible. Title sits cleanly in `y: 40..210`; Keeper spans `y: 210..900`; best records + centred 3-button menu (`Start`, `Settings`, `Quit`) sit in `y: 900..1050`.
- **2.16:1 phone landscape (e.g. 844×390)**: Cover-cropping `1920×1080` around the vertical centre keeps `y: 96..984` visible—leaving headroom above the Keeper (`y: 96..210`) for the title and clear space below (`y: 900..984`) for best records and the centred 2-button menu (`Start`, `Settings`, no Quit).

### Painted Title Wordmarks (Optional)

| File | Size (2× px) | Display (1× pt) | Format | Usage |
| --- | --- | --- | --- | --- |
| `logo_en.png` | 1024×256 | 512×128 | Transparent RGBA | Carved gilded ivory-gold fantasy serif wordmark (`EMBER VIGIL`) with antler-diamond crest |
| `logo_zh_cn.png` | 1024×256 | 512×128 | Transparent RGBA | Matching Simplified Chinese wordmark (`余烬守望`) with antler-diamond crest |

### Nine-Slice Menu Buttons (No Baked Text)

All three states share identical `480×120` dimensions, `4 px` outer transparent margin, and pixel-aligned silhouettes so focus/press transitions never shift. Top/bottom and left/right iron rails are straight and uniform so horizontal stretching fits both English and Chinese labels cleanly.

| File | Size (2× px) | Display (1× pt) | Nine-Slice Insets (2× px, T / R / B / L) | Notes |
| --- | --- | --- | --- | --- |
| `button_normal.png` | 480×120 | 240×60 | `36 / 44 / 36 / 44` | Dark weathered wood interior, riveted wrought-iron & tarnished-bronze rails and corner brackets |
| `button_focused.png` | 480×120 | 240×60 | `36 / 44 / 36 / 44` | Warm lantern-lit amber-gold border, glowing corner rivets, warm inner wood edge glow |
| `button_pressed.png` | 480×120 | 240×60 | `36 / 44 / 36 / 44` | Darkened smoked bronze bevel and recessed charcoal wood interior |

## Prompts (Reproducibility)

Generated via `agiox image edit` / `agiox image generate` (`--quality high`) referencing `assets/Title.png`, `assets/hud/panel_frame.png`, `assets/hud/card_common.png`, `assets/hud/card_focused.png`, and `assets/hud/pause_pressed.png`, then locally chroma-keyed, despilled, pixel-aligned, and resized:

- **`bg.png`**: Remove the hooded Keeper, lantern, staff, text, buttons, and extreme foreground framing trunks from the centred title composition; keep only the continuous nocturnal conifer forest clearing, mist valley, distant antlered silhouettes with dim orange eyes, and dark mossy forest floor slope.
- **`keeper_fg.png`**: Isolate only the centred hooded Keeper (tattered slate-blue cloak, bronze tree clasp, raised glowing wrought-iron lantern, gnarled wooden staff) and immediate mossy rocky knoll with ferns and tiny mushrooms on flat `#FF00FF` chroma-key background; globally keyed with lantern glass amber protection and scaled to phone-safe headroom (`y: 210..900` on `1920×1080`).
- **`canopy_frame.png`**: Isolate only the close foreground framing gnarled mossy tree trunks, overhanging warm rim-lit pine boughs, and bottom-corner ferns along the far left/right borders with the central 65% empty on `#FF00FF`.
- **`mist_layer.png`**: Hand-painted 2D dark fantasy cool slate-blue and muted teal-grey curling forest mist bands on pure `#000000` black, converted via unpremultiplied luminance-to-alpha with soft edge feathering.
- **`lantern_glow.png`**: Soft radial warm golden-amber lantern light bloom fading smoothly to pure `#000000` black, converted via luminance-to-alpha with radial edge fade.
- **`ember_1.png`–`ember_3.png`**: Three distinct hand-painted glowing rising ember spark variants (teardrop spark, faceted diamond cinder, curved drifting flame wisp) on `#000000`, sliced and converted to 64×64 RGBA.
- **`logo_en.png` / `logo_zh_cn.png`**: Hand-painted carved gilded ivory-gold fantasy serif wordmarks (`EMBER VIGIL` / `余烬守望`) with dark wrought-iron outlines and symmetric gilded antler-and-diamond crest on `#FF00FF`.
- **`button_normal.png` / `button_focused.png` / `button_pressed.png`**: Symmetric 4:1 rectangular nine-slice menu button frame with straight uniform riveted iron/bronze rails, ornate corner brackets, and empty dark weathered wood interior (normal, glowing amber-gold focused, darkened recessed pressed), pixel-aligned across all three states.
