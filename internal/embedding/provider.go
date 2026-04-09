/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
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
	Provider string // "voyage", "ollama", "openai", or "gemini"
	Model    string // Model name (provider-specific)

	// Voyage AI-specific
	VoyageAPIKey  string
	VoyageBaseURL string // Base URL for Voyage API (optional, uses default if empty)

	// OpenAI-specific
	OpenAIAPIKey  string
	OpenAIBaseURL string // Base URL for OpenAI API (optional, uses default if empty)

	// Ollama-specific
	OllamaURL string

	// Gemini-specific
	GeminiAPIKey  string
	GeminiBaseURL string // Base URL for Gemini API (optional, uses default if empty)

	// Custom HTTP headers applied to all provider API requests
	CustomHeaders map[string]string
}

// NewProvider creates a new embedding provider based on configuration
func NewProvider(cfg Config) (Provider, error) {
	switch cfg.Provider {
	case "voyage":
		if cfg.VoyageAPIKey == "" {
			return nil, fmt.Errorf("voyage AI API key is required when provider is 'voyage'")
		}
		return NewVoyageProvider(cfg.VoyageAPIKey, cfg.Model, cfg.VoyageBaseURL, cfg.CustomHeaders)

	case "openai":
		if cfg.OpenAIAPIKey == "" && cfg.OpenAIBaseURL == "" {
			return nil, fmt.Errorf("openAI API key is required when provider is 'openai' (unless using a custom base URL for local models)")
		}
		return NewOpenAIProvider(cfg.OpenAIAPIKey, cfg.Model, cfg.OpenAIBaseURL, cfg.CustomHeaders)

	case "ollama":
		if cfg.OllamaURL == "" {
			cfg.OllamaURL = "http://localhost:11434" // Default
		}
		if cfg.Model == "" {
			cfg.Model = "nomic-embed-text" // Default model
		}
		return NewOllamaProvider(cfg.OllamaURL, cfg.Model, cfg.CustomHeaders)

	case "gemini":
		if cfg.GeminiAPIKey == "" {
			return nil, fmt.Errorf("gemini API key is required when provider is 'gemini'")
		}
		return NewGeminiProvider(cfg.GeminiAPIKey, cfg.Model, cfg.GeminiBaseURL, cfg.CustomHeaders)

	default:
		return nil, fmt.Errorf("unsupported embedding provider: %s (supported: voyage, openai, ollama, gemini)", cfg.Provider)
	}
}
