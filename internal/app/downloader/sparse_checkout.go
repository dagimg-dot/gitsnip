package downloader

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dagimg-dot/gitsnip/internal/app/model"
	"github.com/dagimg-dot/gitsnip/internal/apperr"
	"github.com/dagimg-dot/gitsnip/internal/pathspec"
)

type GitRunner interface {
	Run(ctx context.Context, dir string, env []string, args ...string) (string, error)
	HasGit() bool
}

type sparseCheckoutDownloader struct {
	runner GitRunner
}

func NewSparseCheckoutDownloader(runner GitRunner) Downloader {
	return &sparseCheckoutDownloader{runner: runner}
}

type gitSession struct {
	runner GitRunner
	rep    model.Reporter
	env    []string
	url    string
	repo   string
	ref    string
}

func (s *sparseCheckoutDownloader) Download(ctx context.Context, req *model.Request, dir string, rep model.Reporter) (model.Snapshot, error) {
	if !s.runner.HasGit() {
		return model.Snapshot{}, apperr.Wrap(apperr.ErrGitNotInstalled, nil,
			"Git isn't installed", "install git, or use --method api for GitHub repositories")
	}

	g := &gitSession{runner: s.runner, rep: rep, env: authEnv(req.Source.URL, req.Token), url: req.Source.URL, repo: req.Source.Display(), ref: req.Ref}
	repoDir := filepath.Join(dir, "repo")

	paths := req.Paths
	if req.Source.RefPath != "" {
		rep.Stage("resolving the branch")
		ref, rest, err := g.splitRefPath(ctx, req.Source.RefPath)
		if err != nil {
			return model.Snapshot{}, err
		}
		g.ref = ref
		if paths, err = withPath(paths, rest); err != nil {
			return model.Snapshot{}, err
		}
	}

	rep.Stage("cloning")
	rev, obtain := "HEAD", g.clone
	if isCommitID(g.ref) {
		rev, obtain = "FETCH_HEAD", g.fetchCommit
	}
	if err := obtain(ctx, repoDir); err != nil {
		return model.Snapshot{}, g.failure(ctx, err)
	}

	if rules := sparseRules(paths); rules != nil {
		if err := g.restrict(ctx, repoDir, rules); err != nil {
			return model.Snapshot{}, err
		}
	}

	rep.Stage("fetching " + describePaths(paths))
	if _, err := g.run(ctx, repoDir, "read-tree", "-mu", rev); err != nil {
		return model.Snapshot{}, g.failure(ctx, err)
	}

	commit, err := g.run(ctx, repoDir, "rev-parse", rev)
	if err != nil {
		return model.Snapshot{}, g.failure(ctx, err)
	}

	ref := g.ref
	if ref == "" {
		if name, err := g.run(ctx, repoDir, "symbolic-ref", "--quiet", "--short", "HEAD"); err == nil {
			ref = strings.TrimSpace(name)
		}
	}

	commit = strings.TrimSpace(commit)
	list := func(ctx context.Context) ([]string, error) {
		out, err := g.run(ctx, repoDir, "ls-tree", "-r", "-z", "--name-only", commit)
		if err != nil {
			return nil, err
		}
		return strings.Split(strings.TrimSuffix(out, "\x00"), "\x00"), nil
	}

	return model.Snapshot{Dir: repoDir, Ref: ref, Commit: commit, Paths: paths, List: list}, nil
}

func (g *gitSession) splitRefPath(ctx context.Context, refPath string) (ref, rest string, err error) {
	segs := strings.Split(refPath, "/")
	candidates := make([]string, len(segs))
	for i := range segs {
		candidates[i] = strings.Join(segs[:i+1], "/")
	}

	out, err := g.run(ctx, "", append([]string{"ls-remote", "--", g.url}, candidates...)...)
	if err != nil {
		return "", "", g.failure(ctx, err)
	}
	refs := map[string]bool{}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		name := strings.TrimSuffix(fields[1], "^{}")
		for _, prefix := range []string{"refs/heads/", "refs/tags/"} {
			if strings.HasPrefix(name, prefix) {
				refs[strings.TrimPrefix(name, prefix)] = true
			}
		}
	}

	for i := len(candidates) - 1; i >= 0; i-- {
		if refs[candidates[i]] {
			return candidates[i], strings.Join(segs[i+1:], "/"), nil
		}
	}
	if isCommitID(segs[0]) {
		return segs[0], strings.Join(segs[1:], "/"), nil
	}

	missing := apperr.Wrap(apperr.ErrRefNotFound, nil, fmt.Sprintf("Branch or tag %q doesn't exist in %s", segs[0], g.repo), "")
	if branch := g.defaultBranch(ctx); branch != "" {
		missing.Hint = fmt.Sprintf("the default branch is %q", branch)
	}
	return "", "", missing
}

