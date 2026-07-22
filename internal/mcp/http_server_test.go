/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleHealthCheck(t *testing.T) {
	tools := &mockToolProvider{}
	server := NewServer(tools)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	server.handleHealthCheck(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", contentType)
	}

	body := w.Body.String()
	if body == "" {
		t.Error("expected non-empty response body")
	}

	// Parse response
	var response map[string]string
	if err := json.Unmarshal([]byte(body), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response["status"] != "ok" {
		t.Errorf("expected status 'ok', got %q", response["status"])
	}
	if response["server"] != ServerName {
		t.Errorf("expected server %q, got %q", ServerName, response["server"])
	}
	if response["version"] != ServerVersion {
		t.Errorf("expected version %q, got %q", ServerVersion, response["version"])
	}
}

func TestHandleHTTPRequest_MethodNotAllowed(t *testing.T) {
	tools := &mockToolProvider{}
	server := NewServer(tools)

	// Test GET request (should be rejected)
	req := httptest.NewRequest(http.MethodGet, "/mcp/v1", nil)
	w := httptest.NewRecorder()

	server.handleHTTPRequest(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
	if allow := w.Header().Get("Allow"); allow != http.MethodPost {
		t.Errorf("expected Allow header %q, got %q", http.MethodPost, allow)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("response body is not valid JSON: %v (body: %q)", err, w.Body.String())
	}
	if body["error"] == "" {
		t.Error("expected non-empty error message")
	}
}

func TestHandleHTTPRequest_OversizedBody(t *testing.T) {
	tools := &mockToolProvider{}
	server := NewServer(tools)

	oversized := bytes.Repeat([]byte("x"), MaxRequestBodySize+1)
	req := httptest.NewRequest(http.MethodPost, "/mcp/v1", bytes.NewReader(oversized))
	w := httptest.NewRecorder()

	server.handleHTTPRequest(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected status 413, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("response body is not valid JSON: %v (body: %q)", err, w.Body.String())
	}
	if body["error"] == "" {
		t.Error("expected non-empty error message")
	}
}

func TestHandleHTTPRequest_InvalidJSON(t *testing.T) {
	tools := &mockToolProvider{}
	server := NewServer(tools)

	req := httptest.NewRequest(http.MethodPost, "/mcp/v1",
		bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleHTTPRequest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 (JSON-RPC errors use 200), got %d", w.Code)
	}

	var response JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Error == nil {
		t.Fatal("expected error response")
	}
	if response.Error.Code != -32700 {
		t.Errorf("expected parse error code -32700, got %d", response.Error.Code)
	}
}

func TestHandleInitializeHTTP(t *testing.T) {
	tools := &mockToolProvider{}
	server := NewServer(tools)

	rpcReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"clientInfo": map[string]interface{}{
				"name":    "test-client",
				"version": "1.0.0",
			},
		},
	}

	body, _ := json.Marshal(rpcReq)
	req := httptest.NewRequest(http.MethodPost, "/mcp/v1", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleHTTPRequest(w, req)

	var response JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Error != nil {
		t.Fatalf("unexpected error: %v", response.Error)
	}

	result, ok := response.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected result to be a map, got %T", response.Result)
	}

	serverInfo, ok := result["serverInfo"].(map[string]interface{})
	if !ok {
		t.Fatal("expected serverInfo in result")
	}

	if serverInfo["name"] != ServerName {
		t.Errorf("expected server name %q, got %q", ServerName, serverInfo["name"])
	}
}

func TestHandleInitializeHTTP_WithProviders(t *testing.T) {
	tools := &mockToolProvider{}
	server := NewServer(tools)
	server.SetResourceProvider(&mockResourceProvider{})
	server.SetPromptProvider(&mockPromptProvider{})

	rpcReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
	}

	body, _ := json.Marshal(rpcReq)
	req := httptest.NewRequest(http.MethodPost, "/mcp/v1", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleHTTPRequest(w, req)

	var response JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	result, ok := response.Result.(map[string]interface{})
	if !ok {
		t.Fatal("expected result to be a map")
	}

	capabilities, ok := result["capabilities"].(map[string]interface{})
	if !ok {
		t.Fatal("expected capabilities in result")
	}

	// Should have all capabilities
	if _, ok := capabilities["tools"]; !ok {
		t.Error("expected tools capability")
	}
	if _, ok := capabilities["resources"]; !ok {
		t.Error("expected resources capability")
	}
	if _, ok := capabilities["prompts"]; !ok {
		t.Error("expected prompts capability")
	}
}

