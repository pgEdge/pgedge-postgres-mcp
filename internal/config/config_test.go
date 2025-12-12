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
	"os"
	"path/filepath"
	"testing"
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
			expected: "postgres://admin:p@ssw0rd@db.example.com:5433/production?sslmode=verify-full",
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
		{"database_schema nil", ResourcesConfig{}, "pg://database/schema", true},
		{"database_schema explicit false", ResourcesConfig{DatabaseSchema: &falseVal}, "pg://database/schema", false},
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
			name: "HTTP auth without token file",
			config: &Config{
				HTTP: HTTPConfig{
					Enabled: true,
					Auth:    AuthConfig{Enabled: true, TokenFile: ""},
				},
			},
			expectError: true,
			errorMsg:    "authentication token file is required",
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

	applyCLIFlags(cfg, flags)

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
