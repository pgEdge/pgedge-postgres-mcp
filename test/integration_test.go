/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// MCPRequest represents a JSON-RPC request to the MCP server
type MCPRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// MCPResponse represents a JSON-RPC response from the MCP server
type MCPResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *MCPError       `json:"error,omitempty"`
}

// MCPError represents an error response
type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// MCPServer manages a running MCP server process for testing
type MCPServer struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
	reader *bufio.Reader
	t      *testing.T
}

// StartMCPServer starts the MCP server binary for testing
func StartMCPServer(t *testing.T, connString, apiKey string) (*MCPServer, error) {
	// Find the binary
	binaryPath := filepath.Join("..", "bin", "pgedge-postgres-mcp")

	// Check if binary exists, if not try to build it
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Logf("Binary not found at %s, building...", binaryPath)
		buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd/pgedge-pg-mcp-svr")
		buildCmd.Dir = filepath.Dir(binaryPath)
		if output, err := buildCmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("failed to build binary: %w\nOutput: %s", err, output)
		}
	}

	// Force stdio mode even if there's a config file with HTTP enabled
	cmd := exec.Command(binaryPath, "-http=false")

	// Set up environment with database connection from connString
	env := append(os.Environ(),
		"PGEDGE_ANTHROPIC_API_KEY="+apiKey,
	)

	// If connString is provided, parse it and set PG* environment variables
	// The server will use these to connect at startup
	if connString != "" {
		// Use pgxpool to parse the connection string
		config, err := pgxpool.ParseConfig(connString)
		if err == nil {
			if config.ConnConfig.Host != "" {
				env = append(env, "PGHOST="+config.ConnConfig.Host)
			}
			if config.ConnConfig.Port != 0 {
				env = append(env, fmt.Sprintf("PGPORT=%d", config.ConnConfig.Port))
			}
			if config.ConnConfig.Database != "" {
				env = append(env, "PGDATABASE="+config.ConnConfig.Database)
			}
			if config.ConnConfig.User != "" {
				env = append(env, "PGUSER="+config.ConnConfig.User)
			}
			if config.ConnConfig.Password != "" {
				env = append(env, "PGPASSWORD="+config.ConnConfig.Password)
			}
			t.Logf("Setting database connection via PG* environment variables from connection string")
		} else {
			t.Logf("Warning: Failed to parse connection string: %v", err)
		}
	}

	cmd.Env = env

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	// Start capturing stderr in background
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			t.Logf("[SERVER STDERR] %s", scanner.Text())
		}
	}()

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start server: %w", err)
	}

	server := &MCPServer{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
		reader: bufio.NewReader(stdout),
		t:      t,
	}

	// Give the server a moment to start and load metadata
	time.Sleep(500 * time.Millisecond)

	return server, nil
}

// SendRequest sends a JSON-RPC request and returns the response
func (s *MCPServer) SendRequest(method string, params interface{}) (*MCPResponse, error) {
	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  method,
		Params:  params,
	}

	reqJSON, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	s.t.Logf("[CLIENT] Sending: %s", string(reqJSON))

	// Send the request
	if _, err := s.stdin.Write(append(reqJSON, '\n')); err != nil {
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	// Read the response with timeout
	respChan := make(chan string, 1)
	errChan := make(chan error, 1)

	go func() {
		line, err := s.reader.ReadString('\n')
		if err != nil {
			errChan <- err
			return
		}
		respChan <- line
	}()

	select {
	case line := <-respChan:
		s.t.Logf("[SERVER] Response: %s", strings.TrimSpace(line))

		var resp MCPResponse
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			return nil, fmt.Errorf("failed to unmarshal response: %w", err)
		}
		return &resp, nil

	case err := <-errChan:
		return nil, fmt.Errorf("failed to read response: %w", err)

	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("timeout waiting for response")
	}
}

// Close stops the MCP server
func (s *MCPServer) Close() error {
	_ = s.stdin.Close()

	// Give it a moment to shutdown gracefully
	done := make(chan error, 1)
	go func() {
		done <- s.cmd.Wait()
	}()

	select {
	case <-done:
		return nil
	case <-time.After(2 * time.Second):
		s.t.Log("Server didn't shutdown gracefully, killing...")
		return s.cmd.Process.Kill()
	}
}

