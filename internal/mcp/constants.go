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

// Scanner buffer size constants for JSON-RPC message processing
const (
	// ScannerInitialBufferSize is the initial buffer size (64KB)
	// This should be large enough for most MCP messages
	ScannerInitialBufferSize = 64 * 1024

	// ScannerMaxBufferSize is the maximum buffer size (1MB)
	// This prevents unbounded memory growth from malicious or malformed messages
	ScannerMaxBufferSize = 1024 * 1024
)
