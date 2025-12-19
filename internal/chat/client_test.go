/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package chat

import (
	"testing"

	"pgedge-postgres-mcp/internal/mcp"
)

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name string
		text string
		want int
	}{
		{"empty string", "", 0},
		{"short string", "hello", 2},                                    // (5 + 2) / 3 = 2
		{"medium string", "hello world", 4},                             // (11 + 2) / 3 = 4
		{"long string", "This is a longer string with more words.", 14}, // (42 + 2) / 3 = 14
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := estimateTokens(tt.text)
			if got != tt.want {
				t.Errorf("estimateTokens(%q) = %d, want %d", tt.text, got, tt.want)
			}
		})
	}
}

func TestEstimateTotalTokens(t *testing.T) {
	tests := []struct {
		name     string
		messages []Message
		wantMin  int // We check for minimum since estimation includes overhead
	}{
		{
			name:     "empty messages",
			messages: []Message{},
			wantMin:  0,
		},
		{
			name: "single user message",
			messages: []Message{
				{Role: "user", Content: "hello"},
			},
			wantMin: 10, // 2 tokens for "hello" + 10 overhead
		},
		{
			name: "multiple messages",
			messages: []Message{
				{Role: "user", Content: "hello"},
				{Role: "assistant", Content: "hi there"},
			},
			wantMin: 20, // 2 + 10 + 3 + 10 = 25, but we just check minimum
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := estimateTotalTokens(tt.messages)
			if got < tt.wantMin {
				t.Errorf("estimateTotalTokens() = %d, want at least %d", got, tt.wantMin)
			}
		})
	}
}

func TestGetBriefDescription(t *testing.T) {
	tests := []struct {
		name string
		desc string
		want string
	}{
		{
			name: "single line with period",
			desc: "This is a description.",
			want: "This is a description.",
		},
		{
			name: "single line without period",
			desc: "This is a description",
			want: "This is a description",
		},
		{
			name: "multiple lines",
			desc: "First line.\nSecond line.",
			want: "First line.",
		},
		{
			name: "sentence ending with period returns whole line",
			desc: "First sentence. Second sentence continues here.",
			want: "First sentence. Second sentence continues here.",
		},
		{
			name: "sentence without trailing period extracts first",
			desc: "First sentence. Second continues",
			want: "First sentence.",
		},
		{
			name: "empty string",
			desc: "",
			want: "",
		},
		{
			name: "only whitespace lines",
			desc: "\n\n\n",
			want: "\n\n\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getBriefDescription(tt.desc)
			if got != tt.want {
				t.Errorf("getBriefDescription(%q) = %q, want %q", tt.desc, got, tt.want)
			}
		})
	}
}

func TestIsModelAvailable(t *testing.T) {
	tests := []struct {
		name            string
		model           string
		availableModels []string
		want            bool
	}{
		{
			name:            "model in list",
			model:           "gpt-4",
			availableModels: []string{"gpt-3.5", "gpt-4", "gpt-4-turbo"},
			want:            true,
		},
		{
			name:            "model not in list",
			model:           "gpt-5",
			availableModels: []string{"gpt-3.5", "gpt-4"},
			want:            false,
		},
		{
			name:            "nil available models - assume available",
			model:           "any-model",
			availableModels: nil,
			want:            true,
		},
		{
			name:            "empty available list",
			model:           "gpt-4",
			availableModels: []string{},
			want:            false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isModelAvailable(tt.model, tt.availableModels)
			if got != tt.want {
				t.Errorf("isModelAvailable(%q, %v) = %v, want %v", tt.model, tt.availableModels, got, tt.want)
			}
		})
	}
}

func TestGetDefaultModelForProvider(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		want     string
	}{
		{"anthropic", "anthropic", "claude-sonnet-4-5-20250929"},
		{"openai", "openai", "gpt-4o"},
		{"ollama", "ollama", "qwen3-coder:latest"},
		{"unknown", "unknown", ""},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getDefaultModelForProvider(tt.provider)
			if got != tt.want {
				t.Errorf("getDefaultModelForProvider(%q) = %q, want %q", tt.provider, got, tt.want)
			}
		})
	}
}

