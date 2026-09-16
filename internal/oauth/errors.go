/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

// Package oauth implements a minimal OAuth 2.0 authorisation server: this
// file defines the OAuth error type and the helpers that write it back to
// a caller, either as a JSON body or as a redirect.
package oauth

import (
	"encoding/json"
	"net/http"
	"net/url"
)

// Error is an OAuth 2.0 error response, as returned from the token,
// authorisation, device and registration endpoints. Code must be one of
// the values registered by RFC 6749, RFC 8628 or RFC 7591: invalid_request,
// invalid_client, invalid_grant, unauthorized_client,
// unsupported_grant_type, invalid_scope, access_denied, server_error,
// authorization_pending, slow_down, expired_token, invalid_redirect_uri.
type Error struct {
	Code        string
	Description string
	Status      int
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.Description == "" {
		return e.Code
	}
	return e.Code + ": " + e.Description
}

// newError builds an Error with the given code, description and HTTP
// status.
func newError(code, desc string, status int) *Error {
	return &Error{Code: code, Description: desc, Status: status}
}

// jsonErrorBody is the wire format for an OAuth error response body.
type jsonErrorBody struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description,omitempty"`
}

// writeJSONError writes e to w as a JSON OAuth error response, with
// headers that forbid caching of the sensitive response body.
func writeJSONError(w http.ResponseWriter, e *Error) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	w.WriteHeader(e.Status)
	_ = json.NewEncoder(w).Encode(jsonErrorBody{Error: e.Code, ErrorDescription: e.Description})
}

// redirectError reports e to the client by redirecting the browser back
// to redirectURI with the error, error_description and (if non-empty)
// state parameters appended to its query string, as required by the
// authorisation code grant.
func redirectError(w http.ResponseWriter, r *http.Request, redirectURI, state string, e *Error) {
	u, err := url.Parse(redirectURI)
	if err != nil {
		writeJSONError(w, e)
		return
	}
	q := u.Query()
	q.Set("error", e.Code)
	if e.Description != "" {
		q.Set("error_description", e.Description)
	}
	if state != "" {
		q.Set("state", state)
	}
	u.RawQuery = q.Encode()
	http.Redirect(w, r, u.String(), http.StatusFound)
}
