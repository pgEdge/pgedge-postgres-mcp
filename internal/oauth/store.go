/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package oauth

import (
	"errors"
	"sync"
	"time"
)

// ErrStoreFull is returned by a Put* method when its collection has
// reached its configured limit.
var ErrStoreFull = errors.New("oauth: store limit reached")

// Client is a registered OAuth client.
type Client struct {
	ID           string
	Name         string
	RedirectURIs []string
	CreatedAt    time.Time
}

// AuthCode is a single-use authorisation code issued during the
// authorisation code grant.
type AuthCode struct {
	Hash          string
	ClientID      string
	RedirectURI   string
	Subject       string
	Scope         string
	CodeChallenge string
	ExpiresAt     time.Time
}

// DeviceCode is an in-flight device authorisation grant.
type DeviceCode struct {
	DeviceHash string
	UserCode   string
	ClientID   string
	Scope      string
	Subject    string // set on approval
	Approved   bool
	Denied     bool
	ExpiresAt  time.Time
	LastPolled time.Time
	Interval   time.Duration
}

// Token is an issued access or refresh token.
type Token struct {
	Hash        string
	ClientID    string
	Subject     string
	Scope       string
	ExpiresAt   time.Time
	RefreshHash string // for access tokens: the refresh token that issued it (may be empty)
	IsRefresh   bool
	Issued      []string // for refresh tokens: hashes of access tokens issued from it
}

// Limits bounds how many entries each of the store's collections may
// hold, so an unauthenticated caller cannot exhaust memory.
type Limits struct{ Clients, Codes, DeviceCodes, Tokens int }

// DefaultLimits is a sensible default for production use.
var DefaultLimits = Limits{Clients: 1000, Codes: 1000, DeviceCodes: 1000, Tokens: 10000}

// Store is an in-memory, thread-safe store for OAuth clients, codes,
// device codes and tokens. Getters return deep copies so callers cannot
// mutate shared state except through the Update* methods.
type Store struct {
	mu sync.Mutex

	limits Limits

	clients   map[string]*Client
	codes     map[string]*AuthCode
	devices   map[string]*DeviceCode
	userCodes map[string]string // device user code -> device hash
	tokens    map[string]*Token
}

// NewStore creates an empty Store bounded by limits.
func NewStore(limits Limits) *Store {
	return &Store{
		limits:    limits,
		clients:   make(map[string]*Client),
		codes:     make(map[string]*AuthCode),
		devices:   make(map[string]*DeviceCode),
		userCodes: make(map[string]string),
		tokens:    make(map[string]*Token),
	}
}

// cloneStrings returns a copy of ss so a caller mutating the result
// cannot reach the store's own backing array.
func cloneStrings(ss []string) []string {
	if ss == nil {
		return nil
	}
	out := make([]string, len(ss))
	copy(out, ss)
	return out
}

func cloneClient(c *Client) *Client {
	cp := *c
	cp.RedirectURIs = cloneStrings(c.RedirectURIs)
	return &cp
}

func cloneAuthCode(c *AuthCode) *AuthCode {
	cp := *c
	return &cp
}

func cloneDeviceCode(d *DeviceCode) *DeviceCode {
	cp := *d
	return &cp
}

func cloneToken(t *Token) *Token {
	cp := *t
	cp.Issued = cloneStrings(t.Issued)
	return &cp
}

// PutClient adds or replaces a client, provided the collection has not
// reached its limit.
func (s *Store) PutClient(c *Client) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.clients[c.ID]; !exists && len(s.clients) >= s.limits.Clients {
		return ErrStoreFull
	}
	s.clients[c.ID] = cloneClient(c)
	return nil
}

// GetClient returns a copy of the client registered under id, if any.
func (s *Store) GetClient(id string) (*Client, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.clients[id]
	if !ok {
		return nil, false
	}
	return cloneClient(c), true
}

// PutCode adds an authorisation code, provided the collection has not
// reached its limit.
func (s *Store) PutCode(c *AuthCode) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.codes[c.Hash]; !exists && len(s.codes) >= s.limits.Codes {
		return ErrStoreFull
	}
	s.codes[c.Hash] = cloneAuthCode(c)
	return nil
}

// TakeCode returns and removes the authorisation code stored under hash,
// enforcing single use.
func (s *Store) TakeCode(hash string) (*AuthCode, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.codes[hash]
	if !ok {
		return nil, false
	}
	delete(s.codes, hash)
	return cloneAuthCode(c), true
}

