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
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
)

// ModernProtocolVersion is the protocol revision this server speaks
// under the stateless, per-request model introduced in 2026-07-28.
// SupportedModernProtocolVersions is deliberately a separate list from
// SupportedProtocolVersions (server.go): the two are different
// negotiation mechanisms -- legacy negotiation picks a nearby version
// for a connection-scoped initialize handshake, while modern
// negotiation is an exact match checked independently on every
// request, per the versioning spec's "no negotiation handshake" model.
const ModernProtocolVersion = "2026-07-28"

// SupportedModernProtocolVersions lists every modern (stateless,
// per-request) protocol revision this server implements. Reported
// verbatim as server/discover's supportedVersions.
var SupportedModernProtocolVersions = []string{ModernProtocolVersion}

// extractMeta pulls `_meta` out of an arbitrary request's params. A nil
// return (or a non-nil RequestMeta with an empty ProtocolVersion) means
// the request is legacy: either it carries no `_meta` at all, or it
// carries a `_meta` used for something else (e.g. only progressToken)
// without the protocol version field that marks a request as modern.
func extractMeta(params interface{}) *RequestMeta {
	if params == nil {
		return nil
	}
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil
	}
	var holder metaHolder
	if err := json.Unmarshal(paramsJSON, &holder); err != nil {
		return nil
	}
	return holder.Meta
}

// isModernRequest reports whether a request should be served under the
// 2026-07-28 stateless model, and returns its parsed `_meta` when it is.
// The presence of io.modelcontextprotocol/protocolVersion is what marks
// a request as modern; every pre-2026-07-28 client, including every
// initialize handshake, omits it and is served under this server's
// existing (legacy) behavior, unchanged.
func isModernRequest(params interface{}) (*RequestMeta, bool) {
	meta := extractMeta(params)
	if meta == nil || meta.ProtocolVersion == "" {
		return nil, false
	}
	return meta, true
}

// isModernHTTP is isModernRequest for the HTTP transport, which also
// treats a present MCP-Protocol-Version header as marking a request
// modern, even when the body's _meta does not (because it is missing
// entirely, or because its protocolVersion key is missing or
// misspelled -- these long, easily-mistyped reserved keys make that a
// real failure mode, not just a hypothetical one). No pre-2025-06-18
// client -- including this server's own legacy revision, 2024-11-05 --
// ever sends this header at all, so its mere presence, regardless of
// value, is already a reliable modern signal on its own; the exact
// version match is left to validateModernHeaders. Returns a nil meta
// when the header is the only modern signal, which validateModernHeaders
// and validateModernRequestHTTP both treat as "no usable body _meta",
// not as "no _meta was expected."
func isModernHTTP(r *http.Request, params interface{}) (*RequestMeta, bool) {
	if meta, isModern := isModernRequest(params); isModern {
		return meta, true
	}
	if r.Header.Get("MCP-Protocol-Version") != "" {
		return nil, true
	}
	return nil, false
}

// isModernVersionSupported reports whether this server implements the
// given modern protocol revision. Unlike legacy negotiation
// (NegotiateProtocolVersion), there is no nearest-version fallback: the
// spec requires an exact match, with the client responsible for
// retrying against the `supported` list an UnsupportedProtocolVersionError
// carries.
func isModernVersionSupported(version string) bool {
	for _, v := range SupportedModernProtocolVersions {
		if v == version {
			return true
		}
	}
	return false
}

// responseMetaFor builds the `_meta` object this server includes on
// every modern result, identifying itself per the spec's serverInfo
// convention.
func responseMetaFor() ResponseMeta {
	return ResponseMeta{
		ServerInfo: Implementation{
			Name:    ServerName,
			Version: ServerVersion,
		},
	}
}

// cacheableResult builds the resultType/ttlMs/cacheScope fields every
// modern list/read result carries. See CacheableResult's doc comment
// for why every result this server returns is cacheable indefinitely
// and public: nothing here varies per caller or changes after startup.
func cacheableResult() CacheableResult {
	return CacheableResult{
		ResultType: ResultTypeComplete,
		TTLMs:      TTLOneDay,
		CacheScope: CacheScopePublic,
	}
}

// resourceNotFoundCode returns the era-appropriate JSON-RPC error code
// for an unknown resource URI: -32602 (Invalid params) for a modern
// (2026-07-28) request, matching that revision's renumbering, or -32002
// -- the code every earlier revision, including this server's declared
// 2024-11-05, defined for exactly this case -- for a legacy request.
// -32002 is deliberately not reused for modern requests: the 2026-07-28
// spec reserves it and requires implementations of that revision not to
// emit it.
func resourceNotFoundCode(req JSONRPCRequest) int {
	if _, isModern := isModernRequest(req.Params); isModern {
		return -32602
	}
	return -32002
}

