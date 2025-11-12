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
	"net/url"
	"os"
	"regexp"
	"strings"

	"pgedge-postgres-mcp/internal/auth"
	"pgedge-postgres-mcp/internal/database"
	"pgedge-postgres-mcp/internal/mcp"
)

// maskPassword masks the password in a connection string for safe logging
func maskPassword(connStr string) string {
	// Match password in postgres://user:password@host format
	re := regexp.MustCompile(`(postgres(?:ql)?://[^:]+:)([^@]+)(@.+)`)
	return re.ReplaceAllString(connStr, "${1}***${3}")
}

// tryMergeSavedConnection attempts to merge a connection string with a saved connection
// If the connection string references a hostname that matches a saved connection,
// it will use the saved connection's credentials and merge any database name from the request
func tryMergeSavedConnection(ctx context.Context, connString string, connMgr *ConnectionManager, configPath string) (string, string) {
	fmt.Fprintf(os.Stderr, "[DEBUG] tryMergeSavedConnection: Input connection string: %s\n", maskPassword(connString))

	// Parse the connection string
	parsedURL, err := url.Parse(connString)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[DEBUG] tryMergeSavedConnection: Failed to parse URL: %v\n", err)
		// If we can't parse it, just return the original
		return connString, ""
	}

	// Extract hostname and database name from the provided connection string
	requestedHost := parsedURL.Hostname()
	requestedDB := strings.TrimPrefix(parsedURL.Path, "/")
	fmt.Fprintf(os.Stderr, "[DEBUG] tryMergeSavedConnection: Extracted host=%s, db=%s\n", requestedHost, requestedDB)

	if requestedHost == "" {
		// No hostname in the connection string, can't merge
		return connString, ""
	}

	// Try to get the connection store
	store, err := connMgr.GetConnectionStore(ctx)
	if err != nil {
		// No saved connections available
		return connString, ""
	}

	// Search for saved connections by:
	// 1. Case-insensitive alias match
	// 2. Exact hostname match
	var matchedConn *auth.SavedConnection
	var matchedAlias string

	// First, try case-insensitive alias match (most explicit)
	for alias, savedConn := range store.Connections {
		if strings.EqualFold(alias, requestedHost) {
			matchedConn = savedConn
			matchedAlias = alias
			fmt.Fprintf(os.Stderr, "[DEBUG] tryMergeSavedConnection: Matched by alias (case-insensitive): %s\n", alias)
			break
		}
	}

	// If no alias match, try hostname match
	if matchedConn == nil {
		for alias, savedConn := range store.Connections {
			if savedConn.Host == requestedHost {
				matchedConn = savedConn
				matchedAlias = alias
				fmt.Fprintf(os.Stderr, "[DEBUG] tryMergeSavedConnection: Matched by hostname: %s\n", savedConn.Host)
				break
			}
		}
	}

	if matchedConn == nil {
		fmt.Fprintf(os.Stderr, "[DEBUG] tryMergeSavedConnection: No matching saved connection found\n")
		// No matching saved connection found
		return connString, ""
	}

	fmt.Fprintf(os.Stderr, "[DEBUG] tryMergeSavedConnection: Found matching connection: alias=%s, host=%s, port=%d, user=%s, dbname=%s\n",
		matchedAlias, matchedConn.Host, matchedConn.Port, matchedConn.User, matchedConn.DBName)

	// Found a matching saved connection - use its credentials
	decryptedPassword := ""
	if matchedConn.Password != "" {
		decryptedPassword, err = connMgr.encryptionKey.Decrypt(matchedConn.Password)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[DEBUG] tryMergeSavedConnection: Failed to decrypt password: %v\n", err)
			// Can't decrypt password, return original
			return connString, ""
		}
		fmt.Fprintf(os.Stderr, "[DEBUG] tryMergeSavedConnection: Successfully decrypted password\n")
	}

	// Clone the saved connection to avoid modifying the original
	mergedConn := matchedConn.Clone()

	// Override the database name if one was specified in the request
	if requestedDB != "" {
		fmt.Fprintf(os.Stderr, "[DEBUG] tryMergeSavedConnection: Overriding database name: %s -> %s\n", mergedConn.DBName, requestedDB)
		mergedConn.DBName = requestedDB
	}

	// Build the merged connection string
	mergedConnStr := mergedConn.ToConnectionString(decryptedPassword)
	fmt.Fprintf(os.Stderr, "[DEBUG] tryMergeSavedConnection: Final connection string: %s\n", maskPassword(mergedConnStr))

	// Mark the saved connection as used
	if err := store.MarkUsed(matchedAlias); err == nil {
		//nolint:errcheck // Ignore save error as connection can still proceed
		connMgr.saveChanges(configPath)
	}

	return mergedConnStr, matchedAlias
}

