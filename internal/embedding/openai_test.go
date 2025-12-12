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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewOpenAIProvider(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		provider, err := NewOpenAIProvider("sk-test-key-12345678", "text-embedding-3-small")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if provider == nil {
			t.Fatal("expected non-nil provider")
		}
	})

	t.Run("empty API key", func(t *testing.T) {
		_, err := NewOpenAIProvider("", "text-embedding-3-small")
		if err == nil {
			t.Fatal("expected error for empty API key")
		}
		if err.Error() != "OpenAI API key cannot be empty" {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("default model", func(t *testing.T) {
		provider, err := NewOpenAIProvider("sk-test-key-12345678", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if provider.ModelName() != "text-embedding-3-small" {
			t.Errorf("expected default model 'text-embedding-3-small', got %q", provider.ModelName())
		}
	})

	t.Run("unsupported model", func(t *testing.T) {
		_, err := NewOpenAIProvider("sk-test-key-12345678", "unsupported-model")
		if err == nil {
			t.Fatal("expected error for unsupported model")
		}
	})
}

func TestOpenAIProvider_Methods(t *testing.T) {
	provider, err := NewOpenAIProvider("sk-test-key-12345678", "text-embedding-3-large")
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	t.Run("Dimensions", func(t *testing.T) {
		dims := provider.Dimensions()
		if dims != 3072 {
			t.Errorf("expected 3072 dimensions for text-embedding-3-large, got %d", dims)
		}
	})

	t.Run("ModelName", func(t *testing.T) {
		name := provider.ModelName()
		if name != "text-embedding-3-large" {
			t.Errorf("expected model 'text-embedding-3-large', got %q", name)
		}
	})

	t.Run("ProviderName", func(t *testing.T) {
		name := provider.ProviderName()
		if name != "openai" {
			t.Errorf("expected provider 'openai', got %q", name)
		}
	})
}

func TestOpenAIProvider_Dimensions(t *testing.T) {
	tests := []struct {
		model      string
		dimensions int
	}{
		{"text-embedding-3-large", 3072},
		{"text-embedding-3-small", 1536},
		{"text-embedding-ada-002", 1536},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			provider, err := NewOpenAIProvider("sk-test-key", tt.model)
			if err != nil {
				t.Fatalf("failed to create provider: %v", err)
			}
			if provider.Dimensions() != tt.dimensions {
				t.Errorf("expected %d dimensions, got %d", tt.dimensions, provider.Dimensions())
			}
		})
	}
}

func TestOpenAIProvider_Embed_EmptyText(t *testing.T) {
	provider, err := NewOpenAIProvider("sk-test-key-12345678", "text-embedding-3-small")
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	_, err = provider.Embed(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty text")
	}
	if err.Error() != "text cannot be empty" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestOpenAIProvider_Embed_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer sk-test-key-12345678" {
			t.Errorf("missing or invalid authorization header")
		}

		// Return mock embedding response
		response := openaiEmbeddingResponse{
			Object: "list",
			Data: []struct {
				Object    string    `json:"object"`
				Embedding []float64 `json:"embedding"`
				Index     int       `json:"index"`
			}{
				{
					Object:    "embedding",
					Embedding: make([]float64, 1536), // 1536 dimensions
					Index:     0,
				},
			},
			Model: "text-embedding-3-small",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := &OpenAIProvider{
		apiKey:  "sk-test-key-12345678",
		model:   "text-embedding-3-small",
		baseURL: server.URL,
		client:  server.Client(),
	}

	embedding, err := provider.Embed(context.Background(), "test text")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(embedding) != 1536 {
		t.Errorf("expected 1536 dimensions, got %d", len(embedding))
	}
}

func TestOpenAIProvider_Embed_APIError(t *testing.T) {
	// Create mock server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": {"message": "Invalid API key"}}`))
	}))
	defer server.Close()

	provider := &OpenAIProvider{
		apiKey:  "invalid-key",
		model:   "text-embedding-3-small",
		baseURL: server.URL,
		client:  server.Client(),
	}

	_, err := provider.Embed(context.Background(), "test text")
	if err == nil {
		t.Fatal("expected error for API error response")
	}
}

func TestOpenAIProvider_Embed_RateLimit(t *testing.T) {
	// Create mock server that returns rate limit error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error": {"message": "Rate limit exceeded"}}`))
	}))
	defer server.Close()

	provider := &OpenAIProvider{
		apiKey:  "sk-test-key-12345678",
		model:   "text-embedding-3-small",
		baseURL: server.URL,
		client:  server.Client(),
	}

	_, err := provider.Embed(context.Background(), "test text")
	if err == nil {
		t.Fatal("expected error for rate limit")
	}
}

func TestOpenAIProvider_Embed_EmptyResponse(t *testing.T) {
	// Create mock server that returns empty embedding
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := openaiEmbeddingResponse{
			Object: "list",
			Data: []struct {
				Object    string    `json:"object"`
				Embedding []float64 `json:"embedding"`
				Index     int       `json:"index"`
			}{},
			Model: "text-embedding-3-small",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := &OpenAIProvider{
		apiKey:  "sk-test-key-12345678",
		model:   "text-embedding-3-small",
		baseURL: server.URL,
		client:  server.Client(),
	}

	_, err := provider.Embed(context.Background(), "test text")
	if err == nil {
		t.Fatal("expected error for empty embedding")
	}
	if err.Error() != "received empty embedding from API" {
		t.Errorf("unexpected error: %v", err)
	}
}
