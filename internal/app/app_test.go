package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/dagimg-dot/gitsnip/internal/app/model"
	"github.com/dagimg-dot/gitsnip/internal/apperr"
	"github.com/dagimg-dot/gitsnip/internal/pathspec"
	"github.com/dagimg-dot/gitsnip/internal/source"
)

type fakeDownloader struct {
	listing  []string
	paths    []pathspec.Pattern
	files    map[string]string
	links    map[string]string
	unusable []string
	err      error
	dir      string
}

func (f *fakeDownloader) Download(ctx context.Context, req *model.Request, dir string, rep model.Reporter) (model.Snapshot, error) {
	f.dir = dir
	if f.err != nil {
		return model.Snapshot{}, f.err
	}
	for name, content := range f.files {
		full := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return model.Snapshot{}, err
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			return model.Snapshot{}, err
		}
	}
	for name, target := range f.links {
		if err := os.Symlink(target, filepath.Join(dir, filepath.FromSlash(name))); err != nil {
			return model.Snapshot{}, err
		}
	}
	for _, name := range f.unusable {
		if err := os.Chmod(filepath.Join(dir, filepath.FromSlash(name)), 0); err != nil {
			return model.Snapshot{}, err
		}
	}
	snap := model.Snapshot{Dir: dir, Ref: "main", Commit: "abc123", Paths: f.paths}
	if f.listing != nil {
		snap.List = func(context.Context) ([]string, error) { return f.listing, nil }
	}
	return snap, nil
}

type recorder struct {
	model.Discard
	warnings []string
}

func (r *recorder) Warn(text string) {
	r.warnings = append(r.warnings, text)
}

var repoFiles = map[string]string{
	"README.md":         "readme",
	"src/lib/a.txt":     "a",
	"src/lib/sub/b.txt": "b",
	"data/usage.txt":    "usage",
	"data/x_linux.json": "x",
	"data/y_linux.json": "y",
	"data/z_mac.json":   "z",
}

func patterns(t *testing.T, raws ...string) []pathspec.Pattern {
	t.Helper()
	ps, err := pathspec.ParseAll(raws)
	if err != nil {
		t.Fatal(err)
	}
	return ps
}

func inTempDir(t *testing.T) {
	t.Helper()
	t.Chdir(t.TempDir())
}

func assertFile(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("%s = %q, want %q", path, got, want)
	}
}

func assertMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Errorf("%s should not exist (stat error: %v)", path, err)
	}
}

