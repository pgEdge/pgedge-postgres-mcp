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
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Compactor performs smart chat history compaction.
type Compactor struct {
	classifier        *Classifier
	tokenEstimator    *TokenEstimator
	providerEstimator *ProviderTokenEstimator
	llmSummarizer     *LLMSummarizer
	cache             *CompactionCache
	analytics         *Analytics
	maxTokens         int
	recentWindow      int
	keepAnchors       bool
	options           *CompactionOptions
}

// NewCompactor creates a new compactor with the given configuration.
func NewCompactor(req CompactRequest) *Compactor {
	// Set defaults
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = DefaultMaxTokens
	}

	recentWindow := req.RecentWindow
	if recentWindow == 0 {
		recentWindow = DefaultRecentWindow
	}

	options := req.Options
	if options == nil {
		options = &CompactionOptions{
			PreserveToolResults:    true,
			PreserveSchemaInfo:     true,
			EnableSummarization:    true,
			MinImportantMessages:   DefaultMinImportant,
			TokenCounterType:       TokenCounterGeneric,
			EnableLLMSummarization: false,
			EnableCaching:          false,
			EnableAnalytics:        false,
		}
	}

	// Set default token counter type if not specified
	if options.TokenCounterType == "" {
		options.TokenCounterType = TokenCounterGeneric
	}

	// Initialize components
	var cache *CompactionCache
	if options.EnableCaching {
		cache = NewCompactionCache(options.CacheTTL)
	}

	var analytics *Analytics
	if options.EnableAnalytics {
		analytics = NewAnalytics()
	}

	return &Compactor{
		classifier:        NewClassifier(options.PreserveToolResults),
		tokenEstimator:    NewTokenEstimator(),
		providerEstimator: NewProviderTokenEstimator(options.TokenCounterType),
		llmSummarizer:     NewLLMSummarizer(options.EnableLLMSummarization),
		cache:             cache,
		analytics:         analytics,
		maxTokens:         maxTokens,
		recentWindow:      recentWindow,
		keepAnchors:       req.KeepAnchors,
		options:           options,
	}
}

