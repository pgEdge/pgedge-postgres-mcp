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

// PGStatBgwriterResource provides background writer statistics
func PGStatBgwriterResource(dbClient *database.Client) Resource {
	return Resource{
		Definition: mcp.Resource{
			URI:         "pg://stat/bgwriter",
			Name:        "PostgreSQL Background Writer Statistics",
			Description: "Provides statistics about the background writer process including checkpoints, buffer writes, and backend fsync operations. Useful for tuning checkpoint and background writer settings for optimal I/O performance. High values of checkpoints_req or buffers_backend may indicate configuration issues.",
			MimeType:    "application/json",
		},
		Handler: func() (mcp.ResourceContent, error) {
			if !dbClient.IsMetadataLoaded() {
				return mcp.ResourceContent{
					URI: "pg://stat/bgwriter",
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

			var checkpointsTimed, checkpointsReq int64
			var checkpointWriteTime, checkpointSyncTime float64
			var buffersCheckpoint, buffersClean, maxwrittenClean int64
			var buffersBackend, buffersBackendFsync, buffersAlloc int64
			var statsReset *string

			err := pool.QueryRow(ctx, query).Scan(
				&checkpointsTimed, &checkpointsReq,
				&checkpointWriteTime, &checkpointSyncTime,
				&buffersCheckpoint, &buffersClean, &maxwrittenClean,
				&buffersBackend, &buffersBackendFsync, &buffersAlloc,
				&statsReset)
			if err != nil {
				return mcp.ResourceContent{}, fmt.Errorf("failed to query pg_stat_bgwriter: %w", err)
			}

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

			jsonData, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return mcp.ResourceContent{}, fmt.Errorf("failed to marshal JSON: %w", err)
			}

			return mcp.ResourceContent{
				URI:      "pg://stat/bgwriter",
				MimeType: "application/json",
				Contents: []mcp.ContentItem{{Type: "text", Text: string(jsonData)}},
			}, nil
		},
	}
}
