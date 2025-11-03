/*-------------------------------------------------------------------------
 *
 * pgEdge Postgres MCP Server
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package database

import (
	"context"
	"fmt"
	"os"
	"sync"

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
	connections    map[string]*ConnectionInfo // keyed by connection string
	defaultConnStr string                     // current default connection string
	initialConnStr string                     // original connection string from env
	mu             sync.RWMutex
}

// NewClient creates a new database client
func NewClient() *Client {
	return &Client{
		connections: make(map[string]*ConnectionInfo),
	}
}

// NewClientWithConnectionString creates a new client with a specific connection string
func NewClientWithConnectionString(connStr string) *Client {
	c := &Client{
		connections:    make(map[string]*ConnectionInfo),
		initialConnStr: connStr,
		defaultConnStr: connStr,
	}
	return c
}

// Connect establishes a connection to the default PostgreSQL database
func (c *Client) Connect() error {
	// If a connection string was already set (e.g., via NewClientWithConnectionString),
	// use that instead of reading from the environment
	c.mu.RLock()
	existingConnStr := c.defaultConnStr
	c.mu.RUnlock()

	connStr := existingConnStr
	if connStr == "" {
		// No connection string set yet, read from environment
		connStr = os.Getenv("POSTGRES_CONNECTION_STRING")
		if connStr == "" {
			connStr = "postgres://localhost/postgres?sslmode=disable"
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
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if connection already exists
	if _, exists := c.connections[connStr]; exists {
		return nil // Already connected
	}

	pool, err := pgxpool.New(context.Background(), connStr)
	if err != nil {
		return fmt.Errorf("unable to create connection pool: %w", err)
	}

	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		return fmt.Errorf("unable to ping database: %w", err)
	}

	c.connections[connStr] = &ConnectionInfo{
		ConnString:     connStr,
		Pool:           pool,
		Metadata:       make(map[string]TableInfo),
		MetadataLoaded: false,
	}

	return nil
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
				col_description(c.oid, a.attnum) AS column_description
			FROM pg_class c
			JOIN pg_namespace n ON n.oid = c.relnamespace
			JOIN pg_attribute a ON a.attrelid = c.oid
			WHERE c.relkind IN ('r', 'v', 'm')
				AND n.nspname NOT IN ('pg_catalog', 'information_schema', 'pg_toast')
				AND a.attnum > 0
				AND NOT a.attisdropped
			ORDER BY n.nspname, c.relname, a.attnum
		)
		SELECT
			tc.schema_name,
			tc.table_name,
			tc.table_type,
			COALESCE(tc.table_description, '') AS table_description,
			ci.column_name,
			ci.data_type,
			ci.is_nullable,
			COALESCE(ci.column_description, '') AS column_description
		FROM table_comments tc
		LEFT JOIN column_info ci ON tc.schema_name = ci.schema_name AND tc.table_name = ci.table_name
		ORDER BY tc.schema_name, tc.table_name, ci.column_name
	`

	rows, err := conn.Pool.Query(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to query metadata: %w", err)
	}
	defer rows.Close()

	newMetadata := make(map[string]TableInfo)
	for rows.Next() {
		var schemaName, tableName, tableType, tableDesc, columnName, dataType, isNullable, columnDesc string

		err := rows.Scan(&schemaName, &tableName, &tableType, &tableDesc, &columnName, &dataType, &isNullable, &columnDesc)
		if err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}

		key := schemaName + "." + tableName

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
			table.Columns = append(table.Columns, ColumnInfo{
				ColumnName:  columnName,
				DataType:    dataType,
				IsNullable:  isNullable,
				Description: columnDesc,
			})
		}

		newMetadata[key] = table
	}

	if err := rows.Err(); err != nil {
		return err
	}

	// Update metadata atomically
	c.mu.Lock()
	conn.Metadata = newMetadata
	conn.MetadataLoaded = true
	c.mu.Unlock()

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
