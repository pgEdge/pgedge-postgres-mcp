/*-------------------------------------------------------------------------
*
 * pgEdge Natural Language Agent
*
* Portions copyright (c) 2025, pgEdge, Inc.
* This software is released under The PostgreSQL License
*
*-------------------------------------------------------------------------
*/

package logging

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
)

func TestLogLevels(t *testing.T) {
	tests := []struct {
		level    LogLevel
		expected string
	}{
		{LevelDebug, "DEBUG"},
		{LevelInfo, "INFO"},
		{LevelWarn, "WARN"},
		{LevelError, "ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.level.String(); got != tt.expected {
				t.Errorf("LogLevel.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSetAndGetLevel(t *testing.T) {
	original := GetLevel()
	defer SetLevel(original)

	tests := []struct {
		name  string
		level LogLevel
	}{
		{"Debug", LevelDebug},
		{"Info", LevelInfo},
		{"Warn", LevelWarn},
		{"Error", LevelError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetLevel(tt.level)
			if got := GetLevel(); got != tt.level {
				t.Errorf("GetLevel() = %v, want %v", got, tt.level)
			}
		})
	}
}

func TestLogOutput(t *testing.T) {
	// Save original stderr
	originalStderr := os.Stderr

	// Create a pipe to capture stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w

	// Set log level to debug to ensure output
	originalLevel := GetLevel()
	SetLevel(LevelDebug)
	defer func() {
		SetLevel(originalLevel)
		os.Stderr = originalStderr
	}()

	// Log a test message
	Info("test message", "key1", "value1", "key2", 42)

	// Close writer and read output
	w.Close()
	output, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}

	// Parse JSON output
	var entry logEntry
	if err := json.Unmarshal(output, &entry); err != nil {
		t.Fatalf("Failed to parse log output as JSON: %v\nOutput: %s", err, string(output))
	}

	// Verify log entry
	if entry.Level != "INFO" {
		t.Errorf("Level = %v, want INFO", entry.Level)
	}
	if entry.Message != "test message" {
		t.Errorf("Message = %v, want 'test message'", entry.Message)
	}
	if entry.Fields["key1"] != "value1" {
		t.Errorf("Fields[key1] = %v, want 'value1'", entry.Fields["key1"])
	}
	if entry.Fields["key2"] != float64(42) {
		t.Errorf("Fields[key2] = %v, want 42", entry.Fields["key2"])
	}
	if entry.Timestamp == "" {
		t.Error("Timestamp is empty")
	}
}

func TestLogLevelFiltering(t *testing.T) {
	// Save original stderr
	originalStderr := os.Stderr

	// Set log level to WARN
	originalLevel := GetLevel()
	SetLevel(LevelWarn)
	defer func() {
		SetLevel(originalLevel)
		os.Stderr = originalStderr
	}()

	t.Run("Debug below threshold", func(t *testing.T) {
		r, w, _ := os.Pipe()
		os.Stderr = w

		Debug("debug message")

		w.Close()
		output, _ := io.ReadAll(r)

		if len(output) > 0 {
			t.Error("Debug message should not be logged when level is WARN")
		}
	})

	t.Run("Info below threshold", func(t *testing.T) {
		r, w, _ := os.Pipe()
		os.Stderr = w

		Info("info message")

		w.Close()
		output, _ := io.ReadAll(r)

		if len(output) > 0 {
			t.Error("Info message should not be logged when level is WARN")
		}
	})

	t.Run("Warn at threshold", func(t *testing.T) {
		r, w, _ := os.Pipe()
		os.Stderr = w

		Warn("warn message")

		w.Close()
		output, _ := io.ReadAll(r)

		if len(output) == 0 {
			t.Error("Warn message should be logged when level is WARN")
		}

		if !strings.Contains(string(output), "WARN") {
			t.Error("Output should contain WARN level")
		}
	})

	t.Run("Error above threshold", func(t *testing.T) {
		r, w, _ := os.Pipe()
		os.Stderr = w

		Error("error message")

		w.Close()
		output, _ := io.ReadAll(r)

		if len(output) == 0 {
			t.Error("Error message should be logged when level is WARN")
		}

		if !strings.Contains(string(output), "ERROR") {
			t.Error("Output should contain ERROR level")
		}
	})
}

func TestLogWithMultipleKeyValuePairs(t *testing.T) {
	// Save original stderr
	originalStderr := os.Stderr

	// Create a pipe to capture stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	// Set log level to debug
	originalLevel := GetLevel()
	SetLevel(LevelDebug)
	defer func() {
		SetLevel(originalLevel)
		os.Stderr = originalStderr
	}()

	// Log with multiple key-value pairs
	Info("multi-field message",
		"table", "users",
		"rows", 100,
		"duration_ms", 45.6,
		"success", true,
	)

	// Close writer and read output
	w.Close()
	output, _ := io.ReadAll(r)

	// Parse JSON output
	var entry logEntry
	if err := json.Unmarshal(output, &entry); err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	// Verify all fields
	expectedFields := map[string]interface{}{
		"table":       "users",
		"rows":        float64(100),
		"duration_ms": 45.6,
		"success":     true,
	}

	for key, expectedValue := range expectedFields {
		if entry.Fields[key] != expectedValue {
			t.Errorf("Fields[%s] = %v, want %v", key, entry.Fields[key], expectedValue)
		}
	}
}

func TestLogWithOddNumberOfKeyValues(t *testing.T) {
	// Save original stderr
	originalStderr := os.Stderr

	// Create a pipe to capture stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	// Set log level to debug
	originalLevel := GetLevel()
	SetLevel(LevelDebug)
	defer func() {
		SetLevel(originalLevel)
		os.Stderr = originalStderr
	}()

	// Log with odd number of key-value pairs (last key has no value)
	Info("odd-pairs message", "key1", "value1", "key2")

	// Close writer and read output
	w.Close()
	output, _ := io.ReadAll(r)

	// Parse JSON output
	var entry logEntry
	if err := json.Unmarshal(output, &entry); err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	// Verify that key1 is present but key2 is not (no value)
	if entry.Fields["key1"] != "value1" {
		t.Errorf("Fields[key1] = %v, want 'value1'", entry.Fields["key1"])
	}
	if _, exists := entry.Fields["key2"]; exists {
		t.Error("key2 should not exist without a value")
	}
}
