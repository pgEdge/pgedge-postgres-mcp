/*-------------------------------------------------------------------------
 *
 * pgEdge Postgres MCP Server
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the complete server configuration
type Config struct {
	// Database configuration
	Database DatabaseConfig `yaml:"database"`

	// LLM provider configuration
	LLM LLMConfig `yaml:"llm"`

	// Anthropic API configuration (deprecated, use LLM.Provider)
	Anthropic AnthropicConfig `yaml:"anthropic"`

	// Ollama configuration
	Ollama OllamaConfig `yaml:"ollama"`

	// HTTP server configuration
	HTTP HTTPConfig `yaml:"http"`
}

// DatabaseConfig holds PostgreSQL connection settings
type DatabaseConfig struct {
	ConnectionString string `yaml:"connection_string"`
}

// LLMConfig holds LLM provider selection
type LLMConfig struct {
	Provider string `yaml:"provider"` // "anthropic" or "ollama"
}

// AnthropicConfig holds Anthropic API settings
type AnthropicConfig struct {
	APIKey string `yaml:"api_key"`
	Model  string `yaml:"model"`
}

// OllamaConfig holds Ollama settings
type OllamaConfig struct {
	BaseURL string `yaml:"base_url"` // Ollama API URL (e.g., http://localhost:11434)
	Model   string `yaml:"model"`    // Model name (e.g., qwen2.5-coder:32b)
}

// HTTPConfig holds HTTP/HTTPS server settings
type HTTPConfig struct {
	Enabled bool      `yaml:"enabled"`
	Address string    `yaml:"address"`
	TLS     TLSConfig `yaml:"tls"`
	Auth    AuthConfig `yaml:"auth"`
}

// AuthConfig holds authentication settings
type AuthConfig struct {
	Enabled   bool   `yaml:"enabled"`    // Whether authentication is required
	TokenFile string `yaml:"token_file"` // Path to token configuration file
}

// TLSConfig holds TLS/HTTPS settings
type TLSConfig struct {
	Enabled   bool   `yaml:"enabled"`
	CertFile  string `yaml:"cert_file"`
	KeyFile   string `yaml:"key_file"`
	ChainFile string `yaml:"chain_file"`
}

// LoadConfig loads configuration with proper priority:
// 1. Command line flags (highest priority)
// 2. Environment variables
// 3. Configuration file
// 4. Hard-coded defaults (lowest priority)
func LoadConfig(configPath string, cliFlags CLIFlags) (*Config, error) {
	// Start with defaults
	cfg := defaultConfig()

	// Load config file if it exists
	if configPath != "" {
		fileCfg, err := loadConfigFile(configPath)
		if err != nil {
			// If file was explicitly specified, error out
			if cliFlags.ConfigFileSet {
				return nil, fmt.Errorf("failed to load config file %s: %w", configPath, err)
			}
			// Otherwise just use defaults (file may not exist and that's ok)
		} else {
			// Merge file config into defaults
			mergeConfig(cfg, fileCfg)
		}
	}

	// Override with environment variables
	applyEnvironmentVariables(cfg)

	// Override with command line flags (highest priority)
	applyCLIFlags(cfg, cliFlags)

	// Validate final configuration
	if err := validateConfig(cfg); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// CLIFlags represents command line flag values and whether they were explicitly set
type CLIFlags struct {
	ConfigFileSet bool
	ConfigFile    string

	// Database flags
	ConnectionString    string
	ConnectionStringSet bool

	// LLM provider flags
	LLMProvider    string
	LLMProviderSet bool

	// Anthropic flags
	APIKey    string
	APIKeySet bool
	Model     string
	ModelSet  bool

	// Ollama flags
	OllamaBaseURL    string
	OllamaBaseURLSet bool
	OllamaModel      string
	OllamaModelSet   bool

	// HTTP flags
	HTTPEnabled    bool
	HTTPEnabledSet bool
	HTTPAddr       string
	HTTPAddrSet    bool

	// TLS flags
	TLSEnabled    bool
	TLSEnabledSet bool
	TLSCertFile   string
	TLSCertSet    bool
	TLSKeyFile    string
	TLSKeySet     bool
	TLSChainFile  string
	TLSChainSet   bool

	// Auth flags
	AuthEnabled    bool
	AuthEnabledSet bool
	AuthTokenFile  string
	AuthTokenSet   bool
}

// defaultConfig returns configuration with hard-coded defaults
func defaultConfig() *Config {
	return &Config{
		Database: DatabaseConfig{
			ConnectionString: "",
		},
		LLM: LLMConfig{
			Provider: "anthropic", // Default to Anthropic
		},
		Anthropic: AnthropicConfig{
			APIKey: "",
			Model:  "claude-sonnet-4-5",
		},
		Ollama: OllamaConfig{
			BaseURL: "http://localhost:11434",
			Model:   "",
		},
		HTTP: HTTPConfig{
			Enabled: false,
			Address: ":8080",
			TLS: TLSConfig{
				Enabled:   false,
				CertFile:  "./server.crt",
				KeyFile:   "./server.key",
				ChainFile: "",
			},
			Auth: AuthConfig{
				Enabled:   true, // Authentication enabled by default
				TokenFile: "",   // Will be set to default path if not specified
			},
		},
	}
}

// loadConfigFile loads configuration from a YAML file
func loadConfigFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	return &cfg, nil
}

// mergeConfig merges source config into dest, only overriding non-zero values
func mergeConfig(dest, src *Config) {
	// Database
	if src.Database.ConnectionString != "" {
		dest.Database.ConnectionString = src.Database.ConnectionString
	}

	// LLM
	if src.LLM.Provider != "" {
		dest.LLM.Provider = src.LLM.Provider
	}

	// Anthropic
	if src.Anthropic.APIKey != "" {
		dest.Anthropic.APIKey = src.Anthropic.APIKey
	}
	if src.Anthropic.Model != "" {
		dest.Anthropic.Model = src.Anthropic.Model
	}

	// Ollama
	if src.Ollama.BaseURL != "" {
		dest.Ollama.BaseURL = src.Ollama.BaseURL
	}
	if src.Ollama.Model != "" {
		dest.Ollama.Model = src.Ollama.Model
	}

	// HTTP
	if src.HTTP.Enabled {
		dest.HTTP.Enabled = src.HTTP.Enabled
	}
	if src.HTTP.Address != "" {
		dest.HTTP.Address = src.HTTP.Address
	}

	// TLS
	if src.HTTP.TLS.Enabled {
		dest.HTTP.TLS.Enabled = src.HTTP.TLS.Enabled
	}
	if src.HTTP.TLS.CertFile != "" {
		dest.HTTP.TLS.CertFile = src.HTTP.TLS.CertFile
	}
	if src.HTTP.TLS.KeyFile != "" {
		dest.HTTP.TLS.KeyFile = src.HTTP.TLS.KeyFile
	}
	if src.HTTP.TLS.ChainFile != "" {
		dest.HTTP.TLS.ChainFile = src.HTTP.TLS.ChainFile
	}

	// Auth - note: we need to preserve false values, so check if src differs from default
	// Use a simple heuristic: if token file is set, assume auth config is intentional
	if src.HTTP.Auth.TokenFile != "" || !src.HTTP.Auth.Enabled {
		dest.HTTP.Auth.Enabled = src.HTTP.Auth.Enabled
		dest.HTTP.Auth.TokenFile = src.HTTP.Auth.TokenFile
	}
}

// applyEnvironmentVariables overrides config with environment variables if they exist
func applyEnvironmentVariables(cfg *Config) {
	if val := os.Getenv("POSTGRES_CONNECTION_STRING"); val != "" {
		cfg.Database.ConnectionString = val
	}

	if val := os.Getenv("LLM_PROVIDER"); val != "" {
		cfg.LLM.Provider = val
	}

	if val := os.Getenv("ANTHROPIC_API_KEY"); val != "" {
		cfg.Anthropic.APIKey = val
	}

	if val := os.Getenv("ANTHROPIC_MODEL"); val != "" {
		cfg.Anthropic.Model = val
	}

	if val := os.Getenv("OLLAMA_BASE_URL"); val != "" {
		cfg.Ollama.BaseURL = val
	}

	if val := os.Getenv("OLLAMA_MODEL"); val != "" {
		cfg.Ollama.Model = val
	}
}

// applyCLIFlags overrides config with CLI flags if they were explicitly set
func applyCLIFlags(cfg *Config, flags CLIFlags) {
	// Database
	if flags.ConnectionStringSet {
		cfg.Database.ConnectionString = flags.ConnectionString
	}

	// LLM Provider
	if flags.LLMProviderSet {
		cfg.LLM.Provider = flags.LLMProvider
	}

	// Anthropic
	if flags.APIKeySet {
		cfg.Anthropic.APIKey = flags.APIKey
	}
	if flags.ModelSet {
		cfg.Anthropic.Model = flags.Model
	}

	// Ollama
	if flags.OllamaBaseURLSet {
		cfg.Ollama.BaseURL = flags.OllamaBaseURL
	}
	if flags.OllamaModelSet {
		cfg.Ollama.Model = flags.OllamaModel
	}

	// HTTP
	if flags.HTTPEnabledSet {
		cfg.HTTP.Enabled = flags.HTTPEnabled
	}
	if flags.HTTPAddrSet {
		cfg.HTTP.Address = flags.HTTPAddr
	}

	// TLS
	if flags.TLSEnabledSet {
		cfg.HTTP.TLS.Enabled = flags.TLSEnabled
	}
	if flags.TLSCertSet {
		cfg.HTTP.TLS.CertFile = flags.TLSCertFile
	}
	if flags.TLSKeySet {
		cfg.HTTP.TLS.KeyFile = flags.TLSKeyFile
	}
	if flags.TLSChainSet {
		cfg.HTTP.TLS.ChainFile = flags.TLSChainFile
	}

	// Auth
	if flags.AuthEnabledSet {
		cfg.HTTP.Auth.Enabled = flags.AuthEnabled
	}
	if flags.AuthTokenSet {
		cfg.HTTP.Auth.TokenFile = flags.AuthTokenFile
	}
}

// validateConfig checks if the configuration is valid
func validateConfig(cfg *Config) error {
	// Database connection string is required
	if cfg.Database.ConnectionString == "" {
		return fmt.Errorf("database connection string is required (set via -db flag, POSTGRES_CONNECTION_STRING env var, or config file)")
	}

	// Validate LLM provider configuration
	switch cfg.LLM.Provider {
	case "anthropic":
		if cfg.Anthropic.APIKey == "" {
			return fmt.Errorf("anthropic API key is required when using anthropic provider (set via -api-key, ANTHROPIC_API_KEY, or config file)")
		}
		if cfg.Anthropic.Model == "" {
			cfg.Anthropic.Model = "claude-sonnet-4-5" // Set default
		}
	case "ollama":
		if cfg.Ollama.BaseURL == "" {
			cfg.Ollama.BaseURL = "http://localhost:11434" // Set default
		}
		if cfg.Ollama.Model == "" {
			return fmt.Errorf("ollama model is required when using ollama provider (set via -ollama-model, OLLAMA_MODEL, or config file)")
		}
	default:
		return fmt.Errorf("invalid LLM provider %q (must be 'anthropic' or 'ollama')", cfg.LLM.Provider)
	}

	// TLS requires HTTP to be enabled
	if cfg.HTTP.TLS.Enabled && !cfg.HTTP.Enabled {
		return fmt.Errorf("TLS requires HTTP mode to be enabled")
	}

	// If HTTPS is enabled, cert and key are required
	if cfg.HTTP.TLS.Enabled {
		if cfg.HTTP.TLS.CertFile == "" {
			return fmt.Errorf("TLS certificate file is required when HTTPS is enabled")
		}
		if cfg.HTTP.TLS.KeyFile == "" {
			return fmt.Errorf("TLS key file is required when HTTPS is enabled")
		}
	}

	// If HTTP is enabled and auth is enabled, token file is required
	if cfg.HTTP.Enabled && cfg.HTTP.Auth.Enabled {
		if cfg.HTTP.Auth.TokenFile == "" {
			return fmt.Errorf("authentication token file is required when HTTP auth is enabled (use -no-auth to disable)")
		}
	}

	return nil
}

// GetDefaultConfigPath returns the default config file path (same directory as binary)
func GetDefaultConfigPath(binaryPath string) string {
	dir := filepath.Dir(binaryPath)
	return filepath.Join(dir, "pgedge-postgres-mcp.yaml")
}

// ConfigFileExists checks if a config file exists at the given path
func ConfigFileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