func TestRunWritesAFolderIntoItsOwnName(t *testing.T) {
	inTempDir(t)
	dl := &fakeDownloader{files: repoFiles}

	res, err := run(context.Background(), dl, &model.Request{Paths: patterns(t, "src/lib")}, model.Discard{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Output != "lib" || res.Target != "lib" || res.Files != 2 || res.Bytes != 2 || res.Ref != "main" || res.Commit != "abc123" {
		t.Errorf("result = %+v", res)
	}
	assertFile(t, "lib/a.txt", "a")
	assertFile(t, "lib/sub/b.txt", "b")
	assertMissing(t, "lib/README.md")
	assertMissing(t, dl.dir)
}

func TestRunWritesASingleFileIntoTheCurrentDirectory(t *testing.T) {
	inTempDir(t)
	res, err := run(context.Background(), &fakeDownloader{files: repoFiles}, &model.Request{Paths: patterns(t, "src/lib/a.txt")}, model.Discard{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Output != "." || res.Target != "a.txt" || res.Files != 1 {
		t.Errorf("result = %+v", res)
	}
	assertFile(t, "a.txt", "a")
}

func TestRunKeepsStructureBelowTheCommonParent(t *testing.T) {
	inTempDir(t)
	res, err := run(context.Background(), &fakeDownloader{files: repoFiles},
		&model.Request{Paths: patterns(t, "data/usage.txt", "data/*_linux.json")}, model.Discard{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Output != "data" || res.Files != 3 {
		t.Errorf("result = %+v", res)
	}
	assertFile(t, "data/usage.txt", "usage")
	assertFile(t, "data/x_linux.json", "x")
	assertFile(t, "data/y_linux.json", "y")
	assertMissing(t, "data/z_mac.json")
}

func TestRunNamesAWholeRepositoryAfterIt(t *testing.T) {
	inTempDir(t)
	res, err := run(context.Background(), &fakeDownloader{files: repoFiles},
		&model.Request{Source: source.Source{Repo: "snipped"}}, model.Discard{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Output != "snipped" || res.Files != len(repoFiles) {
		t.Errorf("result = %+v", res)
	}
	assertFile(t, "snipped/README.md", "readme")
	assertFile(t, "snipped/src/lib/sub/b.txt", "b")
}

func TestRunHonorsAnExplicitOutput(t *testing.T) {
	inTempDir(t)
	if _, err := run(context.Background(), &fakeDownloader{files: repoFiles},
		&model.Request{Paths: patterns(t, "src/lib/a.txt"), Output: "vendor"}, model.Discard{}); err != nil {
		t.Fatal(err)
	}
	assertFile(t, "vendor/a.txt", "a")
}

func TestRunReportsMissingPaths(t *testing.T) {
	inTempDir(t)
	cases := map[string]string{
		"nope":      `Path "nope" doesn't exist in the repository`,
		"docs/*.md": `No files match "docs/*.md" in the repository`,
	}
	for raw, want := range cases {
		_, err := run(context.Background(), &fakeDownloader{files: repoFiles},
			&model.Request{Paths: patterns(t, "src/lib", raw), Output: "out"}, model.Discard{})
		if !errors.Is(err, apperr.ErrPathNotFound) || err.Error() != want {
			t.Errorf("%s: got %v, want %q", raw, err, want)
		}
		assertMissing(t, "out")
	}
}

func TestRunLeavesNothingBehindWhenTheFetchFails(t *testing.T) {
	inTempDir(t)
	boom := errors.New("boom")
	dl := &fakeDownloader{err: boom}
	if _, err := run(context.Background(), dl, &model.Request{Paths: patterns(t, "src/lib")}, model.Discard{}); !errors.Is(err, boom) {
		t.Errorf("got %v, want the download error", err)
	}
	assertMissing(t, "lib")
	assertMissing(t, dl.dir)
}

func TestRunRemovesAPartialOutputWhenWritingFails(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs a file the current user can't read")
	}
	inTempDir(t)
	dl := &fakeDownloader{files: repoFiles, unusable: []string{"src/lib/sub/b.txt"}}
	if _, err := run(context.Background(), dl, &model.Request{Paths: patterns(t, "src/lib")}, model.Discard{}); err == nil {
		t.Fatal("expected the unreadable file to fail the copy")
	}
	assertMissing(t, "lib")
}

func TestRunStopsWhenCancelled(t *testing.T) {
	inTempDir(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := run(ctx, &fakeDownloader{files: repoFiles}, &model.Request{Paths: patterns(t, "src/lib")}, model.Discard{}); !errors.Is(err, context.Canceled) {
		t.Errorf("got %v, want context.Canceled", err)
	}
	assertMissing(t, "lib")
}

func TestRunNeverCopiesThroughSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks needs extra privileges on Windows")
	}

	outside := t.TempDir()
	os.WriteFile(filepath.Join(outside, "id_rsa"), []byte("PRIVATE KEY"), 0o600)
	dl := func() *fakeDownloader {
		return &fakeDownloader{files: repoFiles, links: map[string]string{
			"docs":        outside,
			"src/lib/key": filepath.Join(outside, "id_rsa"),
		}}
	}

	t.Run("requested path is a symlink", func(t *testing.T) {
		inTempDir(t)
		rec := &recorder{}
		if _, err := run(context.Background(), dl(), &model.Request{Paths: patterns(t, "docs"), Output: "out"}, rec); err != nil {
			t.Fatal(err)
		}
		assertMissing(t, "out")
		if strings.Join(rec.warnings, "\n") != "skipped symlink docs (points outside the folder)" {
			t.Errorf("warnings = %q", rec.warnings)
		}
	})

	t.Run("requested path goes through a symlink", func(t *testing.T) {
		inTempDir(t)
		_, err := run(context.Background(), dl(), &model.Request{Paths: patterns(t, "docs/id_rsa"), Output: "out"}, model.Discard{})
		if !errors.Is(err, apperr.ErrPathNotFound) {
			t.Errorf("got %v, want ErrPathNotFound", err)
		}
		assertMissing(t, "out")
	})

	t.Run("folder contains a symlink", func(t *testing.T) {
		inTempDir(t)
		rec := &recorder{}
		res, err := run(context.Background(), dl(), &model.Request{Paths: patterns(t, "src/lib")}, rec)
		if err != nil {
			t.Fatal(err)
		}
		assertFile(t, "lib/a.txt", "a")
		assertMissing(t, "lib/key")
		if res.Files != 2 {
			t.Errorf("files = %d, want the skipped link left out", res.Files)
		}
		if strings.Join(rec.warnings, "\n") != "skipped symlink src/lib/key (points outside the folder)" {
			t.Errorf("warnings = %q", rec.warnings)
		}
	})
}

func TestRunUsesThePathsTheDownloaderResolved(t *testing.T) {
	inTempDir(t)
	dl := &fakeDownloader{files: repoFiles, paths: patterns(t, "src/lib")}
	res, err := run(context.Background(), dl, &model.Request{}, model.Discard{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Output != "lib" || res.Files != 2 || res.Paths[0].String() != "src/lib" {
		t.Errorf("result = %+v", res)
	}
}

func TestRunRefusesToOverwriteExistingFiles(t *testing.T) {
	inTempDir(t)
	os.MkdirAll("lib", 0o755)
	os.WriteFile("lib/a.txt", []byte("mine"), 0o644)

	_, err := run(context.Background(), &fakeDownloader{files: repoFiles}, &model.Request{Paths: patterns(t, "src/lib")}, model.Discard{})
	want := "." + string(filepath.Separator) + "lib already has 1 of these files"
	if !errors.Is(err, apperr.ErrDestinationExists) || err.Error() != want {
		t.Fatalf("got %v, want %q", err, want)
	}
	assertFile(t, "lib/a.txt", "mine")
	assertMissing(t, "lib/sub")

	if _, err := run(context.Background(), &fakeDownloader{files: repoFiles}, &model.Request{Paths: patterns(t, "src/lib"), Force: true}, model.Discard{}); err != nil {
		t.Fatal(err)
	}
	assertFile(t, "lib/a.txt", "a")
	assertFile(t, "lib/sub/b.txt", "b")
}

func TestRunNamesTheSingleFileThatWouldBeOverwritten(t *testing.T) {
	inTempDir(t)
	os.WriteFile("a.txt", []byte("mine"), 0o644)
	_, err := run(context.Background(), &fakeDownloader{files: repoFiles}, &model.Request{Paths: patterns(t, "src/lib/a.txt")}, model.Discard{})
	if want := "." + string(filepath.Separator) + "a.txt already exists"; err == nil || err.Error() != want {
		t.Fatalf("got %v, want %q", err, want)
	}
	assertFile(t, "a.txt", "mine")
}

func TestRunMergesIntoAFolderWithoutClashes(t *testing.T) {
	inTempDir(t)
	os.MkdirAll("lib", 0o755)
	os.WriteFile("lib/notes.md", []byte("keep"), 0o644)
	if _, err := run(context.Background(), &fakeDownloader{files: repoFiles}, &model.Request{Paths: patterns(t, "src/lib")}, model.Discard{}); err != nil {
		t.Fatal(err)
	}
	assertFile(t, "lib/notes.md", "keep")
	assertFile(t, "lib/a.txt", "a")
}

func TestRunRejectsAFileWhereTheFolderShouldGo(t *testing.T) {
	inTempDir(t)
	os.WriteFile("lib", []byte("not a folder"), 0o644)
	_, err := run(context.Background(), &fakeDownloader{files: repoFiles}, &model.Request{Paths: patterns(t, "src/lib"), Force: true}, model.Discard{})
	if !errors.Is(err, apperr.ErrDestinationExists) {
		t.Fatalf("got %v, want ErrDestinationExists", err)
	}
	assertFile(t, "lib", "not a folder")
}

func TestRunSuggestsTheClosestPath(t *testing.T) {
	inTempDir(t)
	var listing []string
	for name := range repoFiles {
		listing = append(listing, name)
	}
	dl := &fakeDownloader{files: repoFiles, listing: listing}
	req := &model.Request{Source: source.Source{Host: "github.com", Owner: "o", Repo: "r"}, Paths: patterns(t, "src/lb")}

	_, err := run(context.Background(), dl, req, model.Discard{})
	var appErr *apperr.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("got %v, want an app error", err)
	}
	if appErr.Error() != `Path "src/lb" doesn't exist in o/r@main` || appErr.Hint != "did you mean src/lib?" {
		t.Errorf("message %q, hint %q", appErr.Error(), appErr.Hint)
	}
}
