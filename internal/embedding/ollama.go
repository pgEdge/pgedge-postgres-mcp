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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	// OllamaHTTPTimeout is the HTTP client timeout for Ollama API requests
	// Ollama might need time to load models, so this is longer than other providers
	OllamaHTTPTimeout = 60 * time.Second
)

// OllamaProvider implements embedding generation using Ollama
type OllamaProvider struct {
	baseURL string
	model   string
	client  *http.Client
}

// ollamaEmbeddingRequest represents a request to Ollama's embeddings API
type ollamaEmbeddingRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

// ollamaEmbeddingResponse represents a response from Ollama's embeddings API
// Note: Ollama returns an array of embeddings (one per input text)
type ollamaEmbeddingResponse struct {
	Embeddings [][]float64 `json:"embeddings"`
}

// Model dimensions for Ollama embedding models
var ollamaModelDimensions = map[string]int{
	"nomic-embed-text":  768,
	"mxbai-embed-large": 1024,
	"all-minilm":        384,
	"all-minilm:latest": 384,
	"all-minilm:l6-v2":  384,
}

// Mutex to protect concurrent access to ollamaModelDimensions
var ollamaModelDimensionsMu sync.RWMutex

// NewOllamaProvider creates a new Ollama embedding provider
func NewOllamaProvider(baseURL, model string) (*OllamaProvider, error) {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}

	if model == "" {
		model = "nomic-embed-text"
	}

	// Note: For unknown models, we'll discover dimensions on first use
	// This allows using newly released Ollama models

	LogProviderInit("ollama", model, map[string]string{
		"base_url": baseURL,
	})

	return &OllamaProvider{
		baseURL: baseURL,
		model:   model,
		client: &http.Client{
			Timeout: OllamaHTTPTimeout,
		},
	}, nil
}

// Embed generates an embedding vector for the given text
func (p *OllamaProvider) Embed(ctx context.Context, text string) ([]float64, error) {
	startTime := time.Now()
	textLen := len(text)

	if text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	url := p.baseURL + "/api/embed"
	LogAPICallDetails("ollama", p.model, url, textLen)
	LogRequestTrace("ollama", p.model, text)

	reqBody := ollamaEmbeddingRequest{
		Model: p.model,
		Input: text,
	}

	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		LogConnectionError("ollama", url, err)
		duration := time.Since(startTime)
		LogAPICall("ollama", p.model, textLen, duration, 0, err)
		return nil, fmt.Errorf("failed to connect to Ollama at %s: %w (is Ollama running?)", p.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			duration := time.Since(startTime)
			err := fmt.Errorf("Ollama API request failed with status %d (error reading response body: %w)", resp.StatusCode, readErr)
			LogAPICall("ollama", p.model, textLen, duration, 0, err)
			return nil, err
		}

		// Check if this is a rate limit error (though Ollama doesn't typically have rate limits)
		if resp.StatusCode == 429 {
			LogRateLimitError("ollama", p.model, resp.StatusCode, string(body))
		}

		duration := time.Since(startTime)
		err := fmt.Errorf("Ollama API request failed with status %d: %s", resp.StatusCode, string(body))
		LogAPICall("ollama", p.model, textLen, duration, 0, err)
		return nil, err
	}

	var embResp ollamaEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embResp); err != nil {
		duration := time.Since(startTime)
		LogAPICall("ollama", p.model, textLen, duration, 0, err)
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(embResp.Embeddings) == 0 || len(embResp.Embeddings[0]) == 0 {
		duration := time.Since(startTime)
		err := fmt.Errorf("received empty embedding from Ollama (model may not be installed: try 'ollama pull %s')", p.model)
		LogAPICall("ollama", p.model, textLen, duration, 0, err)
		return nil, err
	}

	// Extract the first embedding (we only sent one input text)
	embedding := embResp.Embeddings[0]

	// Update known dimensions if this is a new model
	ollamaModelDimensionsMu.Lock()
	if _, ok := ollamaModelDimensions[p.model]; !ok {
		ollamaModelDimensions[p.model] = len(embedding)
	}
	ollamaModelDimensionsMu.Unlock()

	duration := time.Since(startTime)
	dimensions := len(embedding)
	LogResponseTrace("ollama", p.model, resp.StatusCode, dimensions)
	LogAPICall("ollama", p.model, textLen, duration, dimensions, nil)

	return embedding, nil
}

// Dimensions returns the number of dimensions for this model
func (p *OllamaProvider) Dimensions() int {
	ollamaModelDimensionsMu.RLock()
	defer ollamaModelDimensionsMu.RUnlock()
	if dims, ok := ollamaModelDimensions[p.model]; ok {
		return dims
	}
	// Return 0 if unknown - will be determined on first Embed() call
	return 0
}

// ModelName returns the model name
func (p *OllamaProvider) ModelName() string {
	return p.model
}

// ProviderName returns "ollama"
func (p *OllamaProvider) ProviderName() string {
	return "ollama"
}
