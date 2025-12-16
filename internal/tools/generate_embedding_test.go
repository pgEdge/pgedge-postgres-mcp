/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package tools

import (
	"strings"
	"testing"

	"pgedge-postgres-mcp/internal/config"
)

func TestGenerateEmbeddingTool_Definition(t *testing.T) {
	cfg := &config.Config{}
	tool := GenerateEmbeddingTool(cfg)

	if tool.Definition.Name != "generate_embedding" {
		t.Errorf("expected name 'generate_embedding', got %q", tool.Definition.Name)
	}

	if tool.Definition.Description == "" {
		t.Error("expected non-empty description")
	}

	// Check required parameters
	if len(tool.Definition.InputSchema.Required) != 1 {
		t.Errorf("expected 1 required parameter, got %d", len(tool.Definition.InputSchema.Required))
	}

	if tool.Definition.InputSchema.Required[0] != "text" {
		t.Errorf("expected 'text' to be required, got %q", tool.Definition.InputSchema.Required[0])
	}
}

func TestGenerateEmbeddingTool_NotEnabled(t *testing.T) {
	cfg := &config.Config{
		Embedding: config.EmbeddingConfig{
			Enabled: false,
		},
	}
	tool := GenerateEmbeddingTool(cfg)

	args := map[string]interface{}{
		"text": "test text",
	}

	response, err := tool.Handler(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !response.IsError {
		t.Error("expected error response when embedding is disabled")
	}

	if len(response.Content) == 0 {
		t.Fatal("expected error message in response")
	}

	if !strings.Contains(response.Content[0].Text, "not enabled") {
		t.Errorf("expected 'not enabled' in error message, got: %s", response.Content[0].Text)
	}
}

func TestGenerateEmbeddingTool_MissingText(t *testing.T) {
	cfg := &config.Config{
		Embedding: config.EmbeddingConfig{
			Enabled: true,
		},
	}
	tool := GenerateEmbeddingTool(cfg)

	args := map[string]interface{}{}

	response, err := tool.Handler(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !response.IsError {
		t.Error("expected error response when text is missing")
	}

	if len(response.Content) == 0 {
		t.Fatal("expected error message in response")
	}

	if !strings.Contains(response.Content[0].Text, "Missing or invalid 'text'") {
		t.Errorf("expected 'Missing or invalid' error, got: %s", response.Content[0].Text)
	}
}

func TestGenerateEmbeddingTool_EmptyText(t *testing.T) {
	cfg := &config.Config{
		Embedding: config.EmbeddingConfig{
			Enabled: true,
		},
	}
	tool := GenerateEmbeddingTool(cfg)

	args := map[string]interface{}{
		"text": "",
	}

	response, err := tool.Handler(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !response.IsError {
		t.Error("expected error response when text is empty")
	}
}

func TestGenerateEmbeddingTool_WhitespaceOnlyText(t *testing.T) {
	cfg := &config.Config{
		Embedding: config.EmbeddingConfig{
			Enabled: true,
		},
	}
	tool := GenerateEmbeddingTool(cfg)

	args := map[string]interface{}{
		"text": "   \t\n   ",
	}

	response, err := tool.Handler(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !response.IsError {
		t.Error("expected error response when text is whitespace only")
	}

	if len(response.Content) == 0 {
		t.Fatal("expected error message in response")
	}

	if !strings.Contains(response.Content[0].Text, "empty or whitespace-only") {
		t.Errorf("expected 'empty or whitespace-only' error, got: %s", response.Content[0].Text)
	}
}

func TestGenerateEmbeddingTool_InvalidTextType(t *testing.T) {
	cfg := &config.Config{
		Embedding: config.EmbeddingConfig{
			Enabled: true,
		},
	}
	tool := GenerateEmbeddingTool(cfg)

	args := map[string]interface{}{
		"text": 123, // Wrong type
	}

	response, err := tool.Handler(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !response.IsError {
		t.Error("expected error response when text has wrong type")
	}
}

func TestGenerateEmbeddingTool_InvalidProvider(t *testing.T) {
	cfg := &config.Config{
		Embedding: config.EmbeddingConfig{
			Enabled:  true,
			Provider: "invalid_provider",
		},
	}
	tool := GenerateEmbeddingTool(cfg)

	args := map[string]interface{}{
		"text": "test text",
	}

	response, err := tool.Handler(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !response.IsError {
		t.Error("expected error response for invalid provider")
	}

	if len(response.Content) == 0 {
		t.Fatal("expected error message in response")
	}

	// Should fail to initialize the embedding provider
	if !strings.Contains(response.Content[0].Text, "Failed to initialize") {
		t.Errorf("expected 'Failed to initialize' error, got: %s", response.Content[0].Text)
	}
}
