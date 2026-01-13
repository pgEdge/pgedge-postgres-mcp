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
	"math/big"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
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

func TestFormatValue_PgTypeNumeric(t *testing.T) {
	tests := []struct {
		name     string
		input    pgtype.Numeric
		expected string
	}{
		{
			name:     "null numeric",
			input:    pgtype.Numeric{Valid: false},
			expected: "",
		},
		{
			name:     "NaN",
			input:    pgtype.Numeric{Valid: true, NaN: true},
			expected: "NaN",
		},
		{
			name:     "positive infinity",
			input:    pgtype.Numeric{Valid: true, InfinityModifier: pgtype.Infinity},
			expected: "Infinity",
		},
		{
			name:     "negative infinity",
			input:    pgtype.Numeric{Valid: true, InfinityModifier: pgtype.NegativeInfinity},
			expected: "-Infinity",
		},
		{
			name:     "integer 12345",
			input:    pgtype.Numeric{Valid: true, Int: big.NewInt(12345), Exp: 0},
			expected: "12345",
		},
		{
			name:     "decimal 123.45 (Int=12345, Exp=-2)",
			input:    pgtype.Numeric{Valid: true, Int: big.NewInt(12345), Exp: -2},
			expected: "123.45",
		},
		{
			name:     "decimal 0.00123 (Int=123, Exp=-5)",
			input:    pgtype.Numeric{Valid: true, Int: big.NewInt(123), Exp: -5},
			expected: "0.00123",
		},
		{
			name:     "large number with positive exp (Int=5, Exp=3)",
			input:    pgtype.Numeric{Valid: true, Int: big.NewInt(5), Exp: 3},
			expected: "5000",
		},
		{
			name:     "negative decimal -99.99",
			input:    pgtype.Numeric{Valid: true, Int: big.NewInt(-9999), Exp: -2},
			expected: "-99.99",
		},
		{
			name:     "zero",
			input:    pgtype.Numeric{Valid: true, Int: big.NewInt(0), Exp: 0},
			expected: "0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatValue(tt.input)
			if result != tt.expected {
				t.Errorf("FormatValue(%+v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFormatValue_PgTypeInt(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "Int8 valid",
			input:    pgtype.Int8{Int64: 9223372036854775807, Valid: true},
			expected: "9223372036854775807",
		},
		{
			name:     "Int8 null",
			input:    pgtype.Int8{Valid: false},
			expected: "",
		},
		{
			name:     "Int4 valid",
			input:    pgtype.Int4{Int32: 2147483647, Valid: true},
			expected: "2147483647",
		},
		{
			name:     "Int2 valid",
			input:    pgtype.Int2{Int16: 32767, Valid: true},
			expected: "32767",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatValue(tt.input)
			if result != tt.expected {
				t.Errorf("FormatValue(%+v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFormatValue_PgTypeTimestamp(t *testing.T) {
	testTime := time.Date(2024, 6, 15, 14, 30, 45, 0, time.UTC)

	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "Timestamp valid",
			input:    pgtype.Timestamp{Time: testTime, Valid: true},
			expected: "2024-06-15T14:30:45Z",
		},
		{
			name:     "Timestamp null",
			input:    pgtype.Timestamp{Valid: false},
			expected: "",
		},
		{
			name:     "Timestamptz valid",
			input:    pgtype.Timestamptz{Time: testTime, Valid: true},
			expected: "2024-06-15T14:30:45Z",
		},
		{
			name:     "Date valid",
			input:    pgtype.Date{Time: testTime, Valid: true},
			expected: "2024-06-15",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatValue(tt.input)
			if result != tt.expected {
				t.Errorf("FormatValue(%+v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFormatValue_PgTypeUUID(t *testing.T) {
	uuid := pgtype.UUID{
		Bytes: [16]byte{0x55, 0x0e, 0x84, 0x00, 0xe2, 0x9b, 0x41, 0xd4, 0xa7, 0x16, 0x44, 0x66, 0x55, 0x44, 0x00, 0x00},
		Valid: true,
	}

	result := FormatValue(uuid)
	expected := "550e8400-e29b-41d4-a716-446655440000"

	if result != expected {
		t.Errorf("FormatValue(UUID) = %q, want %q", result, expected)
	}

	// Test null UUID
	nullUUID := pgtype.UUID{Valid: false}
	if result := FormatValue(nullUUID); result != "" {
		t.Errorf("FormatValue(null UUID) = %q, want empty string", result)
	}
}

func TestFormatValue_PgTypeInterval(t *testing.T) {
	tests := []struct {
		name     string
		input    pgtype.Interval
		expected string
	}{
		{
			name:     "null interval",
			input:    pgtype.Interval{Valid: false},
			expected: "",
		},
		{
			name:     "zero interval",
			input:    pgtype.Interval{Valid: true},
			expected: "0",
		},
		{
			name:     "1 day",
			input:    pgtype.Interval{Days: 1, Valid: true},
			expected: "1 day(s)",
		},
		{
			name:     "2 hours",
			input:    pgtype.Interval{Microseconds: 2 * 3600 * 1000000, Valid: true},
			expected: "2 hour(s)",
		},
		{
			name:     "1 year 2 months",
			input:    pgtype.Interval{Months: 14, Valid: true},
			expected: "1 year(s) 2 month(s)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatValue(tt.input)
			if result != tt.expected {
				t.Errorf("FormatValue(%+v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
