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
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v3"
)

const (
	// DefaultSessionExpiration is the default duration for session tokens
	DefaultSessionExpiration = 24 * time.Hour
)

// User represents a user account with credentials and metadata
type User struct {
	Username       string     `yaml:"username"`        // Unique username
	PasswordHash   string     `yaml:"password_hash"`   // Bcrypt hash of password
	CreatedAt      time.Time  `yaml:"created_at"`      // When the user was created
	LastLogin      *time.Time `yaml:"last_login"`      // Last successful login (null if never logged in)
	Enabled        bool       `yaml:"enabled"`         // Whether the user is enabled
	Annotation     string     `yaml:"annotation"`      // User note/description
	FailedAttempts int        `yaml:"failed_attempts"` // Count of consecutive failed login attempts
	SessionToken   string     `yaml:"-"`               // Current session token (not persisted)
	SessionExpires *time.Time `yaml:"-"`               // Session expiration (not persisted)
}

// UserStore manages user accounts
type UserStore struct {
	mu      sync.RWMutex     // Protects concurrent access to Users
	Users   map[string]*User `yaml:"users"` // key is username
	path    string           // File path for auto-reloading
	watcher *FileWatcher     // File watcher for auto-reloading
}

// HashPassword creates a bcrypt hash of the password
// Uses bcrypt cost of 12 for strong security
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(hash), nil
}

// VerifyPassword checks if the provided password matches the hash
func VerifyPassword(password, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

// GenerateSessionToken creates a new random session token
func GenerateSessionToken() (string, error) {
	// Generate 32 bytes of random data
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate session token: %w", err)
	}

	// Encode as base64 for easy transmission
	token := base64.URLEncoding.EncodeToString(bytes)
	return token, nil
}

// LoadUserStore loads users from a YAML file
func LoadUserStore(path string) (*UserStore, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var store UserStore
	if err := yaml.Unmarshal(data, &store); err != nil {
		return nil, fmt.Errorf("failed to parse user file: %w", err)
	}

	if store.Users == nil {
		store.Users = make(map[string]*User)
	}

	store.path = path // Store path for auto-reloading

	return &store, nil
}

// Reload reloads the user store from disk
func (s *UserStore) Reload() error {
	if s.path == "" {
		return fmt.Errorf("no path set for user store")
	}

	data, err := os.ReadFile(s.path)
	if err != nil {
		return fmt.Errorf("failed to read user file: %w", err)
	}

	var newStore UserStore
	if err := yaml.Unmarshal(data, &newStore); err != nil {
		return fmt.Errorf("failed to parse user file: %w", err)
	}

	if newStore.Users == nil {
		newStore.Users = make(map[string]*User)
	}

	// Update the store with new data (with write lock)
	// But preserve session tokens from the current in-memory users
	s.mu.Lock()
	defer s.mu.Unlock()

	// Save current session tokens
	sessionTokens := make(map[string]struct {
		token   string
		expires *time.Time
	})
	for username, user := range s.Users {
		if user.SessionToken != "" {
			sessionTokens[username] = struct {
				token   string
				expires *time.Time
			}{user.SessionToken, user.SessionExpires}
		}
	}

	// Update users
	s.Users = newStore.Users

	// Restore session tokens for users that still exist
	for username, session := range sessionTokens {
		if user, exists := s.Users[username]; exists {
			user.SessionToken = session.token
			user.SessionExpires = session.expires
		}
	}

	return nil
}

// SaveUserStore saves users to a YAML file
func SaveUserStore(path string, store *UserStore) error {
	data, err := yaml.Marshal(store)
	if err != nil {
		return fmt.Errorf("failed to marshal users: %w", err)
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write with restrictive permissions (owner read/write only)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write user file: %w", err)
	}

	return nil
}

// AddUser adds a new user to the store
func (s *UserStore) AddUser(username, password, annotation string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Users == nil {
		s.Users = make(map[string]*User)
	}

	if _, exists := s.Users[username]; exists {
		return fmt.Errorf("user '%s' already exists", username)
	}

	// Hash the password
	passwordHash, err := HashPassword(password)
	if err != nil {
		return err
	}

	s.Users[username] = &User{
		Username:     username,
		PasswordHash: passwordHash,
		CreatedAt:    time.Now(),
		LastLogin:    nil,
		Enabled:      true,
		Annotation:   annotation,
	}

	return nil
}

// UpdateUser updates an existing user's password and/or annotation
func (s *UserStore) UpdateUser(username, newPassword, newAnnotation string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, exists := s.Users[username]
	if !exists {
		return fmt.Errorf("user '%s' not found", username)
	}

	if newPassword != "" {
		passwordHash, err := HashPassword(newPassword)
		if err != nil {
			return err
		}
		user.PasswordHash = passwordHash
	}

	if newAnnotation != "" {
		user.Annotation = newAnnotation
	}

	return nil
}

// RemoveUser removes a user from the store
func (s *UserStore) RemoveUser(username string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Users == nil {
		return fmt.Errorf("user '%s' not found", username)
	}

	if _, exists := s.Users[username]; !exists {
		return fmt.Errorf("user '%s' not found", username)
	}

	delete(s.Users, username)
	return nil
}

