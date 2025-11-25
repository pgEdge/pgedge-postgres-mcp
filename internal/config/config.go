/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the complete server configuration
type Config struct {
	// HTTP server configuration
	HTTP HTTPConfig `yaml:"http"`

	// Database connection configuration
	Database DatabaseConfig `yaml:"database"`

	// Embedding configuration
	Embedding EmbeddingConfig `yaml:"embedding"`

	// LLM configuration (for web client chat proxy)
	LLM LLMConfig `yaml:"llm"`

	// Knowledgebase configuration
	Knowledgebase KnowledgebaseConfig `yaml:"knowledgebase"`

	// Secret file path (for encryption key)
	SecretFile string `yaml:"secret_file"`

	// Custom definitions file path (for user-defined prompts and resources)
	CustomDefinitionsPath string `yaml:"custom_definitions_path"`
}

// HTTPConfig holds HTTP/HTTPS server settings
type HTTPConfig struct {
	Enabled bool       `yaml:"enabled"`
	Address string     `yaml:"address"`
	TLS     TLSConfig  `yaml:"tls"`
	Auth    AuthConfig `yaml:"auth"`
}

// AuthConfig holds authentication settings
type AuthConfig struct {
	Enabled                        bool   `yaml:"enabled"`                            // Whether authentication is required
	TokenFile                      string `yaml:"token_file"`                         // Path to token configuration file
	MaxFailedAttemptsBeforeLockout int    `yaml:"max_failed_attempts_before_lockout"` // Number of failed login attempts before account lockout (0 = disabled)
	RateLimitWindowMinutes         int    `yaml:"rate_limit_window_minutes"`          // Time window in minutes for rate limiting (default: 15)
	RateLimitMaxAttempts           int    `yaml:"rate_limit_max_attempts"`            // Maximum failed attempts per IP in the time window (default: 10)
}

// TLSConfig holds TLS/HTTPS settings
type TLSConfig struct {
	Enabled   bool   `yaml:"enabled"`
	CertFile  string `yaml:"cert_file"`
	KeyFile   string `yaml:"key_file"`
	ChainFile string `yaml:"chain_file"`
}

// DatabaseConfig holds database connection settings
type DatabaseConfig struct {
	Host     string `yaml:"host"`     // Database host (default: localhost)
	Port     int    `yaml:"port"`     // Database port (default: 5432)
	Database string `yaml:"database"` // Database name (default: postgres)
	User     string `yaml:"user"`     // Database user (required)
	Password string `yaml:"password"` // Database password (optional, will use PGEDGE_DB_PASSWORD env var or .pgpass if not set)
	SSLMode  string `yaml:"sslmode"`  // SSL mode: disable, require, verify-ca, verify-full (default: prefer)

	// Connection pool settings
	PoolMaxConns        int    `yaml:"pool_max_conns"`          // Maximum number of connections (default: 4)
	PoolMinConns        int    `yaml:"pool_min_conns"`          // Minimum number of connections (default: 0)
	PoolMaxConnIdleTime string `yaml:"pool_max_conn_idle_time"` // Max time a connection can be idle before being closed (default: 30m)
}

// EmbeddingConfig holds embedding generation settings
type EmbeddingConfig struct {
	Enabled          bool   `yaml:"enabled"`             // Whether embedding generation is enabled (default: false)
	Provider         string `yaml:"provider"`            // "voyage", "openai", or "ollama"
	Model            string `yaml:"model"`               // Provider-specific model name
	VoyageAPIKey     string `yaml:"voyage_api_key"`      // API key for Voyage AI (direct - discouraged, use api_key_file or env var)
	VoyageAPIKeyFile string `yaml:"voyage_api_key_file"` // Path to file containing Voyage API key
	OpenAIAPIKey     string `yaml:"openai_api_key"`      // API key for OpenAI (direct - discouraged, use api_key_file or env var)
	OpenAIAPIKeyFile string `yaml:"openai_api_key_file"` // Path to file containing OpenAI API key
	OllamaURL        string `yaml:"ollama_url"`          // URL for Ollama service (default: http://localhost:11434)
}

