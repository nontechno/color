package main

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const gap = 2

// app is the whole UI state.
type app struct {
	data    *Loaded
	err     error
	loading bool
	frame   int

	theme, lingo int
	mode         string // "ansi" or "rgb"
	focus        int    // 0 theme, 1 lingo, 2 colors
	scroll       int
	w, h         int
	notice       string
	canRGB       bool
	termInfo     string
	source       string

	// terminal findings and the "recolor the whole terminal" feature
	probe                          Probe
	oscInfo                        string
	selCard, selRow                int // highlighted combination (focus on Cards)
	cols                           int
	groups                         []Group
	selLine                        int
	follow                         bool
	appliedBG, appliedFG, appliedN string
	out                            string // escape sequences to send before the next frame
	verifyBG, verifyFG             string // pending read-back check of an applied recolor
	vbuf                           []byte
}

func rlen(s string) int { return utf8.RuneCountInString(s) }

func padRight(s string, n int) string {
	if d := n - rlen(s); d > 0 {
		return s + strings.Repeat(" ", d)
	}
	return s
}

// truncate shortens s to at most n columns, ending in "..." when it was cut.
func truncate(s string, n int) string {
	if rlen(s) <= n {
		return s
	}
	if n <= 3 {
		return strings.Repeat(".", max(n, 0))
	}
	r := []rune(s)
	return string(r[:n-3]) + "..."
}

// nameWidth measures the widest name over every lingo.
func nameWidth(entries []Entry) (w int) {
	for _, e := range entries {
		for _, l := range lingos {
			w = max(w, rlen(e.Names[l]))
		}
	}
	return
}

func (a *app) truecolor() bool { return a.mode == "rgb" }

// cardLines renders one background card: a header and one row per text color.
func (a *app) cardLines(g Group, nameW, cardW, sel int) []string {
	rst := sgr("0")
	bgHex := g.BG
	if a.mode == "ansi" {
		bgHex = xtermHex(g.BGAnsi)
	}
	hdr := sgr("0") + colorSeq(false, "#f3e9d6", 230, a.truecolor()) + colorSeq(true, "#5c4026", 94, a.truecolor()) + sgr("1")
	lines := []string{
		hdr + " " + colorSeq(false, bgHex, g.BGAnsi, a.truecolor()) + "  " + hdr + " " +
			padRight(truncate("Background", cardW-4), cardW-4) + rst,
	}
	for ri, e := range g.Items {
		lead, trail := " ", " "
		if ri == sel {
			lead = ">"
		}
		if a.appliedBG != "" && e.BG == a.appliedBG && e.FG == a.appliedFG {
			trail = "*"
		}
		name := truncate(e.Names[lingos[a.lingo]], nameW)
		row := colorSeq(true, e.FG, e.FGAnsi, a.truecolor()) + colorSeq(false, e.BG, e.BGAnsi, a.truecolor()) +
			lead + padRight(name, nameW) + trail + rst
		lines = append(lines, row)
	}
	return lines
}

// body lays the cards of the selected theme out in as many columns as fit.
func (a *app) body(width int) []string {
	entries := a.data.Entries
	nameW := nameWidth(entries)
	avail := width - 1
	if full := 1 + nameW + 1; full > avail {
		nameW = max(12, nameW-(full-avail))
	}
	cardW := 1 + nameW + 1
	cols := max(1, (avail+gap)/(cardW+gap))

	groups := groupsFor(entries, themes[a.theme])
	a.groups, a.cols = groups, cols
	if a.selCard >= len(groups) {
		a.selCard = 0
	}
	if a.selRow >= len(groups[a.selCard].Items) {
		a.selRow = 0
	}
	var out []string
	for i := 0; i < len(groups); i += cols {
		end := min(i+cols, len(groups))
		var cards [][]string
		rows := 0
		for _, g := range groups[i:end] {
			sel := -1
			if gi := i + len(cards); gi == a.selCard {
				a.selLine = len(out) + 1 + a.selRow
				if a.focus == 3 {
					sel = a.selRow
				}
			}
			c := a.cardLines(g, nameW, cardW, sel)
			cards = append(cards, c)
			rows = max(rows, len(c))
		}
		for r := 0; r < rows; r++ {
			var sb strings.Builder
			sb.WriteString(" ")
			for k, c := range cards {
				if k > 0 {
					sb.WriteString(strings.Repeat(" ", gap))
				}
				if r < len(c) {
					sb.WriteString(c[r])
				} else {
					sb.WriteString(strings.Repeat(" ", cardW))
				}
			}
			out = append(out, sb.String())
		}
		out = append(out, "")
	}
	return out
}