// TestMCPServerIntegration runs basic integration tests
func TestMCPServerIntegration(t *testing.T) {
	// Skip if no database is available
	connString := os.Getenv("TEST_PGEDGE_POSTGRES_CONNECTION_STRING")
	if connString == "" {
		connString = "postgres://localhost/postgres?sslmode=disable"
		t.Logf("TEST_PGEDGE_POSTGRES_CONNECTION_STRING not set, using default: %s", connString)
	}

	// API key is optional for some tests
	apiKey := os.Getenv("TEST_ANTHROPIC_API_KEY")
	if apiKey == "" {
		apiKey = "dummy-key-for-testing"
		t.Log("TEST_ANTHROPIC_API_KEY not set, using dummy key (some tests may be skipped)")
	}

	server, err := StartMCPServer(t, connString, apiKey)
	if err != nil {
		t.Fatalf("Failed to start MCP server: %v", err)
	}
	defer func() { _ = server.Close() }()

	t.Run("Initialize", func(t *testing.T) {
		testInitialize(t, server)
	})

	t.Run("ListTools", func(t *testing.T) {
		testListTools(t, server)
	})

	t.Run("ListResources", func(t *testing.T) {
		testListResources(t, server)
	})

	t.Run("CallGetSchemaInfo", func(t *testing.T) {
		testCallGetSchemaInfo(t, server)
	})

	t.Run("QueryPostgreSQLVersion", func(t *testing.T) {
		testQueryPostgreSQLVersion(t, server, apiKey)
	})
}

func testInitialize(t *testing.T, server *MCPServer) {
	params := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]interface{}{
			"roots": map[string]interface{}{
				"listChanged": true,
			},
		},
		"clientInfo": map[string]interface{}{
			"name":    "test-client",
			"version": "1.0.0",
		},
	}

	resp, err := server.SendRequest("initialize", params)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("Initialize returned error: %s", resp.Error.Message)
	}

	// Parse the result
	var result map[string]interface{}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to parse initialize result: %v", err)
	}

	// Verify protocol version
	if protocolVersion, ok := result["protocolVersion"].(string); !ok || protocolVersion != "2024-11-05" {
		t.Errorf("Expected protocolVersion '2024-11-05', got '%v'", result["protocolVersion"])
	}

	// Verify server info
	serverInfo, ok := result["serverInfo"].(map[string]interface{})
	if !ok {
		t.Fatal("serverInfo not found in result")
	}

	if name, ok := serverInfo["name"].(string); !ok || name != "pgedge-postgres-mcp" {
		t.Errorf("Expected server name 'pgedge-postgres-mcp', got '%v'", serverInfo["name"])
	}

	// Verify capabilities
	capabilities, ok := result["capabilities"].(map[string]interface{})
	if !ok {
		t.Fatal("capabilities not found in result")
	}

	if tools, ok := capabilities["tools"].(map[string]interface{}); !ok || tools == nil {
		t.Error("tools capability not found")
	}

	if resources, ok := capabilities["resources"].(map[string]interface{}); !ok || resources == nil {
		t.Error("resources capability not found")
	}

	t.Log("Initialize test passed")
}

func testListTools(t *testing.T, server *MCPServer) {
	resp, err := server.SendRequest("tools/list", nil)
	if err != nil {
		t.Fatalf("tools/list failed: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("tools/list returned error: %s", resp.Error.Message)
	}

	// Parse the result
	var result map[string]interface{}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to parse tools/list result: %v", err)
	}

	tools, ok := result["tools"].([]interface{})
	if !ok {
		t.Fatal("tools array not found in result")
	}

	// With database connected at startup, all 6 tools should be available
	if len(tools) != 6 {
		t.Errorf("Expected exactly 6 tools with database connection, got %d", len(tools))
	}

	// Verify expected tools exist
	expectedTools := map[string]bool{
		"query_database":     false,
		"get_schema_info":    false,
		"similarity_search":  false,
		"read_resource":      false,
		"generate_embedding": false,
		"execute_explain":    false,
	}

	for _, tool := range tools {
		toolMap, ok := tool.(map[string]interface{})
		if !ok {
			continue
		}
		if name, ok := toolMap["name"].(string); ok {
			if _, exists := expectedTools[name]; exists {
				expectedTools[name] = true
			}
		}
	}

	for toolName, found := range expectedTools {
		if !found {
			t.Errorf("Expected tool '%s' not found", toolName)
		}
	}

	t.Log("ListTools test passed")
}