// LLMConfig holds LLM configuration for web client chat proxy
type LLMConfig struct {
	Enabled             bool    `yaml:"enabled"`                // Whether LLM proxy is enabled (default: false)
	Provider            string  `yaml:"provider"`               // "anthropic", "openai", or "ollama"
	Model               string  `yaml:"model"`                  // Provider-specific model name
	AnthropicAPIKey     string  `yaml:"anthropic_api_key"`      // API key for Anthropic (direct - discouraged, use api_key_file or env var instead)
	AnthropicAPIKeyFile string  `yaml:"anthropic_api_key_file"` // Path to file containing Anthropic API key
	OpenAIAPIKey        string  `yaml:"openai_api_key"`         // API key for OpenAI (direct - discouraged, use api_key_file or env var instead)
	OpenAIAPIKeyFile    string  `yaml:"openai_api_key_file"`    // Path to file containing OpenAI API key
	OllamaURL           string  `yaml:"ollama_url"`             // URL for Ollama service (default: http://localhost:11434)
	MaxTokens           int     `yaml:"max_tokens"`             // Maximum tokens for LLM response (default: 4096)
	Temperature         float64 `yaml:"temperature"`            // Temperature for LLM sampling (default: 0.7)
}

// KnowledgebaseConfig holds knowledgebase configuration
type KnowledgebaseConfig struct {
	Enabled      bool   `yaml:"enabled"`       // Whether knowledgebase search is enabled (default: false)
	DatabasePath string `yaml:"database_path"` // Path to SQLite knowledgebase database

	// Embedding provider configuration for KB similarity search (independent of generate_embeddings tool)
	EmbeddingProvider         string `yaml:"embedding_provider"`            // "voyage", "openai", or "ollama"
	EmbeddingModel            string `yaml:"embedding_model"`               // Provider-specific model name
	EmbeddingVoyageAPIKey     string `yaml:"embedding_voyage_api_key"`      // API key for Voyage AI
	EmbeddingVoyageAPIKeyFile string `yaml:"embedding_voyage_api_key_file"` // Path to file containing Voyage API key
	EmbeddingOpenAIAPIKey     string `yaml:"embedding_openai_api_key"`      // API key for OpenAI
	EmbeddingOpenAIAPIKeyFile string `yaml:"embedding_openai_api_key_file"` // Path to file containing OpenAI API key
	EmbeddingOllamaURL        string `yaml:"embedding_ollama_url"`          // URL for Ollama service (default: http://localhost:11434)
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

	// Database flags
	DBHost     string
	DBHostSet  bool
	DBPort     int
	DBPortSet  bool
	DBName     string
	DBNameSet  bool
	DBUser     string
	DBUserSet  bool
	DBPassword string
	DBPassSet  bool
	DBSSLMode  string
	DBSSLSet   bool

	// Secret file flags
	SecretFile    string
	SecretFileSet bool
}

