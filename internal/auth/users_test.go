/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
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

// TestHashPassword tests password hashing
func TestHashPassword(t *testing.T) {
	t.Run("hashes password successfully", func(t *testing.T) {
		password := "testPassword123!"
		hash, err := HashPassword(password)
		if err != nil {
			t.Fatalf("HashPassword failed: %v", err)
		}

		if hash == "" {
			t.Error("Expected non-empty hash")
		}

		if hash == password {
			t.Error("Hash should not be the same as password")
		}
	})

	t.Run("produces different hashes for same password", func(t *testing.T) {
		password := "testPassword123!"
		hash1, err := HashPassword(password)
		if err != nil {
			t.Fatalf("HashPassword failed: %v", err)
		}

		hash2, err := HashPassword(password)
		if err != nil {
			t.Fatalf("HashPassword failed: %v", err)
		}

		// Bcrypt includes a salt, so same password produces different hashes
		if hash1 == hash2 {
			t.Error("Expected different hashes for same password due to salt")
		}
	})
}

// TestVerifyPassword tests password verification
func TestVerifyPassword(t *testing.T) {
	t.Run("verifies correct password", func(t *testing.T) {
		password := "testPassword123!"
		hash, err := HashPassword(password)
		if err != nil {
			t.Fatalf("HashPassword failed: %v", err)
		}

		err = VerifyPassword(password, hash)
		if err != nil {
			t.Errorf("VerifyPassword failed for correct password: %v", err)
		}
	})

	t.Run("rejects incorrect password", func(t *testing.T) {
		password := "testPassword123!"
		hash, err := HashPassword(password)
		if err != nil {
			t.Fatalf("HashPassword failed: %v", err)
		}

		err = VerifyPassword("wrongPassword", hash)
		if err == nil {
			t.Error("Expected error for incorrect password")
		}
	})
}

// TestGenerateSessionToken tests session token generation
func TestGenerateSessionToken(t *testing.T) {
	t.Run("generates unique tokens", func(t *testing.T) {
		token1, err := GenerateSessionToken()
		if err != nil {
			t.Fatalf("GenerateSessionToken failed: %v", err)
		}

		token2, err := GenerateSessionToken()
		if err != nil {
			t.Fatalf("GenerateSessionToken failed: %v", err)
		}

		if token1 == token2 {
			t.Error("Expected different tokens")
		}
	})

	t.Run("generates tokens of sufficient length", func(t *testing.T) {
		token, err := GenerateSessionToken()
		if err != nil {
			t.Fatalf("GenerateSessionToken failed: %v", err)
		}

		// 32 bytes encoded in base64 should be at least 40 characters
		if len(token) < 40 {
			t.Errorf("Token too short: %d characters", len(token))
		}
	})
}

// TestInitializeUserStore tests user store initialization
func TestInitializeUserStore(t *testing.T) {
	store := InitializeUserStore()

	if store == nil {
		t.Fatal("Expected non-nil store")
	}

	if store.Users == nil {
		t.Error("Expected Users map to be initialized")
	}

	if len(store.Users) != 0 {
		t.Errorf("Expected empty store, got %d users", len(store.Users))
	}
}

// TestAddUser tests adding users
func TestAddUser(t *testing.T) {
	t.Run("adds user successfully", func(t *testing.T) {
		store := InitializeUserStore()

		err := store.AddUser("testuser", "password123", "Test User")
		if err != nil {
			t.Fatalf("AddUser failed: %v", err)
		}

		if len(store.Users) != 1 {
			t.Errorf("Expected 1 user, got %d", len(store.Users))
		}

		user, exists := store.Users["testuser"]
		if !exists {
			t.Fatal("User not found in store")
		}

		if user.Username != "testuser" {
			t.Errorf("Expected username 'testuser', got '%s'", user.Username)
		}

		if user.Annotation != "Test User" {
			t.Errorf("Expected annotation 'Test User', got '%s'", user.Annotation)
		}

		if !user.Enabled {
			t.Error("Expected user to be enabled by default")
		}

		if user.LastLogin != nil {
			t.Error("Expected LastLogin to be nil for new user")
		}

		// Verify password hash works
		err = VerifyPassword("password123", user.PasswordHash)
		if err != nil {
			t.Errorf("Password verification failed: %v", err)
		}
	})

	t.Run("rejects duplicate username", func(t *testing.T) {
		store := InitializeUserStore()

		err := store.AddUser("testuser", "password123", "Test User")
		if err != nil {
			t.Fatalf("First AddUser failed: %v", err)
		}

		err = store.AddUser("testuser", "password456", "Another User")
		if err == nil {
			t.Error("Expected error when adding duplicate user")
		}
	})
}

