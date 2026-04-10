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
	"fmt"
	"os"
	"reflect"
	"sync"

	"pgedge-postgres-mcp/internal/config"
)

// ClientManager manages per-token, per-database clients for connection isolation
// Each authenticated token can have connections to multiple databases
type ClientManager struct {
	mu            sync.RWMutex
	clients       map[string]map[string]*Client          // tokenHash -> dbName -> client
	dbConfigs     map[string]*config.NamedDatabaseConfig // dbName -> config
	currentDB     map[string]string                      // tokenHash -> current dbName
	defaultDBName string                                 // name of default database (first configured)
}

// NewClientManager creates a new client manager with database configurations
func NewClientManager(databases []config.NamedDatabaseConfig) *ClientManager {
	cm := &ClientManager{
		clients:   make(map[string]map[string]*Client),
		dbConfigs: make(map[string]*config.NamedDatabaseConfig),
		currentDB: make(map[string]string),
	}

	// Store database configs
	for i := range databases {
		db := &databases[i]
		cm.dbConfigs[db.Name] = db
		if cm.defaultDBName == "" {
			cm.defaultDBName = db.Name
		}
	}

	return cm
}

// NewClientManagerWithConfig creates a client manager with a single database config
// This provides backward compatibility with code expecting single database setup
func NewClientManagerWithConfig(dbConfig *config.NamedDatabaseConfig) *ClientManager {
	if dbConfig == nil {
		return &ClientManager{
			clients:   make(map[string]map[string]*Client),
			dbConfigs: make(map[string]*config.NamedDatabaseConfig),
			currentDB: make(map[string]string),
		}
	}

	name := dbConfig.Name
	if name == "" {
		name = "default"
	}

	return &ClientManager{
		clients:       make(map[string]map[string]*Client),
		dbConfigs:     map[string]*config.NamedDatabaseConfig{name: dbConfig},
		currentDB:     make(map[string]string),
		defaultDBName: name,
	}
}

// GetClient returns a database client for the given token hash using the current database
// Creates a new client if one doesn't exist for this token/database combination
func (cm *ClientManager) GetClient(tokenHash string) (*Client, error) {
	if tokenHash == "" {
		return nil, fmt.Errorf("token hash is required for authenticated requests")
	}

	// Get current database for this token (or default)
	dbName := cm.GetCurrentDatabase(tokenHash)
	return cm.GetClientForDatabase(tokenHash, dbName)
}

// GetClientForDatabase returns a database client for a specific database
// Creates a new client if one doesn't exist for this token/database combination
func (cm *ClientManager) GetClientForDatabase(tokenHash, dbName string) (*Client, error) {
	if tokenHash == "" {
		return nil, fmt.Errorf("token hash is required for authenticated requests")
	}
	if dbName == "" {
		dbName = cm.defaultDBName
	}

	// Try to get existing client (read lock)
	cm.mu.RLock()
	if tokenClients, exists := cm.clients[tokenHash]; exists {
		if client, exists := tokenClients[dbName]; exists && !client.IsClosed() {
			cm.mu.RUnlock()
			return client, nil
		}
	}
	dbConfig := cm.dbConfigs[dbName]
	cm.mu.RUnlock()

	if dbConfig == nil {
		return nil, fmt.Errorf("database '%s' not configured", dbName)
	}

	// Create new client (write lock)
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Double-check after acquiring write lock
	if tokenClients, exists := cm.clients[tokenHash]; exists {
		if client, exists := tokenClients[dbName]; exists && !client.IsClosed() {
			return client, nil
		}
	}

	// Create and initialize new client with database configuration
	client := NewClient(dbConfig)
	if err := client.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to database '%s': %w", dbName, err)
	}

	if err := client.LoadMetadata(); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to load metadata for database '%s': %w", dbName, err)
	}

	// Ensure token's client map exists
	if cm.clients[tokenHash] == nil {
		cm.clients[tokenHash] = make(map[string]*Client)
	}
	cm.clients[tokenHash][dbName] = client

	return client, nil
}