func testListResources(t *testing.T, server *MCPServer) {
	resp, err := server.SendRequest("resources/list", nil)
	if err != nil {
		t.Fatalf("resources/list failed: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("resources/list returned error: %s", resp.Error.Message)
	}

	// Parse the result
	var result map[string]interface{}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to parse resources/list result: %v", err)
	}

	resources, ok := result["resources"].([]interface{})
	if !ok {
		t.Fatal("resources array not found in result")
	}

	if len(resources) < 2 {
		t.Errorf("Expected at least 2 resources, got %d", len(resources))
	}

	// Verify expected resources exist
	expectedResources := map[string]bool{
		"pg://system_info":     false,
		"pg://database/schema": false,
	}

	for _, resource := range resources {
		resMap, ok := resource.(map[string]interface{})
		if !ok {
			continue
		}
		if uri, ok := resMap["uri"].(string); ok {
			if _, exists := expectedResources[uri]; exists {
				expectedResources[uri] = true
			}
		}
	}

	for resourceURI, found := range expectedResources {
		if !found {
			t.Errorf("Expected resource '%s' not found", resourceURI)
		}
	}

	t.Log("ListResources test passed")
}

func testCallGetSchemaInfo(t *testing.T, server *MCPServer) {
	params := map[string]interface{}{
		"name":      "get_schema_info",
		"arguments": map[string]interface{}{
			// No schema_name specified, should return all
		},
	}

	// Retry a few times in case metadata is still loading
	var resp *MCPResponse
	var err error
	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		resp, err = server.SendRequest("tools/call", params)
		if err != nil {
			t.Fatalf("tools/call failed: %v", err)
		}

		if resp.Error != nil {
			t.Fatalf("tools/call returned error: %s", resp.Error.Message)
		}

		// Parse the result to check if database is ready
		var result map[string]interface{}
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("Failed to parse tools/call result: %v", err)
		}

		// Check if it's an error about initialization
		content, ok := result["content"].([]interface{})
		if ok && len(content) > 0 {
			contentItem, ok := content[0].(map[string]interface{})
			if ok {
				text, textOk := contentItem["text"].(string)
				if !textOk {
					continue
				}
				if strings.Contains(text, "initializing") || strings.Contains(text, "not ready") {
					if i < maxRetries-1 {
						t.Logf("Database not ready, retrying in 1 second... (attempt %d/%d)", i+1, maxRetries)
						time.Sleep(1 * time.Second)
						continue
					}
				}
			}
		}

		// Either success or not a retry-able error
		break
	}

	// Verify we have a response
	var result map[string]interface{}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to parse tools/call result: %v", err)
	}

	// Verify content
	content, ok := result["content"].([]interface{})
	if !ok || len(content) == 0 {
		t.Fatal("content array not found or empty in result")
	}

	// Get the first content item
	contentItem, ok := content[0].(map[string]interface{})
	if !ok {
		t.Fatal("Invalid content format")
	}

	// Verify it's text type
	if contentType, ok := contentItem["type"].(string); !ok || contentType != "text" {
		t.Errorf("Expected content type 'text', got '%v'", contentItem["type"])
	}

	// Verify text contains schema information
	text, ok := contentItem["text"].(string)
	if !ok || text == "" {
		t.Error("Content text is empty")
	}

	// Should contain schema header or empty database message
	// (depending on whether the test database has tables or not)
	if !strings.Contains(text, "Database Schema Information") &&
		!strings.Contains(text, "No tables found matching your criteria") {
		t.Errorf("Expected schema information or empty database message, got: %s", text)
	}

	t.Log("CallGetSchemaInfo test passed")
}

