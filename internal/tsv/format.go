/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package tsv

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// FormatValue converts a value to a TSV-safe string.
// Handles NULLs, special characters, and complex types.
func FormatValue(v interface{}) string {
	if v == nil {
		return "" // NULL represented as empty string
	}

	var s string
	switch val := v.(type) {
	case string:
		s = val
	case []byte:
		s = string(val)
	case time.Time:
		s = val.Format(time.RFC3339)
	case bool:
		if val {
			s = "true"
		} else {
			s = "false"
		}
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		s = fmt.Sprintf("%d", val)
	case float32, float64:
		s = fmt.Sprintf("%v", val)
	case []interface{}, map[string]interface{}:
		// Complex types (arrays, JSON objects) - serialize to JSON
		jsonBytes, err := json.Marshal(val)
		if err != nil {
			s = fmt.Sprintf("%v", val)
		} else {
			s = string(jsonBytes)
		}
	default:
		// For any other type, use default formatting
		s = fmt.Sprintf("%v", val)
	}

	// Escape special characters that would break TSV parsing
	// Replace tabs with \t and newlines with \n (literal backslash sequences)
	s = strings.ReplaceAll(s, "\t", "\\t")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")

	return s
}

// FormatResults converts query results to TSV format.
// Returns header row followed by data rows, tab-separated.
func FormatResults(columnNames []string, results [][]interface{}) string {
	if len(columnNames) == 0 {
		return ""
	}

	var sb strings.Builder

	// Header row
	sb.WriteString(strings.Join(columnNames, "\t"))

	// Data rows
	for _, row := range results {
		sb.WriteString("\n")
		values := make([]string, len(row))
		for i, val := range row {
			values[i] = FormatValue(val)
		}
		sb.WriteString(strings.Join(values, "\t"))
	}

	return sb.String()
}

// BuildRow creates a single TSV row from string values.
// Values are escaped for TSV safety.
func BuildRow(values ...string) string {
	escaped := make([]string, len(values))
	for i, v := range values {
		escaped[i] = FormatValue(v)
	}
	return strings.Join(escaped, "\t")
}
