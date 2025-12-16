/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package search

import (
	"testing"

	"pgedge-postgres-mcp/internal/database"
)

func TestDefaultSearchConfig(t *testing.T) {
	config := DefaultSearchConfig()

	if config.TopN != 10 {
		t.Errorf("expected TopN=10, got %d", config.TopN)
	}
	if config.ChunkSizeTokens != 100 {
		t.Errorf("expected ChunkSizeTokens=100, got %d", config.ChunkSizeTokens)
	}
	if config.OverlapTokens != 25 {
		t.Errorf("expected OverlapTokens=25, got %d", config.OverlapTokens)
	}
	if config.Lambda != 0.6 {
		t.Errorf("expected Lambda=0.6, got %f", config.Lambda)
	}
	if config.MaxOutputTokens != 1000 {
		t.Errorf("expected MaxOutputTokens=1000, got %d", config.MaxOutputTokens)
	}
	if config.DistanceMetric != "cosine" {
		t.Errorf("expected DistanceMetric='cosine', got %q", config.DistanceMetric)
	}
}

func TestInferTextColumnName(t *testing.T) {
	tests := []struct {
		vectorColName string
		expected      string
	}{
		{"title_embedding", "title"},
		{"content_embeddings", "content"},
		{"body_vector", "body"},
		{"text_vectors", "text"},
		{"description_emb", "description"},
		{"titleembedding", "title"},
		{"contentvector", "content"},
		{"no_suffix", "no_suffix"},
		{"embedding", ""},
		{"vector", ""},
		{"title_", "title"},
	}

	for _, tt := range tests {
		t.Run(tt.vectorColName, func(t *testing.T) {
			result := inferTextColumnName(tt.vectorColName)
			if result != tt.expected {
				t.Errorf("inferTextColumnName(%q): expected %q, got %q",
					tt.vectorColName, tt.expected, result)
			}
		})
	}
}

func TestIsTitleName(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		{"title", true},
		{"article_title", true},
		{"name", true},
		{"user_name", true},
		{"heading", true},
		{"section_heading", true},
		{"header", true},
		{"subject", true},
		{"email_subject", true},
		{"label", true},
		{"content", false},
		{"body", false},
		{"description", false},
		{"random_column", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isTitleName(tt.name)
			if result != tt.expected {
				t.Errorf("isTitleName(%q): expected %v, got %v",
					tt.name, tt.expected, result)
			}
		})
	}
}

func TestIsContentName(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		{"content", true},
		{"page_content", true},
		{"text", true},
		{"full_text", true},
		{"body", true},
		{"email_body", true},
		{"description", true},
		{"product_description", true},
		{"detail", true},
		{"order_details", true},
		{"article", true},
		{"document", true},
		{"passage", true},
		{"title", false},
		{"name", false},
		{"id", false},
		{"random_column", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isContentName(tt.name)
			if result != tt.expected {
				t.Errorf("isContentName(%q): expected %v, got %v",
					tt.name, tt.expected, result)
			}
		})
	}
}

func TestIsTextColumn(t *testing.T) {
	tests := []struct {
		dataType string
		expected bool
	}{
		{"text", true},
		{"TEXT", true},
		{"character varying", true},
		{"CHARACTER VARYING(255)", true},
		{"varchar", true},
		{"varchar(100)", true},
		{"character", true},
		{"char", true},
		{"char(10)", true},
		{"string", true},
		{"integer", false},
		{"bigint", false},
		{"boolean", false},
		{"timestamp", false},
		{"vector(1536)", false},
		{"jsonb", false},
	}

	for _, tt := range tests {
		t.Run(tt.dataType, func(t *testing.T) {
			result := isTextColumn(tt.dataType)
			if result != tt.expected {
				t.Errorf("isTextColumn(%q): expected %v, got %v",
					tt.dataType, tt.expected, result)
			}
		})
	}
}