// TestUpdateUser tests updating users
func TestUpdateUser(t *testing.T) {
	t.Run("updates password successfully", func(t *testing.T) {
		store := InitializeUserStore()
		store.AddUser("testuser", "oldpassword", "Test User")

		user := store.Users["testuser"]
		oldHash := user.PasswordHash

		err := store.UpdateUser("testuser", "newpassword", "")
		if err != nil {
			t.Fatalf("UpdateUser failed: %v", err)
		}

		if user.PasswordHash == oldHash {
			t.Error("Expected password hash to change")
		}

		// Verify new password works
		err = VerifyPassword("newpassword", user.PasswordHash)
		if err != nil {
			t.Errorf("New password verification failed: %v", err)
		}

		// Verify old password doesn't work
		err = VerifyPassword("oldpassword", user.PasswordHash)
		if err == nil {
			t.Error("Old password should not work after update")
		}
	})

	t.Run("updates annotation successfully", func(t *testing.T) {
		store := InitializeUserStore()
		store.AddUser("testuser", "password", "Old Annotation")

		err := store.UpdateUser("testuser", "", "New Annotation")
		if err != nil {
			t.Fatalf("UpdateUser failed: %v", err)
		}

		user := store.Users["testuser"]
		if user.Annotation != "New Annotation" {
			t.Errorf("Expected annotation 'New Annotation', got '%s'", user.Annotation)
		}
	})

	t.Run("updates both password and annotation", func(t *testing.T) {
		store := InitializeUserStore()
		store.AddUser("testuser", "oldpassword", "Old Annotation")

		err := store.UpdateUser("testuser", "newpassword", "New Annotation")
		if err != nil {
			t.Fatalf("UpdateUser failed: %v", err)
		}

		user := store.Users["testuser"]

		// Verify new password
		err = VerifyPassword("newpassword", user.PasswordHash)
		if err != nil {
			t.Errorf("New password verification failed: %v", err)
		}

		// Verify new annotation
		if user.Annotation != "New Annotation" {
			t.Errorf("Expected annotation 'New Annotation', got '%s'", user.Annotation)
		}
	})

	t.Run("returns error for non-existent user", func(t *testing.T) {
		store := InitializeUserStore()

		err := store.UpdateUser("nonexistent", "password", "annotation")
		if err == nil {
			t.Error("Expected error when updating non-existent user")
		}
	})
}

// TestRemoveUser tests removing users
func TestRemoveUser(t *testing.T) {
	t.Run("removes user successfully", func(t *testing.T) {
		store := InitializeUserStore()
		store.AddUser("testuser", "password", "Test User")

		err := store.RemoveUser("testuser")
		if err != nil {
			t.Fatalf("RemoveUser failed: %v", err)
		}

		if len(store.Users) != 0 {
			t.Errorf("Expected 0 users, got %d", len(store.Users))
		}

		_, exists := store.Users["testuser"]
		if exists {
			t.Error("User should not exist after removal")
		}
	})

	t.Run("returns error for non-existent user", func(t *testing.T) {
		store := InitializeUserStore()

		err := store.RemoveUser("nonexistent")
		if err == nil {
			t.Error("Expected error when removing non-existent user")
		}
	})
}

// TestEnableDisableUser tests enabling/disabling users
func TestEnableDisableUser(t *testing.T) {
	t.Run("disables user successfully", func(t *testing.T) {
		store := InitializeUserStore()
		store.AddUser("testuser", "password", "Test User")

		err := store.DisableUser("testuser")
		if err != nil {
			t.Fatalf("DisableUser failed: %v", err)
		}

		user := store.Users["testuser"]
		if user.Enabled {
			t.Error("Expected user to be disabled")
		}
	})

	t.Run("enables user successfully", func(t *testing.T) {
		store := InitializeUserStore()
		store.AddUser("testuser", "password", "Test User")
		store.DisableUser("testuser")

		err := store.EnableUser("testuser")
		if err != nil {
			t.Fatalf("EnableUser failed: %v", err)
		}

		user := store.Users["testuser"]
		if !user.Enabled {
			t.Error("Expected user to be enabled")
		}
	})

	t.Run("returns error for non-existent user", func(t *testing.T) {
		store := InitializeUserStore()

		err := store.EnableUser("nonexistent")
		if err == nil {
			t.Error("Expected error when enabling non-existent user")
		}

		err = store.DisableUser("nonexistent")
		if err == nil {
			t.Error("Expected error when disabling non-existent user")
		}
	})
}

