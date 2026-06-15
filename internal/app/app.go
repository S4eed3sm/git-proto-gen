// Package app wires the source, workspace and generate layers into the
// end-to-end run: fetch protos -> stage into a workspace -> run buf -> copy the
// generated code to the output directory. It depends only on small interfaces,
// so it is unit-testable with fakes.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/S4eed3sm/git-proto-gen/internal/config"
	"github.com/S4eed3sm/git-proto-gen/internal/fsutil"
	"github.com/S4eed3sm/git-proto-gen/internal/generate"
	"github.com/S4eed3sm/git-proto-gen/internal/source"
	"github.com/S4eed3sm/git-proto-gen/internal/workspace"
)

// SourceBuilder constructs sources for the orchestrator. The default
// implementation builds real local/clone sources; tests provide fakes.
type SourceBuilder interface {
	Local(dir string) source.Source
	Remote(spec *source.Spec) (source.Source, error)
}

// App is the generation orchestrator.
type App struct {
	build SourceBuilder
	gen   generate.Generator
	log   *slog.Logger
}

// New returns an App wired with a source builder and a generator.
func New(build SourceBuilder, gen generate.Generator, log *slog.Logger) *App {
	return &App{build: build, gen: gen, log: log}
}

// Run executes the full pipeline. The workspace (and its temp dirs) is always
// cleaned up, and ctx cancellation propagates to fetching and generation.
func (a *App) Run(ctx context.Context, cfg *config.Config) error {
	render, err := workspace.NewRender(cfg.BufConfigsPath)
	if err != nil {
		return err
	}
	ws, err := workspace.New(render)
	if err != nil {
		return err
	}
	defer func() {
		if err := ws.Close(); err != nil {
			a.log.Warn("failed to clean up workspace", "err", err)
		}
	}()

	if err := a.fetchAll(ctx, cfg, ws.ProtoDir); err != nil {
		return err
	}

	jobs := make([]*generate.Job, 0, len(cfg.Languages))
	for _, lang := range cfg.Languages {
		jobs = append(jobs, &generate.Job{Lang: lang, TemplateFile: ws.TemplateFile(lang)})
	}
	if err := a.gen.Generate(ctx, ws.Root, ws.OutputDir, jobs); err != nil {
		return err
	}

	dest, err := filepath.Abs(cfg.OutputPath)
	if err != nil {
		return fmt.Errorf("resolve output path %q: %w", cfg.OutputPath, err)
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return fmt.Errorf("create output dir %q: %w", dest, err)
	}
	if err := fsutil.CopyTree(ws.OutputDir, dest, nil); err != nil {
		return fmt.Errorf("copy generated code to %q: %w", dest, err)
	}
	a.log.Info("generation complete", "output", dest)
	return nil
}

func (a *App) fetchAll(ctx context.Context, cfg *config.Config, protoDir string) error {
	if cfg.LocalPath != "" {
		absLocal, err := filepath.Abs(cfg.LocalPath)
		if err != nil {
			return fmt.Errorf("resolve local path %q: %w", cfg.LocalPath, err)
		}
		if _, err := a.build.Local(absLocal).Fetch(ctx, protoDir); err != nil {
			return fmt.Errorf("fetch local protos: %w", err)
		}
		a.log.Info("staged local protos", "path", absLocal)
	}

	for _, raw := range cfg.Repos {
		spec, err := source.ParseSpec(raw)
		if err != nil {
			return err
		}
		if spec.Legacy {
			a.log.Warn("deprecated repo spec; use the '//' separator between repo and subdir",
				"spec", raw, "suggested", spec.Host+"/"+spec.RepoPath+"//"+spec.Subdir)
		}
		src, err := a.build.Remote(spec)
		if err != nil {
			return fmt.Errorf("repo %q: %w", raw, err)
		}
		res, err := src.Fetch(ctx, protoDir)
		if err != nil {
			return fmt.Errorf("fetch repo %q: %w", raw, err)
		}
		if cfg.NamespaceImports {
			if err := source.RewriteImports(res); err != nil {
				return fmt.Errorf("namespace imports for %q: %w", raw, err)
			}
		}
		a.log.Info("staged remote protos", "repo", spec.RepoPath, "host", spec.Host)
	}
	return nil
}
