/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent - LLM Proxy Tests
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package llmproxy

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleProviders_Success(t *testing.T) {
	config := &Config{
		Provider:        "anthropic",
		Model:           "claude-sonnet-4-20250514",
		AnthropicAPIKey: "test-key",
		OpenAIAPIKey:    "test-openai-key",
	}

	req := httptest.NewRequest(http.MethodGet, "/api/llm/providers", nil)
	w := httptest.NewRecorder()

	HandleProviders(w, req, config)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response ProvidersResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Providers) != 2 {
		t.Errorf("expected 2 providers, got %d", len(response.Providers))
	}

	if response.DefaultModel != "claude-sonnet-4-20250514" {
		t.Errorf("expected default model 'claude-sonnet-4-20250514', got %q", response.DefaultModel)
	}

	// Check that anthropic is marked as default
	anthropicFound := false
	for _, p := range response.Providers {
		if p.Name == "anthropic" {
			anthropicFound = true
			if !p.IsDefault {
				t.Error("expected anthropic to be marked as default")
			}
		}
	}
	if !anthropicFound {
		t.Error("expected anthropic provider in list")
	}
}

func TestHandleProviders_MethodNotAllowed(t *testing.T) {
	config := &Config{}

	req := httptest.NewRequest(http.MethodPost, "/api/llm/providers", nil)
	w := httptest.NewRecorder()

	HandleProviders(w, req, config)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestHandleProviders_NoProviders(t *testing.T) {
	config := &Config{}

	req := httptest.NewRequest(http.MethodGet, "/api/llm/providers", nil)
	w := httptest.NewRecorder()

	HandleProviders(w, req, config)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response ProvidersResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Providers) != 0 {
		t.Errorf("expected 0 providers, got %d", len(response.Providers))
	}
}

func TestHandleProviders_AllProviders(t *testing.T) {
	config := &Config{
		Provider:        "openai",
		Model:           "gpt-4o",
		AnthropicAPIKey: "anthropic-key",
		OpenAIAPIKey:    "openai-key",
		OllamaURL:       "http://localhost:11434",
	}

	req := httptest.NewRequest(http.MethodGet, "/api/llm/providers", nil)
	w := httptest.NewRecorder()

	HandleProviders(w, req, config)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response ProvidersResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Providers) != 3 {
		t.Errorf("expected 3 providers, got %d", len(response.Providers))
	}

	// Check that openai is marked as default
	for _, p := range response.Providers {
		if p.Name == "openai" && !p.IsDefault {
			t.Error("expected openai to be marked as default")
		}
		if p.Name != "openai" && p.IsDefault {
			t.Errorf("expected %s to not be marked as default", p.Name)
		}
	}
}

