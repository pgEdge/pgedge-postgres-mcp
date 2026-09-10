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
	"net/url"
	"testing"
)

func TestRevokeRefreshCascades(t *testing.T) {
	ts := newTestServer(t, nil)
	cid, code := obtainCode(t, ts, claudeCB)
	_, tr := exchange(ts, codeForm(cid, code, claudeCB))
	rec := ts.do("POST", RevokePath, "application/x-www-form-urlencoded", url.Values{"token": {tr.RefreshToken}}.Encode())
	if rec.Code != 200 {
		t.Fatal(rec.Code)
	}
	if _, _, ok := ts.srv.ValidateAccessToken(tr.AccessToken); ok {
		t.Fatal("access token survived")
	}
}

func TestRevokeUnknownIs200(t *testing.T) {
	ts := newTestServer(t, nil)
	if rec := ts.do("POST", RevokePath, "application/x-www-form-urlencoded", "token=nothing"); rec.Code != 200 {
		t.Fatal(rec.Code)
	}
}
