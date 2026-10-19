package main

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Color depth levels.
const (
	depthNone  = 0 // no colors at all (NO_COLOR, dumb terminal, pipe)
	depthBasic = 1 // 8/16 colors
	depth256   = 2 // xterm-256
	depthTrue  = 3 // 24-bit
)

var (
	depth     = depth256
	depthNote = ""
)

// envDepth guesses the color depth from the environment only.
func envDepth() (int, string) {
	term := os.Getenv("TERM")
	ct := strings.ToLower(os.Getenv("COLORTERM"))
	switch {
	case os.Getenv("NO_COLOR") != "":
		return depthNone, "NO_COLOR is set"
	case term == "dumb":
		return depthNone, "TERM=dumb"
	case ct == "truecolor" || ct == "24bit":
		return depthTrue, "COLORTERM=" + ct
	case os.Getenv("WT_SESSION") != "":
		return depthTrue, "Windows Terminal"
	case strings.Contains(term, "256"):
		return depth256, "TERM=" + term
	case term == "linux" || term == "ansi" || strings.HasPrefix(term, "vt") || term == "cons25":
		return depthBasic, "TERM=" + term
	case term == "" && runtime.GOOS == "windows":
		return depth256, "Windows console"
	}
	return depth256, "assumed (TERM=" + term + ")"
}

// Probe is what the terminal told us about itself.
type Probe struct {
	Answered  bool   // the terminal replied to at least the DA1 sentinel
	Truecolor bool   // it kept a 24-bit SGR background (DECRQSS)
	OSC       bool   // it answered the OSC 10/11 color query
	FG, BG    string // current default text/background "#rrggbb", when OSC answered
}

var oscRe = regexp.MustCompile(`\x1b\]1([01]);rgb:([0-9a-fA-F]{1,4})/([0-9a-fA-F]{1,4})/([0-9a-fA-F]{1,4})(?:\x07|\x1b\\)`)

// scale16 converts a 1-4 digit hex component to 0-255.
func scale16(s string) int {
	v, _ := strconv.ParseUint(s, 16, 32)
	m := uint64(1)<<(4*uint(len(s))) - 1
	return int((v*255 + m/2) / m)
}

// parseProbe interprets the bytes the terminal sent back.
func parseProbe(acc []byte, answered bool) Probe {
	p := Probe{Answered: answered || len(acc) > 0}
	p.Truecolor = bytes.Contains(acc, []byte("1:2:3")) || bytes.Contains(acc, []byte("1;2;3"))
	for _, m := range oscRe.FindAllSubmatch(acc, -1) {
		hex := fmt.Sprintf("#%02x%02x%02x", scale16(string(m[2])), scale16(string(m[3])), scale16(string(m[4])))
		if string(m[1]) == "0" {
			p.FG = hex
		} else {
			p.BG = hex
		}
		p.OSC = true
	}
	return p
}

// probeTerminal asks the terminal what it can do. It sets a 24-bit background and queries the
// SGR state (DECRQSS), asks for the default text and background colors (OSC 10/11), and sends
// DA1 last as a sentinel, so a silent terminal costs at most the timeout. Replies arrive on chunks.
func probeTerminal(chunks <-chan []byte, timeout time.Duration) Probe {
	os.Stdout.WriteString("\x1b[48;2;1;2;3m\x1bP$qm\x1b\\\x1b[0m\x1b]10;?\x1b\\\x1b]11;?\x1b\\\x1b[c")
	var acc []byte
	timer := time.After(timeout)
	for {
		select {
		case c, ok := <-chunks:
			if !ok {
				return parseProbe(acc, false)
			}
			acc = append(acc, c...)
			if i := bytes.LastIndex(acc, []byte("\x1b[?")); i >= 0 && bytes.IndexByte(acc[i:], 'c') >= 0 {
				return parseProbe(acc, true)
			}
		case <-timer:
			return parseProbe(acc, false)
		}
	}
}

// Recolor sequences (OSC 10 = default text, OSC 11 = default background; 110/111 reset them).
func recolorSeq(bg, fg string) string {
	return "\x1b]11;" + bg + "\x1b\\\x1b]10;" + fg + "\x1b\\"
}

const resetSeq = "\x1b]111\x1b\\\x1b]110\x1b\\"

