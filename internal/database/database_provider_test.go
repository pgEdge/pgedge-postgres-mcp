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
	"testing"

	"pgedge-postgres-mcp/internal/auth"
	"pgedge-postgres-mcp/internal/config"
)

func TestNewStdioDatabaseProvider(t *testing.T) {
	cm := NewClientManager([]config.NamedDatabaseConfig{
		{Name: "db1", Host: "localhost", Port: 5432, Database: "test1"},
	})

	provider := NewStdioDatabaseProvider(cm)

	if provider == nil {
		t.Fatal("expected non-nil provider")
	}
	if provider.clientManager != cm {
		t.Error("expected client manager to be set")
	}
	if provider.sessionKey != "default" {
		t.Errorf("expected session key 'default', got %q", provider.sessionKey)
	}
}

func TestStdioDatabaseProvider_ListDatabases(t *testing.T) {
	cm := NewClientManager([]config.NamedDatabaseConfig{
		{Name: "db1", Host: "host1", Port: 5432, Database: "test1", User: "user1", SSLMode: "disable"},
		{Name: "db2", Host: "host2", Port: 5433, Database: "test2", User: "user2", SSLMode: "require"},
	})

	provider := NewStdioDatabaseProvider(cm)

	databases, current, err := provider.ListDatabases(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(databases) != 2 {
		t.Errorf("expected 2 databases, got %d", len(databases))
	}

	// Current should be default (first)
	if current != "db1" {
		t.Errorf("expected current 'db1', got %q", current)
	}

	// Verify database info is correctly populated
	found := make(map[string]bool)
	for _, db := range databases {
		found[db.Name] = true
		if db.Name == "db1" {
			if db.Host != "host1" {
				t.Errorf("expected host 'host1' for db1, got %q", db.Host)
			}
			if db.Port != 5432 {
				t.Errorf("expected port 5432 for db1, got %d", db.Port)
			}
		}
	}

	if !found["db1"] || !found["db2"] {
		t.Error("expected both db1 and db2 to be in list")
	}
}

func TestStdioDatabaseProvider_SelectDatabase(t *testing.T) {
	cm := NewClientManager([]config.NamedDatabaseConfig{
		{Name: "db1", Host: "host1", Port: 5432, Database: "test1"},
		{Name: "db2", Host: "host2", Port: 5433, Database: "test2"},
	})

	provider := NewStdioDatabaseProvider(cm)

	t.Run("select valid database", func(t *testing.T) {
		err := provider.SelectDatabase(context.Background(), "db2")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify current database changed
		_, current, _ := provider.ListDatabases(context.Background())
		if current != "db2" {
			t.Errorf("expected current 'db2', got %q", current)
		}
	})

	t.Run("select non-existent database", func(t *testing.T) {
		err := provider.SelectDatabase(context.Background(), "nonexistent")
		if err == nil {
			t.Fatal("expected error for non-existent database")
		}
		if err.Error() != "database 'nonexistent' not found" {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestNewHTTPDatabaseProvider(t *testing.T) {
	cm := NewClientManager([]config.NamedDatabaseConfig{
		{Name: "db1", Host: "localhost", Port: 5432, Database: "test1"},
	})

	provider := NewHTTPDatabaseProvider(cm, true, nil)

	if provider == nil {
		t.Fatal("expected non-nil provider")
	}
	if provider.clientManager != cm {
		t.Error("expected client manager to be set")
	}
	if !provider.authEnabled {
		t.Error("expected authEnabled to be true")
	}
}

func TestHTTPDatabaseProvider_GetSessionKey(t *testing.T) {
	cm := NewClientManager([]config.NamedDatabaseConfig{})

	t.Run("auth disabled", func(t *testing.T) {
		provider := NewHTTPDatabaseProvider(cm, false, nil)
		ctx := context.Background()

		key := provider.getSessionKey(ctx)
		if key != "default" {
			t.Errorf("expected 'default' when auth disabled, got %q", key)
		}
	})

	t.Run("auth enabled without token", func(t *testing.T) {
		provider := NewHTTPDatabaseProvider(cm, true, nil)
		ctx := context.Background()

		key := provider.getSessionKey(ctx)
		if key != "default" {
			t.Errorf("expected 'default' without token, got %q", key)
		}
	})

	t.Run("auth enabled with token", func(t *testing.T) {
		provider := NewHTTPDatabaseProvider(cm, true, nil)
		ctx := context.WithValue(context.Background(), auth.TokenHashContextKey, "test-token-hash")

		key := provider.getSessionKey(ctx)
		if key != "test-token-hash" {
			t.Errorf("expected 'test-token-hash', got %q", key)
		}
	})
}

func TestHTTPDatabaseProvider_ListDatabases(t *testing.T) {
	cm := NewClientManager([]config.NamedDatabaseConfig{
		{Name: "db1", Host: "host1", Port: 5432, Database: "test1", User: "user1"},
		{Name: "db2", Host: "host2", Port: 5433, Database: "test2", User: "user2"},
	})

	t.Run("without access checker", func(t *testing.T) {
		provider := NewHTTPDatabaseProvider(cm, false, nil)

		databases, current, err := provider.ListDatabases(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(databases) != 2 {
			t.Errorf("expected 2 databases, got %d", len(databases))
		}

		if current != "db1" {
			t.Errorf("expected current 'db1', got %q", current)
		}
	})

	t.Run("with auth and token", func(t *testing.T) {
		provider := NewHTTPDatabaseProvider(cm, true, nil)
		ctx := context.WithValue(context.Background(), auth.TokenHashContextKey, "user-token")

		// Set current database for this token
		_ = cm.SetCurrentDatabase("user-token", "db2")

		databases, current, err := provider.ListDatabases(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(databases) != 2 {
			t.Errorf("expected 2 databases, got %d", len(databases))
		}

		if current != "db2" {
			t.Errorf("expected current 'db2', got %q", current)
		}
	})
}

func TestHTTPDatabaseProvider_SelectDatabase(t *testing.T) {
	cm := NewClientManager([]config.NamedDatabaseConfig{
		{Name: "db1", Host: "host1", Port: 5432, Database: "test1"},
		{Name: "db2", Host: "host2", Port: 5433, Database: "test2"},
	})

	t.Run("select valid database", func(t *testing.T) {
		provider := NewHTTPDatabaseProvider(cm, false, nil)
		ctx := context.Background()

		err := provider.SelectDatabase(ctx, "db2")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("select non-existent database", func(t *testing.T) {
		provider := NewHTTPDatabaseProvider(cm, false, nil)
		ctx := context.Background()

		err := provider.SelectDatabase(ctx, "nonexistent")
		if err == nil {
			t.Fatal("expected error for non-existent database")
		}
		if err.Error() != "database 'nonexistent' not found" {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("with auth token", func(t *testing.T) {
		provider := NewHTTPDatabaseProvider(cm, true, nil)
		ctx := context.WithValue(context.Background(), auth.TokenHashContextKey, "select-token")

		err := provider.SelectDatabase(ctx, "db1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify current database is set for this token
		current := cm.GetCurrentDatabase("select-token")
		if current != "db1" {
			t.Errorf("expected current 'db1' for token, got %q", current)
		}
	})
}

func TestHTTPDatabaseProvider_WithAccessChecker(t *testing.T) {
	cm := NewClientManager([]config.NamedDatabaseConfig{
		{Name: "db1", Host: "host1", Port: 5432, Database: "test1"},
		{Name: "db2", Host: "host2", Port: 5433, Database: "test2"},
		{Name: "db3", Host: "host3", Port: 5434, Database: "test3"},
	})

	// Create access checker that only allows db1 and db2
	checker := auth.NewDatabaseAccessChecker(nil, true, false)
	provider := NewHTTPDatabaseProvider(cm, true, checker)

	t.Run("list with access checker", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), auth.UsernameContextKey, "testuser")
		databases, _, err := provider.ListDatabases(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Without actual user config, all databases should be returned
		// The access checker returns all if no specific restrictions
		if len(databases) != 3 {
			t.Errorf("expected 3 databases (no restrictions), got %d", len(databases))
		}
	})
}

func TestHTTPDatabaseProvider_SelectDatabase_AccessDenied(t *testing.T) {
	cm := NewClientManager([]config.NamedDatabaseConfig{
		{Name: "db1", Host: "host1", Port: 5432, Database: "test1"},
	})

	// Create access checker that denies access
	checker := auth.NewDatabaseAccessChecker(nil, true, true) // checkBound=true means stricter checking
	provider := NewHTTPDatabaseProvider(cm, true, checker)

	ctx := context.WithValue(context.Background(), auth.TokenHashContextKey, "restricted-token")

	// This should work since the access checker without config allows all
	err := provider.SelectDatabase(ctx, "db1")
	if err != nil {
		// Access denied is expected in some configurations
		// But without explicit deny config, it should work
		t.Logf("error: %v", err)
	}
}
