//go:build windows

package main

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	kernel32                   = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleMode         = kernel32.NewProc("GetConsoleMode")
	procSetConsoleMode         = kernel32.NewProc("SetConsoleMode")
	procGetConsoleScreenBuffer = kernel32.NewProc("GetConsoleScreenBufferInfo")
)

const (
	enableProcessedInput      = 0x0001
	enableLineInput           = 0x0002
	enableEchoInput           = 0x0004
	enableVirtualTerminalIn   = 0x0200
	enableProcessedOutput     = 0x0001
	enableVirtualTerminalProc = 0x0004
)

func getMode(f *os.File) (uint32, bool) {
	var m uint32
	r, _, _ := procGetConsoleMode.Call(f.Fd(), uintptr(unsafe.Pointer(&m)))
	return m, r != 0
}

func setMode(f *os.File, m uint32) bool {
	r, _, _ := procSetConsoleMode.Call(f.Fd(), uintptr(m))
	return r != 0
}

// isTerminal reports whether f is a console.
func isTerminal(f *os.File) bool {
	_, ok := getMode(f)
	return ok
}

// makeRaw switches the console to raw VT input/output and returns a restore function.
func makeRaw() (func(), error) {
	inOld, ok := getMode(os.Stdin)
	if !ok {
		return nil, syscall.EINVAL
	}
	outOld, outOK := getMode(os.Stdout)
	in := inOld&^(enableProcessedInput|enableLineInput|enableEchoInput) | enableVirtualTerminalIn
	if !setMode(os.Stdin, in) {
		return nil, syscall.EINVAL
	}
	if outOK {
		setMode(os.Stdout, outOld|enableProcessedOutput|enableVirtualTerminalProc)
	}
	return func() {
		setMode(os.Stdin, inOld)
		if outOK {
			setMode(os.Stdout, outOld)
		}
	}, nil
}

// enableVT turns on ANSI/VT escape processing for stdout, which older consoles leave off
// (without it the escape sequences are printed literally, e.g. "←[48;2;...").
func enableVT() {
	if m, ok := getMode(os.Stdout); ok {
		setMode(os.Stdout, m|enableProcessedOutput|enableVirtualTerminalProc)
	}
}

// termSize returns the visible console window size in columns and rows.
func termSize() (int, int) {
	var info struct {
		SizeX, SizeY       int16
		CurX, CurY         int16
		Attr               uint16
		Left, Top, Right   int16
		Bottom             int16
		MaxSizeX, MaxSizeY int16
	}
	for _, f := range []*os.File{os.Stdout, os.Stderr} {
		r, _, _ := procGetConsoleScreenBuffer.Call(f.Fd(), uintptr(unsafe.Pointer(&info)))
		if r != 0 {
			w, h := int(info.Right-info.Left)+1, int(info.Bottom-info.Top)+1
			if w > 0 && h > 0 {
				return w, h
			}
		}
	}
	return 80, 24
}
