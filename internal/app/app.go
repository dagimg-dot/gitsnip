package app

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/dagimg-dot/gitsnip/internal/app/downloader"
	"github.com/dagimg-dot/gitsnip/internal/app/model"
	"github.com/dagimg-dot/gitsnip/internal/apperr"
	"github.com/dagimg-dot/gitsnip/internal/util"
)

func Download(ctx context.Context, req model.Request, rep model.Reporter) (model.Result, error) {
	dl, err := downloader.GetDownloader(req)
	if err != nil {
		return model.Result{}, err
	}
	return run(ctx, dl, req, rep)
}

func run(ctx context.Context, dl downloader.Downloader, req model.Request, rep model.Reporter) (model.Result, error) {
	staging, err := os.MkdirTemp("", "gitsnip-*")
	if err != nil {
		return model.Result{}, fmt.Errorf("failed to create a staging directory: %w", err)
	}
	defer os.RemoveAll(staging)

	snap, err := dl.Download(ctx, req, staging, rep)
	if err != nil {
		return model.Result{}, err
	}

	if linkedParent(snap.Dir, req.Subdir) {
		return model.Result{}, &apperr.Error{
			Err:     apperr.ErrPathNotFound,
			Message: fmt.Sprintf("Path '%s' goes through a symlink", req.Subdir),
			Hint:    "Request the folder the symlink points to instead",
		}
	}

	src := filepath.Join(snap.Dir, filepath.FromSlash(req.Subdir))
	info, err := os.Lstat(src)
	if err != nil {
		return model.Result{}, &apperr.Error{
			Err:     apperr.ErrPathNotFound,
			Message: fmt.Sprintf("Directory '%s' not found in the repository", req.Subdir),
			Hint:    "Check that the folder path exists in the repository",
		}
	}

	skipped := func(rel, reason string) {
		rep.Warn(fmt.Sprintf("skipped symlink %s (%s)", rel, reason))
	}
	rep.Stage(fmt.Sprintf("Copying files to %s...", req.OutputDir))
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		err = util.CopySymlink(src, req.OutputDir, snap.Dir, skipped)
	case info.IsDir():
		err = util.CopyTree(src, req.OutputDir, func(rel, reason string) {
			skipped(path.Join(req.Subdir, rel), reason)
		})
	default:
		err = util.CopyFile(src, req.OutputDir)
	}
	if err != nil {
		return model.Result{}, fmt.Errorf("failed to copy %s: %w", req.Subdir, err)
	}

	return model.Result{Ref: snap.Ref, Output: req.OutputDir}, nil
}

func linkedParent(root, rel string) bool {
	parts := strings.Split(strings.Trim(rel, "/"), "/")
	current := root
	for _, part := range parts[:len(parts)-1] {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			return false
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return true
		}
	}
	return false
}