// defaultConfig returns configuration with hard-coded defaults
func defaultConfig() *Config {
	return &Config{
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
				Enabled:                        true, // Authentication enabled by default
				TokenFile:                      "",   // Will be set to default path if not specified
				MaxFailedAttemptsBeforeLockout: 0,    // Disabled by default (0 = no lockout)
				RateLimitWindowMinutes:         15,   // 15 minute window for rate limiting
				RateLimitMaxAttempts:           10,   // 10 attempts per IP per window
			},
		},
		Database: DatabaseConfig{
			Host:                "localhost",
			Port:                5432,
			Database:            "postgres",
			User:                "",       // Required - must be provided
			Password:            "",       // Optional - will use env var or .pgpass
			SSLMode:             "prefer", // Default SSL mode
			PoolMaxConns:        4,        // Default max connections
			PoolMinConns:        0,        // Default min connections
			PoolMaxConnIdleTime: "30m",    // Default idle timeout
		},
		Embedding: EmbeddingConfig{
			Enabled:      false,                    // Disabled by default (opt-in)
			Provider:     "ollama",                 // Default provider
			Model:        "nomic-embed-text",       // Default Ollama model
			VoyageAPIKey: "",                       // Must be provided if using Voyage AI
			OllamaURL:    "http://localhost:11434", // Default Ollama URL
		},
		LLM: LLMConfig{
			Enabled:         false,                    // Disabled by default (opt-in)
			Provider:        "anthropic",              // Default provider
			Model:           "claude-sonnet-4-5",      // Default Anthropic model
			AnthropicAPIKey: "",                       // Must be provided if using Anthropic
			OpenAIAPIKey:    "",                       // Must be provided if using OpenAI
			OllamaURL:       "http://localhost:11434", // Default Ollama URL
			MaxTokens:       4096,                     // Default max tokens
			Temperature:     0.7,                      // Default temperature
		},
		Knowledgebase: KnowledgebaseConfig{
			Enabled:               false,                    // Disabled by default (opt-in)
			DatabasePath:          "",                       // Must be provided if enabled
			EmbeddingProvider:     "ollama",                 // Default provider for KB embeddings
			EmbeddingModel:        "nomic-embed-text",       // Default Ollama model
			EmbeddingOllamaURL:    "http://localhost:11434", // Default Ollama URL
			EmbeddingVoyageAPIKey: "",                       // Must be provided if using Voyage
			EmbeddingOpenAIAPIKey: "",                       // Must be provided if using OpenAI
		},
		SecretFile: "", // Will be set to default path if not specified
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
	if src.HTTP.Auth.MaxFailedAttemptsBeforeLockout >= 0 {
		dest.HTTP.Auth.MaxFailedAttemptsBeforeLockout = src.HTTP.Auth.MaxFailedAttemptsBeforeLockout
	}
	if src.HTTP.Auth.RateLimitWindowMinutes > 0 {
		dest.HTTP.Auth.RateLimitWindowMinutes = src.HTTP.Auth.RateLimitWindowMinutes
	}
	if src.HTTP.Auth.RateLimitMaxAttempts > 0 {
		dest.HTTP.Auth.RateLimitMaxAttempts = src.HTTP.Auth.RateLimitMaxAttempts
	}

	// Database
	if src.Database.Host != "" {
		dest.Database.Host = src.Database.Host
	}
	if src.Database.Port != 0 {
		dest.Database.Port = src.Database.Port
	}
	if src.Database.Database != "" {
		dest.Database.Database = src.Database.Database
	}
	if src.Database.User != "" {
		dest.Database.User = src.Database.User
	}
	if src.Database.Password != "" {
		dest.Database.Password = src.Database.Password
	}
	if src.Database.SSLMode != "" {
		dest.Database.SSLMode = src.Database.SSLMode
	}
	if src.Database.PoolMaxConns != 0 {
		dest.Database.PoolMaxConns = src.Database.PoolMaxConns
	}
	if src.Database.PoolMinConns != 0 {
		dest.Database.PoolMinConns = src.Database.PoolMinConns
	}
	if src.Database.PoolMaxConnIdleTime != "" {
		dest.Database.PoolMaxConnIdleTime = src.Database.PoolMaxConnIdleTime
	}

	// Embedding - merge if any embedding fields are set
	if src.Embedding.Provider != "" || src.Embedding.Enabled {
		dest.Embedding.Enabled = src.Embedding.Enabled
		if src.Embedding.Provider != "" {
			dest.Embedding.Provider = src.Embedding.Provider
		}
		if src.Embedding.Model != "" {
			dest.Embedding.Model = src.Embedding.Model
		}
		if src.Embedding.VoyageAPIKey != "" {
			dest.Embedding.VoyageAPIKey = src.Embedding.VoyageAPIKey
		}
		if src.Embedding.VoyageAPIKeyFile != "" {
			dest.Embedding.VoyageAPIKeyFile = src.Embedding.VoyageAPIKeyFile
		}
		if src.Embedding.OpenAIAPIKey != "" {
			dest.Embedding.OpenAIAPIKey = src.Embedding.OpenAIAPIKey
		}
		if src.Embedding.OpenAIAPIKeyFile != "" {
			dest.Embedding.OpenAIAPIKeyFile = src.Embedding.OpenAIAPIKeyFile
		}
		if src.Embedding.OllamaURL != "" {
			dest.Embedding.OllamaURL = src.Embedding.OllamaURL
		}
	}

	// LLM - merge if any LLM fields are set
	if src.LLM.Provider != "" || src.LLM.Enabled {
		dest.LLM.Enabled = src.LLM.Enabled
		if src.LLM.Provider != "" {
			dest.LLM.Provider = src.LLM.Provider
		}
		if src.LLM.Model != "" {
			dest.LLM.Model = src.LLM.Model
		}
		if src.LLM.AnthropicAPIKey != "" {
			dest.LLM.AnthropicAPIKey = src.LLM.AnthropicAPIKey
		}
		if src.LLM.AnthropicAPIKeyFile != "" {
			dest.LLM.AnthropicAPIKeyFile = src.LLM.AnthropicAPIKeyFile
		}
		if src.LLM.OpenAIAPIKey != "" {
			dest.LLM.OpenAIAPIKey = src.LLM.OpenAIAPIKey
		}
		if src.LLM.OpenAIAPIKeyFile != "" {
			dest.LLM.OpenAIAPIKeyFile = src.LLM.OpenAIAPIKeyFile
		}
		if src.LLM.OllamaURL != "" {
			dest.LLM.OllamaURL = src.LLM.OllamaURL
		}
		if src.LLM.MaxTokens != 0 {
			dest.LLM.MaxTokens = src.LLM.MaxTokens
		}
		if src.LLM.Temperature != 0 {
			dest.LLM.Temperature = src.LLM.Temperature
		}
	}

	// Knowledgebase - merge if any KB fields are set
	if src.Knowledgebase.DatabasePath != "" || src.Knowledgebase.Enabled {
		dest.Knowledgebase.Enabled = src.Knowledgebase.Enabled
		if src.Knowledgebase.DatabasePath != "" {
			dest.Knowledgebase.DatabasePath = src.Knowledgebase.DatabasePath
		}
		if src.Knowledgebase.EmbeddingProvider != "" {
			dest.Knowledgebase.EmbeddingProvider = src.Knowledgebase.EmbeddingProvider
		}
		if src.Knowledgebase.EmbeddingModel != "" {
			dest.Knowledgebase.EmbeddingModel = src.Knowledgebase.EmbeddingModel
		}
		if src.Knowledgebase.EmbeddingVoyageAPIKey != "" {
			dest.Knowledgebase.EmbeddingVoyageAPIKey = src.Knowledgebase.EmbeddingVoyageAPIKey
		}
		if src.Knowledgebase.EmbeddingVoyageAPIKeyFile != "" {
			dest.Knowledgebase.EmbeddingVoyageAPIKeyFile = src.Knowledgebase.EmbeddingVoyageAPIKeyFile
		}
		if src.Knowledgebase.EmbeddingOpenAIAPIKey != "" {
			dest.Knowledgebase.EmbeddingOpenAIAPIKey = src.Knowledgebase.EmbeddingOpenAIAPIKey
		}
		if src.Knowledgebase.EmbeddingOpenAIAPIKeyFile != "" {
			dest.Knowledgebase.EmbeddingOpenAIAPIKeyFile = src.Knowledgebase.EmbeddingOpenAIAPIKeyFile
		}
		if src.Knowledgebase.EmbeddingOllamaURL != "" {
			dest.Knowledgebase.EmbeddingOllamaURL = src.Knowledgebase.EmbeddingOllamaURL
		}
	}

	// Secret file
	if src.SecretFile != "" {
		dest.SecretFile = src.SecretFile
	}

	// Custom definitions path
	if src.CustomDefinitionsPath != "" {
		dest.CustomDefinitionsPath = src.CustomDefinitionsPath
	}
}

