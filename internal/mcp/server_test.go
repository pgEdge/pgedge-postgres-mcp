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
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

// captureStdout runs fn while os.Stdout is redirected to an in-memory
// pipe, and returns whatever fn wrote. Used to test stdio handlers,
// which write JSON-RPC responses directly to os.Stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe failed: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("close write end failed: %v", err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read pipe failed: %v", err)
	}
	return string(out)
}

// Mock implementations for testing

type mockToolProvider struct {
	tools       []Tool
	executeFunc func(ctx context.Context, name string, args map[string]interface{}) (ToolResponse, error)
}

func (m *mockToolProvider) List() []Tool {
	return m.tools
}

func (m *mockToolProvider) ListContext(ctx context.Context) []Tool {
	return m.tools
}

func (m *mockToolProvider) Execute(ctx context.Context, name string, args map[string]interface{}) (ToolResponse, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, name, args)
	}
	return NewToolSuccess("executed")
}

type mockResourceProvider struct {
	resources []Resource
	readFunc  func(ctx context.Context, uri string) (ResourceContent, error)
}

func (m *mockResourceProvider) List() []Resource {
	return m.resources
}

func (m *mockResourceProvider) Read(ctx context.Context, uri string) (ResourceContent, error) {
	if m.readFunc != nil {
		return m.readFunc(ctx, uri)
	}
	return NewResourceSuccess(uri, "text/plain", "content")
}

type mockPromptProvider struct {
	prompts     []Prompt
	executeFunc func(name string, args map[string]string) (PromptResult, error)
}

func (m *mockPromptProvider) List() []Prompt {
	return m.prompts
}

func (m *mockPromptProvider) Execute(name string, args map[string]string) (PromptResult, error) {
	if m.executeFunc != nil {
		return m.executeFunc(name, args)
	}
	return PromptResult{
		Messages: []PromptMessage{
			{Role: "user", Content: ContentItem{Type: "text", Text: "test"}},
		},
	}, nil
}

type mockDatabaseProvider struct {
	databases  []DatabaseInfo
	current    string
	listErr    error
	selectErr  error
	selectFunc func(ctx context.Context, name string) error
}

func (m *mockDatabaseProvider) ListDatabases(ctx context.Context) ([]DatabaseInfo, string, error) {
	if m.listErr != nil {
		return nil, "", m.listErr
	}
	return m.databases, m.current, nil
}

func (m *mockDatabaseProvider) SelectDatabase(ctx context.Context, name string) error {
	if m.selectFunc != nil {
		return m.selectFunc(ctx, name)
	}
	if m.selectErr != nil {
		return m.selectErr
	}
	m.current = name
	return nil
}

func TestNewServer(t *testing.T) {
	tools := &mockToolProvider{
		tools: []Tool{{Name: "test", Description: "Test tool"}},
	}

	server := NewServer(tools)
	if server == nil {
		t.Fatal("expected non-nil server")
	}
	if server.tools == nil {
		t.Error("expected tools provider to be set")
	}
}

func TestServerSetProviders(t *testing.T) {
	tools := &mockToolProvider{}
	server := NewServer(tools)

	// Test SetResourceProvider
	resources := &mockResourceProvider{
		resources: []Resource{{URI: "pg://test", Name: "test"}},
	}
	server.SetResourceProvider(resources)
	if server.resources == nil {
		t.Error("expected resource provider to be set")
	}

	// Test SetPromptProvider
	prompts := &mockPromptProvider{
		prompts: []Prompt{{Name: "test", Description: "Test prompt"}},
	}
	server.SetPromptProvider(prompts)
	if server.prompts == nil {
		t.Error("expected prompt provider to be set")
	}

	// Test SetDatabaseProvider
	databases := &mockDatabaseProvider{
		databases: []DatabaseInfo{{Name: "testdb", Host: "localhost"}},
	}
	server.SetDatabaseProvider(databases)
	if server.databases == nil {
		t.Error("expected database provider to be set")
	}
}

func TestServerConstants(t *testing.T) {
	// Verify server constants are set correctly
	if ProtocolVersion == "" {
		t.Error("ProtocolVersion should not be empty")
	}
	if ServerName == "" {
		t.Error("ServerName should not be empty")
	}
	if ServerVersion == "" {
		t.Error("ServerVersion should not be empty")
	}

	// Verify expected values
	if ServerName != "pgedge-postgres-mcp" {
		t.Errorf("expected ServerName 'pgedge-postgres-mcp', got %q", ServerName)
	}
}

