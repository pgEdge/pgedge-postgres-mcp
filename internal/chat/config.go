/*-------------------------------------------------------------------------
*
 * pgEdge Natural Language Agent
*
* Portions copyright (c) 2025, pgEdge, Inc.
* This software is released under The PostgreSQL License
*
*-------------------------------------------------------------------------
*/

package chat

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config holds all configuration for the chat client
type Config struct {
	MCP         MCPConfig `yaml:"mcp"`
	LLM         LLMConfig `yaml:"llm"`
	UI          UIConfig  `yaml:"ui"`
	HistoryFile string    `yaml:"history_file"` // Path to chat history file
}

// ConfigOverrides tracks which config values were explicitly set via command-line flags
type ConfigOverrides struct {
	ProviderSet bool // LLM provider was explicitly set via flag
	ModelSet    bool // LLM model was explicitly set via flag
}

// MCPConfig holds MCP server connection configuration
type MCPConfig struct {
	Mode             string `yaml:"mode"`               // stdio or http
	URL              string `yaml:"url"`                // HTTP URL (for http mode)
	ServerPath       string `yaml:"server_path"`        // Path to server binary (for stdio mode)
	ServerConfigPath string `yaml:"server_config_path"` // Path to server config file (for stdio mode)
	AuthMode         string `yaml:"auth_mode"`          // token or user (for http mode)
	Token            string `yaml:"token"`              // Authentication token (for token mode)
	Username         string `yaml:"username"`           // Username (for user mode)
	Password         string `yaml:"password"`           // Password (for user mode)
	TLS              bool   `yaml:"tls"`                // Use TLS/HTTPS
}

// LLMConfig holds LLM provider configuration
type LLMConfig struct {
	Provider            string  `yaml:"provider"`               // anthropic, openai, or ollama
	Model               string  `yaml:"model"`                  // Model to use
	AnthropicAPIKey     string  `yaml:"anthropic_api_key"`      // API key for Anthropic (direct - discouraged, use api_key_file or env var)
	AnthropicAPIKeyFile string  `yaml:"anthropic_api_key_file"` // Path to file containing Anthropic API key
	OpenAIAPIKey        string  `yaml:"openai_api_key"`         // API key for OpenAI (direct - discouraged, use api_key_file or env var)
	OpenAIAPIKeyFile    string  `yaml:"openai_api_key_file"`    // Path to file containing OpenAI API key
	OllamaURL           string  `yaml:"ollama_url"`             // Ollama server URL
	MaxTokens           int     `yaml:"max_tokens"`             // Max tokens for response
	Temperature         float64 `yaml:"temperature"`            // Temperature for sampling
}

// UIConfig holds UI configuration
type UIConfig struct {
	NoColor               bool `yaml:"no_color"`                // Disable colored output
	DisplayStatusMessages bool `yaml:"display_status_messages"` // Display status messages during execution
	RenderMarkdown        bool `yaml:"render_markdown"`         // Render markdown with formatting and syntax highlighting
	Debug                 bool `yaml:"debug"`                   // Display debug messages (e.g., LLM token usage)
}

