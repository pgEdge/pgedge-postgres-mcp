/*-------------------------------------------------------------------------
 *
 * pgEdge Postgres MCP Server
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package auth

import (
	"fmt"
	"time"
)

// SavedConnection represents a stored database connection with alias
type SavedConnection struct {
	Alias            string    `yaml:"alias" json:"alias"`
	ConnectionString string    `yaml:"connection_string" json:"connection_string"`
	MaintenanceDB    string    `yaml:"maintenance_db" json:"maintenance_db"`
	Description      string    `yaml:"description,omitempty" json:"description,omitempty"`
	CreatedAt        time.Time `yaml:"created_at" json:"created_at"`
	LastUsedAt       time.Time `yaml:"last_used_at,omitempty" json:"last_used_at,omitempty"`
}

// SavedConnectionStore manages a collection of saved database connections
type SavedConnectionStore struct {
	Connections map[string]*SavedConnection `yaml:"connections" json:"connections"`
}

// NewSavedConnectionStore creates a new empty connection store
func NewSavedConnectionStore() *SavedConnectionStore {
	return &SavedConnectionStore{
		Connections: make(map[string]*SavedConnection),
	}
}

// Add adds a new saved connection
func (s *SavedConnectionStore) Add(alias, connectionString, maintenanceDB, description string) error {
	if alias == "" {
		return fmt.Errorf("alias cannot be empty")
	}
	if connectionString == "" {
		return fmt.Errorf("connection string cannot be empty")
	}
	if maintenanceDB == "" {
		maintenanceDB = "postgres" // Default maintenance database
	}

	if _, exists := s.Connections[alias]; exists {
		return fmt.Errorf("connection with alias '%s' already exists", alias)
	}

	s.Connections[alias] = &SavedConnection{
		Alias:            alias,
		ConnectionString: connectionString,
		MaintenanceDB:    maintenanceDB,
		Description:      description,
		CreatedAt:        time.Now(),
	}

	return nil
}

// Remove removes a saved connection by alias
func (s *SavedConnectionStore) Remove(alias string) error {
	if _, exists := s.Connections[alias]; !exists {
		return fmt.Errorf("connection with alias '%s' not found", alias)
	}

	delete(s.Connections, alias)
	return nil
}

// Get retrieves a saved connection by alias
func (s *SavedConnectionStore) Get(alias string) (*SavedConnection, error) {
	conn, exists := s.Connections[alias]
	if !exists {
		return nil, fmt.Errorf("connection with alias '%s' not found", alias)
	}
	return conn, nil
}

// Update updates an existing saved connection
func (s *SavedConnectionStore) Update(alias, connectionString, maintenanceDB, description string) error {
	conn, exists := s.Connections[alias]
	if !exists {
		return fmt.Errorf("connection with alias '%s' not found", alias)
	}

	if connectionString != "" {
		conn.ConnectionString = connectionString
	}
	if maintenanceDB != "" {
		conn.MaintenanceDB = maintenanceDB
	}
	if description != "" {
		conn.Description = description
	}

	return nil
}

// MarkUsed updates the last used timestamp for a connection
func (s *SavedConnectionStore) MarkUsed(alias string) error {
	conn, exists := s.Connections[alias]
	if !exists {
		return fmt.Errorf("connection with alias '%s' not found", alias)
	}

	conn.LastUsedAt = time.Now()
	return nil
}

// List returns all saved connections
func (s *SavedConnectionStore) List() []*SavedConnection {
	connections := make([]*SavedConnection, 0, len(s.Connections))
	for _, conn := range s.Connections {
		connections = append(connections, conn)
	}
	return connections
}

// Count returns the number of saved connections
func (s *SavedConnectionStore) Count() int {
	return len(s.Connections)
}
