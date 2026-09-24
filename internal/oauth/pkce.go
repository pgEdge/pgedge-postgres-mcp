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
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
)

// verifyPKCE validates that verifier matches the stored challenge under
// method. Only the S256 method is supported; plain is always rejected.
func verifyPKCE(verifier, challenge, method string) bool {
	if method != "S256" {
		return false
	}
	if verifier == "" || challenge == "" {
		return false
	}

	// challenge = BASE64URL(SHA256(verifier))
	h := sha256.Sum256([]byte(verifier))
	computed := base64.RawURLEncoding.EncodeToString(h[:])

	return subtle.ConstantTimeCompare([]byte(computed), []byte(challenge)) == 1
}

// validCodeVerifier reports whether v meets the RFC 7636 requirements for
// a code verifier: 43 to 128 characters, all unreserved per the RFC
// ([A-Z] / [a-z] / [0-9] / "-" / "." / "_" / "~").
func validCodeVerifier(v string) bool {
	if len(v) < 43 || len(v) > 128 {
		return false
	}
	for _, c := range v {
		if !isUnreservedChar(c) {
			return false
		}
	}
	return true
}

// validCodeChallenge reports whether c looks like a valid S256 code
// challenge: a 43-character base64url string, the length a 32-byte
// SHA-256 digest encodes to without padding.
func validCodeChallenge(c string) bool {
	if len(c) != 43 {
		return false
	}
	for _, r := range c {
		if !isBase64URLChar(r) {
			return false
		}
	}
	return true
}

// isUnreservedChar reports whether c is an RFC 7636 unreserved character.
func isUnreservedChar(c rune) bool {
	return (c >= 'A' && c <= 'Z') ||
		(c >= 'a' && c <= 'z') ||
		(c >= '0' && c <= '9') ||
		c == '-' || c == '.' || c == '_' || c == '~'
}

// isBase64URLChar reports whether c is a valid unpadded base64url character.
func isBase64URLChar(c rune) bool {
	return (c >= 'A' && c <= 'Z') ||
		(c >= 'a' && c <= 'z') ||
		(c >= '0' && c <= '9') ||
		c == '-' || c == '_'
}
