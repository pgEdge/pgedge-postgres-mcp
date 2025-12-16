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
	"testing"

	"pgedge-postgres-mcp/internal/database"
)

func TestInferTextColumnName(t *testing.T) {
	tests := []struct {
		name      string
		vectorCol string
		wantText  string
	}{
		{
			name:      "embedding suffix",
			vectorCol: "content_embedding",
			wantText:  "content",
		},
		{
			name:      "embeddings suffix",
			vectorCol: "content_embeddings",
			wantText:  "content",
		},
		{
			name:      "vector suffix",
			vectorCol: "title_vector",
			wantText:  "title",
		},
		{
			name:      "vectors suffix",
			vectorCol: "description_vectors",
			wantText:  "description",
		},
		{
			name:      "emb suffix",
			vectorCol: "text_emb",
			wantText:  "text",
		},
		{
			name:      "no suffix",
			vectorCol: "content",
			wantText:  "content",
		},
		{
			name:      "just embedding",
			vectorCol: "embedding",
			wantText:  "",
		},
		{
			name:      "uppercase suffix",
			vectorCol: "content_EMBEDDING",
			wantText:  "content",
		},
		{
			name:      "mixed case",
			vectorCol: "Title_Vector",
			wantText:  "Title",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inferTextColumnName(tt.vectorCol)
			if got != tt.wantText {
				t.Errorf("inferTextColumnName(%q) = %q, want %q", tt.vectorCol, got, tt.wantText)
			}
		})
	}
}

func TestIsTextDataType(t *testing.T) {
	tests := []struct {
		name     string
		dataType string
		want     bool
	}{
		{"text type", "text", true},
		{"character varying", "character varying", true},
		{"varchar", "varchar", true},
		{"character", "character", true},
		{"char", "char", true},
		{"varchar with length", "varchar(255)", true},
		{"char with length", "char(10)", true},
		{"integer", "integer", false},
		{"boolean", "boolean", false},
		{"timestamp", "timestamp", false},
		{"json", "json", false},
		{"jsonb", "jsonb", false},
		{"vector", "vector", false},
		{"empty", "", false},
		{"uppercase TEXT", "TEXT", true},
		{"uppercase VARCHAR", "VARCHAR", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTextDataType(tt.dataType)
			if got != tt.want {
				t.Errorf("isTextDataType(%q) = %v, want %v", tt.dataType, got, tt.want)
			}
		})
	}
}

func TestGetDistanceOperator(t *testing.T) {
	tests := []struct {
		name   string
		metric string
		want   string
	}{
		{"cosine default", "cosine", "<=>"},
		{"l2", "l2", "<->"},
		{"euclidean", "euclidean", "<->"},
		{"inner_product", "inner_product", "<#>"},
		{"inner", "inner", "<#>"},
		{"empty defaults to cosine", "", "<=>"},
		{"unknown defaults to cosine", "unknown", "<=>"},
		{"uppercase L2", "L2", "<->"},
		{"uppercase COSINE", "COSINE", "<=>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getDistanceOperator(tt.metric)
			if got != tt.want {
				t.Errorf("getDistanceOperator(%q) = %q, want %q", tt.metric, got, tt.want)
			}
		})
	}
}

