/*-------------------------------------------------------------------------
 *
 * pgEdge Postgres MCP Server
 *
 * Copyright (c) 2025, pgEdge, Inc.
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
	"time"
)

// AnthropicProvider implements embedding generation using Anthropic's API
type AnthropicProvider struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

// anthropicEmbeddingRequest represents a request to Anthropic's embeddings API
type anthropicEmbeddingRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

// anthropicEmbeddingResponse represents a response from Anthropic's embeddings API
type anthropicEmbeddingResponse struct {
	Embedding []float64 `json:"embedding"`
	Model     string    `json:"model"`
}

// Model dimensions for Anthropic/Voyage models
var anthropicModelDimensions = map[string]int{
	"voyage-3":      1024,
	"voyage-3-lite": 512,
	"voyage-2":      1024,
	"voyage-2-lite": 1024,
}

// NewAnthropicProvider creates a new Anthropic embedding provider
func NewAnthropicProvider(apiKey, model string) (*AnthropicProvider, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("Anthropic API key cannot be empty")
	}

	// Default to voyage-3-lite if no model specified
	if model == "" {
		model = "voyage-3-lite"
	}

	// Validate model is supported
	if _, ok := anthropicModelDimensions[model]; !ok {
		return nil, fmt.Errorf("unsupported Anthropic model: %s (supported: voyage-3, voyage-3-lite, voyage-2, voyage-2-lite)", model)
	}

	// Mask the API key for logging (show only first/last few characters)
	maskedKey := "(redacted)"
	if len(apiKey) > 8 {
		maskedKey = apiKey[:4] + "..." + apiKey[len(apiKey)-4:]
	}

	LogProviderInit("anthropic", model, map[string]string{
		"api_key":  maskedKey,
		"base_url": "https://api.voyageai.com/v1/embeddings",
	})

	return &AnthropicProvider{
		apiKey:  apiKey,
		model:   model,
		baseURL: "https://api.voyageai.com/v1/embeddings",
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// Embed generates an embedding vector for the given text
func (p *AnthropicProvider) Embed(ctx context.Context, text string) ([]float64, error) {
	startTime := time.Now()
	textLen := len(text)

	if text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	url := p.baseURL
	LogAPICallDetails("anthropic", p.model, url, textLen)
	LogRequestTrace("anthropic", p.model, text)

	reqBody := anthropicEmbeddingRequest{
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
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		LogConnectionError("anthropic", url, err)
		duration := time.Since(startTime)
		LogAPICall("anthropic", p.model, textLen, duration, 0, err)
		return nil, fmt.Errorf("failed to make API request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			duration := time.Since(startTime)
			err := fmt.Errorf("API request failed with status %d (error reading response body: %w)", resp.StatusCode, readErr)
			LogAPICall("anthropic", p.model, textLen, duration, 0, err)
			return nil, err
		}

		// Check if this is a rate limit error
		if resp.StatusCode == 429 {
			LogRateLimitError("anthropic", p.model, resp.StatusCode, string(body))
		}

		duration := time.Since(startTime)
		err := fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
		LogAPICall("anthropic", p.model, textLen, duration, 0, err)
		return nil, err
	}

	var embResp anthropicEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embResp); err != nil {
		duration := time.Since(startTime)
		LogAPICall("anthropic", p.model, textLen, duration, 0, err)
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(embResp.Embedding) == 0 {
		duration := time.Since(startTime)
		err := fmt.Errorf("received empty embedding from API")
		LogAPICall("anthropic", p.model, textLen, duration, 0, err)
		return nil, err
	}

	duration := time.Since(startTime)
	dimensions := len(embResp.Embedding)
	LogResponseTrace("anthropic", p.model, resp.StatusCode, dimensions)
	LogAPICall("anthropic", p.model, textLen, duration, dimensions, nil)

	return embResp.Embedding, nil
}

// Dimensions returns the number of dimensions for this model
func (p *AnthropicProvider) Dimensions() int {
	return anthropicModelDimensions[p.model]
}

// ModelName returns the model name
func (p *AnthropicProvider) ModelName() string {
	return p.model
}

// ProviderName returns "anthropic"
func (p *AnthropicProvider) ProviderName() string {
	return "anthropic"
}
