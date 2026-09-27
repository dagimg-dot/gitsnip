package downloader_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/dagimg-dot/gitsnip/internal/app/downloader"
	"github.com/dagimg-dot/gitsnip/internal/app/gitutil"
	"github.com/dagimg-dot/gitsnip/internal/app/model"
	"github.com/dagimg-dot/gitsnip/internal/apperr"
	"github.com/dagimg-dot/gitsnip/internal/pathspec"
	"github.com/dagimg-dot/gitsnip/internal/source"
)

type fixture struct {
	url     string
	head    string
	feature string
	bigBlob string
}

var fixtureFiles = map[string]string{
	".gitattributes":                "*.bin -diff\n",
	"README.md":                     "# fixture\n",
	"src/.gitattributes":            "*.svg -diff\n",
	"src/components/Button.tsx":     "export const Button = () => null\n",
	"src/components/icons/star.svg": "<svg/>\n",
	"data/usage.txt":                "usage\n",
	"data/a_linux.json":             "{}\n",
	"data/b_linux.json":             "{}\n",
	"data/c_mac.json":               "{}\n",
	"big/blob.bin":                  strings.Repeat("x", 1<<20),
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	if !gitutil.IsGitInstalled() {
		t.Skip("git is not installed")
	}

	root := t.TempDir()
	work := filepath.Join(root, "work")
	bare := filepath.Join(root, "repo.git")
	git := func(dir string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "commit.gpgsign=false", "-c", "tag.gpgsign=false"}, args...)...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}

	for name, content := range fixtureFiles {
		full := filepath.Join(work, filepath.FromSlash(name))
		os.MkdirAll(filepath.Dir(full), 0o755)
		os.WriteFile(full, []byte(content), 0o644)
	}
	git(root, "init", "--quiet", "--initial-branch=trunk", work)
	git(work, "add", "-A")
	git(work, "commit", "--quiet", "-m", "init")
	git(work, "tag", "v1")
	git(work, "switch", "--quiet", "-c", "feature/x")
	os.WriteFile(filepath.Join(work, "src", "components", "Feature.tsx"), []byte("feature\n"), 0o644)
	git(work, "add", "-A")
	git(work, "commit", "--quiet", "-m", "feature")
	git(work, "switch", "--quiet", "trunk")

	git(root, "init", "--quiet", "--bare", bare)
	git(bare, "config", "uploadpack.allowFilter", "true")
	git(bare, "config", "uploadpack.allowAnySHA1InWant", "true")
	git(work, "push", "--quiet", bare, "trunk", "feature/x", "v1")
	git(bare, "symbolic-ref", "HEAD", "refs/heads/trunk")

	return fixture{
		url:     "file://" + filepath.ToSlash(bare),
		head:    git(work, "rev-parse", "trunk"),
		feature: git(work, "rev-parse", "feature/x"),
		bigBlob: git(work, "rev-parse", "trunk:big/blob.bin"),
	}
}

func parseSource(t *testing.T, raw string) source.Source {
	t.Helper()
	src, err := source.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return src
}

func patterns(t *testing.T, raws ...string) []pathspec.Pattern {
	t.Helper()
	ps, err := pathspec.ParseAll(raws)
	if err != nil {
		t.Fatal(err)
	}
	return ps
}

func sparseDownload(t *testing.T, req *model.Request) (model.Snapshot, error) {
	t.Helper()
	return downloader.NewSparseCheckoutDownloader(gitutil.RealRunner{}).Download(context.Background(), req, t.TempDir(), model.Discard{})
}

func checkedOut(t *testing.T, dir string) []string {
	t.Helper()
	var files []string
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		if info.IsDir() && rel == ".git" {
			return filepath.SkipDir
		}
		if !info.IsDir() {
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	return files
}

func TestSparseCheckoutUsesTheDefaultBranchAndSkipsOtherBlobs(t *testing.T) {
	fx := newFixture(t)
	snap, err := sparseDownload(t, &model.Request{Source: parseSource(t, fx.url), Paths: patterns(t, "src/components")})
	if err != nil {
		t.Fatal(err)
	}
	if snap.Ref != "trunk" || snap.Commit != fx.head {
		t.Errorf("snapshot = %+v, want ref trunk at %s", snap, fx.head)
	}
	if got := strings.Join(checkedOut(t, snap.Dir), " "); got != ".gitattributes src/.gitattributes src/components/Button.tsx src/components/icons/star.svg" {
		t.Errorf("checked out %s", got)
	}

	cmd := exec.Command("git", "rev-list", "--objects", "--missing=print", "HEAD")
	cmd.Dir = snap.Dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "?"+fx.bigBlob) {
		t.Error("the partial clone downloaded a blob outside the requested paths")
	}

	all, err := snap.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != len(fixtureFiles) {
		t.Errorf("listed %d paths, want every file in the tree (%d)", len(all), len(fixtureFiles))
	}
}

