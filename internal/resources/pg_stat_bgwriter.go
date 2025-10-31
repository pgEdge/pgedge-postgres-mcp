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

	"pgedge-postgres-mcp/internal/database"
	"pgedge-postgres-mcp/internal/mcp"

	"github.com/jackc/pgx/v5"
)

// PGStatBgwriterResource provides background writer statistics
func PGStatBgwriterResource(dbClient *database.Client) Resource {
	return Resource{
		Definition: mcp.Resource{
			URI:         URIStatBgwriter,
			Name:        "PostgreSQL Background Writer Statistics",
			Description: "Provides statistics about the background writer process including checkpoints, buffer writes, and backend fsync operations. Useful for tuning checkpoint and background writer settings for optimal I/O performance. High values of checkpoints_req or buffers_backend may indicate configuration issues.",
			MimeType:    "application/json",
		},
		Handler: func() (mcp.ResourceContent, error) {
			// Check database readiness
			if !dbClient.IsMetadataLoaded() {
				return mcp.NewResourceError(URIStatBgwriter, mcp.DatabaseNotReadyErrorShort)
			}

			// Check PostgreSQL version
			version, err := getPostgreSQLMajorVersion(dbClient)
			if err != nil {
				version = 14 // Default to 14 if detection fails
			}

			var query string
			var processor func(pgx.Rows) (interface{}, error)

			if version >= 17 {
				// PostgreSQL 17+: checkpoint stats moved to pg_stat_checkpointer
				// Note: buffers_checkpoint renamed to buffers_written in pg_stat_checkpointer
				query = `
					WITH io_backend_stats AS (
						SELECT
							COALESCE(SUM(writes), 0) as buffers_backend,
							COALESCE(SUM(fsyncs), 0) as buffers_backend_fsync
						FROM pg_stat_io
						WHERE backend_type = 'client backend'
					)
					SELECT
						c.num_timed as checkpoints_timed,
						c.num_requested as checkpoints_req,
						c.write_time as checkpoint_write_time,
						c.sync_time as checkpoint_sync_time,
						c.buffers_written as buffers_checkpoint,
						b.buffers_clean,
						b.maxwritten_clean,
						io.buffers_backend,
						io.buffers_backend_fsync,
						b.buffers_alloc,
						b.stats_reset::text as stats_reset
					FROM pg_stat_checkpointer c, pg_stat_bgwriter b, io_backend_stats io`

				processor = func(rows pgx.Rows) (interface{}, error) {
					if !rows.Next() {
						return nil, fmt.Errorf("no data returned from pg_stat_checkpointer/pg_stat_bgwriter")
					}

					var checkpointsTimed, checkpointsReq int64
					var checkpointWriteTime, checkpointSyncTime float64
					var buffersCheckpoint, buffersClean, maxwrittenClean int64
					var buffersBackend, buffersBackendFsync, buffersAlloc int64
					var statsReset *string

					err := rows.Scan(
						&checkpointsTimed, &checkpointsReq,
						&checkpointWriteTime, &checkpointSyncTime,
						&buffersCheckpoint, &buffersClean, &maxwrittenClean,
						&buffersBackend, &buffersBackendFsync, &buffersAlloc,
						&statsReset)
					if err != nil {
						return nil, fmt.Errorf("failed to scan pg_stat_checkpointer/pg_stat_bgwriter: %w", err)
					}

					return processBackgroundWriterStats(checkpointsTimed, checkpointsReq,
						checkpointWriteTime, checkpointSyncTime,
						buffersCheckpoint, buffersClean, maxwrittenClean,
						buffersBackend, buffersBackendFsync, buffersAlloc, statsReset)
				}
			} else {
				// PostgreSQL 14-16: all stats in pg_stat_bgwriter
				query = `
					SELECT
						checkpoints_timed,
						checkpoints_req,
						checkpoint_write_time,
						checkpoint_sync_time,
						buffers_checkpoint,
						buffers_clean,
						maxwritten_clean,
						buffers_backend,
						buffers_backend_fsync,
						buffers_alloc,
						stats_reset::text as stats_reset
					FROM pg_stat_bgwriter`

				processor = func(rows pgx.Rows) (interface{}, error) {
					if !rows.Next() {
						return nil, fmt.Errorf("no data returned from pg_stat_bgwriter")
					}

					var checkpointsTimed, checkpointsReq int64
					var checkpointWriteTime, checkpointSyncTime float64
					var buffersCheckpoint, buffersClean, maxwrittenClean int64
					var buffersBackend, buffersBackendFsync, buffersAlloc int64
					var statsReset *string

					err := rows.Scan(
						&checkpointsTimed, &checkpointsReq,
						&checkpointWriteTime, &checkpointSyncTime,
						&buffersCheckpoint, &buffersClean, &maxwrittenClean,
						&buffersBackend, &buffersBackendFsync, &buffersAlloc,
						&statsReset)
					if err != nil {
						return nil, fmt.Errorf("failed to scan pg_stat_bgwriter: %w", err)
					}

					return processBackgroundWriterStats(checkpointsTimed, checkpointsReq,
						checkpointWriteTime, checkpointSyncTime,
						buffersCheckpoint, buffersClean, maxwrittenClean,
						buffersBackend, buffersBackendFsync, buffersAlloc, statsReset)
				}
			}

			return database.ExecuteResourceQuery(dbClient, URIStatBgwriter, query, processor)
		},
	}
}