// TestAuthenticateUser tests user authentication
func TestAuthenticateUser(t *testing.T) {
	t.Run("authenticates valid credentials", func(t *testing.T) {
		store := InitializeUserStore()
		store.AddUser("testuser", "password123", "Test User")

		token, expiration, err := store.AuthenticateUser("testuser", "password123")
		if err != nil {
			t.Fatalf("AuthenticateUser failed: %v", err)
		}

		if token == "" {
			t.Error("Expected non-empty session token")
		}

		if expiration.IsZero() {
			t.Error("Expected non-zero expiration time")
		}

		// Verify expiration is approximately 24 hours from now
		expectedExpiration := time.Now().Add(24 * time.Hour)
		diff := expiration.Sub(expectedExpiration)
		if diff < -time.Minute || diff > time.Minute {
			t.Errorf("Expiration time not within expected range: %v", diff)
		}

		// Verify user's session info was updated
		user := store.Users["testuser"]
		if user.SessionToken != token {
			t.Error("Session token not stored in user")
		}

		if user.SessionExpires == nil {
			t.Error("Session expiration not stored in user")
		}

		if user.LastLogin == nil {
			t.Error("Last login time not updated")
		}

		// Verify last login is recent
		if time.Since(*user.LastLogin) > time.Minute {
			t.Error("Last login time not recent")
		}
	})

	t.Run("rejects invalid username", func(t *testing.T) {
		store := InitializeUserStore()
		store.AddUser("testuser", "password123", "Test User")

		_, _, err := store.AuthenticateUser("wronguser", "password123")
		if err == nil {
			t.Error("Expected error for invalid username")
		}
	})

	t.Run("rejects invalid password", func(t *testing.T) {
		store := InitializeUserStore()
		store.AddUser("testuser", "password123", "Test User")

		_, _, err := store.AuthenticateUser("testuser", "wrongpassword")
		if err == nil {
			t.Error("Expected error for invalid password")
		}
	})

	t.Run("rejects disabled user", func(t *testing.T) {
		store := InitializeUserStore()
		store.AddUser("testuser", "password123", "Test User")
		store.DisableUser("testuser")

		_, _, err := store.AuthenticateUser("testuser", "password123")
		if err == nil {
			t.Error("Expected error for disabled user")
		}
	})
}

// TestValidateSessionToken tests session token validation
func TestValidateSessionToken(t *testing.T) {
	t.Run("validates valid session token", func(t *testing.T) {
		store := InitializeUserStore()
		store.AddUser("testuser", "password123", "Test User")
		token, _, _ := store.AuthenticateUser("testuser", "password123")

		username, err := store.ValidateSessionToken(token)
		if err != nil {
			t.Fatalf("ValidateSessionToken failed: %v", err)
		}

		if username != "testuser" {
			t.Errorf("Expected username 'testuser', got '%s'", username)
		}
	})

	t.Run("rejects invalid token", func(t *testing.T) {
		store := InitializeUserStore()
		store.AddUser("testuser", "password123", "Test User")

		_, err := store.ValidateSessionToken("invalid-token")
		if err == nil {
			t.Error("Expected error for invalid token")
		}
	})

	t.Run("rejects expired token", func(t *testing.T) {
		store := InitializeUserStore()
		store.AddUser("testuser", "password123", "Test User")
		token, _, _ := store.AuthenticateUser("testuser", "password123")

		// Manually set expiration to past
		user := store.Users["testuser"]
		pastTime := time.Now().Add(-1 * time.Hour)
		user.SessionExpires = &pastTime

		_, err := store.ValidateSessionToken(token)
		if err == nil {
			t.Error("Expected error for expired token")
		}
	})

	t.Run("rejects token for disabled user", func(t *testing.T) {
		store := InitializeUserStore()
		store.AddUser("testuser", "password123", "Test User")
		token, _, _ := store.AuthenticateUser("testuser", "password123")

		store.DisableUser("testuser")

		_, err := store.ValidateSessionToken(token)
		if err == nil {
			t.Error("Expected error for disabled user")
		}
	})
}

