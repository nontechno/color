package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type settings struct {
	Theme  string `json:"theme"`
	Lingo  string `json:"lingo"`
	Colors string `json:"colors"`
}

func settingsPath() string {
	d, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(d, "nontechno-color", "show.json")
}

func loadSettings() (s settings) {
	if p := settingsPath(); p != "" {
		if b, err := os.ReadFile(p); err == nil {
			_ = json.Unmarshal(b, &s)
		}
	}
	return
}

func saveSettings(a *app) {
	p := settingsPath()
	if p == "" {
		return
	}
	b, _ := json.Marshal(settings{Theme: themes[a.theme], Lingo: lingos[a.lingo], Colors: a.mode})
	if os.MkdirAll(filepath.Dir(p), 0o755) == nil {
		_ = os.WriteFile(p, b, 0o644)
	}
}

func indexOf(list []string, s string) int {
	for i, x := range list {
		if strings.EqualFold(x, s) {
			return i
		}
	}
	return -1
}

type loadResult struct {
	l   *Loaded
	err error
}

func main() {
	url := flag.String("url", defaultURL, "address of the palette JSON")
	file := flag.String("file", "", "read the palette from a local file instead of downloading it")
	theme := flag.String("theme", "", "initial theme: dark, between or light")
	lingo := flag.String("lingo", "", "initial lingo: "+strings.Join(lingos, ", "))
	colors := flag.String("colors", "", "ansi (xterm-256 indexes, default) or rgb (truecolor terminals only)")
	print := flag.Bool("print", false, "print the cards once and exit (no interactive UI)")
	width := flag.Int("width", 0, "output width for -print (default: terminal width, or 100)")
	force := flag.Bool("color", false, "with -print: emit color escapes even when not writing to a terminal")
	timeout := flag.Duration("timeout", 15*time.Second, "download timeout")
	table := flag.Bool("table", false, "draw a table of the 256 grey shades #000000..#ffffff (truecolor test) and exit")
	info := flag.Bool("info", false, "report what this terminal supports (colors, truecolor, OSC 10/11 recoloring) and exit")
	flag.Parse()
	enableVT()
	if *table {
		w := *width
		if w <= 0 {
			w = 144
			if isTerminal(os.Stdout) {
				w, _ = termSize()
			}
		}
		for _, l := range greyTable(w) {
			fmt.Println(l)
		}
		os.Exit(0)
	}
	if *info {
		os.Exit(runInfo())
	}

	st := loadSettings()
	pick := func(flagVal, saved string, list []string, def int) int {
		for _, v := range []string{flagVal, saved} {
			if i := indexOf(list, v); i >= 0 {
				return i
			}
		}
		return def
	}
	a := &app{
		theme: pick(*theme, st.Theme, themes, 0),
		lingo: pick(*lingo, st.Lingo, lingos, 1),
		mode:  "ansi",
	}
	if *colors != "" {
		a.mode = strings.ToLower(*colors)
	} else if st.Colors == "rgb" {
		a.mode = "rgb"
	}
	if a.mode != "ansi" && a.mode != "rgb" {
		fmt.Fprintln(os.Stderr, "-colors must be ansi or rgb")
		os.Exit(2)
	}
	if *theme != "" && indexOf(themes, *theme) < 0 || *lingo != "" && indexOf(lingos, *lingo) < 0 {
		fmt.Fprintln(os.Stderr, "unknown -theme or -lingo")
		os.Exit(2)
	}
	a.source = *url
	if *file != "" {
		a.source = *file
	}

	if *print || !isTerminal(os.Stdin) || !isTerminal(os.Stdout) {
		os.Exit(runPrint(a, *url, *file, *timeout, *width, *force))
	}
	if err := runInteractive(a, *url, *file, *timeout); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	saveSettings(a)
}

