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

	"pgedge-postgres-mcp/internal/mcp"
)

// ResourceReader is an interface for reading resources
type ResourceReader interface {
	List() []mcp.Resource
	Read(ctx context.Context, uri string) (mcp.ResourceContent, error)
}

// ReadResourceTool creates a tool that allows Claude to read MCP resources
func ReadResourceTool(resourceProvider ResourceReader) Tool {
	return Tool{
		Definition: mcp.Tool{
			Name: "read_resource",
			Description: `Read MCP resources via tool interface (backward compatibility).

<important>
This tool provides backward compatibility with older MCP clients. Modern MCP clients should use the native resources/read endpoint instead, which is more efficient and follows MCP standards.
</important>

<usecase>
Use read_resource when:
- Your MCP client doesn't support native resources/read endpoint
- You need resource content as tool output (not native resource format)
- Building tool-only workflows without resource support
- Testing or debugging resource access
</usecase>

<available_resources>
1. pg://database-schema
   - Lightweight table listing (schema, table name, owner only)
   - Quick overview without column details

2. pg://system-info
   - PostgreSQL version, OS, architecture
   - Connection details (host, port, user, database)
   - Platform information for compatibility checks
</available_resources>

<alternatives>
For better results, consider these tools instead:
- get_schema_info tool: Much more detailed than pg://database-schema
  - Shows columns, types, constraints, descriptions
  - Supports filtering (schema_name, vector_tables_only)
  - Better for query writing and schema exploration

- Native resources/read: Use if your client supports it
  - More efficient (pull model)
  - Better caching
  - Standard MCP approach
</alternatives>

<usage>
- List all resources: read_resource(list=true)
- Read specific resource: read_resource(uri="pg://system-info")
</usage>`,
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"uri": map[string]interface{}{
						"type":        "string",
						"description": "The URI of the resource to read. Example: 'pg://system_info'",
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
			// Extract context from args (injected by registry.Execute)
			ctx, ok := args["__context"].(context.Context)
			if !ok {
				ctx = context.Background()
			}

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

				return mcp.NewToolSuccess(content)
			}

			// Read a specific resource
			uri, ok := args["uri"].(string)
			if !ok || uri == "" {
				return mcp.NewToolError("Error: 'uri' parameter is required. Provide a resource URI (e.g., 'pg://system_info') or use 'list': true to see all available resources.")
			}

			resourceContent, err := resourceProvider.Read(ctx, uri)
			if err != nil {
				return mcp.NewToolError("Error reading resource: " + err.Error())
			}

			// Return the resource contents
			return mcp.ToolResponse{
				Content: resourceContent.Contents,
			}, nil
		},
	}
}
