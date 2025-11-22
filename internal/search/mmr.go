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
	"fmt"
	"math"
)

// MMRSelector implements Maximal Marginal Relevance for diversity filtering
type MMRSelector struct {
	lambda float64 // Balance between relevance (1.0) and diversity (0.0)
}

// NewMMRSelector creates a new MMR selector
// lambda: 0.0 = maximum diversity, 1.0 = maximum relevance
// Typical values: 0.5-0.7
func NewMMRSelector(lambda float64) *MMRSelector {
	if lambda < 0.0 {
		lambda = 0.0
	}
	if lambda > 1.0 {
		lambda = 1.0
	}

	return &MMRSelector{
		lambda: lambda,
	}
}

// SelectChunks applies MMR to select diverse, relevant chunks
func (m *MMRSelector) SelectChunks(chunks []ScoredChunk, maxChunks int) []ScoredChunk {
	if len(chunks) == 0 {
		return chunks
	}

	if len(chunks) <= maxChunks {
		return chunks
	}

	// Normalize scores to 0-1 range for fair comparison
	maxScore := chunks[0].Score // Already sorted by BM25
	if maxScore == 0 {
		maxScore = 1.0
	}

	normalizedChunks := make([]ScoredChunk, len(chunks))
	copy(normalizedChunks, chunks)
	for i := range normalizedChunks {
		normalizedChunks[i].Score /= maxScore
	}

	// MMR iterative selection
	selected := []ScoredChunk{}
	remaining := normalizedChunks

	for len(selected) < maxChunks && len(remaining) > 0 {
		bestIdx := -1
		bestMMRScore := -math.MaxFloat64

		for i, candidate := range remaining {
			relevance := candidate.Score
			diversity := m.diversityScore(candidate, selected)

			// MMR formula: λ * relevance + (1-λ) * diversity
			mmrScore := m.lambda*relevance + (1.0-m.lambda)*diversity

			if mmrScore > bestMMRScore {
				bestMMRScore = mmrScore
				bestIdx = i
			}
		}

		if bestIdx == -1 {
			break
		}

		// Add best candidate to selected (restore original score)
		selectedChunk := remaining[bestIdx]
		selectedChunk.Score = chunks[len(selected)].Score // Restore original score
		selected = append(selected, selectedChunk)

		// Remove from remaining
		remaining = append(remaining[:bestIdx], remaining[bestIdx+1:]...)
	}

	return selected
}

// diversityScore calculates how different a candidate is from already selected chunks
// Returns a score from 0.0 (very similar) to 1.0 (very different)
func (m *MMRSelector) diversityScore(candidate ScoredChunk, selected []ScoredChunk) float64 {
	if len(selected) == 0 {
		return 1.0 // Maximum diversity when nothing selected yet
	}

	// Find minimum similarity to any selected chunk
	// (maximum diversity = minimum similarity)
	minSimilarity := 1.0

	for _, s := range selected {
		similarity := m.calculateSimilarity(candidate, s)
		if similarity < minSimilarity {
			minSimilarity = similarity
		}
	}

	// Return diversity (inverse of similarity)
	return 1.0 - minSimilarity
}

// calculateSimilarity computes similarity between two chunks
// Returns a value from 0.0 (completely different) to 1.0 (identical)
func (m *MMRSelector) calculateSimilarity(chunk1, chunk2 ScoredChunk) float64 {
	// Rule 1: Same source row ID means related content
	if fmt.Sprint(chunk1.SourceRowID) == fmt.Sprint(chunk2.SourceRowID) {
		// If same column and adjacent chunks, very similar
		if chunk1.SourceColumn == chunk2.SourceColumn {
			if abs(chunk1.ChunkIndex-chunk2.ChunkIndex) <= 1 {
				return 0.9 // Very high similarity for adjacent chunks
			}
			// Same column but distant chunks
			return 0.6
		}
		// Same row but different columns (e.g., title vs content)
		return 0.5
	}

	// Rule 2: Different rows - use token overlap (Jaccard similarity)
	return m.jaccardSimilarity(chunk1.Text, chunk2.Text)
}

// jaccardSimilarity calculates Jaccard similarity between two text chunks
// Jaccard(A, B) = |A ∩ B| / |A ∪ B|
func (m *MMRSelector) jaccardSimilarity(text1, text2 string) float64 {
	tokens1 := Tokenize(text1)
	tokens2 := Tokenize(text2)

	if len(tokens1) == 0 && len(tokens2) == 0 {
		return 1.0 // Both empty, consider identical
	}

	if len(tokens1) == 0 || len(tokens2) == 0 {
		return 0.0 // One empty, completely different
	}

	// Create sets
	set1 := make(map[string]bool)
	for _, token := range tokens1 {
		set1[token] = true
	}

	set2 := make(map[string]bool)
	for _, token := range tokens2 {
		set2[token] = true
	}

	// Calculate intersection
	intersection := 0
	for token := range set1 {
		if set2[token] {
			intersection++
		}
	}

	// Calculate union
	union := len(set1)
	for token := range set2 {
		if !set1[token] {
			union++
		}
	}

	if union == 0 {
		return 0.0
	}

	return float64(intersection) / float64(union)
}

// abs returns absolute value of an integer
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
