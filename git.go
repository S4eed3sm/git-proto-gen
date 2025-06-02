package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/google/go-github/v72/github"
	"golang.org/x/oauth2"
)

// checkSSHKeys checks if SSH keys are available in the user's home directory
func checkSSHKeys() bool {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	sshDir := filepath.Join(homeDir, ".ssh")
	if _, err := os.Stat(sshDir); os.IsNotExist(err) {
		return false
	}

	// Check for common SSH key files
	keyFiles := []string{"id_rsa", "id_ed25519", "id_ecdsa"}
	for _, key := range keyFiles {
		if _, err := os.Stat(filepath.Join(sshDir, key)); err == nil {
			return true
		}
	}

	return false
}

// downloadPrivateRemoteProtoToTemp parses the private-repo GitHub path and downloads .proto files
// into the specified destination directory.
func downloadPrivateRemoteProtoToTemp(ctx context.Context, githubToken, remotePath, dstDir string) error {
	if githubToken == "" {
		return fmt.Errorf("gitHub token is required for private-repo access.")
	}
	logger.Info("downloading private-repo proto files using GitHub API", "remotePath", remotePath)
	parts := strings.SplitN(remotePath, "/", 4)
	if len(parts) < 4 || parts[0] != "github.com" {
		return fmt.Errorf("invalid repo path format: '%s'.", remotePath)
	}

	owner := parts[1]      // e.g: S4ee3sm
	repo := parts[2]       // e.g: public-test-repo
	pathInRepo := parts[3] // e.g:  proto or proto/file.proto

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: githubToken},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	return fetchAndSaveGitHubContents(ctx, client, owner, repo, pathInRepo, dstDir)
}

func downloadPrivateRemoteProtoToTempWithSSH(ctx context.Context, remotePath, dstDir string) error {
	logger.Info("downloading private-repo proto files using SSH", "remotePath", remotePath)
	parts := strings.SplitN(remotePath, "/", 4)
	if len(parts) < 4 || parts[0] != "github.com" {
		return fmt.Errorf("invalid repo path format: '%s'.", remotePath)
	}

	owner := parts[1]      // e.g: S4ee3sm
	repo := parts[2]       // e.g: public-test-repo
	pathInRepo := parts[3] // e.g:  proto or proto/file.proto

	sshURL := fmt.Sprintf("git@github.com:%s/%s.git", owner, repo)
	tempRepoDir := filepath.Join(dstDir, repo)
	if err := os.MkdirAll(tempRepoDir, 0755); err != nil {
		return fmt.Errorf("failed to create temporary directory for repository: %w", err)
	}

	cmd := exec.CommandContext(ctx, "git", "clone", sshURL, tempRepoDir)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to clone repository using SSH: %w", err)
	}

	sourcePath := filepath.Join(tempRepoDir, pathInRepo)
	if err := copyLocalProtoToTemp(sourcePath, dstDir); err != nil {
		return fmt.Errorf("failed to copy proto files from cloned repository: %w", err)
	}

	if err := os.RemoveAll(tempRepoDir); err != nil {
		return fmt.Errorf("failed to clean up temporary repository: %w", err)
	}

	return nil
}

// downloadPublicRemoteProtoToTemp parses the public-repo GitHub path and downloads .proto files
// into the specified destination directory.
func downloadPublicRemoteProtoToTemp(ctx context.Context, remotePath, dstDir string) error {
	parts := strings.SplitN(remotePath, "/", 4)
	if len(parts) < 4 || parts[0] != "github.com" {
		return fmt.Errorf("invalid repo path format: '%s'.", remotePath)
	}

	owner := parts[1]      // e.g: S4ee3sm
	repo := parts[2]       // e.g: public-test-repo
	pathInRepo := parts[3] // e.g:  proto or proto/file.proto

	client := github.NewClient(nil)

	return fetchAndSaveGitHubContents(ctx, client, owner, repo, pathInRepo, dstDir)
}

// fetchAndSaveGitHubContents fetches files (specifically .proto files) or directories
// from a GitHub repository and saves them to the specified host destination directory.
func fetchAndSaveGitHubContents(ctx context.Context, client *github.Client, owner, repo, githubPath, hostDestDir string) error {
	fileContent, directoryContents, resp, err := client.Repositories.GetContents(ctx, owner, repo, githubPath, nil)
	if err != nil {
		if resp != nil && resp.StatusCode == 404 {
			return fmt.Errorf("path '%s' not found within repository '%s/%s'. Check path spelling or ensure it exists", githubPath, owner, repo)
		}

		return fmt.Errorf("failed to get contents for path '%s' in repository '%s/%s': %w", githubPath, owner, repo, err)
	}

	if directoryContents == nil {
		if fileContent.GetType() == "file" && strings.HasSuffix(fileContent.GetName(), ".proto") {
			content, err := fileContent.GetContent()
			if err != nil {
				return fmt.Errorf("failed to decode content for file '%s': %w", fileContent.GetPath(), err)
			}

			filePath := filepath.Join(hostDestDir, fileContent.GetName())
			if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
				return fmt.Errorf("failed to write file '%s': %w", filePath, err)
			}
			return nil
		}

		return fmt.Errorf("private-repo path '%s' is not a .proto file or a directory containing .proto files", githubPath)
	}

	for _, item := range directoryContents {
		itemPath := item.GetPath() // Full path of the item within the GitHub repo.
		itemType := item.GetType() // Type of the item (e.g., "file", "dir").
		itemName := item.GetName() // Name of the item (e.g., "my_service.proto", "sub_dir").

		if itemType == "file" && strings.HasSuffix(itemName, ".proto") {
			singleFileContent, _, _, err := client.Repositories.GetContents(ctx, owner, repo, itemPath, nil)
			if err != nil {
				return fmt.Errorf("failed to get content for individual file '%s': %w", itemPath, err)
			}

			if singleFileContent.GetType() != "file" {
				return fmt.Errorf("expected '%s' to be a file, but GitHub API returned type '%s'", itemPath, singleFileContent.GetType())
			}

			content, err := singleFileContent.GetContent()
			if err != nil {
				return fmt.Errorf("failed to decode content for file '%s': %w", itemPath, err)
			}

			filePath := filepath.Join(hostDestDir, itemName)
			if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
				return fmt.Errorf("failed to write file '%s': %w", filePath, err)
			}
		} else if itemType == "dir" {
			subDirPath := filepath.Join(hostDestDir, itemName)
			if err := os.MkdirAll(subDirPath, 0755); err != nil {
				return fmt.Errorf("failed to create subdirectory '%s': %w", subDirPath, err)
			}

			if err := fetchAndSaveGitHubContents(ctx, client, owner, repo, itemPath, subDirPath); err != nil {
				return err
			}
		}
	}

	return nil
}
