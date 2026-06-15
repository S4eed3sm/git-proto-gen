package source

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRewriteImports(t *testing.T) {
	root := t.TempDir()
	// A repo whose service imports a sibling proto by module-root-relative path,
	// plus a well-known import that must NOT be namespaced.
	write(t, filepath.Join(root, "greeting.proto"), `syntax = "proto3";
package greeting;
`)
	write(t, filepath.Join(root, "service.proto"), `syntax = "proto3";
import "greeting.proto";
import "google/protobuf/timestamp.proto";
// import "greeting.proto";  // commented out, must be ignored
/* import "greeting.proto"; */
`)

	if err := RewriteImports(&Result{RepoName: "myrepo", LocalDir: root}); err != nil {
		t.Fatalf("RewriteImports: %v", err)
	}

	got := read(t, filepath.Join(root, "service.proto"))
	if !strings.Contains(got, `import "myrepo/greeting.proto";`) {
		t.Errorf("expected local import to be namespaced, got:\n%s", got)
	}
	if !strings.Contains(got, `import "google/protobuf/timestamp.proto";`) {
		t.Errorf("well-known import must be left untouched, got:\n%s", got)
	}
	if strings.Contains(got, `import "myrepo/google/protobuf`) {
		t.Errorf("well-known import was wrongly namespaced, got:\n%s", got)
	}
	// The commented-out and block-comment lines must remain unchanged.
	if strings.Count(got, "myrepo/greeting.proto") != 1 {
		t.Errorf("only the real import should be rewritten, got:\n%s", got)
	}
}

func TestRewriteImportsIdempotent(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "a.proto"), "syntax = \"proto3\";\n")
	write(t, filepath.Join(root, "b.proto"), "import \"a.proto\";\n")
	res := &Result{RepoName: "r", LocalDir: root}

	if err := RewriteImports(res); err != nil {
		t.Fatal(err)
	}
	first := read(t, filepath.Join(root, "b.proto"))
	if err := RewriteImports(res); err != nil {
		t.Fatal(err)
	}
	if second := read(t, filepath.Join(root, "b.proto")); second != first {
		t.Errorf("rewrite not idempotent:\n first:  %q\n second: %q", first, second)
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
