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

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
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

// tlsDSNParams are the connection-string parameters that control how the
// client validates the server's certificate. pgxpool.ParseConfig consumes
// all four of these into a *tls.Config: the original strings, including a
// verify-full/verify-ca sslmode and any sslcert/sslkey/sslrootcert file
// paths, do not survive that parse. StartMCPServer needs the original
// strings, not the parsed result, because it forwards them to the server
// under test as PGSSL* environment variables rather than using pgx's parsed
// config directly.
var tlsDSNParams = []string{"sslmode", "sslcert", "sslkey", "sslrootcert"}

// extractTLSParams reads the parameters in tlsDSNParams directly out of a
// libpq connection string, in whichever of the two forms it takes: a
// postgres:// or postgresql:// URI, with the parameters as query values, or
// space-separated key=value pairs. Returns nil if none are present.
func extractTLSParams(connString string) map[string]string {
	trimmed := strings.TrimSpace(connString)
	if strings.HasPrefix(trimmed, "postgres://") || strings.HasPrefix(trimmed, "postgresql://") {
		u, err := url.Parse(trimmed)
		if err != nil {
			return nil
		}
		query := u.Query()
		params := make(map[string]string)
		for _, key := range tlsDSNParams {
			if v := query.Get(key); v != "" {
				params[key] = v
			}
		}
		return params
	}

	// Keyword/value form: space-separated key=value pairs, value optionally
	// wrapped in single quotes with backslash escapes. This covers the same
	// syntax libpq itself accepts; a value can only contain an unescaped
	// space by being quoted, so splitting on runs of whitespace outside
	// quotes is enough for the four keys this function looks for.
	params := make(map[string]string)
	for _, field := range splitDSNFields(trimmed) {
		key, value, ok := strings.Cut(field, "=")
		if !ok {
			continue
		}
		value = strings.Trim(value, `'`)
		value = strings.NewReplacer(`\'`, `'`, `\\`, `\`).Replace(value)
		for _, wanted := range tlsDSNParams {
			if strings.EqualFold(key, wanted) && value != "" {
				params[wanted] = value
			}
		}
	}
	if len(params) == 0 {
		return nil
	}
	return params
}

// splitDSNFields splits a keyword/value libpq connection string into its
// key=value fields, treating single-quoted sections as atomic so a quoted
// value containing whitespace is not split apart.
func splitDSNFields(s string) []string {
	var fields []string
	var current strings.Builder
	inQuotes := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '\\' && i+1 < len(s):
			current.WriteByte(c)
			i++
			current.WriteByte(s[i])
		case c == '\'':
			inQuotes = !inQuotes
			current.WriteByte(c)
		case c == ' ' && !inQuotes:
			if current.Len() > 0 {
				fields = append(fields, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(c)
		}
	}
	if current.Len() > 0 {
		fields = append(fields, current.String())
	}
	return fields
}

func TestExtractTLSParams(t *testing.T) {
	tests := []struct {
		name       string
		connString string
		want       map[string]string
	}{
		{
			name:       "URI form, all four params",
			connString: "postgresql://app:secret@example.com:5432/app?sslmode=verify-full&sslcert=/tmp/client.crt&sslkey=/tmp/client.key&sslrootcert=/tmp/root.crt",
			want: map[string]string{
				"sslmode":     "verify-full",
				"sslcert":     "/tmp/client.crt",
				"sslkey":      "/tmp/client.key",
				"sslrootcert": "/tmp/root.crt",
			},
		},
		{
			name:       "URI form, no TLS params",
			connString: "postgresql://app:secret@example.com:5432/app",
			want:       nil,
		},
		{
			name:       "URI form, sslmode only",
			connString: "postgres://app@example.com/app?sslmode=require",
			want:       map[string]string{"sslmode": "require"},
		},
		{
			name:       "keyword/value form, all four params",
			connString: "host=example.com port=5432 dbname=app user=app sslmode=verify-ca sslcert=/tmp/c.crt sslkey=/tmp/c.key sslrootcert=/tmp/root.crt",
			want: map[string]string{
				"sslmode":     "verify-ca",
				"sslcert":     "/tmp/c.crt",
				"sslkey":      "/tmp/c.key",
				"sslrootcert": "/tmp/root.crt",
			},
		},
		{
			name:       "keyword/value form, quoted value with space",
			connString: `host=example.com sslmode='verify-full' sslcert='/tmp/has space.crt'`,
			want: map[string]string{
				"sslmode": "verify-full",
				"sslcert": "/tmp/has space.crt",
			},
		},
		{
			name:       "keyword/value form, no TLS params",
			connString: "host=example.com port=5432 dbname=app",
			want:       nil,
		},
		{
			name:       "empty string",
			connString: "",
			want:       nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTLSParams(tt.connString)
			if len(got) != len(tt.want) {
				t.Fatalf("extractTLSParams(%q) = %v, want %v", tt.connString, got, tt.want)
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("extractTLSParams(%q)[%q] = %q, want %q", tt.connString, k, got[k], v)
				}
			}
		})
	}
}

// ensureServerBinary returns the path to the server binary, building it
// first if it is not already present.
func ensureServerBinary(t *testing.T) (string, error) {
	binaryPath := filepath.Join("..", "bin", "pgedge-postgres-mcp")

	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Logf("Binary not found at %s, building...", binaryPath)
		buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd/pgedge-pg-mcp-svr")
		buildCmd.Dir = filepath.Dir(binaryPath)
		if output, err := buildCmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("failed to build binary: %w\nOutput: %s", err, output)
		}
	}

	return binaryPath, nil
}

// buildServerEnv returns the environment for the server process: the test's
// own environment, the LLM API key, and, if connString parses, the PG*
// variables the server reads to connect at startup.
func buildServerEnv(t *testing.T, connString, apiKey string) []string {
	env := append(os.Environ(),
		"PGEDGE_ANTHROPIC_API_KEY="+apiKey,
	)
	if connString == "" {
		return env
	}

	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		t.Logf("Warning: Failed to parse connection string: %v", err)
		return env
	}

	env = append(env, pgConnEnvVars(config.ConnConfig)...)
	// pgxpool.ParseConfig turns sslmode/sslcert/sslkey/sslrootcert into a
	// *tls.Config and discards the original strings, so they are read back
	// from connString directly rather than from the parsed config above.
	// Without this, a verify-full or client-certificate DSN would start the
	// server on the config default (sslmode: prefer) instead of the
	// caller's setting.
	for key, value := range extractTLSParams(connString) {
		env = append(env, "PG"+strings.ToUpper(key)+"="+value)
	}
	t.Logf("Setting database connection via PG* environment variables from connection string")

	return env
}

// pgConnEnvVars returns the PGHOST/PGPORT/PGDATABASE/PGUSER/PGPASSWORD
// entries for whichever of those fields config actually has set.
func pgConnEnvVars(config *pgx.ConnConfig) []string {
	var env []string
	if config.Host != "" {
		env = append(env, "PGHOST="+config.Host)
	}
	if config.Port != 0 {
		env = append(env, fmt.Sprintf("PGPORT=%d", config.Port))
	}
	if config.Database != "" {
		env = append(env, "PGDATABASE="+config.Database)
	}
	if config.User != "" {
		env = append(env, "PGUSER="+config.User)
	}
	if config.Password != "" {
		env = append(env, "PGPASSWORD="+config.Password)
	}
	return env
}

// StartMCPServer starts the MCP server binary for testing. Any extraArgs are
// appended to the command line after the default "-http=false", so callers
// can pass flags such as "-config <path>" to exercise non-default settings
// (for example, pinning the connection pool size).
func StartMCPServer(t *testing.T, connString, apiKey string, extraArgs ...string) (*MCPServer, error) {
	binaryPath, err := ensureServerBinary(t)
	if err != nil {
		return nil, err
	}

	// Force stdio mode even if there's a config file with HTTP enabled
	args := append([]string{"-http=false"}, extraArgs...)
	cmd := exec.Command(binaryPath, args...)
	cmd.Env = buildServerEnv(t, connString, apiKey)

	return startServerProcess(t, cmd)
}

// startServerProcess wires up cmd's stdio pipes, starts it, and waits long
// enough for the server to be ready to receive requests.
func startServerProcess(t *testing.T, cmd *exec.Cmd) (*MCPServer, error) {
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

// writeSingleConnPoolConfig writes a minimal server config file that pins the
// default database's connection pool to exactly one connection, and returns
// its path. Host, port, database, user, and password are deliberately left
// at the same sentinel values applyEnvironmentVariables() checks for
// (internal/config/config.go), so the PGHOST/PGPORT/PGDATABASE/PGUSER/
// PGPASSWORD variables set by StartMCPServer from connString still take
// effect; only pool_max_conns is actually forced by this file.
//
// This is used by tests that need to guarantee two successive requests are
// served by the same pooled connection, such as verifying that session-level
// tampering (e.g. RESET ALL) does not leak into a later request.
func writeSingleConnPoolConfig(t *testing.T) string {
	t.Helper()

	const configYAML = `
databases:
  - name: default
    host: localhost
    port: 5432
    database: postgres
    sslmode: prefer
    pool_max_conns: 1
    pool_min_conns: 0
    pool_max_conn_idle_time: 30m
`

	configPath := filepath.Join(t.TempDir(), "single-conn-pool.yaml")
	if err := os.WriteFile(configPath, []byte(configYAML), 0o600); err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}
	return configPath
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

	// With database connected at startup, all 7 tools should be available
	if len(tools) != 7 {
		t.Errorf("Expected exactly 7 tools with database connection, got %d", len(tools))
	}

	// Verify expected tools exist
	expectedTools := map[string]bool{
		"query_database":     false,
		"get_schema_info":    false,
		"similarity_search":  false,
		"read_resource":      false,
		"generate_embedding": false,
		"execute_explain":    false,
		"count_rows":         false,
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

	// Verify expected resources exist
	expectedResources := map[string]bool{
		"pg://system_info": false,
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

// TestReadOnlyStatementSmuggling verifies that a caller cannot escape
// read-only mode by appending statements to a query_database request.
//
// Each payload below was, at some point, a working bypass. They rely on
// pgx falling back to the PostgreSQL simple query protocol when Exec is
// given no bind parameters: that protocol accepts several
// semicolon-separated statements in one message, so a caller could set a
// read-write access mode, or commit the server's own read-only
// transaction, before issuing a write. The final two payloads instead
// attack the pooled connection's session state, so that a later request
// inherits a writable session.
//
// This test requires only a database connection: query_database takes SQL
// directly, so no LLM provider is involved.
func TestReadOnlyStatementSmuggling(t *testing.T) {
	connString := os.Getenv("TEST_PGEDGE_POSTGRES_CONNECTION_STRING")
	if connString == "" {
		t.Skip("TEST_PGEDGE_POSTGRES_CONNECTION_STRING not set")
	}

	// Pin the server's connection pool to a single connection. Two of the
	// subtests below (session characteristics / RESET ALL) send a
	// session-tampering request followed by a probe request and assert the
	// probe was unaffected; that assertion only proves anything if both
	// requests are guaranteed to be served by the very same pooled
	// connection whose session state was tampered with.
	configPath := writeSingleConnPoolConfig(t)

	server, err := StartMCPServer(t, connString, "dummy-key-for-testing", "-config", configPath)
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() { _ = server.Close() }()

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Table names this test may create if a bypass succeeds. They are
	// dropped before and after so a stale table cannot mask a failure.
	targets := []string{
		"ro_bypass_set_transaction",
		"ro_bypass_commit_begin",
		"ro_bypass_leading_comment",
		"ro_bypass_session_characteristics",
		"ro_bypass_reset_all",
	}
	dropTargets := func() {
		for _, name := range targets {
			_, _ = pool.Exec(ctx, "DROP TABLE IF EXISTS "+name) //nolint:errcheck // best-effort cleanup
		}
	}
	dropTargets()
	defer dropTargets()

	// Control: confirm the configured role really can create a table when
	// nothing is stopping it, so that a passing test means the guardrails
	// worked rather than that the role was powerless all along.
	if _, err := pool.Exec(ctx, "CREATE TABLE ro_bypass_control (i int)"); err != nil {
		t.Skipf("configured role cannot create tables, so this test proves nothing: %v", err)
	}
	_, _ = pool.Exec(ctx, "DROP TABLE IF EXISTS ro_bypass_control") //nolint:errcheck // best-effort cleanup

	runQuery := func(t *testing.T, sql string) {
		t.Helper()
		params := map[string]interface{}{
			"name": "query_database",
			"arguments": map[string]interface{}{
				"query": sql,
			},
		}
		if _, err := server.SendRequest("tools/call", params); err != nil {
			t.Fatalf("Failed to send request: %v", err)
		}
		// The response is deliberately not asserted on. A rejection and a
		// database error are both acceptable outcomes; what matters is the
		// out-of-band check that no write took place.
	}

	// sessionReadOnlySetting reads default_transaction_read_only directly, on
	// the same pooled connection, via query_database, as a more precise
	// confirmation than the CREATE TABLE probe below that the session
	// actually reads "on". It does not, on its own, isolate the AfterRelease
	// hook: both RESET ALL and SET SESSION CHARACTERISTICS are now rejected
	// by the statement guard before either ever reaches the database (see
	// readonly_guard.go), so this GUC never actually gets poisoned by these
	// two subtests in the first place, regardless of whether AfterRelease
	// works. AfterRelease is tested directly, independent of the guard and
	// of query_database's own per-transaction pgx.ReadOnly access mode, in
	// TestAfterReleaseRestoresReadOnlyDefault
	// (internal/database/connection_test.go), which poisons a connection
	// through the raw pgx API rather than through any tool.
	sessionReadOnlySetting := func(t *testing.T) string {
		t.Helper()
		params := map[string]interface{}{
			"name": "query_database",
			"arguments": map[string]interface{}{
				"query": "SELECT current_setting('default_transaction_read_only') AS setting",
			},
		}
		resp, err := server.SendRequest("tools/call", params)
		if err != nil {
			t.Fatalf("failed to read default_transaction_read_only: %v", err)
		}
		var result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		}
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("failed to unmarshal query_database response: %v", err)
		}
		if result.IsError || len(result.Content) == 0 {
			t.Fatalf("failed to read default_transaction_read_only: %+v", result)
		}
		// The TSV body is the last line of the response text, following the
		// "setting" header on the line before it.
		lines := strings.Split(strings.TrimSpace(result.Content[0].Text), "\n")
		return lines[len(lines)-1]
	}

	tableExists := func(t *testing.T, name string) bool {
		t.Helper()
		var exists bool
		err := pool.QueryRow(ctx,
			"SELECT EXISTS (SELECT 1 FROM pg_class WHERE relname = $1)",
			name).Scan(&exists)
		if err != nil {
			t.Fatalf("Failed to check for table %s: %v", name, err)
		}
		return exists
	}

	t.Run("SET TRANSACTION READ WRITE then write", func(t *testing.T) {
		runQuery(t, "SET TRANSACTION READ WRITE; CREATE TABLE ro_bypass_set_transaction (i int)")
		if tableExists(t, "ro_bypass_set_transaction") {
			t.Error("read-only bypassed: smuggled CREATE TABLE succeeded")
		}
	})

	t.Run("COMMIT then BEGIN READ WRITE then write", func(t *testing.T) {
		runQuery(t, "COMMIT; BEGIN READ WRITE; CREATE TABLE ro_bypass_commit_begin (i int); COMMIT")
		if tableExists(t, "ro_bypass_commit_begin") {
			t.Error("read-only bypassed: smuggled CREATE TABLE succeeded")
		}
	})

	t.Run("leading comment reaches the smuggling path", func(t *testing.T) {
		// No blocked keyword is needed to reach the vulnerable code path:
		// a leading comment alone stops the statement being recognised as a
		// SELECT, which was enough to route it through Exec.
		runQuery(t, "/* not a select */ SELECT 1; CREATE TABLE ro_bypass_leading_comment (i int)")
		if tableExists(t, "ro_bypass_leading_comment") {
			t.Error("read-only bypassed: smuggled CREATE TABLE succeeded")
		}
	})

	t.Run("session characteristics do not persist to a later request", func(t *testing.T) {
		runQuery(t, "SET SESSION CHARACTERISTICS AS TRANSACTION READ WRITE")
		// A separate request, which may well be served by the same pooled
		// connection.
		runQuery(t, "CREATE TABLE ro_bypass_session_characteristics (i int)")
		if tableExists(t, "ro_bypass_session_characteristics") {
			t.Error("read-only bypassed: session was left writable for a later request")
		}
		// The check above passes even if AfterRelease never restores the
		// session default: query_database's own BeginTx requests
		// pgx.ReadOnly regardless. Read the GUC directly to test AfterRelease
		// itself, independent of that per-transaction protection.
		if setting := sessionReadOnlySetting(t); setting != "on" {
			t.Errorf("default_transaction_read_only = %q after release, want \"on\"; "+
				"AfterRelease did not restore the session default", setting)
		}
	})

	t.Run("RESET ALL does not persist to a later request", func(t *testing.T) {
		runQuery(t, "RESET ALL")
		runQuery(t, "CREATE TABLE ro_bypass_reset_all (i int)")
		if tableExists(t, "ro_bypass_reset_all") {
			t.Error("read-only bypassed: session default was cleared for a later request")
		}
		if setting := sessionReadOnlySetting(t); setting != "on" {
			t.Errorf("default_transaction_read_only = %q after release, want \"on\"; "+
				"AfterRelease did not restore the session default", setting)
		}
	})

	// Ordinary read-only work must still function after all of the above.
	t.Run("legitimate queries still work", func(t *testing.T) {
		params := map[string]interface{}{
			"name": "query_database",
			"arguments": map[string]interface{}{
				"query": "SELECT 42 AS answer",
			},
		}
		resp, err := server.SendRequest("tools/call", params)
		if err != nil {
			t.Fatalf("Failed to send request: %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("SELECT should succeed, got error: %v", resp.Error.Message)
		}
		if !strings.Contains(string(resp.Result), "42") {
			t.Errorf("expected the result to contain 42, got: %s", resp.Result)
		}
	})
}