// radio renders one selector row, wrapping the options to the width.
func radio(label string, opts []string, sel int, focused bool, width int) []string {
	rst := sgr("0")
	lab := padRight(label, 7)
	if focused {
		lab = sgr("7") + lab + rst
	}
	prefix := "  "
	if focused {
		prefix = sgr("1") + "> " + rst
	}
	indent := strings.Repeat(" ", 2+7+1)
	var lines []string
	cur := prefix + lab + " "
	curW := 2 + 7 + 1
	for i, o := range opts {
		mark := "[ ] "
		item := mark + o
		styled := item
		if i == sel {
			styled = sgr("1") + "[*] " + o + rst
		}
		w := rlen(item) + 2
		if curW+w > width && curW > len(indent) {
			lines = append(lines, cur)
			cur, curW = indent, len(indent)
		}
		cur += styled + "  "
		curW += w
	}
	return append(lines, cur)
}

// view builds exactly a.h lines for the interactive screen.
func (a *app) view() []string {
	w, h := a.w, a.h
	rst := sgr("0")
	var head []string
	title := truncate("Terminal color combinations", w-1)
	head = append(head, " "+sgr("1")+title+rst, "")

	var foot []string
	if a.data == nil {
		var lines []string
		switch {
		case a.loading:
			spin := []string{"|", "/", "-", "\\"}[a.frame%4]
			lines = []string{"  " + spin + " Downloading " + truncate(a.source, w-18)}
		case a.err != nil:
			lines = []string{"  Could not load the palette:", ""}
			for _, l := range wrap(a.err.Error(), w-6) {
				lines = append(lines, "    "+l)
			}
			lines = append(lines, "", "  Press r to try again, q to quit.", "  (Use -file <path> to read a local copy of the JSON.)")
		}
		head = append(head, a.findings(w)...)
		return fit(append(append(head, ""), lines...), nil, w, h)
	}

	themeOpts := make([]string, len(themes))
	for i, t := range themes {
		themeOpts[i] = themeLabels[t]
	}
	modeOpts := make([]string, len(colorModes))
	for i, m := range colorModes {
		modeOpts[i] = modeLabels[m]
	}
	head = append(head, radio("Theme", themeOpts, a.theme, a.focus == 0, w)...)
	head = append(head, radio("Lingo", lingos, a.lingo, a.focus == 1, w)...)
	modeSel := 0
	if a.mode == "rgb" {
		modeSel = 1
	}
	head = append(head, radio("Colors", modeOpts, modeSel, a.focus == 2, w)...)
	cardsLab, cardsMark := padRight("Cards", 7), "  "
	if a.focus == 3 {
		cardsLab, cardsMark = sgr("7")+cardsLab+rst, sgr("1")+"> "+rst
	}
	head = append(head, cardsMark+cardsLab+" "+truncate("pick a combination with the arrow keys; Enter (or a) recolors the terminal, x resets", max(0, w-11)))
	head = append(head, a.findings(w)...)
	n := 0
	for _, e := range a.data.Entries {
		if e.Theme == themes[a.theme] {
			n++
		}
	}
	head = append(head, "", " "+sgr("2")+truncate(fmt.Sprintf("%s · %d combinations · %s", themeLabels[themes[a.theme]], n, a.data.Source), w-2)+rst, "")

	status := a.notice
	if status == "" {
		status = a.termInfo
	}
	if a.data.Warning != "" {
		status = a.data.Warning
	}
	if a.notice == "" && a.appliedBG != "" {
		status = fmt.Sprintf("Terminal recolored: %s (text %s on %s). Press x to reset.", a.appliedN, a.appliedFG, a.appliedBG)
	}
	if len(a.data.Issues) > 0 {
		status = fmt.Sprintf("%d invalid entries skipped: %s", len(a.data.Issues), a.data.Issues[0])
	}
	foot = append(foot,
		" "+sgr("2")+truncate(status, w-2)+rst,
		" "+sgr("2")+truncate("←/→ change · Tab next row · t l c · ↑/↓ PgUp/PgDn scroll · a apply · x reset · r reload · q quit", w-2)+rst)

	body := a.body(w)
	view := h - len(head) - len(foot)
	if view < 1 {
		view = 1
	}
	if a.follow {
		top := max(0, a.selLine-a.selRow-1)
		if top < a.scroll {
			a.scroll = top
		}
		if a.selLine >= a.scroll+view {
			a.scroll = a.selLine - view + 1
		}
		a.follow = false
	}
	maxScroll := max(0, len(body)-view)
	a.scroll = min(max(a.scroll, 0), maxScroll)
	body = body[a.scroll:min(len(body), a.scroll+view)]
	return fit(append(head, body...), foot, w, h)
}

