/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package test

// End-to-end coverage of internal/chat's HTTP client against a real
// server, rather than a hand-written mock. Every existing mcp-client
// test (Go and JS) exercises a mock that never implements the server's
// actual Streamable HTTP header/body validation, so none of them would
// have caught a client-side header-encoding bug (see needsEscaping in
// internal/chat/mcp_client.go and encodeMcpNameHeader in
// web/src/lib/mcp-client.js): this test drives chat.NewHTTPClient
// against the real binary started the same way http_integration_test.go
// and mcp_modern_protocol_test.go do, so a header/body mismatch would
// surface here as a real RPC error.

import (
	"context"
	"strings"
	"testing"

	"pgedge-postgres-mcp/internal/chat"
)

func TestClientModernProtocol(t *testing.T) {
	server, err := StartHTTPMCPServer(t, "", "test-key", "localhost:18730", false)
	if err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer func() { _ = server.Close() }()

	client := chat.NewHTTPClient(server.baseURL+"/mcp/v1", "")
	ctx := context.Background()

	if err := client.Initialize(ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	name, version := client.GetServerInfo()
	if name == "" || version == "" {
		t.Errorf("GetServerInfo() = (%q, %q), want non-empty name and version", name, version)
	}

	tools, err := client.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}
	if len(tools) == 0 {
		t.Error("expected at least one tool")
	}

	resources, err := client.ListResources(ctx)
	if err != nil {
		t.Fatalf("ListResources failed: %v", err)
	}
	if len(resources) > 0 {
		// Exercises the Mcp-Name header path for resources/read against
		// the real server -- no existing test does this.
		if _, err := client.ReadResource(ctx, resources[0].URI); err != nil {
			t.Errorf("ReadResource(%q) failed: %v", resources[0].URI, err)
		}
	}

	prompts, err := client.ListPrompts(ctx)
	if err != nil {
		t.Fatalf("ListPrompts failed: %v", err)
	}
	if len(prompts) > 0 {
		// GetPrompt exercises the Mcp-Name header path for prompts/get.
		// Real prompts may require arguments this test doesn't supply,
		// so only fail on a header/protocol-mismatch error -- any other
		// error (e.g. a missing required argument) is not this test's
		// concern.
		if _, err := client.GetPrompt(ctx, prompts[0].Name, map[string]string{}); err != nil {
			msg := err.Error()
			if strings.Contains(msg, "RPC error -32020") || strings.Contains(msg, "HeaderMismatch") ||
				strings.Contains(msg, "RPC error -32022") || strings.Contains(msg, "UnsupportedProtocolVersion") {
				t.Errorf("GetPrompt(%q) failed with a header/protocol-mismatch error: %v", prompts[0].Name, err)
			}
		}
	}
}
