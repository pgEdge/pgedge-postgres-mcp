/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package test

// Integration coverage for the MCP 2026-07-28 revision this server added
// alongside its existing (legacy, 2024-11-05) behavior: server/discover,
// per-request _meta negotiation, required Streamable HTTP headers, and
// the resultType/ttlMs/cacheScope/_meta fields a modern result carries.
// These tests run the real server binary over real HTTP, the same way
// http_integration_test.go does, rather than exercising the handlers
// directly: header validation in particular only exists at the HTTP
// transport layer, so unit tests calling a handler function cannot
// observe it.

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// sendModernHTTPRequest POSTs a JSON-RPC request with the given extra
// headers and returns both the raw HTTP status code and the parsed
// JSON-RPC response, so tests can assert on either. SendHTTPRequest
// (http_integration_test.go) does not expose the status code, which
// every test here needs: header validation and version-mismatch
// failures are only correct if they also produced the spec-mandated
// 400, not merely a JSON-RPC error inside a 200.
func sendModernHTTPRequest(t *testing.T, server *HTTPMCPServer, method string, params interface{}, headers map[string]string) (int, *MCPResponse) {
	t.Helper()

	req := MCPRequest{JSONRPC: "2.0", ID: 1, Method: method, Params: params}
	reqJSON, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, server.baseURL+"/mcp/v1", bytes.NewReader(reqJSON))
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		httpReq.Header.Set(k, v)
	}

	client := server.getHTTPClient()
	resp, err := client.Do(httpReq)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	var mcpResp MCPResponse
	if err := json.Unmarshal(body, &mcpResp); err != nil {
		t.Fatalf("failed to unmarshal response %q: %v", body, err)
	}

	return resp.StatusCode, &mcpResp
}

// modernMeta builds the _meta object a modern request carries.
func modernMeta(protocolVersion string) map[string]interface{} {
	return map[string]interface{}{
		"_meta": map[string]interface{}{
			"io.modelcontextprotocol/protocolVersion":    protocolVersion,
			"io.modelcontextprotocol/clientCapabilities": map[string]interface{}{},
		},
	}
}