// TestListUsers tests listing users
func TestListUsers(t *testing.T) {
	t.Run("lists all users", func(t *testing.T) {
		store := InitializeUserStore()
		store.AddUser("user1", "password1", "First User")
		store.AddUser("user2", "password2", "Second User")
		store.AddUser("user3", "password3", "Third User")

		users := store.ListUsers()
		if len(users) != 3 {
			t.Errorf("Expected 3 users, got %d", len(users))
		}

		// Verify user info is present
		userMap := make(map[string]*UserInfo)
		for _, user := range users {
			userMap[user.Username] = user
		}

		if _, exists := userMap["user1"]; !exists {
			t.Error("user1 not found in list")
		}

		if _, exists := userMap["user2"]; !exists {
			t.Error("user2 not found in list")
		}

		if _, exists := userMap["user3"]; !exists {
			t.Error("user3 not found in list")
		}
	})

	t.Run("returns empty list for empty store", func(t *testing.T) {
		store := InitializeUserStore()

		users := store.ListUsers()
		if len(users) != 0 {
			t.Errorf("Expected 0 users, got %d", len(users))
		}
	})
}

// TestSaveAndLoadUserStore tests file persistence
func TestSaveAndLoadUserStore(t *testing.T) {
	t.Run("saves and loads user store", func(t *testing.T) {
		tempDir := t.TempDir()
		userFile := filepath.Join(tempDir, "users.yaml")

		// Create and save store
		store := InitializeUserStore()
		store.AddUser("user1", "password1", "First User")
		store.AddUser("user2", "password2", "Second User")
		store.DisableUser("user2")

		err := SaveUserStore(userFile, store)
		if err != nil {
			t.Fatalf("SaveUserStore failed: %v", err)
		}

		// Load store
		loadedStore, err := LoadUserStore(userFile)
		if err != nil {
			t.Fatalf("LoadUserStore failed: %v", err)
		}

		// Verify users were loaded
		if len(loadedStore.Users) != 2 {
			t.Errorf("Expected 2 users, got %d", len(loadedStore.Users))
		}

		user1 := loadedStore.Users["user1"]
		if user1 == nil {
			t.Fatal("user1 not found")
		}
		if !user1.Enabled {
			t.Error("user1 should be enabled")
		}

		user2 := loadedStore.Users["user2"]
		if user2 == nil {
			t.Fatal("user2 not found")
		}
		if user2.Enabled {
			t.Error("user2 should be disabled")
		}

		// Verify passwords still work
		err = VerifyPassword("password1", user1.PasswordHash)
		if err != nil {
			t.Error("user1 password verification failed")
		}
	})

	t.Run("file has correct permissions", func(t *testing.T) {
		tempDir := t.TempDir()
		userFile := filepath.Join(tempDir, "users.yaml")

		store := InitializeUserStore()
		store.AddUser("testuser", "password", "Test User")

		err := SaveUserStore(userFile, store)
		if err != nil {
			t.Fatalf("SaveUserStore failed: %v", err)
		}

		info, err := os.Stat(userFile)
		if err != nil {
			t.Fatalf("Failed to stat file: %v", err)
		}

		// Check file permissions (should be 0600)
		mode := info.Mode().Perm()
		expectedMode := os.FileMode(0600)
		if mode != expectedMode {
			t.Errorf("Expected file mode %v, got %v", expectedMode, mode)
		}
	})

	t.Run("returns error for non-existent file", func(t *testing.T) {
		_, err := LoadUserStore("/nonexistent/path/users.yaml")
		if err == nil {
			t.Error("Expected error when loading non-existent file")
		}
	})
}

// TestGetDefaultUserPath tests default path generation
func TestGetDefaultUserPath(t *testing.T) {
	t.Run("returns correct default path", func(t *testing.T) {
		binaryPath := "/path/to/binary/server"
		expectedPath := "/path/to/binary/pgedge-pg-mcp-svr-users.yaml"

		path := GetDefaultUserPath(binaryPath)
		if path != expectedPath {
			t.Errorf("Expected path '%s', got '%s'", expectedPath, path)
		}
	})

	t.Run("handles relative paths", func(t *testing.T) {
		binaryPath := "bin/server"
		path := GetDefaultUserPath(binaryPath)

		if path != "bin/pgedge-pg-mcp-svr-users.yaml" {
			t.Errorf("Expected relative path 'bin/pgedge-pg-mcp-svr-users.yaml', got '%s'", path)
		}
	})
}
