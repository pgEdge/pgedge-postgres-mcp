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
			Name: "similarity_search",
			Description: `Semantic search for NATURAL LANGUAGE and CONCEPT-BASED queries.

<usecase>
Use similarity_search when you need:
- Finding content by meaning, not exact keywords
- "Documents about X" or "Similar to Y" queries
- Topic/theme discovery in unstructured text
- When user language may differ from stored text
- Searching Wikipedia articles, documentation, support tickets, reviews
- Content that benefits from semantic understanding
</usecase>

<technical_details>
- Hybrid: Vector similarity (pgvector) + BM25 lexical ranking
- MMR diversity filtering (λ parameter: 0.0=max diversity, 1.0=max relevance)
- Automatic intelligent chunking with token budgets
- Smart column weighting (title columns vs content columns)
- Configurable distance metrics (cosine, L2, inner product)
</technical_details>

<when_not_to_use>
DO NOT use for:
- Structured filters (status, dates, IDs) → use query_database
- Aggregations (COUNT, SUM, AVG) or joins → use query_database
- Exact keyword matching → use query_database with LIKE/ILIKE
- When you need all matching records → use query_database
</when_not_to_use>

<important>
ALWAYS call get_schema_info with vector_tables_only=true FIRST if you don't know the table name. This tool requires:
- A valid table name with pgvector columns
- Embedding generation must be enabled in server config
- Table must have at least one vector column
</important>

<examples>
✓ "Find tickets about connection timeouts"
✓ "Documents similar to article ID 123"
✓ "Wikipedia articles related to quantum computing"
✓ "Support requests mentioning performance issues"
✗ "Count tickets by status" → use query_database
✗ "Users created last week" → use query_database
✗ "Show order with ID 12345" → use query_database
</examples>

<token_management>
Default budget: 1000 tokens (~10 chunks of ~100 tokens each)
Increase max_output_tokens if more context needed, but beware rate limits.
Consider using output_format parameter to control verbosity.
</token_management>

<rate_limit_awareness>
To avoid rate limits (30,000 input tokens/minute):
- Use output_format="summary" for initial exploration (50 tokens vs 1000)
- Use output_format="ids_only" for quick scans (10 tokens vs 1000)
- Reduce top_n parameter if you don't need many results
- Keep max_output_tokens at default 1000 unless you truly need more
- Call get_schema_info(vector_tables_only=true) to find tables efficiently
- If rate limited, switch to lighter queries or wait 60 seconds
</rate_limit_awareness>`,
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
						"description": "Maximum total tokens to return (default: 1000)",
					},
					"distance_metric": map[string]interface{}{
						"type":        "string",
						"description": "Distance metric: 'cosine', 'l2', or 'inner_product' (default: 'cosine')",
					},
					"output_format": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"full", "summary", "ids_only"},
						"description": "Output format: 'full'=complete chunks (default), 'summary'=titles+snippets only (~50 tokens total, 10x more results), 'ids_only'=just row IDs for progressive disclosure",
						"default":     "full",
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

			// Get output format (default: "full")
			outputFormat := "full"
			if format, ok := args["output_format"].(string); ok {
				outputFormat = format
			}

			// Step 2: Get table metadata and discover columns
			metadataMap := dbClient.GetMetadata()
			tableInfo, err := findTableInMetadataMap(metadataMap, tableName)
			if err != nil {
				connStr := dbClient.GetDefaultConnection()
				sanitizedConn := database.SanitizeConnStr(connStr)

				var errMsg strings.Builder
				errMsg.WriteString(fmt.Sprintf("Table '%s' not found.\n\n", tableName))
				errMsg.WriteString("<current_connection>\n")
				errMsg.WriteString(fmt.Sprintf("Connected to: %s\n", sanitizedConn))
				errMsg.WriteString("</current_connection>\n\n")
				errMsg.WriteString("<diagnosis>\n")
				errMsg.WriteString("Possible reasons:\n")
				errMsg.WriteString("1. Table name is misspelled or doesn't exist\n")
				errMsg.WriteString("2. Table exists in a different schema (try 'schema.table' format)\n")
				errMsg.WriteString("3. Connected to wrong database\n")
				errMsg.WriteString("4. Metadata not loaded yet\n")
				errMsg.WriteString("</diagnosis>\n\n")
				errMsg.WriteString("<next_steps>\n")
				errMsg.WriteString("1. List all tables in the database:\n")
				errMsg.WriteString("   → get_schema_info()\n\n")
				errMsg.WriteString("2. List only tables with vector columns:\n")
				errMsg.WriteString("   → get_schema_info(vector_tables_only=true)\n\n")
				errMsg.WriteString("3. Check current database connection:\n")
				errMsg.WriteString("   → read_resource(uri=\"pg://system-info\")\n\n")
				errMsg.WriteString("4. If table is in a different schema, use qualified name:\n")
				errMsg.WriteString(fmt.Sprintf("   → similarity_search(table_name=\"schema_name.%s\", query_text=\"...\")\n", tableName))
				errMsg.WriteString("</next_steps>\n")

				return mcp.NewToolError(errMsg.String())
			}

			// Discover vector columns
			vectorCols := discoverVectorColumns(tableInfo)
			if len(vectorCols) == 0 {
				var errMsg strings.Builder
				errMsg.WriteString(fmt.Sprintf("No vector columns found in table '%s'.\n\n", tableName))
				errMsg.WriteString("<diagnosis>\n")
				errMsg.WriteString("This tool requires tables with pgvector extension columns (vector data type).\n")
				errMsg.WriteString("Possible reasons:\n")
				errMsg.WriteString("1. Table exists but has no vector columns\n")
				errMsg.WriteString("2. pgvector extension not installed\n")
				errMsg.WriteString("3. Wrong table selected (embeddings stored elsewhere)\n")
				errMsg.WriteString("</diagnosis>\n\n")
				errMsg.WriteString("<next_steps>\n")
				errMsg.WriteString("1. Find tables WITH vector columns:\n")
				errMsg.WriteString("   → get_schema_info(vector_tables_only=true)\n\n")
				errMsg.WriteString("2. Check if pgvector extension is installed:\n")
				errMsg.WriteString("   → query_database(query=\"SELECT * FROM pg_extension WHERE extname = 'vector'\")\n\n")
				errMsg.WriteString("3. If no vector tables exist, install pgvector:\n")
				errMsg.WriteString("   → Contact administrator to install: CREATE EXTENSION vector;\n\n")
				errMsg.WriteString("4. For non-semantic queries, use query_database instead:\n")
				errMsg.WriteString(fmt.Sprintf("   → query_database(query=\"SELECT * FROM %s WHERE ...\")\n", tableName))
				errMsg.WriteString("</next_steps>\n")

				return mcp.NewToolError(errMsg.String())
			}

			// Discover text columns corresponding to vector columns
			textCols := discoverTextColumns(tableInfo, vectorCols)
			if len(textCols) == 0 {
				var errMsg strings.Builder
				errMsg.WriteString(fmt.Sprintf("No text columns found corresponding to vector columns in table '%s'.\n\n", tableName))
				errMsg.WriteString("<diagnosis>\n")
				errMsg.WriteString("This tool needs text columns to search. Vector columns store embeddings, but the original text must be in companion text columns.\n")
				errMsg.WriteString("Expected naming patterns:\n")
				errMsg.WriteString("- Vector column 'content_embedding' → text column 'content'\n")
				errMsg.WriteString("- Vector column 'title_vector' → text column 'title'\n")
				errMsg.WriteString("</diagnosis>\n\n")
				errMsg.WriteString("<next_steps>\n")
				errMsg.WriteString("1. Check table structure:\n")
				errMsg.WriteString(fmt.Sprintf("   → get_schema_info(schema_name=%q)\n\n", strings.Split(tableName, ".")[0]))
				errMsg.WriteString("2. List columns in this table:\n")
				errMsg.WriteString(fmt.Sprintf("   → query_database(query=\"SELECT column_name, data_type FROM information_schema.columns WHERE table_name = '%s'\")\n\n", tableName))
				errMsg.WriteString("3. This table might not be suitable for semantic search\n")
				errMsg.WriteString("   → Try a different table with get_schema_info(vector_tables_only=true)\n")
				errMsg.WriteString("</next_steps>\n")

				return mcp.NewToolError(errMsg.String())
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
				var errMsg strings.Builder
				errMsg.WriteString(fmt.Sprintf("Failed to generate query embedding: %v\n\n", err))
				errMsg.WriteString("<diagnosis>\n")
				errMsg.WriteString("The server's embedding service encountered an error. Possible causes:\n")
				errMsg.WriteString("1. Embedding generation is disabled in server configuration\n")
				errMsg.WriteString("2. API key is missing or invalid (OpenAI, Voyage AI)\n")
				errMsg.WriteString("3. Embedding service (Ollama) is not reachable\n")
				errMsg.WriteString("4. Network connectivity issues\n")
				errMsg.WriteString("5. Query text is empty or malformed\n")
				errMsg.WriteString("</diagnosis>\n\n")
				errMsg.WriteString("<next_steps>\n")
				errMsg.WriteString("1. Contact server administrator to check embedding configuration\n\n")
				errMsg.WriteString("2. Verify API keys and service availability\n\n")
				errMsg.WriteString("3. For non-semantic queries, use query_database instead:\n")
				errMsg.WriteString(fmt.Sprintf("   → query_database(query=\"SELECT * FROM %s WHERE text_column ILIKE '%%%s%%'\")\n", tableName, queryText))
				errMsg.WriteString("</next_steps>\n")

				return mcp.NewToolError(errMsg.String())
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
				var errMsg strings.Builder
				errMsg.WriteString(fmt.Sprintf("Vector search failed: %v\n\n", err))
				errMsg.WriteString("<diagnosis>\n")
				errMsg.WriteString("The database query for vector similarity failed. Possible causes:\n")
				errMsg.WriteString("1. Vector dimension mismatch (embedding size != column size)\n")
				errMsg.WriteString("2. Incompatible distance metric for the vector index\n")
				errMsg.WriteString("3. Database permissions issue\n")
				errMsg.WriteString("4. pgvector extension not properly installed\n")
				errMsg.WriteString("5. Table or vector columns have been modified\n")
				errMsg.WriteString("</diagnosis>\n\n")
				errMsg.WriteString("<next_steps>\n")
				errMsg.WriteString("1. Check vector column dimensions:\n")
				errMsg.WriteString(fmt.Sprintf("   → query_database(query=\"SELECT column_name, atttypmod FROM pg_attribute WHERE attrelid = '%s'::regclass AND atttypid = 'vector'::regtype\")\n\n", tableName))
				errMsg.WriteString("2. Verify pgvector extension:\n")
				errMsg.WriteString("   → query_database(query=\"SELECT * FROM pg_extension WHERE extname = 'vector'\")\n\n")
				errMsg.WriteString("3. Try a different table:\n")
				errMsg.WriteString("   → get_schema_info(vector_tables_only=true)\n\n")
				errMsg.WriteString("4. Contact administrator if error persists\n")
				errMsg.WriteString("</next_steps>\n")

				return mcp.NewToolError(errMsg.String())
			}

			if len(results) == 0 {
				var msg strings.Builder
				msg.WriteString(fmt.Sprintf("No results found for query: %q\n\n", queryText))
				msg.WriteString("<diagnosis>\n")
				msg.WriteString("The vector search completed but found no semantically similar content.\n")
				msg.WriteString("Possible reasons:\n")
				msg.WriteString("1. Table is empty or has very few rows\n")
				msg.WriteString("2. Query is too specific or uses unusual terminology\n")
				msg.WriteString("3. Vector embeddings don't match query semantics\n")
				msg.WriteString("4. Distance threshold is too strict\n")
				msg.WriteString("</diagnosis>\n\n")
				msg.WriteString("<next_steps>\n")
				msg.WriteString("1. Check if table has data:\n")
				msg.WriteString(fmt.Sprintf("   → query_database(query=\"SELECT COUNT(*) FROM %s\")\n\n", tableName))
				msg.WriteString("2. Try a broader or simpler query\n\n")
				msg.WriteString("3. Sample the table to see what content exists:\n")
				msg.WriteString(fmt.Sprintf("   → query_database(query=\"SELECT * FROM %s\", limit=5)\n\n", tableName))
				msg.WriteString("4. Increase top_n parameter to cast a wider net:\n")
				msg.WriteString(fmt.Sprintf("   → similarity_search(table_name=%q, query_text=%q, top_n=50)\n", tableName, queryText))
				msg.WriteString("</next_steps>\n")

				return mcp.NewToolSuccess(msg.String())
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
				var msg strings.Builder
				msg.WriteString("Search completed successfully, but no chunks fit within the token budget.\n\n")
				msg.WriteString("<diagnosis>\n")
				msg.WriteString(fmt.Sprintf("All matching chunks exceed the max_output_tokens limit of %d tokens.\n", searchCfg.MaxOutputTokens))
				msg.WriteString(fmt.Sprintf("Found %d diverse chunks after MMR filtering, but all too large.\n", len(diverseChunks)))
				msg.WriteString("</diagnosis>\n\n")
				msg.WriteString("<next_steps>\n")
				msg.WriteString("1. Increase token budget:\n")
				msg.WriteString(fmt.Sprintf("   → similarity_search(table_name=%q, query_text=%q, max_output_tokens=2500)\n\n", tableName, queryText))
				msg.WriteString("2. Reduce chunk size for more granular results:\n")
				msg.WriteString(fmt.Sprintf("   → similarity_search(table_name=%q, query_text=%q, chunk_size_tokens=50)\n\n", tableName, queryText))
				msg.WriteString("3. Use summary format instead:\n")
				msg.WriteString(fmt.Sprintf("   → similarity_search(table_name=%q, query_text=%q, output_format=\"summary\")\n\n", tableName, queryText))
				msg.WriteString("4. Use ids_only to see what matched:\n")
				msg.WriteString(fmt.Sprintf("   → similarity_search(table_name=%q, query_text=%q, output_format=\"ids_only\")\n", tableName, queryText))
				msg.WriteString("</next_steps>\n")

				return mcp.NewToolSuccess(msg.String())
			}

			// Step 10: Format output based on requested format
			var output string
			switch outputFormat {
			case "ids_only":
				output = formatSearchResultsIDsOnly(results, queryText, searchCfg)
			case "summary":
				output = formatSearchResultsSummary(finalChunks, queryText, columnWeights, searchCfg)
			default: // "full"
				output = formatSearchResults(finalChunks, queryText, columnWeights, searchCfg)
			}

			// Prepend database context
			connStr := dbClient.GetDefaultConnection()
			sanitizedConn := database.SanitizeConnStr(connStr)
			result := fmt.Sprintf("Database: %s\nTable: %s\n\n%s", sanitizedConn, tableName, output)

			return mcp.NewToolSuccess(result)
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
		Provider:     serverCfg.Embedding.Provider,
		Model:        serverCfg.Embedding.Model,
		VoyageAPIKey: serverCfg.Embedding.VoyageAPIKey,
		OpenAIAPIKey: serverCfg.Embedding.OpenAIAPIKey,
		OllamaURL:    serverCfg.Embedding.OllamaURL,
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

