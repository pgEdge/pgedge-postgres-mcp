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
	"testing"
	"time"
)

func TestFormatTSVValue(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "nil value",
			input:    nil,
			expected: "",
		},
		{
			name:     "string value",
			input:    "hello",
			expected: "hello",
		},
		{
			name:     "string with tab",
			input:    "hello\tworld",
			expected: "hello\\tworld",
		},
		{
			name:     "string with newline",
			input:    "hello\nworld",
			expected: "hello\\nworld",
		},
		{
			name:     "string with carriage return",
			input:    "hello\rworld",
			expected: "hello\\rworld",
		},
		{
			name:     "string with multiple special chars",
			input:    "a\tb\nc\rd",
			expected: "a\\tb\\nc\\rd",
		},
		{
			name:     "integer",
			input:    42,
			expected: "42",
		},
		{
			name:     "int64",
			input:    int64(9223372036854775807),
			expected: "9223372036854775807",
		},
		{
			name:     "float64",
			input:    3.14159,
			expected: "3.14159",
		},
		{
			name:     "bool true",
			input:    true,
			expected: "true",
		},
		{
			name:     "bool false",
			input:    false,
			expected: "false",
		},
		{
			name:     "byte slice",
			input:    []byte("bytes"),
			expected: "bytes",
		},
		{
			name:     "array",
			input:    []interface{}{"a", "b", "c"},
			expected: `["a","b","c"]`,
		},
		{
			name:     "map",
			input:    map[string]interface{}{"key": "value"},
			expected: `{"key":"value"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatTSVValue(tt.input)
			if result != tt.expected {
				t.Errorf("FormatTSVValue(%v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFormatTSVValue_Time(t *testing.T) {
	// Test time formatting separately since we need to construct a specific time
	testTime := time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)
	result := FormatTSVValue(testTime)
	expected := "2024-06-15T10:30:00Z"
	if result != expected {
		t.Errorf("FormatTSVValue(time) = %q, want %q", result, expected)
	}
}

func TestFormatResultsAsTSV(t *testing.T) {
	tests := []struct {
		name        string
		columnNames []string
		results     [][]interface{}
		expected    string
	}{
		{
			name:        "empty columns",
			columnNames: []string{},
			results:     [][]interface{}{},
			expected:    "",
		},
		{
			name:        "header only (no results)",
			columnNames: []string{"id", "name", "email"},
			results:     [][]interface{}{},
			expected:    "id\tname\temail",
		},
		{
			name:        "single row",
			columnNames: []string{"id", "name"},
			results:     [][]interface{}{{1, "Alice"}},
			expected:    "id\tname\n1\tAlice",
		},
		{
			name:        "multiple rows",
			columnNames: []string{"id", "name", "active"},
			results: [][]interface{}{
				{1, "Alice", true},
				{2, "Bob", false},
			},
			expected: "id\tname\tactive\n1\tAlice\ttrue\n2\tBob\tfalse",
		},
		{
			name:        "with null values",
			columnNames: []string{"id", "name", "email"},
			results: [][]interface{}{
				{1, "Alice", nil},
				{2, nil, "bob@example.com"},
			},
			expected: "id\tname\temail\n1\tAlice\t\n2\t\tbob@example.com",
		},
		{
			name:        "with special characters",
			columnNames: []string{"id", "notes"},
			results: [][]interface{}{
				{1, "line1\nline2"},
				{2, "col1\tcol2"},
			},
			expected: "id\tnotes\n1\tline1\\nline2\n2\tcol1\\tcol2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatResultsAsTSV(tt.columnNames, tt.results)
			if result != tt.expected {
				t.Errorf("FormatResultsAsTSV() = %q, want %q", result, tt.expected)
			}
		})
	}
}
