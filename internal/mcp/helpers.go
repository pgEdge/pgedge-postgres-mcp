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

// DatabaseNotReadyError is a standard error message for when database is still initializing
const DatabaseNotReadyError = "Database is still initializing. Please wait a moment and try again.\n\nThe server is loading database metadata in the background. This usually takes a few seconds."

// DatabaseNotReadyErrorShort is a shorter version for resources
const DatabaseNotReadyErrorShort = "Error: Database not ready"