// setStringFromEnv sets a string config value from an environment variable if it exists
func setStringFromEnv(dest *string, key string) {
	if val := os.Getenv(key); val != "" {
		*dest = val
	}
}

// setStringFromEnvWithFallback sets a string config value from an environment variable,
// checking multiple environment variable names in priority order
func setStringFromEnvWithFallback(dest *string, keys ...string) {
	for _, key := range keys {
		if val := os.Getenv(key); val != "" {
			*dest = val
			return
		}
	}
}

// setBoolFromEnv sets a boolean config value from an environment variable if it exists
// Accepts "true", "1", or "yes" as true values
func setBoolFromEnv(dest *bool, key string) {
	if val := os.Getenv(key); val != "" {
		*dest = val == "true" || val == "1" || val == "yes"
	}
}

// setIntFromEnv sets an integer config value from an environment variable if it exists
func setIntFromEnv(dest *int, key string) {
	if val := os.Getenv(key); val != "" {
		var intVal int
		_, err := fmt.Sscanf(val, "%d", &intVal)
		if err == nil {
			*dest = intVal
		}
	}
}

// applyEnvironmentVariables overrides config with environment variables if they exist
// All environment variables use the PGEDGE_ prefix to avoid collisions
func applyEnvironmentVariables(cfg *Config) {
	// HTTP
	setBoolFromEnv(&cfg.HTTP.Enabled, "PGEDGE_HTTP_ENABLED")
	setStringFromEnv(&cfg.HTTP.Address, "PGEDGE_HTTP_ADDRESS")

	// TLS
	setBoolFromEnv(&cfg.HTTP.TLS.Enabled, "PGEDGE_TLS_ENABLED")
	setStringFromEnv(&cfg.HTTP.TLS.CertFile, "PGEDGE_TLS_CERT_FILE")
	setStringFromEnv(&cfg.HTTP.TLS.KeyFile, "PGEDGE_TLS_KEY_FILE")
	setStringFromEnv(&cfg.HTTP.TLS.ChainFile, "PGEDGE_TLS_CHAIN_FILE")

	// Auth
	setBoolFromEnv(&cfg.HTTP.Auth.Enabled, "PGEDGE_AUTH_ENABLED")
	setStringFromEnv(&cfg.HTTP.Auth.TokenFile, "PGEDGE_AUTH_TOKEN_FILE")
	setIntFromEnv(&cfg.HTTP.Auth.MaxFailedAttemptsBeforeLockout, "PGEDGE_AUTH_MAX_FAILED_ATTEMPTS_BEFORE_LOCKOUT")
	setIntFromEnv(&cfg.HTTP.Auth.RateLimitWindowMinutes, "PGEDGE_AUTH_RATE_LIMIT_WINDOW_MINUTES")
	setIntFromEnv(&cfg.HTTP.Auth.RateLimitMaxAttempts, "PGEDGE_AUTH_RATE_LIMIT_MAX_ATTEMPTS")

	// Database
	setStringFromEnv(&cfg.Database.Host, "PGEDGE_DB_HOST")
	setIntFromEnv(&cfg.Database.Port, "PGEDGE_DB_PORT")
	setStringFromEnv(&cfg.Database.Database, "PGEDGE_DB_NAME")
	setStringFromEnv(&cfg.Database.User, "PGEDGE_DB_USER")
	setStringFromEnv(&cfg.Database.Password, "PGEDGE_DB_PASSWORD")
	setStringFromEnv(&cfg.Database.SSLMode, "PGEDGE_DB_SSLMODE")

	// Also support standard PostgreSQL environment variables for convenience
	if cfg.Database.Host == "localhost" {
		setStringFromEnv(&cfg.Database.Host, "PGHOST")
	}
	if cfg.Database.Port == 5432 {
		setIntFromEnv(&cfg.Database.Port, "PGPORT")
	}
	if cfg.Database.Database == "postgres" {
		setStringFromEnv(&cfg.Database.Database, "PGDATABASE")
	}
	if cfg.Database.User == "" {
		setStringFromEnv(&cfg.Database.User, "PGUSER")
	}
	if cfg.Database.Password == "" {
		setStringFromEnv(&cfg.Database.Password, "PGPASSWORD")
	}
	if cfg.Database.SSLMode == "prefer" {
		setStringFromEnv(&cfg.Database.SSLMode, "PGSSLMODE")
	}

	// Embedding
	setBoolFromEnv(&cfg.Embedding.Enabled, "PGEDGE_EMBEDDING_ENABLED")
	setStringFromEnv(&cfg.Embedding.Provider, "PGEDGE_EMBEDDING_PROVIDER")
	setStringFromEnv(&cfg.Embedding.Model, "PGEDGE_EMBEDDING_MODEL")
	// API key loading priority: env vars > api_key_file > direct config value
	// 1. Try environment variables first (PGEDGE_ prefixed, then standard)
	setStringFromEnvWithFallback(&cfg.Embedding.VoyageAPIKey, "PGEDGE_VOYAGE_API_KEY", "VOYAGE_API_KEY")
	setStringFromEnvWithFallback(&cfg.Embedding.OpenAIAPIKey, "PGEDGE_OPENAI_API_KEY", "OPENAI_API_KEY")
	// 2. If env vars not set and api_key_file is specified, load from file
	if cfg.Embedding.VoyageAPIKey == "" && cfg.Embedding.VoyageAPIKeyFile != "" {
		if key, err := readAPIKeyFromFile(cfg.Embedding.VoyageAPIKeyFile); err == nil && key != "" {
			cfg.Embedding.VoyageAPIKey = key
		}
		// Note: errors are silently ignored - file may not exist and that's ok
	}
	if cfg.Embedding.OpenAIAPIKey == "" && cfg.Embedding.OpenAIAPIKeyFile != "" {
		if key, err := readAPIKeyFromFile(cfg.Embedding.OpenAIAPIKeyFile); err == nil && key != "" {
			cfg.Embedding.OpenAIAPIKey = key
		}
		// Note: errors are silently ignored - file may not exist and that's ok
	}
	// 3. Direct config value (if set) is already in cfg.Embedding.VoyageAPIKey/OpenAIAPIKey from mergeConfig
	setStringFromEnv(&cfg.Embedding.OllamaURL, "PGEDGE_OLLAMA_URL")

	// LLM
	setBoolFromEnv(&cfg.LLM.Enabled, "PGEDGE_LLM_ENABLED")
	setStringFromEnv(&cfg.LLM.Provider, "PGEDGE_LLM_PROVIDER")
	setStringFromEnv(&cfg.LLM.Model, "PGEDGE_LLM_MODEL")
	// API key loading priority: env vars > api_key_file > direct config value
	// 1. Try environment variables first (PGEDGE_ prefixed, then standard)
	setStringFromEnvWithFallback(&cfg.LLM.AnthropicAPIKey, "PGEDGE_ANTHROPIC_API_KEY", "ANTHROPIC_API_KEY")
	setStringFromEnvWithFallback(&cfg.LLM.OpenAIAPIKey, "PGEDGE_OPENAI_API_KEY", "OPENAI_API_KEY")
	// 2. If env vars not set and api_key_file is specified, load from file
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
	// 3. Direct config value (if set) is already in cfg.LLM.AnthropicAPIKey/OpenAIAPIKey from mergeConfig
	setStringFromEnv(&cfg.LLM.OllamaURL, "PGEDGE_OLLAMA_URL")
	setIntFromEnv(&cfg.LLM.MaxTokens, "PGEDGE_LLM_MAX_TOKENS")
	// Temperature is a float, but we'll handle it specially
	if val := os.Getenv("PGEDGE_LLM_TEMPERATURE"); val != "" {
		var floatVal float64
		_, err := fmt.Sscanf(val, "%f", &floatVal)
		if err == nil {
			cfg.LLM.Temperature = floatVal
		}
	}

	// Secret file
	setStringFromEnv(&cfg.SecretFile, "PGEDGE_SECRET_FILE")

	// Custom definitions path
	setStringFromEnv(&cfg.CustomDefinitionsPath, "PGEDGE_CUSTOM_DEFINITIONS_PATH")
}

