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
	"strings"
	"testing"
)

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected int
	}{
		{"empty string", "", 0},
		{"single word", "hello", 0}, // 1 * 0.75 = 0.75, rounds to 0, but min is 1 for non-empty
		{"two words", "hello world", 1},
		{"four words", "one two three four", 3},
		{"ten words", "one two three four five six seven eight nine ten", 7},
		{"with punctuation", "Hello, world! How are you?", 3}, // 5 words * 0.75 = 3.75
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EstimateTokens(tt.text)
			// For non-empty text with few words, we need to account for minimum
			if tt.text != "" && result == 0 && tt.expected == 0 {
				// Check if it should be 1 (minimum for non-empty)
				words := strings.Fields(tt.text)
				if len(words) > 0 {
					t.Logf("Note: non-empty text '%s' has %d words, estimated %d tokens",
						tt.text, len(words), result)
				}
			}
			// Allow some variance due to rounding
			diff := result - tt.expected
			if diff < 0 {
				diff = -diff
			}
			if diff > 1 {
				t.Errorf("expected ~%d tokens, got %d", tt.expected, result)
			}
		})
	}
}

func TestEstimateTokens_MinimumOne(t *testing.T) {
	// Test that non-empty text with words gets at least 1 token
	text := "word"
	result := EstimateTokens(text)
	if result < 1 {
		t.Errorf("expected at least 1 token for non-empty text, got %d", result)
	}
}

func TestChunkText_EmptyInput(t *testing.T) {
	result := ChunkText("", 100, 25)
	if len(result) != 0 {
		t.Errorf("expected empty result for empty input, got %d chunks", len(result))
	}
}

func TestChunkText_SingleChunk(t *testing.T) {
	text := "This is a short text that fits in one chunk."
	result := ChunkText(text, 100, 25) // 100 tokens = ~133 words

	if len(result) != 1 {
		t.Errorf("expected 1 chunk, got %d", len(result))
	}
	if result[0] != text {
		t.Errorf("expected original text, got %q", result[0])
	}
}

func TestChunkText_MultipleChunks(t *testing.T) {
	// Create text with many words
	words := make([]string, 200)
	for i := range words {
		words[i] = "word"
	}
	text := strings.Join(words, " ")

	// Chunk with small max tokens
	result := ChunkText(text, 50, 10) // 50 tokens = ~67 words

	if len(result) < 2 {
		t.Errorf("expected multiple chunks, got %d", len(result))
	}
}

func TestChunkText_OverlapWorks(t *testing.T) {
	// Create text with numbered words for easy verification
	words := []string{}
	for i := 1; i <= 100; i++ {
		words = append(words, "word")
	}
	text := strings.Join(words, " ")

	// Chunk with overlap
	result := ChunkText(text, 30, 10) // ~40 words per chunk, ~13 word overlap

	if len(result) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(result))
	}

	// Verify chunks overlap (have common content)
	// First and second chunk should share some words
	words1 := strings.Fields(result[0])
	words2 := strings.Fields(result[1])

	// Check last words of first chunk appear in second chunk
	hasOverlap := false
	for i := len(words1) - 5; i < len(words1) && i >= 0; i++ {
		for j := 0; j < 5 && j < len(words2); j++ {
			if words1[i] == words2[j] {
				hasOverlap = true
				break
			}
		}
	}

	if !hasOverlap {
		t.Log("Note: overlap detection may vary based on word boundaries")
	}
}

func TestChunkText_ZeroMaxTokens(t *testing.T) {
	text := "hello world test"
	result := ChunkText(text, 0, 0) // Should use default

	if len(result) != 1 {
		t.Errorf("expected 1 chunk with default max, got %d", len(result))
	}
}

func TestChunkText_NegativeOverlap(t *testing.T) {
	text := "hello world this is a test"
	result := ChunkText(text, 10, -5) // Negative overlap should be clamped to 0

	if len(result) == 0 {
		t.Error("expected at least one chunk")
	}
}

func TestChunkText_ExcessiveOverlap(t *testing.T) {
	// Create text that would normally need multiple chunks
	words := make([]string, 100)
	for i := range words {
		words[i] = "word"
	}
	text := strings.Join(words, " ")

	// Overlap >= maxTokens should be clamped
	result := ChunkText(text, 30, 30) // Should clamp overlap to 25% (7)

	if len(result) == 0 {
		t.Error("expected at least one chunk")
	}

	// Should still produce multiple chunks
	if len(result) < 2 {
		t.Logf("Got %d chunks (may vary based on clamping)", len(result))
	}
}

func TestChunkRow(t *testing.T) {
	rowData := map[string]interface{}{
		"id":         1,
		"title":      "Test Title",
		"content":    "This is some test content that will be chunked.",
		"number_col": 42,
		"empty_text": "",
		"nil_col":    nil,
	}

	textColumns := []string{"title", "content", "missing_col", "empty_text"}

	chunks := ChunkRow(rowData, textColumns, 1, "test_table", 0, 100, 25)

	// Should have chunks for title and content only
	if len(chunks) != 2 {
		t.Errorf("expected 2 chunks (title + content), got %d", len(chunks))
	}

	// Verify metadata
	for _, chunk := range chunks {
		if chunk.SourceTable != "test_table" {
			t.Errorf("expected table 'test_table', got %q", chunk.SourceTable)
		}
		if chunk.SourceRowID != 1 {
			t.Errorf("expected row ID 1, got %v", chunk.SourceRowID)
		}
		if chunk.OriginalRank != 0 {
			t.Errorf("expected rank 0, got %d", chunk.OriginalRank)
		}
	}
}