// countClients returns total number of client connections (internal use)
func (cm *ClientManager) countClients() int {
	count := 0
	for _, tokenClients := range cm.clients {
		count += len(tokenClients)
	}
	return count
}

// SetCurrentDatabase sets the current database for a token
func (cm *ClientManager) SetCurrentDatabase(tokenHash, dbName string) error {
	if tokenHash == "" {
		return fmt.Errorf("token hash is required")
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Verify database exists
	if _, exists := cm.dbConfigs[dbName]; !exists {
		return fmt.Errorf("database '%s' not configured", dbName)
	}

	cm.currentDB[tokenHash] = dbName
	return nil
}

// SetCurrentDatabaseAndCloseOthers sets the current database and closes connections
// to other databases for this session. This is useful in STDIO mode where only
// one database connection is typically needed at a time.
func (cm *ClientManager) SetCurrentDatabaseAndCloseOthers(tokenHash, dbName string) error {
	if tokenHash == "" {
		return fmt.Errorf("token hash is required")
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Verify database exists
	if _, exists := cm.dbConfigs[dbName]; !exists {
		return fmt.Errorf("database '%s' not configured", dbName)
	}

	// Close connections to other databases for this session
	if tokenClients, exists := cm.clients[tokenHash]; exists {
		for otherDB, client := range tokenClients {
			if otherDB != dbName {
				client.Close()
				delete(tokenClients, otherDB)
			}
		}
	}

	cm.currentDB[tokenHash] = dbName
	return nil
}

// GetCurrentDatabase returns the current database name for a token
// Returns the default database if no specific database is set
func (cm *ClientManager) GetCurrentDatabase(tokenHash string) string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if dbName, exists := cm.currentDB[tokenHash]; exists {
		return dbName
	}
	return cm.defaultDBName
}

// GetDefaultDatabaseName returns the name of the default database
func (cm *ClientManager) GetDefaultDatabaseName() string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.defaultDBName
}

// GetDatabaseConfig returns the configuration for a specific database
func (cm *ClientManager) GetDatabaseConfig(name string) *config.NamedDatabaseConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.dbConfigs[name]
}

// ListDatabaseNames returns the names of all configured databases
func (cm *ClientManager) ListDatabaseNames() []string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	names := make([]string, 0, len(cm.dbConfigs))
	for name := range cm.dbConfigs {
		names = append(names, name)
	}
	return names
}

// GetDatabaseConfigs returns all database configurations
func (cm *ClientManager) GetDatabaseConfigs() []config.NamedDatabaseConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	configs := make([]config.NamedDatabaseConfig, 0, len(cm.dbConfigs))
	for _, cfg := range cm.dbConfigs {
		configs = append(configs, *cfg)
	}
	return configs
}

// UpdateDatabaseConfigs updates the database configurations.
// Used for SIGHUP config reload. Existing connections are closed when the
// database is removed or when connection-relevant settings have changed;
// they will be lazily recreated with the new config on the next request.
func (cm *ClientManager) UpdateDatabaseConfigs(databases []config.NamedDatabaseConfig) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Build new config map
	newConfigs := make(map[string]*config.NamedDatabaseConfig)
	newDefaultName := ""
	for i := range databases {
		db := &databases[i]
		newConfigs[db.Name] = db
		if newDefaultName == "" {
			newDefaultName = db.Name
		}
	}

	// Find databases that were removed
	for name := range cm.dbConfigs {
		if _, exists := newConfigs[name]; !exists {
			// Database removed - close all connections to it
			for tokenHash, tokenClients := range cm.clients {
				if client, exists := tokenClients[name]; exists {
					client.Close()
					delete(tokenClients, name)
					fmt.Fprintf(os.Stderr, "Closed connection to removed database '%s' for token\n", name)
				}
				// Update currentDB if it was pointing to removed database
				if cm.currentDB[tokenHash] == name {
					cm.currentDB[tokenHash] = newDefaultName
				}
			}
		}
	}

	// Find databases that exist in both old and new configs but have
	// connection-relevant settings that changed. Close all pooled
	// connections so they get recreated with the new configuration.
	for name, oldCfg := range cm.dbConfigs {
		newCfg, exists := newConfigs[name]
		if !exists {
			continue // already handled above as a removal
		}
		if databaseConfigChanged(oldCfg, newCfg) {
			for _, tokenClients := range cm.clients {
				if client, exists := tokenClients[name]; exists {
					client.Close()
					delete(tokenClients, name)
				}
			}
			fmt.Fprintf(os.Stderr, "Closed connections to database '%s' (configuration changed)\n", name)
		}
	}

	cm.dbConfigs = newConfigs
	cm.defaultDBName = newDefaultName

	fmt.Fprintf(os.Stderr, "Updated database configurations: %d database(s)\n", len(databases))
}

