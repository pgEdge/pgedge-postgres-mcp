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
	"testing"
	"time"
)

func TestFormatValue(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{"nil value", nil, ""},
		{"empty string", "", ""},
		{"simple string", "hello", "hello"},
		{"string with tab", "hello\tworld", "hello\\tworld"},
		{"string with newline", "hello\nworld", "hello\\nworld"},
		{"string with carriage return", "hello\rworld", "hello\\rworld"},
		{"integer", 42, "42"},
		{"negative integer", -17, "-17"},
		{"int64", int64(9223372036854775807), "9223372036854775807"},
		{"float64", 3.14159, "3.14159"},
		{"bool true", true, "true"},
		{"bool false", false, "false"},
		{"byte slice", []byte("bytes"), "bytes"},
		{"array", []interface{}{"a", "b"}, `["a","b"]`},
		{"map", map[string]interface{}{"key": "value"}, `{"key":"value"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatValue(tt.input)
			if result != tt.expected {
				t.Errorf("FormatValue(%v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFormatValue_Time(t *testing.T) {
	testTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	result := FormatValue(testTime)
	expected := "2024-01-15T10:30:00Z"

	if result != expected {
		t.Errorf("FormatValue(time) = %q, want %q", result, expected)
	}
}

func TestFormatResults(t *testing.T) {
	columnNames := []string{"id", "name", "active"}
	results := [][]interface{}{
		{1, "Alice", true},
		{2, "Bob", false},
	}

	result := FormatResults(columnNames, results)
	expected := "id\tname\tactive\n1\tAlice\ttrue\n2\tBob\tfalse"

	if result != expected {
		t.Errorf("FormatResults() = %q, want %q", result, expected)
	}
}

func TestFormatResults_Empty(t *testing.T) {
	result := FormatResults([]string{}, nil)
	if result != "" {
		t.Errorf("FormatResults(empty) = %q, want empty string", result)
	}
}

func TestBuildRow(t *testing.T) {
	result := BuildRow("a", "b\tc", "d")
	expected := "a\tb\\tc\td"

	if result != expected {
		t.Errorf("BuildRow() = %q, want %q", result, expected)
	}
}
