/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

// Package tracing provides JSONL trace logging for all MCP interactions.
// It records tool calls, resource reads, HTTP requests, session lifecycle
// events, and LLM interactions to a structured trace file for debugging
// and auditing.
package tracing

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// sensitiveParamKeys lists parameter names that should be redacted
// from trace output to prevent credential exposure.
var sensitiveParamKeys = map[string]bool{
	"password":      true,
	"session_token": true,
	"secret":        true,
	"api_key":       true,
}

// sanitizeParams returns a copy of params with sensitive values redacted.
func sanitizeParams(params map[string]interface{}) map[string]interface{} {
	if params == nil {
		return nil
	}
	sanitized := make(map[string]interface{}, len(params))
	for k, v := range params {
		if sensitiveParamKeys[strings.ToLower(k)] {
			sanitized[k] = "[REDACTED]"
		} else {
			sanitized[k] = v
		}
	}
	return sanitized
}

// sanitizeResult redacts sensitive fields from tool results.
func sanitizeResult(result interface{}) interface{} {
	m, ok := result.(map[string]interface{})
	if !ok {
		return result
	}
	sanitized := make(map[string]interface{}, len(m))
	for k, v := range m {
		if sensitiveParamKeys[strings.ToLower(k)] {
			sanitized[k] = "[REDACTED]"
		} else {
			sanitized[k] = v
		}
	}
	return sanitized
}

