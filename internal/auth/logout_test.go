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
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
)

func TestEndSession(t *testing.T) {
	forEachBackend(t, func(t *testing.T, us *UserStore, _ SessionStore) {
		mustAdd(t, us, "bob")
		first := login(t, us, "alice")
		second := login(t, us, "alice")
		bobs := login(t, us, "bob")

		if err := us.EndSession(first); err != nil {
			t.Fatal(err)
		}
		if _, err := us.ValidateSessionToken(first); err == nil {
			t.Fatal("an ended session still validates")
		}
		for name, tok := range map[string]string{"alice's other": second, "bob's": bobs} {
			if _, err := us.ValidateSessionToken(tok); err != nil {
				t.Fatalf("%s session was ended too: %v", name, err)
			}
		}

		// Ending it again, or ending a token that was never a
		// session, is not an error.
		for _, tok := range []string{first, "not-a-session-token", ""} {
			if err := us.EndSession(tok); err != nil {
				t.Fatalf("EndSession(%q) = %v", tok, err)
			}
		}
	})
}

func TestLogoutPathIsPublic(t *testing.T) {
	v := &Validator{Methods: Methods{PasswordLogin: true}}
	if !slices.Contains(v.PublicPaths(), LogoutPath) {
		t.Fatalf("PublicPaths() = %v, missing %q", v.PublicPaths(), LogoutPath)
	}

	reached := false
	h := AuthMiddleware(v, true)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached = true
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodPost, LogoutPath, nil)
	req.Header.Set("Authorization", "Bearer expired-or-unknown")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if !reached {
		t.Fatalf("logout was challenged by the middleware: %d", rec.Code)
	}
}
