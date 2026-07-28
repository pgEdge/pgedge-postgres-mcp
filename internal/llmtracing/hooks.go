/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

// Package llmtracing adapts the pgedge-go-llm-lib proxy hooks to our
// internal/tracing logger. Designed to be passed directly into
// proxy.Config.OnRequest / OnResponse / OnError.
package llmtracing

import (
	"context"
	"net/http"
	"time"

	"github.com/pgEdge/pgedge-go-llm-lib/llm/proxy"

	"pgedge-postgres-mcp/internal/auth"
	"pgedge-postgres-mcp/internal/redact"
	"pgedge-postgres-mcp/internal/tracing"
)

// startTimeKey is a context value carrying the proxy request's start
// time so OnResponse can compute duration.
type startTimeKey struct{}

// withStartTime returns a context with the request's start time stored
// under startTimeKey.
func withStartTime(ctx context.Context, t time.Time) context.Context {
	return context.WithValue(ctx, startTimeKey{}, t)
}

// startTimeFromContext returns the stored start time, or the zero
// time.Time if unset.
func startTimeFromContext(ctx context.Context) time.Time {
	if t, ok := ctx.Value(startTimeKey{}).(time.Time); ok {
		return t
	}
	return time.Time{}
}

// session resolves the session ID and token hash from the request
// context. Returns ("anonymous", "") when no token hash is present.
func session(r *http.Request) (sessionID, tokenHash string) {
	tokenHash = auth.GetTokenHashFromContext(r.Context())
	sessionID = tokenHash
	if sessionID == "" {
		sessionID = "anonymous"
	}
	return sessionID, tokenHash
}

// OnRequest is suitable for proxy.Config.OnRequest. It logs the user
// prompt with provider/model/stream metadata and stashes the request's
// start time onto the request context for OnResponse to read.
func OnRequest(r *http.Request, info proxy.RequestInfo) {
	sessionID, tokenHash := session(r)

	*r = *r.WithContext(withStartTime(r.Context(), time.Now()))

	msgCount := 0
	if info.Request != nil {
		msgCount = len(info.Request.Messages)
	}

	tracing.LogUserPrompt(sessionID, tokenHash, info.RequestID, map[string]interface{}{
		"provider":      info.Provider,
		"model":         info.Model,
		"stream":        info.Stream,
		"message_count": msgCount,
	})
}

// OnResponse is suitable for proxy.Config.OnResponse. It computes the
// request duration from the start time stashed by OnRequest and logs
// the LLM response.
func OnResponse(r *http.Request, info proxy.ResponseInfo) {
	sessionID, tokenHash := session(r)

	dur := time.Duration(0)
	if start := startTimeFromContext(r.Context()); !start.IsZero() {
		dur = time.Since(start)
	}

	tracing.LogLLMResponse(sessionID, tokenHash, info.RequestID, map[string]interface{}{
		"provider":    info.Provider,
		"model":       info.Model,
		"stream":      info.Stream,
		"status_code": info.StatusCode,
		"usage":       info.Usage,
		"response":    info.Response,
	}, dur)
}

// OnError is suitable for proxy.Config.OnError. It logs the error
// with the proxy's classification context.
func OnError(r *http.Request, info proxy.ErrorInfo) {
	sessionID, tokenHash := session(r)

	contextLabel := "llm_chat"
	if info.Stream {
		contextLabel = "llm_chat_stream"
	}

	// The error carries the provider's own message, which on an authentication
	// failure quotes the key it rejected, so it is redacted before it reaches
	// the trace file. Traces persist and get attached to support tickets, which
	// makes this the more durable of the two places the fragment escaped.
	tracing.LogError(sessionID, tokenHash, info.RequestID, contextLabel,
		redact.ErrorValue(info.Err))
}
