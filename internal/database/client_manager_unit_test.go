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
	"testing"

	"pgedge-postgres-mcp/internal/config"
)

func TestNewClientManager(t *testing.T) {
	databases := []config.NamedDatabaseConfig{
		{Name: "db1", Host: "localhost", Port: 5432, Database: "test1"},
		{Name: "db2", Host: "localhost", Port: 5433, Database: "test2"},
	}

	cm := NewClientManager(databases)

	if cm == nil {
		t.Fatal("expected non-nil client manager")
	}

	// Check default database is first one
	if cm.GetDefaultDatabaseName() != "db1" {
		t.Errorf("expected default database 'db1', got %q", cm.GetDefaultDatabaseName())
	}

	// Check all databases are configured
	names := cm.ListDatabaseNames()
	if len(names) != 2 {
		t.Errorf("expected 2 database names, got %d", len(names))
	}
}

func TestNewClientManager_EmptyDatabases(t *testing.T) {
	cm := NewClientManager([]config.NamedDatabaseConfig{})

	if cm == nil {
		t.Fatal("expected non-nil client manager")
	}

	// Check no default database
	if cm.GetDefaultDatabaseName() != "" {
		t.Errorf("expected empty default database name, got %q", cm.GetDefaultDatabaseName())
	}

	// Check no databases configured
	names := cm.ListDatabaseNames()
	if len(names) != 0 {
		t.Errorf("expected 0 database names, got %d", len(names))
	}
}

func TestNewClientManagerWithConfig_Nil(t *testing.T) {
	cm := NewClientManagerWithConfig(nil)

	if cm == nil {
		t.Fatal("expected non-nil client manager")
	}

	// Check no databases configured
	if len(cm.ListDatabaseNames()) != 0 {
		t.Errorf("expected 0 databases for nil config, got %d", len(cm.ListDatabaseNames()))
	}
}

func TestNewClientManagerWithConfig_WithConfig(t *testing.T) {
	cfg := &config.NamedDatabaseConfig{
		Name:     "testdb",
		Host:     "localhost",
		Port:     5432,
		Database: "mydb",
	}

	cm := NewClientManagerWithConfig(cfg)

	if cm == nil {
		t.Fatal("expected non-nil client manager")
	}

	// Check database is configured
	if cm.GetDefaultDatabaseName() != "testdb" {
		t.Errorf("expected default 'testdb', got %q", cm.GetDefaultDatabaseName())
	}

	retrievedCfg := cm.GetDatabaseConfig("testdb")
	if retrievedCfg == nil {
		t.Fatal("expected database config to be retrievable")
	}
	if retrievedCfg.Host != "localhost" {
		t.Errorf("expected host 'localhost', got %q", retrievedCfg.Host)
	}
}

func TestNewClientManagerWithConfig_EmptyName(t *testing.T) {
	cfg := &config.NamedDatabaseConfig{
		Name:     "", // Empty name
		Host:     "localhost",
		Port:     5432,
		Database: "mydb",
	}

	cm := NewClientManagerWithConfig(cfg)

	// Should default to "default" name
	if cm.GetDefaultDatabaseName() != "default" {
		t.Errorf("expected default name 'default' for empty config name, got %q",
			cm.GetDefaultDatabaseName())
	}
}

