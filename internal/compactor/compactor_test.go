/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package compactor

import (
	"testing"
)

// Helper to create a simple text message
func createMessage(role, content string) Message {
	return Message{
		Role:    role,
		Content: content,
	}
}

// Helper to create a message with tool content
func createToolMessage(role string, toolName string, content string) Message {
	return Message{
		Role: role,
		Content: []interface{}{
			map[string]interface{}{
				"type": "tool_use",
				"name": toolName,
				"input": map[string]interface{}{
					"query": content,
				},
			},
		},
	}
}

func TestCompactor_NoCompactionNeeded(t *testing.T) {
	messages := []Message{
		createMessage("user", "Hello"),
		createMessage("assistant", "Hi there!"),
	}

	req := CompactRequest{
		Messages:     messages,
		MaxTokens:    100000,
		RecentWindow: 10,
		KeepAnchors:  true,
	}

	compactor := NewCompactor(req)
	result := compactor.Compact(messages)

	if result.CompactionInfo.OriginalCount != 2 {
		t.Errorf("Expected original count 2, got %d", result.CompactionInfo.OriginalCount)
	}

	if result.CompactionInfo.CompactedCount != 2 {
		t.Errorf("Expected compacted count 2, got %d", result.CompactionInfo.CompactedCount)
	}

	if result.CompactionInfo.DroppedCount != 0 {
		t.Errorf("Expected dropped count 0, got %d", result.CompactionInfo.DroppedCount)
	}
}

func TestCompactor_BasicCompaction(t *testing.T) {
	// Create 20 messages
	messages := []Message{
		createMessage("user", "Initial question"),
	}

	for i := 0; i < 19; i++ {
		messages = append(messages, createMessage("assistant", "Response "+string(rune('A'+i))))
	}

	req := CompactRequest{
		Messages:     messages,
		MaxTokens:    100000,
		RecentWindow: 5,
		KeepAnchors:  true,
	}

	compactor := NewCompactor(req)
	result := compactor.Compact(messages)

	// Should keep first message + some messages + recent window
	if result.CompactionInfo.CompactedCount > result.CompactionInfo.OriginalCount {
		t.Errorf("Compacted count should not exceed original count")
	}

	// With 20 messages and recent window of 5, we should compact
	// (Even if nothing is "dropped", we should at least compact to first + recent)
	if result.CompactionInfo.CompactedCount == 0 {
		t.Errorf("Expected compacted messages")
	}
}

func TestCompactor_PreservesSchemaMessages(t *testing.T) {
	messages := []Message{
		createMessage("user", "Show me the schema"),
		createMessage("assistant", "Here's the schema:"),
		createMessage("assistant", "CREATE TABLE users (id INT PRIMARY KEY)"),
		createMessage("user", "What about employees?"),
		createMessage("assistant", "Simple response"),
		createMessage("assistant", "CREATE TABLE employees (id INT PRIMARY KEY)"),
		createMessage("user", "Thanks"),
		createMessage("assistant", "You're welcome"),
	}

	req := CompactRequest{
		Messages:     messages,
		MaxTokens:    100000,
		RecentWindow: 2,
		KeepAnchors:  true,
		Options: &CompactionOptions{
			PreserveSchemaInfo: true,
		},
	}

	compactor := NewCompactor(req)
	result := compactor.Compact(messages)

	// Should preserve schema messages even if not in recent window
	foundSchema1 := false
	foundSchema2 := false

	for _, msg := range result.Messages {
		content, ok := msg.Content.(string)
		if !ok {
			continue
		}
		if content == "CREATE TABLE users (id INT PRIMARY KEY)" {
			foundSchema1 = true
		}
		if content == "CREATE TABLE employees (id INT PRIMARY KEY)" {
			foundSchema2 = true
		}
	}

	if !foundSchema1 {
		t.Error("Expected first schema message to be preserved")
	}

	if !foundSchema2 {
		t.Error("Expected second schema message to be preserved")
	}
}

