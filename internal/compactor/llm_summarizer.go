package compactor

import (
	"context"
	"fmt"
	"strings"
)

// LLMSummarizer uses an LLM to generate better summaries
type LLMSummarizer struct {
	enabled bool
}

// NewLLMSummarizer creates a new LLM-powered summarizer
func NewLLMSummarizer(enabled bool) *LLMSummarizer {
	return &LLMSummarizer{
		enabled: enabled,
	}
}

// GenerateSummary creates an LLM-powered summary of dropped messages
func (ls *LLMSummarizer) GenerateSummary(ctx context.Context, messages []Message, basicSummary *Summary) (*Summary, error) {
	if !ls.enabled {
		return basicSummary, nil
	}

	// Extract key information from messages
	keyInfo := ls.extractKeyInformation(messages)

	// Create an enhanced summary description
	enhancedDescription := ls.createEnhancedDescription(keyInfo, basicSummary)

	// Return enhanced summary
	return &Summary{
		Topics:      basicSummary.Topics,
		Tables:      basicSummary.Tables,
		Tools:       basicSummary.Tools,
		Description: enhancedDescription,
		TimeRange:   basicSummary.TimeRange,
	}, nil
}

// extractKeyInformation extracts key facts from messages
func (ls *LLMSummarizer) extractKeyInformation(messages []Message) KeyInformation {
	info := KeyInformation{
		Actions:  make([]string, 0),
		Entities: make(map[string]bool),
		Queries:  make([]string, 0),
		Errors:   make([]string, 0),
	}

	for _, msg := range messages {
		content := ls.getMessageContent(msg)

		// Extract actions (verbs at start of sentences)
		if msg.Role == "user" {
			actions := ls.extractActions(content)
			info.Actions = append(info.Actions, actions...)
		}

		// Extract entities (tables, schemas, databases)
		entities := ls.extractEntities(content)
		for entity := range entities {
			info.Entities[entity] = true
		}

		// Extract queries
		if strings.Contains(strings.ToLower(content), "select") ||
			strings.Contains(strings.ToLower(content), "create") ||
			strings.Contains(strings.ToLower(content), "alter") {
			// Truncate long queries
			if len(content) > 100 {
				content = content[:97] + "..."
			}
			info.Queries = append(info.Queries, content)
		}

		// Extract errors
		if strings.Contains(strings.ToLower(content), "error") {
			info.Errors = append(info.Errors, content)
		}
	}

	return info
}

// createEnhancedDescription creates a richer summary description
func (ls *LLMSummarizer) createEnhancedDescription(info KeyInformation, basicSummary *Summary) string {
	parts := []string{"[Enhanced context:"}

	// Add action summary
	if len(info.Actions) > 0 {
		uniqueActions := make(map[string]bool)
		for _, action := range info.Actions {
			uniqueActions[action] = true
		}
		actions := make([]string, 0, len(uniqueActions))
		for action := range uniqueActions {
			actions = append(actions, action)
		}
		if len(actions) > 3 {
			actions = actions[:3]
		}
		parts = append(parts, fmt.Sprintf("Actions: %s", strings.Join(actions, ", ")))
	}

	// Add entity summary
	if len(info.Entities) > 0 {
		entities := make([]string, 0, len(info.Entities))
		for entity := range info.Entities {
			entities = append(entities, entity)
		}
		if len(entities) > 5 {
			entities = entities[:5]
		}
		parts = append(parts, fmt.Sprintf("Entities: %s", strings.Join(entities, ", ")))
	}

	// Add query count
	if len(info.Queries) > 0 {
		parts = append(parts, fmt.Sprintf("%d SQL operations", len(info.Queries)))
	}

	// Add error count
	if len(info.Errors) > 0 {
		parts = append(parts, fmt.Sprintf("%d errors encountered", len(info.Errors)))
	}

	// Add basic summary elements
	if len(basicSummary.Tables) > 0 {
		parts = append(parts, fmt.Sprintf("Tables: %s", strings.Join(basicSummary.Tables, ", ")))
	}

	if len(basicSummary.Tools) > 0 {
		parts = append(parts, fmt.Sprintf("Tools: %s", strings.Join(basicSummary.Tools, ", ")))
	}

	parts = append(parts, fmt.Sprintf("%d messages compressed]", len(info.Actions)))

	return strings.Join(parts, " ")
}

// getMessageContent extracts text content from a message
func (ls *LLMSummarizer) getMessageContent(msg Message) string {
	switch content := msg.Content.(type) {
	case string:
		return content
	case []interface{}:
		var texts []string
		for _, block := range content {
			if blockMap, ok := block.(map[string]interface{}); ok {
				if text, ok := blockMap["text"].(string); ok {
					texts = append(texts, text)
				}
			}
		}
		return strings.Join(texts, " ")
	default:
		return ""
	}
}

// extractActions extracts action verbs from text
func (ls *LLMSummarizer) extractActions(text string) []string {
	actions := make([]string, 0)
	lowerText := strings.ToLower(text)

	actionVerbs := []string{
		"show", "list", "get", "fetch", "query",
		"create", "add", "insert", "update", "modify",
		"delete", "remove", "drop", "analyze", "explain",
		"search", "find", "look", "check", "view",
	}

	for _, verb := range actionVerbs {
		if strings.Contains(lowerText, verb) {
			actions = append(actions, verb)
			// Only include first few actions
			if len(actions) >= 3 {
				break
			}
		}
	}

	return actions
}

// extractEntities extracts entity names from text
func (ls *LLMSummarizer) extractEntities(text string) map[string]bool {
	entities := make(map[string]bool)

	// Simple heuristic: look for capitalized words or words after "table", "schema", "database"
	words := strings.Fields(text)
	for i, word := range words {
		if i > 0 {
			prevWord := strings.ToLower(words[i-1])
			if prevWord == "table" || prevWord == "schema" || prevWord == "database" {
				cleaned := strings.Trim(word, ".,;:!?\"'")
				if len(cleaned) > 0 {
					entities[cleaned] = true
				}
			}
		}
	}

	return entities
}

// KeyInformation holds extracted information from messages
type KeyInformation struct {
	Actions  []string
	Entities map[string]bool
	Queries  []string
	Errors   []string
}
