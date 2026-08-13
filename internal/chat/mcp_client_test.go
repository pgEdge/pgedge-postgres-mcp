/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package chat

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"pgedge-postgres-mcp/internal/mcp"
)

func TestHTTPClient_Initialize(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		var req mcp.JSONRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}

		if req.Method != "server/discover" {
			t.Errorf("Expected method 'server/discover', got '%s'", req.Method)
		}

		if got := r.Header.Get("MCP-Protocol-Version"); got != mcp.ModernProtocolVersion {
			t.Errorf("Expected MCP-Protocol-Version '%s', got '%s'", mcp.ModernProtocolVersion, got)
		}
		if got := r.Header.Get("Mcp-Method"); got != "server/discover" {
			t.Errorf("Expected Mcp-Method 'server/discover', got '%s'", got)
		}

		paramsMap, ok := req.Params.(map[string]interface{})
		if !ok {
			t.Errorf("Expected params to be a map, got %T", req.Params)
			return
		}
		meta, ok := paramsMap["_meta"].(map[string]interface{})
		if !ok {
			t.Errorf("Expected _meta in params, got %v", paramsMap)
			return
		}
		if meta["io.modelcontextprotocol/protocolVersion"] != mcp.ModernProtocolVersion {
			t.Errorf("Expected protocolVersion '%s' in _meta, got %v",
				mcp.ModernProtocolVersion, meta["io.modelcontextprotocol/protocolVersion"])
		}
		if _, ok := meta["io.modelcontextprotocol/clientCapabilities"]; !ok {
			t.Errorf("Expected clientCapabilities in _meta, got %v", meta)
		}

		// Send response
		resp := mcp.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: mcp.DiscoverResult{
				Meta: mcp.ResponseMeta{
					ServerInfo: mcp.Implementation{
						Name:    "test-server",
						Version: "1.0.0",
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Create client
	client := NewHTTPClient(server.URL, "test-token")

	// Test initialize
	ctx := context.Background()
	if err := client.Initialize(ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	name, version := client.GetServerInfo()
	if name != "test-server" || version != "1.0.0" {
		t.Errorf("Expected server info 'test-server'/'1.0.0', got '%s'/'%s'", name, version)
	}
}

func TestHTTPClient_ListTools(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		var req mcp.JSONRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}

		if req.Method != "tools/list" {
			t.Errorf("Expected method 'tools/list', got '%s'", req.Method)
		}

		// Send response
		resp := mcp.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: mcp.ToolsListResult{
				Tools: []mcp.Tool{
					{
						Name:        "test_tool",
						Description: "A test tool",
						InputSchema: mcp.InputSchema{
							Type:       "object",
							Properties: map[string]interface{}{},
						},
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Create client
	client := NewHTTPClient(server.URL, "test-token")

	// Test list tools
	ctx := context.Background()
	tools, err := client.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}

	if len(tools) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(tools))
	}

	if tools[0].Name != "test_tool" {
		t.Errorf("Expected tool name 'test_tool', got '%s'", tools[0].Name)
	}
}

func TestHTTPClient_CallTool(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		var req mcp.JSONRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}

		if req.Method != "tools/call" {
			t.Errorf("Expected method 'tools/call', got '%s'", req.Method)
		}
		if got := r.Header.Get("Mcp-Name"); got != "test_tool" {
			t.Errorf("Expected Mcp-Name 'test_tool', got '%s'", got)
		}

		// Send response
		resp := mcp.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: mcp.ToolResponse{
				Content: []mcp.ContentItem{
					{
						Type: "text",
						Text: "Tool executed successfully",
					},
				},
				IsError: false,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Create client
	client := NewHTTPClient(server.URL, "test-token")

	// Test call tool
	ctx := context.Background()
	result, err := client.CallTool(ctx, "test_tool", map[string]interface{}{"arg": "value"})
	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}

	if len(result.Content) != 1 {
		t.Errorf("Expected 1 content item, got %d", len(result.Content))
	}

	if result.Content[0].Text != "Tool executed successfully" {
		t.Errorf("Expected text 'Tool executed successfully', got '%s'", result.Content[0].Text)
	}
}

func TestHTTPClient_Authentication(t *testing.T) {
	expectedToken := "test-token-12345"

	// Create test server that checks authentication
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Authorization header
		auth := r.Header.Get("Authorization")
		expectedAuth := "Bearer " + expectedToken
		if auth != expectedAuth {
			t.Errorf("Expected Authorization header '%s', got '%s'", expectedAuth, auth)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Send dummy response
		resp := mcp.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      1,
			Result: mcp.ToolsListResult{
				Tools: []mcp.Tool{},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Create client with token
	client := NewHTTPClient(server.URL, expectedToken)

	// Test a request to verify authentication
	ctx := context.Background()
	_, err := client.ListTools(ctx)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
}

func TestModernMeta_HasRequiredFields(t *testing.T) {
	meta := modernMeta()

	if meta["io.modelcontextprotocol/protocolVersion"] != mcp.ModernProtocolVersion {
		t.Errorf("Expected protocolVersion '%s', got %v",
			mcp.ModernProtocolVersion, meta["io.modelcontextprotocol/protocolVersion"])
	}

	caps, ok := meta["io.modelcontextprotocol/clientCapabilities"]
	if !ok {
		t.Fatalf("Expected clientCapabilities key to be present, got %v", meta)
	}

	// The required field must survive a JSON round-trip even though it's
	// an empty map -- this is the omitempty trap the RequestMeta struct
	// would fall into if used for outgoing requests.
	data, err := json.Marshal(map[string]interface{}{"_meta": meta})
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var roundTripped struct {
		Meta map[string]interface{} `json:"_meta"`
	}
	if err := json.Unmarshal(data, &roundTripped); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if _, ok := roundTripped.Meta["io.modelcontextprotocol/clientCapabilities"]; !ok {
		t.Errorf("clientCapabilities was dropped by JSON round-trip: %s", data)
	}

	_ = caps
}

func TestEncodeHeaderValue(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  string
	}{
		{"plain ascii passes through", "test_tool", "test_tool"},
		{"resource uri passes through", "pg://some/resource", "pg://some/resource"},
		{"empty string passes through", "", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := encodeHeaderValue(tc.value); got != tc.want {
				t.Errorf("encodeHeaderValue(%q) = %q, want %q", tc.value, got, tc.want)
			}
		})
	}

	escapedCases := []struct {
		name     string
		original string
	}{
		{"non-ASCII value is base64-wrapped and round-trips", "tést_tool"},
		{"leading whitespace is base64-wrapped and round-trips", " leading_space"},
		{"trailing whitespace is base64-wrapped and round-trips", "trailing_space "},
		{"value already shaped like the base64 sentinel is re-wrapped and round-trips", "=?base64?aGk=?="},
	}

	for _, tc := range escapedCases {
		t.Run(tc.name, func(t *testing.T) {
			encoded := encodeHeaderValue(tc.original)

			const prefix, suffix = "=?base64?", "?="
			if len(encoded) < len(prefix)+len(suffix) ||
				encoded[:len(prefix)] != prefix || encoded[len(encoded)-len(suffix):] != suffix {
				t.Fatalf("expected sentinel-wrapped value, got %q", encoded)
			}

			b64 := encoded[len(prefix) : len(encoded)-len(suffix)]
			decoded, err := base64.StdEncoding.DecodeString(b64)
			if err != nil {
				t.Fatalf("base64 decode failed: %v", err)
			}
			if string(decoded) != tc.original {
				t.Errorf("round-trip mismatch: got %q, want %q", string(decoded), tc.original)
			}
		})
	}
}

func TestNameOrURIFor(t *testing.T) {
	cases := []struct {
		name   string
		params interface{}
		want   string
	}{
		{"tool call params", mcp.ToolCallParams{Name: "test_tool"}, "test_tool"},
		{"resource read params", mcp.ResourceReadParams{URI: "pg://x"}, "pg://x"},
		{"prompt get params", mcp.PromptGetParams{Name: "my_prompt"}, "my_prompt"},
		{"nil params", nil, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := nameOrURIFor(tc.params); got != tc.want {
				t.Errorf("nameOrURIFor(%v) = %q, want %q", tc.params, got, tc.want)
			}
		})
	}
}

