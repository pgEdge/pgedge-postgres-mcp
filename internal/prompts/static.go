/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package prompts

import (
	"fmt"
	"strings"

	"pgedge-postgres-mcp/internal/definitions"
	"pgedge-postgres-mcp/internal/mcp"
)

// RegisterStatic registers a static user-defined prompt from a definition
func (r *Registry) RegisterStatic(def definitions.PromptDefinition) error {
	// Convert definition to MCP types
	mcpArgs := make([]mcp.PromptArgument, len(def.Arguments))
	for i, arg := range def.Arguments {
		mcpArgs[i] = mcp.PromptArgument{
			Name:        arg.Name,
			Description: arg.Description,
			Required:    arg.Required,
		}
	}

	// Create handler that interpolates templates
	handler := func(args map[string]string) mcp.PromptResult {
		// Build messages with interpolation
		messages := make([]mcp.PromptMessage, len(def.Messages))
		for i, msgDef := range def.Messages {
			content := mcp.ContentItem{
				Type: msgDef.Content.Type,
			}

			// Interpolate template for text content
			if msgDef.Content.Type == "text" {
				content.Text = interpolateTemplate(msgDef.Content.Text, args)
			} else {
				// For non-text content, just copy as-is
				content.Text = msgDef.Content.Text
			}

			messages[i] = mcp.PromptMessage{
				Role:    msgDef.Role,
				Content: content,
			}
		}

		return mcp.PromptResult{
			Description: def.Description,
			Messages:    messages,
		}
	}

	// Register prompt
	prompt := Prompt{
		Definition: mcp.Prompt{
			Name:        def.Name,
			Description: def.Description,
			Arguments:   mcpArgs,
		},
		Handler: handler,
	}

	r.Register(def.Name, prompt)
	return nil
}

// interpolateTemplate replaces {{arg_name}} placeholders with argument values
func interpolateTemplate(template string, args map[string]string) string {
	result := template
	for key, value := range args {
		placeholder := fmt.Sprintf("{{%s}}", key)
		result = strings.ReplaceAll(result, placeholder, value)
	}
	return result
}
