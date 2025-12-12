/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package mcp

import (
	"encoding/json"
	"testing"
)

func TestJSONRPCRequestMarshal(t *testing.T) {
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "test/method",
		Params:  map[string]string{"key": "value"},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded JSONRPCRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.JSONRPC != req.JSONRPC {
		t.Errorf("expected jsonrpc %q, got %q", req.JSONRPC, decoded.JSONRPC)
	}
	if decoded.Method != req.Method {
		t.Errorf("expected method %q, got %q", req.Method, decoded.Method)
	}
}

func TestJSONRPCRequestWithNilParams(t *testing.T) {
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "test/method",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Should not include "params" in JSON when nil
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if _, exists := raw["params"]; exists {
		t.Error("params should be omitted when nil")
	}
}

func TestJSONRPCResponseMarshal(t *testing.T) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result:  map[string]string{"status": "ok"},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded JSONRPCResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.JSONRPC != resp.JSONRPC {
		t.Errorf("expected jsonrpc %q, got %q", resp.JSONRPC, decoded.JSONRPC)
	}
}

func TestJSONRPCResponseWithError(t *testing.T) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      1,
		Error: &RPCError{
			Code:    -32600,
			Message: "Invalid Request",
			Data:    "test data",
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded JSONRPCResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Error == nil {
		t.Fatal("expected error to be non-nil")
	}
	if decoded.Error.Code != -32600 {
		t.Errorf("expected code -32600, got %d", decoded.Error.Code)
	}
	if decoded.Error.Message != "Invalid Request" {
		t.Errorf("expected message 'Invalid Request', got %q", decoded.Error.Message)
	}
}

func TestRPCErrorMarshal(t *testing.T) {
	err := RPCError{
		Code:    -32601,
		Message: "Method not found",
	}

	data, errMarshal := json.Marshal(err)
	if errMarshal != nil {
		t.Fatalf("failed to marshal: %v", errMarshal)
	}

	var decoded RPCError
	if errUnmarshal := json.Unmarshal(data, &decoded); errUnmarshal != nil {
		t.Fatalf("failed to unmarshal: %v", errUnmarshal)
	}

	if decoded.Code != err.Code {
		t.Errorf("expected code %d, got %d", err.Code, decoded.Code)
	}
	if decoded.Message != err.Message {
		t.Errorf("expected message %q, got %q", err.Message, decoded.Message)
	}
}

func TestInitializeParamsMarshal(t *testing.T) {
	params := InitializeParams{
		ProtocolVersion: "2024-11-05",
		Capabilities:    map[string]interface{}{"tools": true},
		ClientInfo: ClientInfo{
			Name:    "test-client",
			Version: "1.0.0",
		},
	}

	data, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded InitializeParams
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.ProtocolVersion != params.ProtocolVersion {
		t.Errorf("expected protocol version %q, got %q",
			params.ProtocolVersion, decoded.ProtocolVersion)
	}
	if decoded.ClientInfo.Name != params.ClientInfo.Name {
		t.Errorf("expected client name %q, got %q",
			params.ClientInfo.Name, decoded.ClientInfo.Name)
	}
}

