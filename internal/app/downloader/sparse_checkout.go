package downloader

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dagimg-dot/gitsnip/internal/app/model"
	"github.com/dagimg-dot/gitsnip/internal/apperr"
)

type GitRunner interface {
	Run(ctx context.Context, dir string, args ...string) (string, error)
	HasGit() bool
}

type sparseCheckoutDownloader struct {
	runner GitRunner
}

func NewSparseCheckoutDownloader(runner GitRunner) Downloader {
	return &sparseCheckoutDownloader{runner: runner}
}

func (s *sparseCheckoutDownloader) Download(ctx context.Context, req model.Request, dir string, rep model.Reporter) (model.Snapshot, error) {
	if !s.runner.HasGit() {
		return model.Snapshot{}, &apperr.Error{
			Err:     apperr.ErrGitNotInstalled,
			Message: "Git is not installed on this system",
			Hint:    "Please install Git to use the sparse checkout method",
		}
	}

	if req.Branch == "" {
		rep.Stage(fmt.Sprintf("Downloading directory %s from %s (default branch) using sparse checkout...", req.Subdir, req.RepoURL))
	} else {
		rep.Stage(fmt.Sprintf("Downloading directory %s from %s (branch: %s) using sparse checkout...", req.Subdir, req.RepoURL, req.Branch))
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	git := func(args ...string) error {
		if _, err := s.runner.Run(ctx, dir, args...); err != nil {
			return gitFailure(err, req.RepoURL, req.Branch)
		}
		return nil
	}

	setup := [][]string{
		{"init"},
		{"remote", "add", "origin", authenticatedURL(req.RepoURL, req.Token)},
		{"sparse-checkout", "init", "--cone"},
		{"sparse-checkout", "set", req.Subdir},
	}
	for _, args := range setup {
		if err := git(args...); err != nil {
			return model.Snapshot{}, err
		}
	}

	rep.Stage("Downloading content from repository...")
	fetch := []string{"fetch", "--depth=1", "--no-tags", "origin"}
	if req.Branch != "" {
		fetch = append(fetch, req.Branch)
	}
	if err := git(fetch...); err != nil {
		return model.Snapshot{}, err
	}
	if err := git("checkout", "FETCH_HEAD"); err != nil {
		return model.Snapshot{}, err
	}

	return model.Snapshot{Dir: dir, Ref: req.Branch}, nil
}

func authenticatedURL(repoURL, token string) string {
	if strings.HasPrefix(repoURL, "github.com/") {
		repoURL = "https://" + repoURL
	}

	if token == "" {
		return repoURL
	}

	if strings.HasPrefix(repoURL, "https://") {
		parts := strings.SplitN(repoURL[8:], "/", 2)
		if len(parts) == 2 {
			return fmt.Sprintf("https://%s@%s/%s", token, parts[0], parts[1])
		}
	}

	return repoURL
}
