/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package auth

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
)

// TestMetadataPathsPublicWhenOAuthDisabled covers the finding that the
// two OAuth metadata paths bypass authentication whether or not OAuth
// is switched on, so that a client discovering the server gets the
// mux's JSON 404 rather than a 401, which is what RFC 8414 clients
// expect.
func TestMetadataPathsPublicWhenOAuthDisabled(t *testing.T) {
	v := &Validator{Methods: Methods{APITokens: true, OAuth: false}}

	paths := v.PublicPaths()
	for _, want := range []string{OAuthMetadataPath, OAuthProtectedResourcePath} {
		if !slices.Contains(paths, want) {
			t.Errorf("PublicPaths() = %v, missing %q", paths, want)
		}
	}

	for _, path := range []string{OAuthMetadataPath, OAuthProtectedResourcePath} {
		reached := false
		h := AuthMiddleware(v, true)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			reached = true
			w.WriteHeader(http.StatusNotFound)
		}))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if !reached {
			t.Errorf("%s did not pass the middleware (status %d)", path, rec.Code)
		}
	}
}
