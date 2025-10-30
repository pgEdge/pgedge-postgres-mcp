package resources

import (
	"context"
	"encoding/json"
	"fmt"

	"pgedge-postgres-mcp/internal/database"
	"pgedge-postgres-mcp/internal/mcp"
)

// PGStatDatabaseResource provides database-wide statistics
func PGStatDatabaseResource(dbClient *database.Client) Resource {
	return Resource{
		Definition: mcp.Resource{
			URI:         "pg://stat/database",
			Name:        "PostgreSQL Database Statistics",
			Description: "Provides cumulative statistics for each database including transaction counts, block reads/writes, tuple operations, conflicts, and deadlocks. Essential for understanding database-level performance patterns and identifying I/O bottlenecks.",
			MimeType:    "application/json",
		},
		Handler: func() (mcp.ResourceContent, error) {
			if !dbClient.IsMetadataLoaded() {
				return mcp.ResourceContent{
					URI: "pg://stat/database",
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
					datname, numbackends, xact_commit, xact_rollback,
					blks_read, blks_hit, tup_returned, tup_fetched,
					tup_inserted, tup_updated, tup_deleted, conflicts,
					temp_files, temp_bytes, deadlocks
				FROM pg_stat_database
				WHERE datname IS NOT NULL
				ORDER BY datname`

			rows, err := pool.Query(ctx, query)
			if err != nil {
				return mcp.ResourceContent{}, fmt.Errorf("failed to query: %w", err)
			}
			defer rows.Close()

			var databases []map[string]interface{}
			for rows.Next() {
				var datname *string
				var numbackends int
				var xactCommit, xactRollback, blksRead, blksHit int64
				var tupReturned, tupFetched, tupInserted, tupUpdated, tupDeleted int64
				var conflicts, tempFiles, deadlocks int64
				var tempBytes int64

				if err := rows.Scan(&datname, &numbackends, &xactCommit, &xactRollback,
					&blksRead, &blksHit, &tupReturned, &tupFetched, &tupInserted,
					&tupUpdated, &tupDeleted, &conflicts, &tempFiles, &tempBytes,
					&deadlocks); err != nil {
					continue
				}

				var cacheHitRatio float64
				totalReads := blksRead + blksHit
				if totalReads > 0 {
					cacheHitRatio = float64(blksHit) / float64(totalReads) * 100
				}

				databases = append(databases, map[string]interface{}{
					"datname":         datname,
					"numbackends":     numbackends,
					"xact_commit":     xactCommit,
					"xact_rollback":   xactRollback,
					"blks_read":       blksRead,
					"blks_hit":        blksHit,
					"cache_hit_ratio": fmt.Sprintf("%.2f%%", cacheHitRatio),
					"tup_returned":    tupReturned,
					"tup_fetched":     tupFetched,
					"tup_inserted":    tupInserted,
					"tup_updated":     tupUpdated,
					"tup_deleted":     tupDeleted,
					"conflicts":       conflicts,
					"temp_files":      tempFiles,
					"temp_bytes":      tempBytes,
					"deadlocks":       deadlocks,
				})
			}

			jsonData, err := json.MarshalIndent(map[string]interface{}{
				"database_count": len(databases),
				"databases":      databases,
			}, "", "  ")
			if err != nil {
				return mcp.ResourceContent{}, fmt.Errorf("failed to marshal JSON: %w", err)
			}

			return mcp.ResourceContent{
				URI:      "pg://stat/database",
				MimeType: "application/json",
				Contents: []mcp.ContentItem{{Type: "text", Text: string(jsonData)}},
			}, nil
		},
	}
}
