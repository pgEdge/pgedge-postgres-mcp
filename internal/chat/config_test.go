/*-------------------------------------------------------------------------
 *
 * Tests for Configuration
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package chat

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_Defaults(t *testing.T) {
	// Clear environment variables that might interfere
	os.Unsetenv("PGEDGE_MCP_MODE")
	os.Unsetenv("PGEDGE_LLM_PROVIDER")
	os.Unsetenv("ANTHROPIC_API_KEY")

	// Load config with no file
	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Check defaults
	if cfg.MCP.Mode != "stdio" {
		t.Errorf("Expected MCP mode 'stdio', got '%s'", cfg.MCP.Mode)
	}

	if cfg.LLM.Provider != "anthropic" {
		t.Errorf("Expected LLM provider 'anthropic', got '%s'", cfg.LLM.Provider)
	}

	if cfg.LLM.MaxTokens != 4096 {
		t.Errorf("Expected MaxTokens 4096, got %d", cfg.LLM.MaxTokens)
	}

	if cfg.LLM.Temperature != 0.7 {
		t.Errorf("Expected Temperature 0.7, got %f", cfg.LLM.Temperature)
	}
}

func TestLoadConfig_Environment(t *testing.T) {
	// Set environment variables
	os.Setenv("PGEDGE_MCP_MODE", "http")
	os.Setenv("PGEDGE_MCP_URL", "http://localhost:8080")
	os.Setenv("PGEDGE_LLM_PROVIDER", "ollama")
	os.Setenv("PGEDGE_LLM_MODEL", "llama3")
	defer func() {
		os.Unsetenv("PGEDGE_MCP_MODE")
		os.Unsetenv("PGEDGE_MCP_URL")
		os.Unsetenv("PGEDGE_LLM_PROVIDER")
		os.Unsetenv("PGEDGE_LLM_MODEL")
	}()

	// Load config
	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Check environment overrides
	if cfg.MCP.Mode != "http" {
		t.Errorf("Expected MCP mode 'http', got '%s'", cfg.MCP.Mode)
	}

	if cfg.MCP.URL != "http://localhost:8080" {
		t.Errorf("Expected MCP URL 'http://localhost:8080', got '%s'", cfg.MCP.URL)
	}

	if cfg.LLM.Provider != "ollama" {
		t.Errorf("Expected LLM provider 'ollama', got '%s'", cfg.LLM.Provider)
	}

	if cfg.LLM.Model != "llama3" {
		t.Errorf("Expected LLM model 'llama3', got '%s'", cfg.LLM.Model)
	}
}

func TestLoadConfig_File(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.yaml")

	configContent := `
mcp:
  mode: http
  url: http://test.example.com:8080
  token: test-token

llm:
  provider: ollama
  model: test-model
  ollama_url: http://localhost:11434
  max_tokens: 2048
  temperature: 0.5

ui:
  no_color: true
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Load config from file
	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Check file values
	if cfg.MCP.Mode != "http" {
		t.Errorf("Expected MCP mode 'http', got '%s'", cfg.MCP.Mode)
	}

	if cfg.MCP.URL != "http://test.example.com:8080" {
		t.Errorf("Expected MCP URL 'http://test.example.com:8080', got '%s'", cfg.MCP.URL)
	}

	if cfg.LLM.Provider != "ollama" {
		t.Errorf("Expected LLM provider 'ollama', got '%s'", cfg.LLM.Provider)
	}

	if cfg.LLM.Model != "test-model" {
		t.Errorf("Expected LLM model 'test-model', got '%s'", cfg.LLM.Model)
	}

	if cfg.LLM.MaxTokens != 2048 {
		t.Errorf("Expected MaxTokens 2048, got %d", cfg.LLM.MaxTokens)
	}

	if cfg.LLM.Temperature != 0.5 {
		t.Errorf("Expected Temperature 0.5, got %f", cfg.LLM.Temperature)
	}

	if !cfg.UI.NoColor {
		t.Error("Expected NoColor to be true")
	}
}

func TestValidate_StdioMode(t *testing.T) {
	cfg := &Config{
		MCP: MCPConfig{
			Mode:       "stdio",
			ServerPath: "/path/to/server",
		},
		LLM: LLMConfig{
			Provider: "anthropic",
			AnthropicAPIKey: "test-key",
			Model:    "claude-test",
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate failed: %v", err)
	}
}

func TestValidate_HTTPMode(t *testing.T) {
	cfg := &Config{
		MCP: MCPConfig{
			Mode:     "http",
			URL:      "http://localhost:8080",
			AuthMode: "token",
		},
		LLM: LLMConfig{
			Provider: "ollama",
			Model:    "llama3",
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate failed: %v", err)
	}
}

func TestValidate_InvalidMode(t *testing.T) {
	cfg := &Config{
		MCP: MCPConfig{
			Mode: "invalid",
		},
		LLM: LLMConfig{
			Provider: "anthropic",
			AnthropicAPIKey: "test-key",
		},
	}

	if err := cfg.Validate(); err == nil {
		t.Error("Expected validation error for invalid mode")
	}
}

func TestValidate_MissingURL(t *testing.T) {
	cfg := &Config{
		MCP: MCPConfig{
			Mode: "http",
			// URL is missing
		},
		LLM: LLMConfig{
			Provider: "anthropic",
			AnthropicAPIKey: "test-key",
		},
	}

	if err := cfg.Validate(); err == nil {
		t.Error("Expected validation error for missing URL in HTTP mode")
	}
}

func TestValidate_MissingServerPath(t *testing.T) {
	cfg := &Config{
		MCP: MCPConfig{
			Mode: "stdio",
			// ServerPath is missing
		},
		LLM: LLMConfig{
			Provider: "anthropic",
			AnthropicAPIKey: "test-key",
		},
	}

	if err := cfg.Validate(); err == nil {
		t.Error("Expected validation error for missing server path in stdio mode")
	}
}

func TestValidate_InvalidProvider(t *testing.T) {
	cfg := &Config{
		MCP: MCPConfig{
			Mode:       "stdio",
			ServerPath: "/path/to/server",
		},
		LLM: LLMConfig{
			Provider: "invalid",
		},
	}

	if err := cfg.Validate(); err == nil {
		t.Error("Expected validation error for invalid provider")
	}
}

func TestValidate_MissingAPIKey(t *testing.T) {
	cfg := &Config{
		MCP: MCPConfig{
			Mode:       "stdio",
			ServerPath: "/path/to/server",
		},
		LLM: LLMConfig{
			Provider: "anthropic",
			// APIKey is missing
		},
	}

	if err := cfg.Validate(); err == nil {
		t.Error("Expected validation error for missing API key for Anthropic")
	}
}