func TestExtractModelFamily(t *testing.T) {
	tests := []struct {
		name  string
		model string
		want  string
	}{
		{"claude opus 4.5", "claude-opus-4-5-20251101", "claude-opus-4-5-"},
		{"claude sonnet 4.5", "claude-sonnet-4-5-20250929", "claude-sonnet-4-5-"},
		{"claude sonnet 4", "claude-sonnet-4-20250514", "claude-sonnet-4-"},
		{"claude 3 haiku", "claude-3-haiku-20240307", "claude-3-haiku-"},
		{"claude 3.5 sonnet", "claude-3-5-sonnet-20241022", "claude-3-5-sonnet-"},
		{"no date suffix", "gpt-4o-mini", ""},
		{"short model", "model", ""},
		{"empty", "", ""},
		{"just date", "20251101", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractModelFamily(tt.model)
			if got != tt.want {
				t.Errorf("extractModelFamily(%q) = %q, want %q", tt.model, got, tt.want)
			}
		})
	}
}

func TestFindModelFamilyMatch(t *testing.T) {
	availableModels := []string{
		"claude-opus-4-5-20251217",
		"claude-opus-4-5-20251101",
		"claude-sonnet-4-5-20250929",
		"claude-haiku-4-5-20251001",
		"claude-3-haiku-20240307",
	}

	tests := []struct {
		name            string
		savedModel      string
		availableModels []string
		want            string
	}{
		{
			"finds newer opus version",
			"claude-opus-4-5-20251101",
			availableModels,
			"claude-opus-4-5-20251217", // Should pick newest
		},
		{
			"exact match preferred over family",
			"claude-sonnet-4-5-20250929",
			availableModels,
			"claude-sonnet-4-5-20250929",
		},
		{
			"no match returns empty",
			"claude-opus-4-20250514",
			availableModels,
			"", // No models with claude-opus-4- prefix
		},
		{
			"nil available models",
			"claude-opus-4-5-20251101",
			nil,
			"",
		},
		{
			"empty available models",
			"claude-opus-4-5-20251101",
			[]string{},
			"",
		},
		{
			"model without date suffix",
			"gpt-4o",
			[]string{"gpt-4o", "gpt-4o-mini"},
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findModelFamilyMatch(tt.savedModel, tt.availableModels)
			if got != tt.want {
				t.Errorf("findModelFamilyMatch(%q, ...) = %q, want %q", tt.savedModel, got, tt.want)
			}
		})
	}
}

