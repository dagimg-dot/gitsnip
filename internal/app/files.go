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
	"github.com/dagimg-dot/gitsnip/internal/source"
	"github.com/dagimg-dot/gitsnip/internal/util"
)

type selection struct {
	files  []string
	base   string
	single bool
}

type tally struct {
	files int
	bytes int64
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

func selectFiles(patterns []pathspec.Pattern, files []string) (selection, *pathspec.Pattern) {
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
			return selection{}, &p
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

func missing(ctx context.Context, p pathspec.Pattern, req *model.Request, snap *model.Snapshot) error {
	where := location(&req.Source, snap.Ref)
	switch {
	case p.IsAll():
		return apperr.Wrap(apperr.ErrPathNotFound, nil, fmt.Sprintf("There's nothing to download in %s", where), "")
	case p.IsGlob():
		return apperr.Wrap(apperr.ErrPathNotFound, nil,
			fmt.Sprintf("No files match %q in %s", p, where), "check the pattern, and quote it so your shell doesn't expand it")
	}

	hint := "check the path and the branch"
	if all, err := snap.List(ctx); err == nil {
		if guess := pathspec.Suggest(p.String(), all); guess != "" {
			hint = fmt.Sprintf("did you mean %s?", guess)
		}
	}
	return apperr.Wrap(apperr.ErrPathNotFound, nil, fmt.Sprintf("Path %q doesn't exist in %s", p, where), hint)
}

func location(src *source.Source, ref string) string {
	name := src.Display()
	switch {
	case name == "":
		return "the repository"
	case ref == "":
		return name
	default:
		return name + "@" + ref
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

func writeFiles(ctx context.Context, root string, sel selection, output string, force bool, rep model.Reporter) (tally, error) {
	info, statErr := os.Stat(output)
	existed := statErr == nil
	if existed && !info.IsDir() {
		return tally{}, apperr.Wrap(apperr.ErrDestinationExists, nil,
			fmt.Sprintf("%s exists and isn't a folder", DisplayPath(output)), "pick another folder with -o")
	}
	if existed && !force {
		if err := checkConflicts(sel, output); err != nil {
			return tally{}, err
		}
	}

	written, err := copySelection(ctx, root, sel, output, rep)
	if err != nil && !existed {
		_ = os.RemoveAll(output)
	}
	return written, err
}

func copySelection(ctx context.Context, root string, sel selection, output string, rep model.Reporter) (tally, error) {
	base := filepath.Join(root, filepath.FromSlash(sel.base))
	var written tally

	for _, file := range sel.files {
		if err := ctx.Err(); err != nil {
			return written, err
		}

		src := filepath.Join(root, filepath.FromSlash(file))
		dst := filepath.Join(output, filepath.FromSlash(relativeTo(sel.base, file)))
		info, err := os.Lstat(src)
		if err != nil {
			return written, err
		}

		if info.Mode()&os.ModeSymlink != 0 {
			skipped, err := util.CopySymlink(src, dst, base)
			switch {
			case err != nil:
				return written, err
			case skipped != "":
				rep.Warn(fmt.Sprintf("skipped symlink %s (%s)", file, skipped))
			default:
				written.files++
			}
			continue
		}

		if err := util.CopyFile(src, dst); err != nil {
			return written, err
		}
		written.files++
		written.bytes += info.Size()
	}

	return written, nil
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
			fmt.Sprintf("%s already exists", DisplayPath(clashes[0])), "pass --force to overwrite it, or -o to write somewhere else")
	default:
		return apperr.Wrap(apperr.ErrDestinationExists, nil,
			fmt.Sprintf("%s already has %d of these files", DisplayPath(output), len(clashes)), "pass --force to overwrite, or -o to write somewhere else")
	}
}

func DisplayPath(p string) string {
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
