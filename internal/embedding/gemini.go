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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	// GeminiHTTPTimeout is the HTTP client timeout for Gemini API requests
	GeminiHTTPTimeout = 30 * time.Second
)

// GeminiProvider implements embedding generation using Google's Gemini API
type GeminiProvider struct {
	apiKey        string
	model         string
	baseURL       string
	customHeaders map[string]string
	client        *http.Client
}

// geminiEmbedRequest represents a request to Gemini's embedding API
type geminiEmbedRequest struct {
	Model   string             `json:"model"`
	Content geminiEmbedContent `json:"content"`
}

// geminiEmbedContent represents the content field of a Gemini embed request
type geminiEmbedContent struct {
	Parts []geminiEmbedPart `json:"parts"`
}

// geminiEmbedPart represents a single text part in a Gemini embed request
type geminiEmbedPart struct {
	Text string `json:"text"`
}

// geminiEmbedResponse represents a response from Gemini's embedding API
type geminiEmbedResponse struct {
	Embedding struct {
		Values []float64 `json:"values"`
	} `json:"embedding"`
}

// Model dimensions for Gemini embedding models
var geminiModelDimensions = map[string]int{
	"text-embedding-004": 768,
}

// NewGeminiProvider creates a new Gemini embedding provider
// baseURL can be empty to use the default (https://generativelanguage.googleapis.com)
func NewGeminiProvider(apiKey, model, baseURL string, customHeaders map[string]string) (*GeminiProvider, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("Gemini API key cannot be empty")
	}
	if model == "" {
		model = "text-embedding-004"
	}
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com"
	}

	// Mask the API key for logging (show only first/last few characters)
	maskedKey := "(redacted)"
	if len(apiKey) > 8 {
		maskedKey = apiKey[:4] + "..." + apiKey[len(apiKey)-4:]
	}

	LogProviderInit("gemini", model, map[string]string{
		"api_key":  maskedKey,
		"base_url": baseURL,
	})

	return &GeminiProvider{
		apiKey:        apiKey,
		model:         model,
		baseURL:       baseURL,
		customHeaders: customHeaders,
		client:        &http.Client{Timeout: GeminiHTTPTimeout},
	}, nil
}

// Embed generates an embedding vector for the given text
func (p *GeminiProvider) Embed(ctx context.Context, text string) ([]float64, error) {
	startTime := time.Now()
	textLen := len(text)

	if text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	apiURL := fmt.Sprintf("%s/v1beta/models/%s:embedContent?key=%s", p.baseURL, p.model, p.apiKey)
	logURL := fmt.Sprintf("%s/v1beta/models/%s:embedContent", p.baseURL, p.model)
	LogAPICallDetails("gemini", p.model, logURL, textLen)
	LogRequestTrace("gemini", p.model, text)

	reqBody := geminiEmbedRequest{
		Model: "models/" + p.model,
		Content: geminiEmbedContent{
			Parts: []geminiEmbedPart{{Text: text}},
		},
	}

	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(reqBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range p.customHeaders {
		req.Header.Set(k, v)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		LogConnectionError("gemini", logURL, err)
		duration := time.Since(startTime)
		LogAPICall("gemini", p.model, textLen, duration, 0, err)
		return nil, fmt.Errorf("failed to make API request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			duration := time.Since(startTime)
			err := fmt.Errorf("API request failed with status %d (error reading response body: %w)", resp.StatusCode, readErr)
			LogAPICall("gemini", p.model, textLen, duration, 0, err)
			return nil, err
		}

		// Check if this is a rate limit error
		if resp.StatusCode == 429 {
			LogRateLimitError("gemini", p.model, resp.StatusCode, string(body))
		}

		duration := time.Since(startTime)
		err := fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
		LogAPICall("gemini", p.model, textLen, duration, 0, err)
		return nil, err
	}

	var embResp geminiEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&embResp); err != nil {
		duration := time.Since(startTime)
		LogAPICall("gemini", p.model, textLen, duration, 0, err)
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(embResp.Embedding.Values) == 0 {
		duration := time.Since(startTime)
		err := fmt.Errorf("received empty embedding from API")
		LogAPICall("gemini", p.model, textLen, duration, 0, err)
		return nil, err
	}

	duration := time.Since(startTime)
	dimensions := len(embResp.Embedding.Values)
	LogResponseTrace("gemini", p.model, resp.StatusCode, dimensions)
	LogAPICall("gemini", p.model, textLen, duration, dimensions, nil)

	return embResp.Embedding.Values, nil
}

// Dimensions returns the number of dimensions for this model
func (p *GeminiProvider) Dimensions() int {
	if dims, ok := geminiModelDimensions[p.model]; ok {
		return dims
	}
	return 768
}

// ModelName returns the model name
func (p *GeminiProvider) ModelName() string {
	return p.model
}

// ProviderName returns "gemini"
func (p *GeminiProvider) ProviderName() string {
	return "gemini"
}