// unsupportedProtocolVersionError builds the -32022 error data listing
// this server's supported modern revisions, per the versioning spec.
func unsupportedProtocolVersionError(requested string) (int, string, interface{}) {
	return -32022, "Unsupported protocol version", UnsupportedProtocolVersionErrorData{
		Supported: SupportedModernProtocolVersions,
		Requested: requested,
	}
}

// mergeModernFields marshals an existing typed result to a map and
// merges in the fields a modern result must carry, rather than defining
// a parallel "Modern*Result" struct for every existing result type.
// Every result gets resultType and _meta.serverInfo; cacheable adds
// ttlMs/cacheScope on top, for the subset of methods the spec classifies
// as cacheable (tools/list, resources/list, prompts/list, resources/read).
func mergeModernFields(result interface{}, cacheable bool) (map[string]interface{}, error) {
	data, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}

	m["resultType"] = ResultTypeComplete
	if cacheable {
		cr := cacheableResult()
		m["ttlMs"] = cr.TTLMs
		m["cacheScope"] = cr.CacheScope
	}

	meta, _ := m["_meta"].(map[string]interface{})
	if meta == nil {
		meta = map[string]interface{}{}
	}
	meta["io.modelcontextprotocol/serverInfo"] = responseMetaFor().ServerInfo
	m["_meta"] = meta

	return m, nil
}

// wrapModernResult adds resultType and _meta.serverInfo to a result
// from a method the spec does not classify as cacheable (tools/call,
// prompts/get, and this server's pgedge/* extensions). Falls back to
// the unwrapped result on a marshal error, which should not happen for
// any result type this server produces; sendResponseForRequest and
// wrapModernHTTPResult treat that fallback as "serve it unwrapped
// rather than not at all."
func wrapModernResult(result interface{}) interface{} {
	wrapped, err := mergeModernFields(result, false)
	if err != nil {
		return result
	}
	return wrapped
}

// wrapModernCacheableResult is wrapModernResult for the subset of
// methods the spec classifies as cacheable: tools/list, resources/list,
// prompts/list, and resources/read.
func wrapModernCacheableResult(result interface{}) interface{} {
	wrapped, err := mergeModernFields(result, true)
	if err != nil {
		return result
	}
	return wrapped
}

// methodsRequiringMcpName lists the methods for which the Streamable
// HTTP transport spec requires an Mcp-Name header, mirroring
// params.name (tools/call, prompts/get) or params.uri (resources/read).
var methodsRequiringMcpName = map[string]bool{
	"tools/call":     true,
	"resources/read": true,
	"prompts/get":    true,
}

// nameOrURI extracts params.name or params.uri from an arbitrary
// request's params, for validating the Mcp-Name header. Returns "" if
// neither is present or params does not marshal to an object.
func nameOrURI(params interface{}) string {
	if params == nil {
		return ""
	}
	data, err := json.Marshal(params)
	if err != nil {
		return ""
	}
	var holder struct {
		Name string `json:"name"`
		URI  string `json:"uri"`
	}
	if err := json.Unmarshal(data, &holder); err != nil {
		return ""
	}
	if holder.Name != "" {
		return holder.Name
	}
	return holder.URI
}

// headerMismatch is a small helper so every failure branch in
// validateModernHeaders reports the same error shape without repeating
// the RPCError literal.
func headerMismatch(message string) *RPCError {
	return &RPCError{Code: -32020, Message: "Header mismatch: " + message}
}

// validateModernHeaders checks the Streamable HTTP transport's required
// headers (MCP-Protocol-Version, Mcp-Method, and -- for tools/call,
// resources/read, and prompts/get -- Mcp-Name) against the parsed
// request body, per the header-mirroring rules in the transport spec.
// Returns nil when validation passes. Only called for requests already
// identified as modern (isModernRequest); legacy requests never carry
// these headers and are not checked.
func validateModernHeaders(r *http.Request, req JSONRPCRequest, meta *RequestMeta) *RPCError {
	protocolHeader := r.Header.Get("MCP-Protocol-Version")
	if protocolHeader == "" {
		return headerMismatch("MCP-Protocol-Version header is required")
	}
	// meta is nil here exactly when the request is only modern because
	// of the header (see isModernHTTP): the body carries no usable
	// _meta.protocolVersion at all, whether because _meta is absent
	// entirely or because its protocolVersion key is missing or
	// misspelled. Either way there is nothing to compare the header
	// against, and the missing body field -- not a header/body
	// mismatch -- is the actual problem.
	if meta == nil {
		return &RPCError{Code: -32602, Message: "Invalid params",
			Data: "io.modelcontextprotocol/protocolVersion is required in _meta when the MCP-Protocol-Version header is present"}
	}
	if protocolHeader != meta.ProtocolVersion {
		return headerMismatch("MCP-Protocol-Version header value '" + protocolHeader +
			"' does not match body value '" + meta.ProtocolVersion + "'")
	}

	methodHeader := r.Header.Get("Mcp-Method")
	if methodHeader == "" {
		return headerMismatch("Mcp-Method header is required")
	}
	if methodHeader != req.Method {
		return headerMismatch("Mcp-Method header value '" + methodHeader +
			"' does not match body value '" + req.Method + "'")
	}

	if !methodsRequiringMcpName[req.Method] {
		return nil
	}

	rawNameHeader := r.Header.Get("Mcp-Name")
	if rawNameHeader == "" {
		return headerMismatch("Mcp-Name header is required for " + req.Method)
	}
	nameHeader, ok := decodeHeaderValue(rawNameHeader)
	if !ok {
		return headerMismatch("Mcp-Name header is not validly Base64-encoded")
	}
	bodyValue := nameOrURI(req.Params)
	if nameHeader != bodyValue {
		return headerMismatch("Mcp-Name header value '" + nameHeader +
			"' does not match body value '" + bodyValue + "'")
	}

	return nil
}

