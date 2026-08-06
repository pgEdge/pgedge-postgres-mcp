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
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"

	"pgedge-postgres-mcp/internal/httperror"
)

// The Streamable HTTP transport requires a server to validate the Origin
// header on every incoming connection, and to answer 403 when the header
// is present and invalid, so that a hostile page cannot drive a locally
// running server through a browser that has been made to resolve the
// attacker's own domain to a loopback address (DNS rebinding).
//
// The check deliberately keys on Origin alone and never on Host. Under
// rebinding the attacker controls the hostname the browser connects to,
// so Host carries the attacker's domain just as Origin does; comparing
// the two would find them equal and let the request through. Only a list
// of origins the operator expects can distinguish the two cases.
//
// A request with no Origin header at all is allowed through. The header
// is set by browsers, and most MCP clients are not browsers; rejecting
// its absence would break every command line client without preventing
// anything, since an attacker who can set arbitrary headers is not
// working through a browser and is not subject to this control.

// defaultAllowedOrigins matches an origin served from the loopback
// interface on any port, under either scheme. This is the out-of-the-box
// policy: the bundled web client, and an MCP client with a browser-based
// UI, are reached over localhost during ordinary local use, whereas a
// deployment published under a real hostname is expected to name that
// hostname in configuration.
var defaultLoopbackHosts = map[string]bool{
	"localhost": true,
	"127.0.0.1": true,
	"::1":       true,
}

// OriginPolicy decides whether a request's Origin header is acceptable.
// The zero value is not usable; build one with NewOriginPolicy.
type OriginPolicy struct {
	// allowed holds the operator's configured origins, normalised to
	// scheme://host:port with the scheme's default port made explicit so
	// that "https://example.com" and "https://example.com:443" compare
	// equal.
	allowed map[string]bool

	// loopback reports whether an origin on the loopback interface is
	// accepted regardless of port. True when no origins are configured.
	loopback bool
}

// NewOriginPolicy builds a policy from the configured origin list. An
// empty list yields the default loopback-only policy. Every entry must
// be an absolute http or https origin with no path, query or fragment,
// so that a typo in configuration fails at startup rather than silently
// admitting nothing.
func NewOriginPolicy(configured []string) (*OriginPolicy, error) {
	policy := &OriginPolicy{allowed: make(map[string]bool)}

	if len(configured) == 0 {
		policy.loopback = true
		return policy, nil
	}

	for _, entry := range configured {
		trimmed := strings.TrimSpace(entry)
		if trimmed == "" {
			return nil, fmt.Errorf("allowed_origins contains an empty entry")
		}

		normalised, err := normaliseOrigin(trimmed)
		if err != nil {
			return nil, fmt.Errorf("allowed_origins entry %q: %w", entry, err)
		}
		policy.allowed[normalised] = true
	}

	return policy, nil
}

// normaliseOrigin parses an origin and renders it as scheme://host:port
// with the default port for the scheme filled in. Comparing normalised
// forms means a configured "https://example.com" also matches a browser
// sending "https://example.com:443", which is the same origin.
func normaliseOrigin(origin string) (string, error) {
	parsed, err := url.Parse(origin)
	if err != nil {
		return "", fmt.Errorf("not a valid URL: %w", err)
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("scheme must be http or https, got %q", parsed.Scheme)
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("no host")
	}
	// An origin is a scheme, host and port and nothing else. Anything
	// further is a URL that has been mistaken for one, and silently
	// ignoring the extra part would accept a configuration that does not
	// mean what its author thinks it means.
	if parsed.Path != "" && parsed.Path != "/" {
		return "", fmt.Errorf("must not include a path")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("must not include a query or fragment")
	}
	if parsed.User != nil {
		return "", fmt.Errorf("must not include user information")
	}

	host := strings.ToLower(parsed.Hostname())
	if host == "" {
		return "", fmt.Errorf("no host")
	}

	port := parsed.Port()
	if port == "" {
		if scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}

	return scheme + "://" + net.JoinHostPort(host, port), nil
}

// Allow reports whether an Origin header value is acceptable. An empty
// value means the header was absent, which is allowed; see the package
// comment above for why.
func (p *OriginPolicy) Allow(origin string) bool {
	if origin == "" {
		return true
	}

	// "null" is what a browser sends for an opaque origin: a sandboxed
	// iframe, a document loaded from file://, or a redirect chain that
	// has lost its origin. It names no host, so it can never be matched
	// against an expectation and is refused.
	if origin == "null" {
		return false
	}

	normalised, err := normaliseOrigin(origin)
	if err != nil {
		return false
	}

	if p.allowed[normalised] {
		return true
	}

	if p.loopback {
		parsed, err := url.Parse(origin)
		if err != nil {
			return false
		}
		return defaultLoopbackHosts[strings.ToLower(parsed.Hostname())]
	}

	return false
}

// Describe renders the policy for the startup log, so an operator can
// see which origins the running server will accept.
func (p *OriginPolicy) Describe() string {
	if p.loopback {
		return "loopback origins only (localhost, 127.0.0.1, ::1; any port)"
	}

	origins := make([]string, 0, len(p.allowed))
	for origin := range p.allowed {
		origins = append(origins, origin)
	}
	sort.Strings(origins)
	return strings.Join(origins, ", ")
}

// originValidationMiddleware rejects a request whose Origin header is
// present and not permitted by the policy. It sits ahead of
// authentication so a hostile page is turned away before any credential
// handling, and it wraps the whole mux rather than the MCP handler alone
// so that the health endpoint and the LLM proxy routes are covered too.
func originValidationMiddleware(policy *OriginPolicy) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if policy.Allow(origin) {
				next.ServeHTTP(w, r)
				return
			}

			writeForbiddenOrigin(w, r, origin)
		})
	}
}

// writeForbiddenOrigin answers a rejected request with 403. The MCP
// endpoint gets a JSON-RPC error response with a null id, which the
// transport specification explicitly permits and which lets a client
// distinguish this refusal from an unrelated proxy denying the request;
// every other route gets this server's ordinary JSON error body.
func writeForbiddenOrigin(w http.ResponseWriter, r *http.Request, origin string) {
	message := fmt.Sprintf("Origin %q is not permitted", origin)

	if r.URL.Path != "/mcp/v1" {
		httperror.Write(w, http.StatusForbidden, message)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)

	response := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      nil,
		Error: &RPCError{
			Code:    -32600,
			Message: message,
		},
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to encode response: %v\n", err)
	}
}
