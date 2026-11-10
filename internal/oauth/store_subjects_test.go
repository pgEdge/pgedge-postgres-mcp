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
	"sort"
	"testing"
	"time"
)

func TestSubjects(t *testing.T) {
	s := NewStore(DefaultLimits, testClientRetention)
	exp := time.Now().Add(time.Hour)

	if got := s.Subjects(); len(got) != 0 {
		t.Fatalf("empty store Subjects = %v", got)
	}

	_ = s.PutToken(&Token{Hash: "a1", Subject: "alice", ExpiresAt: exp})
	_ = s.PutToken(&Token{Hash: "a2", Subject: "alice", ExpiresAt: exp})
	_ = s.PutToken(&Token{Hash: "b1", Subject: "bob", ExpiresAt: exp, IsRefresh: true})
	_ = s.PutCode(&AuthCode{Hash: "c1", Subject: "carol", ExpiresAt: exp})
	d := newTestDevice("d1", "AAAA-AAAA", exp)
	d.Subject = "dave"
	_ = s.PutDeviceCode(d)
	// An unapproved device grant has no subject and must not appear.
	_ = s.PutDeviceCode(newTestDevice("d2", "BBBB-BBBB", exp))

	got := s.Subjects()
	sort.Strings(got)
	want := []string{"alice", "bob", "carol", "dave"}
	if len(got) != len(want) {
		t.Fatalf("Subjects = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Subjects = %v, want %v", got, want)
		}
	}
}
