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
	"strconv"
	"strings"

	"pgedge-postgres-mcp/internal/database"
	"pgedge-postgres-mcp/internal/mcp"
)

// getPostgreSQLMajorVersion extracts the major version number from the database
func getPostgreSQLMajorVersion(dbClient *database.Client) (int, error) {
	if !dbClient.IsMetadataLoaded() {
		return 0, fmt.Errorf("database not ready")
	}

	pool := dbClient.GetPool()
	if pool == nil {
		return 0, fmt.Errorf("no connection pool available")
	}

	ctx := context.Background()
	var versionStr string
	err := pool.QueryRow(ctx, "SELECT current_setting('server_version_num')").Scan(&versionStr)
	if err != nil {
		return 0, fmt.Errorf("failed to get version: %w", err)
	}

	// server_version_num returns a 6-digit number like 140000, 150004, etc.
	// The first two digits are the major version
	versionNum, err := strconv.Atoi(versionStr)
	if err != nil {
		return 0, fmt.Errorf("failed to parse version: %w", err)
	}

	return versionNum / 10000, nil
}

// PGStatWALResource provides WAL statistics (PostgreSQL 14+)
func PGStatWALResource(dbClient *database.Client) Resource {
	return Resource{
		Definition: mcp.Resource{
			URI:         "pg://stat/wal",
			Name:        "PostgreSQL WAL Statistics",
			Description: "Provides Write-Ahead Log (WAL) statistics including WAL records, FPI, bytes, buffers, and sync operations. Available in PostgreSQL 14 and later. Useful for understanding WAL generation patterns, archive performance, and transaction log activity. Returns version error for PostgreSQL 13 and earlier.",
			MimeType:    "application/json",
		},
		Handler: func() (mcp.ResourceContent, error) {
			if !dbClient.IsMetadataLoaded() {
				return mcp.ResourceContent{
					URI: "pg://stat/wal",
					Contents: []mcp.ContentItem{{Type: "text", Text: "Error: Database not ready"}},
				}, nil
			}

			pool := dbClient.GetPool()
			if pool == nil {
				return mcp.ResourceContent{}, fmt.Errorf("no connection pool available")
			}

			ctx := context.Background()

			// Check PostgreSQL version
			version, err := getPostgreSQLMajorVersion(dbClient)
			if err != nil {
				version = 14 // Default to 14 if detection fails
			}

			if version < 14 {
				errorData := map[string]interface{}{
					"error":              fmt.Sprintf("pg_stat_wal is not available in PostgreSQL %d", version),
					"postgresql_version": version,
					"required_version":   "14+",
					"description":        "The pg_stat_wal view was introduced in PostgreSQL 14. Please upgrade to access WAL statistics.",
				}

				jsonData, _ := json.MarshalIndent(errorData, "", "  ")
				return mcp.ResourceContent{
					URI:      "pg://stat/wal",
					MimeType: "application/json",
					Contents: []mcp.ContentItem{{Type: "text", Text: string(jsonData)}},
				}, nil
			}

			query := `
				SELECT
					wal_records,
					wal_fpi,
					wal_bytes,
					wal_buffers_full,
					wal_write,
					wal_sync,
					wal_write_time,
					wal_sync_time,
					stats_reset::text as stats_reset
				FROM pg_stat_wal`

			var walRecords, walFpi, walBytes, walBuffersFull int64
			var walWrite, walSync int64
			var walWriteTime, walSyncTime float64
			var statsReset *string

			err = pool.QueryRow(ctx, query).Scan(
				&walRecords, &walFpi, &walBytes, &walBuffersFull,
				&walWrite, &walSync, &walWriteTime, &walSyncTime,
				&statsReset)
			if err != nil {
				// If the view doesn't exist, return a friendly error
				if strings.Contains(err.Error(), "does not exist") {
					errorData := map[string]interface{}{
						"error":              "pg_stat_wal view does not exist",
						"postgresql_version": version,
						"description":        "The pg_stat_wal view may not be available in your PostgreSQL installation.",
					}
					jsonData, _ := json.MarshalIndent(errorData, "", "  ")
					return mcp.ResourceContent{
						URI:      "pg://stat/wal",
						MimeType: "application/json",
						Contents: []mcp.ContentItem{{Type: "text", Text: string(jsonData)}},
					}, nil
				}
				return mcp.ResourceContent{}, fmt.Errorf("failed to query pg_stat_wal: %w", err)
			}

			// Calculate derived metrics
			walBytesMB := float64(walBytes) / 1024 / 1024
			walBytesGB := walBytesMB / 1024

			var avgWriteTime, avgSyncTime float64
			if walWrite > 0 {
				avgWriteTime = walWriteTime / float64(walWrite)
			}
			if walSync > 0 {
				avgSyncTime = walSyncTime / float64(walSync)
			}

			wal := map[string]interface{}{
				"wal_records":       walRecords,
				"wal_fpi":           walFpi,
				"wal_bytes":         walBytes,
				"wal_bytes_mb":      fmt.Sprintf("%.2f", walBytesMB),
				"wal_bytes_gb":      fmt.Sprintf("%.2f", walBytesGB),
				"wal_buffers_full":  walBuffersFull,
				"wal_write":         walWrite,
				"wal_sync":          walSync,
				"wal_write_time_ms": walWriteTime,
				"wal_sync_time_ms":  walSyncTime,
				"avg_write_time_ms": fmt.Sprintf("%.4f", avgWriteTime),
				"avg_sync_time_ms":  fmt.Sprintf("%.4f", avgSyncTime),
				"stats_reset":       statsReset,
			}

			jsonData, err := json.MarshalIndent(map[string]interface{}{
				"postgresql_version": version,
				"wal":                wal,
				"description":        "WAL generation and synchronization statistics for monitoring transaction log activity.",
			}, "", "  ")
			if err != nil {
				return mcp.ResourceContent{}, fmt.Errorf("failed to marshal JSON: %w", err)
			}

			return mcp.ResourceContent{
				URI:      "pg://stat/wal",
				MimeType: "application/json",
				Contents: []mcp.ContentItem{{Type: "text", Text: string(jsonData)}},
			}, nil
		},
	}
}
