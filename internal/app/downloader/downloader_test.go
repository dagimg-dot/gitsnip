package downloader_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dagimg-dot/gitsnip/internal/app/downloader"
	"github.com/dagimg-dot/gitsnip/internal/app/gitutil"
	"github.com/dagimg-dot/gitsnip/internal/app/model"
	"github.com/dagimg-dot/gitsnip/internal/apperr"
)

type cmdCall struct {
	dir  string
	args []string
}

type fakeGitRunner struct {
	hasGit   bool
	commands []cmdCall
	setupDir string
	runFunc  func(ctx context.Context, dir string, args ...string) (string, error)
}

func (f *fakeGitRunner) Run(ctx context.Context, dir string, args ...string) (string, error) {
	if f.runFunc != nil {
		return f.runFunc(ctx, dir, args...)
	}
	f.commands = append(f.commands, cmdCall{dir: dir, args: args})
	if args[0] == "checkout" && f.setupDir != "" {
		sparsePath := filepath.Join(dir, f.setupDir)
		os.MkdirAll(sparsePath, 0o755)
		os.WriteFile(filepath.Join(sparsePath, "test.txt"), []byte("content"), 0o644)
	}
	return "", nil
}

func (f *fakeGitRunner) HasGit() bool {
	return f.hasGit
}

func TestSparseCheckoutFetchesIntoTheGivenDir(t *testing.T) {
	fake := &fakeGitRunner{hasGit: true, setupDir: "src/lib"}
	dir := t.TempDir()
	req := model.Request{RepoURL: "https://github.com/owner/repo", Subdir: "src/lib", Branch: "main"}

	snap, err := downloader.NewSparseCheckoutDownloader(fake).Download(context.Background(), req, dir, model.Discard{})
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}
	if snap.Dir != dir || snap.Ref != "main" {
		t.Errorf("snapshot = %+v", snap)
	}

	expected := []string{"init", "remote", "sparse-checkout", "sparse-checkout", "fetch", "checkout"}
	var got []string
	for _, c := range fake.commands {
		got = append(got, c.args[0])
		if c.dir != dir {
			t.Errorf("git %s ran in %s, want %s", c.args[0], c.dir, dir)
		}
	}
	if strings.Join(got, " ") != strings.Join(expected, " ") {
		t.Errorf("commands = %v, want %v", got, expected)
	}

	if _, err := os.Stat(filepath.Join(dir, "src", "lib", "test.txt")); err != nil {
		t.Errorf("checked out file missing: %v", err)
	}
}

func TestSparseCheckoutNeedsGit(t *testing.T) {
	_, err := downloader.NewSparseCheckoutDownloader(&fakeGitRunner{}).Download(
		context.Background(), model.Request{RepoURL: "https://github.com/owner/repo", Subdir: "src"}, t.TempDir(), model.Discard{})
	if !errors.Is(err, apperr.ErrGitNotInstalled) {
		t.Errorf("got %v, want ErrGitNotInstalled", err)
	}
}

func TestSparseCheckoutStopsOnGitFailure(t *testing.T) {
	fake := &fakeGitRunner{hasGit: true}
	fake.runFunc = func(ctx context.Context, dir string, args ...string) (string, error) {
		if args[0] == "fetch" {
			return "", context.DeadlineExceeded
		}
		return "", nil
	}
	_, err := downloader.NewSparseCheckoutDownloader(fake).Download(
		context.Background(), model.Request{RepoURL: "https://github.com/owner/repo", Subdir: "src", Branch: "main"}, t.TempDir(), model.Discard{})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("got %v, want the fetch error", err)
	}
}

func TestSparseCheckoutFetchesTheRequestedBranch(t *testing.T) {
	fake := &fakeGitRunner{hasGit: true, setupDir: "src"}
	_, err := downloader.NewSparseCheckoutDownloader(fake).Download(
		context.Background(), model.Request{RepoURL: "https://github.com/owner/repo", Subdir: "src", Branch: "develop"}, t.TempDir(), model.Discard{})
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}
	for _, c := range fake.commands {
		if c.args[0] == "fetch" && c.args[len(c.args)-1] == "develop" {
			return
		}
	}
	t.Error("expected fetch to include branch name 'develop'")
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

func TestGitHubAPIDownloadsNestedFolders(t *testing.T) {
	dir := t.TempDir()
	req := model.Request{RepoURL: "https://github.com/o/r", Subdir: "src", Branch: "main"}
	if _, err := downloader.NewGitHubAPIDownloader(fakeGitHub(t)).Download(context.Background(), req, dir, model.Discard{}); err != nil {
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
	dir := t.TempDir()
	req := model.Request{RepoURL: "https://github.com/o/r", Subdir: "src/a.txt", Branch: "main"}
	if _, err := downloader.NewGitHubAPIDownloader(fakeGitHub(t)).Download(context.Background(), req, dir, model.Discard{}); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(dir, "src", "a.txt")); got != "content of src/a.txt" {
		t.Errorf("a.txt = %q", got)
	}
}

func TestGitHubAPIRejectsMalformedResponses(t *testing.T) {
	req := model.Request{RepoURL: "https://github.com/o/r", Subdir: "broken", Branch: "main"}
	_, err := downloader.NewGitHubAPIDownloader(fakeGitHub(t)).Download(context.Background(), req, t.TempDir(), model.Discard{})
	if err == nil || !strings.Contains(err.Error(), "failed to parse API response") {
		t.Errorf("got %v, want a parse error", err)
	}
}

func TestGitHubAPIReportsMissingPaths(t *testing.T) {
	req := model.Request{RepoURL: "https://github.com/o/r", Subdir: "nope", Branch: "main"}
	_, err := downloader.NewGitHubAPIDownloader(fakeGitHub(t)).Download(context.Background(), req, t.TempDir(), model.Discard{})
	var appErr *apperr.Error
	if !errors.As(err, &appErr) || appErr.StatusCode != http.StatusNotFound {
		t.Errorf("got %v, want a 404 app error", err)
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
