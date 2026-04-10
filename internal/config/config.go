/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package config

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the complete server configuration
type Config struct {
	// HTTP server configuration
	HTTP HTTPConfig `yaml:"http"`

	// Database connection configurations (list of named databases)
	Databases []NamedDatabaseConfig `yaml:"databases"`

	// Embedding configuration
	Embedding EmbeddingConfig `yaml:"embedding"`

	// LLM configuration (for web client chat proxy)
	LLM LLMConfig `yaml:"llm"`

	// Knowledgebase configuration
	Knowledgebase KnowledgebaseConfig `yaml:"knowledgebase"`

	// Built-in tools, resources, and prompts configuration
	Builtins BuiltinsConfig `yaml:"builtins"`

	// Secret file path (for encryption key)
	SecretFile string `yaml:"secret_file"`

	// Custom definitions file path (for user-defined prompts and resources)
	CustomDefinitionsPath string `yaml:"custom_definitions_path"`

	// Data directory path (for conversation history, etc.)
	DataDir string `yaml:"data_dir"`

	// Trace file path (for logging MCP requests/responses in JSONL format)
	TraceFile string `yaml:"trace_file"`
}

// BuiltinsConfig holds configuration for enabling/disabling built-in tools, resources, and prompts
type BuiltinsConfig struct {
	Tools     ToolsConfig     `yaml:"tools"`
	Resources ResourcesConfig `yaml:"resources"`
	Prompts   PromptsConfig   `yaml:"prompts"`
}

// ToolsConfig holds configuration for enabling/disabling built-in tools
// All tools are enabled by default
// Note: read_resource tool is always enabled as it's used to list resources
type ToolsConfig struct {
	QueryDatabase          *bool `yaml:"query_database"`           // Execute SQL queries (default: true)
	GetSchemaInfo          *bool `yaml:"get_schema_info"`          // Get detailed schema information (default: true)
	SimilaritySearch       *bool `yaml:"similarity_search"`        // Vector similarity search (default: true)
	ExecuteExplain         *bool `yaml:"execute_explain"`          // Execute EXPLAIN queries (default: true)
	GenerateEmbedding      *bool `yaml:"generate_embedding"`       // Generate text embeddings (default: true)
	SearchKnowledgebase    *bool `yaml:"search_knowledgebase"`     // Search knowledgebase (default: true)
	CountRows              *bool `yaml:"count_rows"`               // Count table rows (default: true)
	LLMConnectionSelection *bool `yaml:"llm_connection_selection"` // LLM can list/switch databases (default: false)
}

// ResourcesConfig holds configuration for enabling/disabling built-in resources
// All resources are enabled by default
type ResourcesConfig struct {
	SystemInfo *bool `yaml:"system_info"` // pg://system_info (default: true)
}

// PromptsConfig holds configuration for enabling/disabling built-in prompts
// All prompts are enabled by default
type PromptsConfig struct {
	ExploreDatabase     *bool `yaml:"explore_database"`      // explore-database prompt (default: true)
	SetupSemanticSearch *bool `yaml:"setup_semantic_search"` // setup-semantic-search prompt (default: true)
	DiagnoseQueryIssue  *bool `yaml:"diagnose_query_issue"`  // diagnose-query-issue prompt (default: true)
	DesignSchema        *bool `yaml:"design_schema"`         // design-schema prompt (default: true)
}

// IsToolEnabled returns true if the specified tool is enabled (defaults to true if not set)
func (c *ToolsConfig) IsToolEnabled(toolName string) bool {
	switch toolName {
	case "query_database":
		return c.QueryDatabase == nil || *c.QueryDatabase
	case "get_schema_info":
		return c.GetSchemaInfo == nil || *c.GetSchemaInfo
	case "similarity_search":
		return c.SimilaritySearch == nil || *c.SimilaritySearch
	case "execute_explain":
		return c.ExecuteExplain == nil || *c.ExecuteExplain
	case "generate_embedding":
		return c.GenerateEmbedding == nil || *c.GenerateEmbedding
	case "search_knowledgebase":
		return c.SearchKnowledgebase == nil || *c.SearchKnowledgebase
	case "count_rows":
		return c.CountRows == nil || *c.CountRows
	case "list_database_connections", "select_database_connection":
		// Both tools controlled by single config option (disabled by default)
		return c.LLMConnectionSelection != nil && *c.LLMConnectionSelection
	default:
		return true // Unknown tools are enabled by default
	}
}

// IsResourceEnabled returns true if the specified resource is enabled (defaults to true if not set)
func (c *ResourcesConfig) IsResourceEnabled(resourceURI string) bool {
	switch resourceURI {
	case "pg://system_info":
		return c.SystemInfo == nil || *c.SystemInfo
	default:
		return true // Unknown resources are enabled by default
	}
}

