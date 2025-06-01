package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/docker/docker/api/types/container"
	_ "github.com/docker/go-connections/nat" // Imported for dependency resolution, but not directly used in this snippet
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

var logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
	Level: slog.LevelInfo,
}))

func main() {
	checkOptionalYamlFiles()
	config, err := parseArgs()
	if err != nil {
		panic(err)
	}

	if err := run(context.Background(), &config); err != nil {
		panic(err)
	}
}

func run(ctx context.Context, config *Config) error {
	tempGeneratedOutputDir, tempWorkspace, absOutputPath, err := prepareTempFilesAndDirs(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to prepare temporary files and directories: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(tempWorkspace); err != nil {
			logger.Debug("Warning: failed to remove temporary source workspace '%s': %v", tempWorkspace, err)
		}
	}()

	for _, lang := range config.Languages {
		templateFile := bufGenGoYamlFileName
		if lang == "js" {
			templateFile = bufGenJsYamlFileName
		}

		bufCmd := []string{
			"buf", "generate", ".",
			"--template", filepath.Join("/workspace", templateFile),
			"--output", "/workspace/temp_generated_output",
		}

		containerReq := testcontainers.ContainerRequest{
			Image:      "bufbuild/buf:1.54.0",
			WorkingDir: "/workspace",
			Entrypoint: []string{"sh"},
			Cmd:        []string{"-c", "tail -f /dev/null"}, // Keep container running
			WaitingFor: wait.ForExec([]string{"echo", "ready"}).
				WithStartupTimeout(120 * time.Second).
				WithPollInterval(5 * time.Second),
			HostConfigModifier: func(hostConfig *container.HostConfig) {
				hostConfig.Binds = []string{
					fmt.Sprintf("%s:%s", tempWorkspace, "/workspace"),
					fmt.Sprintf("%s:%s", tempGeneratedOutputDir, "/workspace/temp_generated_output"),
				}
				hostConfig.Memory = 2 * 1024 * 1024 * 1024
				hostConfig.MemorySwap = 2 * 1024 * 1024 * 1024
			},
		}

		container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: containerReq,
			Started:          true, // Start the container immediately
		})
		if err != nil {
			return fmt.Errorf("failed to start container: %w", err)
		}
		defer func() {
			if err := container.Terminate(ctx); err != nil {
				slog.Info("Warning: failed to terminate container '%s': %v", container.GetContainerID(), err)
			}
		}()

		if lang == "js" {
			installDepsCmd := []string{"apk", "add", "--no-cache", "nodejs", "npm", "python3", "make", "g++"}
			exitCode, depsReader, _ := container.Exec(ctx, installDepsCmd)
			depsOutput, _ := io.ReadAll(depsReader)
			if exitCode != 0 {
				return fmt.Errorf("failed to install dependencies, exit code: %d, output: %s", exitCode, string(depsOutput))
			}
			if closer, ok := depsReader.(io.ReadCloser); ok {
				defer closer.Close()
			}

			installNpmLocal := []string{"sh", "-c", "npm install --save-dev --verbose @bufbuild/protobuf @bufbuild/protoc-gen-es @bufbuild/buf 2>&1"}
			exitCode, localReader, _ := container.Exec(ctx, installNpmLocal)
			npmOutput, _ := io.ReadAll(localReader)
			if closer, ok := localReader.(io.ReadCloser); ok {
				defer closer.Close()
			}

			if exitCode != 0 {
				return fmt.Errorf("failed to install npm packages (local), output: %s", string(npmOutput))
			}
			bufCmd = []string{"sh", "-c", "export PATH=./node_modules/.bin:$PATH && buf generate . --template /workspace/" + templateFile + " --output /workspace/temp_generated_output"}
		}

		exitCode, reader, err := container.Exec(ctx, bufCmd)
		if err != nil {
			return fmt.Errorf("failed to execute buf generate command in container: %w", err)
		}

		if closer, ok := reader.(io.ReadCloser); ok {
			defer closer.Close()
		}

		bufOutput, err := io.ReadAll(reader)
		if err != nil {
			return fmt.Errorf("failed to read buf command output: %w", err)
		}

		if exitCode != 0 {
			return fmt.Errorf("buf generate command exited with non-zero status: %d. Check buf command output for details. Output: %s", exitCode, string(bufOutput))
		}

		if err := copyGeneratedFiles(tempGeneratedOutputDir, absOutputPath); err != nil {
			return fmt.Errorf("failed to copy generated files from temporary directory to final output path: %w", err)
		}

		slog.Info("Generated files successfully copied to final output directory.")
	}

	return nil
}
