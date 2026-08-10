/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
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

	_ "modernc.org/sqlite"
	"pgedge-postgres-mcp/internal/config"
	"pgedge-postgres-mcp/internal/mcp"
)

// SearchKnowledgebaseTool creates the search_knowledgebase tool for searching documentation
func SearchKnowledgebaseTool(kbPath string, cfg *config.Config) Tool {
	return Tool{
		Definition: mcp.Tool{
			Name: "search_knowledgebase",
			Description: `Search the pre-built documentation knowledgebase for relevant information.

<critical>
IMPORTANT: Product names require EXACT matches. "pgEdge" will NOT match
"pgEdge RAG Server", "pgEdge Cloud", or "pgEdge Platform" - these are
separate products.

ALWAYS call with list_products=true FIRST to discover exact product names
before filtering by project_names.
</critical>

Use this tool when you need information about:
- PostgreSQL features, syntax, functions
- pgEdge products and capabilities
- Other documented products and technologies

The knowledgebase contains chunked, embedded documentation from multiple sources
with semantic search capabilities.

Note: In this tool, "project" and "product" are used interchangeably - they
both refer to the software product/project being documented.

<workflow>
1. First call: {"list_products": true} to see available products
2. Note the EXACT product names from the output
3. Search with exact names: {"query": "...", "project_names": ["Exact Name"]}
</workflow>

<troubleshooting>
If you get zero results:
- You likely have the wrong product name - call list_products=true
- Try searching without project_names filter to see what's available
- Check for typos or partial names (e.g., "pgEdge" vs "pgEdge RAG Server")
</troubleshooting>

<examples>
✓ {"list_products": true} - ALWAYS do this first!
✓ {"query": "PostgreSQL window functions"}
✓ {"query": "RAG overview", "project_names": ["pgEdge RAG Server"]}
✓ {"query": "replication", "project_names": ["pgEdge Platform"]}
✓ {"query": "JSON functions", "project_names": ["PostgreSQL"], "project_versions": ["17"]}
</examples>`,
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "Natural language search query (required unless list_products is true)",
					},
					"project_names": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "Filter by project/product name(s) (e.g., ['PostgreSQL'], ['pgEdge', 'pgAdmin'])",
					},
					"project_versions": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "Filter by project/product version(s) (e.g., ['17'], ['16', '17'])",
					},
					"top_n": map[string]any{
						"type":        "integer",
						"description": "Number of results to return (default: 5, max: 20)",
						"default":     5,
					},
					"list_products": map[string]any{
						"type":        "boolean",
						"description": "If true, returns only the list of available products and versions in the knowledgebase (ignores other parameters). Use this to discover what documentation is available before searching.",
					},
				},
				Required: []string{},
			},
		},
		Handler: func(args map[string]any) (mcp.ToolResponse, error) {
			// Check for list_products mode first
			if listProducts, ok := args["list_products"].(bool); ok && listProducts {
				products, err := listKBProducts(kbPath)
				if err != nil {
					return mcp.NewToolError(fmt.Sprintf("Failed to list products: %v", err))
				}
				return mcp.NewToolSuccess(products)
			}

			// Validate query
			query, errResp := ValidateStringParam(args, "query")
			if errResp != nil {
				return *errResp, nil
			}

			query = strings.TrimSpace(query)
			if query == "" {
				return mcp.NewToolError("query parameter is required when not using list_products")
			}

			// Get optional parameters
			var projectNames, projectVersions []string
			topN := 5

			// Extract project_names array
			if pn, ok := args["project_names"].([]any); ok {
				for _, v := range pn {
					if s, ok := v.(string); ok && s != "" {
						projectNames = append(projectNames, s)
					}
				}
			}
			// Extract project_versions array
			if pv, ok := args["project_versions"].([]any); ok {
				for _, v := range pv {
					if s, ok := v.(string); ok && s != "" {
						projectVersions = append(projectVersions, s)
					}
				}
			}
			if tn, ok := args["top_n"].(float64); ok {
				topN = int(tn)
				topN = max(topN, 1)
				topN = min(topN, 20)
			}

			// Generate query embedding
			queryEmbedding, provider, err := generateKBQueryEmbedding(cfg, query)
			if err != nil {
				return mcp.NewToolError(fmt.Sprintf("Failed to generate query embedding: %v", err))
			}

			// Search knowledgebase
			results, err := searchKB(kbPath, queryEmbedding, projectNames, projectVersions, topN, provider)
			if err != nil {
				return mcp.NewToolError(fmt.Sprintf("Knowledgebase search failed: %v", err))
			}

			if len(results) == 0 {
				msg := fmt.Sprintf("No results found for query: %q", query)
				if len(projectNames) > 0 {
					msg += fmt.Sprintf(" (projects: %s", strings.Join(projectNames, ", "))
					if len(projectVersions) > 0 {
						msg += fmt.Sprintf("; versions: %s", strings.Join(projectVersions, ", "))
					}
					msg += ")"
				}
				return mcp.NewToolSuccess(msg)
			}

			// Format results
			output := formatKBResults(results, query, projectNames, projectVersions)
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

// listKBProducts returns a formatted list of all products and versions in the knowledgebase
func listKBProducts(kbPath string) (string, error) {
	db, err := sql.Open("sqlite", kbPath)
	if err != nil {
		return "", fmt.Errorf("failed to open knowledgebase: %w", err)
	}
	defer db.Close()

	// Query distinct products and versions with chunk counts
	rows, err := db.Query(`
        SELECT project_name, project_version, COUNT(*) as chunk_count
        FROM chunks
        GROUP BY project_name, project_version
        ORDER BY project_name, project_version
    `)
	if err != nil {
		return "", fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var sb strings.Builder
	sb.WriteString("Available Products in Knowledgebase\n")
	sb.WriteString(strings.Repeat("=", 50))
	sb.WriteString("\n\n")

	currentProduct := ""
	totalChunks := 0

	for rows.Next() {
		var name, version string
		var count int
		if err := rows.Scan(&name, &version, &count); err != nil {
			continue
		}

		if name != currentProduct {
			if currentProduct != "" {
				sb.WriteString("\n")
			}
			fmt.Fprintf(&sb, "Product: %s\n", name)
			currentProduct = name
		}

		if version != "" {
			fmt.Fprintf(&sb, "  - Version %s (%d chunks)\n", version, count)
		} else {
			fmt.Fprintf(&sb, "  - (no version) (%d chunks)\n", count)
		}
		totalChunks += count
	}

	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("=", 50))
	fmt.Fprintf(&sb, "\nTotal: %d chunks across all products\n", totalChunks)

	return sb.String(), nil
}

func generateKBQueryEmbedding(serverCfg *config.Config, queryText string) ([]float32, string, error) {
	// Use KB-specific embedding configuration (independent of generate_embeddings tool)
	kbCfg := serverCfg.Knowledgebase
	if kbCfg.EmbeddingProvider == "" {
		return nil, "", fmt.Errorf("knowledgebase embedding provider not configured")
	}

	client, _, err := newEmbedClient(embedClientConfig{
		Provider:          kbCfg.EmbeddingProvider,
		Model:             kbCfg.EmbeddingModel,
		VoyageAPIKey:      kbCfg.EmbeddingVoyageAPIKey,
		VoyageBaseURL:     kbCfg.EmbeddingVoyageBaseURL,
		OpenAIAPIKey:      kbCfg.EmbeddingOpenAIAPIKey,
		OpenAIBaseURL:     kbCfg.EmbeddingOpenAIBaseURL,
		GeminiAPIKey:      kbCfg.EmbeddingGeminiAPIKey,
		GeminiBaseURL:     kbCfg.EmbeddingGeminiBaseURL,
		OllamaURL:         kbCfg.EmbeddingOllamaURL,
		PerAttemptTimeout: kbCfg.EmbeddingPerAttemptTimeout,
	})
	if err != nil {
		return nil, "", err
	}

	ctx := context.Background()
	vector, err := client.Embed(ctx, queryText)
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

	return vector32, kbCfg.EmbeddingProvider, nil
}

func searchKB(kbPath string, queryEmbedding []float32, projectNames, projectVersions []string, topN int, provider string) ([]KBSearchResult, error) {
	// Open database
	db, err := sql.Open("sqlite", kbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open knowledgebase: %w", err)
	}
	defer db.Close()

	// Knowledgebases built before Gemini support lack the gemini_embedding
	// column, and this server opens the database read-only so it cannot add
	// the column itself. Select a NULL placeholder in that case, so that
	// older knowledgebases keep working for the other providers.
	hasGemini, err := kbHasGeminiColumn(db)
	if err != nil {
		return nil, err
	}
	geminiColumn := "NULL"
	if hasGemini {
		geminiColumn = "gemini_embedding"
	}

	// Build query
	query := fmt.Sprintf(`
        SELECT text, title, section, project_name, project_version, file_path,
               openai_embedding, voyage_embedding, ollama_embedding, %s
        FROM chunks
        WHERE 1=1
    `, geminiColumn)
	args := []any{}

	// Add project_names filter with IN clause
	if len(projectNames) > 0 {
		placeholders := make([]string, len(projectNames))
		for i, name := range projectNames {
			placeholders[i] = "?"
			args = append(args, name)
		}
		query += fmt.Sprintf(" AND project_name IN (%s)", strings.Join(placeholders, ", "))
	}

	// Add project_versions filter with IN clause
	if len(projectVersions) > 0 {
		placeholders := make([]string, len(projectVersions))
		for i, version := range projectVersions {
			placeholders[i] = "?"
			args = append(args, version)
		}
		query += fmt.Sprintf(" AND project_version IN (%s)", strings.Join(placeholders, ", "))
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var results []KBSearchResult

	// Track whether any row was examined at all, and whether the configured
	// provider's own column ever held a vector, so that a knowledgebase
	// built with a different provider can be reported clearly rather than
	// looking like an ordinary empty result set.
	scanned := 0
	providerHasVectors := false
	isGemini := strings.EqualFold(provider, "gemini")

	for rows.Next() {
		var text, title, section, pName, pVersion, filePath string
		var openaiBlob, voyageBlob, ollamaBlob, geminiBlob []byte

		err := rows.Scan(&text, &title, &section, &pName, &pVersion, &filePath,
			&openaiBlob, &voyageBlob, &ollamaBlob, &geminiBlob)
		if err != nil {
			continue
		}
		scanned++

		// Select appropriate embedding based on provider
		var embBlob []byte
		switch strings.ToLower(provider) {
		case "voyage":
			embBlob = voyageBlob
		case "ollama":
			embBlob = ollamaBlob
		case "gemini":
			embBlob = geminiBlob
		default: // openai
			embBlob = openaiBlob
		}

		var docEmbedding []float32
		if len(embBlob) > 0 {
			providerHasVectors = true
			docEmbedding = deserializeEmbedding(embBlob)
		} else if !isGemini {
			// Fall back to another provider's vectors, keeping the
			// existing preference order, but only where the vector has
			// the same dimensions as the query. Comparing vectors of
			// differing dimensions scores zero anyway, and accepting a
			// mismatched vector would silently rank nonsense.
			//
			// Gemini takes no part in this, in either direction: several
			// Gemini and OpenAI models share a width, so a dimension check
			// alone would let the two be scored against each other and
			// return plausible looking but meaningless results.
			for _, candidate := range [][]byte{openaiBlob, voyageBlob, ollamaBlob} {
				if len(candidate) == 0 {
					continue
				}
				if vector := deserializeEmbedding(candidate); len(vector) == len(queryEmbedding) {
					docEmbedding = vector
					break
				}
			}
		}

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

	// Rows matched the filters, but none of them carried a usable vector for
	// the configured provider, so the knowledgebase was built with a
	// different provider (or predates support for this one). Report that
	// explicitly rather than returning a misleading empty result set.
	if len(results) == 0 && scanned > 0 && !providerHasVectors {
		return nil, fmt.Errorf(
			"the knowledgebase contains no %s embeddings; rebuild it with %s, "+
				"or set knowledgebase.embedding_provider to the provider used to build it",
			provider, provider)
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

// kbHasGeminiColumn reports whether the chunks table carries a
// gemini_embedding column. The knowledgebase builder adds the column to
// existing databases on first open, but this server only ever opens the
// database for reading, so a knowledgebase built before Gemini support was
// added will not have it.
func kbHasGeminiColumn(db *sql.DB) (bool, error) {
	rows, err := db.Query("PRAGMA table_info(chunks)")
	if err != nil {
		return false, fmt.Errorf("failed to inspect knowledgebase schema: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return false, fmt.Errorf("failed to inspect knowledgebase schema: %w", err)
	}

	// PRAGMA table_info returns cid, name, type, notnull, dflt_value and pk;
	// only the name is of interest, and dflt_value may be NULL, so scan
	// everything into nullable holders.
	nameIndex := -1
	for i, column := range columns {
		if strings.EqualFold(column, "name") {
			nameIndex = i
			break
		}
	}
	if nameIndex < 0 {
		return false, fmt.Errorf("failed to inspect knowledgebase schema: unexpected PRAGMA output")
	}

	values := make([]sql.NullString, len(columns))
	dest := make([]any, len(columns))
	for i := range values {
		dest[i] = &values[i]
	}

	for rows.Next() {
		if err := rows.Scan(dest...); err != nil {
			return false, fmt.Errorf("failed to inspect knowledgebase schema: %w", err)
		}
		if values[nameIndex].Valid && values[nameIndex].String == "gemini_embedding" {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("failed to inspect knowledgebase schema: %w", err)
	}

	return false, nil
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

func formatKBResults(results []KBSearchResult, query string, projectNames, projectVersions []string) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "Knowledgebase Search Results: %q\n", query)
	if len(projectNames) > 0 {
		fmt.Fprintf(&sb, "Filter - Projects: %s", strings.Join(projectNames, ", "))
		if len(projectVersions) > 0 {
			fmt.Fprintf(&sb, "; Versions: %s", strings.Join(projectVersions, ", "))
		}
		sb.WriteString("\n")
	} else if len(projectVersions) > 0 {
		fmt.Fprintf(&sb, "Filter - Versions: %s\n", strings.Join(projectVersions, ", "))
	}
	sb.WriteString(strings.Repeat("=", 80))
	sb.WriteString("\n\n")

	fmt.Fprintf(&sb, "Found %d relevant chunks:\n\n", len(results))

	for i, result := range results {
		fmt.Fprintf(&sb, "Result %d/%d\n", i+1, len(results))
		if result.ProjectVersion != "" {
			fmt.Fprintf(&sb, "Project: %s %s\n", result.ProjectName, result.ProjectVersion)
		} else {
			fmt.Fprintf(&sb, "Project: %s\n", result.ProjectName)
		}
		if result.Title != "" {
			fmt.Fprintf(&sb, "Title: %s\n", result.Title)
		}
		if result.Section != "" {
			fmt.Fprintf(&sb, "Section: %s\n", result.Section)
		}
		fmt.Fprintf(&sb, "Similarity: %.3f\n\n", result.Similarity)
		sb.WriteString(result.Text)
		sb.WriteString("\n\n")
		sb.WriteString(strings.Repeat("-", 80))
		sb.WriteString("\n\n")
	}

	sb.WriteString(strings.Repeat("=", 80))
	fmt.Fprintf(&sb, "\nTotal: %d results\n", len(results))

	return sb.String()
}
