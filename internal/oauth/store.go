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

	// LastUsed is when the client was last seen at the authorisation or
	// token endpoint. It is set at registration and refreshed by
	// TouchClient, so that Sweep can retire the registrations of
	// clients that have gone away, and PutClient can evict the least
	// recently used one when the table is full.
	LastUsed time.Time
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

	// Family identifies the chain of tokens descended from a single
	// authorisation: the authorisation code grant starts a new family
	// and each refresh rotation carries it forward, so that a replayed
	// refresh token can revoke the whole chain.
	Family string
}

// rotatedRefresh records the family a refresh token belonged to before
// it was rotated away, so that a replay of the old token can be
// recognised and answered by revoking the family. Until bounds how long
// the record is kept.
type rotatedRefresh struct {
	Family string
	Until  time.Time
}

// usedCode records the refresh token issued from an authorisation code
// that has already been redeemed, so a replay of that code can cascade
// the revocation to every token it produced. Family is recorded
// alongside the hash because the refresh token itself is gone once it
// has been rotated, whilst its descendants remain valid and are the
// ones that have to be revoked.
type usedCode struct {
	RefreshHash string
	Family      string
	Until       time.Time
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

	// clientRetention is how long an unused client registration is
	// kept, over and above clientIdleGrace: a server passes its refresh
	// token lifetime, since a client whose longest-lived credential has
	// expired has nothing left to present.
	clientRetention time.Duration

	clients   map[string]*Client
	codes     map[string]*AuthCode
	devices   map[string]*DeviceCode
	userCodes map[string]string // device user code -> device hash
	tokens    map[string]*Token
	usedCodes map[string]*usedCode
	rotated   map[string]*rotatedRefresh
}

// clientIdleGrace is added to a Store's clientRetention before an idle
// client registration is swept, so that a client returning at the very
// end of its refresh token's life still finds itself registered.
const clientIdleGrace = time.Hour

