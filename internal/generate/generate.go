// Package generate runs `buf generate` against a prepared workspace, either in
// a Docker container (default) or with a host buf binary. A single backend is
// reused across all language jobs so per-language toolchain setup happens once.
package generate

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
)

// Runner selection values for Options.Runner.
const (
	RunnerAuto   = "auto"
	RunnerDocker = "docker"
	RunnerHost   = "host"
)

// Options configures the generator. Zero values are replaced by defaults.
type Options struct {
	Runner    string // auto | docker | host
	Image     string // Docker image, default "bufbuild/buf"
	Tag       string // Docker image tag, default "1.54.0"
	MemoryGiB int    // container memory limit in GiB, default 2
}

func (o *Options) withDefaults() *Options {
	out := *o
	if out.Runner == "" {
		out.Runner = RunnerAuto
	}
	if out.Image == "" {
		out.Image = "bufbuild/buf"
	}
	if out.Tag == "" {
		out.Tag = "1.54.0"
	}
	if out.MemoryGiB <= 0 {
		out.MemoryGiB = 2
	}
	return &out
}

// Job is a single language generation: which template within the workspace to
// run buf with.
type Job struct {
	Lang         string // "go" | "js"
	TemplateFile string // buf gen template file name, relative to the workspace root
}

// Generator runs buf for a set of jobs against a prepared workspace, writing
// generated code into outDir.
type Generator interface {
	Generate(ctx context.Context, workspaceDir, outDir string, jobs []*Job) error
}

// New selects a generator per opts.Runner. "auto" prefers a host buf binary and
// falls back to Docker.
func New(opts *Options, log *slog.Logger) (Generator, error) {
	o := opts.withDefaults()
	switch o.Runner {
	case RunnerHost:
		return newHostGenerator(log)
	case RunnerDocker:
		return newDockerGenerator(o, log), nil
	case RunnerAuto:
		if _, err := exec.LookPath("buf"); err == nil {
			return newHostGenerator(log)
		}
		return newDockerGenerator(o, log), nil
	default:
		return nil, fmt.Errorf("unknown runner %q (want auto, docker or host)", o.Runner)
	}
}
