/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package database

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"sync"
	"time"

	"pgedge-postgres-mcp/internal/config"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ConnectionInfo holds a connection pool and its metadata
type ConnectionInfo struct {
	ConnString     string
	Pool           *pgxpool.Pool
	Metadata       map[string]TableInfo
	MetadataLoaded bool
}

// Client manages multiple PostgreSQL connections and metadata
type Client struct {
	connections    map[string]*ConnectionInfo  // keyed by connection string
	defaultConnStr string                      // current default connection string
	initialConnStr string                      // original connection string from env
	dbConfig       *config.NamedDatabaseConfig // database configuration for pool settings
	mu             sync.RWMutex
}

// NewClient creates a new database client with optional database configuration
func NewClient(dbConfig *config.NamedDatabaseConfig) *Client {
	return &Client{
		connections: make(map[string]*ConnectionInfo),
		dbConfig:    dbConfig,
	}
}

// NewClientWithConnectionString creates a new client with a specific connection string and database configuration
func NewClientWithConnectionString(connStr string, dbConfig *config.NamedDatabaseConfig) *Client {
	c := &Client{
		connections:    make(map[string]*ConnectionInfo),
		initialConnStr: connStr,
		defaultConnStr: connStr,
		dbConfig:       dbConfig,
	}
	return c
}

// Connect establishes a connection to the default PostgreSQL database
func (c *Client) Connect() error {
	// If a connection string was already set (e.g., via NewClientWithConnectionString),
	// use that instead of reading from the environment
	c.mu.RLock()
	existingConnStr := c.defaultConnStr
	dbConfig := c.dbConfig
	c.mu.RUnlock()

	connStr := existingConnStr
	if connStr == "" {
		// Priority order for connection string:
		// 1. DatabaseConfig (if provided)
		// 2. PGEDGE_POSTGRES_CONNECTION_STRING environment variable
		// 3. Default fallback
		if dbConfig != nil && dbConfig.User != "" {
			// Build connection string from DatabaseConfig
			connStr = dbConfig.BuildConnectionString()
		} else {
			// No connection string set yet, read from environment
			connStr = os.Getenv("PGEDGE_POSTGRES_CONNECTION_STRING")
			if connStr == "" {
				connStr = "postgres://localhost/postgres?sslmode=disable"
			}
		}

		c.mu.Lock()
		c.initialConnStr = connStr
		c.defaultConnStr = connStr
		c.mu.Unlock()
	}

	return c.ConnectTo(connStr)
}

// ConnectTo establishes a connection to a specific PostgreSQL database
func (c *Client) ConnectTo(connStr string) error {
	startTime := time.Now()

	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if connection already exists
	if _, exists := c.connections[connStr]; exists {
		return nil // Already connected
	}

	// Add application_name to connection string if not already present
	enhancedConnStr, err := addApplicationName(connStr, "pgEdge Natural Language Agent")
	if err != nil {
		return fmt.Errorf("unable to enhance connection string: %w", err)
	}

	// Parse connection string into pgxpool.Config
	poolConfig, err := pgxpool.ParseConfig(enhancedConnStr)
	if err != nil {
		return fmt.Errorf("unable to parse connection string: %w", err)
	}

	// Log connection details if debug logging is enabled
	if GetLogLevel() >= LogLevelDebug {
		poolConfigMap := make(map[string]interface{})
		poolConfigMap["max_conns"] = poolConfig.MaxConns
		poolConfigMap["min_conns"] = poolConfig.MinConns
		poolConfigMap["max_conn_lifetime"] = poolConfig.MaxConnLifetime
		poolConfigMap["max_conn_idle_time"] = poolConfig.MaxConnIdleTime
		LogConnectionDetails(connStr, poolConfigMap)
	}

	// Apply pool configuration if available
	if c.dbConfig != nil {
		// Set pool size limits
		if c.dbConfig.PoolMaxConns > 0 {
			poolConfig.MaxConns = int32(c.dbConfig.PoolMaxConns)
		}
		if c.dbConfig.PoolMinConns > 0 {
			poolConfig.MinConns = int32(c.dbConfig.PoolMinConns)
		}

		// Set idle timeout
		if c.dbConfig.PoolMaxConnIdleTime != "" {
			idleTime, err := time.ParseDuration(c.dbConfig.PoolMaxConnIdleTime)
			if err != nil {
				return fmt.Errorf("invalid pool_max_conn_idle_time: %w", err)
			}
			poolConfig.MaxConnIdleTime = idleTime
		}
	}

	// Set read-only transaction mode for all connections
	// This is enforced at the session level via default_transaction_read_only
	if poolConfig.ConnConfig.RuntimeParams == nil {
		poolConfig.ConnConfig.RuntimeParams = make(map[string]string)
	}
	poolConfig.ConnConfig.RuntimeParams["default_transaction_read_only"] = "on"

	// Create pool with configured settings
	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		duration := time.Since(startTime)
		LogConnection(connStr, duration, err)
		return fmt.Errorf("unable to create connection pool: %w", err)
	}

	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		duration := time.Since(startTime)
		LogConnection(connStr, duration, err)
		return fmt.Errorf("unable to ping database: %w", err)
	}

	c.connections[connStr] = &ConnectionInfo{
		ConnString:     connStr,
		Pool:           pool,
		Metadata:       make(map[string]TableInfo),
		MetadataLoaded: false,
	}

	duration := time.Since(startTime)
	LogConnection(connStr, duration, nil)

	return nil
}