func TestFormatEmbeddingForPostgres(t *testing.T) {
	tests := []struct {
		name      string
		embedding []float64
		want      string
	}{
		{
			name:      "simple embedding",
			embedding: []float64{1.0, 2.0, 3.0},
			want:      "[1.000000,2.000000,3.000000]",
		},
		{
			name:      "empty embedding",
			embedding: []float64{},
			want:      "[]",
		},
		{
			name:      "single value",
			embedding: []float64{0.5},
			want:      "[0.500000]",
		},
		{
			name:      "negative values",
			embedding: []float64{-1.0, 0.0, 1.0},
			want:      "[-1.000000,0.000000,1.000000]",
		},
		{
			name:      "small values",
			embedding: []float64{0.001, 0.002, 0.003},
			want:      "[0.001000,0.002000,0.003000]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatEmbeddingForPostgres(tt.embedding)
			if got != tt.want {
				t.Errorf("formatEmbeddingForPostgres() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMinInt(t *testing.T) {
	tests := []struct {
		a, b int
		want int
	}{
		{1, 2, 1},
		{2, 1, 1},
		{0, 0, 0},
		{-1, 1, -1},
		{100, 50, 50},
		{-10, -5, -10},
	}

	for _, tt := range tests {
		got := minInt(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("minInt(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestDiscoverVectorColumns(t *testing.T) {
	tests := []struct {
		name      string
		tableInfo database.TableInfo
		wantCount int
	}{
		{
			name: "table with vector columns",
			tableInfo: database.TableInfo{
				Columns: []database.ColumnInfo{
					{ColumnName: "id", DataType: "integer", IsVectorColumn: false},
					{ColumnName: "content", DataType: "text", IsVectorColumn: false},
					{ColumnName: "embedding", DataType: "vector", IsVectorColumn: true},
				},
			},
			wantCount: 1,
		},
		{
			name: "table without vector columns",
			tableInfo: database.TableInfo{
				Columns: []database.ColumnInfo{
					{ColumnName: "id", DataType: "integer", IsVectorColumn: false},
					{ColumnName: "name", DataType: "text", IsVectorColumn: false},
				},
			},
			wantCount: 0,
		},
		{
			name: "table with multiple vector columns",
			tableInfo: database.TableInfo{
				Columns: []database.ColumnInfo{
					{ColumnName: "title_embedding", DataType: "vector", IsVectorColumn: true},
					{ColumnName: "content_embedding", DataType: "vector", IsVectorColumn: true},
				},
			},
			wantCount: 2,
		},
		{
			name:      "empty table",
			tableInfo: database.TableInfo{},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := discoverVectorColumns(tt.tableInfo)
			if len(got) != tt.wantCount {
				t.Errorf("discoverVectorColumns() returned %d columns, want %d", len(got), tt.wantCount)
			}
		})
	}
}

func TestDiscoverTextColumns(t *testing.T) {
	tests := []struct {
		name       string
		tableInfo  database.TableInfo
		vectorCols []database.ColumnInfo
		wantCols   []string
	}{
		{
			name: "matches vector to text column",
			tableInfo: database.TableInfo{
				Columns: []database.ColumnInfo{
					{ColumnName: "content", DataType: "text", IsVectorColumn: false},
					{ColumnName: "content_embedding", DataType: "vector", IsVectorColumn: true},
				},
			},
			vectorCols: []database.ColumnInfo{
				{ColumnName: "content_embedding"},
			},
			wantCols: []string{"content"},
		},
		{
			name: "returns all text columns if no match",
			tableInfo: database.TableInfo{
				Columns: []database.ColumnInfo{
					{ColumnName: "title", DataType: "text", IsVectorColumn: false},
					{ColumnName: "description", DataType: "text", IsVectorColumn: false},
					{ColumnName: "embedding", DataType: "vector", IsVectorColumn: true},
				},
			},
			vectorCols: []database.ColumnInfo{
				{ColumnName: "embedding"},
			},
			wantCols: []string{"title", "description"},
		},
		{
			name: "no text columns",
			tableInfo: database.TableInfo{
				Columns: []database.ColumnInfo{
					{ColumnName: "id", DataType: "integer", IsVectorColumn: false},
					{ColumnName: "embedding", DataType: "vector", IsVectorColumn: true},
				},
			},
			vectorCols: []database.ColumnInfo{
				{ColumnName: "embedding"},
			},
			wantCols: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := discoverTextColumns(tt.tableInfo, tt.vectorCols)
			if len(got) != len(tt.wantCols) {
				t.Errorf("discoverTextColumns() returned %d columns, want %d", len(got), len(tt.wantCols))
				return
			}
			for i, col := range got {
				if col != tt.wantCols[i] {
					t.Errorf("discoverTextColumns()[%d] = %q, want %q", i, col, tt.wantCols[i])
				}
			}
		})
	}
}

func TestFindTableInMetadataMap(t *testing.T) {
	metadata := map[string]database.TableInfo{
		"public.users": {
			SchemaName: "public",
			TableName:  "users",
			Columns:    []database.ColumnInfo{{ColumnName: "id"}},
		},
		"public.posts": {
			SchemaName: "public",
			TableName:  "posts",
			Columns:    []database.ColumnInfo{{ColumnName: "id"}},
		},
		"custom.data": {
			SchemaName: "custom",
			TableName:  "data",
			Columns:    []database.ColumnInfo{{ColumnName: "id"}},
		},
	}

	tests := []struct {
		name      string
		tableName string
		wantErr   bool
	}{
		{
			name:      "public.users with schema",
			tableName: "public.users",
			wantErr:   false,
		},
		{
			name:      "users without schema defaults to public",
			tableName: "users",
			wantErr:   false,
		},
		{
			name:      "custom.data with schema",
			tableName: "custom.data",
			wantErr:   false,
		},
		{
			name:      "non-existent table",
			tableName: "nonexistent",
			wantErr:   true,
		},
		{
			name:      "non-existent schema",
			tableName: "other.users",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := findTableInMetadataMap(metadata, tt.tableName)
			if (err != nil) != tt.wantErr {
				t.Errorf("findTableInMetadataMap() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