// TestSupportedProtocolVersions_NewestIsProtocolVersion enforces the
// invariant documented on supportedProtocolVersions: ProtocolVersion, the
// server's own default, must be its last (newest) entry. NegotiateProtocolVersion
// relies on this to answer an empty request with the server's actual
// newest supported revision rather than a stale one left behind when a
// newer revision was appended.
func TestSupportedProtocolVersions_NewestIsProtocolVersion(t *testing.T) {
	if len(supportedProtocolVersions) == 0 {
		t.Fatal("supportedProtocolVersions must not be empty")
	}
	newest := supportedProtocolVersions[len(supportedProtocolVersions)-1]
	if newest != ProtocolVersion {
		t.Errorf("last entry of supportedProtocolVersions = %q, want ProtocolVersion %q", newest, ProtocolVersion)
	}
	for i := 1; i < len(supportedProtocolVersions); i++ {
		if supportedProtocolVersions[i-1] >= supportedProtocolVersions[i] {
			t.Errorf("supportedProtocolVersions is not strictly ascending at index %d: %q >= %q",
				i, supportedProtocolVersions[i-1], supportedProtocolVersions[i])
		}
	}
}

// firstModernProtocolVersion is the earliest MCP revision that abandoned the
// initialize handshake in favour of per-request version metadata and
// server/discover. Revisions from here onwards cannot be negotiated through
// initialize at all, so they must not appear in supportedProtocolVersions.
const firstModernProtocolVersion = "2026-07-28"

// TestSupportedProtocolVersions_AreAllLegacy guards the boundary documented
// on supportedProtocolVersions. NegotiateProtocolVersion answers the
// initialize handshake, which revision 2026-07-28 removed, so appending a
// modern revision to that list would have the server reply to a legacy
// handshake with a revision that has no handshake. Supporting a modern
// revision needs its own path, and this test fails if one is added here
// instead.
func TestSupportedProtocolVersions_AreAllLegacy(t *testing.T) {
	for _, v := range supportedProtocolVersions {
		if v >= firstModernProtocolVersion {
			t.Errorf("supportedProtocolVersions contains %q, which is revision %q or later: "+
				"those negotiate through per-request metadata and server/discover, not the initialize handshake",
				v, firstModernProtocolVersion)
		}
	}
}

func TestNegotiateProtocolVersion(t *testing.T) {
	tests := []struct {
		name      string
		requested string
		want      string
	}{
		{
			name:      "empty request gets the server's own default",
			requested: "",
			want:      ProtocolVersion,
		},
		{
			name:      "exact match",
			requested: ProtocolVersion,
			want:      ProtocolVersion,
		},
		{
			name:      "a revision newer than anything supported falls back to the newest supported",
			requested: "2099-01-01",
			want:      ProtocolVersion,
		},
		{
			// Deliberately more conservative than the specification, which
			// says a server SHOULD answer an unsupported request with its
			// latest revision; see NegotiateProtocolVersion's comment. The
			// two coincide whilst this server implements one revision.
			name:      "a revision older than anything supported falls back to the oldest supported",
			requested: "2020-01-01",
			want:      supportedProtocolVersions[0],
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NegotiateProtocolVersion(tt.requested)
			if got != tt.want {
				t.Errorf("NegotiateProtocolVersion(%q) = %q, want %q", tt.requested, got, tt.want)
			}
		})
	}
}

// TestNegotiateProtocolVersion_NeverEchoesUnsupported is the regression
// test for issue #212: the server must never claim to speak a protocol
// revision it does not actually implement, regardless of what a client
// asks for.
func TestNegotiateProtocolVersion_NeverEchoesUnsupported(t *testing.T) {
	requested := []string{"2025-03-26", "2025-06-18", "2025-11-25", "2026-07-28", "not-even-a-date", ""}
	supported := make(map[string]bool, len(supportedProtocolVersions))
	for _, v := range supportedProtocolVersions {
		supported[v] = true
	}
	for _, r := range requested {
		got := NegotiateProtocolVersion(r)
		if !supported[got] {
			t.Errorf("NegotiateProtocolVersion(%q) = %q, which is not in supportedProtocolVersions", r, got)
		}
	}
}

