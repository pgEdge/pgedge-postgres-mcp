/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent - Execute Explain Tool Tests
 *
 * Portions copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package tools

import (
	"strings"
	"testing"
)

func TestExecuteExplainToolDefinition(t *testing.T) {
	tool := ExecuteExplainTool(nil)

	if tool.Definition.Name != "execute_explain" {
		t.Errorf("Tool name = %v, want execute_explain", tool.Definition.Name)
	}

	if tool.Definition.Description == "" {
		t.Error("Tool description is empty")
	}

	// Verify description contains key sections
	desc := tool.Definition.Description
	requiredSections := []string{
		"<usecase>",
		"<what_it_returns>",
		"<when_not_to_use>",
		"<examples>",
		"<safety>",
	}

	for _, section := range requiredSections {
		if !strings.Contains(desc, section) {
			t.Errorf("Description missing required section: %s", section)
		}
	}

	// Verify input schema
	schema := tool.Definition.InputSchema
	if schema.Type != "object" {
		t.Errorf("InputSchema.Type = %v, want object", schema.Type)
	}

	// Verify required parameters
	if len(schema.Required) != 1 || schema.Required[0] != "query" {
		t.Errorf("Required parameters = %v, want [query]", schema.Required)
	}

	// Verify properties exist
	expectedProps := []string{"query", "analyze", "buffers", "format"}
	for _, prop := range expectedProps {
		if _, exists := schema.Properties[prop]; !exists {
			t.Errorf("Missing property: %s", prop)
		}
	}
}

func TestExecuteExplainValidation(t *testing.T) {
	tool := ExecuteExplainTool(nil)

	tests := []struct {
		name        string
		args        map[string]interface{}
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Missing query parameter",
			args:        map[string]interface{}{},
			expectError: true,
			errorMsg:    "query",
		},
		{
			name: "Empty query parameter",
			args: map[string]interface{}{
				"query": "",
			},
			expectError: true,
			errorMsg:    "non-empty string",
		},
		{
			name: "Non-SELECT query rejected",
			args: map[string]interface{}{
				"query": "INSERT INTO users (name) VALUES ('test')",
			},
			expectError: true,
			errorMsg:    "Only SELECT queries",
		},
		{
			name: "UPDATE query rejected",
			args: map[string]interface{}{
				"query": "UPDATE users SET name = 'test'",
			},
			expectError: true,
			errorMsg:    "Only SELECT queries",
		},
		{
			name: "DELETE query rejected",
			args: map[string]interface{}{
				"query": "DELETE FROM users WHERE id = 1",
			},
			expectError: true,
			errorMsg:    "Only SELECT queries",
		},
		{
			name: "DDL query rejected",
			args: map[string]interface{}{
				"query": "CREATE TABLE test (id INT)",
			},
			expectError: true,
			errorMsg:    "Only SELECT queries",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response, err := tool.Handler(tt.args)

			if tt.expectError {
				if err == nil && !response.IsError {
					t.Errorf("Expected error, but got none")
				}

				// Check error message contains expected text
				var errorText string
				if err != nil {
					errorText = err.Error()
				} else if len(response.Content) > 0 {
					errorText = response.Content[0].Text
				}

				if !strings.Contains(errorText, tt.errorMsg) {
					t.Errorf("Error message %q does not contain %q", errorText, tt.errorMsg)
				}
			}
		})
	}
}

func TestAnalyzeExplainOutput(t *testing.T) {
	tests := []struct {
		name           string
		explainText    string
		expectIssues   bool
		expectInText   []string
		dontExpectText []string
	}{
		{
			name: "Sequential scan detected",
			explainText: `Seq Scan on users  (cost=0.00..10.50 rows=100 width=40)
  actual time=0.010..0.023 rows=100 loops=1`,
			expectIssues: true,
			expectInText: []string{"Sequential scan", "users"},
		},
		{
			name: "Hash join detected",
			explainText: `Hash Join  (cost=22.50..49.20 rows=200 width=16)
  actual time=0.215..0.438 rows=200 loops=1
  Hash Cond: (orders.user_id = users.id)`,
			expectIssues: true,
			expectInText: []string{"Hash join detected"},
		},
		{
			name: "Index scan (no issues)",
			explainText: `Index Scan using idx_users_email on users  (cost=0.28..8.29 rows=1 width=40)
  actual time=0.015..0.016 rows=1 loops=1`,
			expectIssues:   false,
			dontExpectText: []string{"Sequential scan", "⚠️"},
		},
		{
			name: "Sort spilling to disk",
			explainText: `Sort  (cost=120.50..125.25 rows=1900 width=8)
  Sort Key: created_at
  Sort Method: external merge  Disk: 1024kB
  actual time=50.234..60.182 rows=2000 loops=1`,
			expectIssues: true,
			expectInText: []string{"spilling to disk", "work_mem"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := analyzeExplainOutput(tt.explainText)

			if tt.expectIssues && result == "" {
				t.Error("Expected analysis output, got empty string")
			}

			for _, expected := range tt.expectInText {
				if !strings.Contains(result, expected) {
					t.Errorf("Analysis missing expected text: %q\nGot: %s", expected, result)
				}
			}

			for _, notExpected := range tt.dontExpectText {
				if strings.Contains(result, notExpected) {
					t.Errorf("Analysis contains unexpected text: %q\nGot: %s", notExpected, result)
				}
			}
		})
	}
}

