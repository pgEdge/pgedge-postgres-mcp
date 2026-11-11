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
	"fmt"
	"net/http"
	"os"

	"pgedge-postgres-mcp/internal/auth"
	"pgedge-postgres-mcp/internal/httperror"
)

// logoutHandler serves POST auth.LogoutPath, which ends the password-login
// session named by the bearer token. It answers 204 whether or not the
// token was a live session, so it reveals nothing about the token, and it
// touches nothing else: API and OAuth tokens are not sessions, and OAuth
// has its own revocation endpoint. users may be nil, when the server has
// no user store.
func logoutHandler(users *auth.UserStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			httperror.Write(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		if token, ok := auth.ParseBearer(r.Header.Get("Authorization")); ok && users != nil {
			if err := users.EndSession(token); err != nil {
				fmt.Fprintf(os.Stderr, "WARNING: Failed to end session at logout: %v\n", err)
			}
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
