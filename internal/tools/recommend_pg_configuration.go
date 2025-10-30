package tools

import (
	"fmt"
	"math"
	"strings"

	"pgedge-postgres-mcp/internal/mcp"
)

// RecommendPGConfigurationTool creates a tool that recommends PostgreSQL configuration
// settings based on hardware resources and workload characteristics
func RecommendPGConfigurationTool() Tool {
	return Tool{
		Definition: mcp.Tool{
			Name:        "recommend_pg_configuration",
			Description: "Recommends PostgreSQL configuration settings as a STARTING POINT for NEW installations only. NOT for fine-tuning existing or pre-tuned systems. Based on server hardware (RAM, CPU cores, storage type), operating system, and expected workload type (OLTP, OLAP, or Mixed), this tool generates baseline configuration values following industry best practices and proven tuning methodologies. These are initial settings to begin with - production systems should be monitored and tuned based on actual workload patterns.",
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"total_ram_gb": map[string]interface{}{
						"type":        "number",
						"description": "Total system RAM in gigabytes (e.g., 16, 32, 64, 128)",
					},
					"cpu_cores": map[string]interface{}{
						"type":        "integer",
						"description": "Number of CPU cores available to PostgreSQL (e.g., 4, 8, 16, 32)",
					},
					"storage_type": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"HDD", "SSD", "NVMe"},
						"description": "Type of storage: HDD (spinning disk), SSD (solid state drive), or NVMe (high-performance SSD)",
					},
					"workload_type": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"OLTP", "OLAP", "Mixed"},
						"description": "Expected workload: OLTP (many short transactions), OLAP (complex analytical queries), or Mixed (combination of both)",
					},
					"vm_environment": map[string]interface{}{
						"type":        "boolean",
						"description": "Whether PostgreSQL is running in a virtual machine (true) or on bare metal (false). Default: false",
						"default":     false,
					},
					"separate_wal_disk": map[string]interface{}{
						"type":        "boolean",
						"description": "Whether WAL (Write-Ahead Log) is on a separate disk from data. Default: false",
						"default":     false,
					},
					"available_disk_space_gb": map[string]interface{}{
						"type":        "number",
						"description": "Available disk space in GB for WAL storage. Optional, used to calculate max_wal_size",
					},
				},
				Required: []string{"total_ram_gb", "cpu_cores", "storage_type", "workload_type"},
			},
		},
		Handler: func(args map[string]interface{}) (mcp.ToolResponse, error) {
			// Extract parameters
			totalRAMGB, ok := args["total_ram_gb"].(float64)
			if !ok {
				return mcp.ToolResponse{
					Content: []mcp.ContentItem{
						{Type: "text", Text: "Error: total_ram_gb must be a number"},
					},
					IsError: true,
				}, nil
			}

			cpuCores, ok := args["cpu_cores"].(float64)
			if !ok {
				return mcp.ToolResponse{
					Content: []mcp.ContentItem{
						{Type: "text", Text: "Error: cpu_cores must be a number"},
					},
					IsError: true,
				}, nil
			}

			storageType, ok := args["storage_type"].(string)
			if !ok {
				return mcp.ToolResponse{
					Content: []mcp.ContentItem{
						{Type: "text", Text: "Error: storage_type must be a string"},
					},
					IsError: true,
				}, nil
			}

			workloadType, ok := args["workload_type"].(string)
			if !ok {
				return mcp.ToolResponse{
					Content: []mcp.ContentItem{
						{Type: "text", Text: "Error: workload_type must be a string"},
					},
					IsError: true,
				}, nil
			}

			vmEnvironment := false
			if vm, ok := args["vm_environment"].(bool); ok {
				vmEnvironment = vm
			}

			separateWALDisk := false
			if wal, ok := args["separate_wal_disk"].(bool); ok {
				separateWALDisk = wal
			}

			availableDiskSpaceGB := 0.0
			if disk, ok := args["available_disk_space_gb"].(float64); ok {
				availableDiskSpaceGB = disk
			}

			// Validate inputs
			if totalRAMGB <= 0 {
				return mcp.ToolResponse{
					Content: []mcp.ContentItem{
						{Type: "text", Text: "Error: total_ram_gb must be greater than 0"},
					},
					IsError: true,
				}, nil
			}
			if cpuCores <= 0 {
				return mcp.ToolResponse{
					Content: []mcp.ContentItem{
						{Type: "text", Text: "Error: cpu_cores must be greater than 0"},
					},
					IsError: true,
				}, nil
			}

			config := generateConfiguration(totalRAMGB, int(cpuCores), storageType,
				workloadType, vmEnvironment, separateWALDisk, availableDiskSpaceGB)

			return mcp.ToolResponse{
				Content: []mcp.ContentItem{
					{
						Type: "text",
						Text: formatConfigurationOutput(config),
					},
				},
			}, nil
		},
	}
}