func withPath(paths []pathspec.Pattern, extra string) ([]pathspec.Pattern, error) {
	if extra == "" {
		return paths, nil
	}
	p, err := pathspec.Parse(extra)
	if err != nil {
		return nil, apperr.Wrap(apperr.ErrInvalidURL, err, err.Error(), "")
	}
	return append(append([]pathspec.Pattern{}, paths...), p), nil
}

func (g *gitSession) run(ctx context.Context, dir string, args ...string) (string, error) {
	start := time.Now()
	out, err := g.runner.Run(ctx, dir, g.env, args...)
	g.rep.Debug(fmt.Sprintf("git %s (%s)", strings.Join(args, " "), time.Since(start).Round(time.Millisecond)))
	return out, err
}

// clone fetches commits and trees but no file contents. read-tree later
// downloads only the blobs the sparse rules select, in a single batch.
func (g *gitSession) clone(ctx context.Context, repoDir string) error {
	args := []string{"clone", "--quiet", "--depth=1", "--filter=blob:none", "--no-checkout", "--no-tags"}
	if g.ref != "" {
		args = append(args, "--branch", g.ref)
	}
	_, err := g.run(ctx, "", append(args, "--", g.url, repoDir)...)
	return err
}

// fetchCommit sets up the partial clone by hand because clone --branch only
// accepts branch and tag names, not commit hashes.
func (g *gitSession) fetchCommit(ctx context.Context, repoDir string) error {
	if _, err := g.run(ctx, "", "init", "--quiet", "--", repoDir); err != nil {
		return err
	}
	steps := [][]string{
		{"remote", "add", "--", "origin", g.url},
		{"config", "core.repositoryformatversion", "1"},
		{"config", "extensions.partialclone", "origin"},
		{"config", "remote.origin.promisor", "true"},
		{"config", "remote.origin.partialclonefilter", "blob:none"},
		{"fetch", "--quiet", "--depth=1", "--filter=blob:none", "--no-tags", "origin", g.ref},
	}
	for _, args := range steps {
		if _, err := g.run(ctx, repoDir, args...); err != nil {
			return err
		}
	}
	return nil
}

// restrict writes non-cone sparse rules straight to .git/info/sparse-checkout.
// Cone mode can't express single files or globs, and older git releases lack
// "sparse-checkout set --no-cone".
func (g *gitSession) restrict(ctx context.Context, repoDir string, rules []string) error {
	for _, args := range [][]string{
		{"config", "core.sparseCheckout", "true"},
		{"config", "core.sparseCheckoutCone", "false"},
	} {
		if _, err := g.run(ctx, repoDir, args...); err != nil {
			return g.failure(ctx, err)
		}
	}

	info := filepath.Join(repoDir, ".git", "info")
	if err := os.MkdirAll(info, 0o750); err != nil {
		return fmt.Errorf("failed to prepare the sparse checkout: %w", err)
	}
	if err := os.WriteFile(filepath.Join(info, "sparse-checkout"), []byte(strings.Join(rules, "\n")+"\n"), 0o600); err != nil {
		return fmt.Errorf("failed to prepare the sparse checkout: %w", err)
	}
	return nil
}

func (g *gitSession) failure(ctx context.Context, err error) error {
	failure := gitFailure(err, g.repo, g.ref)
	var appErr *apperr.Error
	if g.ref == "" || !errors.Is(failure, apperr.ErrRefNotFound) || !errors.As(failure, &appErr) {
		return failure
	}
	if branch := g.defaultBranch(ctx); branch != "" && branch != g.ref {
		appErr.Hint = fmt.Sprintf("the default branch is %q", branch)
	}
	return failure
}

func (g *gitSession) defaultBranch(ctx context.Context) string {
	out, err := g.run(ctx, "", "ls-remote", "--symref", "--", g.url, "HEAD")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 3 && fields[0] == "ref:" && fields[2] == "HEAD" {
			return strings.TrimPrefix(fields[1], "refs/heads/")
		}
	}
	return ""
}

func isCommitID(ref string) bool {
	if len(ref) != 40 && len(ref) != 64 {
		return false
	}
	for _, c := range ref {
		if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
			return false
		}
	}
	return true
}

func sparseRules(paths []pathspec.Pattern) []string {
	var rules []string
	for _, p := range paths {
		if p.IsAll() {
			return nil
		}
		rules = append(rules, p.Rule())
	}
	return rules
}

func describePaths(paths []pathspec.Pattern) string {
	switch {
	case len(paths) == 0, len(paths) == 1 && paths[0].IsAll():
		return "everything"
	case len(paths) == 1:
		return paths[0].String()
	default:
		return fmt.Sprintf("%d paths", len(paths))
	}
}