// LoadConfig loads configuration from file, environment variables, and defaults
func LoadConfig(configPath string) (*Config, error) {
	cfg := &Config{
		MCP: MCPConfig{
			Mode:             getEnvOrDefault("PGEDGE_MCP_MODE", "stdio"),
			URL:              os.Getenv("PGEDGE_MCP_URL"),
			ServerPath:       getEnvOrDefault("PGEDGE_MCP_SERVER_PATH", "../../bin/pgedge-nla-server"),
			ServerConfigPath: getEnvOrDefault("PGEDGE_MCP_SERVER_CONFIG_PATH", ""),
			AuthMode:         getEnvOrDefault("PGEDGE_MCP_AUTH_MODE", "user"),
			Token:            "", // Will be loaded separately
			Username:         os.Getenv("PGEDGE_MCP_USERNAME"),
			Password:         os.Getenv("PGEDGE_MCP_PASSWORD"),
			TLS:              false,
		},
		LLM: LLMConfig{
			Provider:        getEnvOrDefault("PGEDGE_LLM_PROVIDER", "anthropic"),
			Model:           getEnvOrDefault("PGEDGE_LLM_MODEL", "claude-sonnet-4-20250514"),
			AnthropicAPIKey: getEnvWithFallback("PGEDGE_ANTHROPIC_API_KEY", "ANTHROPIC_API_KEY"),
			OpenAIAPIKey:    getEnvWithFallback("PGEDGE_OPENAI_API_KEY", "OPENAI_API_KEY"),
			OllamaURL:       getEnvOrDefault("PGEDGE_OLLAMA_URL", "http://localhost:11434"),
			MaxTokens:       4096,
			Temperature:     0.7,
		},
		UI: UIConfig{
			NoColor:               os.Getenv("NO_COLOR") != "",
			DisplayStatusMessages: true, // Default to showing status messages
			RenderMarkdown:        true, // Default to rendering markdown
		},
		HistoryFile: filepath.Join(os.Getenv("HOME"), ".pgedge-nla-cli-history"),
	}

	// Load from config file if provided
	if configPath != "" {
		if err := loadConfigFile(configPath, cfg); err != nil {
			return nil, fmt.Errorf("failed to load config file: %w", err)
		}
	} else {
		// Try default locations
		defaultPaths := []string{
			".pgedge-nla-cli.yaml",
			filepath.Join(os.Getenv("HOME"), ".pgedge-nla-cli.yaml"),
			"/etc/pgedge/pgedge-nla-cli.yaml",
		}
		for _, path := range defaultPaths {
			if _, err := os.Stat(path); err == nil {
				if err := loadConfigFile(path, cfg); err == nil {
					break
				}
			}
		}
	}

	// API key loading priority: env vars > api_key_file > direct config value
	// Environment variables were already loaded above, now check for API key files
	// 1. If env vars not set and api_key_file is specified, load from file
	if cfg.LLM.AnthropicAPIKey == "" && cfg.LLM.AnthropicAPIKeyFile != "" {
		if key, err := readAPIKeyFromFile(cfg.LLM.AnthropicAPIKeyFile); err == nil && key != "" {
			cfg.LLM.AnthropicAPIKey = key
		}
		// Note: errors are silently ignored - file may not exist and that's ok
	}
	if cfg.LLM.OpenAIAPIKey == "" && cfg.LLM.OpenAIAPIKeyFile != "" {
		if key, err := readAPIKeyFromFile(cfg.LLM.OpenAIAPIKeyFile); err == nil && key != "" {
			cfg.LLM.OpenAIAPIKey = key
		}
		// Note: errors are silently ignored - file may not exist and that's ok
	}
	// 2. Direct config value (if set) is already in cfg.LLM.AnthropicAPIKey/OpenAIAPIKey from loadConfigFile

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
// 1. Environment variable PGEDGE_MCP_TOKEN
// 2. File ~/.pgedge-pg-mcp-cli-token
// 3. Returns empty string if not found (will prompt if needed)
func loadAuthToken() string {
	// Priority 1: Environment variable
	if token := os.Getenv("PGEDGE_MCP_TOKEN"); token != "" {
		return token
	}

	// Priority 2: Token file
	tokenPath := filepath.Join(os.Getenv("HOME"), ".pgedge-pg-mcp-cli-token")
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

		// Validate auth mode
		if c.MCP.AuthMode != "token" && c.MCP.AuthMode != "user" {
			return fmt.Errorf("invalid auth-mode: %s (must be token or user)", c.MCP.AuthMode)
		}
	} else if c.MCP.ServerPath == "" {
		return fmt.Errorf("mcp-server-path is required for stdio mode")
	}

	// Validate LLM provider
	if c.LLM.Provider != "anthropic" && c.LLM.Provider != "openai" && c.LLM.Provider != "ollama" {
		return fmt.Errorf("invalid llm-provider: %s (must be anthropic, openai, or ollama)", c.LLM.Provider)
	}

	// Validate LLM configuration based on provider
	if c.LLM.Provider == "anthropic" {
		if c.LLM.AnthropicAPIKey == "" {
			return fmt.Errorf("PGEDGE_ANTHROPIC_API_KEY environment variable or anthropic_api_key config is required for Anthropic")
		}
		if c.LLM.Model == "" {
			c.LLM.Model = "claude-sonnet-4-20250514"
		}
	} else if c.LLM.Provider == "openai" {
		if c.LLM.OpenAIAPIKey == "" {
			return fmt.Errorf("PGEDGE_OPENAI_API_KEY environment variable or openai_api_key config is required for OpenAI")
		}
		if c.LLM.Model == "" {
			c.LLM.Model = "gpt-5-main"
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

// getEnvWithFallback checks multiple environment variable names in priority order
func getEnvWithFallback(keys ...string) string {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return ""
}

// readAPIKeyFromFile reads an API key from a file
// Returns the key with whitespace trimmed, or empty string if file doesn't exist or is empty
func readAPIKeyFromFile(filePath string) (string, error) {
	if filePath == "" {
		return "", nil
	}

	// Expand tilde to home directory
	if filePath != "" && filePath[0] == '~' {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		filePath = filepath.Join(homeDir, filePath[1:])
	}

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return "", nil // File doesn't exist, return empty (not an error)
	}

	// Read file contents
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read API key file %s: %w", filePath, err)
	}

	// Return trimmed contents (remove whitespace/newlines)
	key := strings.TrimSpace(string(data))
	return key, nil
}
