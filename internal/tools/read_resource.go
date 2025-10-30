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
	"pgedge-mcp/internal/mcp"
)

// ResourceReader is an interface for reading resources
type ResourceReader interface {
	List() []mcp.Resource
	Read(uri string) (mcp.ResourceContent, error)
}

// ReadResourceTool creates a tool that allows Claude to read MCP resources
func ReadResourceTool(resourceProvider ResourceReader) Tool {
	return Tool{
		Definition: mcp.Tool{
			Name:        "read_resource",
			Description: "Read the contents of an MCP resource by its URI. Resources provide read-only access to PostgreSQL system information. Available resources can be listed, and include pg://system_info (PostgreSQL version and platform information) and pg://settings (server configuration parameters).",
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"uri": map[string]interface{}{
						"type":        "string",
						"description": "The URI of the resource to read (e.g., 'pg://system_info' or 'pg://settings')",
					},
					"list": map[string]interface{}{
						"type":        "boolean",
						"description": "Optional: if true, list all available resources instead of reading a specific one",
					},
				},
				Required: []string{},
			},
		},
		Handler: func(args map[string]interface{}) (mcp.ToolResponse, error) {
			// Check if listing resources was requested
			if list, ok := args["list"].(bool); ok && list {
				resources := resourceProvider.List()

				content := "Available Resources:\n"
				content += "====================\n\n"

				for _, resource := range resources {
					content += "URI: " + resource.URI + "\n"
					content += "Name: " + resource.Name + "\n"
					content += "Description: " + resource.Description + "\n"
					content += "MIME Type: " + resource.MimeType + "\n\n"
				}

				return mcp.ToolResponse{
					Content: []mcp.ContentItem{
						{
							Type: "text",
							Text: content,
						},
					},
				}, nil
			}

			// Read a specific resource
			uri, ok := args["uri"].(string)
			if !ok || uri == "" {
				return mcp.ToolResponse{
					Content: []mcp.ContentItem{
						{
							Type: "text",
							Text: "Error: 'uri' parameter is required. Provide a resource URI (e.g., 'pg://system_info') or use 'list': true to see all available resources.",
						},
					},
					IsError: true,
				}, nil
			}

			resourceContent, err := resourceProvider.Read(uri)
			if err != nil {
				return mcp.ToolResponse{
					Content: []mcp.ContentItem{
						{
							Type: "text",
							Text: "Error reading resource: " + err.Error(),
						},
					},
					IsError: true,
				}, nil
			}

			// Return the resource contents
			return mcp.ToolResponse{
				Content: resourceContent.Contents,
			}, nil
		},
	}
}
