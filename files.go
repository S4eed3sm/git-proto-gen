package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func replaceWithRegex(input []byte) []byte {
	re := regexp.MustCompile(`out:.*$`)

	// Replace all matches with "out: __events__"
	return re.ReplaceAll(input, []byte("out: __events__"))
}

func checkBufOptionalConfigs(dir string) {
	t, exist := getFileIfExists(filepath.Join(dir, bufYamlFileName))
	if exist {
		logger.Debug("Using local buf.yaml file")
		bufYamlContent = replaceWithRegex(t)
	}

	t, exist = getFileIfExists(filepath.Join(dir, bufGenGoYamlFileName))
	if exist {
		logger.Debug("Using local buf.gen.go.yaml file")
		bufGenGoYamlContent = replaceWithRegex(t)
	}

	t, exist = getFileIfExists(filepath.Join(dir, bufGenJsYamlFileName))
	if exist {
		logger.Debug("Using local buf.gen.js.yaml file")
		bufGenJsYamlContent = replaceWithRegex(t)
	}
}

func getFileIfExists(fileName string) ([]byte, bool) {
	_, err := os.Stat(fileName)
	if os.IsNotExist(err) {
		return nil, false
	}

	content, err := os.ReadFile(fileName)
	if err != nil {
		return nil, false
	}

	return content, true
}

func createBufConfigs(tempDir, outputPath string) error {
	if err := os.WriteFile(filepath.Join(tempDir, bufYamlFileName), bufYamlContent, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", bufYamlFileName, err)
	}

	if err := os.WriteFile(filepath.Join(tempDir, bufGenGoYamlFileName), bytes.ReplaceAll(bufGenGoYamlContent, []byte("__events__"), []byte(outputPath)), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", bufGenGoYamlFileName, err)
	}

	if err := os.WriteFile(filepath.Join(tempDir, bufGenJsYamlFileName), bytes.ReplaceAll(bufGenJsYamlContent, []byte("__events__"), []byte(outputPath)), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", bufGenJsYamlFileName, err)
	}

	return nil
}

// copyFile copies a single file from a source path to a destination path.
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file '%s': %w", src, err)
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file '%s': %w", dst, err)
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return fmt.Errorf("failed to copy content from '%s' to '%s': %w", src, dst, err)
	}

	return nil
}

// copyLocalProtoToTemp recursively copies .proto files from srcDir to dstDir
func copyLocalProtoToTemp(srcDir, dstDir string) error {
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path for '%s' from '%s': %w", path, srcDir, err)
		}

		targetPath := filepath.Join(dstDir, relPath)
		if info.IsDir() {
			if err := os.MkdirAll(targetPath, info.Mode()); err != nil {
				return fmt.Errorf("failed to create directory '%s': %w", targetPath, err)
			}
			return nil
		}

		if strings.HasSuffix(info.Name(), ".proto") {
			if err := copyFile(path, targetPath); err != nil {
				return fmt.Errorf("failed to copy proto file '%s' to '%s': %w", path, targetPath, err)
			}
		}

		return nil
	})
}

// copyGeneratedFiles recursively copies all files from srcDir to dstDir
func copyGeneratedFiles(srcDir, dstDir string) error {
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path for '%s' from '%s': %w", path, srcDir, err)
		}

		targetPath := filepath.Join(dstDir, relPath)
		if info.IsDir() {
			if err := os.MkdirAll(targetPath, info.Mode()); err != nil {
				return fmt.Errorf("failed to create directory '%s': %w", targetPath, err)
			}
			return nil
		}

		if err := copyFile(path, targetPath); err != nil {
			return fmt.Errorf("failed to copy file '%s' to '%s': %w", path, targetPath, err)
		}

		return nil
	})
}

func prepareTempFilesAndDirs(ctx context.Context, config *Config) (string, string, string, error) {
	absOutputPath, err := filepath.Abs("")
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get absolute path for output directory: %w", err)
	}

	if err := os.MkdirAll(absOutputPath, 0755); err != nil {
		return "", "", "", fmt.Errorf("failed to create output directory '%s': %w", absOutputPath, err)
	}

	tempWorkspace, err := os.MkdirTemp("", "bufSourceWorkspace")
	if err != nil {
		return "", "", "", fmt.Errorf("failed to create temporary source workspace directory: %w", err)
	}

	hostProtoSubDir := filepath.Join(tempWorkspace, "proto")
	if err := os.MkdirAll(hostProtoSubDir, 0755); err != nil {
		return "", "", "", fmt.Errorf("failed to create 'proto' subdirectory '%s' in temporary source workspace: %w", hostProtoSubDir, err)
	}

	tempGeneratedOutputDir, err := os.MkdirTemp("", "bufGeneratedOutput")
	if err != nil {
		return "", "", "", fmt.Errorf("failed to create temporary generated output directory: %w", err)
	}

	if err := createBufConfigs(tempWorkspace, config.OutputPath); err != nil {
		return "", "", "", fmt.Errorf("failed to create minimal buf config files: %w", err)
	}

	if config.LocalPath != "" {
		absLocalPath, err := filepath.Abs(config.LocalPath)
		if err != nil {
			return "", "", "", fmt.Errorf("failed to get absolute path for local proto path '%s': %w", config.LocalPath, err)
		}

		if err := copyLocalProtoToTemp(absLocalPath, hostProtoSubDir); err != nil {
			return "", "", "", fmt.Errorf("failed to copy local proto files from '%s' to temporary source workspace: %w", absLocalPath, err)
		}
	}

	if len(config.PrivateRepos) > 0 {
		switch config.GithubAuthMethod {
		case GithubAuthMethodToken:
			for _, p := range config.PrivateRepos {
				if err := downloadPrivateRemoteProtoToTemp(ctx, config.GithubToken, p, hostProtoSubDir); err != nil {
					logger.Error("failed to download private-repo", "proto", p, "error", err)
				}
			}
		case GithubAuthMethodSSH:
			for _, p := range config.PrivateRepos {
				if err := downloadPrivateRemoteProtoToTempWithSSH(ctx, p, hostProtoSubDir); err != nil {
					logger.Error("failed to download private-repo", "proto", p, "error", err)
				}
			}
		}
		logger.Info("successfully downloaded all private repos")
	}

	if len(config.PublicRepos) > 0 {
		for _, p := range config.PublicRepos {
			if err := downloadPublicRemoteProtoToTemp(ctx, p, hostProtoSubDir); err != nil {
				logger.Error("failed to download public-repo", "proto", p, "error", err)
			}
		}
		logger.Info("successfully downloaded all public repos")
	}

	return tempGeneratedOutputDir, tempWorkspace, absOutputPath, nil
}