// TraceEntry represents a single entry in the JSONL trace file.
type TraceEntry struct {
	Timestamp  time.Time              `json:"timestamp"`
	SessionID  string                 `json:"session_id"`
	Type       string                 `json:"type"`
	Name       string                 `json:"name,omitempty"`
	Parameters interface{}            `json:"parameters,omitempty"`
	Result     interface{}            `json:"result,omitempty"`
	Error      string                 `json:"error,omitempty"`
	Duration   *int64                 `json:"duration_ms,omitempty"`
	TokenHash  string                 `json:"token_hash,omitempty"`
	RequestID  string                 `json:"request_id,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// Entry type constants define the categories of trace events.
const (
	EntryTypeUserPrompt     = "user_prompt"
	EntryTypeLLMResponse    = "llm_response"
	EntryTypeToolCall       = "tool_call"
	EntryTypeToolResult     = "tool_result"
	EntryTypeResourceRead   = "resource_read"
	EntryTypeResourceResult = "resource_result"
	EntryTypeHTTPRequest    = "http_request"
	EntryTypeHTTPResponse   = "http_response"
	EntryTypeSessionStart   = "session_start"
	EntryTypeSessionEnd     = "session_end"
	EntryTypeError          = "error"
	EntryTypePromptCall     = "prompt_call"
	EntryTypePromptResult   = "prompt_result"
	EntryTypeDatabaseSwitch = "database_switch"
	EntryTypeConfigReload   = "config_reload"
)

// durationMs converts a time.Duration to a pointer to milliseconds.
// Using a pointer allows JSON serialization to distinguish between
// "not measured" (nil, omitted) and "zero milliseconds" (0, included).
func durationMs(d time.Duration) *int64 {
	ms := d.Milliseconds()
	return &ms
}

// Tracer writes structured trace entries to a JSONL file.
type Tracer struct {
	mu           sync.Mutex
	file         *os.File
	encoder      *json.Encoder
	enabled      bool
	filePath     string
	metadataOnly bool
}

var (
	instance *Tracer
	once     sync.Once
)

// Initialize performs one-time initialization of the global tracer.
// If filePath is empty, tracing remains disabled. Errors are non-fatal
// and do not prevent server startup. When metadataOnly is true, trace
// entries omit their Parameters and Result fields entirely, so only
// call metadata (session, token, name, duration, error) is written.
func Initialize(filePath string, metadataOnly bool) error {
	var initErr error

	once.Do(func() {
		if filePath == "" {
			return
		}

		f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			initErr = fmt.Errorf("failed to open trace file %s: %w", filePath, err)
			return
		}

		enc := json.NewEncoder(f)
		enc.SetEscapeHTML(false)

		instance = &Tracer{
			file:         f,
			encoder:      enc,
			enabled:      true,
			filePath:     filePath,
			metadataOnly: metadataOnly,
		}
	})

	return initErr
}

// IsEnabled reports whether tracing is active.
func IsEnabled() bool {
	return instance != nil && instance.enabled
}

// Close shuts down the tracer and closes the underlying file.
func Close() error {
	if instance == nil {
		return nil
	}

	instance.mu.Lock()
	defer instance.mu.Unlock()

	if instance.file != nil {
		return instance.file.Close()
	}
	return nil
}

// GetFilePath returns the path of the active trace file.
func GetFilePath() string {
	if instance == nil {
		return ""
	}
	return instance.filePath
}

// Log writes a single trace entry to the JSONL file. It auto-sets
// Timestamp to the current time if the field is zero. When the tracer
// is configured for metadata-only mode, Parameters and Result are
// dropped before the entry is written, regardless of entry type.
func Log(entry TraceEntry) {
	if !IsEnabled() {
		return
	}

	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	}

	if instance.metadataOnly {
		entry.Parameters = nil
		entry.Result = nil
	}

	instance.mu.Lock()
	defer instance.mu.Unlock()

	// Errors during trace writing are intentionally silenced to avoid
	// disrupting normal server operation.
	//nolint:errcheck // Trace writes must not disrupt server operation
	instance.encoder.Encode(entry)
}

// ResetForTesting tears down the global tracer state so that tests
// can reinitialize cleanly. It closes the file handle if one is open.
func ResetForTesting() {
	if instance != nil {
		instance.mu.Lock()
		if instance.file != nil {
			_ = instance.file.Close()
		}
		instance.mu.Unlock()
	}
	once = sync.Once{}
	instance = nil
}

// ---------------------------------------------------------------------------
// Specialized logging functions
// ---------------------------------------------------------------------------

// LogUserPrompt records an incoming user prompt.
func LogUserPrompt(sessionID, tokenHash, requestID string, prompt interface{}) {
	if !IsEnabled() {
		return
	}
	Log(TraceEntry{
		SessionID:  sessionID,
		Type:       EntryTypeUserPrompt,
		TokenHash:  tokenHash,
		RequestID:  requestID,
		Parameters: prompt,
	})
}

// LogLLMResponse records a response from the language model.
func LogLLMResponse(sessionID, tokenHash, requestID string, response interface{}, duration time.Duration) {
	if !IsEnabled() {
		return
	}
	Log(TraceEntry{
		SessionID: sessionID,
		Type:      EntryTypeLLMResponse,
		TokenHash: tokenHash,
		RequestID: requestID,
		Result:    response,
		Duration:  durationMs(duration),
	})
}

// LogToolCall records an MCP tool invocation.
// Sensitive parameters (passwords, tokens) are automatically redacted.
func LogToolCall(sessionID, tokenHash, requestID, toolName string, params map[string]interface{}) {
	if !IsEnabled() {
		return
	}
	Log(TraceEntry{
		SessionID:  sessionID,
		Type:       EntryTypeToolCall,
		Name:       toolName,
		TokenHash:  tokenHash,
		RequestID:  requestID,
		Parameters: sanitizeParams(params),
	})
}

// LogToolResult records the outcome of an MCP tool invocation.
// Sensitive result fields (tokens, credentials) are automatically redacted.
func LogToolResult(sessionID, tokenHash, requestID, toolName string, result interface{}, err error, duration time.Duration) {
	if !IsEnabled() {
		return
	}
	entry := TraceEntry{
		SessionID: sessionID,
		Type:      EntryTypeToolResult,
		Name:      toolName,
		TokenHash: tokenHash,
		RequestID: requestID,
		Result:    sanitizeResult(result),
		Duration:  durationMs(duration),
	}
	if err != nil {
		entry.Error = err.Error()
	}
	Log(entry)
}

// LogResourceRead records a resource read request.
func LogResourceRead(sessionID, tokenHash, requestID, resourceURI string) {
	if !IsEnabled() {
		return
	}
	Log(TraceEntry{
		SessionID: sessionID,
		Type:      EntryTypeResourceRead,
		Name:      resourceURI,
		TokenHash: tokenHash,
		RequestID: requestID,
	})
}

// LogResourceResult records the outcome of a resource read.
func LogResourceResult(sessionID, tokenHash, requestID, resourceURI string, result interface{}, err error, duration time.Duration) {
	if !IsEnabled() {
		return
	}
	entry := TraceEntry{
		SessionID: sessionID,
		Type:      EntryTypeResourceResult,
		Name:      resourceURI,
		TokenHash: tokenHash,
		RequestID: requestID,
		Result:    result,
		Duration:  durationMs(duration),
	}
	if err != nil {
		entry.Error = err.Error()
	}
	Log(entry)
}

// LogHTTPRequest records an incoming HTTP request.
func LogHTTPRequest(sessionID, tokenHash, requestID, method, path string, body interface{}) {
	if !IsEnabled() {
		return
	}
	Log(TraceEntry{
		SessionID:  sessionID,
		Type:       EntryTypeHTTPRequest,
		Name:       method + " " + path,
		TokenHash:  tokenHash,
		RequestID:  requestID,
		Parameters: body,
	})
}

// LogHTTPResponse records an outgoing HTTP response.
func LogHTTPResponse(sessionID, tokenHash, requestID, method, path string, statusCode int, body interface{}, duration time.Duration) {
	if !IsEnabled() {
		return
	}
	Log(TraceEntry{
		SessionID: sessionID,
		Type:      EntryTypeHTTPResponse,
		Name:      method + " " + path,
		TokenHash: tokenHash,
		RequestID: requestID,
		Result:    body,
		Duration:  durationMs(duration),
		Metadata: map[string]interface{}{
			"status_code": statusCode,
		},
	})
}

// LogSessionStart records the beginning of a client session.
func LogSessionStart(sessionID, tokenHash string, metadata map[string]interface{}) {
	if !IsEnabled() {
		return
	}
	Log(TraceEntry{
		SessionID: sessionID,
		Type:      EntryTypeSessionStart,
		TokenHash: tokenHash,
		Metadata:  metadata,
	})
}

// LogSessionEnd records the end of a client session.
func LogSessionEnd(sessionID, tokenHash string, metadata map[string]interface{}) {
	if !IsEnabled() {
		return
	}
	Log(TraceEntry{
		SessionID: sessionID,
		Type:      EntryTypeSessionEnd,
		TokenHash: tokenHash,
		Metadata:  metadata,
	})
}

// LogError records an error event with contextual information.
func LogError(sessionID, tokenHash, requestID, context string, err error) {
	if !IsEnabled() {
		return
	}
	entry := TraceEntry{
		SessionID: sessionID,
		Type:      EntryTypeError,
		Name:      context,
		TokenHash: tokenHash,
		RequestID: requestID,
	}
	if err != nil {
		entry.Error = err.Error()
	}
	Log(entry)
}

// LogPromptCall records an MCP prompt invocation.
func LogPromptCall(sessionID, tokenHash, requestID, promptName string, args map[string]string) {
	if !IsEnabled() {
		return
	}
	Log(TraceEntry{
		SessionID:  sessionID,
		Type:       EntryTypePromptCall,
		Name:       promptName,
		TokenHash:  tokenHash,
		RequestID:  requestID,
		Parameters: args,
	})
}

// LogPromptResult records the outcome of an MCP prompt invocation.
func LogPromptResult(sessionID, tokenHash, requestID, promptName string, result interface{}, err error, duration time.Duration) {
	if !IsEnabled() {
		return
	}
	entry := TraceEntry{
		SessionID: sessionID,
		Type:      EntryTypePromptResult,
		Name:      promptName,
		TokenHash: tokenHash,
		RequestID: requestID,
		Result:    result,
		Duration:  durationMs(duration),
	}
	if err != nil {
		entry.Error = err.Error()
	}
	Log(entry)
}

// LogDatabaseSwitch records a database context switch event.
func LogDatabaseSwitch(sessionID, tokenHash, requestID, dbName string, metadata map[string]interface{}) {
	if !IsEnabled() {
		return
	}
	Log(TraceEntry{
		SessionID: sessionID,
		Type:      EntryTypeDatabaseSwitch,
		Name:      dbName,
		TokenHash: tokenHash,
		RequestID: requestID,
		Metadata:  metadata,
	})
}

// LogConfigReload records a configuration reload event.
func LogConfigReload(sessionID string, metadata map[string]interface{}) {
	if !IsEnabled() {
		return
	}
	Log(TraceEntry{
		SessionID: sessionID,
		Type:      EntryTypeConfigReload,
		Metadata:  metadata,
	})
}

// ---------------------------------------------------------------------------
// ID generators
// ---------------------------------------------------------------------------

// GenerateRequestID returns a cryptographically random 32-character hex
// string suitable for correlating request/response pairs.
func GenerateRequestID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback to a timestamp-based ID if crypto/rand fails.
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// GenerateSessionID returns a session identifier with a "stdio-" prefix
// followed by a 16-character random hex string.
func GenerateSessionID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("stdio-%x", time.Now().UnixNano())
	}
	return "stdio-" + hex.EncodeToString(b)
}

// ---------------------------------------------------------------------------
// Context helpers
// ---------------------------------------------------------------------------

// contextKey is a private type used for context value keys to prevent
// collisions with keys defined in other packages.
type contextKey string

const (
	// RequestIDKey is the context key for the tracing request ID.
	RequestIDKey contextKey = "tracing_request_id"

	// SessionIDKey is the context key for the tracing session ID.
	SessionIDKey contextKey = "tracing_session_id"
)

// GetRequestIDFromContext extracts the request ID from the context.
// It returns an empty string if no request ID is present.
func GetRequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(RequestIDKey).(string); ok {
		return v
	}
	return ""
}

// GetSessionIDFromContext extracts the session ID from the context.
// It returns an empty string if no session ID is present.
func GetSessionIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(SessionIDKey).(string); ok {
		return v
	}
	return ""
}

// WithRequestID returns a new context with the given request ID attached.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, RequestIDKey, id)
}

// WithSessionID returns a new context with the given session ID attached.
func WithSessionID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, SessionIDKey, id)
}
