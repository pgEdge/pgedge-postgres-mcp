/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"log"
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
}

// UserStore manages user accounts
type UserStore struct {
	mu      sync.RWMutex     // Protects concurrent access to Users
	Users   map[string]*User `yaml:"users"` // key is username
	path    string           // File path for auto-reloading
	watcher *FileWatcher     // File watcher for auto-reloading

	// onRevoked, when set, is called with the username of a user whose
	// access has just been withdrawn, whether by being disabled,
	// deleted, locked out after too many failed attempts, or removed or
	// disabled in the user file whilst the server was running. main
	// wires it to the OAuth server, so that credentials issued from
	// other credential kinds are revoked with the account rather than
	// remaining usable until they expire. It is always called with no
	// lock held.
	onRevoked func(username string)

	// sessions holds password-login sessions. It is a memory store,
	// created on first use, unless SetSessionStore installed another.
	// Guarded by mu.
	sessions SessionStore

	// fingerprints, when set, records each user's password fingerprint
	// for the start-up check on OAuth state; see FingerprintStore.
	// Guarded by mu. fpMu serialises RecordFingerprints.
	fingerprints FingerprintStore
	fpMu         sync.Mutex
}

// SetSessionStore installs the store password-login sessions live in.
// Call it before the store is used; a UserStore with none set keeps
// sessions in memory.
func (s *UserStore) SetSessionStore(ss SessionStore) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sessions = ss
}

// sessionStore returns the session store, creating the in-memory default
// on first use.
func (s *UserStore) sessionStore() SessionStore {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.sessions == nil {
		s.sessions = NewMemorySessionStore()
	}
	return s.sessions
}

// deleteSessions removes every session of the named users. Failures are
// logged rather than returned, since the callers are revocation paths
// that must carry on. Callers must not hold s.mu.
func (s *UserStore) deleteSessions(usernames ...string) {
	ss := s.sessionStore()
	for _, username := range usernames {
		if username == "" {
			continue
		}
		if err := ss.DeleteUser(username); err != nil {
			log.Printf("[AUTH] Failed to delete sessions for %q: %v", username, err)
		}
	}
}

// StartSessionSweeper removes expired sessions every interval until the
// returned function is called, which is safe to call more than once.
func (s *UserStore) StartSessionSweeper(interval time.Duration) (stop func()) {
	if interval <= 0 {
		return func() {}
	}
	done := make(chan struct{})
	var once sync.Once
	ss := s.sessionStore()
	sweep := func(now time.Time) {
		// Take the list of enabled users under the lock, then release
		// it before the database call.
		s.mu.RLock()
		active := make([]string, 0, len(s.Users))
		for name, u := range s.Users {
			if u.Enabled {
				active = append(active, name)
			}
		}
		s.mu.RUnlock()

		if err := ss.Sweep(now, active); err != nil {
			log.Printf("[AUTH] Failed to sweep sessions: %v", err)
		}
	}

	// Sweep once before returning, so sessions of users disabled or
	// deleted whilst the server was stopped are gone before it serves
	// anything and cannot return when the user is enabled again.
	sweep(time.Now())

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case now := <-ticker.C:
				sweep(now)
			}
		}
	}()
	return func() { once.Do(func() { close(done) }) }
}

// SetRevocationHook installs the callback described on
// UserStore.onRevoked. It takes the store's lock, so it is safe to call
// after the file watcher has been started, which matters because the
// OAuth server the hook calls into is built later than the user store.
func (s *UserStore) SetRevocationHook(fn func(username string)) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.onRevoked = fn
}

// notifyRevoked reports each named user to the revocation hook, if one
// is installed. Callers must not hold s.mu.
func (s *UserStore) notifyRevoked(usernames ...string) {
	if len(usernames) == 0 {
		return
	}
	// Password-login sessions go first, so that they are gone before any
	// external hook runs.
	s.deleteSessions(usernames...)
	s.mu.RLock()
	hook := s.onRevoked
	s.mu.RUnlock()
	if hook == nil {
		return
	}
	for _, username := range usernames {
		if username != "" {
			hook(username)
		}
	}
}

// IsActive reports whether username names a user that exists and is
// enabled, which is the condition every credential kind requires before
// it will honour a token standing in that user's name.
func (s *UserStore) IsActive(username string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, exists := s.Users[username]
	return exists && user.Enabled
}

