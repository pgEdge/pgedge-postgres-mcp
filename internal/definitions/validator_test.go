/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package definitions

import (
	"strings"
	"testing"
)

func TestValidateDefinitions_ValidPrompt(t *testing.T) {
	defs := &Definitions{
		Prompts: []PromptDefinition{
			{
				Name:        "test-prompt",
				Description: "Test",
				Arguments: []ArgumentDef{
					{Name: "arg1", Required: true},
				},
				Messages: []MessageDef{
					{
						Role: "user",
						Content: ContentDef{
							Type: "text",
							Text: "Test {{arg1}}",
						},
					},
				},
			},
		},
	}

	err := ValidateDefinitions(defs)
	if err != nil {
		t.Errorf("Expected valid prompt to pass, got error: %v", err)
	}
}

func TestValidateDefinitions_PromptMissingName(t *testing.T) {
	defs := &Definitions{
		Prompts: []PromptDefinition{
			{
				Messages: []MessageDef{
					{Role: "user", Content: ContentDef{Type: "text", Text: "Test"}},
				},
			},
		},
	}

	err := ValidateDefinitions(defs)
	if err == nil {
		t.Error("Expected error for missing prompt name")
	}
	if !strings.Contains(err.Error(), "name is required") {
		t.Errorf("Expected 'name is required' error, got: %v", err)
	}
}

func TestValidateDefinitions_PromptNoMessages(t *testing.T) {
	defs := &Definitions{
		Prompts: []PromptDefinition{
			{Name: "test", Messages: []MessageDef{}},
		},
	}

	err := ValidateDefinitions(defs)
	if err == nil {
		t.Error("Expected error for prompt with no messages")
	}
}

func TestValidateDefinitions_DuplicatePromptName(t *testing.T) {
	defs := &Definitions{
		Prompts: []PromptDefinition{
			{
				Name: "test",
				Messages: []MessageDef{
					{Role: "user", Content: ContentDef{Type: "text", Text: "A"}},
				},
			},
			{
				Name: "test",
				Messages: []MessageDef{
					{Role: "user", Content: ContentDef{Type: "text", Text: "B"}},
				},
			},
		},
	}

	err := ValidateDefinitions(defs)
	if err == nil {
		t.Error("Expected error for duplicate prompt name")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("Expected 'duplicate' error, got: %v", err)
	}
}

func TestValidateDefinitions_InvalidRole(t *testing.T) {
	defs := &Definitions{
		Prompts: []PromptDefinition{
			{
				Name: "test",
				Messages: []MessageDef{
					{Role: "invalid", Content: ContentDef{Type: "text", Text: "Test"}},
				},
			},
		},
	}

	err := ValidateDefinitions(defs)
	if err == nil {
		t.Error("Expected error for invalid role")
	}
}

func TestValidateDefinitions_ValidRoles(t *testing.T) {
	roles := []string{"user", "assistant", "system"}
	for _, role := range roles {
		defs := &Definitions{
			Prompts: []PromptDefinition{
				{
					Name: "test",
					Messages: []MessageDef{
						{Role: role, Content: ContentDef{Type: "text", Text: "Test"}},
					},
				},
			},
		}

		err := ValidateDefinitions(defs)
		if err != nil {
			t.Errorf("Expected role '%s' to be valid, got error: %v", role, err)
		}
	}
}

func TestValidateDefinitions_InvalidContentType(t *testing.T) {
	defs := &Definitions{
		Prompts: []PromptDefinition{
			{
				Name: "test",
				Messages: []MessageDef{
					{Role: "user", Content: ContentDef{Type: "invalid", Text: "Test"}},
				},
			},
		},
	}

	err := ValidateDefinitions(defs)
	if err == nil {
		t.Error("Expected error for invalid content type")
	}
}

func TestValidateDefinitions_TextContentMissingText(t *testing.T) {
	defs := &Definitions{
		Prompts: []PromptDefinition{
			{
				Name: "test",
				Messages: []MessageDef{
					{Role: "user", Content: ContentDef{Type: "text"}},
				},
			},
		},
	}

	err := ValidateDefinitions(defs)
	if err == nil {
		t.Error("Expected error for text content without text field")
	}
}

func TestValidateDefinitions_UndefinedArgument(t *testing.T) {
	defs := &Definitions{
		Prompts: []PromptDefinition{
			{
				Name: "test",
				Arguments: []ArgumentDef{
					{Name: "arg1"},
				},
				Messages: []MessageDef{
					{
						Role: "user",
						Content: ContentDef{
							Type: "text",
							Text: "Test {{undefined_arg}}",
						},
					},
				},
			},
		},
	}

	err := ValidateDefinitions(defs)
	if err == nil {
		t.Error("Expected error for undefined argument in template")
	}
	if !strings.Contains(err.Error(), "undefined argument") {
		t.Errorf("Expected 'undefined argument' error, got: %v", err)
	}
}

func TestValidateDefinitions_ValidSQLResource(t *testing.T) {
	defs := &Definitions{
		Resources: []ResourceDefinition{
			{
				URI:  "custom://test",
				Name: "Test",
				Type: "sql",
				SQL:  "SELECT 1",
			},
		},
	}

	err := ValidateDefinitions(defs)
	if err != nil {
		t.Errorf("Expected valid SQL resource to pass, got error: %v", err)
	}
}

func TestValidateDefinitions_ValidStaticResource(t *testing.T) {
	defs := &Definitions{
		Resources: []ResourceDefinition{
			{
				URI:  "custom://test",
				Name: "Test",
				Type: "static",
				Data: "test value",
			},
		},
	}

	err := ValidateDefinitions(defs)
	if err != nil {
		t.Errorf("Expected valid static resource to pass, got error: %v", err)
	}
}

