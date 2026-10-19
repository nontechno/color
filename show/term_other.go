//go:build !linux && !darwin && !dragonfly && !freebsd && !netbsd && !openbsd && !windows

package main

import (
	"errors"
	"os"
)

func isTerminal(f *os.File) bool { return false }

func makeRaw() (func(), error) {
	return nil, errors.New("interactive mode is not supported on this platform; use -print")
}

func termSize() (int, int) { return 80, 24 }

func enableVT() {}
