package test

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
func StartMCPServer(t *testing.T, connString string, apiKey string) (*MCPServer, error) {
	// Find the binary
	binaryPath := filepath.Join("..", "bin", "pgedge-mcp")

	// Check if binary exists, if not try to build it
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Logf("Binary not found at %s, building...", binaryPath)
		buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd/pgedge-mcp")
		buildCmd.Dir = filepath.Dir(binaryPath)
		if output, err := buildCmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("failed to build binary: %v\nOutput: %s", err, output)
		}
	}

	cmd := exec.Command(binaryPath)
	cmd.Env = append(os.Environ(),
		"POSTGRES_CONNECTION_STRING="+connString,
		"ANTHROPIC_API_KEY="+apiKey,
	)

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
	s.stdin.Close()

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
	connString := os.Getenv("TEST_POSTGRES_CONNECTION_STRING")
	if connString == "" {
		connString = "postgres://localhost/postgres?sslmode=disable"
		t.Logf("TEST_POSTGRES_CONNECTION_STRING not set, using default: %s", connString)
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
	defer server.Close()

	t.Run("Initialize", func(t *testing.T) {
		testInitialize(t, server)
	})

	t.Run("ListTools", func(t *testing.T) {
		testListTools(t, server)
	})

	t.Run("ListResources", func(t *testing.T) {
		testListResources(t, server)
	})

	t.Run("ReadPgSettingsResource", func(t *testing.T) {
		testReadPgSettingsResource(t, server)
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

	if name, ok := serverInfo["name"].(string); !ok || name != "pgedge-mcp" {
		t.Errorf("Expected server name 'pgedge-mcp', got '%v'", serverInfo["name"])
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

	if len(tools) < 3 {
		t.Errorf("Expected at least 3 tools, got %d", len(tools))
	}

	// Verify expected tools exist
	expectedTools := map[string]bool{
		"query_database":       false,
		"get_schema_info":      false,
		"set_pg_configuration": false,
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

	if len(resources) < 1 {
		t.Errorf("Expected at least 1 resource, got %d", len(resources))
	}

	// Verify pg://settings resource exists
	foundPgSettings := false
	for _, resource := range resources {
		resMap, ok := resource.(map[string]interface{})
		if !ok {
			continue
		}
		if uri, ok := resMap["uri"].(string); ok && uri == "pg://settings" {
			foundPgSettings = true
			break
		}
	}

	if !foundPgSettings {
		t.Error("Expected resource 'pg://settings' not found")
	}

	t.Log("ListResources test passed")
}

func testReadPgSettingsResource(t *testing.T, server *MCPServer) {
	params := map[string]interface{}{
		"uri": "pg://settings",
	}

	// Retry a few times in case metadata is still loading
	var resp *MCPResponse
	var err error
	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		resp, err = server.SendRequest("resources/read", params)
		if err != nil {
			t.Fatalf("resources/read failed: %v", err)
		}

		// If no error, break out
		if resp.Error == nil {
			break
		}

		// If error is "database not ready", retry
		if strings.Contains(resp.Error.Message, "not ready") || strings.Contains(resp.Error.Message, "initializing") {
			if i < maxRetries-1 {
				t.Logf("Database not ready, retrying in 1 second... (attempt %d/%d)", i+1, maxRetries)
				time.Sleep(1 * time.Second)
				continue
			}
		}

		t.Fatalf("resources/read returned error: %s", resp.Error.Message)
	}

	// Parse the result
	var result map[string]interface{}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to parse resources/read result: %v", err)
	}

	// Verify contents
	contents, ok := result["contents"].([]interface{})
	if !ok || len(contents) == 0 {
		t.Fatal("contents array not found or empty in result")
	}

	// Get the first content item
	content, ok := contents[0].(map[string]interface{})
	if !ok {
		t.Fatal("Invalid content format")
	}

	// Verify it's text type
	if contentType, ok := content["type"].(string); !ok || contentType != "text" {
		t.Errorf("Expected content type 'text', got '%v'", content["type"])
	}

	// Verify text is not empty
	text, ok := content["text"].(string)
	if !ok || text == "" {
		t.Error("Content text is empty")
	}

	// The text contains a header followed by JSON array
	// Extract just the JSON portion (starts with '[')
	jsonStartIdx := strings.Index(text, "[")
	if jsonStartIdx == -1 {
		t.Error("JSON array not found in text")
	}

	jsonText := text[jsonStartIdx:]

	// Verify it's valid JSON (should be a JSON array of settings)
	var settings []interface{}
	if err := json.Unmarshal([]byte(jsonText), &settings); err != nil {
		t.Errorf("Content JSON is not valid: %v", err)
	}

	if len(settings) == 0 {
		t.Error("Settings array is empty")
	}

	// Verify some expected settings exist
	if len(settings) < 100 {
		t.Errorf("Expected at least 100 settings, got %d", len(settings))
	}

	t.Logf("ReadPgSettingsResource test passed, found %d settings", len(settings))
}

func testCallGetSchemaInfo(t *testing.T, server *MCPServer) {
	params := map[string]interface{}{
		"name": "get_schema_info",
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
				text, _ := contentItem["text"].(string)
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

	// Should contain schema header
	if !strings.Contains(text, "Database Schema Information") {
		t.Error("Schema information header not found in response")
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
				text, _ := contentItem["text"].(string)
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
	isError, _ := result["isError"].(bool)
	if isError {
		t.Errorf("Query returned an error response: %s", text)
	}

	t.Log("QueryPostgreSQLVersion test passed - successfully queried PostgreSQL version using natural language")
}
