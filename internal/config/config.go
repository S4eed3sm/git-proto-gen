// Package config parses command-line flags into a Config. Config holds only raw
// flag values; derived values (such as per-host auth) are computed downstream.
package config

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/S4eed3sm/git-proto-gen/internal/version"
)

// Config is the parsed, raw CLI configuration for one run.
type Config struct {
	LocalPath        string   // --local: directory of local .proto files
	Repos            []string // --repo (+ deprecated --private-repo/--public-repo)
	OutputPath       string   // --output: destination directory for generated code
	Languages        []string // --lang: go, js
	Token            string   // --token: legacy github.com token
	AuthConfigPath   string   // --auth-config: per-host credential file
	BufConfigsPath   string   // --buf-configs: override buf config directory
	NamespaceImports bool     // --namespace-imports: rewrite imports to namespace by repo
	Runner           string   // --runner: auto | docker | host
	Image            string   // --image: buf Docker image
	ImageTag         string   // --image-tag: buf Docker image tag
	MemoryGiB        int      // --memory-gib: container memory limit
}

var allowedLanguages = map[string]bool{"go": true, "js": true}

// Validate checks the cross-flag invariants. It is independent of cobra so it
// can be unit-tested directly.
func (c *Config) Validate() error {
	if c.LocalPath == "" && len(c.Repos) == 0 {
		return errors.New("provide at least one of --local or --repo")
	}
	if len(c.Languages) == 0 {
		return errors.New("provide at least one --lang (go, js, or both)")
	}
	for _, lang := range c.Languages {
		if !allowedLanguages[lang] {
			return fmt.Errorf("invalid language %q: allowed values are go, js", lang)
		}
	}
	return nil
}

// Parse parses args into a Config. It returns (nil, nil) when cobra handled the
// invocation itself (--help or --version) and there is nothing to run.
func Parse(args []string) (*Config, error) {
	cfg := &Config{}
	var privateRepos, publicRepos []string
	ran := false

	cmd := &cobra.Command{
		Use:     "git-proto-gen",
		Short:   "Generate code from .proto files in local dirs or any git vendor",
		Long:    "A CLI tool for generating code from .proto definitions sourced from local directories or remote git repositories (GitHub, self-hosted GitLab, Bitbucket, Gitea, ...).",
		Version: version.Version,
		// Errors and usage are surfaced by the caller, not printed twice by cobra.
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			ran = true
			cfg.Repos = append(cfg.Repos, privateRepos...)
			cfg.Repos = append(cfg.Repos, publicRepos...)
			return cfg.Validate()
		},
	}

	f := cmd.Flags()
	f.StringVar(&cfg.LocalPath, "local", "", "path to local .proto files, e.g. './proto'")
	f.StringSliceVar(&cfg.Repos, "repo", nil,
		`remote proto repo(s) (repeatable), form: host/owner/repo//subdir@ref, e.g. "gitlab.example.com/group/sub/proj//proto@main"`)
	f.StringSliceVar(&privateRepos, "private-repo", nil, "deprecated: use --repo")
	f.StringSliceVar(&publicRepos, "public-repo", nil, "deprecated: use --repo")
	f.StringVar(&cfg.OutputPath, "output", "events", "destination directory for generated files")
	f.StringSliceVar(&cfg.Languages, "lang", []string{"go", "js"}, "target language(s): go, js (comma-separated or repeatable)")
	f.StringVar(&cfg.Token, "token", "", "legacy github.com token; prefer GIT_PROTO_GEN_TOKEN_<HOST> or --auth-config")
	f.StringVar(&cfg.AuthConfigPath, "auth-config", "", "path to a per-host credential YAML file")
	f.StringVar(&cfg.BufConfigsPath, "buf-configs", "", "directory with override buf config files (buf.yaml, buf.gen.go.yaml, buf.gen.js.yaml)")
	f.BoolVar(&cfg.NamespaceImports, "namespace-imports", false, "rewrite proto imports to namespace them by repo name (avoids collisions when merging repos)")
	f.StringVar(&cfg.Runner, "runner", "auto", "buf runner: auto, docker, or host")
	f.StringVar(&cfg.Image, "image", "bufbuild/buf", "buf Docker image (docker runner)")
	f.StringVar(&cfg.ImageTag, "image-tag", "1.54.0", "buf Docker image tag (docker runner)")
	f.IntVar(&cfg.MemoryGiB, "memory-gib", 2, "container memory limit in GiB (docker runner)")

	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		return nil, err
	}
	if !ran {
		return nil, nil
	}
	return cfg, nil
}