type configRecommendation struct {
	Parameter   string
	Value       string
	Explanation string
	Section     string
}

func generateConfiguration(ramGB float64, cpuCores int, storageType, workloadType string, vmEnv, separateWAL bool, diskSpaceGB float64) []configRecommendation {
	var recommendations []configRecommendation

	// Connection Management
	maxConnections := int(math.Max(float64(4*cpuCores), 100))
	recommendations = append(recommendations, configRecommendation{
		Parameter:   "max_connections",
		Value:       fmt.Sprintf("%d", maxConnections),
		Explanation: fmt.Sprintf("Calculated as max(4 × CPU cores, 100) = max(%d, 100). Consider using a connection pooler like pgbouncer if more connections are needed.", 4*cpuCores),
		Section:     "Connection Management",
	})

	recommendations = append(recommendations, configRecommendation{
		Parameter:   "password_encryption",
		Value:       "scram-sha-256",
		Explanation: "Modern secure password encryption method",
		Section:     "Connection Management",
	})

	// Memory Parameters
	sharedBuffers := calculateSharedBuffers(ramGB)
	sharedBuffersGB := sharedBuffers / 1024.0
	recommendations = append(recommendations, configRecommendation{
		Parameter:   "shared_buffers",
		Value:       fmt.Sprintf("%.0fGB", sharedBuffersGB),
		Explanation: fmt.Sprintf("Calculated based on %.0fGB total RAM. Beyond 64GB, there are diminishing returns due to overhead from maintaining large contiguous memory allocation.", ramGB),
		Section:     "Memory",
	})

	workMem := calculateWorkMem(ramGB, sharedBuffers, cpuCores, workloadType)
	recommendations = append(recommendations, configRecommendation{
		Parameter:   "work_mem",
		Value:       fmt.Sprintf("%dMB", workMem),
		Explanation: fmt.Sprintf("Calculated as (Total RAM - shared_buffers) / (16 × CPU cores). Adjusted for %s workload.", workloadType),
		Section:     "Memory",
	})

	maintenanceWorkMem := calculateMaintenanceWorkMem(ramGB, sharedBuffers, 5)
	recommendations = append(recommendations, configRecommendation{
		Parameter:   "maintenance_work_mem",
		Value:       fmt.Sprintf("%dMB", maintenanceWorkMem),
		Explanation: "Used for VACUUM, CREATE INDEX, ALTER TABLE operations. Capped at 1GB maximum.",
		Section:     "Memory",
	})

	effectiveIOConcurrency := 200
	if storageType == "HDD" {
		effectiveIOConcurrency = 2
	}
	recommendations = append(recommendations, configRecommendation{
		Parameter:   "effective_io_concurrency",
		Value:       fmt.Sprintf("%d", effectiveIOConcurrency),
		Explanation: fmt.Sprintf("Set to 200 for solid-state storage (%s), or number of spindles for HDD arrays.", storageType),
		Section:     "Memory",
	})

	effectiveCacheSize := calculateEffectiveCacheSize(ramGB, sharedBuffersGB)
	recommendations = append(recommendations, configRecommendation{
		Parameter:   "effective_cache_size",
		Value:       fmt.Sprintf("%.0fGB", effectiveCacheSize),
		Explanation: "Estimated as shared_buffers + OS buffer cache (approximately 50% of remaining RAM).",
		Section:     "Memory",
	})

	// WAL Parameters
	recommendations = append(recommendations, configRecommendation{
		Parameter:   "wal_compression",
		Value:       "on",
		Explanation: "Compresses full-page images in WAL to reduce storage and I/O.",
		Section:     "Write-Ahead Log (WAL)",
	})

	recommendations = append(recommendations, configRecommendation{
		Parameter:   "wal_log_hints",
		Value:       "on",
		Explanation: "Required for pg_rewind functionality.",
		Section:     "Write-Ahead Log (WAL)",
	})

	recommendations = append(recommendations, configRecommendation{
		Parameter:   "wal_buffers",
		Value:       "64MB",
		Explanation: "WAL segments are 16MB each by default, so buffering multiple segments is inexpensive.",
		Section:     "Write-Ahead Log (WAL)",
	})

	checkpointTimeout := "15min"
	if workloadType == "OLAP" {
		checkpointTimeout = "30min"
	}
	recommendations = append(recommendations, configRecommendation{
		Parameter:   "checkpoint_timeout",
		Value:       checkpointTimeout,
		Explanation: fmt.Sprintf("Longer timeout for %s workload reduces WAL volume but increases crash recovery time.", workloadType),
		Section:     "Write-Ahead Log (WAL)",
	})

	recommendations = append(recommendations, configRecommendation{
		Parameter:   "checkpoint_completion_target",
		Value:       "0.9",
		Explanation: "Spreads checkpoint writes over 90% of checkpoint interval to avoid I/O spikes.",
		Section:     "Write-Ahead Log (WAL)",
	})

	maxWALSize := calculateMaxWALSize(diskSpaceGB, workloadType)
	recommendations = append(recommendations, configRecommendation{
		Parameter:   "max_wal_size",
		Value:       maxWALSize,
		Explanation: "Calculated based on available disk space. Monitor pg_stat_bgwriter to tune checkpoints_timed vs checkpoints_req ratio.",
		Section:     "Write-Ahead Log (WAL)",
	})

	recommendations = append(recommendations, configRecommendation{
		Parameter:   "archive_mode",
		Value:       "on",
		Explanation: "Enables WAL archiving for backup and point-in-time recovery. Requires restart.",
		Section:     "Write-Ahead Log (WAL)",
	})

	recommendations = append(recommendations, configRecommendation{
		Parameter:   "archive_command",
		Value:       "'/bin/true'",
		Explanation: "Placeholder command. Replace with your actual archiving script or service.",
		Section:     "Write-Ahead Log (WAL)",
	})

	// VM-specific WAL settings
	if vmEnv {
		recommendations = append(recommendations, configRecommendation{
			Parameter:   "wal_recycle",
			Value:       "off",
			Explanation: "Disabled for VM environments to allow new WAL file creation instead of recycling.",
			Section:     "Write-Ahead Log (WAL)",
		})

		recommendations = append(recommendations, configRecommendation{
			Parameter:   "wal_init_zero",
			Value:       "off",
			Explanation: "Disabled for VM environments to write only final byte at creation (requires pre-allocated disk space).",
			Section:     "Write-Ahead Log (WAL)",
		})
	}

	// Query Planning
	randomPageCost := "4.0"
	if storageType == "SSD" || storageType == "NVMe" {
		randomPageCost = "1.1"
	}
	recommendations = append(recommendations, configRecommendation{
		Parameter:   "random_page_cost",
		Value:       randomPageCost,
		Explanation: fmt.Sprintf("Set to 1.1 for SSD/NVMe storage to reflect low random access cost. Default 4.0 for HDD."),
		Section:     "Query Planning",
	})

	recommendations = append(recommendations, configRecommendation{
		Parameter:   "cpu_tuple_cost",
		Value:       "0.03",
		Explanation: "Increased from default 0.01 for more realistic query costing on modern hardware.",
		Section:     "Query Planning",
	})

	// Logging & Monitoring
	recommendations = append(recommendations, configRecommendation{
		Parameter:   "logging_collector",
		Value:       "on",
		Explanation: "Enables background log collection process for stderr/csvlog output.",
		Section:     "Logging & Monitoring",
	})

	recommendations = append(recommendations, configRecommendation{
		Parameter:   "log_directory",
		Value:       "'/var/log/postgresql'",
		Explanation: "Place outside data directory to exclude logs from base backups.",
		Section:     "Logging & Monitoring",
	})

	recommendations = append(recommendations, configRecommendation{
		Parameter:   "log_checkpoints",
		Value:       "on",
		Explanation: "Logs checkpoint activity for monitoring I/O patterns.",
		Section:     "Logging & Monitoring",
	})

	recommendations = append(recommendations, configRecommendation{
		Parameter:   "log_min_duration_statement",
		Value:       "1000",
		Explanation: "Logs queries taking longer than 1 second (1000ms). Adjust based on workload expectations.",
		Section:     "Logging & Monitoring",
	})

	recommendations = append(recommendations, configRecommendation{
		Parameter:   "log_line_prefix",
		Value:       "'%m [%p]: u=[%u] db=[%d] app=[%a] c=[%h] s=[%c:%l] tx=[%v:%x]'",
		Explanation: "Detailed log line prefix including timestamp, process, user, database, application, connection, session, and transaction info.",
		Section:     "Logging & Monitoring",
	})

	recommendations = append(recommendations, configRecommendation{
		Parameter:   "log_lock_waits",
		Value:       "on",
		Explanation: "Logs when session waits longer than deadlock_timeout to acquire a lock.",
		Section:     "Logging & Monitoring",
	})

	recommendations = append(recommendations, configRecommendation{
		Parameter:   "log_statement",
		Value:       "ddl",
		Explanation: "Logs all DDL statements (CREATE, ALTER, DROP) for audit trail.",
		Section:     "Logging & Monitoring",
	})

	recommendations = append(recommendations, configRecommendation{
		Parameter:   "log_connections",
		Value:       "on",
		Explanation: "Logs each successful connection for security and auditing.",
		Section:     "Logging & Monitoring",
	})

	recommendations = append(recommendations, configRecommendation{
		Parameter:   "log_disconnections",
		Value:       "on",
		Explanation: "Logs session termination and duration for monitoring.",
		Section:     "Logging & Monitoring",
	})

	recommendations = append(recommendations, configRecommendation{
		Parameter:   "log_temp_files",
		Value:       "0",
		Explanation: "Logs all temporary file creation, indicating work_mem may need tuning.",
		Section:     "Logging & Monitoring",
	})

	// Autovacuum
	recommendations = append(recommendations, configRecommendation{
		Parameter:   "log_autovacuum_min_duration",
		Value:       "0",
		Explanation: "Logs all autovacuum activity for monitoring table maintenance.",
		Section:     "Autovacuum",
	})

	recommendations = append(recommendations, configRecommendation{
		Parameter:   "autovacuum_max_workers",
		Value:       "5",
		Explanation: "Increased from default 3 to enable more parallel vacuum operations.",
		Section:     "Autovacuum",
	})

	recommendations = append(recommendations, configRecommendation{
		Parameter:   "autovacuum_vacuum_cost_limit",
		Value:       "5000",
		Explanation: "Increased from default to allow autovacuum to do more I/O work per iteration.",
		Section:     "Autovacuum",
	})

	// Client Connection Defaults
	recommendations = append(recommendations, configRecommendation{
		Parameter:   "idle_in_transaction_session_timeout",
		Value:       "600000",
		Explanation: "Terminates sessions idle in transaction for more than 10 minutes (600000ms) to prevent lock buildup.",
		Section:     "Client Connection Defaults",
	})

	recommendations = append(recommendations, configRecommendation{
		Parameter:   "lc_messages",
		Value:       "C",
		Explanation: "Sets message locale to C for log analyzer compatibility.",
		Section:     "Client Connection Defaults",
	})

	recommendations = append(recommendations, configRecommendation{
		Parameter:   "shared_preload_libraries",
		Value:       "'pg_stat_statements'",
		Explanation: "Loads pg_stat_statements extension for query performance monitoring. Requires restart.",
		Section:     "Client Connection Defaults",
	})

	return recommendations
}

