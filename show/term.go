package main

import (
	"os"
	"unicode/utf8"
)

// Key identifies a decoded key press.
type Key int

const (
	KeyRune Key = iota
	KeyUp
	KeyDown
	KeyLeft
	KeyRight
	KeyPgUp
	KeyPgDn
	KeyHome
	KeyEnd
	KeyTab
	KeyShiftTab
	KeyEsc
	KeyEnter
	KeyCtrlC
)

// KeyEvent is one key press; R is set when Key == KeyRune.
type KeyEvent struct {
	Key Key
	R   rune
}

// parseKeys decodes a chunk of terminal input (VT sequences on every platform).
func parseKeys(b []byte) []KeyEvent {
	var out []KeyEvent
	for i := 0; i < len(b); {
		c := b[i]
		switch {
		case c == 0x1b:
			if i+1 >= len(b) {
				out = append(out, KeyEvent{Key: KeyEsc})
				i++
				continue
			}
			if b[i+1] == 'P' || b[i+1] == ']' { // DCS / OSC replies: skip to ST or BEL
				j := i + 2
				for j < len(b) && b[j] != 0x07 && !(b[j] == 0x1b && j+1 < len(b) && b[j+1] == '\\') {
					j++
				}
				if j < len(b) && b[j] == 0x1b {
					j++
				}
				i = j + 1
				continue
			}
			if b[i+1] != '[' && b[i+1] != 'O' {
				i++ // Alt+key: ignore the prefix, the key is read next
				continue
			}
			j := i + 2
			for j < len(b) && (b[j] < 0x40 || b[j] > 0x7e) {
				j++
			}
			if j >= len(b) {
				i = len(b)
				continue
			}
			params := string(b[i+2 : j])
			switch final := b[j]; final {
			case 'A':
				out = append(out, KeyEvent{Key: KeyUp})
			case 'B':
				out = append(out, KeyEvent{Key: KeyDown})
			case 'C':
				out = append(out, KeyEvent{Key: KeyRight})
			case 'D':
				out = append(out, KeyEvent{Key: KeyLeft})
			case 'H':
				out = append(out, KeyEvent{Key: KeyHome})
			case 'F':
				out = append(out, KeyEvent{Key: KeyEnd})
			case 'Z':
				out = append(out, KeyEvent{Key: KeyShiftTab})
			case '~':
				switch params {
				case "1", "7":
					out = append(out, KeyEvent{Key: KeyHome})
				case "4", "8":
					out = append(out, KeyEvent{Key: KeyEnd})
				case "5":
					out = append(out, KeyEvent{Key: KeyPgUp})
				case "6":
					out = append(out, KeyEvent{Key: KeyPgDn})
				}
			}
			i = j + 1
		case c == 3:
			out = append(out, KeyEvent{Key: KeyCtrlC})
			i++
		case c == '\t':
			out = append(out, KeyEvent{Key: KeyTab})
			i++
		case c == '\r' || c == '\n':
			out = append(out, KeyEvent{Key: KeyEnter})
			i++
		case c < 0x20:
			i++
		default:
			r, n := utf8.DecodeRune(b[i:])
			out = append(out, KeyEvent{Key: KeyRune, R: r})
			i += n
		}
	}
	return out
}

// readChunks feeds raw stdin chunks into ch until stdin closes.
func readChunks(ch chan<- []byte) {
	buf := make([]byte, 256)
	for {
		n, err := os.Stdin.Read(buf)
		if n > 0 {
			ch <- append([]byte(nil), buf[:n]...)
		}
		if err != nil {
			close(ch)
			return
		}
	}
}
