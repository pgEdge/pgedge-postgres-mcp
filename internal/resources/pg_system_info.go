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
	"context"
	"encoding/json"
	"fmt"

	"pgedge-postgres-mcp/internal/database"
	"pgedge-postgres-mcp/internal/mcp"
)

// PGSystemInfoResource creates a resource for PostgreSQL system information
func PGSystemInfoResource(dbClient *database.Client) Resource {
	return Resource{
		Definition: mcp.Resource{
			URI:         "pg://system_info",
			Name:        "PostgreSQL System Information",
			Description: "Returns PostgreSQL version, operating system, and build architecture information. Provides a quick way to check server version and platform details.",
			MimeType:    "application/json",
		},
		Handler: func() (mcp.ResourceContent, error) {
			// Check if database is ready
			if !dbClient.IsMetadataLoaded() {
				return mcp.ResourceContent{
					URI:      "pg://system_info",
					MimeType: "application/json",
					Contents: []mcp.ContentItem{
						{
							Type: "text",
							Text: "Database is still initializing. Please wait a moment and try again.",
						},
					},
				}, fmt.Errorf("database not ready")
			}

			// Query for PostgreSQL version and system information
			query := `
				SELECT
					version() AS full_version,
					current_setting('server_version') AS version,
					current_setting('server_version_num') AS version_number
			`

			ctx := context.Background()
			pool := dbClient.GetPool()
			if pool == nil {
				return mcp.ResourceContent{
					URI:      "pg://system_info",
					MimeType: "application/json",
					Contents: []mcp.ContentItem{
						{
							Type: "text",
							Text: "Database connection not available",
						},
					},
				}, fmt.Errorf("database connection not available")
			}

			var fullVersion, version, versionNumber string
			err := pool.QueryRow(ctx, query).Scan(&fullVersion, &version, &versionNumber)
			if err != nil {
				return mcp.ResourceContent{
					URI:      "pg://system_info",
					MimeType: "application/json",
					Contents: []mcp.ContentItem{
						{
							Type: "text",
							Text: fmt.Sprintf("Failed to query system information: %v", err),
						},
					},
				}, fmt.Errorf("query failed: %w", err)
			}

			// Parse the version string to extract components
			// Example: "PostgreSQL 15.4 on x86_64-pc-linux-gnu, compiled by gcc (GCC) 11.2.0, 64-bit"
			systemInfo := parseVersionString(fullVersion, version, versionNumber)

			// Convert to JSON
			jsonData, err := json.MarshalIndent(systemInfo, "", "  ")
			if err != nil {
				return mcp.ResourceContent{
					URI:      "pg://system_info",
					MimeType: "application/json",
					Contents: []mcp.ContentItem{
						{
							Type: "text",
							Text: fmt.Sprintf("Failed to format system information: %v", err),
						},
					},
				}, fmt.Errorf("marshal failed: %w", err)
			}

			return mcp.ResourceContent{
				URI:      "pg://system_info",
				MimeType: "application/json",
				Contents: []mcp.ContentItem{
					{
						Type: "text",
						Text: string(jsonData),
					},
				},
			}, nil
		},
	}
}

// SystemInfo represents PostgreSQL system information
type SystemInfo struct {
	PostgreSQLVersion string `json:"postgresql_version"`
	VersionNumber     string `json:"version_number"`
	FullVersion       string `json:"full_version"`
	OperatingSystem   string `json:"operating_system"`
	Architecture      string `json:"architecture"`
	Compiler          string `json:"compiler"`
	BitVersion        string `json:"bit_version"`
}

// parseVersionString extracts system information from PostgreSQL version() output
func parseVersionString(fullVersion, version, versionNumber string) SystemInfo {
	info := SystemInfo{
		PostgreSQLVersion: version,
		VersionNumber:     versionNumber,
		FullVersion:       fullVersion,
		OperatingSystem:   "Unknown",
		Architecture:      "Unknown",
		Compiler:          "Unknown",
		BitVersion:        "Unknown",
	}

	// Parse the full version string
	// Example: "PostgreSQL 15.4 on x86_64-pc-linux-gnu, compiled by gcc (GCC) 11.2.0, 64-bit"

	// Extract OS and architecture
	// Look for " on " pattern
	if idx := findSubstring(fullVersion, " on "); idx != -1 {
		rest := fullVersion[idx+4:]

		// Extract architecture (up to comma)
		if commaIdx := findSubstring(rest, ","); commaIdx != -1 {
			info.Architecture = rest[:commaIdx]

			// Extract OS from architecture string
			// Format is typically: x86_64-pc-linux-gnu or aarch64-apple-darwin
			if dashIdx := findSubstring(info.Architecture, "-"); dashIdx != -1 {
				parts := splitString(info.Architecture, "-")
				if len(parts) >= 3 {
					// Third component is usually the OS
					info.OperatingSystem = parts[2]
				}
			}

			rest = rest[commaIdx+1:]
		}

		// Extract compiler information
		if compiledIdx := findSubstring(rest, "compiled by "); compiledIdx != -1 {
			compilerStart := rest[compiledIdx+12:]
			if commaIdx := findSubstring(compilerStart, ","); commaIdx != -1 {
				info.Compiler = compilerStart[:commaIdx]

				// Extract bit version (32-bit or 64-bit)
				bitStart := compilerStart[commaIdx+1:]
				if bitIdx := findSubstring(bitStart, "-bit"); bitIdx != -1 {
					// Find the start of the bit version (look backwards for space or start)
					for i := bitIdx - 1; i >= 0; i-- {
						if bitStart[i] == ' ' {
							info.BitVersion = bitStart[i+1 : bitIdx+4]
							break
						}
						if i == 0 {
							info.BitVersion = bitStart[0 : bitIdx+4]
							break
						}
					}
				}
			} else {
				info.Compiler = compilerStart
			}
		}
	}

	return info
}

// Helper function to find substring
func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// Helper function to split string
func splitString(s, sep string) []string {
	var result []string
	start := 0

	for i := 0; i <= len(s)-len(sep); i++ {
		if s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
			i += len(sep) - 1
		}
	}
	result = append(result, s[start:])

	return result
}