func TestModernProtocol_ServerDiscover(t *testing.T) {
	server, err := StartHTTPMCPServer(t, "", "test-key", "localhost:18711", false)
	if err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer func() { _ = server.Close() }()

	// A client that does not yet know what this server supports can call
	// server/discover the same way a legacy client would -- no _meta at
	// all -- and get the same answer, since handleDiscoverHTTP does not
	// vary its response by era; see TestModernProtocol_ServerDiscover_AsModernRequest
	// for the same call made as a fully validated modern request.
	status, resp := sendModernHTTPRequest(t, server, "server/discover", nil, nil)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %+v", status, resp)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}

	var result struct {
		ResultType        string                 `json:"resultType"`
		SupportedVersions []string               `json:"supportedVersions"`
		Capabilities      map[string]interface{} `json:"capabilities"`
		TTLMs             int                    `json:"ttlMs"`
		CacheScope        string                 `json:"cacheScope"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to decode result: %v", err)
	}
	if result.ResultType != "complete" {
		t.Errorf("resultType = %q, want %q", result.ResultType, "complete")
	}
	if len(result.SupportedVersions) == 0 || result.SupportedVersions[0] != "2026-07-28" {
		t.Errorf("supportedVersions = %v, want [\"2026-07-28\"]", result.SupportedVersions)
	}
	if _, ok := result.Capabilities["tools"]; !ok {
		t.Error("expected tools capability")
	}
	if _, ok := result.Capabilities["extensions"]; !ok {
		t.Error("expected extensions capability field")
	}
	if result.CacheScope != "public" || result.TTLMs <= 0 {
		t.Errorf("unexpected cache fields: cacheScope=%q ttlMs=%d", result.CacheScope, result.TTLMs)
	}
}

// TestModernProtocol_ServerDiscover_AsModernRequest calls server/discover
// as a fully validated modern request (matching _meta.protocolVersion and
// Streamable HTTP headers), unlike TestModernProtocol_ServerDiscover's
// legacy-shaped call. handleDiscoverHTTP already returns a DiscoverResult
// with every modern field (resultType, ttlMs, cacheScope, _meta.serverInfo)
// baked in, and handleHTTPRequest's centralized wrapModernResultForMethod
// runs on top of it regardless, since server/discover gets no exemption
// from that step. This asserts that pass-through is a no-op rather than a
// corruption: the fields keep the same values, not duplicates or blanks.
func TestModernProtocol_ServerDiscover_AsModernRequest(t *testing.T) {
	server, err := StartHTTPMCPServer(t, "", "test-key", "localhost:18719", false)
	if err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer func() { _ = server.Close() }()

	headers := map[string]string{
		"MCP-Protocol-Version": "2026-07-28",
		"Mcp-Method":           "server/discover",
	}
	status, resp := sendModernHTTPRequest(t, server, "server/discover", modernMeta("2026-07-28"), headers)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %+v", status, resp)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(resp.Result, &raw); err != nil {
		t.Fatalf("failed to decode result: %v", err)
	}
	if n := countKey(resp.Result, "resultType"); n != 1 {
		t.Errorf("resultType appears %d times in raw JSON, want exactly 1", n)
	}
	if n := countKey(resp.Result, "io.modelcontextprotocol/serverInfo"); n != 1 {
		t.Errorf("_meta.serverInfo appears %d times in raw JSON, want exactly 1", n)
	}

	var result struct {
		ResultType        string                 `json:"resultType"`
		SupportedVersions []string               `json:"supportedVersions"`
		Capabilities      map[string]interface{} `json:"capabilities"`
		TTLMs             int                    `json:"ttlMs"`
		CacheScope        string                 `json:"cacheScope"`
		Meta              struct {
			ServerInfo struct {
				Name string `json:"name"`
			} `json:"io.modelcontextprotocol/serverInfo"`
		} `json:"_meta"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to decode result: %v", err)
	}
	if result.ResultType != "complete" {
		t.Errorf("resultType = %q, want %q", result.ResultType, "complete")
	}
	if len(result.SupportedVersions) == 0 || result.SupportedVersions[0] != "2026-07-28" {
		t.Errorf("supportedVersions = %v, want [\"2026-07-28\"]", result.SupportedVersions)
	}
	if result.CacheScope != "public" || result.TTLMs <= 0 {
		t.Errorf("unexpected cache fields: cacheScope=%q ttlMs=%d", result.CacheScope, result.TTLMs)
	}
	if result.Meta.ServerInfo.Name != "pgedge-postgres-mcp" {
		t.Errorf("_meta.serverInfo.name = %q, want %q", result.Meta.ServerInfo.Name, "pgedge-postgres-mcp")
	}
}

// countKey counts how many times a JSON object key appears anywhere in
// raw, used to catch the specific corruption a double-wrap could cause:
// a field present twice (e.g. nested inside itself) rather than merged.
func countKey(raw []byte, key string) int {
	return strings.Count(string(raw), `"`+key+`"`)
}

