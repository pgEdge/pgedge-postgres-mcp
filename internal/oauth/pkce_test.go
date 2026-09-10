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
	"strings"
	"testing"
)

func TestVerifyPKCE_S256(t *testing.T) {
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	challenge := "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM" // RFC 7636 appendix B
	if !verifyPKCE(verifier, challenge, "S256") {
		t.Fatal("expected match")
	}
	if verifyPKCE(verifier, challenge, "plain") {
		t.Fatal("plain must be rejected")
	}
	if verifyPKCE(verifier+"x", challenge, "S256") {
		t.Fatal("mismatch accepted")
	}
}

func TestValidCodeVerifierLength(t *testing.T) {
	if validCodeVerifier(strings.Repeat("a", 42)) {
		t.Fatal("too short accepted")
	}
	if !validCodeVerifier(strings.Repeat("a", 43)) {
		t.Fatal("43 rejected")
	}
	if validCodeVerifier(strings.Repeat("a", 129)) {
		t.Fatal("too long accepted")
	}
	if validCodeVerifier(strings.Repeat("a", 42) + "!") {
		t.Fatal("bad char accepted")
	}
}

func TestValidCodeChallenge(t *testing.T) {
	if !validCodeChallenge(strings.Repeat("a", 43)) {
		t.Fatal("valid 43-char base64url challenge rejected")
	}
	if validCodeChallenge(strings.Repeat("a", 42)) {
		t.Fatal("42-char challenge accepted")
	}
	if validCodeChallenge(strings.Repeat("a", 44)) {
		t.Fatal("44-char challenge accepted")
	}
	if validCodeChallenge(strings.Repeat("a", 42) + "+") {
		t.Fatal("challenge with '+' accepted")
	}
	if validCodeChallenge(strings.Repeat("a", 42) + "/") {
		t.Fatal("challenge with '/' accepted")
	}
	if validCodeChallenge(strings.Repeat("a", 42) + "=") {
		t.Fatal("challenge with '=' accepted")
	}
}
