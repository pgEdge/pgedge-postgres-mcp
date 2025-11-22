/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package tools

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"pgedge-postgres-mcp/internal/mcp"
)

// Mock ResourceReader for testing
type mockResourceReader struct {
	resources []mcp.Resource
	readFunc  func(ctx context.Context, uri string) (mcp.ResourceContent, error)
}

func (m *mockResourceReader) List() []mcp.Resource {
	return m.resources
}

func (m *mockResourceReader) Read(ctx context.Context, uri string) (mcp.ResourceContent, error) {
	if m.readFunc != nil {
		return m.readFunc(ctx, uri)
	}
	return mcp.ResourceContent{}, fmt.Errorf("resource not found")
}

func TestReadResourceTool(t *testing.T) {
	t.Run("list all resources", func(t *testing.T) {
		mockReader := &mockResourceReader{
			resources: []mcp.Resource{
				{
					URI:         "pg://system_info",
					Name:        "PostgreSQL System Information",
					Description: "Returns version and system info",
					MimeType:    "application/json",
				},
			},
		}

		tool := ReadResourceTool(mockReader)
		response, err := tool.Handler(map[string]interface{}{
			"list": true,
		})

		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}
		if response.IsError {
			t.Error("Expected IsError=false when listing resources")
		}
		if len(response.Content) == 0 {
			t.Fatal("Expected content in response")
		}

		content := response.Content[0].Text

		if !strings.Contains(content, "Available Resources") {
			t.Error("Expected 'Available Resources' header")
		}
		if !strings.Contains(content, "pg://system_info") {
			t.Error("Expected 'pg://system_info' URI")
		}
		if !strings.Contains(content, "PostgreSQL System Information") {
			t.Error("Expected system info resource name")
		}
	})

	t.Run("list resources when empty", func(t *testing.T) {
		mockReader := &mockResourceReader{
			resources: []mcp.Resource{},
		}

		tool := ReadResourceTool(mockReader)
		response, err := tool.Handler(map[string]interface{}{
			"list": true,
		})

		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}
		if response.IsError {
			t.Error("Expected IsError=false even with empty resource list")
		}

		content := response.Content[0].Text
		if !strings.Contains(content, "Available Resources") {
			t.Error("Expected 'Available Resources' header")
		}
	})

	t.Run("missing uri parameter", func(t *testing.T) {
		mockReader := &mockResourceReader{}
		tool := ReadResourceTool(mockReader)

		response, err := tool.Handler(map[string]interface{}{})

		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}
		if !response.IsError {
			t.Error("Expected IsError=true when uri is missing")
		}

		content := response.Content[0].Text
		if !strings.Contains(content, "'uri' parameter is required") {
			t.Errorf("Expected uri required error, got: %s", content)
		}
	})

	t.Run("empty uri string", func(t *testing.T) {
		mockReader := &mockResourceReader{}
		tool := ReadResourceTool(mockReader)

		response, err := tool.Handler(map[string]interface{}{
			"uri": "",
		})

		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}
		if !response.IsError {
			t.Error("Expected IsError=true when uri is empty")
		}

		content := response.Content[0].Text
		if !strings.Contains(content, "'uri' parameter is required") {
			t.Errorf("Expected uri required error, got: %s", content)
		}
	})

	t.Run("invalid uri type", func(t *testing.T) {
		mockReader := &mockResourceReader{}
		tool := ReadResourceTool(mockReader)

		response, err := tool.Handler(map[string]interface{}{
			"uri": 123, // Invalid type
		})

		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}
		if !response.IsError {
			t.Error("Expected IsError=true when uri has invalid type")
		}

		content := response.Content[0].Text
		if !strings.Contains(content, "'uri' parameter is required") {
			t.Errorf("Expected uri required error, got: %s", content)
		}
	})

	t.Run("read specific resource successfully", func(t *testing.T) {
		mockReader := &mockResourceReader{
			readFunc: func(ctx context.Context, uri string) (mcp.ResourceContent, error) {
				if uri == "pg://system_info" {
					return mcp.ResourceContent{
						URI:      "pg://system_info",
						MimeType: "application/json",
						Contents: []mcp.ContentItem{
							{
								Type: "text",
								Text: `{"version": "15.4", "os": "linux"}`,
							},
						},
					}, nil
				}
				return mcp.ResourceContent{}, fmt.Errorf("resource not found: %s", uri)
			},
		}

		tool := ReadResourceTool(mockReader)
		response, err := tool.Handler(map[string]interface{}{
			"uri": "pg://system_info",
		})

		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}
		if response.IsError {
			t.Error("Expected IsError=false when reading valid resource")
		}
		if len(response.Content) == 0 {
			t.Fatal("Expected content in response")
		}

		content := response.Content[0].Text
		if !strings.Contains(content, `"version": "15.4"`) {
			t.Errorf("Expected resource content, got: %s", content)
		}
	})

	t.Run("read resource returns error", func(t *testing.T) {
		mockReader := &mockResourceReader{
			readFunc: func(ctx context.Context, uri string) (mcp.ResourceContent, error) {
				return mcp.ResourceContent{}, fmt.Errorf("resource not found: %s", uri)
			},
		}

		tool := ReadResourceTool(mockReader)
		response, err := tool.Handler(map[string]interface{}{
			"uri": "pg://nonexistent",
		})

		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}
		if !response.IsError {
			t.Error("Expected IsError=true when resource read fails")
		}

		content := response.Content[0].Text
		if !strings.Contains(content, "Error reading resource") {
			t.Errorf("Expected read error message, got: %s", content)
		}
		if !strings.Contains(content, "resource not found") {
			t.Errorf("Expected specific error from mock, got: %s", content)
		}
	})

	t.Run("list parameter false uses uri", func(t *testing.T) {
		mockReader := &mockResourceReader{
			readFunc: func(ctx context.Context, uri string) (mcp.ResourceContent, error) {
				return mcp.ResourceContent{
					URI:      uri,
					MimeType: "application/json",
					Contents: []mcp.ContentItem{
						{
							Type: "text",
							Text: "resource content",
						},
					},
				}, nil
			},
		}

		tool := ReadResourceTool(mockReader)
		response, err := tool.Handler(map[string]interface{}{
			"uri":  "pg://system_info",
			"list": false,
		})

		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}
		if response.IsError {
			t.Error("Expected IsError=false when list=false and valid uri provided")
		}

		content := response.Content[0].Text
		if content != "resource content" {
			t.Errorf("Expected resource content, got: %s", content)
		}
	})

	t.Run("tool definition has correct structure", func(t *testing.T) {
		mockReader := &mockResourceReader{}
		tool := ReadResourceTool(mockReader)

		if tool.Definition.Name != "read_resource" {
			t.Errorf("Expected name 'read_resource', got %s", tool.Definition.Name)
		}

		if tool.Definition.Description == "" {
			t.Error("Description should not be empty")
		}

		if tool.Definition.InputSchema.Type != "object" {
			t.Errorf("Expected input schema type 'object', got %s", tool.Definition.InputSchema.Type)
		}

		// Check properties exist
		props := tool.Definition.InputSchema.Properties
		if _, ok := props["uri"]; !ok {
			t.Error("'uri' property should exist")
		}
		if _, ok := props["list"]; !ok {
			t.Error("'list' property should exist")
		}

		// Required should be empty (both params are optional)
		if len(tool.Definition.InputSchema.Required) != 0 {
			t.Errorf("Expected 0 required fields, got %d", len(tool.Definition.InputSchema.Required))
		}
	})
}