func TestModernProtocol_ToolsList_WithHeaders(t *testing.T) {
	server, err := StartHTTPMCPServer(t, "", "test-key", "localhost:18712", false)
	if err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer func() { _ = server.Close() }()

	headers := map[string]string{
		"MCP-Protocol-Version": "2026-07-28",
		"Mcp-Method":           "tools/list",
	}
	status, resp := sendModernHTTPRequest(t, server, "tools/list", modernMeta("2026-07-28"), headers)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %+v", status, resp)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}

	var result struct {
		ResultType string        `json:"resultType"`
		TTLMs      int           `json:"ttlMs"`
		CacheScope string        `json:"cacheScope"`
		Tools      []interface{} `json:"tools"`
		Meta       struct {
			ServerInfo struct {
				Name string `json:"name"`
			} `json:"io.modelcontextprotocol/serverInfo"`
		} `json:"_meta"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to decode result: %v", err)
	}
	if result.ResultType != "complete" {
		t.Errorf("resultType = %q, want %q", result.ResultType, "complete")
	}
	if result.CacheScope != "public" || result.TTLMs <= 0 {
		t.Errorf("unexpected cache fields: cacheScope=%q ttlMs=%d", result.CacheScope, result.TTLMs)
	}
	if len(result.Tools) == 0 {
		t.Error("expected at least one tool")
	}
	if result.Meta.ServerInfo.Name != "pgedge-postgres-mcp" {
		t.Errorf("_meta.serverInfo.name = %q, want %q", result.Meta.ServerInfo.Name, "pgedge-postgres-mcp")
	}
}

func TestModernProtocol_MissingHeaders_RejectedWith400(t *testing.T) {
	server, err := StartHTTPMCPServer(t, "", "test-key", "localhost:18713", false)
	if err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer func() { _ = server.Close() }()

	// A modern-shaped body (carries _meta.protocolVersion) with none of
	// the required Streamable HTTP headers must be rejected with 400 and
	// a HeaderMismatch error, not silently accepted.
	status, resp := sendModernHTTPRequest(t, server, "tools/list", modernMeta("2026-07-28"), nil)
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body: %+v", status, resp)
	}
	if resp.Error == nil || resp.Error.Code != -32020 {
		t.Fatalf("error = %+v, want code -32020 (HeaderMismatch)", resp.Error)
	}
}

func TestModernProtocol_UnsupportedVersion_RejectedWith400(t *testing.T) {
	server, err := StartHTTPMCPServer(t, "", "test-key", "localhost:18714", false)
	if err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer func() { _ = server.Close() }()

	headers := map[string]string{
		"MCP-Protocol-Version": "1900-01-01",
		"Mcp-Method":           "tools/list",
	}
	status, resp := sendModernHTTPRequest(t, server, "tools/list", modernMeta("1900-01-01"), headers)
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body: %+v", status, resp)
	}
	if resp.Error == nil || resp.Error.Code != -32022 {
		t.Fatalf("error = %+v, want code -32022 (UnsupportedProtocolVersion)", resp.Error)
	}
}

func TestModernProtocol_MissingClientCapabilities_RejectedWith400(t *testing.T) {
	server, err := StartHTTPMCPServer(t, "", "test-key", "localhost:18715", false)
	if err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer func() { _ = server.Close() }()

	headers := map[string]string{
		"MCP-Protocol-Version": "2026-07-28",
		"Mcp-Method":           "tools/list",
	}
	params := map[string]interface{}{
		"_meta": map[string]interface{}{
			"io.modelcontextprotocol/protocolVersion": "2026-07-28",
			// clientCapabilities deliberately omitted.
		},
	}
	status, resp := sendModernHTTPRequest(t, server, "tools/list", params, headers)
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body: %+v", status, resp)
	}
	if resp.Error == nil || resp.Error.Code != -32602 {
		t.Fatalf("error = %+v, want code -32602 (Invalid params)", resp.Error)
	}
}

func TestModernProtocol_ToolCall_MismatchedMcpName_RejectedWith400(t *testing.T) {
	server, err := StartHTTPMCPServer(t, "", "test-key", "localhost:18716", false)
	if err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer func() { _ = server.Close() }()

	headers := map[string]string{
		"MCP-Protocol-Version": "2026-07-28",
		"Mcp-Method":           "tools/call",
		"Mcp-Name":             "wrong_tool_name",
	}
	params := map[string]interface{}{
		"name":      "count_rows",
		"arguments": map[string]interface{}{},
		"_meta": map[string]interface{}{
			"io.modelcontextprotocol/protocolVersion":    "2026-07-28",
			"io.modelcontextprotocol/clientCapabilities": map[string]interface{}{},
		},
	}
	status, resp := sendModernHTTPRequest(t, server, "tools/call", params, headers)
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body: %+v", status, resp)
	}
	if resp.Error == nil || resp.Error.Code != -32020 {
		t.Fatalf("error = %+v, want code -32020 (HeaderMismatch)", resp.Error)
	}
}

func TestModernProtocol_LegacyRequestsUnaffected(t *testing.T) {
	server, err := StartHTTPMCPServer(t, "", "test-key", "localhost:18717", false)
	if err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer func() { _ = server.Close() }()

	// A request with no _meta at all -- every pre-2026-07-28 client,
	// including this repo's own bundled CLI and web client -- must see
	// none of the modern fields and must not be subject to header
	// validation.
	status, resp := sendModernHTTPRequest(t, server, "tools/list", nil, nil)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %+v", status, resp)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(resp.Result, &raw); err != nil {
		t.Fatalf("failed to decode result: %v", err)
	}
	if _, ok := raw["resultType"]; ok {
		t.Error("legacy result must not carry resultType")
	}
	if _, ok := raw["_meta"]; ok {
		t.Error("legacy result must not carry _meta")
	}
	if _, ok := raw["ttlMs"]; ok {
		t.Error("legacy result must not carry ttlMs")
	}
}

func TestModernProtocol_ResourceNotFound_ErrorCodeByEra(t *testing.T) {
	server, err := StartHTTPMCPServer(t, "", "test-key", "localhost:18718", false)
	if err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer func() { _ = server.Close() }()

	t.Run("legacy gets -32002", func(t *testing.T) {
		_, resp := sendModernHTTPRequest(t, server, "resources/read",
			map[string]interface{}{"uri": "pg://does-not-exist"}, nil)
		if resp.Error == nil || resp.Error.Code != -32002 {
			t.Fatalf("error = %+v, want code -32002", resp.Error)
		}
	})

	t.Run("modern gets -32602", func(t *testing.T) {
		headers := map[string]string{
			"MCP-Protocol-Version": "2026-07-28",
			"Mcp-Method":           "resources/read",
			"Mcp-Name":             "pg://does-not-exist",
		}
		params := map[string]interface{}{
			"uri": "pg://does-not-exist",
			"_meta": map[string]interface{}{
				"io.modelcontextprotocol/protocolVersion":    "2026-07-28",
				"io.modelcontextprotocol/clientCapabilities": map[string]interface{}{},
			},
		}
		_, resp := sendModernHTTPRequest(t, server, "resources/read", params, headers)
		if resp.Error == nil || resp.Error.Code != -32602 {
			t.Fatalf("error = %+v, want code -32602", resp.Error)
		}
	})
}

// TestModernProtocol_Initialize_RejectedAsMethodNotFound is the HTTP
// counterpart of TestHandleRequest_ModernInitialize_RejectedAsMethodNotFound
// (internal/mcp/server_test.go): "initialize" does not exist as a
// method under the modern (2026-07-28) era, so a fully validated
// modern-shaped request naming it must be rejected with -32601 rather
// than answered with a legacy InitializeResult. Found via CodeRabbit
// review on PR #215.
func TestModernProtocol_Initialize_RejectedAsMethodNotFound(t *testing.T) {
	server, err := StartHTTPMCPServer(t, "", "test-key", "localhost:18720", false)
	if err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer func() { _ = server.Close() }()

	headers := map[string]string{
		"MCP-Protocol-Version": "2026-07-28",
		"Mcp-Method":           "initialize",
	}
	params := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"_meta": map[string]interface{}{
			"io.modelcontextprotocol/protocolVersion":    "2026-07-28",
			"io.modelcontextprotocol/clientCapabilities": map[string]interface{}{},
		},
	}
	status, resp := sendModernHTTPRequest(t, server, "initialize", params, headers)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200 (a handler-level rejection, not a preflight one); body: %+v", status, resp)
	}
	if resp.Error == nil || resp.Error.Code != -32601 {
		t.Fatalf("error = %+v, want code -32601 (Method not found)", resp.Error)
	}
}
