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

// JSONRPCRequest represents an incoming JSON-RPC 2.0 request
type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// JSONRPCResponse represents an outgoing JSON-RPC 2.0 response.
//
// Per JSON-RPC 2.0 §5.1, the response object MUST include the id
// member; the value is the id of the originating request, or null when
// the id cannot be determined (e.g. Parse error / Invalid Request) or
// when the request itself used "id": null. The id tag therefore does
// not use omitempty — Go's encoder collapses a nil interface to JSON
// null, which is exactly the required wire representation in those
// cases. Result and Error remain mutually exclusive (one MUST be
// present, the other absent), so both keep omitempty.
type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

// RPCError represents a JSON-RPC error
type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// InitializeParams represents the parameters for the initialize request
type InitializeParams struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	ClientInfo      ClientInfo             `json:"clientInfo"`
}

// ClientInfo contains information about the MCP client
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Implementation contains server implementation details
type Implementation struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializeResult is the response to an initialize request
type InitializeResult struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	ServerInfo      Implementation         `json:"serverInfo"`
	Instructions    string                 `json:"instructions,omitempty"`
}

// ToolAnnotations provides hints about tool behavior per MCP spec 2025-03-26
type ToolAnnotations struct {
	Title           string `json:"title,omitempty"`
	ReadOnlyHint    *bool  `json:"readOnlyHint,omitempty"`
	DestructiveHint *bool  `json:"destructiveHint,omitempty"`
	IdempotentHint  *bool  `json:"idempotentHint,omitempty"`
	OpenWorldHint   *bool  `json:"openWorldHint,omitempty"`
}

// Tool represents an MCP tool definition
type Tool struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	InputSchema InputSchema      `json:"inputSchema"`
	Annotations *ToolAnnotations `json:"annotations,omitempty"`
}

// InputSchema defines the JSON schema for tool input
type InputSchema struct {
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
	Required   []string               `json:"required,omitempty"`
}

