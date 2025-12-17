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
	"pgedge-postgres-mcp/internal/tsv"
)

// FormatTSVValue converts a value to a TSV-safe string.
// Handles NULLs, special characters, and complex types.
// This is a wrapper around tsv.FormatValue for backward compatibility.
func FormatTSVValue(v interface{}) string {
	return tsv.FormatValue(v)
}

// FormatResultsAsTSV converts query results to TSV format.
// Returns header row followed by data rows, tab-separated.
// This is a wrapper around tsv.FormatResults for backward compatibility.
func FormatResultsAsTSV(columnNames []string, results [][]interface{}) string {
	return tsv.FormatResults(columnNames, results)
}

// BuildTSVRow creates a single TSV row from string values.
// Values are escaped for TSV safety.
// This is a wrapper around tsv.BuildRow for backward compatibility.
func BuildTSVRow(values ...string) string {
	return tsv.BuildRow(values...)
}