func TestClientManager_GetClient_EmptyTokenHash(t *testing.T) {
	cm := NewClientManager([]config.NamedDatabaseConfig{
		{Name: "db1", Host: "localhost", Port: 5432, Database: "test1"},
	})

	_, err := cm.GetClient("")
	if err == nil {
		t.Fatal("expected error for empty token hash")
	}
	if err.Error() != "token hash is required for authenticated requests" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestClientManager_GetClientForDatabase_EmptyTokenHash(t *testing.T) {
	cm := NewClientManager([]config.NamedDatabaseConfig{
		{Name: "db1", Host: "localhost", Port: 5432, Database: "test1"},
	})

	_, err := cm.GetClientForDatabase("", "db1")
	if err == nil {
		t.Fatal("expected error for empty token hash")
	}
}

func TestClientManager_GetClientForDatabase_UnconfiguredDB(t *testing.T) {
	cm := NewClientManager([]config.NamedDatabaseConfig{
		{Name: "db1", Host: "localhost", Port: 5432, Database: "test1"},
	})

	_, err := cm.GetClientForDatabase("token-hash", "nonexistent")
	if err == nil {
		t.Fatal("expected error for unconfigured database")
	}
	if err.Error() != "database 'nonexistent' not configured" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestClientManager_SetCurrentDatabase(t *testing.T) {
	cm := NewClientManager([]config.NamedDatabaseConfig{
		{Name: "db1", Host: "localhost", Port: 5432, Database: "test1"},
		{Name: "db2", Host: "localhost", Port: 5433, Database: "test2"},
	})

	t.Run("empty token hash", func(t *testing.T) {
		err := cm.SetCurrentDatabase("", "db1")
		if err == nil {
			t.Fatal("expected error for empty token hash")
		}
	})

	t.Run("non-existent database", func(t *testing.T) {
		err := cm.SetCurrentDatabase("token", "nonexistent")
		if err == nil {
			t.Fatal("expected error for non-existent database")
		}
	})

	t.Run("valid database", func(t *testing.T) {
		err := cm.SetCurrentDatabase("token", "db2")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		current := cm.GetCurrentDatabase("token")
		if current != "db2" {
			t.Errorf("expected current 'db2', got %q", current)
		}
	})
}

func TestClientManager_SetCurrentDatabaseAndCloseOthers(t *testing.T) {
	cm := NewClientManager([]config.NamedDatabaseConfig{
		{Name: "db1", Host: "localhost", Port: 5432, Database: "test1"},
		{Name: "db2", Host: "localhost", Port: 5433, Database: "test2"},
	})

	t.Run("empty token hash", func(t *testing.T) {
		err := cm.SetCurrentDatabaseAndCloseOthers("", "db1")
		if err == nil {
			t.Fatal("expected error for empty token hash")
		}
	})

	t.Run("non-existent database", func(t *testing.T) {
		err := cm.SetCurrentDatabaseAndCloseOthers("token", "nonexistent")
		if err == nil {
			t.Fatal("expected error for non-existent database")
		}
	})

	t.Run("valid database", func(t *testing.T) {
		err := cm.SetCurrentDatabaseAndCloseOthers("token", "db1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		current := cm.GetCurrentDatabase("token")
		if current != "db1" {
			t.Errorf("expected current 'db1', got %q", current)
		}
	})
}

func TestClientManager_GetCurrentDatabase_Default(t *testing.T) {
	cm := NewClientManager([]config.NamedDatabaseConfig{
		{Name: "db1", Host: "localhost", Port: 5432, Database: "test1"},
		{Name: "db2", Host: "localhost", Port: 5433, Database: "test2"},
	})

	// Without setting, should return default
	current := cm.GetCurrentDatabase("some-token")
	if current != "db1" {
		t.Errorf("expected default 'db1', got %q", current)
	}
}

func TestClientManager_GetDatabaseConfig(t *testing.T) {
	cm := NewClientManager([]config.NamedDatabaseConfig{
		{Name: "db1", Host: "host1", Port: 5432, Database: "test1"},
	})

	t.Run("existing database", func(t *testing.T) {
		cfg := cm.GetDatabaseConfig("db1")
		if cfg == nil {
			t.Fatal("expected config, got nil")
		}
		if cfg.Host != "host1" {
			t.Errorf("expected host 'host1', got %q", cfg.Host)
		}
	})

	t.Run("non-existent database", func(t *testing.T) {
		cfg := cm.GetDatabaseConfig("nonexistent")
		if cfg != nil {
			t.Errorf("expected nil for non-existent database, got %v", cfg)
		}
	})
}

func TestClientManager_ListDatabaseNames(t *testing.T) {
	cm := NewClientManager([]config.NamedDatabaseConfig{
		{Name: "db1", Host: "localhost", Port: 5432, Database: "test1"},
		{Name: "db2", Host: "localhost", Port: 5433, Database: "test2"},
		{Name: "db3", Host: "localhost", Port: 5434, Database: "test3"},
	})

	names := cm.ListDatabaseNames()
	if len(names) != 3 {
		t.Errorf("expected 3 names, got %d", len(names))
	}

	// Check all names are present (order may vary)
	nameSet := make(map[string]bool)
	for _, name := range names {
		nameSet[name] = true
	}
	for _, expected := range []string{"db1", "db2", "db3"} {
		if !nameSet[expected] {
			t.Errorf("expected name '%s' to be in list", expected)
		}
	}
}

func TestClientManager_GetDatabaseConfigs(t *testing.T) {
	cm := NewClientManager([]config.NamedDatabaseConfig{
		{Name: "db1", Host: "host1", Port: 5432, Database: "test1"},
		{Name: "db2", Host: "host2", Port: 5433, Database: "test2"},
	})

	configs := cm.GetDatabaseConfigs()
	if len(configs) != 2 {
		t.Errorf("expected 2 configs, got %d", len(configs))
	}
}

func TestClientManager_UpdateDatabaseConfigs(t *testing.T) {
	cm := NewClientManager([]config.NamedDatabaseConfig{
		{Name: "db1", Host: "host1", Port: 5432, Database: "test1"},
		{Name: "db2", Host: "host2", Port: 5433, Database: "test2"},
	})

	// Set current database for a token
	_ = cm.SetCurrentDatabase("token1", "db1")

	// Update with new configs (removing db1, keeping db2, adding db3)
	newConfigs := []config.NamedDatabaseConfig{
		{Name: "db2", Host: "host2-updated", Port: 5433, Database: "test2"},
		{Name: "db3", Host: "host3", Port: 5434, Database: "test3"},
	}
	cm.UpdateDatabaseConfigs(newConfigs)

	// Check db1 is gone
	if cfg := cm.GetDatabaseConfig("db1"); cfg != nil {
		t.Error("expected db1 to be removed")
	}

	// Check db2 is updated
	cfg := cm.GetDatabaseConfig("db2")
	if cfg == nil {
		t.Fatal("expected db2 to exist")
	}
	if cfg.Host != "host2-updated" {
		t.Errorf("expected updated host 'host2-updated', got %q", cfg.Host)
	}

	// Check db3 is added
	if cfg := cm.GetDatabaseConfig("db3"); cfg == nil {
		t.Error("expected db3 to be added")
	}

	// Check default changed to first new database
	if cm.GetDefaultDatabaseName() != "db2" {
		t.Errorf("expected default 'db2', got %q", cm.GetDefaultDatabaseName())
	}
}

func TestClientManager_SetClient_Validation(t *testing.T) {
	cm := NewClientManager([]config.NamedDatabaseConfig{
		{Name: "db1", Host: "localhost", Port: 5432, Database: "test1"},
	})

	t.Run("empty key", func(t *testing.T) {
		client := NewClient(nil)
		err := cm.SetClient("", client)
		if err == nil {
			t.Fatal("expected error for empty key")
		}
		if err.Error() != "key is required" {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("nil client", func(t *testing.T) {
		err := cm.SetClient("token", nil)
		if err == nil {
			t.Fatal("expected error for nil client")
		}
		if err.Error() != "client cannot be nil" {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("valid client", func(t *testing.T) {
		client := NewClient(nil)
		err := cm.SetClient("token", client)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Check client count
		if count := cm.GetClientCount(); count != 1 {
			t.Errorf("expected 1 client, got %d", count)
		}
	})
}

func TestClientManager_GetOrCreateClient_Validation(t *testing.T) {
	cm := NewClientManager([]config.NamedDatabaseConfig{
		{Name: "db1", Host: "localhost", Port: 5432, Database: "test1"},
	})

	t.Run("empty key", func(t *testing.T) {
		_, err := cm.GetOrCreateClient("", false)
		if err == nil {
			t.Fatal("expected error for empty key")
		}
		if err.Error() != "key is required" {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("no existing client and autoConnect false", func(t *testing.T) {
		_, err := cm.GetOrCreateClient("new-token", false)
		if err == nil {
			t.Fatal("expected error when no client and autoConnect=false")
		}
		if err.Error() != "no database connection configured - please call set_database_connection first" {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestClientManager_RemoveClient_Empty(t *testing.T) {
	cm := NewClientManager([]config.NamedDatabaseConfig{})

	// Removing from empty manager should not error
	err := cm.RemoveClient("nonexistent")
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestClientManager_RemoveClients_Empty(t *testing.T) {
	cm := NewClientManager([]config.NamedDatabaseConfig{})

	// Removing from empty manager should not error
	err := cm.RemoveClients([]string{"a", "b", "c"})
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestClientManager_CloseAll_Empty(t *testing.T) {
	cm := NewClientManager([]config.NamedDatabaseConfig{})

	err := cm.CloseAll()
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	if count := cm.GetClientCount(); count != 0 {
		t.Errorf("expected 0 clients, got %d", count)
	}
}

func TestClientManager_GetClientCount_Empty(t *testing.T) {
	cm := NewClientManager([]config.NamedDatabaseConfig{})

	if count := cm.GetClientCount(); count != 0 {
		t.Errorf("expected 0 clients for empty manager, got %d", count)
	}
}
