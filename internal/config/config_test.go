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
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := defaultConfig()

	// Test defaults
	if cfg.Database.ConnectionString != "" {
		t.Errorf("Expected empty connection string, got %s", cfg.Database.ConnectionString)
	}

	if cfg.Anthropic.Model != "claude-sonnet-4-5" {
		t.Errorf("Expected default model 'claude-sonnet-4-5', got %s", cfg.Anthropic.Model)
	}

	if cfg.HTTP.Enabled {
		t.Error("Expected HTTP to be disabled by default")
	}

	if cfg.HTTP.Address != ":8080" {
		t.Errorf("Expected default address ':8080', got %s", cfg.HTTP.Address)
	}

	if cfg.HTTP.TLS.Enabled {
		t.Error("Expected TLS to be disabled by default")
	}
}

func TestLoadConfigFile(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.yaml")

	configContent := `
database:
  connection_string: "postgres://localhost/testdb"

anthropic:
  api_key: "test-api-key"
  model: "claude-haiku-4-5"

http:
  enabled: true
  address: ":9090"
  tls:
    enabled: true
    cert_file: "/path/to/cert.pem"
    key_file: "/path/to/key.pem"
    chain_file: "/path/to/chain.pem"
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := loadConfigFile(configPath)
	if err != nil {
		t.Fatalf("Failed to load config file: %v", err)
	}

	// Verify values
	if cfg.Database.ConnectionString != "postgres://localhost/testdb" {
		t.Errorf("Expected connection string 'postgres://localhost/testdb', got %s", cfg.Database.ConnectionString)
	}

	if cfg.Anthropic.APIKey != "test-api-key" {
		t.Errorf("Expected API key 'test-api-key', got %s", cfg.Anthropic.APIKey)
	}

	if cfg.Anthropic.Model != "claude-haiku-4-5" {
		t.Errorf("Expected model 'claude-haiku-4-5', got %s", cfg.Anthropic.Model)
	}

	if !cfg.HTTP.Enabled {
		t.Error("Expected HTTP to be enabled")
	}

	if cfg.HTTP.Address != ":9090" {
		t.Errorf("Expected address ':9090', got %s", cfg.HTTP.Address)
	}

	if !cfg.HTTP.TLS.Enabled {
		t.Error("Expected TLS to be enabled")
	}

	if cfg.HTTP.TLS.CertFile != "/path/to/cert.pem" {
		t.Errorf("Expected cert file '/path/to/cert.pem', got %s", cfg.HTTP.TLS.CertFile)
	}
}

func TestLoadConfigWithNonexistentFile(t *testing.T) {
	_, err := loadConfigFile("/nonexistent/config.yaml")
	if err == nil {
		t.Error("Expected error when loading nonexistent file")
	}
}

func TestLoadConfigWithInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.yaml")

	invalidContent := `
database:
  connection_string: "test
    missing closing quote
`

	if err := os.WriteFile(configPath, []byte(invalidContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	_, err := loadConfigFile(configPath)
	if err == nil {
		t.Error("Expected error when loading invalid YAML")
	}
}

func TestMergeConfig(t *testing.T) {
	dest := defaultConfig()
	src := &Config{
		Database: DatabaseConfig{
			ConnectionString: "postgres://localhost/newdb",
		},
		Anthropic: AnthropicConfig{
			APIKey: "new-api-key",
			Model:  "claude-opus-4-1",
		},
		HTTP: HTTPConfig{
			Enabled: true,
			Address: ":7070",
		},
	}

	mergeConfig(dest, src)

	// Verify merged values
	if dest.Database.ConnectionString != "postgres://localhost/newdb" {
		t.Errorf("Expected merged connection string, got %s", dest.Database.ConnectionString)
	}

	if dest.Anthropic.APIKey != "new-api-key" {
		t.Errorf("Expected merged API key, got %s", dest.Anthropic.APIKey)
	}

	if dest.Anthropic.Model != "claude-opus-4-1" {
		t.Errorf("Expected merged model, got %s", dest.Anthropic.Model)
	}

	if !dest.HTTP.Enabled {
		t.Error("Expected HTTP to be enabled after merge")
	}

	if dest.HTTP.Address != ":7070" {
		t.Errorf("Expected merged address, got %s", dest.HTTP.Address)
	}
}

func TestApplyEnvironmentVariables(t *testing.T) {
	// Save original env vars
	originalConn := os.Getenv("POSTGRES_CONNECTION_STRING")
	originalKey := os.Getenv("ANTHROPIC_API_KEY")
	originalModel := os.Getenv("ANTHROPIC_MODEL")

	// Set test env vars
	if err := os.Setenv("POSTGRES_CONNECTION_STRING", "postgres://localhost/envdb"); err != nil {
		t.Fatalf("Failed to set env var: %v", err)
	}
	if err := os.Setenv("ANTHROPIC_API_KEY", "env-api-key"); err != nil {
		t.Fatalf("Failed to set env var: %v", err)
	}
	if err := os.Setenv("ANTHROPIC_MODEL", "claude-haiku-4-5"); err != nil {
		t.Fatalf("Failed to set env var: %v", err)
	}

	// Cleanup function
	defer func() {
		restoreOrUnset := func(key, original string) {
			if original != "" {
				if err := os.Setenv(key, original); err != nil {
					t.Logf("Warning: Failed to restore env var %s: %v", key, err)
				}
			} else {
				if err := os.Unsetenv(key); err != nil {
					t.Logf("Warning: Failed to unset env var %s: %v", key, err)
				}
			}
		}
		restoreOrUnset("POSTGRES_CONNECTION_STRING", originalConn)
		restoreOrUnset("ANTHROPIC_API_KEY", originalKey)
		restoreOrUnset("ANTHROPIC_MODEL", originalModel)
	}()

	// Test
	cfg := defaultConfig()
	applyEnvironmentVariables(cfg)

	if cfg.Database.ConnectionString != "postgres://localhost/envdb" {
		t.Errorf("Expected env connection string, got %s", cfg.Database.ConnectionString)
	}

	if cfg.Anthropic.APIKey != "env-api-key" {
		t.Errorf("Expected env API key, got %s", cfg.Anthropic.APIKey)
	}

	if cfg.Anthropic.Model != "claude-haiku-4-5" {
		t.Errorf("Expected env model, got %s", cfg.Anthropic.Model)
	}
}

func TestApplyCLIFlags(t *testing.T) {
	cfg := defaultConfig()
	flags := CLIFlags{
		ConnectionStringSet: true,
		ConnectionString:    "postgres://localhost/clidb",
		APIKeySet:           true,
		APIKey:              "cli-api-key",
		ModelSet:            true,
		Model:               "claude-opus-4-1",
		HTTPEnabledSet:      true,
		HTTPEnabled:         true,
		HTTPAddrSet:         true,
		HTTPAddr:            ":6060",
		TLSEnabledSet:       true,
		TLSEnabled:          true,
		TLSCertSet:          true,
		TLSCertFile:         "/cli/cert.pem",
	}

	applyCLIFlags(cfg, flags)

	// Verify CLI flags were applied
	if cfg.Database.ConnectionString != "postgres://localhost/clidb" {
		t.Errorf("Expected CLI connection string, got %s", cfg.Database.ConnectionString)
	}

	if cfg.Anthropic.APIKey != "cli-api-key" {
		t.Errorf("Expected CLI API key, got %s", cfg.Anthropic.APIKey)
	}

	if cfg.Anthropic.Model != "claude-opus-4-1" {
		t.Errorf("Expected CLI model, got %s", cfg.Anthropic.Model)
	}

	if !cfg.HTTP.Enabled {
		t.Error("Expected HTTP to be enabled")
	}

	if cfg.HTTP.Address != ":6060" {
		t.Errorf("Expected CLI address, got %s", cfg.HTTP.Address)
	}

	if !cfg.HTTP.TLS.Enabled {
		t.Error("Expected TLS to be enabled")
	}

	if cfg.HTTP.TLS.CertFile != "/cli/cert.pem" {
		t.Errorf("Expected CLI cert file, got %s", cfg.HTTP.TLS.CertFile)
	}
}

func TestConfigPriority(t *testing.T) {
	// Create a config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "priority-test.yaml")

	configContent := `
database:
  connection_string: "postgres://localhost/filedb"

anthropic:
  api_key: "file-api-key"
  model: "claude-haiku-4-5"
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Save original env vars
	originalConn := os.Getenv("POSTGRES_CONNECTION_STRING")
	originalKey := os.Getenv("ANTHROPIC_API_KEY")

	// Set env vars (should override file)
	if err := os.Setenv("POSTGRES_CONNECTION_STRING", "postgres://localhost/envdb"); err != nil {
		t.Fatalf("Failed to set env var: %v", err)
	}
	if err := os.Setenv("ANTHROPIC_API_KEY", "env-api-key"); err != nil {
		t.Fatalf("Failed to set env var: %v", err)
	}

	// Cleanup function
	defer func() {
		restoreOrUnset := func(key, original string) {
			if original != "" {
				if err := os.Setenv(key, original); err != nil {
					t.Logf("Warning: Failed to restore env var %s: %v", key, err)
				}
			} else {
				if err := os.Unsetenv(key); err != nil {
					t.Logf("Warning: Failed to unset env var %s: %v", key, err)
				}
			}
		}
		restoreOrUnset("POSTGRES_CONNECTION_STRING", originalConn)
		restoreOrUnset("ANTHROPIC_API_KEY", originalKey)
	}()

	// CLI flags (should override everything)
	flags := CLIFlags{
		ConnectionStringSet: true,
		ConnectionString:    "postgres://localhost/clidb",
	}

	cfg, err := LoadConfig(configPath, flags)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Test priority: CLI > Env > File > Default
	// Connection string: CLI set, should use CLI value
	if cfg.Database.ConnectionString != "postgres://localhost/clidb" {
		t.Errorf("Expected CLI connection string (highest priority), got %s", cfg.Database.ConnectionString)
	}

	// API key: not set in CLI, should use env value
	if cfg.Anthropic.APIKey != "env-api-key" {
		t.Errorf("Expected env API key (middle priority), got %s", cfg.Anthropic.APIKey)
	}

	// Model: not set in CLI or env, should use file value
	if cfg.Anthropic.Model != "claude-haiku-4-5" {
		t.Errorf("Expected file model (low priority), got %s", cfg.Anthropic.Model)
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name      string
		cfg       *Config
		expectErr bool
		errMsg    string
	}{
		{
			name: "valid config",
			cfg: &Config{
				Database: DatabaseConfig{
					ConnectionString: "postgres://localhost/testdb",
				},
			},
			expectErr: false,
		},
		{
			name: "missing connection string",
			cfg: &Config{
				Database: DatabaseConfig{
					ConnectionString: "",
				},
			},
			expectErr: true,
			errMsg:    "connection string is required",
		},
		{
			name: "TLS without HTTP",
			cfg: &Config{
				Database: DatabaseConfig{
					ConnectionString: "postgres://localhost/testdb",
				},
				HTTP: HTTPConfig{
					Enabled: false,
					TLS: TLSConfig{
						Enabled: true,
					},
				},
			},
			expectErr: true,
			errMsg:    "TLS requires HTTP",
		},
		{
			name: "HTTPS without cert",
			cfg: &Config{
				Database: DatabaseConfig{
					ConnectionString: "postgres://localhost/testdb",
				},
				HTTP: HTTPConfig{
					Enabled: true,
					TLS: TLSConfig{
						Enabled:  true,
						CertFile: "",
						KeyFile:  "/path/to/key",
					},
				},
			},
			expectErr: true,
			errMsg:    "certificate file is required",
		},
		{
			name: "HTTPS without key",
			cfg: &Config{
				Database: DatabaseConfig{
					ConnectionString: "postgres://localhost/testdb",
				},
				HTTP: HTTPConfig{
					Enabled: true,
					TLS: TLSConfig{
						Enabled:  true,
						CertFile: "/path/to/cert",
						KeyFile:  "",
					},
				},
			},
			expectErr: true,
			errMsg:    "key file is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.cfg)
			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected error containing '%s', got nil", tt.errMsg)
				} else if err.Error() == "" {
					t.Error("Expected non-empty error message")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			}
		})
	}
}

