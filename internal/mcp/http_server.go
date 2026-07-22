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
	"context"
	"crypto/tls"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"pgedge-postgres-mcp/internal/auth"
	"pgedge-postgres-mcp/internal/httperror"
	"pgedge-postgres-mcp/internal/tracing"
)

// HTTPConfig holds configuration for HTTP/HTTPS server mode
type HTTPConfig struct {
	Addr          string                         // Server address (e.g., ":8080")
	TLSEnable     bool                           // Enable HTTPS
	CertFile      string                         // Path to TLS certificate file
	KeyFile       string                         // Path to TLS key file
	ChainFile     string                         // Optional path to certificate chain file
	AuthEnabled   bool                           // Enable API token authentication
	TokenStore    *auth.TokenStore               // Token store for authentication
	UserStore     *auth.UserStore                // User store for session token authentication
	SetupHandlers func(mux *http.ServeMux) error // Optional callback to add custom handlers before auth middleware
	Debug         bool                           // Enable debug logging
}

// buildHandler assembles the full mux and middleware chain used in HTTP
// mode. Split out from RunHTTP so the complete chain (including the
// JSON 404 catch-all and panic recovery) can be exercised directly in
// tests without binding a real listener.
func (s *Server) buildHandler(config *HTTPConfig) (http.Handler, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp/v1", s.handleHTTPRequest)
	mux.HandleFunc("/health", s.handleHealthCheck)

	// Call custom handler setup if provided (allows main.go to add LLM proxy endpoints)
	if config.SetupHandlers != nil {
		if err := config.SetupHandlers(mux); err != nil {
			return nil, fmt.Errorf("failed to setup custom handlers: %w", err)
		}
	}

	// Catch-all for any path not matched above. http.ServeMux has no
	// NotFoundHandler hook, so registering "/" (the least-specific
	// pattern) is the standard way to intercept unmatched routes; it
	// only fires when nothing more specific matched.
	mux.HandleFunc("/", jsonNotFoundHandler)

	// Wrap with auth middleware if enabled
	var handler http.Handler = mux
	if config.AuthEnabled {
		handler = auth.AuthMiddleware(config.TokenStore, config.UserStore, true)(handler)
	}

	// Wrap with security headers middleware
	handler = securityHeadersMiddleware(config.TLSEnable)(handler)

	// Wrap with panic recovery so a handler panic always yields a JSON
	// 500 response instead of an abruptly closed connection with no body.
	handler = recoveryMiddleware(handler)

	return handler, nil
}

