package ui

import (
	"io"
	"os"
)

type Style string

const (
	Bold   Style = "\x1b[1m"
	Dim    Style = "\x1b[2m"
	Red    Style = "\x1b[31m"
	Green  Style = "\x1b[32m"
	Yellow Style = "\x1b[33m"
	Cyan   Style = "\x1b[36m"

	reset     = "\x1b[0m"
	clearLine = "\r\x1b[2K"
)

type Paint func(style Style, text string) string

func plain(_ Style, text string) string {
	return text
}

func colored(style Style, text string) string {
	if text == "" {
		return text
	}
	return string(style) + text + reset
}

func Print(w io.Writer, getenv func(string) string, build func(Paint) string) {
	if getenv == nil {
		getenv = os.Getenv
	}
	if color, _, _ := detect(w, getenv); color && attempt(func() error {
		_, err := io.WriteString(w, build(colored))
		return err
	}) {
		return
	}
	io.WriteString(w, build(plain))
}

func attempt(write func() error) (ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	return write() == nil
}
