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
	"encoding/json"
	"errors"
)

// ErrResourceNotFound is returned by a ResourceProvider's Read when the
// requested URI does not match any registered resource. Per the MCP
// spec, an unknown resource URI is a protocol-level JSON-RPC error, not
// a successful result -- this sentinel lets handleResourceRead and
// handleResourceReadHTTP translate it to the correct error code for the
// requesting era (-32602 for a modern (2026-07-28) request, -32002 for
// legacy, per the spec's own renumbering of this exact error).
var ErrResourceNotFound = errors.New("resource not found")

// ResourceError represents a JSON-formatted error response for resources
type ResourceError struct {
	Error     bool   `json:"error"`
	Message   string `json:"message"`
	Code      string `json:"code"`
	Retryable bool   `json:"retryable"`
}

// NewToolError creates a standardized error response for tools
func NewToolError(message string) (ToolResponse, error) {
	return ToolResponse{
		Content: []ContentItem{
			{
				Type: "text",
				Text: message,
			},
		},
		IsError: true,
	}, nil
}

// NewToolSuccess creates a standardized success response for tools
func NewToolSuccess(message string) (ToolResponse, error) {
	return ToolResponse{
		Content: []ContentItem{
			{
				Type: "text",
				Text: message,
			},
		},
		IsError: false,
	}, nil
}

// NewResourceError creates a standardized error response for resources
func NewResourceError(uri string, message string) (ResourceContent, error) {
	return ResourceContent{
		URI: uri,
		Contents: []ContentItem{
			{
				Type: "text",
				Text: message,
			},
		},
	}, nil
}

// NewResourceJSONError creates a JSON-formatted error response for resources.
// This is useful for resources with MimeType "application/json" that need to
// return errors that clients can parse and handle programmatically.
func NewResourceJSONError(uri string, message string, code string, retryable bool) (ResourceContent, error) {
	errResponse := ResourceError{
		Error:     true,
		Message:   message,
		Code:      code,
		Retryable: retryable,
	}

	jsonData, err := json.Marshal(errResponse)
	if err != nil {
		// Fall back to plain text error if JSON marshaling fails
		return NewResourceError(uri, message)
	}

	return ResourceContent{
		URI:      uri,
		MimeType: "application/json",
		Contents: []ContentItem{
			{
				Type: "text",
				Text: string(jsonData),
			},
		},
	}, nil
}

// NewResourceSuccess creates a standardized success response for resources
func NewResourceSuccess(uri string, mimeType string, content string) (ResourceContent, error) {
	return ResourceContent{
		URI:      uri,
		MimeType: mimeType,
		Contents: []ContentItem{
			{
				Type: "text",
				Text: content,
			},
		},
	}, nil
}

// DatabaseNotReadyMessage is a user-friendly message for the database not ready state
const DatabaseNotReadyMessage = "Database is switching. Please wait..."

// Error codes for JSON error responses
const (
	ErrorCodeDatabaseNotReady = "DATABASE_NOT_READY"
)
