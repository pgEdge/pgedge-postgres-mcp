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

// ScoredChunk represents a chunk of text with its relevance score
type ScoredChunk struct {
	SourceTable  string                 // Table name
	SourceRowID  interface{}            // Primary key or row identifier
	SourceColumn string                 // Which text column this came from
	ChunkIndex   int                    // Index of chunk within the row
	Text         string                 // Chunk text
	Score        float64                // BM25 score
	OriginalRank int                    // Original vector search rank
	RowData      map[string]interface{} // Full row data for reference
}

// VectorSearchResult represents a row from vector similarity search
type VectorSearchResult struct {
	RowData       map[string]interface{} // All row data
	Distance      float64                // Combined/weighted distance score
	VectorWeights map[string]float64     // Weight per vector column used
}

// ColumnWeight contains weighting information for a column
type ColumnWeight struct {
	ColumnName string  // Name of the column
	VectorName string  // Corresponding vector column name
	IsTitle    bool    // Detected as title column (vs content)
	Weight     float64 // Search weight (title: 0.3, content: 0.7, default: 0.5)
}

// SearchConfig contains configuration for similarity search
type SearchConfig struct {
	TopN            int     // Number of rows from vector search
	ChunkSizeTokens int     // Maximum tokens per chunk
	OverlapTokens   int     // Overlap between chunks
	Lambda          float64 // MMR diversity parameter (0=max diversity, 1=max relevance)
	MaxOutputTokens int     // Maximum total tokens to return
	DistanceMetric  string  // "cosine", "l2", or "inner_product"
}

// DefaultSearchConfig returns default configuration
func DefaultSearchConfig() SearchConfig {
	return SearchConfig{
		TopN:            10,
		ChunkSizeTokens: 100,
		OverlapTokens:   25, // 25% overlap
		Lambda:          0.6,
		MaxOutputTokens: 1000,
		DistanceMetric:  "cosine",
	}
}
