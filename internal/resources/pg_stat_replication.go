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

// PGStatReplicationResource provides replication status
func PGStatReplicationResource(dbClient *database.Client) Resource {
	return Resource{
		Definition: mcp.Resource{
			URI:         "pg://stat/replication",
			Name:        "PostgreSQL Replication Status",
			Description: "Shows the status of replication connections from this primary server including WAL sender processes, replication lag, and sync state. Empty if the server is not a replication primary or has no active replicas. Critical for monitoring replication health and identifying lag issues.",
			MimeType:    "application/json",
		},
		Handler: func() (mcp.ResourceContent, error) {
			if !dbClient.IsMetadataLoaded() {
				return mcp.ResourceContent{
					URI: "pg://stat/replication",
					Contents: []mcp.ContentItem{{Type: "text", Text: "Error: Database not ready"}},
				}, nil
			}

			pool := dbClient.GetPool()
			if pool == nil {
				return mcp.ResourceContent{}, fmt.Errorf("no connection pool available")
			}

			ctx := context.Background()

			// Check if we have the replay_lag column (PG 10+)
			query := `
				SELECT
					pid,
					usename,
					application_name,
					client_addr::text,
					client_hostname,
					client_port,
					backend_start::text as backend_start,
					state,
					sync_state,
					COALESCE(replay_lag::text, 'N/A') as replay_lag
				FROM pg_stat_replication
				ORDER BY backend_start`

			rows, err := pool.Query(ctx, query)
			if err != nil {
				return mcp.ResourceContent{}, fmt.Errorf("failed to query pg_stat_replication: %w", err)
			}
			defer rows.Close()

			var replicas []map[string]interface{}
			for rows.Next() {
				var pid, clientPort int
				var usename, applicationName, clientAddr, clientHostname, state, syncState, replayLag *string
				var backendStart *string

				if err := rows.Scan(&pid, &usename, &applicationName, &clientAddr,
					&clientHostname, &clientPort, &backendStart, &state, &syncState, &replayLag); err != nil {
					continue
				}

				replicas = append(replicas, map[string]interface{}{
					"pid":              pid,
					"usename":          usename,
					"application_name": applicationName,
					"client_addr":      clientAddr,
					"client_hostname":  clientHostname,
					"client_port":      clientPort,
					"backend_start":    backendStart,
					"state":            state,
					"sync_state":       syncState,
					"replay_lag":       replayLag,
				})
			}

			var statusMsg string
			if len(replicas) == 0 {
				statusMsg = "No active replicas. This server is either not a primary, or has no connected standby servers."
			} else {
				statusMsg = fmt.Sprintf("Primary server with %d active replica(s)", len(replicas))
			}

			jsonData, err := json.MarshalIndent(map[string]interface{}{
				"replica_count": len(replicas),
				"replicas":      replicas,
				"status":        statusMsg,
				"description":   "Replication status for all connected standby servers. Monitor replay_lag to detect replication delays.",
			}, "", "  ")
			if err != nil {
				return mcp.ResourceContent{}, fmt.Errorf("failed to marshal JSON: %w", err)
			}

			return mcp.ResourceContent{
				URI:      "pg://stat/replication",
				MimeType: "application/json",
				Contents: []mcp.ContentItem{{Type: "text", Text: string(jsonData)}},
			}, nil
		},
	}
}
