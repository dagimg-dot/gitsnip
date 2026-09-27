package util

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type SkipFunc func(rel, reason string)

func EnsureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

func SaveToFile(path string, content io.Reader) error {
	dir := filepath.Dir(path)
	if err := EnsureDir(dir); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", path, err)
	}
	defer file.Close()

	_, err = io.Copy(file, content)
	if err != nil {
		return fmt.Errorf("failed to write to file %s: %w", path, err)
	}

	return nil
}

func CopyTree(src, dst string, skip SkipFunc) error {
	return copyTree(src, dst, src, skip)
}

func copyTree(src, dst, root string, skip SkipFunc) error {
	if err := EnsureDir(dst); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("failed to read source directory: %w", err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		var err error
		switch {
		case entry.Type()&os.ModeSymlink != 0:
			err = CopySymlink(srcPath, dstPath, root, skip)
		case entry.IsDir():
			err = copyTree(srcPath, dstPath, root, skip)
		case entry.Type().IsRegular():
			err = CopyFile(srcPath, dstPath)
		}
		if err != nil {
			return err
		}
	}

	return nil
}

func CopySymlink(src, dst, root string, skip SkipFunc) error {
	target, err := os.Readlink(src)
	if err != nil {
		return fmt.Errorf("failed to read symlink: %w", err)
	}

	rel, err := filepath.Rel(root, src)
	if err != nil {
		rel = filepath.Base(src)
	}
	rel = filepath.ToSlash(rel)

	if !linkStaysInside(root, src, target) {
		report(skip, rel, "points outside the folder")
		return nil
	}

	if err := removeIfNotDir(dst); err != nil {
		return err
	}
	if err := EnsureDir(filepath.Dir(dst)); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}
	if err := os.Symlink(target, dst); err != nil {
		report(skip, rel, "symlinks can't be created here")
	}
	return nil
}

func linkStaysInside(root, link, target string) bool {
	if filepath.IsAbs(target) || filepath.VolumeName(target) != "" || strings.HasPrefix(target, "/") || strings.HasPrefix(target, `\`) {
		return false
	}
	rel, err := filepath.Rel(root, filepath.Join(filepath.Dir(link), target))
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func report(skip SkipFunc, rel, reason string) {
	if skip != nil {
		skip(rel, reason)
	}
}

func removeIfNotDir(path string) error {
	info, err := os.Lstat(path)
	if err != nil || info.IsDir() {
		return nil
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("failed to replace %s: %w", path, err)
	}
	return nil
}

func CopyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat source file: %w", err)
	}

	if err := EnsureDir(filepath.Dir(dst)); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}
	if info, err := os.Lstat(dst); err == nil && info.Mode()&os.ModeSymlink != 0 {
		if err := os.Remove(dst); err != nil {
			return fmt.Errorf("failed to replace %s: %w", dst, err)
		}
	}

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close()

	if _, err = io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	if err := os.Chmod(dst, srcInfo.Mode()); err != nil {
		return fmt.Errorf("failed to set permissions on destination file: %w", err)
	}

	return nil
}
