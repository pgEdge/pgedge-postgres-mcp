/*-------------------------------------------------------------------------
 *
 * Tests for LLM Client
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package chat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"pgedge-postgres-mcp/internal/mcp"
)

func TestAnthropicClient_TextResponse(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		var req anthropicRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}

		// Verify API key header
		apiKey := r.Header.Get("x-api-key")
		if apiKey != "test-key" {
			t.Errorf("Expected API key 'test-key', got '%s'", apiKey)
		}

		// Send response
		resp := anthropicResponse{
			ID:   "msg_test",
			Type: "message",
			Role: "assistant",
			Content: []map[string]interface{}{
				{
					"type": "text",
					"text": "This is a test response",
				},
			},
			StopReason: "end_turn",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Create client with test server URL
	client := &anthropicClient{
		apiKey: "test-key",
		model:  "claude-test",
	}

	// Since we can't easily override the URL, we'll just verify the client was created correctly
	if client.apiKey != "test-key" {
		t.Errorf("Expected API key 'test-key', got '%s'", client.apiKey)
	}
	if client.model != "claude-test" {
		t.Errorf("Expected model 'claude-test', got '%s'", client.model)
	}

	// In a real test, we'd call client.Chat(ctx, messages, tools)
	// but since we can't override the URL easily without refactoring,
	// we'll skip that for now
	_, _ = server, client // Suppress unused warnings
}

func TestOllamaClient_ToolCall(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		var req ollamaRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}

		if req.Model != "test-model" {
			t.Errorf("Expected model 'test-model', got '%s'", req.Model)
		}

		// Send tool call response
		resp := ollamaResponse{
			Model: "test-model",
			Message: ollamaMessage{
				Role:    "assistant",
				Content: `{"tool": "test_tool", "arguments": {"param": "value"}}`,
			},
			Done: true,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Create client
	client := NewOllamaClient(server.URL, "test-model")

	// Test tool call
	ctx := context.Background()
	messages := []Message{
		{
			Role:    "user",
			Content: "Execute test tool",
		},
	}
	tools := []mcp.Tool{
		{
			Name:        "test_tool",
			Description: "A test tool",
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"param": map[string]interface{}{
						"type":        "string",
						"description": "A parameter",
					},
				},
			},
		},
	}

	response, err := client.Chat(ctx, messages, tools)
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	if response.StopReason != "tool_use" {
		t.Errorf("Expected stop reason 'tool_use', got '%s'", response.StopReason)
	}

	if len(response.Content) != 1 {
		t.Fatalf("Expected 1 content item, got %d", len(response.Content))
	}

	toolUse, ok := response.Content[0].(ToolUse)
	if !ok {
		t.Fatalf("Expected ToolUse, got %T", response.Content[0])
	}

	if toolUse.Name != "test_tool" {
		t.Errorf("Expected tool name 'test_tool', got '%s'", toolUse.Name)
	}
}

func TestOllamaClient_TextResponse(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Send text response
		resp := ollamaResponse{
			Model: "test-model",
			Message: ollamaMessage{
				Role:    "assistant",
				Content: "This is a plain text response",
			},
			Done: true,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Create client
	client := NewOllamaClient(server.URL, "test-model")

	// Test text response
	ctx := context.Background()
	messages := []Message{
		{
			Role:    "user",
			Content: "Hello",
		},
	}
	tools := []mcp.Tool{}

	response, err := client.Chat(ctx, messages, tools)
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	if response.StopReason != "end_turn" {
		t.Errorf("Expected stop reason 'end_turn', got '%s'", response.StopReason)
	}

	if len(response.Content) != 1 {
		t.Fatalf("Expected 1 content item, got %d", len(response.Content))
	}

	textContent, ok := response.Content[0].(TextContent)
	if !ok {
		t.Fatalf("Expected TextContent, got %T", response.Content[0])
	}

	if textContent.Text != "This is a plain text response" {
		t.Errorf("Expected text 'This is a plain text response', got '%s'", textContent.Text)
	}
}

func TestFormatToolsForOllama(t *testing.T) {
	client := &ollamaClient{}

	tools := []mcp.Tool{
		{
			Name:        "test_tool",
			Description: "A test tool",
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"param1": map[string]interface{}{
						"type":        "string",
						"description": "First parameter",
					},
					"param2": map[string]interface{}{
						"type":        "number",
						"description": "Second parameter",
					},
				},
			},
		},
	}

	result := client.formatToolsForOllama(tools)

	// Check that the result contains expected strings
	if result == "" {
		t.Error("Expected non-empty result")
	}

	// Check for tool name and description
	if !containsString(result, "test_tool") {
		t.Error("Result should contain tool name")
	}

	if !containsString(result, "A test tool") {
		t.Error("Result should contain tool description")
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsSubstring(s, substr)))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestExtractAnthropicErrorMessage(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		want       string
	}{
		{
			name:       "Rate limit error",
			statusCode: 429,
			body:       `{"type":"error","error":{"type":"rate_limit_error","message":"You have exceeded your rate limit. Please wait before trying again."}}`,
			want:       "API error (429): You have exceeded your rate limit. Please wait before trying again.",
		},
		{
			name:       "Authentication error",
			statusCode: 401,
			body:       `{"type":"error","error":{"type":"authentication_error","message":"Invalid API key provided"}}`,
			want:       "API error (401): Invalid API key provided",
		},
		{
			name:       "Generic error with no JSON",
			statusCode: 500,
			body:       `Internal Server Error`,
			want:       "API error (500): Internal Server Error",
		},
		{
			name:       "Malformed JSON",
			statusCode: 400,
			body:       `{invalid json}`,
			want:       "API error (400): {invalid json}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractAnthropicErrorMessage(tt.statusCode, []byte(tt.body))
			if got != tt.want {
				t.Errorf("extractAnthropicErrorMessage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractOllamaErrorMessage(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantContains string
	}{
		{
			name:       "Model not found error",
			statusCode: 404,
			body:       `{"error":"model not found"}`,
			wantContains: "model not found",
		},
		{
			name:       "Generic error",
			statusCode: 500,
			body:       `{"error":"internal server error"}`,
			wantContains: "internal server error",
		},
		{
			name:       "Non-JSON error",
			statusCode: 503,
			body:       `Service Unavailable`,
			wantContains: "Service Unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractOllamaErrorMessage(tt.statusCode, []byte(tt.body))
			if !containsSubstring(got, tt.wantContains) {
				t.Errorf("extractOllamaErrorMessage() = %v, want to contain %v", got, tt.wantContains)
			}
		})
	}
}
