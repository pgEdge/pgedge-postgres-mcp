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
	"fmt"
	"regexp"
)

var (
	// Valid message roles
	validRoles = map[string]bool{
		"user":      true,
		"assistant": true,
		"system":    true,
	}

	// Valid content types
	validContentTypes = map[string]bool{
		"text":     true,
		"image":    true,
		"resource": true,
	}

	// Valid resource types
	validResourceTypes = map[string]bool{
		"sql":    true,
		"static": true,
	}

	// Pattern to find template placeholders like {{arg_name}}
	placeholderPattern = regexp.MustCompile(`\{\{(\w+)\}\}`)
)

// ValidateDefinitions validates all prompt and resource definitions
func ValidateDefinitions(defs *Definitions) error {
	// Track unique names/URIs
	promptNames := make(map[string]bool)
	resourceURIs := make(map[string]bool)

	// Validate prompts
	for i, prompt := range defs.Prompts {
		if err := validatePrompt(&prompt, promptNames); err != nil {
			return fmt.Errorf("prompt %d: %w", i, err)
		}
	}

	// Validate resources
	for i := range defs.Resources {
		if err := validateResource(&defs.Resources[i], resourceURIs); err != nil {
			return fmt.Errorf("resource %d: %w", i, err)
		}
	}

	return nil
}

// validatePrompt validates a single prompt definition
func validatePrompt(prompt *PromptDefinition, seenNames map[string]bool) error {
	// Check required fields
	if prompt.Name == "" {
		return fmt.Errorf("name is required")
	}

	// Check uniqueness
	if seenNames[prompt.Name] {
		return fmt.Errorf("duplicate prompt name: %s", prompt.Name)
	}
	seenNames[prompt.Name] = true

	// Check messages
	if len(prompt.Messages) == 0 {
		return fmt.Errorf("at least one message is required")
	}

	// Build map of valid argument names
	argNames := make(map[string]bool)
	for _, arg := range prompt.Arguments {
		if arg.Name == "" {
			return fmt.Errorf("argument name is required")
		}
		argNames[arg.Name] = true
	}

	// Validate each message
	for j, msg := range prompt.Messages {
		if err := validateMessage(&msg, argNames); err != nil {
			return fmt.Errorf("message %d: %w", j, err)
		}
	}

	return nil
}

// validateMessage validates a prompt message
func validateMessage(msg *MessageDef, validArgs map[string]bool) error {
	// Validate role
	if !validRoles[msg.Role] {
		return fmt.Errorf("invalid role %q (must be user, assistant, or system)", msg.Role)
	}

	// Validate content type
	if !validContentTypes[msg.Content.Type] {
		return fmt.Errorf("invalid content type %q (must be text, image, or resource)", msg.Content.Type)
	}

	// Type-specific validation
	switch msg.Content.Type {
	case "text":
		if msg.Content.Text == "" {
			return fmt.Errorf("text content requires 'text' field")
		}
		// Check that template placeholders reference valid arguments
		matches := placeholderPattern.FindAllStringSubmatch(msg.Content.Text, -1)
		for _, match := range matches {
			argName := match[1]
			if !validArgs[argName] {
				return fmt.Errorf("template references undefined argument: %s", argName)
			}
		}
	case "image":
		if msg.Content.Data == "" {
			return fmt.Errorf("image content requires 'data' field")
		}
		if msg.Content.MimeType == "" {
			return fmt.Errorf("image content requires 'mimeType' field")
		}
	case "resource":
		if msg.Content.URI == "" {
			return fmt.Errorf("resource content requires 'uri' field")
		}
	}

	return nil
}

// validateResource validates a single resource definition
func validateResource(res *ResourceDefinition, seenURIs map[string]bool) error {
	// Check required fields
	if res.URI == "" {
		return fmt.Errorf("uri is required")
	}
	if res.Name == "" {
		return fmt.Errorf("name is required")
	}
	if res.Type == "" {
		return fmt.Errorf("type is required")
	}

	// Check uniqueness
	if seenURIs[res.URI] {
		return fmt.Errorf("duplicate resource URI: %s", res.URI)
	}
	seenURIs[res.URI] = true

	// Validate type
	if !validResourceTypes[res.Type] {
		return fmt.Errorf("invalid type %q (must be sql or static)", res.Type)
	}

	// Set default mime type if not specified
	if res.MimeType == "" {
		res.MimeType = "application/json"
	}

	// Type-specific validation
	switch res.Type {
	case "sql":
		if res.SQL == "" {
			return fmt.Errorf("sql type requires 'sql' field")
		}
		// Note: We could warn about potentially destructive queries (INSERT, UPDATE, DELETE, etc.)
		// but that's left to the user's discretion
	case "static":
		if res.Data == nil {
			return fmt.Errorf("static type requires 'data' field")
		}
	}

	return nil
}

// GetTemplatePlaceholders extracts all {{placeholder}} names from a template string
func GetTemplatePlaceholders(template string) []string {
	matches := placeholderPattern.FindAllStringSubmatch(template, -1)
	placeholders := make([]string, 0, len(matches))
	for _, match := range matches {
		placeholders = append(placeholders, match[1])
	}
	return placeholders
}
