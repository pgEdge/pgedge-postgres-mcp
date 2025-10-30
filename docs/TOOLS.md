# MCP Tools

The pgEdge MCP Server provides six tools that enable natural language database interaction, configuration management, and maintenance analysis.

## Available Tools

### query_database

Executes a natural language query against the PostgreSQL database. Supports dynamic connection strings to query different databases.

**Input Examples**:

Basic query:
```json
{
  "query": "Show me all users created in the last week"
}
```

Query with temporary connection:
```json
{
  "query": "Show me table list at postgres://localhost:5433/other_db"
}
```

Set new default connection:
```json
{
  "query": "Set default database to postgres://localhost/analytics"
}
```

**Output**:
```
Natural Language Query: Show me all users created in the last week

Generated SQL:
SELECT * FROM users WHERE created_at >= NOW() - INTERVAL '7 days' ORDER BY created_at DESC

Results (15 rows):
[
  {
    "id": 123,
    "username": "john_doe",
    "created_at": "2024-10-25T14:30:00Z",
    ...
  },
  ...
]
```

**Security**: All queries are executed in read-only transactions using `SET TRANSACTION READ ONLY`, preventing INSERT, UPDATE, DELETE, and other data modifications. Write operations will fail with "cannot execute ... in a read-only transaction".

### get_schema_info

Retrieves database schema information including tables, views, columns, data types, and comments from pg_description.

**Input** (optional):
```json
{
  "schema_name": "public"
}
```

**Output**:
```
Database Schema Information:
============================

public.users (TABLE)
  Description: User accounts and authentication
  Columns:
    - id: bigint
    - username: character varying(255)
      Description: Unique username for login
    - created_at: timestamp with time zone (nullable)
      Description: Account creation timestamp
    ...
```

### set_pg_configuration

Sets PostgreSQL server configuration parameters using ALTER SYSTEM SET. Changes persist across server restarts. Some parameters require a restart to take effect.

**Input**:
```json
{
  "parameter": "max_connections",
  "value": "200"
}
```

Use "DEFAULT" as the value to reset to default:
```json
{
  "parameter": "work_mem",
  "value": "DEFAULT"
}
```

**Output**:
```
Configuration parameter 'max_connections' updated successfully.

Parameter: max_connections
Description: Sets the maximum number of concurrent connections
Type: integer
Context: postmaster

Previous value: 100
New value: 200

⚠️  WARNING: This parameter requires a server restart to take effect.
The change has been saved to postgresql.auto.conf but will not be active until the server is restarted.

SQL executed: ALTER SYSTEM SET max_connections = '200'
```

**Security Considerations**:
- Requires PostgreSQL superuser privileges
- Changes persist across server restarts via `postgresql.auto.conf`
- Test configuration changes in development before applying to production
- Some parameters require a server restart to take effect
- Keep backups of configuration files before making changes

### recommend_pg_configuration

Recommends PostgreSQL configuration settings as a **STARTING POINT for NEW installations ONLY**. This tool is **NOT intended for fine-tuning existing or pre-tuned PostgreSQL systems**.

Based on server hardware, operating system, and expected workload characteristics, it generates baseline configuration values following industry best practices and proven tuning methodologies. These are initial settings to begin with - production systems must be monitored and tuned based on actual workload patterns over time.

**⚠️ CRITICAL WARNING**: Do not blindly apply these recommendations to existing production databases or systems that have already been tuned. These settings are for fresh PostgreSQL installations only.

**Input**:
```json
{
  "total_ram_gb": 32,
  "cpu_cores": 8,
  "storage_type": "SSD",
  "workload_type": "Mixed",
  "vm_environment": false,
  "separate_wal_disk": false,
  "available_disk_space_gb": 500
}
```

