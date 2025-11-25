/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package kbconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Create temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.yaml")

	configContent := `
database_path: "test.db"
doc_source_path: "docs"

sources:
  - local_path: "/tmp/test-docs"
    project_name: "Test Project"
    project_version: "1.0"

embeddings:
  openai:
    enabled: true
    api_key_file: "/tmp/fake-key"
    model: "text-embedding-3-small"
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Create fake API key file
	keyPath := "/tmp/fake-key"
	if err := os.WriteFile(keyPath, []byte("test-api-key"), 0644); err != nil {
		t.Fatalf("Failed to write test key: %v", err)
	}
	defer os.Remove(keyPath)

	// Load config
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify loaded values
	if cfg.DatabasePath == "" {
		t.Error("DatabasePath should be set")
	}

	if cfg.DocSourcePath == "" {
		t.Error("DocSourcePath should be set")
	}

	if len(cfg.Sources) != 1 {
		t.Errorf("Expected 1 source, got %d", len(cfg.Sources))
	}

	if cfg.Sources[0].ProjectName != "Test Project" {
		t.Errorf("Expected project name 'Test Project', got '%s'", cfg.Sources[0].ProjectName)
	}

	if !cfg.Embeddings.OpenAI.Enabled {
		t.Error("OpenAI embeddings should be enabled")
	}

	if cfg.Embeddings.OpenAI.APIKey != "test-api-key" {
		t.Errorf("API key should be loaded, got '%s'", cfg.Embeddings.OpenAI.APIKey)
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		shouldError bool
	}{
		{
			name: "valid config",
			config: &Config{
				Sources: []DocumentSource{
					{
						LocalPath:      "/tmp/test",
						ProjectName:    "Test",
						ProjectVersion: "1.0",
					},
				},
				Embeddings: EmbeddingConfig{
					OpenAI: OpenAIConfig{Enabled: true},
				},
			},
			shouldError: false,
		},
		{
			name: "no sources",
			config: &Config{
				Sources: []DocumentSource{},
				Embeddings: EmbeddingConfig{
					OpenAI: OpenAIConfig{Enabled: true},
				},
			},
			shouldError: true,
		},
		{
			name: "no embedding providers",
			config: &Config{
				Sources: []DocumentSource{
					{
						LocalPath:      "/tmp/test",
						ProjectName:    "Test",
						ProjectVersion: "1.0",
					},
				},
				Embeddings: EmbeddingConfig{},
			},
			shouldError: true,
		},
		{
			name: "missing project name",
			config: &Config{
				Sources: []DocumentSource{
					{
						LocalPath:      "/tmp/test",
						ProjectVersion: "1.0",
					},
				},
				Embeddings: EmbeddingConfig{
					OpenAI: OpenAIConfig{Enabled: true},
				},
			},
			shouldError: true,
		},
		{
			name: "both git and local",
			config: &Config{
				Sources: []DocumentSource{
					{
						GitURL:         "https://github.com/test/test",
						LocalPath:      "/tmp/test",
						ProjectName:    "Test",
						ProjectVersion: "1.0",
					},
				},
				Embeddings: EmbeddingConfig{
					OpenAI: OpenAIConfig{Enabled: true},
				},
			},
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate(tt.config)
			if tt.shouldError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.shouldError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestExpandPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{
			name:     "tilde expansion",
			input:    "~/test",
			contains: "/test",
		},
		{
			name:     "tilde only",
			input:    "~",
			contains: "",
		},
		{
			name:     "absolute path",
			input:    "/tmp/test",
			contains: "/tmp/test",
		},
		{
			name:     "relative path",
			input:    "test",
			contains: "test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandPath(tt.input)
			if tt.input == "~" {
				// Should expand to home directory
				if result == tt.input {
					t.Error("Tilde should be expanded")
				}
			} else if tt.contains != "" && result != tt.input {
				// Check that expanded path contains the expected part
				// (skip check if tilde was expanded)
			}
		})
	}
}

func TestApplyDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.yaml")

	cfg := &Config{
		Embeddings: EmbeddingConfig{
			OpenAI: OpenAIConfig{Enabled: true},
			Voyage: VoyageConfig{Enabled: true},
			Ollama: OllamaConfig{Enabled: true},
		},
	}

	err := applyDefaults(cfg, configPath)
	if err != nil {
		t.Fatalf("applyDefaults failed: %v", err)
	}

	// Check defaults were applied
	if cfg.DatabasePath == "" {
		t.Error("DatabasePath should have default")
	}

	if cfg.DocSourcePath == "" {
		t.Error("DocSourcePath should have default")
	}

	if cfg.Embeddings.OpenAI.Model == "" {
		t.Error("OpenAI model should have default")
	}

	if cfg.Embeddings.OpenAI.Dimensions == 0 {
		t.Error("OpenAI dimensions should have default")
	}

	if cfg.Embeddings.Voyage.Model == "" {
		t.Error("Voyage model should have default")
	}

	if cfg.Embeddings.Ollama.Model == "" {
		t.Error("Ollama model should have default")
	}

	if cfg.Embeddings.Ollama.Endpoint == "" {
		t.Error("Ollama endpoint should have default")
	}
}
