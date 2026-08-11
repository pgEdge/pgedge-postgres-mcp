/*-------------------------------------------------------------------------
*
 * pgEdge Natural Language Agent
*
* Copyright (c) 2025 - 2026, pgEdge, Inc.
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
  # Retired option: still accepted in existing files, and ignored.
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
			Provider:        "anthropic",
			AnthropicAPIKey: "test-key",
			Model:           "claude-test",
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
			Provider:        "anthropic",
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
			Provider:        "anthropic",
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
			Provider:        "anthropic",
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

func TestValidate_Gemini(t *testing.T) {
	cfg := &Config{
		MCP: MCPConfig{
			Mode:       "stdio",
			ServerPath: "/path/to/server",
		},
		LLM: LLMConfig{
			Provider:     "gemini",
			GeminiAPIKey: "test-key",
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	if cfg.LLM.Model != "gemini-2.5-flash" {
		t.Errorf("Expected default Gemini model 'gemini-2.5-flash', got '%s'", cfg.LLM.Model)
	}
}

func TestValidate_GeminiMissingAPIKey(t *testing.T) {
	cfg := &Config{
		MCP: MCPConfig{
			Mode:       "stdio",
			ServerPath: "/path/to/server",
		},
		LLM: LLMConfig{
			Provider: "gemini",
			// GeminiAPIKey is missing
		},
	}

	if err := cfg.Validate(); err == nil {
		t.Error("Expected validation error for missing API key for Gemini")
	}
}

func TestIsProviderConfigured(t *testing.T) {
	cfg := &Config{
		LLM: LLMConfig{
			GeminiAPIKey: "test-key",
		},
	}

	if !cfg.IsProviderConfigured("gemini") {
		t.Error("Expected gemini to be configured when the API key is set")
	}
	if cfg.IsProviderConfigured("anthropic") {
		t.Error("Expected anthropic to be unconfigured without an API key")
	}
	if cfg.IsProviderConfigured("unknown") {
		t.Error("Expected an unknown provider to be unconfigured")
	}
}

func TestGetConfiguredProviders_IncludesGemini(t *testing.T) {
	cfg := &Config{
		LLM: LLMConfig{
			AnthropicAPIKey: "test-key",
			OpenAIAPIKey:    "test-key",
			GeminiAPIKey:    "test-key",
			OllamaURL:       "http://localhost:11434",
		},
	}

	providers := cfg.GetConfiguredProviders()
	expected := []string{"anthropic", "openai", "gemini", "ollama"}

	if len(providers) != len(expected) {
		t.Fatalf("Expected %d providers, got %d (%v)", len(expected), len(providers), providers)
	}
	for i, want := range expected {
		if providers[i] != want {
			t.Errorf("Expected provider %d to be %q, got %q", i, want, providers[i])
		}
	}
}

func TestLoadConfig_GeminiAPIKeyFromEnvironment(t *testing.T) {
	t.Setenv("PGEDGE_GEMINI_API_KEY", "")
	t.Setenv("GEMINI_API_KEY", "env-gemini-key")

	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.LLM.GeminiAPIKey != "env-gemini-key" {
		t.Errorf("Expected Gemini API key 'env-gemini-key', got '%s'", cfg.LLM.GeminiAPIKey)
	}

	// The prefixed variable takes priority over the bare one.
	t.Setenv("PGEDGE_GEMINI_API_KEY", "prefixed-gemini-key")

	cfg, err = LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.LLM.GeminiAPIKey != "prefixed-gemini-key" {
		t.Errorf("Expected Gemini API key 'prefixed-gemini-key', got '%s'", cfg.LLM.GeminiAPIKey)
	}
}

func TestLoadConfig_LLMBaseURLsFromEnvironment(t *testing.T) {
	t.Setenv("PGEDGE_ANTHROPIC_BASE_URL", "https://anthropic.proxy.example.com")
	t.Setenv("PGEDGE_OPENAI_BASE_URL", "https://openai.proxy.example.com")
	t.Setenv("PGEDGE_GEMINI_BASE_URL", "https://gemini.proxy.example.com")

	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.LLM.AnthropicBaseURL != "https://anthropic.proxy.example.com" {
		t.Errorf("Expected the Anthropic base URL from the environment, got '%s'", cfg.LLM.AnthropicBaseURL)
	}
	if cfg.LLM.OpenAIBaseURL != "https://openai.proxy.example.com" {
		t.Errorf("Expected the OpenAI base URL from the environment, got '%s'", cfg.LLM.OpenAIBaseURL)
	}
	if cfg.LLM.GeminiBaseURL != "https://gemini.proxy.example.com" {
		t.Errorf("Expected the Gemini base URL from the environment, got '%s'", cfg.LLM.GeminiBaseURL)
	}
}

// The embedding variables belong to the server, and carry an _EMBEDDING_
// infix so that PGEDGE_GEMINI_BASE_URL stays free for the chat side; the CLI
// must therefore ignore the embedding name entirely.
func TestLoadConfig_GeminiEmbeddingBaseURLIgnored(t *testing.T) {
	t.Setenv("PGEDGE_GEMINI_BASE_URL", "")
	t.Setenv("PGEDGE_GEMINI_EMBEDDING_BASE_URL", "https://embed.proxy.example.com")

	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.LLM.GeminiBaseURL != "" {
		t.Errorf("Expected an empty Gemini base URL, got '%s'", cfg.LLM.GeminiBaseURL)
	}
}

// The environment must beat the configuration file, as it does on the
// server. The client used to read the environment into its defaults before
// the file was parsed, so the file quietly won instead, which meant the same
// variable resolved differently depending on which binary read it.
func TestLoadConfig_EnvironmentBeatsTheFile(t *testing.T) {
	t.Setenv("PGEDGE_GEMINI_API_KEY", "")
	t.Setenv("GEMINI_API_KEY", "")

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.yaml")
	configContent := "" +
		"mcp:\n" +
		"  mode: http\n" +
		"llm:\n" +
		"  provider: openai\n" +
		"  model: gpt-4o\n" +
		"  gemini_base_url: https://file.example.com\n" +
		"  ollama_url: http://file.example.com:11434\n"
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	t.Setenv("PGEDGE_MCP_MODE", "stdio")
	t.Setenv("PGEDGE_LLM_PROVIDER", "gemini")
	t.Setenv("PGEDGE_LLM_MODEL", "gemini-2.5-flash")
	t.Setenv("PGEDGE_GEMINI_BASE_URL", "https://env.example.com")
	t.Setenv("PGEDGE_OLLAMA_URL", "http://env.example.com:11434")

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	checks := []struct {
		name string
		got  string
		want string
	}{
		{"MCP.Mode", cfg.MCP.Mode, "stdio"},
		{"LLM.Provider", cfg.LLM.Provider, "gemini"},
		{"LLM.Model", cfg.LLM.Model, "gemini-2.5-flash"},
		{"LLM.GeminiBaseURL", cfg.LLM.GeminiBaseURL, "https://env.example.com"},
		{"LLM.OllamaURL", cfg.LLM.OllamaURL, "http://env.example.com:11434"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %q, want the environment value %q", c.name, c.got, c.want)
		}
	}
}

// An absent variable must leave the file's value in place, which is what
// stops the environment pass from flattening a configuration back to the
// defaults.
func TestLoadConfig_FileSurvivesAnAbsentEnvironment(t *testing.T) {
	for _, key := range []string{
		"PGEDGE_MCP_MODE", "PGEDGE_LLM_PROVIDER", "PGEDGE_LLM_MODEL",
		"PGEDGE_GEMINI_BASE_URL", "PGEDGE_OLLAMA_URL",
	} {
		t.Setenv(key, "")
	}

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.yaml")
	configContent := "" +
		"llm:\n" +
		"  provider: openai\n" +
		"  model: gpt-4o\n" +
		"  gemini_base_url: https://file.example.com\n"
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.LLM.Provider != "openai" {
		t.Errorf("LLM.Provider = %q, want the file value", cfg.LLM.Provider)
	}
	if cfg.LLM.Model != "gpt-4o" {
		t.Errorf("LLM.Model = %q, want the file value", cfg.LLM.Model)
	}
	if cfg.LLM.GeminiBaseURL != "https://file.example.com" {
		t.Errorf("LLM.GeminiBaseURL = %q, want the file value", cfg.LLM.GeminiBaseURL)
	}
	// Untouched by either source, so still the built-in default.
	if cfg.LLM.OllamaURL != "http://localhost:11434" {
		t.Errorf("LLM.OllamaURL = %q, want the default", cfg.LLM.OllamaURL)
	}
}

func TestLoadConfig_GeminiAPIKeyFromFile(t *testing.T) {
	t.Setenv("PGEDGE_GEMINI_API_KEY", "")
	t.Setenv("GEMINI_API_KEY", "")

	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "gemini-key")
	if err := os.WriteFile(keyPath, []byte("file-gemini-key\n"), 0600); err != nil {
		t.Fatalf("Failed to write key file: %v", err)
	}

	configPath := filepath.Join(tmpDir, "test-config.yaml")
	configContent := "llm:\n  provider: gemini\n  gemini_api_key_file: " + keyPath + "\n"
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.LLM.GeminiAPIKey != "file-gemini-key" {
		t.Errorf("Expected Gemini API key 'file-gemini-key', got '%s'", cfg.LLM.GeminiAPIKey)
	}
}

func TestReadAPIKeyFile(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "gemini-key")
	if err := os.WriteFile(keyPath, []byte("  file-gemini-key\n"), 0600); err != nil {
		t.Fatalf("Failed to write key file: %v", err)
	}

	key, err := ReadAPIKeyFile(keyPath)
	if err != nil {
		t.Fatalf("ReadAPIKeyFile failed: %v", err)
	}
	if key != "file-gemini-key" {
		t.Errorf("Expected 'file-gemini-key', got '%s'", key)
	}

	// A missing file is reported as an empty key rather than an error, so
	// that the caller can decide how to treat it.
	key, err = ReadAPIKeyFile(filepath.Join(tmpDir, "no-such-file"))
	if err != nil {
		t.Fatalf("ReadAPIKeyFile failed for a missing file: %v", err)
	}
	if key != "" {
		t.Errorf("Expected an empty key for a missing file, got '%s'", key)
	}
}
