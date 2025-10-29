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
	"testing"

	"pgedge-mcp/internal/mcp"
)

func TestNewRegistry(t *testing.T) {
	registry := NewRegistry()

	if registry == nil {
		t.Fatal("NewRegistry() returned nil")
	}

	if registry.tools == nil {
		t.Error("tools map is nil")
	}

	if len(registry.tools) != 0 {
		t.Errorf("tools map should be empty, got %d entries", len(registry.tools))
	}
}

func TestRegister(t *testing.T) {
	registry := NewRegistry()

	tool := Tool{
		Definition: mcp.Tool{
			Name:        "test_tool",
			Description: "A test tool",
		},
		Handler: func(args map[string]interface{}) (mcp.ToolResponse, error) {
			return mcp.ToolResponse{}, nil
		},
	}

	registry.Register("test_tool", tool)

	if len(registry.tools) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(registry.tools))
	}

	retrieved, exists := registry.tools["test_tool"]
	if !exists {
		t.Error("Tool 'test_tool' was not registered")
	}

	if retrieved.Definition.Name != "test_tool" {
		t.Errorf("Tool name = %q, want %q", retrieved.Definition.Name, "test_tool")
	}
}

func TestGet(t *testing.T) {
	registry := NewRegistry()

	tool := Tool{
		Definition: mcp.Tool{
			Name:        "existing_tool",
			Description: "An existing tool",
		},
		Handler: func(args map[string]interface{}) (mcp.ToolResponse, error) {
			return mcp.ToolResponse{}, nil
		},
	}

	registry.Register("existing_tool", tool)

	t.Run("existing tool", func(t *testing.T) {
		retrieved, exists := registry.Get("existing_tool")
		if !exists {
			t.Error("Get() returned exists=false for existing tool")
		}
		if retrieved.Definition.Name != "existing_tool" {
			t.Errorf("Tool name = %q, want %q", retrieved.Definition.Name, "existing_tool")
		}
	})

	t.Run("non-existent tool", func(t *testing.T) {
		_, exists := registry.Get("non_existent")
		if exists {
			t.Error("Get() returned exists=true for non-existent tool")
		}
	})
}

func TestList(t *testing.T) {
	registry := NewRegistry()

	t.Run("empty registry", func(t *testing.T) {
		tools := registry.List()
		if len(tools) != 0 {
			t.Errorf("List() returned %d tools, want 0", len(tools))
		}
	})

	t.Run("with tools", func(t *testing.T) {
		tool1 := Tool{
			Definition: mcp.Tool{
				Name:        "tool1",
				Description: "First tool",
			},
			Handler: func(args map[string]interface{}) (mcp.ToolResponse, error) {
				return mcp.ToolResponse{}, nil
			},
		}

		tool2 := Tool{
			Definition: mcp.Tool{
				Name:        "tool2",
				Description: "Second tool",
			},
			Handler: func(args map[string]interface{}) (mcp.ToolResponse, error) {
				return mcp.ToolResponse{}, nil
			},
		}

		registry.Register("tool1", tool1)
		registry.Register("tool2", tool2)

		tools := registry.List()
		if len(tools) != 2 {
			t.Errorf("List() returned %d tools, want 2", len(tools))
		}

		// Verify both tools are in the list
		names := make(map[string]bool)
		for _, tool := range tools {
			names[tool.Name] = true
		}

		if !names["tool1"] {
			t.Error("List() missing 'tool1'")
		}
		if !names["tool2"] {
			t.Error("List() missing 'tool2'")
		}
	})
}

func TestExecute(t *testing.T) {
	registry := NewRegistry()

	t.Run("successful execution", func(t *testing.T) {
		callCount := 0
		tool := Tool{
			Definition: mcp.Tool{
				Name:        "counter",
				Description: "Counts calls",
			},
			Handler: func(args map[string]interface{}) (mcp.ToolResponse, error) {
				callCount++
				value, ok := args["value"].(string)
				if !ok {
					value = "default"
				}

				return mcp.ToolResponse{
					Content: []mcp.ContentItem{
						{
							Type: "text",
							Text: "Called with: " + value,
						},
					},
				}, nil
			},
		}

		registry.Register("counter", tool)

		args := map[string]interface{}{
			"value": "test",
		}

		response, err := registry.Execute("counter", args)
		if err != nil {
			t.Errorf("Execute() unexpected error: %v", err)
		}

		if callCount != 1 {
			t.Errorf("Handler was called %d times, want 1", callCount)
		}

		if len(response.Content) != 1 {
			t.Errorf("Response has %d content items, want 1", len(response.Content))
		}

		if response.Content[0].Text != "Called with: test" {
			t.Errorf("Response text = %q, want %q", response.Content[0].Text, "Called with: test")
		}
	})

	t.Run("non-existent tool", func(t *testing.T) {
		response, err := registry.Execute("non_existent", map[string]interface{}{})
		if err != nil {
			t.Errorf("Execute() unexpected error: %v", err)
		}

		if !response.IsError {
			t.Error("Execute() should set IsError=true for non-existent tool")
		}

		if len(response.Content) == 0 {
			t.Fatal("Response should have content")
		}

		if response.Content[0].Text != "Tool not found: non_existent" {
			t.Errorf("Response text = %q, want %q", response.Content[0].Text, "Tool not found: non_existent")
		}
	})

	t.Run("handler with error response", func(t *testing.T) {
		tool := Tool{
			Definition: mcp.Tool{
				Name:        "error_tool",
				Description: "Returns an error",
			},
			Handler: func(args map[string]interface{}) (mcp.ToolResponse, error) {
				return mcp.ToolResponse{
					Content: []mcp.ContentItem{
						{
							Type: "text",
							Text: "Something went wrong",
						},
					},
					IsError: true,
				}, nil
			},
		}

		registry.Register("error_tool", tool)

		response, err := registry.Execute("error_tool", map[string]interface{}{})
		if err != nil {
			t.Errorf("Execute() unexpected error: %v", err)
		}

		if !response.IsError {
			t.Error("Execute() should preserve IsError=true from handler")
		}

		if response.Content[0].Text != "Something went wrong" {
			t.Errorf("Response text = %q, want %q", response.Content[0].Text, "Something went wrong")
		}
	})

	t.Run("multiple registrations overwrite", func(t *testing.T) {
		version := 1
		tool1 := Tool{
			Definition: mcp.Tool{
				Name:        "versioned_tool",
				Description: "Version 1",
			},
			Handler: func(args map[string]interface{}) (mcp.ToolResponse, error) {
				return mcp.ToolResponse{
					Content: []mcp.ContentItem{
						{
							Type: "text",
							Text: "Version 1",
						},
					},
				}, nil
			},
		}

		registry.Register("versioned_tool", tool1)

		// Register again with different handler
		tool2 := Tool{
			Definition: mcp.Tool{
				Name:        "versioned_tool",
				Description: "Version 2",
			},
			Handler: func(args map[string]interface{}) (mcp.ToolResponse, error) {
				version = 2
				return mcp.ToolResponse{
					Content: []mcp.ContentItem{
						{
							Type: "text",
							Text: "Version 2",
						},
					},
				}, nil
			},
		}

		registry.Register("versioned_tool", tool2)

		response, err := registry.Execute("versioned_tool", map[string]interface{}{})
		if err != nil {
			t.Errorf("Execute() unexpected error: %v", err)
		}

		if version != 2 {
			t.Errorf("Version = %d, want 2 (latest registration)", version)
		}

		if response.Content[0].Text != "Version 2" {
			t.Errorf("Response text = %q, want %q", response.Content[0].Text, "Version 2")
		}
	})
}
