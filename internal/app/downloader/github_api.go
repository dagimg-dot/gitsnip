package downloader

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dagimg-dot/gitsnip/internal/app/model"
	"github.com/dagimg-dot/gitsnip/internal/apperr"
	"github.com/dagimg-dot/gitsnip/internal/pathspec"
	"github.com/dagimg-dot/gitsnip/internal/util"
)

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

const (
	GitHubAPIBaseURL = "https://api.github.com"
	GitHubRawBaseURL = "https://raw.githubusercontent.com"
)

type gitHubAPIDownloader struct {
	client  HTTPDoer
	apiURL  string
	rawURL  string
	workers int
}

func NewGitHubAPIDownloader(client HTTPDoer) Downloader {
	return &gitHubAPIDownloader{client: client, apiURL: GitHubAPIBaseURL, rawURL: GitHubRawBaseURL, workers: 8}
}

type githubClient struct {
	doer   HTTPDoer
	apiURL string
	rawURL string
	token  string
	owner  string
	repo   string
	rep    model.Reporter
}

type treeEntry struct {
	Path string `json:"path"`
	Mode string `json:"mode"`
	Type string `json:"type"`
	SHA  string `json:"sha"`
}

type tree struct {
	Entries   []treeEntry `json:"tree"`
	Truncated bool        `json:"truncated"`
}

func (g *gitHubAPIDownloader) Download(ctx context.Context, req model.Request, dir string, rep model.Reporter) (model.Snapshot, error) {
	if !req.Source.GitHub() {
		return model.Snapshot{}, apperr.Wrap(apperr.ErrUnsupported, nil,
			"The api method only works with github.com", "use --method sparse for other hosts")
	}
	c := &githubClient{doer: g.client, apiURL: g.apiURL, rawURL: g.rawURL, token: req.Token, owner: req.Source.Owner, repo: req.Source.Repo, rep: rep}

	rep.Stage("resolving the branch")
	ref, sha, paths, err := c.resolve(ctx, req)
	if err != nil {
		return model.Snapshot{}, err
	}

	rep.Stage("listing files")
	t, err := c.tree(ctx, sha)
	if err != nil {
		return model.Snapshot{}, err
	}
	if t.Truncated {
		return model.Snapshot{}, apperr.Wrap(apperr.ErrUnsupported, nil,
			fmt.Sprintf("%s is too large for the api method", c.slug()), "use --method sparse")
	}

	files, submodules := pick(t.Entries, paths)
	for _, path := range submodules {
		rep.Warn(fmt.Sprintf("skipped submodule %s (submodules aren't downloaded)", path))
	}
	if err := g.fetchAll(ctx, c, sha, files, dir); err != nil {
		return model.Snapshot{}, err
	}

	var all []string
	for _, e := range t.Entries {
		if e.Type == "blob" || e.Type == "commit" {
			all = append(all, e.Path)
		}
	}
	list := func(context.Context) ([]string, error) { return all, nil }

	return model.Snapshot{Dir: dir, Ref: ref, Commit: sha, Paths: paths, List: list}, nil
}

func (c *githubClient) resolve(ctx context.Context, req model.Request) (string, string, []pathspec.Pattern, error) {
	if req.Source.RefPath == "" {
		ref := req.Ref
		if ref == "" {
			var err error
			if ref, err = c.defaultBranch(ctx); err != nil {
				return "", "", nil, err
			}
		}
		sha, err := c.commit(ctx, ref)
		return ref, sha, req.Paths, err
	}

	segs := strings.Split(req.Source.RefPath, "/")
	for i := 1; i <= len(segs); i++ {
		ref := strings.Join(segs[:i], "/")
		sha, found, err := c.lookupCommit(ctx, ref)
		if err != nil {
			return "", "", nil, err
		}
		if found {
			paths, err := withPath(req.Paths, strings.Join(segs[i:], "/"))
			return ref, sha, paths, err
		}
	}
	return "", "", nil, c.missingRef(ctx, segs[0])
}

func pick(entries []treeEntry, paths []pathspec.Pattern) (files []treeEntry, submodules []string) {
	for _, e := range entries {
		if (e.Type != "blob" && e.Type != "commit") || !matchesAny(paths, e.Path) {
			continue
		}
		if e.Type == "commit" {
			submodules = append(submodules, e.Path)
			continue
		}
		if filepath.IsLocal(filepath.FromSlash(e.Path)) {
			files = append(files, e)
		}
	}
	return files, submodules
}

