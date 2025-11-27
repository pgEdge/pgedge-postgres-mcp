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
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"pgedge-postgres-mcp/internal/config"
	"pgedge-postgres-mcp/internal/embedding"
	"pgedge-postgres-mcp/internal/mcp"
)

// SearchKnowledgebaseTool creates the search_knowledgebase tool for searching documentation
func SearchKnowledgebaseTool(kbPath string, cfg *config.Config) Tool {
	return Tool{
		Definition: mcp.Tool{
			Name: "search_knowledgebase",
			Description: `Search the pre-built documentation knowledgebase for relevant information.

Use this tool when you need information about:
- PostgreSQL features, syntax, functions
- pgEdge products and capabilities
- Other documented products and technologies

The knowledgebase contains chunked, embedded documentation from multiple sources
with semantic search capabilities.

Note: In this tool, "project" and "product" are used interchangeably - they both refer
to the software product/project being documented (e.g., PostgreSQL, pgEdge, pgAdmin).

<usage>
This tool performs semantic similarity search across pre-built documentation.
Results are ranked by relevance and limited by token budget.
</usage>

<examples>
✓ "PostgreSQL window functions"
✓ "pgEdge replication setup"
✓ "How to configure authentication"
✓ "JSON functions in PostgreSQL"
</examples>`,
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "Natural language search query",
					},
					"project_name": map[string]interface{}{
						"type":        "string",
						"description": "Filter by project/product name (e.g., 'PostgreSQL', 'pgEdge', 'pgAdmin')",
					},
					"project_version": map[string]interface{}{
						"type":        "string",
						"description": "Filter by project/product version (e.g., '17', '16')",
					},
					"top_n": map[string]interface{}{
						"type":        "integer",
						"description": "Number of results to return (default: 5, max: 20)",
						"default":     5,
					},
				},
				Required: []string{"query"},
			},
		},
		Handler: func(args map[string]interface{}) (mcp.ToolResponse, error) {
			// Validate query
			query, errResp := ValidateStringParam(args, "query")
			if errResp != nil {
				return *errResp, nil
			}

			query = strings.TrimSpace(query)
			if query == "" {
				return mcp.NewToolError("query cannot be empty")
			}

			// Get optional parameters
			var projectName, projectVersion string
			topN := 5

			if pn, ok := args["project_name"].(string); ok {
				projectName = pn
			}
			if pv, ok := args["project_version"].(string); ok {
				projectVersion = pv
			}
			if tn, ok := args["top_n"].(float64); ok {
				topN = int(tn)
				if topN < 1 {
					topN = 1
				}
				if topN > 20 {
					topN = 20
				}
			}

			// Generate query embedding
			queryEmbedding, provider, err := generateKBQueryEmbedding(cfg, query)
			if err != nil {
				return mcp.NewToolError(fmt.Sprintf("Failed to generate query embedding: %v", err))
			}

			// Search knowledgebase
			results, err := searchKB(kbPath, queryEmbedding, projectName, projectVersion, topN, provider)
			if err != nil {
				return mcp.NewToolError(fmt.Sprintf("Knowledgebase search failed: %v", err))
			}

			if len(results) == 0 {
				msg := fmt.Sprintf("No results found for query: %q", query)
				if projectName != "" {
					msg += fmt.Sprintf(" (project: %s", projectName)
					if projectVersion != "" {
						msg += fmt.Sprintf(" %s", projectVersion)
					}
					msg += ")"
				}
				return mcp.NewToolSuccess(msg)
			}

			// Format results
			output := formatKBResults(results, query, projectName, projectVersion)
			return mcp.NewToolSuccess(output)
		},
	}
}

// KBSearchResult represents a search result from the knowledgebase
type KBSearchResult struct {
	Text           string
	Title          string
	Section        string
	ProjectName    string
	ProjectVersion string
	FilePath       string
	Similarity     float64
}

func generateKBQueryEmbedding(serverCfg *config.Config, queryText string) ([]float32, string, error) {
	// Use KB-specific embedding configuration (independent of generate_embeddings tool)
	kbCfg := serverCfg.Knowledgebase
	if kbCfg.EmbeddingProvider == "" {
		return nil, "", fmt.Errorf("knowledgebase embedding provider not configured")
	}

	embCfg := embedding.Config{
		Provider:     kbCfg.EmbeddingProvider,
		Model:        kbCfg.EmbeddingModel,
		VoyageAPIKey: kbCfg.EmbeddingVoyageAPIKey,
		OpenAIAPIKey: kbCfg.EmbeddingOpenAIAPIKey,
		OllamaURL:    kbCfg.EmbeddingOllamaURL,
	}

	provider, err := embedding.NewProvider(embCfg)
	if err != nil {
		return nil, "", err
	}

	ctx := context.Background()
	vector, err := provider.Embed(ctx, queryText)
	if err != nil {
		return nil, "", err
	}

	if len(vector) == 0 {
		return nil, "", fmt.Errorf("received empty embedding vector")
	}

	// Convert float64 to float32
	vector32 := make([]float32, len(vector))
	for i, v := range vector {
		vector32[i] = float32(v)
	}

	return vector32, embCfg.Provider, nil
}

