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
)

func TestNewMMRSelector(t *testing.T) {
	tests := []struct {
		name           string
		lambda         float64
		expectedLambda float64
	}{
		{"normal value", 0.5, 0.5},
		{"max relevance", 1.0, 1.0},
		{"max diversity", 0.0, 0.0},
		{"negative clamped to 0", -0.5, 0.0},
		{"above 1 clamped to 1", 1.5, 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selector := NewMMRSelector(tt.lambda)
			if selector.lambda != tt.expectedLambda {
				t.Errorf("expected lambda=%f, got %f", tt.expectedLambda, selector.lambda)
			}
		})
	}
}

func TestMMRSelectChunks_EmptyInput(t *testing.T) {
	selector := NewMMRSelector(0.5)
	result := selector.SelectChunks([]ScoredChunk{}, 5)
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d chunks", len(result))
	}
}

func TestMMRSelectChunks_LessThanMax(t *testing.T) {
	selector := NewMMRSelector(0.5)
	chunks := []ScoredChunk{
		{Text: "chunk 1", Score: 1.0},
		{Text: "chunk 2", Score: 0.8},
	}

	result := selector.SelectChunks(chunks, 5)
	if len(result) != 2 {
		t.Errorf("expected 2 chunks (all input), got %d", len(result))
	}
}

func TestMMRSelectChunks_LimitSelection(t *testing.T) {
	selector := NewMMRSelector(0.5)
	chunks := []ScoredChunk{
		{Text: "chunk 1", Score: 1.0},
		{Text: "chunk 2", Score: 0.9},
		{Text: "chunk 3", Score: 0.8},
		{Text: "chunk 4", Score: 0.7},
		{Text: "chunk 5", Score: 0.6},
	}

	result := selector.SelectChunks(chunks, 3)
	if len(result) != 3 {
		t.Errorf("expected 3 chunks, got %d", len(result))
	}
}

func TestMMRSelectChunks_HighRelevance(t *testing.T) {
	// With lambda=1.0, should select purely by relevance score
	selector := NewMMRSelector(1.0)
	chunks := []ScoredChunk{
		{Text: "best match", Score: 1.0, SourceRowID: 1},
		{Text: "second best", Score: 0.8, SourceRowID: 2},
		{Text: "third best", Score: 0.6, SourceRowID: 3},
	}

	result := selector.SelectChunks(chunks, 2)
	if len(result) != 2 {
		t.Errorf("expected 2 chunks, got %d", len(result))
	}

	// With high lambda, should pick top scores
	if result[0].Text != "best match" {
		t.Errorf("expected first chunk to be 'best match', got %q", result[0].Text)
	}
}

func TestMMRSelectChunks_DiversitySelection(t *testing.T) {
	// With lambda=0.0, should maximize diversity
	selector := NewMMRSelector(0.0)

	// Create chunks with same row ID (similar) and different row IDs (diverse)
	chunks := []ScoredChunk{
		{Text: "postgresql query one", Score: 1.0, SourceRowID: 1, SourceColumn: "content", ChunkIndex: 0},
		{Text: "postgresql query two", Score: 0.9, SourceRowID: 1, SourceColumn: "content", ChunkIndex: 1},
		{Text: "mysql database info", Score: 0.8, SourceRowID: 2, SourceColumn: "content", ChunkIndex: 0},
		{Text: "sqlite overview text", Score: 0.7, SourceRowID: 3, SourceColumn: "content", ChunkIndex: 0},
	}

	result := selector.SelectChunks(chunks, 2)
	if len(result) != 2 {
		t.Errorf("expected 2 chunks, got %d", len(result))
	}

	// With max diversity, should prefer chunks from different rows
	// After selecting first chunk (best score), second should be from different row
	selectedRows := make(map[interface{}]bool)
	for _, chunk := range result {
		selectedRows[chunk.SourceRowID] = true
	}

	// Should have selected from different rows for diversity
	if len(selectedRows) < 2 {
		t.Errorf("expected chunks from different rows, got %d unique rows", len(selectedRows))
	}
}

func TestMMRSelectChunks_ZeroMaxScore(t *testing.T) {
	selector := NewMMRSelector(0.5)
	chunks := []ScoredChunk{
		{Text: "chunk 1", Score: 0.0},
		{Text: "chunk 2", Score: 0.0},
	}

	result := selector.SelectChunks(chunks, 1)
	if len(result) != 1 {
		t.Errorf("expected 1 chunk, got %d", len(result))
	}
}

func TestDiversityScore_EmptySelected(t *testing.T) {
	selector := NewMMRSelector(0.5)
	candidate := ScoredChunk{Text: "test", SourceRowID: 1}

	score := selector.diversityScore(candidate, []ScoredChunk{})
	if score != 1.0 {
		t.Errorf("expected diversity=1.0 for empty selected, got %f", score)
	}
}

func TestDiversityScore_SameRow(t *testing.T) {
	selector := NewMMRSelector(0.5)
	candidate := ScoredChunk{
		Text:         "chunk 2",
		SourceRowID:  1,
		SourceColumn: "content",
		ChunkIndex:   1,
	}
	selected := []ScoredChunk{
		{Text: "chunk 1", SourceRowID: 1, SourceColumn: "content", ChunkIndex: 0},
	}

	score := selector.diversityScore(candidate, selected)
	// Adjacent chunks from same row should have low diversity (high similarity)
	if score > 0.2 {
		t.Errorf("expected low diversity for adjacent same-row chunks, got %f", score)
	}
}