func TestHandleToolsListHTTP(t *testing.T) {
	tools := &mockToolProvider{
		tools: []Tool{
			{Name: "tool1", Description: "First tool"},
			{Name: "tool2", Description: "Second tool"},
		},
	}
	server := NewServer(tools)

	rpcReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
	}

	body, _ := json.Marshal(rpcReq)
	req := httptest.NewRequest(http.MethodPost, "/mcp/v1", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleHTTPRequest(w, req)

	var response JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Error != nil {
		t.Fatalf("unexpected error: %v", response.Error)
	}

	result, ok := response.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected result to be a map, got %T", response.Result)
	}

	toolsList, ok := result["tools"].([]interface{})
	if !ok {
		t.Fatal("expected tools array in result")
	}

	if len(toolsList) != 2 {
		t.Errorf("expected 2 tools, got %d", len(toolsList))
	}
}

func TestHandleToolCallHTTP_Success(t *testing.T) {
	tools := &mockToolProvider{
		executeFunc: func(ctx context.Context, name string, args map[string]interface{}) (ToolResponse, error) {
			return NewToolSuccess("executed " + name)
		},
	}
	server := NewServer(tools)

	rpcReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name":      "test_tool",
			"arguments": map[string]interface{}{"key": "value"},
		},
	}

	body, _ := json.Marshal(rpcReq)
	req := httptest.NewRequest(http.MethodPost, "/mcp/v1", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleHTTPRequest(w, req)

	var response JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Error != nil {
		t.Fatalf("unexpected error: %v", response.Error)
	}
}

func TestHandleToolCallHTTP_ExecutionError(t *testing.T) {
	tools := &mockToolProvider{
		executeFunc: func(ctx context.Context, name string, args map[string]interface{}) (ToolResponse, error) {
			return ToolResponse{}, errors.New("execution failed")
		},
	}
	server := NewServer(tools)

	rpcReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name": "failing_tool",
		},
	}

	body, _ := json.Marshal(rpcReq)
	req := httptest.NewRequest(http.MethodPost, "/mcp/v1", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleHTTPRequest(w, req)

	var response JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Error == nil {
		t.Fatal("expected error response")
	}
	if response.Error.Code != -32603 {
		t.Errorf("expected internal error code -32603, got %d", response.Error.Code)
	}
}

func TestHandleResourcesListHTTP_NoProvider(t *testing.T) {
	tools := &mockToolProvider{}
	server := NewServer(tools)
	// No resource provider set

	rpcReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "resources/list",
	}

	body, _ := json.Marshal(rpcReq)
	req := httptest.NewRequest(http.MethodPost, "/mcp/v1", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleHTTPRequest(w, req)

	var response JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Error == nil {
		t.Fatal("expected error response")
	}
}

func TestHandleResourcesListHTTP_Success(t *testing.T) {
	tools := &mockToolProvider{}
	resources := &mockResourceProvider{
		resources: []Resource{
			{URI: "pg://schema", Name: "schema"},
		},
	}
	server := NewServer(tools)
	server.SetResourceProvider(resources)

	rpcReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "resources/list",
	}

	body, _ := json.Marshal(rpcReq)
	req := httptest.NewRequest(http.MethodPost, "/mcp/v1", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleHTTPRequest(w, req)

	var response JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Error != nil {
		t.Fatalf("unexpected error: %v", response.Error)
	}
}

func TestHandleResourceReadHTTP_Success(t *testing.T) {
	tools := &mockToolProvider{}
	resources := &mockResourceProvider{
		readFunc: func(ctx context.Context, uri string) (ResourceContent, error) {
			return NewResourceSuccess(uri, "application/json", `{"data": "test"}`)
		},
	}
	server := NewServer(tools)
	server.SetResourceProvider(resources)

	rpcReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "resources/read",
		Params: map[string]interface{}{
			"uri": "pg://schema",
		},
	}

	body, _ := json.Marshal(rpcReq)
	req := httptest.NewRequest(http.MethodPost, "/mcp/v1", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleHTTPRequest(w, req)

	var response JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Error != nil {
		t.Fatalf("unexpected error: %v", response.Error)
	}
}

