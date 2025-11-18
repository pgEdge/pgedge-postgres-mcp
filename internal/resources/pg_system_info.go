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
	"fmt"
	"strings"

	"pgedge-postgres-mcp/internal/database"
	"pgedge-postgres-mcp/internal/mcp"

	"github.com/jackc/pgx/v5"
)

// PGSystemInfoResource creates a resource for PostgreSQL system information
func PGSystemInfoResource(dbClient *database.Client) Resource {
	return Resource{
		Definition: mcp.Resource{
			URI:         URISystemInfo,
			Name:        "PostgreSQL System Information",
			Description: "Returns PostgreSQL version, operating system, and build architecture information. Provides a quick way to check server version and platform details.",
			MimeType:    "application/json",
		},
		Handler: func() (mcp.ResourceContent, error) {
			query := `
				SELECT
					version() AS full_version,
					current_setting('server_version') AS version,
					current_setting('server_version_num') AS version_number,
					current_database() AS database,
					current_user AS user,
					COALESCE(inet_server_addr()::text, 'unix socket') AS host,
					COALESCE(inet_server_port(), 0) AS port
			`

			processor := func(rows pgx.Rows) (interface{}, error) {
				if !rows.Next() {
					return nil, fmt.Errorf("no system information returned")
				}

				var fullVersion, version, versionNumber, database, user, host string
				var port int
				err := rows.Scan(&fullVersion, &version, &versionNumber, &database, &user, &host, &port)
				if err != nil {
					return nil, fmt.Errorf("failed to scan system info: %w", err)
				}

				// Parse the version string to extract components
				// Example: "PostgreSQL 15.4 on x86_64-pc-linux-gnu, compiled by gcc (GCC) 11.2.0, 64-bit"
				systemInfo := parseVersionString(fullVersion, version, versionNumber)
				systemInfo.Database = database
				systemInfo.User = user
				systemInfo.Host = host
				systemInfo.Port = port
				return systemInfo, nil
			}

			return database.ExecuteResourceQuery(dbClient, URISystemInfo, query, processor)
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
	Database          string `json:"database"`
	User              string `json:"user"`
	Host              string `json:"host"`
	Port              int    `json:"port"`
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
	if idx := strings.Index(fullVersion, " on "); idx != -1 {
		rest := fullVersion[idx+4:]

		// Extract architecture (up to comma)
		if arch, after, ok := strings.Cut(rest, ","); ok {
			info.Architecture = arch

			// Extract OS from architecture string
			// Format is typically: x86_64-pc-linux-gnu or aarch64-apple-darwin
			if dashIdx := strings.Index(info.Architecture, "-"); dashIdx != -1 {
				parts := strings.Split(info.Architecture, "-")
				if len(parts) >= 3 {
					// Third component is usually the OS
					info.OperatingSystem = parts[2]
				}
			}

			rest = after
		}

		// Extract compiler information
		if compiledIdx := strings.Index(rest, "compiled by "); compiledIdx != -1 {
			compilerStart := rest[compiledIdx+12:]
			if commaIdx := strings.Index(compilerStart, ","); commaIdx != -1 {
				info.Compiler = compilerStart[:commaIdx]

				// Extract bit version (32-bit or 64-bit)
				bitStart := compilerStart[commaIdx+1:]
				if bitIdx := strings.Index(bitStart, "-bit"); bitIdx != -1 {
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
