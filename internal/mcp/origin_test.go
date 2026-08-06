/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package mcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewOriginPolicy_RejectsMalformedEntries(t *testing.T) {
	// A typo in configuration must stop the server at startup. Accepting
	// it and silently matching nothing would leave an operator believing
	// an origin is permitted when every request from it is refused.
	tests := []struct {
		name       string
		configured []string
	}{
		{"empty entry", []string{""}},
		{"whitespace only", []string{"   "}},
		{"no scheme", []string{"example.com"}},
		{"unsupported scheme", []string{"ftp://example.com"}},
		{"file scheme", []string{"file:///etc/passwd"}},
		{"no host", []string{"https://"}},
		{"includes a path", []string{"https://example.com/app"}},
		{"includes a query", []string{"https://example.com?a=1"}},
		{"includes a fragment", []string{"https://example.com#frag"}},
		{"includes credentials", []string{"https://user:pass@example.com"}},
		{"one bad among good", []string{"https://good.example", "nonsense"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewOriginPolicy(tt.configured); err == nil {
				t.Errorf("NewOriginPolicy(%q) succeeded, want an error", tt.configured)
			}
		})
	}
}

func TestOriginPolicy_DefaultAllowsLoopbackOnly(t *testing.T) {
	policy, err := NewOriginPolicy(nil)
	if err != nil {
		t.Fatalf("NewOriginPolicy(nil) failed: %v", err)
	}

	allowed := []string{
		"",                       // no header at all: a non-browser client
		"http://localhost",       // implicit port 80
		"http://localhost:8080",  // the bundled web client's usual port
		"https://localhost:8443", // the same over TLS
		"http://127.0.0.1:3000",  // a development server
		"http://[::1]:8080",      // IPv6 loopback
		"HTTP://LOCALHOST:8080",  // case is not significant
	}
	for _, origin := range allowed {
		if !policy.Allow(origin) {
			t.Errorf("Allow(%q) = false, want true under the default policy", origin)
		}
	}

	refused := []string{
		"https://mcp.example.com",
		"http://evil.example",
		"null",                      // opaque origin: sandboxed iframe or file://
		"http://localhost.evil.com", // suffix trickery, not loopback
		"http://notlocalhost",
		"http://127.0.0.1.evil.com",
		"not a url at all",
	}
	for _, origin := range refused {
		if policy.Allow(origin) {
			t.Errorf("Allow(%q) = true, want false under the default policy", origin)
		}
	}
}

func TestOriginPolicy_ConfiguredListReplacesLoopbackDefault(t *testing.T) {
	// Naming an origin is a deliberate act, so the default stops applying:
	// an operator who lists a production origin has said what this server
	// serves, and silently keeping localhost would widen that.
	policy, err := NewOriginPolicy([]string{"https://mcp.example.com"})
	if err != nil {
		t.Fatalf("NewOriginPolicy failed: %v", err)
	}

	if !policy.Allow("https://mcp.example.com") {
		t.Error("Allow(configured origin) = false, want true")
	}
	// The same origin written with its default port explicit is the same
	// origin, and a browser may send either form.
	if !policy.Allow("https://mcp.example.com:443") {
		t.Error("Allow(configured origin with explicit default port) = false, want true")
	}
	if policy.Allow("http://localhost:8080") {
		t.Error("Allow(loopback) = true once origins are configured, want false")
	}
	if policy.Allow("http://mcp.example.com") {
		t.Error("Allow(http against an https origin) = true, want false: scheme is part of an origin")
	}
	if policy.Allow("https://mcp.example.com:8443") {
		t.Error("Allow(wrong port) = true, want false: port is part of an origin")
	}
	if !policy.Allow("") {
		t.Error("Allow(no header) = false, want true: non-browser clients send none")
	}
}

func TestOriginPolicy_DescribeNamesTheEffectivePolicy(t *testing.T) {
	defaultPolicy, err := NewOriginPolicy(nil)
	if err != nil {
		t.Fatalf("NewOriginPolicy(nil) failed: %v", err)
	}
	if got := defaultPolicy.Describe(); !strings.Contains(got, "loopback") {
		t.Errorf("Describe() = %q, want it to mention loopback", got)
	}

	configured, err := NewOriginPolicy([]string{"https://b.example", "https://a.example"})
	if err != nil {
		t.Fatalf("NewOriginPolicy failed: %v", err)
	}
	got := configured.Describe()
	for _, want := range []string{"a.example", "b.example"} {
		if !strings.Contains(got, want) {
			t.Errorf("Describe() = %q, want it to mention %q", got, want)
		}
	}
	// Sorted, so the startup line does not reshuffle between restarts.
	if strings.Index(got, "a.example") > strings.Index(got, "b.example") {
		t.Errorf("Describe() = %q, want origins in sorted order", got)
	}
}

// newOriginTestHandler builds the middleware over a handler that records
// whether the request reached it, which is what distinguishes a refusal
// from a request that was merely answered with an error further in.
func newOriginTestHandler(t *testing.T, configured []string) (http.Handler, *bool) {
	t.Helper()

	policy, err := NewOriginPolicy(configured)
	if err != nil {
		t.Fatalf("NewOriginPolicy failed: %v", err)
	}

	reached := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	})

	return originValidationMiddleware(policy)(inner), &reached
}