func TestScannerConstants(t *testing.T) {
	// Verify buffer size constants are reasonable
	if ScannerInitialBufferSize <= 0 {
		t.Error("ScannerInitialBufferSize should be positive")
	}
	if ScannerMaxBufferSize <= ScannerInitialBufferSize {
		t.Error("ScannerMaxBufferSize should be greater than initial size")
	}

	// Verify expected values
	if ScannerInitialBufferSize != 64*1024 {
		t.Errorf("expected initial buffer size 64KB, got %d", ScannerInitialBufferSize)
	}
	if ScannerMaxBufferSize != 1024*1024 {
		t.Errorf("expected max buffer size 1MB, got %d", ScannerMaxBufferSize)
	}
}

func TestListDatabasesResponseStruct(t *testing.T) {
	resp := ListDatabasesResponse{
		Databases: []DatabaseInfo{
			{Name: "db1", Host: "localhost", Port: 5432},
			{Name: "db2", Host: "localhost", Port: 5433},
		},
		Current: "db1",
	}

	if len(resp.Databases) != 2 {
		t.Errorf("expected 2 databases, got %d", len(resp.Databases))
	}
	if resp.Current != "db1" {
		t.Errorf("expected current 'db1', got %q", resp.Current)
	}
}

func TestSelectDatabaseParamsStruct(t *testing.T) {
	params := SelectDatabaseParams{Name: "testdb"}
	if params.Name != "testdb" {
		t.Errorf("expected name 'testdb', got %q", params.Name)
	}
}

func TestSelectDatabaseResponseStruct(t *testing.T) {
	// Success response
	success := SelectDatabaseResponse{
		Success: true,
		Current: "testdb",
		Message: "Switched to database",
	}
	if !success.Success {
		t.Error("expected success=true")
	}
	if success.Current != "testdb" {
		t.Errorf("expected current 'testdb', got %q", success.Current)
	}
	if success.Message != "Switched to database" {
		t.Errorf("expected message 'Switched to database', got %q", success.Message)
	}

	// Error response
	errResp := SelectDatabaseResponse{
		Success: false,
		Error:   "database not found",
	}
	if errResp.Success {
		t.Error("expected success=false")
	}
	if errResp.Error != "database not found" {
		t.Errorf("expected error 'database not found', got %q", errResp.Error)
	}
}

func TestDatabaseInfoStruct(t *testing.T) {
	info := DatabaseInfo{
		Name:     "testdb",
		Host:     "localhost",
		Port:     5432,
		Database: "mydb",
		User:     "user",
		SSLMode:  "disable",
	}

	if info.Name != "testdb" {
		t.Errorf("expected name 'testdb', got %q", info.Name)
	}
	if info.Host != "localhost" {
		t.Errorf("expected host 'localhost', got %q", info.Host)
	}
	if info.Port != 5432 {
		t.Errorf("expected port 5432, got %d", info.Port)
	}
	if info.Database != "mydb" {
		t.Errorf("expected database 'mydb', got %q", info.Database)
	}
	if info.User != "user" {
		t.Errorf("expected user 'user', got %q", info.User)
	}
	if info.SSLMode != "disable" {
		t.Errorf("expected sslmode 'disable', got %q", info.SSLMode)
	}
}

