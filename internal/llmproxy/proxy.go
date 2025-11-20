/*-------------------------------------------------------------------------
 *
 * pgEdge Postgres MCP Server - LLM Proxy
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package llmproxy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"pgedge-postgres-mcp/internal/chat"
)

// Config holds LLM configuration from the server config
type Config struct {
	Provider        string
	Model           string
	AnthropicAPIKey string
	OpenAIAPIKey    string
	OllamaURL       string
	MaxTokens       int
	Temperature     float64
}

// Message represents a message in the chat conversation
type Message struct {
	Role         string                 `json:"role"`
	Content      interface{}            `json:"content"`
	CacheControl map[string]interface{} `json:"cache_control,omitempty"`
}

// Tool represents an MCP tool definition
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema InputSchema `json:"inputSchema"`
}

// InputSchema defines the JSON schema for tool input
type InputSchema struct {
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
	Required   []string               `json:"required,omitempty"`
}

// ProvidersResponse represents the response for GET /api/llm/providers
type ProvidersResponse struct {
	Providers    []ProviderInfo `json:"providers"`
	DefaultModel string         `json:"defaultModel"`
}

// ProviderInfo represents information about an LLM provider
type ProviderInfo struct {
	Name      string `json:"name"`
	Display   string `json:"display"`
	IsDefault bool   `json:"isDefault"`
}

// ModelsResponse represents the response for GET /api/llm/models
type ModelsResponse struct {
	Models []ModelInfo `json:"models"`
}

// ModelInfo represents information about an LLM model
type ModelInfo struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// ChatRequest represents the request body for POST /api/llm/chat
type ChatRequest struct {
	Messages []Message `json:"messages"`
	Tools    []Tool    `json:"tools"`
	Provider string    `json:"provider,omitempty"` // Override default provider
	Model    string    `json:"model,omitempty"`    // Override default model
}

// ChatResponse represents the response body for POST /api/llm/chat
type ChatResponse struct {
	Content    []interface{} `json:"content"`
	StopReason string        `json:"stop_reason"`
}

// HandleProviders handles GET /api/llm/providers
func HandleProviders(w http.ResponseWriter, r *http.Request, config *Config) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	providers := []ProviderInfo{}

	// Check which providers are configured
	if config.AnthropicAPIKey != "" {
		providers = append(providers, ProviderInfo{
			Name:      "anthropic",
			Display:   "Anthropic Claude",
			IsDefault: config.Provider == "anthropic",
		})
	}

	if config.OpenAIAPIKey != "" {
		providers = append(providers, ProviderInfo{
			Name:      "openai",
			Display:   "OpenAI",
			IsDefault: config.Provider == "openai",
		})
	}

	if config.OllamaURL != "" {
		providers = append(providers, ProviderInfo{
			Name:      "ollama",
			Display:   "Ollama",
			IsDefault: config.Provider == "ollama",
		})
	}

	response := ProvidersResponse{
		Providers:    providers,
		DefaultModel: config.Model,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to encode LLM providers response: %v\n", err)
	}
}

// HandleModels handles GET /api/llm/models?provider=<provider>
func HandleModels(w http.ResponseWriter, r *http.Request, config *Config) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	provider := r.URL.Query().Get("provider")
	if provider == "" {
		http.Error(w, "Provider parameter is required", http.StatusBadRequest)
		return
	}

	// Create LLM client for the provider
	var client chat.LLMClient
	switch provider {
	case "anthropic":
		if config.AnthropicAPIKey == "" {
			http.Error(w, "Anthropic API key not configured", http.StatusBadRequest)
			return
		}
		client = chat.NewAnthropicClient(config.AnthropicAPIKey, config.Model, config.MaxTokens, config.Temperature)
	case "openai":
		if config.OpenAIAPIKey == "" {
			http.Error(w, "OpenAI API key not configured", http.StatusBadRequest)
			return
		}
		client = chat.NewOpenAIClient(config.OpenAIAPIKey, config.Model, config.MaxTokens, config.Temperature)
	case "ollama":
		if config.OllamaURL == "" {
			http.Error(w, "Ollama URL not configured", http.StatusBadRequest)
			return
		}
		client = chat.NewOllamaClient(config.OllamaURL, config.Model)
	default:
		http.Error(w, fmt.Sprintf("Unsupported provider: %s", provider), http.StatusBadRequest)
		return
	}

	// List models
	modelNames, err := client.ListModels(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list models: %v", err), http.StatusInternalServerError)
		return
	}

	// Convert to model info
	models := make([]ModelInfo, len(modelNames))
	for i, name := range modelNames {
		models[i] = ModelInfo{
			Name: name,
		}
	}

	response := ModelsResponse{
		Models: models,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to encode LLM models response: %v\n", err)
	}
}

// HandleChat handles POST /api/llm/chat
func HandleChat(w http.ResponseWriter, r *http.Request, config *Config) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Ensure request body is closed
	defer func() {
		if err := r.Body.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: Failed to close request body: %v\n", err)
		}
	}()

	// Parse request body
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Use provided provider/model or defaults
	provider := req.Provider
	if provider == "" {
		provider = config.Provider
	}

	model := req.Model
	if model == "" {
		model = config.Model
	}

	// Create LLM client
	var client chat.LLMClient
	switch provider {
	case "anthropic":
		if config.AnthropicAPIKey == "" {
			http.Error(w, "Anthropic API key not configured", http.StatusBadRequest)
			return
		}
		client = chat.NewAnthropicClient(config.AnthropicAPIKey, model, config.MaxTokens, config.Temperature)
	case "openai":
		if config.OpenAIAPIKey == "" {
			http.Error(w, "OpenAI API key not configured", http.StatusBadRequest)
			return
		}
		client = chat.NewOpenAIClient(config.OpenAIAPIKey, model, config.MaxTokens, config.Temperature)
	case "ollama":
		if config.OllamaURL == "" {
			http.Error(w, "Ollama URL not configured", http.StatusBadRequest)
			return
		}
		client = chat.NewOllamaClient(config.OllamaURL, model)
	default:
		http.Error(w, fmt.Sprintf("Unsupported provider: %s", provider), http.StatusBadRequest)
		return
	}

	// Convert proxy messages to chat messages
	chatMessages := make([]chat.Message, len(req.Messages))
	for i, msg := range req.Messages {
		chatMessages[i] = chat.Message{
			Role:         msg.Role,
			Content:      msg.Content,
			CacheControl: msg.CacheControl,
		}
	}

	// Call LLM - pass tools as []interface{} to avoid import cycle
	// The chat client will access tool fields which are structurally identical to mcp.Tool
	ctx := context.Background()
	llmResponse, err := client.Chat(ctx, chatMessages, req.Tools)
	if err != nil {
		http.Error(w, fmt.Sprintf("LLM error: %v", err), http.StatusInternalServerError)
		return
	}

	// Return response
	response := ChatResponse{
		Content:    llmResponse.Content,
		StopReason: llmResponse.StopReason,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to encode LLM chat response: %v\n", err)
	}
}
