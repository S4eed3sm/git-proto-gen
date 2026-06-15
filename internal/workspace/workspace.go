// Package workspace owns the temporary directories used during a generation
// run and renders the buf configuration into them. A Workspace bundles the
// source tree that buf reads, the directory generated code is written to, and
// the cleanup of both.
package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Workspace is a set of temporary directories for one generation run.
type Workspace struct {
	// Root is the buf source workspace (mounted as /workspace in Docker). It
	// holds the rendered buf config files and a proto/ subdirectory.
	Root string
	// ProtoDir is Root/proto, where .proto sources from every source are staged.
	ProtoDir string
	// OutputDir is where buf writes generated code before it is copied to the
	// caller's final output directory.
	OutputDir string
}

// New creates the source workspace and output directory and writes the rendered
// buf configuration files into the workspace root. On any error it removes
// whatever it already created. The caller must call Close when done.
func New(render *Render) (*Workspace, error) {
	root, err := os.MkdirTemp("", "gpg-workspace-")
	if err != nil {
		return nil, fmt.Errorf("create workspace dir: %w", err)
	}
	w := &Workspace{Root: root, ProtoDir: filepath.Join(root, "proto")}

	if err := os.MkdirAll(w.ProtoDir, 0o755); err != nil {
		w.Close()
		return nil, fmt.Errorf("create proto dir: %w", err)
	}

	outDir, err := os.MkdirTemp("", "gpg-output-")
	if err != nil {
		w.Close()
		return nil, fmt.Errorf("create output dir: %w", err)
	}
	w.OutputDir = outDir

	if err := w.writeConfigs(render); err != nil {
		w.Close()
		return nil, err
	}
	return w, nil
}

func (w *Workspace) writeConfigs(render *Render) error {
	for _, name := range []string{bufYamlFile, genGoFile, genJSFile} {
		content, err := render.file(name)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(w.Root, name), content, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}
	return nil
}

// TemplateFile returns the buf gen-template file name for a language. Anything
// other than "js" uses the Go template.
func (w *Workspace) TemplateFile(lang string) string {
	if lang == "js" {
		return genJSFile
	}
	return genGoFile
}

// Close removes both temporary directories. It is safe to call on a partially
// constructed Workspace and to call more than once.
func (w *Workspace) Close() error {
	var errs []error
	for _, dir := range []string{w.Root, w.OutputDir} {
		if dir == "" {
			continue
		}
		if err := os.RemoveAll(dir); err != nil {
			errs = append(errs, fmt.Errorf("remove %q: %w", dir, err))
		}
	}
	return errors.Join(errs...)
}