func TestDetectColumnTypes_Empty(t *testing.T) {
	tableInfo := database.TableInfo{
		Columns: []database.ColumnInfo{},
	}

	weights := DetectColumnTypes(tableInfo, nil)

	if len(weights) != 0 {
		t.Errorf("expected empty weights for empty table, got %d", len(weights))
	}
}

func TestDetectColumnTypes_NoVectorColumns(t *testing.T) {
	tableInfo := database.TableInfo{
		TableName: "test_table",
		Columns: []database.ColumnInfo{
			{ColumnName: "id", DataType: "integer"},
			{ColumnName: "title", DataType: "text"},
			{ColumnName: "content", DataType: "text"},
		},
	}

	weights := DetectColumnTypes(tableInfo, nil)

	// No vector columns means no weighted columns
	if len(weights) != 0 {
		t.Errorf("expected no weights without vector columns, got %d", len(weights))
	}
}

func TestDetectColumnTypes_TitleAndContent(t *testing.T) {
	tableInfo := database.TableInfo{
		TableName: "articles",
		Columns: []database.ColumnInfo{
			{ColumnName: "id", DataType: "integer"},
			{ColumnName: "title", DataType: "text", IsVectorColumn: false},
			{ColumnName: "title_embedding", DataType: "vector(1536)", IsVectorColumn: true},
			{ColumnName: "content", DataType: "text", IsVectorColumn: false},
			{ColumnName: "content_embedding", DataType: "vector(1536)", IsVectorColumn: true},
		},
	}

	weights := DetectColumnTypes(tableInfo, nil)

	if len(weights) != 2 {
		t.Fatalf("expected 2 weighted columns, got %d", len(weights))
	}

	// Check title column
	var titleWeight, contentWeight *ColumnWeight
	for i := range weights {
		if weights[i].ColumnName == "title" {
			titleWeight = &weights[i]
		}
		if weights[i].ColumnName == "content" {
			contentWeight = &weights[i]
		}
	}

	if titleWeight == nil {
		t.Fatal("expected title column in weights")
	}
	if !titleWeight.IsTitle {
		t.Error("expected title to be detected as title column")
	}
	if titleWeight.VectorName != "title_embedding" {
		t.Errorf("expected vector name 'title_embedding', got %q", titleWeight.VectorName)
	}

	if contentWeight == nil {
		t.Fatal("expected content column in weights")
	}
	if contentWeight.IsTitle {
		t.Error("expected content to not be a title column")
	}
}

func TestDetectColumnTypes_SingleColumn(t *testing.T) {
	tableInfo := database.TableInfo{
		TableName: "documents",
		Columns: []database.ColumnInfo{
			{ColumnName: "id", DataType: "integer"},
			{ColumnName: "body", DataType: "text", IsVectorColumn: false},
			{ColumnName: "body_embedding", DataType: "vector(1536)", IsVectorColumn: true},
		},
	}

	weights := DetectColumnTypes(tableInfo, nil)

	if len(weights) != 1 {
		t.Fatalf("expected 1 weighted column, got %d", len(weights))
	}

	// Single column should have weight 1.0
	if weights[0].Weight != 1.0 {
		t.Errorf("expected single column weight=1.0, got %f", weights[0].Weight)
	}
}

func TestDetectColumnTypes_WithDescription(t *testing.T) {
	tableInfo := database.TableInfo{
		TableName: "documents",
		Columns: []database.ColumnInfo{
			{ColumnName: "headline", DataType: "text", Description: "Article title/heading"},
			{ColumnName: "headline_vector", DataType: "vector(1536)", IsVectorColumn: true},
		},
	}

	weights := DetectColumnTypes(tableInfo, nil)

	if len(weights) != 1 {
		t.Fatalf("expected 1 weighted column, got %d", len(weights))
	}

	// Description contains "title" so should be detected as title
	if !weights[0].IsTitle {
		t.Error("expected column with 'title' in description to be marked as title")
	}
}

