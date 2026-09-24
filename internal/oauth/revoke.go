/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

// Package oauth implements a minimal OAuth 2.0 authorisation server: this
// file implements the /oauth/revoke endpoint (RFC 7009).
package oauth

import "net/http"

// maxRevokeBodyBytes bounds the size of a revocation request body, so an
// unauthenticated caller cannot exhaust memory with an oversized payload.
const maxRevokeBodyBytes = 64 * 1024

// handleRevoke implements RFC 7009 token revocation. Per the RFC, the
// endpoint always responds 200 with an empty body, whether or not the
// presented token existed, so as not to leak which tokens are valid.
func (s *Server) handleRevoke(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")

	// Revocation is unauthenticated, and its failures (a wrong method, a
	// malformed body, a missing token) are not credential guesses, so it
	// meters on the anonymous limiter rather than the one that gates
	// password login: counting them there would let a handful of empty
	// POSTs lock every user at an address out of signing in.
	//
	// Only those failures are checked against the limiter. A request that
	// carries a token is never refused, because the anonymous budget is
	// also spent by successful registrations and device requests, and
	// both clients clear their local session whatever the response: a 429
	// there would tell the user they had signed out whilst leaving the
	// refresh token working on the server.
	ip := s.clientIP(r)
	limiter := s.opts.AnonymousRateLimiter
	fail := func(e *Error) {
		if limiter != nil {
			if !limiter.IsAllowed(ip) {
				w.Header().Set("Retry-After", "60")
				writeJSONError(w, newError("access_denied", "too many requests", http.StatusTooManyRequests))
				return
			}
			limiter.RecordFailedAttempt(ip)
		}
		writeJSONError(w, e)
	}

	if r.Method != http.MethodPost {
		fail(newError("invalid_request", "method not allowed", http.StatusMethodNotAllowed))
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRevokeBodyBytes)
	if err := r.ParseForm(); err != nil {
		fail(newError("invalid_request", "malformed request", http.StatusBadRequest))
		return
	}

	token := r.PostFormValue("token")
	if token == "" {
		fail(newError("invalid_request", "token is required", http.StatusBadRequest))
		return
	}

	s.store.DeleteToken(hashToken(token))

	w.WriteHeader(http.StatusOK)
}