// Test mock providers work correctly
func TestMockToolProvider(t *testing.T) {
	provider := &mockToolProvider{
		tools: []Tool{
			{Name: "tool1", Description: "First tool"},
			{Name: "tool2", Description: "Second tool"},
		},
	}

	tools := provider.List()
	if len(tools) != 2 {
		t.Errorf("expected 2 tools, got %d", len(tools))
	}

	// Test execute with default behavior
	resp, err := provider.Execute(context.Background(), "test", nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if resp.IsError {
		t.Error("expected success response")
	}

	// Test execute with custom function
	provider.executeFunc = func(ctx context.Context, name string, args map[string]interface{}) (ToolResponse, error) {
		if name == "fail" {
			return NewToolError("failed")
		}
		return NewToolSuccess("custom: " + name)
	}

	resp, err = provider.Execute(context.Background(), "test", nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if resp.Content[0].Text != "custom: test" {
		t.Errorf("expected custom response, got %q", resp.Content[0].Text)
	}
}

func TestMockResourceProvider(t *testing.T) {
	provider := &mockResourceProvider{
		resources: []Resource{
			{URI: "pg://schema", Name: "schema"},
		},
	}

	resources := provider.List()
	if len(resources) != 1 {
		t.Errorf("expected 1 resource, got %d", len(resources))
	}

	// Test read with default behavior
	content, err := provider.Read(context.Background(), "pg://test")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if content.URI != "pg://test" {
		t.Errorf("expected URI 'pg://test', got %q", content.URI)
	}

	// Test read with custom function
	provider.readFunc = func(ctx context.Context, uri string) (ResourceContent, error) {
		if uri == "pg://error" {
			return ResourceContent{}, errors.New("not found")
		}
		return NewResourceSuccess(uri, "application/json", `{"key": "value"}`)
	}

	content, err = provider.Read(context.Background(), "pg://custom")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if content.MimeType != "application/json" {
		t.Errorf("expected application/json, got %q", content.MimeType)
	}
}

func TestMockPromptProvider(t *testing.T) {
	provider := &mockPromptProvider{
		prompts: []Prompt{
			{Name: "prompt1", Description: "First prompt"},
		},
	}

	prompts := provider.List()
	if len(prompts) != 1 {
		t.Errorf("expected 1 prompt, got %d", len(prompts))
	}

	// Test execute with default behavior
	result, err := provider.Execute("test", nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(result.Messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(result.Messages))
	}

	// Test execute with custom function
	provider.executeFunc = func(name string, args map[string]string) (PromptResult, error) {
		if name == "fail" {
			return PromptResult{}, errors.New("prompt not found")
		}
		return PromptResult{
			Description: "Custom prompt",
			Messages: []PromptMessage{
				{Role: "user", Content: ContentItem{Type: "text", Text: args["query"]}},
			},
		}, nil
	}

	result, err = provider.Execute("custom", map[string]string{"query": "test query"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result.Description != "Custom prompt" {
		t.Errorf("expected description 'Custom prompt', got %q", result.Description)
	}
}

func TestMockDatabaseProvider(t *testing.T) {
	provider := &mockDatabaseProvider{
		databases: []DatabaseInfo{
			{Name: "db1", Host: "localhost", Port: 5432},
			{Name: "db2", Host: "localhost", Port: 5433},
		},
		current: "db1",
	}

	// Test ListDatabases
	databases, current, err := provider.ListDatabases(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(databases) != 2 {
		t.Errorf("expected 2 databases, got %d", len(databases))
	}
	if current != "db1" {
		t.Errorf("expected current 'db1', got %q", current)
	}

	// Test SelectDatabase
	err = provider.SelectDatabase(context.Background(), "db2")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if provider.current != "db2" {
		t.Errorf("expected current to be 'db2', got %q", provider.current)
	}

	// Test ListDatabases with error
	provider.listErr = errors.New("connection failed")
	_, _, err = provider.ListDatabases(context.Background())
	if err == nil {
		t.Error("expected error")
	}

	// Test SelectDatabase with error
	provider.listErr = nil
	provider.selectErr = errors.New("database not found")
	err = provider.SelectDatabase(context.Background(), "invalid")
	if err == nil {
		t.Error("expected error")
	}
}

func TestHandlePing_Stdio_WithID(t *testing.T) {
	server := NewServer(&mockToolProvider{})
	req := JSONRPCRequest{JSONRPC: "2.0", ID: float64(1), Method: "ping"}

	out := captureStdout(t, func() {
		server.handlePing(req)
	})

	if out == "" {
		t.Fatal("expected a JSON-RPC response on stdout, got nothing")
	}

	var resp JSONRPCResponse
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("failed to decode response %q: %v", out, err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error in response: %v", resp.Error)
	}
	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", resp.Result)
	}
	if len(result) != 0 {
		t.Errorf("expected empty object result, got %v", result)
	}
}

// TestServerDiscover_LegacyShapedRequestAlsoWorks is the regression test
// for a self-audit finding: handleRequest once special-cased
// server/discover, skipping modern validation for it on the theory that
// a client probing for supported versions cannot be expected to already
// know one. That exemption was removed because it was unnecessary: a
// server/discover call with no _meta at all is legacy-shaped, exactly
// like any other method, and reaches handleDiscover without any
// validation gate applying in the first place. This drives the call
// through handleRequest itself (not handleDiscover directly), so it
// exercises the validation gate that was the site of the fix.
func TestServerDiscover_LegacyShapedRequestAlsoWorks(t *testing.T) {
	server := NewServer(&mockToolProvider{})
	req := JSONRPCRequest{JSONRPC: "2.0", ID: float64(1), Method: "server/discover"}

	out := captureStdout(t, func() {
		server.handleRequest(req)
	})

	var resp JSONRPCResponse
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("failed to decode response %q: %v", out, err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error in response: %v", resp.Error)
	}
	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", resp.Result)
	}
	if versions, ok := result["supportedVersions"].([]interface{}); !ok || len(versions) == 0 {
		t.Errorf("supportedVersions = %v, want a non-empty list", result["supportedVersions"])
	}
}

// TestServerDiscover_ModernRequestMissingClientCapabilities_Rejected
// covers the other side of the same fix: a server/discover call that
// does carry _meta.protocolVersion (so it is modern-shaped) is no longer
// exempt from validation, and must be rejected the same way any other
// modern method would be when it omits the required clientCapabilities
// field.
func TestServerDiscover_ModernRequestMissingClientCapabilities_Rejected(t *testing.T) {
	server := NewServer(&mockToolProvider{})
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "server/discover",
		Params: map[string]interface{}{
			"_meta": map[string]interface{}{
				"io.modelcontextprotocol/protocolVersion": "2026-07-28",
			},
		},
	}

	out := captureStdout(t, func() {
		server.handleRequest(req)
	})

	var resp JSONRPCResponse
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("failed to decode response %q: %v", out, err)
	}
	if resp.Error == nil || resp.Error.Code != -32602 {
		t.Fatalf("error = %+v, want code -32602 (Invalid params)", resp.Error)
	}
}

// TestHandleInitialize_Stdio_NegotiatesProtocolVersion is the regression
// test for issue #212 on the stdio transport: a client requesting a
// revision this server does not implement must get back a revision the
// server actually speaks, not its own request echoed unchanged.
func TestHandleInitialize_Stdio_NegotiatesProtocolVersion(t *testing.T) {
	server := NewServer(&mockToolProvider{})
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "initialize",
		Params: map[string]interface{}{
			"protocolVersion": "2099-01-01",
			"clientInfo":      map[string]interface{}{"name": "test-client", "version": "1.0.0"},
		},
	}

	out := captureStdout(t, func() {
		server.handleInitialize(req)
	})

	var resp JSONRPCResponse
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("failed to decode response %q: %v", out, err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error in response: %v", resp.Error)
	}
	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", resp.Result)
	}
	if result["protocolVersion"] == "2099-01-01" {
		t.Fatal("server echoed back an unsupported protocolVersion instead of negotiating")
	}
	if result["protocolVersion"] != ProtocolVersion {
		t.Errorf("protocolVersion = %v, want %q", result["protocolVersion"], ProtocolVersion)
	}
}

