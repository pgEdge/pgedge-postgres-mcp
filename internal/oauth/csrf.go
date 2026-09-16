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
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"time"
)

const (
	csrfKeyBytes   = 32
	csrfNonceBytes = 16
	csrfMACBytes   = sha256.Size
	csrfTokenBytes = 8 + csrfNonceBytes + csrfMACBytes
	// csrfClockSkew bounds how far into the future an issued timestamp may
	// legitimately sit, to tolerate minor clock drift between issue and
	// verification without accepting arbitrarily forward-dated tokens.
	csrfClockSkew = time.Minute
)

// csrfSigner issues and verifies short-lived, HMAC-signed CSRF tokens
// bound to a server-side key. It holds no other state, so a signer can be
// created once and reused across requests.
type csrfSigner struct{ key []byte }

// newCSRFSigner creates a signer with a fresh, random 32-byte key.
func newCSRFSigner() (*csrfSigner, error) {
	key := make([]byte, csrfKeyBytes)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("oauth: csrf signer: %w", err)
	}
	return &csrfSigner{key: key}, nil
}

// Issue returns a new CSRF token timestamped at now: base64url encoding of
// an 8-byte big-endian Unix timestamp, a 16-byte random nonce and a
// 32-byte HMAC-SHA256 over the timestamp and nonce.
func (s *csrfSigner) Issue(now time.Time) string {
	buf := make([]byte, csrfTokenBytes)
	binary.BigEndian.PutUint64(buf[:8], uint64(now.Unix()))
	// crypto/rand.Read only fails if the OS entropy source is unavailable,
	// which is unrecoverable here; a zero nonce would still be
	// timestamp-unique and HMAC-bound, so ignore the error rather than
	// change Issue's signature for an effectively impossible case.
	_, _ = rand.Read(buf[8 : 8+csrfNonceBytes])

	mac := hmac.New(sha256.New, s.key)
	mac.Write(buf[:8+csrfNonceBytes])
	copy(buf[8+csrfNonceBytes:], mac.Sum(nil))

	return base64.RawURLEncoding.EncodeToString(buf)
}

// Verify reports whether tok is a token issued by s, not tampered with,
// and still valid at now given maxAge.
func (s *csrfSigner) Verify(tok string, now time.Time, maxAge time.Duration) bool {
	buf, err := base64.RawURLEncoding.DecodeString(tok)
	if err != nil || len(buf) != csrfTokenBytes {
		return false
	}

	ts := int64(binary.BigEndian.Uint64(buf[:8]))
	signed := buf[:8+csrfNonceBytes]
	gotMAC := buf[8+csrfNonceBytes:]

	mac := hmac.New(sha256.New, s.key)
	mac.Write(signed)
	wantMAC := mac.Sum(nil)
	if !hmac.Equal(gotMAC, wantMAC) {
		return false
	}

	issued := time.Unix(ts, 0)
	if now.Sub(issued) > maxAge {
		return false
	}
	if issued.After(now.Add(csrfClockSkew)) {
		return false
	}
	return true
}