// Compact performs smart compaction on the message history.
func (c *Compactor) Compact(messages []Message) CompactResponse {
	startTime := time.Now()
	originalCount := len(messages)

	// Check cache first
	if c.cache != nil {
		if cached, found := c.cache.Get(messages, c.maxTokens, c.recentWindow); found {
			return CompactResponse{
				Messages:       cached.CompactedMsgs,
				Summary:        cached.Summary,
				TokenEstimate:  0, // Cached, no need to recalculate
				CompactionInfo: cached.CompactionInfo,
			}
		}
	}

	// Use provider-specific token estimation if available
	originalTokens := c.estimateTokens(messages)

	// If already within limits, no compaction needed
	if len(messages) <= c.recentWindow+1 || originalTokens <= c.maxTokens {
		result := CompactResponse{
			Messages:      messages,
			TokenEstimate: originalTokens,
			CompactionInfo: CompactionInfo{
				OriginalCount:    originalCount,
				CompactedCount:   len(messages),
				DroppedCount:     0,
				AnchorCount:      0,
				TokensSaved:      0,
				CompressionRatio: 1.0,
			},
		}

		// Record analytics
		if c.analytics != nil {
			c.analytics.RecordCompaction(result.CompactionInfo, time.Since(startTime))
		}

		return result
	}

	// Always keep first message (original context)
	anchors := []Message{messages[0]}

	// Calculate recent window start, ensuring we don't break tool_use/tool_result pairs
	recentStart := len(messages) - c.recentWindow
	if recentStart < 1 {
		recentStart = 1
	}
	// Adjust for tool pairs - if first recent message has tool_results, include preceding message
	recentStart = c.adjustStartForToolPairs(messages, recentStart)

	// Classify middle messages
	middleStart := 1
	middleEnd := recentStart
	if middleEnd <= middleStart {
		middleEnd = middleStart
	}

	middle := messages[middleStart:middleEnd]
	important := c.classifyAndKeepImportant(middle)

	// Ensure tool pairs are preserved in important messages
	important = c.ensureToolPairsInSlice(middle, important, middleStart)

	// Always keep recent messages
	recent := messages[recentStart:]
	if len(messages) < c.recentWindow {
		recent = messages[1:]
	}

	// Build compacted message list
	compacted := make([]Message, 0, len(anchors)+len(important)+len(recent))
	compacted = append(compacted, anchors...)
	compacted = append(compacted, important...)
	compacted = append(compacted, recent...)

	// Check if we're within token budget
	compactedTokens := c.tokenEstimator.EstimateTokensForMessages(compacted)

	// If still over budget or summarization is enabled, create summary
	var summary *Summary
	if compactedTokens > c.maxTokens || c.options.EnableSummarization {
		summary = c.createSummary(middle, important)

		// Enhance summary with LLM if enabled
		if c.llmSummarizer != nil && c.options.EnableLLMSummarization {
			ctx := context.Background()
			enhanced, err := c.llmSummarizer.GenerateSummary(ctx, middle, summary)
			if err == nil {
				summary = enhanced
			}
		}

		// Insert summary message after first anchor
		summaryMsg := Message{
			Role:    "assistant",
			Content: c.formatSummary(summary),
		}
		compacted = append([]Message{compacted[0], summaryMsg}, compacted[1:]...)
		compactedTokens = c.tokenEstimator.EstimateTokensForMessages(compacted)
	}

	// Calculate statistics
	tokensSaved := originalTokens - compactedTokens
	compressionRatio := float64(compactedTokens) / float64(originalTokens)

	result := CompactResponse{
		Messages:      compacted,
		Summary:       summary,
		TokenEstimate: compactedTokens,
		CompactionInfo: CompactionInfo{
			OriginalCount:    originalCount,
			CompactedCount:   len(compacted),
			DroppedCount:     len(messages) - len(compacted),
			AnchorCount:      len(important) + 1, // +1 for first message
			TokensSaved:      tokensSaved,
			CompressionRatio: compressionRatio,
		},
	}

	// Record analytics
	if c.analytics != nil {
		c.analytics.RecordCompaction(result.CompactionInfo, time.Since(startTime))
	}

	// Cache result
	if c.cache != nil {
		c.cache.Set(messages, c.maxTokens, c.recentWindow, result)
	}

	return result
}

// classifyAndKeepImportant classifies middle messages and keeps important ones.
func (c *Compactor) classifyAndKeepImportant(messages []Message) []Message {
	type classifiedMsg struct {
		msg    Message
		result ClassificationResult
	}

	classified := make([]classifiedMsg, 0, len(messages))

	for _, msg := range messages {
		result := c.classifier.Classify(msg)
		classified = append(classified, classifiedMsg{
			msg:    msg,
			result: result,
		})
	}

	// Keep anchors and important messages
	important := make([]Message, 0)
	for _, cm := range classified {
		if c.keepAnchors && cm.result.Class == ClassAnchor {
			important = append(important, cm.msg)
		} else if cm.result.Class == ClassImportant {
			important = append(important, cm.msg)
		} else if cm.result.Importance >= 0.7 {
			important = append(important, cm.msg)
		}
	}

	// Ensure we keep at least MinImportantMessages
	if len(important) < c.options.MinImportantMessages && len(classified) > 0 {
		// Sort by importance and keep top N
		remaining := c.options.MinImportantMessages - len(important)
		for i := 0; i < len(classified) && remaining > 0; i++ {
			found := false
			for _, kept := range important {
				if c.messagesEqual(kept, classified[i].msg) {
					found = true
					break
				}
			}
			if !found && classified[i].result.Class != ClassTransient {
				important = append(important, classified[i].msg)
				remaining--
			}
		}
	}

	return important
}

