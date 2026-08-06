/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

// Package openapi provides a programmatic OpenAPI 3.0.3 specification
// builder for the pgEdge Postgres MCP Server REST API.
package openapi

import (
	"pgedge-postgres-mcp/internal/mcp"
)

// M is a shorthand alias for building nested map structures.
type M = map[string]interface{}

// A is a shorthand alias for building slices in the spec.
type A = []interface{}

// BuildSpec returns the complete OpenAPI 3.0.3 specification as a
// map that can be serialized to JSON with encoding/json.
func BuildSpec() map[string]interface{} {
	return M{
		"openapi": "3.0.3",
		"info":    buildInfo(),
		"paths":   buildPaths(),
		"components": M{
			"securitySchemes": buildSecuritySchemes(),
			"schemas":         buildSchemas(),
			"parameters":      buildParameters(),
		},
		"tags": buildTags(),
	}
}

// buildInfo returns the info object for the specification.
func buildInfo() M {
	return M{
		"title":       "pgEdge Postgres MCP Server API",
		"description": "REST API for the pgEdge Postgres MCP Server, providing database management, MCP protocol access, LLM proxy, chat compaction, and conversation management.",
		"version":     mcp.ServerVersion,
		"license": M{
			"name": "PostgreSQL License",
		},
		"contact": M{
			"name": "pgEdge",
			"url":  "https://www.pgedge.com",
		},
	}
}

// buildTags returns the tag definitions used to group endpoints.
func buildTags() A {
	return A{
		M{"name": "Health", "description": "Server health check endpoints."},
		M{"name": "MCP", "description": "Model Context Protocol (JSON-RPC 2.0) endpoints."},
		M{"name": "Databases", "description": "Database listing and selection endpoints."},
		M{"name": "User", "description": "User information endpoints."},
		M{"name": "Chat", "description": "Chat compaction endpoints."},
		M{"name": "LLM Proxy", "description": "LLM provider and chat proxy endpoints."},
		M{"name": "Conversations", "description": "Conversation management endpoints."},
		M{"name": "OpenAPI", "description": "OpenAPI specification endpoint."},
	}
}

// buildSecuritySchemes returns the security scheme definitions.
func buildSecuritySchemes() M {
	return M{
		"BearerAuth": M{
			"type":         "http",
			"scheme":       "bearer",
			"description":  "Bearer token authentication. Required when auth is enabled on the server.",
			"bearerFormat": "token",
		},
	}
}

// bearerSecurity returns the security requirement for bearer auth.
func bearerSecurity() A {
	return A{M{"BearerAuth": A{}}}
}

// ref returns a JSON reference object pointing to a component schema.
func ref(name string) M {
	return M{"$ref": "#/components/schemas/" + name}
}

// refParam returns a JSON reference object pointing to a reusable
// component parameter.
func refParam(name string) M {
	return M{"$ref": "#/components/parameters/" + name}
}

// buildParameters returns the reusable header parameters the
// Streamable HTTP transport requires on a modern (2026-07-28) MCP
// request. They are optional at the OpenAPI level -- a legacy request
// (one whose body carries no _meta.protocolVersion) must not send
// them at all -- but conditionally required once a request carries
// them: see internal/mcp/modern.go's validateModernHeaders, which
// this documents.
func buildParameters() M {
	return M{
		"MCPProtocolVersionHeader": M{
			"name":        "MCP-Protocol-Version",
			"in":          "header",
			"required":    false,
			"description": "Required on a modern (2026-07-28) request; omit entirely for a legacy request. Must equal the request body's _meta.io.modelcontextprotocol/protocolVersion field, or the request is rejected with 400 and a HeaderMismatch (-32020) JSON-RPC error.",
			"schema": M{
				"type":    "string",
				"example": "2026-07-28",
			},
		},
		"McpMethodHeader": M{
			"name":        "Mcp-Method",
			"in":          "header",
			"required":    false,
			"description": "Required alongside MCP-Protocol-Version on a modern request. Must equal the JSON-RPC request body's method field, or the request is rejected with 400 and a HeaderMismatch (-32020) JSON-RPC error.",
			"schema": M{
				"type": "string",
			},
		},
		"McpNameHeader": M{
			"name":        "Mcp-Name",
			"in":          "header",
			"required":    false,
			"description": "Required on a modern tools/call, resources/read, or prompts/get request; not sent for any other method. Must equal the body's params.name (tools/call, prompts/get) or params.uri (resources/read), Base64-sentinel-encoded (\"=?base64?<value>?=\") if the raw value is not safely representable as a plain ASCII header.",
			"schema": M{
				"type": "string",
			},
		},
		"AcceptHeader": M{
			"name":        "Accept",
			"in":          "header",
			"required":    false,
			"description": "The Streamable HTTP transport requires a modern (2026-07-28) client to list both application/json and text/event-stream here, because a conforming server may answer any request with either a single JSON object or a request-scoped SSE stream. This server always answers with application/json -- it implements no streaming method -- and does not inspect this header, so a request that omits it is not rejected; it is documented because a client that talks to other MCP servers must send it to remain conforming.",
			"schema": M{
				"type":    "string",
				"example": "application/json, text/event-stream",
			},
		},
	}
}

