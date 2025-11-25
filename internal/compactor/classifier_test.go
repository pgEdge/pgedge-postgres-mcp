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

func TestClassifier_UserCorrections(t *testing.T) {
	classifier := NewClassifier(false)

	testCases := []struct {
		content  string
		expected MessageClass
	}{
		{"Actually, I meant the employees table", ClassAnchor},
		{"Correction: use users instead", ClassAnchor},
		{"That's wrong, should be orders", ClassAnchor},
		{"No, instead use the products table", ClassAnchor},
		{"Regular question about tables", ClassContextual},
	}

	for _, tc := range testCases {
		msg := createMessage("user", tc.content)
		result := classifier.Classify(msg)

		if result.Class != tc.expected {
			t.Errorf("For content '%s', expected class %s, got %s",
				tc.content, tc.expected.String(), result.Class.String())
		}
	}
}

func TestClassifier_SchemaMessages(t *testing.T) {
	classifier := NewClassifier(false)

	testCases := []struct {
		content  string
		expected MessageClass
	}{
		{"CREATE TABLE users (id INT PRIMARY KEY)", ClassAnchor},
		{"ALTER TABLE employees ADD COLUMN name VARCHAR(100)", ClassAnchor},
		{"DROP TABLE old_data", ClassAnchor},
		{"CREATE INDEX idx_name ON users(name)", ClassAnchor},
		{"Regular query result", ClassRoutine},
	}

	for _, tc := range testCases {
		msg := createMessage("assistant", tc.content)
		result := classifier.Classify(msg)

		if result.Class != tc.expected {
			t.Errorf("For content '%s', expected class %s, got %s",
				tc.content, tc.expected.String(), result.Class.String())
		}
	}
}

func TestClassifier_QueryAnalysis(t *testing.T) {
	classifier := NewClassifier(false)

	testCases := []struct {
		content  string
		expected MessageClass
	}{
		{"EXPLAIN ANALYZE SELECT * FROM users", ClassImportant},
		{"Query plan shows sequential scan", ClassImportant},
		{"Execution time: 45ms", ClassImportant},
		{"Index scan on users_pkey", ClassImportant},
		{"Simple response", ClassRoutine},
	}

	for _, tc := range testCases {
		msg := createMessage("assistant", tc.content)
		result := classifier.Classify(msg)

		if result.Class != tc.expected {
			t.Errorf("For content '%s', expected class %s, got %s",
				tc.content, tc.expected.String(), result.Class.String())
		}
	}
}

func TestClassifier_ErrorMessages(t *testing.T) {
	classifier := NewClassifier(false)

	testCases := []struct {
		content  string
		expected MessageClass
	}{
		{"ERROR: syntax error at or near 'SELECT'", ClassImportant},
		{"Error: table does not exist", ClassImportant},
		{"ERROR 42P01: undefined_table", ClassImportant},
		{"Permission denied for relation users", ClassImportant},
		{"Success message", ClassRoutine},
	}

	for _, tc := range testCases {
		msg := createMessage("assistant", tc.content)
		result := classifier.Classify(msg)

		if result.Class != tc.expected {
			t.Errorf("For content '%s', expected class %s, got %s",
				tc.content, tc.expected.String(), result.Class.String())
		}
	}
}

func TestClassifier_InsightMessages(t *testing.T) {
	classifier := NewClassifier(false)

	testCases := []struct {
		content  string
		expected MessageClass
	}{
		{"Key finding: The index is not being used", ClassImportant},
		{"Important: This query is very slow", ClassImportant},
		{"Note: Consider adding an index", ClassImportant},
		{"Warning: High memory usage detected", ClassImportant},
		{"Recommendation: Use prepared statements", ClassImportant},
		{"Regular observation", ClassRoutine},
	}

	for _, tc := range testCases {
		msg := createMessage("assistant", tc.content)
		result := classifier.Classify(msg)

		if result.Class != tc.expected {
			t.Errorf("For content '%s', expected class %s, got %s",
				tc.content, tc.expected.String(), result.Class.String())
		}
	}
}

