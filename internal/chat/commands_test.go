/*-------------------------------------------------------------------------
 *
 * Tests for Slash Command Handler
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package chat

import (
	"testing"
)

func TestParseSlashCommand(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectedCmd  string
		expectedArgs []string
		shouldBeNil  bool
	}{
		{
			name:         "help command",
			input:        "/help",
			expectedCmd:  "help",
			expectedArgs: []string{},
		},
		{
			name:         "set command with args",
			input:        "/set status-messages on",
			expectedCmd:  "set",
			expectedArgs: []string{"status-messages", "on"},
		},
		{
			name:         "show command with arg",
			input:        "/show llm-provider",
			expectedCmd:  "show",
			expectedArgs: []string{"llm-provider"},
		},
		{
			name:         "list command with arg",
			input:        "/list models",
			expectedCmd:  "list",
			expectedArgs: []string{"models"},
		},
		{
			name:        "not a slash command",
			input:       "help",
			shouldBeNil: true,
		},
		{
			name:        "empty slash command",
			input:       "/",
			shouldBeNil: true,
		},
		{
			name:        "slash with whitespace",
			input:       "/  ",
			shouldBeNil: true,
		},
		{
			name:         "command with extra spaces",
			input:        "/set   status-messages    on",
			expectedCmd:  "set",
			expectedArgs: []string{"status-messages", "on"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := ParseSlashCommand(tt.input)

			if tt.shouldBeNil {
				if cmd != nil {
					t.Errorf("Expected nil, got command: %+v", cmd)
				}
				return
			}

			if cmd == nil {
				t.Fatal("Expected command, got nil")
			}

			if cmd.Command != tt.expectedCmd {
				t.Errorf("Expected command %q, got %q", tt.expectedCmd, cmd.Command)
			}

			if len(cmd.Args) != len(tt.expectedArgs) {
				t.Errorf("Expected %d args, got %d", len(tt.expectedArgs), len(cmd.Args))
			}

			for i, arg := range cmd.Args {
				if i >= len(tt.expectedArgs) {
					break
				}
				if arg != tt.expectedArgs[i] {
					t.Errorf("Expected arg[%d] = %q, got %q", i, tt.expectedArgs[i], arg)
				}
			}
		})
	}
}

func TestHandleSetStatusMessages(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected bool
	}{
		{"on", "on", true},
		{"ON uppercase", "ON", true},
		{"true", "true", true},
		{"1", "1", true},
		{"yes", "yes", true},
		{"off", "off", false},
		{"OFF uppercase", "OFF", false},
		{"false", "false", false},
		{"0", "0", false},
		{"no", "no", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test client with minimal config
			cfg := &Config{
				UI: UIConfig{
					NoColor:               false,
					DisplayStatusMessages: false,
				},
			}
			client, _ := NewClient(cfg)

			// Call handleSetStatusMessages
			client.handleSetStatusMessages(tt.value)

			// Check if the setting was updated correctly
			if client.config.UI.DisplayStatusMessages != tt.expected {
				t.Errorf("Expected DisplayStatusMessages=%v, got %v", tt.expected, client.config.UI.DisplayStatusMessages)
			}

			if client.ui.DisplayStatusMessages != tt.expected {
				t.Errorf("Expected ui.DisplayStatusMessages=%v, got %v", tt.expected, client.ui.DisplayStatusMessages)
			}
		})
	}
}

func TestHandleSetLLMProvider(t *testing.T) {
	tests := []struct {
		name        string
		provider    string
		expectError bool
	}{
		{"anthropic", "anthropic", false},
		{"openai", "openai", false},
		{"ollama", "ollama", false},
		{"ANTHROPIC uppercase", "ANTHROPIC", false},
		{"invalid provider", "invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test client with Anthropic configured
			cfg := &Config{
				LLM: LLMConfig{
					Provider:        "anthropic",
					Model:           "claude-sonnet-4-20250514",
					AnthropicAPIKey: "test-key",
					OpenAIAPIKey:    "test-key",
					OllamaURL:       "http://localhost:11434",
				},
				UI: UIConfig{
					NoColor: false,
				},
			}
			client, _ := NewClient(cfg)

			// Initialize LLM client
			_ = client.initializeLLM()

			// Call handleSetLLMProvider
			handled := client.handleSetLLMProvider(tt.provider)

			// Should always return true (handled)
			if !handled {
				t.Error("Expected handleSetLLMProvider to return true")
			}

			// If not expecting error, check if provider was set
			if !tt.expectError {
				expectedProvider := tt.provider
				if expectedProvider == "ANTHROPIC" {
					expectedProvider = "anthropic" // Should be lowercased
				}
				if client.config.LLM.Provider != expectedProvider {
					t.Errorf("Expected provider %q, got %q", expectedProvider, client.config.LLM.Provider)
				}
			}
		})
	}
}

func TestHandleSetLLMModel(t *testing.T) {
	// Create a test client
	cfg := &Config{
		LLM: LLMConfig{
			Provider:        "anthropic",
			Model:           "claude-sonnet-4-20250514",
			AnthropicAPIKey: "test-key",
		},
		UI: UIConfig{
			NoColor: false,
		},
	}
	client, _ := NewClient(cfg)
	_ = client.initializeLLM()

	// Test setting a new model
	newModel := "claude-3-opus-20240229"
	handled := client.handleSetLLMModel(newModel)

	if !handled {
		t.Error("Expected handleSetLLMModel to return true")
	}

	if client.config.LLM.Model != newModel {
		t.Errorf("Expected model %q, got %q", newModel, client.config.LLM.Model)
	}
}