// addApplicationName adds application_name parameter to a PostgreSQL connection string
func addApplicationName(connStr, appName string) (string, error) {
	// Parse the connection string
	u, err := url.Parse(connStr)
	if err != nil {
		return "", fmt.Errorf("invalid connection string: %w", err)
	}

	// Get existing query parameters
	query := u.Query()

	// Add application_name if not already present
	if !query.Has("application_name") {
		query.Set("application_name", appName)
		u.RawQuery = query.Encode()
	}

	return u.String(), nil
}

// SetDefaultConnection sets the default connection string to use for queries
func (c *Client) SetDefaultConnection(connStr string) error {
	// Ensure the connection exists
	if err := c.ConnectTo(connStr); err != nil {
		return err
	}

	c.mu.Lock()
	c.defaultConnStr = connStr
	c.mu.Unlock()

	// Load metadata if not already loaded
	if !c.IsMetadataLoadedFor(connStr) {
		return c.LoadMetadataFor(connStr)
	}

	return nil
}

// GetDefaultConnection returns the current default connection string
func (c *Client) GetDefaultConnection() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.defaultConnStr
}

// Close closes all database connections
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, conn := range c.connections {
		if conn.Pool != nil {
			conn.Pool.Close()
		}
	}
	c.connections = make(map[string]*ConnectionInfo)
}

// LoadMetadata loads table and column metadata for the default database
func (c *Client) LoadMetadata() error {
	c.mu.RLock()
	connStr := c.defaultConnStr
	c.mu.RUnlock()

	return c.LoadMetadataFor(connStr)
}

