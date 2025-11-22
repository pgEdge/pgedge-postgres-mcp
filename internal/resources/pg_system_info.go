/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
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
			URI:  URISystemInfo,
			Name: "PostgreSQL System Information",
			Description: `PostgreSQL server metadata: version, OS, architecture, connection details.

<usecase>
Use for:
- Version compatibility checks before using features
- Platform verification (Linux, macOS, Windows)
- Connection debugging and troubleshooting
- System architecture discovery (x86_64, ARM, etc.)
- Determining database capabilities
</usecase>

<provided_info>
Returns JSON with:
- postgresql_version: Major.minor.patch version string
- version_number: Numeric version for comparisons
- full_version: Complete version string with build details
- operating_system: OS name (linux, darwin, windows)
- architecture: CPU architecture (x86_64, aarch64, etc.)
- compiler: Compiler used to build PostgreSQL
- bit_version: 32-bit or 64-bit
- database: Currently connected database name
- user: Current database user
- host: Connection host (or "unix socket")
- port: Connection port number
</provided_info>

<caching>
This resource is highly cacheable - system info rarely changes during a session. Safe to call multiple times without performance concern.
</caching>

<examples>
Use before:
- Checking if pgvector is supported (version >= 12)
- Using version-specific SQL syntax
- Debugging connection issues
- Verifying deployment environment
</examples>`,
			MimeType: "application/json",
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