// messagesEqual checks if two messages are the same.
func (c *Compactor) messagesEqual(m1, m2 Message) bool {
	if m1.Role != m2.Role {
		return false
	}
	// Simple content comparison (could be enhanced)
	return fmt.Sprintf("%v", m1.Content) == fmt.Sprintf("%v", m2.Content)
}

// createSummary creates a summary of the compacted messages.
func (c *Compactor) createSummary(middle, kept []Message) *Summary {
	context := c.extractContext(middle)

	topics := make([]string, 0, len(context.Topics))
	for topic := range context.Topics {
		topics = append(topics, topic)
	}

	tables := make([]string, 0, len(context.Tables))
	for table := range context.Tables {
		tables = append(tables, table)
	}

	tools := make([]string, 0, len(context.Tools))
	for tool := range context.Tools {
		tools = append(tools, tool)
	}

	droppedCount := len(middle) - len(kept)
	description := c.buildSummaryDescription(topics, tables, tools, droppedCount)

	return &Summary{
		Topics:      topics,
		Tables:      tables,
		Tools:       tools,
		Description: description,
	}
}

// extractContext extracts context information from messages.
func (c *Compactor) extractContext(messages []Message) ExtractedContext {
	context := ExtractedContext{
		Topics: make(map[string]bool),
		Tables: make(map[string]bool),
		Tools:  make(map[string]bool),
	}

	tableRegex := regexp.MustCompile(`(?i)\b(\w+)\s+table`)

	for _, msg := range messages {
		text := c.classifier.getContentText(msg)

		// Extract table references
		matches := tableRegex.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if len(match) > 1 {
				tableName := strings.ToLower(match[1])
				// Filter out SQL keywords
				if !c.isSQLKeyword(tableName) {
					context.Tables[tableName] = true
				}
			}
		}

		// Extract tool names
		toolNames := c.classifier.extractToolNames(msg)
		for _, tool := range toolNames {
			context.Tools[tool] = true
		}

		// Extract topics from user messages
		if msg.Role == "user" && len(text) > 20 {
			// Take first few meaningful words as topic
			words := strings.Fields(text)
			if len(words) > 2 {
				topic := strings.Join(words[:min(5, len(words))], " ")
				// Limit topic length
				if len(topic) > 80 {
					topic = topic[:80] + "..."
				}
				context.Topics[topic] = true
			}
		}
	}

	return context
}

// isSQLKeyword checks if a word is a SQL keyword.
func (c *Compactor) isSQLKeyword(word string) bool {
	keywords := map[string]bool{
		"select": true, "from": true, "where": true, "join": true,
		"inner": true, "outer": true, "left": true, "right": true,
		"create": true, "alter": true, "drop": true, "insert": true,
		"update": true, "delete": true, "into": true, "values": true,
	}
	return keywords[word]
}

// buildSummaryDescription creates a human-readable summary description.
func (c *Compactor) buildSummaryDescription(topics, tables, tools []string, droppedCount int) string {
	parts := []string{"[Compressed context:"}

	if len(topics) > 0 {
		// Limit topics shown
		maxTopics := 3
		topicList := topics
		if len(topicList) > maxTopics {
			topicList = topicList[:maxTopics]
		}
		parts = append(parts, fmt.Sprintf("Topics: %s", strings.Join(topicList, ", ")))
	}

	if len(tables) > 0 {
		parts = append(parts, fmt.Sprintf("Tables: %s", strings.Join(tables, ", ")))
	}

	if len(tools) > 0 {
		parts = append(parts, fmt.Sprintf("Tools used: %s", strings.Join(tools, ", ")))
	}

	parts = append(parts, fmt.Sprintf("%d messages compressed]", droppedCount))

	return strings.Join(parts, " ")
}

// formatSummary formats a summary for insertion as a message.
func (c *Compactor) formatSummary(summary *Summary) string {
	return summary.Description
}

// estimateTokens uses provider-specific estimation if configured, otherwise falls back to generic.
func (c *Compactor) estimateTokens(messages []Message) int {
	if c.providerEstimator != nil {
		total := 0
		for _, msg := range messages {
			text := c.classifier.getContentText(msg)
			total += c.providerEstimator.EstimateTokens(text)
		}
		return total
	}
	return c.tokenEstimator.EstimateTokensForMessages(messages)
}

