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
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGenerateToken(t *testing.T) {
	t.Run("generates unique tokens", func(t *testing.T) {
		token1, err := GenerateToken()
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}
		if token1 == "" {
			t.Fatal("Generated token is empty")
		}

		token2, err := GenerateToken()
		if err != nil {
			t.Fatalf("Failed to generate second token: %v", err)
		}
		if token1 == token2 {
			t.Fatal("Generated tokens are not unique")
		}
	})

	t.Run("generates tokens of correct length", func(t *testing.T) {
		token, err := GenerateToken()
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}
		// Base64 encoding of 32 bytes is 44 characters (with padding)
		if len(token) < 40 {
			t.Fatalf("Token too short: got %d characters, expected at least 40", len(token))
		}
	})
}

func TestHashToken(t *testing.T) {
	t.Run("produces consistent hashes", func(t *testing.T) {
		token := "test-token-123"
		hash1 := HashToken(token)
		hash2 := HashToken(token)
		if hash1 != hash2 {
			t.Fatal("Same token produced different hashes")
		}
	})

	t.Run("produces different hashes for different tokens", func(t *testing.T) {
		token1 := "test-token-1"
		token2 := "test-token-2"
		hash1 := HashToken(token1)
		hash2 := HashToken(token2)
		if hash1 == hash2 {
			t.Fatal("Different tokens produced same hash")
		}
	})

	t.Run("produces SHA256 length hash", func(t *testing.T) {
		token := "test-token"
		hash := HashToken(token)
		// SHA256 produces 64 hex characters
		if len(hash) != 64 {
			t.Fatalf("Hash length incorrect: got %d, expected 64", len(hash))
		}
	})
}

func TestInitializeTokenStore(t *testing.T) {
	store := InitializeTokenStore()
	if store == nil {
		t.Fatal("InitializeTokenStore returned nil")
	}
	if store.Tokens == nil {
		t.Fatal("Token map is nil")
	}
	if len(store.Tokens) != 0 {
		t.Fatal("New token store should be empty")
	}
}

func TestAddToken(t *testing.T) {
	t.Run("adds token successfully", func(t *testing.T) {
		store := InitializeTokenStore()
		tokenID := "token-123"
		hash := "test-hash"
		note := "Test token"
		expiry := time.Now().Add(24 * time.Hour)

		store.AddToken(tokenID, hash, note, &expiry)

		if len(store.Tokens) != 1 {
			t.Fatalf("Expected 1 token, got %d", len(store.Tokens))
		}

		token, exists := store.Tokens[tokenID]
		if !exists {
			t.Fatal("Token not found in store")
		}
		if token.Hash != hash {
			t.Errorf("Hash mismatch: got %s, expected %s", token.Hash, hash)
		}
		if token.Annotation != note {
			t.Errorf("Annotation mismatch: got %s, expected %s", token.Annotation, note)
		}
		if token.ExpiresAt == nil {
			t.Fatal("ExpiresAt is nil")
		}
		if !token.ExpiresAt.Equal(expiry) {
			t.Errorf("ExpiresAt mismatch: got %v, expected %v", token.ExpiresAt, expiry)
		}
	})

	t.Run("adds token without expiry", func(t *testing.T) {
		store := InitializeTokenStore()
		tokenID := "token-123"
		hash := "test-hash"
		note := "Test token"

		store.AddToken(tokenID, hash, note, nil)

		token, exists := store.Tokens[tokenID]
		if !exists {
			t.Fatal("Token not found in store")
		}
		if token.ExpiresAt != nil {
			t.Fatal("ExpiresAt should be nil for never-expiring token")
		}
	})
}

