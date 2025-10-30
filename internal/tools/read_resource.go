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
			Description: "Read the contents of an MCP resource by its URI. Resources provide read-only access to PostgreSQL system information and statistics. Available resources include: System Info (pg://system_info, pg://settings), Activity (pg://stat/activity), Database Stats (pg://stat/database), Table Stats (pg://stat/user_tables), Index Stats (pg://stat/user_indexes), Replication (pg://stat/replication), Background Writer (pg://stat/bgwriter), and WAL Stats (pg://stat/wal). Use list=true to see all resources with full descriptions.",
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"uri": map[string]interface{}{
						"type":        "string",
						"description": "The URI of the resource to read. Examples: 'pg://system_info', 'pg://settings', 'pg://stat/activity', 'pg://stat/database', 'pg://stat/user_tables', 'pg://stat/user_indexes', 'pg://stat/replication', 'pg://stat/bgwriter', 'pg://stat/wal'",
					},
					"list": map[string]interface{}{
						"type":        "boolean",
						"description": "Optional: if true, list all available resources with their full descriptions instead of reading a specific one",
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
