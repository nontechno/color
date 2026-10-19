package main

import (
	"regexp"
	"strings"
	"testing"
)

var ansiRe = regexp.MustCompile("\x1b\\[[0-9;?]*[A-Za-z]")

func loadTest(t *testing.T) *Loaded {
	t.Helper()
	l, err := loadPalette("", "testdata/terminal-color-combinations.json", 0)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func TestParse(t *testing.T) {
	l := loadTest(t)
	if len(l.Entries) != 108 || len(l.Issues) != 0 {
		t.Fatalf("entries=%d issues=%v", len(l.Entries), l.Issues)
	}
	if _, iss, _ := parseEntries([]byte(`[{"Theme":"x"}]`)); len(iss) != 1 {
		t.Fatal("invalid entry should be reported")
	}
	if _, _, err := parseEntries([]byte(`{}`)); err == nil {
		t.Fatal("non-array should fail")
	}
}

func TestPalette(t *testing.T) {
	for i, want := range map[int]string{16: "#000000", 231: "#ffffff", 232: "#080808", 255: "#eeeeee", 196: "#ff0000"} {
		if got := xtermHex(i); got != want {
			t.Errorf("xtermHex(%d)=%s want %s", i, got, want)
		}
	}
	if c := contrast("#000000", "#ffffff"); c < 20.9 || c > 21.1 {
		t.Errorf("contrast=%v", c)
	}
}

func TestGroups(t *testing.T) {
	l := loadTest(t)
	for _, th := range themes {
		gs := groupsFor(l.Entries, th)
		if len(gs) != 12 {
			t.Fatalf("%s: %d groups", th, len(gs))
		}
		for _, g := range gs {
			if len(g.Items) != 3 {
				t.Fatalf("%s %s: %d items", th, g.BG, len(g.Items))
			}
			for i := 1; i < 3; i++ {
				if g.Items[i].Lum < g.Items[i-1].Lum {
					t.Fatal("rows not ascending by luminosity")
				}
			}
		}
		for i := 1; i < len(gs); i++ {
			if gs[i].Items[0].Lum < gs[i-1].Items[0].Lum {
				t.Fatal("cards not ascending")
			}
		}
	}
}

func TestKeys(t *testing.T) {
	ks := parseKeys([]byte("\x1b[A\x1b[6~q\x1bOB\x1b[Z\x1bP1$r0;48:2::1:2:3m\x1b\\x"))
	want := []Key{KeyUp, KeyPgDn, KeyRune, KeyDown, KeyShiftTab, KeyRune}
	if len(ks) != len(want) {
		t.Fatalf("%v", ks)
	}
	for i, k := range ks {
		if k.Key != want[i] {
			t.Errorf("key %d = %v", i, k)
		}
	}
	if k := parseKeys([]byte("\x1b")); len(k) != 1 || k[0].Key != KeyEsc {
		t.Fatal("lone esc")
	}
}

func TestRenderWidths(t *testing.T) {
	l := loadTest(t)
	for _, d := range []int{depthNone, depthBasic, depth256, depthTrue} {
		depth = d
		for _, mode := range colorModes {
			for _, w := range []int{40, 80, 130, 200} {
				a := &app{data: l, mode: mode, w: w, h: 40}
				for lg := range lingos {
					a.lingo = lg
					for _, line := range a.body(w) {
						if n := rlen(ansiRe.ReplaceAllString(line, "")); n > w {
							t.Fatalf("depth %d %s w=%d lingo=%d: line is %d wide", d, mode, w, lg, n)
						}
					}
				}
				a.theme = 2
				if v := a.view(); len(v) != 40 {
					t.Fatalf("view has %d lines", len(v))
				}
			}
		}
	}
}

func TestOneLineNames(t *testing.T) {
	if got := truncate("Orchid Comet Showers Cocoa Dust", 12); got != "Orchid Co..." {
		t.Fatal(got)
	}
	if got := truncate("short", 12); got != "short" {
		t.Fatal(got)
	}
}

func TestParseProbe(t *testing.T) {
	reply := "\x1bP1$r0;48:2::1:2:3m\x1b\\\x1b]10;rgb:c5c5/c8c8/c6c6\x1b\\\x1b]11;rgb:1d/1f/21\x07\x1b[?62;c"
	p := parseProbe([]byte(reply), true)
	if !p.Answered || !p.Truecolor || !p.OSC || p.FG != "#c5c8c6" || p.BG != "#1d1f21" {
		t.Fatalf("%+v", p)
	}
	if p := parseProbe([]byte("\x1b[?62;c"), true); p.OSC || p.Truecolor || !p.Answered {
		t.Fatalf("%+v", p)
	}
	if p := parseProbe(nil, false); p.Answered || p.OSC {
		t.Fatalf("%+v", p)
	}
}

func TestRecolor(t *testing.T) {
	depth = depth256
	l := loadTest(t)
	a := &app{data: l, mode: "ansi", w: 130, h: 40, focus: 3}
	a.body(130)
	a.handle(KeyEvent{Key: KeyDown})
	e := a.groups[a.selCard].Items[a.selRow]
	a.handle(KeyEvent{Key: KeyEnter})
	want := "\x1b]11;" + e.BG + "\x1b\\\x1b]10;" + e.FG + "\x1b\\"
	if a.out != want {
		t.Fatalf("got %q want %q", a.out, want)
	}
	a.out = ""
	a.handle(KeyEvent{Key: KeyRune, R: 'x'})
	if a.out != resetSeq || a.appliedBG != "" {
		t.Fatalf("reset: %q", a.out)
	}
	// navigation never leaves the grid
	for i := 0; i < 60; i++ {
		a.handle(KeyEvent{Key: KeyDown})
		a.body(130)
		if a.selCard >= len(a.groups) || a.selRow > 2 {
			t.Fatal("selection out of range")
		}
	}
	if v := a.view(); len(v) != 40 {
		t.Fatalf("view %d lines", len(v))
	}
}

func TestGreyTable(t *testing.T) {
	for _, w := range []int{5, 80, 144, 300} {
		lines := greyTable(w)[3:]
		cells := 0
		for _, l := range lines {
			plain := ansiRe.ReplaceAllString(l, "")
			if rlen(plain)%cellW != 0 || rlen(plain) > max(w, cellW) && w >= cellW {
				t.Fatalf("w=%d: row of %d chars", w, rlen(plain))
			}
			cells += rlen(plain) / cellW
		}
		if cells != 256 {
			t.Fatalf("w=%d: %d cells", w, cells)
		}
	}
	first := greyTable(144)[3]
	if !strings.Contains(first, "\x1b[48;2;0;0;0m\x1b[38;2;255;255;255m #000000 ") ||
		!strings.Contains(first, "\x1b[48;2;15;15;15m\x1b[38;2;240;240;240m #0f0f0f ") {
		t.Fatalf("unexpected first row %q", first)
	}
}
