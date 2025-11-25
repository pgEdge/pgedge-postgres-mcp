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
	"strings"
	"testing"
)

func TestTokenEstimator_BasicEstimation(t *testing.T) {
	estimator := NewTokenEstimator()

	testCases := []struct {
		name      string
		content   string
		minTokens int
	}{
		{"Short message", "Hello", 1},
		{"Medium message", "This is a longer message with multiple words", 5},
		{"Long message", strings.Repeat("word ", 100), 80},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			msg := createMessage("user", tc.content)
			tokens := estimator.EstimateTokens(msg)

			if tokens < tc.minTokens {
				t.Errorf("Expected at least %d tokens for '%s', got %d",
					tc.minTokens, tc.name, tokens)
			}
		})
	}
}

func TestTokenEstimator_SQLContentMultiplier(t *testing.T) {
	estimator := NewTokenEstimator()

	// Use same character count but different content
	regularText := "Show me all the user data now" // 30 chars
	sqlText := "SELECT * FROM users WHERE x"       // 30 chars (roughly)

	regularMsg := createMessage("user", regularText)
	regularTokens := estimator.EstimateTokens(regularMsg)

	sqlMsg := createMessage("user", sqlText)
	sqlTokens := estimator.EstimateTokens(sqlMsg)

	// SQL should have more tokens due to multiplier (or at least same)
	if sqlTokens < regularTokens {
		t.Errorf("Expected SQL content to have >= tokens than regular text, got SQL=%d, regular=%d", sqlTokens, regularTokens)
	}
}

func TestTokenEstimator_ContainsSQL(t *testing.T) {
	estimator := NewTokenEstimator()

	testCases := []struct {
		content  string
		expected bool
	}{
		{"SELECT * FROM users", true},
		{"INSERT INTO employees VALUES (1)", true},
		{"CREATE TABLE orders (id INT)", true},
		{"UPDATE users SET name = 'John'", true},
		{"DELETE FROM old_data", true},
		{"Regular text without SQL", false},
	}

	for _, tc := range testCases {
		result := estimator.containsSQL(strings.ToLower(tc.content))
		if result != tc.expected {
			t.Errorf("For content '%s', expected containsSQL=%v, got %v",
				tc.content, tc.expected, result)
		}
	}
}

func TestTokenEstimator_ContainsJSON(t *testing.T) {
	estimator := NewTokenEstimator()

	testCases := []struct {
		content  string
		expected bool
	}{
		{`{"key": "value"}`, true},
		{`[1, 2, 3]`, true},
		{`{"nested": {"key": "value"}}`, true},
		{"Regular text", false},
		{"{incomplete", false},
	}

	for _, tc := range testCases {
		result := estimator.containsJSON(tc.content)
		if result != tc.expected {
			t.Errorf("For content '%s', expected containsJSON=%v, got %v",
				tc.content, tc.expected, result)
		}
	}
}

func TestTokenEstimator_ContainsCode(t *testing.T) {
	estimator := NewTokenEstimator()

	testCases := []struct {
		content  string
		expected bool
	}{
		{"function myFunc() { return true; }", true},
		{"const x = 10;", true},
		{"let data = fetchData();", true},
		{"```python\nprint('hello')\n```", true},
		{"def my_function():", true},
		{"import numpy as np", true},
		{"Regular text without code", false},
	}

	for _, tc := range testCases {
		result := estimator.containsCode(tc.content)
		if result != tc.expected {
			t.Errorf("For content '%s', expected containsCode=%v, got %v",
				tc.content, tc.expected, result)
		}
	}
}

func TestTokenEstimator_EstimateTokensForMessages(t *testing.T) {
	estimator := NewTokenEstimator()

	messages := []Message{
		createMessage("user", "Hello"),
		createMessage("assistant", "Hi there!"),
		createMessage("user", "How are you?"),
	}

	total := estimator.EstimateTokensForMessages(messages)

	if total <= 0 {
		t.Error("Expected positive total token count")
	}

	// Total should be at least the sum of individual estimates
	individualSum := 0
	for _, msg := range messages {
		individualSum += estimator.EstimateTokens(msg)
	}

	if total != individualSum {
		t.Errorf("Expected total %d to equal sum of individuals %d", total, individualSum)
	}
}

