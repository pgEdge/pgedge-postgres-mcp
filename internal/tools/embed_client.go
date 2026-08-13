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
	"fmt"
	"strings"
	"time"

	"github.com/pgEdge/pgedge-go-llm-lib/llm"
	_ "github.com/pgEdge/pgedge-go-llm-lib/llm/provider/gemini"
	_ "github.com/pgEdge/pgedge-go-llm-lib/llm/provider/ollama"
	_ "github.com/pgEdge/pgedge-go-llm-lib/llm/provider/openai"
	_ "github.com/pgEdge/pgedge-go-llm-lib/llm/provider/voyage"
)

// embedClientConfig collects the per-provider config fields the tools
// need to construct an llm.Client for embeddings.
type embedClientConfig struct {
	Provider      string
	Model         string
	VoyageAPIKey  string
	VoyageBaseURL string
	OpenAIAPIKey  string
	OpenAIBaseURL string
	GeminiAPIKey  string
	GeminiBaseURL string
	OllamaURL     string

	PerAttemptTimeout int
}

// newEmbedClient builds an llm.Client for the configured embedding
// provider, applying the same defaults the previous embedding.Provider
// wrapper applied (voyage-3-lite model default, nomic-embed-text for
// ollama, localhost:11434 for the ollama URL), along with a
// gemini-embedding-001 default for gemini and a text-embedding-3-small
// default for openai, whose request URLs embed the model name and so
// cannot be left empty. It returns the client and the resolved model
// name (after defaults are applied) so callers can display it without
// re-deriving the default logic.
func newEmbedClient(cfg embedClientConfig) (llm.Client, string, error) {
	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	var opts llm.Options
	switch provider {
	case "voyage":
		if cfg.VoyageAPIKey == "" {
			return nil, "", fmt.Errorf("missing Voyage AI API key for embedding provider 'voyage'")
		}
		model := cfg.Model
		if model == "" {
			model = "voyage-3-lite"
		}
		opts = llm.Options{
			APIKey:  cfg.VoyageAPIKey,
			Model:   model,
			BaseURL: cfg.VoyageBaseURL,
		}
	case "openai":
		if cfg.OpenAIAPIKey == "" {
			return nil, "", fmt.Errorf("missing OpenAI API key for embedding provider 'openai'")
		}
		model := cfg.Model
		if model == "" {
			model = "text-embedding-3-small"
		}
		opts = llm.Options{
			APIKey:  cfg.OpenAIAPIKey,
			Model:   model,
			BaseURL: cfg.OpenAIBaseURL,
		}
	case "gemini":
		if cfg.GeminiAPIKey == "" {
			return nil, "", fmt.Errorf("missing Gemini API key for embedding provider 'gemini'")
		}
		model := cfg.Model
		if model == "" {
			model = "gemini-embedding-001"
		}
		opts = llm.Options{
			APIKey:  cfg.GeminiAPIKey,
			Model:   model,
			BaseURL: cfg.GeminiBaseURL,
		}
	case "ollama":
		baseURL := cfg.OllamaURL
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
		model := cfg.Model
		if model == "" {
			model = "nomic-embed-text"
		}
		opts = llm.Options{
			Model:   model,
			BaseURL: baseURL,
		}
	default:
		return nil, "", fmt.Errorf(
			"unsupported embedding provider: %s (supported: voyage, openai, gemini, ollama)",
			provider,
		)
	}

	opts.PerAttemptTimeout = time.Duration(cfg.PerAttemptTimeout) * time.Second

	client, err := llm.NewClient(provider, opts)
	if err != nil {
		return nil, "", fmt.Errorf("create %s embedding client: %w", provider, err)
	}
	return client, opts.Model, nil
}