// fit pads/clips content so that content + foot is exactly h lines (foot pinned to the bottom).
func fit(content, foot []string, w, h int) []string {
	room := h - len(foot)
	if room < 0 {
		room = 0
	}
	if len(content) > room {
		content = content[:room]
	}
	for len(content) < room {
		content = append(content, "")
	}
	return append(content, foot...)
}

func wrap(s string, width int) []string {
	if width < 10 {
		width = 10
	}
	var out []string
	line := ""
	for _, w := range strings.Fields(s) {
		for rlen(w) > width {
			if line != "" {
				out = append(out, line)
				line = ""
			}
			out = append(out, string([]rune(w)[:width]))
			w = string([]rune(w)[width:])
		}
		if line != "" && rlen(line)+1+rlen(w) > width {
			out = append(out, line)
			line = ""
		}
		if line != "" {
			line += " "
		}
		line += w
	}
	if line != "" {
		out = append(out, line)
	}
	return out
}

// handle applies a key; it reports whether to quit or reload.
func (a *app) handle(k KeyEvent) (quit, reload bool) {
	a.notice = ""
	if k.Key == KeyCtrlC || k.Key == KeyEsc || (k.Key == KeyRune && (k.R == 'q' || k.R == 'Q')) {
		return true, false
	}
	if k.Key == KeyRune && (k.R == 'r' || k.R == 'R') {
		return false, true
	}
	if a.data == nil {
		return
	}
	step := func(delta int) {
		switch a.focus {
		case 0:
			a.theme = (a.theme + delta + len(themes)) % len(themes)
			a.scroll, a.selCard, a.selRow = 0, 0, 0
		case 1:
			a.lingo = (a.lingo + delta + len(lingos)) % len(lingos)
		case 2:
			a.toggleMode()
		}
	}
	cards := a.focus == 3
	switch k.Key {
	case KeyEnter:
		if cards {
			a.apply()
		} else {
			a.focus = (a.focus + 1) % 4
		}
	case KeyTab:
		a.focus = (a.focus + 1) % 4
	case KeyShiftTab:
		a.focus = (a.focus + 3) % 4
	case KeyLeft:
		if cards {
			a.moveSel(-1, 0)
		} else {
			step(-1)
		}
	case KeyRight:
		if cards {
			a.moveSel(1, 0)
		} else {
			step(1)
		}
	case KeyUp:
		if cards {
			a.moveSel(0, -1)
		} else {
			a.scroll--
		}
	case KeyDown:
		if cards {
			a.moveSel(0, 1)
		} else {
			a.scroll++
		}
	case KeyPgUp:
		a.scroll -= max(1, a.h-8)
	case KeyPgDn:
		a.scroll += max(1, a.h-8)
	case KeyHome:
		a.scroll = 0
	case KeyEnd:
		a.scroll = 1 << 20
	case KeyRune:
		switch k.R {
		case 't', 'T':
			a.focus = 0
			step(1)
		case 'l', 'L':
			a.focus = 1
			step(1)
		case 'c', 'C':
			a.focus = 2
			step(1)
		case '1', '2', '3':
			a.focus = 0
			a.theme = int(k.R - '1')
			a.scroll, a.selCard, a.selRow = 0, 0, 0
		case 'a', 'A':
			a.apply()
		case 'x', 'X':
			a.reset()
		case 'j':
			a.scroll++
		case 'k':
			a.scroll--
		case ' ':
			a.scroll += max(1, a.h-8)
		case 'b':
			a.scroll -= max(1, a.h-8)
		case 'g':
			a.scroll = 0
		case 'G':
			a.scroll = 1 << 20
		}
	}
	return
}

