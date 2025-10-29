/*-------------------------------------------------------------------------
 *
 * pgEdge Postgres MCP Server
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package test

import (
	"strings"
	"testing"
)

// TestVersionDetectionPatterns tests the version detection logic used in testQueryPostgreSQLVersion
// This ensures the test will work with any PostgreSQL version without hardcoding version numbers
func TestVersionDetectionPatterns(t *testing.T) {
	tests := []struct {
		name           string
		responseText   string
		shouldDetect   bool
		description    string
	}{
		{
			name:         "PostgreSQL 17.0",
			responseText: "Results: PostgreSQL 17.0",
			shouldDetect: true,
			description:  "Latest version with keyword and version pattern",
		},
		{
			name:         "PostgreSQL 16.3",
			responseText: "Results: PostgreSQL 16.3",
			shouldDetect: true,
			description:  "Current stable version",
		},
		{
			name:         "PostgreSQL 15.5",
			responseText: "Results: PostgreSQL 15.5",
			shouldDetect: true,
			description:  "Older stable version",
		},
		{
			name:         "PostgreSQL 14.10",
			responseText: "Results: PostgreSQL 14.10",
			shouldDetect: true,
			description:  "Older version with two-digit minor",
		},
		{
			name:         "PostgreSQL 13.12",
			responseText: "Results: PostgreSQL 13.12",
			shouldDetect: true,
			description:  "Older version",
		},
		{
			name:         "PostgreSQL 12.17",
			responseText: "Results: PostgreSQL 12.17",
			shouldDetect: true,
			description:  "Even older version",
		},
		{
			name:         "PostgreSQL 11.22",
			responseText: "Results: PostgreSQL 11.22",
			shouldDetect: true,
			description:  "Older version",
		},
		{
			name:         "PostgreSQL 10.23",
			responseText: "Results: PostgreSQL 10.23",
			shouldDetect: true,
			description:  "Version 10",
		},
		{
			name:         "PostgreSQL 9.6.24",
			responseText: "Results: PostgreSQL 9.6.24",
			shouldDetect: true,
			description:  "Old three-part version",
		},
		{
			name:         "Version 18.0 future",
			responseText: "Results: version 18.0",
			shouldDetect: true,
			description:  "Future version with lowercase 'version'",
		},
		{
			name:         "Just version number 15.2",
			responseText: "Results: 15.2",
			shouldDetect: true,
			description:  "Version pattern without keyword",
		},
		{
			name:         "Development version",
			responseText: "Results: PostgreSQL 17devel",
			shouldDetect: true,
			description:  "Development version",
		},
		{
			name:         "Beta version",
			responseText: "Results: PostgreSQL 17beta1",
			shouldDetect: true,
			description:  "Beta version",
		},
		{
			name:         "RC version",
			responseText: "Results: PostgreSQL 17rc1",
			shouldDetect: true,
			description:  "Release candidate",
		},
		{
			name:         "Version in sentence",
			responseText: "The database is running PostgreSQL version 16.1 on Linux",
			shouldDetect: true,
			description:  "Version embedded in sentence",
		},
		{
			name:         "Just numbers 16",
			responseText: "Results: 16",
			shouldDetect: true,
			description:  "Just major version number (2 digits)",
		},
		{
			name:         "Version with build info",
			responseText: "PostgreSQL 15.4 (Ubuntu 15.4-1.pgdg22.04+1)",
			shouldDetect: true,
			description:  "Version with build information",
		},
		{
			name:         "No version info at all",
			responseText: "Error: Could not connect",
			shouldDetect: false,
			description:  "Error message with no version",
		},
		{
			name:         "Empty response",
			responseText: "",
			shouldDetect: false,
			description:  "Empty response",
		},
		{
			name:         "Single digit only",
			responseText: "Results: 5",
			shouldDetect: false,
			description:  "Single digit doesn't indicate version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text := tt.responseText
			textLower := strings.ToLower(text)

			// Pattern 1: Contains "postgresql" or "version"
			hasVersionKeyword := strings.Contains(textLower, "postgresql") ||
				strings.Contains(textLower, "version")

			// Pattern 2: Contains version-like number pattern (e.g., "14.5", "15.2")
			hasVersionPattern := false
			for i := 0; i < len(text)-2; i++ {
				if text[i] >= '0' && text[i] <= '9' {
					if text[i+1] == '.' {
						if i+2 < len(text) && text[i+2] >= '0' && text[i+2] <= '9' {
							hasVersionPattern = true
							break
						}
					}
				}
			}

			// Pattern 3: Contains 2+ consecutive digits (version number)
			hasMultiDigit := false
			digitCount := 0
			for _, char := range text {
				if char >= '0' && char <= '9' {
					digitCount++
					if digitCount >= 2 {
						hasMultiDigit = true
						break
					}
				} else {
					digitCount = 0
				}
			}

			hasVersionInfo := hasVersionKeyword || hasVersionPattern || hasMultiDigit

			if hasVersionInfo != tt.shouldDetect {
				t.Errorf("%s: Detection failed\n  Text: %q\n  Expected detect: %v\n  Got: %v\n  Details: keyword=%v, pattern=%v, multidigit=%v",
					tt.description, text, tt.shouldDetect, hasVersionInfo,
					hasVersionKeyword, hasVersionPattern, hasMultiDigit)
			} else {
				t.Logf("✓ %s: Correctly %s version info in %q",
					tt.description,
					map[bool]string{true: "detected", false: "rejected"}[hasVersionInfo],
					text)
			}
		})
	}
}