// IsPromptEnabled returns true if the specified prompt is enabled (defaults to true if not set)
func (c *PromptsConfig) IsPromptEnabled(promptName string) bool {
	switch promptName {
	case "explore-database":
		return c.ExploreDatabase == nil || *c.ExploreDatabase
	case "setup-semantic-search":
		return c.SetupSemanticSearch == nil || *c.SetupSemanticSearch
	case "diagnose-query-issue":
		return c.DiagnoseQueryIssue == nil || *c.DiagnoseQueryIssue
	case "design-schema":
		return c.DesignSchema == nil || *c.DesignSchema
	default:
		return true // Unknown prompts are enabled by default
	}
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
	UserFile                       string `yaml:"user_file"`                          // Path to user configuration file
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

// HostEntry represents a single host:port pair for multi-host connection strings.
type HostEntry struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

// ParseHostEntries parses a comma-separated list of host:port pairs.
// Port defaults to 5432 if omitted.
func ParseHostEntries(s string) ([]HostEntry, error) {
	var entries []HostEntry
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		var host string
		port := 5432

		if strings.HasPrefix(part, "[") {
			// Bracketed IPv6: [2001:db8::1]:5432 or [2001:db8::1]
			closeBracket := strings.Index(part, "]")
			if closeBracket < 0 {
				return nil, fmt.Errorf(
					"missing closing bracket in host entry %q", part)
			}
			host = part[1:closeBracket]
			rest := part[closeBracket+1:]
			if strings.HasPrefix(rest, ":") {
				var err error
				port, err = strconv.Atoi(rest[1:])
				if err != nil {
					return nil, fmt.Errorf(
						"invalid port in host entry %q: %w",
						part, err)
				}
			} else if rest != "" {
				return nil, fmt.Errorf(
					"unexpected characters after bracket in "+
						"host entry %q", part)
			}
		} else if strings.Count(part, ":") > 1 {
			// Unbracketed IPv6: 2001:db8::1 (no port, use default)
			host = part
		} else {
			// IPv4 or hostname with optional :port
			if idx := strings.LastIndex(part, ":"); idx >= 0 {
				var err error
				port, err = strconv.Atoi(part[idx+1:])
				if err != nil {
					return nil, fmt.Errorf(
						"invalid port in host entry %q: %w",
						part, err)
				}
				host = part[:idx]
			} else {
				host = part
			}
		}

		if port < 1 || port > 65535 {
			return nil, fmt.Errorf(
				"port %d out of range (1-65535) in host entry %q",
				port, part)
		}
		entries = append(entries, HostEntry{Host: host, Port: port})
	}
	return entries, nil
}

// NamedDatabaseConfig holds named database connection settings with access control
type NamedDatabaseConfig struct {
	Name     string `yaml:"name"`     // Unique name for this database connection (required)
	Host     string `yaml:"host"`     // Database host (default: localhost)
	Port     int    `yaml:"port"`     // Database port (default: 5432)
	Database string `yaml:"database"` // Database name (default: postgres)
	User     string `yaml:"user"`     // Database user (required)
	Password string `yaml:"password"` // Database password (optional, will use PGEDGE_DB_PASSWORD env var or .pgpass if not set)
	SSLMode  string `yaml:"sslmode"`  // SSL mode: disable, require, verify-ca, verify-full (default: prefer)

	// Multi-host connection support (optional; overrides Host/Port when set).
	// Each entry specifies a host:port pair. pgx/v5 tries hosts in order for failover.
	Hosts []HostEntry `yaml:"hosts,omitempty"`

	// Target session attributes for multi-host connections.
	// Valid values: any, read-write, read-only, primary, standby, prefer-standby.
	// Only used when Hosts is set.
	TargetSessionAttrs string `yaml:"target_session_attrs,omitempty"`

	AvailableToUsers  []string `yaml:"available_to_users,omitempty"`  // List of usernames allowed to access this database (empty = all users)
	AllowWrites       bool     `yaml:"allow_writes"`                  // Allow LLM to execute write queries (default: false - read-only)
	AllowLLMSwitching *bool    `yaml:"allow_llm_switching,omitempty"` // Allow LLM to discover/switch to this database (default: true when feature enabled)

	// Custom tool settings
	AllowedPLLanguages []string `yaml:"allowed_pl_languages,omitempty"` // PL languages allowed for custom tools (e.g., ["plpgsql", "plpython3u"]). Use ["*"] for all.

	// Connection pool settings
	PoolMaxConns          int    `yaml:"pool_max_conns"`           // Maximum number of connections (default: 4)
	PoolMinConns          int    `yaml:"pool_min_conns"`           // Minimum number of connections (default: 0)
	PoolMaxConnIdleTime   string `yaml:"pool_max_conn_idle_time"`  // Max time a connection can be idle before being closed (default: 30m)
	PoolHealthCheckPeriod string `yaml:"pool_health_check_period"` // How often idle connections are checked (default: 30s for multi-host, 0 for single-host)
	PoolMaxConnLifetime   string `yaml:"pool_max_conn_lifetime"`   // Max lifetime of a connection before it is closed and recreated (default: 5m for multi-host, 0 for single-host)
	ConnectTimeout        string `yaml:"connect_timeout"`          // Timeout for initial connection (default: 10s)
	MetadataTTL           string `yaml:"metadata_ttl"`             // How long cached schema metadata remains valid before automatic refresh (default: "5m", "0" = always refresh)
}

