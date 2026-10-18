# Terminal color combinations: contents and usage

- **Live preview:** <https://color.nontechno.com/>
- **Sources:** <https://github.com/nontechno/color>

`terminal-color-combinations.json` is a curated set of **108 background + text color combinations** for terminal sessions, ready to apply with the standard xterm 256-color escape sequences or with plain RGB.

Every combination comes with:

- the background and text colors as RGB hex **and** as the nearest xterm-256 color index,
- a theme (dark, between, light) and a single brightness number to sort by,
- eight different names, one in each of eight naming styles, from "Pedantic" to "Vulgar".

For how and why these colors were chosen, see `origins.md`.

## What is inside

The file is a JSON array with 108 objects: **36 background colors, each paired with 3 text colors**.

| Theme | Backgrounds | Combinations |
|---|---|---|
| `dark` | 12 | 36 |
| `between` | 12 | 36 |
| `light` | 12 | 36 |

Backgrounds are muted and distinct from each other (no neon). Each text color is readable on its background (contrast of at least 4.5:1, with two documented exceptions down to 3.5:1) and the three text colors on one background are clearly different from each other. The file is indented with tabs and the keys and values are aligned in columns, so it reads well as plain text. It is ordinary valid JSON.

One entry looks like this (tabs shown as spaces):

```json
{
    "id":               29456,
    "Theme":            "dark",
    "Luminosity":       407,
    "Background.rgb":   "#000000",
    "Background.ansi":  16,
    "Text.rgb":         "#7ddcc4",
    "Text.ansi":        115,
    "Pedantic":         "Aquamarine on Black",
    "Funny":            "Minty Fresh Void",
    "Outrageous":       "Mint Tsunami Devours Eternity",
    "Vulgar":           "Minty Motherfucking Void",
    "Corporate":        "Mint Value-Add",
    "Geeky":            "Null Pointer Mint",
    "Wholesome":        "Fresh Mint Hug",
    "Sports":           "Jags Teal Blackout"
}
```

## Fields

| Field | Type | Meaning |
|---|---|---|
| `id` | integer | `Text.ansi * 256 + Background.ansi`. Packs the two xterm indexes into one number: text in the high byte, background in the low byte. Decode with `text = id >> 8` and `background = id & 255`. **Not unique:** different RGB colors can snap to the same xterm index, so the 108 entries give 103 distinct ids. If you need a unique key, use `Background.rgb` plus `Text.rgb`. |
| `Theme` | string | `"dark"`, `"between"` or `"light"`. It describes the **background** (dark backgrounds, mid-tone backgrounds, light backgrounds). 36 entries each. |
| `Luminosity` | integer, 0–999 | Overall perceived brightness of the pairing. It is the average of the background's and the text's CIELAB lightness (L\*), scaled so black on black is 0 and white on white is 999: `round(999 * (L*_bg + L*_text) / 200)`. In this data it runs from 350 to 718. It measures brightness, **not** readability. Use it to sort. |
| `Background.rgb` | string | Background color as sRGB hex, `#rrggbb`, lowercase. |
| `Background.ansi` | integer, 16–255 | Index of the nearest xterm-256 color to `Background.rgb`. |
| `Text.rgb` | string | Text (foreground) color as sRGB hex, `#rrggbb`, lowercase. |
| `Text.ansi` | integer, 16–255 | Index of the nearest xterm-256 color to `Text.rgb`. |
| `Pedantic` | string | Literal description: `<text color> on <background color>`, e.g. "Aquamarine on Black". Each color has a single fixed name everywhere in the file. |
| `Funny` | string | A pun or joke name. |
| `Outrageous` | string | An over-the-top, dramatic name. |
| `Vulgar` | string | A deliberately profane name. Leave this field out of anything user-facing where profanity is not wanted. |
| `Corporate` | string | Office-speak and business jargon. |
| `Geeky` | string | Programmer and sysadmin references. |
| `Wholesome` | string | Warm, cozy and encouraging. |
| `Sports` | string | Football, basketball, hockey and soccer team names that echo the colors or sound like them. |

Notes that apply to all of the above:

- **Names:** all 864 names (108 × 8) are unique across the whole file, and each is at most 32 characters.
- **ANSI indexes:** they come from a nearest-color search (CIELAB distance) over xterm indexes 16–255 only. Indexes 0–15 are left out on purpose, because terminal themes redefine them. The match is approximate, so the indexed color can look a little different from the `.rgb` value, particularly for dark, muted colors.
- **Ordering:** entries are grouped by theme, then by background, then by text color contrast, highest first.

## How to use it

### Pick a combination and apply it

Terminals that support 256 colors use these escape sequences:

```text
ESC[38;5;<text.ansi>m        set text color
ESC[48;5;<background.ansi>m  set background color
ESC[0m                       reset
```

For example, for the entry above:

```sh
printf '\033[38;5;115;48;5;16m Minty Fresh Void \033[0m\n'
```

For exact colors on terminals with true color support, use the RGB values instead (`#7ddcc4` is 125, 220, 196 and `#000000` is 0, 0, 0):

```sh
printf '\033[38;2;125;220;196;48;2;0;0;0m Minty Fresh Void \033[0m\n'
```

To recolor a whole terminal window (supported by many terminal emulators, e.g. xterm, iTerm2, Windows Terminal), set the default background and foreground with OSC 11 and OSC 10:

```sh
printf '\033]11;#000000\007\033]10;#7ddcc4\007'
```

### Read it in Go

```go
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

type Combo struct {
	ID            int    `json:"id"`
	Theme         string `json:"Theme"`
	Luminosity    int    `json:"Luminosity"`
	BackgroundRGB string `json:"Background.rgb"`
	BackgroundANSI int   `json:"Background.ansi"`
	TextRGB       string `json:"Text.rgb"`
	TextANSI      int    `json:"Text.ansi"`
	Pedantic      string `json:"Pedantic"`
	Funny         string `json:"Funny"`
	Outrageous    string `json:"Outrageous"`
	Vulgar        string `json:"Vulgar"`
	Corporate     string `json:"Corporate"`
	Geeky         string `json:"Geeky"`
	Wholesome     string `json:"Wholesome"`
	Sports        string `json:"Sports"`
}

func main() {
	data, err := os.ReadFile("terminal-color-combinations.json")
	if err != nil {
		panic(err)
	}
	var all []Combo
	if err := json.Unmarshal(data, &all); err != nil {
		panic(err)
	}

	// Dark combinations, dimmest first.
	var dark []Combo
	for _, c := range all {
		if c.Theme == "dark" {
			dark = append(dark, c)
		}
	}
	sort.Slice(dark, func(i, j int) bool { return dark[i].Luminosity < dark[j].Luminosity })

	for _, c := range dark {
		fmt.Printf("\x1b[38;5;%d;48;5;%dm %-32s \x1b[0m %s on %s\n",
			c.TextANSI, c.BackgroundANSI, c.Funny, c.TextRGB, c.BackgroundRGB)
	}
}
```

### Common ways to use the data

- **Offer a theme picker.** Filter by `Theme`, let the user choose one of the 36 combinations, and show it under the naming style they like (`Funny`, `Sports`, and so on).
- **Sort by brightness.** `Luminosity` gives a single number to order combinations from dimmest to brightest.
- **Support both color depths.** Use `Background.ansi` and `Text.ansi` on 256-color terminals and `Background.rgb` and `Text.rgb` on true-color terminals.
- **Store a choice compactly.** Save `id`, then recover the two indexes with `id >> 8` and `id & 255`. Because `id` can repeat (see above), save the two RGB values instead if you must tell every entry apart.
- **Show a randomly named combination** as a startup banner or a "color of the day", picking the name field that suits the audience.

### Things to be aware of

- The `Vulgar` names are profane on purpose.
- `Luminosity` is a plain average of two lightness values, so a dark-on-light pairing and its reverse get the same number.
- Contrast and distinctness were computed on the RGB values. After snapping to xterm indexes they can shift slightly, and two different backgrounds can end up on the same index.

## Links

- Preview (browse the combinations by theme and naming style): <https://color.nontechno.com/>
- Sources (data, viewer and notes): <https://github.com/nontechno/color>