func TestOriginMiddleware_RefusesDisallowedOriginWithJSONRPCError(t *testing.T) {
	handler, reached := newOriginTestHandler(t, nil)

	req := httptest.NewRequest(http.MethodPost, "/mcp/v1", strings.NewReader("{}"))
	req.Header.Set("Origin", "https://evil.example")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if *reached {
		t.Error("the request reached the wrapped handler, want it refused by the middleware")
	}
	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	// The transport specification permits a JSON-RPC error response with
	// no id here, and it is what lets a client tell this refusal apart
	// from an unrelated proxy returning a bare 403.
	var response JSONRPCResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("response body is not JSON: %v (body %q)", err, w.Body.String())
	}
	if response.JSONRPC != "2.0" {
		t.Errorf("jsonrpc = %q, want \"2.0\"", response.JSONRPC)
	}
	if response.ID != nil {
		t.Errorf("id = %v, want null", response.ID)
	}
	if response.Error == nil {
		t.Fatal("response carries no error object")
	}
	if !strings.Contains(response.Error.Message, "evil.example") {
		t.Errorf("error message = %q, want it to name the refused origin", response.Error.Message)
	}
}

func TestOriginMiddleware_AllowsPermittedAndAbsentOrigins(t *testing.T) {
	tests := []struct {
		name   string
		origin string
		set    bool
	}{
		{"no Origin header", "", false},
		{"loopback origin", "http://localhost:8080", true},
		{"IPv6 loopback origin", "http://[::1]:8080", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, reached := newOriginTestHandler(t, nil)

			req := httptest.NewRequest(http.MethodPost, "/mcp/v1", strings.NewReader("{}"))
			if tt.set {
				req.Header.Set("Origin", tt.origin)
			}
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if !*reached {
				t.Error("the request did not reach the wrapped handler, want it allowed")
			}
			if w.Code != http.StatusOK {
				t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
			}
		})
	}
}

func TestOriginMiddleware_CoversEveryRouteNotJustMCP(t *testing.T) {
	// The health endpoint and the LLM proxy routes are as reachable from
	// a hostile page as the MCP endpoint is, so the middleware wraps the
	// whole mux. Non-MCP routes get this server's ordinary JSON error
	// body rather than a JSON-RPC one, since they do not speak JSON-RPC.
	for _, path := range []string{"/health", "/api/llm/models", "/"} {
		t.Run(path, func(t *testing.T) {
			handler, reached := newOriginTestHandler(t, nil)

			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Header.Set("Origin", "https://evil.example")
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if *reached {
				t.Errorf("%s reached the wrapped handler, want it refused", path)
			}
			if w.Code != http.StatusForbidden {
				t.Errorf("%s status = %d, want %d", path, w.Code, http.StatusForbidden)
			}

			var body struct {
				Error string `json:"error"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("%s body is not JSON: %v", path, err)
			}
			if body.Error == "" {
				t.Errorf("%s body carries no error message", path)
			}
		})
	}
}

func TestOriginMiddleware_RebindingCannotPassByMatchingHost(t *testing.T) {
	// The whole point of the control. Under DNS rebinding the attacker
	// owns the hostname the browser resolves to a loopback address, so
	// Host and Origin agree with each other; a check that compared them
	// would admit exactly the request this is meant to stop.
	handler, reached := newOriginTestHandler(t, nil)

	req := httptest.NewRequest(http.MethodPost, "/mcp/v1", strings.NewReader("{}"))
	req.Host = "rebound.attacker.example"
	req.Header.Set("Origin", "http://rebound.attacker.example")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if *reached {
		t.Error("a rebinding request whose Host matches its Origin was allowed through")
	}
	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestBuildHandler_RejectsInvalidConfiguredOrigins(t *testing.T) {
	// A malformed origin must stop the server rather than degrade to a
	// policy nobody chose.
	server := NewServer(&mockToolProvider{})
	_, err := server.buildHandler(&HTTPConfig{AllowedOrigins: []string{"not-an-origin"}})
	if err == nil {
		t.Fatal("buildHandler succeeded with a malformed origin, want an error")
	}
	if !strings.Contains(err.Error(), "origin") {
		t.Errorf("error = %q, want it to mention the origin configuration", err)
	}
}

func TestBuildHandler_OriginCheckRunsBeforeAuthentication(t *testing.T) {
	// A hostile page must be turned away before any credential handling,
	// so the refusal is a 403 about the origin rather than a 401 that
	// would invite a credentialed retry.
	server := NewServer(&mockToolProvider{})
	handler, err := server.buildHandler(&HTTPConfig{AuthEnabled: true})
	if err != nil {
		t.Fatalf("buildHandler failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/mcp/v1", strings.NewReader("{}"))
	req.Header.Set("Origin", "https://evil.example")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d (the origin check must precede auth)", w.Code, http.StatusForbidden)
	}
}
