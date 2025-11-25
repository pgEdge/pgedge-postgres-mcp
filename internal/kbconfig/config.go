//-------------------------------------------------------------------------
//
// pgEdge PostgreSQL MCP - Knowledgebase Builder
//
// Portions copyright (c) 2025, pgEdge, Inc.
// This software is released under The PostgreSQL License
//
//-------------------------------------------------------------------------

package kbconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the kb-builder configuration
type Config struct {
	// Output database path
	DatabasePath string `yaml:"database_path"`

	// Directory for storing downloaded/processed documentation
	DocSourcePath string `yaml:"doc_source_path"`

	// Documentation sources
	Sources []DocumentSource `yaml:"sources"`

	// Embedding provider configurations
	Embeddings EmbeddingConfig `yaml:"embeddings"`
}

// DocumentSource represents a source of documentation
type DocumentSource struct {
	// For Git repositories
	GitURL string `yaml:"git_url,omitempty"`
	Branch string `yaml:"branch,omitempty"`
	Tag    string `yaml:"tag,omitempty"`

	// For local paths
	LocalPath string `yaml:"local_path,omitempty"`

	// Common fields
	DocPath        string `yaml:"doc_path"`        // Path within project containing docs
	ProjectName    string `yaml:"project_name"`    // User-defined project name
	ProjectVersion string `yaml:"project_version"` // User-defined version
}

// EmbeddingConfig contains configuration for all embedding providers
type EmbeddingConfig struct {
	OpenAI OpenAIConfig `yaml:"openai"`
	Voyage VoyageConfig `yaml:"voyage"`
	Ollama OllamaConfig `yaml:"ollama"`
}

// OpenAIConfig contains OpenAI embedding configuration
type OpenAIConfig struct {
	Enabled    bool   `yaml:"enabled"`
	APIKeyFile string `yaml:"api_key_file"`
	APIKey     string // Loaded at runtime, not from YAML
	Model      string `yaml:"model"`      // e.g., "text-embedding-3-small"
	Dimensions int    `yaml:"dimensions"` // Optional, model-specific
}

// VoyageConfig contains Voyage AI embedding configuration
type VoyageConfig struct {
	Enabled    bool   `yaml:"enabled"`
	APIKeyFile string `yaml:"api_key_file"`
	APIKey     string // Loaded at runtime, not from YAML
	Model      string `yaml:"model"` // e.g., "voyage-3"
}

// OllamaConfig contains Ollama embedding configuration
type OllamaConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Endpoint string `yaml:"endpoint"` // e.g., "http://localhost:11434"
	Model    string `yaml:"model"`    // e.g., "nomic-embed-text"
}

// Load reads and parses the configuration file
func Load(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply defaults
	if err := applyDefaults(&config, configPath); err != nil {
		return nil, err
	}

	// Validate configuration
	if err := validate(&config); err != nil {
		return nil, err
	}

	// Load API keys
	if err := loadAPIKeys(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

// applyDefaults sets default values for unspecified config fields
func applyDefaults(config *Config, configPath string) error {
	configDir := filepath.Dir(configPath)

	// Default database path
	if config.DatabasePath == "" {
		config.DatabasePath = filepath.Join(configDir, "pgedge-mcp-kb.db")
	}

	// Default doc source path
	if config.DocSourcePath == "" {
		config.DocSourcePath = filepath.Join(configDir, "doc-source")
	}

	// Default OpenAI settings
	if config.Embeddings.OpenAI.Enabled {
		if config.Embeddings.OpenAI.APIKeyFile == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get home directory: %w", err)
			}
			config.Embeddings.OpenAI.APIKeyFile = filepath.Join(home, ".openai-api-key")
		}
		if config.Embeddings.OpenAI.Model == "" {
			config.Embeddings.OpenAI.Model = "text-embedding-3-small"
		}
		if config.Embeddings.OpenAI.Dimensions == 0 {
			config.Embeddings.OpenAI.Dimensions = 1536
		}
	}

	// Default Voyage settings
	if config.Embeddings.Voyage.Enabled {
		if config.Embeddings.Voyage.APIKeyFile == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get home directory: %w", err)
			}
			config.Embeddings.Voyage.APIKeyFile = filepath.Join(home, ".voyage-api-key")
		}
		if config.Embeddings.Voyage.Model == "" {
			config.Embeddings.Voyage.Model = "voyage-3"
		}
	}

	// Default Ollama settings
	if config.Embeddings.Ollama.Enabled {
		if config.Embeddings.Ollama.Endpoint == "" {
			config.Embeddings.Ollama.Endpoint = "http://localhost:11434"
		}
		if config.Embeddings.Ollama.Model == "" {
			config.Embeddings.Ollama.Model = "nomic-embed-text"
		}
	}

	// Expand paths with ~
	config.DatabasePath = expandPath(config.DatabasePath)
	config.DocSourcePath = expandPath(config.DocSourcePath)
	if config.Embeddings.OpenAI.APIKeyFile != "" {
		config.Embeddings.OpenAI.APIKeyFile = expandPath(config.Embeddings.OpenAI.APIKeyFile)
	}
	if config.Embeddings.Voyage.APIKeyFile != "" {
		config.Embeddings.Voyage.APIKeyFile = expandPath(config.Embeddings.Voyage.APIKeyFile)
	}

	return nil
}

// validate checks that the configuration is valid
func validate(config *Config) error {
	if len(config.Sources) == 0 {
		return fmt.Errorf("no documentation sources configured")
	}

	for i, source := range config.Sources {
		// Check that either Git or local path is specified
		hasGit := source.GitURL != ""
		hasLocal := source.LocalPath != ""

		if !hasGit && !hasLocal {
			return fmt.Errorf("source %d: must specify either git_url or local_path", i)
		}
		if hasGit && hasLocal {
			return fmt.Errorf("source %d: cannot specify both git_url and local_path", i)
		}

		// Check required fields
		if source.ProjectName == "" {
			return fmt.Errorf("source %d: project_name is required", i)
		}
		if source.ProjectVersion == "" {
			return fmt.Errorf("source %d: project_version is required", i)
		}
	}

	// Check that at least one embedding provider is enabled
	if !config.Embeddings.OpenAI.Enabled &&
		!config.Embeddings.Voyage.Enabled &&
		!config.Embeddings.Ollama.Enabled {
		return fmt.Errorf("at least one embedding provider must be enabled")
	}

	return nil
}

// loadAPIKeys reads API keys from files
func loadAPIKeys(config *Config) error {
	if config.Embeddings.OpenAI.Enabled {
		key, err := readAPIKey(config.Embeddings.OpenAI.APIKeyFile)
		if err != nil {
			return fmt.Errorf("OpenAI API key: %w", err)
		}
		config.Embeddings.OpenAI.APIKey = key
	}

	if config.Embeddings.Voyage.Enabled {
		key, err := readAPIKey(config.Embeddings.Voyage.APIKeyFile)
		if err != nil {
			return fmt.Errorf("Voyage API key: %w", err)
		}
		config.Embeddings.Voyage.APIKey = key
	}

	return nil
}

// readAPIKey reads an API key from a file
func readAPIKey(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read API key file %s: %w", path, err)
	}

	key := strings.TrimSpace(string(data))
	if key == "" {
		return "", fmt.Errorf("API key file %s is empty", path)
	}

	return key, nil
}

// expandPath expands ~ to home directory
func expandPath(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}

	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}

	return path
}
