package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

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

	src := filepath.Join(snap.Dir, filepath.FromSlash(req.Subdir))
	info, err := os.Stat(src)
	if err != nil {
		return model.Result{}, &apperr.Error{
			Err:     apperr.ErrPathNotFound,
			Message: fmt.Sprintf("Directory '%s' not found in the repository", req.Subdir),
			Hint:    "Check that the folder path exists in the repository",
		}
	}

	rep.Stage(fmt.Sprintf("Copying files to %s...", req.OutputDir))
	if info.IsDir() {
		err = util.CopyDirectory(src, req.OutputDir)
	} else {
		err = util.CopyFile(src, req.OutputDir)
	}
	if err != nil {
		return model.Result{}, fmt.Errorf("failed to copy %s: %w", req.Subdir, err)
	}

	return model.Result{Ref: snap.Ref, Output: req.OutputDir}, nil
}
