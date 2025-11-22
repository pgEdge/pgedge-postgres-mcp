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
	"fmt"
	"os"
	"sync"

	"pgedge-postgres-mcp/internal/config"
)

// ClientManager manages per-token database clients for connection isolation
// Each authenticated token gets its own database client to prevent connection sharing
type ClientManager struct {
	mu       sync.RWMutex
	clients  map[string]*Client     // map of token hash -> client
	dbConfig *config.DatabaseConfig // database configuration for new clients
}

// NewClientManager creates a new client manager with optional database configuration
func NewClientManager(dbConfig *config.DatabaseConfig) *ClientManager {
	return &ClientManager{
		clients:  make(map[string]*Client),
		dbConfig: dbConfig,
	}
}

// GetClient returns a database client for the given token hash
// Creates a new client if one doesn't exist for this token
func (cm *ClientManager) GetClient(tokenHash string) (*Client, error) {
	if tokenHash == "" {
		return nil, fmt.Errorf("token hash is required for authenticated requests")
	}

	// Try to get existing client (read lock)
	cm.mu.RLock()
	if client, exists := cm.clients[tokenHash]; exists {
		cm.mu.RUnlock()
		return client, nil
	}
	cm.mu.RUnlock()

	// Create new client (write lock)
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Double-check after acquiring write lock
	if client, exists := cm.clients[tokenHash]; exists {
		return client, nil
	}

	// Create and initialize new client with database configuration
	client := NewClient(cm.dbConfig)
	if err := client.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect database for token: %w", err)
	}

	if err := client.LoadMetadata(); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to load metadata for token: %w", err)
	}

	cm.clients[tokenHash] = client

	// Log with truncated hash for security (only if hash is long enough)
	hashPreview := tokenHash
	if len(tokenHash) > 12 {
		hashPreview = tokenHash[:12]
	}
	fmt.Fprintf(os.Stderr, "Created new database connection for token hash: %s... (total: %d)\n",
		hashPreview, len(cm.clients))

	return client, nil
}

// RemoveClient removes and closes the database client for the given token hash
// This should be called when a token is removed or expires
func (cm *ClientManager) RemoveClient(tokenHash string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	client, exists := cm.clients[tokenHash]
	if !exists {
		return nil // Already removed
	}

	// Close the client connection
	client.Close()

	// Remove from map
	delete(cm.clients, tokenHash)

	// Log with truncated hash for security (only if hash is long enough)
	hashPreview := tokenHash
	if len(tokenHash) > 12 {
		hashPreview = tokenHash[:12]
	}
	fmt.Fprintf(os.Stderr, "Removed database connection for token hash: %s... (remaining: %d)\n",
		hashPreview, len(cm.clients))

	return nil
}

// RemoveClients removes and closes database clients for multiple token hashes
// This is useful for bulk cleanup when multiple tokens expire
func (cm *ClientManager) RemoveClients(tokenHashes []string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for _, tokenHash := range tokenHashes {
		if client, exists := cm.clients[tokenHash]; exists {
			// Close the client connection
			client.Close()

			// Remove from map
			delete(cm.clients, tokenHash)
		}
	}

	if len(tokenHashes) > 0 {
		fmt.Fprintf(os.Stderr, "Removed %d database connection(s) (remaining: %d)\n",
			len(tokenHashes), len(cm.clients))
	}

	return nil
}

// CloseAll closes all managed database clients
// This should be called on server shutdown
func (cm *ClientManager) CloseAll() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for _, client := range cm.clients {
		client.Close()
	}

	cm.clients = make(map[string]*Client)

	return nil
}

// GetClientCount returns the number of active database clients
// Useful for monitoring and testing
func (cm *ClientManager) GetClientCount() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return len(cm.clients)
}

// SetClient sets a database client for the given key (token hash or "default")
// This allows runtime configuration of database connections
func (cm *ClientManager) SetClient(key string, client *Client) error {
	if key == "" {
		return fmt.Errorf("key is required")
	}
	if client == nil {
		return fmt.Errorf("client cannot be nil")
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Close existing client if it exists
	if existingClient, exists := cm.clients[key]; exists {
		existingClient.Close()
	}

	cm.clients[key] = client

	return nil
}

// GetOrCreateClient returns a database client for the given key
// If no client exists and autoConnect is true, creates and connects a new client using PGEDGE_POSTGRES_CONNECTION_STRING
// If no client exists and autoConnect is false, returns an error
func (cm *ClientManager) GetOrCreateClient(key string, autoConnect bool) (*Client, error) {
	if key == "" {
		return nil, fmt.Errorf("key is required")
	}

	// Try to get existing client (read lock)
	cm.mu.RLock()
	if client, exists := cm.clients[key]; exists {
		cm.mu.RUnlock()
		return client, nil
	}
	cm.mu.RUnlock()

	if !autoConnect {
		return nil, fmt.Errorf("no database connection configured - please call set_database_connection first")
	}

	// Create new client (write lock)
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Double-check after acquiring write lock
	if client, exists := cm.clients[key]; exists {
		return client, nil
	}

	// Create and initialize new client with database configuration
	client := NewClient(cm.dbConfig)
	if err := client.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect database: %w", err)
	}

	if err := client.LoadMetadata(); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to load metadata: %w", err)
	}

	cm.clients[key] = client

	fmt.Fprintf(os.Stderr, "Created new database connection for key: %s (total: %d)\n", key, len(cm.clients))

	return client, nil
}