func TestSparseCheckoutMatchesFilesAndGlobs(t *testing.T) {
	fx := newFixture(t)
	snap, err := sparseDownload(t, &model.Request{Source: parseSource(t, fx.url), Paths: patterns(t, "data/usage.txt", "data/*_linux.json")})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(checkedOut(t, snap.Dir), " "); got != ".gitattributes data/a_linux.json data/b_linux.json data/usage.txt" {
		t.Errorf("checked out %s", got)
	}
}

func lazyFetches(t *testing.T, trace string) int {
	t.Helper()
	data, err := os.ReadFile(trace)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for line := range strings.SplitSeq(string(data), "\n") {
		var event struct {
			Event string   `json:"event"`
			Argv  []string `json:"argv"`
		}
		if json.Unmarshal([]byte(line), &event) == nil && event.Event == "child_start" &&
			slices.Contains(event.Argv, "fetch") && slices.Contains(event.Argv, "--stdin") {
			n++
		}
	}
	return n
}

func TestSparseCheckoutFetchesFilesAndParentAttributesInOneBatch(t *testing.T) {
	fx := newFixture(t)
	trace := filepath.Join(t.TempDir(), "trace.json")
	t.Setenv("GIT_TRACE2_EVENT", trace)
	snap, err := sparseDownload(t, &model.Request{Source: parseSource(t, fx.url), Paths: patterns(t, "src/components/icons")})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(checkedOut(t, snap.Dir), " "); got != ".gitattributes src/.gitattributes src/components/icons/star.svg" {
		t.Errorf("checked out %s", got)
	}
	if n := lazyFetches(t, trace); n > 1 {
		t.Errorf("git fetched file contents %d times, want one batch", n)
	}
}

func TestSparseCheckoutFetchesEverythingWithoutPaths(t *testing.T) {
	fx := newFixture(t)
	for _, ref := range []string{"", fx.head} {
		snap, err := sparseDownload(t, &model.Request{Source: parseSource(t, fx.url), Ref: ref})
		if err != nil {
			t.Fatalf("%q: %v", ref, err)
		}
		if got := len(checkedOut(t, snap.Dir)); got != len(fixtureFiles) {
			t.Errorf("%q: checked out %d files, want %d", ref, got, len(fixtureFiles))
		}
		promisor := exec.Command("git", "config", "--get", "remote.origin.promisor")
		promisor.Dir = snap.Dir
		if out, _ := promisor.Output(); len(out) > 0 {
			t.Errorf("%q: a whole-repository download used a partial clone", ref)
		}
	}
}

func TestSparseCheckoutResolvesBranchesTagsAndCommits(t *testing.T) {
	fx := newFixture(t)
	cases := []struct {
		ref, wantCommit string
		hasFeature      bool
	}{
		{"feature/x", fx.feature, true},
		{"v1", fx.head, false},
		{fx.feature, fx.feature, true},
	}
	for _, tc := range cases {
		snap, err := sparseDownload(t, &model.Request{Source: parseSource(t, fx.url), Ref: tc.ref, Paths: patterns(t, "src/components")})
		if err != nil {
			t.Fatalf("%s: %v", tc.ref, err)
		}
		if snap.Ref != tc.ref || snap.Commit != tc.wantCommit {
			t.Errorf("%s: snapshot = %+v", tc.ref, snap)
		}
		_, err = os.Stat(filepath.Join(snap.Dir, "src", "components", "Feature.tsx"))
		if (err == nil) != tc.hasFeature {
			t.Errorf("%s: Feature.tsx present = %v", tc.ref, err == nil)
		}
	}
}

func TestSparseCheckoutPointsAtTheDefaultBranchWhenTheRefIsMissing(t *testing.T) {
	fx := newFixture(t)
	_, err := sparseDownload(t, &model.Request{Source: parseSource(t, fx.url), Ref: "main", Paths: patterns(t, "src")})
	var appErr *apperr.Error
	if !errors.Is(err, apperr.ErrRefNotFound) || !errors.As(err, &appErr) {
		t.Fatalf("got %v, want ErrRefNotFound", err)
	}
	if appErr.Hint != `the default branch is "trunk"` {
		t.Errorf("hint = %q", appErr.Hint)
	}
}

