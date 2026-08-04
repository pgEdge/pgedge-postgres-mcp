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
	"testing"
)

// TestDecodeHeaderValue_PlainASCIIPassesThrough covers a header value
// with no Base64 sentinel wrapping: it must be returned unchanged.
func TestDecodeHeaderValue_PlainASCIIPassesThrough(t *testing.T) {
	got, ok := decodeHeaderValue("count_rows")
	if !ok || got != "count_rows" {
		t.Errorf("decodeHeaderValue() = (%q, %v), want (%q, true)", got, ok, "count_rows")
	}
}

// TestDecodeHeaderValue_WellFormedSentinelRoundTrips covers the intended
// use of the sentinel: a value not safely representable as plain ASCII,
// wrapped and Base64-encoded per the transport spec.
func TestDecodeHeaderValue_WellFormedSentinelRoundTrips(t *testing.T) {
	original := "pg://a resource/with spaces"
	wrapped := "=?base64?" + base64.StdEncoding.EncodeToString([]byte(original)) + "?="

	got, ok := decodeHeaderValue(wrapped)
	if !ok || got != original {
		t.Errorf("decodeHeaderValue(%q) = (%q, %v), want (%q, true)", wrapped, got, ok, original)
	}
}

// TestDecodeHeaderValue_InvalidBase64ContentIsRejected covers a
// well-formed sentinel wrapper around content that is not valid Base64.
func TestDecodeHeaderValue_InvalidBase64ContentIsRejected(t *testing.T) {
	_, ok := decodeHeaderValue("=?base64?not-valid-base64!!?=")
	if ok {
		t.Error("decodeHeaderValue() = ok=true for invalid Base64 content, want ok=false")
	}
}

// TestDecodeHeaderValue_ShortOverlappingSentinelDoesNotPanic guards
// against a regression: the sentinel's prefix ("=?base64?") and suffix
// ("?=") share a "?", so a value shorter than their combined length can
// satisfy both strings.HasPrefix and strings.HasSuffix by overlapping on
// that character (e.g. "=?base64?=", 10 bytes, vs. 11 needed). Without a
// length check, the encoded-content slice below the sentinel check has a
// negative length and panics. Such a malformed value carries no valid
// Base64 content to extract, so it must be treated as a literal,
// unencoded header value instead.
func TestDecodeHeaderValue_ShortOverlappingSentinelDoesNotPanic(t *testing.T) {
	const malformed = "=?base64?="
	got, ok := decodeHeaderValue(malformed)
	if !ok || got != malformed {
		t.Errorf("decodeHeaderValue(%q) = (%q, %v), want (%q, true)", malformed, got, ok, malformed)
	}
}

// TestDecodeHeaderValue_EmptyString covers the zero-length input edge
// case, which is also shorter than the sentinel's combined delimiter
// length and must not panic.
func TestDecodeHeaderValue_EmptyString(t *testing.T) {
	got, ok := decodeHeaderValue("")
	if !ok || got != "" {
		t.Errorf("decodeHeaderValue(\"\") = (%q, %v), want (\"\", true)", got, ok)
	}
}