// databaseConfigChanged returns true when connection-relevant fields differ
// between two NamedDatabaseConfig values. These are the fields that affect
// the DSN, pool behavior, or transaction mode.
func databaseConfigChanged(oldCfg, newCfg *config.NamedDatabaseConfig) bool {
	if oldCfg.Host != newCfg.Host ||
		oldCfg.Port != newCfg.Port ||
		oldCfg.Database != newCfg.Database ||
		oldCfg.User != newCfg.User ||
		oldCfg.Password != newCfg.Password ||
		oldCfg.SSLMode != newCfg.SSLMode ||
		oldCfg.TargetSessionAttrs != newCfg.TargetSessionAttrs ||
		oldCfg.AllowWrites != newCfg.AllowWrites ||
		oldCfg.PoolMaxConns != newCfg.PoolMaxConns ||
		oldCfg.PoolMinConns != newCfg.PoolMinConns ||
		oldCfg.PoolMaxConnIdleTime != newCfg.PoolMaxConnIdleTime ||
		oldCfg.PoolHealthCheckPeriod != newCfg.PoolHealthCheckPeriod ||
		oldCfg.PoolMaxConnLifetime != newCfg.PoolMaxConnLifetime ||
		oldCfg.ConnectTimeout != newCfg.ConnectTimeout ||
		oldCfg.MetadataTTL != newCfg.MetadataTTL {
		return true
	}
	if !reflect.DeepEqual(oldCfg.Hosts, newCfg.Hosts) {
		return true
	}
	return false
}

// RemoveClient removes and closes all database clients for a given token hash
// This should be called when a token is removed or expires
func (cm *ClientManager) RemoveClient(tokenHash string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	tokenClients, exists := cm.clients[tokenHash]
	if !exists {
		return nil // Already removed
	}

	// Close all connections for this token
	for dbName, client := range tokenClients {
		client.Close()
		fmt.Fprintf(os.Stderr, "Closed connection to '%s' for removed token\n", dbName)
	}

	// Remove from maps
	delete(cm.clients, tokenHash)
	delete(cm.currentDB, tokenHash)

	// Log with truncated hash for security
	hashPreview := tokenHash
	if len(tokenHash) > 12 {
		hashPreview = tokenHash[:12]
	}
	fmt.Fprintf(os.Stderr, "Removed all database connections for token hash: %s...\n", hashPreview)

	return nil
}

// RemoveClients removes and closes database clients for multiple token hashes
// This is useful for bulk cleanup when multiple tokens expire
func (cm *ClientManager) RemoveClients(tokenHashes []string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	removedCount := 0
	for _, tokenHash := range tokenHashes {
		if tokenClients, exists := cm.clients[tokenHash]; exists {
			// Close all connections for this token
			for _, client := range tokenClients {
				client.Close()
			}
			delete(cm.clients, tokenHash)
			delete(cm.currentDB, tokenHash)
			removedCount++
		}
	}

	if removedCount > 0 {
		fmt.Fprintf(os.Stderr, "Removed connections for %d token(s)\n", removedCount)
	}

	return nil
}

