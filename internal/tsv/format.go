/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package tsv

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
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

	// PostgreSQL pgtype types from pgx driver
	case pgtype.Numeric:
		if !val.Valid {
			return "" // NULL
		}
		if val.NaN {
			return "NaN"
		}
		if val.InfinityModifier == pgtype.Infinity {
			return "Infinity"
		}
		if val.InfinityModifier == pgtype.NegativeInfinity {
			return "-Infinity"
		}
		// Convert to string representation: Int * 10^Exp
		s = formatNumeric(val.Int, val.Exp)

	case pgtype.Float8:
		if !val.Valid {
			return ""
		}
		s = fmt.Sprintf("%v", val.Float64)

	case pgtype.Float4:
		if !val.Valid {
			return ""
		}
		s = fmt.Sprintf("%v", val.Float32)

	case pgtype.Int8:
		if !val.Valid {
			return ""
		}
		s = fmt.Sprintf("%d", val.Int64)

	case pgtype.Int4:
		if !val.Valid {
			return ""
		}
		s = fmt.Sprintf("%d", val.Int32)

	case pgtype.Int2:
		if !val.Valid {
			return ""
		}
		s = fmt.Sprintf("%d", val.Int16)

	case pgtype.Text:
		if !val.Valid {
			return ""
		}
		s = val.String

	case pgtype.Bool:
		if !val.Valid {
			return ""
		}
		if val.Bool {
			s = "true"
		} else {
			s = "false"
		}

	case pgtype.Timestamp:
		if !val.Valid {
			return ""
		}
		s = val.Time.Format(time.RFC3339)

	case pgtype.Timestamptz:
		if !val.Valid {
			return ""
		}
		s = val.Time.Format(time.RFC3339)

	case pgtype.Date:
		if !val.Valid {
			return ""
		}
		s = val.Time.Format("2006-01-02")

	case pgtype.Interval:
		if !val.Valid {
			return ""
		}
		// Format interval as ISO 8601 duration or human-readable
		s = formatInterval(val)

	case pgtype.UUID:
		if !val.Valid {
			return ""
		}
		// Format UUID as standard string
		s = fmt.Sprintf("%x-%x-%x-%x-%x",
			val.Bytes[0:4], val.Bytes[4:6], val.Bytes[6:8],
			val.Bytes[8:10], val.Bytes[10:16])

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

// formatNumeric converts a PostgreSQL numeric value (Int * 10^Exp) to string.
// Handles arbitrary precision decimals correctly.
func formatNumeric(intVal *big.Int, exp int32) string {
	if intVal == nil {
		return "0"
	}

	// Get the string representation of the integer part
	intStr := intVal.String()

	// Handle negative numbers
	negative := false
	if intStr != "" && intStr[0] == '-' {
		negative = true
		intStr = intStr[1:]
	}

	if exp >= 0 {
		// Positive exponent: append zeros
		result := intStr + strings.Repeat("0", int(exp))
		if negative {
			return "-" + result
		}
		return result
	}

	// Negative exponent: insert decimal point
	decimalPlaces := int(-exp)

	// Pad with leading zeros if necessary
	for len(intStr) <= decimalPlaces {
		intStr = "0" + intStr
	}

	// Insert decimal point
	insertPos := len(intStr) - decimalPlaces
	result := intStr[:insertPos] + "." + intStr[insertPos:]

	// Remove trailing zeros after decimal point (optional, for cleaner output)
	// result = strings.TrimRight(result, "0")
	// result = strings.TrimRight(result, ".")

	if negative {
		return "-" + result
	}
	return result
}

// formatInterval converts a PostgreSQL interval to a human-readable string.
func formatInterval(val pgtype.Interval) string {
	var parts []string

	if val.Months != 0 {
		years := val.Months / 12
		months := val.Months % 12
		if years != 0 {
			parts = append(parts, fmt.Sprintf("%d year(s)", years))
		}
		if months != 0 {
			parts = append(parts, fmt.Sprintf("%d month(s)", months))
		}
	}

	if val.Days != 0 {
		parts = append(parts, fmt.Sprintf("%d day(s)", val.Days))
	}

	if val.Microseconds != 0 {
		// Convert microseconds to hours, minutes, seconds
		totalSeconds := val.Microseconds / 1000000
		hours := totalSeconds / 3600
		minutes := (totalSeconds % 3600) / 60
		seconds := totalSeconds % 60
		microsRemainder := val.Microseconds % 1000000

		if hours != 0 {
			parts = append(parts, fmt.Sprintf("%d hour(s)", hours))
		}
		if minutes != 0 {
			parts = append(parts, fmt.Sprintf("%d minute(s)", minutes))
		}
		if seconds != 0 || microsRemainder != 0 {
			if microsRemainder != 0 {
				parts = append(parts, fmt.Sprintf("%d.%06d second(s)", seconds, microsRemainder))
			} else {
				parts = append(parts, fmt.Sprintf("%d second(s)", seconds))
			}
		}
	}

	if len(parts) == 0 {
		return "0"
	}

	return strings.Join(parts, " ")
}