func TestExecuteExplainToolResponseFormat(t *testing.T) {
	// This test verifies the tool definition format
	tool := ExecuteExplainTool(nil)

	// Verify tool definition structure
	if tool.Definition.Name != "execute_explain" {
		t.Errorf("Tool name = %v, want execute_explain", tool.Definition.Name)
	}

	// Verify handler exists
	if tool.Handler == nil {
		t.Error("Handler is nil")
	}

	// Verify input schema has required structure
	schema := tool.Definition.InputSchema
	if schema.Type != "object" {
		t.Errorf("Schema type = %v, want object", schema.Type)
	}

	// Note: We don't execute the handler with nil client as it would crash
	// Execution tests require a real database connection
}

func TestExecuteExplainBooleanDefaults(t *testing.T) {
	tool := ExecuteExplainTool(nil)

	// Test that boolean parameters have proper defaults
	schema := tool.Definition.InputSchema

	// Check analyze default
	if analyzeProps, ok := schema.Properties["analyze"].(map[string]interface{}); ok {
		if analyzeProps["default"] != true {
			t.Errorf("analyze default = %v, want true", analyzeProps["default"])
		}
	}

	// Check buffers default
	if buffersProps, ok := schema.Properties["buffers"].(map[string]interface{}); ok {
		if buffersProps["default"] != true {
			t.Errorf("buffers default = %v, want true", buffersProps["default"])
		}
	}

	// Check format default
	if formatProps, ok := schema.Properties["format"].(map[string]interface{}); ok {
		if formatProps["default"] != "text" {
			t.Errorf("format default = %v, want 'text'", formatProps["default"])
		}
	}
}

func TestExecuteExplainToolRegistration(t *testing.T) {
	// Verify that execute_explain tool can be registered
	registry := NewRegistry()
	tool := ExecuteExplainTool(nil)

	registry.Register("execute_explain", tool)

	retrieved, exists := registry.Get("execute_explain")
	if !exists {
		t.Error("execute_explain tool not found after registration")
	}

	if retrieved.Definition.Name != "execute_explain" {
		t.Errorf("Retrieved tool name = %v, want execute_explain", retrieved.Definition.Name)
	}
}

func TestExecuteExplainReturnsToolResponse(t *testing.T) {
	// Test that validation errors return proper tool responses without requiring DB
	tool := ExecuteExplainTool(nil)

	// Test with missing query (validation error, no DB needed)
	response, _ := tool.Handler(map[string]interface{}{})

	// Verify response is a valid ToolResponse type
	if response.Content == nil {
		t.Error("Response Content is nil")
	}

	// Response should have content (even if it's an error)
	if len(response.Content) == 0 {
		t.Error("Response has no content items")
	}

	// First content item should be text type
	if response.Content[0].Type != "text" {
		t.Errorf("Content type = %v, want text", response.Content[0].Type)
	}

	// Should be an error response
	if !response.IsError {
		t.Error("Validation error should set IsError to true")
	}
}

func TestExecuteExplainToolResponse(t *testing.T) {
	// Test that execute_explain properly uses mcp.NewToolError and mcp.NewToolSuccess
	// This is tested implicitly through the validation tests above
	tool := ExecuteExplainTool(nil)

	// Test validation error response
	response, _ := tool.Handler(map[string]interface{}{})
	if !response.IsError {
		t.Error("Missing query should return error response")
	}

	// Test non-SELECT error response
	response, _ = tool.Handler(map[string]interface{}{"query": "INSERT INTO test VALUES (1)"})
	if !response.IsError {
		t.Error("Non-SELECT query should return error response")
	}
}
