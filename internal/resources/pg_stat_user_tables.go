package resources

import (
	"context"
	"encoding/json"
	"fmt"

	"pgedge-mcp/internal/database"
	"pgedge-mcp/internal/mcp"
)

// PGStatUserTablesResource provides table access statistics
func PGStatUserTablesResource(dbClient *database.Client) Resource {
	return Resource{
		Definition: mcp.Resource{
			URI:         "pg://stat/user_tables",
			Name:        "PostgreSQL Table Statistics",
			Description: "Shows statistics for user tables including sequential and index scans, tuple operations (inserts/updates/deletes), and vacuum/analyze activity. Critical for identifying tables that need optimization or indexing improvements.",
			MimeType:    "application/json",
		},
		Handler: func() (mcp.ResourceContent, error) {
			if !dbClient.IsMetadataLoaded() {
				return mcp.ResourceContent{
					URI: "pg://stat/user_tables",
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
					schemaname, relname, seq_scan, seq_tup_read,
					idx_scan, idx_tup_fetch, n_tup_ins, n_tup_upd,
					n_tup_del, n_live_tup, n_dead_tup
				FROM pg_stat_user_tables
				ORDER BY schemaname, relname
				LIMIT 100`

			rows, err := pool.Query(ctx, query)
			if err != nil {
				return mcp.ResourceContent{}, fmt.Errorf("failed to query: %w", err)
			}
			defer rows.Close()

			var tables []map[string]interface{}
			for rows.Next() {
				var schemaname, relname string
				var seqScan, seqTupRead, nTupIns, nTupUpd, nTupDel int64
				var idxScan, idxTupFetch *int64
				var nLiveTup, nDeadTup int64

				if err := rows.Scan(&schemaname, &relname, &seqScan, &seqTupRead,
					&idxScan, &idxTupFetch, &nTupIns, &nTupUpd, &nTupDel,
					&nLiveTup, &nDeadTup); err != nil {
					continue
				}

				tables = append(tables, map[string]interface{}{
					"schemaname":    schemaname,
					"relname":       relname,
					"seq_scan":      seqScan,
					"seq_tup_read":  seqTupRead,
					"idx_scan":      idxScan,
					"idx_tup_fetch": idxTupFetch,
					"n_tup_ins":     nTupIns,
					"n_tup_upd":     nTupUpd,
					"n_tup_del":     nTupDel,
					"n_live_tup":    nLiveTup,
					"n_dead_tup":    nDeadTup,
				})
			}

			jsonData, err := json.MarshalIndent(map[string]interface{}{
				"table_count": len(tables),
				"tables":      tables,
			}, "", "  ")
			if err != nil {
				return mcp.ResourceContent{}, fmt.Errorf("failed to marshal JSON: %w", err)
			}

			return mcp.ResourceContent{
				URI:      "pg://stat/user_tables",
				MimeType: "application/json",
				Contents: []mcp.ContentItem{{Type: "text", Text: string(jsonData)}},
			}, nil
		},
	}
}
