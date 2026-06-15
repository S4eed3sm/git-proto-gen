package generate

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/moby/moby/client"
	"github.com/testcontainers/testcontainers-go"
)

// copyOut copies the generated tree from the container's output directory to the
// host outDir. It streams a tar archive from the Docker daemon's archive
// endpoint, so it does not rely on a tar binary inside the image. Files are
// written by the current process, so they are owned by the invoking user rather
// than the container's root.
func copyOut(ctx context.Context, c testcontainers.Container, outDir string) error {
	cli, err := testcontainers.NewDockerClientWithOpts(ctx)
	if err != nil {
		return fmt.Errorf("docker client: %w", err)
	}
	defer cli.Close()

	res, err := cli.CopyFromContainer(ctx, c.GetContainerID(), client.CopyFromContainerOptions{
		SourcePath: containerOutputDir,
	})
	if err != nil {
		return fmt.Errorf("copy %s from container: %w", containerOutputDir, err)
	}
	defer res.Content.Close()

	if err := extractTar(res.Content, outDir); err != nil {
		return fmt.Errorf("extract generated code to %q: %w", outDir, err)
	}
	return nil
}

// extractTar writes the regular files and directories of a tar stream into dst.
// The daemon's archive endpoint prefixes every entry with the source directory's
// base name (e.g. "generated/..."), so the leading component is stripped to land
// files directly under dst. Entries that would escape dst are rejected.
func extractTar(r io.Reader, dst string) error {
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read tar stream: %w", err)
		}

		rel := stripLeading(hdr.Name)
		if rel == "" {
			continue // the top-level output directory entry itself
		}
		target := filepath.Join(dst, rel)
		if !withinDir(dst, target) {
			return fmt.Errorf("tar entry %q escapes output directory", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err := writeFile(target, tr, fileMode(hdr.Mode)); err != nil {
				return err
			}
		}
		// Symlinks, devices and other entry types are skipped: generated code is
		// plain files and directories.
	}
}

// stripLeading removes the first path component of a slash-separated tar entry
// name, returning the remainder (empty when only one component is present).
func stripLeading(name string) string {
	name = strings.TrimPrefix(name, "./")
	_, rest, found := strings.Cut(name, "/")
	if !found {
		return ""
	}
	return strings.Trim(rest, "/")
}

// withinDir reports whether target lies inside dir, guarding against tar entry
// names that would escape via "..".
func withinDir(dir, target string) bool {
	rel, err := filepath.Rel(dir, target)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// fileMode returns the permission bits to use for an extracted file, falling
// back to 0644 when the tar header carries no usable mode.
func fileMode(mode int64) os.FileMode {
	if m := os.FileMode(mode).Perm(); m != 0 {
		return m
	}
	return 0o644
}

func writeFile(path string, r io.Reader, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(f, r)
	closeErr := f.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