// SetDatabaseConnectionTool creates a tool for setting the database connection at runtime
// Now supports both connection strings and aliases to saved connections
func SetDatabaseConnectionTool(clientManager *database.ClientManager, connMgr *ConnectionManager, configPath string) Tool {
	return Tool{
		Definition: mcp.Tool{
			Name:        "set_database_connection",
			Description: "Set the PostgreSQL database connection for this session. This tool intelligently matches connection names (case-insensitive) and reuses saved credentials. IMPORTANT: This tool does NOT modify saved connections. **Smart Matching**: When a user mentions a server or connection by name (e.g., 'connect to server1', 'use production database'), look for a matching saved connection. If found, ALL saved parameters are automatically used (host, port, user, password, SSL settings). The database name can be overridden. **Usage**: 1) Use saved connection as-is: 'production' 2) Use saved credentials with different database: 'postgres://user@server1/different_db' (uses saved 'server1' credentials) 3) New connection: 'postgres://user:pass@newhost/db'. This must be called before using any database-dependent tools.",
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"connection_string": map[string]interface{}{
						"type":        "string",
						"description": "PostgreSQL connection string OR alias to a saved connection. The tool automatically matches aliases/hostnames (case-insensitive) and merges with saved credentials. When a hostname matches a saved connection (e.g., 'server1' matches saved 'server1'), all saved credentials are used (host, port, user, password, SSL). Only the database name can be overridden. Format: 'alias' (e.g., 'production'), 'postgres://user@alias/database' (uses saved credentials), or 'postgres://user:pass@host:port/database' (new connection)",
					},
				},
				Required: []string{"connection_string"},
			},
		},
		Handler: func(args map[string]interface{}) (mcp.ToolResponse, error) {
			connStrOrAlias, ok := args["connection_string"].(string)
			if !ok {
				return mcp.ToolResponse{
					Content: []mcp.ContentItem{
						{
							Type: "text",
							Text: "Error: connection_string must be a string",
						},
					},
					IsError: true,
				}, nil
			}

			if connStrOrAlias == "" {
				return mcp.ToolResponse{
					Content: []mcp.ContentItem{
						{
							Type: "text",
							Text: "Error: connection_string cannot be empty",
						},
					},
					IsError: true,
				}, nil
			}

			fmt.Fprintf(os.Stderr, "[DEBUG] set_database_connection: Received connection_string=%s\n", maskPassword(connStrOrAlias))

			connStr := connStrOrAlias
			alias := ""
			ctx := context.Background()

			// Check if this looks like an alias (no postgres:// prefix)
			if !strings.HasPrefix(connStrOrAlias, "postgres://") && !strings.HasPrefix(connStrOrAlias, "postgresql://") {
				fmt.Fprintf(os.Stderr, "[DEBUG] set_database_connection: Input looks like alias, trying to resolve\n")
				// Try to resolve as alias (case-insensitive)
				store, err := connMgr.GetConnectionStore(ctx)
				if err == nil {
					// First try exact match
					savedConn, err := store.Get(connStrOrAlias)
					if err != nil {
						// Try case-insensitive match
						for candidateAlias, candidate := range store.Connections {
							if strings.EqualFold(candidateAlias, connStrOrAlias) {
								savedConn = candidate
								err = nil
								break
							}
						}
					}
					if err == nil && savedConn != nil {
						fmt.Fprintf(os.Stderr, "[DEBUG] set_database_connection: Found saved connection for alias=%s (actual: %s)\n", connStrOrAlias, savedConn.Alias)
						// Found saved connection - decrypt password and build connection string
						decryptedPassword := ""
						if savedConn.Password != "" {
							decryptedPassword, err = connMgr.encryptionKey.Decrypt(savedConn.Password)
							if err != nil {
								return mcp.ToolResponse{
									Content: []mcp.ContentItem{
										{
											Type: "text",
											Text: fmt.Sprintf("Error: Failed to decrypt password: %v", err),
										},
									},
									IsError: true,
								}, nil
							}
						}

						connStr = savedConn.ToConnectionString(decryptedPassword)
						alias = savedConn.Alias

						// Mark as used (ignore errors as this is non-critical metadata)
						if err := store.MarkUsed(alias); err == nil {
							//nolint:errcheck // Ignore save error as connection can still proceed
							connMgr.saveChanges(configPath)
						}
					}
				}
			} else {
				fmt.Fprintf(os.Stderr, "[DEBUG] set_database_connection: Input is connection string, trying hostname merge\n")
				// It's a connection string - try to merge with saved connection by hostname
				connStr, _ = tryMergeSavedConnection(ctx, connStrOrAlias, connMgr, configPath)
			}

			fmt.Fprintf(os.Stderr, "[DEBUG] set_database_connection: Attempting connection with: %s\n", maskPassword(connStr))

			// Create a new client with the provided connection string
			client := database.NewClientWithConnectionString(connStr)

			// Test the connection
			if err := client.Connect(); err != nil {
				return mcp.ToolResponse{
					Content: []mcp.ContentItem{
						{
							Type: "text",
							Text: fmt.Sprintf("Error: Failed to connect to database: %v", err),
						},
					},
					IsError: true,
				}, nil
			}

			// Load metadata
			if err := client.LoadMetadata(); err != nil {
				client.Close()
				return mcp.ToolResponse{
					Content: []mcp.ContentItem{
						{
							Type: "text",
							Text: fmt.Sprintf("Error: Failed to load database metadata: %v", err),
						},
					},
					IsError: true,
				}, nil
			}

			// Set as the default client for this session
			// For stdio mode, use a fixed key
			// For HTTP mode with auth, the context-aware provider will use the token hash
			if err := clientManager.SetClient("default", client); err != nil {
				client.Close()
				return mcp.ToolResponse{
					Content: []mcp.ContentItem{
						{
							Type: "text",
							Text: fmt.Sprintf("Error: Failed to set database connection: %v", err),
						},
					},
					IsError: true,
				}, nil
			}

			metadata := client.GetMetadata()
			return mcp.ToolResponse{
				Content: []mcp.ContentItem{
					{
						Type: "text",
						Text: fmt.Sprintf("Successfully connected to database. Loaded metadata for %d tables/views.", len(metadata)),
					},
				},
			}, nil
		},
	}
}