func TestGetDefaultConfigPath(t *testing.T) {
	binaryPath := "/usr/local/bin/pgedge-postgres-mcp"
	expected := "/usr/local/bin/pgedge-postgres-mcp.yaml"

	result := GetDefaultConfigPath(binaryPath)
	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}

func TestConfigFileExists(t *testing.T) {
	// Test with existing file
	tmpDir := t.TempDir()
	existingFile := filepath.Join(tmpDir, "exists.yaml")
	if err := os.WriteFile(existingFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	if !ConfigFileExists(existingFile) {
		t.Error("Expected ConfigFileExists to return true for existing file")
	}

	// Test with nonexistent file
	nonexistent := filepath.Join(tmpDir, "nonexistent.yaml")
	if ConfigFileExists(nonexistent) {
		t.Error("Expected ConfigFileExists to return false for nonexistent file")
	}
}

func TestLoadConfigWithoutFile(t *testing.T) {
	// Save original env vars
	originalConn := os.Getenv("POSTGRES_CONNECTION_STRING")

	// Set required env var
	if err := os.Setenv("POSTGRES_CONNECTION_STRING", "postgres://localhost/testdb"); err != nil {
		t.Fatalf("Failed to set env var: %v", err)
	}

	// Cleanup function to restore or unset env var
	defer func() {
		if originalConn != "" {
			if err := os.Setenv("POSTGRES_CONNECTION_STRING", originalConn); err != nil {
				t.Logf("Warning: Failed to restore env var: %v", err)
			}
		} else {
			if err := os.Unsetenv("POSTGRES_CONNECTION_STRING"); err != nil {
				t.Logf("Warning: Failed to unset env var: %v", err)
			}
		}
	}()

	// Load config without a config file
	flags := CLIFlags{}
	cfg, err := LoadConfig("", flags)
	if err != nil {
		t.Fatalf("Failed to load config without file: %v", err)
	}

	// Should use env var value
	if cfg.Database.ConnectionString != "postgres://localhost/testdb" {
		t.Errorf("Expected env connection string, got %s", cfg.Database.ConnectionString)
	}

	// Should use default model
	if cfg.Anthropic.Model != "claude-sonnet-4-5" {
		t.Errorf("Expected default model, got %s", cfg.Anthropic.Model)
	}
}

func TestPartialConfigFile(t *testing.T) {
	// Save and clear environment variables to ensure clean test
	originalConn := os.Getenv("POSTGRES_CONNECTION_STRING")
	originalKey := os.Getenv("ANTHROPIC_API_KEY")
	originalModel := os.Getenv("ANTHROPIC_MODEL")

	// Unset all env vars for this test
	if err := os.Unsetenv("POSTGRES_CONNECTION_STRING"); err != nil {
		t.Logf("Warning: Failed to unset env var: %v", err)
	}
	if err := os.Unsetenv("ANTHROPIC_API_KEY"); err != nil {
		t.Logf("Warning: Failed to unset env var: %v", err)
	}
	if err := os.Unsetenv("ANTHROPIC_MODEL"); err != nil {
		t.Logf("Warning: Failed to unset env var: %v", err)
	}

	// Restore environment variables after test
	defer func() {
		restoreOrUnset := func(key, original string) {
			if original != "" {
				if err := os.Setenv(key, original); err != nil {
					t.Logf("Warning: Failed to restore env var %s: %v", key, err)
				}
			}
			// If original was empty, leave it unset
		}
		restoreOrUnset("POSTGRES_CONNECTION_STRING", originalConn)
		restoreOrUnset("ANTHROPIC_API_KEY", originalKey)
		restoreOrUnset("ANTHROPIC_MODEL", originalModel)
	}()

	// Create a partial config file (only sets some values)
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "partial.yaml")

	configContent := `
database:
  connection_string: "postgres://localhost/partialdb"
# anthropic section omitted
http:
  enabled: true
  # address omitted, should use default
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	flags := CLIFlags{}
	cfg, err := LoadConfig(configPath, flags)
	if err != nil {
		t.Fatalf("Failed to load partial config: %v", err)
	}

	// Should use file value
	if cfg.Database.ConnectionString != "postgres://localhost/partialdb" {
		t.Errorf("Expected file connection string, got %s", cfg.Database.ConnectionString)
	}

	// Should use default (not set in file)
	if cfg.HTTP.Address != ":8080" {
		t.Errorf("Expected default address, got %s", cfg.HTTP.Address)
	}

	// Should use default model
	if cfg.Anthropic.Model != "claude-sonnet-4-5" {
		t.Errorf("Expected default model, got %s", cfg.Anthropic.Model)
	}

	// Should use file value
	if !cfg.HTTP.Enabled {
		t.Error("Expected HTTP to be enabled from file")
	}
}