func TestHandleResourceReadHTTP_Error(t *testing.T) {
	tools := &mockToolProvider{}
	resources := &mockResourceProvider{
		readFunc: func(ctx context.Context, uri string) (ResourceContent, error) {
			return ResourceContent{}, errors.New("resource not found")
		},
	}
	server := NewServer(tools)
	server.SetResourceProvider(resources)

	rpcReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "resources/read",
		Params: map[string]interface{}{
			"uri": "pg://invalid",
		},
	}

	body, _ := json.Marshal(rpcReq)
	req := httptest.NewRequest(http.MethodPost, "/mcp/v1", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleHTTPRequest(w, req)

	var response JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Error == nil {
		t.Fatal("expected error response")
	}
}

func TestHandlePromptsListHTTP_NoProvider(t *testing.T) {
	tools := &mockToolProvider{}
	server := NewServer(tools)

	rpcReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "prompts/list",
	}

	body, _ := json.Marshal(rpcReq)
	req := httptest.NewRequest(http.MethodPost, "/mcp/v1", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleHTTPRequest(w, req)

	var response JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Error == nil {
		t.Fatal("expected error response")
	}
}

func TestHandlePromptsListHTTP_Success(t *testing.T) {
	tools := &mockToolProvider{}
	prompts := &mockPromptProvider{
		prompts: []Prompt{
			{Name: "prompt1", Description: "First prompt"},
		},
	}
	server := NewServer(tools)
	server.SetPromptProvider(prompts)

	rpcReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "prompts/list",
	}

	body, _ := json.Marshal(rpcReq)
	req := httptest.NewRequest(http.MethodPost, "/mcp/v1", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleHTTPRequest(w, req)

	var response JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Error != nil {
		t.Fatalf("unexpected error: %v", response.Error)
	}
}

func TestHandlePromptGetHTTP_Success(t *testing.T) {
	tools := &mockToolProvider{}
	prompts := &mockPromptProvider{
		executeFunc: func(name string, args map[string]string) (PromptResult, error) {
			return PromptResult{
				Description: "Test prompt",
				Messages: []PromptMessage{
					{Role: "user", Content: ContentItem{Type: "text", Text: "Hello"}},
				},
			}, nil
		},
	}
	server := NewServer(tools)
	server.SetPromptProvider(prompts)

	rpcReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "prompts/get",
		Params: map[string]interface{}{
			"name":      "test_prompt",
			"arguments": map[string]string{"key": "value"},
		},
	}

	body, _ := json.Marshal(rpcReq)
	req := httptest.NewRequest(http.MethodPost, "/mcp/v1", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleHTTPRequest(w, req)

	var response JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Error != nil {
		t.Fatalf("unexpected error: %v", response.Error)
	}
}

func TestHandlePromptGetHTTP_Error(t *testing.T) {
	tools := &mockToolProvider{}
	prompts := &mockPromptProvider{
		executeFunc: func(name string, args map[string]string) (PromptResult, error) {
			return PromptResult{}, errors.New("prompt not found")
		},
	}
	server := NewServer(tools)
	server.SetPromptProvider(prompts)

	rpcReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "prompts/get",
		Params: map[string]interface{}{
			"name": "invalid",
		},
	}

	body, _ := json.Marshal(rpcReq)
	req := httptest.NewRequest(http.MethodPost, "/mcp/v1", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleHTTPRequest(w, req)

	var response JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Error == nil {
		t.Fatal("expected error response")
	}
}

func TestHandleListDatabasesHTTP_NoProvider(t *testing.T) {
	tools := &mockToolProvider{}
	server := NewServer(tools)

	rpcReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "pgedge/listDatabases",
	}

	body, _ := json.Marshal(rpcReq)
	req := httptest.NewRequest(http.MethodPost, "/mcp/v1", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleHTTPRequest(w, req)

	var response JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Error == nil {
		t.Fatal("expected error response")
	}
}

func TestHandleListDatabasesHTTP_Success(t *testing.T) {
	tools := &mockToolProvider{}
	databases := &mockDatabaseProvider{
		databases: []DatabaseInfo{
			{Name: "db1", Host: "localhost", Port: 5432},
		},
		current: "db1",
	}
	server := NewServer(tools)
	server.SetDatabaseProvider(databases)

	rpcReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "pgedge/listDatabases",
	}

	body, _ := json.Marshal(rpcReq)
	req := httptest.NewRequest(http.MethodPost, "/mcp/v1", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleHTTPRequest(w, req)

	var response JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Error != nil {
		t.Fatalf("unexpected error: %v", response.Error)
	}
}

