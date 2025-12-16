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
	"math"
	"testing"
)

func TestNewBM25Scorer(t *testing.T) {
	scorer := NewBM25Scorer()

	if scorer.k1 != 1.5 {
		t.Errorf("expected k1=1.5, got %f", scorer.k1)
	}
	if scorer.b != 0.75 {
		t.Errorf("expected b=0.75, got %f", scorer.b)
	}
}

func TestTokenize(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "simple words",
			input:    "hello world",
			expected: []string{"hello", "world"},
		},
		{
			name:     "mixed case",
			input:    "Hello World TEST",
			expected: []string{"hello", "world", "test"},
		},
		{
			name:     "with punctuation",
			input:    "hello, world! how are you?",
			expected: []string{"hello", "world", "how", "are", "you"},
		},
		{
			name:     "with numbers",
			input:    "version 1.2.3 release",
			expected: []string{"version", "1", "2", "3", "release"},
		},
		{
			name:     "single digit numbers",
			input:    "test 5 items",
			expected: []string{"test", "5", "items"},
		},
		{
			name:     "single characters filtered",
			input:    "a b c test",
			expected: []string{"test"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:     "only punctuation",
			input:    "!@#$%",
			expected: nil,
		},
		{
			name:     "hyphenated words",
			input:    "high-performance database",
			expected: []string{"high", "performance", "database"},
		},
		{
			name:     "underscores",
			input:    "user_name column_type",
			expected: []string{"user", "name", "column", "type"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Tokenize(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d tokens, got %d: %v", len(tt.expected), len(result), result)
				return
			}
			for i, token := range result {
				if token != tt.expected[i] {
					t.Errorf("token %d: expected %q, got %q", i, tt.expected[i], token)
				}
			}
		})
	}
}

func TestCalculateIDF(t *testing.T) {
	tests := []struct {
		name      string
		documents [][]string
		checkTerm string
		checkFunc func(float64) bool
	}{
		{
			name:      "empty corpus",
			documents: [][]string{},
			checkTerm: "test",
			checkFunc: func(idf float64) bool { return idf == 0 },
		},
		{
			name: "term in all documents has low IDF",
			documents: [][]string{
				{"the", "quick", "brown"},
				{"the", "lazy", "dog"},
				{"the", "fox", "jumped"},
			},
			checkTerm: "the",
			checkFunc: func(idf float64) bool { return idf < 0.3 },
		},
		{
			name: "term in one document has high IDF",
			documents: [][]string{
				{"postgresql", "database", "query"},
				{"mysql", "database", "query"},
				{"sqlite", "database", "query"},
			},
			checkTerm: "postgresql",
			checkFunc: func(idf float64) bool { return idf > 0.5 },
		},
		{
			name: "common terms have lower IDF than rare terms",
			documents: [][]string{
				{"database", "postgresql", "unique"},
				{"database", "mysql"},
				{"database", "sqlite"},
			},
			checkTerm: "unique",
			checkFunc: func(idf float64) bool { return idf > 0.5 },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idf := CalculateIDF(tt.documents)
			value := idf[tt.checkTerm]
			if !tt.checkFunc(value) {
				t.Errorf("IDF check failed for term %q: got %f", tt.checkTerm, value)
			}
		})
	}
}

func TestBM25Score(t *testing.T) {
	scorer := NewBM25Scorer()

	tests := []struct {
		name         string
		queryTokens  []string
		docTokens    []string
		avgDocLength float64
		idf          map[string]float64
		checkFunc    func(float64) bool
	}{
		{
			name:         "empty query",
			queryTokens:  []string{},
			docTokens:    []string{"hello", "world"},
			avgDocLength: 10.0,
			idf:          map[string]float64{"hello": 1.0, "world": 1.0},
			checkFunc:    func(score float64) bool { return score == 0.0 },
		},
		{
			name:         "empty document",
			queryTokens:  []string{"hello"},
			docTokens:    []string{},
			avgDocLength: 10.0,
			idf:          map[string]float64{"hello": 1.0},
			checkFunc:    func(score float64) bool { return score == 0.0 },
		},
		{
			name:         "matching terms",
			queryTokens:  []string{"database"},
			docTokens:    []string{"postgresql", "database", "query"},
			avgDocLength: 5.0,
			idf:          map[string]float64{"database": 1.0},
			checkFunc:    func(score float64) bool { return score > 0 },
		},
		{
			name:         "no matching terms",
			queryTokens:  []string{"nosql"},
			docTokens:    []string{"postgresql", "database", "query"},
			avgDocLength: 5.0,
			idf:          map[string]float64{"nosql": 1.0},
			checkFunc:    func(score float64) bool { return score == 0.0 },
		},
		{
			name:         "multiple matching terms",
			queryTokens:  []string{"postgresql", "database"},
			docTokens:    []string{"postgresql", "database", "query"},
			avgDocLength: 5.0,
			idf:          map[string]float64{"postgresql": 1.5, "database": 0.5},
			checkFunc:    func(score float64) bool { return score > 1.0 },
		},
		{
			name:         "term frequency matters",
			queryTokens:  []string{"database"},
			docTokens:    []string{"database", "database", "database"},
			avgDocLength: 3.0,
			idf:          map[string]float64{"database": 1.0},
			checkFunc:    func(score float64) bool { return score > 0.5 },
		},
		{
			name:         "missing IDF defaults to zero",
			queryTokens:  []string{"unknown"},
			docTokens:    []string{"unknown", "term"},
			avgDocLength: 5.0,
			idf:          map[string]float64{}, // No IDF for "unknown"
			checkFunc:    func(score float64) bool { return score == 0.0 },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := scorer.Score(tt.queryTokens, tt.docTokens, tt.avgDocLength, tt.idf)
			if !tt.checkFunc(score) {
				t.Errorf("score check failed: got %f", score)
			}
		})
	}
}