// TestHandleInitialize_Stdio_MalformedProtocolVersion checks that a
// protocolVersion of the wrong JSON type is rejected with a proper
// JSON-RPC error, matching the HTTP transport's handleInitializeHTTP --
// not silently negotiated as if the field had been omitted. This
// behavior predates issue #212's fix and is unchanged by it; the test
// pins it down explicitly since the equivalent HTTP behavior was added
// as part of that fix and the two transports must stay consistent.
func TestHandleInitialize_Stdio_MalformedProtocolVersion(t *testing.T) {
	server := NewServer(&mockToolProvider{})
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "initialize",
		Params: map[string]interface{}{
			"protocolVersion": 12345, // wrong type: InitializeParams wants a string
		},
	}

	out := captureStdout(t, func() {
		server.handleInitialize(req)
	})

	var resp JSONRPCResponse
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("failed to decode response %q: %v", out, err)
	}
	if resp.Error == nil {
		t.Fatal("expected an Invalid params error, got a successful result")
	}
	if resp.Error.Code != -32602 {
		t.Errorf("error code = %d, want -32602", resp.Error.Code)
	}
}

// runStdio runs server.Run() with the given input piped to stdin and
// returns whatever the server wrote to stdout. The input may contain
// multiple newline-separated JSON-RPC messages; closing the write end
// after writing causes Run to return cleanly on EOF.
func runStdio(t *testing.T, server *Server, input string) string {
	t.Helper()

	oldIn := os.Stdin
	rIn, wIn, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe (stdin) failed: %v", err)
	}
	os.Stdin = rIn
	defer func() {
		os.Stdin = oldIn
		_ = rIn.Close()
	}()

	if _, err := io.WriteString(wIn, input); err != nil {
		t.Fatalf("write stdin failed: %v", err)
	}
	if err := wIn.Close(); err != nil {
		t.Fatalf("close stdin write end failed: %v", err)
	}

	return captureStdout(t, func() {
		if err := server.Run(); err != nil {
			t.Fatalf("server.Run failed: %v", err)
		}
	})
}

