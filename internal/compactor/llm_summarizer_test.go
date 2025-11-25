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
	"context"
	"strings"
	"testing"
)

func TestLLMSummarizer_Disabled(t *testing.T) {
	summarizer := NewLLMSummarizer(false)

	messages := []Message{
		{Role: "user", Content: "Show me the users table"},
	}

	basicSummary := &Summary{
		Topics: []string{"users table"},
		Tables: []string{"users"},
	}

	result, err := summarizer.GenerateSummary(context.Background(), messages, basicSummary)
	if err != nil {
		t.Fatalf("GenerateSummary failed: %v", err)
	}

	// When disabled, should return basic summary unchanged
	if result != basicSummary {
		t.Error("Expected basic summary to be returned when disabled")
	}
}

func TestLLMSummarizer_ExtractKeyInformation(t *testing.T) {
	summarizer := NewLLMSummarizer(true)

	messages := []Message{
		{Role: "user", Content: "Show me all users from the users table"},
		{Role: "assistant", Content: "SELECT * FROM users"},
		{Role: "user", Content: "Create a new products schema"},
		{Role: "assistant", Content: "CREATE SCHEMA products"},
	}

	info := summarizer.extractKeyInformation(messages)

	// Check actions
	if len(info.Actions) == 0 {
		t.Error("Expected to extract some actions")
	}

	// Check entities
	if len(info.Entities) == 0 {
		t.Error("Expected to extract some entities")
	}

	// Check queries
	if len(info.Queries) == 0 {
		t.Error("Expected to extract some SQL queries")
	}
}

func TestLLMSummarizer_ExtractActions(t *testing.T) {
	summarizer := NewLLMSummarizer(true)

	tests := []struct {
		name          string
		text          string
		expectActions bool
	}{
		{
			name:          "Show command",
			text:          "show me the users",
			expectActions: true,
		},
		{
			name:          "Create command",
			text:          "create a new table",
			expectActions: true,
		},
		{
			name:          "List command",
			text:          "list all databases",
			expectActions: true,
		},
		{
			name:          "No actions",
			text:          "this is just text",
			expectActions: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actions := summarizer.extractActions(tt.text)
			hasActions := len(actions) > 0
			if hasActions != tt.expectActions {
				t.Errorf("extractActions(%q) has actions = %v, want %v", tt.text, hasActions, tt.expectActions)
			}
		})
	}
}

func TestLLMSummarizer_ExtractEntities(t *testing.T) {
	summarizer := NewLLMSummarizer(true)

	tests := []struct {
		name     string
		text     string
		expected []string
	}{
		{
			name:     "Table reference",
			text:     "select from table users",
			expected: []string{"users"},
		},
		{
			name:     "Schema reference",
			text:     "in schema public",
			expected: []string{"public"},
		},
		{
			name:     "Database reference",
			text:     "connect to database postgres",
			expected: []string{"postgres"},
		},
		{
			name:     "Multiple entities",
			text:     "table users and schema products",
			expected: []string{"users", "products"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entities := summarizer.extractEntities(tt.text)
			for _, exp := range tt.expected {
				if !entities[exp] {
					t.Errorf("Expected entity %q not found in %v", exp, entities)
				}
			}
		})
	}
}

func TestLLMSummarizer_GetMessageContent(t *testing.T) {
	summarizer := NewLLMSummarizer(true)

	tests := []struct {
		name     string
		msg      Message
		expected string
	}{
		{
			name:     "String content",
			msg:      Message{Role: "user", Content: "Hello world"},
			expected: "Hello world",
		},
		{
			name: "Block content",
			msg: Message{
				Role: "assistant",
				Content: []interface{}{
					map[string]interface{}{
						"text": "First block",
					},
					map[string]interface{}{
						"text": "Second block",
					},
				},
			},
			expected: "First block Second block",
		},
		{
			name:     "Other content type",
			msg:      Message{Role: "user", Content: 123},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := summarizer.getMessageContent(tt.msg)
			if content != tt.expected {
				t.Errorf("getMessageContent() = %q, want %q", content, tt.expected)
			}
		})
	}
}

