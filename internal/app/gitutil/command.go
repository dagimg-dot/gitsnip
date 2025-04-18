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

func RunGitCommandWithInput(ctx context.Context, dir, input string, args ...string) (string, error) {
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
	cmd.Stdin = strings.NewReader(input)

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

func GitVersion() (string, error) {
	cmd := exec.Command("git", "--version")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get git version: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
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