func TestTokenEstimator_ExtractTextFromString(t *testing.T) {
	estimator := NewTokenEstimator()

	msg := createMessage("user", "Simple text content")
	text := estimator.extractText(msg)

	if text != "Simple text content" {
		t.Errorf("Expected 'Simple text content', got '%s'", text)
	}
}

func TestTokenEstimator_ExtractTextFromBlocks(t *testing.T) {
	estimator := NewTokenEstimator()

	msg := Message{
		Role: "assistant",
		Content: []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": "First block",
			},
			map[string]interface{}{
				"type": "text",
				"text": "Second block",
			},
		},
	}

	text := estimator.extractText(msg)

	if !strings.Contains(text, "First block") || !strings.Contains(text, "Second block") {
		t.Errorf("Expected text to contain both blocks, got '%s'", text)
	}
}

func TestTokenEstimator_ExtractTextFromToolUse(t *testing.T) {
	estimator := NewTokenEstimator()

	msg := Message{
		Role: "assistant",
		Content: []interface{}{
			map[string]interface{}{
				"type": "tool_use",
				"name": "query_database",
				"input": map[string]interface{}{
					"query": "SELECT * FROM users",
				},
			},
		},
	}

	text := estimator.extractText(msg)

	if !strings.Contains(text, "query_database") {
		t.Errorf("Expected text to contain tool name, got '%s'", text)
	}
}

func TestTokenEstimator_ExtractTextFromToolResult(t *testing.T) {
	estimator := NewTokenEstimator()

	msg := Message{
		Role: "user",
		Content: []interface{}{
			map[string]interface{}{
				"type": "tool_result",
				"content": []interface{}{
					map[string]interface{}{
						"type": "text",
						"text": "Query result data",
					},
				},
			},
		},
	}

	text := estimator.extractText(msg)

	if !strings.Contains(text, "Query result data") {
		t.Errorf("Expected text to contain tool result, got '%s'", text)
	}
}

func TestTokenEstimator_OverheadPerMessage(t *testing.T) {
	estimator := NewTokenEstimator()

	// Very short message
	msg := createMessage("user", "Hi")
	tokens := estimator.EstimateTokens(msg)

	// Should include overhead even for short message
	if tokens <= 1 {
		t.Errorf("Expected overhead to be included, got only %d tokens", tokens)
	}
}

func TestCountWords(t *testing.T) {
	testCases := []struct {
		text     string
		expected int
	}{
		{"Hello world", 2},
		{"One", 1},
		{"", 0},
		{"Multiple words in this sentence", 5},
		{"Words, with! punctuation?", 3},
	}

	for _, tc := range testCases {
		count := CountWords(tc.text)
		if count != tc.expected {
			t.Errorf("For text '%s', expected %d words, got %d",
				tc.text, tc.expected, count)
		}
	}
}

func TestTruncateText(t *testing.T) {
	testCases := []struct {
		name           string
		text           string
		maxTokens      int
		shouldTruncate bool
	}{
		{"Short text", "Hello world", 100, false},
		{"Long text", strings.Repeat("word ", 200), 50, true},
		{"Exact length", "Exact", 2, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := TruncateText(tc.text, tc.maxTokens)

			if tc.shouldTruncate && !strings.HasSuffix(result, "...") {
				t.Error("Expected truncated text to end with '...'")
			}

			if !tc.shouldTruncate && result != tc.text {
				t.Error("Expected text to remain unchanged")
			}

			// Result should not be longer than original
			if len(result) > len(tc.text) {
				t.Error("Truncated text should not be longer than original")
			}
		})
	}
}

func TestTokenEstimator_ContentMultiplier(t *testing.T) {
	estimator := NewTokenEstimator()

	testCases := []struct {
		name          string
		content       string
		minMultiplier float64
	}{
		{"Regular text", "This is regular text", 1.0},
		{"SQL content", "SELECT * FROM users WHERE id = 1", 1.2},
		{"JSON content", `{"key": "value"}`, 1.15},
		{"Code content", "function test() { return true; }", 1.1},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			multiplier := estimator.getContentMultiplier(tc.content)

			if multiplier < tc.minMultiplier {
				t.Errorf("Expected multiplier >= %.2f for %s, got %.2f",
					tc.minMultiplier, tc.name, multiplier)
			}
		})
	}
}
