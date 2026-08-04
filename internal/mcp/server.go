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
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"pgedge-postgres-mcp/internal/tracing"
)

const (
	ProtocolVersion = "2024-11-05"
	ServerName      = "pgedge-postgres-mcp"
	ServerVersion   = "1.0.0"

	// ServerInstructions provides guidance to MCP clients about tool usage
	ServerInstructions = "For PostgreSQL database operations, prefer the tools advertised by this server in tools/list instead of psql or other shell commands. Use the available MCP tools for schema discovery, query execution, performance analysis, row counts, and database management. These tools apply the server's connection handling, authentication, access control, and logging policies automatically."
)

// SupportedProtocolVersions lists every MCP protocol revision this server
// implements, oldest first. ProtocolVersion, the server's own default and
// preferred revision, must be the last (newest) entry; TestSupportedProtocolVersions_NewestIsProtocolVersion
// in server_test.go enforces that invariant as more revisions are added.
var SupportedProtocolVersions = []string{
	"2024-11-05",
}

// NegotiateProtocolVersion returns the protocol revision this server will
// actually speak in response to a client's requested revision, per the MCP
// specification's version negotiation: the newest supported revision at or
// below the client's request, or the oldest supported revision if the
// request predates every revision this server implements.
//
// Version negotiation is the server's half of the handshake: the client
// proposes a revision and the server replies with the revision it will
// actually use, which the client is expected to check against what it can
// support. Echoing the client's request back unconditionally -- what this
// function replaces -- answers with no information at all, so a client
// requesting a revision this server does not implement proceeds believing
// its request was honoured, and fails later against a missing capability
// rather than at the version check meant to catch exactly this case.
//
// An empty request (a client that omitted the field) gets the server's own
// default, ProtocolVersion, rather than falling through the negotiation
// logic below: an absent preference is not the same as a request for the
// oldest revision.
func NegotiateProtocolVersion(requested string) string {
	if requested == "" {
		return ProtocolVersion
	}
	best := SupportedProtocolVersions[0]
	for _, v := range SupportedProtocolVersions {
		if v <= requested {
			best = v
		}
	}
	return best
}

// ToolProvider is an interface for listing and executing tools
type ToolProvider interface {
	List() []Tool
	ListContext(ctx context.Context) []Tool
	Execute(ctx context.Context, name string, args map[string]interface{}) (ToolResponse, error)
}

// ResourceProvider is an interface for listing and reading resources
type ResourceProvider interface {
	List() []Resource
	Read(ctx context.Context, uri string) (ResourceContent, error)
}

// PromptProvider is an interface for listing and executing prompts
type PromptProvider interface {
	List() []Prompt
	Execute(name string, args map[string]string) (PromptResult, error)
}

