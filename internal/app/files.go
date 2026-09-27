package app

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dagimg-dot/gitsnip/internal/app/model"
	"github.com/dagimg-dot/gitsnip/internal/apperr"
	"github.com/dagimg-dot/gitsnip/internal/pathspec"
	"github.com/dagimg-dot/gitsnip/internal/util"
)

type selection struct {
	files  []string
	base   string
	single bool
}

func listFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil || rel == "." {
			return err
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if rel == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type().IsRegular() || d.Type()&fs.ModeSymlink != 0 {
			files = append(files, rel)
		}
		return nil
	})
	sort.Strings(files)
	return files, err
}

func selectFiles(patterns []pathspec.Pattern, files []string) (selection, error) {
	if len(patterns) == 0 {
		patterns = []pathspec.Pattern{{}}
	}

	chosen := map[string]bool{}
	anchors := make([]string, 0, len(patterns))
	single := len(patterns) == 1
	for _, p := range patterns {
		matched, exact := 0, false
		for _, file := range files {
			if !p.Match(file) {
				continue
			}
			chosen[file] = true
			matched++
			exact = exact || p.IsExactly(file)
		}
		if matched == 0 {
			return selection{}, missing(p)
		}
		anchors = append(anchors, p.Anchor(exact))
		single = single && exact
	}

	sel := selection{base: pathspec.CommonDir(anchors), single: single}
	for file := range chosen {
		sel.files = append(sel.files, file)
	}
	sort.Strings(sel.files)
	return sel, nil
}

func missing(p pathspec.Pattern) error {
	switch {
	case p.IsAll():
		return apperr.Wrap(apperr.ErrPathNotFound, nil, "The repository is empty", "")
	case p.IsGlob():
		return apperr.Wrap(apperr.ErrPathNotFound, nil,
			fmt.Sprintf("No files match %q", p), "check the pattern, and quote it so your shell doesn't expand it")
	default:
		return apperr.Wrap(apperr.ErrPathNotFound, nil,
			fmt.Sprintf("Path %q doesn't exist in the repository", p), "check the path and the branch")
	}
}

func defaultOutput(sel selection, repo string) string {
	switch {
	case sel.single:
		return "."
	case sel.base != "":
		return path.Base(sel.base)
	case repo != "":
		return repo
	default:
		return "snip"
	}
}

func writeFiles(ctx context.Context, root string, sel selection, output string, force bool, rep model.Reporter) (int, int64, error) {
	info, statErr := os.Stat(output)
	existed := statErr == nil
	if existed && !info.IsDir() {
		return 0, 0, apperr.Wrap(apperr.ErrDestinationExists, nil,
			fmt.Sprintf("%s exists and isn't a folder", displayPath(output)), "pick another folder with -o")
	}
	if existed && !force {
		if err := checkConflicts(sel, output); err != nil {
			return 0, 0, err
		}
	}

	written, size, err := copySelection(ctx, root, sel, output, rep)
	if err != nil && !existed {
		os.RemoveAll(output)
	}
	return written, size, err
}

func copySelection(ctx context.Context, root string, sel selection, output string, rep model.Reporter) (int, int64, error) {
	base := filepath.Join(root, filepath.FromSlash(sel.base))
	written, size := 0, int64(0)

	for _, file := range sel.files {
		if err := ctx.Err(); err != nil {
			return written, size, err
		}

		src := filepath.Join(root, filepath.FromSlash(file))
		dst := filepath.Join(output, filepath.FromSlash(relativeTo(sel.base, file)))
		info, err := os.Lstat(src)
		if err != nil {
			return written, size, err
		}

		if info.Mode()&os.ModeSymlink != 0 {
			kept := true
			err := util.CopySymlink(src, dst, base, func(_, reason string) {
				kept = false
				rep.Warn(fmt.Sprintf("skipped symlink %s (%s)", file, reason))
			})
			if err != nil {
				return written, size, err
			}
			if kept {
				written++
			}
			continue
		}

		if err := util.CopyFile(src, dst); err != nil {
			return written, size, err
		}
		written++
		size += info.Size()
	}

	return written, size, nil
}

func checkConflicts(sel selection, output string) error {
	var clashes []string
	for _, file := range sel.files {
		dst := filepath.Join(output, filepath.FromSlash(relativeTo(sel.base, file)))
		if _, err := os.Lstat(dst); err == nil {
			clashes = append(clashes, dst)
		}
	}

	switch {
	case len(clashes) == 0:
		return nil
	case sel.single:
		return apperr.Wrap(apperr.ErrDestinationExists, nil,
			fmt.Sprintf("%s already exists", displayPath(clashes[0])), "pass --force to overwrite it, or -o to write somewhere else")
	default:
		return apperr.Wrap(apperr.ErrDestinationExists, nil,
			fmt.Sprintf("%s already has %d of these files", displayPath(output), len(clashes)), "pass --force to overwrite, or -o to write somewhere else")
	}
}

func displayPath(p string) string {
	sep := string(filepath.Separator)
	if filepath.IsAbs(p) || p == "." || p == ".." || strings.HasPrefix(p, "."+sep) || strings.HasPrefix(p, ".."+sep) {
		return p
	}
	return "." + sep + p
}

func relativeTo(base, file string) string {
	if base == "" {
		return file
	}
	return strings.TrimPrefix(file, base+"/")
}