// formatHostPort returns a host:port string, bracketing IPv6 addresses.
func formatHostPort(host string, port int) string {
	if strings.Contains(host, ":") {
		return fmt.Sprintf("[%s]:%d", host, port)
	}
	return fmt.Sprintf("%s:%d", host, port)
}

// BuildConnectionString creates a PostgreSQL connection string from NamedDatabaseConfig
// If password is not set, pgx will automatically look it up from .pgpass file
func (cfg *NamedDatabaseConfig) BuildConnectionString() string {
	u := &url.URL{
		Scheme: "postgres",
	}

	// Determine host portion
	if len(cfg.Hosts) > 0 {
		// Multi-host: build comma-separated host:port list
		parts := make([]string, len(cfg.Hosts))
		for i, h := range cfg.Hosts {
			port := h.Port
			if port == 0 {
				port = 5432
			}
			parts[i] = formatHostPort(h.Host, port)
		}
		u.Host = strings.Join(parts, ",")
	} else {
		// Single host (existing behavior)
		u.Host = formatHostPort(cfg.Host, cfg.Port)
	}

	u.Path = cfg.Database

	// Set user info with proper encoding
	if cfg.Password != "" {
		u.User = url.UserPassword(cfg.User, cfg.Password)
	} else {
		u.User = url.User(cfg.User)
	}

	// Add query parameters
	q := u.Query()
	if cfg.SSLMode != "" {
		q.Set("sslmode", cfg.SSLMode)
	}
	if cfg.TargetSessionAttrs != "" && len(cfg.Hosts) > 0 {
		q.Set("target_session_attrs", cfg.TargetSessionAttrs)
	}
	if cfg.ConnectTimeout != "" {
		if d, err := time.ParseDuration(cfg.ConnectTimeout); err == nil {
			q.Set("connect_timeout", strconv.Itoa(int(d.Seconds())))
		}
	}
	if len(q) > 0 {
		u.RawQuery = q.Encode()
	}

	return u.String()
}

// validTargetSessionAttrs lists the valid values for
// target_session_attrs per the libpq specification.
var validTargetSessionAttrs = map[string]bool{
	"any":            true,
	"read-write":     true,
	"read-only":      true,
	"primary":        true,
	"standby":        true,
	"prefer-standby": true,
}

// validateHostname checks that a hostname does not contain characters
// that would corrupt a libpq connection string (commas, whitespace,
// at-signs, slashes, or question marks).
func validateHostname(host string) error {
	if strings.ContainsAny(host, ", \t\n\r@/?") {
		return fmt.Errorf(
			"invalid hostname %q: must not contain commas, "+
				"whitespace, or URI-special characters",
			host,
		)
	}
	return nil
}

// Validate checks the NamedDatabaseConfig for invalid combinations.
func (cfg *NamedDatabaseConfig) Validate() error {
	// Cannot set both host/port and hosts list
	if len(cfg.Hosts) > 0 && cfg.Host != "" {
		return fmt.Errorf(
			"database %q: cannot set both 'host' and 'hosts'; "+
				"use 'hosts' for multi-host or 'host' for single-host",
			cfg.Name,
		)
	}

	// Validate single-host hostname
	if cfg.Host != "" {
		if err := validateHostname(cfg.Host); err != nil {
			return fmt.Errorf("database %q: %w", cfg.Name, err)
		}
	}

	// Validate each host entry
	for i, h := range cfg.Hosts {
		if h.Host == "" {
			return fmt.Errorf(
				"database %q: hosts[%d] has empty host",
				cfg.Name, i,
			)
		}
		if err := validateHostname(h.Host); err != nil {
			return fmt.Errorf("database %q: hosts[%d]: %w",
				cfg.Name, i, err)
		}
		if h.Port != 0 && (h.Port < 1 || h.Port > 65535) {
			return fmt.Errorf(
				"database %q: hosts[%d] has invalid port %d",
				cfg.Name, i, h.Port,
			)
		}
	}

	// target_session_attrs only valid with multi-host
	if cfg.TargetSessionAttrs != "" && len(cfg.Hosts) == 0 {
		return fmt.Errorf(
			"database %q: target_session_attrs requires 'hosts' "+
				"to be set for multi-host connections",
			cfg.Name,
		)
	}

	// Validate target_session_attrs value
	if cfg.TargetSessionAttrs != "" {
		if !validTargetSessionAttrs[cfg.TargetSessionAttrs] {
			return fmt.Errorf(
				"database %q: invalid target_session_attrs %q; "+
					"valid values: any, read-write, read-only, "+
					"primary, standby, prefer-standby",
				cfg.Name, cfg.TargetSessionAttrs,
			)
		}
	}

	return nil
}

