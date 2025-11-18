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
	"context"
	"fmt"
	"strings"

	"pgedge-postgres-mcp/internal/config"
	"pgedge-postgres-mcp/internal/database"
	"pgedge-postgres-mcp/internal/embedding"
	"pgedge-postgres-mcp/internal/mcp"
	"pgedge-postgres-mcp/internal/search"
)

// SimilaritySearchTool creates the similarity_search tool for hybrid semantic + lexical search
func SimilaritySearchTool(dbClient *database.Client, cfg *config.Config) Tool {
	return Tool{
		Definition: mcp.Tool{
			Name:        "similarity_search",
			Description: "Advanced hybrid search combining vector similarity with BM25 lexical matching and MMR diversity filtering. Automatically discovers vector columns, generates query embeddings, chunks results intelligently, and returns the most relevant excerpts within token limits. Ideal for searching through large documents like Wikipedia articles.",
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"table_name": map[string]interface{}{
						"type":        "string",
						"description": "Name of the table to search (can include schema: 'schema.table')",
					},
					"query_text": map[string]interface{}{
						"type":        "string",
						"description": "Natural language search query",
					},
					"top_n": map[string]interface{}{
						"type":        "integer",
						"description": "Number of rows to retrieve from vector search (default: 10)",
					},
					"chunk_size_tokens": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum tokens per chunk (default: 100)",
					},
					"lambda": map[string]interface{}{
						"type":        "number",
						"description": "MMR diversity parameter: 0.0=max diversity, 1.0=max relevance (default: 0.6)",
					},
					"max_output_tokens": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum total tokens to return (default: 2500)",
					},
					"distance_metric": map[string]interface{}{
						"type":        "string",
						"description": "Distance metric: 'cosine', 'l2', or 'inner_product' (default: 'cosine')",
					},
				},
				Required: []string{"table_name", "query_text"},
			},
		},
		Handler: func(args map[string]interface{}) (mcp.ToolResponse, error) {
			// Step 1: Validate and extract parameters
			tableName, errResp := ValidateStringParam(args, "table_name")
			if errResp != nil {
				return *errResp, nil
			}

			queryText, errResp := ValidateStringParam(args, "query_text")
			if errResp != nil {
				return *errResp, nil
			}

			queryText = strings.TrimSpace(queryText)
			if queryText == "" {
				return mcp.NewToolError("query_text cannot be empty")
			}

			// Get search configuration with defaults
			searchCfg := search.DefaultSearchConfig()
			if topN, ok := args["top_n"].(float64); ok {
				searchCfg.TopN = int(topN)
			}
			if chunkSize, ok := args["chunk_size_tokens"].(float64); ok {
				searchCfg.ChunkSizeTokens = int(chunkSize)
			}
			if lambda, ok := args["lambda"].(float64); ok {
				searchCfg.Lambda = lambda
			}
			if maxTokens, ok := args["max_output_tokens"].(float64); ok {
				searchCfg.MaxOutputTokens = int(maxTokens)
			}
			if metric, ok := args["distance_metric"].(string); ok {
				searchCfg.DistanceMetric = metric
			}

			// Step 2: Get table metadata and discover columns
			metadataMap := dbClient.GetMetadata()
			tableInfo, err := findTableInMetadataMap(metadataMap, tableName)
			if err != nil {
				return mcp.NewToolError(fmt.Sprintf("Table not found: %v", err))
			}

			// Discover vector columns
			vectorCols := discoverVectorColumns(tableInfo)
			if len(vectorCols) == 0 {
				return mcp.NewToolError(fmt.Sprintf("No vector columns found in table '%s'. This tool requires at least one pgvector column.", tableName))
			}

			// Discover text columns corresponding to vector columns
			textCols := discoverTextColumns(tableInfo, vectorCols)
			if len(textCols) == 0 {
				return mcp.NewToolError(fmt.Sprintf("No text columns found that correspond to vector columns in table '%s'", tableName))
			}

			// Step 3: Sample data for smart column type detection
			sampleData, err := sampleTableData(dbClient, tableName, textCols, 3)
			if err != nil {
				// Non-fatal: proceed with default weights
				sampleData = make(map[string]string)
			}

			// Detect column types and weights
			columnWeights := search.DetectColumnTypes(tableInfo, sampleData)

			// Step 4: Generate query embedding (use the global cfg variable, not the search config)
			queryEmbedding, err := generateQueryEmbeddingWithConfig(cfg, queryText)
			if err != nil {
				return mcp.NewToolError(fmt.Sprintf("Failed to generate query embedding: %v", err))
			}

			// Step 5: Perform weighted vector search
			results, err := performWeightedVectorSearch(
				dbClient,
				tableName,
				vectorCols,
				textCols,
				queryEmbedding,
				columnWeights,
				searchCfg.TopN,
				searchCfg.DistanceMetric,
			)
			if err != nil {
				return mcp.NewToolError(fmt.Sprintf("Vector search failed: %v", err))
			}

			if len(results) == 0 {
				return mcp.NewToolSuccess("No results found for the query.")
			}

			// Step 6: Chunk all results
			allChunks := chunkResults(results, textCols, tableName, searchCfg.ChunkSizeTokens, searchCfg.OverlapTokens)

			// Step 7: Re-rank chunks using BM25
			rankedChunks := search.RankChunks(allChunks, queryText)

			// Step 8: Apply MMR diversity filtering
			mmr := search.NewMMRSelector(searchCfg.Lambda)
			maxChunksBeforeBudget := (searchCfg.MaxOutputTokens / searchCfg.ChunkSizeTokens) * 2 // Allow 2x before budget cut
			if maxChunksBeforeBudget < 10 {
				maxChunksBeforeBudget = 10
			}
			diverseChunks := mmr.SelectChunks(rankedChunks, maxChunksBeforeBudget)

			// Step 9: Apply token budget
			finalChunks := search.SelectChunksWithinBudget(diverseChunks, searchCfg.MaxOutputTokens)

			if len(finalChunks) == 0 {
				return mcp.NewToolSuccess("Search completed but no chunks fit within token budget.")
			}

			// Step 10: Format output
			output := formatSearchResults(finalChunks, queryText, columnWeights, searchCfg)

			return mcp.NewToolSuccess(output)
		},
	}
}