func TestHandleListDatabasesHTTP_Error(t *testing.T) {
	tools := &mockToolProvider{}
	databases := &mockDatabaseProvider{
		listErr: errors.New("connection failed"),
	}
	server := NewServer(tools)
	server.SetDatabaseProvider(databases)

	rpcReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "pgedge/listDatabases",
	}

	body, _ := json.Marshal(rpcReq)
	req := httptest.NewRequest(http.MethodPost, "/mcp/v1", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleHTTPRequest(w, req)

	var response JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Error == nil {
		t.Fatal("expected error response")
	}
}

func TestHandleSelectDatabaseHTTP_Success(t *testing.T) {
	tools := &mockToolProvider{}
	databases := &mockDatabaseProvider{
		databases: []DatabaseInfo{
			{Name: "db1", Host: "localhost", Port: 5432},
		},
		current: "db1",
	}
	server := NewServer(tools)
	server.SetDatabaseProvider(databases)

	rpcReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "pgedge/selectDatabase",
		Params: map[string]interface{}{
			"name": "db1",
		},
	}

	body, _ := json.Marshal(rpcReq)
	req := httptest.NewRequest(http.MethodPost, "/mcp/v1", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleHTTPRequest(w, req)

	var response JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Error != nil {
		t.Fatalf("unexpected error: %v", response.Error)
	}

	// Check response has success field
	result, ok := response.Result.(map[string]interface{})
	if !ok {
		t.Fatal("expected result to be a map")
	}
	if result["success"] != true {
		t.Errorf("expected success=true, got %v", result["success"])
	}
}

func TestHandleSelectDatabaseHTTP_EmptyName(t *testing.T) {
	tools := &mockToolProvider{}
	databases := &mockDatabaseProvider{}
	server := NewServer(tools)
	server.SetDatabaseProvider(databases)

	rpcReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "pgedge/selectDatabase",
		Params: map[string]interface{}{
			"name": "",
		},
	}

	body, _ := json.Marshal(rpcReq)
	req := httptest.NewRequest(http.MethodPost, "/mcp/v1", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleHTTPRequest(w, req)

	var response JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Error == nil {
		t.Fatal("expected error response for empty name")
	}
	if response.Error.Code != -32602 {
		t.Errorf("expected invalid params error -32602, got %d", response.Error.Code)
	}
}

func TestHandleSelectDatabaseHTTP_SelectError(t *testing.T) {
	tools := &mockToolProvider{}
	databases := &mockDatabaseProvider{
		selectErr: errors.New("database not found"),
	}
	server := NewServer(tools)
	server.SetDatabaseProvider(databases)

	rpcReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "pgedge/selectDatabase",
		Params: map[string]interface{}{
			"name": "invalid",
		},
	}

	body, _ := json.Marshal(rpcReq)
	req := httptest.NewRequest(http.MethodPost, "/mcp/v1", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleHTTPRequest(w, req)

	var response JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Error should be in result, not as RPC error
	if response.Error != nil {
		t.Fatal("expected error in result, not RPC error")
	}

	result, ok := response.Result.(map[string]interface{})
	if !ok {
		t.Fatal("expected result to be a map")
	}
	if result["success"] != false {
		t.Errorf("expected success=false, got %v", result["success"])
	}
}

