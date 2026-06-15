// Package fsutil provides small filesystem helpers shared across the service.
package fsutil

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// CopyTree recursively copies the file tree rooted at src into dst, creating
// directories as needed and overwriting existing files. Directories are always
// traversed; a file is copied only when keep is nil or keep(baseName) is true.
func CopyTree(src, dst string, keep func(name string) bool) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return fmt.Errorf("relativize %q against %q: %w", path, src, err)
		}
		target := filepath.Join(dst, rel)

		if d.IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("create dir %q: %w", target, err)
			}
			return nil
		}

		if keep != nil && !keep(d.Name()) {
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("create dir %q: %w", filepath.Dir(target), err)
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open %q: %w", src, err)
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create %q: %w", dst, err)
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return fmt.Errorf("copy %q to %q: %w", src, dst, err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("close %q: %w", dst, err)
	}
	return nil
}
