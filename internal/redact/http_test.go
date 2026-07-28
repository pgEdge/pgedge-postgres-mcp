/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package redact

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// leakyErrorBody is the shape the LLM proxy writes when a provider call fails:
// the provider's own message, which on an authentication failure quotes the key.
func leakyErrorBody() string {
	return `{"error":"openai (401): Incorrect API key provided: ` + fakeOpenAIKey + `"}`
}

func TestHandlerRedactsResponseBody(t *testing.T) {
	Reset()

	handler := Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = fmt.Fprint(w, leakyErrorBody())
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/chat", nil))

	body := rec.Body.String()
	if strings.Contains(body, "AAAABBBB") {
		t.Errorf("response body leaked the key: %s", body)
	}
	if !strings.Contains(body, Placeholder) {
		t.Errorf("expected a placeholder in the body: %s", body)
	}

	// The status and the rest of the message must survive, so the client can
	// still tell what went wrong.
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if !strings.Contains(body, "401") {
		t.Errorf("expected the provider status to survive: %s", body)
	}

	// The body must remain valid JSON, since clients parse it.
	var parsed struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
		t.Errorf("redacted body is not valid JSON: %v (%s)", err, body)
	}
}

// TestHandlerReportsCallerLength guards the io.Writer contract. Redaction
// shortens the payload, and a handler told it wrote fewer bytes than it asked
// to write treats that as a short write; json.Encoder reports it as an error.
func TestHandlerReportsCallerLength(t *testing.T) {
	Reset()

	var writeErr error
	var reported int

	handler := Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload := []byte(leakyErrorBody())
		reported, writeErr = w.Write(payload)
		if reported != len(payload) {
			t.Errorf("Write reported %d bytes, want %d", reported, len(payload))
		}
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/chat", nil))

	if writeErr != nil {
		t.Errorf("unexpected write error: %v", writeErr)
	}
}

// TestHandlerEncoderRoundTrip exercises the path the proxy actually uses,
// which is a json.Encoder writing to the response.
func TestHandlerEncoderRoundTrip(t *testing.T) {
	Reset()

	handler := Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := json.NewEncoder(w).Encode(map[string]string{
			"error": "openai (401): Incorrect API key provided: " + fakeOpenAIKey,
		})
		if err != nil {
			t.Errorf("encoder failed against the wrapped writer: %v", err)
		}
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/chat", nil))

	if strings.Contains(rec.Body.String(), "AAAABBBB") {
		t.Errorf("encoded body leaked the key: %s", rec.Body.String())
	}
}

// TestHandlerPreservesStreaming covers server-sent events, which the proxy uses
// for streaming chat. Each event must reach the client as it is produced, so
// the wrapper has to forward Flush.
func TestHandlerPreservesStreaming(t *testing.T) {
	Reset()

	flushes := 0

	handler := Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("wrapped writer must still implement http.Flusher")
		}

		for _, event := range []string{
			"data: {\"delta\":\"hello\"}\n\n",
			"data: {\"error\":\"openai (401): Incorrect API key provided: " + fakeOpenAIKey + "\"}\n\n",
			"data: [DONE]\n\n",
		} {
			if _, err := fmt.Fprint(w, event); err != nil {
				t.Errorf("write failed: %v", err)
			}
			flusher.Flush()
			flushes++
		}
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/chat/stream", nil))

	body := rec.Body.String()
	if strings.Contains(body, "AAAABBBB") {
		t.Errorf("streamed body leaked the key: %s", body)
	}
	if flushes != 3 {
		t.Errorf("flushes = %d, want 3", flushes)
	}
	// The surrounding events must be intact.
	for _, want := range []string{`"delta":"hello"`, "[DONE]"} {
		if !strings.Contains(body, want) {
			t.Errorf("expected %q to survive: %s", want, body)
		}
	}
	if !rec.Flushed {
		t.Error("expected the recorder to have been flushed")
	}
}

// TestHandlerDropsStaleContentLength covers the framing hazard: redaction
// changes the body's length, so a Content-Length set by the wrapped handler
// would describe the unredacted body and truncate the response.
func TestHandlerDropsStaleContentLength(t *testing.T) {
	Reset()

	body := leakyErrorBody()

	handler := Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprint(len(body)))
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = fmt.Fprint(w, body)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/chat", nil))

	if got := rec.Header().Get("Content-Length"); got != "" {
		t.Errorf("Content-Length = %q, want it dropped so the body is not truncated", got)
	}
	if strings.Contains(rec.Body.String(), "AAAABBBB") {
		t.Errorf("response body leaked the key: %s", rec.Body.String())
	}
}

func TestHandlerPassesCleanResponsesThrough(t *testing.T) {
	Reset()

	const clean = `{"content":"the answer is 42","usage":{"input_tokens":10}}`

	handler := Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, clean)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/chat", nil))

	if rec.Body.String() != clean {
		t.Errorf("body = %q, want it unchanged", rec.Body.String())
	}
}

func TestHandlerUnwrap(t *testing.T) {
	Reset()

	handler := Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		unwrapper, ok := w.(interface{ Unwrap() http.ResponseWriter })
		if !ok {
			t.Fatal("wrapped writer should expose Unwrap for http.ResponseController")
		}
		if unwrapper.Unwrap() == nil {
			t.Error("Unwrap returned nil")
		}
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
}
