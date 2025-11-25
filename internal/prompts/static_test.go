/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package prompts

import (
	"testing"

	"pgedge-postgres-mcp/internal/definitions"
)

func TestRegisterStatic_BasicPrompt(t *testing.T) {
	registry := NewRegistry()

	def := definitions.PromptDefinition{
		Name:        "test-prompt",
		Description: "Test description",
		Arguments:   []definitions.ArgumentDef{},
		Messages: []definitions.MessageDef{
			{
				Role: "user",
				Content: definitions.ContentDef{
					Type: "text",
					Text: "Test message",
				},
			},
		},
	}

	err := registry.RegisterStatic(def)
	if err != nil {
		t.Fatalf("Failed to register prompt: %v", err)
	}

	// Verify prompt is registered
	prompt, exists := registry.Get("test-prompt")
	if !exists {
		t.Fatal("Prompt not found in registry")
	}

	if prompt.Definition.Name != "test-prompt" {
		t.Errorf("Expected name 'test-prompt', got '%s'", prompt.Definition.Name)
	}

	if prompt.Definition.Description != "Test description" {
		t.Errorf("Expected description 'Test description', got '%s'", prompt.Definition.Description)
	}
}

func TestRegisterStatic_WithArguments(t *testing.T) {
	registry := NewRegistry()

	def := definitions.PromptDefinition{
		Name: "test-prompt",
		Arguments: []definitions.ArgumentDef{
			{Name: "arg1", Description: "First arg", Required: true},
			{Name: "arg2", Description: "Second arg", Required: false},
		},
		Messages: []definitions.MessageDef{
			{
				Role: "user",
				Content: definitions.ContentDef{
					Type: "text",
					Text: "Test",
				},
			},
		},
	}

	err := registry.RegisterStatic(def)
	if err != nil {
		t.Fatalf("Failed to register prompt: %v", err)
	}

	prompt, _ := registry.Get("test-prompt")
	if len(prompt.Definition.Arguments) != 2 {
		t.Errorf("Expected 2 arguments, got %d", len(prompt.Definition.Arguments))
	}

	if prompt.Definition.Arguments[0].Name != "arg1" {
		t.Errorf("Expected first argument name 'arg1', got '%s'", prompt.Definition.Arguments[0].Name)
	}

	if !prompt.Definition.Arguments[0].Required {
		t.Error("Expected first argument to be required")
	}

	if prompt.Definition.Arguments[1].Required {
		t.Error("Expected second argument to be optional")
	}
}

func TestRegisterStatic_TemplateInterpolation(t *testing.T) {
	registry := NewRegistry()

	def := definitions.PromptDefinition{
		Name: "test-prompt",
		Arguments: []definitions.ArgumentDef{
			{Name: "name", Required: true},
			{Name: "action", Required: true},
		},
		Messages: []definitions.MessageDef{
			{
				Role: "user",
				Content: definitions.ContentDef{
					Type: "text",
					Text: "Hello {{name}}, please {{action}}",
				},
			},
		},
	}

	err := registry.RegisterStatic(def)
	if err != nil {
		t.Fatalf("Failed to register prompt: %v", err)
	}

	// Execute prompt with arguments
	result, err := registry.Execute("test-prompt", map[string]string{
		"name":   "Alice",
		"action": "run tests",
	})

	if err != nil {
		t.Fatalf("Failed to execute prompt: %v", err)
	}

	if len(result.Messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(result.Messages))
	}

	expected := "Hello Alice, please run tests"
	if result.Messages[0].Content.Text != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result.Messages[0].Content.Text)
	}
}

func TestRegisterStatic_MultipleMessages(t *testing.T) {
	registry := NewRegistry()

	def := definitions.PromptDefinition{
		Name: "test-prompt",
		Messages: []definitions.MessageDef{
			{
				Role: "user",
				Content: definitions.ContentDef{
					Type: "text",
					Text: "First message",
				},
			},
			{
				Role: "assistant",
				Content: definitions.ContentDef{
					Type: "text",
					Text: "Second message",
				},
			},
			{
				Role: "system",
				Content: definitions.ContentDef{
					Type: "text",
					Text: "Third message",
				},
			},
		},
	}

	err := registry.RegisterStatic(def)
	if err != nil {
		t.Fatalf("Failed to register prompt: %v", err)
	}

	result, err := registry.Execute("test-prompt", map[string]string{})
	if err != nil {
		t.Fatalf("Failed to execute prompt: %v", err)
	}

	if len(result.Messages) != 3 {
		t.Fatalf("Expected 3 messages, got %d", len(result.Messages))
	}

	if result.Messages[0].Role != "user" {
		t.Errorf("Expected first message role 'user', got '%s'", result.Messages[0].Role)
	}

	if result.Messages[1].Role != "assistant" {
		t.Errorf("Expected second message role 'assistant', got '%s'", result.Messages[1].Role)
	}

	if result.Messages[2].Role != "system" {
		t.Errorf("Expected third message role 'system', got '%s'", result.Messages[2].Role)
	}
}