// AuthenticateUser verifies credentials and returns a session token
// Returns the token and expiration time on success
// maxFailedAttempts: if > 0, will disable account after N consecutive failed attempts
func (s *UserStore) AuthenticateUser(username, password string, maxFailedAttempts int) (string, time.Time, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, exists := s.Users[username]
	if !exists {
		return "", time.Time{}, fmt.Errorf("invalid username or password")
	}

	if !user.Enabled {
		return "", time.Time{}, fmt.Errorf("user account is disabled")
	}

	// Verify password
	if err := VerifyPassword(password, user.PasswordHash); err != nil {
		// Increment failed attempts counter
		user.FailedAttempts++

		// Lock account if threshold is reached (only if maxFailedAttempts > 0)
		if maxFailedAttempts > 0 && user.FailedAttempts >= maxFailedAttempts {
			user.Enabled = false
		}

		return "", time.Time{}, fmt.Errorf("invalid username or password")
	}

	// Generate session token
	token, err := GenerateSessionToken()
	if err != nil {
		return "", time.Time{}, err
	}

	// Session valid for the default duration
	expiration := time.Now().Add(DefaultSessionExpiration)

	// Update user's session info (in memory only, not persisted)
	user.SessionToken = token
	user.SessionExpires = &expiration

	// Update last login time (this will be persisted)
	now := time.Now()
	user.LastLogin = &now

	// Reset failed attempts counter on successful login
	user.FailedAttempts = 0

	return token, expiration, nil
}

// ResetFailedAttempts resets the failed login attempts counter for a user
func (s *UserStore) ResetFailedAttempts(username string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, exists := s.Users[username]
	if !exists {
		return fmt.Errorf("user '%s' not found", username)
	}

	user.FailedAttempts = 0
	return nil
}

// GetFailedAttempts returns the number of failed login attempts for a user
func (s *UserStore) GetFailedAttempts(username string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, exists := s.Users[username]
	if !exists {
		return 0, fmt.Errorf("user '%s' not found", username)
	}

	return user.FailedAttempts, nil
}

// ValidateSessionToken checks if a session token is valid for a user
func (s *UserStore) ValidateSessionToken(token string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.Users == nil {
		return "", fmt.Errorf("invalid session token")
	}

	// Find user with this session token
	for username, user := range s.Users {
		if user.SessionToken == token {
			// Check if token has expired
			if user.SessionExpires == nil || user.SessionExpires.Before(time.Now()) {
				return "", fmt.Errorf("session has expired")
			}

			if !user.Enabled {
				return "", fmt.Errorf("user account is disabled")
			}

			return username, nil
		}
	}

	return "", fmt.Errorf("invalid session token")
}

// ListUsers returns all users with their metadata
func (s *UserStore) ListUsers() []*UserInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.Users == nil {
		return []*UserInfo{}
	}

	result := make([]*UserInfo, 0, len(s.Users))

	for _, user := range s.Users {
		result = append(result, &UserInfo{
			Username:   user.Username,
			CreatedAt:  user.CreatedAt,
			LastLogin:  user.LastLogin,
			Enabled:    user.Enabled,
			Annotation: user.Annotation,
		})
	}

	return result
}

// UserInfo is a display-friendly representation of a user
type UserInfo struct {
	Username   string
	CreatedAt  time.Time
	LastLogin  *time.Time
	Enabled    bool
	Annotation string
}

// GetDefaultUserPath returns the default user file path
// Searches /etc/pgedge/postgres-mcp/ first, then binary directory
func GetDefaultUserPath(binaryPath string) string {
	systemPath := "/etc/pgedge/postgres-mcp/pgedge-nla-server-users.yaml"
	if _, err := os.Stat(systemPath); err == nil {
		return systemPath
	}

	dir := filepath.Dir(binaryPath)
	return filepath.Join(dir, "pgedge-nla-server-users.yaml")
}

// InitializeUserStore creates a new empty user store
func InitializeUserStore() *UserStore {
	return &UserStore{
		Users: make(map[string]*User),
	}
}

// EnableUser enables a user account
func (s *UserStore) EnableUser(username string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, exists := s.Users[username]
	if !exists {
		return fmt.Errorf("user '%s' not found", username)
	}
	user.Enabled = true
	return nil
}

// DisableUser disables a user account
func (s *UserStore) DisableUser(username string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, exists := s.Users[username]
	if !exists {
		return fmt.Errorf("user '%s' not found", username)
	}
	user.Enabled = false
	return nil
}

// StartWatching starts watching the user file for changes
func (s *UserStore) StartWatching() error {
	if s.path == "" {
		return fmt.Errorf("no path set for user store")
	}

	watcher, err := NewFileWatcher(s.path, s.Reload)
	if err != nil {
		return err
	}

	s.watcher = watcher
	s.watcher.Start()
	return nil
}

// StopWatching stops watching the user file for changes
func (s *UserStore) StopWatching() {
	if s.watcher != nil {
		s.watcher.Stop()
		s.watcher = nil
	}
}
