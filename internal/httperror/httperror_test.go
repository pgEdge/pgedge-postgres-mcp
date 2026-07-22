/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package httperror

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestWrite(t *testing.T) {
	w := httptest.NewRecorder()

	Write(w, 418, "I'm a teapot")

	if w.Code != 418 {
		t.Errorf("expected status 418, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}

	var resp Response
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("response body is not valid JSON: %v (body: %q)", err, w.Body.String())
	}
	if resp.Error != "I'm a teapot" {
		t.Errorf("expected error message %q, got %q", "I'm a teapot", resp.Error)
	}
}

func TestWrite_EmptyMessage(t *testing.T) {
	w := httptest.NewRecorder()

	Write(w, 500, "")

	var resp Response
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("response body is not valid JSON: %v (body: %q)", err, w.Body.String())
	}
	if resp.Error != "" {
		t.Errorf("expected empty error message, got %q", resp.Error)
	}
}