// Helper functions

func findTableInMetadataMap(metadata map[string]database.TableInfo, tableName string) (database.TableInfo, error) {
	// Handle schema.table format
	parts := strings.Split(tableName, ".")
	var schemaName, tblName string

	if len(parts) == 2 {
		schemaName = parts[0]
		tblName = parts[1]
	} else {
		schemaName = "public"
		tblName = tableName
	}

	// Build full table name key
	fullName := schemaName + "." + tblName

	// Try to find the table
	if table, ok := metadata[fullName]; ok {
		return table, nil
	}

	return database.TableInfo{}, fmt.Errorf("table '%s' not found in schema '%s'", tblName, schemaName)
}

func discoverVectorColumns(tableInfo database.TableInfo) []database.ColumnInfo {
	var vectorCols []database.ColumnInfo
	for _, col := range tableInfo.Columns {
		if col.IsVectorColumn {
			vectorCols = append(vectorCols, col)
		}
	}
	return vectorCols
}

func discoverTextColumns(tableInfo database.TableInfo, vectorCols []database.ColumnInfo) []string {
	// Try to match vector columns to text columns by name
	var textCols []string
	matched := make(map[string]bool)

	for _, vecCol := range vectorCols {
		// Try to infer text column name
		textColName := inferTextColumnName(vecCol.ColumnName)

		// Check if this column exists in the table
		for _, col := range tableInfo.Columns {
			if col.ColumnName == textColName && isTextDataType(col.DataType) {
				textCols = append(textCols, col.ColumnName)
				matched[col.ColumnName] = true
				break
			}
		}
	}

	// If no matches found, return all text columns
	if len(textCols) == 0 {
		for _, col := range tableInfo.Columns {
			if !col.IsVectorColumn && isTextDataType(col.DataType) {
				textCols = append(textCols, col.ColumnName)
			}
		}
	}

	return textCols
}

func inferTextColumnName(vectorColName string) string {
	name := vectorColName

	suffixes := []string{
		"_embedding", "_embeddings", "_vector", "_vectors", "_emb",
		"embedding", "vector",
	}

	for _, suffix := range suffixes {
		if strings.HasSuffix(strings.ToLower(name), suffix) {
			name = name[:len(name)-len(suffix)]
			break
		}
	}

	return strings.TrimSuffix(name, "_")
}

