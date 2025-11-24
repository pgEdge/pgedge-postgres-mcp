// Package compactor provides smart chat history compaction for MCP clients.
// It implements PostgreSQL and MCP-aware message classification to optimize
// token usage while preserving semantically important context.
package compactor

import (
	"time"
)

// Message represents a chat message with role and content.
type Message struct {
	Role         string      `json:"role"`
	Content      interface{} `json:"content"`
	CacheControl interface{} `json:"cache_control,omitempty"`
}

// CompactRequest represents a request to compact chat history.
type CompactRequest struct {
	Messages     []Message          `json:"messages"`
	MaxTokens    int                `json:"max_tokens,omitempty"`
	RecentWindow int                `json:"recent_window,omitempty"`
	KeepAnchors  bool               `json:"keep_anchors"`
	Options      *CompactionOptions `json:"options,omitempty"`
}

// CompactionOptions provides fine-grained control over compaction behavior.
type CompactionOptions struct {
	// PreserveToolResults keeps all tool execution results
	PreserveToolResults bool `json:"preserve_tool_results"`

	// PreserveSchemaInfo keeps all schema-related messages
	PreserveSchemaInfo bool `json:"preserve_schema_info"`

	// EnableSummarization creates summaries for compressed segments
	EnableSummarization bool `json:"enable_summarization"`

	// MinImportantMessages minimum important messages to keep
	MinImportantMessages int `json:"min_important_messages"`

	// TokenCounterType specifies the token counting strategy
	TokenCounterType TokenCounterType `json:"token_counter_type,omitempty"`

	// EnableLLMSummarization uses LLM to generate better summaries
	EnableLLMSummarization bool `json:"enable_llm_summarization"`

	// EnableCaching enables persistent compaction cache
	EnableCaching bool `json:"enable_caching"`

	// CacheTTL specifies cache entry time-to-live (0 = no expiry)
	CacheTTL time.Duration `json:"cache_ttl,omitempty"`

	// EnableAnalytics enables compression metrics tracking
	EnableAnalytics bool `json:"enable_analytics"`
}

// CompactResponse contains the compacted messages and statistics.
type CompactResponse struct {
	Messages       []Message      `json:"messages"`
	Summary        *Summary       `json:"summary,omitempty"`
	TokenEstimate  int            `json:"token_estimate"`
	CompactionInfo CompactionInfo `json:"compaction_info"`
}

// Summary contains a compressed representation of dropped messages.
type Summary struct {
	Topics      []string   `json:"topics"`
	Tables      []string   `json:"tables"`
	Tools       []string   `json:"tools"`
	Description string     `json:"description"`
	TimeRange   *TimeRange `json:"time_range,omitempty"`
}

// TimeRange represents a time span for summarized messages.
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// CompactionInfo provides statistics about the compaction operation.
type CompactionInfo struct {
	OriginalCount    int     `json:"original_count"`
	CompactedCount   int     `json:"compacted_count"`
	DroppedCount     int     `json:"dropped_count"`
	AnchorCount      int     `json:"anchor_count"`
	TokensSaved      int     `json:"tokens_saved"`
	CompressionRatio float64 `json:"compression_ratio"`
}

// MessageClass represents the importance classification of a message.
type MessageClass int

const (
	// ClassAnchor - critical context that must always be kept
	ClassAnchor MessageClass = iota

	// ClassImportant - high-value messages to keep if possible
	ClassImportant

	// ClassContextual - useful context, keep if space allows
	ClassContextual

	// ClassRoutine - standard messages that can be compressed
	ClassRoutine

	// ClassTransient - low-value messages that can be dropped
	ClassTransient
)

// String returns the string representation of a MessageClass.
func (mc MessageClass) String() string {
	switch mc {
	case ClassAnchor:
		return "anchor"
	case ClassImportant:
		return "important"
	case ClassContextual:
		return "contextual"
	case ClassRoutine:
		return "routine"
	case ClassTransient:
		return "transient"
	default:
		return "unknown"
	}
}

// ClassificationResult contains the classification outcome for a message.
type ClassificationResult struct {
	Class      MessageClass           `json:"class"`
	Importance float64                `json:"importance"`
	Reasons    []string               `json:"reasons"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// ExtractedContext contains information extracted from a set of messages.
type ExtractedContext struct {
	Topics map[string]bool
	Tables map[string]bool
	Tools  map[string]bool
}

// CompactionMetrics tracks analytics about compaction operations
type CompactionMetrics struct {
	TotalCompactions   int64         `json:"total_compactions"`
	TotalMessagesIn    int64         `json:"total_messages_in"`
	TotalMessagesOut   int64         `json:"total_messages_out"`
	TotalTokensSaved   int64         `json:"total_tokens_saved"`
	AverageCompression float64       `json:"average_compression"`
	TotalDuration      time.Duration `json:"total_duration"`
	AverageDuration    time.Duration `json:"average_duration"`
	LastCompactionTime time.Time     `json:"last_compaction_time"`
}

// Default configuration values
const (
	DefaultMaxTokens    = 100000
	DefaultRecentWindow = 10
	DefaultMinImportant = 3
)