// LoadMetadataFor loads table and column metadata for a specific connection
func (c *Client) LoadMetadataFor(connStr string) error {
	startTime := time.Now()

	c.mu.RLock()
	conn, exists := c.connections[connStr]
	c.mu.RUnlock()

	if !exists {
		return fmt.Errorf("connection not found: %s", connStr)
	}

	ctx := context.Background()

	query := `
		WITH table_comments AS (
			SELECT
				n.nspname AS schema_name,
				c.relname AS table_name,
				CASE c.relkind
					WHEN 'r' THEN 'TABLE'
					WHEN 'v' THEN 'VIEW'
					WHEN 'm' THEN 'MATERIALIZED VIEW'
				END AS table_type,
				obj_description(c.oid) AS table_description
			FROM pg_class c
			JOIN pg_namespace n ON n.oid = c.relnamespace
			WHERE c.relkind IN ('r', 'v', 'm')
				AND n.nspname NOT IN ('pg_catalog', 'information_schema', 'pg_toast')
			ORDER BY n.nspname, c.relname
		),
		column_info AS (
			SELECT
				n.nspname AS schema_name,
				c.relname AS table_name,
				a.attname AS column_name,
				pg_catalog.format_type(a.atttypid, a.atttypmod) AS data_type,
				CASE WHEN a.attnotnull THEN 'NO' ELSE 'YES' END AS is_nullable,
				col_description(c.oid, a.attnum) AS column_description,
				t.typname AS type_name,
				a.atttypmod AS type_modifier,
				a.attnum AS column_num,
				c.oid AS table_oid,
				a.attidentity::text AS identity_type
			FROM pg_class c
			JOIN pg_namespace n ON n.oid = c.relnamespace
			JOIN pg_attribute a ON a.attrelid = c.oid
			JOIN pg_type t ON t.oid = a.atttypid
			WHERE c.relkind IN ('r', 'v', 'm')
				AND n.nspname NOT IN ('pg_catalog', 'information_schema', 'pg_toast')
				AND a.attnum > 0
				AND NOT a.attisdropped
			ORDER BY n.nspname, c.relname, a.attnum
		),
		pk_columns AS (
			SELECT
				n.nspname AS schema_name,
				c.relname AS table_name,
				a.attname AS column_name
			FROM pg_constraint con
			JOIN pg_class c ON c.oid = con.conrelid
			JOIN pg_namespace n ON n.oid = c.relnamespace
			JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum = ANY(con.conkey)
			WHERE con.contype = 'p'
		),
		unique_columns AS (
			SELECT DISTINCT
				n.nspname AS schema_name,
				c.relname AS table_name,
				a.attname AS column_name
			FROM pg_constraint con
			JOIN pg_class c ON c.oid = con.conrelid
			JOIN pg_namespace n ON n.oid = c.relnamespace
			JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum = ANY(con.conkey)
			WHERE con.contype = 'u'
		),
		fk_columns AS (
			SELECT
				n.nspname AS schema_name,
				c.relname AS table_name,
				a.attname AS column_name,
				fn.nspname || '.' || fc.relname || '.' || fa.attname AS fk_reference
			FROM pg_constraint con
			JOIN pg_class c ON c.oid = con.conrelid
			JOIN pg_namespace n ON n.oid = c.relnamespace
			JOIN pg_class fc ON fc.oid = con.confrelid
			JOIN pg_namespace fn ON fn.oid = fc.relnamespace
			JOIN LATERAL unnest(con.conkey, con.confkey) WITH ORDINALITY AS cols(col_num, ref_num, ord) ON true
			JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum = cols.col_num
			JOIN pg_attribute fa ON fa.attrelid = fc.oid AND fa.attnum = cols.ref_num
			WHERE con.contype = 'f'
		),
		indexed_columns AS (
			SELECT DISTINCT
				n.nspname AS schema_name,
				c.relname AS table_name,
				a.attname AS column_name
			FROM pg_index i
			JOIN pg_class c ON c.oid = i.indrelid
			JOIN pg_namespace n ON n.oid = c.relnamespace
			JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum = ANY(i.indkey)
			WHERE n.nspname NOT IN ('pg_catalog', 'information_schema', 'pg_toast')
		),
		column_defaults AS (
			SELECT
				n.nspname AS schema_name,
				c.relname AS table_name,
				a.attname AS column_name,
				pg_get_expr(d.adbin, d.adrelid) AS default_value
			FROM pg_attrdef d
			JOIN pg_class c ON c.oid = d.adrelid
			JOIN pg_namespace n ON n.oid = c.relnamespace
			JOIN pg_attribute a ON a.attrelid = d.adrelid AND a.attnum = d.adnum
			WHERE n.nspname NOT IN ('pg_catalog', 'information_schema', 'pg_toast')
				AND NOT a.attisdropped
		)
		SELECT
			tc.schema_name,
			tc.table_name,
			tc.table_type,
			COALESCE(tc.table_description, '') AS table_description,
			ci.column_name,
			ci.data_type,
			ci.is_nullable,
			COALESCE(ci.column_description, '') AS column_description,
			ci.type_name,
			ci.type_modifier,
			CASE WHEN pk.column_name IS NOT NULL THEN true ELSE false END AS is_primary_key,
			CASE WHEN uq.column_name IS NOT NULL THEN true ELSE false END AS is_unique,
			COALESCE(fk.fk_reference, '') AS fk_reference,
			CASE WHEN ix.column_name IS NOT NULL THEN true ELSE false END AS is_indexed,
			COALESCE(ci.identity_type, '') AS identity_type,
			COALESCE(cd.default_value, '') AS default_value
		FROM table_comments tc
		LEFT JOIN column_info ci ON tc.schema_name = ci.schema_name AND tc.table_name = ci.table_name
		LEFT JOIN pk_columns pk ON ci.schema_name = pk.schema_name AND ci.table_name = pk.table_name AND ci.column_name = pk.column_name
		LEFT JOIN unique_columns uq ON ci.schema_name = uq.schema_name AND ci.table_name = uq.table_name AND ci.column_name = uq.column_name
		LEFT JOIN fk_columns fk ON ci.schema_name = fk.schema_name AND ci.table_name = fk.table_name AND ci.column_name = fk.column_name
		LEFT JOIN indexed_columns ix ON ci.schema_name = ix.schema_name AND ci.table_name = ix.table_name AND ci.column_name = ix.column_name
		LEFT JOIN column_defaults cd ON ci.schema_name = cd.schema_name AND ci.table_name = cd.table_name AND ci.column_name = cd.column_name
		ORDER BY tc.schema_name, tc.table_name, ci.column_name
	`

	rows, err := conn.Pool.Query(ctx, query)
	if err != nil {
		duration := time.Since(startTime)
		LogMetadataLoad(connStr, 0, duration, err)
		return fmt.Errorf("failed to query metadata: %w", err)
	}
	defer rows.Close()

	newMetadata := make(map[string]TableInfo)
	schemaSet := make(map[string]bool)
	columnCount := 0

	for rows.Next() {
		var schemaName, tableName, tableType, tableDesc, columnName, dataType, isNullable, columnDesc string
		var typeName sql.NullString
		var typeModifier sql.NullInt32
		var isPrimaryKey, isUnique, isIndexed bool
		var fkReference, identityType, defaultValue string

		err := rows.Scan(&schemaName, &tableName, &tableType, &tableDesc, &columnName, &dataType, &isNullable, &columnDesc, &typeName, &typeModifier, &isPrimaryKey, &isUnique, &fkReference, &isIndexed, &identityType, &defaultValue)
		if err != nil {
			duration := time.Since(startTime)
			LogMetadataLoad(connStr, 0, duration, err)
			return fmt.Errorf("failed to scan row: %w", err)
		}

		key := schemaName + "." + tableName
		schemaSet[schemaName] = true

		table, exists := newMetadata[key]
		if !exists {
			table = TableInfo{
				SchemaName:  schemaName,
				TableName:   tableName,
				TableType:   tableType,
				Description: tableDesc,
				Columns:     []ColumnInfo{},
			}
		}

		if columnName != "" {
			// Detect vector columns and extract dimensions
			isVector := false
			dimensions := 0
			if typeName.Valid && typeName.String == "vector" {
				isVector = true
				// Parse dimensions from data_type (e.g., "vector(1536)")
				re := regexp.MustCompile(`vector\((\d+)\)`)
				if matches := re.FindStringSubmatch(dataType); len(matches) > 1 {
					if dim, err := strconv.Atoi(matches[1]); err == nil {
						dimensions = dim
					}
				}
			}

			table.Columns = append(table.Columns, ColumnInfo{
				ColumnName:       columnName,
				DataType:         dataType,
				IsNullable:       isNullable,
				Description:      columnDesc,
				IsPrimaryKey:     isPrimaryKey,
				IsUnique:         isUnique,
				ForeignKeyRef:    fkReference,
				IsIndexed:        isIndexed,
				IsIdentity:       identityType,
				DefaultValue:     defaultValue,
				IsVectorColumn:   isVector,
				VectorDimensions: dimensions,
			})
			columnCount++
		}

		newMetadata[key] = table
	}

	if err := rows.Err(); err != nil {
		duration := time.Since(startTime)
		LogMetadataLoad(connStr, 0, duration, err)
		return err
	}

	// Update metadata atomically
	c.mu.Lock()
	conn.Metadata = newMetadata
	conn.MetadataLoaded = true
	c.mu.Unlock()

	duration := time.Since(startTime)
	LogMetadataLoad(connStr, len(newMetadata), duration, nil)

	// Log detailed metadata info if debug logging is enabled
	if GetLogLevel() >= LogLevelDebug {
		LogMetadataDetails(connStr, len(schemaSet), len(newMetadata), columnCount)
	}

	return nil
}

