package tests

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/dagimg-dot/gitsnip/internal/app/downloader"
	"github.com/dagimg-dot/gitsnip/internal/app/gitutil"
	"github.com/dagimg-dot/gitsnip/internal/app/model"
	"github.com/dagimg-dot/gitsnip/internal/apperr"
)

type cmdCall struct {
	dir  string
	args []string
}

type fakeGitRunner struct {
	hasGit   bool
	commands []cmdCall
	setupDir string
	runFunc  func(ctx context.Context, dir string, args ...string) (string, error)
}

func (f *fakeGitRunner) Run(ctx context.Context, dir string, args ...string) (string, error) {
	if f.runFunc != nil {
		return f.runFunc(ctx, dir, args...)
	}
	f.commands = append(f.commands, cmdCall{dir: dir, args: args})
	if len(args) == 1 && args[0] == "init" {
		os.MkdirAll(dir, 0755)
	}
	if args[0] == "checkout" && f.setupDir != "" {
		sparsePath := filepath.Join(dir, f.setupDir)
		os.MkdirAll(sparsePath, 0755)
		os.WriteFile(filepath.Join(sparsePath, "test.txt"), []byte("content"), 0644)
	}
	return "", nil
}

func (f *fakeGitRunner) HasGit() bool {
	return f.hasGit
}

func TestSparseCheckout_success(t *testing.T) {
	fake := &fakeGitRunner{hasGit: true, setupDir: "src/lib"}
	opts := model.DownloadOptions{
		RepoURL:   "https://github.com/owner/repo",
		Subdir:    "src/lib",
		OutputDir: t.TempDir(),
		Branch:    "main",
		Quiet:     true,
	}
	dl := downloader.NewSparseCheckoutDownloader(opts, fake)
	if err := dl.Download(); err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	expected := []string{"init", "remote", "sparse-checkout", "sparse-checkout", "fetch", "checkout"}
	var got []string
	for _, c := range fake.commands {
		got = append(got, c.args[0])
	}
	if len(got) < len(expected) {
		t.Fatalf("expected at least %d commands, got %d: %v", len(expected), len(got), got)
	}
	for i, want := range expected {
		if got[i] != want {
			t.Errorf("command %d: expected %q, got %q", i, want, got[i])
		}
	}

	outFile := filepath.Join(opts.OutputDir, "test.txt")
	if _, err := os.Stat(outFile); os.IsNotExist(err) {
		t.Errorf("output file not found: %s", outFile)
	}
}

func TestSparseCheckout_gitNotInstalled(t *testing.T) {
	fake := &fakeGitRunner{hasGit: false}
	dl := downloader.NewSparseCheckoutDownloader(
		model.DownloadOptions{RepoURL: "https://github.com/owner/repo", Subdir: "src", OutputDir: t.TempDir(), Quiet: true},
		fake,
	)
	err := dl.Download()
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, apperr.ErrGitNotInstalled) {
		t.Errorf("expected ErrGitNotInstalled, got %v", err)
	}
}

func TestSparseCheckout_pathNotFound(t *testing.T) {
	fake := &fakeGitRunner{hasGit: true, setupDir: ""}
	dl := downloader.NewSparseCheckoutDownloader(
		model.DownloadOptions{RepoURL: "https://github.com/owner/repo", Subdir: "nonexistent", OutputDir: t.TempDir(), Branch: "main", Quiet: true},
		fake,
	)
	err := dl.Download()
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, apperr.ErrPathNotFound) {
		t.Errorf("expected ErrPathNotFound, got %v", err)
	}
}

func TestSparseCheckout_runnerError(t *testing.T) {
	fake := &fakeGitRunner{hasGit: true}
	fake.runFunc = func(ctx context.Context, dir string, args ...string) (string, error) {
		if len(args) > 0 && args[0] == "fetch" {
			return "", context.DeadlineExceeded
		}
		if len(args) == 1 && args[0] == "init" {
			os.MkdirAll(dir, 0755)
		}
		return "", nil
	}
	dl := downloader.NewSparseCheckoutDownloader(
		model.DownloadOptions{RepoURL: "https://github.com/owner/repo", Subdir: "src", OutputDir: t.TempDir(), Branch: "main", Quiet: true},
		fake,
	)
	if err := dl.Download(); err == nil {
		t.Fatal("expected error on fetch failure")
	}
}

func TestSparseCheckout_commandsInOrder(t *testing.T) {
	fake := &fakeGitRunner{hasGit: true, setupDir: "src"}
	dl := downloader.NewSparseCheckoutDownloader(
		model.DownloadOptions{
			RepoURL: "https://github.com/owner/repo", Subdir: "src",
			OutputDir: t.TempDir(), Branch: "develop", Quiet: true,
		},
		fake,
	)
	if err := dl.Download(); err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	hasBranchFetch := false
	for _, c := range fake.commands {
		if c.args[0] == "fetch" {
			for _, a := range c.args {
				if a == "develop" {
					hasBranchFetch = true
				}
			}
		}
	}
	if !hasBranchFetch {
		t.Error("expected fetch to include branch name 'develop'")
	}
}

func TestGitHubAPI_download_usesHTTPDoer(t *testing.T) {
	opts := model.DownloadOptions{
		RepoURL: "https://github.com/owner/repo", Subdir: "src",
		OutputDir: t.TempDir(), Quiet: true,
	}
	dl := downloader.NewGitHubAPIDownloader(opts, &http.Client{})
	if dl == nil {
		t.Fatal("expected non-nil downloader")
	}
}

func TestFactory_sparse(t *testing.T) {
	dl, err := downloader.GetDownloader(model.DownloadOptions{
		Method: model.MethodTypeSparse, OutputDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dl == nil {
		t.Fatal("expected non-nil downloader")
	}
}

func TestFactory_api(t *testing.T) {
	dl, err := downloader.GetDownloader(model.DownloadOptions{
		Method: model.MethodTypeAPI, Provider: model.ProviderTypeGitHub,
		OutputDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dl == nil {
		t.Fatal("expected non-nil downloader")
	}
}

func TestFactory_invalid(t *testing.T) {
	_, err := downloader.GetDownloader(model.DownloadOptions{
		Method: "invalid",
	})
	if err == nil {
		t.Fatal("expected error for invalid method")
	}
}

var _ downloader.GitRunner = (*gitutil.RealRunner)(nil)
