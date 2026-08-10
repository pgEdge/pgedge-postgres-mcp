/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package tools

import (
	"strings"
	"testing"
)

func TestNewEmbedClient(t *testing.T) {
	tests := []struct {
		name      string
		cfg       embedClientConfig
		wantModel string
		wantErr   string
	}{
		{
			name: "gemini with api key defaults the model",
			cfg: embedClientConfig{
				Provider:     "gemini",
				GeminiAPIKey: "test-gemini-key",
			},
			wantModel: "gemini-embedding-001",
		},
		{
			name: "gemini honours an explicit model",
			cfg: embedClientConfig{
				Provider:     "gemini",
				Model:        "text-embedding-004",
				GeminiAPIKey: "test-gemini-key",
			},
			wantModel: "text-embedding-004",
		},
		{
			name: "gemini accepts a base URL override",
			cfg: embedClientConfig{
				Provider:      "gemini",
				GeminiAPIKey:  "test-gemini-key",
				GeminiBaseURL: "https://gemini.example.com",
			},
			wantModel: "gemini-embedding-001",
		},
		{
			name: "gemini is matched case-insensitively",
			cfg: embedClientConfig{
				Provider:     "  GEMINI ",
				GeminiAPIKey: "test-gemini-key",
			},
			wantModel: "gemini-embedding-001",
		},
		{
			name: "gemini without an api key fails",
			cfg: embedClientConfig{
				Provider: "gemini",
			},
			wantErr: "missing Gemini API key for embedding provider 'gemini'",
		},
		{
			name: "voyage defaults the model",
			cfg: embedClientConfig{
				Provider:     "voyage",
				VoyageAPIKey: "test-voyage-key",
			},
			wantModel: "voyage-3-lite",
		},
		{
			name: "openai leaves the model to the library",
			cfg: embedClientConfig{
				Provider:     "openai",
				Model:        "text-embedding-3-small",
				OpenAIAPIKey: "test-openai-key",
			},
			wantModel: "text-embedding-3-small",
		},
		{
			name: "ollama defaults the model",
			cfg: embedClientConfig{
				Provider: "ollama",
			},
			wantModel: "nomic-embed-text",
		},
		{
			name: "an unknown provider lists the supported ones",
			cfg: embedClientConfig{
				Provider: "nonesuch",
			},
			wantErr: "unsupported embedding provider: nonesuch (supported: voyage, openai, gemini, ollama)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, model, err := newEmbedClient(tt.cfg)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("error = %q, want it to contain %q", err.Error(), tt.wantErr)
				}
				if client != nil {
					t.Error("expected a nil client alongside the error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if client == nil {
				t.Fatal("expected a non-nil client")
			}
			if model != tt.wantModel {
				t.Errorf("resolved model = %q, want %q", model, tt.wantModel)
			}
		})
	}
}