func testQueryPostgreSQLVersion(t *testing.T, server *MCPServer, apiKey string) {
	// Skip if no real API key is provided
	if apiKey == "" || apiKey == "dummy-key-for-testing" {
		t.Skip("Skipping QueryPostgreSQLVersion test - requires TEST_ANTHROPIC_API_KEY environment variable")
	}

	params := map[string]interface{}{
		"name": "query_database",
		"arguments": map[string]interface{}{
			"query": "What is the PostgreSQL version?",
		},
	}

	// Retry a few times in case metadata is still loading
	var resp *MCPResponse
	var err error
	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		resp, err = server.SendRequest("tools/call", params)
		if err != nil {
			t.Fatalf("tools/call failed: %v", err)
		}

		if resp.Error != nil {
			// Check if it's a temporary error about database not ready
			if strings.Contains(resp.Error.Message, "initializing") || strings.Contains(resp.Error.Message, "not ready") {
				if i < maxRetries-1 {
					t.Logf("Database not ready, retrying in 1 second... (attempt %d/%d)", i+1, maxRetries)
					time.Sleep(1 * time.Second)
					continue
				}
			}
			t.Fatalf("tools/call returned error: %s", resp.Error.Message)
		}

		// Parse the result to check if database is ready
		var result map[string]interface{}
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("Failed to parse tools/call result: %v", err)
		}

		// Check if it's an error about initialization
		content, ok := result["content"].([]interface{})
		if ok && len(content) > 0 {
			contentItem, ok := content[0].(map[string]interface{})
			if ok {
				text, textOk := contentItem["text"].(string)
				if !textOk {
					continue
				}
				if strings.Contains(text, "initializing") || strings.Contains(text, "not ready") {
					if i < maxRetries-1 {
						t.Logf("Database not ready, retrying in 1 second... (attempt %d/%d)", i+1, maxRetries)
						time.Sleep(1 * time.Second)
						continue
					}
				}

				// Check if it's an API key error
				if strings.Contains(text, "ANTHROPIC_API_KEY") {
					t.Skip("Skipping test - ANTHROPIC_API_KEY not configured on server")
				}
			}
		}

		// Either success or not a retry-able error
		break
	}

	// Verify we have a response
	var result map[string]interface{}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to parse tools/call result: %v", err)
	}

	// Verify content
	content, ok := result["content"].([]interface{})
	if !ok || len(content) == 0 {
		t.Fatal("content array not found or empty in result")
	}

	// Get the first content item
	contentItem, ok := content[0].(map[string]interface{})
	if !ok {
		t.Fatal("Invalid content format")
	}

	// Verify it's text type
	if contentType, ok := contentItem["type"].(string); !ok || contentType != "text" {
		t.Errorf("Expected content type 'text', got '%v'", contentItem["type"])
	}

	// Get the response text
	text, ok := contentItem["text"].(string)
	if !ok || text == "" {
		t.Error("Content text is empty")
	}

	t.Logf("Response text: %s", text)

	// Verify the response contains key elements
	// Should contain "Natural Language Query" or the query text
	if !strings.Contains(text, "Natural Language Query") && !strings.Contains(text, "PostgreSQL version") {
		t.Logf("Warning: Response doesn't contain expected query reference: %s", text)
	}

	// Should contain "Generated SQL" or SQL-like content
	if !strings.Contains(text, "Generated SQL") && !strings.Contains(text, "SELECT") {
		t.Error("Response should contain 'Generated SQL' or SQL content")
	}

	// Should contain "Results" or result data
	if !strings.Contains(text, "Results") {
		t.Error("Response should contain 'Results'")
	}

	// Should contain version information
	// PostgreSQL version format is typically like "PostgreSQL 14.5" or "14.5" or just version numbers
	// We'll look for common patterns:
	// 1. The word "postgresql" or "version"
	// 2. Version-like patterns: numbers with dots (e.g., "14.5", "15.2", "16.1")
	// 3. Two or more digits (version numbers are typically multi-digit)

	textLower := strings.ToLower(text)

	// Pattern 1: Contains "postgresql" or "version"
	hasVersionKeyword := strings.Contains(textLower, "postgresql") ||
		strings.Contains(textLower, "version")

	// Pattern 2: Contains version-like number pattern (e.g., "14.5", "15.2")
	// Use a simple check for digits followed by dot followed by digits
	hasVersionPattern := false
	for i := 0; i < len(text)-2; i++ {
		if text[i] >= '0' && text[i] <= '9' {
			if text[i+1] == '.' {
				if i+2 < len(text) && text[i+2] >= '0' && text[i+2] <= '9' {
					hasVersionPattern = true
					break
				}
			}
		}
	}

	// Pattern 3: Contains 2+ consecutive digits (version number)
	hasMultiDigit := false
	digitCount := 0
	for _, char := range text {
		if char >= '0' && char <= '9' {
			digitCount++
			if digitCount >= 2 {
				hasMultiDigit = true
				break
			}
		} else {
			digitCount = 0
		}
	}

	hasVersionInfo := hasVersionKeyword || hasVersionPattern || hasMultiDigit

	if !hasVersionInfo {
		t.Errorf("Response should contain PostgreSQL version information. Got: %s", text)
	}

	// Verify it's not an error response
	isError, ok := result["isError"].(bool)
	if ok && isError {
		t.Errorf("Query returned an error response: %s", text)
	}

	t.Log("QueryPostgreSQLVersion test passed - successfully queried PostgreSQL version using natural language")
}

