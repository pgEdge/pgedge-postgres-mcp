/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025 - 2026, pgEdge, Inc.
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
	"net/url"
	"strings"
	"time"
)

const (
	// VoyageHTTPTimeout is the HTTP client timeout for Voyage API requests
	VoyageHTTPTimeout = 30 * time.Second
)

// VoyageProvider implements embedding generation using Voyage AI's API
type VoyageProvider struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

// voyageEmbeddingRequest represents a request to Voyage AI's embeddings API
type voyageEmbeddingRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

// voyageEmbeddingResponse represents a response from Voyage AI's embeddings API
type voyageEmbeddingResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}

// Model dimensions for Voyage models
var voyageModelDimensions = map[string]int{
	"voyage-3":      1024,
	"voyage-3-lite": 512,
	"voyage-2":      1024,
	"voyage-2-lite": 1024,
}

// NewVoyageProvider creates a new Voyage AI embedding provider.
// baseURL can be empty to use the default (https://api.voyageai.com/v1/embeddings).
// NOTE: Unlike some other providers, custom baseURL values must include the full
// API path (e.g., "https://proxy.example.com/v1/embeddings"), not just the base
// host. The URL is used directly without appending any path.
func NewVoyageProvider(apiKey, model, baseURL string) (*VoyageProvider, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("Voyage AI API key cannot be empty")
	}

	// Default to voyage-3-lite if no model specified
	if model == "" {
		model = "voyage-3-lite"
	}

	// Validate model is supported
	if _, ok := voyageModelDimensions[model]; !ok {
		return nil, fmt.Errorf("unsupported Voyage AI model: %s (supported: voyage-3, voyage-3-lite, voyage-2, voyage-2-lite)", model)
	}

	// Default base URL if not specified
	if baseURL == "" {
		baseURL = "https://api.voyageai.com/v1/embeddings"
	} else {
		// Validate and normalize the base URL
		baseURL = strings.TrimSpace(baseURL)
		baseURL = strings.TrimSuffix(baseURL, "/")

		parsedURL, err := url.Parse(baseURL)
		if err != nil {
			return nil, fmt.Errorf("invalid Voyage AI base URL: %w", err)
		}
		if parsedURL.Scheme != "https" && parsedURL.Scheme != "http" {
			return nil, fmt.Errorf("Voyage AI base URL must use http or https scheme, got: %s", parsedURL.Scheme)
		}
		if parsedURL.Host == "" {
			return nil, fmt.Errorf("Voyage AI base URL must include a host")
		}
	}

	// Mask the API key for logging (show only first/last few characters)
	maskedKey := "(redacted)"
	if len(apiKey) > 8 {
		maskedKey = apiKey[:4] + "..." + apiKey[len(apiKey)-4:]
	}

	LogProviderInit("voyage", model, map[string]string{
		"api_key":  maskedKey,
		"base_url": baseURL,
	})

	return &VoyageProvider{
		apiKey:  apiKey,
		model:   model,
		baseURL: baseURL,
		client: &http.Client{
			Timeout: VoyageHTTPTimeout,
		},
	}, nil
}

// Embed generates an embedding vector for the given text
func (p *VoyageProvider) Embed(ctx context.Context, text string) ([]float64, error) {
	startTime := time.Now()
	textLen := len(text)

	if text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	url := p.baseURL
	LogAPICallDetails("voyage", p.model, url, textLen)
	LogRequestTrace("voyage", p.model, text)

	reqBody := voyageEmbeddingRequest{
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
		LogConnectionError("voyage", url, err)
		duration := time.Since(startTime)
		LogAPICall("voyage", p.model, textLen, duration, 0, err)
		return nil, fmt.Errorf("failed to make API request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			duration := time.Since(startTime)
			err := fmt.Errorf("API request failed with status %d (error reading response body: %w)", resp.StatusCode, readErr)
			LogAPICall("voyage", p.model, textLen, duration, 0, err)
			return nil, err
		}

		// Check if this is a rate limit error
		if resp.StatusCode == 429 {
			LogRateLimitError("voyage", p.model, resp.StatusCode, string(body))
		}

		duration := time.Since(startTime)
		err := fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
		LogAPICall("voyage", p.model, textLen, duration, 0, err)
		return nil, err
	}

	var embResp voyageEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embResp); err != nil {
		duration := time.Since(startTime)
		LogAPICall("voyage", p.model, textLen, duration, 0, err)
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(embResp.Data) == 0 || len(embResp.Data[0].Embedding) == 0 {
		duration := time.Since(startTime)
		err := fmt.Errorf("received empty embedding from API")
		LogAPICall("voyage", p.model, textLen, duration, 0, err)
		return nil, err
	}

	duration := time.Since(startTime)
	embedding := embResp.Data[0].Embedding
	dimensions := len(embedding)
	LogResponseTrace("voyage", p.model, resp.StatusCode, dimensions)
	LogAPICall("voyage", p.model, textLen, duration, dimensions, nil)

	return embedding, nil
}

// Dimensions returns the number of dimensions for this model
func (p *VoyageProvider) Dimensions() int {
	return voyageModelDimensions[p.model]
}

// ModelName returns the model name
func (p *VoyageProvider) ModelName() string {
	return p.model
}

// ProviderName returns "voyage"
func (p *VoyageProvider) ProviderName() string {
	return "voyage"
}