func TestDetectColumnTypes_WithSampleData(t *testing.T) {
	tableInfo := database.TableInfo{
		TableName: "documents",
		Columns: []database.ColumnInfo{
			{ColumnName: "short_field", DataType: "text"},
			{ColumnName: "short_field_embedding", DataType: "vector(1536)", IsVectorColumn: true},
			{ColumnName: "long_field", DataType: "text"},
			{ColumnName: "long_field_embedding", DataType: "vector(1536)", IsVectorColumn: true},
		},
	}

	sampleData := map[string]string{
		"short_field": "Brief title",                                           // < 100 chars
		"long_field":  string(make([]byte, 600)) + "Long content goes here...", // > 500 chars
	}

	weights := DetectColumnTypes(tableInfo, sampleData)

	if len(weights) != 2 {
		t.Fatalf("expected 2 weighted columns, got %d", len(weights))
	}

	// Find weights by column name
	var shortWeight, longWeight *ColumnWeight
	for i := range weights {
		if weights[i].ColumnName == "short_field" {
			shortWeight = &weights[i]
		}
		if weights[i].ColumnName == "long_field" {
			longWeight = &weights[i]
		}
	}

	if shortWeight == nil || longWeight == nil {
		t.Fatal("expected both columns in weights")
	}

	// Short text should be detected as title
	if !shortWeight.IsTitle {
		t.Error("expected short text to be detected as title")
	}

	// Long text should be detected as content
	if longWeight.IsTitle {
		t.Error("expected long text to be detected as content (not title)")
	}
}

func TestDetectColumnTypes_WeightNormalization(t *testing.T) {
	tableInfo := database.TableInfo{
		TableName: "articles",
		Columns: []database.ColumnInfo{
			{ColumnName: "title", DataType: "text"},
			{ColumnName: "title_embedding", DataType: "vector", IsVectorColumn: true},
			{ColumnName: "content", DataType: "text"},
			{ColumnName: "content_embedding", DataType: "vector", IsVectorColumn: true},
		},
	}

	weights := DetectColumnTypes(tableInfo, nil)

	// Weights should be normalized
	totalWeight := 0.0
	for _, w := range weights {
		totalWeight += w.Weight
	}

	// Total should be approximately 1.0 (normalized)
	if totalWeight < 0.99 || totalWeight > 1.01 {
		t.Errorf("expected total weight ~1.0, got %f", totalWeight)
	}
}

func TestCalculateWeightedDistance(t *testing.T) {
	tests := []struct {
		name      string
		distances map[string]float64
		weights   []ColumnWeight
		expected  float64
	}{
		{
			name:      "empty weights",
			distances: map[string]float64{"vec1": 0.5},
			weights:   []ColumnWeight{},
			expected:  0.0,
		},
		{
			name:      "single weight",
			distances: map[string]float64{"vec1": 0.5},
			weights: []ColumnWeight{
				{VectorName: "vec1", Weight: 1.0},
			},
			expected: 0.5,
		},
		{
			name:      "multiple weights",
			distances: map[string]float64{"vec1": 0.3, "vec2": 0.7},
			weights: []ColumnWeight{
				{VectorName: "vec1", Weight: 0.4},
				{VectorName: "vec2", Weight: 0.6},
			},
			expected: 0.3*0.4 + 0.7*0.6, // 0.12 + 0.42 = 0.54
		},
		{
			name:      "missing distance",
			distances: map[string]float64{"vec1": 0.5},
			weights: []ColumnWeight{
				{VectorName: "vec1", Weight: 0.5},
				{VectorName: "vec2", Weight: 0.5}, // Not in distances
			},
			expected: 0.5 * 0.5, // Only vec1 contributes
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateWeightedDistance(tt.distances, tt.weights)
			diff := result - tt.expected
			if diff < 0 {
				diff = -diff
			}
			if diff > 0.001 {
				t.Errorf("expected %f, got %f", tt.expected, result)
			}
		})
	}
}
