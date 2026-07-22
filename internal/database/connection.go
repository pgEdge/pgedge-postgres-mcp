/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
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
	"time"

	"pgedge-postgres-mcp/internal/config"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ConnectionInfo holds a connection pool and its metadata
type ConnectionInfo struct {
	ConnString       string
	Pool             *pgxpool.Pool
	Metadata         map[string]TableInfo
	MetadataLoaded   bool
	MetadataLoadedAt time.Time
}

// Client manages multiple PostgreSQL connections and metadata
type Client struct {
	connections    map[string]*ConnectionInfo  // keyed by connection string
	defaultConnStr string                      // current default connection string
	initialConnStr string                      // original connection string from env
	dbConfig       *config.NamedDatabaseConfig // database configuration for pool settings
	closed         bool                        // true after Close() has been called
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

	// Parse connection string into pgxpool.Config
	poolConfig, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return fmt.Errorf("unable to parse connection string: %w", err)
	}

	// Set application_name if not already present in the connection string
	setApplicationName(poolConfig, "pgEdge Natural Language Agent")

	// Log connection details if debug logging is enabled
	if GetLogLevel() >= LogLevelDebug {
		poolConfigMap := make(map[string]interface{})
		poolConfigMap["max_conns"] = poolConfig.MaxConns
		poolConfigMap["min_conns"] = poolConfig.MinConns
		poolConfigMap["max_conn_lifetime"] = poolConfig.MaxConnLifetime
		poolConfigMap["max_conn_idle_time"] = poolConfig.MaxConnIdleTime
		poolConfigMap["health_check_period"] = poolConfig.HealthCheckPeriod
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

		// Set health check period
		if c.dbConfig.PoolHealthCheckPeriod != "" {
			hcp, err := time.ParseDuration(c.dbConfig.PoolHealthCheckPeriod)
			if err != nil {
				return fmt.Errorf("invalid pool_health_check_period: %w", err)
			}
			poolConfig.HealthCheckPeriod = hcp
		}

		// Set max connection lifetime
		if c.dbConfig.PoolMaxConnLifetime != "" {
			mcl, err := time.ParseDuration(c.dbConfig.PoolMaxConnLifetime)
			if err != nil {
				return fmt.Errorf("invalid pool_max_conn_lifetime: %w", err)
			}
			poolConfig.MaxConnLifetime = mcl
		}
	}

	// For multi-host connections, enable health checking by default
	// so broken connections to failed hosts are cleaned up quickly.
	if c.dbConfig != nil && len(c.dbConfig.Hosts) > 0 {
		if c.dbConfig.PoolHealthCheckPeriod == "" {
			poolConfig.HealthCheckPeriod = 30 * time.Second
		}
		if c.dbConfig.PoolMaxConnLifetime == "" {
			poolConfig.MaxConnLifetime = 5 * time.Minute
		}
	}

	// Set read-only transaction mode at the session level unless writes are explicitly allowed.
	// This provides defense-in-depth: even if query_database logic fails, writes are blocked.
	// We use AfterConnect instead of RuntimeParams because RuntimeParams sets startup
	// parameters that connection poolers like PgBouncer and HAProxy do not support.
	if c.dbConfig == nil || !c.dbConfig.AllowWrites {
		poolConfig.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
			_, err := conn.Exec(ctx, "SET default_transaction_read_only = 'on'")
			return err
		}
	}

	// Determine connect timeout (default: 10s)
	connectTimeout := 10 * time.Second
	if c.dbConfig != nil && c.dbConfig.ConnectTimeout != "" {
		if d, err := time.ParseDuration(c.dbConfig.ConnectTimeout); err != nil {
			return fmt.Errorf("invalid connect_timeout: %w", err)
		} else {
			connectTimeout = d
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()

	// Create pool with configured settings
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		duration := time.Since(startTime)
		LogConnection(connStr, duration, err)
		return fmt.Errorf("unable to create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
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

// setApplicationName sets application_name on a pgxpool.Config if not
// already present. This avoids string manipulation on the DSN and works
// correctly with multi-host connection strings.
func setApplicationName(cfg *pgxpool.Config, appName string) {
	if cfg.ConnConfig.RuntimeParams == nil {
		cfg.ConnConfig.RuntimeParams = make(map[string]string)
	}
	if _, ok := cfg.ConnConfig.RuntimeParams["application_name"]; !ok {
		cfg.ConnConfig.RuntimeParams["application_name"] = appName
	}
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

// AllowWrites returns whether write operations are allowed on this database connection
// Returns false if no config is set or if writes are explicitly disabled
func (c *Client) AllowWrites() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.dbConfig == nil {
		return false
	}
	return c.dbConfig.AllowWrites
}

// DisplayName returns the operator-configured display name for this
// client's database (config.NamedDatabaseConfig.Name), for use in any
// user-facing string. It deliberately never returns the raw host/port: an
// internal-only address (a Kubernetes pod IP, a private VPC host) that the
// caller cannot reach is not useful information and only exposes
// infrastructure detail (issue #187).
//
// Falls back to a password-masked connection string when no name is
// configured, e.g. a bare client built without a NamedDatabaseConfig.
func (c *Client) DisplayName() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.dbConfig != nil && c.dbConfig.Name != "" {
		return c.dbConfig.Name
	}
	return SanitizeConnStr(c.defaultConnStr)
}

// ConfiguredHost returns the operator-configured host for this client's
// database (the first entry of Hosts when multi-host failover is
// configured, matching the same backward-compatibility rule used by
// GET /api/databases), defaulting to "localhost" when a single-host config
// omits it (matching NamedDatabaseConfig.Host's documented default), or
// "unix socket" if no config is set.
//
// This is deliberately the value the operator wrote in the config file,
// never a live-resolved server address such as inet_server_addr(): the
// latter can report an internal-only address (a container or pod IP) that
// differs from, and may be unreachable via, the host the operator actually
// configured (issue #187).
func (c *Client) ConfiguredHost() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.dbConfig == nil {
		return "unix socket"
	}
	if len(c.dbConfig.Hosts) > 0 {
		// Validate rejects empty entries in Hosts, so no default needed here.
		return c.dbConfig.Hosts[0].Host
	}
	if c.dbConfig.Host == "" {
		return "localhost"
	}
	return c.dbConfig.Host
}

// ConfiguredPort returns the operator-configured port for this client's
// database (the first entry of Hosts when multi-host failover is
// configured), defaulting to 5432 when unset to match
// NamedDatabaseConfig.BuildConnectionString's default. Returns 0 if no
// config is set.
func (c *Client) ConfiguredPort() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.dbConfig == nil {
		return 0
	}
	if len(c.dbConfig.Hosts) > 0 {
		if c.dbConfig.Hosts[0].Port == 0 {
			return 5432
		}
		return c.dbConfig.Hosts[0].Port
	}
	if c.dbConfig.Port == 0 {
		return 5432
	}
	return c.dbConfig.Port
}

