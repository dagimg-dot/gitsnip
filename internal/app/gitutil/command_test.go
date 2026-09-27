package gitutil_test

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"testing"

	"github.com/dagimg-dot/gitsnip/internal/app/gitutil"
)

func requireGit(t *testing.T) {
	t.Helper()
	if !gitutil.IsGitInstalled() {
		t.Skip("git is not installed")
	}
}

func TestRunGitCommandCapturesStderr(t *testing.T) {
	requireGit(t)
	_, err := gitutil.RunGitCommand(context.Background(), t.TempDir(), nil, "rev-parse", "--verify", "no-such-ref")
	var gerr *gitutil.Error
	if !errors.As(err, &gerr) {
		t.Fatalf("got %T %v, want *gitutil.Error", err, err)
	}
	if gerr.Args[0] != "rev-parse" {
		t.Errorf("args = %v", gerr.Args)
	}
	if !strings.HasPrefix(gerr.Stderr, "fatal:") {
		t.Errorf("stderr = %q, want git's fatal message", gerr.Stderr)
	}
}

func TestRunGitCommandIsNonInteractiveEnglishAndStallAware(t *testing.T) {
	requireGit(t)
	if runtime.GOOS == "windows" {
		t.Skip("relies on a POSIX shell alias")
	}
	t.Setenv("GIT_TERMINAL_PROMPT", "1")
	t.Setenv("LC_ALL", "de_DE.UTF-8")
	t.Setenv("GIT_HTTP_LOW_SPEED_LIMIT", "")
	t.Setenv("GIT_HTTP_LOW_SPEED_TIME", "")
	out, err := gitutil.RunGitCommand(context.Background(), t.TempDir(), nil,
		"-c", `alias.env=!printf '%s %s %s/%s' "$GIT_TERMINAL_PROMPT" "$LC_ALL" "$GIT_HTTP_LOW_SPEED_LIMIT" "$GIT_HTTP_LOW_SPEED_TIME"`, "env")
	if err != nil {
		t.Fatal(err)
	}
	if want := "0 C 1000/60"; out != want {
		t.Errorf("git saw %q, want %q", out, want)
	}
}

func TestRunGitCommandReportsCancellation(t *testing.T) {
	requireGit(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := gitutil.RunGitCommand(ctx, t.TempDir(), nil, "version"); !errors.Is(err, context.Canceled) {
		t.Errorf("got %v, want context.Canceled", err)
	}
}

func TestErrorRedactsCredentials(t *testing.T) {
	err := &gitutil.Error{
		Args:   []string{"remote", "add", "origin", "https://ghp_secret@github.com/o/r"},
		Stderr: "fatal: unable to access 'https://user:pass@github.com/o/r/'",
		Err:    errors.New("exit status 128"),
	}
	msg := err.Error()
	if strings.Contains(msg, "ghp_secret") || strings.Contains(msg, "user:pass") {
		t.Fatalf("credentials leaked: %s", msg)
	}
	if !strings.Contains(msg, "https://***@github.com/o/r") {
		t.Errorf("message = %s", msg)
	}
}