func TestHandleNotificationsInitialized(t *testing.T) {
	tools := &mockToolProvider{}
	server := NewServer(tools)

	// A real JSON-RPC notification has NO "id" member (per §4.1). The server
	// must respond with 202 Accepted and an empty body.
	body := []byte(`{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}`)
	req := httptest.NewRequest(http.MethodPost, "/mcp/v1", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleHTTPRequest(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("expected status 202 Accepted for a notification, got %d", w.Code)
	}
	if w.Body.Len() != 0 {
		t.Errorf("expected empty body for a notification, got %q", w.Body.String())
	}
}

func TestHandleNotification_UnknownMethod(t *testing.T) {
	// Per the issue, sending -32601 Method not found in response to a
	// notification is doubly wrong: the server must not reply at all, and
	// the reply has no id (which is itself a malformed JSON-RPC body).
	tools := &mockToolProvider{}
	server := NewServer(tools)

	for _, method := range []string{
		"notifications/cancelled",
		"notifications/roots/list_changed",
		"some/unknown/notification",
	} {
		t.Run(method, func(t *testing.T) {
			body := []byte(`{"jsonrpc":"2.0","method":"` + method + `","params":{}}`)
			req := httptest.NewRequest(http.MethodPost, "/mcp/v1", bytes.NewReader(body))
			w := httptest.NewRecorder()

			server.handleHTTPRequest(w, req)

			if w.Code != http.StatusAccepted {
				t.Errorf("expected status 202 Accepted, got %d", w.Code)
			}
			if w.Body.Len() != 0 {
				t.Errorf("expected empty body, got %q", w.Body.String())
			}
		})
	}
}

func TestHandleNotification_NullIDIsRequest(t *testing.T) {
	// JSON-RPC 2.0 distinguishes "id absent" (notification, no reply) from
	// "id: null" (a request whose id happens to be null; reply required).
	// A request with explicit null id targeting an unknown method must
	// receive a -32601 response with id == null, not 202 Accepted.
	tools := &mockToolProvider{}
	server := NewServer(tools)

	body := []byte(`{"jsonrpc":"2.0","id":null,"method":"unknown/method"}`)
	req := httptest.NewRequest(http.MethodPost, "/mcp/v1", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleHTTPRequest(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200 for a request with null id, got %d", w.Code)
	}

	// Per JSON-RPC 2.0 §5.1 the response object MUST include the id
	// member; for a request with explicit "id": null the response id
	// is also null. Verify on the raw bytes because Decode into
	// JSONRPCResponse cannot distinguish absent id from id: null.
	raw := w.Body.String()
	if !strings.Contains(raw, `"id":null`) {
		t.Errorf("expected response to include `\"id\":null`, got %q", raw)
	}

	var response JSONRPCResponse
	if err := json.Unmarshal([]byte(raw), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Error == nil || response.Error.Code != -32601 {
		t.Errorf("expected -32601 Method not found, got %+v", response.Error)
	}
}

func TestHandlePing(t *testing.T) {
	tools := &mockToolProvider{}
	server := NewServer(tools)

	rpcReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "ping",
	}

	body, _ := json.Marshal(rpcReq)
	req := httptest.NewRequest(http.MethodPost, "/mcp/v1", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleHTTPRequest(w, req)

	var response JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Error != nil {
		t.Fatalf("unexpected error: %v", response.Error)
	}

	result, ok := response.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected result object, got %T", response.Result)
	}
	if len(result) != 0 {
		t.Errorf("expected empty object result for ping, got %v", result)
	}
}

func TestHandleUnknownMethod(t *testing.T) {
	tools := &mockToolProvider{}
	server := NewServer(tools)

	rpcReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "unknown/method",
	}

	body, _ := json.Marshal(rpcReq)
	req := httptest.NewRequest(http.MethodPost, "/mcp/v1", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleHTTPRequest(w, req)

	var response JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Error == nil {
		t.Fatal("expected error response")
	}
	if response.Error.Code != -32601 {
		t.Errorf("expected method not found error -32601, got %d", response.Error.Code)
	}
}

func TestCreateErrorResponse(t *testing.T) {
	tests := []struct {
		name    string
		id      interface{}
		code    int
		message string
		data    interface{}
	}{
		{
			name:    "basic error",
			id:      1,
			code:    -32600,
			message: "Invalid Request",
			data:    nil,
		},
		{
			name:    "error with data",
			id:      "request-1",
			code:    -32603,
			message: "Internal error",
			data:    "additional details",
		},
		{
			name:    "nil id",
			id:      nil,
			code:    -32700,
			message: "Parse error",
			data:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := createErrorResponse(tt.id, tt.code, tt.message, tt.data)

			if resp.JSONRPC != "2.0" {
				t.Errorf("expected jsonrpc '2.0', got %q", resp.JSONRPC)
			}
			if resp.ID != tt.id {
				t.Errorf("expected id %v, got %v", tt.id, resp.ID)
			}
			if resp.Error == nil {
				t.Fatal("expected error to be set")
			}
			if resp.Error.Code != tt.code {
				t.Errorf("expected code %d, got %d", tt.code, resp.Error.Code)
			}
			if resp.Error.Message != tt.message {
				t.Errorf("expected message %q, got %q", tt.message, resp.Error.Message)
			}
		})
	}
}

func TestHTTPConfigStruct(t *testing.T) {
	config := HTTPConfig{
		Addr:        ":8080",
		TLSEnable:   true,
		CertFile:    "/path/to/cert.pem",
		KeyFile:     "/path/to/key.pem",
		ChainFile:   "/path/to/chain.pem",
		AuthEnabled: true,
		Debug:       true,
	}

	if config.Addr != ":8080" {
		t.Errorf("expected addr ':8080', got %q", config.Addr)
	}
	if !config.TLSEnable {
		t.Error("expected TLSEnable=true")
	}
	if config.CertFile != "/path/to/cert.pem" {
		t.Errorf("expected CertFile '/path/to/cert.pem', got %q", config.CertFile)
	}
	if config.KeyFile != "/path/to/key.pem" {
		t.Errorf("expected KeyFile '/path/to/key.pem', got %q", config.KeyFile)
	}
	if config.ChainFile != "/path/to/chain.pem" {
		t.Errorf("expected ChainFile '/path/to/chain.pem', got %q", config.ChainFile)
	}
	if !config.AuthEnabled {
		t.Error("expected AuthEnabled=true")
	}
	if !config.Debug {
		t.Error("expected Debug=true")
	}
}

func TestSecurityHeadersMiddleware_LinkHeader(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := securityHeadersMiddleware(false)(handler)

	// Link header should be present on /api/* paths
	req := httptest.NewRequest(http.MethodGet, "/api/databases", nil)
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, req)

	link := w.Header().Get("Link")
	expected := `</api/openapi.json>; rel="service-desc"`
	if link != expected {
		t.Errorf("expected Link header %q on /api/ path, got %q", expected, link)
	}

	// Link header should NOT be present on non-API paths
	req = httptest.NewRequest(http.MethodGet, "/health", nil)
	w = httptest.NewRecorder()

	wrapped.ServeHTTP(w, req)

	if link := w.Header().Get("Link"); link != "" {
		t.Errorf("expected no Link header on /health, got %q", link)
	}

	// Verify other security headers are still present on all paths
	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("missing X-Content-Type-Options header")
	}
	if w.Header().Get("X-Frame-Options") != "DENY" {
		t.Error("missing X-Frame-Options header")
	}
}

