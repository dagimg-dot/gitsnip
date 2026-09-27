package downloader

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
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

	rep.Stage("cloning")
	paths, rev, err := g.obtain(ctx, repoDir, req.Source.RefPath, req.Paths)
	if err == nil {
		err = g.checkout(ctx, repoDir, rev, paths)
	}
	if err != nil {
		return model.Snapshot{}, err
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

type attempt struct {
	rev string
	err error
}

func (g *gitSession) obtain(ctx context.Context, repoDir, refPath string, paths []pathspec.Pattern) ([]pathspec.Pattern, string, error) {
	if refPath != "" {
		return g.obtainLink(ctx, repoDir, refPath, paths)
	}
	rev, err := g.fetch(ctx, repoDir, paths)
	if err != nil {
		return nil, "", g.failure(ctx, err)
	}
	return paths, rev, nil
}

// obtainLink clones the link's first segment while ls-remote looks for the
// longest matching branch or tag, since most links name a one-segment ref.
// If a longer ref matches, the guess is discarded and that ref is cloned.
func (g *gitSession) obtainLink(ctx context.Context, repoDir, refPath string, paths []pathspec.Pattern) ([]pathspec.Pattern, string, error) {
	first, extra, _ := strings.Cut(refPath, "/")
	guess := *g
	guess.ref = first
	guessed := make(chan attempt, 1)
	go func() {
		guessed <- guess.try(ctx, repoDir, paths, extra)
	}()

	ref, rest, err := g.splitRefPath(ctx, refPath)
	early := <-guessed
	if err != nil {
		return nil, "", err
	}
	g.ref = ref
	if paths, err = withPath(paths, rest); err != nil {
		return nil, "", err
	}

	rev := early.rev
	if ref == first {
		err = early.err
	} else if err = os.RemoveAll(repoDir); err == nil {
		rev, err = g.fetch(ctx, repoDir, paths)
	}
	if err != nil {
		return nil, "", g.failure(ctx, err)
	}
	return paths, rev, nil
}

func (g *gitSession) try(ctx context.Context, repoDir string, paths []pathspec.Pattern, extra string) attempt {
	paths, err := withPath(paths, extra)
	if err != nil {
		return attempt{err: err}
	}
	rev, err := g.fetch(ctx, repoDir, paths)
	return attempt{rev: rev, err: err}
}

func (g *gitSession) fetch(ctx context.Context, repoDir string, paths []pathspec.Pattern) (string, error) {
	partial := sparseRules(paths) != nil
	if isCommitID(g.ref) {
		return "FETCH_HEAD", g.fetchCommit(ctx, repoDir, partial)
	}
	return "HEAD", g.clone(ctx, repoDir, partial)
}

func (g *gitSession) checkout(ctx context.Context, repoDir, rev string, paths []pathspec.Pattern) error {
	if rules := sparseRules(paths); rules != nil {
		if err := g.restrict(ctx, repoDir, rules); err != nil {
			return err
		}
	}
	g.rep.Stage("fetching " + describePaths(paths))
	if _, err := g.run(ctx, repoDir, "read-tree", "-mu", rev); err != nil {
		return g.failure(ctx, err)
	}
	return nil
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
	for line := range strings.SplitSeq(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		name := strings.TrimSuffix(fields[1], "^{}")
		for _, prefix := range []string{"refs/heads/", "refs/tags/"} {
			if after, ok := strings.CutPrefix(name, prefix); ok {
				refs[after] = true
			}
		}
	}

	for i, candidate := range slices.Backward(candidates) {
		if refs[candidate] {
			return candidate, strings.Join(segs[i+1:], "/"), nil
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

// clone with partial set fetches commits and trees but no file contents.
// read-tree later downloads only the blobs the sparse rules select, in a
// single batch. A whole-repository download skips the filter, since one plain
// shallow clone is faster than a second request naming every blob.
func (g *gitSession) clone(ctx context.Context, repoDir string, partial bool) error {
	args := []string{"clone", "--quiet", "--depth=1"}
	if partial {
		args = append(args, "--filter=blob:none")
	}
	args = append(args, "--no-checkout", "--no-tags")
	if g.ref != "" {
		args = append(args, "--branch", g.ref)
	}
	_, err := g.run(ctx, "", append(args, "--", g.url, repoDir)...)
	return err
}

// fetchCommit sets up the clone by hand because clone --branch only accepts
// branch and tag names, not commit hashes.
func (g *gitSession) fetchCommit(ctx context.Context, repoDir string, partial bool) error {
	if _, err := g.run(ctx, "", "init", "--quiet", "--", repoDir); err != nil {
		return err
	}
	steps := [][]string{{"remote", "add", "--", "origin", g.url}}
	fetchArgs := []string{"fetch", "--quiet", "--depth=1"}
	if partial {
		steps = append(steps,
			[]string{"config", "core.repositoryformatversion", "1"},
			[]string{"config", "extensions.partialclone", "origin"},
			[]string{"config", "remote.origin.promisor", "true"},
			[]string{"config", "remote.origin.partialclonefilter", "blob:none"},
		)
		fetchArgs = append(fetchArgs, "--filter=blob:none")
	}
	steps = append(steps, append(fetchArgs, "--no-tags", "origin", g.ref))
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
	for line := range strings.SplitSeq(out, "\n") {
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
		rules = append(rules, attributeRules(p)...)
	}
	slices.Sort(rules)
	return slices.Compact(rules)
}

// attributeRules covers the .gitattributes files git reads while checking
// out the pattern's files. Outside the sparse rules, git would fetch each
// one in its own round trip instead of in the batch.
func attributeRules(p pathspec.Pattern) []string {
	base := p.String()
	if p.IsGlob() {
		base = p.Anchor(false)
	}
	var rules []string
	dir := ""
	for segment := range strings.SplitSeq(base, "/") {
		rules = append(rules, "/"+path.Join(dir, ".gitattributes"))
		dir = path.Join(dir, segment)
	}
	if p.IsGlob() {
		rules = append(rules, "/"+path.Join(base, "**", ".gitattributes"))
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
