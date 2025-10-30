# MCP Resources

Resources provide read-only access to PostgreSQL system information and statistics. All resources are accessed via the `read_resource` tool or through MCP protocol resource methods.

## System Information Resources

### pg://settings

Returns PostgreSQL server configuration parameters including current values, default values, pending changes, and descriptions.

**Access**: Read the resource to view all PostgreSQL configuration settings from pg_settings.

**Output**: JSON array with detailed information about each configuration parameter:
```json
[
  {
    "name": "max_connections",
    "current_value": "100",
    "category": "Connections and Authentication / Connection Settings",
    "description": "Sets the maximum number of concurrent connections.",
    "context": "postmaster",
    "type": "integer",
    "source": "configuration file",
    "min_value": "1",
    "max_value": "262143",
    "default_value": "100",
    "reset_value": "100",
    "pending_restart": false
  },
  ...
]
```

### pg://system_info

Returns PostgreSQL version, operating system, and build architecture information. Provides a quick and efficient way to check server version and platform details without executing natural language queries.

**Access**: Read the resource to view PostgreSQL system information.

**Output**: JSON object with detailed system information:
```json
{
  "postgresql_version": "15.4",
  "version_number": "150004",
  "full_version": "PostgreSQL 15.4 on x86_64-pc-linux-gnu, compiled by gcc (GCC) 11.2.0, 64-bit",
  "operating_system": "linux",
  "architecture": "x86_64-pc-linux-gnu",
  "compiler": "gcc (GCC) 11.2.0",
  "bit_version": "64-bit"
}
```

**Fields:**
- `postgresql_version`: Short version string (e.g., "15.4")
- `version_number`: Numeric version identifier (e.g., "150004")
- `full_version`: Complete version string from PostgreSQL version() function
- `operating_system`: Operating system (e.g., "linux", "darwin", "mingw32")
- `architecture`: Full architecture string (e.g., "x86_64-pc-linux-gnu", "aarch64-apple-darwin")
- `compiler`: Compiler used to build PostgreSQL (e.g., "gcc (GCC) 11.2.0")
- `bit_version`: Architecture bit version (e.g., "64-bit", "32-bit")

**Use Cases:**
- Quickly check PostgreSQL version without natural language queries
- Verify server platform and architecture
- Audit server build information
- Troubleshoot compatibility issues

## Statistics Resources

All statistics resources are compatible with PostgreSQL 14 and later. They provide real-time monitoring data from PostgreSQL's `pg_stat_*` system views.

### pg://stat/activity

Shows information about currently executing queries and connections. Essential for monitoring active database sessions and identifying long-running queries.

**Output**: JSON with current database activity:
```json
{
  "activity_count": 5,
  "activities": [
    {
      "datname": "mydb",
      "pid": 12345,
      "usename": "myuser",
      "application_name": "psql",
      "client_addr": "127.0.0.1",
      "backend_start": "2024-10-30T10:00:00",
      "state": "active",
      "query": "SELECT * FROM users"
    }
  ],
  "description": "Current database activity showing all non-idle connections and their queries."
}
```

**Use Cases:**
- Monitor currently executing queries
- Identify long-running queries
- Track connection counts
- Troubleshoot performance issues

### pg://stat/database

Provides database-wide statistics including transactions, cache hits, tuple operations, and deadlocks.

**Output**: JSON with database statistics:
```json
{
  "database_count": 3,
  "databases": [
    {
      "datname": "mydb",
      "numbackends": 5,
      "xact_commit": 150000,
      "xact_rollback": 100,
      "blks_read": 10000,
      "blks_hit": 990000,
      "cache_hit_ratio": "99.00%",
      "tup_returned": 1000000,
      "tup_fetched": 50000,
      "tup_inserted": 5000,
      "tup_updated": 2000,
      "tup_deleted": 100,
      "deadlocks": 0
    }
  ],
  "description": "Database-wide statistics for monitoring overall database health and performance."
}
```

**Key Metrics:**
- `cache_hit_ratio`: Percentage of reads served from cache (should be >99% for good performance)
- `xact_commit`/`xact_rollback`: Transaction statistics
- `deadlocks`: Number of deadlocks detected

### pg://stat/user_tables

Provides per-table statistics including scans, tuple operations, and vacuum/analyze activity. Essential for identifying tables that need maintenance or optimization.

**Output**: JSON with table statistics:
```json
{
  "table_count": 25,
  "tables": [
    {
      "schemaname": "public",
      "relname": "users",
      "seq_scan": 100,
      "seq_tup_read": 50000,
      "idx_scan": 10000,
      "idx_tup_fetch": 100000,
      "n_tup_ins": 5000,
      "n_tup_upd": 2000,
      "n_tup_del": 100,
      "n_live_tup": 10000,
      "n_dead_tup": 50,
      "last_vacuum": "2024-10-29T10:00:00",
      "last_autovacuum": "2024-10-30T02:00:00",
      "last_analyze": "2024-10-29T10:00:00"
    }
  ],
  "description": "Per-table statistics showing access patterns and maintenance activity."
}
```

**Use Cases:**
- Identify tables with high sequential scans (may need indexes)
- Monitor vacuum and analyze activity
- Track table growth and dead tuple accumulation
- Analyze access patterns

### pg://stat/user_indexes

Provides index usage statistics for identifying unused indexes that can be dropped and finding tables that might benefit from additional indexes.