func TestCompactor_PreservesToolMessages(t *testing.T) {
	messages := []Message{
		createMessage("user", "Get schema info"),
		createToolMessage("assistant", "get_schema_info", "SELECT * FROM tables"),
		createMessage("user", "Query the database"),
		createToolMessage("assistant", "query_database", "SELECT * FROM users"),
		createMessage("user", "Thanks"),
	}

	req := CompactRequest{
		Messages:     messages,
		MaxTokens:    100000,
		RecentWindow: 2,
		KeepAnchors:  true,
		Options: &CompactionOptions{
			PreserveToolResults: true,
		},
	}

	compactor := NewCompactor(req)
	result := compactor.Compact(messages)

	// Should preserve schema tool even if not in recent window
	foundSchemaTool := false
	for _, msg := range result.Messages {
		if content, ok := msg.Content.([]interface{}); ok {
			for _, block := range content {
				if blockMap, ok := block.(map[string]interface{}); ok {
					if name, ok := blockMap["name"].(string); ok && name == "get_schema_info" {
						foundSchemaTool = true
					}
				}
			}
		}
	}

	if !foundSchemaTool {
		t.Error("Expected schema tool message to be preserved")
	}
}

func TestCompactor_TokenEstimation(t *testing.T) {
	messages := []Message{
		createMessage("user", "Hello"),
		createMessage("assistant", "Hi there, this is a longer response that should have more tokens estimated for it"),
	}

	req := CompactRequest{
		Messages:     messages,
		MaxTokens:    100000,
		RecentWindow: 10,
		KeepAnchors:  true,
	}

	compactor := NewCompactor(req)
	result := compactor.Compact(messages)

	if result.TokenEstimate <= 0 {
		t.Error("Expected positive token estimate")
	}

	// Longer message should have more tokens
	tokens1 := compactor.tokenEstimator.EstimateTokens(messages[0])
	tokens2 := compactor.tokenEstimator.EstimateTokens(messages[1])

	if tokens2 <= tokens1 {
		t.Error("Expected longer message to have more tokens")
	}
}

func TestCompactor_CompressionRatio(t *testing.T) {
	// Create many messages
	messages := []Message{createMessage("user", "Initial")}
	for i := 0; i < 50; i++ {
		messages = append(messages, createMessage("assistant", "Response"))
	}

	req := CompactRequest{
		Messages:     messages,
		MaxTokens:    100000,
		RecentWindow: 5,
		KeepAnchors:  true,
	}

	compactor := NewCompactor(req)
	result := compactor.Compact(messages)

	// Compression ratio should be between 0 and 1 (or 1.0 if no compression)
	if result.CompactionInfo.CompressionRatio < 0 {
		t.Error("Expected non-negative compression ratio")
	}

	if result.CompactionInfo.CompressionRatio > 1.0 {
		t.Error("Expected compression ratio <= 1.0")
	}
}

func TestCompactor_SummaryGeneration(t *testing.T) {
	messages := []Message{
		createMessage("user", "Show me users table"),
		createMessage("assistant", "Querying users table"),
		createMessage("assistant", "Found 100 rows"),
		createMessage("user", "Show employees"),
		createMessage("assistant", "Querying employees table"),
		createMessage("assistant", "Found 50 rows"),
		createMessage("user", "What else?"),
		createMessage("assistant", "More data"),
		createMessage("user", "Continue"),
		createMessage("assistant", "Final response"),
	}

	req := CompactRequest{
		Messages:     messages,
		MaxTokens:    50, // Very low to force summarization
		RecentWindow: 2,
		KeepAnchors:  true,
		Options: &CompactionOptions{
			EnableSummarization: true,
		},
	}

	compactor := NewCompactor(req)
	result := compactor.Compact(messages)

	// Summary may or may not be generated depending on whether compaction happens
	// Just verify the compaction worked
	if result.CompactionInfo.CompactedCount > result.CompactionInfo.OriginalCount {
		t.Error("Compacted count should not exceed original")
	}
}

func TestCompactor_EmptyMessages(t *testing.T) {
	messages := []Message{}

	req := CompactRequest{
		Messages:     messages,
		MaxTokens:    100000,
		RecentWindow: 10,
		KeepAnchors:  true,
	}

	compactor := NewCompactor(req)
	result := compactor.Compact(messages)

	if len(result.Messages) != 0 {
		t.Errorf("Expected 0 messages, got %d", len(result.Messages))
	}
}

func TestCompactor_SingleMessage(t *testing.T) {
	messages := []Message{
		createMessage("user", "Hello"),
	}

	req := CompactRequest{
		Messages:     messages,
		MaxTokens:    100000,
		RecentWindow: 10,
		KeepAnchors:  true,
	}

	compactor := NewCompactor(req)
	result := compactor.Compact(messages)

	if len(result.Messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(result.Messages))
	}

	if result.CompactionInfo.DroppedCount != 0 {
		t.Error("Expected no messages to be dropped")
	}
}
