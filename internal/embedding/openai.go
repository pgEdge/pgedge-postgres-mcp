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
	"time"
)

const (
	// OpenAIHTTPTimeout is the HTTP client timeout for OpenAI API requests
	OpenAIHTTPTimeout = 30 * time.Second
)

// OpenAIProvider implements embedding generation using OpenAI's API
type OpenAIProvider struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

// openaiEmbeddingRequest represents a request to OpenAI's embeddings API
type openaiEmbeddingRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

// openaiEmbeddingResponse represents a response from OpenAI's embeddings API
type openaiEmbeddingResponse struct {
	Object string `json:"object"`
	Data   []struct {
		Object    string    `json:"object"`
		Embedding []float64 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

// Model dimensions for OpenAI embedding models
var openaiModelDimensions = map[string]int{
	"text-embedding-3-large": 3072,
	"text-embedding-3-small": 1536,
	"text-embedding-ada-002": 1536,
}

// NewOpenAIProvider creates a new OpenAI embedding provider
func NewOpenAIProvider(apiKey, model string) (*OpenAIProvider, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key cannot be empty")
	}

	// Default to text-embedding-3-small if no model specified
	if model == "" {
		model = "text-embedding-3-small"
	}

	// Validate model is supported
	if _, ok := openaiModelDimensions[model]; !ok {
		return nil, fmt.Errorf("unsupported OpenAI model: %s (supported: text-embedding-3-large, text-embedding-3-small, text-embedding-ada-002)", model)
	}

	// Mask the API key for logging (show only first/last few characters)
	maskedKey := "(redacted)"
	if len(apiKey) > 8 {
		maskedKey = apiKey[:4] + "..." + apiKey[len(apiKey)-4:]
	}

	LogProviderInit("openai", model, map[string]string{
		"api_key":  maskedKey,
		"base_url": "https://api.openai.com/v1",
	})

	return &OpenAIProvider{
		apiKey:  apiKey,
		model:   model,
		baseURL: "https://api.openai.com/v1",
		client: &http.Client{
			Timeout: OpenAIHTTPTimeout,
		},
	}, nil
}

// Embed generates an embedding vector for the given text
func (p *OpenAIProvider) Embed(ctx context.Context, text string) ([]float64, error) {
	startTime := time.Now()
	textLen := len(text)

	if text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	url := p.baseURL + "/embeddings"
	LogAPICallDetails("openai", p.model, url, textLen)
	LogRequestTrace("openai", p.model, text)

	reqBody := openaiEmbeddingRequest{
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
		LogConnectionError("openai", url, err)
		duration := time.Since(startTime)
		LogAPICall("openai", p.model, textLen, duration, 0, err)
		return nil, fmt.Errorf("failed to make API request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			duration := time.Since(startTime)
			err := fmt.Errorf("API request failed with status %d (error reading response body: %w)", resp.StatusCode, readErr)
			LogAPICall("openai", p.model, textLen, duration, 0, err)
			return nil, err
		}

		// Check if this is a rate limit error
		if resp.StatusCode == 429 {
			LogRateLimitError("openai", p.model, resp.StatusCode, string(body))
		}

		duration := time.Since(startTime)
		err := fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
		LogAPICall("openai", p.model, textLen, duration, 0, err)
		return nil, err
	}

	var embResp openaiEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embResp); err != nil {
		duration := time.Since(startTime)
		LogAPICall("openai", p.model, textLen, duration, 0, err)
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(embResp.Data) == 0 || len(embResp.Data[0].Embedding) == 0 {
		duration := time.Since(startTime)
		err := fmt.Errorf("received empty embedding from API")
		LogAPICall("openai", p.model, textLen, duration, 0, err)
		return nil, err
	}

	duration := time.Since(startTime)
	dimensions := len(embResp.Data[0].Embedding)
	LogResponseTrace("openai", p.model, resp.StatusCode, dimensions)
	LogAPICall("openai", p.model, textLen, duration, dimensions, nil)

	return embResp.Data[0].Embedding, nil
}

// Dimensions returns the number of dimensions for this model
func (p *OpenAIProvider) Dimensions() int {
	return openaiModelDimensions[p.model]
}

// ModelName returns the model name
func (p *OpenAIProvider) ModelName() string {
	return p.model
}

// ProviderName returns "openai"
func (p *OpenAIProvider) ProviderName() string {
	return "openai"
}