**Parameters**:
- `total_ram_gb` (required): Total system RAM in gigabytes (e.g., 16, 32, 64, 128)
- `cpu_cores` (required): Number of CPU cores available to PostgreSQL (e.g., 4, 8, 16, 32)
- `storage_type` (required): Type of storage - `HDD` (spinning disk), `SSD` (solid state drive), or `NVMe` (high-performance SSD)
- `workload_type` (required): Expected workload - `OLTP` (many short transactions), `OLAP` (complex analytical queries), or `Mixed` (combination of both)
- `vm_environment` (optional): Whether PostgreSQL is running in a virtual machine (default: false)
- `separate_wal_disk` (optional): Whether WAL is on a separate disk from data (default: false)
- `available_disk_space_gb` (optional): Available disk space in GB for WAL storage, used to calculate max_wal_size

**Output**:
```
PostgreSQL Configuration Recommendations for NEW Installations
===============================================================

⚠️  IMPORTANT: These recommendations are STARTING POINTS for NEW PostgreSQL deployments.
⚠️  DO NOT apply to existing production systems or pre-tuned installations without careful review.
⚠️  Production systems should be monitored and tuned based on actual workload patterns.

Based on your hardware specifications and workload requirements,
here are the recommended baseline PostgreSQL configuration parameters:

## Connection Management

**max_connections** = 100
  Calculated as max(4 × CPU cores, 100) = max(32, 100). Consider using a connection pooler like pgbouncer if more connections are needed.

**password_encryption** = scram-sha-256
  Modern secure password encryption method

## Memory

**shared_buffers** = 8GB
  Calculated based on 32GB total RAM. Beyond 64GB, there are diminishing returns due to overhead from maintaining large contiguous memory allocation.

**work_mem** = 23MB
  Calculated as (Total RAM - shared_buffers) / (16 × CPU cores). Adjusted for Mixed workload.

**maintenance_work_mem** = 732MB
  Used for VACUUM, CREATE INDEX, ALTER TABLE operations. Capped at 1GB maximum.

**effective_io_concurrency** = 200
  Set to 200 for solid-state storage (SSD), or number of spindles for HDD arrays.

**effective_cache_size** = 20GB
  Estimated as shared_buffers + OS buffer cache (approximately 50% of remaining RAM).

## Write-Ahead Log (WAL)

**wal_compression** = on
  Compresses full-page images in WAL to reduce storage and I/O.

**wal_log_hints** = on
  Required for pg_rewind functionality.

**wal_buffers** = 64MB
  WAL segments are 16MB each by default, so buffering multiple segments is inexpensive.

**checkpoint_timeout** = 15min
  Longer timeout for Mixed workload reduces WAL volume but increases crash recovery time.

**checkpoint_completion_target** = 0.9
  Spreads checkpoint writes over 90% of checkpoint interval to avoid I/O spikes.

**max_wal_size** = 150GB
  Calculated based on available disk space. Monitor pg_stat_bgwriter to tune checkpoints_timed vs checkpoints_req ratio.

**archive_mode** = on
  Enables WAL archiving for backup and point-in-time recovery. Requires restart.

**archive_command** = '/bin/true'
  Placeholder command. Replace with your actual archiving script or service.

## Query Planning

**random_page_cost** = 1.1
  Set to 1.1 for SSD/NVMe storage to reflect low random access cost. Default 4.0 for HDD.

**cpu_tuple_cost** = 0.03
  Increased from default 0.01 for more realistic query costing on modern hardware.

## Logging & Monitoring

**logging_collector** = on
  Enables background log collection process for stderr/csvlog output.

**log_directory** = '/var/log/postgresql'
  Place outside data directory to exclude logs from base backups.

**log_checkpoints** = on
  Logs checkpoint activity for monitoring I/O patterns.

**log_min_duration_statement** = 1000
  Logs queries taking longer than 1 second (1000ms). Adjust based on workload expectations.

... [additional parameters]

## Additional Recommendations

### Operating System Tuning

1. **Filesystem Settings**
   - Use XFS filesystem for data and WAL directories
   - Add 'noatime' to mount options in /etc/fstab
   - Increase read-ahead from 128KB to 4096KB

2. **I/O Scheduler**
   - For HDD: Use 'mq-deadline' (RHEL 8+) or 'deadline' (RHEL 7)
   - For SSD/NVMe: Use 'none' (RHEL 8+) or 'noop' (RHEL 7)

3. **Memory Settings (Linux)**
   - vm.dirty_bytes = 1073741824 (1GB, or set to storage cache size)
   - vm.dirty_background_bytes = 268435456 (1/4 of dirty_bytes)

### PostgreSQL Best Practices

1. **Connection Pooling**
   - Use pgbouncer or pgpool for connection pooling if you need more than the recommended max_connections

2. **Monitoring**
   - Monitor pg_stat_bgwriter for checkpoint tuning
   - Use pg_stat_statements to identify slow queries
   - Monitor autovacuum activity via logs

3. **Storage Layout**
   - Consider separate mount points for:
     * Data directory (/pgdata)
     * WAL directory (/pgwaldata)
     * Indexes (optional, for specific workloads)

4. **Backup and Recovery**
   - Configure archive_command with your backup solution
   - Test recovery procedures regularly
   - Consider using pg_basebackup or WAL-based backup solutions
```