// adjustStartForToolPairs adjusts the start index to ensure tool_use/tool_result
// message pairs are kept together. If the message at startIdx contains tool_results,
// we need to include the preceding assistant message with tool_use blocks.
func (c *Compactor) adjustStartForToolPairs(messages []Message, startIdx int) int {
	if startIdx <= 1 || startIdx >= len(messages) {
		return startIdx
	}

	// Check if the message at startIdx is a user message with tool_results
	msg := messages[startIdx]
	if msg.Role != "user" {
		return startIdx
	}

	// Check if this message contains tool_result blocks
	if c.hasToolResults(msg) {
		// Include the preceding assistant message (which should have tool_use)
		if startIdx > 1 {
			startIdx--
		}
	}

	return startIdx
}

// hasToolResults checks if a message contains tool_result blocks.
func (c *Compactor) hasToolResults(msg Message) bool {
	content, ok := msg.Content.([]interface{})
	if !ok {
		return false
	}

	for _, item := range content {
		if itemMap, ok := item.(map[string]interface{}); ok {
			if itemType, ok := itemMap["type"].(string); ok && itemType == "tool_result" {
				return true
			}
		}
	}

	return false
}

// hasToolUse checks if a message contains tool_use blocks.
func (c *Compactor) hasToolUse(msg Message) bool {
	content, ok := msg.Content.([]interface{})
	if !ok {
		return false
	}

	for _, item := range content {
		if itemMap, ok := item.(map[string]interface{}); ok {
			if itemType, ok := itemMap["type"].(string); ok && itemType == "tool_use" {
				return true
			}
		}
	}

	return false
}

// ensureToolPairsInSlice ensures that if we keep a message with tool_results,
// we also keep its corresponding tool_use message (and vice versa).
// The middleSlice is the original slice, important is what we've decided to keep,
// and offset is the starting index of middleSlice in the original messages array.
func (c *Compactor) ensureToolPairsInSlice(middleSlice, important []Message, offset int) []Message {
	if len(important) == 0 || len(middleSlice) == 0 {
		return important
	}

	// Build a set of kept message indices (relative to middleSlice)
	keptIndices := make(map[int]bool)
	for _, kept := range important {
		for i, msg := range middleSlice {
			if c.messagesEqual(kept, msg) {
				keptIndices[i] = true
				break
			}
		}
	}

	// Check each kept message and ensure its pair is also kept
	additionalMessages := []Message{}
	for idx := range keptIndices {
		msg := middleSlice[idx]

		// If this is a user message with tool_results, ensure preceding assistant is kept
		if msg.Role == "user" && c.hasToolResults(msg) {
			if idx > 0 && !keptIndices[idx-1] {
				prevMsg := middleSlice[idx-1]
				if prevMsg.Role == "assistant" && c.hasToolUse(prevMsg) {
					additionalMessages = append(additionalMessages, prevMsg)
					keptIndices[idx-1] = true
				}
			}
		}

		// If this is an assistant message with tool_use, ensure following user is kept
		if msg.Role == "assistant" && c.hasToolUse(msg) {
			if idx+1 < len(middleSlice) && !keptIndices[idx+1] {
				nextMsg := middleSlice[idx+1]
				if nextMsg.Role == "user" && c.hasToolResults(nextMsg) {
					additionalMessages = append(additionalMessages, nextMsg)
					keptIndices[idx+1] = true
				}
			}
		}
	}

	// Combine and sort by original order
	if len(additionalMessages) > 0 {
		// Sort by position in middleSlice to maintain order
		sortedResult := make([]Message, 0, len(important)+len(additionalMessages))
		for i := range middleSlice {
			if keptIndices[i] {
				sortedResult = append(sortedResult, middleSlice[i])
			}
		}
		return sortedResult
	}

	return important
}
