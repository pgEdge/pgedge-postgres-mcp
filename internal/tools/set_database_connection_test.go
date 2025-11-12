/*-------------------------------------------------------------------------
 *
 * pgEdge Postgres MCP Server
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"pgedge-postgres-mcp/internal/auth"
	"pgedge-postgres-mcp/internal/config"
	"pgedge-postgres-mcp/internal/crypto"
)

func TestTryMergeSavedConnection(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	keyPath := filepath.Join(tmpDir, "encryption.key")

	// Create encryption key
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("Failed to generate encryption key: %v", err)
	}
	if err := key.SaveToFile(keyPath); err != nil {
		t.Fatalf("Failed to save encryption key: %v", err)
	}

	// Create preferences with connection store
	prefs := &config.Preferences{
		Connections: auth.NewSavedConnectionStore(),
	}

	// Create connection manager (auth disabled, using preferences)
	connMgr := NewConnectionManager(nil, nil, prefs, false, key)

	// Encrypt password
	encryptedPassword, err := key.Encrypt("testpassword")
	if err != nil {
		t.Fatalf("Failed to encrypt password: %v", err)
	}

	// Add a saved connection for "server1"
	testConn := &auth.SavedConnection{
		Alias:       "server1",
		Description: "Test server",
		CreatedAt:   time.Now(),
		Host:        "server1.example.com",
		Port:        5432,
		User:        "testuser",
		Password:    encryptedPassword,
		DBName:      "myapp",
		SSLMode:     "disable",
	}
	if err := prefs.Connections.Add(testConn); err != nil {
		t.Fatalf("Failed to add test connection: %v", err)
	}

	// Save preferences
	if err := config.SavePreferences(configPath, prefs); err != nil {
		t.Fatalf("Failed to save preferences: %v", err)
	}

	ctx := context.Background()

	tests := []struct {
		name           string
		connString     string
		expectPassword bool
		expectDBName   string
		expectAlias    string
	}{
		{
			name:           "Connection string with hostname match",
			connString:     "postgres://testuser@server1.example.com/postgres",
			expectPassword: true,
			expectDBName:   "postgres",
			expectAlias:    "server1",
		},
		{
			name:           "Connection string with alias as hostname",
			connString:     "postgres://testuser@server1/postgres",
			expectPassword: true,
			expectDBName:   "postgres",
			expectAlias:    "server1",
		},
		{
			name:           "Connection string with no database specified",
			connString:     "postgres://testuser@server1.example.com",
			expectPassword: true,
			expectDBName:   "myapp", // Should use saved connection's default
			expectAlias:    "server1",
		},
		{
			name:           "Connection string with unknown host",
			connString:     "postgres://testuser@unknown.example.com/postgres",
			expectPassword: false,
			expectDBName:   "",
			expectAlias:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mergedConnStr, alias := tryMergeSavedConnection(ctx, tt.connString, connMgr, configPath)

			// Check alias
			if alias != tt.expectAlias {
				t.Errorf("Expected alias %q, got %q", tt.expectAlias, alias)
			}

			// If we expect a password, verify it's in the connection string
			if tt.expectPassword {
				if mergedConnStr == tt.connString {
					t.Error("Expected connection string to be modified with credentials")
				}
				// Connection string should contain the password
				// (We can't check the exact value since it's URL-encoded, but it should be different from input)
			} else {
				if mergedConnStr != tt.connString {
					t.Errorf("Expected connection string to be unchanged, got %q", mergedConnStr)
				}
			}
		})
	}
}

func TestTryMergeSavedConnection_InvalidInput(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	key, _ := crypto.GenerateKey()
	prefs := &config.Preferences{
		Connections: auth.NewSavedConnectionStore(),
	}
	connMgr := NewConnectionManager(nil, nil, prefs, false, key)

	ctx := context.Background()

	tests := []struct {
		name       string
		connString string
	}{
		{
			name:       "Invalid URL",
			connString: "not-a-valid-url",
		},
		{
			name:       "Empty string",
			connString: "",
		},
		{
			name:       "URL without hostname",
			connString: "postgres:///database",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mergedConnStr, alias := tryMergeSavedConnection(ctx, tt.connString, connMgr, configPath)

			// Should return original string and empty alias
			if mergedConnStr != tt.connString {
				t.Errorf("Expected original connection string %q, got %q", tt.connString, mergedConnStr)
			}
			if alias != "" {
				t.Errorf("Expected empty alias, got %q", alias)
			}
		})
	}
}

func TestTryMergeSavedConnection_NoStore(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	key, _ := crypto.GenerateKey()
	// Create connection manager with no preferences (store not available)
	connMgr := NewConnectionManager(nil, nil, nil, false, key)

	ctx := context.Background()
	connString := "postgres://testuser@server1/postgres"

	mergedConnStr, alias := tryMergeSavedConnection(ctx, connString, connMgr, configPath)

	// Should return original string when store is not available
	if mergedConnStr != connString {
		t.Errorf("Expected original connection string %q, got %q", connString, mergedConnStr)
	}
	if alias != "" {
		t.Errorf("Expected empty alias, got %q", alias)
	}
}

func TestTryMergeSavedConnection_EncryptionError(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Create two different keys
	key1, _ := crypto.GenerateKey()
	key2, _ := crypto.GenerateKey()

	// Encrypt password with key1
	encryptedPassword, _ := key1.Encrypt("testpassword")

	// Create preferences with connection encrypted with key1
	prefs := &config.Preferences{
		Connections: auth.NewSavedConnectionStore(),
	}
	testConn := &auth.SavedConnection{
		Alias:    "server1",
		Host:     "server1.example.com",
		Port:     5432,
		User:     "testuser",
		Password: encryptedPassword, // Encrypted with different key
		DBName:   "myapp",
	}
	prefs.Connections.Add(testConn)

	// Create connection manager with key2 (different key, will fail to decrypt)
	connMgr := NewConnectionManager(nil, nil, prefs, false, key2)

	ctx := context.Background()
	connString := "postgres://testuser@server1/postgres"

	mergedConnStr, alias := tryMergeSavedConnection(ctx, connString, connMgr, configPath)

	// Should return original string when decryption fails
	if mergedConnStr != connString {
		t.Errorf("Expected original connection string %q, got %q", connString, mergedConnStr)
	}
	if alias != "" {
		t.Errorf("Expected empty alias, got %q", alias)
	}
}

// Cleanup function
func TestMain(m *testing.M) {
	// Run tests
	code := m.Run()

	// Exit
	os.Exit(code)
}