**Output**: JSON with index statistics:
```json
{
  "index_count": 50,
  "indexes": [
    {
      "schemaname": "public",
      "relname": "users",
      "indexrelname": "users_pkey",
      "idx_scan": 10000,
      "idx_tup_read": 100000,
      "idx_tup_fetch": 95000,
      "usage_status": "active"
    },
    {
      "schemaname": "public",
      "relname": "orders",
      "indexrelname": "orders_old_idx",
      "idx_scan": 0,
      "idx_tup_read": 0,
      "idx_tup_fetch": 0,
      "usage_status": "unused"
    }
  ],
  "description": "Per-index statistics showing usage patterns and effectiveness. Indexes with idx_scan=0 may be candidates for removal."
}
```

**Usage Status Classifications:**
- `active`: idx_scan >= 100 (regularly used)
- `rarely_used`: 0 < idx_scan < 100 (infrequently used)
- `unused`: idx_scan = 0 (never used, candidate for removal)

**Use Cases:**
- Identify unused indexes to reduce storage and write overhead
- Find rarely used indexes that may be candidates for removal
- Verify that new indexes are being utilized
- Optimize query performance

### pg://stat/replication

Shows the status of replication connections from this primary server including WAL sender processes, replication lag, and sync state. Empty if the server is not a replication primary or has no active replicas.

**Output**: JSON with replication status:
```json
{
  "replica_count": 2,
  "replicas": [
    {
      "pid": 12345,
      "usename": "replicator",
      "application_name": "walreceiver",
      "client_addr": "192.168.1.100",
      "client_hostname": "replica1",
      "client_port": 5432,
      "backend_start": "2024-10-30T10:00:00",
      "state": "streaming",
      "sync_state": "async",
      "replay_lag": "00:00:02"
    }
  ],
  "status": "Primary server with 2 active replica(s)",
  "description": "Replication status for all connected standby servers. Monitor replay_lag to detect replication delays."
}
```

**Key Fields:**
- `state`: Replication state (startup, catchup, streaming, backup, stopping)
- `sync_state`: Synchronization state (sync, async, quorum, potential)
- `replay_lag`: Time delay between primary and replica

**Use Cases:**
- Monitor replication health
- Identify replication lag issues
- Verify replica connections
- Track synchronous vs asynchronous replicas

### pg://stat/bgwriter

Provides background writer and checkpoint statistics with automatic tuning recommendations based on observed patterns.

**Output**: JSON with background writer statistics:
```json
{
  "bgwriter": {
    "checkpoints_timed": 1000,
    "checkpoints_req": 50,
    "checkpoint_timed_ratio": "95.24%",
    "checkpoint_write_time_ms": 50000,
    "checkpoint_sync_time_ms": 1000,
    "buffers_checkpoint": 500000,
    "buffers_clean": 10000,
    "maxwritten_clean": 5,
    "buffers_backend": 5000,
    "buffers_backend_ratio": "0.97%",
    "buffers_backend_fsync": 0,
    "buffers_alloc": 100000,
    "stats_reset": "2024-10-01T00:00:00"
  },
  "recommendations": [
    "Background writer halted due to too many buffers - increase bgwriter_lru_maxpages"
  ],
  "description": "Background writer and checkpoint statistics for monitoring I/O patterns and tuning."
}
```

**Automatic Recommendations:**
- High requested checkpoints → increase `checkpoint_timeout` or `max_wal_size`
- High backend buffer writes → tune bgwriter parameters
- Non-zero `maxwritten_clean` → increase `bgwriter_lru_maxpages`

**Use Cases:**
- Tune checkpoint and background writer settings
- Optimize I/O performance
- Identify configuration issues
- Monitor buffer write patterns

### pg://stat/wal

Provides Write-Ahead Log (WAL) statistics including WAL records, full page images, bytes, buffers, and sync operations. Available in PostgreSQL 14 and later.

**Output**: JSON with WAL statistics:
```json
{
  "postgresql_version": 15,
  "wal": {
    "wal_records": 1000000,
    "wal_fpi": 50000,
    "wal_bytes": 10737418240,
    "wal_bytes_mb": "10240.00",
    "wal_bytes_gb": "10.00",
    "wal_buffers_full": 100,
    "wal_write": 50000,
    "wal_sync": 10000,
    "wal_write_time_ms": 5000,
    "wal_sync_time_ms": 1000,
    "avg_write_time_ms": "0.1000",
    "avg_sync_time_ms": "0.1000",
    "stats_reset": "2024-10-01T00:00:00"
  },
  "description": "WAL generation and synchronization statistics for monitoring transaction log activity."
}
```

**Version Compatibility:**
- PostgreSQL 14+: Full statistics available
- PostgreSQL 13 and earlier: Returns error message with version information

**Use Cases:**
- Monitor WAL generation patterns
- Analyze archive performance
- Understand transaction log activity
- Optimize WAL settings

## Accessing Resources

Resources can be accessed in two ways:

### 1. Via read_resource Tool

```json
{
  "uri": "pg://system_info"
}
```

Or list all resources:
```json
{
  "list": true
}
```

### 2. Via Natural Language (Claude Desktop)

Simply ask Claude to read a resource:
- "Show me the output from pg://system_info"
- "Read the pg://settings resource"
- "What's the current PostgreSQL version?" (uses pg://system_info)
- "Show me database statistics" (uses pg://stat/database)
