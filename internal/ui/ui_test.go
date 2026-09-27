package ui

import (
	"bytes"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

var summary = &Summary{
	Repo:    "dagimg-dot/gitsnip",
	Ref:     "main",
	What:    "internal/app",
	Target:  "./app",
	Files:   7,
	Bytes:   12155,
	Elapsed: 1400 * time.Millisecond,
}

func TestPlainSuccessMatchesTheDesign(t *testing.T) {
	var buf bytes.Buffer
	New(&buf, Options{}).Success(summary)
	want := "✓ dagimg-dot/gitsnip@main · internal/app → ./app   7 files · 11.9 KB · 1.4s\n"
	if buf.String() != want {
		t.Errorf("got  %q\nwant %q", buf.String(), want)
	}
}

func TestWholeRepositorySummarySkipsThePaths(t *testing.T) {
	var buf bytes.Buffer
	s := *summary
	s.What, s.Target, s.Files, s.Bytes = "", "./gitsnip", 1, 512
	New(&buf, Options{}).Success(&s)
	if want := "✓ dagimg-dot/gitsnip@main → ./gitsnip   1 file · 512 B · 1.4s\n"; buf.String() != want {
		t.Errorf("got  %q\nwant %q", buf.String(), want)
	}
}

func TestFailShowsTheHintAndOnlyShowsDetailsWhenVerbose(t *testing.T) {
	var buf bytes.Buffer
	New(&buf, Options{}).Fail(`Branch or tag "main" doesn't exist in torvalds/linux`, `the default branch is "master"`, "fatal: couldn't find remote ref main")
	want := "✗ Branch or tag \"main\" doesn't exist in torvalds/linux\n  → the default branch is \"master\"\n"
	if buf.String() != want {
		t.Errorf("got  %q\nwant %q", buf.String(), want)
	}

	buf.Reset()
	New(&buf, Options{Verbose: true}).Fail("Boom", "", "fatal: first\nsecond")
	if want := "✗ Boom\n  fatal: first\n  second\n"; buf.String() != want {
		t.Errorf("verbose got %q, want %q", buf.String(), want)
	}
}

func TestQuietAndJSONKeepStderrForErrorsOnly(t *testing.T) {
	for _, opts := range []Options{{Quiet: true}, {JSON: true}} {
		var buf bytes.Buffer
		u := New(&buf, opts)
		u.Start("o/r")
		u.Stage("fetching")
		u.Warn("skipped symlink x")
		u.Success(summary)
		if buf.Len() != 0 {
			t.Errorf("%+v printed %q", opts, buf.String())
		}
		if got := u.Warnings(); len(got) != 1 {
			t.Errorf("%+v kept warnings %q", opts, got)
		}
		u.Fail("Boom", "", "")
		if buf.String() != "✗ Boom\n" {
			t.Errorf("%+v error output %q", opts, buf.String())
		}
	}
}

func TestVerboseNarratesStagesWhenNothingIsAnimated(t *testing.T) {
	var buf bytes.Buffer
	u := New(&buf, Options{Verbose: true})
	u.Stage("cloning")
	u.Debug("git clone --depth=1 (0.9s)")
	u.Warn("skipped submodule vendor/lib")
	want := "· cloning\n  git clone --depth=1 (0.9s)\n! skipped submodule vendor/lib\n"
	if buf.String() != want {
		t.Errorf("got  %q\nwant %q", buf.String(), want)
	}
}

func TestColorsWrapTheSymbolsAndTheRepository(t *testing.T) {
	var buf bytes.Buffer
	u := &UI{w: &buf, color: true, width: 80}
	u.Success(summary)
	out := buf.String()
	for _, want := range []string{string(Green) + "✓" + reset, string(Bold) + "dagimg-dot/gitsnip" + reset, string(Cyan) + "@main" + reset} {
		if !strings.Contains(out, want) {
			t.Errorf("%q is missing %q", out, want)
		}
	}
}

type failingTerminal struct {
	buf    bytes.Buffer
	panics bool
}

func (f *failingTerminal) Write(p []byte) (int, error) {
	if bytes.ContainsRune(p, '\x1b') {
		if f.panics {
			panic("terminal exploded")
		}
		return 0, errors.New("terminal rejected escape codes")
	}
	return f.buf.Write(p)
}

func (f *failingTerminal) String() string {
	return f.buf.String()
}

func TestBrokenTerminalsFallBackToPlainOutput(t *testing.T) {
	for _, panics := range []bool{false, true} {
		term := &failingTerminal{panics: panics}
		u := &UI{w: term, color: true, live: true, width: 80}
		u.Warn("skipped symlink src/key (points outside the folder)")
		u.Success(summary)

		want := "! skipped symlink src/key (points outside the folder)\n" +
			"✓ dagimg-dot/gitsnip@main · internal/app → ./app   7 files · 11.9 KB · 1.4s\n"
		if term.String() != want {
			t.Errorf("panics=%v got %q", panics, term.String())
		}
		if u.color || u.live {
			t.Errorf("panics=%v: the UI kept using escape codes after a failure", panics)
		}
	}
}

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

func TestSpinnerDrawsInPlaceAndClearsItself(t *testing.T) {
	out := &syncBuffer{}
	u := &UI{w: out, live: true, width: 80}
	u.Start("dagimg-dot/gitsnip")
	u.Stage("downloading")
	u.Progress(3, 7)
	time.Sleep(200 * time.Millisecond)
	u.Success(summary)

	got := out.String()
	if !strings.Contains(got, " dagimg-dot/gitsnip · downloading 3/7") {
		t.Errorf("no spinner frame in %q", got)
	}
	if !strings.HasSuffix(got, clearLine+"✓ dagimg-dot/gitsnip@main · internal/app → ./app   7 files · 11.9 KB · 1.4s\n") {
		t.Errorf("summary didn't replace the spinner line: %q", got)
	}
}

func TestSpinnerLineFitsTheTerminal(t *testing.T) {
	u := &UI{width: 40, subject: "dagimg-dot/gitsnip", stage: "fetching src/components/very/deep/path"}
	line := u.spinnerLine(0)
	if n := len([]rune(line)); n > 39 {
		t.Errorf("line is %d runes wide: %q", n, line)
	}
	if !strings.HasSuffix(line, "…") {
		t.Errorf("long status should be truncated: %q", line)
	}
}

func TestFormatting(t *testing.T) {
	sizes := map[int64]string{0: "0 B", 1023: "1023 B", 1024: "1.0 KB", 12155: "11.9 KB", 5 << 20: "5.0 MB", 3 << 30: "3.0 GB"}
	for n, want := range sizes {
		if got := Size(n); got != want {
			t.Errorf("Size(%d) = %q, want %q", n, got, want)
		}
	}
	durations := map[time.Duration]string{1400 * time.Millisecond: "1.4s", 65 * time.Second: "1m05s"}
	for d, want := range durations {
		if got := Duration(d); got != want {
			t.Errorf("Duration(%v) = %q, want %q", d, got, want)
		}
	}
	if Count(1, "file", "files") != "1 file" || Count(7, "file", "files") != "7 files" {
		t.Error("Count pluralizes wrongly")
	}
}

func TestPrintFallsBackForNonTerminals(t *testing.T) {
	var buf bytes.Buffer
	Print(&buf, func(p Paint) string { return p(Bold, "Usage") + "\n" })
	if buf.String() != "Usage\n" {
		t.Errorf("got %q", buf.String())
	}
}
