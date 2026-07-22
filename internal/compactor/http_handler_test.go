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
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func decodeJSONError(t *testing.T, w *httptest.ResponseRecorder) map[string]string {
	t.Helper()
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("response body is not valid JSON: %v (body: %q)", err, w.Body.String())
	}
	if body["error"] == "" {
		t.Error("expected non-empty error message")
	}
	return body
}

func TestHandleCompact_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/chat/compact", nil)
	w := httptest.NewRecorder()

	HandleCompact(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
	if allow := w.Header().Get("Allow"); allow != http.MethodPost {
		t.Errorf("expected Allow header %q, got %q", http.MethodPost, allow)
	}
	decodeJSONError(t, w)
}

func TestHandleCompact_InvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/chat/compact",
		strings.NewReader("{not valid json"))
	w := httptest.NewRecorder()

	HandleCompact(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
	decodeJSONError(t, w)
}

func TestHandleCompact_EmptyMessages(t *testing.T) {
	body, _ := json.Marshal(CompactRequest{Messages: nil})
	req := httptest.NewRequest(http.MethodPost, "/api/chat/compact", bytes.NewReader(body))
	w := httptest.NewRecorder()

	HandleCompact(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
	decodeJSONError(t, w)
}

func TestHandleCompact_OversizedBody(t *testing.T) {
	// Must be syntactically valid JSON so json.Decoder keeps reading
	// (buffering the string token) until it exceeds the body-size cap,
	// rather than failing fast on a syntax error before the cap is hit.
	huge := bytes.Repeat([]byte("x"), maxRequestBodySize+1)
	oversized := append(append([]byte(`{"messages":[{"role":"user","content":"`), huge...), []byte(`"}]}`)...)
	req := httptest.NewRequest(http.MethodPost, "/api/chat/compact", bytes.NewReader(oversized))
	w := httptest.NewRecorder()

	HandleCompact(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected status 413, got %d", w.Code)
	}
	decodeJSONError(t, w)
}

func TestHandleCompact_Success(t *testing.T) {
	req := CompactRequest{
		Messages: []Message{
			{Role: "user", Content: "hello"},
			{Role: "assistant", Content: "hi there"},
		},
	}
	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/api/chat/compact", bytes.NewReader(body))
	w := httptest.NewRecorder()

	HandleCompact(w, httpReq)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d (body: %q)", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
	var resp CompactResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("response body is not valid JSON: %v (body: %q)", err, w.Body.String())
	}
	if len(resp.Messages) == 0 {
		t.Error("expected non-empty compacted messages")
	}
}
