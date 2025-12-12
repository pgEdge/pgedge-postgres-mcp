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
	"context"
	"errors"
	"testing"
)

// Mock implementations for testing

type mockToolProvider struct {
	tools       []Tool
	executeFunc func(ctx context.Context, name string, args map[string]interface{}) (ToolResponse, error)
}

func (m *mockToolProvider) List() []Tool {
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
