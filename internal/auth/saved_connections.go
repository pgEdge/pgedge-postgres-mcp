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
	"net/url"
	"strconv"
	"time"
)

// SavedConnection represents a stored database connection with alias
// All connection parameters are stored separately for security and flexibility
type SavedConnection struct {
	Alias       string    `yaml:"alias" json:"alias"`
	Description string    `yaml:"description,omitempty" json:"description,omitempty"`
	CreatedAt   time.Time `yaml:"created_at" json:"created_at"`
	LastUsedAt  time.Time `yaml:"last_used_at,omitempty" json:"last_used_at,omitempty"`

	// Database connection parameters
	Host     string `yaml:"host,omitempty" json:"host,omitempty"`         // hostname or IP address
	Port     int    `yaml:"port,omitempty" json:"port,omitempty"`         // port number (default 5432)
	User     string `yaml:"user,omitempty" json:"user,omitempty"`         // database user
	Password string `yaml:"password,omitempty" json:"password,omitempty"` // encrypted password
	DBName   string `yaml:"dbname,omitempty" json:"dbname,omitempty"`     // database name

	// SSL/TLS parameters
	SSLMode       string `yaml:"sslmode,omitempty" json:"sslmode,omitempty"`             // disable, allow, prefer, require, verify-ca, verify-full
	SSLCert       string `yaml:"sslcert,omitempty" json:"sslcert,omitempty"`             // path to client certificate
	SSLKey        string `yaml:"sslkey,omitempty" json:"sslkey,omitempty"`               // path to client key
	SSLRootCert   string `yaml:"sslrootcert,omitempty" json:"sslrootcert,omitempty"`     // path to root CA certificate
	SSLPassword   string `yaml:"sslpassword,omitempty" json:"sslpassword,omitempty"`     // encrypted password for client key
	SSLCRL        string `yaml:"sslcrl,omitempty" json:"sslcrl,omitempty"`               // path to certificate revocation list

	// Additional connection parameters
	ConnectTimeout   int    `yaml:"connect_timeout,omitempty" json:"connect_timeout,omitempty"`     // connection timeout in seconds
	ApplicationName  string `yaml:"application_name,omitempty" json:"application_name,omitempty"`   // application name
}

// ToConnectionString builds a PostgreSQL connection string from the parameters
// The password parameter should be the decrypted password (caller is responsible for decryption)
func (c *SavedConnection) ToConnectionString(decryptedPassword string) string {
	// Build connection string in PostgreSQL connection URL format
	// postgres://[user[:password]@][host][:port]/[dbname][?param=value&...]

	// Build base URL
	baseURL := "postgres://"

	// Add user and password
	if c.User != "" {
		baseURL += url.QueryEscape(c.User)
		if decryptedPassword != "" {
			baseURL += ":" + url.QueryEscape(decryptedPassword)
		}
		baseURL += "@"
	}

	// Add host and port
	if c.Host != "" {
		baseURL += c.Host
	}
	if c.Port != 0 && c.Port != 5432 {
		baseURL += ":" + strconv.Itoa(c.Port)
	}

	// Add database name
	baseURL += "/"
	if c.DBName != "" {
		baseURL += c.DBName
	}

	// Build query parameters
	params := url.Values{}

	// SSL parameters
	if c.SSLMode != "" {
		params.Add("sslmode", c.SSLMode)
	}
	if c.SSLCert != "" {
		params.Add("sslcert", c.SSLCert)
	}
	if c.SSLKey != "" {
		params.Add("sslkey", c.SSLKey)
	}
	if c.SSLRootCert != "" {
		params.Add("sslrootcert", c.SSLRootCert)
	}
	if c.SSLCRL != "" {
		params.Add("sslcrl", c.SSLCRL)
	}

	// Additional parameters
	if c.ConnectTimeout != 0 {
		params.Add("connect_timeout", strconv.Itoa(c.ConnectTimeout))
	}
	if c.ApplicationName != "" {
		params.Add("application_name", c.ApplicationName)
	}

	// Add query string if there are parameters
	if len(params) > 0 {
		baseURL += "?" + params.Encode()
	}

	return baseURL
}

// Clone creates a deep copy of the connection (useful for updates)
func (c *SavedConnection) Clone() *SavedConnection {
	clone := *c // Shallow copy
	return &clone
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
func (s *SavedConnectionStore) Add(conn *SavedConnection) error {
	if conn.Alias == "" {
		return fmt.Errorf("alias cannot be empty")
	}
	if conn.Host == "" {
		return fmt.Errorf("host cannot be empty")
	}
	if conn.User == "" {
		return fmt.Errorf("user cannot be empty")
	}

	// Set defaults
	if conn.Port == 0 {
		conn.Port = 5432 // Default PostgreSQL port
	}
	if conn.DBName == "" {
		conn.DBName = conn.User // Default to username if not specified
	}

	if _, exists := s.Connections[conn.Alias]; exists {
		return fmt.Errorf("connection with alias '%s' already exists", conn.Alias)
	}

	// Set timestamps
	conn.CreatedAt = time.Now()

	s.Connections[conn.Alias] = conn

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

// Update updates an existing saved connection with provided values
// Only non-zero values are updated
func (s *SavedConnectionStore) Update(alias string, updates *SavedConnection) error {
	conn, exists := s.Connections[alias]
	if !exists {
		return fmt.Errorf("connection with alias '%s' not found", alias)
	}

	// Update only non-empty/non-zero fields
	if updates.Host != "" {
		conn.Host = updates.Host
	}
	if updates.Port != 0 {
		conn.Port = updates.Port
	}
	if updates.User != "" {
		conn.User = updates.User
	}
	if updates.Password != "" {
		conn.Password = updates.Password
	}
	if updates.DBName != "" {
		conn.DBName = updates.DBName
	}
	if updates.Description != "" {
		conn.Description = updates.Description
	}

	// SSL parameters
	if updates.SSLMode != "" {
		conn.SSLMode = updates.SSLMode
	}
	if updates.SSLCert != "" {
		conn.SSLCert = updates.SSLCert
	}
	if updates.SSLKey != "" {
		conn.SSLKey = updates.SSLKey
	}
	if updates.SSLRootCert != "" {
		conn.SSLRootCert = updates.SSLRootCert
	}
	if updates.SSLPassword != "" {
		conn.SSLPassword = updates.SSLPassword
	}
	if updates.SSLCRL != "" {
		conn.SSLCRL = updates.SSLCRL
	}

	// Additional parameters
	if updates.ConnectTimeout != 0 {
		conn.ConnectTimeout = updates.ConnectTimeout
	}
	if updates.ApplicationName != "" {
		conn.ApplicationName = updates.ApplicationName
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