func TestDiversityScore_DifferentRows(t *testing.T) {
	selector := NewMMRSelector(0.5)
	candidate := ScoredChunk{
		Text:        "completely different topic about weather",
		SourceRowID: 2,
	}
	selected := []ScoredChunk{
		{Text: "postgresql database administration", SourceRowID: 1},
	}

	score := selector.diversityScore(candidate, selected)
	// Different rows with different content should have high diversity
	if score < 0.5 {
		t.Errorf("expected high diversity for different rows, got %f", score)
	}
}

func TestCalculateSimilarity_SameRowAdjacentChunks(t *testing.T) {
	selector := NewMMRSelector(0.5)
	chunk1 := ScoredChunk{SourceRowID: 1, SourceColumn: "content", ChunkIndex: 0}
	chunk2 := ScoredChunk{SourceRowID: 1, SourceColumn: "content", ChunkIndex: 1}

	similarity := selector.calculateSimilarity(chunk1, chunk2)
	if similarity != 0.9 {
		t.Errorf("expected similarity=0.9 for adjacent chunks, got %f", similarity)
	}
}

func TestCalculateSimilarity_SameRowSameColumnDistant(t *testing.T) {
	selector := NewMMRSelector(0.5)
	chunk1 := ScoredChunk{SourceRowID: 1, SourceColumn: "content", ChunkIndex: 0}
	chunk2 := ScoredChunk{SourceRowID: 1, SourceColumn: "content", ChunkIndex: 5}

	similarity := selector.calculateSimilarity(chunk1, chunk2)
	if similarity != 0.6 {
		t.Errorf("expected similarity=0.6 for distant same-column chunks, got %f", similarity)
	}
}

func TestCalculateSimilarity_SameRowDifferentColumns(t *testing.T) {
	selector := NewMMRSelector(0.5)
	chunk1 := ScoredChunk{SourceRowID: 1, SourceColumn: "title"}
	chunk2 := ScoredChunk{SourceRowID: 1, SourceColumn: "content"}

	similarity := selector.calculateSimilarity(chunk1, chunk2)
	if similarity != 0.5 {
		t.Errorf("expected similarity=0.5 for different columns same row, got %f", similarity)
	}
}

func TestCalculateSimilarity_DifferentRowsUsesJaccard(t *testing.T) {
	selector := NewMMRSelector(0.5)
	chunk1 := ScoredChunk{
		SourceRowID: 1,
		Text:        "postgresql database query",
	}
	chunk2 := ScoredChunk{
		SourceRowID: 2,
		Text:        "postgresql database administration",
	}

	similarity := selector.calculateSimilarity(chunk1, chunk2)
	// Should use Jaccard, common tokens: postgresql, database
	// Set1: {postgresql, database, query}
	// Set2: {postgresql, database, administration}
	// Intersection: 2, Union: 4
	// Expected: 2/4 = 0.5
	if similarity < 0.4 || similarity > 0.6 {
		t.Errorf("expected Jaccard similarity ~0.5, got %f", similarity)
	}
}

func TestJaccardSimilarity(t *testing.T) {
	selector := NewMMRSelector(0.5)

	tests := []struct {
		name      string
		text1     string
		text2     string
		expected  float64
		tolerance float64
	}{
		{
			name:      "identical texts",
			text1:     "hello world test",
			text2:     "hello world test",
			expected:  1.0,
			tolerance: 0.01,
		},
		{
			name:      "completely different",
			text1:     "hello world",
			text2:     "foo bar baz",
			expected:  0.0,
			tolerance: 0.01,
		},
		{
			name:      "partial overlap",
			text1:     "postgresql database query",
			text2:     "postgresql database admin",
			expected:  0.5, // 2 common out of 4 unique
			tolerance: 0.01,
		},
		{
			name:      "both empty",
			text1:     "",
			text2:     "",
			expected:  1.0,
			tolerance: 0.01,
		},
		{
			name:      "one empty",
			text1:     "hello world",
			text2:     "",
			expected:  0.0,
			tolerance: 0.01,
		},
		{
			name:      "superset",
			text1:     "hello",
			text2:     "hello world test",
			expected:  0.333, // 1/3
			tolerance: 0.01,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := selector.jaccardSimilarity(tt.text1, tt.text2)
			diff := result - tt.expected
			if diff < 0 {
				diff = -diff
			}
			if diff > tt.tolerance {
				t.Errorf("expected %f (±%f), got %f", tt.expected, tt.tolerance, result)
			}
		})
	}
}

func TestAbs(t *testing.T) {
	tests := []struct {
		input    int
		expected int
	}{
		{5, 5},
		{-5, 5},
		{0, 0},
		{-100, 100},
	}

	for _, tt := range tests {
		result := abs(tt.input)
		if result != tt.expected {
			t.Errorf("abs(%d): expected %d, got %d", tt.input, tt.expected, result)
		}
	}
}

func TestMMRBalancedSelection(t *testing.T) {
	// Test with typical lambda value (0.6)
	selector := NewMMRSelector(0.6)

	chunks := []ScoredChunk{
		// High score, will be selected first
		{Text: "postgresql database performance tuning guide", Score: 1.0, SourceRowID: 1},
		// Similar to first (same topic), should be penalized
		{Text: "postgresql database optimization tips", Score: 0.95, SourceRowID: 2},
		// Different topic, might get selected for diversity
		{Text: "mysql installation instructions", Score: 0.7, SourceRowID: 3},
		// Another postgresql topic
		{Text: "postgresql backup and recovery", Score: 0.6, SourceRowID: 4},
	}

	result := selector.SelectChunks(chunks, 3)

	if len(result) != 3 {
		t.Errorf("expected 3 chunks, got %d", len(result))
	}

	// First should always be highest score
	if result[0].SourceRowID != 1 {
		t.Errorf("expected first chunk from row 1, got row %v", result[0].SourceRowID)
	}
}
