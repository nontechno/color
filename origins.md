# Origins of `terminal-color-combinations.json`

## What the file is

A list of **108 terminal color combinations**: 36 background colors × 3 text colors each. Every entry says which colors to use, how they map to the xterm 256-color palette, how bright the pairing is overall, and gives the combination eight names in eight different registers.

The file is a JSON array of flat objects, one per combination:

```json
{
    "id": 29456,
    "Theme": "dark",
    "Luminosity": 407,
    "Background.rgb": "#000000",
    "Background.ansi": 16,
    "Text.rgb": "#7ddcc4",
    "Text.ansi": 115,
    "Pedantic": "Aquamarine on Absolute Black",
    "Funny": "Minty Fresh Void",
    "Outrageous": "Mint Tsunami Devours Eternity",
    "Vulgar": "Minty Motherfucking Void",
    "Corporate": "Mint Value-Add",
    "Geeky": "Null Pointer Mint",
    "Wholesome": "Fresh Mint Hug",
    "Sports": "Mint Hat-Trick"
}
```

Entries are ordered by theme (dark, between, light), then by background (darkest first within a group, roughly), then by the text colors' contrast against the background (highest first).

## Fields

| Field | Type | Meaning |
|---|---|---|
| `id` | integer | `Text.ansi * 256 + Background.ansi`. A compact key packing the two ANSI indexes into one number (text in the high byte, background in the low byte). **Not guaranteed unique**, see "Known limitations". |
| `Theme` | `"dark"` \| `"between"` \| `"light"` | Which of the three background groups the combination belongs to (36 each). It describes the **background**, not the text. |
| `Luminosity` | integer 0–999 | Overall perceived brightness of the pairing: the average of the background's and the text's CIELAB lightness (L\*, 0–100), scaled so black-on-black is 0 and white-on-white is 999. Formula: `round(999 * (L*_bg + L*_text) / 200)`. In this data it ranges from 350 to 718, since no pair is near either extreme. It measures brightness, **not** readability (see contrast below). |
| `Background.rgb` | `"#rrggbb"` | The background color as sRGB hex. |
| `Background.ansi` | integer 16–255 | Index of the nearest xterm-256 color to `Background.rgb`. |
| `Text.rgb` | `"#rrggbb"` | The text (foreground) color as sRGB hex. |
| `Text.ansi` | integer 16–255 | Index of the nearest xterm-256 color to `Text.rgb`. |
| `Pedantic` | string | A literal, systematic description: `<text color> on <background color>`, e.g. "Aquamarine on Black". Each color has one fixed name everywhere it appears, so the same color is never described two ways. |
| `Funny` | string | A pun or joke name. |
| `Outrageous` | string | An over-the-top, dramatic name. |
| `Vulgar` | string | A deliberately profane name (requested on purpose). Filter this field out where profanity is not wanted. |
| `Corporate` | string | Office-speak and business jargon, at most three words, e.g. "Mint Value-Add". |
| `Geeky` | string | Programmer and sysadmin references, at most three words, e.g. "Null Pointer Mint". |
| `Wholesome` | string | Warm, cozy and encouraging, at most three words, e.g. "Fresh Mint Hug". |
| `Sports` | string | Sports flavor, mostly built from football, basketball, hockey and soccer team names that echo the colors or sound like them, e.g. "Jags Teal Blackout" (Jacksonville's teal-and-black) or "Aston Villa Claret Cream". At most 32 characters. |

### How the ANSI index is chosen

- The palette searched is the xterm 256-color table, **indexes 16–255 only**: the 6×6×6 color cube (16–231, channel levels 0, 95, 135, 175, 215, 255) and the 24-step gray ramp (232–255, levels 8, 18, … 238).
- Indexes 0–15 (the "system" colors) are excluded on purpose. Terminal themes redefine them, so they do not map to a fixed RGB.
- "Closest" means the smallest Euclidean distance in CIELAB (ΔE76), which tracks perceived difference better than distance in RGB.
- The mapping is lossy. The ANSI color can look noticeably different from the `.rgb` value, especially for muted, dark colors, because the cube is coarse.

## How the data was produced

The goal was a set of backgrounds and text colors for terminal sessions that are **tasteful, distinct from each other, readable, and mapped to 256-color indexes**. It was built in stages.

### 1. Starting point: the xterm-256 palette

The first work was extracting the xterm-256 palette and splitting it into dark, light and in-between groups. That showed the palette alone gives few muted backgrounds: many entries are saturated or neon, and the dark end collapses into near-identical shades.

### 2. Candidate backgrounds from real terminal themes

Instead of inventing colors, backgrounds were harvested from existing terminal color schemes: the Windows Terminal JSON exports in the open-source **mbadolato/iTerm2-Color-Schemes** collection (818 schemes). Each scheme's `background` value was a candidate. Well-known schemes (Solarized, Gruvbox, Nord, Dracula, Tango, Ubuntu and so on) were given a mild preference.

### 3. Selection criteria for backgrounds

- **Not obnoxious.** No neon: a cap on CIELAB chroma (about 55–65 depending on group), so everything is muted.
- **Three lightness bands (by L\*):** dark (≤ 25), in-between (about 25–72), light (≥ 72). The in-between band was widened from an initial narrower range because it had too few candidates.
- **Visibly distinct.** Colors were de-duplicated by ΔE76 distance. An early set of 17 dark backgrounds looked identical to a human, which led to a minimum pairwise ΔE of about 10 (differences of 5–6 had looked like the same color). Selection used a farthest-point (max-min) approach so each group spreads across hue and lightness.
- **Distinct across groups too.** Groups were chosen one after another with earlier groups' colors on an avoid list, so a dark and an in-between color are not near-duplicates.
- **Twelve per group.** The light group cannot reach ΔE 10 for all 12 colors without garish yellows and pinks, so its closest pair is about ΔE 7.1. This was accepted rather than adding loud colors.

### 4. Three text colors per background

For each of the 36 backgrounds, three text colors were chosen from a pool made of the other backgrounds' colors (to encourage reuse of the palette) plus 26 extra accent colors. Criteria:

- **Readable:** WCAG contrast ratio of at least 4.5:1 against the background. For two backgrounds (Peacock Blue `#006984` and Emerald `#13773d`) no clearly different trio reaches 4.5:1, so the bar was relaxed in steps (4.0, 3.5, …) and the result is flagged on the HTML page. The lowest contrast in the data is 3.5:1.
- **Clearly different from each other:** at least ΔE 24 between the three text colors on a background.
- **Soft, not neon:** low-to-moderate chroma.
- **No vibrating pairs:** a penalty for hues opposite the background's hue on strongly colored backgrounds.
- **Reuse bonus:** a small preference for colors that are already in the background palette.
- **Variety:** a penalty for using the same text color on too many backgrounds. Without it, aqua and orchid were picked on about 20 of the 36 backgrounds.
- **At most one neutral per trio.** Pure black and pure white are never used as text.

### 5. Names

Names were added in rounds as the request sharpened: first plain descriptive names for single colors, then names for background+text combinations, then four names for each of the 108 pairs , and finally four more styles (Corporate, Geeky, Wholesome, Sports). The last round also revised the first four:

- `Pedantic` was regenerated from fixed color-name tables, so one color always has one name (before, the same color could appear as "Cyan", "Light Cyan" and "Ice Cyan"). 45 of the 108 Pedantic names changed.
- The other three original styles were kept as written.
- Corporate, Geeky and Wholesome were limited to three words, each using a word that fits one of the two colors (or both). Sports was later redone around real football, basketball, hockey and soccer team names (32 characters at most).

All 864 names (108 × 8) are unique across the file. Every name is at most 32 characters long (the Outrageous names were shortened to meet this).

### 6. Derived fields

`Theme` is the background's group. `Luminosity`, `*.ansi` and `id` were computed from the RGB values with the formulas above.

## Known limitations

- **`id` is not unique.** Distinct RGB colors can quantize to the same ANSI index, so 108 entries produce **103 distinct ids**. The colliding pairs: `#432708` and `#2e3440` with text `#79d4e6`; `#ecffe5` with `#145654` and `#fbf1c7` with `#245032`; `#fff0e3` with `#7a251e` and `#e5ede6` with `#5b0e05`; `#fff0e3` with `#245032` and `#e5ede6` with `#145654`; `#d8b590` and `#d7b37a` with text `#40145f`. Use `Background.rgb` plus `Text.rgb` when you need a unique key.
- **Contrast and distinctness are computed on the RGB values.** After snapping to ANSI indexes, contrast and separation can drift. Two backgrounds can even share an index (for example `#300a24` and `#1e1e1e` both map to 234).
- **Luminosity is a simple average.** It ignores which of the two colors is the text, so a dark-on-light pair and its inverse score the same.
- **Metrics are approximations.** ΔE76 is a simple color-difference formula. "Visibly distinct" and "readable" are targets backed by those numbers, not guarantees for every display or viewer.
