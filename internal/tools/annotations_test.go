/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package tools

import (
	"testing"

	"pgedge-postgres-mcp/internal/config"
)

// TestBuiltinReadOnlyToolAnnotations checks that every built-in tool other
// than query_database (whose annotations depend on write access and are
// tested alongside it) advertises itself as read-only, and flags those that
// call an external embedding provider as open-world.
func TestBuiltinReadOnlyToolAnnotations(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tests := []struct {
		tool      Tool
		openWorld bool
	}{
		{ReadResourceTool(nil), false},
		{GenerateEmbeddingTool(cfg), true},
		{SearchKnowledgebaseTool("", cfg), true},
		{ListDatabaseConnectionsTool(nil, nil, cfg), false},
		{SelectDatabaseConnectionTool(nil, nil, cfg), false},
		{GetSchemaInfoTool(nil), false},
		{SimilaritySearchTool(nil, cfg), true},
		{ExecuteExplainTool(nil), false},
		{CountRowsTool(nil), false},
	}

	for _, tt := range tests {
		def := tt.tool.Definition
		t.Run(def.Name, func(t *testing.T) {
			t.Parallel()
			a := def.Annotations
			if a == nil {
				t.Fatal("tool has no annotations")
			}
			if a.ReadOnlyHint == nil || !*a.ReadOnlyHint {
				t.Error("readOnlyHint should be true")
			}
			if a.DestructiveHint != nil {
				t.Error("destructiveHint should be unset on a read-only tool")
			}
			if a.OpenWorldHint == nil || *a.OpenWorldHint != tt.openWorld {
				t.Errorf("openWorldHint should be %v", tt.openWorld)
			}
		})
	}
}