// NewStore creates an empty Store bounded by limits, retaining an
// unused client registration for clientRetention (plus an hour's
// grace) after it was last seen.
func NewStore(limits Limits, clientRetention time.Duration) *Store {
	return &Store{
		limits:          limits,
		clientRetention: clientRetention,
		clients:         make(map[string]*Client),
		codes:           make(map[string]*AuthCode),
		devices:         make(map[string]*DeviceCode),
		userCodes:       make(map[string]string),
		tokens:          make(map[string]*Token),
		usedCodes:       make(map[string]*usedCode),
		rotated:         make(map[string]*rotatedRefresh),
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

// PutClient adds or replaces a client. When the collection is already
// full, the least recently used client that has no live tokens is
// evicted to make room, since a registration nobody has come back for
// is worth less than the one being made now; ErrStoreFull is returned
// only when every registered client still holds a live token.
//
// c.LastUsed stands in for the current time, so that eviction does not
// need a clock of its own; a zero LastUsed makes every token look live
// and so fails closed with ErrStoreFull rather than evicting.
func (s *Store) PutClient(c *Client) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.clients[c.ID]; !exists && len(s.clients) >= s.limits.Clients {
		victim, found := s.lruEvictableClientLocked(c.LastUsed)
		if !found {
			return ErrStoreFull
		}
		delete(s.clients, victim)
	}
	s.clients[c.ID] = cloneClient(c)
	return nil
}

// lruEvictableClientLocked returns the id of the least recently used
// client that has no live token at now. Callers must hold s.mu.
func (s *Store) lruEvictableClientLocked(now time.Time) (id string, found bool) {
	live := s.clientsWithLiveTokensLocked(now)
	var oldest time.Time
	for cid, c := range s.clients {
		if live[cid] {
			continue
		}
		if !found || c.LastUsed.Before(oldest) {
			id, oldest, found = cid, c.LastUsed, true
		}
	}
	return id, found
}

// clientsWithLiveTokensLocked returns the set of client ids holding at
// least one token that has not expired at now. It is built once per
// pass, rather than asking the question per client, so a sweep or an
// eviction costs one walk of the tokens instead of one per client.
// Callers must hold s.mu.
func (s *Store) clientsWithLiveTokensLocked(now time.Time) map[string]bool {
	live := make(map[string]bool)
	for _, t := range s.tokens {
		if t.ExpiresAt.After(now) {
			live[t.ClientID] = true
		}
	}
	return live
}

// TouchClient records that the client registered under id has just been
// seen, at now. It is a no-op for an unknown id, so that a request
// naming a client that has already been swept does not resurrect it.
func (s *Store) TouchClient(id string, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if c, ok := s.clients[id]; ok {
		c.LastUsed = now
	}
}

// MarkRefreshRotated records that the refresh token hashed to hash has
// been rotated away from family, keeping the record until until so that
// a replay of the old token can be recognised. The records are bounded
// by the same limit as tokens; when that is reached the record expiring
// soonest is discarded to make room, rather than the new one being
// dropped, so that rotating one token over and over cannot fill the
// table and quietly switch replay detection off for everyone else.
func (s *Store) MarkRefreshRotated(hash, family string, until time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.rotated[hash]; !exists && len(s.rotated) >= s.limits.Tokens {
		s.evictOldestRotatedLocked()
	}
	s.rotated[hash] = &rotatedRefresh{Family: family, Until: until}
}

// evictOldestRotatedLocked removes the rotation record whose retention
// ends soonest, which is the one closest to being swept anyway.
// Callers must hold s.mu.
func (s *Store) evictOldestRotatedLocked() {
	var (
		oldestHash string
		oldest     time.Time
		found      bool
	)
	for hash, r := range s.rotated {
		if !found || r.Until.Before(oldest) {
			oldestHash, oldest, found = hash, r.Until, true
		}
	}
	if found {
		delete(s.rotated, oldestHash)
	}
}

// RotatedRefreshFamily returns the family a rotated refresh token
// belonged to, if hash names one that is still on record.
func (s *Store) RotatedRefreshFamily(hash string) (family string, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	r, found := s.rotated[hash]
	if !found {
		return "", false
	}
	return r.Family, true
}

// DeleteFamily removes every token belonging to family, which is how a
// replayed refresh token is answered: the whole chain descended from
// the original authorisation is revoked.
func (s *Store) DeleteFamily(family string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if family == "" {
		return
	}
	for hash, t := range s.tokens {
		if t.Family == family {
			delete(s.tokens, hash)
		}
	}
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

// MarkCodeUsed records that the authorisation code hashed to codeHash has
// been redeemed for the refresh token hashed to refreshHash, belonging to
// family, so that a replay of the same code can be detected and its
// issued tokens cascade-revoked. The entry is bounded by the same limit
// as Codes and expires at until, which should be no earlier than the
// code's original expiry plus the access token lifetime.
func (s *Store) MarkCodeUsed(codeHash, refreshHash, family string, until time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.usedCodes[codeHash]; !exists && len(s.usedCodes) >= s.limits.Codes {
		return
	}
	s.usedCodes[codeHash] = &usedCode{RefreshHash: refreshHash, Family: family, Until: until}
}

// TakeUsedCode returns and removes the refresh token hash and token
// family recorded for a previously redeemed authorisation code,
// enforcing single use of the replay record itself.
func (s *Store) TakeUsedCode(codeHash string) (refreshHash, family string, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	u, found := s.usedCodes[codeHash]
	if !found {
		return "", "", false
	}
	delete(s.usedCodes, codeHash)
	return u.RefreshHash, u.Family, true
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
// is a no-op if that hash is not present. If d.UserCode differs from the
// stored record's, the userCodes index is re-keyed so it never goes
// stale.
func (s *Store) UpdateDevice(d *DeviceCode) {
	s.mu.Lock()
	defer s.mu.Unlock()

	old, ok := s.devices[d.DeviceHash]
	if !ok {
		return
	}
	if old.UserCode != d.UserCode {
		delete(s.userCodes, old.UserCode)
		s.userCodes[d.UserCode] = d.DeviceHash
	}
	s.devices[d.DeviceHash] = cloneDeviceCode(d)
}

// ApproveDevice marks the device code stored under hash as approved by
// subject, but only if it is not already approved or denied. The check
// and the write happen under one lock, so of two concurrent callers
// approving the same device code (whether as the same or different
// resource owners), at most one can ever succeed: the loser's write is
// discarded and it receives ok == false, which makes approval
// single-writer regardless of how many verification requests race for
// the same user code.
func (s *Store) ApproveDevice(hash, subject string) (ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	dev, found := s.devices[hash]
	if !found || dev.Approved || dev.Denied {
		return false
	}
	dev.Subject = subject
	dev.Approved = true
	return true
}

// DenyDevice marks the device code stored under hash as refused by the
// resource owner, but only if it is not already approved or denied. Like
// ApproveDevice, the check and the write happen under one lock, so the
// outcome of a race between an approval and a refusal is whichever
// arrived first, and never both.
func (s *Store) DenyDevice(hash string) (ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	dev, found := s.devices[hash]
	if !found || dev.Approved || dev.Denied {
		return false
	}
	dev.Denied = true
	return true
}

// DeleteDevice removes the device code stored under hash, along with its
// user-code mapping.
func (s *Store) DeleteDevice(hash string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.deleteDeviceLocked(hash)
}

// deleteDeviceLocked implements DeleteDevice; callers must hold s.mu.
func (s *Store) deleteDeviceLocked(hash string) {
	if d, ok := s.devices[hash]; ok {
		delete(s.userCodes, d.UserCode)
	}
	delete(s.devices, hash)
}

// TouchDevicePoll records a poll of the device code stored under hash at
// now, reporting whether the caller must back off. The read of
// LastPolled, the tooSoon comparison against Interval and the write of
// the new LastPolled all happen under one lock, so two concurrent polls
// can never both observe the same stale LastPolled and both proceed: at
// most one of them can see tooSoon == false for a given now. A poll that
// arrives too soon is rejected without moving LastPolled, so the next
// deadline stays a fixed Interval after the last accepted poll. ok is
// false if hash names no device code, in which case d is nil and tooSoon
// is meaningless.
func (s *Store) TouchDevicePoll(hash string, now time.Time) (d *DeviceCode, tooSoon bool, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	dev, found := s.devices[hash]
	if !found {
		return nil, false, false
	}
	tooSoon = now.Sub(dev.LastPolled) < dev.Interval
	if !tooSoon {
		// Only an accepted poll moves the clock on. Advancing it on a
		// rejected one too would let a client polling at any fixed
		// period shorter than Interval push the deadline out on every
		// attempt, so it would be told to slow down for ever.
		dev.LastPolled = now
	}
	return cloneDeviceCode(dev), tooSoon, true
}

// TakeDeviceIfApproved removes and returns the device code stored under
// hash if, and only if, it is currently approved and not denied. The
// check and the removal happen under one lock, so of two concurrent
// callers presenting the same approved device code, at most one can ever
// receive ok == true, making token issuance from the device grant single
// use.
func (s *Store) TakeDeviceIfApproved(hash string) (d *DeviceCode, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	dev, found := s.devices[hash]
	if !found || !dev.Approved || dev.Denied {
		return nil, false
	}
	cp := cloneDeviceCode(dev)
	s.deleteDeviceLocked(hash)
	return cp, true
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

// TakeToken atomically looks up and removes the token stored under hash,
// applying the same cascade as DeleteToken, and returns a copy of the
// token as it stood immediately before removal. Because the lookup and
// removal happen under a single lock, at most one of two concurrent
// callers presenting the same token can ever receive ok == true, which
// makes it safe to use for single-use tokens such as a refresh token
// being rotated.
func (s *Store) TakeToken(hash string) (*Token, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.tokens[hash]
	if !ok {
		return nil, false
	}
	cp := cloneToken(t)
	s.deleteTokenLocked(hash)
	return cp, true
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

// Sweep removes every client-facing entry whose ExpiresAt is before
// now, along with the client registrations that have not been used
// within clientRetention plus clientIdleGrace and hold no live tokens,
// so that a flood of dynamic registrations drains away by itself.
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
	for hash, u := range s.usedCodes {
		if u.Until.Before(now) {
			delete(s.usedCodes, hash)
		}
	}
	for hash, r := range s.rotated {
		if r.Until.Before(now) {
			delete(s.rotated, hash)
		}
	}

	// Clients are swept last, so that the token expiry above has
	// already run and a client whose last token has just expired is
	// eligible in the same pass.
	cutoff := now.Add(-(s.clientRetention + clientIdleGrace))
	live := s.clientsWithLiveTokensLocked(now)
	for id, c := range s.clients {
		if c.LastUsed.Before(cutoff) && !live[id] {
			delete(s.clients, id)
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