func TestValidateDefinitions_ResourceMissingURI(t *testing.T) {
	defs := &Definitions{
		Resources: []ResourceDefinition{
			{Name: "Test", Type: "static", Data: "test"},
		},
	}

	err := ValidateDefinitions(defs)
	if err == nil {
		t.Error("Expected error for missing resource URI")
	}
}

func TestValidateDefinitions_ResourceMissingName(t *testing.T) {
	defs := &Definitions{
		Resources: []ResourceDefinition{
			{URI: "custom://test", Type: "static", Data: "test"},
		},
	}

	err := ValidateDefinitions(defs)
	if err == nil {
		t.Error("Expected error for missing resource name")
	}
}

func TestValidateDefinitions_ResourceMissingType(t *testing.T) {
	defs := &Definitions{
		Resources: []ResourceDefinition{
			{URI: "custom://test", Name: "Test"},
		},
	}

	err := ValidateDefinitions(defs)
	if err == nil {
		t.Error("Expected error for missing resource type")
	}
}

func TestValidateDefinitions_DuplicateResourceURI(t *testing.T) {
	defs := &Definitions{
		Resources: []ResourceDefinition{
			{URI: "custom://test", Name: "Test1", Type: "static", Data: "a"},
			{URI: "custom://test", Name: "Test2", Type: "static", Data: "b"},
		},
	}

	err := ValidateDefinitions(defs)
	if err == nil {
		t.Error("Expected error for duplicate resource URI")
	}
}

func TestValidateDefinitions_InvalidResourceType(t *testing.T) {
	defs := &Definitions{
		Resources: []ResourceDefinition{
			{URI: "custom://test", Name: "Test", Type: "invalid"},
		},
	}

	err := ValidateDefinitions(defs)
	if err == nil {
		t.Error("Expected error for invalid resource type")
	}
}

func TestValidateDefinitions_SQLResourceMissingSQL(t *testing.T) {
	defs := &Definitions{
		Resources: []ResourceDefinition{
			{URI: "custom://test", Name: "Test", Type: "sql"},
		},
	}

	err := ValidateDefinitions(defs)
	if err == nil {
		t.Error("Expected error for SQL resource without sql field")
	}
}

func TestValidateDefinitions_StaticResourceMissingData(t *testing.T) {
	defs := &Definitions{
		Resources: []ResourceDefinition{
			{URI: "custom://test", Name: "Test", Type: "static"},
		},
	}

	err := ValidateDefinitions(defs)
	if err == nil {
		t.Error("Expected error for static resource without data field")
	}
}

func TestValidateDefinitions_DefaultMimeType(t *testing.T) {
	defs := &Definitions{
		Resources: []ResourceDefinition{
			{
				URI:  "custom://test",
				Name: "Test",
				Type: "static",
				Data: "test",
			},
		},
	}

	err := ValidateDefinitions(defs)
	if err != nil {
		t.Errorf("Validation failed: %v", err)
	}

	if defs.Resources[0].MimeType != "application/json" {
		t.Errorf("Expected default mimeType 'application/json', got '%s'", defs.Resources[0].MimeType)
	}
}

func TestValidateDefinitions_ArgumentMissingName(t *testing.T) {
	defs := &Definitions{
		Prompts: []PromptDefinition{
			{
				Name: "test",
				Arguments: []ArgumentDef{
					{Description: "No name"},
				},
				Messages: []MessageDef{
					{Role: "user", Content: ContentDef{Type: "text", Text: "Test"}},
				},
			},
		},
	}

	err := ValidateDefinitions(defs)
	if err == nil {
		t.Error("Expected error for argument without name")
	}
}

func TestGetTemplatePlaceholders(t *testing.T) {
	tests := []struct {
		template    string
		expected    []string
		description string
	}{
		{
			template:    "No placeholders",
			expected:    []string{},
			description: "Text without placeholders",
		},
		{
			template:    "Hello {{name}}",
			expected:    []string{"name"},
			description: "Single placeholder",
		},
		{
			template:    "{{greeting}} {{name}}!",
			expected:    []string{"greeting", "name"},
			description: "Multiple placeholders",
		},
		{
			template:    "{{arg1}} and {{arg1}} again",
			expected:    []string{"arg1", "arg1"},
			description: "Duplicate placeholders",
		},
		{
			template:    "Nested {{outer_{{inner}}}}",
			expected:    []string{"inner"},
			description: "Nested braces",
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			result := GetTemplatePlaceholders(tt.template)
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d placeholders, got %d", len(tt.expected), len(result))
				return
			}
			for i, exp := range tt.expected {
				if result[i] != exp {
					t.Errorf("Expected placeholder '%s' at index %d, got '%s'", exp, i, result[i])
				}
			}
		})
	}
}

func TestValidateDefinitions_EmptyDefinitions(t *testing.T) {
	defs := &Definitions{}

	err := ValidateDefinitions(defs)
	if err != nil {
		t.Errorf("Expected empty definitions to be valid, got error: %v", err)
	}
}

func TestValidateDefinitions_MultipleErrors(t *testing.T) {
	defs := &Definitions{
		Prompts: []PromptDefinition{
			{Name: "test1"}, // Missing messages
			{Messages: []MessageDef{{Role: "user", Content: ContentDef{Type: "text", Text: "A"}}}}, // Missing name
		},
	}

	err := ValidateDefinitions(defs)
	if err == nil {
		t.Error("Expected error for multiple validation failures")
	}
	// Should report first error encountered
}
