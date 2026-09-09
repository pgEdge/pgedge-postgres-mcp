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
	s := NewStore(DefaultLimits, testClientRetention)
	_ = s.PutCode(&AuthCode{Hash: "h", ExpiresAt: time.Now().Add(time.Minute)})
	if _, ok := s.TakeCode("h"); !ok {
		t.Fatal("first take failed")
	}
	if _, ok := s.TakeCode("h"); ok {
		t.Fatal("second take succeeded")
	}
}

func TestStoreLimits(t *testing.T) {
	s := NewStore(Limits{Clients: 1, Codes: 1, DeviceCodes: 1, Tokens: 1}, testClientRetention)
	_ = s.PutCode(&AuthCode{Hash: "a", ExpiresAt: time.Now().Add(time.Minute)})
	if err := s.PutCode(&AuthCode{Hash: "b", ExpiresAt: time.Now().Add(time.Minute)}); !errors.Is(err, ErrStoreFull) {
		t.Fatalf("got %v", err)
	}
}

func TestSweepRemovesExpired(t *testing.T) {
	s := NewStore(DefaultLimits, testClientRetention)
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
	s := NewStore(DefaultLimits, testClientRetention)
	exp := time.Now().Add(time.Hour)
	_ = s.PutToken(&Token{Hash: "r", IsRefresh: true, Issued: []string{"a1"}, ExpiresAt: exp})
	_ = s.PutToken(&Token{Hash: "a1", RefreshHash: "r", ExpiresAt: exp})
	s.DeleteToken("r")
	if _, ok := s.GetToken("a1"); ok {
		t.Fatal("access token survived refresh deletion")
	}
}

func TestTakeTokenCascadesAndIsSingleUse(t *testing.T) {
	s := NewStore(DefaultLimits, testClientRetention)
	exp := time.Now().Add(time.Hour)
	_ = s.PutToken(&Token{Hash: "r", IsRefresh: true, Issued: []string{"a1"}, ExpiresAt: exp})
	_ = s.PutToken(&Token{Hash: "a1", RefreshHash: "r", ExpiresAt: exp})

	t1, ok := s.TakeToken("r")
	if !ok || t1.Hash != "r" || !t1.IsRefresh {
		t.Fatalf("first take: %+v, %v", t1, ok)
	}
	if _, ok := s.GetToken("a1"); ok {
		t.Fatal("cascade did not remove the access token")
	}

	if _, ok := s.TakeToken("r"); ok {
		t.Fatal("second take of the same hash should fail")
	}
}

func TestDeleteAccessTokenPrunesParentIssued(t *testing.T) {
	s := NewStore(DefaultLimits, testClientRetention)
	exp := time.Now().Add(time.Hour)
	_ = s.PutToken(&Token{Hash: "r", IsRefresh: true, Issued: []string{"a1", "a2"}, ExpiresAt: exp})
	_ = s.PutToken(&Token{Hash: "a1", RefreshHash: "r", ExpiresAt: exp})
	_ = s.PutToken(&Token{Hash: "a2", RefreshHash: "r", ExpiresAt: exp})

	s.DeleteToken("a1")

	if _, ok := s.GetToken("a1"); ok {
		t.Fatal("deleted access token still present")
	}
	if _, ok := s.GetToken("a2"); !ok {
		t.Fatal("sibling access token wrongly removed")
	}
	r, ok := s.GetToken("r")
	if !ok {
		t.Fatal("refresh token wrongly removed")
	}
	for _, h := range r.Issued {
		if h == "a1" {
			t.Fatal("deleted access token hash still in parent's Issued")
		}
	}
	if len(r.Issued) != 1 || r.Issued[0] != "a2" {
		t.Fatalf("unexpected Issued after prune: %v", r.Issued)
	}
}

func TestPutDeviceCodeEnforcesLimit(t *testing.T) {
	s := NewStore(Limits{Clients: 1, Codes: 1, DeviceCodes: 1, Tokens: 1}, testClientRetention)
	exp := time.Now().Add(time.Hour)
	if err := s.PutDeviceCode(&DeviceCode{DeviceHash: "d1", UserCode: "U1", ExpiresAt: exp}); err != nil {
		t.Fatalf("first put failed: %v", err)
	}
	if err := s.PutDeviceCode(&DeviceCode{DeviceHash: "d2", UserCode: "U2", ExpiresAt: exp}); !errors.Is(err, ErrStoreFull) {
		t.Fatalf("got %v", err)
	}
}

func TestGetDeviceByHashAndUserCode(t *testing.T) {
	s := NewStore(DefaultLimits, testClientRetention)
	exp := time.Now().Add(time.Hour)
	_ = s.PutDeviceCode(&DeviceCode{DeviceHash: "d1", UserCode: "ABCD", ExpiresAt: exp})

	byHash, ok := s.GetDeviceByHash("d1")
	if !ok {
		t.Fatal("GetDeviceByHash: not found")
	}
	byCode, ok := s.GetDeviceByUserCode("ABCD")
	if !ok {
		t.Fatal("GetDeviceByUserCode: not found")
	}
	if byHash.DeviceHash != byCode.DeviceHash || byHash.UserCode != byCode.UserCode {
		t.Fatalf("records differ: %+v vs %+v", byHash, byCode)
	}

	if _, ok := s.GetDeviceByHash("nope"); ok {
		t.Fatal("unknown hash found")
	}
	if _, ok := s.GetDeviceByUserCode("nope"); ok {
		t.Fatal("unknown user code found")
	}
}

