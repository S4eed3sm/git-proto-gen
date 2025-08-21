package main

import (
	"embed"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

const (
	bufYamlFileName      = "buf.yaml"
	bufGenGoYamlFileName = "buf.gen.go.yaml"
	bufGenJsYamlFileName = "buf.gen.js.yaml"
)

//go:embed buf/buf.yaml
var f embed.FS
var bufYamlContent, _ = f.ReadFile("buf/buf.yaml")

//go:embed buf/buf.gen.go.yaml
var f1 embed.FS
var bufGenGoYamlContent, _ = f1.ReadFile("buf/buf.gen.go.yaml")

//go:embed buf/buf.gen.js.yaml
var f2 embed.FS
var bufGenJsYamlContent, _ = f2.ReadFile("buf/buf.gen.js.yaml")

type GithubAuthMethodType string

const (
	GithubAuthMethodSSH   GithubAuthMethodType = "ssh"
	GithubAuthMethodToken GithubAuthMethodType = "token"
)

type Config struct {
	LocalPath              string
	PrivateRepos           []string
	PublicRepos            []string
	OutputPath             string
	Languages              []string
	GithubToken            string
	GithubAuthMethod       GithubAuthMethodType
	OptionalBufConfigsPath string
}

func parseArgs() (*Config, error) {
	var cfg Config
	cmd := &cobra.Command{
		Use:   "git-proto-gen",
		Short: "Generate code from .proto files",
		Long:  "A CLI tool for generating code from .proto definitions from local or remote GitHub sources.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if cfg.LocalPath == "" && len(cfg.PrivateRepos) == 0 && len(cfg.PublicRepos) == 0 {
				return errors.New("you must provide at least one of --local, --private-repo, or --public-repo")
			}

			allowed := map[string]bool{"go": true, "js": true}
			for _, lang := range cfg.Languages {
				if !allowed[lang] {
					return fmt.Errorf("invalid language '%s'. Allowed values: go, js", lang)
				}
			}

			if len(cfg.Languages) == 0 {
				return errors.New("you must provide at least one --lang (go, js, or both)")
			}

			if len(cfg.PrivateRepos) > 0 && cfg.GithubToken == "" && !checkSSHKeys() {
				return errors.New("you must provide a GitHub token with --token for private repos or have SSH keys configured")
			}

			if len(cfg.PrivateRepos) > 0 {
				if cfg.GithubToken == "" {
					cfg.GithubAuthMethod = GithubAuthMethodSSH
				} else {
					cfg.GithubAuthMethod = GithubAuthMethodToken
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&cfg.LocalPath, "local", "", "Path to local .proto files, e.g: './proto'")
	cmd.Flags().StringSliceVar(&cfg.PrivateRepos, "private-repo", nil, `GitHub path(s) to private proto repos, branch name is optional (repeatable, comma-separated), e.g: "github.com/S4eed3sm/private-test-proto/proto@main"`)
	cmd.Flags().StringSliceVar(&cfg.PublicRepos, "public-repo", nil, `GitHub path(s) to public proto repos, branch name is optional (repeatable, comma-separated), e.g: "github.com/S4eed3sm/public-test-proto/proto@dev"`)
	cmd.Flags().StringVar(&cfg.OutputPath, "output", "events", "Output directory for generated files")
	cmd.Flags().StringSliceVar(&cfg.Languages, "lang", []string{"go", "js"}, "Target language(s) for code generation: go, js (comma-separated or repeatable)")
	cmd.Flags().StringVar(&cfg.OptionalBufConfigsPath, "buf-configs", "", "Path to optional buf config files (buf.yaml, buf.gen.go.yaml, buf.gen.js.yaml)")
	cmd.Flags().StringVar(&cfg.GithubToken, "token", "", "GitHub token for private repos")

	if err := cmd.Execute(); err != nil {
		return nil, err
	}
	return &cfg, nil
}