// formatSearchResultsSummary returns a compact summary with titles/snippets only
func formatSearchResultsSummary(
	chunks []search.ScoredChunk,
	queryText string,
	columnWeights []search.ColumnWeight,
	cfg search.SearchConfig,
) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Similarity Search Results (Summary): %q\n", queryText))
	sb.WriteString(strings.Repeat("=", 80))
	sb.WriteString("\n\n")

	sb.WriteString(fmt.Sprintf("Found %d relevant chunks. Showing summaries:\n\n", len(chunks)))

	// Show compact results
	for i, chunk := range chunks {
		// Create snippet (first 100 chars)
		snippet := chunk.Text
		if len(snippet) > 100 {
			snippet = snippet[:100] + "..."
		}

		sb.WriteString(fmt.Sprintf("%d. Score: %.3f | Source: %s.%s (rank #%d)\n",
			i+1, chunk.Score, chunk.SourceTable, chunk.SourceColumn, chunk.OriginalRank+1))
		sb.WriteString(fmt.Sprintf("   %s\n\n", snippet))
	}

	sb.WriteString(strings.Repeat("=", 80))
	sb.WriteString(fmt.Sprintf("\nTotal: %d results shown in summary mode\n", len(chunks)))
	sb.WriteString("Use output_format='full' to see complete content\n")

	return sb.String()
}

// formatSearchResultsIDsOnly returns just the row IDs for progressive disclosure
func formatSearchResultsIDsOnly(
	results []search.VectorSearchResult,
	queryText string,
	cfg search.SearchConfig,
) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Similarity Search Results (IDs Only): %q\n", queryText))
	sb.WriteString(strings.Repeat("=", 80))
	sb.WriteString("\n\n")

	sb.WriteString(fmt.Sprintf("Found %d matching rows. Row IDs and distances:\n\n", len(results)))

	// Show just IDs and distances
	for i, result := range results {
		var rowID interface{} = i
		if id, ok := result.RowData["id"]; ok {
			rowID = id
		}

		sb.WriteString(fmt.Sprintf("%d. ID: %v | Distance: %.4f\n", i+1, rowID, result.Distance))
	}

	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("=", 80))
	sb.WriteString(fmt.Sprintf("\nTotal: %d results\n", len(results)))
	sb.WriteString("Use output_format='summary' for snippets or 'full' for complete content\n")

	return sb.String()
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