func TestUpdateDevice(t *testing.T) {
	s := NewStore(DefaultLimits, testClientRetention)
	exp := time.Now().Add(time.Hour)
	_ = s.PutDeviceCode(&DeviceCode{DeviceHash: "d1", UserCode: "ABCD", ExpiresAt: exp})

	s.UpdateDevice(&DeviceCode{DeviceHash: "d1", UserCode: "ABCD", Approved: true, Subject: "user1", ExpiresAt: exp})

	got, ok := s.GetDeviceByHash("d1")
	if !ok {
		t.Fatal("device disappeared after update")
	}
	if !got.Approved || got.Subject != "user1" {
		t.Fatalf("update not applied: %+v", got)
	}
}

func TestApproveDeviceSingleWriter(t *testing.T) {
	s := NewStore(DefaultLimits, testClientRetention)
	exp := time.Now().Add(time.Hour)
	_ = s.PutDeviceCode(&DeviceCode{DeviceHash: "d1", UserCode: "ABCD", ExpiresAt: exp})

	if ok := s.ApproveDevice("d1", "alice"); !ok {
		t.Fatal("first approval should succeed")
	}
	if ok := s.ApproveDevice("d1", "bob"); ok {
		t.Fatal("second approval should fail")
	}
	got, _ := s.GetDeviceByHash("d1")
	if !got.Approved || got.Subject != "alice" {
		t.Fatalf("approval overwritten: %+v", got)
	}
	if ok := s.ApproveDevice("nope", "alice"); ok {
		t.Fatal("approving an unknown hash should fail")
	}
}

func TestUpdateDeviceRekeysUserCode(t *testing.T) {
	s := NewStore(DefaultLimits, testClientRetention)
	exp := time.Now().Add(time.Hour)
	_ = s.PutDeviceCode(&DeviceCode{DeviceHash: "d1", UserCode: "OLD1", ExpiresAt: exp})

	s.UpdateDevice(&DeviceCode{DeviceHash: "d1", UserCode: "NEW1", ExpiresAt: exp})

	if _, ok := s.GetDeviceByUserCode("OLD1"); ok {
		t.Fatal("stale user code still resolves")
	}
	got, ok := s.GetDeviceByUserCode("NEW1")
	if !ok {
		t.Fatal("new user code does not resolve")
	}
	if got.DeviceHash != "d1" {
		t.Fatalf("unexpected device hash: %s", got.DeviceHash)
	}
}

func TestDeleteDeviceRemovesBothEntries(t *testing.T) {
	s := NewStore(DefaultLimits, testClientRetention)
	exp := time.Now().Add(time.Hour)
	_ = s.PutDeviceCode(&DeviceCode{DeviceHash: "d1", UserCode: "ABCD", ExpiresAt: exp})

	s.DeleteDevice("d1")

	if _, ok := s.GetDeviceByHash("d1"); ok {
		t.Fatal("device hash entry survived DeleteDevice")
	}
	if _, ok := s.GetDeviceByUserCode("ABCD"); ok {
		t.Fatal("user code index entry survived DeleteDevice")
	}
}

func TestSweepRemovesExpiredDeviceUserCodeIndex(t *testing.T) {
	s := NewStore(DefaultLimits, testClientRetention)
	now := time.Now()
	_ = s.PutDeviceCode(&DeviceCode{DeviceHash: "d1", UserCode: "ABCD", ExpiresAt: now.Add(-time.Second)})

	s.Sweep(now)

	if _, ok := s.GetDeviceByHash("d1"); ok {
		t.Fatal("expired device hash entry survived Sweep")
	}
	if _, ok := s.GetDeviceByUserCode("ABCD"); ok {
		t.Fatal("expired device's user code index entry survived Sweep")
	}
}

func TestGettersReturnCopies(t *testing.T) {
	s := NewStore(DefaultLimits, testClientRetention)
	_ = s.PutClient(&Client{ID: "a", RedirectURIs: []string{"x"}})
	c, _ := s.GetClient("a")
	c.RedirectURIs[0] = "y"
	c2, _ := s.GetClient("a")
	if c2.RedirectURIs[0] != "x" {
		t.Fatal("store mutated through getter")
	}
}

func TestMarkAndTakeUsedCode(t *testing.T) {
	s := NewStore(DefaultLimits, testClientRetention)
	until := time.Now().Add(time.Hour)
	s.MarkCodeUsed("codehash", "refreshhash", until)

	refresh, ok := s.TakeUsedCode("codehash")
	if !ok || refresh != "refreshhash" {
		t.Fatalf("got %q, %v", refresh, ok)
	}
	if _, ok := s.TakeUsedCode("codehash"); ok {
		t.Fatal("used code entry should be single use")
	}
}

func TestTakeUsedCodeUnknown(t *testing.T) {
	s := NewStore(DefaultLimits, testClientRetention)
	if _, ok := s.TakeUsedCode("nope"); ok {
		t.Fatal("unknown code hash should not be found")
	}
}

func TestSweepRemovesExpiredUsedCode(t *testing.T) {
	s := NewStore(DefaultLimits, testClientRetention)
	now := time.Now()
	s.MarkCodeUsed("codehash", "refreshhash", now.Add(-time.Second))

	s.Sweep(now)

	if _, ok := s.TakeUsedCode("codehash"); ok {
		t.Fatal("expired used-code entry survived Sweep")
	}
}