// authErrorResponses returns the standard 401 and 403 error responses
// that apply to all endpoints requiring bearer authentication.
func authErrorResponses() M {
	return M{
		"401": M{
			"description": "Missing or invalid authentication token.",
			"content":     jsonContent(ref("ErrorResponse")),
		},
		"403": M{
			"description": "Insufficient permissions.",
			"content":     jsonContent(ref("ErrorResponse")),
		},
	}
}

// mergeResponses combines a base response map with the standard
// authentication error responses.
func mergeResponses(base M) M {
	for k, v := range authErrorResponses() {
		base[k] = v
	}
	return base
}

// jsonContent wraps a schema reference in an application/json content
// block suitable for request or response bodies.
func jsonContent(schemaRef M) M {
	return M{
		"application/json": M{
			"schema": schemaRef,
		},
	}
}

// buildPaths returns every path item in the specification.
func buildPaths() M {
	return M{
		"/health":                 buildHealthPath(),
		"/mcp/v1":                 buildMCPPath(),
		"/api/databases":          buildDatabasesPath(),
		"/api/databases/select":   buildDatabasesSelectPath(),
		"/api/user/info":          buildUserInfoPath(),
		"/api/chat/compact":       buildChatCompactPath(),
		"/api/llm/v1/providers":   buildLLMProvidersPath(),
		"/api/llm/v1/models":      buildLLMModelsPath(),
		"/api/llm/v1/chat":        buildLLMChatPath(),
		"/api/llm/v1/chat/stream": buildLLMChatStreamPath(),
		"/api/llm/v1/health":      buildLLMHealthPath(),
		"/api/conversations":      buildConversationsPath(),
		"/api/conversations/{id}": buildConversationByIDPath(),
		"/api/openapi.json":       buildOpenAPISpecPath(),
	}
}

// ---------------------------------------------------------------------------
// Path builders
// ---------------------------------------------------------------------------

func buildHealthPath() M {
	return M{
		"get": M{
			"tags":        A{"Health"},
			"summary":     "Check server health",
			"description": "Returns the current health status, server name, and version. No authentication is required.",
			"operationId": "getHealth",
			"responses": M{
				"200": M{
					"description": "Server is healthy.",
					"content":     jsonContent(ref("HealthResponse")),
				},
			},
		},
	}
}

func buildMCPPath() M {
	return M{
		"post": M{
			"tags":        A{"MCP"},
			"summary":     "Send an MCP JSON-RPC 2.0 request",
			"description": "Accepts a JSON-RPC 2.0 request implementing the Model Context Protocol. Supported methods are initialize, server/discover, ping, tools/list, tools/call, resources/list, resources/read, prompts/list, prompts/get, pgedge/listDatabases, and pgedge/selectDatabase. Bearer token authentication is required when auth is enabled. Two protocol revisions are served on this same endpoint: a request whose body carries _meta.io.modelcontextprotocol/protocolVersion (or, on HTTP, a present MCP-Protocol-Version header) is served under the current, stateless revision (2026-07-28), which requires the MCP-Protocol-Version, Mcp-Method, and (for tools/call, resources/read, prompts/get) Mcp-Name headers described below, and which also expects a conforming client to offer both application/json and text/event-stream in Accept; any other request is served under the original, handshake-based revision (2024-11-05), which sends none of those headers and negotiates via initialize instead. initialize itself exists only under that original revision: it is not a method the current revision recognizes.",
			"operationId": "postMCP",
			"security":    bearerSecurity(),
			"parameters": A{
				refParam("MCPProtocolVersionHeader"),
				refParam("McpMethodHeader"),
				refParam("McpNameHeader"),
				refParam("AcceptHeader"),
			},
			"requestBody": M{
				"required":    true,
				"description": "A JSON-RPC 2.0 request object.",
				"content":     jsonContent(ref("JSONRPCRequest")),
			},
			"responses": mergeResponses(M{
				"200": M{
					"description": "JSON-RPC 2.0 response. This includes a JSON-RPC-level error (for example, a tool's own argument validation failure) that is not one of the two rejections documented below.",
					"content":     jsonContent(ref("JSONRPCResponse")),
				},
				"400": M{
					"description": "The request identified itself as the current (2026-07-28) revision -- via body _meta or the MCP-Protocol-Version header -- and failed validation before dispatch: a required header is missing or does not match the request body (HeaderMismatch, -32020), a required _meta field is missing (-32602), or the requested protocol version is not supported (UnsupportedProtocolVersion, -32022).",
					"content":     jsonContent(ref("JSONRPCResponse")),
				},
				"404": M{
					"description": "The request identified itself as the current (2026-07-28) revision and named a method that revision does not recognize (-32601, Method not found). This includes initialize, which exists only under the original (2024-11-05) revision.",
					"content":     jsonContent(ref("JSONRPCResponse")),
				},
			}),
		},
	}
}

