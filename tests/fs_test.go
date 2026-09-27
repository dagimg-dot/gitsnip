package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dagimg-dot/gitsnip/internal/util"
)

func TestEnsureDir(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "a", "b", "c")
	if err := util.EnsureDir(sub); err != nil {
		t.Fatalf("EnsureDir failed: %v", err)
	}
	if _, err := os.Stat(sub); os.IsNotExist(err) {
		t.Error("directory was not created")
	}
}

func TestEnsureDir_existing(t *testing.T) {
	if err := util.EnsureDir(t.TempDir()); err != nil {
		t.Fatalf("EnsureDir on existing dir failed: %v", err)
	}
}

func TestSaveToFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "test.txt")
	if err := util.SaveToFile(path, strings.NewReader("hello world")); err != nil {
		t.Fatalf("SaveToFile failed: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello world" {
		t.Errorf("got %q, want %q", string(data), "hello world")
	}
}

func TestCopyFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "sub", "dst.txt")
	os.WriteFile(src, []byte("file content"), 0644)
	if err := util.CopyFile(src, dst); err != nil {
		t.Fatalf("CopyFile failed: %v", err)
	}
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "file content" {
		t.Errorf("got %q, want %q", string(data), "file content")
	}
}

func TestCopyDirectory(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	files := map[string]string{
		"root.txt":           "root",
		"sub/file.txt":       "sub",
		"sub/nested/deep.txt": "deep",
	}
	for path, content := range files {
		full := filepath.Join(src, path)
		os.MkdirAll(filepath.Dir(full), 0755)
		os.WriteFile(full, []byte(content), 0644)
	}

	target := filepath.Join(dst, "copied")
	if err := util.CopyDirectory(src, target); err != nil {
		t.Fatalf("CopyDirectory failed: %v", err)
	}

	for path, content := range files {
		full := filepath.Join(target, path)
		data, err := os.ReadFile(full)
		if err != nil {
			t.Errorf("file %s: %v", path, err)
			continue
		}
		if string(data) != content {
			t.Errorf("file %s: got %q, want %q", path, string(data), content)
		}
	}
}