// calculateSharedBuffers calculates shared_buffers based on total RAM
func calculateSharedBuffers(ramGB float64) float64 {
	ramMB := ramGB * 1024.0
	base := ramMB / 4.0

	if ramGB < 3 {
		base = base * 0.5
	} else if ramGB < 8 {
		base = base * 0.75
	} else if ramGB > 64 {
		base = math.Max(16*1024.0, ramMB/6.0)
	}

	// Cap at 64GB
	return math.Min(base, 64*1024.0)
}

// calculateWorkMem implements the formula: (Total RAM - shared_buffers) / (16 × CPU cores)
func calculateWorkMem(ramGB, sharedBuffersMB float64, cpuCores int, workloadType string) int {
	ramMB := ramGB * 1024.0
	availableMB := ramMB - sharedBuffersMB
	workMemMB := availableMB / float64(16*cpuCores)

	// Adjust for workload type
	if workloadType == "OLAP" {
		workMemMB = workMemMB * 2.0 // OLAP benefits from more work_mem
	} else if workloadType == "OLTP" {
		workMemMB = workMemMB * 0.75 // OLTP uses less per operation
	}

	// Cap at reasonable maximums
	workMemMB = math.Min(workMemMB, 512.0)
	workMemMB = math.Max(workMemMB, 4.0) // Minimum 4MB

	return int(workMemMB)
}