func TestHasToolResults(t *testing.T) {
	// Create a client for testing
	cfg := &Config{
		LLM: LLMConfig{
			Provider:  "ollama",
			OllamaURL: "http://localhost:11434",
		},
		UI: UIConfig{
			NoColor: true,
		},
	}
	client, err := NewClient(cfg, &ConfigOverrides{ProviderSet: true})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	tests := []struct {
		name string
		msg  Message
		want bool
	}{
		{
			name: "message with ToolResult slice",
			msg: Message{
				Role: "user",
				Content: []ToolResult{
					{Type: "tool_result", ToolUseID: "123", Content: "result"},
				},
			},
			want: true,
		},
		{
			name: "message with interface slice containing tool_result",
			msg: Message{
				Role: "user",
				Content: []interface{}{
					map[string]interface{}{
						"type":        "tool_result",
						"tool_use_id": "123",
					},
				},
			},
			want: true,
		},
		{
			name: "message with string content",
			msg: Message{
				Role:    "user",
				Content: "hello",
			},
			want: false,
		},
		{
			name: "message with interface slice without tool_result",
			msg: Message{
				Role: "user",
				Content: []interface{}{
					map[string]interface{}{
						"type": "text",
						"text": "hello",
					},
				},
			},
			want: false,
		},
		{
			name: "message with empty ToolResult slice",
			msg: Message{
				Role:    "user",
				Content: []ToolResult{},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := client.hasToolResults(tt.msg)
			if got != tt.want {
				t.Errorf("hasToolResults() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAdjustStartForToolPairs(t *testing.T) {
	// Create a client for testing
	cfg := &Config{
		LLM: LLMConfig{
			Provider:  "ollama",
			OllamaURL: "http://localhost:11434",
		},
		UI: UIConfig{
			NoColor: true,
		},
	}
	client, err := NewClient(cfg, &ConfigOverrides{ProviderSet: true})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	tests := []struct {
		name     string
		messages []Message
		startIdx int
		want     int
	}{
		{
			name:     "startIdx 0 returns 0",
			messages: []Message{{Role: "user", Content: "hello"}},
			startIdx: 0,
			want:     0,
		},
		{
			name:     "startIdx 1 returns 1",
			messages: []Message{{Role: "user", Content: "hello"}, {Role: "assistant", Content: "hi"}},
			startIdx: 1,
			want:     1,
		},
		{
			name: "user message with tool_result adjusts index",
			messages: []Message{
				{Role: "user", Content: "first"},
				{Role: "assistant", Content: []interface{}{map[string]interface{}{"type": "tool_use"}}},
				{Role: "user", Content: []ToolResult{{Type: "tool_result", ToolUseID: "123"}}},
			},
			startIdx: 2,
			want:     1, // Should adjust to include assistant message with tool_use
		},
		{
			name: "non-user message does not adjust",
			messages: []Message{
				{Role: "user", Content: "first"},
				{Role: "assistant", Content: "response"},
				{Role: "assistant", Content: "another response"},
			},
			startIdx: 2,
			want:     2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := client.adjustStartForToolPairs(tt.messages, tt.startIdx)
			if got != tt.want {
				t.Errorf("adjustStartForToolPairs() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestLocalCompactMessages(t *testing.T) {
	// Create a client for testing
	cfg := &Config{
		LLM: LLMConfig{
			Provider:  "ollama",
			OllamaURL: "http://localhost:11434",
		},
		UI: UIConfig{
			NoColor: true,
		},
	}
	client, err := NewClient(cfg, &ConfigOverrides{ProviderSet: true})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	tests := []struct {
		name              string
		messages          []Message
		maxRecentMessages int
		wantLen           int
	}{
		{
			name: "single user message",
			messages: []Message{
				{Role: "user", Content: "hello"},
			},
			maxRecentMessages: 5,
			wantLen:           1,
		},
		{
			name: "fewer messages than max",
			messages: []Message{
				{Role: "user", Content: "hello"},
				{Role: "assistant", Content: "hi"},
			},
			maxRecentMessages: 5,
			wantLen:           2,
		},
		{
			name: "keeps first user message and recent",
			messages: []Message{
				{Role: "user", Content: "first message"},
				{Role: "assistant", Content: "response 1"},
				{Role: "user", Content: "second"},
				{Role: "assistant", Content: "response 2"},
				{Role: "user", Content: "third"},
				{Role: "assistant", Content: "response 3"},
			},
			maxRecentMessages: 2,
			wantLen:           3, // first + last 2
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := client.localCompactMessages(tt.messages, tt.maxRecentMessages)
			if len(got) != tt.wantLen {
				t.Errorf("localCompactMessages() returned %d messages, want %d", len(got), tt.wantLen)
			}
		})
	}
}

func TestEstimateTotalTokensWithToolContent(t *testing.T) {
	// Test with tool result content
	messages := []Message{
		{
			Role: "user",
			Content: []ToolResult{
				{
					Type:      "tool_result",
					ToolUseID: "123",
					Content: []mcp.ContentItem{
						{Type: "text", Text: "This is the tool result"},
					},
				},
			},
		},
	}

	tokens := estimateTotalTokens(messages)
	// Should have some tokens for the content
	if tokens < 10 {
		t.Errorf("Expected at least 10 tokens, got %d", tokens)
	}
}
