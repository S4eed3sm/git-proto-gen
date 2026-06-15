package generate

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"path"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// containerOutputDir is a non-mounted directory inside the container where buf
// writes generated code.
const containerOutputDir = "/tmp/buf_generated"

// dockerGenerator runs buf inside one reused container for all jobs.
type dockerGenerator struct {
	opts *Options
	log  *slog.Logger
}

func newDockerGenerator(opts *Options, log *slog.Logger) Generator {
	return &dockerGenerator{opts: opts, log: log}
}

func (g *dockerGenerator) Generate(ctx context.Context, workspaceDir, outDir string, jobs []*Job) error {
	image := g.opts.Image + ":" + g.opts.Tag
	mem := int64(g.opts.MemoryGiB) * 1024 * 1024 * 1024

	req := testcontainers.ContainerRequest{
		Image:      image,
		WorkingDir: "/workspace",
		Entrypoint: []string{"sh"},
		Cmd:        []string{"-c", "tail -f /dev/null"},
		WaitingFor: wait.ForExec([]string{"echo", "ready"}).
			WithStartupTimeout(120 * time.Second).
			WithPollInterval(5 * time.Second),
		HostConfigModifier: func(hc *container.HostConfig) {
			hc.Binds = []string{
				fmt.Sprintf("%s:%s", workspaceDir, "/workspace"),
			}
			hc.Memory = mem
			hc.MemorySwap = mem
		},
	}

	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return fmt.Errorf("start buf container: %w", err)
	}
	defer func() {
		if err := c.Terminate(ctx); err != nil {
			g.log.Warn("failed to terminate buf container", "err", err)
		}
	}()

	jsReady := false
	for _, job := range jobs {
		if job.Lang == "js" && !jsReady {
			if err := g.installJSToolchain(ctx, c); err != nil {
				return err
			}
			jsReady = true
		}
		if err := g.runJob(ctx, c, job); err != nil {
			return err
		}
		g.log.Info("generated code", "lang", job.Lang)
	}
	return nil
}

func (g *dockerGenerator) runJob(ctx context.Context, c testcontainers.Container, job *Job) error {
	tmpl := path.Join("/workspace", job.TemplateFile)
	
	// Ensure the internal output dir exists
	_, _, _ = execAndRead(ctx, c, []string{"mkdir", "-p", containerOutputDir})

	var cmd []string
	if job.Lang == "js" {
		// protoc-gen-es is installed locally under node_modules/.bin.
		cmd = []string{"sh", "-c",
			"export PATH=./node_modules/.bin:$PATH && buf generate . --template " + tmpl +
				" --output " + containerOutputDir}
	} else {
		cmd = []string{"buf", "generate", ".", "--template", tmpl, "--output", containerOutputDir}
	}

	code, out, err := execAndRead(ctx, c, cmd)
	if err != nil {
		return fmt.Errorf("run buf generate (%s): %w", job.Lang, err)
	}
	if code != 0 {
		return fmt.Errorf("buf generate (%s) exited with status %d: %s", job.Lang, code, out)
	}

	// Copy the generated files to the bind-mounted directory
	copyCmd := []string{"sh", "-c", "cp -r " + containerOutputDir + "/* /workspace/temp_generated_output/"}
	code, out, err = execAndRead(ctx, c, copyCmd)
	if err != nil {
		return fmt.Errorf("copy generated files: %w", err)
	}
	if code != 0 {
		return fmt.Errorf("copy generated files (exit %d): %s", code, out)
	}

	return nil
}

func (g *dockerGenerator) installJSToolchain(ctx context.Context, c testcontainers.Container) error {
	steps := [][]string{
		{"apk", "add", "--no-cache", "nodejs", "npm", "python3", "make", "g++"},
		{"sh", "-c", "npm install --save-dev @bufbuild/protobuf @bufbuild/protoc-gen-es @bufbuild/buf 2>&1"},
	}
	for _, step := range steps {
		code, out, err := execAndRead(ctx, c, step)
		if err != nil {
			return fmt.Errorf("install js toolchain: %w", err)
		}
		if code != 0 {
			return fmt.Errorf("install js toolchain (exit %d): %s", code, out)
		}
	}
	return nil
}

// execAndRead runs a command in the container and returns its exit code and
// combined output, draining and closing the reader synchronously.
func execAndRead(ctx context.Context, c testcontainers.Container, cmd []string) (int, string, error) {
	code, reader, err := c.Exec(ctx, cmd)
	if err != nil {
		return code, "", err
	}
	out, readErr := io.ReadAll(reader)
	if closer, ok := reader.(io.Closer); ok {
		closer.Close()
	}
	if readErr != nil {
		return code, "", fmt.Errorf("read command output: %w", readErr)
	}
	return code, string(out), nil
}
