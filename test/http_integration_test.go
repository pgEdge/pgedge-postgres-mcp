/*-------------------------------------------------------------------------
 *
 * pgEdge Postgres MCP Server
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package test

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// HTTPMCPServer manages an MCP server running in HTTP/HTTPS mode
type HTTPMCPServer struct {
	cmd        *exec.Cmd
	baseURL    string
	t          *testing.T
	certFile   string
	keyFile    string
	connString string
}

// StartHTTPMCPServer starts the MCP server in HTTP mode for testing
func StartHTTPMCPServer(t *testing.T, connString, apiKey, addr string, useTLS bool) (*HTTPMCPServer, error) {
	// Find the binary
	binaryPath := filepath.Join("..", "bin", "pgedge-pg-mcp-svr")

	// Check if binary exists, if not try to build it
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Logf("Binary not found at %s, building...", binaryPath)
		buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd/pgedge-pg-mcp-svr")
		buildCmd.Dir = filepath.Dir(binaryPath)
		if output, err := buildCmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("failed to build binary: %v\nOutput: %s", err, output)
		}
	}

	server := &HTTPMCPServer{
		t:          t,
		connString: connString,
	}

	// Build command line arguments
	// Disable authentication for testing
	args := []string{"-http", "-addr", addr, "-no-auth"}

	// Generate self-signed certificates for HTTPS testing
	if useTLS {
		certFile, keyFile, err := generateSelfSignedCert(t)
		if err != nil {
			return nil, fmt.Errorf("failed to generate certificates: %w", err)
		}
		server.certFile = certFile
		server.keyFile = keyFile

		args = append(args, "-tls", "-cert", certFile, "-key", keyFile)
		server.baseURL = "https://" + addr
	} else {
		server.baseURL = "http://" + addr
	}

	// Create command
	cmd := exec.Command(binaryPath, args...)
	cmd.Env = append(os.Environ(),
		"PGEDGE_ANTHROPIC_API_KEY="+apiKey,
	)

	// Capture stderr for logging
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	// Start capturing stderr in background
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stderr.Read(buf)
			if n > 0 {
				t.Logf("[HTTP SERVER] %s", string(buf[:n]))
			}
			if err != nil {
				break
			}
		}
	}()

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start server: %w", err)
	}

	server.cmd = cmd

	// Wait for server to be ready
	if err := server.waitForReady(); err != nil {
		_ = server.Close()
		return nil, fmt.Errorf("server failed to become ready: %w", err)
	}

	return server, nil
}

// waitForReady waits for the HTTP server to be ready to accept connections
func (s *HTTPMCPServer) waitForReady() error {
	client := s.getHTTPClient()
	maxAttempts := 30 // 30 attempts * 100ms = 3 seconds
	for i := 0; i < maxAttempts; i++ {
		resp, err := client.Get(s.baseURL + "/health")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				s.t.Log("HTTP server is ready")
				return nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for server to be ready")
}

// getHTTPClient returns an HTTP client configured for TLS if needed
func (s *HTTPMCPServer) getHTTPClient() *http.Client {
	if strings.HasPrefix(s.baseURL, "https") {
		// For self-signed certificates, skip verification
		return &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
			Timeout: 5 * time.Second,
		}
	}
	return &http.Client{Timeout: 5 * time.Second}
}

// SendHTTPRequest sends a JSON-RPC request via HTTP POST
func (s *HTTPMCPServer) SendHTTPRequest(method string, params interface{}) (*MCPResponse, error) {
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

	s.t.Logf("[HTTP CLIENT] Sending to %s: %s", s.baseURL+"/mcp/v1", string(reqJSON))

	client := s.getHTTPClient()
	resp, err := client.Post(s.baseURL+"/mcp/v1", "application/json", bytes.NewReader(reqJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			s.t.Logf("Warning: failed to close response body: %v", err)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	s.t.Logf("[HTTP SERVER] Response: %s", string(body))

	var mcpResp MCPResponse
	if err := json.Unmarshal(body, &mcpResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &mcpResp, nil
}

// Close stops the HTTP MCP server
func (s *HTTPMCPServer) Close() error {
	if s.cmd != nil && s.cmd.Process != nil {
		if err := s.cmd.Process.Kill(); err != nil {
			s.t.Logf("Warning: failed to kill process: %v", err)
		}
		_ = s.cmd.Wait()
	}

	// Clean up certificate files
	if s.certFile != "" {
		_ = os.Remove(s.certFile)
	}
	if s.keyFile != "" {
		_ = os.Remove(s.keyFile)
	}

	return nil
}

// generateSelfSignedCert generates a self-signed certificate for testing
func generateSelfSignedCert(t *testing.T) (certFile, keyFile string, err error) {
	// Generate private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate private key: %w", err)
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test"},
			CommonName:   "localhost",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
	}

	// Create certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to create certificate: %w", err)
	}

	// Write certificate file
	certFile = filepath.Join(os.TempDir(), fmt.Sprintf("test-cert-%d.pem", time.Now().UnixNano()))
	certOut, err := os.Create(certFile)
	if err != nil {
		return "", "", fmt.Errorf("failed to create cert file: %w", err)
	}
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		_ = certOut.Close()
		return "", "", fmt.Errorf("failed to write cert: %w", err)
	}
	if err := certOut.Close(); err != nil {
		return "", "", fmt.Errorf("failed to close cert file: %w", err)
	}

	// Write key file
	keyFile = filepath.Join(os.TempDir(), fmt.Sprintf("test-key-%d.pem", time.Now().UnixNano()))
	keyOut, err := os.Create(keyFile)
	if err != nil {
		_ = os.Remove(certFile)
		return "", "", fmt.Errorf("failed to create key file: %w", err)
	}
	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	if err := pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: privateKeyBytes}); err != nil {
		_ = keyOut.Close()
		_ = os.Remove(certFile)
		return "", "", fmt.Errorf("failed to write key: %w", err)
	}
	if err := keyOut.Close(); err != nil {
		_ = os.Remove(certFile)
		return "", "", fmt.Errorf("failed to close key file: %w", err)
	}

	t.Logf("Generated self-signed certificate: cert=%s, key=%s", certFile, keyFile)

	return certFile, keyFile, nil
}

// TestHTTPModeIntegration tests the HTTP transport mode
func TestHTTPModeIntegration(t *testing.T) {
	connString := os.Getenv("TEST_PGEDGE_POSTGRES_CONNECTION_STRING")
	if connString == "" {
		connString = "postgres://localhost/postgres?sslmode=disable"
		t.Logf("TEST_PGEDGE_POSTGRES_CONNECTION_STRING not set, using default: %s", connString)
	}

	apiKey := os.Getenv("TEST_ANTHROPIC_API_KEY")
	if apiKey == "" {
		apiKey = "dummy-key-for-testing"
		t.Log("TEST_ANTHROPIC_API_KEY not set, using dummy key")
	}

	server, err := StartHTTPMCPServer(t, connString, apiKey, "localhost:18080", false)
	if err != nil {
		t.Fatalf("Failed to start HTTP server: %v", err)
	}
	defer func() { _ = server.Close() }()

	t.Run("HealthCheck", func(t *testing.T) {
		testHTTPHealthCheck(t, server)
	})

	t.Run("Initialize", func(t *testing.T) {
		testHTTPInitialize(t, server)
	})

	t.Run("SetDatabaseConnection", func(t *testing.T) {
		testHTTPSetDatabaseConnection(t, server)
	})

	t.Run("ListTools", func(t *testing.T) {
		testHTTPListTools(t, server)
	})

	t.Run("ListResources", func(t *testing.T) {
		testHTTPListResources(t, server)
	})

	t.Run("CallGetSchemaInfo", func(t *testing.T) {
		testHTTPCallGetSchemaInfo(t, server)
	})

	t.Run("InvalidMethod", func(t *testing.T) {
		testHTTPInvalidMethod(t, server)
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		testHTTPInvalidJSON(t, server)
	})

	t.Run("GETRequestRejected", func(t *testing.T) {
		testHTTPGETRejected(t, server)
	})
}

// TestHTTPSModeIntegration tests the HTTPS transport mode with TLS
func TestHTTPSModeIntegration(t *testing.T) {
	connString := os.Getenv("TEST_PGEDGE_POSTGRES_CONNECTION_STRING")
	if connString == "" {
		connString = "postgres://localhost/postgres?sslmode=disable"
		t.Logf("TEST_PGEDGE_POSTGRES_CONNECTION_STRING not set, using default: %s", connString)
	}

	apiKey := os.Getenv("TEST_ANTHROPIC_API_KEY")
	if apiKey == "" {
		apiKey = "dummy-key-for-testing"
		t.Log("TEST_ANTHROPIC_API_KEY not set, using dummy key")
	}

	server, err := StartHTTPMCPServer(t, connString, apiKey, "localhost:18443", true)
	if err != nil {
		t.Fatalf("Failed to start HTTPS server: %v", err)
	}
	defer func() { _ = server.Close() }()

	t.Run("HealthCheck", func(t *testing.T) {
		testHTTPHealthCheck(t, server)
	})

	t.Run("Initialize", func(t *testing.T) {
		testHTTPInitialize(t, server)
	})

	t.Run("SetDatabaseConnection", func(t *testing.T) {
		testHTTPSetDatabaseConnection(t, server)
	})

	t.Run("ListTools", func(t *testing.T) {
		testHTTPListTools(t, server)
	})

	t.Run("TLSConnection", func(t *testing.T) {
		testHTTPSTLSConnection(t, server)
	})
}

func testHTTPHealthCheck(t *testing.T, server *HTTPMCPServer) {
	client := server.getHTTPClient()
	resp, err := client.Get(server.baseURL + "/health")
	if err != nil {
		t.Fatalf("Health check request failed: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Logf("Warning: failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	bodyStr := string(body)
	if !strings.Contains(bodyStr, "ok") && !strings.Contains(bodyStr, "healthy") {
		t.Logf("Warning: health check response doesn't contain 'ok' or 'healthy': %s", bodyStr)
	}

	t.Logf("Health check passed: %s", bodyStr)
}

func testHTTPInitialize(t *testing.T, server *HTTPMCPServer) {
	params := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
		"clientInfo": map[string]interface{}{
			"name":    "test-http-client",
			"version": "1.0.0",
		},
	}

	resp, err := server.SendHTTPRequest("initialize", params)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("Initialize returned error: %s", resp.Error.Message)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	// Verify protocol version
	if protocolVersion, ok := result["protocolVersion"].(string); !ok || protocolVersion != "2024-11-05" {
		t.Errorf("Expected protocolVersion '2024-11-05', got '%v'", result["protocolVersion"])
	}

	t.Log("HTTP Initialize test passed")
}

func testHTTPSetDatabaseConnection(t *testing.T, server *HTTPMCPServer) {
	params := map[string]interface{}{
		"name": "manage_connections",
		"arguments": map[string]interface{}{
			"operation":         "connect",
			"connection_string": server.connString,
		},
	}

	resp, err := server.SendHTTPRequest("tools/call", params)
	if err != nil {
		t.Fatalf("tools/call (manage_connections connect) failed: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("manage_connections connect returned error: %s", resp.Error.Message)
	}

	// Parse the result
	var result map[string]interface{}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to parse manage_connections result: %v", err)
	}

	// Check for error response in the tool result
	if isError, ok := result["isError"].(bool); ok && isError {
		content := result["content"].([]interface{})
		if len(content) > 0 {
			contentMap := content[0].(map[string]interface{})
			t.Fatalf("manage_connections connect returned error: %s", contentMap["text"])
		}
	}

	// Give the database a moment to fully initialize
	time.Sleep(500 * time.Millisecond)

	t.Log("HTTP SetDatabaseConnection test passed")
}

func testHTTPListTools(t *testing.T, server *HTTPMCPServer) {
	resp, err := server.SendHTTPRequest("tools/list", nil)
	if err != nil {
		t.Fatalf("tools/list failed: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("tools/list returned error: %s", resp.Error.Message)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	tools, ok := result["tools"].([]interface{})
	if !ok {
		t.Fatal("tools array not found in result")
	}

	// After calling manage_connections connect, all 7 tools should be available
	if len(tools) != 7 {
		t.Errorf("Expected exactly 7 tools after database connection, got %d", len(tools))
	}

	t.Logf("HTTP ListTools test passed, found %d tools", len(tools))
}

func testHTTPListResources(t *testing.T, server *HTTPMCPServer) {
	resp, err := server.SendHTTPRequest("resources/list", nil)
	if err != nil {
		t.Fatalf("resources/list failed: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("resources/list returned error: %s", resp.Error.Message)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	resources, ok := result["resources"].([]interface{})
	if !ok {
		t.Fatal("resources array not found in result")
	}

	if len(resources) < 4 {
		t.Errorf("Expected at least 4 resources, got %d", len(resources))
	}

	t.Logf("HTTP ListResources test passed, found %d resources", len(resources))
}

func testHTTPCallGetSchemaInfo(t *testing.T, server *HTTPMCPServer) {
	params := map[string]interface{}{
		"name":      "get_schema_info",
		"arguments": map[string]interface{}{},
	}

	// May need to retry if database is still loading
	maxRetries := 5
	var resp *MCPResponse
	var err error

	for i := 0; i < maxRetries; i++ {
		resp, err = server.SendHTTPRequest("tools/call", params)
		if err != nil {
			t.Fatalf("tools/call failed: %v", err)
		}

		if resp.Error == nil {
			break
		}

		if strings.Contains(resp.Error.Message, "initializing") || strings.Contains(resp.Error.Message, "not ready") {
			if i < maxRetries-1 {
				t.Logf("Database not ready, retrying... (attempt %d/%d)", i+1, maxRetries)
				time.Sleep(1 * time.Second)
				continue
			}
		}

		t.Fatalf("tools/call returned error: %s", resp.Error.Message)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	content, ok := result["content"].([]interface{})
	if !ok || len(content) == 0 {
		t.Fatal("content array not found or empty")
	}

	t.Log("HTTP CallGetSchemaInfo test passed")
}

func testHTTPInvalidMethod(t *testing.T, server *HTTPMCPServer) {
	resp, err := server.SendHTTPRequest("invalid/method", nil)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.Error == nil {
		t.Error("Expected error for invalid method, but got success")
	}

	if resp.Error.Code != -32601 {
		t.Errorf("Expected error code -32601 (Method not found), got %d", resp.Error.Code)
	}

	t.Logf("HTTP InvalidMethod test passed: %s", resp.Error.Message)
}

func testHTTPInvalidJSON(t *testing.T, server *HTTPMCPServer) {
	client := server.getHTTPClient()

	// Send invalid JSON
	resp, err := client.Post(server.baseURL+"/mcp/v1", "application/json", bytes.NewReader([]byte("{invalid json")))
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Logf("Warning: failed to close response body: %v", err)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	var mcpResp MCPResponse
	if err := json.Unmarshal(body, &mcpResp); err != nil {
		t.Fatalf("Failed to unmarshal error response: %v", err)
	}

	if mcpResp.Error == nil {
		t.Error("Expected error for invalid JSON, but got success")
	}

	if mcpResp.Error.Code != -32700 {
		t.Errorf("Expected error code -32700 (Parse error), got %d", mcpResp.Error.Code)
	}

	t.Logf("HTTP InvalidJSON test passed: %s", mcpResp.Error.Message)
}

func testHTTPGETRejected(t *testing.T, server *HTTPMCPServer) {
	client := server.getHTTPClient()

	resp, err := client.Get(server.baseURL + "/mcp/v1")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Logf("Warning: failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405 (Method Not Allowed), got %d", resp.StatusCode)
	}

	t.Log("HTTP GET rejection test passed")
}

func testHTTPSTLSConnection(t *testing.T, server *HTTPMCPServer) {
	// Verify that we're actually using HTTPS
	if !strings.HasPrefix(server.baseURL, "https") {
		t.Fatal("Server should be using HTTPS")
	}

	// Try to connect with a client that verifies certificates (should fail with self-signed)
	strictClient := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: false,
			},
		},
	}

	_, err := strictClient.Get(server.baseURL + "/health")
	if err == nil {
		t.Log("Note: Self-signed certificate was accepted by strict client (may have been added to system trust)")
	} else {
		t.Logf("Strict client correctly rejected self-signed certificate: %v", err)
	}

	// Our test client with InsecureSkipVerify should work
	client := server.getHTTPClient()
	resp, err := client.Get(server.baseURL + "/health")
	if err != nil {
		t.Fatalf("HTTPS connection failed: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Logf("Warning: failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	t.Log("HTTPS TLS connection test passed")
}

// TestHTTPCommandLineFlags tests command line flag validation
func TestHTTPCommandLineFlags(t *testing.T) {
	binaryPath := filepath.Join("..", "bin", "pgedge-pg-mcp-svr")

	// Check if binary exists
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Skip("Binary not found, skipping command line flag tests")
	}

	t.Run("TLSWithoutHTTPFails", func(t *testing.T) {
		// Try to use -tls without -http
		cmd := exec.Command(binaryPath, "-tls")
		cmd.Env = append(os.Environ(),
			"PGEDGE_POSTGRES_CONNECTION_STRING=postgres://localhost/postgres",
			"PGEDGE_ANTHROPIC_API_KEY=dummy",
		)

		output, err := cmd.CombinedOutput()
		if err == nil {
			t.Error("Expected command to fail when using -tls without -http")
		}

		outputStr := string(output)
		if !strings.Contains(outputStr, "require") && !strings.Contains(outputStr, "-http") {
			t.Errorf("Expected error message about requiring -http flag, got: %s", outputStr)
		}

		t.Logf("Correctly rejected -tls without -http: %s", outputStr)
	})

	t.Run("CertWithoutHTTPFails", func(t *testing.T) {
		// Try to use -cert without -http
		cmd := exec.Command(binaryPath, "-cert", "/tmp/cert.pem")
		cmd.Env = append(os.Environ(),
			"PGEDGE_POSTGRES_CONNECTION_STRING=postgres://localhost/postgres",
			"PGEDGE_ANTHROPIC_API_KEY=dummy",
		)

		output, err := cmd.CombinedOutput()
		if err == nil {
			t.Error("Expected command to fail when using -cert without -http")
		}

		outputStr := string(output)
		if !strings.Contains(outputStr, "require") && !strings.Contains(outputStr, "-http") {
			t.Errorf("Expected error message about requiring -http flag, got: %s", outputStr)
		}

		t.Logf("Correctly rejected -cert without -http: %s", outputStr)
	})

	t.Run("HelpOutput", func(t *testing.T) {
		// Check help output includes all flags
		cmd := exec.Command(binaryPath, "-h")
		output, _ := cmd.CombinedOutput()
		outputStr := string(output)

		requiredFlags := []string{"-http", "-addr", "-tls", "-cert", "-key", "-chain"}
		for _, flag := range requiredFlags {
			if !strings.Contains(outputStr, flag) {
				t.Errorf("Help output should contain flag %s", flag)
			}
		}

		t.Log("Help output contains all expected flags")
	})
}