// BcryptCost is the cost factor used for every password hash this
// package generates. It is exported so that callers needing a
// comparison of the same duration, such as the dummy comparison the
// OAuth authenticator runs for an unknown user, can match it.
const BcryptCost = 12

// CanVerifyPassword reports whether AuthenticateUser would reach the
// bcrypt comparison for username, which it does only for a user that
// exists and is enabled. A caller that has just been told the
// credentials were wrong can use it to tell an actual password
// mismatch, which cost a bcrypt comparison, from an unknown or disabled
// user, which cost nothing and would otherwise answer measurably
// faster.
func (s *UserStore) CanVerifyPassword(username string) bool {
	return s.IsActive(username)
}

// HashPassword creates a bcrypt hash of the password
// Uses bcrypt cost of 12 for strong security
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
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

	// Update the store with new data (with write lock). The lock is
	// released explicitly at the end, rather than by defer, because the
	// revocation hook must not run under it. Sessions live in their own
	// store and are checked against the new users at validation.
	s.mu.Lock()

	// Note which users the reload has withdrawn access from, by removing
	// them from the file or disabling them there, and which it has given
	// a new password, so their credentials can be revoked once the lock
	// is released. A password change ends OAuth grants as well as
	// sessions, as it does across a restart.
	var revoked []string
	for username, user := range s.Users {
		replacement, exists := newStore.Users[username]
		switch {
		case exists && replacement.PasswordHash != user.PasswordHash:
			revoked = append(revoked, username)
		case !user.Enabled:
		case !exists || !replacement.Enabled:
			revoked = append(revoked, username)
		}
	}

	s.Users = newStore.Users
	s.mu.Unlock()

	s.notifyRevoked(revoked...)
	s.recordFingerprints()
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

// UpdateUser updates an existing user's password and/or annotation. A
// changed password ends the user's password-login sessions and OAuth
// grants.
func (s *UserStore) UpdateUser(username, newPassword, newAnnotation string) error {
	s.mu.Lock()

	user, exists := s.Users[username]
	if !exists {
		s.mu.Unlock()
		return fmt.Errorf("user '%s' not found", username)
	}

	if newPassword != "" {
		passwordHash, err := HashPassword(newPassword)
		if err != nil {
			s.mu.Unlock()
			return err
		}
		user.PasswordHash = passwordHash
	}

	if newAnnotation != "" {
		user.Annotation = newAnnotation
	}
	s.mu.Unlock()

	if newPassword != "" {
		s.notifyRevoked(username)
		s.recordFingerprints()
	}
	return nil
}

// RemoveUser removes a user from the store
func (s *UserStore) RemoveUser(username string) error {
	s.mu.Lock()

	if s.Users == nil {
		s.mu.Unlock()
		return fmt.Errorf("user '%s' not found", username)
	}

	if _, exists := s.Users[username]; !exists {
		s.mu.Unlock()
		return fmt.Errorf("user '%s' not found", username)
	}

	delete(s.Users, username)
	s.mu.Unlock()

	// A deleted user keeps nothing: any OAuth token standing in its
	// name is revoked now rather than at its own expiry.
	s.notifyRevoked(username)
	s.recordFingerprints()
	return nil
}

// VerifyCredentials checks the username and password, counting failures
// and locking the account out after maxFailedAttempts (when > 0), and on
// success records the login time and resets the failure counter. It
// issues no session, which is what callers that mint their own
// credentials, such as the OAuth login page, need.
func (s *UserStore) VerifyCredentials(username, password string, maxFailedAttempts int) error {
	_, err := s.verifyCredentials(username, password, maxFailedAttempts)
	return err
}

// verifyCredentials is VerifyCredentials, additionally returning the
// fingerprint of the password hash that was verified, so a session issued
// afterwards is bound to that hash even if the password changes meanwhile.
func (s *UserStore) verifyCredentials(username, password string, maxFailedAttempts int) (string, error) {
	s.mu.Lock()

	user, exists := s.Users[username]
	if !exists {
		s.mu.Unlock()
		return "", fmt.Errorf("invalid username or password")
	}

	if !user.Enabled {
		s.mu.Unlock()
		return "", fmt.Errorf("user account is disabled")
	}

	// Verify password
	if err := VerifyPassword(password, user.PasswordHash); err != nil {
		// Increment failed attempts counter
		user.FailedAttempts++

		// Lock account if threshold is reached (only if maxFailedAttempts > 0)
		lockedOut := maxFailedAttempts > 0 && user.FailedAttempts >= maxFailedAttempts
		if lockedOut {
			user.Enabled = false
		}
		s.mu.Unlock()

		// An automatic lockout withdraws access just as a manual
		// disable does, so it revokes the account's tokens and
		// sessions too.
		if lockedOut {
			s.notifyRevoked(username)
		}
		return "", fmt.Errorf("invalid username or password")
	}

	// Update last login time (this will be persisted)
	now := time.Now()
	user.LastLogin = &now

	// Reset failed attempts counter on successful login
	user.FailedAttempts = 0
	fingerprint := passwordFingerprint(user.PasswordHash)
	s.mu.Unlock()

	return fingerprint, nil
}

