package generate

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
)

// hostGenerator runs the buf binary installed on the host.
type hostGenerator struct {
	bufPath string
	log     *slog.Logger
}

func newHostGenerator(log *slog.Logger) (Generator, error) {
	p, err := exec.LookPath("buf")
	if err != nil {
		return nil, fmt.Errorf("buf binary not found in PATH (required for the host runner): %w", err)
	}
	return &hostGenerator{bufPath: p, log: log}, nil
}

func (g *hostGenerator) Generate(ctx context.Context, workspaceDir, outDir string, jobs []*Job) error {
	for _, job := range jobs {
		tmpl := filepath.Join(workspaceDir, job.TemplateFile)
		cmd := exec.CommandContext(ctx, g.bufPath,
			"generate", ".", "--template", tmpl, "--output", outDir)
		cmd.Dir = workspaceDir

		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("buf generate (%s): %w: %s", job.Lang, err, out)
		}
		g.log.Info("generated code", "lang", job.Lang)
	}
	return nil
}