func buildDatabasesPath() M {
	return M{
		"get": M{
			"tags":        A{"Databases"},
			"summary":     "List available databases",
			"description": "Returns the list of configured database connections and the currently selected database.",
			"operationId": "listDatabases",
			"security":    bearerSecurity(),
			"responses": mergeResponses(M{
				"200": M{
					"description": "A list of available databases.",
					"content":     jsonContent(ref("ListDatabasesResponse")),
				},
			}),
		},
	}
}

func buildDatabasesSelectPath() M {
	return M{
		"post": M{
			"tags":        A{"Databases"},
			"summary":     "Select a database",
			"description": "Switches the active database connection to the specified database name.",
			"operationId": "selectDatabase",
			"security":    bearerSecurity(),
			"requestBody": M{
				"required":    true,
				"description": "The name of the database to select.",
				"content":     jsonContent(ref("SelectDatabaseRequest")),
			},
			"responses": mergeResponses(M{
				"200": M{
					"description": "Database selected successfully.",
					"content":     jsonContent(ref("SelectDatabaseResponse")),
				},
				"400": M{
					"description": "Invalid request body.",
					"content":     jsonContent(ref("ErrorResponse")),
				},
				"404": M{
					"description": "Database not found.",
					"content":     jsonContent(ref("ErrorResponse")),
				},
			}),
		},
	}
}

func buildUserInfoPath() M {
	return M{
		"get": M{
			"tags":        A{"User"},
			"summary":     "Get current user information",
			"description": "Returns authentication status and username. When no bearer token is provided the response indicates an unauthenticated session. This endpoint does not require authentication.",
			"operationId": "getUserInfo",
			"responses": M{
				"200": M{
					"description": "User information.",
					"content":     jsonContent(ref("UserInfoResponse")),
				},
			},
		},
	}
}

func buildChatCompactPath() M {
	return M{
		"post": M{
			"tags":        A{"Chat"},
			"summary":     "Compact a chat message history",
			"description": "Compacts a conversation's message history by removing or summarizing older messages while preserving important context such as anchors and tool results.",
			"operationId": "compactChat",
			"security":    bearerSecurity(),
			"requestBody": M{
				"required":    true,
				"description": "The messages to compact along with compaction options.",
				"content":     jsonContent(ref("CompactRequest")),
			},
			"responses": mergeResponses(M{
				"200": M{
					"description": "Compacted message history.",
					"content":     jsonContent(ref("CompactResponse")),
				},
			}),
		},
	}
}

func buildLLMProvidersPath() M {
	return M{
		"get": M{
			"tags":        A{"LLM Proxy"},
			"summary":     "List LLM providers",
			"description": "Returns the configured LLM providers (with each provider's default model) and the overall default provider name.",
			"operationId": "listLLMProviders",
			"security":    bearerSecurity(),
			"responses": mergeResponses(M{
				"200": M{
					"description": "Available LLM providers.",
					"content":     jsonContent(ref("ProvidersResponse")),
				},
			}),
		},
	}
}

func buildLLMModelsPath() M {
	return M{
		"get": M{
			"tags":        A{"LLM Proxy"},
			"summary":     "List models for a provider",
			"description": "Returns the models available for the specified LLM provider.",
			"operationId": "listLLMModels",
			"security":    bearerSecurity(),
			"parameters": A{
				M{
					"name":        "provider",
					"in":          "query",
					"required":    true,
					"description": "The LLM provider name.",
					"schema": M{
						"type": "string",
					},
				},
			},
			"responses": mergeResponses(M{
				"200": M{
					"description": "Available models for the provider.",
					"content":     jsonContent(ref("ModelsResponse")),
				},
			}),
		},
	}
}

func buildLLMChatPath() M {
	return M{
		"post": M{
			"tags":        A{"LLM Proxy"},
			"summary":     "Send a chat request to an LLM",
			"description": "Proxies a chat completion request to the specified LLM provider and model. Supports tool definitions for function calling.",
			"operationId": "llmChat",
			"security":    bearerSecurity(),
			"requestBody": M{
				"required":    true,
				"description": "The chat request including messages, optional tools, and LLM provider details.",
				"content":     jsonContent(ref("LLMChatRequest")),
			},
			"responses": mergeResponses(M{
				"200": M{
					"description": "LLM chat completion response.",
					"content":     jsonContent(ref("LLMChatResponse")),
				},
			}),
		},
	}
}

func buildLLMChatStreamPath() M {
	return M{
		"post": M{
			"tags":        A{"LLM Proxy"},
			"summary":     "Streaming chat (SSE) via the LLM proxy",
			"description": "Returns Server-Sent Events conforming to pgedge-go-llm-lib's SSE wire format. See https://github.com/pgEdge/pgedge-go-llm-lib for the full chunk/event schema.",
			"operationId": "llmChatStream",
			"security":    bearerSecurity(),
			"requestBody": M{
				"required": true,
				"content":  jsonContent(ref("LLMChatRequest")),
			},
			"responses": mergeResponses(M{
				"200": M{
					"description": "SSE stream of chat chunks. Each data line is a JSON-encoded llm.StreamChunk; a terminator 'event: done' or 'event: error' precedes connection close.",
					"content":     M{"text/event-stream": M{}},
				},
			}),
		},
	}
}

