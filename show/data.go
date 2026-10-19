package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Themes and lingos (naming styles), in display order.
var (
	themes      = []string{"dark", "between", "light"}
	themeLabels = map[string]string{"dark": "Dark", "between": "In between", "light": "Light"}
	lingos      = []string{"Pedantic", "Funny", "Outrageous", "Vulgar", "Corporate", "Geeky", "Wholesome", "Sports"}
	colorModes  = []string{"ansi", "rgb"}
	modeLabels  = map[string]string{"rgb": "RGB (truecolor)", "ansi": "ANSI 256"}
)

// Entry is one validated background + text combination.
type Entry struct {
	ID     int
	Theme  string
	Lum    int
	BG, FG string // "#rrggbb", lowercase
	BGAnsi int
	FGAnsi int
	Names  map[string]string
}

var hexRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func asInt(v any, lo, hi int) (int, bool) {
	f, ok := v.(float64)
	if !ok || f != math.Trunc(f) || f < float64(lo) || f > float64(hi) {
		return 0, false
	}
	return int(f), true
}

// parseEntries parses and validates the JSON document. Invalid entries are skipped and
// described in issues. err is set only when the document as a whole is unusable.
func parseEntries(data []byte) (entries []Entry, issues []string, err error) {
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, nil, fmt.Errorf("not valid JSON: %w", err)
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil, nil, fmt.Errorf("top level must be a JSON array")
	}
	for i, item := range arr {
		obj, ok := item.(map[string]any)
		if !ok {
			issues = append(issues, fmt.Sprintf("Entry %d: not an object", i))
			continue
		}
		var p []string
		e := Entry{Names: map[string]string{}}

		th, _ := obj["Theme"].(string)
		if !contains(themes, th) {
			p = append(p, "Theme must be one of "+strings.Join(themes, ", "))
		}
		e.Theme = th

		var okL bool
		if e.Lum, okL = asInt(obj["Luminosity"], 0, 999); !okL {
			p = append(p, "Luminosity must be an integer 0-999")
		}
		for _, k := range []string{"Background", "Text"} {
			h, _ := obj[k+".rgb"].(string)
			a, okA := asInt(obj[k+".ansi"], 0, 255)
			if !hexRe.MatchString(h) {
				p = append(p, k+".rgb must look like #rrggbb")
			}
			if !okA {
				p = append(p, k+".ansi must be an integer 0-255")
			}
			if k == "Background" {
				e.BG, e.BGAnsi = strings.ToLower(h), a
			} else {
				e.FG, e.FGAnsi = strings.ToLower(h), a
			}
		}
		for _, k := range lingos {
			s, _ := obj[k].(string)
			if strings.TrimSpace(s) == "" {
				p = append(p, k+" must be a non-empty string")
			}
			e.Names[k] = s
		}
		id, okID := asInt(obj["id"], 0, 65535)
		switch {
		case !okID:
			p = append(p, "id must be an integer 0-65535")
		case id != e.FGAnsi*256+e.BGAnsi:
			p = append(p, fmt.Sprintf("id %d != Text.ansi*256+Background.ansi (%d)", id, e.FGAnsi*256+e.BGAnsi))
		}
		e.ID = id
		if len(p) > 0 {
			issues = append(issues, fmt.Sprintf("Entry %d: %s", i, strings.Join(p, "; ")))
			continue
		}
		entries = append(entries, e)
	}
	return entries, issues, nil
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// ---------- download, cache ----------

const defaultURL = "https://color.nontechno.com/terminal-color-combinations.json"

func cachePath() string {
	d, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(d, "nontechno-color", "terminal-color-combinations.json")
}

func download(url string, timeout time.Duration) ([]byte, error) {
	c := &http.Client{Timeout: timeout}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "nontechno-color-show")
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %s", resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 16<<20))
}

// Loaded is the result of loading the palette.
type Loaded struct {
	Entries []Entry
	Issues  []string
	Source  string // human-readable origin
	Warning string // e.g. "download failed, using cached copy"
}