func matchesAny(paths []pathspec.Pattern, file string) bool {
	if len(paths) == 0 {
		return true
	}
	for _, p := range paths {
		if p.Match(file) {
			return true
		}
	}
	return false
}

func (g *gitHubAPIDownloader) fetchAll(ctx context.Context, c *githubClient, sha string, files []treeEntry, dir string) error {
	total := len(files)
	if total == 0 {
		return nil
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	c.rep.Stage("downloading")
	c.rep.Progress(0, total)

	jobs := make(chan treeEntry)
	var (
		wg       sync.WaitGroup
		done     atomic.Int64
		once     sync.Once
		firstErr error
	)
	for range min(g.workers, total) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for e := range jobs {
				if err := c.fetch(ctx, sha, e, dir); err != nil {
					once.Do(func() {
						firstErr = err
						cancel()
					})
					return
				}
				c.rep.Progress(int(done.Add(1)), total)
			}
		}()
	}

feed:
	for _, e := range files {
		select {
		case jobs <- e:
		case <-ctx.Done():
			break feed
		}
	}
	close(jobs)
	wg.Wait()

	if firstErr != nil {
		return firstErr
	}
	return ctx.Err()
}

func (c *githubClient) slug() string {
	return c.owner + "/" + c.repo
}

func (c *githubClient) get(ctx context.Context, target, accept string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "gitsnip")
	req.Header.Set("Accept", accept)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if c.token != "" {
		req.Header.Set("Authorization", "token "+c.token)
	}

	start := time.Now()
	resp, err := c.doer.Do(req)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		host := req.URL.Hostname()
		return nil, apperr.Wrap(apperr.ErrNetworkFailure, err, fmt.Sprintf("Couldn't reach %s", host), "check your connection and try again")
	}
	c.rep.Debug(fmt.Sprintf("GET %s %d (%s)", target, resp.StatusCode, time.Since(start).Round(time.Millisecond)))
	return resp, nil
}

func (c *githubClient) getJSON(ctx context.Context, target string, into any) (int, error) {
	resp, err := c.get(ctx, target, "application/vnd.github+json")
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, c.failure(resp)
	}
	if err := json.NewDecoder(resp.Body).Decode(into); err != nil {
		return resp.StatusCode, fmt.Errorf("failed to parse the GitHub API response: %w", err)
	}
	return resp.StatusCode, nil
}

func (c *githubClient) defaultBranch(ctx context.Context) (string, error) {
	var repo struct {
		DefaultBranch string `json:"default_branch"`
	}
	status, err := c.getJSON(ctx, fmt.Sprintf("%s/repos/%s", c.apiURL, c.escapedSlug()), &repo)
	if status == http.StatusNotFound {
		return "", apperr.Wrap(apperr.ErrRepositoryNotFound, err,
			fmt.Sprintf("Repository %s doesn't exist or is private", c.slug()),
			"check the name; for private repositories set GITHUB_TOKEN or pass --token")
	}
	if err != nil {
		return "", err
	}
	return repo.DefaultBranch, nil
}

func (c *githubClient) commit(ctx context.Context, ref string) (string, error) {
	sha, found, err := c.lookupCommit(ctx, ref)
	if err != nil || found {
		return sha, err
	}
	return "", c.missingRef(ctx, ref)
}

func (c *githubClient) lookupCommit(ctx context.Context, ref string) (string, bool, error) {
	resp, err := c.get(ctx, fmt.Sprintf("%s/repos/%s/commits/%s", c.apiURL, c.escapedSlug(), escapePath(ref)), "application/vnd.github.sha")
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		body, err := io.ReadAll(io.LimitReader(resp.Body, 256))
		if err != nil {
			return "", false, fmt.Errorf("failed to read the commit: %w", err)
		}
		return strings.TrimSpace(string(body)), true, nil
	case http.StatusNotFound, http.StatusUnprocessableEntity:
		return "", false, nil
	default:
		return "", false, c.failure(resp)
	}
}

