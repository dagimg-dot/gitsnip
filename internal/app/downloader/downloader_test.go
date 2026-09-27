package downloader_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dagimg-dot/gitsnip/internal/app/downloader"
	"github.com/dagimg-dot/gitsnip/internal/app/gitutil"
	"github.com/dagimg-dot/gitsnip/internal/app/model"
	"github.com/dagimg-dot/gitsnip/internal/apperr"
	"github.com/dagimg-dot/gitsnip/internal/pathspec"
)

type fixture struct {
	url     string
	head    string
	feature string
	bigBlob string
}

var fixtureFiles = map[string]string{
	"README.md":                     "# fixture\n",
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

func patterns(t *testing.T, raws ...string) []pathspec.Pattern {
	t.Helper()
	ps, err := pathspec.ParseAll(raws)
	if err != nil {
		t.Fatal(err)
	}
	return ps
}

func sparseDownload(t *testing.T, req model.Request) (model.Snapshot, error) {
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
	snap, err := sparseDownload(t, model.Request{RepoURL: fx.url, Paths: patterns(t, "src/components")})
	if err != nil {
		t.Fatal(err)
	}
	if snap.Ref != "trunk" || snap.Commit != fx.head {
		t.Errorf("snapshot = %+v, want ref trunk at %s", snap, fx.head)
	}
	if got := strings.Join(checkedOut(t, snap.Dir), " "); got != "src/components/Button.tsx src/components/icons/star.svg" {
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
}

func TestSparseCheckoutMatchesFilesAndGlobs(t *testing.T) {
	fx := newFixture(t)
	snap, err := sparseDownload(t, model.Request{RepoURL: fx.url, Paths: patterns(t, "data/usage.txt", "data/*_linux.json")})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(checkedOut(t, snap.Dir), " "); got != "data/a_linux.json data/b_linux.json data/usage.txt" {
		t.Errorf("checked out %s", got)
	}
}

func TestSparseCheckoutFetchesEverythingWithoutPaths(t *testing.T) {
	fx := newFixture(t)
	snap, err := sparseDownload(t, model.Request{RepoURL: fx.url})
	if err != nil {
		t.Fatal(err)
	}
	if got := len(checkedOut(t, snap.Dir)); got != len(fixtureFiles) {
		t.Errorf("checked out %d files, want %d", got, len(fixtureFiles))
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
		snap, err := sparseDownload(t, model.Request{RepoURL: fx.url, Ref: tc.ref, Paths: patterns(t, "src/components")})
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
	_, err := sparseDownload(t, model.Request{RepoURL: fx.url, Ref: "main", Paths: patterns(t, "src")})
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
	if _, err := sparseDownload(t, model.Request{RepoURL: missing}); !errors.Is(err, apperr.ErrRepositoryNotFound) {
		t.Errorf("got %v, want ErrRepositoryNotFound", err)
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
		context.Background(), model.Request{RepoURL: "https://github.com/owner/repo"}, t.TempDir(), model.Discard{})
	if !errors.Is(err, apperr.ErrGitNotInstalled) {
		t.Errorf("got %v, want ErrGitNotInstalled", err)
	}
}

type redirectDoer struct {
	target *url.URL
}

func (d redirectDoer) Do(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.URL.Scheme, req.URL.Host, req.Host = d.target.Scheme, d.target.Host, ""
	return http.DefaultClient.Do(req)
}

func fakeGitHub(t *testing.T) downloader.HTTPDoer {
	t.Helper()
	item := func(name, path, typ string) string {
		dl := "null"
		if typ == "file" {
			dl = fmt.Sprintf("%q", "https://raw.githubusercontent.com/o/r/main/"+path)
		}
		return fmt.Sprintf(`{"name":%q,"path":%q,"type":%q,"download_url":%s}`, name, path, typ, dl)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/o/r/contents/", func(w http.ResponseWriter, r *http.Request) {
		switch strings.TrimPrefix(r.URL.Path, "/repos/o/r/contents/") {
		case "src":
			fmt.Fprintf(w, "[%s,%s]", item("a.txt", "src/a.txt", "file"), item("sub", "src/sub", "dir"))
		case "src/sub":
			fmt.Fprintf(w, "[%s]", item("b.txt", "src/sub/b.txt", "file"))
		case "src/a.txt":
			fmt.Fprint(w, item("a.txt", "src/a.txt", "file"))
		case "broken":
			fmt.Fprint(w, "<html>upstream error</html>")
		default:
			http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
		}
	})
	mux.HandleFunc("/o/r/main/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "content of "+strings.TrimPrefix(r.URL.Path, "/o/r/main/"))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	target, _ := url.Parse(srv.URL)
	return redirectDoer{target: target}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func apiDownload(t *testing.T, raws ...string) (string, error) {
	t.Helper()
	dir := t.TempDir()
	req := model.Request{RepoURL: "https://github.com/o/r", Ref: "main", Paths: patterns(t, raws...)}
	_, err := downloader.NewGitHubAPIDownloader(fakeGitHub(t)).Download(context.Background(), req, dir, model.Discard{})
	return dir, err
}

func TestGitHubAPIDownloadsNestedFolders(t *testing.T) {
	dir, err := apiDownload(t, "src")
	if err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(dir, "src", "a.txt")); got != "content of src/a.txt" {
		t.Errorf("a.txt = %q", got)
	}
	if got := readFile(t, filepath.Join(dir, "src", "sub", "b.txt")); got != "content of src/sub/b.txt" {
		t.Errorf("sub/b.txt = %q", got)
	}
}

func TestGitHubAPIDownloadsASingleFile(t *testing.T) {
	dir, err := apiDownload(t, "src/a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(dir, "src", "a.txt")); got != "content of src/a.txt" {
		t.Errorf("a.txt = %q", got)
	}
}

func TestGitHubAPIRejectsMalformedResponses(t *testing.T) {
	if _, err := apiDownload(t, "broken"); err == nil || !strings.Contains(err.Error(), "failed to parse API response") {
		t.Errorf("got %v, want a parse error", err)
	}
}

func TestGitHubAPIReportsMissingPaths(t *testing.T) {
	_, err := apiDownload(t, "nope")
	var appErr *apperr.Error
	if !errors.As(err, &appErr) || appErr.StatusCode != http.StatusNotFound {
		t.Errorf("got %v, want a 404 app error", err)
	}
}

func TestGitHubAPIRefusesGlobsForNow(t *testing.T) {
	if _, err := apiDownload(t, "src/*.txt"); !errors.Is(err, apperr.ErrUnsupported) {
		t.Errorf("got %v, want ErrUnsupported", err)
	}
}

func TestFactory_sparse(t *testing.T) {
	dl, err := downloader.GetDownloader(model.Request{Method: model.MethodSparse})
	if err != nil || dl == nil {
		t.Fatalf("got %v, %v", dl, err)
	}
}

func TestFactory_api(t *testing.T) {
	dl, err := downloader.GetDownloader(model.Request{Method: model.MethodAPI, Provider: model.ProviderTypeGitHub})
	if err != nil || dl == nil {
		t.Fatalf("got %v, %v", dl, err)
	}
}

func TestFactory_invalid(t *testing.T) {
	if _, err := downloader.GetDownloader(model.Request{Method: "invalid"}); err == nil {
		t.Fatal("expected error for invalid method")
	}
}

var _ downloader.GitRunner = (*gitutil.RealRunner)(nil)