// RunHTTP starts the MCP server in HTTP/HTTPS mode
func (s *Server) RunHTTP(config *HTTPConfig) error {
	if config == nil {
		return fmt.Errorf("HTTP config is required")
	}

	// Store debug flag for use in handlers
	s.debug = config.Debug

	handler, err := s.buildHandler(config)
	if err != nil {
		return err
	}

	// Configure server
	httpServer := &http.Server{
		Addr:    config.Addr,
		Handler: handler,

		// Guard against slow-header and slow-body attacks (e.g.
		// Slowloris): these fire before a request ever reaches a
		// handler, so the connection is simply closed - there is no
		// application-level body to write at this point.
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// Start server with or without TLS
	if config.TLSEnable {
		// Load TLS configuration
		tlsConfig, err := s.loadTLSConfig(config)
		if err != nil {
			return fmt.Errorf("failed to load TLS config: %w", err)
		}
		httpServer.TLSConfig = tlsConfig

		return httpServer.ListenAndServeTLS(config.CertFile, config.KeyFile)
	}

	return httpServer.ListenAndServe()
}

// loadTLSConfig loads TLS certificates and creates a TLS configuration
func (s *Server) loadTLSConfig(config *HTTPConfig) (*tls.Config, error) {
	// Load certificate and key
	cert, err := tls.LoadX509KeyPair(config.CertFile, config.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load certificate and key: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}

	// Load certificate chain if provided
	if config.ChainFile != "" {
		chainData, err := os.ReadFile(config.ChainFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read certificate chain: %w", err)
		}

		// Parse PEM-encoded certificates and append to chain
		for len(chainData) > 0 {
			var block *pem.Block
			block, chainData = pem.Decode(chainData)
			if block == nil {
				break
			}
			if block.Type == "CERTIFICATE" {
				cert.Certificate = append(cert.Certificate, block.Bytes)
			}
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return tlsConfig, nil
}

// jsonNotFoundHandler responds to any request that didn't match a more
// specific registered route with a JSON 404, instead of net/http's
// default plaintext "404 page not found".
func jsonNotFoundHandler(w http.ResponseWriter, r *http.Request) {
	httperror.Write(w, http.StatusNotFound, "Not found")
}

// recoveryMiddleware recovers from a panic anywhere in the wrapped
// handler chain and responds with a JSON 500 instead of leaving the
// client with an abruptly closed connection and no body at all. The
// stack trace is logged the same way an unrecovered panic would be.
func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if p := recover(); p != nil {
				// http.ErrAbortHandler is a sentinel panic value net/http
				// itself treats specially: it silently aborts the
				// response with no logging and no body. Preserve that
				// behavior by re-panicking so the stdlib's own handling
				// takes over instead of writing a JSON error for it.
				if p == http.ErrAbortHandler {
					panic(p)
				}
				fmt.Fprintf(os.Stderr, "PANIC serving %s %s: %v\n%s\n",
					r.Method, r.URL.Path, p, debug.Stack())
				httperror.Write(w, http.StatusInternalServerError, "Internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// securityHeadersMiddleware adds standard HTTP security headers to all
// responses to mitigate clickjacking, MIME-type confusion, and XSS.
// It also adds the RFC 8631 Link header on /api/* paths for API
// discoverability.
func securityHeadersMiddleware(tlsEnabled bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			if strings.HasPrefix(r.URL.Path, "/api/") {
				w.Header().Set("Link",
					`</api/openapi.json>; rel="service-desc"`)
			}
			if tlsEnabled {
				w.Header().Set("Strict-Transport-Security",
					"max-age=63072000; includeSubDomains")
			}
			next.ServeHTTP(w, r)
		})
	}
}

// MaxRequestBodySize is the maximum allowed size for HTTP request bodies (10MB)
// This prevents memory exhaustion from malicious oversized requests
const MaxRequestBodySize = 10 * 1024 * 1024

// handleHTTPRequest handles HTTP requests and translates them to JSON-RPC
func (s *Server) handleHTTPRequest(w http.ResponseWriter, r *http.Request) {
	// Only accept POST requests
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		httperror.Write(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Extract IP address and add to context
	ipAddress := auth.ExtractIPAddress(r)
	ctx := context.WithValue(r.Context(), auth.IPAddressContextKey, ipAddress)

	// Limit request body size to prevent memory exhaustion attacks
	r.Body = http.MaxBytesReader(w, r.Body, MaxRequestBodySize)

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			httperror.Write(w, http.StatusRequestEntityTooLarge, "Request body too large")
			return
		}
		httperror.Write(w, http.StatusBadRequest, "Failed to read request body")
		return
	}
	defer func() {
		if err := r.Body.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: Failed to close request body: %v\n", err)
		}
	}()

	// Parse JSON-RPC request
	var req JSONRPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		sendHTTPError(w, nil, -32700, "Parse error", err.Error())
		return
	}

	// Detect a JSON-RPC notification: a Request object without an "id" member
	// (per JSON-RPC 2.0 §4.1). Note that "id": null is a valid request id and
	// is NOT a notification, so we must inspect the raw JSON — interface{}
	// unmarshaling collapses "absent" and "null" to the same nil value.
	isNotification := !hasIDField(body)

	// Set up tracing context
	tokenHash := auth.GetTokenHashFromContext(ctx)
	sessionID := tokenHash
	if sessionID == "" {
		sessionID = "anonymous"
	}
	requestID := tracing.GenerateRequestID()
	ctx = tracing.WithRequestID(ctx, requestID)
	ctx = tracing.WithSessionID(ctx, sessionID)

	tracing.LogHTTPRequest(sessionID, tokenHash, requestID,
		r.Method, "/mcp/v1", req.Params)
	httpStart := time.Now()

	// Debug logging: log incoming request
	if s.debug {
		fmt.Fprintf(os.Stderr, "[DEBUG] Incoming request: method=%s id=%v ip=%s notification=%t\n",
			req.Method, req.ID, ipAddress, isNotification)
		if req.Params != nil {
			if paramsJSON, err := json.Marshal(req.Params); err == nil {
				fmt.Fprintf(os.Stderr, "[DEBUG] Request params: %s\n", string(paramsJSON))
			}
		}
	}

	// Per JSON-RPC 2.0 §4.1 ("The Server MUST NOT reply to a Notification")
	// and the MCP streamable HTTP transport spec ("If the input consists
	// solely of (any number of) JSON-RPC responses or notifications: ...
	// the server MUST return HTTP status code 202 Accepted with no body"),
	// short-circuit notifications before dispatch. This applies uniformly
	// to known notification methods (e.g. notifications/initialized) and
	// unknown ones — replying with -32601 Method not found to a
	// notification would be doubly wrong (replying when forbidden, and
	// replying without an id, which is itself a malformed JSON-RPC body).
	if isNotification {
		tracing.LogHTTPResponse(sessionID, tokenHash, requestID,
			r.Method, "/mcp/v1", http.StatusAccepted, nil,
			time.Since(httpStart))
		w.WriteHeader(http.StatusAccepted)
		return
	}

	// Handle the request and capture the response (pass context with IP address)
	response := s.handleRequestHTTP(ctx, req)

	tracing.LogHTTPResponse(sessionID, tokenHash, requestID,
		r.Method, "/mcp/v1", http.StatusOK, nil,
		time.Since(httpStart))

	// Debug logging: log outgoing response
	if s.debug {
		if responseJSON, err := json.Marshal(response); err == nil {
			fmt.Fprintf(os.Stderr, "[DEBUG] Outgoing response: %s\n", string(responseJSON))
		}
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to encode response: %v\n", err)
	}
}

