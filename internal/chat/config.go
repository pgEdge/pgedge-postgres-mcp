/*-------------------------------------------------------------------------
 *
 * Configuration loading for MCP Chat Client
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package chat

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds all configuration for the chat client
type Config struct {
	MCP MCPConfig `yaml:"mcp"`
	LLM LLMConfig `yaml:"llm"`
	UI  UIConfig  `yaml:"ui"`
}

// MCPConfig holds MCP server connection configuration
type MCPConfig struct {
	Mode       string `yaml:"mode"`        // stdio or http
	URL        string `yaml:"url"`         // HTTP URL (for http mode)
	ServerPath string `yaml:"server_path"` // Path to server binary (for stdio mode)
	Token      string `yaml:"token"`       // Authentication token (for http mode)
	TLS        bool   `yaml:"tls"`         // Use TLS/HTTPS
}

// LLMConfig holds LLM provider configuration
type LLMConfig struct {
	Provider   string `yaml:"provider"`    // anthropic or ollama
	Model      string `yaml:"model"`       // Model to use
	APIKey     string `yaml:"api_key"`     // API key (for Anthropic)
	OllamaURL  string `yaml:"ollama_url"`  // Ollama server URL
	MaxTokens  int    `yaml:"max_tokens"`  // Max tokens for response
	Temperature float64 `yaml:"temperature"` // Temperature for sampling
}

// UIConfig holds UI configuration
type UIConfig struct {
	NoColor bool `yaml:"no_color"` // Disable colored output
}

// LoadConfig loads configuration from file, environment variables, and defaults
func LoadConfig(configPath string) (*Config, error) {
	cfg := &Config{
		MCP: MCPConfig{
			Mode:       getEnvOrDefault("PGEDGE_MCP_MODE", "stdio"),
			URL:        os.Getenv("PGEDGE_MCP_URL"),
			ServerPath: getEnvOrDefault("PGEDGE_MCP_SERVER_PATH", "../../bin/pgedge-postgres-mcp"),
			Token:      "", // Will be loaded separately
			TLS:        false,
		},
		LLM: LLMConfig{
			Provider:    getEnvOrDefault("PGEDGE_LLM_PROVIDER", "anthropic"),
			Model:       getEnvOrDefault("PGEDGE_LLM_MODEL", "claude-sonnet-4-20250514"),
			APIKey:      os.Getenv("ANTHROPIC_API_KEY"),
			OllamaURL:   getEnvOrDefault("OLLAMA_BASE_URL", "http://localhost:11434"),
			MaxTokens:   4096,
			Temperature: 0.7,
		},
		UI: UIConfig{
			NoColor: os.Getenv("NO_COLOR") != "",
		},
	}

	// Load from config file if provided
	if configPath != "" {
		if err := loadConfigFile(configPath, cfg); err != nil {
			return nil, fmt.Errorf("failed to load config file: %w", err)
		}
	} else {
		// Try default locations
		defaultPaths := []string{
			".pgedge-mcp-chat.yaml",
			filepath.Join(os.Getenv("HOME"), ".pgedge-mcp-chat.yaml"),
			"/etc/pgedge-mcp/chat.yaml",
		}
		for _, path := range defaultPaths {
			if _, err := os.Stat(path); err == nil {
				if err := loadConfigFile(path, cfg); err == nil {
					break
				}
			}
		}
	}

	// Load authentication token with priority
	cfg.MCP.Token = loadAuthToken()

	return cfg, nil
}

// loadConfigFile loads configuration from a YAML file
func loadConfigFile(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	return yaml.Unmarshal(data, cfg)
}

// loadAuthToken loads the authentication token with priority:
// 1. Environment variable PGEDGE_POSTGRES_MCP_SERVER_TOKEN
// 2. File ~/.pgedge-postgres-mcp-server-token
// 3. Returns empty string if not found (will prompt if needed)
func loadAuthToken() string {
	// Priority 1: Environment variable
	if token := os.Getenv("PGEDGE_POSTGRES_MCP_SERVER_TOKEN"); token != "" {
		return token
	}

	// Priority 2: Token file
	tokenPath := filepath.Join(os.Getenv("HOME"), ".pgedge-postgres-mcp-server-token")
	if data, err := os.ReadFile(tokenPath); err == nil {
		// Trim whitespace and newlines
		return string(data)
	}

	return ""
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate MCP mode
	if c.MCP.Mode != "stdio" && c.MCP.Mode != "http" {
		return fmt.Errorf("invalid mcp-mode: %s (must be stdio or http)", c.MCP.Mode)
	}

	// Validate MCP configuration based on mode
	if c.MCP.Mode == "http" {
		if c.MCP.URL == "" {
			return fmt.Errorf("mcp-url is required for HTTP mode")
		}
	} else {
		if c.MCP.ServerPath == "" {
			return fmt.Errorf("mcp-server-path is required for stdio mode")
		}
	}

	// Validate LLM provider
	if c.LLM.Provider != "anthropic" && c.LLM.Provider != "ollama" {
		return fmt.Errorf("invalid llm-provider: %s (must be anthropic or ollama)", c.LLM.Provider)
	}

	// Validate LLM configuration based on provider
	if c.LLM.Provider == "anthropic" {
		if c.LLM.APIKey == "" {
			return fmt.Errorf("ANTHROPIC_API_KEY environment variable or api-key config is required for Anthropic")
		}
		if c.LLM.Model == "" {
			c.LLM.Model = "claude-sonnet-4-20250514"
		}
	} else {
		if c.LLM.OllamaURL == "" {
			c.LLM.OllamaURL = "http://localhost:11434"
		}
		if c.LLM.Model == "" {
			c.LLM.Model = "llama3"
		}
	}

	return nil
}

// getEnvOrDefault returns the environment variable value or default
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