**Use Cases**:
- Fresh PostgreSQL installations requiring initial configuration
- Setting up NEW development, testing, or staging environments
- Planning hardware specifications for new deployments
- Learning PostgreSQL configuration best practices and parameter relationships
- Establishing baseline settings before workload-specific optimization

**NOT Suitable For**:
- Fine-tuning existing production systems
- Optimizing pre-tuned PostgreSQL installations
- Systems that have already been customized for specific workloads
- Making incremental adjustments to running databases
- Performance troubleshooting of existing installations

**Important Notes**:
- **⚠️ CRITICAL**: These are BASELINE settings for NEW installations ONLY
- DO NOT blindly apply to existing production or pre-tuned PostgreSQL installations
- These are starting points that require monitoring and adjustment based on actual workload
- Existing tuned systems have been optimized for specific workloads - do not overwrite
- Always test configuration changes in a non-production environment first
- Monitor key metrics (cache hit ratio, checkpoint frequency, query performance) after deployment
- Adjust parameters incrementally based on observed behavior over days/weeks
- Consider consulting a PostgreSQL DBA for production fine-tuning
- Some parameters require a server restart to take effect
- Recommendations based on PostgreSQL tuning best practices and industry-standard formulas

### analyze_bloat

Analyzes PostgreSQL tables and indexes for bloat, providing statistics and recommendations for vacuum and reindex operations. Uses `pg_stat_user_tables` and `pg_stat_user_indexes` to identify dead tuples, maintenance history, and index health.

**Input Examples**:

Analyze all tables with default 5% threshold:
```json
{}
```

Analyze tables with high bloat only (20% or more):
```json
{
  "min_dead_tuple_percent": 20.0
}
```

Analyze specific table:
```json
{
  "schema_name": "public",
  "table_name": "users"
}
```

Full analysis with index details:
```json
{
  "schema_name": "public",
  "min_dead_tuple_percent": 0.0,
  "include_indexes": true
}
```

**Parameters**:
- `schema_name` (optional): Filter to specific schema (default: all schemas)
- `table_name` (optional): Filter to specific table (requires schema_name)
- `min_dead_tuple_percent` (optional): Minimum dead tuple percentage to show (default: 5.0, range: 0-100)
- `include_indexes` (optional): Include index bloat analysis (default: true)

