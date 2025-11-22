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
	"context"
	"fmt"
)

// Provider defines the interface for embedding generation
type Provider interface {
	// Embed generates an embedding vector for the given text
	Embed(ctx context.Context, text string) ([]float64, error)

	// Dimensions returns the number of dimensions in the embedding vector
	Dimensions() int

	// ModelName returns the name of the model being used
	ModelName() string

	// ProviderName returns the name of the provider (e.g., "voyage", "ollama", "openai")
	ProviderName() string
}

// Config holds configuration for embedding providers
type Config struct {
	Provider string // "voyage", "ollama", or "openai"
	Model    string // Model name (provider-specific)

	// Voyage AI-specific
	VoyageAPIKey string

	// OpenAI-specific
	OpenAIAPIKey string

	// Ollama-specific
	OllamaURL string
}

// NewProvider creates a new embedding provider based on configuration
func NewProvider(cfg Config) (Provider, error) {
	switch cfg.Provider {
	case "voyage":
		if cfg.VoyageAPIKey == "" {
			return nil, fmt.Errorf("Voyage AI API key is required when provider is 'voyage'")
		}
		return NewVoyageProvider(cfg.VoyageAPIKey, cfg.Model)

	case "openai":
		if cfg.OpenAIAPIKey == "" {
			return nil, fmt.Errorf("OpenAI API key is required when provider is 'openai'")
		}
		return NewOpenAIProvider(cfg.OpenAIAPIKey, cfg.Model)

	case "ollama":
		if cfg.OllamaURL == "" {
			cfg.OllamaURL = "http://localhost:11434" // Default
		}
		if cfg.Model == "" {
			cfg.Model = "nomic-embed-text" // Default model
		}
		return NewOllamaProvider(cfg.OllamaURL, cfg.Model)

	default:
		return nil, fmt.Errorf("unsupported embedding provider: %s (supported: voyage, openai, ollama)", cfg.Provider)
	}
}
