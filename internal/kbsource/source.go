/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package kbsource

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"pgedge-postgres-mcp/internal/kbconfig"
)

// SourceInfo represents information about a processed documentation source
type SourceInfo struct {
	Source   kbconfig.DocumentSource
	BasePath string // Path to the documentation files
}

// FetchAll fetches all documentation sources and returns their info
func FetchAll(config *kbconfig.Config, skipUpdates bool) ([]SourceInfo, error) {
	// Ensure doc source directory exists
	if err := os.MkdirAll(config.DocSourcePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create doc source directory: %w", err)
	}

	var sources []SourceInfo
	// Track cloned repos to enable local cloning for subsequent versions
	clonedRepos := make(map[string]string) // git URL -> local repo path

	for i, source := range config.Sources {
		fmt.Printf("\n[%d/%d] Processing source: %s %s\n",
			i+1, len(config.Sources), source.ProjectName, source.ProjectVersion)

		info, err := fetchSource(source, config.DocSourcePath, skipUpdates, clonedRepos)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch source %s: %w", source.ProjectName, err)
		}

		sources = append(sources, info)
	}

	return sources, nil
}

// fetchSource fetches a single documentation source
func fetchSource(source kbconfig.DocumentSource, docSourcePath string, skipUpdates bool, clonedRepos map[string]string) (SourceInfo, error) {
	if source.GitURL != "" {
		return fetchGitSource(source, docSourcePath, skipUpdates, clonedRepos)
	}
	return fetchLocalSource(source, docSourcePath)
}

// fetchGitSource clones or updates a Git repository
func fetchGitSource(source kbconfig.DocumentSource, docSourcePath string, skipUpdates bool, clonedRepos map[string]string) (SourceInfo, error) {
	// Create a directory name from project name and version
	// If version is empty, just use the project name
	var dirName string
	if source.ProjectVersion != "" {
		dirName = fmt.Sprintf("%s-%s", sanitizeName(source.ProjectName), sanitizeName(source.ProjectVersion))
	} else {
		dirName = sanitizeName(source.ProjectName)
	}
	repoPath := filepath.Join(docSourcePath, dirName)

	// Check if already cloned
	if _, err := os.Stat(filepath.Join(repoPath, ".git")); err == nil {
		if skipUpdates {
			fmt.Printf("  Repository already exists, skipping updates (--skip-updates)\n")
		} else {
			fmt.Printf("  Repository already exists, fetching updates...\n")
			if err := gitFetch(repoPath); err != nil {
				return SourceInfo{}, fmt.Errorf("failed to fetch updates: %w", err)
			}
		}
	} else {
		// Check if we've already cloned this git URL for another version
		if firstClonePath, exists := clonedRepos[source.GitURL]; exists {
			fmt.Printf("  Cloning with reference to local copy (%s)...\n", firstClonePath)
			if err := gitCloneWithReference(source.GitURL, repoPath, firstClonePath); err != nil {
				return SourceInfo{}, fmt.Errorf("failed to clone with reference: %w", err)
			}
		} else {
			fmt.Printf("  Cloning repository from %s...\n", source.GitURL)
			if err := gitClone(source.GitURL, repoPath); err != nil {
				return SourceInfo{}, fmt.Errorf("failed to clone repository: %w", err)
			}
			// Record this as the first clone for this git URL
			clonedRepos[source.GitURL] = repoPath
		}
	}

	// Checkout specific branch or tag if specified
	if source.Branch != "" {
		fmt.Printf("  Checking out branch: %s\n", source.Branch)
		if !skipUpdates {
			// Use checkout -B to create/reset local branch to match remote
			// This handles both new clones and existing repos with behind branches
			if err := gitCheckoutBranch(repoPath, source.Branch); err != nil {
				return SourceInfo{}, fmt.Errorf("failed to checkout branch: %w", err)
			}
		} else {
			// Just checkout without updating
			if err := gitCheckout(repoPath, source.Branch); err != nil {
				return SourceInfo{}, fmt.Errorf("failed to checkout branch: %w", err)
			}
		}
	} else if source.Tag != "" {
		fmt.Printf("  Checking out tag: %s\n", source.Tag)
		if err := gitCheckout(repoPath, source.Tag); err != nil {
			return SourceInfo{}, fmt.Errorf("failed to checkout tag: %w", err)
		}
	}

	// Determine base path for documentation
	basePath := repoPath
	if source.DocPath != "" {
		basePath = filepath.Join(repoPath, source.DocPath)
	}

	// Verify the documentation path exists
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		return SourceInfo{}, fmt.Errorf("documentation path does not exist: %s", basePath)
	}

	fmt.Printf("  Documentation path: %s\n", basePath)

	return SourceInfo{
		Source:   source,
		BasePath: basePath,
	}, nil
}

