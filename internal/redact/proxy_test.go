/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package redact_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pgEdge/pgedge-go-llm-lib/llm"
	_ "github.com/pgEdge/pgedge-go-llm-lib/llm/all"
	"github.com/pgEdge/pgedge-go-llm-lib/llm/proxy"

	"pgedge-postgres-mcp/internal/redact"
)

// testKey mimics the format of an OpenAI project key. It is invented, and is
// not a credential.
const testKey = "sk-proj-TESTAAAABBBBCCCCDDDDEEEEFFFFGGGGHHHH"

// fakeProviderRejectingKey stands in for the upstream provider. It answers 401
// with the body shape OpenAI actually returns, which quotes the key it was
// given back at the caller.
func fakeProviderRejectingKey(t *testing.T) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		body := map[string]any{
			"error": map[string]string{
				"message": "Incorrect API key provided: " + testKey +
					". You can find your API key at https://platform.openai.com/account/api-keys.",
				"type": "invalid_request_error",
			},
		}
		//nolint:errcheck // test double
		json.NewEncoder(w).Encode(body)
	}))
}

// newProxyHandler builds the same proxy this server mounts, pointed at the fake
// provider, so the library's real error handling is exercised.
func newProxyHandler(baseURL string) http.Handler {
	p := proxy.New(proxy.Config{
		DefaultProvider: "openai",
		Providers: map[string]llm.Options{
			"openai": {
				APIKey:  testKey,
				Model:   "gpt-4o-mini",
				BaseURL: baseURL,
			},
		},
	})
	return p.Handler()
}

func chatRequest(t *testing.T) *http.Request {
	t.Helper()

	body, err := json.Marshal(proxy.ChatRequest{
		Messages: []llm.Message{{
			Role:    llm.RoleUser,
			Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "hello"}},
		}},
	})
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/chat", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	return req
}

// TestProxyLeaksKeyWithoutRedaction documents the defect this package exists to
// contain. It asserts the current behaviour of the dependency: the provider's
// message, key and all, is relayed into the response body verbatim.
//
// If this test starts failing, the library has been fixed, which is the outcome
// we want. Read the failure as the cue to reconsider whether the filter is
// still needed, not as a regression.
func TestProxyLeaksKeyWithoutRedaction(t *testing.T) {
	upstream := fakeProviderRejectingKey(t)
	defer upstream.Close()

	rec := httptest.NewRecorder()
	newProxyHandler(upstream.URL).ServeHTTP(rec, chatRequest(t))

	body := rec.Body.String()
	if !strings.Contains(body, testKey) {
		t.Skipf("the library no longer relays the provider's message verbatim; "+
			"review whether redact.Handler is still required. Body: %s", body)
	}
}

// TestProxyWrappedIsRedacted is the test that matters: the proxy as this server
// actually mounts it, wrapped in redact.Handler, must not disclose the key.
func TestProxyWrappedIsRedacted(t *testing.T) {
	redact.Reset()
	defer redact.Reset()
	redact.Register(testKey)

	upstream := fakeProviderRejectingKey(t)
	defer upstream.Close()

	rec := httptest.NewRecorder()
	redact.Handler(newProxyHandler(upstream.URL)).ServeHTTP(rec, chatRequest(t))

	body := rec.Body.String()

	if strings.Contains(body, testKey) {
		t.Errorf("response disclosed the API key: %s", body)
	}
	// The distinctive middle of the key must be gone, not merely the whole
	// string: a provider quoting a truncated form is the usual case.
	if strings.Contains(body, "TESTAAAABBBB") {
		t.Errorf("response disclosed part of the API key: %s", body)
	}
	if !strings.Contains(body, redact.Placeholder) {
		t.Errorf("expected a placeholder in the body: %s", body)
	}

	// The response must remain useful and well formed. The library reports an
	// upstream authentication failure as 502, not as the provider's own 401,
	// so the status is asserted as the library defines it rather than as the
	// provider returned it.
	if rec.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadGateway)
	}
	var parsed struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("body is not valid JSON: %v (%s)", err, body)
	}
	if parsed.Error == "" {
		t.Error("expected an error message to survive redaction")
	}
	if !strings.Contains(parsed.Error, "401") {
		t.Errorf("expected the upstream status to survive: %q", parsed.Error)
	}
}

// TestProxyStreamWrappedIsRedacted covers the streaming endpoint, where the
// error arrives after the response has begun and the wrapper must neither
// disclose the key nor break the stream.
func TestProxyStreamWrappedIsRedacted(t *testing.T) {
	redact.Reset()
	defer redact.Reset()
	redact.Register(testKey)

	upstream := fakeProviderRejectingKey(t)
	defer upstream.Close()

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/stream",
		strings.NewReader(`{"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]}`))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	redact.Handler(newProxyHandler(upstream.URL)).ServeHTTP(rec, req)

	body := rec.Body.String()
	if strings.Contains(body, testKey) || strings.Contains(body, "TESTAAAABBBB") {
		t.Errorf("streaming response disclosed the API key: %s", body)
	}
}
