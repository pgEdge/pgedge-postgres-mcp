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

func TestTakeCodeIsSingleUse(t *testing.T) {
	s := NewStore(DefaultLimits)
	_ = s.PutCode(&AuthCode{Hash: "h", ExpiresAt: time.Now().Add(time.Minute)})
	if _, ok := s.TakeCode("h"); !ok {
		t.Fatal("first take failed")
	}
	if _, ok := s.TakeCode("h"); ok {
		t.Fatal("second take succeeded")
	}
}

func TestStoreLimits(t *testing.T) {
	s := NewStore(Limits{Clients: 1, Codes: 1, DeviceCodes: 1, Tokens: 1})
	_ = s.PutClient(&Client{ID: "a"})
	if err := s.PutClient(&Client{ID: "b"}); !errors.Is(err, ErrStoreFull) {
		t.Fatalf("got %v", err)
	}
}

func TestSweepRemovesExpired(t *testing.T) {
	s := NewStore(DefaultLimits)
	now := time.Now()
	_ = s.PutToken(&Token{Hash: "old", ExpiresAt: now.Add(-time.Second)})
	_ = s.PutToken(&Token{Hash: "new", ExpiresAt: now.Add(time.Hour)})
	s.Sweep(now)
	if _, ok := s.GetToken("old"); ok {
		t.Fatal("expired kept")
	}
	if _, ok := s.GetToken("new"); !ok {
		t.Fatal("live removed")
	}
}

func TestDeleteRefreshCascades(t *testing.T) {
	s := NewStore(DefaultLimits)
	exp := time.Now().Add(time.Hour)
	_ = s.PutToken(&Token{Hash: "r", IsRefresh: true, Issued: []string{"a1"}, ExpiresAt: exp})
	_ = s.PutToken(&Token{Hash: "a1", RefreshHash: "r", ExpiresAt: exp})
	s.DeleteToken("r")
	if _, ok := s.GetToken("a1"); ok {
		t.Fatal("access token survived refresh deletion")
	}
}

func TestGettersReturnCopies(t *testing.T) {
	s := NewStore(DefaultLimits)
	_ = s.PutClient(&Client{ID: "a", RedirectURIs: []string{"x"}})
	c, _ := s.GetClient("a")
	c.RedirectURIs[0] = "y"
	c2, _ := s.GetClient("a")
	if c2.RedirectURIs[0] != "x" {
		t.Fatal("store mutated through getter")
	}
}
