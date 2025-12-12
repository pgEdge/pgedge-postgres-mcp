/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package embedding

import (
	"testing"
)

func TestNewProvider_Voyage(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		cfg := Config{
			Provider:     "voyage",
			Model:        "voyage-3-lite",
			VoyageAPIKey: "test-api-key-12345678",
		}

		provider, err := NewProvider(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if provider == nil {
			t.Fatal("expected non-nil provider")
		}
		if provider.ProviderName() != "voyage" {
			t.Errorf("expected provider name 'voyage', got %q", provider.ProviderName())
		}
	})

	t.Run("missing API key", func(t *testing.T) {
		cfg := Config{
			Provider: "voyage",
			Model:    "voyage-3-lite",
		}

		_, err := NewProvider(cfg)
		if err == nil {
			t.Fatal("expected error for missing API key")
		}
	})
}

func TestNewProvider_OpenAI(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		cfg := Config{
			Provider:     "openai",
			Model:        "text-embedding-3-small",
			OpenAIAPIKey: "test-api-key-12345678",
		}

		provider, err := NewProvider(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if provider == nil {
			t.Fatal("expected non-nil provider")
		}
		if provider.ProviderName() != "openai" {
			t.Errorf("expected provider name 'openai', got %q", provider.ProviderName())
		}
	})

	t.Run("missing API key", func(t *testing.T) {
		cfg := Config{
			Provider: "openai",
			Model:    "text-embedding-3-small",
		}

		_, err := NewProvider(cfg)
		if err == nil {
			t.Fatal("expected error for missing API key")
		}
	})
}

func TestNewProvider_Ollama(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		cfg := Config{
			Provider:  "ollama",
			Model:     "nomic-embed-text",
			OllamaURL: "http://localhost:11434",
		}

		provider, err := NewProvider(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if provider == nil {
			t.Fatal("expected non-nil provider")
		}
		if provider.ProviderName() != "ollama" {
			t.Errorf("expected provider name 'ollama', got %q", provider.ProviderName())
		}
	})

	t.Run("with defaults", func(t *testing.T) {
		cfg := Config{
			Provider: "ollama",
		}

		provider, err := NewProvider(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if provider == nil {
			t.Fatal("expected non-nil provider")
		}
		// Should use default model
		if provider.ModelName() != "nomic-embed-text" {
			t.Errorf("expected default model 'nomic-embed-text', got %q", provider.ModelName())
		}
	})
}

func TestNewProvider_Unsupported(t *testing.T) {
	cfg := Config{
		Provider: "unsupported",
	}

	_, err := NewProvider(cfg)
	if err == nil {
		t.Fatal("expected error for unsupported provider")
	}
	if err.Error() != "unsupported embedding provider: unsupported (supported: voyage, openai, ollama)" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestConfigStruct(t *testing.T) {
	cfg := Config{
		Provider:     "voyage",
		Model:        "voyage-3",
		VoyageAPIKey: "voyage-key",
		OpenAIAPIKey: "openai-key",
		OllamaURL:    "http://localhost:11434",
	}

	if cfg.Provider != "voyage" {
		t.Errorf("expected provider 'voyage', got %q", cfg.Provider)
	}
	if cfg.Model != "voyage-3" {
		t.Errorf("expected model 'voyage-3', got %q", cfg.Model)
	}
}