func TestLLMSummarizer_CreateEnhancedDescription(t *testing.T) {
	summarizer := NewLLMSummarizer(true)

	info := KeyInformation{
		Actions:  []string{"show", "create", "list"},
		Entities: map[string]bool{"users": true, "products": true},
		Queries:  []string{"SELECT * FROM users", "CREATE TABLE products"},
		Errors:   []string{"error: permission denied"},
	}

	basicSummary := &Summary{
		Tables: []string{"users", "products"},
		Tools:  []string{"query_database"},
	}

	description := summarizer.createEnhancedDescription(info, basicSummary)

	// Check that key components are present
	if !strings.Contains(description, "[Enhanced context:") {
		t.Error("Description should start with '[Enhanced context:'")
	}

	if !strings.Contains(description, "Actions:") {
		t.Error("Description should contain 'Actions:'")
	}

	if !strings.Contains(description, "Entities:") {
		t.Error("Description should contain 'Entities:'")
	}

	if !strings.Contains(description, "SQL operations") {
		t.Error("Description should mention SQL operations")
	}

	if !strings.Contains(description, "errors encountered") {
		t.Error("Description should mention errors")
	}

	if !strings.Contains(description, "Tables:") {
		t.Error("Description should contain 'Tables:'")
	}

	if !strings.Contains(description, "Tools:") {
		t.Error("Description should contain 'Tools:'")
	}

	if !strings.Contains(description, "messages compressed]") {
		t.Error("Description should end with 'messages compressed]'")
	}
}

func TestLLMSummarizer_GenerateSummary(t *testing.T) {
	summarizer := NewLLMSummarizer(true)

	messages := []Message{
		{Role: "user", Content: "Show me the users table"},
		{Role: "assistant", Content: "SELECT * FROM users"},
		{Role: "user", Content: "Create products schema"},
	}

	basicSummary := &Summary{
		Topics: []string{"database queries"},
		Tables: []string{"users"},
		Tools:  []string{"query_database"},
	}

	enhanced, err := summarizer.GenerateSummary(context.Background(), messages, basicSummary)
	if err != nil {
		t.Fatalf("GenerateSummary failed: %v", err)
	}

	// Check that enhanced summary has all basic fields
	if len(enhanced.Topics) != len(basicSummary.Topics) {
		t.Error("Enhanced summary should preserve Topics")
	}

	if len(enhanced.Tables) != len(basicSummary.Tables) {
		t.Error("Enhanced summary should preserve Tables")
	}

	if len(enhanced.Tools) != len(basicSummary.Tools) {
		t.Error("Enhanced summary should preserve Tools")
	}

	// Check that description was enhanced
	if enhanced.Description == basicSummary.Description {
		t.Error("Expected enhanced description to be different from basic")
	}

	if !strings.Contains(enhanced.Description, "[Enhanced context:") {
		t.Error("Enhanced description should start with '[Enhanced context:'")
	}
}

func TestLLMSummarizer_QueryTruncation(t *testing.T) {
	summarizer := NewLLMSummarizer(true)

	longQuery := strings.Repeat("SELECT * FROM users WHERE id = 1 AND ", 10)
	messages := []Message{
		{Role: "assistant", Content: longQuery},
	}

	info := summarizer.extractKeyInformation(messages)

	if len(info.Queries) == 0 {
		t.Fatal("Expected to extract query")
	}

	// Check that long queries are truncated
	if len(info.Queries[0]) > 100 {
		t.Errorf("Query length = %v, should be truncated to <= 100", len(info.Queries[0]))
	}

	if !strings.HasSuffix(info.Queries[0], "...") {
		t.Error("Truncated query should end with '...'")
	}
}

func TestLLMSummarizer_ErrorDetection(t *testing.T) {
	summarizer := NewLLMSummarizer(true)

	messages := []Message{
		{Role: "assistant", Content: "Error: connection failed"},
		{Role: "assistant", Content: "Query executed successfully"},
		{Role: "assistant", Content: "error: permission denied"},
	}

	info := summarizer.extractKeyInformation(messages)

	if len(info.Errors) != 2 {
		t.Errorf("Expected 2 error messages, got %v", len(info.Errors))
	}
}