// DatabaseInfo represents a database connection for listing
type DatabaseInfo struct {
	Name        string `json:"name"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Database    string `json:"database"`
	User        string `json:"user"`
	SSLMode     string `json:"sslmode"`
	AllowWrites bool   `json:"allow_writes"`
}

// DatabaseProvider is an interface for managing database connections
type DatabaseProvider interface {
	// ListDatabases returns available databases and the current database name
	ListDatabases(ctx context.Context) ([]DatabaseInfo, string, error)
	// SelectDatabase sets the current database for the session
	SelectDatabase(ctx context.Context, name string) error
}

// Server handles MCP protocol communication
type Server struct {
	tools          ToolProvider
	resources      ResourceProvider
	prompts        PromptProvider
	databases      DatabaseProvider
	debug          bool   // Enable debug logging for HTTP mode
	stdioSessionID string // Session ID for STDIO mode tracing
}

// NewServer creates a new MCP server
func NewServer(tools ToolProvider) *Server {
	return &Server{
		tools: tools,
	}
}

// SetResourceProvider sets the resource provider for the server
func (s *Server) SetResourceProvider(resources ResourceProvider) {
	s.resources = resources
}

// SetPromptProvider sets the prompt provider for the server
func (s *Server) SetPromptProvider(prompts PromptProvider) {
	s.prompts = prompts
}

// SetDatabaseProvider sets the database provider for the server
func (s *Server) SetDatabaseProvider(databases DatabaseProvider) {
	s.databases = databases
}

// Run starts the stdio server loop
func (s *Server) Run() error {
	s.stdioSessionID = tracing.GenerateSessionID()
	tracing.LogSessionStart(s.stdioSessionID, "", nil)
	defer tracing.LogSessionEnd(s.stdioSessionID, "", nil)

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, ScannerInitialBufferSize), ScannerMaxBufferSize)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			sendError(nil, -32700, "Parse error", err.Error())
			continue
		}

		// Per JSON-RPC 2.0 §4.1, a Notification is a Request object
		// without an "id" member, and the server MUST NOT reply. Note
		// that "id": null is a valid request id and is NOT a
		// notification, so we must inspect the raw JSON — interface{}
		// unmarshaling collapses "absent" and "null" to the same nil
		// value. This mirrors the HTTP transport's handling in
		// handleHTTPRequest so the two transports behave identically.
		if !hasIDField(line) {
			continue
		}

		s.handleRequest(req)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %w", err)
	}

	return nil
}

// handleRequest dispatches a JSON-RPC request to the appropriate handler.
//
// Notifications (requests without an "id" member, per JSON-RPC 2.0 §4.1)
// are filtered out by Run before reaching this function, so every dispatch
// path here corresponds to a request that requires a response — including
// requests whose id is explicitly null.
func (s *Server) handleRequest(req JSONRPCRequest) {
	switch req.Method {
	case "initialize":
		s.handleInitialize(req)
	case "ping":
		s.handlePing(req)
	case "tools/list":
		s.handleToolsList(req)
	case "tools/call":
		s.handleToolCall(req)
	case "resources/list":
		s.handleResourcesList(req)
	case "resources/read":
		s.handleResourceRead(req)
	case "prompts/list":
		s.handlePromptsList(req)
	case "prompts/get":
		s.handlePromptsGet(req)
	case "pgedge/listDatabases":
		s.handleListDatabases(req)
	case "pgedge/selectDatabase":
		s.handleSelectDatabase(req)
	default:
		sendError(req.ID, -32601, "Method not found", nil)
	}
}

func (s *Server) handleInitialize(req JSONRPCRequest) {
	paramsBytes, err := json.Marshal(req.Params)
	if err != nil {
		sendError(req.ID, -32602, "Invalid params", err.Error())
		return
	}
	var params InitializeParams
	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		sendError(req.ID, -32602, "Invalid params", err.Error())
		return
	}

	protocolVersion := NegotiateProtocolVersion(params.ProtocolVersion)

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
		ProtocolVersion: protocolVersion,
		Capabilities:    capabilities,
		ServerInfo: Implementation{
			Name:    ServerName,
			Version: ServerVersion,
		},
		Instructions: ServerInstructions,
	}

	sendResponse(req.ID, result)
}

// handlePing replies with the standard empty-object result. Notifications
// are filtered out by Run before reaching dispatch, so every ping that
// arrives here is a request requiring a response — including one whose
// id is explicitly null.
func (s *Server) handlePing(req JSONRPCRequest) {
	sendResponse(req.ID, map[string]interface{}{})
}

func (s *Server) handleToolsList(req JSONRPCRequest) {
	// Use ListContext for context-aware tool descriptions
	// In STDIO mode, use background context (no authentication)
	tools := s.tools.ListContext(context.Background())

	result := map[string]interface{}{
		"tools": tools,
	}

	sendResponse(req.ID, result)
}

func (s *Server) handleToolCall(req JSONRPCRequest) {
	paramsBytes, err := json.Marshal(req.Params)
	if err != nil {
		sendError(req.ID, -32602, "Invalid params", err.Error())
		return
	}
	var params ToolCallParams
	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		sendError(req.ID, -32602, "Invalid params", err.Error())
		return
	}

	requestID := fmt.Sprintf("%v", req.ID)
	tracing.LogToolCall(s.stdioSessionID, "", requestID,
		params.Name, params.Arguments)
	start := time.Now()

	// For stdio mode, use background context (no authentication)
	response, err := s.tools.Execute(context.Background(), params.Name, params.Arguments)

	tracing.LogToolResult(s.stdioSessionID, "", requestID,
		params.Name, response, err, time.Since(start))

	if err != nil {
		sendError(req.ID, -32603, "Tool execution error", err.Error())
		return
	}

	sendResponse(req.ID, response)
}

func (s *Server) handleResourcesList(req JSONRPCRequest) {
	if s.resources == nil {
		sendError(req.ID, -32601, "Resources not supported", nil)
		return
	}

	resources := s.resources.List()

	result := map[string]interface{}{
		"resources": resources,
	}

	sendResponse(req.ID, result)
}

func (s *Server) handleResourceRead(req JSONRPCRequest) {
	if s.resources == nil {
		sendError(req.ID, -32601, "Resources not supported", nil)
		return
	}

	paramsBytes, err := json.Marshal(req.Params)
	if err != nil {
		sendError(req.ID, -32602, "Invalid params", err.Error())
		return
	}
	var params ResourceReadParams
	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		sendError(req.ID, -32602, "Invalid params", err.Error())
		return
	}

	requestID := fmt.Sprintf("%v", req.ID)
	tracing.LogResourceRead(s.stdioSessionID, "", requestID, params.URI)
	start := time.Now()

	// Use background context for stdio mode (no HTTP request context available)
	content, err := s.resources.Read(context.Background(), params.URI)

	tracing.LogResourceResult(s.stdioSessionID, "", requestID,
		params.URI, content, err, time.Since(start))

	if err != nil {
		sendError(req.ID, -32603, "Resource read error", err.Error())
		return
	}

	sendResponse(req.ID, content)
}

func (s *Server) handlePromptsList(req JSONRPCRequest) {
	if s.prompts == nil {
		sendError(req.ID, -32601, "Prompts not supported", nil)
		return
	}

	prompts := s.prompts.List()

	result := PromptsListResult{
		Prompts: prompts,
	}

	sendResponse(req.ID, result)
}

func (s *Server) handlePromptsGet(req JSONRPCRequest) {
	if s.prompts == nil {
		sendError(req.ID, -32601, "Prompts not supported", nil)
		return
	}

	paramsBytes, err := json.Marshal(req.Params)
	if err != nil {
		sendError(req.ID, -32602, "Invalid params", err.Error())
		return
	}
	var params PromptGetParams
	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		sendError(req.ID, -32602, "Invalid params", err.Error())
		return
	}

	requestID := fmt.Sprintf("%v", req.ID)
	tracing.LogPromptCall(s.stdioSessionID, "", requestID,
		params.Name, params.Arguments)
	start := time.Now()

	result, err := s.prompts.Execute(params.Name, params.Arguments)

	tracing.LogPromptResult(s.stdioSessionID, "", requestID,
		params.Name, result, err, time.Since(start))

	if err != nil {
		sendError(req.ID, -32603, "Prompt execution error", err.Error())
		return
	}

	sendResponse(req.ID, result)
}

// ListDatabasesResponse is the response for pgedge/listDatabases
type ListDatabasesResponse struct {
	Databases []DatabaseInfo `json:"databases"`
	Current   string         `json:"current"`
}

// SelectDatabaseParams are the parameters for pgedge/selectDatabase
type SelectDatabaseParams struct {
	Name string `json:"name"`
}

// SelectDatabaseResponse is the response for pgedge/selectDatabase
type SelectDatabaseResponse struct {
	Success bool   `json:"success"`
	Current string `json:"current,omitempty"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

func (s *Server) handleListDatabases(req JSONRPCRequest) {
	if s.databases == nil {
		sendError(req.ID, -32601, "Database management not supported", nil)
		return
	}

	// Use background context for stdio mode (no HTTP request context available)
	databases, current, err := s.databases.ListDatabases(context.Background())
	if err != nil {
		sendError(req.ID, -32603, "Failed to list databases", err.Error())
		return
	}

	result := ListDatabasesResponse{
		Databases: databases,
		Current:   current,
	}

	sendResponse(req.ID, result)
}

func (s *Server) handleSelectDatabase(req JSONRPCRequest) {
	if s.databases == nil {
		sendError(req.ID, -32601, "Database management not supported", nil)
		return
	}

	paramsBytes, err := json.Marshal(req.Params)
	if err != nil {
		sendError(req.ID, -32602, "Invalid params", err.Error())
		return
	}
	var params SelectDatabaseParams
	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		sendError(req.ID, -32602, "Invalid params", err.Error())
		return
	}

	if params.Name == "" {
		sendError(req.ID, -32602, "Invalid params", "database name is required")
		return
	}

	// Use background context for stdio mode (no HTTP request context available)
	if err := s.databases.SelectDatabase(context.Background(), params.Name); err != nil {
		result := SelectDatabaseResponse{
			Success: false,
			Error:   err.Error(),
		}
		sendResponse(req.ID, result)
		return
	}

	tracing.LogDatabaseSwitch(s.stdioSessionID, "",
		fmt.Sprintf("%v", req.ID), params.Name, nil)

	result := SelectDatabaseResponse{
		Success: true,
		Current: params.Name,
		Message: fmt.Sprintf("Switched to database: %s", params.Name),
	}

	sendResponse(req.ID, result)
}

func sendResponse(id, result interface{}) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to marshal response: %v\n", err)
		return
	}
	fmt.Println(string(data))
	_ = os.Stdout.Sync()
}

func sendError(id interface{}, code int, message string, data interface{}) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &RPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}

	respData, err := json.Marshal(resp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to marshal error response: %v\n", err)
		return
	}
	fmt.Println(string(respData))
	_ = os.Stdout.Sync()
}
