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

// TokenEstimator provides utilities for estimating token counts in messages.
type TokenEstimator struct {
	// charsPerToken is the average characters per token ratio
	charsPerToken float64

	// overheadPerMessage is the fixed token overhead per message
	overheadPerMessage int
}

// NewTokenEstimator creates a new token estimator with default settings.
func NewTokenEstimator() *TokenEstimator {
	return &TokenEstimator{
		charsPerToken:      4.0, // Rough average across most tokenizers
		overheadPerMessage: 10,  // JSON structure overhead
	}
}

// EstimateTokens estimates the token count for a single message.
func (te *TokenEstimator) EstimateTokens(msg Message) int {
	text := te.extractText(msg)
	baseEstimate := float64(len(text)) / te.charsPerToken

	// Adjust for content type
	multiplier := te.getContentMultiplier(text)
	estimate := int(baseEstimate * multiplier)

	return estimate + te.overheadPerMessage
}

// EstimateTokensForMessages estimates total tokens for a list of messages.
func (te *TokenEstimator) EstimateTokensForMessages(messages []Message) int {
	total := 0
	for _, msg := range messages {
		total += te.EstimateTokens(msg)
	}
	return total
}

// extractText extracts text content from a message's content field.
func (te *TokenEstimator) extractText(msg Message) string {
	switch content := msg.Content.(type) {
	case string:
		return content

	case []interface{}:
		var texts []string
		for _, block := range content {
			if blockMap, ok := block.(map[string]interface{}); ok {
				text := te.extractTextFromBlock(blockMap)
				if text != "" {
					texts = append(texts, text)
				}
			}
		}
		return strings.Join(texts, " ")

	default:
		// Try to marshal to JSON as fallback
		jsonBytes, err := json.Marshal(content)
		if err != nil {
			return ""
		}
		return string(jsonBytes)
	}
}

// extractTextFromBlock extracts text from a content block.
func (te *TokenEstimator) extractTextFromBlock(block map[string]interface{}) string {
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
		// Include tool name and input for token estimation
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
		// Include tool result content
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

// getContentMultiplier returns a multiplier based on content type.
// SQL, code, and structured data tend to have different token densities.
func (te *TokenEstimator) getContentMultiplier(text string) float64 {
	lowerText := strings.ToLower(text)

	// SQL queries tend to be more token-dense
	if te.containsSQL(lowerText) {
		return 1.2
	}

	// JSON/structured data is also denser
	if te.containsJSON(text) {
		return 1.15
	}

	// Code blocks
	if te.containsCode(text) {
		return 1.1
	}

	return 1.0
}

// containsSQL checks if text contains SQL keywords.
func (te *TokenEstimator) containsSQL(text string) bool {
	sqlKeywords := []string{
		"select ", "from ", "where ", "join ",
		"create table", "alter table", "drop table",
		"insert into", "update ", "delete from",
		"group by", "order by", "having ",
	}

	for _, keyword := range sqlKeywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

// containsJSON checks if text looks like JSON.
func (te *TokenEstimator) containsJSON(text string) bool {
	trimmed := strings.TrimSpace(text)
	return (strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")) ||
		(strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]"))
}

// containsCode checks if text contains code patterns.
func (te *TokenEstimator) containsCode(text string) bool {
	codePatterns := []string{
		"```", "function ", "const ", "let ", "var ",
		"def ", "class ", "import ", "package ",
	}

	for _, pattern := range codePatterns {
		if strings.Contains(text, pattern) {
			return true
		}
	}
	return false
}

// CountWords counts words in a string for rough estimation.
func CountWords(text string) int {
	wordRegex := regexp.MustCompile(`\w+`)
	return len(wordRegex.FindAllString(text, -1))
}

// TruncateText truncates text to approximately maxTokens.
func TruncateText(text string, maxTokens int) string {
	// Rough approximation: 4 chars per token
	maxChars := maxTokens * 4

	if len(text) <= maxChars {
		return text
	}

	// Truncate and add ellipsis
	truncated := text[:maxChars-3]

	// Try to break at a word boundary
	lastSpace := strings.LastIndex(truncated, " ")
	if lastSpace > maxChars/2 {
		truncated = truncated[:lastSpace]
	}

	return truncated + "..."
}
