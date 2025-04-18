package downloader

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dagimg-dot/gitsnip/internal/app/gitutil"
	"github.com/dagimg-dot/gitsnip/internal/app/model"
	"github.com/dagimg-dot/gitsnip/internal/errors"
	"github.com/dagimg-dot/gitsnip/internal/util"
)

type sparseCheckoutDownloader struct {
	opts model.DownloadOptions
}

func NewSparseCheckoutDownloader(opts model.DownloadOptions) Downloader {
	return &sparseCheckoutDownloader{opts: opts}
}

func (s *sparseCheckoutDownloader) Download() error {
	if !gitutil.IsGitInstalled() {
		return &errors.AppError{
			Err:     errors.ErrGitNotInstalled,
			Message: "Git is not installed on this system",
			Hint:    "Please install Git to use the sparse checkout method",
		}
	}

	if err := util.EnsureDir(s.opts.OutputDir); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	if !s.opts.Quiet {
		if s.opts.Branch == "" {
			fmt.Printf("Downloading directory %s from %s (default branch) using sparse checkout...\n",
				s.opts.Subdir, s.opts.RepoURL)
		} else {
			fmt.Printf("Downloading directory %s from %s (branch: %s) using sparse checkout...\n",
				s.opts.Subdir, s.opts.RepoURL, s.opts.Branch)
		}
	}

	tempDir, err := gitutil.CreateTempDir()
	if err != nil {
		return err
	}
	defer gitutil.CleanupTempDir(tempDir)

	repoURL := s.getAuthenticatedRepoURL()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if !s.opts.Quiet {
		fmt.Println("Setting up Git repository...")
	}
	if err := s.initRepo(ctx, tempDir, repoURL); err != nil {
		return err
	}

	if err := s.setupSparseCheckout(ctx, tempDir); err != nil {
		return err
	}

	if err := s.pullContent(ctx, tempDir); err != nil {
		return err
	}

	sparsePath := filepath.Join(tempDir, s.opts.Subdir)
	if _, err := os.Stat(sparsePath); os.IsNotExist(err) {
		return &errors.AppError{
			Err:     errors.ErrPathNotFound,
			Message: fmt.Sprintf("Directory '%s' not found in the repository", s.opts.Subdir),
			Hint:    "Check that the folder path exists in the repository",
		}
	}

	if !s.opts.Quiet {
		fmt.Printf("Copying files to %s...\n", s.opts.OutputDir)
	}

	if err := util.CopyDirectory(sparsePath, s.opts.OutputDir); err != nil {
		return fmt.Errorf("failed to copy directory: %w", err)
	}

	if !s.opts.Quiet {
		fmt.Println("Download completed successfully.")
	}
	return nil
}

func (s *sparseCheckoutDownloader) getAuthenticatedRepoURL() string {
	if s.opts.Token == "" {
		return s.opts.RepoURL
	}

	if strings.HasPrefix(s.opts.RepoURL, "https://") {
		parts := strings.SplitN(s.opts.RepoURL[8:], "/", 2)
		if len(parts) == 2 {
			return fmt.Sprintf("https://%s@%s/%s", s.opts.Token, parts[0], parts[1])
		}
	}

	return s.opts.RepoURL
}

func (s *sparseCheckoutDownloader) initRepo(ctx context.Context, dir, repoURL string) error {
	if _, err := gitutil.RunGitCommand(ctx, dir, "init"); err != nil {
		return errors.ParseGitError(err, "git init failed")
	}

	if _, err := gitutil.RunGitCommand(ctx, dir, "remote", "add", "origin", repoURL); err != nil {
		return errors.ParseGitError(err, "failed to add remote")
	}

	return nil
}

func (s *sparseCheckoutDownloader) setupSparseCheckout(ctx context.Context, dir string) error {
	if _, err := gitutil.RunGitCommand(ctx, dir, "config", "core.sparseCheckout", "true"); err != nil {
		return errors.ParseGitError(err, "failed to enable sparse checkout")
	}

	err := s.setupModernSparseCheckout(ctx, dir)
	if err != nil {
		return s.setupLegacySparseCheckout(ctx, dir)
	}

	return nil
}

func (s *sparseCheckoutDownloader) setupModernSparseCheckout(ctx context.Context, dir string) error {
	_, err := gitutil.RunGitCommand(ctx, dir, "sparse-checkout", "set", s.opts.Subdir)
	if err != nil {
		return err
	}
	return nil
}

func (s *sparseCheckoutDownloader) setupLegacySparseCheckout(_ context.Context, dir string) error {
	sparseCheckoutPath := filepath.Join(dir, ".git", "info", "sparse-checkout")
	sparseCheckoutDir := filepath.Dir(sparseCheckoutPath)

	if err := os.MkdirAll(sparseCheckoutDir, 0755); err != nil {
		return fmt.Errorf("failed to create sparse checkout directory: %w", err)
	}

	sparseCheckoutPattern := fmt.Sprintf("%s/**", s.opts.Subdir)
	if err := os.WriteFile(sparseCheckoutPath, []byte(sparseCheckoutPattern), 0644); err != nil {
		return fmt.Errorf("failed to write sparse checkout file: %w", err)
	}

	return nil
}

func (s *sparseCheckoutDownloader) pullContent(ctx context.Context, dir string) error {
	args := []string{"pull", "--depth=1", "origin"}

	if s.opts.Branch != "" {
		args = append(args, s.opts.Branch)
	}

	if !s.opts.Quiet {
		fmt.Println("Downloading content from repository...")
	}

	_, err := gitutil.RunGitCommand(ctx, dir, args...)
	if err != nil {
		return errors.ParseGitError(err, "failed to pull content")
	}

	return nil
}