// CloseAll closes all managed database clients
// This should be called on server shutdown
func (cm *ClientManager) CloseAll() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for _, tokenClients := range cm.clients {
		for _, client := range tokenClients {
			client.Close()
		}
	}

	cm.clients = make(map[string]map[string]*Client)
	cm.currentDB = make(map[string]string)

	return nil
}

// IsConnected checks whether a non-closed client exists for the given
// token hash and database name. It does not create a new connection.
func (cm *ClientManager) IsConnected(tokenHash, dbName string) bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if tokenClients, exists := cm.clients[tokenHash]; exists {
		if client, exists := tokenClients[dbName]; exists && !client.IsClosed() {
			return true
		}
	}
	return false
}

// GetClientCount returns the number of active database client connections
// Useful for monitoring and testing
func (cm *ClientManager) GetClientCount() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.countClients()
}

// SetClientForDatabase sets a database client for a specific database name
// under the given key (token hash or "default").
func (cm *ClientManager) SetClientForDatabase(key, dbName string, client *Client) error {
	if key == "" {
		return fmt.Errorf("key is required")
	}
	if dbName == "" {
		return fmt.Errorf("database name is required")
	}
	if client == nil {
		return fmt.Errorf("client cannot be nil")
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Close existing client if it exists
	if tokenClients, exists := cm.clients[key]; exists {
		if existingClient, exists := tokenClients[dbName]; exists {
			existingClient.Close()
		}
	} else {
		cm.clients[key] = make(map[string]*Client)
	}

	cm.clients[key][dbName] = client
	return nil
}

// SetClient sets a database client for the given key (token hash or "default")
// This allows runtime configuration of database connections
// The client is associated with the default database
func (cm *ClientManager) SetClient(key string, client *Client) error {
	if key == "" {
		return fmt.Errorf("key is required")
	}
	if client == nil {
		return fmt.Errorf("client cannot be nil")
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	dbName := cm.defaultDBName
	if dbName == "" {
		dbName = "default"
	}

	// Close existing client if it exists
	if tokenClients, exists := cm.clients[key]; exists {
		if existingClient, exists := tokenClients[dbName]; exists {
			existingClient.Close()
		}
	} else {
		cm.clients[key] = make(map[string]*Client)
	}

	cm.clients[key][dbName] = client

	return nil
}

// GetOrCreateClient returns a database client for the given key
// If no client exists and autoConnect is true, creates and connects a new client
// If no client exists and autoConnect is false, returns an error
func (cm *ClientManager) GetOrCreateClient(key string, autoConnect bool) (*Client, error) {
	if key == "" {
		return nil, fmt.Errorf("key is required")
	}

	dbName := cm.GetCurrentDatabase(key)

	// Try to get existing client (read lock)
	cm.mu.RLock()
	if tokenClients, exists := cm.clients[key]; exists {
		if client, exists := tokenClients[dbName]; exists && !client.IsClosed() {
			cm.mu.RUnlock()
			return client, nil
		}
	}
	dbConfig := cm.dbConfigs[dbName]
	cm.mu.RUnlock()

	if !autoConnect {
		return nil, fmt.Errorf("no database connection configured - please call set_database_connection first")
	}

	if dbConfig == nil {
		return nil, fmt.Errorf("database '%s' not configured", dbName)
	}

	// Create new client (write lock)
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Double-check after acquiring write lock
	if tokenClients, exists := cm.clients[key]; exists {
		if client, exists := tokenClients[dbName]; exists && !client.IsClosed() {
			return client, nil
		}
	}

	// Create and initialize new client with database configuration
	client := NewClient(dbConfig)
	if err := client.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to database '%s': %w", dbName, err)
	}

	if err := client.LoadMetadata(); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to load metadata for database '%s': %w", dbName, err)
	}

	if cm.clients[key] == nil {
		cm.clients[key] = make(map[string]*Client)
	}
	cm.clients[key][dbName] = client

	return client, nil
}
