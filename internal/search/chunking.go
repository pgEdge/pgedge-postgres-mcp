/*-------------------------------------------------------------------------
 *
 * pgEdge Postgres MCP Server
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package search

import (
	"fmt"
	"strings"
)

// EstimateTokens approximates token count using: tokens ≈ words * 0.75
// This is a fast approximation suitable for chunking without a full tokenizer
func EstimateTokens(text string) int {
	if text == "" {
		return 0
	}

	// Count words
	words := strings.Fields(text)
	wordCount := len(words)

	// Apply approximation factor
	tokenCount := int(float64(wordCount) * 0.75)

	// Ensure minimum of 1 token for non-empty text
	if tokenCount == 0 && wordCount > 0 {
		tokenCount = 1
	}

	return tokenCount
}

// ChunkText splits text into overlapping chunks based on token limits
// Returns array of chunk texts
func ChunkText(text string, maxTokens int, overlapTokens int) []string {
	if text == "" {
		return []string{}
	}

	// Convert tokens to words for chunking
	maxWords := int(float64(maxTokens) / 0.75)
	overlapWords := int(float64(overlapTokens) / 0.75)

	if maxWords <= 0 {
		maxWords = 100 // Default fallback
	}

	if overlapWords < 0 {
		overlapWords = 0
	}

	// Ensure overlap is less than chunk size
	if overlapWords >= maxWords {
		overlapWords = maxWords / 4 // 25% overlap
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{}
	}

	// If text fits in one chunk, return as-is
	if len(words) <= maxWords {
		return []string{text}
	}

	var chunks []string
	stepSize := maxWords - overlapWords

	if stepSize <= 0 {
		stepSize = 1 // Prevent infinite loop
	}

	for i := 0; i < len(words); i += stepSize {
		end := i + maxWords
		if end > len(words) {
			end = len(words)
		}

		chunk := strings.Join(words[i:end], " ")
		chunks = append(chunks, chunk)

		// If this is the last chunk, break
		if end >= len(words) {
			break
		}
	}

	return chunks
}

// ChunkRow processes all text columns in a row and returns chunks with metadata
func ChunkRow(
	rowData map[string]interface{},
	textColumns []string,
	rowID interface{},
	tableName string,
	rank int,
	maxTokens int,
	overlapTokens int,
) []ScoredChunk {
	var allChunks []ScoredChunk

	for _, colName := range textColumns {
		// Get text from column
		colValue, ok := rowData[colName]
		if !ok {
			continue
		}

		text, ok := colValue.(string)
		if !ok || text == "" {
			continue
		}

		// Chunk the text
		chunks := ChunkText(text, maxTokens, overlapTokens)

		// Create ScoredChunk for each chunk
		for i, chunkText := range chunks {
			chunk := ScoredChunk{
				SourceTable:  tableName,
				SourceRowID:  rowID,
				SourceColumn: colName,
				ChunkIndex:   i,
				Text:         chunkText,
				Score:        0.0, // Will be set by BM25 scoring
				OriginalRank: rank,
				RowData:      rowData,
			}
			allChunks = append(allChunks, chunk)
		}
	}

	return allChunks
}

// SelectChunksWithinBudget selects chunks up to the token limit
func SelectChunksWithinBudget(chunks []ScoredChunk, maxTokens int) []ScoredChunk {
	var selected []ScoredChunk
	totalTokens := 0

	for _, chunk := range chunks {
		chunkTokens := EstimateTokens(chunk.Text)

		// Check if adding this chunk would exceed the budget
		if totalTokens+chunkTokens > maxTokens {
			break
		}

		selected = append(selected, chunk)
		totalTokens += chunkTokens
	}

	return selected
}

// FormatChunksForOutput formats chunks into a readable string for LLM consumption
func FormatChunksForOutput(chunks []ScoredChunk, queryText string) string {
	if len(chunks) == 0 {
		return "No relevant chunks found."
	}

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Search Results for: %q\n", queryText))
	sb.WriteString(strings.Repeat("=", 80))
	sb.WriteString("\n\n")

	totalTokens := 0
	for i, chunk := range chunks {
		sb.WriteString(fmt.Sprintf("--- Result %d ---\n", i+1))
		sb.WriteString(fmt.Sprintf("Source: %s (row rank: %d)\n", chunk.SourceColumn, chunk.OriginalRank+1))
		sb.WriteString(fmt.Sprintf("Relevance Score: %.3f\n\n", chunk.Score))
		sb.WriteString(chunk.Text)
		sb.WriteString("\n\n")

		totalTokens += EstimateTokens(chunk.Text)
	}

	sb.WriteString(strings.Repeat("=", 80))
	sb.WriteString(fmt.Sprintf("\nTotal Results: %d chunks (~%d tokens)\n", len(chunks), totalTokens))

	return sb.String()
}
