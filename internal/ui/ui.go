package ui

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

var frames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type Options struct {
	Quiet   bool
	Verbose bool
	JSON    bool
}

type Summary struct {
	Repo    string
	Ref     string
	What    string
	Target  string
	Files   int
	Bytes   int64
	Elapsed time.Duration
}

type UI struct {
	mu       sync.Mutex
	w        io.Writer
	opts     Options
	color    bool
	live     bool
	width    int
	subject  string
	stage    string
	done     int
	total    int
	shown    bool
	warnings []string
	stop     chan struct{}
	stopped  chan struct{}
}

func New(stderr io.Writer, opts Options) *UI {
	u := &UI{w: stderr, opts: opts}
	u.color, u.live, u.width = detect(stderr)
	if opts.Quiet || opts.JSON {
		u.live = false
	}
	return u
}

func (u *UI) Start(subject string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.subject = subject
	if !u.live || u.stop != nil {
		return
	}
	u.stop, u.stopped = make(chan struct{}), make(chan struct{})
	go u.spin(u.stop, u.stopped)
}

func (u *UI) Stop() {
	u.mu.Lock()
	stop, stopped := u.stop, u.stopped
	u.stop, u.stopped = nil, nil
	u.mu.Unlock()
	if stop != nil {
		close(stop)
		<-stopped
	}

	u.mu.Lock()
	defer u.mu.Unlock()
	if u.shown && !attempt(func() error {
		_, err := io.WriteString(u.w, clearLine)
		return err
	}) {
		u.degrade()
	}
	u.shown = false
}

func (u *UI) Stage(text string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.stage, u.done, u.total = text, 0, 0
	if u.opts.Verbose && !u.live && !u.opts.Quiet && !u.opts.JSON {
		u.print(func(p Paint) string { return p(Dim, "· "+text) + "\n" })
	}
}

func (u *UI) Progress(done, total int) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.done, u.total = done, total
}

func (u *UI) Warn(text string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.warnings = append(u.warnings, text)
	if u.opts.Quiet || u.opts.JSON {
		return
	}
	u.print(func(p Paint) string { return p(Yellow, "!") + " " + text + "\n" })
}

func (u *UI) Debug(text string) {
	if !u.opts.Verbose {
		return
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	u.print(func(p Paint) string { return p(Dim, "  "+text) + "\n" })
}

func (u *UI) Warnings() []string {
	u.mu.Lock()
	defer u.mu.Unlock()
	return append([]string(nil), u.warnings...)
}

func (u *UI) Success(s *Summary) {
	u.Stop()
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.opts.Quiet || u.opts.JSON {
		return
	}
	u.print(func(p Paint) string {
		var b strings.Builder
		b.WriteString(p(Green, "✓") + " " + p(Bold, s.Repo))
		if s.Ref != "" {
			b.WriteString(p(Cyan, "@"+s.Ref))
		}
		if s.What != "" {
			b.WriteString(p(Dim, " ·") + " " + s.What)
		}
		b.WriteString(" " + p(Dim, "→") + " " + s.Target)
		b.WriteString("   " + p(Dim, fmt.Sprintf("%s · %s · %s", Count(s.Files, "file", "files"), Size(s.Bytes), Duration(s.Elapsed))))
		return b.String() + "\n"
	})
}

func (u *UI) Fail(message, hint, detail string) {
	u.Stop()
	u.mu.Lock()
	defer u.mu.Unlock()
	u.print(func(p Paint) string {
		var b strings.Builder
		b.WriteString(p(Red, "✗") + " " + p(Bold, message) + "\n")
		if hint != "" {
			b.WriteString("  " + p(Dim, "→ "+hint) + "\n")
		}
		if detail != "" && u.opts.Verbose {
			for line := range strings.SplitSeq(strings.TrimSpace(detail), "\n") {
				b.WriteString("  " + p(Dim, line) + "\n")
			}
		}
		return b.String()
	})
}

// print tries the styled line first. If that write fails or panics, the UI
// drops to plain text for the rest of the run and writes the same line again
// without escape codes, so no output is lost.
func (u *UI) print(build func(Paint) string) {
	if u.color || u.live {
		paint := plain
		if u.color {
			paint = colored
		}
		prefix := ""
		if u.shown {
			prefix = clearLine
		}
		if attempt(func() error {
			_, err := io.WriteString(u.w, prefix+build(paint))
			return err
		}) {
			u.shown = false
			return
		}
		u.degrade()
	}
	_, _ = io.WriteString(u.w, build(plain))
}

func (u *UI) degrade() {
	u.color, u.live, u.shown = false, false, false
}

func (u *UI) spin(stop, stopped chan struct{}) {
	defer close(stopped)
	ticker := time.NewTicker(80 * time.Millisecond)
	defer ticker.Stop()
	for frame := 0; ; frame++ {
		u.draw(frame)
		select {
		case <-stop:
			return
		case <-ticker.C:
		}
	}
}

func (u *UI) draw(frame int) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if !u.live {
		return
	}
	if attempt(func() error {
		_, err := io.WriteString(u.w, clearLine+u.spinnerLine(frame))
		return err
	}) {
		u.shown = true
		return
	}
	u.degrade()
}

func (u *UI) spinnerLine(frame int) string {
	paint := plain
	if u.color {
		paint = colored
	}

	room := max(u.width-1, 10)
	icon := frames[frame%len(frames)]
	subject := truncate(u.subject, room-2)
	line := paint(Cyan, icon) + " " + paint(Bold, subject)

	status := u.stage
	if u.total > 0 {
		status = fmt.Sprintf("%s %d/%d", u.stage, u.done, u.total)
	}
	if left := room - 2 - utf8.RuneCountInString(subject) - 3; status != "" && left >= 4 {
		line += paint(Dim, " · "+truncate(status, left))
	}
	return line
}

func truncate(s string, limit int) string {
	if utf8.RuneCountInString(s) <= limit {
		return s
	}
	if limit <= 1 {
		return "…"
	}
	runes := []rune(s)
	return string(runes[:limit-1]) + "…"
}