func buildLLMHealthPath() M {
	return M{
		"get": M{
			"tags":        A{"LLM Proxy"},
			"summary":     "Check provider connectivity",
			"operationId": "llmHealth",
			"security":    bearerSecurity(),
			"responses": mergeResponses(M{
				"200": M{"description": "All providers healthy"},
				"503": M{"description": "One or more providers unhealthy"},
			}),
		},
	}
}

func buildConversationsPath() M {
	return M{
		"get": M{
			"tags":        A{"Conversations"},
			"summary":     "List conversations",
			"description": "Returns a paginated list of conversations for the authenticated user.",
			"operationId": "listConversations",
			"security":    bearerSecurity(),
			"parameters": A{
				M{
					"name":        "limit",
					"in":          "query",
					"required":    false,
					"description": "Maximum number of conversations to return.",
					"schema": M{
						"type":    "integer",
						"default": 50,
					},
				},
				M{
					"name":        "offset",
					"in":          "query",
					"required":    false,
					"description": "Number of conversations to skip.",
					"schema": M{
						"type":    "integer",
						"default": 0,
					},
				},
			},
			"responses": mergeResponses(M{
				"200": M{
					"description": "A paginated list of conversations.",
					"content": jsonContent(M{
						"type": "object",
						"properties": M{
							"conversations": M{
								"type":        "array",
								"description": "The list of conversation summaries.",
								"items":       ref("ConversationSummary"),
							},
						},
					}),
				},
			}),
		},
		"post": M{
			"tags":        A{"Conversations"},
			"summary":     "Create a conversation",
			"description": "Creates a new conversation with the specified provider, model, connection, and initial messages.",
			"operationId": "createConversation",
			"security":    bearerSecurity(),
			"requestBody": M{
				"required":    true,
				"description": "The conversation to create.",
				"content":     jsonContent(ref("CreateConversationRequest")),
			},
			"responses": mergeResponses(M{
				"201": M{
					"description": "Conversation created successfully.",
					"content":     jsonContent(ref("Conversation")),
				},
			}),
		},
		"delete": M{
			"tags":        A{"Conversations"},
			"summary":     "Delete all conversations",
			"description": "Deletes all conversations for the authenticated user. Requires the query parameter all=true.",
			"operationId": "deleteAllConversations",
			"security":    bearerSecurity(),
			"parameters": A{
				M{
					"name":        "all",
					"in":          "query",
					"required":    true,
					"description": "Must be set to true to confirm deletion of all conversations.",
					"schema": M{
						"type": "boolean",
					},
				},
			},
			"responses": mergeResponses(M{
				"200": M{
					"description": "All conversations deleted.",
					"content":     jsonContent(ref("DeleteAllResponse")),
				},
			}),
		},
	}
}

func buildConversationByIDPath() M {
	return M{
		"parameters": A{
			M{
				"name":        "id",
				"in":          "path",
				"required":    true,
				"description": "The conversation identifier.",
				"schema": M{
					"type": "string",
				},
			},
		},
		"get": M{
			"tags":        A{"Conversations"},
			"summary":     "Get a conversation",
			"description": "Returns a single conversation including its full message history.",
			"operationId": "getConversation",
			"security":    bearerSecurity(),
			"responses": mergeResponses(M{
				"200": M{
					"description": "The requested conversation.",
					"content":     jsonContent(ref("Conversation")),
				},
			}),
		},
		"put": M{
			"tags":        A{"Conversations"},
			"summary":     "Update a conversation",
			"description": "Replaces the provider, model, connection, and messages of an existing conversation.",
			"operationId": "updateConversation",
			"security":    bearerSecurity(),
			"requestBody": M{
				"required":    true,
				"description": "The updated conversation data.",
				"content":     jsonContent(ref("CreateConversationRequest")),
			},
			"responses": mergeResponses(M{
				"200": M{
					"description": "Conversation updated successfully.",
					"content":     jsonContent(ref("Conversation")),
				},
			}),
		},
		"patch": M{
			"tags":        A{"Conversations"},
			"summary":     "Rename a conversation",
			"description": "Updates the title of an existing conversation.",
			"operationId": "renameConversation",
			"security":    bearerSecurity(),
			"requestBody": M{
				"required":    true,
				"description": "The new conversation title.",
				"content":     jsonContent(ref("RenameConversationRequest")),
			},
			"responses": mergeResponses(M{
				"200": M{
					"description": "Conversation renamed successfully.",
					"content":     jsonContent(ref("SuccessResponse")),
				},
			}),
		},
		"delete": M{
			"tags":        A{"Conversations"},
			"summary":     "Delete a conversation",
			"description": "Deletes a single conversation by identifier.",
			"operationId": "deleteConversation",
			"security":    bearerSecurity(),
			"responses": mergeResponses(M{
				"200": M{
					"description": "Conversation deleted successfully.",
					"content":     jsonContent(ref("SuccessResponse")),
				},
			}),
		},
	}
}

