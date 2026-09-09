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
)

// Well-known paths served by the authorisation server, and the OAuth
// constants shared between the metadata, authorisation and token
// handlers.
const (
	MetadataPath          = "/.well-known/oauth-authorization-server"
	ProtectedResourcePath = "/.well-known/oauth-protected-resource"
	RegisterPath          = "/oauth/register"
	AuthorizePath         = "/oauth/authorize"
	TokenPath             = "/oauth/token"
	DevicePath            = "/oauth/device"
	DeviceVerifyPath      = "/oauth/device/verify"
	RevokePath            = "/oauth/revoke"
	LogoPath              = "/oauth/static/logo"
	ScopeMCP              = "mcp"
	DeviceGrantType       = "urn:ietf:params:oauth:grant-type:device_code"
)

// PublicPaths lists every path above, for use by middleware that must
// bypass authentication for the authorisation server's own endpoints.
func PublicPaths() []string {
	return []string{
		MetadataPath,
		ProtectedResourcePath,
		RegisterPath,
		AuthorizePath,
		TokenPath,
		DevicePath,
		DeviceVerifyPath,
		RevokePath,
		LogoPath,
	}
}

// Metadata is the RFC 8414 authorisation server metadata document.
type Metadata struct {
	Issuer                            string   `json:"issuer"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint"`
	TokenEndpoint                     string   `json:"token_endpoint"`
	RegistrationEndpoint              string   `json:"registration_endpoint,omitempty"`
	DeviceAuthorizationEndpoint       string   `json:"device_authorization_endpoint"`
	RevocationEndpoint                string   `json:"revocation_endpoint"`
	ResponseTypesSupported            []string `json:"response_types_supported"`
	GrantTypesSupported               []string `json:"grant_types_supported"`
	CodeChallengeMethodsSupported     []string `json:"code_challenge_methods_supported"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported"`
	ScopesSupported                   []string `json:"scopes_supported"`
}

// ProtectedResourceMetadata is the RFC 9728 protected resource metadata
// document, which points a client at the authorisation server that
// protects this resource.
type ProtectedResourceMetadata struct {
	Resource               string   `json:"resource"`
	AuthorizationServers   []string `json:"authorization_servers"`
	BearerMethodsSupported []string `json:"bearer_methods_supported"`
	ScopesSupported        []string `json:"scopes_supported"`
}

// buildMetadata builds the authorisation server metadata document for
// issuer, omitting the registration endpoint when allowRegistration is
// false.
func buildMetadata(issuer string, allowRegistration bool) Metadata {
	m := Metadata{
		Issuer:                            issuer,
		AuthorizationEndpoint:             issuer + AuthorizePath,
		TokenEndpoint:                     issuer + TokenPath,
		DeviceAuthorizationEndpoint:       issuer + DevicePath,
		RevocationEndpoint:                issuer + RevokePath,
		ResponseTypesSupported:            []string{"code"},
		GrantTypesSupported:               []string{"authorization_code", "refresh_token", DeviceGrantType},
		CodeChallengeMethodsSupported:     []string{"S256"},
		TokenEndpointAuthMethodsSupported: []string{"none"},
		ScopesSupported:                   []string{ScopeMCP},
	}
	if allowRegistration {
		m.RegistrationEndpoint = issuer + RegisterPath
	}
	return m
}

// buildProtectedResourceMetadata builds the protected resource metadata
// document for issuer, which also acts as the resource server.
func buildProtectedResourceMetadata(issuer string) ProtectedResourceMetadata {
	return ProtectedResourceMetadata{
		Resource:               issuer,
		AuthorizationServers:   []string{issuer},
		BearerMethodsSupported: []string{"header"},
		ScopesSupported:        []string{ScopeMCP},
	}
}

// handleMetadata serves the authorisation server metadata document.
func (s *Server) handleMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, newError("invalid_request", "method not allowed", http.StatusMethodNotAllowed))
		return
	}
	writeMetadataJSON(w, buildMetadata(s.Issuer(), s.opts.Config.DynamicRegistrationAllowed()))
}

// handleProtectedResourceMetadata serves the protected resource metadata
// document.
func (s *Server) handleProtectedResourceMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, newError("invalid_request", "method not allowed", http.StatusMethodNotAllowed))
		return
	}
	writeMetadataJSON(w, buildProtectedResourceMetadata(s.Issuer()))
}

// writeMetadataJSON writes v as a cacheable JSON metadata document.
func writeMetadataJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_ = json.NewEncoder(w).Encode(v)
}
