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

func RunGitCommand(ctx context.Context, dir string, env []string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(baseEnv(), env...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return "", ctxErr
		}
		return "", &Error{Args: args, Stderr: strings.TrimSpace(stderr.String()), Err: err}
	}
	return stdout.String(), nil
}

// baseEnv keeps git from prompting for credentials, since a prompt would fight
// the spinner for the terminal, and forces the C locale because git_errors.go
// matches git's English messages. With no overall timeout, the low-speed limit
// is what aborts a stalled transfer, unless the user set their own limits.
func baseEnv() []string {
	env := append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "LC_ALL=C")
	if os.Getenv("GIT_HTTP_LOW_SPEED_LIMIT") == "" && os.Getenv("GIT_HTTP_LOW_SPEED_TIME") == "" {
		env = append(env, "GIT_HTTP_LOW_SPEED_LIMIT=1000", "GIT_HTTP_LOW_SPEED_TIME=60")
	}
	return env
}

func IsGitInstalled() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

type RealRunner struct{}

func (RealRunner) Run(ctx context.Context, dir string, env []string, args ...string) (string, error) {
	return RunGitCommand(ctx, dir, env, args...)
}

func (RealRunner) HasGit() bool {
	return IsGitInstalled()
}