// calculateMaintenanceWorkMem: 15% of (Total RAM - shared_buffers) / autovacuum_max_workers, capped at 1GB
func calculateMaintenanceWorkMem(ramGB, sharedBuffersMB float64, autovacuumWorkers int) int {
	ramMB := ramGB * 1024.0
	availableMB := ramMB - sharedBuffersMB
	maintenanceMB := (0.15 * availableMB) / float64(autovacuumWorkers)

	// Cap at 1GB
	maintenanceMB = math.Min(maintenanceMB, 1024.0)
	maintenanceMB = math.Max(maintenanceMB, 64.0) // Minimum 64MB

	return int(maintenanceMB)
}

// calculateEffectiveCacheSize: shared_buffers + estimated OS cache (50% of remaining RAM)
func calculateEffectiveCacheSize(ramGB, sharedBuffersGB float64) float64 {
	remainingRAM := ramGB - sharedBuffersGB
	osCacheGB := remainingRAM * 0.5
	return sharedBuffersGB + osCacheGB
}

// calculateMaxWALSize based on available disk space and workload type
func calculateMaxWALSize(diskSpaceGB float64, workloadType string) string {
	if diskSpaceGB == 0 {
		// Default recommendations if disk space not provided
		if workloadType == "OLAP" {
			return "16GB"
		}
		return "4GB"
	}

	// Use 50-75% of available space for high-performance systems
	maxWALGB := diskSpaceGB * 0.6

	// Workload adjustments
	if workloadType == "OLTP" {
		maxWALGB = maxWALGB * 0.5 // OLTP generates less WAL
	} else if workloadType == "OLAP" {
		maxWALGB = maxWALGB * 1.25 // OLAP can generate more WAL
	}

	// Cap at reasonable values
	maxWALGB = math.Max(maxWALGB, 1.0)   // Minimum 1GB
	maxWALGB = math.Min(maxWALGB, 200.0) // Maximum 200GB

	return fmt.Sprintf("%.0fGB", maxWALGB)
}

