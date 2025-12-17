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
				LLM: LLMConfig{
					Provider:  "ollama",
					OllamaURL: "http://localhost:11434",
				},
				UI: UIConfig{
					NoColor:               false,
					DisplayStatusMessages: false,
				},
			}
			client, err := NewClient(cfg, &ConfigOverrides{ProviderSet: true})
			if err != nil {
				t.Fatalf("NewClient failed: %v", err)
			}

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

func TestHandleSetColor(t *testing.T) {
	tests := []struct {
		name            string
		value           string
		expectedNoColor bool // NoColor is the inverse of Color
	}{
		{"on", "on", false}, // color on = noColor false
		{"ON uppercase", "ON", false},
		{"true", "true", false},
		{"1", "1", false},
		{"yes", "yes", false},
		{"off", "off", true}, // color off = noColor true
		{"OFF uppercase", "OFF", true},
		{"false", "false", true},
		{"0", "0", true},
		{"no", "no", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test client with minimal config
			cfg := &Config{
				LLM: LLMConfig{
					Provider:  "ollama",
					OllamaURL: "http://localhost:11434",
				},
				UI: UIConfig{
					NoColor: !tt.expectedNoColor, // Start with opposite value to verify change
				},
			}
			client, err := NewClient(cfg, &ConfigOverrides{ProviderSet: true})
			if err != nil {
				t.Fatalf("NewClient failed: %v", err)
			}

			// Call handleSetColor
			client.handleSetColor(tt.value)

			// Check if the config setting was updated correctly
			if client.config.UI.NoColor != tt.expectedNoColor {
				t.Errorf("Expected config.UI.NoColor=%v, got %v", tt.expectedNoColor, client.config.UI.NoColor)
			}

			// Check if the UI setting was updated correctly
			if client.ui.IsNoColor() != tt.expectedNoColor {
				t.Errorf("Expected ui.IsNoColor()=%v, got %v", tt.expectedNoColor, client.ui.IsNoColor())
			}

			// Check if the preferences were updated correctly
			// preferences.UI.Color should be the opposite of NoColor
			expectedColor := !tt.expectedNoColor
			if client.preferences.UI.Color != expectedColor {
				t.Errorf("Expected preferences.UI.Color=%v, got %v", expectedColor, client.preferences.UI.Color)
			}
		})
	}
}

func TestHandleSetColorInvalidValue(t *testing.T) {
	// Create a test client
	cfg := &Config{
		LLM: LLMConfig{
			Provider:  "ollama",
			OllamaURL: "http://localhost:11434",
		},
		UI: UIConfig{
			NoColor: false, // Start with colors enabled
		},
	}
	client, err := NewClient(cfg, &ConfigOverrides{ProviderSet: true})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	// Store original value
	originalNoColor := client.config.UI.NoColor

	// Call handleSetColor with invalid value
	client.handleSetColor("invalid")

	// Config should remain unchanged
	if client.config.UI.NoColor != originalNoColor {
		t.Errorf("Config should not change for invalid value, expected NoColor=%v, got %v",
			originalNoColor, client.config.UI.NoColor)
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
			// Create a test client with all providers configured
			cfg := &Config{
				LLM: LLMConfig{
					Provider:        "anthropic",
					Model:           "claude-sonnet-4-5-20250929",
					AnthropicAPIKey: "test-key",
					OpenAIAPIKey:    "test-key",
					OllamaURL:       "http://localhost:11434",
				},
				UI: UIConfig{
					NoColor: false,
				},
			}
			client, err := NewClient(cfg, &ConfigOverrides{ProviderSet: true, ModelSet: true})
			if err != nil {
				t.Fatalf("NewClient failed: %v", err)
			}

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
			Model:           "claude-sonnet-4-5-20250929",
			AnthropicAPIKey: "test-key",
		},
		UI: UIConfig{
			NoColor: false,
		},
	}
	client, err := NewClient(cfg, &ConfigOverrides{ProviderSet: true, ModelSet: true})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	_ = client.initializeLLM()

	// Test setting a new model (use a valid model from the Anthropic list)
	newModel := "claude-3-haiku-20240307"
	handled := client.handleSetLLMModel(newModel)

	if !handled {
		t.Error("Expected handleSetLLMModel to return true")
	}

	if client.config.LLM.Model != newModel {
		t.Errorf("Expected model %q, got %q", newModel, client.config.LLM.Model)
	}
}

