//-------------------------------------------------------------------------
//
// pgEdge PostgreSQL MCP - Knowledgebase Builder
//
// Portions copyright (c) 2025, pgEdge, Inc.
// This software is released under The PostgreSQL License
//
//-------------------------------------------------------------------------

package kbsource

import (
	"os"
	"path/filepath"
	"testing"

	"pgedge-postgres-mcp/internal/kbconfig"
)

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple name",
			input:    "PostgreSQL",
			expected: "postgresql",
		},
		{
			name:     "with spaces",
			input:    "PostgreSQL 17",
			expected: "postgresql-17",
		},
		{
			name:     "with slashes",
			input:    "path/to/docs",
			expected: "path-to-docs",
		},
		{
			name:     "with backslashes",
			input:    "path\\to\\docs",
			expected: "path-to-docs",
		},
		{
			name:     "with special characters",
			input:    "Test@#$%Project!",
			expected: "testproject",
		},
		{
			name:     "with underscores and dots",
			input:    "test_project.v1.0",
			expected: "test_project.v1.0",
		},
		{
			name:     "with numbers",
			input:    "Version 123",
			expected: "version-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeName(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeName(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFetchLocalSource(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test directory structure
	docsDir := filepath.Join(tmpDir, "docs")
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	tests := []struct {
		name        string
		source      kbconfig.DocumentSource
		shouldError bool
	}{
		{
			name: "valid local path",
			source: kbconfig.DocumentSource{
				LocalPath:      tmpDir,
				ProjectName:    "Test",
				ProjectVersion: "1.0",
			},
			shouldError: false,
		},
		{
			name: "valid with doc path",
			source: kbconfig.DocumentSource{
				LocalPath:      tmpDir,
				DocPath:        "docs",
				ProjectName:    "Test",
				ProjectVersion: "1.0",
			},
			shouldError: false,
		},
		{
			name: "non-existent path",
			source: kbconfig.DocumentSource{
				LocalPath:      "/nonexistent/path",
				ProjectName:    "Test",
				ProjectVersion: "1.0",
			},
			shouldError: true,
		},
		{
			name: "non-existent doc path",
			source: kbconfig.DocumentSource{
				LocalPath:      tmpDir,
				DocPath:        "nonexistent",
				ProjectName:    "Test",
				ProjectVersion: "1.0",
			},
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := fetchLocalSource(tt.source, tmpDir)

			if tt.shouldError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
				if info.BasePath == "" {
					t.Error("BasePath should not be empty")
				}
				if info.Source.ProjectName != tt.source.ProjectName {
					t.Error("Source info mismatch")
				}
			}
		})
	}
}

func TestFetchLocalSource_WithTilde(t *testing.T) {
	// Test tilde expansion
	source := kbconfig.DocumentSource{
		LocalPath:      "~/test",
		ProjectName:    "Test",
		ProjectVersion: "1.0",
	}

	// This will fail because ~/test doesn't exist, but we can check if tilde was expanded
	_, err := fetchLocalSource(source, t.TempDir())
	if err == nil {
		t.Error("Expected error for non-existent path")
	}

	// Check that the error message doesn't contain tilde (meaning it was expanded)
	if err != nil && err.Error() == "documentation path does not exist: ~/test" {
		t.Error("Tilde was not expanded in error message")
	}
}

func TestFetchLocalSource_FileNotDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file instead of directory
	filePath := filepath.Join(tmpDir, "testfile")
	if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	source := kbconfig.DocumentSource{
		LocalPath:      filePath,
		ProjectName:    "Test",
		ProjectVersion: "1.0",
	}

	_, err := fetchLocalSource(source, tmpDir)
	if err == nil {
		t.Error("Expected error when path is a file, not directory")
	}
}

func TestFetchSource_LocalVsGit(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test directory
	docsDir := filepath.Join(tmpDir, "docs")
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Test local source
	localSource := kbconfig.DocumentSource{
		LocalPath:      tmpDir,
		ProjectName:    "Test",
		ProjectVersion: "1.0",
	}

	clonedRepos := make(map[string]string)
	info, err := fetchSource(localSource, tmpDir, false, clonedRepos)
	if err != nil {
		t.Errorf("Expected no error for local source, got: %v", err)
	}
	if info.BasePath == "" {
		t.Error("BasePath should not be empty for local source")
	}

	// Test git source (should fail without actual git repo)
	gitSource := kbconfig.DocumentSource{
		GitURL:         "https://github.com/test/test",
		ProjectName:    "Test",
		ProjectVersion: "1.0",
	}

	_, err = fetchSource(gitSource, tmpDir, false, clonedRepos)
	// Git operations will likely fail in test environment, which is expected
	if err == nil {
		// If it doesn't fail, that's okay too (might have network access)
		// Just verify we took the git path
	}
}

func TestFetchAll_EmptyConfig(t *testing.T) {
	tmpDir := t.TempDir()

	config := &kbconfig.Config{
		DocSourcePath: tmpDir,
		Sources:       []kbconfig.DocumentSource{},
	}

	sources, err := FetchAll(config, false)
	if err != nil {
		t.Errorf("FetchAll should not error with empty sources: %v", err)
	}
	if len(sources) != 0 {
		t.Errorf("Expected 0 sources, got %d", len(sources))
	}
}

func TestFetchAll_ValidSource(t *testing.T) {
	tmpDir := t.TempDir()
	docsDir := filepath.Join(tmpDir, "docs")
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	config := &kbconfig.Config{
		DocSourcePath: tmpDir,
		Sources: []kbconfig.DocumentSource{
			{
				LocalPath:      docsDir,
				ProjectName:    "Test",
				ProjectVersion: "1.0",
			},
		},
	}

	sources, err := FetchAll(config, false)
	if err != nil {
		t.Errorf("FetchAll failed: %v", err)
	}
	if len(sources) != 1 {
		t.Errorf("Expected 1 source, got %d", len(sources))
	}
	if len(sources) > 0 && sources[0].Source.ProjectName != "Test" {
		t.Error("Source info mismatch")
	}
}

func TestFetchAll_InvalidSource(t *testing.T) {
	tmpDir := t.TempDir()

	config := &kbconfig.Config{
		DocSourcePath: tmpDir,
		Sources: []kbconfig.DocumentSource{
			{
				LocalPath:      "/nonexistent/path",
				ProjectName:    "Test",
				ProjectVersion: "1.0",
			},
		},
	}

	_, err := FetchAll(config, false)
	if err == nil {
		t.Error("Expected error for invalid source")
	}
}