func runPrint(a *app, url, file string, timeout time.Duration, width int, force bool) int {
	l, err := loadPalette(url, file, timeout)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	a.data = l
	d, note := envDepth()
	if !isTerminal(os.Stdout) && !force {
		d = depthNone
	}
	depth, depthNote = d, note
	a.canRGB = depth >= depthTrue
	if width <= 0 {
		width = 100
		if isTerminal(os.Stdout) {
			width, _ = termSize()
		}
	}
	fmt.Printf("%s · %s · %s\n\n", themeLabels[themes[a.theme]], lingos[a.lingo], modeLabels[a.mode])
	for _, line := range a.body(width) {
		fmt.Println(line)
	}
	for _, s := range l.Issues {
		fmt.Fprintln(os.Stderr, "skipped:", s)
	}
	if l.Warning != "" {
		fmt.Fprintln(os.Stderr, l.Warning)
	}
	return 0
}

// detect combines the environment with what the terminal says about itself.
func detect(chunks <-chan []byte) (int, string, Probe) {
	d, note := envDepth()
	if d == depthNone {
		return d, note, Probe{}
	}
	p := probeTerminal(chunks, 250*time.Millisecond)
	switch {
	case p.Truecolor:
		d, note = depthTrue, "confirmed by the terminal"
	case p.Answered && d == depthTrue:
		d, note = depth256, "terminal does not confirm truecolor"
	case p.Answered:
	}
	return d, note, p
}

// runInfo prints what the terminal reports about colors, then exits.
func runInfo() int {
	restore, err := makeRaw()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: -info needs an interactive terminal:", err)
		return 1
	}
	chunks := make(chan []byte, 16)
	go readChunks(chunks)
	d, note, p := detect(chunks)
	var testMsg string
	if d != depthNone {
		_, testMsg = testRecolor(chunks, 400*time.Millisecond)
	}
	restore() // back to cooked mode before printing normally
	names := map[int]string{depthNone: "none", depthBasic: "8/16 colors", depth256: "ANSI 256", depthTrue: "truecolor"}
	yes := func(b bool) string {
		if b {
			return "yes"
		}
		return "no"
	}
	fmt.Printf("TERM=%q COLORTERM=%q\n", os.Getenv("TERM"), os.Getenv("COLORTERM"))
	fmt.Printf("Color depth:       %s (%s)\n", names[d], note)
	fmt.Printf("Replies to probe:  %s\n", yes(p.Answered))
	fmt.Printf("Truecolor (SGR):   %s\n", yes(p.Truecolor))
	fmt.Printf("OSC 10/11 query:   %s", yes(p.OSC))
	if p.OSC {
		fmt.Printf(" (text %s, background %s)", p.FG, p.BG)
	}
	fmt.Println()
	fmt.Println(describeProbe(p))
	if testMsg != "" {
		fmt.Println()
		fmt.Println("Recolor test (sets background #204060 and text #f0e0c0 for a moment, then restores):")
		fmt.Println("  " + testMsg + ".")
		if !p.OSC {
			fmt.Println("  Run the app and press 'a' on a combination: if the window colors change, it works.")
		}
	}
	return 0
}