func isTextDataType(dataType string) bool {
	textTypes := []string{"text", "character varying", "varchar", "character", "char"}
	lowerType := strings.ToLower(dataType)
	for _, textType := range textTypes {
		if strings.Contains(lowerType, textType) {
			return true
		}
	}
	return false
}

func sampleTableData(dbClient *database.Client, tableName string, textCols []string, sampleSize int) (map[string]string, error) {
	if len(textCols) == 0 {
		return make(map[string]string), nil
	}

	connStr := dbClient.GetDefaultConnection()
	pool := dbClient.GetPoolFor(connStr)
	if pool == nil {
		return nil, fmt.Errorf("no connection pool available")
	}

	ctx := context.Background()

	// Build query to sample data
	colList := strings.Join(textCols, ", ")
	query := fmt.Sprintf("SELECT %s FROM %s LIMIT %d", colList, tableName, sampleSize)

	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sampleData := make(map[string]string)
	count := 0

	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			continue
		}

		for i, col := range textCols {
			if i < len(values) {
				if str, ok := values[i].(string); ok {
					// Accumulate sample text
					existing := sampleData[col]
					if existing == "" {
						sampleData[col] = str
					} else {
						sampleData[col] = existing + " " + str
					}
				}
			}
		}
		count++
	}

	// Calculate average lengths
	if count > 0 {
		for col := range sampleData {
			sampleData[col] = sampleData[col][:minInt(len(sampleData[col]), 1000)] // Limit sample size
		}
	}

	return sampleData, nil
}

func generateQueryEmbeddingWithConfig(serverCfg *config.Config, queryText string) ([]float64, error) {
	if !serverCfg.Embedding.Enabled {
		return nil, fmt.Errorf("embedding generation is not enabled in server configuration")
	}

	embCfg := embedding.Config{
		Provider:        serverCfg.Embedding.Provider,
		Model:           serverCfg.Embedding.Model,
		AnthropicAPIKey: serverCfg.Embedding.AnthropicAPIKey,
		OpenAIAPIKey:    serverCfg.Embedding.OpenAIAPIKey,
		OllamaURL:       serverCfg.Embedding.OllamaURL,
	}

	provider, err := embedding.NewProvider(embCfg)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	vector, err := provider.Embed(ctx, queryText)
	if err != nil {
		return nil, err
	}

	if len(vector) == 0 {
		return nil, fmt.Errorf("received empty embedding vector")
	}

	return vector, nil
}

func performWeightedVectorSearch(
	dbClient *database.Client,
	tableName string,
	vectorCols []database.ColumnInfo,
	textCols []string,
	queryEmbedding []float64,
	columnWeights []search.ColumnWeight,
	topN int,
	distanceMetric string,
) ([]search.VectorSearchResult, error) {

	connStr := dbClient.GetDefaultConnection()
	pool := dbClient.GetPoolFor(connStr)
	if pool == nil {
		return nil, fmt.Errorf("no connection pool available")
	}

	ctx := context.Background()

	// Build SQL query with weighted distance
	distOp := getDistanceOperator(distanceMetric)

	// Build column list
	allCols := append([]string{"*"}, textCols...)
	colList := strings.Join(allCols, ", ")

	// Build weighted distance calculation
	var weightedParts []string
	weightMap := make(map[string]float64)

	for _, weight := range columnWeights {
		weightedParts = append(weightedParts, fmt.Sprintf("(%s %s $1::vector) * %f", weight.VectorName, distOp, weight.Weight))
		weightMap[weight.VectorName] = weight.Weight
	}

	// If no weights, use equal weighting
	if len(weightedParts) == 0 {
		for _, vecCol := range vectorCols {
			weight := 1.0 / float64(len(vectorCols))
			weightedParts = append(weightedParts, fmt.Sprintf("(%s %s $1::vector) * %f", vecCol.ColumnName, distOp, weight))
			weightMap[vecCol.ColumnName] = weight
		}
	}

	weightedDistance := strings.Join(weightedParts, " + ")

	query := fmt.Sprintf(`
        SELECT %s, (%s) as weighted_distance
        FROM %s
        ORDER BY weighted_distance
        LIMIT $2
    `, colList, weightedDistance, tableName)

	// Convert embedding to PostgreSQL array format
	embeddingStr := formatEmbeddingForPostgres(queryEmbedding)

	rows, err := pool.Query(ctx, query, embeddingStr, topN)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []search.VectorSearchResult

	fieldDescs := rows.FieldDescriptions()
	columnNames := make([]string, len(fieldDescs))
	for i, fd := range fieldDescs {
		columnNames[i] = string(fd.Name)
	}

	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			continue
		}

		rowData := make(map[string]interface{})
		var distance float64

		for i, colName := range columnNames {
			if i < len(values) {
				if colName == "weighted_distance" {
					if dist, ok := values[i].(float64); ok {
						distance = dist
					}
				} else {
					rowData[colName] = values[i]
				}
			}
		}

		result := search.VectorSearchResult{
			RowData:       rowData,
			Distance:      distance,
			VectorWeights: weightMap,
		}
		results = append(results, result)
	}

	return results, nil
}

