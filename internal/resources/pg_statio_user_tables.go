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
	"os"

	"pgedge-postgres-mcp/internal/database"
	"pgedge-postgres-mcp/internal/mcp"

	"github.com/jackc/pgx/v5"
)

// PGStatIOUserTablesResource provides I/O statistics for user tables
func PGStatIOUserTablesResource(dbClient *database.Client) Resource {
	return Resource{
		Definition: mcp.Resource{
			URI:         URIStatIOUserTables,
			Name:        "PostgreSQL Table I/O Statistics",
			Description: "Shows disk block I/O statistics for user tables including heap, index, TOAST, and TOAST index blocks. Tracks blocks read from disk vs. cache hits. Essential for identifying I/O bottlenecks and cache efficiency. High read counts indicate potential need for more memory or query optimization.",
			MimeType:    "application/json",
		},
		Handler: func() (mcp.ResourceContent, error) {
			query := fmt.Sprintf(`
				SELECT
					schemaname, relname,
					heap_blks_read, heap_blks_hit,
					idx_blks_read, idx_blks_hit,
					toast_blks_read, toast_blks_hit,
					tidx_blks_read, tidx_blks_hit
				FROM pg_statio_user_tables
				ORDER BY (heap_blks_read + idx_blks_read + COALESCE(toast_blks_read, 0) + COALESCE(tidx_blks_read, 0)) DESC
				LIMIT %d`, DefaultQueryLimit)

			processor := func(rows pgx.Rows) (interface{}, error) {
				var tables []map[string]interface{}

				for rows.Next() {
					var schemaname, relname string
					var heapBlksRead, heapBlksHit int64
					var idxBlksRead, idxBlksHit int64
					var toastBlksRead, toastBlksHit *int64 // Nullable - NULL when no TOAST table
					var tidxBlksRead, tidxBlksHit *int64   // Nullable - NULL when no TOAST index

					if err := rows.Scan(&schemaname, &relname,
						&heapBlksRead, &heapBlksHit,
						&idxBlksRead, &idxBlksHit,
						&toastBlksRead, &toastBlksHit,
						&tidxBlksRead, &tidxBlksHit); err != nil {
						fmt.Fprintf(os.Stderr, "WARNING: Failed to scan row in pg_statio_user_tables: %v\n", err)
						continue
					}

					// Calculate cache hit ratios
					totalHeapBlks := heapBlksRead + heapBlksHit
					totalIdxBlks := idxBlksRead + idxBlksHit

					var heapHitRatio, idxHitRatio, toastHitRatio, tidxHitRatio *float64

					if totalHeapBlks > 0 {
						ratio := float64(heapBlksHit) / float64(totalHeapBlks) * 100
						heapHitRatio = &ratio
					}
					if totalIdxBlks > 0 {
						ratio := float64(idxBlksHit) / float64(totalIdxBlks) * 100
						idxHitRatio = &ratio
					}

					// Handle nullable TOAST columns
					if toastBlksRead != nil && toastBlksHit != nil {
						totalToastBlks := *toastBlksRead + *toastBlksHit
						if totalToastBlks > 0 {
							ratio := float64(*toastBlksHit) / float64(totalToastBlks) * 100
							toastHitRatio = &ratio
						}
					}

					if tidxBlksRead != nil && tidxBlksHit != nil {
						totalTidxBlks := *tidxBlksRead + *tidxBlksHit
						if totalTidxBlks > 0 {
							ratio := float64(*tidxBlksHit) / float64(totalTidxBlks) * 100
							tidxHitRatio = &ratio
						}
					}

					tables = append(tables, map[string]interface{}{
						"schemaname":       schemaname,
						"relname":          relname,
						"heap_blks_read":   heapBlksRead,
						"heap_blks_hit":    heapBlksHit,
						"heap_hit_ratio":   heapHitRatio,
						"idx_blks_read":    idxBlksRead,
						"idx_blks_hit":     idxBlksHit,
						"idx_hit_ratio":    idxHitRatio,
						"toast_blks_read":  toastBlksRead,
						"toast_blks_hit":   toastBlksHit,
						"toast_hit_ratio":  toastHitRatio,
						"tidx_blks_read":   tidxBlksRead,
						"tidx_blks_hit":    tidxBlksHit,
						"tidx_hit_ratio":   tidxHitRatio,
					})
				}

				return map[string]interface{}{
					"table_count": len(tables),
					"tables":      tables,
					"description": "Per-table I/O statistics showing disk reads vs cache hits. Tables ordered by total disk reads (highest first). Hit ratios above 95% indicate good cache efficiency.",
				}, nil
			}

			return database.ExecuteResourceQuery(dbClient, URIStatIOUserTables, query, processor)
		},
	}
}