func runInteractive(a *app, url, file string, timeout time.Duration) error {
	restore, err := makeRaw()
	if err != nil {
		return err
	}
	defer restore()
	os.Stdout.WriteString("\x1b[?1049h\x1b[?25l\x1b[2J")
	defer func() {
		if a.appliedBG != "" {
			os.Stdout.WriteString(resetSeq) // never leave the terminal recolored
		}
		os.Stdout.WriteString("\x1b[0m\x1b[?25h\x1b[?1049l")
	}()

	chunks := make(chan []byte, 16)
	go readChunks(chunks)

	// Ask the terminal about its colors; fall back to what the environment says.
	d, note, probe := detect(chunks)
	a.probe = probe
	a.oscInfo = describeProbe(probe)
	depth = d
	switch depth {
	case depthNone:
		a.termInfo = "no color support detected (" + note + ")"
	case depthBasic:
		a.termInfo = "only 8/16 colors detected (" + note + "): ANSI indexes are approximated"
	case depth256:
		a.termInfo = "terminal colors: ANSI 256 · " + note
	default:
		a.termInfo = "terminal colors: truecolor + ANSI 256 · " + note
	}
	a.canRGB = depth >= depthTrue

	done := make(chan loadResult, 1)
	start := func() {
		a.loading, a.err = true, nil
		go func() {
			l, err := loadPalette(url, file, timeout)
			done <- loadResult{l, err}
		}()
	}
	start()

	tick := time.NewTicker(120 * time.Millisecond)
	defer tick.Stop()
	dirty := true
	for {
		if w, h := termSize(); w != a.w || h != a.h {
			a.w, a.h, dirty = w, h, true
			os.Stdout.WriteString("\x1b[2J")
		}
		if dirty {
			draw(a)
			dirty = false
		}
		select {
		case c, ok := <-chunks:
			if !ok {
				return nil
			}
			if a.verifyBG != "" {
				a.vbuf = append(a.vbuf, c...)
				if i := bytes.LastIndex(a.vbuf, []byte("\x1b[?")); i >= 0 && bytes.IndexByte(a.vbuf[i:], 'c') >= 0 {
					_, msg := verifyResult(parseProbe(a.vbuf, false), a.verifyBG, a.verifyFG)
					a.notice = "Applied " + a.appliedN + ": " + msg + ". Press x to reset."
					a.verifyBG, a.vbuf = "", nil
					dirty = true
				}
			}
			for _, k := range parseKeys(c) {
				quit, reload := a.handle(k)
				if quit {
					return nil
				}
				if reload && !a.loading {
					a.data = nil
					start()
				}
			}
			dirty = true
		case r := <-done:
			a.loading = false
			a.l(r)
			dirty = true
		case <-tick.C:
			if a.loading {
				a.frame++
				dirty = true
			}
		}
	}
}

func (a *app) l(r loadResult) {
	if r.err != nil {
		a.err = r.err
		return
	}
	a.data = r.l
}

func draw(a *app) {
	lines := a.view()
	var sb strings.Builder
	sb.WriteString(a.out)
	a.out = ""
	sb.WriteString("\x1b[H")
	for i, l := range lines {
		sb.WriteString(l)
		sb.WriteString("\x1b[0m\x1b[K")
		if i < len(lines)-1 {
			sb.WriteString("\r\n")
		}
	}
	os.Stdout.WriteString(sb.String())
}

// cellW is the width of one cell of the grey table: " #rrggbb ".
const cellW = 9

// greyTable draws the 256 grey shades #000000..#ffffff in 24-bit color as cells of cellW
// columns, each showing its own #rrggbb in the inverted color. Cells flow left to right,
// as many per row as fit in width (at most 16).
func greyTable(width int) []string {
	cols := 1
	for _, c := range []int{16, 8, 4, 2} { // keep the grid tidy: 16x16, 8x32, 4x64, ...
		if c*cellW <= width {
			cols = c
			break
		}
	}
	lines := []string{
		fmt.Sprintf("256 greys, #000000 to #ffffff, drawn with 24-bit color (ESC[48;2;r;g;bm); text is the inverted color."),
		"A smooth gradient means truecolor works; visible steps or odd colors mean it is rounded or unsupported.",
		"",
	}
	var sb strings.Builder
	for v := 0; v < 256; v++ {
		inv := 255 - v
		fmt.Fprintf(&sb, "\x1b[48;2;%d;%d;%dm\x1b[38;2;%d;%d;%dm #%02x%02x%02x \x1b[0m", v, v, v, inv, inv, inv, v, v, v)
		if (v+1)%cols == 0 || v == 255 {
			lines = append(lines, sb.String())
			sb.Reset()
		}
	}
	return lines
}
