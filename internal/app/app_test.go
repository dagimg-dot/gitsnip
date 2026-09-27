package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dagimg-dot/gitsnip/internal/app/model"
	"github.com/dagimg-dot/gitsnip/internal/apperr"
)

type fakeDownloader struct {
	files map[string]string
	err   error
	dir   string
}

func (f *fakeDownloader) Download(ctx context.Context, req model.Request, dir string, rep model.Reporter) (model.Snapshot, error) {
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
	return model.Snapshot{Dir: dir, Ref: req.Branch}, nil
}

var repoFiles = map[string]string{
	"README.md":         "readme",
	"src/lib/a.txt":     "a",
	"src/lib/sub/b.txt": "b",
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
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("%s should not exist (stat error: %v)", path, err)
	}
}

func TestRunCopiesTheRequestedFolder(t *testing.T) {
	out := filepath.Join(t.TempDir(), "lib")
	dl := &fakeDownloader{files: repoFiles}

	res, err := run(context.Background(), dl, model.Request{Subdir: "src/lib", OutputDir: out, Branch: "main"}, model.Discard{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Output != out || res.Ref != "main" {
		t.Errorf("result = %+v", res)
	}
	assertFile(t, filepath.Join(out, "a.txt"), "a")
	assertFile(t, filepath.Join(out, "sub", "b.txt"), "b")
	assertMissing(t, filepath.Join(out, "README.md"))
	assertMissing(t, dl.dir)
}

func TestRunCopiesASingleFile(t *testing.T) {
	out := filepath.Join(t.TempDir(), "a.txt")
	if _, err := run(context.Background(), &fakeDownloader{files: repoFiles}, model.Request{Subdir: "src/lib/a.txt", OutputDir: out}, model.Discard{}); err != nil {
		t.Fatal(err)
	}
	assertFile(t, out, "a")
}

func TestRunReportsMissingPaths(t *testing.T) {
	out := filepath.Join(t.TempDir(), "nope")
	_, err := run(context.Background(), &fakeDownloader{files: repoFiles}, model.Request{Subdir: "nope", OutputDir: out}, model.Discard{})
	if !errors.Is(err, apperr.ErrPathNotFound) {
		t.Errorf("got %v, want ErrPathNotFound", err)
	}
	assertMissing(t, out)
}

func TestRunLeavesNothingBehindWhenTheFetchFails(t *testing.T) {
	out := filepath.Join(t.TempDir(), "lib")
	boom := errors.New("boom")
	dl := &fakeDownloader{err: boom}
	if _, err := run(context.Background(), dl, model.Request{Subdir: "src/lib", OutputDir: out}, model.Discard{}); !errors.Is(err, boom) {
		t.Errorf("got %v, want the download error", err)
	}
	assertMissing(t, out)
	assertMissing(t, dl.dir)
}
