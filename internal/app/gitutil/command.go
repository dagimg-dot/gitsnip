package gitutil

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

var credentials = regexp.MustCompile(`(://)[^/@\s]+@`)

type Error struct {
	Args   []string
	Stderr string
	Err    error
}

func (e *Error) Error() string {
	msg := fmt.Sprintf("git %s: %v", redact(strings.Join(e.Args, " ")), e.Err)
	if e.Stderr != "" {
		msg += ": " + redact(e.Stderr)
	}
	return msg
}

func (e *Error) Unwrap() error {
	return e.Err
}

func redact(s string) string {
	return credentials.ReplaceAllString(s, "${1}***@")
}

func RunGitCommand(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "LC_ALL=C")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", &Error{Args: args, Stderr: strings.TrimSpace(stderr.String()), Err: err}
	}
	return stdout.String(), nil
}

func IsGitInstalled() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

func CreateTempDir() (string, error) {
	tempDir, err := os.MkdirTemp("", "gitsnip-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary directory: %w", err)
	}
	return tempDir, nil
}

func CleanupTempDir(dir string) error {
	return os.RemoveAll(dir)
}

type RealRunner struct{}

func (RealRunner) Run(ctx context.Context, dir string, args ...string) (string, error) {
	return RunGitCommand(ctx, dir, args...)
}

func (RealRunner) HasGit() bool {
	return IsGitInstalled()
}
