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

func TestNewVoyageProvider(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		provider, err := NewVoyageProvider("pa-test-key-12345678", "voyage-3-lite")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if provider == nil {
			t.Fatal("expected non-nil provider")
		}
	})

	t.Run("empty API key", func(t *testing.T) {
		_, err := NewVoyageProvider("", "voyage-3-lite")
		if err == nil {
			t.Fatal("expected error for empty API key")
		}
		if err.Error() != "Voyage AI API key cannot be empty" {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("default model", func(t *testing.T) {
		provider, err := NewVoyageProvider("pa-test-key-12345678", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if provider.ModelName() != "voyage-3-lite" {
			t.Errorf("expected default model 'voyage-3-lite', got %q", provider.ModelName())
		}
	})

	t.Run("unsupported model", func(t *testing.T) {
		_, err := NewVoyageProvider("pa-test-key-12345678", "unsupported-model")
		if err == nil {
			t.Fatal("expected error for unsupported model")
		}
	})
}

func TestVoyageProvider_Methods(t *testing.T) {
	provider, err := NewVoyageProvider("pa-test-key-12345678", "voyage-3")
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	t.Run("Dimensions", func(t *testing.T) {
		dims := provider.Dimensions()
		if dims != 1024 {
			t.Errorf("expected 1024 dimensions for voyage-3, got %d", dims)
		}
	})

	t.Run("ModelName", func(t *testing.T) {
		name := provider.ModelName()
		if name != "voyage-3" {
			t.Errorf("expected model 'voyage-3', got %q", name)
		}
	})

	t.Run("ProviderName", func(t *testing.T) {
		name := provider.ProviderName()
		if name != "voyage" {
			t.Errorf("expected provider 'voyage', got %q", name)
		}
	})
}

func TestVoyageProvider_Dimensions(t *testing.T) {
	tests := []struct {
		model      string
		dimensions int
	}{
		{"voyage-3", 1024},
		{"voyage-3-lite", 512},
		{"voyage-2", 1024},
		{"voyage-2-lite", 1024},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			provider, err := NewVoyageProvider("pa-test-key", tt.model)
			if err != nil {
				t.Fatalf("failed to create provider: %v", err)
			}
			if provider.Dimensions() != tt.dimensions {
				t.Errorf("expected %d dimensions, got %d", tt.dimensions, provider.Dimensions())
			}
		})
	}
}

func TestVoyageProvider_Embed_EmptyText(t *testing.T) {
	provider, err := NewVoyageProvider("pa-test-key-12345678", "voyage-3-lite")
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

func TestVoyageProvider_Embed_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer pa-test-key-12345678" {
			t.Errorf("missing or invalid authorization header")
		}

		// Return mock embedding response
		response := voyageEmbeddingResponse{
			Data: []struct {
				Embedding []float64 `json:"embedding"`
				Index     int       `json:"index"`
			}{
				{
					Embedding: make([]float64, 512), // 512 dimensions for voyage-3-lite
					Index:     0,
				},
			},
			Model: "voyage-3-lite",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := &VoyageProvider{
		apiKey:  "pa-test-key-12345678",
		model:   "voyage-3-lite",
		baseURL: server.URL,
		client:  server.Client(),
	}

	embedding, err := provider.Embed(context.Background(), "test text")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(embedding) != 512 {
		t.Errorf("expected 512 dimensions, got %d", len(embedding))
	}
}

func TestVoyageProvider_Embed_APIError(t *testing.T) {
	// Create mock server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": {"message": "Invalid API key"}}`))
	}))
	defer server.Close()

	provider := &VoyageProvider{
		apiKey:  "invalid-key",
		model:   "voyage-3-lite",
		baseURL: server.URL,
		client:  server.Client(),
	}

	_, err := provider.Embed(context.Background(), "test text")
	if err == nil {
		t.Fatal("expected error for API error response")
	}
}

func TestVoyageProvider_Embed_RateLimit(t *testing.T) {
	// Create mock server that returns rate limit error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error": {"message": "Rate limit exceeded"}}`))
	}))
	defer server.Close()

	provider := &VoyageProvider{
		apiKey:  "pa-test-key-12345678",
		model:   "voyage-3-lite",
		baseURL: server.URL,
		client:  server.Client(),
	}

	_, err := provider.Embed(context.Background(), "test text")
	if err == nil {
		t.Fatal("expected error for rate limit")
	}
}

func TestVoyageProvider_Embed_EmptyResponse(t *testing.T) {
	// Create mock server that returns empty embedding
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := voyageEmbeddingResponse{
			Data: []struct {
				Embedding []float64 `json:"embedding"`
				Index     int       `json:"index"`
			}{},
			Model: "voyage-3-lite",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := &VoyageProvider{
		apiKey:  "pa-test-key-12345678",
		model:   "voyage-3-lite",
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