func searchKB(kbPath string, queryEmbedding []float32, projectName, projectVersion string, topN int, provider string) ([]KBSearchResult, error) {
	// Open database
	db, err := sql.Open("sqlite3", kbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open knowledgebase: %w", err)
	}
	defer db.Close()

	// Build query
	query := `
        SELECT text, title, section, project_name, project_version, file_path,
               openai_embedding, voyage_embedding, ollama_embedding
        FROM chunks
        WHERE 1=1
    `
	args := []interface{}{}

	if projectName != "" {
		query += " AND project_name = ?"
		args = append(args, projectName)
	}
	if projectVersion != "" {
		query += " AND project_version = ?"
		args = append(args, projectVersion)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var results []KBSearchResult

	for rows.Next() {
		var text, title, section, pName, pVersion, filePath string
		var openaiBlob, voyageBlob, ollamaBlob []byte

		err := rows.Scan(&text, &title, &section, &pName, &pVersion, &filePath,
			&openaiBlob, &voyageBlob, &ollamaBlob)
		if err != nil {
			continue
		}

		// Select appropriate embedding based on provider
		var embBlob []byte
		switch strings.ToLower(provider) {
		case "voyage":
			embBlob = voyageBlob
		case "ollama":
			embBlob = ollamaBlob
		default: // openai
			embBlob = openaiBlob
		}

		if len(embBlob) == 0 {
			// Try other providers if selected one is empty
			if len(openaiBlob) > 0 {
				embBlob = openaiBlob
			} else if len(voyageBlob) > 0 {
				embBlob = voyageBlob
			} else if len(ollamaBlob) > 0 {
				embBlob = ollamaBlob
			} else {
				continue // No embeddings available
			}
		}

		// Deserialize embedding
		docEmbedding := deserializeEmbedding(embBlob)
		if len(docEmbedding) == 0 {
			continue
		}

		// Calculate cosine similarity
		similarity := cosineSimilarity(queryEmbedding, docEmbedding)

		results = append(results, KBSearchResult{
			Text:           text,
			Title:          title,
			Section:        section,
			ProjectName:    pName,
			ProjectVersion: pVersion,
			FilePath:       filePath,
			Similarity:     similarity,
		})
	}

	// Sort by similarity (descending)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Similarity > results[j].Similarity
	})

	// Return top N
	if len(results) > topN {
		results = results[:topN]
	}

	return results, nil
}

func deserializeEmbedding(data []byte) []float32 {
	if len(data) == 0 || len(data)%4 != 0 {
		return nil
	}

	embedding := make([]float32, len(data)/4)
	for i := range embedding {
		bits := binary.LittleEndian.Uint32(data[i*4:])
		embedding[i] = math.Float32frombits(bits)
	}
	return embedding
}

func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

func formatKBResults(results []KBSearchResult, query, projectName, projectVersion string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Knowledgebase Search Results: %q\n", query))
	if projectName != "" {
		sb.WriteString(fmt.Sprintf("Filter: %s", projectName))
		if projectVersion != "" {
			sb.WriteString(fmt.Sprintf(" %s", projectVersion))
		}
		sb.WriteString("\n")
	}
	sb.WriteString(strings.Repeat("=", 80))
	sb.WriteString("\n\n")

	sb.WriteString(fmt.Sprintf("Found %d relevant chunks:\n\n", len(results)))

	for i, result := range results {
		sb.WriteString(fmt.Sprintf("Result %d/%d\n", i+1, len(results)))
		if result.ProjectVersion != "" {
			sb.WriteString(fmt.Sprintf("Project: %s %s\n", result.ProjectName, result.ProjectVersion))
		} else {
			sb.WriteString(fmt.Sprintf("Project: %s\n", result.ProjectName))
		}
		if result.Title != "" {
			sb.WriteString(fmt.Sprintf("Title: %s\n", result.Title))
		}
		if result.Section != "" {
			sb.WriteString(fmt.Sprintf("Section: %s\n", result.Section))
		}
		sb.WriteString(fmt.Sprintf("Similarity: %.3f\n\n", result.Similarity))
		sb.WriteString(result.Text)
		sb.WriteString("\n\n")
		sb.WriteString(strings.Repeat("-", 80))
		sb.WriteString("\n\n")
	}

	sb.WriteString(strings.Repeat("=", 80))
	sb.WriteString(fmt.Sprintf("\nTotal: %d results\n", len(results)))

	return sb.String()
}