func buildOpenAPISpecPath() M {
	return M{
		"get": M{
			"tags":        A{"OpenAPI"},
			"summary":     "Get the OpenAPI specification",
			"description": "Returns this OpenAPI 3.0.3 specification as a JSON document. No authentication is required.",
			"operationId": "getOpenAPISpec",
			"responses": M{
				"200": M{
					"description": "The OpenAPI specification.",
					"content": M{
						"application/json": M{
							"schema": M{
								"type":        "object",
								"description": "An OpenAPI 3.0.3 specification document.",
							},
						},
					},
				},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Component schemas
// ---------------------------------------------------------------------------

// buildSchemas returns all reusable schema definitions.
func buildSchemas() M {
	return M{
		"ErrorResponse":             schemaErrorResponse(),
		"HealthResponse":            schemaHealthResponse(),
		"JSONRPCRequest":            schemaJSONRPCRequest(),
		"JSONRPCResponse":           schemaJSONRPCResponse(),
		"RPCError":                  schemaRPCError(),
		"DatabaseInfo":              schemaDatabaseInfo(),
		"ListDatabasesResponse":     schemaListDatabasesResponse(),
		"SelectDatabaseRequest":     schemaSelectDatabaseRequest(),
		"SelectDatabaseResponse":    schemaSelectDatabaseResponse(),
		"UserInfoResponse":          schemaUserInfoResponse(),
		"ChatMessage":               schemaChatMessage(),
		"CompactRequest":            schemaCompactRequest(),
		"CompactResponse":           schemaCompactResponse(),
		"ProviderInfo":              schemaProviderInfo(),
		"ProvidersResponse":         schemaProvidersResponse(),
		"ModelInfo":                 schemaModelInfo(),
		"ModelsResponse":            schemaModelsResponse(),
		"LLMChatRequest":            schemaLLMChatRequest(),
		"LLMChatResponse":           schemaLLMChatResponse(),
		"LLMMessage":                schemaLLMMessage(),
		"LLMContentBlock":           schemaLLMContentBlock(),
		"LLMTool":                   schemaLLMTool(),
		"LLMTokenUsage":             schemaLLMTokenUsage(),
		"ConversationSummary":       schemaConversationSummary(),
		"Conversation":              schemaConversation(),
		"CreateConversationRequest": schemaCreateConversationRequest(),
		"RenameConversationRequest": schemaRenameConversationRequest(),
		"SuccessResponse":           schemaSuccessResponse(),
		"DeleteAllResponse":         schemaDeleteAllResponse(),
	}
}

func schemaErrorResponse() M {
	return M{
		"type":        "object",
		"description": "A generic error response.",
		"properties": M{
			"error": M{
				"type":        "string",
				"description": "A human-readable error message.",
			},
		},
		"required": A{"error"},
	}
}

func schemaHealthResponse() M {
	return M{
		"type":        "object",
		"description": "Server health check response.",
		"properties": M{
			"status": M{
				"type":        "string",
				"description": "Health status indicator.",
				"example":     "ok",
			},
			"server": M{
				"type":        "string",
				"description": "The server name.",
				"example":     mcp.ServerName,
			},
			"version": M{
				"type":        "string",
				"description": "The server version.",
				"example":     mcp.ServerVersion,
			},
		},
		"required": A{"status", "server", "version"},
	}
}

func schemaJSONRPCRequest() M {
	return M{
		"type":        "object",
		"description": "A JSON-RPC 2.0 request object for the MCP protocol.",
		"properties": M{
			"jsonrpc": M{
				"type":        "string",
				"description": "The JSON-RPC protocol version. Must be \"2.0\".",
				"enum":        A{"2.0"},
			},
			"id": M{
				"description": "A unique request identifier. Can be a string or integer.",
			},
			"method": M{
				"type":        "string",
				"description": "The MCP method to invoke.",
				"enum": A{
					"initialize",
					"server/discover",
					"ping",
					"tools/list",
					"tools/call",
					"resources/list",
					"resources/read",
					"prompts/list",
					"prompts/get",
					"pgedge/listDatabases",
					"pgedge/selectDatabase",
				},
			},
			"params": M{
				"type":        "object",
				"description": "Optional parameters for the method.",
			},
		},
		"required": A{"jsonrpc", "id", "method"},
	}
}

func schemaJSONRPCResponse() M {
	return M{
		"type":        "object",
		"description": "A JSON-RPC 2.0 response object.",
		"properties": M{
			"jsonrpc": M{
				"type":        "string",
				"description": "The JSON-RPC protocol version.",
				"enum":        A{"2.0"},
			},
			"id": M{
				"description": "The request identifier that this response corresponds to.",
			},
			"result": M{
				"description": "The result of the method invocation. Present on success.",
			},
			"error": M{
				"description": "Error information. Present on failure.",
				"allOf":       A{ref("RPCError")},
			},
		},
		"required": A{"jsonrpc", "id"},
	}
}

func schemaRPCError() M {
	return M{
		"type":        "object",
		"description": "A JSON-RPC 2.0 error object.",
		"properties": M{
			"code": M{
				"type":        "integer",
				"description": "A numeric error code.",
			},
			"message": M{
				"type":        "string",
				"description": "A short description of the error.",
			},
			"data": M{
				"description": "Additional error data.",
			},
		},
		"required": A{"code", "message"},
	}
}

func schemaDatabaseInfo() M {
	return M{
		"type":        "object",
		"description": "Information about a configured database connection.",
		"properties": M{
			"name": M{
				"type":        "string",
				"description": "The logical name of the database connection.",
			},
			"host": M{
				"type":        "string",
				"description": "The database server hostname.",
			},
			"port": M{
				"type":        "integer",
				"description": "The database server port.",
			},
			"database": M{
				"type":        "string",
				"description": "The database name.",
			},
			"user": M{
				"type":        "string",
				"description": "The database user.",
			},
			"sslmode": M{
				"type":        "string",
				"description": "The SSL mode for the connection.",
			},
			"allow_writes": M{
				"type":        "boolean",
				"description": "Whether write operations are permitted on this connection.",
			},
		},
		"required": A{
			"name", "host", "port", "database",
			"user", "sslmode", "allow_writes",
		},
	}
}

func schemaListDatabasesResponse() M {
	return M{
		"type":        "object",
		"description": "Response containing the list of databases and the currently selected database.",
		"properties": M{
			"databases": M{
				"type":        "array",
				"description": "The available database connections.",
				"items":       ref("DatabaseInfo"),
			},
			"current": M{
				"type":        "string",
				"description": "The name of the currently selected database.",
			},
		},
		"required": A{"databases", "current"},
	}
}

func schemaSelectDatabaseRequest() M {
	return M{
		"type":        "object",
		"description": "Request to select a database by name.",
		"properties": M{
			"name": M{
				"type":        "string",
				"description": "The name of the database to select.",
			},
		},
		"required": A{"name"},
	}
}

func schemaSelectDatabaseResponse() M {
	return M{
		"type":        "object",
		"description": "Response after selecting a database.",
		"properties": M{
			"success": M{
				"type":        "boolean",
				"description": "Whether the database was selected successfully.",
			},
			"current": M{
				"type":        "string",
				"description": "The name of the now-current database.",
			},
			"message": M{
				"type":        "string",
				"description": "A human-readable status message.",
			},
			"error": M{
				"type":        "string",
				"description": "An error message if the operation failed.",
			},
		},
		"required": A{"success", "current", "message"},
	}
}

func schemaUserInfoResponse() M {
	return M{
		"type":        "object",
		"description": "User authentication and identity information.",
		"properties": M{
			"authenticated": M{
				"type":        "boolean",
				"description": "Whether the user is authenticated.",
			},
			"username": M{
				"type":        "string",
				"description": "The username of the authenticated user. Absent when not authenticated.",
			},
			"error": M{
				"type":        "string",
				"description": "An error message if user lookup failed.",
			},
		},
		"required": A{"authenticated"},
	}
}

func schemaChatMessage() M {
	return M{
		"type":        "object",
		"description": "A single chat message with a role and content.",
		"properties": M{
			"role": M{
				"type":        "string",
				"description": "The role of the message author (e.g., user, assistant, system).",
			},
			"content": M{
				"type":        "string",
				"description": "The text content of the message.",
			},
		},
		"required": A{"role", "content"},
	}
}

func schemaCompactRequest() M {
	return M{
		"type":        "object",
		"description": "Request to compact a chat message history.",
		"properties": M{
			"messages": M{
				"type":        "array",
				"description": "The messages to compact.",
				"items":       ref("ChatMessage"),
			},
			"max_tokens": M{
				"type":        "integer",
				"description": "The maximum token budget for the compacted output.",
				"default":     100000,
			},
			"recent_window": M{
				"type":        "integer",
				"description": "The number of recent messages to always preserve.",
				"default":     10,
			},
			"keep_anchors": M{
				"type":        "boolean",
				"description": "Whether to preserve anchor messages during compaction.",
				"default":     true,
			},
			"options": M{
				"type":        "object",
				"description": "Advanced compaction options.",
				"properties": M{
					"preserve_tool_results": M{
						"type":        "boolean",
						"description": "Keep messages containing tool results.",
					},
					"preserve_schema_info": M{
						"type":        "boolean",
						"description": "Keep messages containing schema information.",
					},
					"enable_summarization": M{
						"type":        "boolean",
						"description": "Enable rule-based summarisation of dropped messages.",
					},
					"min_important_messages": M{
						"type":        "integer",
						"description": "Minimum number of important messages to retain.",
					},
					"token_counter_type": M{
						"type":        "string",
						"description": "The token counting strategy to use.",
					},
					"enable_llm_summarization": M{
						"type":        "boolean",
						"description": "Enable LLM-based summarisation of dropped messages.",
					},
					"enable_caching": M{
						"type":        "boolean",
						"description": "Enable caching of compaction results.",
					},
					"enable_analytics": M{
						"type":        "boolean",
						"description": "Enable analytics collection for compaction.",
					},
				},
			},
		},
		"required": A{"messages"},
	}
}

func schemaCompactResponse() M {
	return M{
		"type":        "object",
		"description": "Result of a chat compaction operation.",
		"properties": M{
			"messages": M{
				"type":        "array",
				"description": "The compacted messages.",
				"items":       ref("ChatMessage"),
			},
			"summary": M{
				"type":        "object",
				"description": "A summary of the topics discussed in dropped messages.",
				"properties": M{
					"topics": M{
						"type":        "array",
						"description": "Topic keywords extracted from the conversation.",
						"items":       M{"type": "string"},
					},
					"tables": M{
						"type":        "array",
						"description": "Database tables referenced in the conversation.",
						"items":       M{"type": "string"},
					},
					"tools": M{
						"type":        "array",
						"description": "Tools used during the conversation.",
						"items":       M{"type": "string"},
					},
					"description": M{
						"type":        "string",
						"description": "A prose description of the conversation context.",
					},
				},
			},
			"token_estimate": M{
				"type":        "integer",
				"description": "Estimated token count of the compacted output.",
			},
			"compaction_info": M{
				"type":        "object",
				"description": "Statistics about the compaction operation.",
				"properties": M{
					"original_count": M{
						"type":        "integer",
						"description": "Number of messages before compaction.",
					},
					"compacted_count": M{
						"type":        "integer",
						"description": "Number of messages after compaction.",
					},
					"dropped_count": M{
						"type":        "integer",
						"description": "Number of messages dropped during compaction.",
					},
					"anchor_count": M{
						"type":        "integer",
						"description": "Number of anchor messages preserved.",
					},
					"tokens_saved": M{
						"type":        "integer",
						"description": "Estimated tokens saved by compaction.",
					},
					"compression_ratio": M{
						"type":        "number",
						"description": "Ratio of output size to input size.",
					},
				},
			},
		},
		"required": A{"messages", "summary", "token_estimate", "compaction_info"},
	}
}

func schemaProviderInfo() M {
	return M{
		"type":        "object",
		"description": "Information about an LLM provider.",
		"properties": M{
			"name": M{
				"type":        "string",
				"description": "The provider identifier.",
			},
			"model": M{
				"type":        "string",
				"description": "The default model for this provider.",
			},
			"default": M{
				"type":        "boolean",
				"description": "Whether this provider is the default.",
			},
		},
		"required": A{"name", "model", "default"},
	}
}

func schemaProvidersResponse() M {
	return M{
		"type":        "object",
		"description": "Response listing available LLM providers.",
		"properties": M{
			"providers": M{
				"type":        "array",
				"description": "The available LLM providers.",
				"items":       ref("ProviderInfo"),
			},
			"default_provider": M{
				"type":        "string",
				"description": "The name of the default provider.",
			},
		},
		"required": A{"providers", "default_provider"},
	}
}

func schemaModelInfo() M {
	return M{
		"type":        "object",
		"description": "Information about an LLM model.",
		"properties": M{
			"name": M{
				"type":        "string",
				"description": "The model identifier.",
			},
			"description": M{
				"type":        "string",
				"description": "A short description of the model.",
			},
		},
		"required": A{"name", "description"},
	}
}

func schemaModelsResponse() M {
	return M{
		"type":        "object",
		"description": "Response listing models for an LLM provider.",
		"properties": M{
			"models": M{
				"type":        "array",
				"description": "The available model identifiers.",
				"items":       M{"type": "string"},
			},
		},
		"required": A{"models"},
	}
}

func schemaLLMChatRequest() M {
	return M{
		"type":     "object",
		"required": A{"messages"},
		"properties": M{
			"messages": M{
				"type":  "array",
				"items": ref("LLMMessage"),
			},
			"tools":          M{"type": "array", "items": ref("LLMTool")},
			"system_prompt":  M{"type": "string"},
			"max_tokens":     M{"type": "integer"},
			"temperature":    M{"type": "number"},
			"provider":       M{"type": "string", "description": "Override default provider"},
			"model":          M{"type": "string", "description": "Override default model"},
			"stop_sequences": M{"type": "array", "items": M{"type": "string"}},
		},
	}
}

func schemaLLMChatResponse() M {
	return M{
		"type":     "object",
		"required": A{"content", "stop_reason"},
		"properties": M{
			"content":     M{"type": "array", "items": ref("LLMContentBlock")},
			"stop_reason": M{"type": "string", "enum": A{"end_turn", "max_tokens", "stop_sequence", "tool_use", "content_filter", "error"}},
			"usage":       ref("LLMTokenUsage"),
		},
	}
}

func schemaLLMMessage() M {
	return M{
		"type":     "object",
		"required": A{"role", "content"},
		"properties": M{
			"role":    M{"type": "string", "enum": A{"user", "assistant", "system", "tool"}},
			"content": M{"type": "array", "items": ref("LLMContentBlock")},
		},
	}
}

func schemaLLMContentBlock() M {
	return M{
		"type":     "object",
		"required": A{"type"},
		"properties": M{
			"type": M{"type": "string", "enum": A{"text", "image", "document", "tool_use", "tool_result"}},
			"text": M{"type": "string"},
			"tool_use": M{
				"type":        "object",
				"description": "Populated when type=tool_use",
				"properties": M{
					"id":    M{"type": "string"},
					"name":  M{"type": "string"},
					"input": M{"type": "object"},
				},
			},
			"tool_use_id": M{"type": "string", "description": "Populated when type=tool_result"},
			"is_error":    M{"type": "boolean", "description": "Populated when type=tool_result"},
		},
	}
}

func schemaLLMTool() M {
	return M{
		"type":     "object",
		"required": A{"name", "description", "input_schema"},
		"properties": M{
			"name":         M{"type": "string"},
			"description":  M{"type": "string"},
			"input_schema": M{"type": "object"},
		},
	}
}

func schemaLLMTokenUsage() M {
	return M{
		"type": "object",
		"properties": M{
			"prompt_tokens":     M{"type": "integer"},
			"completion_tokens": M{"type": "integer"},
			"total_tokens":      M{"type": "integer"},
		},
	}
}

func schemaConversationSummary() M {
	return M{
		"type":        "object",
		"description": "A summary of a conversation for list views.",
		"properties": M{
			"id": M{
				"type":        "string",
				"description": "The unique conversation identifier.",
			},
			"title": M{
				"type":        "string",
				"description": "The conversation title.",
			},
			"connection": M{
				"type":        "string",
				"description": "The database connection name used in the conversation.",
			},
			"created_at": M{
				"type":        "string",
				"format":      "date-time",
				"description": "When the conversation was created.",
			},
			"updated_at": M{
				"type":        "string",
				"format":      "date-time",
				"description": "When the conversation was last updated.",
			},
			"preview": M{
				"type":        "string",
				"description": "A short preview of the conversation content.",
			},
		},
		"required": A{
			"id", "title", "connection",
			"created_at", "updated_at", "preview",
		},
	}
}

func schemaConversation() M {
	return M{
		"type":        "object",
		"description": "A full conversation object including messages.",
		"properties": M{
			"id": M{
				"type":        "string",
				"description": "The unique conversation identifier.",
			},
			"username": M{
				"type":        "string",
				"description": "The username that owns the conversation.",
			},
			"title": M{
				"type":        "string",
				"description": "The conversation title.",
			},
			"provider": M{
				"type":        "string",
				"description": "The LLM provider used.",
			},
			"model": M{
				"type":        "string",
				"description": "The LLM model used.",
			},
			"connection": M{
				"type":        "string",
				"description": "The database connection name.",
			},
			"messages": M{
				"type":        "array",
				"description": "The conversation messages.",
				"items":       ref("ChatMessage"),
			},
			"created_at": M{
				"type":        "string",
				"format":      "date-time",
				"description": "When the conversation was created.",
			},
			"updated_at": M{
				"type":        "string",
				"format":      "date-time",
				"description": "When the conversation was last updated.",
			},
		},
		"required": A{
			"id", "username", "title", "provider", "model",
			"connection", "messages", "created_at", "updated_at",
		},
	}
}

func schemaCreateConversationRequest() M {
	return M{
		"type":        "object",
		"description": "Request to create or update a conversation.",
		"properties": M{
			"provider": M{
				"type":        "string",
				"description": "The LLM provider to use.",
			},
			"model": M{
				"type":        "string",
				"description": "The LLM model to use.",
			},
			"connection": M{
				"type":        "string",
				"description": "The database connection name.",
			},
			"messages": M{
				"type":        "array",
				"description": "The conversation messages.",
				"items":       ref("ChatMessage"),
			},
		},
		"required": A{"provider", "model", "connection", "messages"},
	}
}

func schemaRenameConversationRequest() M {
	return M{
		"type":        "object",
		"description": "Request to rename a conversation.",
		"properties": M{
			"title": M{
				"type":        "string",
				"description": "The new title for the conversation.",
			},
		},
		"required": A{"title"},
	}
}

func schemaSuccessResponse() M {
	return M{
		"type":        "object",
		"description": "A generic success response.",
		"properties": M{
			"success": M{
				"type":        "boolean",
				"description": "Whether the operation succeeded.",
			},
		},
		"required": A{"success"},
	}
}

func schemaDeleteAllResponse() M {
	return M{
		"type":        "object",
		"description": "Response after deleting all conversations.",
		"properties": M{
			"success": M{
				"type":        "boolean",
				"description": "Whether the operation succeeded.",
			},
			"deleted": M{
				"type":        "integer",
				"description": "The number of conversations deleted.",
			},
		},
		"required": A{"success", "deleted"},
	}
}