func TestRemoveToken(t *testing.T) {
	t.Run("removes token by ID", func(t *testing.T) {
		store := InitializeTokenStore()
		tokenID := "token-123"
		store.AddToken(tokenID, "hash", "note", nil)

		removed, err := store.RemoveToken(tokenID)
		if err != nil {
			t.Fatalf("Failed to remove token: %v", err)
		}
		if !removed {
			t.Fatal("Token was not removed")
		}

		if len(store.Tokens) != 0 {
			t.Fatal("Token was not removed from store")
		}
	})

	t.Run("removes token by hash prefix", func(t *testing.T) {
		store := InitializeTokenStore()
		// Use a properly-sized hash (minimum 12 characters for display)
		hash := "abcdef1234567890abcdef1234567890"
		store.AddToken("token-123", hash, "note", nil)

		// Use at least 8 characters for prefix matching
		removed, err := store.RemoveToken("abcdef12")
		if err != nil {
			t.Fatalf("Failed to remove token by prefix: %v", err)
		}
		if !removed {
			t.Fatal("Token was not removed")
		}

		if len(store.Tokens) != 0 {
			t.Fatal("Token was not removed from store")
		}
	})

	t.Run("returns false for non-existent token", func(t *testing.T) {
		store := InitializeTokenStore()
		removed, err := store.RemoveToken("nonexistent")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if removed {
			t.Fatal("Should not have removed anything")
		}
	})

	t.Run("prefix too short returns false", func(t *testing.T) {
		store := InitializeTokenStore()
		// Use properly-sized hashes
		store.AddToken("token-1", "abc1234567890123456789012345678901234567890123456789012345678901", "note1", nil)
		store.AddToken("token-2", "abc4567890123456789012345678901234567890123456789012345678901234", "note2", nil)

		// Prefix less than 8 characters should return false
		removed, err := store.RemoveToken("abc")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if removed {
			t.Fatal("Should not have removed anything with short prefix")
		}
	})
}

func TestValidateToken(t *testing.T) {
	t.Run("validates valid token", func(t *testing.T) {
		store := InitializeTokenStore()
		token := "test-token"
		hash := HashToken(token)
		expiry := time.Now().Add(24 * time.Hour)
		store.AddToken("token-123", hash, "note", &expiry)

		valid, err := store.ValidateToken(token)
		if err != nil {
			t.Fatalf("Validation failed: %v", err)
		}
		if !valid {
			t.Fatal("Token should be valid")
		}
	})

	t.Run("validates token without expiry", func(t *testing.T) {
		store := InitializeTokenStore()
		token := "test-token"
		hash := HashToken(token)
		store.AddToken("token-123", hash, "note", nil)

		valid, err := store.ValidateToken(token)
		if err != nil {
			t.Fatalf("Validation failed: %v", err)
		}
		if !valid {
			t.Fatal("Token should be valid")
		}
	})

	t.Run("rejects expired token", func(t *testing.T) {
		store := InitializeTokenStore()
		token := "test-token"
		hash := HashToken(token)
		expiry := time.Now().Add(-1 * time.Hour) // Expired 1 hour ago
		store.AddToken("token-123", hash, "note", &expiry)

		valid, err := store.ValidateToken(token)
		if err == nil {
			t.Fatal("Expected error for expired token")
		}
		if valid {
			t.Fatal("Expired token should not be valid")
		}
	})

	t.Run("rejects invalid token", func(t *testing.T) {
		store := InitializeTokenStore()
		store.AddToken("token-123", HashToken("correct-token"), "note", nil)

		valid, err := store.ValidateToken("wrong-token")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if valid {
			t.Fatal("Invalid token should not be valid")
		}
	})

	t.Run("handles empty token store", func(t *testing.T) {
		store := InitializeTokenStore()

		valid, err := store.ValidateToken("any-token")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if valid {
			t.Fatal("Token should not be valid in empty store")
		}
	})
}

