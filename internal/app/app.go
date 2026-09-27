package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dagimg-dot/gitsnip/internal/app/downloader"
	"github.com/dagimg-dot/gitsnip/internal/app/model"
)

func Download(ctx context.Context, req model.Request, rep model.Reporter) (model.Result, error) {
	dl, method, err := downloader.GetDownloader(req)
	if err != nil {
		return model.Result{}, err
	}
	res, err := run(ctx, dl, req, rep)
	res.Method = method
	return res, err
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

	sel, gap := selectFiles(paths, files)
	if gap != nil {
		return model.Result{}, missing(ctx, *gap, req, snap)
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

	target := output
	if sel.single {
		target = filepath.Join(output, filepath.FromSlash(relativeTo(sel.base, sel.files[0])))
	}

	return model.Result{
		Ref:    snap.Ref,
		Commit: snap.Commit,
		Paths:  paths,
		Output: output,
		Target: target,
		Files:  written,
		Bytes:  size,
	}, nil
}