// AuthenticateUser verifies credentials and returns a session token
// Returns the token and expiration time on success
// maxFailedAttempts: if > 0, will disable account after N consecutive failed attempts
func (s *UserStore) AuthenticateUser(username, password string, maxFailedAttempts int) (string, time.Time, error) {
	fingerprint, err := s.verifyCredentials(username, password, maxFailedAttempts)
	if err != nil {
		return "", time.Time{}, err
	}

	token, err := GenerateSessionToken()
	if err != nil {
		return "", time.Time{}, err
	}

	// Session valid for the default duration
	now := time.Now()
	expiration := now.Add(DefaultSessionExpiration)

	if err := s.sessionStore().Create(Session{
		TokenHash:           HashToken(token),
		Username:            username,
		PasswordFingerprint: fingerprint,
		CreatedAt:           now,
		ExpiresAt:           expiration,
	}); err != nil {
		log.Printf("[AUTH] Failed to store session for %q: %v", username, err)
		return "", time.Time{}, fmt.Errorf("failed to create session")
	}

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

// ValidateSessionToken checks if a session token is valid for a user. A
// session whose user is gone or disabled, or whose password has changed
// since it was issued, is deleted as well as rejected.
func (s *UserStore) ValidateSessionToken(token string) (string, error) {
	ss := s.sessionStore()
	hash := HashToken(token)

	session, found, err := ss.Get(hash)
	if err != nil {
		log.Printf("[AUTH] Failed to look up session: %v", err)
		return "", fmt.Errorf("invalid session token")
	}
	if !found {
		return "", fmt.Errorf("invalid session token")
	}

	if !session.ExpiresAt.After(time.Now()) {
		return "", fmt.Errorf("session has expired")
	}

	s.mu.RLock()
	user, exists := s.Users[session.Username]
	var (
		enabled     bool
		fingerprint string
	)
	if exists {
		enabled = user.Enabled
		fingerprint = passwordFingerprint(user.PasswordHash)
	}
	s.mu.RUnlock()

	var reject error
	switch {
	case !exists:
		reject = fmt.Errorf("invalid session token")
	case !enabled:
		reject = fmt.Errorf("user account is disabled")
	case subtle.ConstantTimeCompare([]byte(fingerprint), []byte(session.PasswordFingerprint)) != 1:
		reject = fmt.Errorf("invalid session token")
	}
	if reject != nil {
		if err := ss.Delete(hash); err != nil {
			log.Printf("[AUTH] Failed to delete session: %v", err)
		}
		return "", reject
	}

	return session.Username, nil
}

// EndSession deletes the password-login session token stands for, if
// any. It looks the session up by the hash of token, exactly as
// ValidateSessionToken does, so it can end only the session of whoever
// holds the token; any other token, an API or OAuth token included, is
// not a session and is left alone. Ending an absent session is not an
// error.
func (s *UserStore) EndSession(token string) error {
	if token == "" {
		return nil
	}
	return s.sessionStore().Delete(HashToken(token))
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
// Searches /etc/pgedge/ first, then binary directory
func GetDefaultUserPath(binaryPath string) string {
	systemPath := "/etc/pgedge/postgres-mcp-users.yaml"
	if _, err := os.Stat(systemPath); err == nil {
		return systemPath
	}

	dir := filepath.Dir(binaryPath)
	return filepath.Join(dir, "postgres-mcp-users.yaml")
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

	user, exists := s.Users[username]
	if !exists {
		s.mu.Unlock()
		return fmt.Errorf("user '%s' not found", username)
	}
	user.Enabled = false
	s.mu.Unlock()

	// Disabling is meant to take effect at once, so the account's
	// existing OAuth tokens go with it.
	s.notifyRevoked(username)
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
