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
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := defaultConfig()

	// Test LLM defaults
	if cfg.LLM.Provider != "anthropic" {
		t.Errorf("Expected default provider 'anthropic', got %s", cfg.LLM.Provider)
	}

	if cfg.Anthropic.Model != "claude-sonnet-4-5" {
		t.Errorf("Expected default model 'claude-sonnet-4-5', got %s", cfg.Anthropic.Model)
	}

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
}

func TestValidateConfig_LLMProvider(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name: "Valid Anthropic config",
			cfg: &Config{
				LLM: LLMConfig{Provider: "anthropic"},
				Anthropic: AnthropicConfig{
					APIKey: "test-key",
					Model:  "claude-sonnet-4-5",
				},
			},
			wantErr: false,
		},
		{
			name: "Valid Ollama config",
			cfg: &Config{
				LLM:    LLMConfig{Provider: "ollama"},
				Ollama: OllamaConfig{
					BaseURL: "http://localhost:11434",
					Model:   "qwen2.5-coder:32b",
				},
			},
			wantErr: false,
		},
		{
			name: "Invalid provider",
			cfg: &Config{
				LLM: LLMConfig{Provider: "invalid"},
			},
			wantErr: true,
		},
		{
			name: "Anthropic missing API key",
			cfg: &Config{
				LLM:       LLMConfig{Provider: "anthropic"},
				Anthropic: AnthropicConfig{},
			},
			wantErr: true,
		},
		{
			name: "Ollama missing model",
			cfg: &Config{
				LLM: LLMConfig{Provider: "ollama"},
				Ollama: OllamaConfig{
					BaseURL: "http://localhost:11434",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
