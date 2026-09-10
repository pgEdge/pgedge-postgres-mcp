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
	"testing"
	"time"
)

func TestCSRFRoundTrip(t *testing.T) {
	s, _ := newCSRFSigner()
	now := time.Now()
	tok := s.Issue(now)
	if !s.Verify(tok, now.Add(time.Minute), 10*time.Minute) {
		t.Fatal("valid token rejected")
	}
	if s.Verify(tok, now.Add(11*time.Minute), 10*time.Minute) {
		t.Fatal("expired token accepted")
	}
	if s.Verify(tok+"a", now, 10*time.Minute) {
		t.Fatal("tampered accepted")
	}
	other, _ := newCSRFSigner()
	if other.Verify(tok, now, 10*time.Minute) {
		t.Fatal("foreign key accepted")
	}
}
