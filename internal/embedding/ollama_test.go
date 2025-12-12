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

func TestNewOllamaProvider(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		provider, err := NewOllamaProvider("http://localhost:11434", "nomic-embed-text")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if provider == nil {
			t.Fatal("expected non-nil provider")
		}
	})

	t.Run("default URL", func(t *testing.T) {
		provider, err := NewOllamaProvider("", "nomic-embed-text")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if provider.baseURL != "http://localhost:11434" {
			t.Errorf("expected default URL, got %q", provider.baseURL)
		}
	})

	t.Run("default model", func(t *testing.T) {
		provider, err := NewOllamaProvider("http://localhost:11434", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if provider.model != "nomic-embed-text" {
			t.Errorf("expected default model 'nomic-embed-text', got %q", provider.model)
		}
	})
}

func TestOllamaProvider_Methods(t *testing.T) {
	provider, err := NewOllamaProvider("http://localhost:11434", "nomic-embed-text")
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	t.Run("Dimensions - known model", func(t *testing.T) {
		dims := provider.Dimensions()
		if dims != 768 {
			t.Errorf("expected 768 dimensions for nomic-embed-text, got %d", dims)
		}
	})

	t.Run("ModelName", func(t *testing.T) {
		name := provider.ModelName()
		if name != "nomic-embed-text" {
			t.Errorf("expected model 'nomic-embed-text', got %q", name)
		}
	})

	t.Run("ProviderName", func(t *testing.T) {
		name := provider.ProviderName()
		if name != "ollama" {
			t.Errorf("expected provider 'ollama', got %q", name)
		}
	})
}

func TestOllamaProvider_Dimensions_KnownModels(t *testing.T) {
	tests := []struct {
		model      string
		dimensions int
	}{
		{"nomic-embed-text", 768},
		{"mxbai-embed-large", 1024},
		{"all-minilm", 384},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			provider, err := NewOllamaProvider("http://localhost:11434", tt.model)
			if err != nil {
				t.Fatalf("failed to create provider: %v", err)
			}
			if provider.Dimensions() != tt.dimensions {
				t.Errorf("expected %d dimensions, got %d", tt.dimensions, provider.Dimensions())
			}
		})
	}
}

func TestOllamaProvider_Dimensions_UnknownModel(t *testing.T) {
	provider, err := NewOllamaProvider("http://localhost:11434", "custom-model")
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	// Unknown models return 0 until first embedding call
	dims := provider.Dimensions()
	if dims != 0 {
		t.Errorf("expected 0 dimensions for unknown model, got %d", dims)
	}
}

func TestOllamaProvider_Embed_EmptyText(t *testing.T) {
	provider, err := NewOllamaProvider("http://localhost:11434", "nomic-embed-text")
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

func TestOllamaProvider_Embed_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/embed" {
			t.Errorf("expected path /api/embed, got %s", r.URL.Path)
		}

		// Return mock embedding response
		response := ollamaEmbeddingResponse{
			Embeddings: [][]float64{
				make([]float64, 768), // 768 dimensions
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := &OllamaProvider{
		baseURL: server.URL,
		model:   "nomic-embed-text",
		client:  server.Client(),
	}

	embedding, err := provider.Embed(context.Background(), "test text")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(embedding) != 768 {
		t.Errorf("expected 768 dimensions, got %d", len(embedding))
	}
}

func TestOllamaProvider_Embed_APIError(t *testing.T) {
	// Create mock server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "model not found"}`))
	}))
	defer server.Close()

	provider := &OllamaProvider{
		baseURL: server.URL,
		model:   "nonexistent-model",
		client:  server.Client(),
	}

	_, err := provider.Embed(context.Background(), "test text")
	if err == nil {
		t.Fatal("expected error for API error response")
	}
}

func TestOllamaProvider_Embed_EmptyResponse(t *testing.T) {
	// Create mock server that returns empty embeddings
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := ollamaEmbeddingResponse{
			Embeddings: [][]float64{},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := &OllamaProvider{
		baseURL: server.URL,
		model:   "nomic-embed-text",
		client:  server.Client(),
	}

	_, err := provider.Embed(context.Background(), "test text")
	if err == nil {
		t.Fatal("expected error for empty embedding")
	}
}

func TestOllamaProvider_Embed_UpdatesDimensions(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := ollamaEmbeddingResponse{
			Embeddings: [][]float64{
				make([]float64, 512), // Custom dimensions
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := &OllamaProvider{
		baseURL: server.URL,
		model:   "new-custom-model-for-test",
		client:  server.Client(),
	}

	// Initially dimensions should be 0 for unknown model
	if dims := provider.Dimensions(); dims != 0 {
		t.Errorf("expected 0 initial dimensions, got %d", dims)
	}

	// After embedding call, dimensions should be updated
	_, err := provider.Embed(context.Background(), "test text")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Now dimensions should be discovered
	if dims := provider.Dimensions(); dims != 512 {
		t.Errorf("expected 512 dimensions after embed, got %d", dims)
	}
}
