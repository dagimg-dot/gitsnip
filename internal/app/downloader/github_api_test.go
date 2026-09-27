package downloader_test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dagimg-dot/gitsnip/internal/app/downloader"
	"github.com/dagimg-dot/gitsnip/internal/app/model"
	"github.com/dagimg-dot/gitsnip/internal/apperr"
)

const (
	shaMain = "1111111111111111111111111111111111111111"
	shaV1   = "2222222222222222222222222222222222222222"
)

type hubFile struct {
	content string
	mode    string
}

type fakeHub struct {
	token     string
	limited   bool
	truncated bool
	brokenAPI bool
	files     map[string]hubFile
}

func newHub() *fakeHub {
	return &fakeHub{files: map[string]hubFile{
		"README.md":         {"readme", "100644"},
		"src/a.txt":         {"", "100644"},
		"src/sub/b.txt":     {"", "100644"},
		"src/run.sh":        {"", "100755"},
		"src/link":          {"a.txt", "120000"},
		"vendor/lib":        {"", "160000"},
		"data/x_linux.json": {"", "100644"},
		"data/y_mac.json":   {"", "100644"},
	}}
}

func (h *fakeHub) serve(t *testing.T) downloader.HTTPDoer {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/o/r", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"default_branch":"main"}`)
	})
	mux.HandleFunc("GET /repos/o/r/commits/{ref...}", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/vnd.github.sha" {
			http.Error(w, "wrong accept header", http.StatusBadRequest)
			return
		}
		switch r.PathValue("ref") {
		case "main":
			fmt.Fprint(w, shaMain)
		case "v1":
			fmt.Fprint(w, shaV1)
		default:
			http.Error(w, `{"message":"No commit found for SHA"}`, http.StatusUnprocessableEntity)
		}
	})
	mux.HandleFunc("GET /repos/o/r/git/trees/{sha}", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("recursive") != "1" {
			http.Error(w, "not recursive", http.StatusBadRequest)
			return
		}
		if h.brokenAPI {
			fmt.Fprint(w, "<html>upstream error</html>")
			return
		}
		type entry struct {
			Path string `json:"path"`
			Mode string `json:"mode"`
			Type string `json:"type"`
			SHA  string `json:"sha"`
		}
		var entries []entry
		dirs := map[string]bool{}
		for name, f := range h.files {
			typ := "blob"
			if f.mode == "160000" {
				typ = "commit"
			}
			entries = append(entries, entry{name, f.mode, typ, hex.EncodeToString([]byte(name))})
			for dir := path.Dir(name); dir != "." && !dirs[dir]; dir = path.Dir(dir) {
				dirs[dir] = true
				entries = append(entries, entry{dir, "040000", "tree", hex.EncodeToString([]byte(dir + "/"))})
			}
		}
		json.NewEncoder(w).Encode(map[string]any{"sha": r.PathValue("sha"), "tree": entries, "truncated": h.truncated})
	})
	mux.HandleFunc("GET /repos/o/r/git/blobs/{sha}", func(w http.ResponseWriter, r *http.Request) {
		name, _ := hex.DecodeString(r.PathValue("sha"))
		fmt.Fprint(w, h.files[string(name)].content)
	})
	mux.HandleFunc("GET /o/r/{sha}/{file...}", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s at %s", r.PathValue("file"), r.PathValue("sha"))
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case h.token != "" && r.Header.Get("Authorization") == "":
			http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
		case h.token != "" && r.Header.Get("Authorization") != "token "+h.token:
			http.Error(w, `{"message":"Bad credentials"}`, http.StatusUnauthorized)
		case h.limited && strings.HasPrefix(r.URL.Path, "/repos/"):
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Date(2030, 1, 1, 15, 4, 0, 0, time.Local).Unix(), 10))
			http.Error(w, `{"message":"API rate limit exceeded"}`, http.StatusForbidden)
		default:
			mux.ServeHTTP(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	target, _ := url.Parse(srv.URL)
	return redirectDoer{target: target}
}

type redirectDoer struct {
	target *url.URL
}

