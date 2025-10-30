/*-------------------------------------------------------------------------
 *
 * pgEdge Postgres MCP Server
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package tools

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"pgedge-postgres-mcp/internal/database"
	"pgedge-postgres-mcp/internal/mcp"
)

// ReadServerLogTool creates a tool to read the PostgreSQL server log
func ReadServerLogTool(dbClient *database.Client) Tool {
	return Tool{
		Definition: mcp.Tool{
			Name:        "read_server_log",
			Description: "Read the PostgreSQL server log file. Returns the most recent log entries. Requires superuser privileges or pg_monitor role. Use the 'lines' parameter to limit output size for large log files.",
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"lines": map[string]interface{}{
						"type":        "number",
						"description": "Optional: Number of lines to read from the end of the log file. Default: 100. Maximum: 10000.",
						"default":     100,
					},
				},
			},
		},
		Handler: func(args map[string]interface{}) (mcp.ToolResponse, error) {
			// Validate lines parameter
			lines := ValidateOptionalNumberParam(args, "lines", 100)
			if lines < 1 || lines > 10000 {
				return mcp.NewToolError("Error: lines must be between 1 and 10000")
			}

			// Check if database is ready
			if !dbClient.IsMetadataLoaded() {
				return mcp.NewToolError(mcp.DatabaseNotReadyError)
			}

			pool := dbClient.GetPool()
			if pool == nil {
				return mcp.NewToolError("Error: Database connection not available")
			}

			ctx := context.Background()

			// Get log directory and current log file
			var logDir, logFilename string
			err := pool.QueryRow(ctx, "SELECT setting FROM pg_settings WHERE name = 'log_directory'").Scan(&logDir)
			if err != nil {
				return mcp.NewToolError(fmt.Sprintf("Error: Failed to get log_directory: %v", err))
			}

			err = pool.QueryRow(ctx, "SELECT setting FROM pg_settings WHERE name = 'log_filename'").Scan(&logFilename)
			if err != nil {
				return mcp.NewToolError(fmt.Sprintf("Error: Failed to get log_filename: %v", err))
			}

			// Query for current log file from pg_ls_logdir()
			var currentLog string
			err = pool.QueryRow(ctx, `
				SELECT name
				FROM pg_ls_logdir()
				ORDER BY modification DESC
				LIMIT 1
			`).Scan(&currentLog)
			if err != nil {
				return mcp.NewToolError(fmt.Sprintf("Error: Failed to find current log file: %v. Ensure logging is enabled and you have pg_monitor or superuser privileges.", err))
			}

			// Read log file content using pg_read_file
			// We read from the end by getting file size and calculating offset
			var logContent string
			query := `
				WITH log_info AS (
					SELECT size FROM pg_ls_logdir() WHERE name = $1
				)
				SELECT pg_read_file('log/' || $1,
					GREATEST(0, (SELECT size FROM log_info) - $2 * 200),
					$2 * 200
				)
			`

			err = pool.QueryRow(ctx, query, currentLog, int(lines)).Scan(&logContent)
			if err != nil {
				return mcp.NewToolError(fmt.Sprintf("Error: Failed to read log file: %v. Ensure you have pg_monitor or superuser privileges.", err))
			}

			// Format output
			var output strings.Builder
			output.WriteString("PostgreSQL Server Log\n")
			output.WriteString("=====================\n\n")
			output.WriteString(fmt.Sprintf("Log Directory: %s\n", logDir))
			output.WriteString(fmt.Sprintf("Current Log File: %s\n", currentLog))
			output.WriteString(fmt.Sprintf("Showing last ~%d lines:\n", int(lines)))
			output.WriteString("\n")
			output.WriteString("--- Log Contents ---\n\n")
			output.WriteString(logContent)

			return mcp.NewToolSuccess(output.String())
		},
	}
}

// ReadPostgresqlConfTool creates a tool to read postgresql.conf and included files
func ReadPostgresqlConfTool(dbClient *database.Client) Tool {
	return Tool{
		Definition: mcp.Tool{
			Name:        "read_postgresql_conf",
			Description: "Read the contents of postgresql.conf and any files included via include, include_if_exists, or include_dir directives. Returns the complete configuration with file locations. Requires superuser privileges or pg_read_server_files role.",
			InputSchema: mcp.InputSchema{
				Type:       "object",
				Properties: map[string]interface{}{},
			},
		},
		Handler: func(args map[string]interface{}) (mcp.ToolResponse, error) {
			// Check if database is ready
			if !dbClient.IsMetadataLoaded() {
				return mcp.NewToolError(mcp.DatabaseNotReadyError)
			}

			pool := dbClient.GetPool()
			if pool == nil {
				return mcp.NewToolError("Error: Database connection not available")
			}

			ctx := context.Background()

			// Get config file path
			var configFile string
			err := pool.QueryRow(ctx, "SELECT setting FROM pg_settings WHERE name = 'config_file'").Scan(&configFile)
			if err != nil {
				return mcp.NewToolError(fmt.Sprintf("Error: Failed to get config_file path: %v", err))
			}

			// Read main postgresql.conf
			mainConfig, err := readConfigFile(ctx, pool, configFile)
			if err != nil {
				return mcp.NewToolError(fmt.Sprintf("Error: Failed to read postgresql.conf: %v. Ensure you have pg_read_server_files or superuser privileges.", err))
			}

			// Parse for include directives
			includedFiles := parseIncludeDirectives(mainConfig, configFile)

			// Build output
			var output strings.Builder
			output.WriteString("PostgreSQL Configuration Files\n")
			output.WriteString("==============================\n\n")
			output.WriteString(fmt.Sprintf("Main Configuration File: %s\n", configFile))
			output.WriteString("\n--- postgresql.conf ---\n\n")
			output.WriteString(mainConfig)

			// Read and display included files
			if len(includedFiles) > 0 {
				output.WriteString("\n\n")
				output.WriteString(fmt.Sprintf("Found %d included file(s):\n", len(includedFiles)))
				for i, includeFile := range includedFiles {
					output.WriteString(fmt.Sprintf("\n--- Included File %d: %s ---\n\n", i+1, includeFile))
					content, err := readConfigFile(ctx, pool, includeFile)
					if err != nil {
						output.WriteString(fmt.Sprintf("[Error reading file: %v]\n", err))
					} else {
						output.WriteString(content)
					}
				}
			}

			return mcp.NewToolSuccess(output.String())
		},
	}
}

// ReadPgHbaConfTool creates a tool to read pg_hba.conf
func ReadPgHbaConfTool(dbClient *database.Client) Tool {
	return Tool{
		Definition: mcp.Tool{
			Name:        "read_pg_hba_conf",
			Description: "Read the contents of pg_hba.conf (Host-Based Authentication configuration). This file controls client authentication and connection permissions. Requires superuser privileges or pg_read_server_files role.",
			InputSchema: mcp.InputSchema{
				Type:       "object",
				Properties: map[string]interface{}{},
			},
		},
		Handler: func(args map[string]interface{}) (mcp.ToolResponse, error) {
			return readSingleConfigFile(dbClient, "hba_file", "pg_hba.conf", "Host-Based Authentication")
		},
	}
}

// ReadPgIdentConfTool creates a tool to read pg_ident.conf
func ReadPgIdentConfTool(dbClient *database.Client) Tool {
	return Tool{
		Definition: mcp.Tool{
			Name:        "read_pg_ident_conf",
			Description: "Read the contents of pg_ident.conf (User Name Mapping configuration). This file maps external authentication identities to PostgreSQL user names. Requires superuser privileges or pg_read_server_files role.",
			InputSchema: mcp.InputSchema{
				Type:       "object",
				Properties: map[string]interface{}{},
			},
		},
		Handler: func(args map[string]interface{}) (mcp.ToolResponse, error) {
			return readSingleConfigFile(dbClient, "ident_file", "pg_ident.conf", "User Name Mapping")
		},
	}
}

// Helper function to read a single configuration file
func readSingleConfigFile(dbClient *database.Client, settingName, fileName, description string) (mcp.ToolResponse, error) {
	// Check if database is ready
	if !dbClient.IsMetadataLoaded() {
		return mcp.NewToolError(mcp.DatabaseNotReadyError)
	}

	pool := dbClient.GetPool()
	if pool == nil {
		return mcp.NewToolError("Error: Database connection not available")
	}

	ctx := context.Background()

	// Get file path from pg_settings
	var filePath string
	err := pool.QueryRow(ctx, fmt.Sprintf("SELECT setting FROM pg_settings WHERE name = '%s'", settingName)).Scan(&filePath)
	if err != nil {
		return mcp.NewToolError(fmt.Sprintf("Error: Failed to get %s path: %v", fileName, err))
	}

	// Read file content
	content, err := readConfigFile(ctx, pool, filePath)
	if err != nil {
		return mcp.NewToolError(fmt.Sprintf("Error: Failed to read %s: %v. Ensure you have pg_read_server_files or superuser privileges.", fileName, err))
	}

	// Format output
	var output strings.Builder
	output.WriteString(fmt.Sprintf("PostgreSQL %s Configuration\n", description))
	output.WriteString(strings.Repeat("=", 50) + "\n\n")
	output.WriteString(fmt.Sprintf("File: %s\n", filePath))
	output.WriteString("\n--- Contents ---\n\n")
	output.WriteString(content)

	return mcp.NewToolSuccess(output.String())
}

// Helper function to read a config file using pg_read_file
func readConfigFile(ctx context.Context, pool *pgxpool.Pool, filePath string) (string, error) {
	var content string
	err := pool.QueryRow(ctx, "SELECT pg_read_file($1)", filePath).Scan(&content)
	return content, err
}

// Helper function to parse include directives from postgresql.conf
func parseIncludeDirectives(content, baseDir string) []string {
	var includes []string

	// Regular expressions for include directives
	includeRe := regexp.MustCompile(`(?m)^\s*include\s*=\s*'([^']+)'`)
	includeIfExistsRe := regexp.MustCompile(`(?m)^\s*include_if_exists\s*=\s*'([^']+)'`)

	// Find all include directives
	for _, match := range includeRe.FindAllStringSubmatch(content, -1) {
		if len(match) > 1 {
			includes = append(includes, match[1])
		}
	}

	for _, match := range includeIfExistsRe.FindAllStringSubmatch(content, -1) {
		if len(match) > 1 {
			includes = append(includes, match[1])
		}
	}

	// Note: include_dir would require directory listing which is more complex
	// For now, we just report the files explicitly included

	return includes
}