// handleRequestHTTP handles a JSON-RPC request and returns the response.
//
// Notifications (requests without an "id" member, per JSON-RPC 2.0 §4.1) are
// filtered out by handleHTTPRequest before reaching this function and are
// answered with 202 Accepted and an empty body. As a result, every dispatch
// path here corresponds to a request that requires a response.
func (s *Server) handleRequestHTTP(ctx context.Context, req JSONRPCRequest) JSONRPCResponse {
	switch req.Method {
	case "initialize":
		return s.handleInitializeHTTP(req)
	case "ping":
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]interface{}{},
		}
	case "tools/list":
		return s.handleToolsListHTTP(ctx, req)
	case "tools/call":
		return s.handleToolCallHTTP(ctx, req)
	case "resources/list":
		return s.handleResourcesListHTTP(req)
	case "resources/read":
		return s.handleResourceReadHTTP(ctx, req)
	case "prompts/list":
		return s.handlePromptsListHTTP(req)
	case "prompts/get":
		return s.handlePromptGetHTTP(ctx, req)
	case "pgedge/listDatabases":
		return s.handleListDatabasesHTTP(ctx, req)
	case "pgedge/selectDatabase":
		return s.handleSelectDatabaseHTTP(ctx, req)
	default:
		return createErrorResponse(req.ID, -32601, "Method not found", nil)
	}
}

// HTTP-specific handlers that return responses instead of sending them

