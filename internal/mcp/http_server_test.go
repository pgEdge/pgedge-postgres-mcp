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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
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

	rpcReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "notifications/initialized",
	}

	body, _ := json.Marshal(rpcReq)
	req := httptest.NewRequest(http.MethodPost, "/mcp/v1", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleHTTPRequest(w, req)

	var response JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should return empty response without error
	if response.Error != nil {
		t.Fatalf("unexpected error: %v", response.Error)
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
	if !config.AuthEnabled {
		t.Error("expected AuthEnabled=true")
	}
	if !config.Debug {
		t.Error("expected Debug=true")
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
