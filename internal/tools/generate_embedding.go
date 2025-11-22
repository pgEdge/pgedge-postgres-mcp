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
	"encoding/json"
	"fmt"
	"strings"

	"pgedge-postgres-mcp/internal/config"
	"pgedge-postgres-mcp/internal/embedding"
	"pgedge-postgres-mcp/internal/mcp"
)

// GenerateEmbeddingTool creates the generate_embedding tool for converting text to embedding vectors
func GenerateEmbeddingTool(cfg *config.Config) Tool {
	return Tool{
		Definition: mcp.Tool{
			Name:        "generate_embedding",
			Description: "Generate embedding vector from text using configured provider (OpenAI, Anthropic Voyage, or Ollama). Returns the embedding vector for storage or semantic search operations.",
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"text": map[string]interface{}{
						"type":        "string",
						"description": "The text to generate an embedding for (must be non-empty)",
					},
				},
				Required: []string{"text"},
			},
		},
		Handler: func(args map[string]interface{}) (mcp.ToolResponse, error) {
			// Check if embedding generation is enabled
			if !cfg.Embedding.Enabled {
				return mcp.NewToolError("Embedding generation is not enabled. Please enable it in the server configuration (PGEDGE_EMBEDDING_ENABLED=true) and configure a provider (Anthropic or Ollama).")
			}

			// Extract and validate text parameter
			text, ok := args["text"].(string)
			if !ok || text == "" {
				return mcp.NewToolError("Missing or invalid 'text' parameter")
			}

			text = strings.TrimSpace(text)
			if text == "" {
				return mcp.NewToolError("'text' parameter cannot be empty or whitespace-only")
			}

			// Create embedding provider from config
			embCfg := embedding.Config{
				Provider:     cfg.Embedding.Provider,
				Model:        cfg.Embedding.Model,
				VoyageAPIKey: cfg.Embedding.VoyageAPIKey,
				OpenAIAPIKey: cfg.Embedding.OpenAIAPIKey,
				OllamaURL:    cfg.Embedding.OllamaURL,
			}

			provider, err := embedding.NewProvider(embCfg)
			if err != nil {
				return mcp.NewToolError(fmt.Sprintf("Failed to initialize embedding provider: %v", err))
			}

			// Generate embedding
			ctx := context.Background()
			vector, err := provider.Embed(ctx, text)
			if err != nil {
				return mcp.NewToolError(fmt.Sprintf("Failed to generate embedding: %v", err))
			}

			if len(vector) == 0 {
				return mcp.NewToolError("Received empty embedding vector from provider")
			}

			// Format response
			vectorJSON, err := json.MarshalIndent(vector, "", "  ")
			if err != nil {
				return mcp.NewToolError(fmt.Sprintf("Failed to format embedding vector: %v", err))
			}

			var sb strings.Builder
			sb.WriteString("Embedding Generated Successfully\n")
			sb.WriteString(strings.Repeat("=", 50))
			sb.WriteString("\n\n")
			sb.WriteString(fmt.Sprintf("Provider: %s\n", provider.ProviderName()))
			sb.WriteString(fmt.Sprintf("Model: %s\n", provider.ModelName()))
			sb.WriteString(fmt.Sprintf("Dimensions: %d\n", provider.Dimensions()))
			sb.WriteString(fmt.Sprintf("Text Length: %d characters\n", len(text)))
			sb.WriteString(fmt.Sprintf("\nText:\n%s\n\n", text))
			sb.WriteString(fmt.Sprintf("Embedding Vector (%d dimensions):\n%s", len(vector), string(vectorJSON)))

			return mcp.NewToolSuccess(sb.String())
		},
	}
}