// PutDeviceCode adds a device code, provided the collection has not
// reached its limit.
func (s *Store) PutDeviceCode(d *DeviceCode) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.devices[d.DeviceHash]; !exists && len(s.devices) >= s.limits.DeviceCodes {
		return ErrStoreFull
	}
	s.devices[d.DeviceHash] = cloneDeviceCode(d)
	s.userCodes[d.UserCode] = d.DeviceHash
	return nil
}

// GetDeviceByHash returns a copy of the device code stored under hash.
func (s *Store) GetDeviceByHash(hash string) (*DeviceCode, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	d, ok := s.devices[hash]
	if !ok {
		return nil, false
	}
	return cloneDeviceCode(d), true
}

// GetDeviceByUserCode returns a copy of the device code registered under
// the given user-facing code.
func (s *Store) GetDeviceByUserCode(code string) (*DeviceCode, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	hash, ok := s.userCodes[code]
	if !ok {
		return nil, false
	}
	d, ok := s.devices[hash]
	if !ok {
		return nil, false
	}
	return cloneDeviceCode(d), true
}

// UpdateDevice replaces the stored device code sharing d's DeviceHash. It
// is a no-op if that hash is not present.
func (s *Store) UpdateDevice(d *DeviceCode) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.devices[d.DeviceHash]; !ok {
		return
	}
	s.devices[d.DeviceHash] = cloneDeviceCode(d)
}

// DeleteDevice removes the device code stored under hash, along with its
// user-code mapping.
func (s *Store) DeleteDevice(hash string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if d, ok := s.devices[hash]; ok {
		delete(s.userCodes, d.UserCode)
	}
	delete(s.devices, hash)
}

// PutToken adds an access or refresh token, provided the collection has
// not reached its limit. Access and refresh tokens share one collection
// and one limit, distinguished by IsRefresh.
func (s *Store) PutToken(t *Token) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tokens[t.Hash]; !exists && len(s.tokens) >= s.limits.Tokens {
		return ErrStoreFull
	}
	s.tokens[t.Hash] = cloneToken(t)
	return nil
}

// GetToken returns a copy of the token stored under hash.
func (s *Store) GetToken(hash string) (*Token, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.tokens[hash]
	if !ok {
		return nil, false
	}
	return cloneToken(t), true
}

// DeleteToken removes the token stored under hash. Deleting a refresh
// token cascades to every access token it issued; deleting an access
// token removes its hash from its parent refresh token's Issued list.
func (s *Store) DeleteToken(hash string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.deleteTokenLocked(hash)
}

// deleteTokenLocked implements DeleteToken; callers must hold s.mu.
func (s *Store) deleteTokenLocked(hash string) {
	t, ok := s.tokens[hash]
	if !ok {
		return
	}
	delete(s.tokens, hash)

	if t.IsRefresh {
		for _, issued := range t.Issued {
			delete(s.tokens, issued)
		}
		return
	}

	if t.RefreshHash == "" {
		return
	}
	if parent, ok := s.tokens[t.RefreshHash]; ok {
		for i, h := range parent.Issued {
			if h == hash {
				parent.Issued = append(parent.Issued[:i], parent.Issued[i+1:]...)
				break
			}
		}
	}
}

// Sweep removes every client-facing entry whose ExpiresAt is before now.
// Clients have no expiry and are left untouched.
func (s *Store) Sweep(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for hash, c := range s.codes {
		if c.ExpiresAt.Before(now) {
			delete(s.codes, hash)
		}
	}
	for hash, d := range s.devices {
		if d.ExpiresAt.Before(now) {
			delete(s.userCodes, d.UserCode)
			delete(s.devices, hash)
		}
	}
	for hash, t := range s.tokens {
		if t.ExpiresAt.Before(now) {
			s.deleteTokenLocked(hash)
		}
	}
}

// StartSweeper runs Sweep on a ticker of interval in a background
// goroutine, until the returned stop function is called. stop is
// idempotent and safe to call more than once.
func (s *Store) StartSweeper(interval time.Duration) (stop func()) {
	ticker := time.NewTicker(interval)
	done := make(chan struct{})

	go func() {
		for {
			select {
			case <-ticker.C:
				s.Sweep(time.Now())
			case <-done:
				return
			}
		}
	}()

	var once sync.Once
	return func() {
		once.Do(func() {
			ticker.Stop()
			close(done)
		})
	}
}
