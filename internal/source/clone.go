package source

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/S4eed3sm/git-proto-gen/internal/fsutil"
)

// tokenEnvVar is the environment variable the per-invocation credential helper
// reads the token from. The token is passed via the environment, never as a
// command-line argument, so it does not appear in `ps` output.
const tokenEnvVar = "GIT_PROTO_GEN_CRED_TOKEN"

var _ Source = (*cloneSource)(nil)

// cloneSource fetches a remote repository with a shallow, partial, sparse
// `git clone`, so only the blobs under the requested subdirectory are
// downloaded. It is vendor-agnostic: the same path serves GitHub, GitLab,
// Bitbucket, Gitea and any other git host.
type cloneSource struct {
	spec   *Spec
	cred   *Credential
	runner GitRunner
}

// NewCloneSource returns a Source that clones spec using cred via runner.
func NewCloneSource(spec *Spec, cred *Credential, runner GitRunner) Source {
	return &cloneSource{spec: spec, cred: cred, runner: runner}
}

func (s *cloneSource) Fetch(ctx context.Context, dst string) (*Result, error) {
	tmp, err := os.MkdirTemp("", "gpg-clone-")
	if err != nil {
		return nil, fmt.Errorf("create clone temp dir: %w", err)
	}
	defer os.RemoveAll(tmp)

	useSSH := s.cred != nil && s.cred.Kind == KindSSH
	url := s.spec.cloneURL(useSSH)
	authArgs, env := s.authConfig()

	// Shallow + partial clone with no checkout, so the subsequent sparse
	// checkout fetches only the blobs we need.
	clone := func(filter bool) error {
		args := append([]string{}, authArgs...)
		args = append(args, "clone", "--depth", "1", "--no-checkout")
		if filter {
			args = append(args, "--filter=blob:none")
		}
		if s.spec.Ref != "" {
			args = append(args, "--branch", s.spec.Ref)
		}
		args = append(args, url, tmp)
		_, err := s.runner.Run(ctx, &RunOpts{Args: args, Env: env})
		return err
	}

	if err := clone(true); err != nil {
		// Some servers reject partial clone; retry without the blob filter.
		if err := clone(false); err != nil {
			return nil, fmt.Errorf("clone %s: %w", s.spec.RepoPath, err)
		}
	}

	// Restrict the working tree to the proto subdir before checkout. Best
	// effort: if sparse-checkout is unsupported, the full tree is checked out
	// and the copy step below still selects only the subdir.
	if s.spec.Subdir != "" {
		_, _ = s.runner.Run(ctx, &RunOpts{
			Dir:  tmp,
			Args: []string{"sparse-checkout", "set", "--no-cone", s.spec.Subdir},
		})
	}

	// Checkout materializes the working tree; with a partial clone this is where
	// the in-scope blobs are fetched, so it needs auth too.
	coArgs := append([]string{}, authArgs...)
	coArgs = append(coArgs, "checkout")
	if _, err := s.runner.Run(ctx, &RunOpts{Dir: tmp, Args: coArgs, Env: env}); err != nil {
		return nil, fmt.Errorf("checkout %s: %w", s.spec.RepoPath, err)
	}

	repo := s.spec.RepoName()
	srcSub := filepath.Join(tmp, filepath.FromSlash(s.spec.Subdir))
	destDir := filepath.Join(dst, repo)
	if err := fsutil.CopyTree(srcSub, destDir, isProto); err != nil {
		return nil, fmt.Errorf("copy protos from %s: %w", s.spec.RepoPath, err)
	}
	return &Result{RepoName: repo, LocalDir: destDir}, nil
}

// authConfig returns the git `-c` arguments and environment that authenticate
// an HTTPS clone via a credential helper, keeping the token out of argv. For
// SSH or anonymous access it returns nothing.
func (s *cloneSource) authConfig() (args, env []string) {
	if s.cred == nil || s.cred.Kind != KindToken {
		return nil, nil
	}
	// The helper prints username/password on git's "get" action, reading the
	// secret from the environment. The leading empty helper clears any
	// system/global helper so ours is the only one consulted.
	helper := `!f() { test "$1" = get && echo username=oauth2 && echo "password=$` + tokenEnvVar + `"; }; f`
	return []string{"-c", "credential.helper=", "-c", "credential.helper=" + helper},
		[]string{tokenEnvVar + "=" + s.cred.token}
}