func TestRunHTTP_NilConfig(t *testing.T) {
	tools := &mockToolProvider{}
	server := NewServer(tools)

	err := server.RunHTTP(nil)
	if err == nil {
		t.Error("expected error for nil config")
	}
}

func TestBuildHandler_UnknownRouteReturnsJSON404(t *testing.T) {
	tools := &mockToolProvider{}
	server := NewServer(tools)

	handler, err := server.buildHandler(&HTTPConfig{})
	if err != nil {
		t.Fatalf("unexpected error building handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/this-route-does-not-exist", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("response body is not valid JSON: %v (body: %q)", err, w.Body.String())
	}
	if body["error"] == "" {
		t.Error("expected non-empty error message")
	}
}

func TestBuildHandler_KnownRouteStillWorks(t *testing.T) {
	tools := &mockToolProvider{}
	server := NewServer(tools)

	handler, err := server.buildHandler(&HTTPConfig{})
	if err != nil {
		t.Fatalf("unexpected error building handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected the JSON 404 catch-all to not shadow /health; got status %d", w.Code)
	}
}

func TestBuildHandler_PanicRecoveryReturnsJSON500(t *testing.T) {
	tools := &mockToolProvider{}
	server := NewServer(tools)

	handler, err := server.buildHandler(&HTTPConfig{
		SetupHandlers: func(mux *http.ServeMux) error {
			mux.HandleFunc("/panic", func(w http.ResponseWriter, r *http.Request) {
				panic("boom")
			})
			return nil
		},
	})
	if err != nil {
		t.Fatalf("unexpected error building handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	w := httptest.NewRecorder()

	// The handler must not panic out of ServeHTTP itself; recovery
	// middleware should catch it and produce a JSON 500.
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("response body is not valid JSON: %v (body: %q)", err, w.Body.String())
	}
	if body["error"] == "" {
		t.Error("expected non-empty error message")
	}
}

func TestBuildHandler_ErrAbortHandlerNotRecovered(t *testing.T) {
	tools := &mockToolProvider{}
	server := NewServer(tools)

	handler, err := server.buildHandler(&HTTPConfig{
		SetupHandlers: func(mux *http.ServeMux) error {
			mux.HandleFunc("/abort", func(w http.ResponseWriter, r *http.Request) {
				panic(http.ErrAbortHandler)
			})
			return nil
		},
	})
	if err != nil {
		t.Fatalf("unexpected error building handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/abort", nil)
	w := httptest.NewRecorder()

	defer func() {
		p := recover()
		if p != http.ErrAbortHandler {
			t.Errorf("expected http.ErrAbortHandler to propagate past recoveryMiddleware, got %v", p)
		}
	}()
	handler.ServeHTTP(w, req)
	t.Error("expected handler.ServeHTTP to panic with http.ErrAbortHandler")
}
