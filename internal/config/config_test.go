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
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := defaultConfig()

	// Test HTTP defaults
	if cfg.HTTP.Enabled {
		t.Error("Expected HTTP to be disabled by default")
	}

	if cfg.HTTP.Address != ":8080" {
		t.Errorf("Expected default address ':8080', got %s", cfg.HTTP.Address)
	}

	if cfg.HTTP.TLS.Enabled {
		t.Error("Expected TLS to be disabled by default")
	}

	if !cfg.HTTP.Auth.Enabled {
		t.Error("Expected Auth to be enabled by default")
	}

	// Test embedding defaults
	if cfg.Embedding.Enabled {
		t.Error("Expected embedding to be disabled by default")
	}
	if cfg.Embedding.Provider != "ollama" {
		t.Errorf("Expected default embedding provider 'ollama', got %s", cfg.Embedding.Provider)
	}

	// Test LLM defaults
	if cfg.LLM.Enabled {
		t.Error("Expected LLM to be disabled by default")
	}
	if cfg.LLM.MaxTokens != 4096 {
		t.Errorf("Expected default max tokens 4096, got %d", cfg.LLM.MaxTokens)
	}
	if cfg.LLM.Temperature != 0.7 {
		t.Errorf("Expected default temperature 0.7, got %f", cfg.LLM.Temperature)
	}

	// Test knowledgebase defaults
	if cfg.Knowledgebase.Enabled {
		t.Error("Expected knowledgebase to be disabled by default")
	}

	// Test rate limiting defaults
	if cfg.HTTP.Auth.RateLimitWindowMinutes != 15 {
		t.Errorf("Expected rate limit window 15 minutes, got %d", cfg.HTTP.Auth.RateLimitWindowMinutes)
	}
	if cfg.HTTP.Auth.RateLimitMaxAttempts != 10 {
		t.Errorf("Expected rate limit max attempts 10, got %d", cfg.HTTP.Auth.RateLimitMaxAttempts)
	}
}

