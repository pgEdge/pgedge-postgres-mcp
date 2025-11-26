/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package mcp

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"pgedge-postgres-mcp/internal/auth"
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

// RunHTTP starts the MCP server in HTTP/HTTPS mode
func (s *Server) RunHTTP(config *HTTPConfig) error {
	if config == nil {
		return fmt.Errorf("HTTP config is required")
	}

	// Store debug flag for use in handlers
	s.debug = config.Debug

	// Create HTTP handler
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp/v1", s.handleHTTPRequest)
	mux.HandleFunc("/health", s.handleHealthCheck)

	// Call custom handler setup if provided (allows main.go to add LLM proxy endpoints)
	if config.SetupHandlers != nil {
		if err := config.SetupHandlers(mux); err != nil {
			return fmt.Errorf("failed to setup custom handlers: %w", err)
		}
	}

	// Wrap with auth middleware if enabled
	var handler http.Handler = mux
	if config.AuthEnabled {
		handler = auth.AuthMiddleware(config.TokenStore, config.UserStore, true)(handler)
	}

	// Configure server
	httpServer := &http.Server{
		Addr:    config.Addr,
		Handler: handler,
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

		// Append chain to certificate
		cert.Certificate = append(cert.Certificate, chainData)
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return tlsConfig, nil
}

// handleHTTPRequest handles HTTP requests and translates them to JSON-RPC
func (s *Server) handleHTTPRequest(w http.ResponseWriter, r *http.Request) {
	// Only accept POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract IP address and add to context
	ipAddress := auth.ExtractIPAddress(r)
	ctx := context.WithValue(r.Context(), auth.IPAddressContextKey, ipAddress)

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
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

	// Debug logging: log incoming request
	if s.debug {
		fmt.Fprintf(os.Stderr, "[DEBUG] Incoming request: method=%s id=%v ip=%s\n", req.Method, req.ID, ipAddress)
		if req.Params != nil {
			if paramsJSON, err := json.Marshal(req.Params); err == nil {
				fmt.Fprintf(os.Stderr, "[DEBUG] Request params: %s\n", string(paramsJSON))
			}
		}
	}

	// Handle the request and capture the response (pass context with IP address)
	response := s.handleRequestHTTP(ctx, req)

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

// handleRequestHTTP handles a JSON-RPC request and returns the response
func (s *Server) handleRequestHTTP(ctx context.Context, req JSONRPCRequest) JSONRPCResponse {
	switch req.Method {
	case "initialize":
		return s.handleInitializeHTTP(req)
	case "notifications/initialized":
		// Client notification - return empty response
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  json.RawMessage(`{}`),
		}
	case "tools/list":
		return s.handleToolsListHTTP(req)
	case "tools/call":
		return s.handleToolCallHTTP(ctx, req)
	case "resources/list":
		return s.handleResourcesListHTTP(req)
	case "resources/read":
		return s.handleResourceReadHTTP(ctx, req)
	case "prompts/list":
		return s.handlePromptsListHTTP(req)
	case "prompts/get":
		return s.handlePromptGetHTTP(req)
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
	}

	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

func (s *Server) handleToolsListHTTP(req JSONRPCRequest) JSONRPCResponse {
	tools := s.tools.List()
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

	// Pass context for per-token connection isolation
	response, err := s.tools.Execute(ctx, params.Name, params.Arguments)
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

	content, err := s.resources.Read(ctx, params.URI)
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

func (s *Server) handlePromptGetHTTP(req JSONRPCRequest) JSONRPCResponse {
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

	result, err := s.prompts.Execute(params.Name, params.Arguments)
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
