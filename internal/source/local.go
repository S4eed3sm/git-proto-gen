package source

import (
	"context"
	"fmt"
	"strings"

	"github.com/S4eed3sm/git-proto-gen/internal/fsutil"
)

// localSource copies .proto files from a directory on the local filesystem.
type localSource struct {
	dir string
}

// NewLocalSource returns a Source that copies .proto files from dir.
func NewLocalSource(dir string) Source {
	return &localSource{dir: dir}
}

func (s *localSource) Fetch(_ context.Context, dst string) (*Result, error) {
	if err := fsutil.CopyTree(s.dir, dst, isProto); err != nil {
		return nil, fmt.Errorf("copy local protos from %q: %w", s.dir, err)
	}
	return &Result{LocalDir: dst}, nil
}

func isProto(name string) bool {
	return strings.HasSuffix(name, ".proto")
}