func TestToolMarshal(t *testing.T) {
	tool := Tool{
		Name:        "test_tool",
		Description: "A test tool",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "The query to execute",
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

	if decoded.Name != tool.Name {
		t.Errorf("expected name %q, got %q", tool.Name, decoded.Name)
	}
	if decoded.Description != tool.Description {
		t.Errorf("expected description %q, got %q", tool.Description, decoded.Description)
	}
	if len(decoded.InputSchema.Required) != 1 || decoded.InputSchema.Required[0] != "query" {
		t.Errorf("expected required [query], got %v", decoded.InputSchema.Required)
	}
}

func TestToolResponseMarshal(t *testing.T) {
	resp := ToolResponse{
		Content: []ContentItem{
			{Type: "text", Text: "result"},
		},
		IsError: false,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded ToolResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(decoded.Content) != 1 {
		t.Fatalf("expected 1 content item, got %d", len(decoded.Content))
	}
	if decoded.Content[0].Text != "result" {
		t.Errorf("expected text 'result', got %q", decoded.Content[0].Text)
	}
	if decoded.IsError {
		t.Error("expected IsError to be false")
	}
}

func TestResourceMarshal(t *testing.T) {
	resource := Resource{
		URI:         "pg://schema",
		Name:        "schema",
		Description: "Database schema",
		MimeType:    "application/json",
	}

	data, err := json.Marshal(resource)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded Resource
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.URI != resource.URI {
		t.Errorf("expected URI %q, got %q", resource.URI, decoded.URI)
	}
	if decoded.MimeType != resource.MimeType {
		t.Errorf("expected mimeType %q, got %q", resource.MimeType, decoded.MimeType)
	}
}

func TestPromptMarshal(t *testing.T) {
	prompt := Prompt{
		Name:        "test-prompt",
		Description: "A test prompt",
		Arguments: []PromptArgument{
			{Name: "query", Description: "The query", Required: true},
			{Name: "format", Description: "Output format", Required: false},
		},
	}

	data, err := json.Marshal(prompt)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded Prompt
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Name != prompt.Name {
		t.Errorf("expected name %q, got %q", prompt.Name, decoded.Name)
	}
	if len(decoded.Arguments) != 2 {
		t.Fatalf("expected 2 arguments, got %d", len(decoded.Arguments))
	}
	if !decoded.Arguments[0].Required {
		t.Error("expected first argument to be required")
	}
}

func TestPromptResultMarshal(t *testing.T) {
	result := PromptResult{
		Description: "Test prompt result",
		Messages: []PromptMessage{
			{
				Role:    "user",
				Content: ContentItem{Type: "text", Text: "Hello"},
			},
			{
				Role:    "assistant",
				Content: ContentItem{Type: "text", Text: "Hi there!"},
			},
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded PromptResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(decoded.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(decoded.Messages))
	}
	if decoded.Messages[0].Role != "user" {
		t.Errorf("expected first message role 'user', got %q", decoded.Messages[0].Role)
	}
	if decoded.Messages[1].Role != "assistant" {
		t.Errorf("expected second message role 'assistant', got %q", decoded.Messages[1].Role)
	}
}

func TestToolsListResultMarshal(t *testing.T) {
	result := ToolsListResult{
		Tools: []Tool{
			{Name: "tool1", Description: "First tool"},
			{Name: "tool2", Description: "Second tool"},
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded ToolsListResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(decoded.Tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(decoded.Tools))
	}
}

func TestResourcesListResultMarshal(t *testing.T) {
	result := ResourcesListResult{
		Resources: []Resource{
			{URI: "pg://schema", Name: "schema"},
			{URI: "pg://tables", Name: "tables"},
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded ResourcesListResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(decoded.Resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(decoded.Resources))
	}
}

func TestPromptsListResultMarshal(t *testing.T) {
	result := PromptsListResult{
		Prompts: []Prompt{
			{Name: "prompt1", Description: "First prompt"},
			{Name: "prompt2", Description: "Second prompt"},
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded PromptsListResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(decoded.Prompts) != 2 {
		t.Fatalf("expected 2 prompts, got %d", len(decoded.Prompts))
	}
}

func TestResourceContentMarshal(t *testing.T) {
	content := ResourceContent{
		URI:      "pg://test",
		MimeType: "application/json",
		Contents: []ContentItem{
			{Type: "text", Text: `{"data": "test"}`},
		},
	}

	data, err := json.Marshal(content)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded ResourceContent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.URI != content.URI {
		t.Errorf("expected URI %q, got %q", content.URI, decoded.URI)
	}
	if len(decoded.Contents) != 1 {
		t.Fatalf("expected 1 content item, got %d", len(decoded.Contents))
	}
}

func TestToolCallParamsMarshal(t *testing.T) {
	params := ToolCallParams{
		Name: "query_database",
		Arguments: map[string]interface{}{
			"query":    "SELECT * FROM users",
			"readonly": true,
		},
	}

	data, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded ToolCallParams
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Name != params.Name {
		t.Errorf("expected name %q, got %q", params.Name, decoded.Name)
	}
	if decoded.Arguments["query"] != "SELECT * FROM users" {
		t.Errorf("expected query argument, got %v", decoded.Arguments)
	}
}
