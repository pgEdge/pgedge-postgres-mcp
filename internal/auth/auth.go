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
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Token represents an API token with metadata
type Token struct {
	Hash       string     `yaml:"hash"`       // SHA256 hash of the token
	ExpiresAt  *time.Time `yaml:"expires_at"` // Expiry date (null for indefinite)
	Annotation string     `yaml:"annotation"` // User note/description
	CreatedAt  time.Time  `yaml:"created_at"` // When the token was created
}

// TokenStore manages API tokens
type TokenStore struct {
	mu      sync.RWMutex      // Protects concurrent access to Tokens
	Tokens  map[string]*Token `yaml:"tokens"` // key is a unique identifier
	path    string            // File path for auto-reloading
	watcher *FileWatcher      // File watcher for auto-reloading
}

// GenerateToken creates a new random API token
func GenerateToken() (string, error) {
	// Generate 32 bytes of random data
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random token: %w", err)
	}

	// Encode as base64 for easy copying
	token := base64.URLEncoding.EncodeToString(bytes)
	return token, nil
}

// HashToken creates a SHA256 hash of the token
func HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// LoadTokenStore loads tokens from a YAML file
func LoadTokenStore(path string) (*TokenStore, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var store TokenStore
	if err := yaml.Unmarshal(data, &store); err != nil {
		return nil, fmt.Errorf("failed to parse token file: %w", err)
	}

	if store.Tokens == nil {
		store.Tokens = make(map[string]*Token)
	}

	store.path = path // Store path for auto-reloading

	return &store, nil
}

// Reload reloads the token store from disk
func (s *TokenStore) Reload() error {
	if s.path == "" {
		return fmt.Errorf("no path set for token store")
	}

	data, err := os.ReadFile(s.path)
	if err != nil {
		return fmt.Errorf("failed to read token file: %w", err)
	}

	var newStore TokenStore
	if err := yaml.Unmarshal(data, &newStore); err != nil {
		return fmt.Errorf("failed to parse token file: %w", err)
	}

	if newStore.Tokens == nil {
		newStore.Tokens = make(map[string]*Token)
	}

	// Update the store with new data (with write lock)
	s.mu.Lock()
	s.Tokens = newStore.Tokens
	s.mu.Unlock()

	return nil
}

// SaveTokenStore saves tokens to a YAML file
func SaveTokenStore(path string, store *TokenStore) error {
	data, err := yaml.Marshal(store)
	if err != nil {
		return fmt.Errorf("failed to marshal tokens: %w", err)
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write with restrictive permissions (owner read/write only)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write token file: %w", err)
	}

	return nil
}

// AddToken adds a new token to the store
func (s *TokenStore) AddToken(tokenID, hash, annotation string, expiresAt *time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Tokens == nil {
		s.Tokens = make(map[string]*Token)
	}

	if _, exists := s.Tokens[tokenID]; exists {
		return fmt.Errorf("token with ID '%s' already exists", tokenID)
	}

	s.Tokens[tokenID] = &Token{
		Hash:       hash,
		ExpiresAt:  expiresAt,
		Annotation: annotation,
		CreatedAt:  time.Now(),
	}

	return nil
}

// RemoveToken removes a token from the store by ID or hash prefix
func (s *TokenStore) RemoveToken(identifier string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Tokens == nil {
		return false, nil
	}

	// Try exact ID match first
	if _, exists := s.Tokens[identifier]; exists {
		delete(s.Tokens, identifier)
		return true, nil
	}

	// Try hash prefix match
	for id, token := range s.Tokens {
		if len(identifier) >= 8 && token.Hash[:len(identifier)] == identifier {
			delete(s.Tokens, id)
			return true, nil
		}
	}

	return false, nil
}

// ValidateToken checks if a token is valid (exists and not expired)
func (s *TokenStore) ValidateToken(token string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.Tokens == nil {
		return false, nil
	}

	hash := HashToken(token)
	now := time.Now()

	for _, storedToken := range s.Tokens {
		if storedToken.Hash == hash {
			// Check if expired
			if storedToken.ExpiresAt != nil && storedToken.ExpiresAt.Before(now) {
				return false, fmt.Errorf("token has expired")
			}
			return true, nil
		}
	}

	return false, nil
}

// ListTokens returns all tokens with their metadata
func (s *TokenStore) ListTokens() []*TokenInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.Tokens == nil {
		return []*TokenInfo{}
	}

	result := make([]*TokenInfo, 0, len(s.Tokens))
	now := time.Now()

	for id, token := range s.Tokens {
		expired := token.ExpiresAt != nil && token.ExpiresAt.Before(now)
		result = append(result, &TokenInfo{
			ID:         id,
			HashPrefix: token.Hash[:12], // Show first 12 chars
			ExpiresAt:  token.ExpiresAt,
			Annotation: token.Annotation,
			CreatedAt:  token.CreatedAt,
			Expired:    expired,
		})
	}

	return result
}

// TokenInfo is a display-friendly representation of a token
type TokenInfo struct {
	ID         string
	HashPrefix string
	ExpiresAt  *time.Time
	Annotation string
	CreatedAt  time.Time
	Expired    bool
}

// GetDefaultTokenPath returns the default token file path
// Searches /etc/pgedge/postgres-mcp/ first, then binary directory
func GetDefaultTokenPath(binaryPath string) string {
	systemPath := "/etc/pgedge/postgres-mcp/pgedge-nla-server-tokens.yaml"
	if _, err := os.Stat(systemPath); err == nil {
		return systemPath
	}

	dir := filepath.Dir(binaryPath)
	return filepath.Join(dir, "pgedge-nla-server-tokens.yaml")
}

// InitializeTokenStore creates a new empty token store
func InitializeTokenStore() *TokenStore {
	return &TokenStore{
		Tokens: make(map[string]*Token),
	}
}

// CleanupExpiredTokens removes expired tokens from the store
// Returns the number of tokens removed and their hashes (for connection cleanup)
func (s *TokenStore) CleanupExpiredTokens() (int, []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Tokens == nil {
		return 0, nil
	}

	var removedHashes []string
	now := time.Now()

	for id, token := range s.Tokens {
		if token.ExpiresAt != nil && token.ExpiresAt.Before(now) {
			removedHashes = append(removedHashes, token.Hash)
			delete(s.Tokens, id)
		}
	}

	return len(removedHashes), removedHashes
}

// StartWatching starts watching the token file for changes
func (s *TokenStore) StartWatching() error {
	if s.path == "" {
		return fmt.Errorf("no path set for token store")
	}

	watcher, err := NewFileWatcher(s.path, s.Reload)
	if err != nil {
		return err
	}

	s.watcher = watcher
	s.watcher.Start()
	return nil
}

// StopWatching stops watching the token file for changes
func (s *TokenStore) StopWatching() {
	if s.watcher != nil {
		s.watcher.Stop()
		s.watcher = nil
	}
}
