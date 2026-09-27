package ui

import (
	"io"

	"golang.org/x/term"
)

var enableVirtualTerminal = func(uintptr) error { return nil }

type fdWriter interface {
	io.Writer
	Fd() uintptr
}

func detect(w io.Writer, getenv func(string) string) (color, live bool, width int) {
	f, ok := w.(fdWriter)
	if !ok || getenv("TERM") == "dumb" || !term.IsTerminal(int(f.Fd())) {
		return false, false, 0
	}
	if err := enableVirtualTerminal(f.Fd()); err != nil {
		return false, false, 0
	}
	width, _, err := term.GetSize(int(f.Fd()))
	if err != nil || width <= 0 {
		width = 80
	}
	return getenv("NO_COLOR") == "", true, width
}
