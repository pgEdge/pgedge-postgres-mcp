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
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
)

const (
	ProtocolVersion = "2024-11-05"
	ServerName      = "pgedge-mcp-server"
	ServerVersion   = "1.0.0-alpha5"
)

// ToolProvider is an interface for listing and executing tools
type ToolProvider interface {
	List() []Tool
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
	Name     string `json:"name"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Database string `json:"database"`
	User     string `json:"user"`
	SSLMode  string `json:"sslmode"`
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
	tools     ToolProvider
	resources ResourceProvider
	prompts   PromptProvider
	databases DatabaseProvider
	debug     bool // Enable debug logging for HTTP mode
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
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, ScannerInitialBufferSize), ScannerMaxBufferSize)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			sendError(nil, -32700, "Parse error", err.Error())
			continue
		}

		s.handleRequest(req)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %w", err)
	}

	return nil
}

func (s *Server) handleRequest(req JSONRPCRequest) {
	switch req.Method {
	case "initialize":
		s.handleInitialize(req)
	case "notifications/initialized":
		// Client notification - no response needed
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
		if req.ID != nil {
			sendError(req.ID, -32601, "Method not found", nil)
		}
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

	// Accept the client's protocol version for compatibility
	protocolVersion := params.ProtocolVersion
	if protocolVersion == "" {
		protocolVersion = ProtocolVersion
	}

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
	}

	sendResponse(req.ID, result)
}

func (s *Server) handleToolsList(req JSONRPCRequest) {
	tools := s.tools.List()

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

	// For stdio mode, use background context (no authentication)
	response, err := s.tools.Execute(context.Background(), params.Name, params.Arguments)
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

	// Use background context for stdio mode (no HTTP request context available)
	content, err := s.resources.Read(context.Background(), params.URI)
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

	result, err := s.prompts.Execute(params.Name, params.Arguments)
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
