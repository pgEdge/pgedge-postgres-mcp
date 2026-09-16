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

	ip := s.clientIP(r)
	if s.opts.RateLimiter != nil && !s.opts.RateLimiter.IsAllowed(ip) {
		w.Header().Set("Retry-After", "60")
		writeJSONError(w, newError("access_denied", "too many requests", http.StatusTooManyRequests))
		return
	}

	if r.Method != http.MethodPost {
		if s.opts.RateLimiter != nil {
			s.opts.RateLimiter.RecordFailedAttempt(ip)
		}
		writeJSONError(w, newError("invalid_request", "method not allowed", http.StatusMethodNotAllowed))
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRevokeBodyBytes)
	if err := r.ParseForm(); err != nil {
		if s.opts.RateLimiter != nil {
			s.opts.RateLimiter.RecordFailedAttempt(ip)
		}
		writeJSONError(w, newError("invalid_request", "malformed request", http.StatusBadRequest))
		return
	}

	token := r.PostFormValue("token")
	if token == "" {
		if s.opts.RateLimiter != nil {
			s.opts.RateLimiter.RecordFailedAttempt(ip)
		}
		writeJSONError(w, newError("invalid_request", "token is required", http.StatusBadRequest))
		return
	}

	s.store.DeleteToken(hashToken(token))

	w.WriteHeader(http.StatusOK)
}