func TestSparseCheckoutReportsMissingRepositories(t *testing.T) {
	if !gitutil.IsGitInstalled() {
		t.Skip("git is not installed")
	}
	missing := "file://" + filepath.ToSlash(filepath.Join(t.TempDir(), "nope.git"))
	if _, err := sparseDownload(t, &model.Request{Source: parseSource(t, missing)}); !errors.Is(err, apperr.ErrRepositoryNotFound) {
		t.Errorf("got %v, want ErrRepositoryNotFound", err)
	}
}

func TestSparseCheckoutResolvesTreeLinks(t *testing.T) {
	fx := newFixture(t)
	cases := []struct {
		refPath, ref, path string
	}{
		{"feature/x/src/components", "feature/x", "src/components"},
		{"trunk/data/usage.txt", "trunk", "data/usage.txt"},
		{"v1", "v1", ""},
		{fx.feature + "/src", fx.feature, "src"},
	}
	for _, tc := range cases {
		src := parseSource(t, fx.url)
		src.RefPath = tc.refPath
		snap, err := sparseDownload(t, &model.Request{Source: src})
		if err != nil {
			t.Fatalf("%s: %v", tc.refPath, err)
		}
		gotPath := ""
		if len(snap.Paths) == 1 {
			gotPath = snap.Paths[0].String()
		}
		if snap.Ref != tc.ref || gotPath != tc.path || len(snap.Paths) > 1 {
			t.Errorf("%s: ref %q paths %v, want %q and %q", tc.refPath, snap.Ref, snap.Paths, tc.ref, tc.path)
		}
	}

	src := parseSource(t, fx.url)
	src.RefPath = "nope/src"
	_, err := sparseDownload(t, &model.Request{Source: src})
	var appErr *apperr.Error
	if !errors.As(err, &appErr) || !errors.Is(err, apperr.ErrRefNotFound) || appErr.Hint != `the default branch is "trunk"` {
		t.Errorf("missing ref in link: got %v (hint %q)", err, appErr.Hint)
	}
}

type noGit struct{}

func (noGit) Run(context.Context, string, []string, ...string) (string, error) {
	return "", errors.New("unexpected git call")
}

func (noGit) HasGit() bool {
	return false
}

func TestSparseCheckoutNeedsGit(t *testing.T) {
	_, err := downloader.NewSparseCheckoutDownloader(noGit{}).Download(
		context.Background(), &model.Request{Source: parseSource(t, "owner/repo")}, t.TempDir(), model.Discard{})
	if !errors.Is(err, apperr.ErrGitNotInstalled) {
		t.Errorf("got %v, want ErrGitNotInstalled", err)
	}
}

func TestFactoryPicksTheRequestedMethod(t *testing.T) {
	for _, m := range []model.Method{model.MethodSparse, model.MethodAPI} {
		dl, method, err := downloader.GetDownloader(&model.Request{Method: m, Source: parseSource(t, "o/r")})
		if err != nil || dl == nil || method != m {
			t.Errorf("%s: got %v, %s, %v", m, dl, method, err)
		}
	}
	if _, _, err := downloader.GetDownloader(&model.Request{Method: model.MethodAPI, Source: parseSource(t, "gitlab.com/g/p")}); !errors.Is(err, apperr.ErrUnsupported) {
		t.Errorf("api method for gitlab: got %v, want ErrUnsupported", err)
	}
	if _, _, err := downloader.GetDownloader(&model.Request{Method: "invalid"}); err == nil {
		t.Error("expected an error for an unknown method")
	}
}

func TestFactoryAutoPrefersGitAndFallsBackToTheAPI(t *testing.T) {
	if gitutil.IsGitInstalled() {
		_, method, err := downloader.GetDownloader(&model.Request{Method: model.MethodAuto, Source: parseSource(t, "gitlab.com/g/p")})
		if err != nil || method != model.MethodSparse {
			t.Errorf("with git: got %s, %v", method, err)
		}
	}

	t.Setenv("PATH", "")
	_, method, err := downloader.GetDownloader(&model.Request{Source: parseSource(t, "o/r")})
	if err != nil || method != model.MethodAPI {
		t.Errorf("without git on github: got %s, %v", method, err)
	}
	if _, _, err := downloader.GetDownloader(&model.Request{Source: parseSource(t, "gitlab.com/g/p")}); !errors.Is(err, apperr.ErrGitNotInstalled) {
		t.Errorf("without git elsewhere: got %v, want ErrGitNotInstalled", err)
	}
}

var _ downloader.GitRunner = (*gitutil.RealRunner)(nil)
