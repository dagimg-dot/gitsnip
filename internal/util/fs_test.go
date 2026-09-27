package util_test

import (
	"os"
	"path/filepath"
	"runtime"
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
	if err := util.SaveToFile(path, strings.NewReader("hello world"), 0o755); err != nil {
		t.Fatalf("SaveToFile failed: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello world" {
		t.Errorf("got %q, want %q", string(data), "hello world")
	}
	if info, _ := os.Stat(path); runtime.GOOS != "windows" && info.Mode().Perm()&0o100 == 0 {
		t.Errorf("mode = %v, want the executable bit kept", info.Mode())
	}
}

func TestCopyFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "sub", "dst.txt")
	os.WriteFile(src, []byte("file content"), 0o644)
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

func TestCopyTree(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	files := map[string]string{
		"root.txt":            "root",
		"sub/file.txt":        "sub",
		"sub/nested/deep.txt": "deep",
	}
	for path, content := range files {
		full := filepath.Join(src, path)
		os.MkdirAll(filepath.Dir(full), 0o755)
		os.WriteFile(full, []byte(content), 0o644)
	}

	target := filepath.Join(dst, "copied")
	if err := util.CopyTree(src, target, nil); err != nil {
		t.Fatalf("CopyTree failed: %v", err)
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

func TestCopyTreeNeverFollowsSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks needs extra privileges on Windows")
	}

	outside := filepath.Join(t.TempDir(), "id_rsa")
	os.WriteFile(outside, []byte("PRIVATE KEY"), 0o600)

	src := t.TempDir()
	os.MkdirAll(filepath.Join(src, "icons", "deep"), 0o755)
	os.WriteFile(filepath.Join(src, "icons", "star.svg"), []byte("<svg/>"), 0o644)
	os.WriteFile(filepath.Join(src, "a.txt"), []byte("a"), 0o644)
	links := map[string]string{
		"to-file":          "a.txt",
		"to-dir":           "icons",
		"icons/deep/up":    "../../a.txt",
		"absolute":         outside,
		"escape":           "../" + filepath.Base(filepath.Dir(outside)) + "/id_rsa",
		"icons/deep/climb": "../../../x",
	}
	for link, target := range links {
		if err := os.Symlink(target, filepath.Join(src, link)); err != nil {
			t.Fatal(err)
		}
	}

	dst := filepath.Join(t.TempDir(), "out")
	skipped := map[string]string{}
	err := util.CopyTree(src, dst, func(rel, reason string) { skipped[rel] = reason })
	if err != nil {
		t.Fatalf("CopyTree failed: %v", err)
	}

	for _, link := range []string{"to-file", "to-dir", "icons/deep/up"} {
		got, err := os.Readlink(filepath.Join(dst, link))
		if err != nil {
			t.Errorf("%s: %v", link, err)
			continue
		}
		if got != links[link] {
			t.Errorf("%s -> %q, want %q", link, got, links[link])
		}
	}

	for _, link := range []string{"absolute", "escape", "icons/deep/climb"} {
		if _, err := os.Lstat(filepath.Join(dst, link)); !os.IsNotExist(err) {
			t.Errorf("%s should have been skipped", link)
		}
		if skipped[link] != "points outside the folder" {
			t.Errorf("%s skip reason = %q", link, skipped[link])
		}
	}

	filepath.Walk(dst, func(path string, info os.FileInfo, err error) error {
		if err == nil && info.Mode().IsRegular() {
			if data, _ := os.ReadFile(path); strings.Contains(string(data), "PRIVATE KEY") {
				t.Errorf("%s leaked a file from outside the repository", path)
			}
		}
		return nil
	})
}

func TestCopyFileReplacesSymlinksInsteadOfWritingThroughThem(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks needs extra privileges on Windows")
	}

	dir := t.TempDir()
	victim := filepath.Join(dir, "victim.txt")
	os.WriteFile(victim, []byte("untouched"), 0o644)
	src := filepath.Join(dir, "src.txt")
	os.WriteFile(src, []byte("new"), 0o644)
	dst := filepath.Join(dir, "dst.txt")
	os.Symlink(victim, dst)

	if err := util.CopyFile(src, dst); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(victim); string(data) != "untouched" {
		t.Errorf("wrote through the symlink: victim = %q", data)
	}
	if info, _ := os.Lstat(dst); info.Mode()&os.ModeSymlink != 0 {
		t.Error("destination is still a symlink")
	}
}
