/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

// Package httperror provides a single, consistent JSON error body for all
// REST-style HTTP endpoints, including framework-level cases that bypass
// the normal handlers (unknown route, method not allowed, request
// timeout, oversized body, panic recovery).
package httperror

import (
	"encoding/json"
	"net/http"
)

// Response is the JSON body written for any HTTP error.
type Response struct {
	Error string `json:"error"`
}

// Write sets the JSON content type, writes statusCode, and encodes a
// Response with message as the body.
func Write(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	//nolint:errcheck // Error would only occur if connection is closed
	json.NewEncoder(w).Encode(Response{Error: message})
}