// IsAllowedForLLMSwitching returns true if LLM is allowed to switch to this database
// Defaults to true if not explicitly set (when LLM connection selection is enabled)
func (cfg *NamedDatabaseConfig) IsAllowedForLLMSwitching() bool {
	return cfg.AllowLLMSwitching == nil || *cfg.AllowLLMSwitching
}

// EmbeddingConfig holds embedding generation settings
type EmbeddingConfig struct {
	Enabled          bool   `yaml:"enabled"`             // Whether embedding generation is enabled (default: false)
	Provider         string `yaml:"provider"`            // "voyage", "openai", or "ollama"
	Model            string `yaml:"model"`               // Provider-specific model name
	VoyageAPIKey     string `yaml:"voyage_api_key"`      // API key for Voyage AI (direct - discouraged, use api_key_file or env var)
	VoyageAPIKeyFile string `yaml:"voyage_api_key_file"` // Path to file containing Voyage API key
	VoyageBaseURL    string `yaml:"voyage_base_url"`     // Base URL for Voyage API (default: https://api.voyageai.com/v1/embeddings)
	OpenAIAPIKey     string `yaml:"openai_api_key"`      // API key for OpenAI (direct - discouraged, use api_key_file or env var)
	OpenAIAPIKeyFile string `yaml:"openai_api_key_file"` // Path to file containing OpenAI API key
	OpenAIBaseURL    string `yaml:"openai_base_url"`     // Base URL for OpenAI API (default: https://api.openai.com/v1)
	OllamaURL        string `yaml:"ollama_url"`          // URL for Ollama service (default: http://localhost:11434)
}