// TestReadOnlyProtection tests that generated queries are executed in read-only mode
func TestReadOnlyProtection(t *testing.T) {
	connString := os.Getenv("TEST_PGEDGE_POSTGRES_CONNECTION_STRING")
	if connString == "" {
		t.Skip("TEST_PGEDGE_POSTGRES_CONNECTION_STRING not set")
	}

	apiKey := os.Getenv("TEST_ANTHROPIC_API_KEY")
	if apiKey == "" || apiKey == "dummy-key-for-testing" {
		t.Skip("Skipping read-only protection test - requires TEST_ANTHROPIC_API_KEY")
	}

	server, err := StartMCPServer(t, connString, apiKey)
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() { _ = server.Close() }()

	// First, create a test table directly using SQL (not through the MCP server)
	// This bypasses the read-only protection
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Create a test table
	_, err = pool.Exec(ctx, `
		DROP TABLE IF EXISTS read_only_test;
		CREATE TABLE read_only_test (
			id SERIAL PRIMARY KEY,
			test_value TEXT
		);
		INSERT INTO read_only_test (test_value) VALUES ('initial value');
	`)
	if err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}
	defer func() { _, _ = pool.Exec(ctx, "DROP TABLE IF EXISTS read_only_test") }()

	// Wait for server to be ready and load metadata
	time.Sleep(2 * time.Second)

	// Test 1: Verify SELECT queries work (read-only should allow this)
	t.Run("SELECT query succeeds", func(t *testing.T) {
		params := map[string]interface{}{
			"name": "query_database",
			"arguments": map[string]interface{}{
				"query": "Show me all values from read_only_test table",
			},
		}

		resp, err := server.SendRequest("tools/call", params)
		if err != nil {
			t.Fatalf("Failed to send request: %v", err)
		}

		if resp.Error != nil {
			t.Errorf("SELECT query should succeed, but got error: %v", resp.Error.Message)
		}

		// Verify we got results
		if len(resp.Result) == 0 {
			t.Fatal("Expected result but got empty")
		}

		// Unmarshal the Result
		var result map[string]interface{}
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("Failed to unmarshal result: %v", err)
		}

		content, ok := result["content"].([]interface{})
		if !ok || len(content) == 0 {
			t.Fatal("Expected content array in result")
		}

		contentItem, ok := content[0].(map[string]interface{})
		if !ok {
			t.Fatal("Expected content item to be a map")
		}

		text, ok := contentItem["text"].(string)
		if !ok {
			t.Fatal("Expected text field in content item")
		}

		if !strings.Contains(text, "initial value") {
			t.Errorf("Expected query result to contain 'initial value', got: %s", text)
		}

		t.Log("✓ SELECT query succeeded as expected")
	})

	// Test 2: Verify INSERT queries fail due to read-only protection
	t.Run("INSERT query blocked by read-only", func(t *testing.T) {
		params := map[string]interface{}{
			"name": "query_database",
			"arguments": map[string]interface{}{
				"query": "Insert a new row with test_value 'attempted insert' into read_only_test table",
			},
		}

		resp, err := server.SendRequest("tools/call", params)
		if err != nil {
			t.Fatalf("Failed to send request: %v", err)
		}

		// We expect this to fail - either as an error response or in the result
		if len(resp.Result) > 0 {
			var result map[string]interface{}
			if err := json.Unmarshal(resp.Result, &result); err == nil {
				content, ok := result["content"].([]interface{})
				if ok && len(content) > 0 {
					contentItem, ok := content[0].(map[string]interface{})
					if ok {
						text, ok := contentItem["text"].(string)
						if ok {
							// Check if the error message indicates read-only protection
							textLower := strings.ToLower(text)
							if strings.Contains(textLower, "read-only") ||
								strings.Contains(textLower, "cannot execute") ||
								strings.Contains(textLower, "read only") {
								t.Logf("✓ INSERT query correctly blocked by read-only protection: %s", text)
								return
							}
							t.Errorf("Expected read-only error, but got: %s", text)
						}
					}
				}
			}
		}

		if resp.Error == nil {
			t.Error("Expected INSERT query to fail due to read-only protection, but it succeeded")
		} else {
			t.Logf("✓ INSERT query correctly blocked with error: %s", resp.Error.Message)
		}
	})

	// Verify that the INSERT did not actually modify the data
	t.Run("Verify no data modification occurred", func(t *testing.T) {
		var count int
		err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM read_only_test").Scan(&count)
		if err != nil {
			t.Fatalf("Failed to query table: %v", err)
		}

		if count != 1 {
			t.Errorf("Expected exactly 1 row in table, got %d - INSERT may have succeeded", count)
		} else {
			t.Log("✓ Table still contains exactly 1 row - no modification occurred")
		}
	})
}