// processBackgroundWriterStats processes the statistics and generates recommendations
func processBackgroundWriterStats(checkpointsTimed, checkpointsReq int64,
	checkpointWriteTime, checkpointSyncTime float64,
	buffersCheckpoint, buffersClean, maxwrittenClean int64,
	buffersBackend, buffersBackendFsync, buffersAlloc int64,
	statsReset *string) (interface{}, error) {

	// Calculate useful ratios
	totalCheckpoints := checkpointsTimed + checkpointsReq
	var timedCheckpointRatio float64
	if totalCheckpoints > 0 {
		timedCheckpointRatio = float64(checkpointsTimed) / float64(totalCheckpoints) * 100
	}

	totalBuffers := buffersCheckpoint + buffersClean + buffersBackend
	var backendBufferRatio float64
	if totalBuffers > 0 {
		backendBufferRatio = float64(buffersBackend) / float64(totalBuffers) * 100
	}

	// Provide tuning recommendations
	var recommendations []string
	if checkpointsReq > checkpointsTimed {
		recommendations = append(recommendations, "Consider increasing checkpoint_timeout or max_wal_size - too many requested checkpoints")
	}
	if backendBufferRatio > 10 {
		recommendations = append(recommendations, "High backend buffer writes - consider tuning bgwriter parameters")
	}
	if maxwrittenClean > 0 {
		recommendations = append(recommendations, "Background writer halted due to too many buffers - increase bgwriter_lru_maxpages")
	}

	bgwriter := map[string]interface{}{
		"checkpoints_timed":        checkpointsTimed,
		"checkpoints_req":          checkpointsReq,
		"checkpoint_timed_ratio":   fmt.Sprintf("%.2f%%", timedCheckpointRatio),
		"checkpoint_write_time_ms": checkpointWriteTime,
		"checkpoint_sync_time_ms":  checkpointSyncTime,
		"buffers_checkpoint":       buffersCheckpoint,
		"buffers_clean":            buffersClean,
		"maxwritten_clean":         maxwrittenClean,
		"buffers_backend":          buffersBackend,
		"buffers_backend_ratio":    fmt.Sprintf("%.2f%%", backendBufferRatio),
		"buffers_backend_fsync":    buffersBackendFsync,
		"buffers_alloc":            buffersAlloc,
		"stats_reset":              statsReset,
	}

	result := map[string]interface{}{
		"bgwriter":    bgwriter,
		"description": "Background writer and checkpoint statistics for monitoring I/O patterns and tuning.",
	}

	if len(recommendations) > 0 {
		result["recommendations"] = recommendations
	}

	return result, nil
}
