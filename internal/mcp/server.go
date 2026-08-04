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
	"errors"
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

// supportedProtocolVersions lists the handshake-era ("legacy") MCP protocol
// revisions this server implements, oldest first. ProtocolVersion, the
// server's own default and preferred revision, must be the last (newest)
// entry; TestSupportedProtocolVersions_NewestIsProtocolVersion in
// server_test.go enforces that invariant as more revisions are added.
//
// Only revisions that negotiate through the initialize handshake belong
// here, which means 2025-11-25 and earlier. Revision 2026-07-28 removed the
// handshake entirely: a client declares its version in per-request _meta
// (and the MCP-Protocol-Version header over HTTP), servers must implement
// server/discover to advertise what they support, and a version mismatch
// returns UnsupportedProtocolVersionError rather than being settled at
// connect time. Adding 2026-07-28 or later to this list would make
// NegotiateProtocolVersion answer a legacy handshake with a revision that
// has no handshake, so supporting a modern revision needs its own path
// rather than another entry here.
var supportedProtocolVersions = []string{
	"2024-11-05",
}

// NegotiateProtocolVersion returns the protocol revision this server will
// actually speak in response to a client's requested revision: the newest
// supported revision at or below the client's request, or the oldest
// supported revision if the request predates every revision this server
// implements.
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
// The specification's own rule is narrower: reply with the requested
// revision if it is supported, and otherwise with a supported revision,
// which SHOULD be the server's latest. Answering with the newest revision
// at or below the request is a deliberately more conservative reading of
// that SHOULD, on the grounds that a client asking for an older revision is
// likelier to cope with something older still than with something newer
// than it asked for. The two rules coincide exactly whilst this server
// implements a single revision, so today the distinction is a statement of
// intent for whoever adds the second one.
//
// An empty request (a client that omitted the field) gets the server's own
// default, ProtocolVersion, rather than falling through the negotiation
// logic below: an absent preference is not the same as a request for the
// oldest revision.
func NegotiateProtocolVersion(requested string) string {
	if requested == "" {
		return ProtocolVersion
	}
	best := supportedProtocolVersions[0]
	for _, v := range supportedProtocolVersions {
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
	// A request carrying io.modelcontextprotocol/protocolVersion in its
	// _meta is modern and validated uniformly here, including
	// server/discover: the spec's header/negotiation rules name no
	// exception for it, and none is needed in practice, since a client
	// that does not yet know what to send can simply omit _meta
	// entirely -- handleDiscover answers a request shaped that way
	// exactly as fully as a validated modern one (see
	// TestServerDiscover_LegacyShapedRequestAlsoWorks). A real modern
	// client never sends "initialize" (2026-07-28 has no handshake), so
	// this is the same check below.
	meta, isModern := isModernRequest(req.Params)
	if isModern {
		if rpcErr := validateModernRequestStdio(meta); rpcErr != nil {
			sendError(req.ID, rpcErr.Code, rpcErr.Message, rpcErr.Data)
			return
		}
	}

	switch req.Method {
	case "initialize":
		// "initialize" does not exist as a method in the modern era
		// (2026-07-28 removed the handshake entirely), so a
		// modern-shaped request naming it is treated exactly like any
		// other method the modern era doesn't recognize: -32601, not
		// a legacy InitializeResult.
		if isModern {
			sendError(req.ID, -32601, "Method not found", nil)
			return
		}
		s.handleInitialize(req)
	case "server/discover":
		s.handleDiscover(req)
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

// buildCapabilities returns this server's capabilities object, shared by
// the legacy initialize handshake, server/discover, and their HTTP
// equivalents. The empty "extensions" map is always present per the
// 2026-07-28 versioning spec's extension-negotiation convention; this
// server implements no extensions, so it is always empty rather than
// omitted, to distinguish "no extensions supported" from "capabilities
// not yet reported" for a client checking the key.
func (s *Server) buildCapabilities() map[string]interface{} {
	capabilities := map[string]interface{}{
		"tools":      map[string]interface{}{},
		"extensions": map[string]interface{}{},
	}

	if s.resources != nil {
		capabilities["resources"] = map[string]interface{}{}
	}

	if s.prompts != nil {
		capabilities["prompts"] = map[string]interface{}{}
	}

	return capabilities
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

	result := InitializeResult{
		ProtocolVersion: protocolVersion,
		Capabilities:    s.buildCapabilities(),
		ServerInfo: Implementation{
			Name:    ServerName,
			Version: ServerVersion,
		},
		Instructions: ServerInstructions,
	}

	sendResponse(req.ID, result)
}

// handleDiscover implements server/discover (stdio transport), which
// the 2026-07-28 revision requires every server to implement: it lets a
// modern client learn this server's supported protocol versions,
// capabilities, and identity without a connection-scoped handshake. See
// modern.go for the version list and cacheableResult/responseMetaFor
// helpers shared with every other modern result.
func (s *Server) handleDiscover(req JSONRPCRequest) {
	result := DiscoverResult{
		CacheableResult:   cacheableResult(),
		SupportedVersions: SupportedModernProtocolVersions,
		Capabilities:      s.buildCapabilities(),
		Meta:              responseMetaFor(),
		Instructions:      ServerInstructions,
	}
	sendResponse(req.ID, result)
}

// handlePing replies with the standard empty-object result. Notifications
// are filtered out by Run before reaching dispatch, so every ping that
// arrives here is a request requiring a response — including one whose
// id is explicitly null.
func (s *Server) handlePing(req JSONRPCRequest) {
	sendResponseFor(req, map[string]interface{}{}, false)
}

func (s *Server) handleToolsList(req JSONRPCRequest) {
	// Use ListContext for context-aware tool descriptions
	// In STDIO mode, use background context (no authentication)
	tools := s.tools.ListContext(context.Background())

	result := map[string]interface{}{
		"tools": tools,
	}

	sendResponseFor(req, result, true)
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

	sendResponseFor(req, response, false)
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

	sendResponseFor(req, result, true)
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

	if errors.Is(err, ErrResourceNotFound) {
		sendError(req.ID, resourceNotFoundCode(req), "Resource not found", params.URI)
		return
	}
	if err != nil {
		sendError(req.ID, -32603, "Resource read error", err.Error())
		return
	}

	sendResponseFor(req, content, true)
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

	sendResponseFor(req, result, true)
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

	sendResponseFor(req, result, false)
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

	sendResponseFor(req, result, false)
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
		sendResponseFor(req, result, false)
		return
	}

	tracing.LogDatabaseSwitch(s.stdioSessionID, "",
		fmt.Sprintf("%v", req.ID), params.Name, nil)

	result := SelectDatabaseResponse{
		Success: true,
		Current: params.Name,
		Message: fmt.Sprintf("Switched to database: %s", params.Name),
	}

	sendResponseFor(req, result, false)
}

// sendResponseFor sends a handler's result, wrapping it with modern
// (2026-07-28) fields first if the originating request was modern. Every
// stdio handler except handleInitialize and handleDiscover (which build
// their own response shape directly, and are never subject to this
// wrapping) calls this instead of the bare sendResponse, so the
// modern/legacy branch lives in one place rather than in each handler.
func sendResponseFor(req JSONRPCRequest, result interface{}, cacheable bool) {
	if _, isModern := isModernRequest(req.Params); isModern {
		if cacheable {
			result = wrapModernCacheableResult(result)
		} else {
			result = wrapModernResult(result)
		}
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