// Close closes all database connections
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.closed = true

	for _, conn := range c.connections {
		if conn.Pool != nil {
			conn.Pool.Close()
		}
	}
	c.connections = make(map[string]*ConnectionInfo)
}

// IsClosed returns whether this client has been closed
func (c *Client) IsClosed() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.closed
}

// LoadMetadata loads table and column metadata for the default database
func (c *Client) LoadMetadata() error {
	c.mu.RLock()
	connStr := c.defaultConnStr
	c.mu.RUnlock()

	return c.LoadMetadataFor(connStr)
}

// LoadMetadataFor loads table and column metadata for a specific connection.
//
// The function orchestrates four steps:
//  1. Run loadMetadataSQL against the connection's pool.
//  2. Scan each row via scanMetadataRow and fold it straight into a
//     metadataAccumulator, so rows are never buffered in a slice.
//  3. Take the per-table metadata map the accumulator produced.
//  4. Atomically swap the result in under the client's lock and log.
func (c *Client) LoadMetadataFor(connStr string) error {
	startTime := time.Now()

	c.mu.RLock()
	conn, exists := c.connections[connStr]
	c.mu.RUnlock()

	if !exists {
		return fmt.Errorf("connection not found: %s", connStr)
	}

	// Use the configured connect timeout (default: 30s) so we don't
	// block indefinitely if the database is unreachable.
	metadataTimeout := 30 * time.Second
	if c.dbConfig != nil && c.dbConfig.ConnectTimeout != "" {
		if d, err := time.ParseDuration(c.dbConfig.ConnectTimeout); err == nil {
			metadataTimeout = d
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), metadataTimeout)
	defer cancel()

	rows, err := conn.Pool.Query(ctx, loadMetadataSQL)
	if err != nil {
		duration := time.Since(startTime)
		LogMetadataLoad(connStr, 0, duration, err)
		return fmt.Errorf("failed to query metadata: %w", err)
	}
	defer rows.Close()

	acc := newMetadataAccumulator()
	for rows.Next() {
		r, err := scanMetadataRow(rows)
		if err != nil {
			duration := time.Since(startTime)
			LogMetadataLoad(connStr, 0, duration, err)
			return fmt.Errorf("failed to scan row: %w", err)
		}
		acc.add(r)
	}
	if err := rows.Err(); err != nil {
		duration := time.Since(startTime)
		LogMetadataLoad(connStr, 0, duration, err)
		return err
	}

	newMetadata, schemaSet, columnCount := acc.metadata, acc.schemas, acc.columnCount

	c.mu.Lock()
	conn.Metadata = newMetadata
	conn.MetadataLoaded = true
	conn.MetadataLoadedAt = time.Now()
	c.mu.Unlock()

	duration := time.Since(startTime)
	LogMetadataLoad(connStr, len(newMetadata), duration, nil)

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

// IsMetadataLoadedFor returns whether valid (non-stale) metadata
// exists for a specific connection. Metadata is considered stale
// when it is older than the configured metadata_ttl (default: 5m).
// A TTL of 0 means metadata is always considered stale.
func (c *Client) IsMetadataLoadedFor(connStr string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	conn, exists := c.connections[connStr]
	if !exists {
		return false
	}
	if !conn.MetadataLoaded {
		return false
	}

	// Resolve TTL: default 5 minutes
	ttl := 5 * time.Minute
	if c.dbConfig != nil && c.dbConfig.MetadataTTL != "" {
		if parsed, err := time.ParseDuration(c.dbConfig.MetadataTTL); err == nil {
			ttl = parsed
		}
	}

	// TTL of 0 means always refresh
	if ttl == 0 {
		return false
	}

	return time.Since(conn.MetadataLoadedAt) <= ttl
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