func TestParseQuotedArgs(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "simple args",
			input:    "arg1 arg2 arg3",
			expected: []string{"arg1", "arg2", "arg3"},
		},
		{
			name:     "double quoted arg",
			input:    `query_text="What is pgAgent?"`,
			expected: []string{`query_text=What is pgAgent?`},
		},
		{
			name:     "single quoted arg",
			input:    `query_text='What is pgAgent?'`,
			expected: []string{`query_text=What is pgAgent?`},
		},
		{
			name:     "mixed quotes",
			input:    `query_text="How does PostgreSQL work?" table_name='users'`,
			expected: []string{`query_text=How does PostgreSQL work?`, `table_name=users`},
		},
		{
			name:     "arg with spaces in quotes",
			input:    `query_text="PostgreSQL vector search capabilities"`,
			expected: []string{`query_text=PostgreSQL vector search capabilities`},
		},
		{
			name:     "multiple args with and without quotes",
			input:    `setup-semantic-search query_text="What is pgAgent?" table_name=docs`,
			expected: []string{`setup-semantic-search`, `query_text=What is pgAgent?`, `table_name=docs`},
		},
		{
			name:     "escaped quotes",
			input:    `query_text="She said \"hello\""`,
			expected: []string{`query_text=She said "hello"`},
		},
		{
			name:     "empty string",
			input:    "",
			expected: []string{},
		},
		{
			name:     "only spaces",
			input:    "   ",
			expected: []string{},
		},
		{
			name:     "extra spaces between args",
			input:    "arg1   arg2    arg3",
			expected: []string{"arg1", "arg2", "arg3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseQuotedArgs(tt.input)

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d args, got %d\nExpected: %v\nGot: %v",
					len(tt.expected), len(result), tt.expected, result)
				return
			}

			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("Arg %d: expected %q, got %q", i, tt.expected[i], result[i])
				}
			}
		})
	}
}

func TestParseHistoryCommands(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectedCmd  string
		expectedArgs []string
		shouldBeNil  bool
	}{
		{
			name:         "history list",
			input:        "/history",
			expectedCmd:  "history",
			expectedArgs: []string{},
		},
		{
			name:         "history list explicit",
			input:        "/history list",
			expectedCmd:  "history",
			expectedArgs: []string{"list"},
		},
		{
			name:         "history load",
			input:        "/history load conv_1234567890",
			expectedCmd:  "history",
			expectedArgs: []string{"load", "conv_1234567890"},
		},
		{
			name:         "history rename with unquoted title",
			input:        "/history rename conv_123 New Title",
			expectedCmd:  "history",
			expectedArgs: []string{"rename", "conv_123", "New", "Title"},
		},
		{
			name:         "history delete",
			input:        "/history delete conv_123",
			expectedCmd:  "history",
			expectedArgs: []string{"delete", "conv_123"},
		},
		{
			name:         "history delete-all",
			input:        "/history delete-all",
			expectedCmd:  "history",
			expectedArgs: []string{"delete-all"},
		},
		{
			name:         "new conversation",
			input:        "/new",
			expectedCmd:  "new",
			expectedArgs: []string{},
		},
		{
			name:         "save conversation",
			input:        "/save",
			expectedCmd:  "save",
			expectedArgs: []string{},
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
				t.Errorf("Expected %d args, got %d\nExpected: %v\nGot: %v",
					len(tt.expectedArgs), len(cmd.Args), tt.expectedArgs, cmd.Args)
				return
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

func TestParsePromptCommand(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectedCmd  string
		expectedArgs []string
		shouldBeNil  bool
	}{
		{
			name:         "prompt with quoted args",
			input:        `/prompt setup-semantic-search query_text="What is pgAgent?"`,
			expectedCmd:  "prompt",
			expectedArgs: []string{"setup-semantic-search", `query_text=What is pgAgent?`},
		},
		{
			name:         "prompt with single quotes",
			input:        `/prompt setup-semantic-search query_text='PostgreSQL vector search'`,
			expectedCmd:  "prompt",
			expectedArgs: []string{"setup-semantic-search", `query_text=PostgreSQL vector search`},
		},
		{
			name:         "prompt with multiple quoted args",
			input:        `/prompt setup-semantic-search query_text="vector search" table_name="docs"`,
			expectedCmd:  "prompt",
			expectedArgs: []string{"setup-semantic-search", `query_text=vector search`, `table_name=docs`},
		},
		{
			name:         "prompt without args",
			input:        "/prompt explore-database",
			expectedCmd:  "prompt",
			expectedArgs: []string{"explore-database"},
		},
		{
			name:         "prompt with unquoted args",
			input:        "/prompt diagnose-query-issue issue_description=no-results",
			expectedCmd:  "prompt",
			expectedArgs: []string{"diagnose-query-issue", "issue_description=no-results"},
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
				t.Errorf("Expected %d args, got %d\nExpected: %v\nGot: %v",
					len(tt.expectedArgs), len(cmd.Args), tt.expectedArgs, cmd.Args)
				return
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
