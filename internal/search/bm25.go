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
	"math"
	"sort"
	"strings"
	"unicode"
)

// BM25Scorer implements the BM25 ranking algorithm
type BM25Scorer struct {
	k1 float64 // Term frequency saturation parameter (typical: 1.2-2.0)
	b  float64 // Length normalization parameter (typical: 0.75)
}

// NewBM25Scorer creates a new BM25 scorer with default parameters
func NewBM25Scorer() *BM25Scorer {
	return &BM25Scorer{
		k1: 1.5,  // Standard value
		b:  0.75, // Standard value
	}
}

// Tokenize converts text to lowercase tokens (words only, no punctuation)
func Tokenize(text string) []string {
	// Convert to lowercase and split by non-alphanumeric characters
	text = strings.ToLower(text)

	words := strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})

	// Filter out very short tokens (single characters except numbers)
	var filtered []string
	for _, word := range words {
		if len(word) > 1 || unicode.IsNumber(rune(word[0])) {
			filtered = append(filtered, word)
		}
	}

	return filtered
}

// CalculateIDF computes inverse document frequency for all terms in the corpus
// Higher IDF means term is more unique/important
func CalculateIDF(documents [][]string) map[string]float64 {
	totalDocs := float64(len(documents))
	if totalDocs == 0 {
		return make(map[string]float64)
	}

	// Count documents containing each term
	docFreq := make(map[string]int)
	for _, doc := range documents {
		seen := make(map[string]bool)
		for _, token := range doc {
			if !seen[token] {
				docFreq[token]++
				seen[token] = true
			}
		}
	}

	// Calculate IDF for each term
	// IDF formula: log((N - df + 0.5) / (df + 0.5) + 1)
	idf := make(map[string]float64)
	for term, df := range docFreq {
		idf[term] = math.Log((totalDocs-float64(df)+0.5)/(float64(df)+0.5) + 1.0)
	}

	return idf
}

// Score computes BM25 score for a document given a query
func (bm *BM25Scorer) Score(
	queryTokens []string,
	docTokens []string,
	avgDocLength float64,
	idf map[string]float64,
) float64 {
	if len(queryTokens) == 0 || len(docTokens) == 0 {
		return 0.0
	}

	// Count term frequencies in document
	termFreq := make(map[string]int)
	for _, token := range docTokens {
		termFreq[token]++
	}

	docLength := float64(len(docTokens))
	score := 0.0

	// Calculate BM25 score for each query term
	for _, queryToken := range queryTokens {
		// Skip if term not in document
		tf, exists := termFreq[queryToken]
		if !exists {
			continue
		}

		// Get IDF for this term (default to 0 if not in corpus)
		termIDF := 0.0
		if idfVal, ok := idf[queryToken]; ok {
			termIDF = idfVal
		}

		// BM25 formula:
		// score += IDF(qi) * (f(qi, D) * (k1 + 1)) / (f(qi, D) + k1 * (1 - b + b * |D| / avgdl))
		numerator := float64(tf) * (bm.k1 + 1)
		denominator := float64(tf) + bm.k1*(1-bm.b+bm.b*docLength/avgDocLength)

		score += termIDF * (numerator / denominator)
	}

	return score
}

// RankChunks scores and ranks chunks using BM25
func RankChunks(chunks []ScoredChunk, queryText string) []ScoredChunk {
	if len(chunks) == 0 {
		return chunks
	}

	// Tokenize query
	queryTokens := Tokenize(queryText)
	if len(queryTokens) == 0 {
		return chunks
	}

	// Tokenize all documents and collect tokens
	documents := make([][]string, len(chunks))
	totalLength := 0

	for i, chunk := range chunks {
		tokens := Tokenize(chunk.Text)
		documents[i] = tokens
		totalLength += len(tokens)
	}

	// Calculate average document length
	avgDocLength := float64(totalLength) / float64(len(chunks))

	// Calculate IDF for all terms
	idf := CalculateIDF(documents)

	// Score each chunk
	scorer := NewBM25Scorer()
	for i := range chunks {
		chunks[i].Score = scorer.Score(queryTokens, documents[i], avgDocLength, idf)
	}

	// Sort by score (descending)
	sort.Slice(chunks, func(i, j int) bool {
		return chunks[i].Score > chunks[j].Score
	})

	return chunks
}
