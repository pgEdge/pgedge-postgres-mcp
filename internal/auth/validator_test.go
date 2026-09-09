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
	"context"
	"errors"
	"testing"
)

type fakeOAuth struct{ tokens map[string]string }

func (f fakeOAuth) ValidateAccessToken(t string) (string, string, bool) {
	s, ok := f.tokens[t]
	return s, "cid", ok
}
func (f fakeOAuth) Issuer() string { return "https://mcp.example.com" }

func newValidator(t *testing.T) (*Validator, string, string) {
	t.Helper()
	ts := &TokenStore{Tokens: map[string]*Token{}}
	apiTok, err := GenerateToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := ts.AddToken("t1", HashToken(apiTok), "test", nil, ""); err != nil {
		t.Fatal(err)
	}
	us := InitializeUserStore()
	if err := us.AddUser("alice", "pw", ""); err != nil {
		t.Fatal(err)
	}
	sess, _, err := us.AuthenticateUser("alice", "pw", 0)
	if err != nil {
		t.Fatal(err)
	}
	v := &Validator{Tokens: ts, Users: us, OAuth: fakeOAuth{tokens: map[string]string{"oa1": "bob"}}, Methods: Methods{true, true, true}}
	return v, apiTok, sess
}

func TestValidateEachKind(t *testing.T) {
	v, apiTok, sess := newValidator(t)
	if id, err := v.Validate(apiTok); err != nil || id.Kind != IdentityAPIToken || id.Username != "" {
		t.Fatal(id, err)
	}
	if id, err := v.Validate(sess); err != nil || id.Kind != IdentitySession || id.Username != "alice" {
		t.Fatal(id, err)
	}
	if id, err := v.Validate("oa1"); err != nil || id.Kind != IdentityOAuth || id.Username != "bob" || id.TokenHash != HashToken("oa1") {
		t.Fatal(id, err)
	}
	if _, err := v.Validate("junk"); !errors.Is(err, ErrInvalidToken) {
		t.Fatal(err)
	}
}

func TestValidateHonoursToggles(t *testing.T) {
	v, apiTok, sess := newValidator(t)
	v.Methods = Methods{APITokens: false, PasswordLogin: true, OAuth: true}
	if _, err := v.Validate(apiTok); err == nil {
		t.Fatal("api token accepted when disabled")
	}
	v.Methods = Methods{APITokens: true, PasswordLogin: false, OAuth: true}
	if _, err := v.Validate(sess); err == nil {
		t.Fatal("session accepted when disabled")
	}
	v.Methods = Methods{APITokens: true, PasswordLogin: true, OAuth: false}
	if _, err := v.Validate("oa1"); err == nil {
		t.Fatal("oauth accepted when disabled")
	}
	if v.OAuthEnabled() {
		t.Fatal("OAuthEnabled should follow the toggle")
	}
}

func TestContextWithIdentity(t *testing.T) {
	v := &Validator{}
	ctx := v.ContextWithIdentity(context.Background(), Identity{Kind: IdentityOAuth, TokenHash: "h", Username: "bob"})
	if GetTokenHashFromContext(ctx) != "h" || GetUsernameFromContext(ctx) != "bob" || IsAPITokenFromContext(ctx) {
		t.Fatal("context values wrong")
	}
}

func TestParseBearer(t *testing.T) {
	if tok, ok := ParseBearer("Bearer abc"); !ok || tok != "abc" {
		t.Fatal(tok, ok)
	}
	if _, ok := ParseBearer("Basic abc"); ok {
		t.Fatal("basic accepted")
	}
	if _, ok := ParseBearer(""); ok {
		t.Fatal("empty accepted")
	}
}