func TestCleanupExpiredTokens(t *testing.T) {
	t.Run("removes expired tokens", func(t *testing.T) {
		store := InitializeTokenStore()
		expiredTime := time.Now().Add(-1 * time.Hour)
		validTime := time.Now().Add(1 * time.Hour)

		store.AddToken("expired-1", "hash1", "note1", &expiredTime)
		store.AddToken("expired-2", "hash2", "note2", &expiredTime)
		store.AddToken("valid-1", "hash3", "note3", &validTime)
		store.AddToken("valid-2", "hash4", "note4", nil) // Never expires

		removed, hashes := store.CleanupExpiredTokens()
		if removed != 2 {
			t.Fatalf("Expected 2 tokens removed, got %d", removed)
		}
		if len(hashes) != 2 {
			t.Fatalf("Expected 2 hashes returned, got %d", len(hashes))
		}
		if len(store.Tokens) != 2 {
			t.Fatalf("Expected 2 tokens remaining, got %d", len(store.Tokens))
		}
		// Verify the correct hashes were returned
		expectedHashes := map[string]bool{"hash1": true, "hash2": true}
		for _, hash := range hashes {
			if !expectedHashes[hash] {
				t.Fatalf("Unexpected hash in removed list: %s", hash)
			}
		}
	})

	t.Run("does nothing when no expired tokens", func(t *testing.T) {
		store := InitializeTokenStore()
		validTime := time.Now().Add(1 * time.Hour)
		store.AddToken("valid-1", "hash1", "note1", &validTime)
		store.AddToken("valid-2", "hash2", "note2", nil)

		removed, hashes := store.CleanupExpiredTokens()
		if removed != 0 {
			t.Fatalf("Expected 0 tokens removed, got %d", removed)
		}
		if len(hashes) != 0 {
			t.Fatalf("Expected 0 hashes returned, got %d", len(hashes))
		}
		if len(store.Tokens) != 2 {
			t.Fatalf("Expected 2 tokens remaining, got %d", len(store.Tokens))
		}
	})
}

func TestListTokens(t *testing.T) {
	t.Run("lists all tokens with metadata", func(t *testing.T) {
		store := InitializeTokenStore()
		expiry := time.Now().Add(24 * time.Hour)
		// Use properly-sized hashes (SHA256 produces 64 chars)
		hash1 := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
		hash2 := "123456789012345678901234567890123456789012345678901234567890abcd"
		store.AddToken("token-1", hash1, "First token", &expiry)
		store.AddToken("token-2", hash2, "Second token", nil)

		tokens := store.ListTokens()
		if len(tokens) != 2 {
			t.Fatalf("Expected 2 tokens, got %d", len(tokens))
		}

		// Verify hash prefix is correctly truncated
		for _, token := range tokens {
			if len(token.HashPrefix) != 12 {
				t.Errorf("Expected hash prefix length 12, got %d", len(token.HashPrefix))
			}
		}
	})

	t.Run("returns empty list for empty store", func(t *testing.T) {
		store := InitializeTokenStore()
		tokens := store.ListTokens()
		if len(tokens) != 0 {
			t.Fatalf("Expected empty list, got %d tokens", len(tokens))
		}
	})
}

