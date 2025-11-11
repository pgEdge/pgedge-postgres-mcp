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
	"os"
	"testing"
)

// TestClientManager_GetClient tests that different tokens get different clients
func TestClientManager_GetClient(t *testing.T) {
	// Skip if no database connection available
	if os.Getenv("TEST_PGEDGE_POSTGRES_CONNECTION_STRING") == "" {
		t.Skip("TEST_PGEDGE_POSTGRES_CONNECTION_STRING not set, skipping database test")
	}

	cm := NewClientManager()
	defer cm.CloseAll()

	t.Run("creates new client for new token", func(t *testing.T) {
		client1, err := cm.GetClient("token-hash-1")
		if err != nil {
			t.Fatalf("Failed to get client: %v", err)
		}
		if client1 == nil {
			t.Fatal("Expected client, got nil")
		}

		// Verify client count
		if count := cm.GetClientCount(); count != 1 {
			t.Fatalf("Expected 1 client, got %d", count)
		}
	})

	t.Run("returns same client for same token", func(t *testing.T) {
		client1, err := cm.GetClient("token-hash-2")
		if err != nil {
			t.Fatalf("Failed to get first client: %v", err)
		}

		client2, err := cm.GetClient("token-hash-2")
		if err != nil {
			t.Fatalf("Failed to get second client: %v", err)
		}

		if client1 != client2 {
			t.Fatal("Expected same client instance for same token")
		}

		// Client count should still be 2 (token-hash-1 and token-hash-2)
		if count := cm.GetClientCount(); count != 2 {
			t.Fatalf("Expected 2 clients, got %d", count)
		}
	})

	t.Run("different tokens get different clients", func(t *testing.T) {
		client1, err := cm.GetClient("token-hash-3")
		if err != nil {
			t.Fatalf("Failed to get first client: %v", err)
		}

		client2, err := cm.GetClient("token-hash-4")
		if err != nil {
			t.Fatalf("Failed to get second client: %v", err)
		}

		if client1 == client2 {
			t.Fatal("Expected different client instances for different tokens")
		}

		// Client count should now be 4
		if count := cm.GetClientCount(); count != 4 {
			t.Fatalf("Expected 4 clients, got %d", count)
		}
	})

	t.Run("rejects empty token hash", func(t *testing.T) {
		_, err := cm.GetClient("")
		if err == nil {
			t.Fatal("Expected error for empty token hash")
		}
	})
}

// TestClientManager_RemoveClient tests removing individual clients
func TestClientManager_RemoveClient(t *testing.T) {
	// Skip if no database connection available
	if os.Getenv("TEST_PGEDGE_POSTGRES_CONNECTION_STRING") == "" {
		t.Skip("TEST_PGEDGE_POSTGRES_CONNECTION_STRING not set, skipping database test")
	}

	cm := NewClientManager()
	defer cm.CloseAll()

	// Create some clients
	_, err := cm.GetClient("token-hash-a")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	_, err = cm.GetClient("token-hash-b")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	if count := cm.GetClientCount(); count != 2 {
		t.Fatalf("Expected 2 clients, got %d", count)
	}

	// Remove one client
	err = cm.RemoveClient("token-hash-a")
	if err != nil {
		t.Fatalf("Failed to remove client: %v", err)
	}

	if count := cm.GetClientCount(); count != 1 {
		t.Fatalf("Expected 1 client after removal, got %d", count)
	}

	// Removing non-existent client should not error
	err = cm.RemoveClient("token-hash-nonexistent")
	if err != nil {
		t.Fatalf("Expected no error for non-existent client, got: %v", err)
	}
}

// TestClientManager_RemoveClients tests removing multiple clients at once
func TestClientManager_RemoveClients(t *testing.T) {
	// Skip if no database connection available
	if os.Getenv("TEST_PGEDGE_POSTGRES_CONNECTION_STRING") == "" {
		t.Skip("TEST_PGEDGE_POSTGRES_CONNECTION_STRING not set, skipping database test")
	}

	cm := NewClientManager()
	defer cm.CloseAll()

	// Create several clients
	for i := 1; i <= 5; i++ {
		tokenHash := "token-hash-" + string(rune('0'+i))
		_, err := cm.GetClient(tokenHash)
		if err != nil {
			t.Fatalf("Failed to create client %d: %v", i, err)
		}
	}

	if count := cm.GetClientCount(); count != 5 {
		t.Fatalf("Expected 5 clients, got %d", count)
	}

	// Remove multiple clients
	toRemove := []string{"token-hash-1", "token-hash-2", "token-hash-3"}
	err := cm.RemoveClients(toRemove)
	if err != nil {
		t.Fatalf("Failed to remove clients: %v", err)
	}

	if count := cm.GetClientCount(); count != 2 {
		t.Fatalf("Expected 2 clients after removal, got %d", count)
	}
}

// TestClientManager_CloseAll tests closing all clients
func TestClientManager_CloseAll(t *testing.T) {
	// Skip if no database connection available
	if os.Getenv("TEST_PGEDGE_POSTGRES_CONNECTION_STRING") == "" {
		t.Skip("TEST_PGEDGE_POSTGRES_CONNECTION_STRING not set, skipping database test")
	}

	cm := NewClientManager()

	// Create several clients
	for i := 1; i <= 3; i++ {
		tokenHash := "token-hash-x" + string(rune('0'+i))
		_, err := cm.GetClient(tokenHash)
		if err != nil {
			t.Fatalf("Failed to create client %d: %v", i, err)
		}
	}

	if count := cm.GetClientCount(); count != 3 {
		t.Fatalf("Expected 3 clients, got %d", count)
	}

	// Close all clients
	err := cm.CloseAll()
	if err != nil {
		t.Fatalf("Failed to close all clients: %v", err)
	}

	if count := cm.GetClientCount(); count != 0 {
		t.Fatalf("Expected 0 clients after CloseAll, got %d", count)
	}
}

// TestClientManager_Concurrency tests thread-safety of client manager
func TestClientManager_Concurrency(t *testing.T) {
	// Skip if no database connection available
	if os.Getenv("TEST_PGEDGE_POSTGRES_CONNECTION_STRING") == "" {
		t.Skip("TEST_PGEDGE_POSTGRES_CONNECTION_STRING not set, skipping database test")
	}

	cm := NewClientManager()
	defer cm.CloseAll()

	// Launch multiple goroutines trying to get the same client
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			_, err := cm.GetClient("concurrent-token")
			if err != nil {
				t.Errorf("Failed to get client: %v", err)
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should only have one client despite concurrent requests
	if count := cm.GetClientCount(); count != 1 {
		t.Fatalf("Expected 1 client despite concurrent access, got %d", count)
	}
}