func TestHandleModels_MethodNotAllowed(t *testing.T) {
	config := &Config{}

	req := httptest.NewRequest(http.MethodPost, "/api/llm/models", nil)
	w := httptest.NewRecorder()

	HandleModels(w, req, config)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestHandleModels_MissingProvider(t *testing.T) {
	config := &Config{}

	req := httptest.NewRequest(http.MethodGet, "/api/llm/models", nil)
	w := httptest.NewRecorder()

	HandleModels(w, req, config)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandleModels_UnsupportedProvider(t *testing.T) {
	config := &Config{}

	req := httptest.NewRequest(http.MethodGet, "/api/llm/models?provider=unsupported", nil)
	w := httptest.NewRecorder()

	HandleModels(w, req, config)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandleModels_AnthropicNotConfigured(t *testing.T) {
	config := &Config{}

	req := httptest.NewRequest(http.MethodGet, "/api/llm/models?provider=anthropic", nil)
	w := httptest.NewRecorder()

	HandleModels(w, req, config)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandleModels_OpenAINotConfigured(t *testing.T) {
	config := &Config{}

	req := httptest.NewRequest(http.MethodGet, "/api/llm/models?provider=openai", nil)
	w := httptest.NewRecorder()

	HandleModels(w, req, config)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandleModels_OllamaNotConfigured(t *testing.T) {
	config := &Config{}

	req := httptest.NewRequest(http.MethodGet, "/api/llm/models?provider=ollama", nil)
	w := httptest.NewRecorder()

	HandleModels(w, req, config)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandleChat_MethodNotAllowed(t *testing.T) {
	config := &Config{}

	req := httptest.NewRequest(http.MethodGet, "/api/llm/chat", nil)
	w := httptest.NewRecorder()

	HandleChat(w, req, config)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestHandleChat_InvalidBody(t *testing.T) {
	config := &Config{
		Provider: "anthropic",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/llm/chat",
		bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	HandleChat(w, req, config)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandleChat_UnsupportedProvider(t *testing.T) {
	config := &Config{
		Provider: "unsupported",
	}

	body := ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hello"}},
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/llm/chat",
		bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	HandleChat(w, req, config)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandleChat_AnthropicNotConfigured(t *testing.T) {
	config := &Config{
		Provider: "anthropic",
		// No API key
	}

	body := ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hello"}},
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/llm/chat",
		bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	HandleChat(w, req, config)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandleChat_OpenAINotConfigured(t *testing.T) {
	config := &Config{
		Provider: "openai",
		// No API key
	}

	body := ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hello"}},
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/llm/chat",
		bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	HandleChat(w, req, config)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandleChat_OllamaNotConfigured(t *testing.T) {
	config := &Config{
		Provider: "ollama",
		// No URL
	}

	body := ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hello"}},
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/llm/chat",
		bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	HandleChat(w, req, config)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandleChat_OverrideProvider(t *testing.T) {
	config := &Config{
		Provider:     "anthropic",
		OpenAIAPIKey: "test-key",
	}

	// Override to openai
	body := ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hello"}},
		Provider: "openai",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/llm/chat",
		bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	HandleChat(w, req, config)

	// Should fail because we don't have a real API key, but it should get
	// past the "not configured" check since OpenAI key is set
	// The error will be from the actual API call
	if w.Code == http.StatusBadRequest {
		// Check that it's not because openai is "not configured"
		body := w.Body.String()
		if body == "OpenAI API key not configured\n" {
			t.Error("should not get 'not configured' error when key is set")
		}
	}
}

// Test struct serialization
func TestConfigStruct(t *testing.T) {
	config := Config{
		Provider:        "anthropic",
		Model:           "claude-sonnet-4-20250514",
		AnthropicAPIKey: "test-key",
		MaxTokens:       4096,
		Temperature:     0.7,
	}

	if config.Provider != "anthropic" {
		t.Errorf("expected provider 'anthropic', got %q", config.Provider)
	}
	if config.MaxTokens != 4096 {
		t.Errorf("expected max tokens 4096, got %d", config.MaxTokens)
	}
	if config.Model != "claude-sonnet-4-20250514" {
		t.Errorf("expected model 'claude-sonnet-4-20250514', got %q", config.Model)
	}
	if config.AnthropicAPIKey != "test-key" {
		t.Errorf("expected AnthropicAPIKey 'test-key', got %q", config.AnthropicAPIKey)
	}
	if config.Temperature != 0.7 {
		t.Errorf("expected temperature 0.7, got %f", config.Temperature)
	}
}

func TestMessageStruct(t *testing.T) {
	msg := Message{
		Role:    "user",
		Content: "Hello, world!",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded Message
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Role != "user" {
		t.Errorf("expected role 'user', got %q", decoded.Role)
	}
	if decoded.Content != "Hello, world!" {
		t.Errorf("expected content 'Hello, world!', got %v", decoded.Content)
	}
}

func TestToolStruct(t *testing.T) {
	tool := Tool{
		Name:        "query_database",
		Description: "Execute a SQL query",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "The SQL query to execute",
				},
			},
			Required: []string{"query"},
		},
	}

	data, err := json.Marshal(tool)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded Tool
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Name != "query_database" {
		t.Errorf("expected name 'query_database', got %q", decoded.Name)
	}
	if len(decoded.InputSchema.Required) != 1 {
		t.Errorf("expected 1 required field, got %d", len(decoded.InputSchema.Required))
	}
}

func TestProviderInfoStruct(t *testing.T) {
	info := ProviderInfo{
		Name:      "anthropic",
		Display:   "Anthropic Claude",
		IsDefault: true,
	}

	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded ProviderInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Name != "anthropic" {
		t.Errorf("expected name 'anthropic', got %q", decoded.Name)
	}
	if !decoded.IsDefault {
		t.Error("expected isDefault to be true")
	}
}

func TestChatRequestStruct(t *testing.T) {
	req := ChatRequest{
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
		Tools:    []Tool{},
		Provider: "anthropic",
		Model:    "claude-sonnet-4-20250514",
		Debug:    true,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded ChatRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(decoded.Messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(decoded.Messages))
	}
	if !decoded.Debug {
		t.Error("expected debug to be true")
	}
}

func TestChatResponseStruct(t *testing.T) {
	resp := ChatResponse{
		Content: []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": "Hello!",
			},
		},
		StopReason: "end_turn",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded ChatResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.StopReason != "end_turn" {
		t.Errorf("expected stop_reason 'end_turn', got %q", decoded.StopReason)
	}
	if len(decoded.Content) != 1 {
		t.Errorf("expected 1 content item, got %d", len(decoded.Content))
	}
}

func TestModelsResponseStruct(t *testing.T) {
	resp := ModelsResponse{
		Models: []ModelInfo{
			{Name: "claude-sonnet-4-20250514", Description: "Balanced model"},
			{Name: "claude-opus-4-20250514", Description: "Most capable"},
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded ModelsResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(decoded.Models) != 2 {
		t.Errorf("expected 2 models, got %d", len(decoded.Models))
	}
}