// LLMConfig holds LLM configuration for web client chat proxy
type LLMConfig struct {
	Enabled             bool    `yaml:"enabled"`                // Whether LLM proxy is enabled (default: false)
	Provider            string  `yaml:"provider"`               // "anthropic", "openai", or "ollama"
	Model               string  `yaml:"model"`                  // Provider-specific model name
	AnthropicAPIKey     string  `yaml:"anthropic_api_key"`      // API key for Anthropic (direct - discouraged, use api_key_file or env var instead)
	AnthropicAPIKeyFile string  `yaml:"anthropic_api_key_file"` // Path to file containing Anthropic API key
	AnthropicBaseURL    string  `yaml:"anthropic_base_url"`     // Base URL for Anthropic API (default: https://api.anthropic.com)
	OpenAIAPIKey        string  `yaml:"openai_api_key"`         // API key for OpenAI (direct - discouraged, use api_key_file or env var instead)
	OpenAIAPIKeyFile    string  `yaml:"openai_api_key_file"`    // Path to file containing OpenAI API key
	OpenAIBaseURL       string  `yaml:"openai_base_url"`        // Base URL for OpenAI API (default: https://api.openai.com)
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
	EmbeddingVoyageBaseURL    string `yaml:"embedding_voyage_base_url"`     // Base URL for Voyage API (default: https://api.voyageai.com/v1/embeddings)
	EmbeddingOpenAIAPIKey     string `yaml:"embedding_openai_api_key"`      // API key for OpenAI
	EmbeddingOpenAIAPIKeyFile string `yaml:"embedding_openai_api_key_file"` // Path to file containing OpenAI API key
	EmbeddingOpenAIBaseURL    string `yaml:"embedding_openai_base_url"`     // Base URL for OpenAI API (default: https://api.openai.com/v1)
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
	if err := applyCLIFlags(cfg, cliFlags); err != nil {
		return nil, fmt.Errorf("applying CLI flags: %w", err)
	}

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
	AuthUserFile   string
	AuthUserSet    bool

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

	// Multi-host database flags
	DBHosts                 string
	DBHostsSet              bool
	DBTargetSessionAttrs    string
	DBTargetSessionAttrsSet bool

	// Secret file flags
	SecretFile    string
	SecretFileSet bool

	// Trace file flags
	TraceFile    string
	TraceFileSet bool
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
		Databases: []NamedDatabaseConfig{}, // Empty by default, populated from config file
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
	if src.HTTP.Auth.UserFile != "" {
		dest.HTTP.Auth.UserFile = src.HTTP.Auth.UserFile
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

	// Databases - if source has databases defined, use them (replace, don't merge)
	if len(src.Databases) > 0 {
		dest.Databases = src.Databases
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
		if src.Embedding.VoyageBaseURL != "" {
			dest.Embedding.VoyageBaseURL = src.Embedding.VoyageBaseURL
		}
		if src.Embedding.OpenAIAPIKey != "" {
			dest.Embedding.OpenAIAPIKey = src.Embedding.OpenAIAPIKey
		}
		if src.Embedding.OpenAIAPIKeyFile != "" {
			dest.Embedding.OpenAIAPIKeyFile = src.Embedding.OpenAIAPIKeyFile
		}
		if src.Embedding.OpenAIBaseURL != "" {
			dest.Embedding.OpenAIBaseURL = src.Embedding.OpenAIBaseURL
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
		if src.LLM.AnthropicBaseURL != "" {
			dest.LLM.AnthropicBaseURL = src.LLM.AnthropicBaseURL
		}
		if src.LLM.OpenAIAPIKey != "" {
			dest.LLM.OpenAIAPIKey = src.LLM.OpenAIAPIKey
		}
		if src.LLM.OpenAIAPIKeyFile != "" {
			dest.LLM.OpenAIAPIKeyFile = src.LLM.OpenAIAPIKeyFile
		}
		if src.LLM.OpenAIBaseURL != "" {
			dest.LLM.OpenAIBaseURL = src.LLM.OpenAIBaseURL
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
		if src.Knowledgebase.EmbeddingVoyageBaseURL != "" {
			dest.Knowledgebase.EmbeddingVoyageBaseURL = src.Knowledgebase.EmbeddingVoyageBaseURL
		}
		if src.Knowledgebase.EmbeddingOpenAIAPIKey != "" {
			dest.Knowledgebase.EmbeddingOpenAIAPIKey = src.Knowledgebase.EmbeddingOpenAIAPIKey
		}
		if src.Knowledgebase.EmbeddingOpenAIAPIKeyFile != "" {
			dest.Knowledgebase.EmbeddingOpenAIAPIKeyFile = src.Knowledgebase.EmbeddingOpenAIAPIKeyFile
		}
		if src.Knowledgebase.EmbeddingOpenAIBaseURL != "" {
			dest.Knowledgebase.EmbeddingOpenAIBaseURL = src.Knowledgebase.EmbeddingOpenAIBaseURL
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

	// Data directory
	if src.DataDir != "" {
		dest.DataDir = src.DataDir
	}

	// Trace file
	if src.TraceFile != "" {
		dest.TraceFile = src.TraceFile
	}

	// Builtins - merge individual settings (pointer fields preserve explicit false values)
	// Tools
	if src.Builtins.Tools.QueryDatabase != nil {
		dest.Builtins.Tools.QueryDatabase = src.Builtins.Tools.QueryDatabase
	}
	if src.Builtins.Tools.GetSchemaInfo != nil {
		dest.Builtins.Tools.GetSchemaInfo = src.Builtins.Tools.GetSchemaInfo
	}
	if src.Builtins.Tools.SimilaritySearch != nil {
		dest.Builtins.Tools.SimilaritySearch = src.Builtins.Tools.SimilaritySearch
	}
	if src.Builtins.Tools.ExecuteExplain != nil {
		dest.Builtins.Tools.ExecuteExplain = src.Builtins.Tools.ExecuteExplain
	}
	if src.Builtins.Tools.GenerateEmbedding != nil {
		dest.Builtins.Tools.GenerateEmbedding = src.Builtins.Tools.GenerateEmbedding
	}
	if src.Builtins.Tools.SearchKnowledgebase != nil {
		dest.Builtins.Tools.SearchKnowledgebase = src.Builtins.Tools.SearchKnowledgebase
	}
	if src.Builtins.Tools.LLMConnectionSelection != nil {
		dest.Builtins.Tools.LLMConnectionSelection = src.Builtins.Tools.LLMConnectionSelection
	}
	// Resources
	if src.Builtins.Resources.SystemInfo != nil {
		dest.Builtins.Resources.SystemInfo = src.Builtins.Resources.SystemInfo
	}
	// Prompts
	if src.Builtins.Prompts.ExploreDatabase != nil {
		dest.Builtins.Prompts.ExploreDatabase = src.Builtins.Prompts.ExploreDatabase
	}
	if src.Builtins.Prompts.SetupSemanticSearch != nil {
		dest.Builtins.Prompts.SetupSemanticSearch = src.Builtins.Prompts.SetupSemanticSearch
	}
	if src.Builtins.Prompts.DiagnoseQueryIssue != nil {
		dest.Builtins.Prompts.DiagnoseQueryIssue = src.Builtins.Prompts.DiagnoseQueryIssue
	}
	if src.Builtins.Prompts.DesignSchema != nil {
		dest.Builtins.Prompts.DesignSchema = src.Builtins.Prompts.DesignSchema
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
// Accepts "true", "1", or "yes" (case-insensitive) as true values
func setBoolFromEnv(dest *bool, key string) {
	if val := os.Getenv(key); val != "" {
		lower := strings.ToLower(val)
		*dest = lower == "true" || lower == "1" || lower == "yes"
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
	setStringFromEnv(&cfg.HTTP.Auth.UserFile, "PGEDGE_AUTH_USER_FILE")
	setIntFromEnv(&cfg.HTTP.Auth.MaxFailedAttemptsBeforeLockout, "PGEDGE_AUTH_MAX_FAILED_ATTEMPTS_BEFORE_LOCKOUT")
	setIntFromEnv(&cfg.HTTP.Auth.RateLimitWindowMinutes, "PGEDGE_AUTH_RATE_LIMIT_WINDOW_MINUTES")
	setIntFromEnv(&cfg.HTTP.Auth.RateLimitMaxAttempts, "PGEDGE_AUTH_RATE_LIMIT_MAX_ATTEMPTS")

	// Database environment variables apply to the first database in the list
	// If no databases configured yet, create a default one from env vars
	if len(cfg.Databases) == 0 {
		// Check if any database env vars are set
		if os.Getenv("PGEDGE_DB_USER") != "" || os.Getenv("PGUSER") != "" {
			cfg.Databases = []NamedDatabaseConfig{{
				Name:                "default",
				Host:                "localhost",
				Port:                5432,
				Database:            "postgres",
				SSLMode:             "prefer",
				PoolMaxConns:        4,
				PoolMinConns:        0,
				PoolMaxConnIdleTime: "30m",
			}}
		}
	}

	// Apply env vars to first database if it exists
	if len(cfg.Databases) > 0 {
		setStringFromEnv(&cfg.Databases[0].Host, "PGEDGE_DB_HOST")
		setIntFromEnv(&cfg.Databases[0].Port, "PGEDGE_DB_PORT")
		setStringFromEnv(&cfg.Databases[0].Database, "PGEDGE_DB_NAME")
		setStringFromEnv(&cfg.Databases[0].User, "PGEDGE_DB_USER")
		setStringFromEnv(&cfg.Databases[0].Password, "PGEDGE_DB_PASSWORD")
		setStringFromEnv(&cfg.Databases[0].SSLMode, "PGEDGE_DB_SSLMODE")
		setBoolFromEnv(&cfg.Databases[0].AllowWrites, "PGEDGE_DB_ALLOW_WRITES")

		// Also support standard PostgreSQL environment variables for convenience
		if cfg.Databases[0].Host == "localhost" {
			setStringFromEnv(&cfg.Databases[0].Host, "PGHOST")
		}
		if cfg.Databases[0].Port == 5432 {
			setIntFromEnv(&cfg.Databases[0].Port, "PGPORT")
		}
		if cfg.Databases[0].Database == "postgres" {
			setStringFromEnv(&cfg.Databases[0].Database, "PGDATABASE")
		}
		if cfg.Databases[0].User == "" {
			setStringFromEnv(&cfg.Databases[0].User, "PGUSER")
		}
		if cfg.Databases[0].Password == "" {
			setStringFromEnv(&cfg.Databases[0].Password, "PGPASSWORD")
		}
		if cfg.Databases[0].SSLMode == "prefer" {
			setStringFromEnv(&cfg.Databases[0].SSLMode, "PGSSLMODE")
		}

		// Multi-host connection support via environment variable
		if hostsEnv := os.Getenv("PGEDGE_DB_HOSTS"); hostsEnv != "" {
			entries, err := ParseHostEntries(hostsEnv)
			if err != nil {
				log.Printf("WARNING: ignoring invalid PGEDGE_DB_HOSTS value %q: %v", hostsEnv, err)
			} else {
				cfg.Databases[0].Hosts = entries
				cfg.Databases[0].Host = "" // Clear single host to avoid conflict
			}
		}

		setStringFromEnv(&cfg.Databases[0].TargetSessionAttrs, "PGEDGE_DB_TARGET_SESSION_ATTRS")
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
	// Base URL overrides for embedding providers (useful for proxies)
	setStringFromEnv(&cfg.Embedding.VoyageBaseURL, "PGEDGE_VOYAGE_BASE_URL")
	setStringFromEnv(&cfg.Embedding.OpenAIBaseURL, "PGEDGE_OPENAI_EMBEDDING_BASE_URL")

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
	// Base URL overrides for LLM providers (useful for proxies)
	setStringFromEnv(&cfg.LLM.AnthropicBaseURL, "PGEDGE_ANTHROPIC_BASE_URL")
	setStringFromEnv(&cfg.LLM.OpenAIBaseURL, "PGEDGE_OPENAI_BASE_URL")
	setIntFromEnv(&cfg.LLM.MaxTokens, "PGEDGE_LLM_MAX_TOKENS")
	// Temperature is a float, but we'll handle it specially
	if val := os.Getenv("PGEDGE_LLM_TEMPERATURE"); val != "" {
		var floatVal float64
		_, err := fmt.Sscanf(val, "%f", &floatVal)
		if err == nil {
			cfg.LLM.Temperature = floatVal
		}
	}

	// Knowledgebase
	setBoolFromEnv(&cfg.Knowledgebase.Enabled, "PGEDGE_KB_ENABLED")
	setStringFromEnv(&cfg.Knowledgebase.DatabasePath, "PGEDGE_KB_DATABASE_PATH")
	setStringFromEnv(&cfg.Knowledgebase.EmbeddingProvider, "PGEDGE_KB_EMBEDDING_PROVIDER")
	setStringFromEnv(&cfg.Knowledgebase.EmbeddingModel, "PGEDGE_KB_EMBEDDING_MODEL")
	// API key loading priority: env vars > api_key_file > direct config value
	// 1. Try environment variables first (PGEDGE_ prefixed, then standard)
	setStringFromEnvWithFallback(&cfg.Knowledgebase.EmbeddingVoyageAPIKey, "PGEDGE_KB_VOYAGE_API_KEY", "VOYAGE_API_KEY")
	setStringFromEnvWithFallback(&cfg.Knowledgebase.EmbeddingOpenAIAPIKey, "PGEDGE_KB_OPENAI_API_KEY", "OPENAI_API_KEY")
	// 2. If env vars not set and api_key_file is specified, load from file
	if cfg.Knowledgebase.EmbeddingVoyageAPIKey == "" && cfg.Knowledgebase.EmbeddingVoyageAPIKeyFile != "" {
		if key, err := readAPIKeyFromFile(cfg.Knowledgebase.EmbeddingVoyageAPIKeyFile); err == nil && key != "" {
			cfg.Knowledgebase.EmbeddingVoyageAPIKey = key
		}
		// Note: errors are silently ignored - file may not exist and that's ok
	}
	if cfg.Knowledgebase.EmbeddingOpenAIAPIKey == "" && cfg.Knowledgebase.EmbeddingOpenAIAPIKeyFile != "" {
		if key, err := readAPIKeyFromFile(cfg.Knowledgebase.EmbeddingOpenAIAPIKeyFile); err == nil && key != "" {
			cfg.Knowledgebase.EmbeddingOpenAIAPIKey = key
		}
		// Note: errors are silently ignored - file may not exist and that's ok
	}
	// 3. Direct config value (if set) is already in cfg.Knowledgebase.EmbeddingVoyageAPIKey/EmbeddingOpenAIAPIKey from mergeConfig
	setStringFromEnv(&cfg.Knowledgebase.EmbeddingOllamaURL, "PGEDGE_KB_OLLAMA_URL")
	// Base URL overrides for KB embedding providers (useful for proxies)
	setStringFromEnv(&cfg.Knowledgebase.EmbeddingVoyageBaseURL, "PGEDGE_KB_VOYAGE_BASE_URL")
	setStringFromEnv(&cfg.Knowledgebase.EmbeddingOpenAIBaseURL, "PGEDGE_KB_OPENAI_BASE_URL")

	// Secret file
	setStringFromEnv(&cfg.SecretFile, "PGEDGE_SECRET_FILE")

	// Custom definitions path
	setStringFromEnv(&cfg.CustomDefinitionsPath, "PGEDGE_CUSTOM_DEFINITIONS_PATH")

	// Data directory
	setStringFromEnv(&cfg.DataDir, "PGEDGE_DATA_DIR")

	// Trace file
	setStringFromEnv(&cfg.TraceFile, "PGEDGE_TRACE_FILE")

	// Note: Builtins (tools, resources, prompts) are only configurable via
	// config file, not environment variables
}

// applyCLIFlags overrides config with CLI flags if they were explicitly set
func applyCLIFlags(cfg *Config, flags CLIFlags) error {
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
	if flags.AuthUserSet {
		cfg.HTTP.Auth.UserFile = flags.AuthUserFile
	}

	// Database CLI flags apply to the first database in the list
	// Create a default database if none exists and any DB flag is set
	if len(cfg.Databases) == 0 && (flags.DBHostSet || flags.DBPortSet || flags.DBNameSet || flags.DBUserSet || flags.DBPassSet || flags.DBSSLSet || flags.DBHostsSet || flags.DBTargetSessionAttrsSet) {
		cfg.Databases = []NamedDatabaseConfig{{
			Name:                "default",
			Host:                "localhost",
			Port:                5432,
			Database:            "postgres",
			SSLMode:             "prefer",
			PoolMaxConns:        4,
			PoolMinConns:        0,
			PoolMaxConnIdleTime: "30m",
		}}
	}

	if len(cfg.Databases) > 0 {
		if flags.DBHostSet {
			cfg.Databases[0].Host = flags.DBHost
		}
		if flags.DBPortSet {
			cfg.Databases[0].Port = flags.DBPort
		}
		if flags.DBNameSet {
			cfg.Databases[0].Database = flags.DBName
		}
		if flags.DBUserSet {
			cfg.Databases[0].User = flags.DBUser
		}
		if flags.DBPassSet {
			cfg.Databases[0].Password = flags.DBPassword
		}
		if flags.DBSSLSet {
			cfg.Databases[0].SSLMode = flags.DBSSLMode
		}
		if flags.DBHostsSet {
			entries, err := ParseHostEntries(flags.DBHosts)
			if err != nil {
				return fmt.Errorf("invalid --db-hosts value: %w", err)
			}
			cfg.Databases[0].Hosts = entries
			cfg.Databases[0].Host = "" // Clear single host to avoid validation conflict
		}
		if flags.DBTargetSessionAttrsSet {
			cfg.Databases[0].TargetSessionAttrs = flags.DBTargetSessionAttrs
		}
	}

	// Secret file
	if flags.SecretFileSet {
		cfg.SecretFile = flags.SecretFile
	}

	// Trace file
	if flags.TraceFileSet {
		cfg.TraceFile = flags.TraceFile
	}

	return nil
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

	// Database configuration validation
	// Validate each database in the list
	seenNames := make(map[string]bool)
	for i := range cfg.Databases {
		db := &cfg.Databases[i]
		// Require name field
		if db.Name == "" {
			return fmt.Errorf("database at index %d: name is required", i)
		}

		// Check for duplicate names
		if seenNames[db.Name] {
			return fmt.Errorf("duplicate database name: %s", db.Name)
		}
		seenNames[db.Name] = true

		// Require user field
		if db.User == "" {
			return fmt.Errorf("database '%s': user is required (set via -db-user, PGEDGE_DB_USER, PGUSER env var, or config file)", db.Name)
		}

		// Validate multi-host settings
		if err := db.Validate(); err != nil {
			return err
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
// Searches /etc/pgedge/ first, then binary directory
func GetDefaultConfigPath(binaryPath string) string {
	systemPath := "/etc/pgedge/postgres-mcp.yaml"
	if _, err := os.Stat(systemPath); err == nil {
		return systemPath
	}

	dir := filepath.Dir(binaryPath)
	return filepath.Join(dir, "postgres-mcp.yaml")
}

// GetDefaultSecretPath returns the default secret file path
// Searches /etc/pgedge/ first, then binary directory
func GetDefaultSecretPath(binaryPath string) string {
	systemPath := "/etc/pgedge/postgres-mcp.secret"
	if _, err := os.Stat(systemPath); err == nil {
		return systemPath
	}

	dir := filepath.Dir(binaryPath)
	return filepath.Join(dir, "postgres-mcp.secret")
}

// GetDatabaseByName returns the named database config or nil if not found
func (cfg *Config) GetDatabaseByName(name string) *NamedDatabaseConfig {
	for i := range cfg.Databases {
		if cfg.Databases[i].Name == name {
			return &cfg.Databases[i]
		}
	}
	return nil
}

// GetDefaultDatabaseName returns the name of the first database in the list
// Returns empty string if no databases are configured
func (cfg *Config) GetDefaultDatabaseName() string {
	if len(cfg.Databases) > 0 {
		return cfg.Databases[0].Name
	}
	return ""
}

// GetDatabasesForUser returns databases accessible to a username
// A database is accessible if its AvailableToUsers list is empty (all users)
// or if the username is in the list
func (cfg *Config) GetDatabasesForUser(username string) []NamedDatabaseConfig {
	var result []NamedDatabaseConfig
	for i := range cfg.Databases {
		db := &cfg.Databases[i]
		// Empty AvailableToUsers means accessible to all users
		if len(db.AvailableToUsers) == 0 {
			result = append(result, *db)
			continue
		}
		// Check if user is in the allowed list
		for _, allowedUser := range db.AvailableToUsers {
			if allowedUser == username {
				result = append(result, *db)
				break
			}
		}
	}
	return result
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

	// Create directory if it doesn't exist (owner only for security)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write with restrictive permissions (owner read/write only)
	// Config files may contain sensitive information like database passwords
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