// TestRunStdio_NotificationsInitialized verifies that a real JSON-RPC
// notification (no "id" member, per §4.1) produces no response on stdio.
func TestRunStdio_NotificationsInitialized(t *testing.T) {
	server := NewServer(&mockToolProvider{})
	input := `{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}` + "\n"

	out := runStdio(t, server, input)
	if out != "" {
		t.Errorf("notification must produce no response, got %q", out)
	}
}

// TestRunStdio_UnknownNotification verifies that unknown-method
// notifications are silently acknowledged. Replying with -32601 to a
// notification would be doubly wrong per JSON-RPC 2.0 §4.1: the server
// must not reply at all, and the reply would have no id (itself a
// malformed JSON-RPC body).
func TestRunStdio_UnknownNotification(t *testing.T) {
	for _, method := range []string{
		"notifications/cancelled",
		"notifications/roots/list_changed",
		"some/unknown/notification",
	} {
		t.Run(method, func(t *testing.T) {
			server := NewServer(&mockToolProvider{})
			input := `{"jsonrpc":"2.0","method":"` + method + `","params":{}}` + "\n"

			out := runStdio(t, server, input)
			if out != "" {
				t.Errorf("unknown notification must produce no response, got %q", out)
			}
		})
	}
}

// TestRunStdio_NullIDIsRequest_UnknownMethod is the regression for issue
// #152: JSON-RPC 2.0 distinguishes "id absent" (notification, no reply)
// from "id: null" (a request whose id happens to be null; reply
// required). A request with explicit null id targeting an unknown
// method must receive a -32601 response, not be silently dropped.
func TestRunStdio_NullIDIsRequest_UnknownMethod(t *testing.T) {
	server := NewServer(&mockToolProvider{})
	input := `{"jsonrpc":"2.0","id":null,"method":"unknown/method"}` + "\n"

	out := runStdio(t, server, input)
	if out == "" {
		t.Fatal("expected a -32601 response for a request with null id, got nothing")
	}

	// Per JSON-RPC 2.0 §5.1 the response object MUST include the id
	// member; for a request with explicit "id": null the response id
	// is also null. Verify on the raw bytes because Decode into
	// JSONRPCResponse cannot distinguish absent id from id: null.
	if !strings.Contains(out, `"id":null`) {
		t.Errorf("expected response to include `\"id\":null`, got %q", out)
	}

	var resp JSONRPCResponse
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("failed to decode response %q: %v", out, err)
	}
	if resp.Error == nil || resp.Error.Code != -32601 {
		t.Errorf("expected -32601 Method not found, got %+v", resp.Error)
	}
}

// TestRunStdio_NullIDIsRequest_Ping is the companion to
// TestRunStdio_NullIDIsRequest_UnknownMethod for a method the
// dispatcher recognises: a ping with explicit "id": null is a request
// and must be answered with the standard empty-object result.
func TestRunStdio_NullIDIsRequest_Ping(t *testing.T) {
	server := NewServer(&mockToolProvider{})
	input := `{"jsonrpc":"2.0","id":null,"method":"ping"}` + "\n"

	out := runStdio(t, server, input)
	if out == "" {
		t.Fatal("expected a ping response for a request with null id, got nothing")
	}

	if !strings.Contains(out, `"id":null`) {
		t.Errorf("expected response to include `\"id\":null`, got %q", out)
	}

	var resp JSONRPCResponse
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("failed to decode response %q: %v", out, err)
	}
	if resp.Error != nil {
		t.Errorf("unexpected error in response: %+v", resp.Error)
	}
	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", resp.Result)
	}
	if len(result) != 0 {
		t.Errorf("expected empty object result, got %v", result)
	}
}
