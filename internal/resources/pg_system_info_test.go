/*-------------------------------------------------------------------------
 *
 * pgEdge Postgres MCP Server
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package resources

import (
	"strings"
	"testing"
)

func TestParseVersionString(t *testing.T) {
	tests := []struct {
		name              string
		fullVersion       string
		version           string
		versionNumber     string
		expectedOS        string
		expectedArch      string
		expectedCompiler  string
		expectedBitVer    string
	}{
		{
			name:             "Linux x86_64 with GCC",
			fullVersion:      "PostgreSQL 15.4 on x86_64-pc-linux-gnu, compiled by gcc (GCC) 11.2.0, 64-bit",
			version:          "15.4",
			versionNumber:    "150004",
			expectedOS:       "linux",
			expectedArch:     "x86_64-pc-linux-gnu",
			expectedCompiler: "gcc (GCC) 11.2.0",
			expectedBitVer:   "64-bit",
		},
		{
			name:             "macOS ARM64",
			fullVersion:      "PostgreSQL 16.0 on aarch64-apple-darwin23.0.0, compiled by Apple clang version 15.0.0, 64-bit",
			version:          "16.0",
			versionNumber:    "160000",
			expectedOS:       "darwin23.0.0",
			expectedArch:     "aarch64-apple-darwin23.0.0",
			expectedCompiler: "Apple clang version 15.0.0",
			expectedBitVer:   "64-bit",
		},
		{
			name:             "Windows x64",
			fullVersion:      "PostgreSQL 14.5 on x86_64-w64-mingw32, compiled by gcc (x86_64-w64-mingw32-gcc) 10.3.0, 64-bit",
			version:          "14.5",
			versionNumber:    "140005",
			expectedOS:       "mingw32",
			expectedArch:     "x86_64-w64-mingw32",
			expectedCompiler: "gcc (x86_64-w64-mingw32-gcc) 10.3.0",
			expectedBitVer:   "64-bit",
		},
		{
			name:             "Older PostgreSQL version",
			fullVersion:      "PostgreSQL 12.17 on x86_64-pc-linux-gnu, compiled by gcc (GCC) 9.3.0, 64-bit",
			version:          "12.17",
			versionNumber:    "120017",
			expectedOS:       "linux",
			expectedArch:     "x86_64-pc-linux-gnu",
			expectedCompiler: "gcc (GCC) 9.3.0",
			expectedBitVer:   "64-bit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseVersionString(tt.fullVersion, tt.version, tt.versionNumber)

			if result.PostgreSQLVersion != tt.version {
				t.Errorf("PostgreSQLVersion = %v, want %v", result.PostgreSQLVersion, tt.version)
			}

			if result.VersionNumber != tt.versionNumber {
				t.Errorf("VersionNumber = %v, want %v", result.VersionNumber, tt.versionNumber)
			}

			if result.FullVersion != tt.fullVersion {
				t.Errorf("FullVersion = %v, want %v", result.FullVersion, tt.fullVersion)
			}

			if result.OperatingSystem != tt.expectedOS {
				t.Errorf("OperatingSystem = %v, want %v", result.OperatingSystem, tt.expectedOS)
			}

			if result.Architecture != tt.expectedArch {
				t.Errorf("Architecture = %v, want %v", result.Architecture, tt.expectedArch)
			}

			if result.Compiler != tt.expectedCompiler {
				t.Errorf("Compiler = %v, want %v", result.Compiler, tt.expectedCompiler)
			}

			if result.BitVersion != tt.expectedBitVer {
				t.Errorf("BitVersion = %v, want %v", result.BitVersion, tt.expectedBitVer)
			}
		})
	}
}

func TestFindSubstring(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substr   string
		expected int
	}{
		{"Found at start", "hello world", "hello", 0},
		{"Found in middle", "hello world", "o w", 4},
		{"Found at end", "hello world", "world", 6},
		{"Not found", "hello world", "xyz", -1},
		{"Empty substring", "hello", "", 0},
		{"Substring longer than string", "hi", "hello", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findSubstring(tt.s, tt.substr)
			if result != tt.expected {
				t.Errorf("findSubstring(%q, %q) = %d, want %d", tt.s, tt.substr, result, tt.expected)
			}
		})
	}
}

func TestSplitString(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		sep      string
		expected []string
	}{
		{
			name:     "Split by dash",
			s:        "x86_64-pc-linux-gnu",
			sep:      "-",
			expected: []string{"x86_64", "pc", "linux", "gnu"},
		},
		{
			name:     "Split by space",
			s:        "hello world test",
			sep:      " ",
			expected: []string{"hello", "world", "test"},
		},
		{
			name:     "No separator found",
			s:        "hello",
			sep:      "-",
			expected: []string{"hello"},
		},
		{
			name:     "Empty string",
			s:        "",
			sep:      "-",
			expected: []string{""},
		},
		{
			name:     "Consecutive separators",
			s:        "a--b",
			sep:      "-",
			expected: []string{"a", "", "b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitString(tt.s, tt.sep)
			if len(result) != len(tt.expected) {
				t.Errorf("splitString(%q, %q) returned %d parts, want %d", tt.s, tt.sep, len(result), len(tt.expected))
				return
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("splitString(%q, %q)[%d] = %q, want %q", tt.s, tt.sep, i, result[i], tt.expected[i])
				}
			}
		})
	}
}

func TestSystemInfoStructure(t *testing.T) {
	// Test that the structure contains all expected fields
	info := SystemInfo{
		PostgreSQLVersion: "15.4",
		VersionNumber:     "150004",
		FullVersion:       "PostgreSQL 15.4 on x86_64-pc-linux-gnu, compiled by gcc (GCC) 11.2.0, 64-bit",
		OperatingSystem:   "linux",
		Architecture:      "x86_64-pc-linux-gnu",
		Compiler:          "gcc (GCC) 11.2.0",
		BitVersion:        "64-bit",
	}

	if info.PostgreSQLVersion == "" {
		t.Error("PostgreSQLVersion should not be empty")
	}
	if info.VersionNumber == "" {
		t.Error("VersionNumber should not be empty")
	}
	if info.FullVersion == "" {
		t.Error("FullVersion should not be empty")
	}
	if info.OperatingSystem == "" {
		t.Error("OperatingSystem should not be empty")
	}
	if info.Architecture == "" {
		t.Error("Architecture should not be empty")
	}
	if info.Compiler == "" {
		t.Error("Compiler should not be empty")
	}
	if info.BitVersion == "" {
		t.Error("BitVersion should not be empty")
	}
}

func TestParseVersionStringWithVariations(t *testing.T) {
	// Test with minimal version string
	result := parseVersionString("PostgreSQL 10.0", "10.0", "100000")
	if result.PostgreSQLVersion != "10.0" {
		t.Errorf("Expected version 10.0, got %s", result.PostgreSQLVersion)
	}
	if result.OperatingSystem != "Unknown" {
		t.Errorf("Expected Unknown OS for minimal version string, got %s", result.OperatingSystem)
	}

	// Test with unusual format
	result2 := parseVersionString(
		"PostgreSQL 17.0 (Debian 17.0-1.pgdg110+1) on aarch64-unknown-linux-gnu, compiled by gcc (Debian 10.2.1-6) 10.2.1 20210110, 64-bit",
		"17.0",
		"170000",
	)
	if result2.OperatingSystem != "linux" {
		t.Errorf("Expected linux OS, got %s", result2.OperatingSystem)
	}
	if !strings.Contains(result2.Compiler, "gcc") {
		t.Errorf("Expected compiler to contain 'gcc', got %s", result2.Compiler)
	}
}