func TestBuildConnectionString(t *testing.T) {
	tests := []struct {
		name     string
		config   NamedDatabaseConfig
		expected string
	}{
		{
			name: "basic connection",
			config: NamedDatabaseConfig{
				User:     "postgres",
				Host:     "localhost",
				Port:     5432,
				Database: "testdb",
			},
			expected: "postgres://postgres@localhost:5432/testdb",
		},
		{
			name: "with password",
			config: NamedDatabaseConfig{
				User:     "postgres",
				Password: "secret123",
				Host:     "localhost",
				Port:     5432,
				Database: "testdb",
			},
			expected: "postgres://postgres:secret123@localhost:5432/testdb",
		},
		{
			name: "with sslmode",
			config: NamedDatabaseConfig{
				User:     "postgres",
				Host:     "localhost",
				Port:     5432,
				Database: "testdb",
				SSLMode:  "require",
			},
			expected: "postgres://postgres@localhost:5432/testdb?sslmode=require",
		},
		{
			name: "full configuration",
			config: NamedDatabaseConfig{
				User:     "admin",
				Password: "p@ssw0rd",
				Host:     "db.example.com",
				Port:     5433,
				Database: "production",
				SSLMode:  "verify-full",
			},
			expected: "postgres://admin:p%40ssw0rd@db.example.com:5433/production?sslmode=verify-full",
		},
		{
			name: "special characters in user and password are URL-encoded",
			config: NamedDatabaseConfig{
				User:     "user:name",
				Password: "p@ss:word/123#test",
				Host:     "localhost",
				Port:     5432,
				Database: "testdb",
			},
			expected: "postgres://user%3Aname:p%40ss%3Aword%2F123%23test@localhost:5432/testdb",
		},
		{
			name: "multi-host with hosts list",
			config: NamedDatabaseConfig{
				User:     "postgres",
				Database: "mydb",
				SSLMode:  "require",
				Hosts: []HostEntry{
					{Host: "primary.example.com", Port: 5432},
					{Host: "replica.example.com", Port: 5433},
				},
			},
			expected: "postgres://postgres@primary.example.com:5432,replica.example.com:5433/mydb?sslmode=require",
		},
		{
			name: "multi-host with target_session_attrs",
			config: NamedDatabaseConfig{
				User:     "postgres",
				Database: "mydb",
				Hosts: []HostEntry{
					{Host: "node1.example.com", Port: 5432},
					{Host: "node2.example.com", Port: 5432},
				},
				TargetSessionAttrs: "read-write",
			},
			expected: "postgres://postgres@node1.example.com:5432,node2.example.com:5432/mydb?target_session_attrs=read-write",
		},
		{
			name: "hosts list with single entry behaves like host/port",
			config: NamedDatabaseConfig{
				User:     "postgres",
				Database: "mydb",
				Hosts: []HostEntry{
					{Host: "single.example.com", Port: 5432},
				},
			},
			expected: "postgres://postgres@single.example.com:5432/mydb",
		},
		{
			name: "hosts list with password containing special chars",
			config: NamedDatabaseConfig{
				User:     "admin",
				Password: "p@ss:word",
				Database: "mydb",
				Hosts: []HostEntry{
					{Host: "h1.example.com", Port: 5432},
					{Host: "h2.example.com", Port: 5433},
				},
				SSLMode: "verify-full",
			},
			expected: "postgres://admin:p%40ss%3Aword@h1.example.com:5432,h2.example.com:5433/mydb?sslmode=verify-full",
		},
		{
			name: "IPv6 single host is bracketed",
			config: NamedDatabaseConfig{
				User:     "postgres",
				Host:     "2001:db8::1",
				Port:     5432,
				Database: "mydb",
			},
			expected: "postgres://postgres@[2001:db8::1]:5432/mydb",
		},
		{
			name: "IPv6 multi-host entries are bracketed",
			config: NamedDatabaseConfig{
				User:     "postgres",
				Database: "mydb",
				Hosts: []HostEntry{
					{Host: "2001:db8::1", Port: 5432},
					{Host: "::1", Port: 5433},
				},
				TargetSessionAttrs: "read-write",
			},
			expected: "postgres://postgres@[2001:db8::1]:5432,[::1]:5433/mydb?target_session_attrs=read-write",
		},
		{
			name: "with connect_timeout",
			config: NamedDatabaseConfig{
				User:           "postgres",
				Host:           "localhost",
				Port:           5432,
				Database:       "testdb",
				ConnectTimeout: "15s",
			},
			expected: "postgres://postgres@localhost:5432/testdb?connect_timeout=15",
		},
		{
			name: "connect_timeout with sslmode",
			config: NamedDatabaseConfig{
				User:           "postgres",
				Host:           "localhost",
				Port:           5432,
				Database:       "testdb",
				SSLMode:        "require",
				ConnectTimeout: "30s",
			},
			expected: "postgres://postgres@localhost:5432/testdb?connect_timeout=30&sslmode=require",
		},
		{
			name: "connect_timeout with sub-second duration rounds down",
			config: NamedDatabaseConfig{
				User:           "postgres",
				Host:           "localhost",
				Port:           5432,
				Database:       "testdb",
				ConnectTimeout: "5500ms",
			},
			expected: "postgres://postgres@localhost:5432/testdb?connect_timeout=5",
		},
		{
			name: "connect_timeout invalid duration is ignored",
			config: NamedDatabaseConfig{
				User:           "postgres",
				Host:           "localhost",
				Port:           5432,
				Database:       "testdb",
				ConnectTimeout: "not-a-duration",
			},
			expected: "postgres://postgres@localhost:5432/testdb",
		},
		{
			name: "with sslcert and sslkey",
			config: NamedDatabaseConfig{
				User:     "postgres",
				Host:     "localhost",
				Port:     5432,
				Database: "testdb",
				SSLCert:  "/certs/client.crt",
				SSLKey:   "/certs/client.key",
			},
			expected: "postgres://postgres@localhost:5432/testdb?sslcert=%2Fcerts%2Fclient.crt&sslkey=%2Fcerts%2Fclient.key",
		},
		{
			name: "with sslrootcert alone",
			config: NamedDatabaseConfig{
				User:        "postgres",
				Host:        "localhost",
				Port:        5432,
				Database:    "testdb",
				SSLRootCert: "/certs/ca.crt",
			},
			expected: "postgres://postgres@localhost:5432/testdb?sslrootcert=%2Fcerts%2Fca.crt",
		},
		{
			name: "with sslmode, sslcert, sslkey, and sslrootcert combined",
			config: NamedDatabaseConfig{
				User:        "postgres",
				Host:        "localhost",
				Port:        5432,
				Database:    "testdb",
				SSLMode:     "verify-full",
				SSLCert:     "/certs/client.crt",
				SSLKey:      "/certs/client.key",
				SSLRootCert: "/certs/ca.crt",
			},
			expected: "postgres://postgres@localhost:5432/testdb?sslcert=%2Fcerts%2Fclient.crt&sslkey=%2Fcerts%2Fclient.key&sslmode=verify-full&sslrootcert=%2Fcerts%2Fca.crt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.BuildConnectionString()
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestToolsConfig_IsToolEnabled(t *testing.T) {
	falseVal := false
	trueVal := true

	tests := []struct {
		name     string
		config   ToolsConfig
		toolName string
		expected bool
	}{
		{"nil value returns true", ToolsConfig{}, "query_database", true},
		{"explicit true", ToolsConfig{QueryDatabase: &trueVal}, "query_database", true},
		{"explicit false", ToolsConfig{QueryDatabase: &falseVal}, "query_database", false},
		{"unknown tool returns true", ToolsConfig{}, "unknown_tool", true},
		{"get_schema_info nil", ToolsConfig{}, "get_schema_info", true},
		{"similarity_search nil", ToolsConfig{}, "similarity_search", true},
		{"execute_explain nil", ToolsConfig{}, "execute_explain", true},
		{"generate_embedding nil", ToolsConfig{}, "generate_embedding", true},
		{"search_knowledgebase nil", ToolsConfig{}, "search_knowledgebase", true},
		{"count_rows nil", ToolsConfig{}, "count_rows", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.IsToolEnabled(tt.toolName)
			if result != tt.expected {
				t.Errorf("IsToolEnabled(%q): expected %v, got %v", tt.toolName, tt.expected, result)
			}
		})
	}
}

func TestResourcesConfig_IsResourceEnabled(t *testing.T) {
	falseVal := false
	trueVal := true

	tests := []struct {
		name        string
		config      ResourcesConfig
		resourceURI string
		expected    bool
	}{
		{"nil value returns true", ResourcesConfig{}, "pg://system_info", true},
		{"explicit true", ResourcesConfig{SystemInfo: &trueVal}, "pg://system_info", true},
		{"explicit false", ResourcesConfig{SystemInfo: &falseVal}, "pg://system_info", false},
		{"unknown resource returns true", ResourcesConfig{}, "pg://unknown", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.IsResourceEnabled(tt.resourceURI)
			if result != tt.expected {
				t.Errorf("IsResourceEnabled(%q): expected %v, got %v", tt.resourceURI, tt.expected, result)
			}
		})
	}
}

func TestPromptsConfig_IsPromptEnabled(t *testing.T) {
	falseVal := false
	trueVal := true

	tests := []struct {
		name       string
		config     PromptsConfig
		promptName string
		expected   bool
	}{
		{"nil value returns true", PromptsConfig{}, "explore-database", true},
		{"explicit true", PromptsConfig{ExploreDatabase: &trueVal}, "explore-database", true},
		{"explicit false", PromptsConfig{ExploreDatabase: &falseVal}, "explore-database", false},
		{"unknown prompt returns true", PromptsConfig{}, "unknown-prompt", true},
		{"setup-semantic-search nil", PromptsConfig{}, "setup-semantic-search", true},
		{"diagnose-query-issue nil", PromptsConfig{}, "diagnose-query-issue", true},
		{"design-schema nil", PromptsConfig{}, "design-schema", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.IsPromptEnabled(tt.promptName)
			if result != tt.expected {
				t.Errorf("IsPromptEnabled(%q): expected %v, got %v", tt.promptName, tt.expected, result)
			}
		})
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config",
			config: &Config{
				HTTP: HTTPConfig{Enabled: false},
			},
			expectError: false,
		},
		{
			name: "TLS without HTTP",
			config: &Config{
				HTTP: HTTPConfig{
					Enabled: false,
					TLS:     TLSConfig{Enabled: true},
				},
			},
			expectError: true,
			errorMsg:    "TLS requires HTTP mode",
		},
		{
			name: "TLS without cert file",
			config: &Config{
				HTTP: HTTPConfig{
					Enabled: true,
					TLS:     TLSConfig{Enabled: true, KeyFile: "key.pem"},
				},
			},
			expectError: true,
			errorMsg:    "certificate file is required",
		},
		{
			name: "TLS without key file",
			config: &Config{
				HTTP: HTTPConfig{
					Enabled: true,
					TLS:     TLSConfig{Enabled: true, CertFile: "cert.pem"},
				},
			},
			expectError: true,
			errorMsg:    "key file is required",
		},
		{
			name: "duplicate database names",
			config: &Config{
				HTTP: HTTPConfig{Enabled: false},
				Databases: []NamedDatabaseConfig{
					{Name: "db1", User: "user1"},
					{Name: "db1", User: "user2"},
				},
			},
			expectError: true,
			errorMsg:    "duplicate database name",
		},
		{
			name: "database without name",
			config: &Config{
				HTTP: HTTPConfig{Enabled: false},
				Databases: []NamedDatabaseConfig{
					{Name: "", User: "user1"},
				},
			},
			expectError: true,
			errorMsg:    "name is required",
		},
		{
			name: "database without user",
			config: &Config{
				HTTP: HTTPConfig{Enabled: false},
				Databases: []NamedDatabaseConfig{
					{Name: "db1", User: ""},
				},
			},
			expectError: true,
			errorMsg:    "user is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.config)
			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestGetDatabaseByName(t *testing.T) {
	cfg := &Config{
		Databases: []NamedDatabaseConfig{
			{Name: "db1", Host: "host1"},
			{Name: "db2", Host: "host2"},
		},
	}

	// Test finding existing database
	db := cfg.GetDatabaseByName("db1")
	if db == nil {
		t.Fatal("expected to find db1")
	}
	if db.Host != "host1" {
		t.Errorf("expected host 'host1', got %q", db.Host)
	}

	// Test non-existent database
	db = cfg.GetDatabaseByName("nonexistent")
	if db != nil {
		t.Error("expected nil for non-existent database")
	}
}

func TestGetDefaultDatabaseName(t *testing.T) {
	// Test with databases
	cfg := &Config{
		Databases: []NamedDatabaseConfig{
			{Name: "primary"},
			{Name: "secondary"},
		},
	}
	name := cfg.GetDefaultDatabaseName()
	if name != "primary" {
		t.Errorf("expected 'primary', got %q", name)
	}

	// Test without databases
	cfg = &Config{Databases: []NamedDatabaseConfig{}}
	name = cfg.GetDefaultDatabaseName()
	if name != "" {
		t.Errorf("expected empty string, got %q", name)
	}
}

func TestGetDatabasesForUser(t *testing.T) {
	cfg := &Config{
		Databases: []NamedDatabaseConfig{
			{Name: "public", AvailableToUsers: []string{}},                   // Available to all
			{Name: "restricted", AvailableToUsers: []string{"admin", "dev"}}, // Restricted
			{Name: "admin_only", AvailableToUsers: []string{"admin"}},        // Admin only
		},
	}

	// Test admin user (has access to all)
	dbs := cfg.GetDatabasesForUser("admin")
	if len(dbs) != 3 {
		t.Errorf("admin should have access to 3 databases, got %d", len(dbs))
	}

	// Test dev user
	dbs = cfg.GetDatabasesForUser("dev")
	if len(dbs) != 2 {
		t.Errorf("dev should have access to 2 databases, got %d", len(dbs))
	}

	// Test unknown user
	dbs = cfg.GetDatabasesForUser("unknown")
	if len(dbs) != 1 {
		t.Errorf("unknown user should have access to 1 database (public), got %d", len(dbs))
	}
}

func TestReadAPIKeyFromFile(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Test reading valid file
	keyFile := filepath.Join(tmpDir, "api_key.txt")
	if err := os.WriteFile(keyFile, []byte("  test-api-key-123  \n"), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	key, err := readAPIKeyFromFile(keyFile)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if key != "test-api-key-123" {
		t.Errorf("expected 'test-api-key-123', got %q", key)
	}

	// Test empty path
	key, err = readAPIKeyFromFile("")
	if err != nil {
		t.Errorf("unexpected error for empty path: %v", err)
	}
	if key != "" {
		t.Errorf("expected empty string for empty path, got %q", key)
	}

	// Test non-existent file (should return empty, not error)
	key, err = readAPIKeyFromFile(filepath.Join(tmpDir, "nonexistent.txt"))
	if err != nil {
		t.Errorf("unexpected error for non-existent file: %v", err)
	}
	if key != "" {
		t.Errorf("expected empty string for non-existent file, got %q", key)
	}
}

func TestConfigFileExists(t *testing.T) {
	tmpDir := t.TempDir()

	// Test existing file
	existingFile := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(existingFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	if !ConfigFileExists(existingFile) {
		t.Error("expected ConfigFileExists to return true for existing file")
	}

	// Test non-existent file
	if ConfigFileExists(filepath.Join(tmpDir, "nonexistent.yaml")) {
		t.Error("expected ConfigFileExists to return false for non-existent file")
	}
}

func TestSaveConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "subdir", "config.yaml")

	cfg := &Config{
		HTTP: HTTPConfig{
			Enabled: true,
			Address: ":9090",
		},
		Databases: []NamedDatabaseConfig{
			{Name: "test", Host: "localhost", Port: 5432, User: "testuser"},
		},
	}

	// Test saving config (should create directory)
	if err := SaveConfig(configPath, cfg); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	// Verify file exists
	if !ConfigFileExists(configPath) {
		t.Error("config file should exist after save")
	}

	// Load and verify
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read saved config: %v", err)
	}
	if len(data) == 0 {
		t.Error("saved config file is empty")
	}
}

func TestLoadConfigWithTempFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Create a minimal valid config file
	configContent := `
http:
    enabled: true
    address: ":9000"
    auth:
        enabled: false
databases:
    - name: testdb
      host: localhost
      port: 5432
      user: testuser
      database: test
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Load config
	flags := CLIFlags{ConfigFileSet: true, ConfigFile: configPath}
	cfg, err := LoadConfig(configPath, flags)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Verify loaded values
	if !cfg.HTTP.Enabled {
		t.Error("expected HTTP to be enabled")
	}
	if cfg.HTTP.Address != ":9000" {
		t.Errorf("expected address ':9000', got %q", cfg.HTTP.Address)
	}
	if len(cfg.Databases) != 1 {
		t.Fatalf("expected 1 database, got %d", len(cfg.Databases))
	}
	if cfg.Databases[0].Name != "testdb" {
		t.Errorf("expected database name 'testdb', got %q", cfg.Databases[0].Name)
	}
}

func TestLoadConfigNonExistentFile(t *testing.T) {
	// Test with ConfigFileSet=true (should error)
	flags := CLIFlags{ConfigFileSet: true, ConfigFile: "/nonexistent/config.yaml"}
	_, err := LoadConfig("/nonexistent/config.yaml", flags)
	if err == nil {
		t.Error("expected error for non-existent config file with ConfigFileSet=true")
	}

	// Test with ConfigFileSet=false (should use defaults)
	flags = CLIFlags{ConfigFileSet: false}
	cfg, err := LoadConfig("/nonexistent/config.yaml", flags)
	if err != nil {
		t.Errorf("unexpected error for non-existent config file with ConfigFileSet=false: %v", err)
	}
	if cfg == nil {
		t.Error("expected config to be returned")
	}
}

func TestGetDefaultConfigPath(t *testing.T) {
	// Test with a known binary path
	result := GetDefaultConfigPath("/usr/local/bin/pgedge-postgres-mcp")

	// If system path exists, it would return that instead
	// Just check that we get a .yaml file
	if filepath.Ext(result) != ".yaml" {
		t.Errorf("expected .yaml extension, got %q", result)
	}
}

func TestGetDefaultSecretPath(t *testing.T) {
	result := GetDefaultSecretPath("/usr/local/bin/pgedge-postgres-mcp")

	// If system path exists, it would return that instead
	// Just check that we get a .secret file
	if filepath.Ext(result) != ".secret" {
		t.Errorf("expected .secret extension, got %q", result)
	}
}

func TestMergeConfig(t *testing.T) {
	dest := defaultConfig()
	src := &Config{
		HTTP: HTTPConfig{
			Enabled: true,
			Address: ":9090",
		},
		Databases: []NamedDatabaseConfig{
			{Name: "newdb", Host: "newhost"},
		},
		SecretFile: "/new/secret",
	}

	mergeConfig(dest, src)

	if !dest.HTTP.Enabled {
		t.Error("expected HTTP.Enabled to be merged")
	}
	if dest.HTTP.Address != ":9090" {
		t.Errorf("expected address ':9090', got %q", dest.HTTP.Address)
	}
	if len(dest.Databases) != 1 || dest.Databases[0].Name != "newdb" {
		t.Error("expected databases to be merged")
	}
	if dest.SecretFile != "/new/secret" {
		t.Errorf("expected SecretFile '/new/secret', got %q", dest.SecretFile)
	}
}

func TestMergePerAttemptTimeout(t *testing.T) {
	dest := defaultConfig()
	src := &Config{
		LLM: LLMConfig{
			Provider:          "anthropic",
			PerAttemptTimeout: 25,
		},
		Embedding: EmbeddingConfig{
			Provider:          "voyage",
			PerAttemptTimeout: 12,
		},
		Knowledgebase: KnowledgebaseConfig{
			Enabled:                    true,
			EmbeddingProvider:          "voyage",
			EmbeddingPerAttemptTimeout: 18,
		},
	}

	mergeConfig(dest, src)

	if dest.LLM.PerAttemptTimeout != 25 {
		t.Errorf("LLM.PerAttemptTimeout = %d, want 25", dest.LLM.PerAttemptTimeout)
	}
	if dest.Embedding.PerAttemptTimeout != 12 {
		t.Errorf("Embedding.PerAttemptTimeout = %d, want 12", dest.Embedding.PerAttemptTimeout)
	}
	if dest.Knowledgebase.EmbeddingPerAttemptTimeout != 18 {
		t.Errorf("Knowledgebase.EmbeddingPerAttemptTimeout = %d, want 18", dest.Knowledgebase.EmbeddingPerAttemptTimeout)
	}
}

func TestApplyCLIFlags(t *testing.T) {
	cfg := defaultConfig()
	flags := CLIFlags{
		HTTPEnabledSet: true,
		HTTPEnabled:    true,
		HTTPAddrSet:    true,
		HTTPAddr:       ":7070",
		DBUserSet:      true,
		DBUser:         "cliuser",
	}

	if err := applyCLIFlags(cfg, flags); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cfg.HTTP.Enabled {
		t.Error("expected HTTP.Enabled to be set from CLI")
	}
	if cfg.HTTP.Address != ":7070" {
		t.Errorf("expected address ':7070', got %q", cfg.HTTP.Address)
	}
	// Database should be created when DB flags are set
	if len(cfg.Databases) != 1 {
		t.Fatalf("expected 1 database to be created, got %d", len(cfg.Databases))
	}
	if cfg.Databases[0].User != "cliuser" {
		t.Errorf("expected user 'cliuser', got %q", cfg.Databases[0].User)
	}
}

func TestApplyCLIFlags_SSLCerts(t *testing.T) {
	cfg := defaultConfig()
	flags := CLIFlags{
		DBSSLCertSet:     true,
		DBSSLCert:        "/cli/client.crt",
		DBSSLKeySet:      true,
		DBSSLKey:         "/cli/client.key",
		DBSSLRootCertSet: true,
		DBSSLRootCert:    "/cli/ca.crt",
	}

	if err := applyCLIFlags(cfg, flags); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// A lone SSL cert flag (with no other DB flags set) should still
	// trigger creation of the default database, just like DBUserSet does.
	if len(cfg.Databases) != 1 {
		t.Fatalf("expected 1 database to be created, got %d", len(cfg.Databases))
	}
	db := cfg.Databases[0]
	if db.SSLCert != "/cli/client.crt" {
		t.Errorf("SSLCert = %q, want /cli/client.crt", db.SSLCert)
	}
	if db.SSLKey != "/cli/client.key" {
		t.Errorf("SSLKey = %q, want /cli/client.key", db.SSLKey)
	}
	if db.SSLRootCert != "/cli/ca.crt" {
		t.Errorf("SSLRootCert = %q, want /cli/ca.crt", db.SSLRootCert)
	}
}

func TestApplyCLIFlags_SSLCertsOverrideConfigFile(t *testing.T) {
	cfg := defaultConfig()
	cfg.Databases = []NamedDatabaseConfig{{
		Name: "default", User: "postgres",
		SSLCert: "/configured/client.crt",
	}}
	flags := CLIFlags{
		DBSSLCertSet: true,
		DBSSLCert:    "/cli/client.crt",
	}

	if err := applyCLIFlags(cfg, flags); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := cfg.Databases[0].SSLCert; got != "/cli/client.crt" {
		t.Errorf("SSLCert = %q, want /cli/client.crt (CLI flag should override config file)", got)
	}
}

func TestSetStringFromEnv(t *testing.T) {
	os.Setenv("TEST_STRING_VAR", "test_value")
	defer os.Unsetenv("TEST_STRING_VAR")

	var dest string
	setStringFromEnv(&dest, "TEST_STRING_VAR")

	if dest != "test_value" {
		t.Errorf("expected 'test_value', got %q", dest)
	}

	// Test with non-existent var
	dest = "original"
	setStringFromEnv(&dest, "NONEXISTENT_VAR")
	if dest != "original" {
		t.Errorf("expected 'original' (unchanged), got %q", dest)
	}
}

func TestSetBoolFromEnv(t *testing.T) {
	tests := []struct {
		envValue string
		expected bool
	}{
		{"true", true},
		{"1", true},
		{"yes", true},
		{"false", false},
		{"0", false},
		{"no", false},
	}

	for _, tt := range tests {
		os.Setenv("TEST_BOOL_VAR", tt.envValue)
		var dest bool
		setBoolFromEnv(&dest, "TEST_BOOL_VAR")
		if dest != tt.expected {
			t.Errorf("setBoolFromEnv with %q: expected %v, got %v", tt.envValue, tt.expected, dest)
		}
	}
	os.Unsetenv("TEST_BOOL_VAR")
}

func TestSetBoolPtrFromEnv(t *testing.T) {
	const key = "TEST_BOOL_PTR_VAR"

	t.Run("unset leaves nil pointer", func(t *testing.T) {
		os.Unsetenv(key)
		var dest *bool
		setBoolPtrFromEnv(&dest, key)
		if dest != nil {
			t.Errorf("expected dest to remain nil when env var unset, got %v", *dest)
		}
	})

	truthy := []string{"true", "TRUE", "True", "1", "yes", "YES"}
	for _, v := range truthy {
		t.Run("truthy_"+v, func(t *testing.T) {
			os.Setenv(key, v)
			defer os.Unsetenv(key)
			var dest *bool
			setBoolPtrFromEnv(&dest, key)
			if dest == nil {
				t.Fatalf("expected non-nil pointer for value %q", v)
			}
			if !*dest {
				t.Errorf("expected true for value %q, got false", v)
			}
		})
	}

	falsy := []string{"false", "0", "no", "anything-else"}
	for _, v := range falsy {
		t.Run("falsy_"+v, func(t *testing.T) {
			os.Setenv(key, v)
			defer os.Unsetenv(key)
			var dest *bool
			setBoolPtrFromEnv(&dest, key)
			if dest == nil {
				t.Fatalf("expected non-nil pointer for value %q", v)
			}
			if *dest {
				t.Errorf("expected false for value %q, got true", v)
			}
		})
	}

	t.Run("overrides existing pointer", func(t *testing.T) {
		os.Setenv(key, "false")
		defer os.Unsetenv(key)
		existing := true
		dest := &existing
		setBoolPtrFromEnv(&dest, key)
		if dest == nil {
			t.Fatal("expected non-nil pointer")
		}
		if *dest {
			t.Errorf("expected false (override), got true")
		}
	})

	t.Run("unrecognised value is treated as false", func(t *testing.T) {
		os.Setenv(key, "enabled")
		defer os.Unsetenv(key)
		var dest *bool
		setBoolPtrFromEnv(&dest, key)
		if dest == nil {
			t.Fatal("expected non-nil pointer for unrecognised value")
		}
		if *dest {
			t.Errorf("expected false for unrecognised value, got true")
		}
	})
}

func TestApplyEnvironmentVariables_Builtins(t *testing.T) {
	envVars := []string{
		"PGEDGE_BUILTIN_TOOL_QUERY_DATABASE",
		"PGEDGE_BUILTIN_TOOL_GET_SCHEMA_INFO",
		"PGEDGE_BUILTIN_TOOL_SIMILARITY_SEARCH",
		"PGEDGE_BUILTIN_TOOL_EXECUTE_EXPLAIN",
		"PGEDGE_BUILTIN_TOOL_GENERATE_EMBEDDING",
		"PGEDGE_BUILTIN_TOOL_SEARCH_KNOWLEDGEBASE",
		"PGEDGE_BUILTIN_TOOL_COUNT_ROWS",
		"PGEDGE_BUILTIN_TOOL_LLM_CONNECTION_SELECTION",
		"PGEDGE_BUILTIN_RESOURCE_SYSTEM_INFO",
		"PGEDGE_BUILTIN_PROMPT_EXPLORE_DATABASE",
		"PGEDGE_BUILTIN_PROMPT_SETUP_SEMANTIC_SEARCH",
		"PGEDGE_BUILTIN_PROMPT_DIAGNOSE_QUERY_ISSUE",
		"PGEDGE_BUILTIN_PROMPT_DESIGN_SCHEMA",
	}
	for _, k := range envVars {
		t.Setenv(k, "false")
	}

	cfg := defaultConfig()
	applyEnvironmentVariables(cfg)

	checks := []struct {
		name string
		ptr  *bool
	}{
		{"QueryDatabase", cfg.Builtins.Tools.QueryDatabase},
		{"GetSchemaInfo", cfg.Builtins.Tools.GetSchemaInfo},
		{"SimilaritySearch", cfg.Builtins.Tools.SimilaritySearch},
		{"ExecuteExplain", cfg.Builtins.Tools.ExecuteExplain},
		{"GenerateEmbedding", cfg.Builtins.Tools.GenerateEmbedding},
		{"SearchKnowledgebase", cfg.Builtins.Tools.SearchKnowledgebase},
		{"CountRows", cfg.Builtins.Tools.CountRows},
		{"LLMConnectionSelection", cfg.Builtins.Tools.LLMConnectionSelection},
		{"SystemInfo", cfg.Builtins.Resources.SystemInfo},
		{"ExploreDatabase", cfg.Builtins.Prompts.ExploreDatabase},
		{"SetupSemanticSearch", cfg.Builtins.Prompts.SetupSemanticSearch},
		{"DiagnoseQueryIssue", cfg.Builtins.Prompts.DiagnoseQueryIssue},
		{"DesignSchema", cfg.Builtins.Prompts.DesignSchema},
	}
	for _, c := range checks {
		if c.ptr == nil {
			t.Errorf("%s: expected non-nil pointer after env var override", c.name)
			continue
		}
		if *c.ptr {
			t.Errorf("%s: expected false after env var override, got true", c.name)
		}
	}
}

func TestApplyEnvironmentVariables_DatabaseSSLCerts(t *testing.T) {
	t.Run("PGEDGE_DB_ vars are applied", func(t *testing.T) {
		t.Setenv("PGEDGE_DB_SSLCERT", "/pgedge/client.crt")
		t.Setenv("PGEDGE_DB_SSLKEY", "/pgedge/client.key")
		t.Setenv("PGEDGE_DB_SSLROOTCERT", "/pgedge/ca.crt")

		cfg := defaultConfig()
		cfg.Databases = []NamedDatabaseConfig{{Name: "default", User: "postgres"}}
		applyEnvironmentVariables(cfg)

		db := cfg.Databases[0]
		if db.SSLCert != "/pgedge/client.crt" {
			t.Errorf("SSLCert = %q, want /pgedge/client.crt", db.SSLCert)
		}
		if db.SSLKey != "/pgedge/client.key" {
			t.Errorf("SSLKey = %q, want /pgedge/client.key", db.SSLKey)
		}
		if db.SSLRootCert != "/pgedge/ca.crt" {
			t.Errorf("SSLRootCert = %q, want /pgedge/ca.crt", db.SSLRootCert)
		}
	})

	t.Run("libpq-standard PG vars are used as fallback", func(t *testing.T) {
		t.Setenv("PGSSLCERT", "/libpq/client.crt")
		t.Setenv("PGSSLKEY", "/libpq/client.key")
		t.Setenv("PGSSLROOTCERT", "/libpq/ca.crt")

		cfg := defaultConfig()
		cfg.Databases = []NamedDatabaseConfig{{Name: "default", User: "postgres"}}
		applyEnvironmentVariables(cfg)

		db := cfg.Databases[0]
		if db.SSLCert != "/libpq/client.crt" {
			t.Errorf("SSLCert = %q, want /libpq/client.crt", db.SSLCert)
		}
		if db.SSLKey != "/libpq/client.key" {
			t.Errorf("SSLKey = %q, want /libpq/client.key", db.SSLKey)
		}
		if db.SSLRootCert != "/libpq/ca.crt" {
			t.Errorf("SSLRootCert = %q, want /libpq/ca.crt", db.SSLRootCert)
		}
	})

	t.Run("PGEDGE_DB_ vars take precedence over PG vars", func(t *testing.T) {
		t.Setenv("PGEDGE_DB_SSLCERT", "/pgedge/client.crt")
		t.Setenv("PGSSLCERT", "/libpq/client.crt")

		cfg := defaultConfig()
		cfg.Databases = []NamedDatabaseConfig{{Name: "default", User: "postgres"}}
		applyEnvironmentVariables(cfg)

		if got := cfg.Databases[0].SSLCert; got != "/pgedge/client.crt" {
			t.Errorf("SSLCert = %q, want /pgedge/client.crt (PGEDGE_DB_SSLCERT should win)", got)
		}
	})

	t.Run("config file value is not overridden when no env vars are set", func(t *testing.T) {
		cfg := defaultConfig()
		cfg.Databases = []NamedDatabaseConfig{{
			Name: "default", User: "postgres",
			SSLCert: "/configured/client.crt",
		}}
		applyEnvironmentVariables(cfg)

		if got := cfg.Databases[0].SSLCert; got != "/configured/client.crt" {
			t.Errorf("SSLCert = %q, want /configured/client.crt (should be left untouched)", got)
		}
	})
}

func TestParseHostEntries(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []HostEntry
		wantErr bool
		errMsg  string
	}{
		{
			name:  "two hosts with ports",
			input: "h1:5432,h2:5433",
			want: []HostEntry{
				{Host: "h1", Port: 5432},
				{Host: "h2", Port: 5433},
			},
		},
		{
			name:  "hosts without ports default to 5432",
			input: "h1,h2",
			want: []HostEntry{
				{Host: "h1", Port: 5432},
				{Host: "h2", Port: 5432},
			},
		},
		{
			name:  "single host with port",
			input: "myhost:5433",
			want: []HostEntry{
				{Host: "myhost", Port: 5433},
			},
		},
		{
			name:  "whitespace trimmed",
			input: " h1:5432 , h2:5433 ",
			want: []HostEntry{
				{Host: "h1", Port: 5432},
				{Host: "h2", Port: 5433},
			},
		},
		{
			name:    "invalid port",
			input:   "h1:abc",
			wantErr: true,
		},
		{
			name:    "port out of range",
			input:   "host1:99999",
			wantErr: true,
			errMsg:  "out of range",
		},
		{
			name:    "port zero",
			input:   "host1:0",
			wantErr: true,
			errMsg:  "out of range",
		},
		{
			name:    "negative port",
			input:   "host1:-1",
			wantErr: true,
		},
		{
			name:  "empty entries skipped",
			input: "h1:5432,,h2:5433",
			want: []HostEntry{
				{Host: "h1", Port: 5432},
				{Host: "h2", Port: 5433},
			},
		},
		{
			name:  "bracketed IPv6 with port",
			input: "[2001:db8::1]:5432,[2001:db8::2]:5433",
			want: []HostEntry{
				{Host: "2001:db8::1", Port: 5432},
				{Host: "2001:db8::2", Port: 5433},
			},
		},
		{
			name:  "bracketed IPv6 without port",
			input: "[2001:db8::1]",
			want: []HostEntry{
				{Host: "2001:db8::1", Port: 5432},
			},
		},
		{
			name:  "unbracketed IPv6 uses default port",
			input: "2001:db8::1",
			want: []HostEntry{
				{Host: "2001:db8::1", Port: 5432},
			},
		},
		{
			name:  "mixed IPv4 and IPv6",
			input: "h1:5432,[::1]:5433,2001:db8::1",
			want: []HostEntry{
				{Host: "h1", Port: 5432},
				{Host: "::1", Port: 5433},
				{Host: "2001:db8::1", Port: 5432},
			},
		},
		{
			name:    "missing closing bracket",
			input:   "[2001:db8::1:5432",
			wantErr: true,
			errMsg:  "missing closing bracket",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseHostEntries(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errMsg != "" &&
					!strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error %q should contain %q",
						err.Error(), tt.errMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("expected %d entries, got %d", len(tt.want), len(got))
			}
			for i := range tt.want {
				if got[i].Host != tt.want[i].Host || got[i].Port != tt.want[i].Port {
					t.Errorf("entry %d: expected %+v, got %+v", i, tt.want[i], got[i])
				}
			}
		})
	}
}

func TestSetIntFromEnv(t *testing.T) {
	os.Setenv("TEST_INT_VAR", "42")
	defer os.Unsetenv("TEST_INT_VAR")

	var dest int
	setIntFromEnv(&dest, "TEST_INT_VAR")

	if dest != 42 {
		t.Errorf("expected 42, got %d", dest)
	}

	// Test with invalid value
	os.Setenv("TEST_INT_VAR", "not_a_number")
	dest = 0
	setIntFromEnv(&dest, "TEST_INT_VAR")
	if dest != 0 {
		t.Errorf("expected 0 for invalid int, got %d", dest)
	}
}

func TestPoolHealthSettings(t *testing.T) {
	cfg := NamedDatabaseConfig{
		PoolHealthCheckPeriod: "15s",
		PoolMaxConnLifetime:   "1h",
	}

	// Verify fields exist and are parseable as durations
	hcp, err := time.ParseDuration(cfg.PoolHealthCheckPeriod)
	if err != nil {
		t.Fatalf("PoolHealthCheckPeriod not parseable: %v", err)
	}
	if hcp != 15*time.Second {
		t.Errorf("expected 15s, got %v", hcp)
	}

	mcl, err := time.ParseDuration(cfg.PoolMaxConnLifetime)
	if err != nil {
		t.Fatalf("PoolMaxConnLifetime not parseable: %v", err)
	}
	if mcl != time.Hour {
		t.Errorf("expected 1h, got %v", mcl)
	}
}

func TestNamedDatabaseConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  NamedDatabaseConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid single host",
			config: NamedDatabaseConfig{
				Name: "db1", User: "postgres",
				Host: "localhost", Port: 5432,
				Database: "mydb",
			},
			wantErr: false,
		},
		{
			name: "valid multi-host",
			config: NamedDatabaseConfig{
				Name: "db1", User: "postgres",
				Database: "mydb",
				Hosts: []HostEntry{
					{Host: "h1", Port: 5432},
					{Host: "h2", Port: 5432},
				},
			},
			wantErr: false,
		},
		{
			name: "host and hosts both set",
			config: NamedDatabaseConfig{
				Name: "db1", User: "postgres",
				Host: "localhost", Port: 5432,
				Database: "mydb",
				Hosts: []HostEntry{
					{Host: "h1", Port: 5432},
				},
			},
			wantErr: true,
			errMsg:  "host",
		},
		{
			name: "target_session_attrs without hosts",
			config: NamedDatabaseConfig{
				Name: "db1", User: "postgres",
				Host: "localhost", Port: 5432,
				Database:           "mydb",
				TargetSessionAttrs: "read-write",
			},
			wantErr: true,
			errMsg:  "target_session_attrs",
		},
		{
			name: "invalid target_session_attrs value",
			config: NamedDatabaseConfig{
				Name: "db1", User: "postgres",
				Database: "mydb",
				Hosts: []HostEntry{
					{Host: "h1", Port: 5432},
				},
				TargetSessionAttrs: "invalid-value",
			},
			wantErr: true,
			errMsg:  "target_session_attrs",
		},
		{
			name: "valid target_session_attrs prefer-standby",
			config: NamedDatabaseConfig{
				Name: "db1", User: "postgres",
				Database: "mydb",
				Hosts: []HostEntry{
					{Host: "h1", Port: 5432},
					{Host: "h2", Port: 5432},
				},
				TargetSessionAttrs: "prefer-standby",
			},
			wantErr: false,
		},
		{
			name: "hosts entry missing host",
			config: NamedDatabaseConfig{
				Name: "db1", User: "postgres",
				Database: "mydb",
				Hosts: []HostEntry{
					{Host: "", Port: 5432},
				},
			},
			wantErr: true,
			errMsg:  "host",
		},
		{
			name: "port out of range in hosts",
			config: NamedDatabaseConfig{
				Name: "db1", User: "postgres",
				Database: "mydb",
				Hosts: []HostEntry{
					{Host: "h1", Port: 99999},
				},
			},
			wantErr: true,
			errMsg:  "invalid port",
		},
		{
			name: "hostname with comma in single host",
			config: NamedDatabaseConfig{
				Name: "db1", User: "postgres",
				Host: "bad,host", Port: 5432,
				Database: "mydb",
			},
			wantErr: true,
			errMsg:  "invalid hostname",
		},
		{
			name: "hostname with space in single host",
			config: NamedDatabaseConfig{
				Name: "db1", User: "postgres",
				Host: "bad host", Port: 5432,
				Database: "mydb",
			},
			wantErr: true,
			errMsg:  "invalid hostname",
		},
		{
			name: "hostname with comma in hosts entry",
			config: NamedDatabaseConfig{
				Name: "db1", User: "postgres",
				Database: "mydb",
				Hosts: []HostEntry{
					{Host: "good", Port: 5432},
					{Host: "bad,host", Port: 5432},
				},
			},
			wantErr: true,
			errMsg:  "invalid hostname",
		},
		{
			name: "hostname with at-sign",
			config: NamedDatabaseConfig{
				Name: "db1", User: "postgres",
				Host: "user@host", Port: 5432,
				Database: "mydb",
			},
			wantErr: true,
			errMsg:  "invalid hostname",
		},
		{
			name: "sslcert without sslkey",
			config: NamedDatabaseConfig{
				Name: "db1", User: "postgres",
				Host: "localhost", Port: 5432,
				Database: "mydb",
				SSLCert:  "/certs/client.crt",
			},
			wantErr: true,
			errMsg:  "sslcert",
		},
		{
			name: "sslkey without sslcert",
			config: NamedDatabaseConfig{
				Name: "db1", User: "postgres",
				Host: "localhost", Port: 5432,
				Database: "mydb",
				SSLKey:   "/certs/client.key",
			},
			wantErr: true,
			errMsg:  "sslkey",
		},
		{
			name: "sslcert and sslkey together",
			config: NamedDatabaseConfig{
				Name: "db1", User: "postgres",
				Host: "localhost", Port: 5432,
				Database: "mydb",
				SSLCert:  "/certs/client.crt",
				SSLKey:   "/certs/client.key",
			},
			wantErr: false,
		},
		// sslrootcert is threaded through verbatim under every sslmode,
		// mirroring libpq/psql: pgx consults it only under the verifying
		// modes and silently ignores it otherwise, so none of these are
		// rejected at config time.
		{
			name: "sslrootcert without an sslmode defaults to prefer and is accepted",
			config: NamedDatabaseConfig{
				Name: "db1", User: "postgres",
				Host: "localhost", Port: 5432,
				Database:    "mydb",
				SSLRootCert: "/certs/ca.crt",
			},
			wantErr: false,
		},
		{
			name: "sslrootcert with sslmode disable is accepted (ignored by pgx)",
			config: NamedDatabaseConfig{
				Name: "db1", User: "postgres",
				Host: "localhost", Port: 5432,
				Database:    "mydb",
				SSLMode:     "disable",
				SSLRootCert: "/certs/ca.crt",
			},
			wantErr: false,
		},
		{
			name: "sslrootcert with sslmode allow is accepted (ignored by pgx)",
			config: NamedDatabaseConfig{
				Name: "db1", User: "postgres",
				Host: "localhost", Port: 5432,
				Database:    "mydb",
				SSLMode:     "allow",
				SSLRootCert: "/certs/ca.crt",
			},
			wantErr: false,
		},
		{
			name: "sslrootcert with sslmode prefer is accepted (ignored by pgx)",
			config: NamedDatabaseConfig{
				Name: "db1", User: "postgres",
				Host: "localhost", Port: 5432,
				Database:    "mydb",
				SSLMode:     "prefer",
				SSLRootCert: "/certs/ca.crt",
			},
			wantErr: false,
		},
		{
			name: "sslrootcert with sslmode require is valid",
			config: NamedDatabaseConfig{
				Name: "db1", User: "postgres",
				Host: "localhost", Port: 5432,
				Database:    "mydb",
				SSLMode:     "require",
				SSLRootCert: "/certs/ca.crt",
			},
			wantErr: false,
		},
		{
			name: "sslrootcert with sslmode verify-ca is valid",
			config: NamedDatabaseConfig{
				Name: "db1", User: "postgres",
				Host: "localhost", Port: 5432,
				Database:    "mydb",
				SSLMode:     "verify-ca",
				SSLRootCert: "/certs/ca.crt",
			},
			wantErr: false,
		},
		{
			name: "sslrootcert with sslmode verify-full is valid",
			config: NamedDatabaseConfig{
				Name: "db1", User: "postgres",
				Host: "localhost", Port: 5432,
				Database:    "mydb",
				SSLMode:     "verify-full",
				SSLRootCert: "/certs/ca.crt",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errMsg != "" &&
					!strings.Contains(
						strings.ToLower(err.Error()),
						tt.errMsg,
					) {
					t.Errorf("error %q should contain %q",
						err.Error(), tt.errMsg)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestPerAttemptTimeoutDefaults(t *testing.T) {
	cfg := defaultConfig()
	if cfg.LLM.PerAttemptTimeout != 60 {
		t.Errorf("LLM.PerAttemptTimeout default = %d, want 60", cfg.LLM.PerAttemptTimeout)
	}
	if cfg.Embedding.PerAttemptTimeout != 60 {
		t.Errorf("Embedding.PerAttemptTimeout default = %d, want 60", cfg.Embedding.PerAttemptTimeout)
	}
	if cfg.Knowledgebase.EmbeddingPerAttemptTimeout != 60 {
		t.Errorf("Knowledgebase.EmbeddingPerAttemptTimeout default = %d, want 60", cfg.Knowledgebase.EmbeddingPerAttemptTimeout)
	}
}

func TestPerAttemptTimeoutFromEnv(t *testing.T) {
	t.Setenv("PGEDGE_LLM_PER_ATTEMPT_TIMEOUT", "30")
	t.Setenv("PGEDGE_EMBEDDING_PER_ATTEMPT_TIMEOUT", "15")
	t.Setenv("PGEDGE_KB_EMBEDDING_PER_ATTEMPT_TIMEOUT", "20")
	cfg := defaultConfig()
	applyEnvironmentVariables(cfg)
	if cfg.LLM.PerAttemptTimeout != 30 {
		t.Errorf("LLM.PerAttemptTimeout = %d, want 30", cfg.LLM.PerAttemptTimeout)
	}
	if cfg.Embedding.PerAttemptTimeout != 15 {
		t.Errorf("Embedding.PerAttemptTimeout = %d, want 15", cfg.Embedding.PerAttemptTimeout)
	}
	if cfg.Knowledgebase.EmbeddingPerAttemptTimeout != 20 {
		t.Errorf("Knowledgebase.EmbeddingPerAttemptTimeout = %d, want 20", cfg.Knowledgebase.EmbeddingPerAttemptTimeout)
	}
}
