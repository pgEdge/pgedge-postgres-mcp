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
	"errors"
	"testing"
	"time"
)

// testClientRetention is the client idle retention passed to NewStore by
// the tests that do not care about it, standing in for the refresh token
// lifetime a real server passes.
const testClientRetention = 24 * time.Hour

func TestSweepRemovesIdleClientsButKeepsThoseWithLiveTokens(t *testing.T) {
	s := NewStore(DefaultLimits, time.Hour)
	now := time.Now()
	idle := now.Add(-3 * time.Hour) // older than retention plus grace

	_ = s.PutClient(&Client{ID: "idle", LastUsed: idle})
	_ = s.PutClient(&Client{ID: "busy", LastUsed: idle})
	_ = s.PutClient(&Client{ID: "recent", LastUsed: now})
	_ = s.PutToken(&Token{Hash: "t", ClientID: "busy", ExpiresAt: now.Add(time.Hour)})

	s.Sweep(now)

	if _, ok := s.GetClient("idle"); ok {
		t.Error("idle client with no live tokens survived Sweep")
	}
	if _, ok := s.GetClient("busy"); !ok {
		t.Error("idle client with a live token was swept")
	}
	if _, ok := s.GetClient("recent"); !ok {
		t.Error("recently used client was swept")
	}
}

func TestPutClientEvictsLeastRecentlyUsedWhenFull(t *testing.T) {
	s := NewStore(Limits{Clients: 2, Codes: 1, DeviceCodes: 1, Tokens: 10}, testClientRetention)
	now := time.Now()
	_ = s.PutClient(&Client{ID: "old", LastUsed: now.Add(-time.Hour)})
	_ = s.PutClient(&Client{ID: "newer", LastUsed: now.Add(-time.Minute)})

	if err := s.PutClient(&Client{ID: "fresh", LastUsed: now}); err != nil {
		t.Fatalf("PutClient: %v", err)
	}
	if _, ok := s.GetClient("old"); ok {
		t.Error("least recently used client was not evicted")
	}
	if _, ok := s.GetClient("newer"); !ok {
		t.Error("wrong client evicted")
	}
	if _, ok := s.GetClient("fresh"); !ok {
		t.Error("new client not stored")
	}
}

func TestPutClientFullWhenEveryClientHasLiveTokens(t *testing.T) {
	s := NewStore(Limits{Clients: 1, Codes: 1, DeviceCodes: 1, Tokens: 10}, testClientRetention)
	now := time.Now()
	_ = s.PutClient(&Client{ID: "busy", LastUsed: now})
	_ = s.PutToken(&Token{Hash: "t", ClientID: "busy", ExpiresAt: now.Add(time.Hour)})

	if err := s.PutClient(&Client{ID: "fresh", LastUsed: now}); !errors.Is(err, ErrStoreFull) {
		t.Fatalf("got %v, want ErrStoreFull", err)
	}
}

func TestTouchClientRefreshesLastUsed(t *testing.T) {
	s := NewStore(DefaultLimits, testClientRetention)
	start := time.Now()
	_ = s.PutClient(&Client{ID: "c", LastUsed: start})

	later := start.Add(time.Hour)
	s.TouchClient("c", later)

	c, ok := s.GetClient("c")
	if !ok || !c.LastUsed.Equal(later) {
		t.Fatalf("LastUsed = %v, want %v", c, later)
	}
	s.TouchClient("nope", later) // must neither panic nor create anything
	if _, ok := s.GetClient("nope"); ok {
		t.Error("TouchClient created an unknown client")
	}
}

func TestDeleteFamilyRemovesEveryTokenInIt(t *testing.T) {
	s := NewStore(DefaultLimits, testClientRetention)
	exp := time.Now().Add(time.Hour)
	_ = s.PutToken(&Token{Hash: "r1", Family: "f1", IsRefresh: true, ExpiresAt: exp})
	_ = s.PutToken(&Token{Hash: "a1", Family: "f1", ExpiresAt: exp})
	_ = s.PutToken(&Token{Hash: "r2", Family: "f2", IsRefresh: true, ExpiresAt: exp})

	s.DeleteFamily("f1")

	if _, ok := s.GetToken("r1"); ok {
		t.Error("family refresh token survived")
	}
	if _, ok := s.GetToken("a1"); ok {
		t.Error("family access token survived")
	}
	if _, ok := s.GetToken("r2"); !ok {
		t.Error("unrelated family was deleted")
	}
}

// TestMarkRefreshRotatedEvictsOldestWhenFull covers the finding that a
// full rotation table must give up its soonest-expiring record rather
// than refuse the new one, so that rotating a single token repeatedly
// cannot switch replay detection off for everybody else.
func TestMarkRefreshRotatedEvictsOldestWhenFull(t *testing.T) {
	s := NewStore(Limits{Clients: 10, Codes: 10, DeviceCodes: 10, Tokens: 2}, testClientRetention)
	now := time.Now()

	s.MarkRefreshRotated("oldest", "f1", now.Add(time.Minute))
	s.MarkRefreshRotated("middle", "f2", now.Add(time.Hour))
	s.MarkRefreshRotated("newest", "f3", now.Add(2*time.Hour))

	if _, ok := s.RotatedRefreshFamily("oldest"); ok {
		t.Error("soonest-expiring record was not evicted")
	}
	for _, hash := range []string{"middle", "newest"} {
		if _, ok := s.RotatedRefreshFamily(hash); !ok {
			t.Errorf("record %q was lost", hash)
		}
	}
}

func TestRotatedRefreshLookupAndSweep(t *testing.T) {
	s := NewStore(DefaultLimits, testClientRetention)
	now := time.Now()
	s.MarkRefreshRotated("oldhash", "fam", now.Add(time.Hour))

	if fam, ok := s.RotatedRefreshFamily("oldhash"); !ok || fam != "fam" {
		t.Fatalf("got %q, %v", fam, ok)
	}
	if _, ok := s.RotatedRefreshFamily("unknown"); ok {
		t.Error("unknown hash reported as rotated")
	}

	s.Sweep(now.Add(2 * time.Hour))
	if _, ok := s.RotatedRefreshFamily("oldhash"); ok {
		t.Error("expired rotation record survived Sweep")
	}
}
