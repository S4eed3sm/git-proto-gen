// Package source materializes .proto files from local directories and remote
// git repositories into a staging directory. Remote fetching is vendor-agnostic:
// every repository is obtained with a single `git clone` path (HTTPS with a
// token or SSH), so GitHub, self-hosted GitLab, Bitbucket, Gitea and others all
// work without per-vendor code.
package source

import "context"

// Source materializes one source's .proto files under dst and reports where
// they landed. dst already exists; implementations create subdirectories as
// needed.
type Source interface {
	Fetch(ctx context.Context, dst string) (*Result, error)
}

// Result describes what a Fetch produced, for the orchestrator and the optional
// import rewriter.
type Result struct {
	// RepoName is the repository's base name, used as the import namespace when
	// rewriting is enabled. Empty for local sources.
	RepoName string
	// LocalDir is the absolute directory the .proto files were written to.
	LocalDir string
}
