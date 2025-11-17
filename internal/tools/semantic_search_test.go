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
	"strings"
	"testing"

	"pgedge-postgres-mcp/internal/config"
	"pgedge-postgres-mcp/internal/database"
)

// Helper function to create a mock config for tests
func createMockConfig() *config.Config {
	return &config.Config{
		Embedding: config.EmbeddingConfig{
			Enabled:         false, // Disabled by default for tests
			Provider:        "ollama",
			Model:           "nomic-embed-text",
			AnthropicAPIKey: "",
			OllamaURL:       "http://localhost:11434",
		},
	}
}

func TestSemanticSearchTool(t *testing.T) {
	t.Run("database not ready", func(t *testing.T) {
		client := database.NewClient(nil)
		// Don't add any connections - database is not ready

		tool := SemanticSearchTool(client, createMockConfig())
		response, err := tool.Handler(map[string]interface{}{
			"table_name":    "documents",
			"vector_column": "embedding",
			"query_vector":  []float64{0.1, 0.2, 0.3},
		})

		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}
		if !response.IsError {
			t.Error("Expected IsError=true when database not ready")
		}
		if len(response.Content) == 0 {
			t.Fatal("Expected content in response")
		}
		content := response.Content[0].Text
		if !strings.Contains(content, "Database is still initializing") {
			t.Errorf("Expected database not ready message, got: %s", content)
		}
	})

	t.Run("missing table_name parameter", func(t *testing.T) {
		metadata := map[string]database.TableInfo{}
		client := createMockClient(metadata)

		tool := SemanticSearchTool(client, createMockConfig())
		response, err := tool.Handler(map[string]interface{}{
			"vector_column": "embedding",
			"query_vector":  []float64{0.1, 0.2, 0.3},
		})

		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}
		if !response.IsError {
			t.Error("Expected IsError=true when table_name is missing")
		}
		content := response.Content[0].Text
		if !strings.Contains(content, "Missing or invalid 'table_name' parameter") {
			t.Errorf("Expected missing table_name error, got: %s", content)
		}
	})

	t.Run("missing vector_column parameter", func(t *testing.T) {
		metadata := map[string]database.TableInfo{}
		client := createMockClient(metadata)

		tool := SemanticSearchTool(client, createMockConfig())
		response, err := tool.Handler(map[string]interface{}{
			"table_name":   "documents",
			"query_vector": []float64{0.1, 0.2, 0.3},
		})

		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}
		if !response.IsError {
			t.Error("Expected IsError=true when vector_column is missing")
		}
		content := response.Content[0].Text
		if !strings.Contains(content, "Missing or invalid 'vector_column' parameter") {
			t.Errorf("Expected missing vector_column error, got: %s", content)
		}
	})

	t.Run("missing query_vector parameter", func(t *testing.T) {
		metadata := map[string]database.TableInfo{}
		client := createMockClient(metadata)

		tool := SemanticSearchTool(client, createMockConfig())
		response, err := tool.Handler(map[string]interface{}{
			"table_name":    "documents",
			"vector_column": "embedding",
		})

		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}
		if !response.IsError {
			t.Error("Expected IsError=true when both query_vector and text_query are missing")
		}
		content := response.Content[0].Text
		if !strings.Contains(content, "Either 'query_vector' or 'text_query' parameter must be provided") {
			t.Errorf("Expected missing query_vector/text_query error, got: %s", content)
		}
	})

	t.Run("empty query_vector", func(t *testing.T) {
		metadata := map[string]database.TableInfo{}
		client := createMockClient(metadata)

		tool := SemanticSearchTool(client, createMockConfig())
		response, err := tool.Handler(map[string]interface{}{
			"table_name":    "documents",
			"vector_column": "embedding",
			"query_vector":  []interface{}{},
		})

		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}
		if !response.IsError {
			t.Error("Expected IsError=true when query_vector is empty")
		}
		content := response.Content[0].Text
		if !strings.Contains(content, "'query_vector' cannot be empty") {
			t.Errorf("Expected empty query_vector error, got: %s", content)
		}
	})

	t.Run("invalid query_vector type", func(t *testing.T) {
		metadata := map[string]database.TableInfo{}
		client := createMockClient(metadata)

		tool := SemanticSearchTool(client, createMockConfig())
		response, err := tool.Handler(map[string]interface{}{
			"table_name":    "documents",
			"vector_column": "embedding",
			"query_vector":  "not an array",
		})

		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}
		if !response.IsError {
			t.Error("Expected IsError=true when query_vector has invalid type")
		}
		content := response.Content[0].Text
		if !strings.Contains(content, "Invalid 'query_vector' parameter") {
			t.Errorf("Expected invalid query_vector error, got: %s", content)
		}
	})

	t.Run("invalid top_k value", func(t *testing.T) {
		metadata := map[string]database.TableInfo{}
		client := createMockClient(metadata)

		tool := SemanticSearchTool(client, createMockConfig())
		response, err := tool.Handler(map[string]interface{}{
			"table_name":    "documents",
			"vector_column": "embedding",
			"query_vector":  []float64{0.1, 0.2, 0.3},
			"top_k":         0,
		})

		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}
		if !response.IsError {
			t.Error("Expected IsError=true when top_k is 0")
		}
		content := response.Content[0].Text
		if !strings.Contains(content, "'top_k' must be greater than 0") {
			t.Errorf("Expected invalid top_k error, got: %s", content)
		}
	})

	t.Run("invalid distance_metric", func(t *testing.T) {
		metadata := map[string]database.TableInfo{}
		client := createMockClient(metadata)

		tool := SemanticSearchTool(client, createMockConfig())
		response, err := tool.Handler(map[string]interface{}{
			"table_name":      "documents",
			"vector_column":   "embedding",
			"query_vector":    []float64{0.1, 0.2, 0.3},
			"distance_metric": "invalid_metric",
		})

		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}
		if !response.IsError {
			t.Error("Expected IsError=true when distance_metric is invalid")
		}
		content := response.Content[0].Text
		if !strings.Contains(content, "invalid distance metric") {
			t.Errorf("Expected invalid distance_metric error, got: %s", content)
		}
	})

	t.Run("table not found", func(t *testing.T) {
		metadata := map[string]database.TableInfo{
			"public.other_table": {
				SchemaName: "public",
				TableName:  "other_table",
				TableType:  "TABLE",
				Columns:    []database.ColumnInfo{},
			},
		}
		client := createMockClient(metadata)

		tool := SemanticSearchTool(client, createMockConfig())
		response, err := tool.Handler(map[string]interface{}{
			"table_name":    "documents",
			"vector_column": "embedding",
			"query_vector":  []float64{0.1, 0.2, 0.3},
		})

		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}
		if !response.IsError {
			t.Error("Expected IsError=true when table not found")
		}
		content := response.Content[0].Text
		if !strings.Contains(content, "Table 'documents' not found") {
			t.Errorf("Expected table not found error, got: %s", content)
		}
	})

	t.Run("column not found", func(t *testing.T) {
		metadata := map[string]database.TableInfo{
			"public.documents": {
				SchemaName: "public",
				TableName:  "documents",
				TableType:  "TABLE",
				Columns: []database.ColumnInfo{
					{
						ColumnName: "id",
						DataType:   "integer",
						IsNullable: "NO",
					},
				},
			},
		}
		client := createMockClient(metadata)

		tool := SemanticSearchTool(client, createMockConfig())
		response, err := tool.Handler(map[string]interface{}{
			"table_name":    "documents",
			"vector_column": "embedding",
			"query_vector":  []float64{0.1, 0.2, 0.3},
		})

		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}
		if !response.IsError {
			t.Error("Expected IsError=true when column not found")
		}
		content := response.Content[0].Text
		if !strings.Contains(content, "Column 'embedding' not found") {
			t.Errorf("Expected column not found error, got: %s", content)
		}
	})

	t.Run("column is not vector type", func(t *testing.T) {
		metadata := map[string]database.TableInfo{
			"public.documents": {
				SchemaName: "public",
				TableName:  "documents",
				TableType:  "TABLE",
				Columns: []database.ColumnInfo{
					{
						ColumnName:     "text_data",
						DataType:       "text",
						IsNullable:     "YES",
						IsVectorColumn: false,
					},
				},
			},
		}
		client := createMockClient(metadata)

		tool := SemanticSearchTool(client, createMockConfig())
		response, err := tool.Handler(map[string]interface{}{
			"table_name":    "documents",
			"vector_column": "text_data",
			"query_vector":  []float64{0.1, 0.2, 0.3},
		})

		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}
		if !response.IsError {
			t.Error("Expected IsError=true when column is not a vector")
		}
		content := response.Content[0].Text
		if !strings.Contains(content, "is not a pgvector column") {
			t.Errorf("Expected not a vector column error, got: %s", content)
		}
	})

	t.Run("dimension mismatch", func(t *testing.T) {
		metadata := map[string]database.TableInfo{
			"public.documents": {
				SchemaName: "public",
				TableName:  "documents",
				TableType:  "TABLE",
				Columns: []database.ColumnInfo{
					{
						ColumnName:       "embedding",
						DataType:         "vector(1536)",
						IsNullable:       "YES",
						IsVectorColumn:   true,
						VectorDimensions: 1536,
					},
				},
			},
		}
		client := createMockClient(metadata)

		tool := SemanticSearchTool(client, createMockConfig())
		response, err := tool.Handler(map[string]interface{}{
			"table_name":    "documents",
			"vector_column": "embedding",
			"query_vector":  []float64{0.1, 0.2, 0.3}, // Only 3 dimensions, need 1536
		})

		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}
		if !response.IsError {
			t.Error("Expected IsError=true when dimensions don't match")
		}
		content := response.Content[0].Text
		if !strings.Contains(content, "Query vector dimensions (3) don't match column dimensions (1536)") {
			t.Errorf("Expected dimension mismatch error, got: %s", content)
		}
	})

	t.Run("parse query vector from interface slice", func(t *testing.T) {
		tests := []struct {
			name     string
			input    interface{}
			expected []float64
			wantErr  bool
		}{
			{
				name:     "float64 slice",
				input:    []interface{}{1.0, 2.0, 3.0},
				expected: []float64{1.0, 2.0, 3.0},
				wantErr:  false,
			},
			{
				name:     "int values",
				input:    []interface{}{1, 2, 3},
				expected: []float64{1.0, 2.0, 3.0},
				wantErr:  false,
			},
			{
				name:     "mixed float and int",
				input:    []interface{}{1.5, 2, 3.7},
				expected: []float64{1.5, 2.0, 3.7},
				wantErr:  false,
			},
			{
				name:    "invalid element type",
				input:   []interface{}{"string", 2.0, 3.0},
				wantErr: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result, err := parseQueryVector(tt.input)
				if tt.wantErr {
					if err == nil {
						t.Error("Expected error but got none")
					}
				} else {
					if err != nil {
						t.Errorf("Unexpected error: %v", err)
					}
					if len(result) != len(tt.expected) {
						t.Errorf("Expected length %d, got %d", len(tt.expected), len(result))
					}
					for i := range result {
						if result[i] != tt.expected[i] {
							t.Errorf("At index %d: expected %f, got %f", i, tt.expected[i], result[i])
						}
					}
				}
			})
		}
	})

	t.Run("distance operator mapping", func(t *testing.T) {
		tests := []struct {
			metric   string
			wantOp   string
			wantName string
			wantErr  bool
		}{
			{"cosine", "<=>", "Cosine Distance", false},
			{"l2", "<->", "L2 (Euclidean) Distance", false},
			{"euclidean", "<->", "L2 (Euclidean) Distance", false},
			{"inner_product", "<#>", "Inner Product (Negative)", false},
			{"inner", "<#>", "Inner Product (Negative)", false},
			{"invalid", "", "", true},
		}

		for _, tt := range tests {
			t.Run(tt.metric, func(t *testing.T) {
				op, name, err := getDistanceOperator(tt.metric)
				if tt.wantErr {
					if err == nil {
						t.Error("Expected error but got none")
					}
				} else {
					if err != nil {
						t.Errorf("Unexpected error: %v", err)
					}
					if op != tt.wantOp {
						t.Errorf("Expected operator %s, got %s", tt.wantOp, op)
					}
					if name != tt.wantName {
						t.Errorf("Expected name %s, got %s", tt.wantName, name)
					}
				}
			})
		}
	})

	t.Run("format vector literal", func(t *testing.T) {
		tests := []struct {
			name     string
			input    []float64
			expected string
		}{
			{
				name:     "single element",
				input:    []float64{1.5},
				expected: "[1.500000]",
			},
			{
				name:     "multiple elements",
				input:    []float64{1.0, 2.5, 3.7},
				expected: "[1.000000,2.500000,3.700000]",
			},
			{
				name:     "negative values",
				input:    []float64{-1.5, 0.0, 2.3},
				expected: "[-1.500000,0.000000,2.300000]",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := formatVectorLiteral(tt.input)
				if result != tt.expected {
					t.Errorf("Expected %s, got %s", tt.expected, result)
				}
			})
		}
	})

	t.Run("format available tables", func(t *testing.T) {
		metadata := map[string]database.TableInfo{
			"public.users": {
				SchemaName: "public",
				TableName:  "users",
			},
			"public.orders": {
				SchemaName: "public",
				TableName:  "orders",
			},
		}

		result := formatAvailableTables(metadata)
		if !strings.Contains(result, "public.users") {
			t.Error("Expected 'public.users' in result")
		}
		if !strings.Contains(result, "public.orders") {
			t.Error("Expected 'public.orders' in result")
		}
	})

	t.Run("format available columns", func(t *testing.T) {
		tableInfo := database.TableInfo{
			SchemaName: "public",
			TableName:  "documents",
			Columns: []database.ColumnInfo{
				{
					ColumnName:     "id",
					DataType:       "integer",
					IsVectorColumn: false,
				},
				{
					ColumnName:       "embedding",
					DataType:         "vector(1536)",
					IsVectorColumn:   true,
					VectorDimensions: 1536,
				},
			},
		}

		result := formatAvailableColumns(tableInfo)
		if !strings.Contains(result, "id") {
			t.Error("Expected 'id' in result")
		}
		if !strings.Contains(result, "embedding (vector(1536))") {
			t.Errorf("Expected 'embedding (vector(1536))' in result, got: %s", result)
		}
	})
}
