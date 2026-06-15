// Command git-proto-gen generates code from .proto files sourced from local
// directories or remote git repositories (GitHub, self-hosted GitLab, Bitbucket,
// Gitea, ...) by running buf, in Docker or with a host buf binary.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/S4eed3sm/git-proto-gen/internal/app"
	"github.com/S4eed3sm/git-proto-gen/internal/config"
	"github.com/S4eed3sm/git-proto-gen/internal/generate"
	"github.com/S4eed3sm/git-proto-gen/internal/source"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "git-proto-gen:", err)
		os.Exit(1)
	}
}

func run() error {
	// Cancel in-flight work (clones, container exec) on Ctrl-C / SIGTERM so the
	// deferred cleanup of temp dirs and the container still runs.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Parse(os.Args[1:])
	if err != nil {
		return err
	}
	if cfg == nil {
		return nil // --help or --version was handled
	}

	log := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	var runner source.GitRunner
	if len(cfg.Repos) > 0 {
		runner, err = source.NewExecGitRunner()
		if err != nil {
			return err
		}
	}
	auth, err := source.NewEnvAuthResolver(cfg.Token, cfg.AuthConfigPath)
	if err != nil {
		return err
	}
	builder := app.NewSourceBuilder(auth, runner)

	gen, err := generate.New(&generate.Options{
		Runner:    cfg.Runner,
		Image:     cfg.Image,
		Tag:       cfg.ImageTag,
		MemoryGiB: cfg.MemoryGiB,
	}, log)
	if err != nil {
		return err
	}

	return app.New(builder, gen, log).Run(ctx, cfg)
}