// ToolCallParams represents parameters for calling a tool
type ToolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// ToolResponse represents the response from a tool execution
type ToolResponse struct {
	Content []ContentItem `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// ContentItem represents a piece of content in a tool response
type ContentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Resource represents an MCP resource definition
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// ResourceReadParams represents parameters for reading a resource
type ResourceReadParams struct {
	URI string `json:"uri"`
}

// ResourceContent represents the content of a resource
type ResourceContent struct {
	URI      string        `json:"uri"`
	MimeType string        `json:"mimeType,omitempty"`
	Contents []ContentItem `json:"contents"`
}

// ToolsListResult represents the result of tools/list request
type ToolsListResult struct {
	Tools []Tool `json:"tools"`
}

// ResourcesListResult represents the result of resources/list request
type ResourcesListResult struct {
	Resources []Resource `json:"resources"`
}

// Prompt represents an MCP prompt definition
type Prompt struct {
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Arguments   []PromptArgument `json:"arguments,omitempty"`
}

// PromptArgument represents an argument for a prompt
type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
	Type        string `json:"type,omitempty"` // "string" (default), "boolean"
}

// PromptGetParams represents parameters for getting a prompt
type PromptGetParams struct {
	Name      string            `json:"name"`
	Arguments map[string]string `json:"arguments,omitempty"`
}

// PromptResult represents the result of getting a prompt
type PromptResult struct {
	Description string          `json:"description,omitempty"`
	Messages    []PromptMessage `json:"messages"`
}

// PromptMessage represents a message in a prompt template
type PromptMessage struct {
	Role    string      `json:"role"` // "user" or "assistant"
	Content ContentItem `json:"content"`
}

// PromptsListResult represents the result of prompts/list request
type PromptsListResult struct {
	Prompts []Prompt `json:"prompts"`
}

// --- MCP spec 2026-07-28: stateless, per-request protocol fields ---
//
// Revision 2026-07-28 removed the initialize/notifications/initialized
// handshake in favor of per-request metadata: every request carries its
// own protocol version and client capabilities in a reserved `_meta`
// object, and the server includes its own identity in every result's
// `_meta`. This server is dual-era: a request carrying
// io.modelcontextprotocol/protocolVersion in `_meta` is served under
// this (modern) model; a request without it -- including every
// `initialize` handshake -- is served under the pre-2026-07-28 (legacy)
// model this server already implemented. See docs/mcp-spec-compliance.md
// for the full compliance decision record, including what was
// deliberately not adopted and why.

// RequestMeta holds the io.modelcontextprotocol/* keys a modern request
// carries in its `_meta` object. ClientCapabilities is a required field
// on every modern request per spec; validateModernRequestStdio and
// validateModernRequestHTTP reject a nil value with -32602 before
// dispatch. This server does not otherwise inspect its contents or
// require any specific client capability to process any request it
// currently supports (it never uses sampling, elicitation, or roots) --
// only bare presence is checked, an empty object (`{}`) is accepted.
type RequestMeta struct {
	ProtocolVersion    string                 `json:"io.modelcontextprotocol/protocolVersion,omitempty"`
	ClientInfo         *ClientInfo            `json:"io.modelcontextprotocol/clientInfo,omitempty"`
	ClientCapabilities map[string]interface{} `json:"io.modelcontextprotocol/clientCapabilities,omitempty"`
	LogLevel           string                 `json:"io.modelcontextprotocol/logLevel,omitempty"`
}

// ResponseMeta holds the io.modelcontextprotocol/* keys this server
// includes in a modern result's `_meta` object.
type ResponseMeta struct {
	ServerInfo Implementation `json:"io.modelcontextprotocol/serverInfo"`
}

// metaHolder is used only to extract `_meta` from an arbitrary request's
// params without needing to know the rest of that request's shape.
type metaHolder struct {
	Meta *RequestMeta `json:"_meta,omitempty"`
}

// CacheableResult holds the fields the 2026-07-28 revision requires on
// every result returned by tools/list, resources/list, prompts/list, and
// resources/read: resultType (required on every result, not just
// cacheable ones), ttlMs, and cacheScope. This server's registries are
// populated once at startup and never change at runtime (custom
// tools/resources/prompts load from a definitions file read once during
// initialization), so every list is valid for the life of the process;
// TTLOneDay reflects that rather than an arbitrary guess.
type CacheableResult struct {
	ResultType string `json:"resultType"`
	TTLMs      int    `json:"ttlMs"`
	CacheScope string `json:"cacheScope"`
}

// ResultTypeComplete and CacheScopePublic/Private are the only values
// this server ever produces: it does not implement Multi Round-Trip
// Requests (no sampling/elicitation/roots), so no result is ever
// "input_required".
const (
	ResultTypeComplete = "complete"
	CacheScopePublic   = "public"
	CacheScopePrivate  = "private"
	// TTLOneDay is used for every cacheable result this server
	// returns: its registries never change after startup, so there is
	// no meaningful expiry, and this is a concrete number to report
	// rather than an unbounded one.
	TTLOneDay = 24 * 60 * 60 * 1000 // 24h in ms; see cacheScope note above
)

// DiscoverResult is the response to server/discover.
type DiscoverResult struct {
	CacheableResult
	SupportedVersions []string               `json:"supportedVersions"`
	Capabilities      map[string]interface{} `json:"capabilities"`
	Meta              ResponseMeta           `json:"_meta"`
	Instructions      string                 `json:"instructions,omitempty"`
}

// UnsupportedProtocolVersionErrorData is the `error.data` payload for a
// -32022 UnsupportedProtocolVersion error.
type UnsupportedProtocolVersionErrorData struct {
	Supported []string `json:"supported"`
	Requested string   `json:"requested"`
}

// MissingRequiredClientCapabilityErrorData is the `error.data` payload
// for a -32021 MissingRequiredClientCapability error. This server never
// emits this error today (see RequestMeta), but the type exists so a
// future request type that does need a client capability has it ready.
type MissingRequiredClientCapabilityErrorData struct {
	RequiredCapabilities map[string]interface{} `json:"requiredCapabilities"`
}