func TestBM25ScoreDocumentLength(t *testing.T) {
	// Test that longer documents get penalized (with same term frequency)
	scorer := NewBM25Scorer()
	query := []string{"database"}
	idf := map[string]float64{"database": 1.0, "padding": 0.1}

	shortDoc := []string{"database"}
	longDoc := []string{"database", "padding", "padding", "padding", "padding",
		"padding", "padding", "padding", "padding", "padding"}

	avgDocLength := 5.0

	shortScore := scorer.Score(query, shortDoc, avgDocLength, idf)
	longScore := scorer.Score(query, longDoc, avgDocLength, idf)

	if shortScore <= longScore {
		t.Errorf("expected short doc (%f) to score higher than long doc (%f)",
			shortScore, longScore)
	}
}

func TestRankChunks(t *testing.T) {
	tests := []struct {
		name      string
		chunks    []ScoredChunk
		query     string
		checkFunc func([]ScoredChunk) bool
	}{
		{
			name:   "empty chunks",
			chunks: []ScoredChunk{},
			query:  "test",
			checkFunc: func(result []ScoredChunk) bool {
				return len(result) == 0
			},
		},
		{
			name: "empty query returns original chunks",
			chunks: []ScoredChunk{
				{Text: "hello world"},
			},
			query: "",
			checkFunc: func(result []ScoredChunk) bool {
				return len(result) == 1 && result[0].Score == 0
			},
		},
		{
			name: "single chunk",
			chunks: []ScoredChunk{
				{Text: "postgresql database query optimization"},
			},
			query: "postgresql optimization",
			checkFunc: func(result []ScoredChunk) bool {
				return len(result) == 1 && result[0].Score > 0
			},
		},
		{
			name: "chunks sorted by score",
			chunks: []ScoredChunk{
				{Text: "unrelated content about weather"},
				{Text: "postgresql database performance tuning"},
				{Text: "postgresql query optimization guide"},
			},
			query: "postgresql query",
			checkFunc: func(result []ScoredChunk) bool {
				// Should be sorted by score descending
				for i := 0; i < len(result)-1; i++ {
					if result[i].Score < result[i+1].Score {
						return false
					}
				}
				return true
			},
		},
		{
			name: "relevant chunk scores higher",
			chunks: []ScoredChunk{
				{Text: "weather forecast for today"},
				{Text: "postgresql database administration"},
			},
			query: "postgresql database",
			checkFunc: func(result []ScoredChunk) bool {
				// Database chunk should be first
				return result[0].Score > result[1].Score
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RankChunks(tt.chunks, tt.query)
			if !tt.checkFunc(result) {
				t.Errorf("check failed for chunks: %v", result)
			}
		})
	}
}

func TestIDFFormula(t *testing.T) {
	// Test that IDF formula produces expected values
	// IDF = log((N - df + 0.5) / (df + 0.5) + 1)

	documents := [][]string{
		{"term"},
		{"term"},
		{"term"},
		{"other"},
		{"other"},
	}

	idf := CalculateIDF(documents)

	// Term appears in 3/5 docs, should have lower IDF
	// Other appears in 2/5 docs, should have higher IDF
	termIDF := idf["term"]
	otherIDF := idf["other"]

	if termIDF >= otherIDF {
		t.Errorf("expected term IDF (%f) < other IDF (%f)", termIDF, otherIDF)
	}

	// Verify IDF is positive
	if termIDF <= 0 || otherIDF <= 0 {
		t.Errorf("IDF should be positive: term=%f, other=%f", termIDF, otherIDF)
	}

	// Verify IDF calculation manually for "term"
	// N=5, df=3
	// IDF = log((5 - 3 + 0.5) / (3 + 0.5) + 1) = log(2.5/3.5 + 1) = log(1.714...) ≈ 0.539
	expectedTermIDF := math.Log((5.0-3.0+0.5)/(3.0+0.5) + 1.0)
	if math.Abs(termIDF-expectedTermIDF) > 0.001 {
		t.Errorf("term IDF: expected %f, got %f", expectedTermIDF, termIDF)
	}
}
