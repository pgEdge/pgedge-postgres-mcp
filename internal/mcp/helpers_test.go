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

import (
	"testing"
)

func TestNewToolError(t *testing.T) {
	message := "Test error message"
	response, err := NewToolError(message)

	if err != nil {
		t.Errorf("Expected nil error, got %v", err)
	}

	if !response.IsError {
		t.Error("Expected IsError to be true")
	}

	if len(response.Content) != 1 {
		t.Errorf("Expected 1 content item, got %d", len(response.Content))
	}

	if response.Content[0].Type != "text" {
		t.Errorf("Expected content type 'text', got '%s'", response.Content[0].Type)
	}

	if response.Content[0].Text != message {
		t.Errorf("Expected message '%s', got '%s'", message, response.Content[0].Text)
	}
}

func TestNewToolSuccess(t *testing.T) {
	message := "Test success message"
	response, err := NewToolSuccess(message)

	if err != nil {
		t.Errorf("Expected nil error, got %v", err)
	}

	if response.IsError {
		t.Error("Expected IsError to be false")
	}

	if len(response.Content) != 1 {
		t.Errorf("Expected 1 content item, got %d", len(response.Content))
	}

	if response.Content[0].Type != "text" {
		t.Errorf("Expected content type 'text', got '%s'", response.Content[0].Type)
	}

	if response.Content[0].Text != message {
		t.Errorf("Expected message '%s', got '%s'", message, response.Content[0].Text)
	}
}

func TestNewResourceError(t *testing.T) {
	uri := "pg://test"
	message := "Test error message"
	resource, err := NewResourceError(uri, message)

	if err != nil {
		t.Errorf("Expected nil error, got %v", err)
	}

	if resource.URI != uri {
		t.Errorf("Expected URI '%s', got '%s'", uri, resource.URI)
	}

	if len(resource.Contents) != 1 {
		t.Errorf("Expected 1 content item, got %d", len(resource.Contents))
	}

	if resource.Contents[0].Type != "text" {
		t.Errorf("Expected content type 'text', got '%s'", resource.Contents[0].Type)
	}

	if resource.Contents[0].Text != message {
		t.Errorf("Expected message '%s', got '%s'", message, resource.Contents[0].Text)
	}
}

func TestNewResourceSuccess(t *testing.T) {
	uri := "pg://test"
	mimeType := "application/json"
	content := `{"test": "data"}`
	resource, err := NewResourceSuccess(uri, mimeType, content)

	if err != nil {
		t.Errorf("Expected nil error, got %v", err)
	}

	if resource.URI != uri {
		t.Errorf("Expected URI '%s', got '%s'", uri, resource.URI)
	}

	if resource.MimeType != mimeType {
		t.Errorf("Expected MimeType '%s', got '%s'", mimeType, resource.MimeType)
	}

	if len(resource.Contents) != 1 {
		t.Errorf("Expected 1 content item, got %d", len(resource.Contents))
	}

	if resource.Contents[0].Type != "text" {
		t.Errorf("Expected content type 'text', got '%s'", resource.Contents[0].Type)
	}

	if resource.Contents[0].Text != content {
		t.Errorf("Expected content '%s', got '%s'", content, resource.Contents[0].Text)
	}
}
