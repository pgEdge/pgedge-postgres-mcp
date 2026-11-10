/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"pgedge-postgres-mcp/internal/auth"
)

const fakePassword = "correct-horse-battery"

func logoutUsers(t *testing.T) *auth.UserStore {
	t.Helper()
	us := auth.InitializeUserStore()
	for _, name := range []string{"alice", "bob"} {
		if err := us.AddUser(name, fakePassword, ""); err != nil {
			t.Fatal(err)
		}
	}
	return us
}

func loginAs(t *testing.T, us *auth.UserStore, name string) string {
	t.Helper()
	tok, _, err := us.AuthenticateUser(name, fakePassword, 0)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func postLogout(h http.Handler, method, authorization string) int {
	req := httptest.NewRequest(method, auth.LogoutPath, nil)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code
}

func TestLogoutHandler_EndsOnlyThePresentedSession(t *testing.T) {
	us := logoutUsers(t)
	alice := loginAs(t, us, "alice")
	aliceOther := loginAs(t, us, "alice")
	bob := loginAs(t, us, "bob")
	h := logoutHandler(us)

	if code := postLogout(h, http.MethodPost, "Bearer "+alice); code != http.StatusNoContent {
		t.Fatalf("logout = %d, want 204", code)
	}
	if _, err := us.ValidateSessionToken(alice); err == nil {
		t.Fatal("the presented session survived logout")
	}
	for _, tok := range []string{aliceOther, bob} {
		if _, err := us.ValidateSessionToken(tok); err != nil {
			t.Fatalf("another session was ended: %v", err)
		}
	}
}

func TestLogoutHandler_AlwaysNoContent(t *testing.T) {
	us := logoutUsers(t)
	alice := loginAs(t, us, "alice")
	h := logoutHandler(us)
	_ = postLogout(h, http.MethodPost, "Bearer "+alice)

	for name, authz := range map[string]string{
		"already ended": "Bearer " + alice,
		"unknown token": "Bearer not-a-session-token",
		"no header":     "",
		"malformed":     "Basic YWxpY2U6eA==",
	} {
		if code := postLogout(h, http.MethodPost, authz); code != http.StatusNoContent {
			t.Errorf("%s: logout = %d, want 204", name, code)
		}
	}

	if code := postLogout(logoutHandler(nil), http.MethodPost, "Bearer x"); code != http.StatusNoContent {
		t.Errorf("no user store: logout = %d, want 204", code)
	}
}

func TestLogoutHandler_RejectsOtherMethods(t *testing.T) {
	us := logoutUsers(t)
	alice := loginAs(t, us, "alice")
	h := logoutHandler(us)

	for _, m := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		if code := postLogout(h, m, "Bearer "+alice); code != http.StatusMethodNotAllowed {
			t.Errorf("%s: logout = %d, want 405", m, code)
		}
	}
	if _, err := us.ValidateSessionToken(alice); err != nil {
		t.Fatalf("a rejected method ended the session: %v", err)
	}
}
