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

	"pgedge-postgres-mcp/internal/database"
)

// DetectColumnTypes analyzes columns to determine if they're titles or content
// and assigns appropriate weights for search
func DetectColumnTypes(tableInfo database.TableInfo, sampleData map[string]string) []ColumnWeight {
	var weights []ColumnWeight

	// Find vector columns and their corresponding text columns
	vectorCols := make(map[string]database.ColumnInfo) // map[textCol]vectorCol
	for _, col := range tableInfo.Columns {
		if col.IsVectorColumn {
			// Try to find corresponding text column
			textColName := inferTextColumnName(col.ColumnName)
			vectorCols[textColName] = col
		}
	}

	// Analyze each text column
	for _, col := range tableInfo.Columns {
		// Skip non-text columns and vector columns
		if col.IsVectorColumn {
			continue
		}
		if !isTextColumn(col.DataType) {
			continue
		}

		// Check if this text column has a corresponding vector column
		vectorCol, hasVector := vectorCols[col.ColumnName]
		if !hasVector {
			continue
		}

		weight := ColumnWeight{
			ColumnName: col.ColumnName,
			VectorName: vectorCol.ColumnName,
			IsTitle:    false,
			Weight:     0.5, // Default
		}

		// Rule 1: Check column name
		lowerName := strings.ToLower(col.ColumnName)
		if isTitleName(lowerName) {
			weight.IsTitle = true
			weight.Weight = 0.3
		} else if isContentName(lowerName) {
			weight.IsTitle = false
			weight.Weight = 0.7
		}

		// Rule 2: Check description from pg_description
		if col.Description != "" {
			lowerDesc := strings.ToLower(col.Description)
			if strings.Contains(lowerDesc, "title") || strings.Contains(lowerDesc, "heading") || strings.Contains(lowerDesc, "name") {
				weight.IsTitle = true
				weight.Weight = 0.3
			}
		}

		// Rule 3: Check sample data length (if available)
		if sampleText, ok := sampleData[col.ColumnName]; ok {
			avgLength := len(sampleText)
			if avgLength < 100 && avgLength > 0 {
				// Short text, likely a title
				weight.IsTitle = true
				weight.Weight = 0.3
			} else if avgLength > 500 {
				// Long text, likely content
				weight.IsTitle = false
				weight.Weight = 0.7
			}
		}

		weights = append(weights, weight)
	}

	// Normalize weights if we only have one column
	if len(weights) == 1 {
		weights[0].Weight = 1.0
	} else if len(weights) > 1 {
		// Ensure weights sum to a reasonable value
		// If we have both title and content, weights should roughly sum to 1.0
		totalWeight := 0.0
		for _, w := range weights {
			totalWeight += w.Weight
		}
		if totalWeight > 0 {
			for i := range weights {
				weights[i].Weight /= totalWeight
			}
		}
	}

	return weights
}

// inferTextColumnName removes common vector suffixes to find text column
// Example: "title_embedding" → "title", "content_vector" → "content"
func inferTextColumnName(vectorColName string) string {
	name := vectorColName

	// Remove common suffixes
	suffixes := []string{
		"_embedding",
		"_embeddings",
		"_vector",
		"_vectors",
		"_emb",
		"embedding",
		"vector",
	}

	for _, suffix := range suffixes {
		if strings.HasSuffix(strings.ToLower(name), suffix) {
			name = name[:len(name)-len(suffix)]
			break
		}
	}

	return strings.TrimSuffix(name, "_")
}

// isTitleName checks if a column name suggests it contains titles
func isTitleName(name string) bool {
	titleIndicators := []string{"title", "name", "heading", "header", "subject", "label"}
	for _, indicator := range titleIndicators {
		if strings.Contains(name, indicator) {
			return true
		}
	}
	return false
}

// isContentName checks if a column name suggests it contains content/body text
func isContentName(name string) bool {
	contentIndicators := []string{"content", "text", "body", "description", "detail", "article", "document", "passage"}
	for _, indicator := range contentIndicators {
		if strings.Contains(name, indicator) {
			return true
		}
	}
	return false
}

// isTextColumn checks if a data type represents text
func isTextColumn(dataType string) bool {
	textTypes := []string{
		"text",
		"character varying",
		"varchar",
		"character",
		"char",
		"string",
	}

	lowerType := strings.ToLower(dataType)
	for _, textType := range textTypes {
		if strings.Contains(lowerType, textType) {
			return true
		}
	}
	return false
}

// CalculateWeightedDistance combines distance scores using column weights
func CalculateWeightedDistance(distances map[string]float64, weights []ColumnWeight) float64 {
	if len(weights) == 0 {
		return 0.0
	}

	totalScore := 0.0
	for _, weight := range weights {
		if dist, ok := distances[weight.VectorName]; ok {
			// Weight the distance
			totalScore += dist * weight.Weight
		}
	}

	return totalScore
}
