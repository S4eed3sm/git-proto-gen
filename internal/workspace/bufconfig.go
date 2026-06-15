package workspace

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"

	"github.com/S4eed3sm/git-proto-gen/buf"
)

// Buf configuration file names, shared by the embedded defaults and any
// user-supplied overrides.
const (
	bufYamlFile = "buf.yaml"
	genGoFile   = "buf.gen.go.yaml"
	genJSFile   = "buf.gen.js.yaml"

	// outputMarker is the placeholder in the embedded gen templates for each
	// plugin's output directory. It is rendered to "." so generated files land
	// directly under buf's --output directory, which the generator controls.
	outputMarker = "__events__"
)

// outLine matches a plugin `out:` line so user-supplied gen configs can have
// their output directory normalized to the marker. The (?m) flag makes `$`
// match each line end, so every plugin's `out:` is rewritten (the templates
// declare more than one plugin).
var outLine = regexp.MustCompile(`(?m)^(\s*)out:.*$`)

// Render produces buf configuration files from the embedded defaults plus any
// optional on-disk overrides. It holds no package-level state and mutates
// nothing, so it is safe to construct per run and to unit-test.
type Render struct {
	overrides map[string][]byte
}

// NewRender loads override files (buf.yaml, buf.gen.go.yaml, buf.gen.js.yaml)
// from overrideDir, falling back to the embedded defaults for any that are
// absent. overrideDir may be empty to use only the embedded defaults.
func NewRender(overrideDir string) (*Render, error) {
	r := &Render{overrides: make(map[string][]byte)}
	if overrideDir == "" {
		return r, nil
	}

	for _, name := range []string{bufYamlFile, genGoFile, genJSFile} {
		content, ok, err := readIfExists(filepath.Join(overrideDir, name))
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		// Normalize plugin output dirs in user gen configs to the marker so the
		// generator, not the user's config, decides where files are written.
		if name != bufYamlFile {
			content = outLine.ReplaceAll(content, []byte("${1}out: "+outputMarker))
		}
		r.overrides[name] = content
	}
	return r, nil
}

// file returns the rendered bytes for one config file: the override if present,
// otherwise the embedded default, with the output marker substituted.
func (r *Render) file(name string) ([]byte, error) {
	content, ok := r.overrides[name]
	if !ok {
		var err error
		content, err = buf.FS.ReadFile(name)
		if err != nil {
			return nil, fmt.Errorf("read embedded %s: %w", name, err)
		}
	}
	return bytes.ReplaceAll(content, []byte(outputMarker), []byte(".")), nil
}

func readIfExists(path string) ([]byte, bool, error) {
	content, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read override %q: %w", path, err)
	}
	return content, true, nil
}
