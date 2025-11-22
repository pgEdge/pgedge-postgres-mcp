/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package crypto

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateKey(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	if key == nil {
		t.Fatal("Expected non-nil key")
	}

	if len(key.key) != KeySize {
		t.Errorf("Expected key size %d, got %d", KeySize, len(key.key))
	}

	// Generate another key and verify they're different
	key2, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	if string(key.key) == string(key2.key) {
		t.Error("Expected different keys, got identical keys")
	}
}

func TestEncryptDecrypt(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	testCases := []struct {
		name      string
		plaintext string
	}{
		{"simple password", "mypassword123"},
		{"complex password", "P@ssw0rd!@#$%^&*()"},
		{"unicode password", "пароль密码"},
		{"empty string", ""},
		{"long password", "this is a very long password with many characters to test larger plaintexts"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Encrypt
			ciphertext, err := key.Encrypt(tc.plaintext)
			if err != nil {
				t.Fatalf("Encrypt failed: %v", err)
			}

			if tc.plaintext == "" {
				if ciphertext != "" {
					t.Error("Expected empty ciphertext for empty plaintext")
				}
				return
			}

			if ciphertext == "" {
				t.Error("Expected non-empty ciphertext")
			}

			// Ciphertext should be different from plaintext
			if ciphertext == tc.plaintext {
				t.Error("Ciphertext should not match plaintext")
			}

			// Decrypt
			decrypted, err := key.Decrypt(ciphertext)
			if err != nil {
				t.Fatalf("Decrypt failed: %v", err)
			}

			if decrypted != tc.plaintext {
				t.Errorf("Decryption mismatch: expected %q, got %q", tc.plaintext, decrypted)
			}
		})
	}
}

func TestEncryptionNonDeterministic(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	plaintext := "test password"

	// Encrypt the same plaintext twice
	ciphertext1, err := key.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	ciphertext2, err := key.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Ciphertexts should be different due to random nonces
	if ciphertext1 == ciphertext2 {
		t.Error("Expected different ciphertexts for same plaintext (nonce should be random)")
	}

	// But both should decrypt to the same plaintext
	decrypted1, err := key.Decrypt(ciphertext1)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	decrypted2, err := key.Decrypt(ciphertext2)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if decrypted1 != plaintext || decrypted2 != plaintext {
		t.Error("Both ciphertexts should decrypt to the same plaintext")
	}
}

func TestDecryptWithWrongKey(t *testing.T) {
	key1, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	key2, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	plaintext := "secret password"

	// Encrypt with key1
	ciphertext, err := key1.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Try to decrypt with key2 (should fail)
	_, err = key2.Decrypt(ciphertext)
	if err == nil {
		t.Error("Expected decryption to fail with wrong key")
	}
}

func TestSaveAndLoadKey(t *testing.T) {
	// Create temporary directory for test
	tmpDir, err := os.MkdirTemp("", "pgedge-crypto-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	keyPath := filepath.Join(tmpDir, "test.key")

	// Generate and save key
	originalKey, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	if err := originalKey.SaveToFile(keyPath); err != nil {
		t.Fatalf("SaveToFile failed: %v", err)
	}

	// Verify file exists and has correct permissions
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("Failed to stat key file: %v", err)
	}

	if info.Mode().Perm() != 0600 {
		t.Errorf("Expected file permissions 0600, got %o", info.Mode().Perm())
	}

	// Load key from file
	loadedKey, err := LoadKeyFromFile(keyPath)
	if err != nil {
		t.Fatalf("LoadKeyFromFile failed: %v", err)
	}

	// Verify keys are identical
	if string(originalKey.key) != string(loadedKey.key) {
		t.Error("Loaded key does not match original key")
	}

	// Test encryption/decryption with loaded key
	plaintext := "test password"
	ciphertext, err := originalKey.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	decrypted, err := loadedKey.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt with loaded key failed: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("Expected %q, got %q", plaintext, decrypted)
	}
}

func TestLoadKeyFromInvalidFile(t *testing.T) {
	testCases := []struct {
		name     string
		content  string
		expected string
	}{
		{"nonexistent file", "", "failed to read key file"},
		{"invalid base64", "not-valid-base64!", "failed to decode key"},
		{"wrong size", "YWJjZGVm", "invalid key size"}, // "abcdef" in base64, too short
	}

	tmpDir, err := os.MkdirTemp("", "pgedge-crypto-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			keyPath := filepath.Join(tmpDir, tc.name+".key")

			if tc.content != "" {
				if err := os.WriteFile(keyPath, []byte(tc.content), 0600); err != nil {
					t.Fatalf("Failed to write test file: %v", err)
				}
			}

			_, err := LoadKeyFromFile(keyPath)
			if err == nil {
				t.Error("Expected error, got nil")
			}
		})
	}
}

func TestLoadKeyWithInsecurePermissions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pgedge-crypto-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	keyPath := filepath.Join(tmpDir, "insecure.key")

	// Generate a valid key file
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	// Save with correct permissions first
	if err := key.SaveToFile(keyPath); err != nil {
		t.Fatalf("SaveToFile failed: %v", err)
	}

	// Test various insecure permission modes
	insecurePermissions := []os.FileMode{
		0644, // world-readable
		0666, // world-readable and writable
		0640, // group-readable
		0660, // group-readable and writable
		0604, // world-readable
	}

	for _, perm := range insecurePermissions {
		t.Run(fmt.Sprintf("mode_%04o", perm), func(t *testing.T) {
			// Change to insecure permissions
			if err := os.Chmod(keyPath, perm); err != nil {
				t.Fatalf("Failed to change permissions: %v", err)
			}

			// Try to load - should fail
			_, err := LoadKeyFromFile(keyPath)
			if err == nil {
				t.Errorf("Expected error for permissions %04o, got nil", perm)
			}

			// Verify error message mentions permissions
			if err != nil && !strings.Contains(err.Error(), "insecure permissions") {
				t.Errorf("Expected 'insecure permissions' error, got: %v", err)
			}
		})
	}
}

func TestDecryptInvalidCiphertext(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	testCases := []struct {
		name       string
		ciphertext string
	}{
		{"invalid base64", "not-valid-base64!"},
		{"too short", "YWJj"}, // "abc" in base64, too short for nonce
		{"corrupted", "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXo="},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := key.Decrypt(tc.ciphertext)
			if err == nil {
				t.Error("Expected error for invalid ciphertext, got nil")
			}
		})
	}
}