func (d redirectDoer) Do(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.URL.Scheme, req.URL.Host, req.Host = d.target.Scheme, d.target.Host, ""
	return http.DefaultClient.Do(req)
}

type events struct {
	mu       sync.Mutex
	warnings []string
	done     int
	total    int
}

func (e *events) Stage(string) {}
func (e *events) Debug(string) {}

func (e *events) Progress(done, total int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.done, e.total = max(e.done, done), total
}

func (e *events) Warn(text string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.warnings = append(e.warnings, text)
}

func apiDownload(t *testing.T, hub *fakeHub, req model.Request) (string, model.Snapshot, *events, error) {
	t.Helper()
	dir := t.TempDir()
	if req.Source.Repo == "" {
		req.Source = parseSource(t, "https://github.com/o/r")
	}
	ev := &events{}
	snap, err := downloader.NewGitHubAPIDownloader(hub.serve(t)).Download(context.Background(), req, dir, ev)
	return dir, snap, ev, err
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestGitHubAPIDownloadsFromTheDefaultBranch(t *testing.T) {
	dir, snap, _, err := apiDownload(t, newHub(), model.Request{Paths: patterns(t, "src")})
	if err != nil {
		t.Fatal(err)
	}
	if snap.Ref != "main" || snap.Commit != shaMain {
		t.Errorf("snapshot = %+v", snap)
	}
	if got := readFile(t, filepath.Join(dir, "src", "sub", "b.txt")); got != "src/sub/b.txt at "+shaMain {
		t.Errorf("b.txt = %q", got)
	}
	if runtime.GOOS != "windows" {
		if info, _ := os.Stat(filepath.Join(dir, "src", "run.sh")); info.Mode().Perm()&0o100 == 0 {
			t.Errorf("run.sh mode = %v, want executable", info.Mode())
		}
		if target, err := os.Readlink(filepath.Join(dir, "src", "link")); err != nil || target != "a.txt" {
			t.Errorf("link -> %q (%v), want a.txt", target, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "README.md")); !os.IsNotExist(err) {
		t.Error("downloaded a file outside the requested path")
	}
	if all, _ := snap.List(context.Background()); len(all) != 8 {
		t.Errorf("listed %v, want every blob and submodule", all)
	}
}

func TestGitHubAPIMatchesFilesAndGlobs(t *testing.T) {
	dir, _, _, err := apiDownload(t, newHub(), model.Request{Paths: patterns(t, "README.md", "data/*_linux.json")})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(checkedOut(t, dir), " "); got != "README.md data/x_linux.json" {
		t.Errorf("downloaded %s", got)
	}
}

func TestGitHubAPIWarnsAboutSubmodules(t *testing.T) {
	_, _, ev, err := apiDownload(t, newHub(), model.Request{Paths: patterns(t, "vendor")})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(ev.warnings, "\n") != "skipped submodule vendor/lib (submodules aren't downloaded)" {
		t.Errorf("warnings = %q", ev.warnings)
	}
}

func TestGitHubAPIUsesTheRequestedRef(t *testing.T) {
	dir, snap, _, err := apiDownload(t, newHub(), model.Request{Ref: "v1", Paths: patterns(t, "src/a.txt")})
	if err != nil {
		t.Fatal(err)
	}
	if snap.Ref != "v1" || snap.Commit != shaV1 {
		t.Errorf("snapshot = %+v", snap)
	}
	if got := readFile(t, filepath.Join(dir, "src", "a.txt")); got != "src/a.txt at "+shaV1 {
		t.Errorf("a.txt = %q", got)
	}
}

func TestGitHubAPIPointsAtTheDefaultBranchWhenTheRefIsMissing(t *testing.T) {
	_, _, _, err := apiDownload(t, newHub(), model.Request{Ref: "nope", Paths: patterns(t, "src")})
	var appErr *apperr.Error
	if !errors.Is(err, apperr.ErrRefNotFound) || !errors.As(err, &appErr) {
		t.Fatalf("got %v, want ErrRefNotFound", err)
	}
	if appErr.Error() != `Branch or tag "nope" doesn't exist in o/r` || appErr.Hint != `the default branch is "main"` {
		t.Errorf("message = %q, hint = %q", appErr.Error(), appErr.Hint)
	}
}

func TestGitHubAPIReportsMissingRepositories(t *testing.T) {
	_, _, _, err := apiDownload(t, newHub(), model.Request{Source: parseSource(t, "x/y")})
	if !errors.Is(err, apperr.ErrRepositoryNotFound) || err.Error() != "Repository x/y doesn't exist or is private" {
		t.Errorf("got %v, want ErrRepositoryNotFound", err)
	}
}

func TestGitHubAPIExplainsRateLimits(t *testing.T) {
	hub := newHub()
	hub.limited = true
	_, _, _, err := apiDownload(t, hub, model.Request{Paths: patterns(t, "src")})
	var appErr *apperr.Error
	if !errors.Is(err, apperr.ErrRateLimitExceeded) || !errors.As(err, &appErr) {
		t.Fatalf("got %v, want ErrRateLimitExceeded", err)
	}
	if !strings.HasPrefix(appErr.Hint, "it resets at 15:04; set GITHUB_TOKEN") {
		t.Errorf("hint = %q", appErr.Hint)
	}
}

func TestGitHubAPIRefusesTruncatedTrees(t *testing.T) {
	hub := newHub()
	hub.truncated = true
	if _, _, _, err := apiDownload(t, hub, model.Request{}); !errors.Is(err, apperr.ErrUnsupported) {
		t.Errorf("got %v, want ErrUnsupported", err)
	}
}

func TestGitHubAPIRejectsMalformedResponses(t *testing.T) {
	hub := newHub()
	hub.brokenAPI = true
	if _, _, _, err := apiDownload(t, hub, model.Request{}); err == nil || !strings.Contains(err.Error(), "failed to parse") {
		t.Errorf("got %v, want a parse error", err)
	}
}

func TestGitHubAPISendsTheToken(t *testing.T) {
	hub := newHub()
	hub.token = "secret"
	if _, _, _, err := apiDownload(t, hub, model.Request{Token: "secret", Paths: patterns(t, "README.md")}); err != nil {
		t.Errorf("with the right token: %v", err)
	}
	if _, _, _, err := apiDownload(t, hub, model.Request{Token: "wrong"}); !errors.Is(err, apperr.ErrAuthenticationRequired) {
		t.Errorf("with a wrong token: got %v, want ErrAuthenticationRequired", err)
	}
}

func TestGitHubAPIDownloadsManyFilesConcurrently(t *testing.T) {
	hub := newHub()
	for i := range 40 {
		hub.files[fmt.Sprintf("many/%02d.txt", i)] = hubFile{"", "100644"}
	}
	dir, _, ev, err := apiDownload(t, hub, model.Request{Paths: patterns(t, "many")})
	if err != nil {
		t.Fatal(err)
	}
	if got := len(checkedOut(t, dir)); got != 40 {
		t.Errorf("downloaded %d files, want 40", got)
	}
	if ev.done != 40 || ev.total != 40 {
		t.Errorf("progress = %d/%d, want 40/40", ev.done, ev.total)
	}
}

func TestGitHubAPIResolvesTreeLinks(t *testing.T) {
	src := parseSource(t, "https://github.com/o/r/tree/v1/src/sub")
	dir, snap, _, err := apiDownload(t, newHub(), model.Request{Source: src})
	if err != nil {
		t.Fatal(err)
	}
	if snap.Ref != "v1" || snap.Commit != shaV1 || len(snap.Paths) != 1 || snap.Paths[0].String() != "src/sub" {
		t.Errorf("snapshot = %+v", snap)
	}
	if got := strings.Join(checkedOut(t, dir), " "); got != "src/sub/b.txt" {
		t.Errorf("downloaded %s", got)
	}

	src = parseSource(t, "https://github.com/o/r/tree/nope/src")
	if _, _, _, err := apiDownload(t, newHub(), model.Request{Source: src}); !errors.Is(err, apperr.ErrRefNotFound) {
		t.Errorf("missing ref in link: got %v", err)
	}
}