// loadPalette reads the palette from a local file, or downloads it (falling back to the
// last good download kept in the user cache directory).
func loadPalette(url, file string, timeout time.Duration) (*Loaded, error) {
	if file != "" {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		ents, iss, err := parseEntries(data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", file, err)
		}
		return &Loaded{Entries: ents, Issues: iss, Source: file}, nil
	}

	data, dlErr := download(url, timeout)
	if dlErr == nil {
		ents, iss, err := parseEntries(data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", url, err)
		}
		if p := cachePath(); p != "" && len(ents) > 0 {
			if os.MkdirAll(filepath.Dir(p), 0o755) == nil {
				_ = os.WriteFile(p, data, 0o644)
			}
		}
		return &Loaded{Entries: ents, Issues: iss, Source: url}, nil
	}

	if p := cachePath(); p != "" {
		if cached, err := os.ReadFile(p); err == nil {
			if ents, iss, err := parseEntries(cached); err == nil && len(ents) > 0 {
				return &Loaded{Entries: ents, Issues: iss, Source: p,
					Warning: "download failed (" + dlErr.Error() + "); showing the last downloaded copy"}, nil
			}
		}
	}
	return nil, fmt.Errorf("could not download %s: %w", url, dlErr)
}

// ---------- colors ----------

var cube = [6]int{0, 95, 135, 175, 215, 255}

// xtermRGB returns the RGB of xterm-256 index i (16..255).
func xtermRGB(i int) (r, g, b int) {
	switch {
	case i >= 232:
		v := 8 + 10*(i-232)
		return v, v, v
	case i >= 16:
		n := i - 16
		return cube[n/36], cube[(n/6)%6], cube[n%6]
	}
	return 0, 0, 0
}

func xtermHex(i int) string {
	r, g, b := xtermRGB(i)
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

func hexRGB(h string) (r, g, b int) {
	fmt.Sscanf(h, "#%02x%02x%02x", &r, &g, &b)
	return
}

// nearestAnsi returns the xterm-256 index (16..255) closest to hex in RGB space.
func nearestAnsi(h string) int {
	r, g, b := hexRGB(h)
	best, bd := 16, math.MaxInt
	for i := 16; i < 256; i++ {
		xr, xg, xb := xtermRGB(i)
		d := (r-xr)*(r-xr) + (g-xg)*(g-xg) + (b-xb)*(b-xb)
		if d < bd {
			best, bd = i, d
		}
	}
	return best
}

func lin(c int) float64 {
	v := float64(c) / 255
	if v <= 0.04045 {
		return v / 12.92
	}
	return math.Pow((v+0.055)/1.055, 2.4)
}

func relLum(h string) float64 {
	r, g, b := hexRGB(h)
	return 0.2126*lin(r) + 0.7152*lin(g) + 0.0722*lin(b)
}

func contrast(a, b string) float64 {
	x, y := relLum(a), relLum(b)
	if x < y {
		x, y = y, x
	}
	return (x + 0.05) / (y + 0.05)
}

// ---------- grouping ----------

// Group is one background with its combinations.
type Group struct {
	BG     string
	BGAnsi int
	Items  []Entry
}

// groupsFor returns the groups of a theme, ordered by luminosity ascending: groups by their
// lowest-luminosity combination (ties: darker background first), rows by luminosity (ties: id).
func groupsFor(entries []Entry, theme string) []Group {
	idx := map[string]int{}
	var gs []Group
	for _, e := range entries {
		if e.Theme != theme {
			continue
		}
		i, ok := idx[e.BG]
		if !ok {
			i = len(gs)
			idx[e.BG] = i
			gs = append(gs, Group{BG: e.BG, BGAnsi: e.BGAnsi})
		}
		gs[i].Items = append(gs[i].Items, e)
	}
	minLum := func(g Group) int {
		m := 1 << 30
		for _, e := range g.Items {
			if e.Lum < m {
				m = e.Lum
			}
		}
		return m
	}
	for i := range gs {
		items := gs[i].Items
		sort.SliceStable(items, func(a, b int) bool {
			if items[a].Lum != items[b].Lum {
				return items[a].Lum < items[b].Lum
			}
			return items[a].ID < items[b].ID
		})
	}
	sort.SliceStable(gs, func(a, b int) bool {
		ma, mb := minLum(gs[a]), minLum(gs[b])
		if ma != mb {
			return ma < mb
		}
		return relLum(gs[a].BG) < relLum(gs[b].BG)
	})
	return gs
}