func getDistanceOperator(metric string) string {
	switch strings.ToLower(metric) {
	case "l2", "euclidean":
		return "<->"
	case "inner_product", "inner":
		return "<#>"
	default: // cosine
		return "<=>"
	}
}

func formatEmbeddingForPostgres(embedding []float64) string {
	parts := make([]string, len(embedding))
	for i, val := range embedding {
		parts[i] = fmt.Sprintf("%f", val)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func chunkResults(
	results []search.VectorSearchResult,
	textCols []string,
	tableName string,
	chunkSizeTokens int,
	overlapTokens int,
) []search.ScoredChunk {
	var allChunks []search.ScoredChunk

	for rank, result := range results {
		// Use first column value as row ID if available
		var rowID interface{} = rank
		if id, ok := result.RowData["id"]; ok {
			rowID = id
		}

		chunks := search.ChunkRow(
			result.RowData,
			textCols,
			rowID,
			tableName,
			rank,
			chunkSizeTokens,
			overlapTokens,
		)

		allChunks = append(allChunks, chunks...)
	}

	return allChunks
}

func formatSearchResults(
	chunks []search.ScoredChunk,
	queryText string,
	columnWeights []search.ColumnWeight,
	cfg search.SearchConfig,
) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Similarity Search Results: %q\n", queryText))
	sb.WriteString(strings.Repeat("=", 80))
	sb.WriteString("\n\n")

	// Show configuration
	sb.WriteString("Configuration:\n")
	sb.WriteString(fmt.Sprintf("  - Vector Search: Top %d rows\n", cfg.TopN))
	sb.WriteString(fmt.Sprintf("  - Chunking: %d tokens per chunk, %d token overlap\n", cfg.ChunkSizeTokens, cfg.OverlapTokens))
	sb.WriteString(fmt.Sprintf("  - Diversity: λ=%.2f (%.0f%% relevance, %.0f%% diversity)\n", cfg.Lambda, cfg.Lambda*100, (1-cfg.Lambda)*100))
	sb.WriteString(fmt.Sprintf("  - Distance Metric: %s\n", cfg.DistanceMetric))

	// Show column weights
	if len(columnWeights) > 0 {
		sb.WriteString("  - Column Weights:\n")
		for _, w := range columnWeights {
			colType := "content"
			if w.IsTitle {
				colType = "title"
			}
			sb.WriteString(fmt.Sprintf("      %s (%.1f%%) [%s]\n", w.ColumnName, w.Weight*100, colType))
		}
	}
	sb.WriteString("\n")

	// Show results
	totalTokens := 0
	for i, chunk := range chunks {
		chunkTokens := search.EstimateTokens(chunk.Text)
		totalTokens += chunkTokens

		sb.WriteString(fmt.Sprintf("Result %d/%d\n", i+1, len(chunks)))
		sb.WriteString(fmt.Sprintf("Source: %s.%s (vector search rank: #%d, chunk: %d)\n",
			chunk.SourceTable, chunk.SourceColumn, chunk.OriginalRank+1, chunk.ChunkIndex+1))
		sb.WriteString(fmt.Sprintf("Relevance Score: %.3f\n", chunk.Score))
		sb.WriteString(fmt.Sprintf("Tokens: ~%d\n\n", chunkTokens))
		sb.WriteString(chunk.Text)
		sb.WriteString("\n\n")
		sb.WriteString(strings.Repeat("-", 80))
		sb.WriteString("\n\n")
	}

	sb.WriteString(strings.Repeat("=", 80))
	sb.WriteString(fmt.Sprintf("\nTotal: %d chunks, ~%d tokens\n", len(chunks), totalTokens))

	return sb.String()
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
