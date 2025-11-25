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
	"encoding/json"
	"regexp"
	"strings"
)

// Classifier analyzes messages and determines their importance for compaction.
type Classifier struct {
	// PostgreSQL-specific patterns
	schemaPatterns []*regexp.Regexp
	queryPatterns  []*regexp.Regexp
	errorPatterns  []*regexp.Regexp

	// MCP-specific
	preserveToolResults bool
}

// NewClassifier creates a new message classifier.
func NewClassifier(preserveToolResults bool) *Classifier {
	return &Classifier{
		schemaPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)CREATE\s+(TABLE|INDEX|VIEW|SCHEMA|DATABASE)`),
			regexp.MustCompile(`(?i)ALTER\s+TABLE`),
			regexp.MustCompile(`(?i)DROP\s+(TABLE|INDEX|VIEW)`),
			regexp.MustCompile(`(?i)ADD\s+CONSTRAINT`),
			regexp.MustCompile(`(?i)PRIMARY\s+KEY`),
			regexp.MustCompile(`(?i)FOREIGN\s+KEY`),
		},
		queryPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)EXPLAIN\s+(ANALYZE)?`),
			regexp.MustCompile(`(?i)query\s+plan`),
			regexp.MustCompile(`(?i)execution\s+time`),
			regexp.MustCompile(`(?i)index\s+scan`),
			regexp.MustCompile(`(?i)sequential\s+scan`),
		},
		errorPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)error:`),
			regexp.MustCompile(`(?i)ERROR\s+\d+`),
			regexp.MustCompile(`(?i)syntax\s+error`),
			regexp.MustCompile(`(?i)does\s+not\s+exist`),
			regexp.MustCompile(`(?i)permission\s+denied`),
		},
		preserveToolResults: preserveToolResults,
	}
}

// Classify determines the importance class of a message.
func (c *Classifier) Classify(msg Message) ClassificationResult {
	result := ClassificationResult{
		Class:      ClassRoutine, // Default
		Importance: 0.5,
		Reasons:    []string{},
		Metadata:   make(map[string]interface{}),
	}

	// Check for tool content (MCP-specific)
	if c.hasToolContent(msg) {
		c.classifyToolMessage(msg, &result)
		return result
	}

	// Extract text content
	text := c.getContentText(msg)
	lowerText := strings.ToLower(text)

	// Classify based on role and content
	switch msg.Role {
	case "user":
		c.classifyUserMessage(text, lowerText, &result)
	case "assistant":
		c.classifyAssistantMessage(text, lowerText, &result)
	case "system":
		// System messages are typically important context
		result.Class = ClassImportant
		result.Importance = 0.8
		result.Reasons = append(result.Reasons, "system message")
	}

	return result
}

// classifyUserMessage classifies messages from the user.
func (c *Classifier) classifyUserMessage(text, lowerText string, result *ClassificationResult) {
	// First user message is always anchor
	result.Metadata["is_first"] = false

	// User corrections/clarifications are anchors
	corrections := []string{
		"actually", "correction", "instead", "wrong",
		"should be", "meant to say", "not quite",
	}
	for _, phrase := range corrections {
		if strings.Contains(lowerText, phrase) {
			result.Class = ClassAnchor
			result.Importance = 1.0
			result.Reasons = append(result.Reasons, "user correction")
			return
		}
	}

	// Questions establishing new context are important
	if strings.Contains(text, "?") && len(text) > 50 {
		result.Class = ClassImportant
		result.Importance = 0.8
		result.Reasons = append(result.Reasons, "substantive question")
		return
	}

	// Short acknowledgments are transient
	shortPhrases := []string{
		"ok", "yes", "no", "thanks", "great", "got it",
	}
	if len(text) < 30 {
		for _, phrase := range shortPhrases {
			if lowerText == phrase || lowerText == phrase+"." {
				result.Class = ClassTransient
				result.Importance = 0.1
				result.Reasons = append(result.Reasons, "short acknowledgment")
				return
			}
		}
	}

	// Default for user messages is contextual
	result.Class = ClassContextual
	result.Importance = 0.6
	result.Reasons = append(result.Reasons, "user message")
}

// classifyAssistantMessage classifies messages from the assistant.
func (c *Classifier) classifyAssistantMessage(text, lowerText string, result *ClassificationResult) {
	// Schema information is always anchor
	for _, pattern := range c.schemaPatterns {
		if pattern.MatchString(text) {
			result.Class = ClassAnchor
			result.Importance = 1.0
			result.Reasons = append(result.Reasons, "schema definition")
			result.Metadata["has_schema"] = true
			return
		}
	}

	// Query analysis is important
	for _, pattern := range c.queryPatterns {
		if pattern.MatchString(text) {
			result.Class = ClassImportant
			result.Importance = 0.85
			result.Reasons = append(result.Reasons, "query analysis")
			result.Metadata["has_query_analysis"] = true
			return
		}
	}

	// Error information is important
	for _, pattern := range c.errorPatterns {
		if pattern.MatchString(text) {
			result.Class = ClassImportant
			result.Importance = 0.8
			result.Reasons = append(result.Reasons, "error information")
			result.Metadata["has_error"] = true
			return
		}
	}

	// Significant insights
	insightPhrases := []string{
		"key finding:", "important:", "note:",
		"warning:", "recommendation:", "insight:",
	}
	for _, phrase := range insightPhrases {
		if strings.Contains(lowerText, phrase) {
			result.Class = ClassImportant
			result.Importance = 0.85
			result.Reasons = append(result.Reasons, "significant insight")
			return
		}
	}

	// Documentation references
	if strings.Contains(lowerText, "documentation:") ||
		strings.Contains(lowerText, "from docs:") ||
		strings.Contains(lowerText, "postgresql.org") {
		result.Class = ClassImportant
		result.Importance = 0.75
		result.Reasons = append(result.Reasons, "documentation reference")
		return
	}

	// Long, detailed responses are contextual
	if len(text) > 500 {
		result.Class = ClassContextual
		result.Importance = 0.65
		result.Reasons = append(result.Reasons, "detailed response")
		return
	}

	// Short responses are routine
	result.Class = ClassRoutine
	result.Importance = 0.4
	result.Reasons = append(result.Reasons, "routine response")
}

// classifyToolMessage classifies messages containing tool calls or results.
func (c *Classifier) classifyToolMessage(msg Message, result *ClassificationResult) {
	text := c.getContentText(msg)

	// Extract tool names
	toolNames := c.extractToolNames(msg)
	result.Metadata["tools"] = toolNames

	// If preserving all tool results, mark as important
	if c.preserveToolResults {
		result.Class = ClassImportant
		result.Importance = 0.8
		result.Reasons = append(result.Reasons, "tool execution")
		return
	}

	// Classify based on tool type
	for _, toolName := range toolNames {
		switch toolName {
		case "get_schema_info", "pg_dump_schema":
			result.Class = ClassAnchor
			result.Importance = 1.0
			result.Reasons = append(result.Reasons, "schema tool")
			return

		case "execute_explain", "analyze_query":
			result.Class = ClassImportant
			result.Importance = 0.85
			result.Reasons = append(result.Reasons, "query analysis tool")
			return

		case "query_database":
			// Check if results contain significant data
			if len(text) > 500 {
				result.Class = ClassImportant
				result.Importance = 0.75
				result.Reasons = append(result.Reasons, "substantial query results")
			} else {
				result.Class = ClassContextual
				result.Importance = 0.6
				result.Reasons = append(result.Reasons, "query results")
			}
			return

		case "similarity_search", "search_documentation":
			result.Class = ClassContextual
			result.Importance = 0.65
			result.Reasons = append(result.Reasons, "search tool")
			return
		}
	}

	// Check for schema content in tool results
	for _, pattern := range c.schemaPatterns {
		if pattern.MatchString(text) {
			result.Class = ClassAnchor
			result.Importance = 1.0
			result.Reasons = append(result.Reasons, "schema in tool result")
			return
		}
	}

	// Check for errors in tool results
	for _, pattern := range c.errorPatterns {
		if pattern.MatchString(text) {
			result.Class = ClassImportant
			result.Importance = 0.8
			result.Reasons = append(result.Reasons, "error in tool result")
			return
		}
	}

	// Default tool message classification
	result.Class = ClassContextual
	result.Importance = 0.6
	result.Reasons = append(result.Reasons, "tool message")
}

// hasToolContent checks if a message contains tool use or tool result blocks.
func (c *Classifier) hasToolContent(msg Message) bool {
	switch content := msg.Content.(type) {
	case []interface{}:
		for _, block := range content {
			if blockMap, ok := block.(map[string]interface{}); ok {
				if blockType, ok := blockMap["type"].(string); ok {
					if blockType == "tool_use" || blockType == "tool_result" {
						return true
					}
				}
			}
		}
	}
	return false
}

// extractToolNames extracts tool names from a message.
func (c *Classifier) extractToolNames(msg Message) []string {
	var names []string

	switch content := msg.Content.(type) {
	case []interface{}:
		for _, block := range content {
			if blockMap, ok := block.(map[string]interface{}); ok {
				if blockType, ok := blockMap["type"].(string); ok {
					if blockType == "tool_use" {
						if name, ok := blockMap["name"].(string); ok {
							names = append(names, name)
						}
					}
				}
			}
		}
	}

	return names
}

// getContentText extracts text from message content.
func (c *Classifier) getContentText(msg Message) string {
	switch content := msg.Content.(type) {
	case string:
		return content

	case []interface{}:
		var texts []string
		for _, block := range content {
			if blockMap, ok := block.(map[string]interface{}); ok {
				text := c.extractTextFromBlock(blockMap)
				if text != "" {
					texts = append(texts, text)
				}
			}
		}
		return strings.Join(texts, " ")

	default:
		jsonBytes, _ := json.Marshal(content)
		return string(jsonBytes)
	}
}

// extractTextFromBlock extracts text from a content block.
func (c *Classifier) extractTextFromBlock(block map[string]interface{}) string {
	blockType, ok := block["type"].(string)
	if !ok {
		return ""
	}

	switch blockType {
	case "text":
		if text, ok := block["text"].(string); ok {
			return text
		}

	case "tool_use":
		var parts []string
		if name, ok := block["name"].(string); ok {
			parts = append(parts, name)
		}
		if input, ok := block["input"]; ok {
			inputJSON, _ := json.Marshal(input)
			parts = append(parts, string(inputJSON))
		}
		return strings.Join(parts, " ")

	case "tool_result":
		if content, ok := block["content"]; ok {
			switch contentVal := content.(type) {
			case string:
				return contentVal
			case []interface{}:
				var texts []string
				for _, item := range contentVal {
					if itemMap, ok := item.(map[string]interface{}); ok {
						if text, ok := itemMap["text"].(string); ok {
							texts = append(texts, text)
						}
					}
				}
				return strings.Join(texts, " ")
			default:
				contentJSON, _ := json.Marshal(content)
				return string(contentJSON)
			}
		}
	}

	return ""
}