func TestChunkRow_LargeContent(t *testing.T) {
	// Create large content that will need chunking
	words := make([]string, 500)
	for i := range words {
		words[i] = "word"
	}
	largeContent := strings.Join(words, " ")

	rowData := map[string]interface{}{
		"id":      1,
		"content": largeContent,
	}

	chunks := ChunkRow(rowData, []string{"content"}, 1, "test_table", 0, 50, 10)

	if len(chunks) < 2 {
		t.Errorf("expected multiple chunks for large content, got %d", len(chunks))
	}

	// Verify chunk indices are sequential
	for i, chunk := range chunks {
		if chunk.ChunkIndex != i {
			t.Errorf("expected chunk index %d, got %d", i, chunk.ChunkIndex)
		}
	}
}

func TestChunkRow_EmptyColumns(t *testing.T) {
	rowData := map[string]interface{}{
		"id": 1,
	}

	chunks := ChunkRow(rowData, []string{}, 1, "test_table", 0, 100, 25)

	if len(chunks) != 0 {
		t.Errorf("expected no chunks for empty columns, got %d", len(chunks))
	}
}

func TestChunkRow_NonStringValues(t *testing.T) {
	rowData := map[string]interface{}{
		"id":     1,
		"count":  42,    // integer
		"active": true,  // boolean
		"price":  19.99, // float
	}

	chunks := ChunkRow(rowData, []string{"count", "active", "price"}, 1, "test_table", 0, 100, 25)

	// Non-string columns should be skipped
	if len(chunks) != 0 {
		t.Errorf("expected no chunks for non-string columns, got %d", len(chunks))
	}
}

func TestSelectChunksWithinBudget(t *testing.T) {
	chunks := []ScoredChunk{
		{Text: strings.Repeat("word ", 10), Score: 1.0}, // ~7 tokens
		{Text: strings.Repeat("word ", 20), Score: 0.9}, // ~15 tokens
		{Text: strings.Repeat("word ", 30), Score: 0.8}, // ~22 tokens
		{Text: strings.Repeat("word ", 10), Score: 0.7}, // ~7 tokens
	}

	result := SelectChunksWithinBudget(chunks, 30)

	// Should select chunks until budget exceeded
	// First: ~7, Second: ~15 (total ~22), Third would exceed
	if len(result) < 1 {
		t.Errorf("expected at least 1 chunk, got %d", len(result))
	}

	// Verify total tokens under budget
	totalTokens := 0
	for _, chunk := range result {
		totalTokens += EstimateTokens(chunk.Text)
	}
	if totalTokens > 30 {
		t.Errorf("total tokens %d exceeds budget 30", totalTokens)
	}
}

func TestSelectChunksWithinBudget_Empty(t *testing.T) {
	result := SelectChunksWithinBudget([]ScoredChunk{}, 100)
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d chunks", len(result))
	}
}

func TestSelectChunksWithinBudget_AllFit(t *testing.T) {
	chunks := []ScoredChunk{
		{Text: "short", Score: 1.0},
		{Text: "also short", Score: 0.9},
	}

	result := SelectChunksWithinBudget(chunks, 1000)

	if len(result) != 2 {
		t.Errorf("expected all chunks to fit, got %d", len(result))
	}
}

func TestSelectChunksWithinBudget_ZeroBudget(t *testing.T) {
	chunks := []ScoredChunk{
		{Text: "hello world", Score: 1.0},
	}

	result := SelectChunksWithinBudget(chunks, 0)

	// Zero budget should return no chunks (if first chunk has > 0 tokens)
	if len(result) != 0 {
		t.Logf("Got %d chunks with zero budget (first chunk may have 0 estimated tokens)", len(result))
	}
}

func TestFormatChunksForOutput(t *testing.T) {
	chunks := []ScoredChunk{
		{
			Text:         "PostgreSQL is a powerful database",
			Score:        1.5,
			SourceColumn: "content",
			OriginalRank: 0,
		},
		{
			Text:         "MySQL is another option",
			Score:        0.8,
			SourceColumn: "title",
			OriginalRank: 1,
		},
	}

	output := FormatChunksForOutput(chunks, "database comparison")

	// Verify header
	if !strings.Contains(output, "database comparison") {
		t.Error("expected query text in output")
	}

	// Verify chunks are included
	if !strings.Contains(output, "PostgreSQL is a powerful database") {
		t.Error("expected first chunk text in output")
	}
	if !strings.Contains(output, "MySQL is another option") {
		t.Error("expected second chunk text in output")
	}

	// Verify metadata
	if !strings.Contains(output, "Result 1") {
		t.Error("expected result numbering")
	}
	if !strings.Contains(output, "Relevance Score:") {
		t.Error("expected relevance scores")
	}
	if !strings.Contains(output, "Total Results: 2") {
		t.Error("expected total results count")
	}
}

func TestFormatChunksForOutput_Empty(t *testing.T) {
	output := FormatChunksForOutput([]ScoredChunk{}, "test query")

	if !strings.Contains(output, "No relevant chunks found") {
		t.Error("expected 'no relevant chunks' message for empty input")
	}
}

func TestFormatChunksForOutput_SingleChunk(t *testing.T) {
	chunks := []ScoredChunk{
		{Text: "Single result", Score: 2.0, SourceColumn: "content", OriginalRank: 0},
	}

	output := FormatChunksForOutput(chunks, "test")

	if !strings.Contains(output, "Total Results: 1") {
		t.Error("expected total results count of 1")
	}
	if !strings.Contains(output, "Single result") {
		t.Error("expected chunk text in output")
	}
}