func (c *githubClient) missingRef(ctx context.Context, ref string) error {
	branch, err := c.defaultBranch(ctx)
	if err != nil {
		return err
	}
	missing := apperr.Wrap(apperr.ErrRefNotFound, nil, fmt.Sprintf("Branch or tag %q doesn't exist in %s", ref, c.slug()), "")
	if branch != ref {
		missing.Hint = fmt.Sprintf("the default branch is %q", branch)
	}
	return missing
}

func (c *githubClient) tree(ctx context.Context, sha string) (tree, error) {
	var t tree
	_, err := c.getJSON(ctx, fmt.Sprintf("%s/repos/%s/git/trees/%s?recursive=1", c.apiURL, c.escapedSlug(), sha), &t)
	return t, err
}

func (c *githubClient) fetch(ctx context.Context, sha string, e treeEntry, dir string) error {
	target := filepath.Join(dir, filepath.FromSlash(e.Path))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("failed to create a directory for %s: %w", e.Path, err)
	}

	if e.Mode == "120000" {
		return c.fetchSymlink(ctx, e, target)
	}

	resp, err := c.get(ctx, fmt.Sprintf("%s/%s/%s/%s", c.rawURL, c.escapedSlug(), sha, escapePath(e.Path)), "*/*")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return c.failure(resp)
	}

	perm := os.FileMode(0o644)
	if e.Mode == "100755" {
		perm = 0o755
	}
	return util.SaveToFile(target, resp.Body, perm)
}

func (c *githubClient) fetchSymlink(ctx context.Context, e treeEntry, target string) error {
	resp, err := c.get(ctx, fmt.Sprintf("%s/repos/%s/git/blobs/%s", c.apiURL, c.escapedSlug(), e.SHA), "application/vnd.github.raw")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return c.failure(resp)
	}

	link, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return fmt.Errorf("failed to read symlink %s: %w", e.Path, err)
	}
	if err := os.Symlink(string(link), target); err != nil {
		c.rep.Warn(fmt.Sprintf("skipped symlink %s (symlinks can't be created here)", e.Path))
	}
	return nil
}

func (c *githubClient) failure(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	message := strings.ToLower(string(body))
	cause := fmt.Errorf("GitHub responded %s: %s", resp.Status, strings.TrimSpace(string(body)))

	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return apperr.Wrap(apperr.ErrAuthenticationRequired, cause,
			"GitHub rejected the token", "check GITHUB_TOKEN or the value passed to --token")
	case resp.StatusCode == http.StatusTooManyRequests,
		resp.StatusCode == http.StatusForbidden && (resp.Header.Get("X-RateLimit-Remaining") == "0" || strings.Contains(message, "rate limit")):
		return apperr.Wrap(apperr.ErrRateLimitExceeded, cause, "GitHub's API rate limit is used up", c.rateLimitHint(resp))
	case resp.StatusCode == http.StatusForbidden:
		return apperr.Wrap(apperr.ErrAuthenticationRequired, cause,
			fmt.Sprintf("GitHub denied access to %s", c.slug()), "check that the token can read this repository")
	case resp.StatusCode >= 500:
		return apperr.Wrap(apperr.ErrNetworkFailure, cause,
			fmt.Sprintf("GitHub responded %s", resp.Status), "try again in a moment")
	}
	return apperr.Wrap(errors.New("unexpected GitHub response"), cause, fmt.Sprintf("GitHub responded %s", resp.Status), "")
}

func (c *githubClient) rateLimitHint(resp *http.Response) string {
	hint := "use --method sparse, which isn't rate limited"
	if c.token == "" {
		hint = "set GITHUB_TOKEN to raise the limit, or " + hint
	}
	if reset, err := strconv.ParseInt(resp.Header.Get("X-RateLimit-Reset"), 10, 64); err == nil {
		hint = fmt.Sprintf("it resets at %s; %s", time.Unix(reset, 0).Format("15:04"), hint)
	}
	return hint
}

func (c *githubClient) escapedSlug() string {
	return url.PathEscape(c.owner) + "/" + url.PathEscape(c.repo)
}

func escapePath(p string) string {
	segments := strings.Split(p, "/")
	for i, segment := range segments {
		segments[i] = url.PathEscape(segment)
	}
	return strings.Join(segments, "/")
}