func (s *Server) handleInitializeHTTP(req JSONRPCRequest) JSONRPCResponse {
	capabilities := map[string]interface{}{
		"tools": map[string]interface{}{},
	}

	// Add resources capability if resource provider is set
	if s.resources != nil {
		capabilities["resources"] = map[string]interface{}{}
	}

	// Add prompts capability if prompt provider is set
	if s.prompts != nil {
		capabilities["prompts"] = map[string]interface{}{}
	}

	result := InitializeResult{
		ProtocolVersion: ProtocolVersion,
		Capabilities:    capabilities,
		ServerInfo: Implementation{
			Name:    ServerName,
			Version: ServerVersion,
		},
		Instructions: ServerInstructions,
	}

	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

func (s *Server) handleToolsListHTTP(ctx context.Context, req JSONRPCRequest) JSONRPCResponse {
	// Use ListContext to get tools with context-aware descriptions
	// This ensures tools like query_database show correct write access status
	tools := s.tools.ListContext(ctx)
	result := ToolsListResult{Tools: tools}

	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

func (s *Server) handleToolCallHTTP(ctx context.Context, req JSONRPCRequest) JSONRPCResponse {
	var params ToolCallParams

	// Convert interface{} to JSON bytes first
	paramsJSON, err := json.Marshal(req.Params)
	if err != nil {
		return createErrorResponse(req.ID, -32602, "Invalid params", err.Error())
	}

	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return createErrorResponse(req.ID, -32602, "Invalid params", err.Error())
	}

	sessionID := tracing.GetSessionIDFromContext(ctx)
	tokenHash := auth.GetTokenHashFromContext(ctx)
	toolRequestID := fmt.Sprintf("%v", req.ID)
	tracing.LogToolCall(sessionID, tokenHash, toolRequestID,
		params.Name, params.Arguments)
	start := time.Now()

	// Pass context for per-token connection isolation
	response, err := s.tools.Execute(ctx, params.Name, params.Arguments)

	tracing.LogToolResult(sessionID, tokenHash, toolRequestID,
		params.Name, response, err, time.Since(start))

	if err != nil {
		return createErrorResponse(req.ID, -32603, "Internal error", err.Error())
	}

	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  response,
	}
}

func (s *Server) handleResourcesListHTTP(req JSONRPCRequest) JSONRPCResponse {
	if s.resources == nil {
		return createErrorResponse(req.ID, -32603, "Resources not available", nil)
	}

	resources := s.resources.List()
	result := ResourcesListResult{Resources: resources}

	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

func (s *Server) handleResourceReadHTTP(ctx context.Context, req JSONRPCRequest) JSONRPCResponse {
	if s.resources == nil {
		return createErrorResponse(req.ID, -32603, "Resources not available", nil)
	}

	var params ResourceReadParams

	// Convert interface{} to JSON bytes first
	paramsJSON, err := json.Marshal(req.Params)
	if err != nil {
		return createErrorResponse(req.ID, -32602, "Invalid params", err.Error())
	}

	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return createErrorResponse(req.ID, -32602, "Invalid params", err.Error())
	}

	sessionID := tracing.GetSessionIDFromContext(ctx)
	tokenHash := auth.GetTokenHashFromContext(ctx)
	resRequestID := fmt.Sprintf("%v", req.ID)
	tracing.LogResourceRead(sessionID, tokenHash, resRequestID, params.URI)
	start := time.Now()

	content, err := s.resources.Read(ctx, params.URI)

	tracing.LogResourceResult(sessionID, tokenHash, resRequestID,
		params.URI, content, err, time.Since(start))

	if err != nil {
		return createErrorResponse(req.ID, -32603, "Failed to read resource", err.Error())
	}

	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  content,
	}
}