**Output**:
```
Table Bloat Analysis
====================

Schema: public
Table: orders
  Live tuples: 1,250,000
  Dead tuples: 312,500 (20.00%)
  Total size: 1.2 GB

  Maintenance history:
     - Last VACUUM: 2024-10-15 08:30:00 (15 days ago)
     - Last ANALYZE: 2024-10-20 14:45:00 (10 days ago)
     - Modifications since analyze: 45,678

  Lifetime stats:
     - Inserts: 1,500,000, Updates: 850,000, Deletes: 100,000
     - Vacuums: 12 manual + 45 auto
     - Analyzes: 8 manual + 32 auto

  Recommendations:
     - ⚠️  URGENT: Run VACUUM immediately - high dead tuple percentage
     - Run ANALYZE to update query planner statistics

Schema: public
Table: user_sessions
  Live tuples: 50,000
  Dead tuples: 8,500 (14.54%)
  Total size: 125 MB

  Maintenance history:
     - Last VACUUM: 2024-10-28 03:15:00 (2 days ago)
     - Last ANALYZE: 2024-10-28 03:15:00 (2 days ago)
     - Modifications since analyze: 2,341

  Lifetime stats:
     - Inserts: 125,000, Updates: 85,000, Deletes: 60,000
     - Vacuums: 5 manual + 78 auto
     - Analyzes: 3 manual + 52 auto

  Recommendations:
     - Run VACUUM soon to reclaim space

Index Analysis for: public.orders
==================================

  idx_orders_customer (btree on customer_id)
     Size: 256 MB | Scans: 12,450 | Tuples read: 8,500,000 | Fetched: 3,200,000

  idx_orders_date (btree on order_date)
     Size: 198 MB | Scans: 8,932 | Tuples read: 5,200,000 | Fetched: 2,100,000

  idx_orders_status (btree on status)
     ⚠️  Unused index - 0 scans | Size: 145 MB

Summary: 3 tables analyzed
```

**Recommendations Thresholds**:
- **>50% dead tuples**: Urgent VACUUM + consider VACUUM FULL during maintenance window
- **>20% dead tuples**: Urgent VACUUM immediately
- **>10% dead tuples**: Run VACUUM soon to reclaim space
- **>5% dead tuples**: Schedule VACUUM during next maintenance window
- **>1000 modifications since analyze**: Run ANALYZE to update query planner statistics
- **Never vacuumed**: Consider running VACUUM
- **Never analyzed**: Consider running ANALYZE
- **>7 days since last vacuum with high write activity**: Consider running VACUUM
- **>7 days since last analyze with modifications**: Consider running ANALYZE

**Use Cases**:
- Identifying tables requiring maintenance (VACUUM/ANALYZE)
- Planning maintenance windows
- Monitoring table bloat over time
- Finding unused indexes that can be dropped
- Understanding write activity patterns
- Troubleshooting query performance issues related to bloat

**Important Notes**:
- Dead tuple percentage is calculated as: `dead_tuples / (live_tuples + dead_tuples) × 100`
- VACUUM reclaims space within the table file
- VACUUM FULL rewrites the entire table (requires exclusive lock and more time)
- ANALYZE updates query planner statistics for better query optimization
- Autovacuum typically handles maintenance automatically, but heavy write workloads may require manual intervention
- Regular monitoring helps prevent excessive bloat and maintain query performance

### read_resource

Reads MCP resources by their URI. Provides access to system information and statistics.

**Input Examples**:

List all available resources:
```json
{
  "list": true
}
```

Read a specific resource:
```json
{
  "uri": "pg://system_info"
}
```

**Available Resource URIs**:
- `pg://settings` - PostgreSQL configuration parameters
- `pg://system_info` - PostgreSQL version, OS, and build architecture
- `pg://stat/activity` - Current connections and queries
- `pg://stat/database` - Database-wide statistics
- `pg://stat/user_tables` - Per-table statistics
- `pg://stat/user_indexes` - Index usage statistics
- `pg://stat/replication` - Replication status
- `pg://stat/bgwriter` - Background writer statistics
- `pg://stat/wal` - WAL statistics (PostgreSQL 14+ only)

See [RESOURCES.md](RESOURCES.md) for detailed information about each resource.