func TestStdioClient_Initialize(t *testing.T) {
	clientToServerR, clientToServerW := io.Pipe()
	serverToClientR, serverToClientW := io.Pipe()

	scanner := bufio.NewScanner(serverToClientR)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	c := &stdioClient{
		stdin:   clientToServerW,
		stdout:  serverToClientR,
		scanner: scanner,
	}

	fakeServerErr := make(chan error, 1)
	go func() {
		fakeServerErr <- serveFakeStdioDiscover(clientToServerR, serverToClientW)
	}()

	ctx := context.Background()
	if err := c.Initialize(ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	if err := <-fakeServerErr; err != nil {
		t.Fatalf("fake server error: %v", err)
	}

	name, version := c.GetServerInfo()
	if name != "test-server" || version != "1.0.0" {
		t.Errorf("Expected server info 'test-server'/'1.0.0', got '%s'/'%s'", name, version)
	}
}

// serveFakeStdioDiscover reads one server/discover request from r, verifies
// it carries the modern _meta envelope, and writes a matching DiscoverResult
// response to w. Extracted from TestStdioClient_Initialize so that test's
// setup stays a single, simple goroutine launch; split further into
// readDiscoverRequest/validateModernEnvelope to keep each function's
// cyclomatic complexity low.
func serveFakeStdioDiscover(r io.Reader, w io.Writer) error {
	req, err := readDiscoverRequest(r)
	if err != nil {
		return err
	}
	if err := validateModernEnvelope(req.Params); err != nil {
		return err
	}
	return writeDiscoverResponse(w, req.ID)
}

// readDiscoverRequest scans one JSON-RPC request line from r and confirms
// its method is server/discover.
func readDiscoverRequest(r io.Reader) (mcp.JSONRPCRequest, error) {
	reqScanner := bufio.NewScanner(r)
	reqScanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	if !reqScanner.Scan() {
		return mcp.JSONRPCRequest{}, fmt.Errorf("no request received: %v", reqScanner.Err())
	}

	var req mcp.JSONRPCRequest
	if err := json.Unmarshal(reqScanner.Bytes(), &req); err != nil {
		return mcp.JSONRPCRequest{}, err
	}
	if req.Method != "server/discover" {
		return mcp.JSONRPCRequest{}, fmt.Errorf("expected method 'server/discover', got %q", req.Method)
	}
	return req, nil
}

// validateModernEnvelope checks that params carries the modern _meta
// envelope (protocolVersion and clientCapabilities) this test expects
// stdioClient.sendRequest to attach to every outgoing request.
func validateModernEnvelope(params interface{}) error {
	paramsMap, ok := params.(map[string]interface{})
	if !ok {
		return fmt.Errorf("expected params to be a map, got %T", params)
	}
	meta, ok := paramsMap["_meta"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("expected _meta in params, got %v", paramsMap)
	}
	if meta["io.modelcontextprotocol/protocolVersion"] != mcp.ModernProtocolVersion {
		return fmt.Errorf("unexpected protocolVersion: %v",
			meta["io.modelcontextprotocol/protocolVersion"])
	}
	if _, ok := meta["io.modelcontextprotocol/clientCapabilities"]; !ok {
		return fmt.Errorf("expected clientCapabilities in _meta, got %v", meta)
	}
	return nil
}

// writeDiscoverResponse writes a DiscoverResult-shaped JSON-RPC response
// for the given request id to w.
func writeDiscoverResponse(w io.Writer, id interface{}) error {
	resp := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"result": map[string]interface{}{
			"_meta": map[string]interface{}{
				"io.modelcontextprotocol/serverInfo": map[string]interface{}{
					"name":    "test-server",
					"version": "1.0.0",
				},
			},
		},
	}
	respData, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	if _, err := w.Write(append(respData, '\n')); err != nil {
		return err
	}
	return nil
}
