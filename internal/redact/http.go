/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package redact

import "net/http"

// Handler wraps h so that everything it writes to the response body passes
// through String first.
//
// It exists for handlers this project does not own. The LLM proxy comes from
// the shared library and builds its own error bodies from a provider's message,
// so there is no call site here at which the text could be cleaned before it is
// written; the response itself has to be filtered instead.
//
// Two consequences are worth knowing about. Redaction happens per write, so a
// credential split across two Write calls would pass through: that does not
// arise for a JSON error body, which is encoded in one go, nor for a
// server-sent event, which is written as a unit, but it is a real limit of
// filtering a stream rather than fixing the source. And Content-Length is
// dropped when the wrapped handler sets it, because redaction changes the
// body's length and a stale header would truncate the response; Go then either
// computes the correct length or uses chunked encoding.
func Handler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(&responseWriter{ResponseWriter: w}, r)
	})
}

// responseWriter redacts each write on its way to the client.
type responseWriter struct {
	http.ResponseWriter
}

// WriteHeader drops a Content-Length set by the wrapped handler, since
// redaction may change how many bytes are actually written.
func (w *responseWriter) WriteHeader(status int) {
	if w.Header().Get("Content-Length") != "" {
		w.Header().Del("Content-Length")
	}
	w.ResponseWriter.WriteHeader(status)
}

// Write redacts p and reports the caller's own length on success.
//
// Returning the redacted length instead would break the io.Writer contract as
// callers understand it: a shorter count than requested reads as a short write,
// and json.Encoder and friends treat that as an error.
func (w *responseWriter) Write(p []byte) (int, error) {
	n, err := w.ResponseWriter.Write(Bytes(p))
	if err != nil {
		return n, err
	}
	return len(p), nil
}

// Flush forwards to the underlying writer so that streaming responses are not
// held up by the wrapper. Server-sent events depend on it.
func (w *responseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap exposes the underlying writer to http.ResponseController.
func (w *responseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