func TestClassifier_DocumentationReferences(t *testing.T) {
	classifier := NewClassifier(false)

	testCases := []struct {
		content  string
		expected MessageClass
	}{
		{"Documentation: https://postgresql.org/docs/current/sql-select.html", ClassImportant},
		{"From docs: Indexes improve query performance", ClassImportant},
		{"See postgresql.org for more information", ClassImportant},
		{"Regular comment", ClassRoutine},
	}

	for _, tc := range testCases {
		msg := createMessage("assistant", tc.content)
		result := classifier.Classify(msg)

		if result.Class != tc.expected {
			t.Errorf("For content '%s', expected class %s, got %s",
				tc.content, tc.expected.String(), result.Class.String())
		}
	}
}

func TestClassifier_ShortAcknowledgments(t *testing.T) {
	classifier := NewClassifier(false)

	testCases := []struct {
		content  string
		expected MessageClass
	}{
		{"ok", ClassTransient},
		{"yes", ClassTransient},
		{"no", ClassTransient},
		{"thanks", ClassTransient},
		{"got it", ClassTransient},
		{"This is a longer question with more content?", ClassContextual},
	}

	for _, tc := range testCases {
		msg := createMessage("user", tc.content)
		result := classifier.Classify(msg)

		if result.Class != tc.expected {
			t.Errorf("For content '%s', expected class %s, got %s",
				tc.content, tc.expected.String(), result.Class.String())
		}
	}
}

func TestClassifier_ToolMessages(t *testing.T) {
	classifier := NewClassifier(true) // preserveToolResults = true

	// Schema tool should be important (or higher)
	msg := createToolMessage("assistant", "get_schema_info", "SELECT * FROM pg_tables")
	result := classifier.Classify(msg)

	if result.Class != ClassAnchor && result.Class != ClassImportant {
		t.Errorf("Expected get_schema_info to be classified as anchor or important, got %s", result.Class.String())
	}

	// Query analysis tool should be important
	msg = createToolMessage("assistant", "execute_explain", "EXPLAIN SELECT * FROM users")
	result = classifier.Classify(msg)

	if result.Class != ClassImportant {
		t.Errorf("Expected execute_explain to be classified as important, got %s", result.Class.String())
	}
}

func TestClassifier_SystemMessages(t *testing.T) {
	classifier := NewClassifier(false)

	msg := Message{
		Role:    "system",
		Content: "System initialization message",
	}

	result := classifier.Classify(msg)

	if result.Class != ClassImportant {
		t.Errorf("Expected system message to be classified as important, got %s", result.Class.String())
	}
}

func TestClassifier_ImportanceScoring(t *testing.T) {
	classifier := NewClassifier(false)

	// Anchor messages should have importance 1.0
	msg := createMessage("user", "Actually, use the orders table")
	result := classifier.Classify(msg)

	if result.Importance != 1.0 {
		t.Errorf("Expected anchor message to have importance 1.0, got %.2f", result.Importance)
	}

	// Transient messages should have low importance
	msg = createMessage("user", "ok")
	result = classifier.Classify(msg)

	if result.Importance >= 0.5 {
		t.Errorf("Expected transient message to have low importance, got %.2f", result.Importance)
	}
}

func TestClassifier_HasToolContent(t *testing.T) {
	classifier := NewClassifier(false)

	// Message with tool content
	toolMsg := Message{
		Role: "assistant",
		Content: []interface{}{
			map[string]interface{}{
				"type": "tool_use",
				"name": "query_database",
			},
		},
	}

	if !classifier.hasToolContent(toolMsg) {
		t.Error("Expected hasToolContent to return true for tool message")
	}

	// Regular text message
	textMsg := createMessage("assistant", "Regular text")

	if classifier.hasToolContent(textMsg) {
		t.Error("Expected hasToolContent to return false for text message")
	}
}

func TestClassifier_ExtractToolNames(t *testing.T) {
	classifier := NewClassifier(false)

	msg := Message{
		Role: "assistant",
		Content: []interface{}{
			map[string]interface{}{
				"type": "tool_use",
				"name": "query_database",
			},
			map[string]interface{}{
				"type": "tool_use",
				"name": "get_schema_info",
			},
		},
	}

	names := classifier.extractToolNames(msg)

	if len(names) != 2 {
		t.Errorf("Expected 2 tool names, got %d", len(names))
	}

	expectedNames := map[string]bool{
		"query_database":  true,
		"get_schema_info": true,
	}

	for _, name := range names {
		if !expectedNames[name] {
			t.Errorf("Unexpected tool name: %s", name)
		}
	}
}
