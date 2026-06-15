package fsutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCopyTreeWithFilter(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	mk := func(rel, content string) {
		p := filepath.Join(src, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk("a.proto", "a")
	mk("sub/b.proto", "b")
	mk("sub/README.md", "ignore me")

	keepProto := func(name string) bool { return strings.HasSuffix(name, ".proto") }
	if err := CopyTree(src, dst, keepProto); err != nil {
		t.Fatalf("CopyTree: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dst, "a.proto")); err != nil {
		t.Errorf("a.proto not copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "sub", "b.proto")); err != nil {
		t.Errorf("sub/b.proto not copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "sub", "README.md")); !os.IsNotExist(err) {
		t.Errorf("README.md should have been filtered out, err=%v", err)
	}
}

func TestCopyTreeOverwrites(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "f.txt"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dst, "f.txt"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := CopyTree(src, dst, nil); err != nil {
		t.Fatalf("CopyTree: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dst, "f.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Errorf("expected overwrite to 'new', got %q", got)
	}
}
