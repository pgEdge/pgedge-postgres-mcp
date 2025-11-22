/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package database

import (
	"regexp"
	"strings"
)

// QueryContext contains information parsed from a natural language query
type QueryContext struct {
	CleanedQuery     string // The query with connection string references removed
	ConnectionString string // The extracted connection string (empty if none)
	SetAsDefault     bool   // Whether to set this as the new default connection
}

// ParseQueryForConnection extracts connection string and intent from a natural language query
func ParseQueryForConnection(query string) *QueryContext {
	ctx := &QueryContext{
		CleanedQuery: query,
	}

	// Patterns for detecting connection strings
	// Matches: postgres://..., postgresql://...
	connStringPattern := regexp.MustCompile(`postgres(?:ql)?://[^\s'"]+`)

	// Check for "set default" or "use database" commands
	lowerQuery := strings.ToLower(query)

	// Pattern: "set default database to postgres://..."
	if strings.Contains(lowerQuery, "set default") ||
		strings.Contains(lowerQuery, "use database") ||
		strings.Contains(lowerQuery, "switch to") {
		ctx.SetAsDefault = true

		// Extract the connection string
		if match := connStringPattern.FindString(query); match != "" {
			ctx.ConnectionString = match
			// Remove the command from the query
			ctx.CleanedQuery = ""
		}
		return ctx
	}

	// Pattern: "... at postgres://..." or "... from postgres://..." or "... on postgres://..."
	atPattern := regexp.MustCompile(`(?i)\s+(?:at|from|on|for|in)\s+(postgres(?:ql)?://[^\s'"]+)`)
	if matches := atPattern.FindStringSubmatch(query); len(matches) > 1 {
		ctx.ConnectionString = matches[1]
		// Remove the connection string reference from the query
		ctx.CleanedQuery = atPattern.ReplaceAllString(query, "")
		ctx.CleanedQuery = strings.TrimSpace(ctx.CleanedQuery)
		return ctx
	}

	// Pattern: "database postgres://... " at the beginning
	dbPattern := regexp.MustCompile(`(?i)^(?:database|db)\s+(postgres(?:ql)?://[^\s'"]+)\s+`)
	if matches := dbPattern.FindStringSubmatch(query); len(matches) > 1 {
		ctx.ConnectionString = matches[1]
		// Remove the database prefix from the query
		ctx.CleanedQuery = dbPattern.ReplaceAllString(query, "")
		ctx.CleanedQuery = strings.TrimSpace(ctx.CleanedQuery)
		return ctx
	}

	return ctx
}

// IsSetDefaultCommand checks if the query is a command to set the default database
func IsSetDefaultCommand(query string) bool {
	lowerQuery := strings.ToLower(strings.TrimSpace(query))
	return strings.HasPrefix(lowerQuery, "set default") ||
		strings.HasPrefix(lowerQuery, "use database") ||
		strings.HasPrefix(lowerQuery, "switch to database") ||
		strings.HasPrefix(lowerQuery, "change database to")
}