// describeProbe turns the findings into a sentence for the screen.
func describeProbe(p Probe) string {
	tmux := ""
	if os.Getenv("TMUX") != "" {
		tmux = " (inside tmux)"
	}
	if p.OSC {
		cur := ""
		if p.FG != "" && p.BG != "" {
			cur = fmt.Sprintf("; current text %s, background %s", p.FG, p.BG)
		}
		return "Recolor: available" + tmux + " - the terminal answered the OSC 10/11 query" + cur + "."
	}
	if p.Answered {
		return "Recolor: not confirmed" + tmux + " - the terminal ignored the OSC 10/11 query. Applying will still be tried, but may do nothing."
	}
	return "Recolor: not confirmed" + tmux + " - the terminal did not reply to any query. Applying will still be tried, but may do nothing."
}

var basic16 = [16][3]int{
	{0, 0, 0}, {205, 0, 0}, {0, 205, 0}, {205, 205, 0}, {0, 0, 238}, {205, 0, 205}, {0, 205, 205}, {229, 229, 229},
	{127, 127, 127}, {255, 0, 0}, {0, 255, 0}, {255, 255, 0}, {92, 92, 255}, {255, 0, 255}, {0, 255, 255}, {255, 255, 255},
}

func nearest16(h string) int {
	r, g, b := hexRGB(h)
	best, bd := 0, 1<<30
	for i, c := range basic16 {
		d := (r-c[0])*(r-c[0]) + (g-c[1])*(g-c[1]) + (b-c[2])*(b-c[2])
		if d < bd {
			best, bd = i, d
		}
	}
	return best
}

// colorSeq returns the SGR sequence that sets a foreground/background color. hex is the exact
// color, idx its xterm-256 index; the exact color is used only when truecolor is allowed and
// the terminal supports it, otherwise the ANSI index (reduced to 16 colors on basic terminals).
func colorSeq(fg bool, hex string, idx int, truecolor bool) string {
	if depth == depthNone {
		return ""
	}
	base := 38
	if !fg {
		base = 48
	}
	switch {
	case truecolor: // RGB mode was chosen: send 24-bit colors (ESC[48;2;r;g;bm) as the -table test does
		r, g, b := hexRGB(hex)
		return fmt.Sprintf("\x1b[%d;2;%d;%d;%dm", base, r, g, b)
	case depth >= depth256:
		return fmt.Sprintf("\x1b[%d;5;%dm", base, idx)
	}
	n := nearest16(xtermHex(idx))
	if n < 8 {
		return fmt.Sprintf("\x1b[%dm", base-8+n)
	}
	return fmt.Sprintf("\x1b[%dm", base+52+n-8)
}

func sgr(codes string) string {
	if depth == depthNone {
		return ""
	}
	return "\x1b[" + codes + "m"
}

// readbackQuery asks the terminal for its current default colors, followed by DA1 as a sentinel.
const readbackQuery = "\x1b]11;?\x1b\\\x1b]10;?\x1b\\\x1b[c"

// verifyResult compares what the terminal reports after a recolor with what was set.
func verifyResult(p Probe, bg, fg string) (ok bool, msg string) {
	if !p.OSC {
		return false, "the terminal did not report its colors back, so this cannot be verified"
	}
	if p.BG == bg && p.FG == fg {
		return true, fmt.Sprintf("verified: the terminal reports background %s and text %s", p.BG, p.FG)
	}
	return false, fmt.Sprintf("the terminal reports background %s and text %s (set %s / %s): it accepted the command but rounds or ignores the colors", p.BG, p.FG, bg, fg)
}

// testRecolor sets a test pair, reads it back, restores the defaults and returns the verdict.
func testRecolor(chunks <-chan []byte, timeout time.Duration) (bool, string) {
	const bg, fg = "#204060", "#f0e0c0"
	os.Stdout.WriteString(recolorSeq(bg, fg) + readbackQuery)
	var acc []byte
	timer := time.After(timeout)
loop:
	for {
		select {
		case b, ok := <-chunks:
			if !ok {
				break loop
			}
			acc = append(acc, b...)
			if i := bytes.LastIndex(acc, []byte("\x1b[?")); i >= 0 && bytes.IndexByte(acc[i:], 'c') >= 0 {
				break loop
			}
		case <-timer:
			break loop
		}
	}
	os.Stdout.WriteString(resetSeq)
	return verifyResult(parseProbe(acc, false), bg, fg)
}
