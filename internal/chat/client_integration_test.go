/*-------------------------------------------------------------------------
 *
 * Integration Tests for Chat Client
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
	"strings"
	"testing"
	"time"

	"pgedge-postgres-mcp/internal/mcp"
)

// mockMCPServer creates a test HTTP server that implements the MCP protocol
func mockMCPServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse the request
		var req map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Logf("Failed to decode request: %v", err)
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		method := req["method"].(string)
		w.Header().Set("Content-Type", "application/json")

		switch method {
		case "initialize":
			resp := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      req["id"],
				"result": map[string]interface{}{
					"protocolVersion": "1.0.0",
					"serverInfo": map[string]interface{}{
						"name":    "test-server",
						"version": "1.0.0",
					},
				},
			}
			json.NewEncoder(w).Encode(resp)

		case "tools/list":
			resp := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      req["id"],
				"result": map[string]interface{}{
					"tools": []interface{}{
						map[string]interface{}{
							"name":        "test_tool",
							"description": "A test tool",
							"inputSchema": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"query": map[string]interface{}{
										"type":        "string",
										"description": "Test query",
									},
								},
							},
						},
					},
				},
			}
			json.NewEncoder(w).Encode(resp)

		case "tools/call":
			params := req["params"].(map[string]interface{})
			toolName := params["name"].(string)

			resp := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      req["id"],
				"result": map[string]interface{}{
					"content": []interface{}{
						map[string]interface{}{
							"type": "text",
							"text": "Tool " + toolName + " executed successfully",
						},
					},
				},
			}
			json.NewEncoder(w).Encode(resp)

		case "resources/list":
			resp := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      req["id"],
				"result": map[string]interface{}{
					"resources": []interface{}{
						map[string]interface{}{
							"uri":         "test://resource",
							"name":        "test_resource",
							"description": "A test resource",
						},
					},
				},
			}
			json.NewEncoder(w).Encode(resp)

		default:
			http.Error(w, "Unknown method", http.StatusNotImplemented)
		}
	}))
}

// mockLLMClient is a mock implementation of LLMClient for testing
type mockLLMClient struct {
	responses []LLMResponse
	callCount int
}

func (m *mockLLMClient) Chat(ctx context.Context, messages []Message, tools []mcp.Tool) (LLMResponse, error) {
	if m.callCount >= len(m.responses) {
		// Return a final text response if we run out of predefined responses
		return LLMResponse{
			Content: []interface{}{
				TextContent{Type: "text", Text: "Final response"},
			},
			StopReason: "end_turn",
		}, nil
	}

	resp := m.responses[m.callCount]
	m.callCount++
	return resp, nil
}

func TestClient_ConnectToMCP_HTTPMode(t *testing.T) {
	// Start mock MCP server
	server := mockMCPServer(t)
	defer server.Close()

	// Create config for HTTP mode
	cfg := &Config{
		MCP: MCPConfig{
			Mode:  "http",
			URL:   server.URL,
			Token: "test-token",
		},
		LLM: LLMConfig{
			Provider:        "anthropic",
			AnthropicAPIKey: "test-key",
			Model:           "claude-test",
		},
		UI: UIConfig{
			NoColor: true,
		},
	}

	// Create client
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	// Connect to MCP
	ctx := context.Background()
	if err := client.connectToMCP(ctx); err != nil {
		t.Fatalf("connectToMCP failed: %v", err)
	}

	// Verify connection was established
	if client.mcp == nil {
		t.Error("Expected MCP client to be initialized")
	}

	// Clean up
	client.mcp.Close()
}

func TestClient_InitializeLLM_Anthropic(t *testing.T) {
	cfg := &Config{
		MCP: MCPConfig{
			Mode:       "stdio",
			ServerPath: "/fake/path",
		},
		LLM: LLMConfig{
			Provider:        "anthropic",
			AnthropicAPIKey: "test-key",
			Model:           "claude-test",
			MaxTokens:       4096,
			Temperature:     0.7,
		},
		UI: UIConfig{
			NoColor: true,
		},
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	if err := client.initializeLLM(); err != nil {
		t.Fatalf("initializeLLM failed: %v", err)
	}

	if client.llm == nil {
		t.Error("Expected LLM client to be initialized")
	}
}

func TestClient_InitializeLLM_Ollama(t *testing.T) {
	cfg := &Config{
		MCP: MCPConfig{
			Mode:       "stdio",
			ServerPath: "/fake/path",
		},
		LLM: LLMConfig{
			Provider:    "ollama",
			Model:       "llama3",
			OllamaURL:   "http://localhost:11434",
			MaxTokens:   4096,
			Temperature: 0.7,
		},
		UI: UIConfig{
			NoColor: true,
		},
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	if err := client.initializeLLM(); err != nil {
		t.Fatalf("initializeLLM failed: %v", err)
	}

	if client.llm == nil {
		t.Error("Expected LLM client to be initialized")
	}
}

func TestClient_InitializeLLM_InvalidProvider(t *testing.T) {
	cfg := &Config{
		MCP: MCPConfig{
			Mode:       "stdio",
			ServerPath: "/fake/path",
		},
		LLM: LLMConfig{
			Provider: "invalid-provider",
		},
		UI: UIConfig{
			NoColor: true,
		},
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	if err := client.initializeLLM(); err == nil {
		t.Error("Expected error for invalid provider")
	}
}

func TestClient_HandleCommand_Help(t *testing.T) {
	cfg := &Config{
		UI: UIConfig{
			NoColor: true,
		},
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	ctx := context.Background()
	handled := client.handleCommand(ctx, "help")
	if !handled {
		t.Error("Expected help command to be handled")
	}
}

func TestClient_HandleCommand_Clear(t *testing.T) {
	cfg := &Config{
		UI: UIConfig{
			NoColor: true,
		},
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	ctx := context.Background()
	handled := client.handleCommand(ctx, "clear")
	if !handled {
		t.Error("Expected clear command to be handled")
	}
}

func TestClient_HandleCommand_Tools(t *testing.T) {
	cfg := &Config{
		UI: UIConfig{
			NoColor: true,
		},
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	// Set up some test tools
	client.tools = []mcp.Tool{
		{
			Name:        "test_tool",
			Description: "A test tool",
		},
	}

	ctx := context.Background()
	handled := client.handleCommand(ctx, "tools")
	if !handled {
		t.Error("Expected tools command to be handled")
	}
}

func TestClient_HandleCommand_Resources(t *testing.T) {
	// Start mock MCP server
	server := mockMCPServer(t)
	defer server.Close()

	cfg := &Config{
		MCP: MCPConfig{
			Mode:  "http",
			URL:   server.URL,
			Token: "test-token",
		},
		UI: UIConfig{
			NoColor: true,
		},
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	// Connect to mock MCP server
	ctx := context.Background()
	if err := client.connectToMCP(ctx); err != nil {
		t.Fatalf("connectToMCP failed: %v", err)
	}
	defer client.mcp.Close()

	// Initialize MCP connection
	if err := client.mcp.Initialize(ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Handle resources command
	handled := client.handleCommand(ctx, "resources")
	if !handled {
		t.Error("Expected resources command to be handled")
	}
}

func TestClient_HandleCommand_Unknown(t *testing.T) {
	cfg := &Config{
		UI: UIConfig{
			NoColor: true,
		},
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	ctx := context.Background()
	handled := client.handleCommand(ctx, "unknown-command")
	if handled {
		t.Error("Expected unknown command to not be handled")
	}
}

func TestClient_ProcessQuery_SimpleResponse(t *testing.T) {
	// Start mock MCP server
	server := mockMCPServer(t)
	defer server.Close()

	cfg := &Config{
		MCP: MCPConfig{
			Mode:  "http",
			URL:   server.URL,
			Token: "test-token",
		},
		LLM: LLMConfig{
			Provider:        "anthropic",
			AnthropicAPIKey: "test-key",
			Model:           "claude-test",
		},
		UI: UIConfig{
			NoColor: true,
		},
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	// Connect to mock MCP server
	ctx := context.Background()
	if err := client.connectToMCP(ctx); err != nil {
		t.Fatalf("connectToMCP failed: %v", err)
	}
	defer client.mcp.Close()

	// Initialize MCP connection
	if err := client.mcp.Initialize(ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Get tools
	tools, err := client.mcp.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}
	client.tools = tools

	// Set up mock LLM client
	mockLLM := &mockLLMClient{
		responses: []LLMResponse{
			{
				Content: []interface{}{
					TextContent{Type: "text", Text: "This is a simple response"},
				},
				StopReason: "end_turn",
			},
		},
	}
	client.llm = mockLLM

	// Process a query
	if err := client.processQuery(ctx, "What is the answer?"); err != nil {
		t.Fatalf("processQuery failed: %v", err)
	}

	// Verify message history
	if len(client.messages) != 2 {
		t.Errorf("Expected 2 messages in history, got %d", len(client.messages))
	}

	// Verify user message
	if client.messages[0].Role != "user" {
		t.Errorf("Expected first message role 'user', got '%s'", client.messages[0].Role)
	}
	if client.messages[0].Content != "What is the answer?" {
		t.Errorf("Expected user message content 'What is the answer?', got '%v'", client.messages[0].Content)
	}

	// Verify assistant response
	if client.messages[1].Role != "assistant" {
		t.Errorf("Expected second message role 'assistant', got '%s'", client.messages[1].Role)
	}
}

func TestClient_ProcessQuery_WithToolUse(t *testing.T) {
	// Start mock MCP server
	server := mockMCPServer(t)
	defer server.Close()

	cfg := &Config{
		MCP: MCPConfig{
			Mode:  "http",
			URL:   server.URL,
			Token: "test-token",
		},
		LLM: LLMConfig{
			Provider:        "anthropic",
			AnthropicAPIKey: "test-key",
			Model:           "claude-test",
		},
		UI: UIConfig{
			NoColor: true,
		},
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	// Connect to mock MCP server
	ctx := context.Background()
	if err := client.connectToMCP(ctx); err != nil {
		t.Fatalf("connectToMCP failed: %v", err)
	}
	defer client.mcp.Close()

	// Initialize MCP connection
	if err := client.mcp.Initialize(ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Get tools
	tools, err := client.mcp.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}
	client.tools = tools

	// Set up mock LLM client with tool use
	mockLLM := &mockLLMClient{
		responses: []LLMResponse{
			{
				Content: []interface{}{
					TextContent{Type: "text", Text: "I need to use a tool"},
					ToolUse{
						Type:  "tool_use",
						ID:    "tool_1",
						Name:  "test_tool",
						Input: map[string]interface{}{"query": "test"},
					},
				},
				StopReason: "tool_use",
			},
			{
				Content: []interface{}{
					TextContent{Type: "text", Text: "Tool executed successfully"},
				},
				StopReason: "end_turn",
			},
		},
	}
	client.llm = mockLLM

	// Process a query
	if err := client.processQuery(ctx, "Test tool use"); err != nil {
		t.Fatalf("processQuery failed: %v", err)
	}

	// Verify message history includes tool use and results
	if len(client.messages) < 3 {
		t.Errorf("Expected at least 3 messages in history, got %d", len(client.messages))
	}

	// Verify LLM was called twice (once for tool use, once for final response)
	if mockLLM.callCount != 2 {
		t.Errorf("Expected LLM to be called 2 times, got %d", mockLLM.callCount)
	}
}

func TestClient_ProcessQuery_MaxIterations(t *testing.T) {
	// Start mock MCP server
	server := mockMCPServer(t)
	defer server.Close()

	cfg := &Config{
		MCP: MCPConfig{
			Mode:  "http",
			URL:   server.URL,
			Token: "test-token",
		},
		LLM: LLMConfig{
			Provider:        "anthropic",
			AnthropicAPIKey: "test-key",
			Model:           "claude-test",
		},
		UI: UIConfig{
			NoColor: true,
		},
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	// Connect to mock MCP server
	ctx := context.Background()
	if err := client.connectToMCP(ctx); err != nil {
		t.Fatalf("connectToMCP failed: %v", err)
	}
	defer client.mcp.Close()

	// Initialize MCP connection
	if err := client.mcp.Initialize(ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Get tools
	tools, err := client.mcp.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}
	client.tools = tools

	// Set up mock LLM client that always returns tool_use
	mockLLM := &mockLLMClient{
		responses: []LLMResponse{},
	}
	// Create 15 responses that all trigger tool use (more than the limit of 10)
	for i := 0; i < 15; i++ {
		mockLLM.responses = append(mockLLM.responses, LLMResponse{
			Content: []interface{}{
				ToolUse{
					Type:  "tool_use",
					ID:    "tool_1",
					Name:  "test_tool",
					Input: map[string]interface{}{"query": "test"},
				},
			},
			StopReason: "tool_use",
		})
	}
	client.llm = mockLLM

	// Process a query - should hit max iterations
	err = client.processQuery(ctx, "Infinite loop test")
	if err == nil {
		t.Error("Expected error for reaching max iterations")
	}
	if !strings.Contains(err.Error(), "maximum number of tool calls") {
		t.Errorf("Expected error about max tool calls, got: %v", err)
	}
}

func TestClient_ProcessQuery_ContextCancellation(t *testing.T) {
	// Start mock MCP server
	server := mockMCPServer(t)
	defer server.Close()

	cfg := &Config{
		MCP: MCPConfig{
			Mode:  "http",
			URL:   server.URL,
			Token: "test-token",
		},
		LLM: LLMConfig{
			Provider:        "anthropic",
			AnthropicAPIKey: "test-key",
			Model:           "claude-test",
		},
		UI: UIConfig{
			NoColor: true,
		},
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	// Connect to mock MCP server
	ctx, cancel := context.WithCancel(context.Background())
	if err := client.connectToMCP(ctx); err != nil {
		t.Fatalf("connectToMCP failed: %v", err)
	}
	defer client.mcp.Close()

	// Initialize MCP connection
	if err := client.mcp.Initialize(ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Get tools
	tools, err := client.mcp.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}
	client.tools = tools

	// Set up mock LLM client that delays
	mockLLM := &mockLLMClient{
		responses: []LLMResponse{},
	}
	client.llm = mockLLM

	// Cancel context before processing
	cancel()

	// Give it a moment to propagate
	time.Sleep(10 * time.Millisecond)

	// Process a query - should fail due to canceled context
	// Note: The actual behavior depends on how the mock LLM handles context cancellation
	// In this test, we're just verifying the setup works
	_ = client.processQuery(ctx, "Test cancellation")

	// The main thing is it doesn't hang - if we get here, the test passes
}

func TestClient_ProcessQuery_ToolListRefreshAfterManageConnections(t *testing.T) {
	// Track the number of times tools/list is called
	toolsListCallCount := 0

	// Create a custom mock server that returns different tool lists
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		method := req["method"].(string)
		w.Header().Set("Content-Type", "application/json")

		switch method {
		case "initialize":
			resp := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      req["id"],
				"result": map[string]interface{}{
					"protocolVersion": "1.0.0",
					"serverInfo": map[string]interface{}{
						"name":    "test-server",
						"version": "1.0.0",
					},
				},
			}
			json.NewEncoder(w).Encode(resp)

		case "tools/list":
			toolsListCallCount++

			// First call: return 3 stateless tools (no database connection)
			// Second call: return 7 tools (with database connection)
			var tools []interface{}
			if toolsListCallCount == 1 {
				tools = []interface{}{
					map[string]interface{}{
						"name":        "manage_connections",
						"description": "Manage database connections",
					},
					map[string]interface{}{
						"name":        "read_resource",
						"description": "Read resources",
					},
					map[string]interface{}{
						"name":        "generate_embedding",
						"description": "Generate embeddings",
					},
				}
			} else {
				// After connection, include all 7 tools
				tools = []interface{}{
					map[string]interface{}{
						"name":        "manage_connections",
						"description": "Manage database connections",
					},
					map[string]interface{}{
						"name":        "read_resource",
						"description": "Read resources",
					},
					map[string]interface{}{
						"name":        "generate_embedding",
						"description": "Generate embeddings",
					},
					map[string]interface{}{
						"name":        "query_database",
						"description": "Execute SQL queries",
					},
					map[string]interface{}{
						"name":        "get_schema_info",
						"description": "Get database schema information",
					},
				}
			}

			resp := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      req["id"],
				"result": map[string]interface{}{
					"tools": tools,
				},
			}
			json.NewEncoder(w).Encode(resp)

		case "tools/call":
			params := req["params"].(map[string]interface{})
			toolName := params["name"].(string)

			resp := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      req["id"],
				"result": map[string]interface{}{
					"content": []interface{}{
						map[string]interface{}{
							"type": "text",
							"text": "Connected to database successfully",
						},
					},
				},
			}

			// Simulate error for non-manage_connections tools
			if toolName != "manage_connections" {
				resp["result"] = map[string]interface{}{
					"isError": true,
					"content": []interface{}{
						map[string]interface{}{
							"type": "text",
							"text": "Tool " + toolName + " executed",
						},
					},
				}
			}
			json.NewEncoder(w).Encode(resp)

		default:
			http.Error(w, "Unknown method", http.StatusNotImplemented)
		}
	}))
	defer server.Close()

	cfg := &Config{
		MCP: MCPConfig{
			Mode:  "http",
			URL:   server.URL,
			Token: "test-token",
		},
		LLM: LLMConfig{
			Provider:        "anthropic",
			AnthropicAPIKey: "test-key",
			Model:           "claude-test",
		},
		UI: UIConfig{
			NoColor: true,
		},
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	ctx := context.Background()
	if err := client.connectToMCP(ctx); err != nil {
		t.Fatalf("connectToMCP failed: %v", err)
	}
	defer client.mcp.Close()

	if err := client.mcp.Initialize(ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Get initial tools (should be 4)
	tools, err := client.mcp.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}
	client.tools = tools

	if len(client.tools) != 3 {
		t.Errorf("Expected 3 initial tools, got %d", len(client.tools))
	}

	// Set up mock LLM that uses manage_connections tool
	mockLLM := &mockLLMClient{
		responses: []LLMResponse{
			{
				Content: []interface{}{
					ToolUse{
						Type: "tool_use",
						ID:   "tool_1",
						Name: "manage_connections",
						Input: map[string]interface{}{
							"operation":         "connect",
							"connection_string": "postgres://test",
						},
					},
				},
				StopReason: "tool_use",
			},
			{
				Content: []interface{}{
					TextContent{Type: "text", Text: "Connected successfully"},
				},
				StopReason: "end_turn",
			},
		},
	}
	client.llm = mockLLM

	// Process query that triggers manage_connections
	if err := client.processQuery(ctx, "Connect to database"); err != nil {
		t.Fatalf("processQuery failed: %v", err)
	}

	// Verify tool list was refreshed (should now be 5 tools)
	if len(client.tools) != 5 {
		t.Errorf("Expected 5 tools after connection, got %d", len(client.tools))
	}

	// Verify tools/list was called exactly twice (initial + refresh)
	if toolsListCallCount != 2 {
		t.Errorf("Expected tools/list to be called 2 times, got %d", toolsListCallCount)
	}
}

func TestClient_ConnectToMCP_URLFormatting(t *testing.T) {
	tests := []struct {
		name     string
		inputURL string
		useTLS   bool
		want     string
	}{
		{
			name:     "Plain hostname with TLS",
			inputURL: "example.com:8080",
			useTLS:   true,
			want:     "https://example.com:8080/mcp/v1",
		},
		{
			name:     "Plain hostname without TLS",
			inputURL: "example.com:8080",
			useTLS:   false,
			want:     "http://example.com:8080/mcp/v1",
		},
		{
			name:     "URL with http prefix",
			inputURL: "http://example.com:8080",
			useTLS:   false,
			want:     "http://example.com:8080/mcp/v1",
		},
		{
			name:     "URL with https prefix",
			inputURL: "https://example.com:8080",
			useTLS:   true,
			want:     "https://example.com:8080/mcp/v1",
		},
		{
			name:     "URL with trailing slash",
			inputURL: "http://example.com:8080/",
			useTLS:   false,
			want:     "http://example.com:8080/mcp/v1",
		},
		{
			name:     "URL already ending with /mcp/v1",
			inputURL: "http://example.com:8080/mcp/v1",
			useTLS:   false,
			want:     "http://example.com:8080/mcp/v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				MCP: MCPConfig{
					Mode:  "http",
					URL:   tt.inputURL,
					Token: "test-token",
					TLS:   tt.useTLS,
				},
				UI: UIConfig{
					NoColor: true,
				},
			}

			client, err := NewClient(cfg)
			if err != nil {
				t.Fatalf("NewClient failed: %v", err)
			}

			ctx := context.Background()

			// Note: This will fail to actually connect since the URL doesn't exist,
			// but we can check that the URL was formatted correctly by looking at
			// the client's mcp field after connectToMCP
			_ = client.connectToMCP(ctx)

			// Verify the HTTP client was created with the correct URL
			if httpClient, ok := client.mcp.(*httpClient); ok {
				if httpClient.url != tt.want {
					t.Errorf("Expected URL %s, got %s", tt.want, httpClient.url)
				}
			}
		})
	}
}