// applyCLIFlags overrides config with CLI flags if they were explicitly set
func applyCLIFlags(cfg *Config, flags CLIFlags) {
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

	// Database
	if flags.DBHostSet {
		cfg.Database.Host = flags.DBHost
	}
	if flags.DBPortSet {
		cfg.Database.Port = flags.DBPort
	}
	if flags.DBNameSet {
		cfg.Database.Database = flags.DBName
	}
	if flags.DBUserSet {
		cfg.Database.User = flags.DBUser
	}
	if flags.DBPassSet {
		cfg.Database.Password = flags.DBPassword
	}
	if flags.DBSSLSet {
		cfg.Database.SSLMode = flags.DBSSLMode
	}

	// Secret file
	if flags.SecretFileSet {
		cfg.SecretFile = flags.SecretFile
	}
}

// validateConfig checks if the configuration is valid
func validateConfig(cfg *Config) error {
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

	// Database configuration is optional - server can run with only stateless tools
	// If database parameters are partially set, validate they're complete
	if cfg.Database.Host != "" || cfg.Database.Port != 0 || cfg.Database.Database != "" {
		if cfg.Database.User == "" {
			return fmt.Errorf("database user is required when database host/port/database are configured (set via -db-user, PGEDGE_DB_USER, PGUSER env var, or config file)")
		}
	}

	return nil
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

// GetDefaultConfigPath returns the default config file path
// Searches /etc/pgedge/postgres-mcp/ first, then binary directory
func GetDefaultConfigPath(binaryPath string) string {
	systemPath := "/etc/pgedge/postgres-mcp/pgedge-nla-server.yaml"
	if _, err := os.Stat(systemPath); err == nil {
		return systemPath
	}

	dir := filepath.Dir(binaryPath)
	return filepath.Join(dir, "pgedge-nla-server.yaml")
}

// GetDefaultSecretPath returns the default secret file path
// Searches /etc/pgedge/postgres-mcp/ first, then binary directory
func GetDefaultSecretPath(binaryPath string) string {
	systemPath := "/etc/pgedge/postgres-mcp/pgedge-nla-server.secret"
	if _, err := os.Stat(systemPath); err == nil {
		return systemPath
	}

	dir := filepath.Dir(binaryPath)
	return filepath.Join(dir, "pgedge-nla-server.secret")
}

// BuildConnectionString creates a PostgreSQL connection string from DatabaseConfig
// If password is not set, pgx will automatically look it up from .pgpass file
func (cfg *DatabaseConfig) BuildConnectionString() string {
	// Build connection string components
	connStr := fmt.Sprintf("postgres://%s", cfg.User)

	// Add password only if explicitly set
	// If not set, pgx will use .pgpass file automatically
	if cfg.Password != "" {
		connStr += ":" + cfg.Password
	}

	connStr += fmt.Sprintf("@%s:%d/%s", cfg.Host, cfg.Port, cfg.Database)

	// Add SSL mode
	if cfg.SSLMode != "" {
		connStr += "?sslmode=" + cfg.SSLMode
	}

	return connStr
}

// ConfigFileExists checks if a config file exists at the given path
func ConfigFileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// SaveConfig saves the configuration to a YAML file
func SaveConfig(path string, cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write with appropriate permissions
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
