package app

import (
	"context"
	"fmt"
	"os"

	"github.com/dagimg-dot/gitsnip/internal/app/downloader"
	"github.com/dagimg-dot/gitsnip/internal/app/model"
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

	files, err := listFiles(snap.Dir)
	if err != nil {
		return model.Result{}, fmt.Errorf("failed to read the downloaded files: %w", err)
	}

	paths := snap.Paths
	if paths == nil {
		paths = req.Paths
	}

	sel, err := selectFiles(paths, files)
	if err != nil {
		return model.Result{}, err
	}

	output := req.Output
	if output == "" {
		output = defaultOutput(sel, req.Source.Repo)
	}

	rep.Stage(fmt.Sprintf("writing %d files", len(sel.files)))
	written, size, err := writeFiles(ctx, snap.Dir, sel, output, req.Force, rep)
	if err != nil {
		return model.Result{}, err
	}

	return model.Result{
		Ref:    snap.Ref,
		Commit: snap.Commit,
		Paths:  paths,
		Output: output,
		Files:  written,
		Bytes:  size,
	}, nil
}
