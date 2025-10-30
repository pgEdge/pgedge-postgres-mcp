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

// PGStatUserIndexesResource provides index usage statistics
func PGStatUserIndexesResource(dbClient *database.Client) Resource {
	return Resource{
		Definition: mcp.Resource{
			URI:         "pg://stat/user_indexes",
			Name:        "PostgreSQL Index Statistics",
			Description: "Provides statistics about index usage including scan counts and tuple operations. Essential for identifying unused indexes that can be dropped and finding tables that might benefit from additional indexes. Helps optimize query performance and reduce storage overhead.",
			MimeType:    "application/json",
		},
		Handler: func() (mcp.ResourceContent, error) {
			if !dbClient.IsMetadataLoaded() {
				return mcp.ResourceContent{
					URI: "pg://stat/user_indexes",
					Contents: []mcp.ContentItem{{Type: "text", Text: "Error: Database not ready"}},
				}, nil
			}

			pool := dbClient.GetPool()
			if pool == nil {
				return mcp.ResourceContent{}, fmt.Errorf("no connection pool available")
			}

			ctx := context.Background()
			query := `
				SELECT
					schemaname,
					relname,
					indexrelname,
					idx_scan,
					idx_tup_read,
					idx_tup_fetch
				FROM pg_stat_user_indexes
				ORDER BY schemaname, relname, indexrelname`

			rows, err := pool.Query(ctx, query)
			if err != nil {
				return mcp.ResourceContent{}, fmt.Errorf("failed to query pg_stat_user_indexes: %w", err)
			}
			defer rows.Close()

			var indexes []map[string]interface{}
			for rows.Next() {
				var schemaname, relname, indexrelname string
				var idxScan, idxTupRead, idxTupFetch int64

				if err := rows.Scan(&schemaname, &relname, &indexrelname,
					&idxScan, &idxTupRead, &idxTupFetch); err != nil {
					continue
				}

				// Mark potentially unused indexes
				var usage string
				if idxScan == 0 {
					usage = "unused"
				} else if idxScan < 100 {
					usage = "rarely_used"
				} else {
					usage = "active"
				}

				indexes = append(indexes, map[string]interface{}{
					"schemaname":    schemaname,
					"relname":       relname,
					"indexrelname":  indexrelname,
					"idx_scan":      idxScan,
					"idx_tup_read":  idxTupRead,
					"idx_tup_fetch": idxTupFetch,
					"usage_status":  usage,
				})
			}

			jsonData, err := json.MarshalIndent(map[string]interface{}{
				"index_count": len(indexes),
				"indexes":     indexes,
				"description": "Per-index statistics showing usage patterns and effectiveness. Indexes with idx_scan=0 may be candidates for removal.",
			}, "", "  ")
			if err != nil {
				return mcp.ResourceContent{}, fmt.Errorf("failed to marshal JSON: %w", err)
			}

			return mcp.ResourceContent{
				URI:      "pg://stat/user_indexes",
				MimeType: "application/json",
				Contents: []mcp.ContentItem{{Type: "text", Text: string(jsonData)}},
			}, nil
		},
	}
}
