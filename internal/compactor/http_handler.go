/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package compactor

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"

	"pgedge-postgres-mcp/internal/httperror"
)

// maxRequestBodySize is the maximum allowed size for a compaction
// request body (10MB), preventing memory exhaustion from an oversized
// message history.
const maxRequestBodySize = 10 * 1024 * 1024

// HandleCompact is the HTTP handler for the /api/chat/compact endpoint.
func HandleCompact(w http.ResponseWriter, r *http.Request) {
	// Only accept POST requests
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		httperror.Write(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Limit request body size to prevent memory exhaustion attacks
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)

	// Parse request
	var req CompactRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			httperror.Write(w, http.StatusRequestEntityTooLarge, "Request body too large")
			return
		}
		httperror.Write(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}

	// Validate request
	if len(req.Messages) == 0 {
		httperror.Write(w, http.StatusBadRequest, "Messages array cannot be empty")
		return
	}

	// Set defaults for optional fields
	if req.MaxTokens == 0 {
		req.MaxTokens = DefaultMaxTokens
	}
	if req.RecentWindow == 0 {
		req.RecentWindow = DefaultRecentWindow
	}

	// Create compactor and perform compaction
	compactor := NewCompactor(req)
	response := compactor.Compact(req.Messages)

	// Send response
	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	if err := encoder.Encode(response); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: Failed to encode compact response: %v\n", err)
	}
}
