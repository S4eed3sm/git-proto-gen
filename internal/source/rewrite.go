package source

import (
	"bufio"
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// importLine matches a proto import statement, capturing the prefix (up to and
// including the opening quote), the imported path, and the trailing remainder
// (closing quote, optional comment). It deliberately requires `import` at the
// start of the trimmed line, so commented-out imports beginning with "//" do
// not match.
var importLine = regexp.MustCompile(`^(\s*import\s+(?:public\s+|weak\s+)?")([^"]+)(".*)$`)

// RewriteImports namespaces module-root-relative imports in every .proto file
// under result.LocalDir by prefixing them with the repository name, so protos
// merged from multiple repositories do not collide. An import is rewritten only
// when its target resolves to a file inside this repository's tree; imports
// that resolve elsewhere (e.g. google/protobuf/*) are left untouched.
//
// It is opt-in because it mutates proto source.
func RewriteImports(result *Result) error {
	if result == nil || result.RepoName == "" {
		return nil
	}
	root := result.LocalDir
	prefix := result.RepoName + "/"

	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".proto") {
			return nil
		}
		return rewriteFile(path, root, prefix)
	})
}

func rewriteFile(path, root, prefix string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %q: %w", path, err)
	}

	var out bytes.Buffer
	changed := false
	inBlockComment := false

	scanner := bufio.NewScanner(bytes.NewReader(content))
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		rewritten := line

		if !inBlockComment {
			if m := importLine.FindStringSubmatch(line); m != nil {
				target := m[2]
				if !strings.HasPrefix(target, prefix) && importResolvesIn(root, target) {
					rewritten = m[1] + prefix + target + m[3]
					changed = true
				}
			}
		}
		inBlockComment = trackBlockComment(line, inBlockComment)

		out.WriteString(rewritten)
		out.WriteByte('\n')
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan %q: %w", path, err)
	}
	if !changed {
		return nil
	}

	// Preserve absence of a trailing newline in the original.
	result := out.Bytes()
	if len(content) > 0 && content[len(content)-1] != '\n' && len(result) > 0 {
		result = result[:len(result)-1]
	}
	if err := os.WriteFile(path, result, 0o644); err != nil {
		return fmt.Errorf("write %q: %w", path, err)
	}
	return nil
}

// importResolvesIn reports whether an import path points to a .proto file that
// exists within root.
func importResolvesIn(root, importPath string) bool {
	candidate := filepath.Join(root, filepath.FromSlash(importPath))
	info, err := os.Stat(candidate)
	return err == nil && !info.IsDir()
}

// trackBlockComment updates whether the scanner is inside a /* */ block after
// the given line. It is a pragmatic approximation: it does not parse strings,
// which is acceptable because importLine only matches genuine import lines.
func trackBlockComment(line string, inBlock bool) bool {
	for {
		if inBlock {
			i := strings.Index(line, "*/")
			if i < 0 {
				return true
			}
			line = line[i+2:]
			inBlock = false
			continue
		}
		i := strings.Index(line, "/*")
		if i < 0 {
			return false
		}
		line = line[i+2:]
		inBlock = true
	}
}
