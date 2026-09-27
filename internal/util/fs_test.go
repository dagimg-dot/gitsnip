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

func TestCopySymlinkKeepsOnlyLinksThatStayInside(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks needs extra privileges on Windows")
	}

	outside := filepath.Join(t.TempDir(), "id_rsa")
	os.WriteFile(outside, []byte("PRIVATE KEY"), 0o600)

	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "icons", "deep"), 0o755)
	cases := []struct {
		link, target, skipped string
	}{
		{"to-file", "a.txt", ""},
		{"to-dir", "icons", ""},
		{"icons/deep/up", "../../a.txt", ""},
		{"absolute", outside, "points outside the folder"},
		{"escape", "../" + filepath.Base(filepath.Dir(outside)) + "/id_rsa", "points outside the folder"},
		{"icons/deep/climb", "../../../x", "points outside the folder"},
	}

	dst := t.TempDir()
	for _, tc := range cases {
		src := filepath.Join(root, filepath.FromSlash(tc.link))
		if err := os.Symlink(tc.target, src); err != nil {
			t.Fatal(err)
		}
		out := filepath.Join(dst, filepath.FromSlash(tc.link))
		skipped, err := util.CopySymlink(src, out, root)
		if err != nil {
			t.Fatalf("%s: %v", tc.link, err)
		}
		if skipped != tc.skipped {
			t.Errorf("%s: skipped = %q, want %q", tc.link, skipped, tc.skipped)
		}
		got, err := os.Readlink(out)
		switch {
		case tc.skipped != "" && err == nil:
			t.Errorf("%s: created a link to %q, want none", tc.link, got)
		case tc.skipped == "" && got != tc.target:
			t.Errorf("%s -> %q, want %q", tc.link, got, tc.target)
		}
	}
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
