package source

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// RunOpts are the inputs to one git invocation. Env entries are appended to the
// inherited environment, so secrets passed via a credential helper stay out of
// the process arguments visible in `ps`.
type RunOpts struct {
	Dir  string
	Args []string
	Env  []string
}

// GitRunner runs the git binary. It is the seam that makes cloneSource testable
// with a fake that records invocations.
type GitRunner interface {
	Run(ctx context.Context, opts *RunOpts) ([]byte, error)
}

// execGitRunner runs the real git binary found on PATH.
type execGitRunner struct {
	gitPath string
}

// NewExecGitRunner locates the git binary, returning an error if it is absent.
func NewExecGitRunner() (GitRunner, error) {
	p, err := exec.LookPath("git")
	if err != nil {
		return nil, fmt.Errorf("git binary not found in PATH (required for remote repos): %w", err)
	}
	return &execGitRunner{gitPath: p}, nil
}

func (r *execGitRunner) Run(ctx context.Context, opts *RunOpts) ([]byte, error) {
	cmd := exec.CommandContext(ctx, r.gitPath, opts.Args...)
	cmd.Dir = opts.Dir
	// GIT_TERMINAL_PROMPT=0 makes git fail fast instead of blocking on an
	// interactive credential prompt when auth is missing or wrong.
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	cmd.Env = append(cmd.Env, opts.Env...)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("git %s: %w: %s",
			strings.Join(redactArgs(opts.Args), " "), err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

// credInURL matches credentials embedded in a URL (scheme://user:pass@host) so
// they can be redacted from error messages.
var credInURL = regexp.MustCompile(`(://[^/@:]+):[^/@]*@`)

func redactArgs(args []string) []string {
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = credInURL.ReplaceAllString(a, "$1:redacted@")
	}
	return out
}