// cacheableMethods lists the RPC methods the 2026-07-28 spec classifies
// as cacheable: their modern results carry ttlMs/cacheScope in addition
// to resultType. This server has no resources/templates/list (the fifth
// method the spec lists), since it has no resource templates.
var cacheableMethods = map[string]bool{
	"tools/list":     true,
	"resources/list": true,
	"prompts/list":   true,
	"resources/read": true,
}

// wrapModernResultForMethod wraps a successful result for a modern
// request, choosing the cacheable or non-cacheable field set by method
// name. Called once, centrally, in handleHTTPRequest -- every HTTP
// handler (handleToolsListHTTP, handleResourceReadHTTP, etc.) is
// unaware of era and returns its normal result either way.
func wrapModernResultForMethod(method string, result interface{}) interface{} {
	if cacheableMethods[method] {
		return wrapModernCacheableResult(result)
	}
	return wrapModernResult(result)
}

// validateModernRequestStdio runs the checks a modern request needs
// before dispatch on the stdio transport: presence of the required
// clientCapabilities field, and that the requested protocol version is
// one this server implements. There are no HTTP headers to validate on
// this transport.
func validateModernRequestStdio(meta *RequestMeta) *RPCError {
	if meta.ClientCapabilities == nil {
		return &RPCError{Code: -32602, Message: "Invalid params",
			Data: "io.modelcontextprotocol/clientCapabilities is required in _meta for a modern request"}
	}
	if !isModernVersionSupported(meta.ProtocolVersion) {
		code, message, data := unsupportedProtocolVersionError(meta.ProtocolVersion)
		return &RPCError{Code: code, Message: message, Data: data}
	}
	return nil
}

// validateModernRequestHTTP runs every check the Streamable HTTP
// transport spec requires before a modern request reaches its handler:
// header/body agreement, presence of the required clientCapabilities
// field, and that the requested protocol version is one this server
// implements. Returns nil when the request may proceed.
func validateModernRequestHTTP(r *http.Request, req JSONRPCRequest, meta *RequestMeta) *RPCError {
	if rpcErr := validateModernHeaders(r, req, meta); rpcErr != nil {
		return rpcErr
	}
	if meta.ClientCapabilities == nil {
		return &RPCError{Code: -32602, Message: "Invalid params",
			Data: "io.modelcontextprotocol/clientCapabilities is required in _meta for a modern request"}
	}
	if !isModernVersionSupported(meta.ProtocolVersion) {
		code, message, data := unsupportedProtocolVersionError(meta.ProtocolVersion)
		return &RPCError{Code: code, Message: message, Data: data}
	}
	return nil
}

// decodeHeaderValue reverses the Base64 sentinel encoding the
// Streamable HTTP transport spec requires for header values that
// cannot be represented as plain ASCII (Mcp-Name and Mcp-Param-*): a
// value wrapped as "=?base64?<encoded>?=" is base64-decoded; any other
// value is returned unchanged, since the sentinel is only used when
// plain ASCII would not round-trip safely.
func decodeHeaderValue(v string) (string, bool) {
	const prefix, suffix = "=?base64?", "?="
	// The length check must come first: prefix and suffix share a "?",
	// so a value shorter than len(prefix)+len(suffix) (e.g. "=?base64?=")
	// can satisfy both HasPrefix and HasSuffix by overlapping on that
	// character, which would make the slice below negative-length.
	if len(v) < len(prefix)+len(suffix) || !strings.HasPrefix(v, prefix) || !strings.HasSuffix(v, suffix) {
		return v, true
	}
	encoded := v[len(prefix) : len(v)-len(suffix)]
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", false
	}
	return string(decoded), true
}