func (s *Server) handlePromptsListHTTP(req JSONRPCRequest) JSONRPCResponse {
	if s.prompts == nil {
		return createErrorResponse(req.ID, -32601, "Prompts not supported", nil)
	}

	prompts := s.prompts.List()
	result := PromptsListResult{Prompts: prompts}

	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

func (s *Server) handlePromptGetHTTP(ctx context.Context, req JSONRPCRequest) JSONRPCResponse {
	if s.prompts == nil {
		return createErrorResponse(req.ID, -32601, "Prompts not supported", nil)
	}

	var params PromptGetParams

	// Convert interface{} to JSON bytes first
	paramsJSON, err := json.Marshal(req.Params)
	if err != nil {
		return createErrorResponse(req.ID, -32602, "Invalid params", err.Error())
	}

	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return createErrorResponse(req.ID, -32602, "Invalid params", err.Error())
	}

	sessionID := tracing.GetSessionIDFromContext(ctx)
	tokenHash := auth.GetTokenHashFromContext(ctx)
	promptRequestID := fmt.Sprintf("%v", req.ID)
	tracing.LogPromptCall(sessionID, tokenHash, promptRequestID,
		params.Name, params.Arguments)
	start := time.Now()

	result, err := s.prompts.Execute(params.Name, params.Arguments)

	tracing.LogPromptResult(sessionID, tokenHash, promptRequestID,
		params.Name, result, err, time.Since(start))

	if err != nil {
		return createErrorResponse(req.ID, -32603, "Prompt execution error", err.Error())
	}

	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

func (s *Server) handleListDatabasesHTTP(ctx context.Context, req JSONRPCRequest) JSONRPCResponse {
	if s.databases == nil {
		return createErrorResponse(req.ID, -32601, "Database management not supported", nil)
	}

	databases, current, err := s.databases.ListDatabases(ctx)
	if err != nil {
		return createErrorResponse(req.ID, -32603, "Failed to list databases", err.Error())
	}

	result := ListDatabasesResponse{
		Databases: databases,
		Current:   current,
	}

	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

func (s *Server) handleSelectDatabaseHTTP(ctx context.Context, req JSONRPCRequest) JSONRPCResponse {
	if s.databases == nil {
		return createErrorResponse(req.ID, -32601, "Database management not supported", nil)
	}

	var params SelectDatabaseParams

	// Convert interface{} to JSON bytes first
	paramsJSON, err := json.Marshal(req.Params)
	if err != nil {
		return createErrorResponse(req.ID, -32602, "Invalid params", err.Error())
	}

	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return createErrorResponse(req.ID, -32602, "Invalid params", err.Error())
	}

	if params.Name == "" {
		return createErrorResponse(req.ID, -32602, "Invalid params", "database name is required")
	}

	if err := s.databases.SelectDatabase(ctx, params.Name); err != nil {
		result := SelectDatabaseResponse{
			Success: false,
			Error:   err.Error(),
		}
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  result,
		}
	}

	sessionID := tracing.GetSessionIDFromContext(ctx)
	tokenHash := auth.GetTokenHashFromContext(ctx)
	tracing.LogDatabaseSwitch(sessionID, tokenHash,
		fmt.Sprintf("%v", req.ID), params.Name, nil)

	result := SelectDatabaseResponse{
		Success: true,
		Current: params.Name,
		Message: fmt.Sprintf("Switched to database: %s", params.Name),
	}

	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

// handleHealthCheck provides a simple health check endpoint
func (s *Server) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := fmt.Fprintf(w, `{"status":"ok","server":"%s","version":"%s"}`, ServerName, ServerVersion); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: Failed to write health check response: %v\n", err)
	}
}

// Helper functions

// hasIDField reports whether the given raw JSON-RPC message body has an "id"
// member at its top level. This is used to distinguish a JSON-RPC notification
// (no id member) from a request whose id is explicitly null — the JSON-RPC 2.0
// spec treats these very differently, but Go's interface{} unmarshaling
// collapses both to nil. We probe the raw bytes via json.RawMessage so we do
// not have to re-parse the entire payload.
func hasIDField(body []byte) bool {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(body, &probe); err != nil {
		return false
	}
	_, ok := probe["id"]
	return ok
}

func sendHTTPError(w http.ResponseWriter, id interface{}, code int, message string, data interface{}) {
	response := createErrorResponse(id, code, message, data)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK) // JSON-RPC errors are still HTTP 200
	if err := json.NewEncoder(w).Encode(response); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: Failed to encode error response: %v\n", err)
	}
}

func createErrorResponse(id interface{}, code int, message string, data interface{}) JSONRPCResponse {
	errResp := RPCError{
		Code:    code,
		Message: message,
	}
	if data != nil {
		errResp.Data = data
	}

	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &errResp,
	}
}
