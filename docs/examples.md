# Query Examples

This document provides example natural language queries you can use with the pgEdge MCP server. All examples assume you're using Claude Desktop or another MCP client with the server configured.

## Table of Contents

- [Basic Data Queries](#basic-data-queries)
- [Schema Discovery](#schema-discovery)
- [Configuration Management](#configuration-management)
    - [Viewing Configuration](#viewing-configuration)
    - [Modifying Configuration](#modifying-configuration)

- [Multi-Database Queries](#multi-database-queries)
    - [Temporary Connection (Single Query)](#temporary-connection-single-query)
    - [Setting Default Database](#setting-default-database)

- [Advanced Queries](#advanced-queries)
- [Connection String Format](#connection-string-format)

## Basic Data Queries

These queries work against your default database connection:

### Customer/User Queries
- "Show me all customers who made purchases in the last month"
- "List all users who haven't logged in for more than 30 days"
- "Find users who registered this week"
- "Show me the most active users in the last quarter"

### Product/Inventory Queries
- "What are the top 10 products by revenue?"
- "Find all orders with items that are out of stock"
- "Show me products with low inventory levels"
- "List products that haven't sold in the last 60 days"

### Analytics Queries
- "Show me the average order value by customer segment"
- "What's the total revenue for this month?"
- "Calculate the conversion rate by marketing channel"
- "Show daily active users for the past 7 days"

### Time-Based Queries
- "Show me all orders placed today"
- "Find records created in the last hour"
- "List events from the past week grouped by day"
- "Show monthly sales trends for the last year"

## Schema Discovery

Use these queries to understand your database structure:

### General Schema Information
- "Show me the database schema"
- "What tables are available?"
- "List all views in the database"
- "Show me all tables in the public schema"

### Table Details
- "Describe the customers table"
- "What columns are in the orders table?"
- "Show me the structure of the users table"
- "What data types are in the products table?"

### Relationship Queries
- "What tables reference the users table?"
- "Show me foreign key relationships"
- "Which tables are related to orders?"

## Configuration Management

The pgEdge MCP server provides access to PostgreSQL configuration parameters through the `pg://settings` resource and the `set_pg_configuration` tool.

### Viewing Configuration

Access the `pg://settings` resource to view all PostgreSQL configuration parameters:

**Resource Access:**

- "Show me the pg://settings resource"
- "Read the PostgreSQL settings resource"
- "Display server configuration from pg://settings"

**Example Questions About Configuration:**

- "What is the current value of max_connections?"
- "Show me all memory-related configuration parameters"
- "Which settings require a restart to take effect?"
- "What are the default values for connection settings?"
- "Show me all configuration parameters that have been changed from defaults"

**The resource returns:**

- Current value
- Default value
- Reset value (value after next reload)
- Whether a restart is pending
- Parameter description
- Valid range (for numeric parameters)
- Valid options (for enum parameters)
- Configuration context (when it can be changed)

### Modifying Configuration

Use the `set_pg_configuration` tool to modify PostgreSQL server configuration:

**Setting Values:**
```
"Set max_connections to 200"
"Change work_mem to 16MB"
"Set shared_buffers to 2GB"
"Modify maintenance_work_mem to 512MB"
```

**Resetting to Defaults:**
```
"Reset max_connections to default"
"Set work_mem back to default value"
"Restore default value for shared_buffers"
```

**Configuration Examples by Category:**

#### Connection Settings
```
"Set max_connections to 300"
"Change superuser_reserved_connections to 5"
"Set tcp_keepalives_idle to 600"
```

#### Memory Settings
```
"Set shared_buffers to 4GB"
"Change work_mem to 32MB"
"Set maintenance_work_mem to 1GB"
"Modify effective_cache_size to 16GB"
```

#### Write-Ahead Log
```
"Set wal_level to replica"
"Change max_wal_size to 2GB"
"Set checkpoint_timeout to 15min"
```

#### Query Planning
```
"Set random_page_cost to 1.1"
"Change effective_io_concurrency to 200"
"Set default_statistics_target to 200"
```

#### Logging
```
"Set log_min_duration_statement to 1000"  (log queries > 1 second)
"Change log_statement to 'all'"
"Set log_line_prefix to '%t [%p]: '"
```

#### Autovacuum
```
"Set autovacuum_naptime to 30s"
"Change autovacuum_vacuum_scale_factor to 0.1"
"Set autovacuum_max_workers to 4"
```

**Important Notes:**

1. **Restart Requirements**: Some parameters require a PostgreSQL restart to take effect:
   - Connection settings (max_connections, shared_buffers)
   - Most memory settings
   - WAL-related settings
   - The tool will warn you when a restart is required

2. **Permissions**: You need superuser privileges to use ALTER SYSTEM SET

3. **Persistence**: Changes are written to `postgresql.auto.conf` and persist across restarts

4. **Reload**: The tool automatically calls `pg_reload_conf()` for parameters that don't require restart

**Verification:**
After changing a setting, you can verify it:
```
"Show me the current value of max_connections"
"Check if max_connections has a pending restart"
```

## Multi-Database Queries

The pgEdge MCP server supports querying multiple PostgreSQL databases without changing configuration files.

### Temporary Connection (Single Query)

Query a different database for a single query while keeping your default connection unchanged. Include the connection string in your natural language query using one of these patterns:

#### Using "at" Pattern
```
"Show me the table list at postgres://user:pass@localhost:5432/other_db"
"Count users at postgres://analytics-db:5432/analytics"
"What is the PostgreSQL version at postgres://localhost/test_db"
```

#### Using "from" Pattern
```
"Show me all tables from postgres://prod-server/production_db"
"List database size from postgres://localhost:5433/warehouse"
"Get active connections from postgres://monitoring-db/metrics"
```

#### Using "on" Pattern
```
"List all users on postgres://dev-server:5433/dev_db?sslmode=require"
"Show table count on postgres://staging-db/staging"
"Query user activity on postgres://logs-db:5432/application_logs"
```

#### Real-World Examples
```
"What's the total order count at postgres://replica:5432/production_readonly"
"Show me table sizes from postgres://dba@warehouse-01/analytics?sslmode=require"
"List all schemas on postgres://reporting-server:5433/reports"
```

**How it works:**

1. Server connects to the specified database
2. Loads metadata (if not already cached)
3. Executes your query against that database
4. Returns results
5. Keeps your original default connection unchanged

### Setting Default Database

Permanently switch to a different database for all subsequent queries:

#### Using "Set Default" Pattern
```
"Set default database to postgres://user:pass@localhost:5432/analytics_db"
"Set default database to postgres://prod-server:5432/production"
```

#### Using "Use Database" Pattern
```
"Use database postgres://data-warehouse/metrics"
"Use database postgres://localhost:5433/reporting"
```

#### Using "Switch To" Pattern
```
"Switch to postgres://reporting-server/reports"
"Switch to database postgres://analytics-cluster/analytics"
```

#### Real-World Examples
```
"Set default database to postgres://analytics:5432/user_analytics?sslmode=require"
"Use database postgres://warehouse-db:5433/data_warehouse"
"Switch to postgres://backup-server/production_backup"
```

**How it works:**

1. Server connects to the new database
2. Loads metadata
3. Sets it as the default for all future queries
4. Confirms the switch with a metadata summary
5. All subsequent queries will use this connection

**To revert to original:**
```
"Set default database to postgres://localhost/postgres"
```

## Advanced Queries

### Combining Multi-Database with Complex Queries

Query different databases with sophisticated data requests:

```
"Show me top 10 customers by revenue from postgres://analytics-db/sales"
"Calculate average response time for last 24 hours at postgres://metrics-db/performance"
"Find tables larger than 1GB on postgres://dba-server/production"
"Show database connections grouped by state from postgres://monitoring/postgres_stats"
```

### Cross-Database Comparisons

While you can't join across databases in a single query, you can run separate queries:

```
1. "Show user count from postgres://prod-db/production"
2. "Show user count from postgres://dev-db/development"
3. "Show user count from postgres://staging-db/staging"
```

### Schema Exploration Across Databases

```
"List all tables at postgres://legacy-db/old_system"
"Show table sizes from postgres://new-db/current_system"
"Compare schema of users table at postgres://db1/app vs postgres://db2/app"
```

### Working with Replicas

Query read replicas for reporting without impacting primary database:

```
"Generate monthly sales report from postgres://replica-01:5432/production_readonly"
"Show customer analytics at postgres://reporting-replica/analytics?sslmode=require"
"Calculate aggregate statistics from postgres://readonly-replica:5433/warehouse"
```

### Connection with SSL/TLS

For secure connections, include SSL parameters:

```
"Show tables at postgres://prod-db:5432/production?sslmode=require"
"Query users from postgres://secure-db/data?sslmode=verify-full&sslrootcert=/path/to/ca.crt"
```

## Connection String Format

PostgreSQL connection strings follow this format:

```
postgres://[user[:password]@][host][:port][/dbname][?param=value]
```

### Components

- **user**: PostgreSQL username
- **password**: User password (optional, can use other auth methods)
- **host**: Server hostname or IP address
- **port**: Port number (default: 5432)
- **dbname**: Database name
- **param=value**: Query parameters (e.g., sslmode, connect_timeout)

### Example Connection Strings

**Local Development:**
```
postgres://localhost/mydb
postgres://localhost:5432/development
postgres://postgres@localhost/test_db
```

**With Authentication:**
```
postgres://username:password@localhost:5432/production
postgres://dbuser:secretpass@db-server/analytics
```

**With SSL:**
```
postgres://user@host:5432/db?sslmode=require
postgres://user@host/db?sslmode=verify-full
postgres://user@host/db?sslmode=disable
```

**Remote Servers:**
```
postgres://analytics-user@analytics-01.example.com:5432/warehouse
postgres://readonly@replica.example.com:5433/production_readonly
```

**Complete Example:**
```
postgres://analytics_user:secret123@warehouse-01.company.com:5432/analytics_db?sslmode=require&connect_timeout=10
```

### Supported Parameters

Common connection parameters:

- `sslmode`: SSL connection mode (disable, allow, prefer, require, verify-ca, verify-full)
- `connect_timeout`: Connection timeout in seconds
- `application_name`: Application name for logging
- `options`: PostgreSQL runtime parameters

## Tips and Best Practices

### 1. Start with Schema Discovery

Before querying data, understand your database structure:
```
"Show me the database schema"
"What tables are available?"
```

### 2. Use Specific Table Names

More specific queries generate better SQL:
```
Good: "Show me orders from the last week"
Better: "Show me all orders from the orders table created in the last 7 days"
```

### 3. Reference Column Descriptions

If your database has column comments (see main README for adding comments), the system will use them to generate more accurate queries.

### 4. Test Queries on Development First

When working with multiple databases, test queries on development environments:
```
"Show users at postgres://localhost/dev_db"
```

Then apply to production:
```
"Show users at postgres://prod-server/production_db"
```

### 5. Use Read Replicas for Heavy Queries

For expensive analytical queries, use read replicas:
```
"Generate sales report from postgres://replica-01/production_readonly"
```

### 6. Keep Connection Strings Secure

Never commit connection strings with passwords to version control. Use environment variables for the default connection, and be cautious when switching databases with embedded credentials.

## Troubleshooting

### Query Returns Unexpected Results

Try asking Claude to show the generated SQL:
```
"Show me users created today and display the SQL query"
```

### Connection Errors

If a connection fails, verify:

1. Database is accessible from your machine
2. Credentials are correct
3. Firewall rules allow connections
4. SSL settings match server requirements

### Slow Queries

For queries taking too long:

1. Check database indexes
2. Use read replicas for analytics
3. Limit result sets: "Show me top 100 users"

## More Examples

For more complex scenarios, see:

- [Troubleshooting Guide](troubleshooting.md) - Debugging and common issues
- [Architecture Guide](architecture.md) - Understanding how the server works
