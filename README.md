# Terminal Color Combinations

A ready-to-use collection of **108 background + text color combinations** for terminal sessions, shipped as a single JSON file: [`terminal-color-combinations.json`](https://color.nontechno.com/terminal-color-combinations.json).

**[Live preview &rarr;](https://color.nontechno.com/)**

## What it is for

Picking a terminal color scheme usually means choosing a background first and then hunting for text colors that look good and stay readable on it. This file does that work up front. It is meant for programs that need a good-looking, readable color pair, for example:

- a terminal multiplexer, shell prompt or TUI that lets users pick a theme,
- a tool that gives each session, pane or host its own distinguishable colors,
- a color picker or theme browser for terminal users,
- anything that wants a pool of tasteful, readable color pairs, with a name to show for each.

## What is in the file

- **36 backgrounds**, split evenly into `dark`, `between` and `light` themes (12 each). They are muted, with no neon, and distinct enough from one another that they don't look like the same color.
- **3 text colors per background**, each readable on it (contrast of at least 4.5:1, with two documented exceptions) and clearly different from the other two.
- **Both color systems:** every color is given as RGB hex and as the nearest **xterm-256 color index**, so it works on 256-color and true-color terminals alike.
- **A brightness number** (`Luminosity`, 0&ndash;999) for sorting.
- **Eight names per combination**, in eight styles: `Pedantic`, `Funny`, `Outrageous`, `Vulgar`, `Corporate`, `Geeky`, `Wholesome` and `Sports`. Every name is unique and at most 32 characters.

One entry:

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

> [!NOTE]
> The `Vulgar` names are profane on purpose. Leave that field out of anything where profanity isn't welcome.

## Quick start

Fetch the file and apply a combination with the standard 256-color escape sequences:

```sh
curl -sO https://color.nontechno.com/terminal-color-combinations.json
```

```sh
# text color 115 on background color 16
printf '\033[38;5;115;48;5;16m Minty Fresh Void \033[0m\n'
```

On a true-color terminal, use the RGB values instead:

```sh
printf '\033[38;2;125;220;196;48;2;0;0;0m Minty Fresh Void \033[0m\n'
```

In Go:

```go
var all []struct {
	Theme          string `json:"Theme"`
	BackgroundANSI int    `json:"Background.ansi"`
	TextANSI       int    `json:"Text.ansi"`
	Funny          string `json:"Funny"`
}
data, _ := os.ReadFile("terminal-color-combinations.json")
_ = json.Unmarshal(data, &all)

for _, c := range all {
	if c.Theme == "dark" {
		fmt.Printf("\x1b[38;5;%d;48;5;%dm %s \x1b[0m\n", c.TextANSI, c.BackgroundANSI, c.Funny)
	}
}
```

The [usage guide](https://github.com/nontechno/color/blob/main/usage.md) has every field explained, more examples and a list of things to watch for (for instance, `id` is not unique).

## Documentation

| | |
|---|---|
| [Preview](https://color.nontechno.com/) | Browse the combinations by theme and naming style |
| [Usage](https://github.com/nontechno/color/blob/main/usage.md) | The fields in detail and how to use the data |
| [Origins](https://github.com/nontechno/color/blob/main/origins.md) | Where the colors came from and the criteria behind them |
| [Source data](https://color.nontechno.com/terminal-color-combinations.json) | The JSON file itself |
| [Repository](https://github.com/nontechno/color) | The sources for the data and the preview page |