func TestRegisterStatic_MultipleTemplates(t *testing.T) {
	registry := NewRegistry()

	def := definitions.PromptDefinition{
		Name: "test-prompt",
		Arguments: []definitions.ArgumentDef{
			{Name: "table", Required: true},
		},
		Messages: []definitions.MessageDef{
			{
				Role: "user",
				Content: definitions.ContentDef{
					Type: "text",
					Text: "Analyze table {{table}}",
				},
			},
			{
				Role: "assistant",
				Content: definitions.ContentDef{
					Type: "text",
					Text: "I will analyze {{table}}",
				},
			},
		},
	}

	err := registry.RegisterStatic(def)
	if err != nil {
		t.Fatalf("Failed to register prompt: %v", err)
	}

	result, err := registry.Execute("test-prompt", map[string]string{"table": "users"})
	if err != nil {
		t.Fatalf("Failed to execute prompt: %v", err)
	}

	if !contains(result.Messages[0].Content.Text, "users") {
		t.Error("First message should contain interpolated value 'users'")
	}

	if !contains(result.Messages[1].Content.Text, "users") {
		t.Error("Second message should contain interpolated value 'users'")
	}
}

func TestRegisterStatic_EmptyArgs(t *testing.T) {
	registry := NewRegistry()

	def := definitions.PromptDefinition{
		Name: "test-prompt",
		Arguments: []definitions.ArgumentDef{
			{Name: "optional", Required: false},
		},
		Messages: []definitions.MessageDef{
			{
				Role: "user",
				Content: definitions.ContentDef{
					Type: "text",
					Text: "Value: {{optional}}",
				},
			},
		},
	}

	err := registry.RegisterStatic(def)
	if err != nil {
		t.Fatalf("Failed to register prompt: %v", err)
	}

	// Execute without providing the optional argument
	result, err := registry.Execute("test-prompt", map[string]string{})
	if err != nil {
		t.Fatalf("Failed to execute prompt: %v", err)
	}

	// Template should still contain placeholder since arg wasn't provided
	if !contains(result.Messages[0].Content.Text, "{{optional}}") {
		t.Error("Template should contain unreplaced placeholder")
	}
}

func TestInterpolateTemplate(t *testing.T) {
	tests := []struct {
		template string
		args     map[string]string
		expected string
	}{
		{
			template: "No placeholders",
			args:     map[string]string{},
			expected: "No placeholders",
		},
		{
			template: "Hello {{name}}",
			args:     map[string]string{"name": "World"},
			expected: "Hello World",
		},
		{
			template: "{{greeting}} {{name}}!",
			args:     map[string]string{"greeting": "Hi", "name": "Alice"},
			expected: "Hi Alice!",
		},
		{
			template: "Repeat {{val}} and {{val}}",
			args:     map[string]string{"val": "test"},
			expected: "Repeat test and test",
		},
		{
			template: "Unused {{placeholder}}",
			args:     map[string]string{"other": "value"},
			expected: "Unused {{placeholder}}",
		},
	}

	for _, tt := range tests {
		result := interpolateTemplate(tt.template, tt.args)
		if result != tt.expected {
			t.Errorf("For template '%s', expected '%s', got '%s'",
				tt.template, tt.expected, result)
		}
	}
}

func TestRegisterStatic_InRegistry(t *testing.T) {
	registry := NewRegistry()

	def := definitions.PromptDefinition{
		Name: "test-prompt",
		Messages: []definitions.MessageDef{
			{
				Role: "user",
				Content: definitions.ContentDef{
					Type: "text",
					Text: "Test",
				},
			},
		},
	}

	err := registry.RegisterStatic(def)
	if err != nil {
		t.Fatalf("Failed to register prompt: %v", err)
	}

	// Verify prompt appears in list
	prompts := registry.List()
	found := false
	for _, p := range prompts {
		if p.Name == "test-prompt" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Registered prompt not found in List()")
	}
}

func TestRegisterStatic_Description(t *testing.T) {
	registry := NewRegistry()

	def := definitions.PromptDefinition{
		Name:        "test-prompt",
		Description: "Custom description",
		Messages: []definitions.MessageDef{
			{
				Role: "user",
				Content: definitions.ContentDef{
					Type: "text",
					Text: "Test",
				},
			},
		},
	}

	err := registry.RegisterStatic(def)
	if err != nil {
		t.Fatalf("Failed to register prompt: %v", err)
	}

	result, err := registry.Execute("test-prompt", map[string]string{})
	if err != nil {
		t.Fatalf("Failed to execute prompt: %v", err)
	}

	if result.Description != "Custom description" {
		t.Errorf("Expected description 'Custom description', got '%s'", result.Description)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			len(s) > len(substr)+1 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
