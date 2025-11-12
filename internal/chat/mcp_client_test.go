/*-------------------------------------------------------------------------
 *
 * Tests for MCP Client
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package chat

import (
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"

    "pgedge-postgres-mcp/internal/mcp"
)

func TestHTTPClient_Initialize(t *testing.T) {
    // Create test server
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Verify request
        var req mcp.JSONRPCRequest
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            t.Fatalf("Failed to decode request: %v", err)
        }

        if req.Method != "initialize" {
            t.Errorf("Expected method 'initialize', got '%s'", req.Method)
        }

        // Send response
        resp := mcp.JSONRPCResponse{
            JSONRPC: "2.0",
            ID:      req.ID,
            Result: mcp.InitializeResult{
                ProtocolVersion: mcp.ProtocolVersion,
                Capabilities: map[string]interface{}{
                    "tools": map[string]interface{}{},
                },
                ServerInfo: mcp.Implementation{
                    Name:    "test-server",
                    Version: "1.0.0",
                },
            },
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(resp)
    }))
    defer server.Close()

    // Create client
    client := NewHTTPClient(server.URL, "test-token")

    // Test initialize
    ctx := context.Background()
    if err := client.Initialize(ctx); err != nil {
        t.Fatalf("Initialize failed: %v", err)
    }
}

func TestHTTPClient_ListTools(t *testing.T) {
    // Create test server
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Verify request
        var req mcp.JSONRPCRequest
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            t.Fatalf("Failed to decode request: %v", err)
        }

        if req.Method != "tools/list" {
            t.Errorf("Expected method 'tools/list', got '%s'", req.Method)
        }

        // Send response
        resp := mcp.JSONRPCResponse{
            JSONRPC: "2.0",
            ID:      req.ID,
            Result: mcp.ToolsListResult{
                Tools: []mcp.Tool{
                    {
                        Name:        "test_tool",
                        Description: "A test tool",
                        InputSchema: mcp.InputSchema{
                            Type:       "object",
                            Properties: map[string]interface{}{},
                        },
                    },
                },
            },
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(resp)
    }))
    defer server.Close()

    // Create client
    client := NewHTTPClient(server.URL, "test-token")

    // Test list tools
    ctx := context.Background()
    tools, err := client.ListTools(ctx)
    if err != nil {
        t.Fatalf("ListTools failed: %v", err)
    }

    if len(tools) != 1 {
        t.Errorf("Expected 1 tool, got %d", len(tools))
    }

    if tools[0].Name != "test_tool" {
        t.Errorf("Expected tool name 'test_tool', got '%s'", tools[0].Name)
    }
}

func TestHTTPClient_CallTool(t *testing.T) {
    // Create test server
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Verify request
        var req mcp.JSONRPCRequest
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            t.Fatalf("Failed to decode request: %v", err)
        }

        if req.Method != "tools/call" {
            t.Errorf("Expected method 'tools/call', got '%s'", req.Method)
        }

        // Send response
        resp := mcp.JSONRPCResponse{
            JSONRPC: "2.0",
            ID:      req.ID,
            Result: mcp.ToolResponse{
                Content: []mcp.ContentItem{
                    {
                        Type: "text",
                        Text: "Tool executed successfully",
                    },
                },
                IsError: false,
            },
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(resp)
    }))
    defer server.Close()

    // Create client
    client := NewHTTPClient(server.URL, "test-token")

    // Test call tool
    ctx := context.Background()
    result, err := client.CallTool(ctx, "test_tool", map[string]interface{}{"arg": "value"})
    if err != nil {
        t.Fatalf("CallTool failed: %v", err)
    }

    if len(result.Content) != 1 {
        t.Errorf("Expected 1 content item, got %d", len(result.Content))
    }

    if result.Content[0].Text != "Tool executed successfully" {
        t.Errorf("Expected text 'Tool executed successfully', got '%s'", result.Content[0].Text)
    }
}

func TestHTTPClient_Authentication(t *testing.T) {
    expectedToken := "test-token-12345"

    // Create test server that checks authentication
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Verify Authorization header
        auth := r.Header.Get("Authorization")
        expectedAuth := "Bearer " + expectedToken
        if auth != expectedAuth {
            t.Errorf("Expected Authorization header '%s', got '%s'", expectedAuth, auth)
            w.WriteHeader(http.StatusUnauthorized)
            return
        }

        // Send dummy response
        resp := mcp.JSONRPCResponse{
            JSONRPC: "2.0",
            ID:      1,
            Result: mcp.ToolsListResult{
                Tools: []mcp.Tool{},
            },
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(resp)
    }))
    defer server.Close()

    // Create client with token
    client := NewHTTPClient(server.URL, expectedToken)

    // Test a request to verify authentication
    ctx := context.Background()
    _, err := client.ListTools(ctx)
    if err != nil {
        t.Fatalf("Request failed: %v", err)
    }
}
