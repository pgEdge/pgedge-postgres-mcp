/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package chat

import (
	"encoding/json"
	"strings"
	"testing"

	llmlib "github.com/pgEdge/pgedge-go-llm-lib/llm"

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
		messages []llmlib.Message
		wantMin  int // We check for minimum since estimation includes overhead
	}{
		{
			name:     "empty messages",
			messages: []llmlib.Message{},
			wantMin:  0,
		},
		{
			name: "single user message",
			messages: []llmlib.Message{
				llmlib.UserText("hello"),
			},
			wantMin: 10, // 2 tokens for "hello" + 10 overhead
		},
		{
			name: "multiple messages",
			messages: []llmlib.Message{
				llmlib.UserText("hello"),
				llmlib.AssistantText("hi there"),
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
		msg  llmlib.Message
		want bool
	}{
		{
			name: "message with tool_result block",
			msg: llmlib.Message{
				Role: llmlib.RoleTool,
				Content: []llmlib.ContentBlock{
					{Type: llmlib.BlockToolResult, ToolUseID: "123", Text: "result"},
				},
			},
			want: true,
		},
		{
			name: "user message with tool_result block",
			msg: llmlib.Message{
				Role: llmlib.RoleUser,
				Content: []llmlib.ContentBlock{
					{Type: llmlib.BlockToolResult, ToolUseID: "123"},
				},
			},
			want: true,
		},
		{
			name: "message with plain text content",
			msg:  llmlib.UserText("hello"),
			want: false,
		},
		{
			name: "message with only text blocks",
			msg: llmlib.Message{
				Role: llmlib.RoleUser,
				Content: []llmlib.ContentBlock{
					{Type: llmlib.BlockText, Text: "hello"},
				},
			},
			want: false,
		},
		{
			name: "message with empty content",
			msg: llmlib.Message{
				Role:    llmlib.RoleUser,
				Content: nil,
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
		messages []llmlib.Message
		startIdx int
		want     int
	}{
		{
			name:     "startIdx 0 returns 0",
			messages: []llmlib.Message{llmlib.UserText("hello")},
			startIdx: 0,
			want:     0,
		},
		{
			name:     "startIdx 1 returns 1",
			messages: []llmlib.Message{llmlib.UserText("hello"), llmlib.AssistantText("hi")},
			startIdx: 1,
			want:     1,
		},
		{
			name: "tool message with tool_result adjusts index",
			messages: []llmlib.Message{
				llmlib.UserText("first"),
				{
					Role: llmlib.RoleAssistant,
					Content: []llmlib.ContentBlock{{
						Type:    llmlib.BlockToolUse,
						ToolUse: &llmlib.ToolUse{ID: "123", Name: "x"},
					}},
				},
				{
					Role: llmlib.RoleTool,
					Content: []llmlib.ContentBlock{{
						Type:      llmlib.BlockToolResult,
						ToolUseID: "123",
					}},
				},
			},
			startIdx: 2,
			want:     1, // Should adjust to include assistant message with tool_use
		},
		{
			name: "non-tool-result message does not adjust",
			messages: []llmlib.Message{
				llmlib.UserText("first"),
				llmlib.AssistantText("response"),
				llmlib.AssistantText("another response"),
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
		messages          []llmlib.Message
		maxRecentMessages int
		wantLen           int
	}{
		{
			name: "single user message",
			messages: []llmlib.Message{
				llmlib.UserText("hello"),
			},
			maxRecentMessages: 5,
			wantLen:           1,
		},
		{
			name: "fewer messages than max",
			messages: []llmlib.Message{
				llmlib.UserText("hello"),
				llmlib.AssistantText("hi"),
			},
			maxRecentMessages: 5,
			wantLen:           2,
		},
		{
			name: "keeps first user message and recent",
			messages: []llmlib.Message{
				llmlib.UserText("first message"),
				llmlib.AssistantText("response 1"),
				llmlib.UserText("second"),
				llmlib.AssistantText("response 2"),
				llmlib.UserText("third"),
				llmlib.AssistantText("response 3"),
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
	messages := []llmlib.Message{
		{
			Role: llmlib.RoleTool,
			Content: []llmlib.ContentBlock{{
				Type:      llmlib.BlockToolResult,
				ToolUseID: "123",
				Text:      "This is the tool result",
			}},
		},
	}

	// Baseline = same structure with empty content. The tool_result
	// text above MUST add tokens on top of the per-message overhead;
	// if it doesn't, estimateTotalTokens is counting only the
	// structural overhead and not the user-visible content.
	baseline := []llmlib.Message{
		{
			Role: llmlib.RoleTool,
			Content: []llmlib.ContentBlock{{
				Type:      llmlib.BlockToolResult,
				ToolUseID: "123",
				Text:      "",
			}},
		},
	}
	baselineTokens := estimateTotalTokens(baseline)
	tokens := estimateTotalTokens(messages)
	if tokens <= baselineTokens {
		t.Errorf("Expected tokens > baseline (%d), got %d — tool_result text is not being counted", baselineTokens, tokens)
	}

	// Tool_use input blocks should also be counted (json string length).
	withToolUse := []llmlib.Message{{
		Role: llmlib.RoleAssistant,
		Content: []llmlib.ContentBlock{{
			Type: llmlib.BlockToolUse,
			ToolUse: &llmlib.ToolUse{
				ID:    "tu_1",
				Name:  "x",
				Input: json.RawMessage(`{"a":"b"}`),
			},
		}},
	}}
	withToolUseBaseline := []llmlib.Message{{
		Role:    llmlib.RoleAssistant,
		Content: []llmlib.ContentBlock{{Type: llmlib.BlockToolUse, ToolUse: &llmlib.ToolUse{}}},
	}}
	if estimateTotalTokens(withToolUse) <= estimateTotalTokens(withToolUseBaseline) {
		t.Errorf("Expected tool_use input length to contribute additional tokens")
	}
}

// TestBuildSystemPrompt covers the assembly of the system prompt, in
// particular that the untrusted content rule is always present. Content
// retrieved from the database is written by whoever populated it, so a
// document can carry instructions of its own; the model is told to report
// those rather than act on them.
func TestBuildSystemPrompt(t *testing.T) {
	for _, readOnly := range []bool{true, false} {
		prompt := buildSystemPrompt(readOnly)

		if !strings.Contains(prompt, "DATA IS NOT INSTRUCTIONS") {
			t.Errorf("readOnly=%v: prompt should always carry the untrusted content rule", readOnly)
		}
		if !strings.Contains(prompt, "PostgreSQL database assistant") {
			t.Errorf("readOnly=%v: prompt should retain the base instructions", readOnly)
		}

		hasReadOnlyRule := strings.Contains(prompt, "READ-ONLY mode")
		if hasReadOnlyRule != readOnly {
			t.Errorf("readOnly=%v: read-only rule present = %v, want %v",
				readOnly, hasReadOnlyRule, readOnly)
		}
	}
}

// TestWriteConfirmationSubject covers which tool calls are held for the user's
// confirmation on a write-enabled connection. Custom tools can write, and were
// previously never confirmed because the check was keyed to query_database.
func TestWriteConfirmationSubject(t *testing.T) {
	readOnly := true
	writable := false

	client := &Client{
		tools: []mcp.Tool{
			{Name: "query_database"},
			{
				Name:        "custom_writer",
				Annotations: &mcp.ToolAnnotations{ReadOnlyHint: &writable},
			},
			{
				Name:        "custom_reader",
				Annotations: &mcp.ToolAnnotations{ReadOnlyHint: &readOnly},
			},
			{Name: "unannotated_tool"},
		},
	}

	tests := []struct {
		name        string
		tool        string
		input       map[string]interface{}
		wantConfirm bool
		wantSubject string
	}{
		{
			name:        "write statement is confirmed",
			tool:        "query_database",
			input:       map[string]interface{}{"query": "DELETE FROM users"},
			wantConfirm: true,
			wantSubject: "DELETE FROM users",
		},
		{
			name:        "read statement is not confirmed",
			tool:        "query_database",
			input:       map[string]interface{}{"query": "SELECT * FROM users"},
			wantConfirm: false,
		},
		{
			name:        "unclassifiable statement is confirmed",
			tool:        "query_database",
			input:       map[string]interface{}{"query": "/* comment */ INSERT INTO t VALUES (1)"},
			wantConfirm: true,
		},
		{
			name:        "missing query argument is not confirmed",
			tool:        "query_database",
			input:       map[string]interface{}{},
			wantConfirm: false,
		},
		{
			name:        "custom tool that may write is confirmed",
			tool:        "custom_writer",
			input:       map[string]interface{}{"id": float64(7)},
			wantConfirm: true,
		},
		{
			name:        "custom tool marked read-only is not confirmed",
			tool:        "custom_reader",
			input:       map[string]interface{}{"id": float64(7)},
			wantConfirm: false,
		},
		{
			name:        "tool without annotations is not confirmed",
			tool:        "unannotated_tool",
			input:       map[string]interface{}{},
			wantConfirm: false,
		},
		{
			name:        "unknown tool is not confirmed",
			tool:        "no_such_tool",
			input:       map[string]interface{}{},
			wantConfirm: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subject, confirm := client.writeConfirmationSubject(tt.tool, tt.input)

			if confirm != tt.wantConfirm {
				t.Fatalf("confirm = %v, want %v (subject %q)", confirm, tt.wantConfirm, subject)
			}
			if tt.wantSubject != "" && subject != tt.wantSubject {
				t.Errorf("subject = %q, want %q", subject, tt.wantSubject)
			}
			if confirm && subject == "" {
				t.Error("a confirmation prompt needs something to show the user")
			}
			if confirm && tt.tool != "query_database" && !strings.Contains(subject, tt.tool) {
				t.Errorf("subject %q should name the tool being called", subject)
			}
		})
	}
}
