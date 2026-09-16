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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
)

func TestMetadataDocument(t *testing.T) {
	m := buildMetadata("https://mcp.example.com", true)
	if m.AuthorizationEndpoint != "https://mcp.example.com/oauth/authorize" {
		t.Fatal(m.AuthorizationEndpoint)
	}
	if m.RegistrationEndpoint == "" {
		t.Fatal("registration endpoint missing")
	}
	if buildMetadata("https://mcp.example.com", false).RegistrationEndpoint != "" {
		t.Fatal("registration should be omitted")
	}
	if !slices.Contains(m.GrantTypesSupported, DeviceGrantType) {
		t.Fatal("device grant missing")
	}
	if !slices.Equal(m.CodeChallengeMethodsSupported, []string{"S256"}) {
		t.Fatal(m.CodeChallengeMethodsSupported)
	}
}

func TestMetadataEndpointServesJSON(t *testing.T) {
	srv := newTestServer(t, nil) // helper defined in register_test.go below
	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, MetadataPath, nil))
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "application/json" {
		t.Fatal(rec.Code, rec.Header())
	}
	var m Metadata
	_ = json.Unmarshal(rec.Body.Bytes(), &m)
	if m.Issuer != "https://mcp.example.com" {
		t.Fatal(m.Issuer)
	}
	rec = httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, ProtectedResourcePath, nil))
	var p ProtectedResourceMetadata
	_ = json.Unmarshal(rec.Body.Bytes(), &p)
	if len(p.AuthorizationServers) != 1 || p.AuthorizationServers[0] != "https://mcp.example.com" {
		t.Fatal(p)
	}
}
