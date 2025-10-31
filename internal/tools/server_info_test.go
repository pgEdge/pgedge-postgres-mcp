/*-------------------------------------------------------------------------
 *
 * pgEdge Postgres MCP Server
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package tools

import (
	"strings"
	"testing"
)

func TestServerInfoTool(t *testing.T) {
	t.Run("basic server info", func(t *testing.T) {
		info := ServerInfo{
			Name:     "Test MCP Server",
			Company:  "Test Company",
			Version:  "1.0.0",
			Provider: "anthropic",
			Model:    "claude-sonnet-4-5",
		}

		tool := ServerInfoTool(info)

		// Verify tool definition
		if tool.Definition.Name != "server_info" {
			t.Errorf("Expected tool name 'server_info', got %q", tool.Definition.Name)
		}

		if tool.Definition.Description == "" {
			t.Error("Expected non-empty description")
		}

		// Execute the tool
		response, err := tool.Handler(map[string]interface{}{})
		if err != nil {
			t.Fatalf("Handler returned unexpected error: %v", err)
		}

		if response.IsError {
			t.Error("Expected successful response, got error")
		}

		if len(response.Content) != 1 {
			t.Fatalf("Expected 1 content item, got %d", len(response.Content))
		}

		output := response.Content[0].Text

		// Verify all fields are present in output
		expectedStrings := []string{
			"Test MCP Server",
			"Test Company",
			"1.0.0",
			"anthropic",
			"claude-sonnet-4-5",
		}

		for _, expected := range expectedStrings {
			if !strings.Contains(output, expected) {
				t.Errorf("Expected output to contain %q, got:\n%s", expected, output)
			}
		}
	})

	t.Run("ollama provider", func(t *testing.T) {
		info := ServerInfo{
			Name:     "pgEdge PostgreSQL MCP Server",
			Company:  "pgEdge, Inc.",
			Version:  "1.0.0",
			Provider: "ollama",
			Model:    "qwen2.5-coder:32b",
		}

		tool := ServerInfoTool(info)
		response, err := tool.Handler(map[string]interface{}{})
		if err != nil {
			t.Fatalf("Handler returned unexpected error: %v", err)
		}

		output := response.Content[0].Text

		// Verify Ollama-specific info
		if !strings.Contains(output, "ollama") {
			t.Error("Expected output to contain provider 'ollama'")
		}
		if !strings.Contains(output, "qwen2.5-coder:32b") {
			t.Error("Expected output to contain model 'qwen2.5-coder:32b'")
		}
	})

	t.Run("tool accepts no arguments", func(t *testing.T) {
		info := ServerInfo{
			Name:     "Test Server",
			Company:  "Test Co",
			Version:  "1.0",
			Provider: "test",
			Model:    "test-model",
		}

		tool := ServerInfoTool(info)

		// Verify input schema has no required properties
		if tool.Definition.InputSchema.Type != "object" {
			t.Errorf("Expected input schema type 'object', got %q", tool.Definition.InputSchema.Type)
		}

		// Execute with arguments (should still work, arguments ignored)
		response, err := tool.Handler(map[string]interface{}{
			"ignored_arg": "value",
		})
		if err != nil {
			t.Fatalf("Handler returned unexpected error: %v", err)
		}

		if response.IsError {
			t.Error("Expected successful response even with extra arguments")
		}
	})

	t.Run("output format consistency", func(t *testing.T) {
		info := ServerInfo{
			Name:     "Server",
			Company:  "Company",
			Version:  "2.0.0",
			Provider: "provider",
			Model:    "model",
		}

		tool := ServerInfoTool(info)
		response, err := tool.Handler(map[string]interface{}{})
		if err != nil {
			t.Fatalf("Handler returned unexpected error: %v", err)
		}

		output := response.Content[0].Text

		// Verify output contains section headers
		expectedHeaders := []string{
			"Server Information:",
			"Server Name:",
			"Company:",
			"Version:",
			"LLM Provider:",
			"LLM Model:",
		}

		for _, header := range expectedHeaders {
			if !strings.Contains(output, header) {
				t.Errorf("Expected output to contain header %q", header)
			}
		}
	})

	t.Run("empty string fields", func(t *testing.T) {
		// Test with empty fields (edge case)
		info := ServerInfo{
			Name:     "",
			Company:  "",
			Version:  "",
			Provider: "",
			Model:    "",
		}

		tool := ServerInfoTool(info)
		response, err := tool.Handler(map[string]interface{}{})
		if err != nil {
			t.Fatalf("Handler returned unexpected error: %v", err)
		}

		if response.IsError {
			t.Error("Expected successful response even with empty fields")
		}

		// Tool should still return structured output
		output := response.Content[0].Text
		if !strings.Contains(output, "Server Name:") {
			t.Error("Expected output to contain 'Server Name:' label")
		}
	})
}
