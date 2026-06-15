package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderDefaultsSubstituteMarker(t *testing.T) {
	r, err := NewRender("")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{genGoFile, genJSFile} {
		content, err := r.file(name)
		if err != nil {
			t.Fatalf("file(%s): %v", name, err)
		}
		s := string(content)
		if strings.Contains(s, outputMarker) {
			t.Errorf("%s still contains marker %q:\n%s", name, outputMarker, s)
		}
		if !strings.Contains(s, "out: .") {
			t.Errorf("%s expected rendered 'out: .':\n%s", name, s)
		}
	}
}

func TestRenderOverrideNormalizesOutAndTakesPrecedence(t *testing.T) {
	dir := t.TempDir()
	// User override declares a custom out: that must be normalized so the
	// generator controls the output dir; a marker token confirms the override
	// (not the embedded default) was used.
	override := "version: v2\nplugins:\n  - local: protoc-gen-es\n    out: /some/custom/path\n    opt: target=ts # OVERRIDE_MARKER\n"
	if err := os.WriteFile(filepath.Join(dir, genJSFile), []byte(override), 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := NewRender(dir)
	if err != nil {
		t.Fatal(err)
	}
	content, err := r.file(genJSFile)
	if err != nil {
		t.Fatal(err)
	}
	s := string(content)
	if !strings.Contains(s, "OVERRIDE_MARKER") {
		t.Errorf("override not used:\n%s", s)
	}
	if strings.Contains(s, "/some/custom/path") {
		t.Errorf("custom out: not normalized:\n%s", s)
	}
	if !strings.Contains(s, "out: .") {
		t.Errorf("expected normalized 'out: .':\n%s", s)
	}
}

func TestWorkspaceLifecycle(t *testing.T) {
	r, err := NewRender("")
	if err != nil {
		t.Fatal(err)
	}
	ws, err := New(r)
	if err != nil {
		t.Fatal(err)
	}

	for _, dir := range []string{ws.Root, ws.ProtoDir, ws.OutputDir} {
		if _, err := os.Stat(dir); err != nil {
			t.Errorf("expected %q to exist: %v", dir, err)
		}
	}
	for _, name := range []string{bufYamlFile, genGoFile, genJSFile} {
		if _, err := os.Stat(filepath.Join(ws.Root, name)); err != nil {
			t.Errorf("expected config %q in workspace: %v", name, err)
		}
	}

	if err := ws.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := os.Stat(ws.Root); !os.IsNotExist(err) {
		t.Errorf("Root not removed after Close: %v", err)
	}
	if _, err := os.Stat(ws.OutputDir); !os.IsNotExist(err) {
		t.Errorf("OutputDir not removed after Close (leak): %v", err)
	}
}