// fetchLocalSource handles local documentation paths
func fetchLocalSource(source kbconfig.DocumentSource, docSourcePath string) (SourceInfo, error) {
	// Expand ~ in local path
	localPath := source.LocalPath
	if strings.HasPrefix(localPath, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return SourceInfo{}, fmt.Errorf("failed to get home directory: %w", err)
		}
		if localPath == "~" {
			localPath = home
		} else if strings.HasPrefix(localPath, "~/") {
			localPath = filepath.Join(home, localPath[2:])
		}
	}

	// Determine base path for documentation
	basePath := localPath
	if source.DocPath != "" {
		basePath = filepath.Join(localPath, source.DocPath)
	}

	// Verify the path exists
	info, err := os.Stat(basePath)
	if os.IsNotExist(err) {
		return SourceInfo{}, fmt.Errorf("documentation path does not exist: %s", basePath)
	}
	if err != nil {
		return SourceInfo{}, fmt.Errorf("failed to stat path: %w", err)
	}
	if !info.IsDir() {
		return SourceInfo{}, fmt.Errorf("documentation path is not a directory: %s", basePath)
	}

	fmt.Printf("  Using local path: %s\n", basePath)

	return SourceInfo{
		Source:   source,
		BasePath: basePath,
	}, nil
}

// gitClone clones a Git repository
func gitClone(url, path string) error {
	cmd := exec.Command("git", "clone", url, path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// gitCloneWithReference clones a Git repository using an existing local repo as a reference
// This saves bandwidth by reusing objects from the reference repo while still fetching all branches
func gitCloneWithReference(url, path, referencePath string) error {
	cmd := exec.Command("git", "clone", "--reference", referencePath, url, path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// gitFetch fetches updates from a Git repository (works with both branches and tags)
func gitFetch(path string) error {
	cmd := exec.Command("git", "fetch", "--all", "--tags")
	cmd.Dir = path
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// gitCheckout checks out a specific branch or tag
func gitCheckout(path, ref string) error {
	cmd := exec.Command("git", "checkout", ref)
	cmd.Dir = path
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// gitCheckoutBranch checks out a branch and resets it to match the remote.
// Uses "git checkout -B <branch> origin/<branch>" to create/reset the local
// branch to match the remote, handling cases where the local branch doesn't
// exist or is behind.
func gitCheckoutBranch(path, branch string) error {
	// Try checkout -B first (creates/resets local branch to match remote)
	cmd := exec.Command("git", "checkout", "-B", branch, "origin/"+branch)
	cmd.Dir = path
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		// Fallback to regular checkout if origin/<branch> doesn't exist
		// (e.g., if it's actually a tag misconfigured as a branch)
		cmd = exec.Command("git", "checkout", branch)
		cmd.Dir = path
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	return nil
}

// sanitizeName converts a name to a safe directory name
func sanitizeName(name string) string {
	// Replace spaces and special characters with hyphens
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, "\\", "-")
	name = strings.ToLower(name)

	// Remove any remaining unsafe characters
	safe := strings.Builder{}
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			safe.WriteRune(r)
		}
	}

	return safe.String()
}