func formatConfigurationOutput(recommendations []configRecommendation) string {
	var output strings.Builder

	output.WriteString("PostgreSQL Configuration Recommendations for NEW Installations\n")
	output.WriteString("===============================================================\n\n")
	output.WriteString("⚠️  IMPORTANT: These recommendations are STARTING POINTS for NEW PostgreSQL deployments.\n")
	output.WriteString("⚠️  DO NOT apply to existing production systems or pre-tuned installations without careful review.\n")
	output.WriteString("⚠️  Production systems should be monitored and tuned based on actual workload patterns.\n\n")
	output.WriteString("Based on your hardware specifications and workload requirements,\n")
	output.WriteString("here are the recommended baseline PostgreSQL configuration parameters:\n\n")

	// Group by section
	sections := make(map[string][]configRecommendation)
	var sectionOrder []string
	seenSections := make(map[string]bool)

	for _, rec := range recommendations {
		if !seenSections[rec.Section] {
			sectionOrder = append(sectionOrder, rec.Section)
			seenSections[rec.Section] = true
		}
		sections[rec.Section] = append(sections[rec.Section], rec)
	}

	// Output by section
	for _, section := range sectionOrder {
		output.WriteString(fmt.Sprintf("## %s\n\n", section))
		for _, rec := range sections[section] {
			output.WriteString(fmt.Sprintf("**%s** = %s\n", rec.Parameter, rec.Value))
			output.WriteString(fmt.Sprintf("  %s\n\n", rec.Explanation))
		}
	}

	// Add additional recommendations
	output.WriteString("## Additional Recommendations\n\n")
	output.WriteString("### Operating System Tuning\n\n")
	output.WriteString("1. **Filesystem Settings**\n")
	output.WriteString("   - Use XFS filesystem for data and WAL directories\n")
	output.WriteString("   - Add 'noatime' to mount options in /etc/fstab\n")
	output.WriteString("   - Increase read-ahead from 128KB to 4096KB\n\n")

	output.WriteString("2. **I/O Scheduler**\n")
	output.WriteString("   - For HDD: Use 'mq-deadline' (RHEL 8+) or 'deadline' (RHEL 7)\n")
	output.WriteString("   - For SSD/NVMe: Use 'none' (RHEL 8+) or 'noop' (RHEL 7)\n\n")

	output.WriteString("3. **Memory Settings (Linux)**\n")
	output.WriteString("   - vm.dirty_bytes = 1073741824 (1GB, or set to storage cache size)\n")
	output.WriteString("   - vm.dirty_background_bytes = 268435456 (1/4 of dirty_bytes)\n\n")

	output.WriteString("### PostgreSQL Best Practices\n\n")
	output.WriteString("1. **Connection Pooling**\n")
	output.WriteString("   - Use pgbouncer or pgpool for connection pooling if you need more than the recommended max_connections\n\n")

	output.WriteString("2. **Monitoring**\n")
	output.WriteString("   - Monitor pg_stat_bgwriter for checkpoint tuning\n")
	output.WriteString("   - Use pg_stat_statements to identify slow queries\n")
	output.WriteString("   - Monitor autovacuum activity via logs\n\n")

	output.WriteString("3. **Storage Layout**\n")
	output.WriteString("   - Consider separate mount points for:\n")
	output.WriteString("     * Data directory (/pgdata)\n")
	output.WriteString("     * WAL directory (/pgwaldata)\n")
	output.WriteString("     * Indexes (optional, for specific workloads)\n\n")

	output.WriteString("4. **Backup and Recovery**\n")
	output.WriteString("   - Configure archive_command with your backup solution\n")
	output.WriteString("   - Test recovery procedures regularly\n")
	output.WriteString("   - Consider using pg_basebackup or WAL-based backup solutions\n\n")

	output.WriteString("### How to Apply These Settings\n\n")
	output.WriteString("1. Edit postgresql.conf file (or use ALTER SYSTEM commands)\n")
	output.WriteString("2. Parameters requiring restart: max_connections, shared_buffers, shared_preload_libraries, archive_mode\n")
	output.WriteString("3. Reload configuration: SELECT pg_reload_conf(); (for non-restart parameters)\n")
	output.WriteString("4. Restart PostgreSQL: sudo systemctl restart postgresql (for restart-required parameters)\n")
	output.WriteString("5. Verify settings: SELECT name, setting, unit FROM pg_settings WHERE name IN (...);\n\n")

	output.WriteString("### Important Notes\n\n")
	output.WriteString("⚠️  **CRITICAL**: These are BASELINE settings for NEW installations ONLY:\n")
	output.WriteString("- DO NOT blindly apply to existing production or pre-tuned PostgreSQL installations\n")
	output.WriteString("- These are starting points that require monitoring and adjustment based on actual workload\n")
	output.WriteString("- Existing tuned systems have been optimized for specific workloads - do not overwrite\n")
	output.WriteString("- Always test configuration changes in a non-production environment first\n")
	output.WriteString("- Monitor key metrics (cache hit ratio, checkpoint frequency, query performance) after deployment\n")
	output.WriteString("- Adjust parameters incrementally based on observed behavior over days/weeks\n")
	output.WriteString("- Consider consulting a PostgreSQL DBA for production fine-tuning\n")
	output.WriteString("- Consult PostgreSQL documentation for parameter-specific restrictions and dependencies\n\n")

	output.WriteString("Based on PostgreSQL tuning best practices and industry-standard formulas.\n")

	return output.String()
}