// GetMetadata returns a copy of the metadata map for the default connection
func (c *Client) GetMetadata() map[string]TableInfo {
	c.mu.RLock()
	connStr := c.defaultConnStr
	c.mu.RUnlock()

	return c.GetMetadataFor(connStr)
}

// GetMetadataFor returns a copy of the metadata map for a specific connection
func (c *Client) GetMetadataFor(connStr string) map[string]TableInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	conn, exists := c.connections[connStr]
	if !exists {
		return make(map[string]TableInfo)
	}

	result := make(map[string]TableInfo, len(conn.Metadata))
	for k, v := range conn.Metadata {
		result[k] = v
	}
	return result
}

// IsMetadataLoaded returns whether metadata has been loaded for the default connection
func (c *Client) IsMetadataLoaded() bool {
	c.mu.RLock()
	connStr := c.defaultConnStr
	c.mu.RUnlock()

	return c.IsMetadataLoadedFor(connStr)
}

// IsMetadataLoadedFor returns whether metadata has been loaded for a specific connection
func (c *Client) IsMetadataLoadedFor(connStr string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	conn, exists := c.connections[connStr]
	if !exists {
		return false
	}
	return conn.MetadataLoaded
}

// GetPool returns the connection pool for the default connection
func (c *Client) GetPool() *pgxpool.Pool {
	c.mu.RLock()
	connStr := c.defaultConnStr
	c.mu.RUnlock()

	return c.GetPoolFor(connStr)
}

// GetPoolFor returns the connection pool for a specific connection
func (c *Client) GetPoolFor(connStr string) *pgxpool.Pool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	conn, exists := c.connections[connStr]
	if !exists {
		return nil
	}
	return conn.Pool
}

// GetConnectionInfo returns information about a specific connection
func (c *Client) GetConnectionInfo(connStr string) (*ConnectionInfo, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	conn, exists := c.connections[connStr]
	return conn, exists
}

// ListConnections returns a list of all connection strings
func (c *Client) ListConnections() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []string
	for connStr := range c.connections {
		result = append(result, connStr)
	}
	return result
}