func (a *app) toggleMode() {
	if a.mode == "ansi" {
		a.mode = "rgb"
		if !a.canRGB {
			a.notice = "RGB sends 24-bit #rrggbb colors; this terminal did not confirm truecolor, so if the colors look wrong switch back to ANSI 256 (show -table tests it)"
		}
		return
	}
	a.mode = "ansi"
}

// moveSel moves the highlighted combination: dx between cards, dy between rows (and, past the
// first/last row, to the card above/below in the same column).
func (a *app) moveSel(dx, dy int) {
	n := len(a.groups)
	if n == 0 {
		return
	}
	a.follow = true
	if dx != 0 {
		a.selCard = min(max(a.selCard+dx, 0), n-1)
		a.selRow = min(a.selRow, len(a.groups[a.selCard].Items)-1)
		return
	}
	r := a.selRow + dy
	switch {
	case r < 0:
		if a.selCard-a.cols >= 0 {
			a.selCard -= a.cols
			a.selRow = len(a.groups[a.selCard].Items) - 1
		}
	case r >= len(a.groups[a.selCard].Items):
		if a.selCard+a.cols < n {
			a.selCard += a.cols
			a.selRow = 0
		}
	default:
		a.selRow = r
	}
}

// apply recolors the whole terminal with the highlighted combination, using its #rrggbb values.
func (a *app) apply() {
	if a.selCard >= len(a.groups) || a.selRow >= len(a.groups[a.selCard].Items) {
		return
	}
	e := a.groups[a.selCard].Items[a.selRow]
	a.out += recolorSeq(e.BG, e.FG)
	a.appliedBG, a.appliedFG, a.appliedN = e.BG, e.FG, e.Names[lingos[a.lingo]]
	a.notice = fmt.Sprintf("Applied %s: text %s on %s. Press x to reset.", a.appliedN, e.FG, e.BG)
	if a.probe.OSC {
		a.out += readbackQuery // read the colors back to verify the terminal took them
		a.verifyBG, a.verifyFG, a.vbuf = e.BG, e.FG, nil
	} else {
		a.notice += " (Not confirmed: if nothing changed, this terminal does not support OSC 10/11.)"
	}
}

// reset restores the terminal's own default colors.
func (a *app) reset() {
	if a.appliedBG == "" {
		a.notice = "Nothing to reset."
		return
	}
	a.out += resetSeq
	a.appliedBG, a.appliedFG, a.appliedN = "", "", ""
	a.notice = "Terminal colors restored."
}

// findings describes what the terminal reported about itself (shown on every screen).
func (a *app) findings(w int) []string {
	var out []string
	for _, l := range wrap(a.oscInfo, w-3) {
		out = append(out, "  "+sgr("2")+l+sgr("0"))
	}
	return out
}
