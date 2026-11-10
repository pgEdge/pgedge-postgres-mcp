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
	"net/url"
	"sync"
	"testing"
	"time"
)

// flakyPersister fails every write whilst failing is set, and records the
// token rows deleted and written once it succeeds.
type flakyPersister struct {
	nopPersister
	mu      sync.Mutex
	failing bool
	deleted []string
	puts    int
}

var errFlaky = errors.New("flaky persister: write failed")

func (f *flakyPersister) setFailing(v bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failing = v
}

func (f *flakyPersister) deleteToken(hash string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failing {
		return errFlaky
	}
	f.deleted = append(f.deleted, hash)
	return nil
}

func (f *flakyPersister) putToken(*Token) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failing {
		return errFlaky
	}
	f.puts++
	return nil
}

func (f *flakyPersister) tx(fn func(persister) error) error { return fn(f) }

func (f *flakyPersister) deletedHashes() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.deleted...)
}

func newFlakyStore(t *testing.T) (*Store, *flakyPersister) {
	t.Helper()
	f := &flakyPersister{}
	return newStore(DefaultLimits, time.Hour, f, nil), f
}

func TestStore_FailedDeleteIsRetriedOnNextWrite(t *testing.T) {
	s, f := newFlakyStore(t)
	expires := time.Now().Add(time.Hour)
	mustPutToken(t, s, &Token{Hash: "a", ExpiresAt: expires})
	mustPutToken(t, s, &Token{Hash: "b", ExpiresAt: expires})

	f.setFailing(true)
	if _, err := s.deleteToken("a"); err == nil {
		t.Fatal("deleteToken reported success for a failed write")
	}
	requireToken(t, s, "a", false, "token still usable after a failed delete")

	f.setFailing(false)
	s.DeleteToken("b")
	got := f.deletedHashes()
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("deleted rows = %v, want [a b]", got)
	}
}

func TestStore_FailedDeleteIsRetriedOnce(t *testing.T) {
	s, f := newFlakyStore(t)
	mustPutToken(t, s, &Token{Hash: "a", ExpiresAt: time.Now().Add(time.Hour)})

	f.setFailing(true)
	s.DeleteToken("a")
	f.setFailing(false)
	s.DeleteToken("unknown")
	s.DeleteToken("unknown")
	// Deleting an unknown token writes nothing, so a is the only row.
	if got := f.deletedHashes(); len(got) != 1 || got[0] != "a" {
		t.Fatalf("deleted rows = %v, want [a], retried exactly once", got)
	}
}

func TestStore_FailedUpsertIsNotRetried(t *testing.T) {
	s, f := newFlakyStore(t)
	expires := time.Now().Add(time.Hour)
	mustPutToken(t, s, &Token{Hash: "r", IsRefresh: true, ExpiresAt: expires})
	mustPutToken(t, s, &Token{Hash: "a", RefreshHash: "r", ExpiresAt: expires})
	r := mustGetToken(t, s, "r", "refresh token missing")
	r.Issued = []string{"a"}
	mustPutToken(t, s, r)
	puts := f.puts

	// Deleting the access token rewrites its parent's Issued list.
	f.setFailing(true)
	s.DeleteToken("a")
	f.setFailing(false)
	s.DeleteToken("unknown")
	if f.puts != puts {
		t.Fatalf("parent rewritten %d times on retry, want none", f.puts-puts)
	}
}

func TestStore_RetryQueueIsBounded(t *testing.T) {
	s, f := newFlakyStore(t)
	f.setFailing(true)
	s.mu.Lock()
	for i := 0; i < maxRetryDeletes+5; i++ {
		s.deleteNowLocked(func(p persister) error { return p.deleteToken("x") }, "test")
	}
	n := len(s.retryDeletes)
	s.mu.Unlock()
	if n != maxRetryDeletes {
		t.Fatalf("retry queue holds %d deletions, want %d", n, maxRetryDeletes)
	}
}

func TestRevoke_FailedDeleteIs503(t *testing.T) {
	ts := newTestServer(t, nil)
	cid, code := obtainCode(t, ts, claudeCB)
	_, tr := exchange(ts, codeForm(cid, code, claudeCB))

	f := &flakyPersister{failing: true}
	ts.store.mu.Lock()
	ts.store.p = f
	ts.store.mu.Unlock()

	rec := ts.do("POST", RevokePath, "application/x-www-form-urlencoded",
		url.Values{"token": {tr.RefreshToken}}.Encode())
	if rec.Code != 503 {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Fatal("503 without Retry-After")
	}
	if _, _, ok := ts.srv.ValidateAccessToken(tr.AccessToken); ok {
		t.Fatal("access token still valid in process after revocation")
	}
}

func TestRevoke_RepeatWhilstUnsavedIs503(t *testing.T) {
	ts := newTestServer(t, nil)
	cid, code := obtainCode(t, ts, claudeCB)
	_, tr := exchange(ts, codeForm(cid, code, claudeCB))

	f := &flakyPersister{failing: true}
	ts.store.mu.Lock()
	ts.store.p = f
	ts.store.mu.Unlock()

	body := url.Values{"token": {tr.RefreshToken}}.Encode()
	for i := 0; i < 2; i++ {
		rec := ts.do("POST", RevokePath, "application/x-www-form-urlencoded", body)
		if rec.Code != 503 {
			t.Fatalf("attempt %d: status = %d, want 503", i+1, rec.Code)
		}
	}

	f.setFailing(false)
	rec := ts.do("POST", RevokePath, "application/x-www-form-urlencoded", body)
	if rec.Code != 200 {
		t.Fatalf("after recovery: status = %d, want 200", rec.Code)
	}
}

func TestStore_DeleteBySubjectReportsUnsaved(t *testing.T) {
	s, f := newFlakyStore(t)
	mustPutToken(t, s, &Token{Hash: "a", Subject: "alice", ExpiresAt: time.Now().Add(time.Hour)})

	f.setFailing(true)
	if _, err := s.DeleteBySubject("alice"); err == nil {
		t.Fatal("DeleteBySubject reported success for a failed write")
	}
	if _, err := s.DeleteBySubject("alice"); err == nil {
		t.Fatal("repeat DeleteBySubject reported success whilst the first is unsaved")
	}
	f.setFailing(false)
	if _, err := s.DeleteBySubject("alice"); err != nil {
		t.Fatalf("DeleteBySubject after recovery: %v", err)
	}
}
