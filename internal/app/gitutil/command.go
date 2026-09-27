package gitutil

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const DefaultTimeout = 60 * time.Second

func RunGitCommand(ctx context.Context, dir string, args ...string) (string, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), DefaultTimeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		cmdStr := fmt.Sprintf("git %s", strings.Join(args, " "))
		return "", fmt.Errorf("%s: %w (%s)", cmdStr, err, stderr.String())
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

// RealRunner is a production git command runner that delegates to the
// package-level functions. It satisfies the downloader.gitRunner interface.
type RealRunner struct{}

func (RealRunner) Run(ctx context.Context, dir string, args ...string) (string, error) {
	return RunGitCommand(ctx, dir, args...)
}

func (RealRunner) HasGit() bool {
	return IsGitInstalled()
}
