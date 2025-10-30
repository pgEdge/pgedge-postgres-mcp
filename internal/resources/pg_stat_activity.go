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

	"pgedge-mcp/internal/database"
	"pgedge-mcp/internal/mcp"
)

// PGStatActivityResource provides current activity and connections
func PGStatActivityResource(dbClient *database.Client) Resource {
	return Resource{
		Definition: mcp.Resource{
			URI:         "pg://stat/activity",
			Name:        "PostgreSQL Current Activity",
			Description: "Shows information about currently executing queries and connections. Useful for monitoring active sessions, identifying long-running queries, and understanding current database load. Each row represents one server process with details about its current activity.",
			MimeType:    "application/json",
		},
		Handler: func() (mcp.ResourceContent, error) {
			if !dbClient.IsMetadataLoaded() {
				return mcp.ResourceContent{
					URI: "pg://stat/activity",
					Contents: []mcp.ContentItem{
						{
							Type: "text",
							Text: "Error: Database not ready",
						},
					},
				}, nil
			}

			pool := dbClient.GetPool()
			if pool == nil {
				return mcp.ResourceContent{}, fmt.Errorf("no connection pool available")
			}

			ctx := context.Background()

			query := `
				SELECT
					datname,
					pid,
					usename,
					application_name,
					client_addr::text,
					backend_start::text as backend_start,
					state,
					query
				FROM pg_stat_activity
				WHERE pid != pg_backend_pid()
				ORDER BY backend_start DESC
				LIMIT 100
			`

			rows, err := pool.Query(ctx, query)
			if err != nil {
				return mcp.ResourceContent{}, fmt.Errorf("failed to query pg_stat_activity: %w", err)
			}
			defer rows.Close()

			var activities []map[string]interface{}
			for rows.Next() {
				var datname, usename, applicationName, clientAddr, state, query *string
				var pid int
				var backendStart *string

				err := rows.Scan(&datname, &pid, &usename, &applicationName, &clientAddr,
					&backendStart, &state, &query)
				if err != nil {
					continue
				}

				activity := map[string]interface{}{
					"datname":          datname,
					"pid":              pid,
					"usename":          usename,
					"application_name": applicationName,
					"client_addr":      clientAddr,
					"backend_start":    backendStart,
					"state":            state,
					"query":            query,
				}

				activities = append(activities, activity)
			}

			jsonData, err := json.MarshalIndent(map[string]interface{}{
				"activity_count": len(activities),
				"activities":     activities,
				"description":    "Current database activity including active queries and connections.",
			}, "", "  ")
			if err != nil {
				return mcp.ResourceContent{}, fmt.Errorf("failed to marshal JSON: %w", err)
			}

			return mcp.ResourceContent{
				URI:      "pg://stat/activity",
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
