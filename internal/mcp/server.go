package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

const (
	ProtocolVersion = "2024-11-05"
	ServerName      = "pgedge-mcp"
	ServerVersion   = "0.1.0"
)

// ToolProvider is an interface for listing and executing tools
type ToolProvider interface {
	List() []Tool
	Execute(name string, args map[string]interface{}) (ToolResponse, error)
}

// ResourceProvider is an interface for listing and reading resources
type ResourceProvider interface {
	List() []Resource
	Read(uri string) (ResourceContent, error)
}

// Server handles MCP protocol communication
type Server struct {
	tools     ToolProvider
	resources ResourceProvider
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

// Run starts the stdio server loop
func (s *Server) Run() error {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

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
	default:
		if req.ID != nil {
			sendError(req.ID, -32601, "Method not found", nil)
		}
	}
}

func (s *Server) handleInitialize(req JSONRPCRequest) {
	paramsBytes, _ := json.Marshal(req.Params)
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
	paramsBytes, _ := json.Marshal(req.Params)
	var params ToolCallParams
	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		sendError(req.ID, -32602, "Invalid params", err.Error())
		return
	}

	response, err := s.tools.Execute(params.Name, params.Arguments)
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

	paramsBytes, _ := json.Marshal(req.Params)
	var params ResourceReadParams
	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		sendError(req.ID, -32602, "Invalid params", err.Error())
		return
	}

	content, err := s.resources.Read(params.URI)
	if err != nil {
		sendError(req.ID, -32603, "Resource read error", err.Error())
		return
	}

	sendResponse(req.ID, content)
}

func sendResponse(id interface{}, result interface{}) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}

	data, _ := json.Marshal(resp)
	fmt.Println(string(data))
	os.Stdout.Sync()
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

	respData, _ := json.Marshal(resp)
	fmt.Println(string(respData))
	os.Stdout.Sync()
}
