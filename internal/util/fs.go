package util

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func EnsureDir(path string) error {
	return os.MkdirAll(path, 0o755)
}

func SaveToFile(path string, content io.Reader, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := EnsureDir(dir); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	file, err := os.OpenFile(filepath.Clean(path), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", path, err)
	}

	_, err = io.Copy(file, content)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("failed to write to file %s: %w", path, err)
	}
	return nil
}

func CopySymlink(src, dst, root string) (skipped string, err error) {
	target, err := os.Readlink(src)
	if err != nil {
		return "", fmt.Errorf("failed to read symlink: %w", err)
	}
	if !linkStaysInside(root, src, target) {
		return "points outside the folder", nil
	}

	if err := removeIfNotDir(dst); err != nil {
		return "", err
	}
	if err := EnsureDir(filepath.Dir(dst)); err != nil {
		return "", fmt.Errorf("failed to create destination directory: %w", err)
	}
	if os.Symlink(target, dst) != nil {
		skipped = "symlinks can't be created here"
	}
	return skipped, nil
}

// linkStaysInside judges a symlink by its text alone and never follows it: a
// repository can ship config -> ~/.ssh/id_rsa, and following that would copy
// the user's own file into the output.
func linkStaysInside(root, link, target string) bool {
	if filepath.IsAbs(target) || filepath.VolumeName(target) != "" || strings.HasPrefix(target, "/") || strings.HasPrefix(target, `\`) {
		return false
	}
	rel, err := filepath.Rel(root, filepath.Join(filepath.Dir(link), target))
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func removeIfNotDir(path string) error {
	info, err := os.Lstat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil
	case err != nil:
		return fmt.Errorf("failed to inspect %s: %w", path, err)
	case info.IsDir():
		return nil
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("failed to replace %s: %w", path, err)
	}
	return nil
}

func CopyFile(src, dst string) error {
	srcFile, err := os.Open(filepath.Clean(src))
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer func() { _ = srcFile.Close() }()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat source file: %w", err)
	}

	if err = EnsureDir(filepath.Dir(dst)); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}
	if info, statErr := os.Lstat(dst); statErr == nil && info.Mode()&os.ModeSymlink != 0 {
		if err = os.Remove(dst); err != nil {
			return fmt.Errorf("failed to replace %s: %w", dst, err)
		}
	}

	dstFile, err := os.Create(filepath.Clean(dst))
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}

	_, err = io.Copy(dstFile, srcFile)
	if closeErr := dstFile.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	if err := os.Chmod(dst, srcInfo.Mode()); err != nil {
		return fmt.Errorf("failed to set permissions on destination file: %w", err)
	}

	return nil
}
