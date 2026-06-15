package source

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// fakeGitRunner records invocations and, on checkout, materializes a fixture
// .proto file so the copy step has something to find.
type fakeGitRunner struct {
	calls   []*RunOpts
	cloneTo string // the temp dir git would clone into (last clone arg)
}

func (f *fakeGitRunner) Run(_ context.Context, opts *RunOpts) ([]byte, error) {
	f.calls = append(f.calls, opts)
	// "clone" may be preceded by global `-c` config args, so scan for it.
	if slices.Contains(opts.Args, "clone") {
		f.cloneTo = opts.Args[len(opts.Args)-1]
	}
	// the last non-flag git subcommand decides what to fake
	if slices.Contains(opts.Args, "checkout") {
		dir := filepath.Join(f.cloneTo, "proto")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(dir, "greeting.proto"), []byte("syntax = \"proto3\";\n"), 0o644); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func argString(opts *RunOpts) string { return strings.Join(opts.Args, " ") }

func TestCloneSourceTokenAuth(t *testing.T) {
	spec := &Spec{Host: "gitlab.example.com", RepoPath: "group/sub/proj", Subdir: "proto", Ref: "main"}
	cred := &Credential{Kind: KindToken, token: "secret-token"}
	runner := &fakeGitRunner{}
	dst := t.TempDir()

	res, err := NewCloneSource(spec, cred, runner).Fetch(context.Background(), dst)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if res.RepoName != "proj" {
		t.Errorf("RepoName = %q, want proj", res.RepoName)
	}

	// Cloned file copied under dst/<repo>.
	if _, err := os.Stat(filepath.Join(dst, "proj", "greeting.proto")); err != nil {
		t.Errorf("expected copied proto: %v", err)
	}

	clone := runner.calls[0]
	got := argString(clone)
	for _, want := range []string{"clone", "--depth 1", "--no-checkout", "--filter=blob:none", "--branch main", "https://gitlab.example.com/group/sub/proj.git"} {
		if !strings.Contains(got, want) {
			t.Errorf("clone args %q missing %q", got, want)
		}
	}

	// Token must travel via env + credential helper, never in argv.
	if strings.Contains(got, "secret-token") {
		t.Errorf("token leaked into git args: %q", got)
	}
	if !slices.Contains(clone.Args, "credential.helper=") {
		t.Errorf("expected credential.helper config arg, got %q", got)
	}
	foundEnv := false
	for _, e := range clone.Env {
		if strings.HasPrefix(e, tokenEnvVar+"=") {
			if !strings.HasSuffix(e, "secret-token") {
				t.Errorf("token env malformed: %q", e)
			}
			foundEnv = true
		}
	}
	if !foundEnv {
		t.Errorf("token not passed via %s env var", tokenEnvVar)
	}
}

func TestCloneSourceSSHAuth(t *testing.T) {
	spec := &Spec{Host: "github.com", RepoPath: "o/r", Subdir: "proto"}
	cred := &Credential{Kind: KindSSH}
	runner := &fakeGitRunner{}

	if _, err := NewCloneSource(spec, cred, runner).Fetch(context.Background(), t.TempDir()); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	clone := runner.calls[0]
	got := argString(clone)
	if !strings.Contains(got, "git@github.com:o/r.git") {
		t.Errorf("ssh clone should use scp url, got %q", got)
	}
	if slices.Contains(clone.Args, "credential.helper=") || len(clone.Env) != 0 {
		t.Errorf("ssh clone must not set credential helper or token env, args=%q env=%v", got, clone.Env)
	}
}

func TestCloneSourceFiltersFallback(t *testing.T) {
	// A runner that fails the first (filtered) clone forces the no-filter retry.
	spec := &Spec{Host: "github.com", RepoPath: "o/r", Subdir: "proto"}
	runner := &filterRejectingRunner{inner: &fakeGitRunner{}}

	if _, err := NewCloneSource(spec, &Credential{Kind: KindAnonymous}, runner).Fetch(context.Background(), t.TempDir()); err != nil {
		t.Fatalf("Fetch with filter fallback: %v", err)
	}
	if !runner.sawFilteredClone || !runner.sawUnfilteredClone {
		t.Errorf("expected filtered clone then unfiltered retry; filtered=%v unfiltered=%v",
			runner.sawFilteredClone, runner.sawUnfilteredClone)
	}
}

type filterRejectingRunner struct {
	inner              *fakeGitRunner
	sawFilteredClone   bool
	sawUnfilteredClone bool
}

func (r *filterRejectingRunner) Run(ctx context.Context, opts *RunOpts) ([]byte, error) {
	if slices.Contains(opts.Args, "clone") {
		if slices.Contains(opts.Args, "--filter=blob:none") {
			r.sawFilteredClone = true
			return []byte("server does not support partial clone"), os.ErrInvalid
		}
		r.sawUnfilteredClone = true
	}
	return r.inner.Run(ctx, opts)
}