func TestSaveAndLoadTokenStore(t *testing.T) {
	t.Run("saves and loads token store", func(t *testing.T) {
		tmpDir := t.TempDir()
		tokenFile := filepath.Join(tmpDir, "tokens.yaml")

		// Create and save token store
		store := InitializeTokenStore()
		expiry := time.Now().Add(24 * time.Hour)
		hash1 := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
		hash2 := "123456789012345678901234567890123456789012345678901234567890abcd"
		store.AddToken("token-123", hash1, "Test token", &expiry)
		store.AddToken("token-456", hash2, "Never expires", nil)

		err := SaveTokenStore(tokenFile, store)
		if err != nil {
			t.Fatalf("Failed to save token store: %v", err)
		}

		// Load token store
		loadedStore, err := LoadTokenStore(tokenFile)
		if err != nil {
			t.Fatalf("Failed to load token store: %v", err)
		}

		// Verify tokens
		if len(loadedStore.Tokens) != 2 {
			t.Fatalf("Expected 2 tokens, got %d", len(loadedStore.Tokens))
		}

		token1, exists := loadedStore.Tokens["token-123"]
		if !exists {
			t.Fatal("Token 'token-123' not found")
		}
		if token1.Hash != hash1 {
			t.Errorf("Hash mismatch: got %s, expected %s", token1.Hash, hash1)
		}
		if token1.Annotation != "Test token" {
			t.Errorf("Annotation mismatch: got %s, expected 'Test token'", token1.Annotation)
		}
		if token1.ExpiresAt == nil {
			t.Fatal("ExpiresAt should not be nil")
		}

		token2, exists := loadedStore.Tokens["token-456"]
		if !exists {
			t.Fatal("Token 'token-456' not found")
		}
		if token2.ExpiresAt != nil {
			t.Fatal("ExpiresAt should be nil for never-expiring token")
		}
	})

	t.Run("file has correct permissions", func(t *testing.T) {
		tmpDir := t.TempDir()
		tokenFile := filepath.Join(tmpDir, "tokens.yaml")

		store := InitializeTokenStore()
		hash := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
		store.AddToken("token-123", hash, "note", nil)

		err := SaveTokenStore(tokenFile, store)
		if err != nil {
			t.Fatalf("Failed to save token store: %v", err)
		}

		info, err := os.Stat(tokenFile)
		if err != nil {
			t.Fatalf("Failed to stat token file: %v", err)
		}

		mode := info.Mode()
		expectedMode := os.FileMode(0600)
		if mode != expectedMode {
			t.Errorf("File permissions incorrect: got %o, expected %o", mode, expectedMode)
		}
	})

	t.Run("returns error for non-existent file", func(t *testing.T) {
		_, err := LoadTokenStore("/nonexistent/path/tokens.yaml")
		if err == nil {
			t.Fatal("Expected error for non-existent file")
		}
	})
}

func TestGetDefaultTokenPath(t *testing.T) {
	t.Run("returns correct default path", func(t *testing.T) {
		binaryPath := "/usr/local/bin/pgedge-postgres-mcp"
		expectedPath := "/usr/local/bin/pgedge-postgres-mcp-server-tokens.yaml"

		path := GetDefaultTokenPath(binaryPath)
		if path != expectedPath {
			t.Errorf("Default path incorrect: got %s, expected %s", path, expectedPath)
		}
	})

	t.Run("handles relative paths", func(t *testing.T) {
		binaryPath := "./bin/pgedge-postgres-mcp"
		path := GetDefaultTokenPath(binaryPath)

		if filepath.Base(path) != "pgedge-postgres-mcp-server-tokens.yaml" {
			t.Errorf("Expected filename 'pgedge-postgres-mcp-server-tokens.yaml', got %s", filepath.Base(path))
		}
	})
}

func TestTokenExpirationEdgeCases(t *testing.T) {
	t.Run("token expiring in 1 second", func(t *testing.T) {
		store := InitializeTokenStore()
		token := "test-token"
		hash := HashToken(token)
		expiry := time.Now().Add(1 * time.Second)
		store.AddToken("token-123", hash, "note", &expiry)

		// Should be valid now
		valid, err := store.ValidateToken(token)
		if err != nil || !valid {
			t.Fatal("Token should be valid before expiry")
		}

		// Wait for expiration
		time.Sleep(2 * time.Second)

		// Should be invalid now
		valid, err = store.ValidateToken(token)
		if err == nil {
			t.Fatal("Expected error for expired token")
		}
		if valid {
			t.Fatal("Token should be invalid after expiry")
		}
	})

	t.Run("token with exact current time expiry", func(t *testing.T) {
		store := InitializeTokenStore()
		token := "test-token"
		hash := HashToken(token)
		expiry := time.Now()
		store.AddToken("token-123", hash, "note", &expiry)

		// Token expiring at current time should be treated as expired
		valid, err := store.ValidateToken(token)
		if err == nil {
			t.Fatal("Expected error for token expiring at current time")
		}
		if valid {
			t.Fatal("Token expiring at current time should be invalid")
		}
	})
}
