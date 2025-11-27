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
	"strings"
	"testing"
)

func TestNewRegistry(t *testing.T) {
	registry := NewRegistry()

	if registry == nil {
		t.Fatal("Expected registry to be created, got nil")
	}

	if registry.prompts == nil {
		t.Error("Expected prompts map to be initialized")
	}
}

func TestRegisterAndGet(t *testing.T) {
	registry := NewRegistry()

	// Create a test prompt
	testPrompt := SetupSemanticSearch()

	// Register it
	registry.Register("setup-semantic-search", testPrompt)

	// Retrieve it
	prompt, found := registry.Get("setup-semantic-search")

	if !found {
		t.Fatal("Expected to find registered prompt")
	}

	if prompt.Definition.Name != "setup-semantic-search" {
		t.Errorf("Expected prompt name 'setup-semantic-search', got %q", prompt.Definition.Name)
	}
}

func TestGetNonExistent(t *testing.T) {
	registry := NewRegistry()

	_, found := registry.Get("non-existent")

	if found {
		t.Error("Expected not to find non-existent prompt")
	}
}

func TestList(t *testing.T) {
	registry := NewRegistry()

	// Register multiple prompts
	registry.Register("setup-semantic-search", SetupSemanticSearch())
	registry.Register("explore-database", ExploreDatabase())
	registry.Register("diagnose-query-issue", DiagnoseQueryIssue())
	registry.Register("design-schema", DesignSchema())

	// List all prompts
	prompts := registry.List()

	if len(prompts) != 4 {
		t.Errorf("Expected 4 prompts, got %d", len(prompts))
	}

	// Verify all prompts have required fields
	for _, prompt := range prompts {
		if prompt.Name == "" {
			t.Error("Prompt is missing name")
		}
		if prompt.Description == "" {
			t.Errorf("Prompt %q is missing description", prompt.Name)
		}
	}
}

func TestExecute(t *testing.T) {
	registry := NewRegistry()
	registry.Register("setup-semantic-search", SetupSemanticSearch())

	args := map[string]string{
		"query_text": "What is PostgreSQL?",
	}

	result, err := registry.Execute("setup-semantic-search", args)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result.Description == "" {
		t.Error("Result description should not be empty")
	}

	if len(result.Messages) == 0 {
		t.Error("Result should have at least one message")
	}
}

func TestExecuteNonExistent(t *testing.T) {
	registry := NewRegistry()

	// Register a prompt so we can verify it appears in the error message
	registry.Register("test-prompt", SetupSemanticSearch())

	args := map[string]string{}
	_, err := registry.Execute("non-existent", args)

	if err == nil {
		t.Error("Expected error when executing non-existent prompt")
	}

	// Verify error message contains the prompt name and lists available prompts
	errMsg := err.Error()
	if !strings.Contains(errMsg, "non-existent") {
		t.Errorf("Error should contain the requested prompt name, got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "Available prompts") {
		t.Errorf("Error should list available prompts, got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "test-prompt") {
		t.Errorf("Error should include the registered prompt name, got: %s", errMsg)
	}
}

func TestSetupSemanticSearchPrompt(t *testing.T) {
	prompt := SetupSemanticSearch()

	// Verify prompt structure
	if prompt.Definition.Name != "setup-semantic-search" {
		t.Errorf("Expected name 'setup-semantic-search', got %q", prompt.Definition.Name)
	}

	if prompt.Definition.Description == "" {
		t.Error("Description should not be empty")
	}

	// Verify arguments
	if len(prompt.Definition.Arguments) != 1 {
		t.Errorf("Expected 1 argument, got %d", len(prompt.Definition.Arguments))
	}

	// Check query_text argument
	if prompt.Definition.Arguments[0].Name != "query_text" {
		t.Errorf("Expected argument name 'query_text', got %q",
			prompt.Definition.Arguments[0].Name)
	}
	if !prompt.Definition.Arguments[0].Required {
		t.Error("query_text should be required")
	}

	// Test handler execution
	args := map[string]string{
		"query_text": "What is PostgreSQL?",
	}
	result := prompt.Handler(args)

	if result.Description == "" {
		t.Error("Result description should not be empty")
	}

	if len(result.Messages) == 0 {
		t.Error("Result should have at least one message")
	}

	if result.Messages[0].Role != "user" {
		t.Errorf("Expected first message role 'user', got %q", result.Messages[0].Role)
	}

	if result.Messages[0].Content.Type != "text" {
		t.Errorf("Expected content type 'text', got %q", result.Messages[0].Content.Type)
	}

	if result.Messages[0].Content.Text == "" {
		t.Error("Message text should not be empty")
	}
}

func TestExploreDatabasePrompt(t *testing.T) {
	prompt := ExploreDatabase()

	// Verify prompt structure
	if prompt.Definition.Name != "explore-database" {
		t.Errorf("Expected name 'explore-database', got %q", prompt.Definition.Name)
	}

	if prompt.Definition.Description == "" {
		t.Error("Description should not be empty")
	}

	// Verify no arguments required
	if len(prompt.Definition.Arguments) != 0 {
		t.Errorf("Expected 0 arguments, got %d", len(prompt.Definition.Arguments))
	}

	// Test handler execution
	args := map[string]string{}
	result := prompt.Handler(args)

	if result.Description == "" {
		t.Error("Result description should not be empty")
	}

	if len(result.Messages) == 0 {
		t.Error("Result should have at least one message")
	}

	if result.Messages[0].Role != "user" {
		t.Errorf("Expected first message role 'user', got %q", result.Messages[0].Role)
	}

	if result.Messages[0].Content.Text == "" {
		t.Error("Message text should not be empty")
	}
}

func TestDiagnoseQueryIssuePrompt(t *testing.T) {
	prompt := DiagnoseQueryIssue()

	// Verify prompt structure
	if prompt.Definition.Name != "diagnose-query-issue" {
		t.Errorf("Expected name 'diagnose-query-issue', got %q", prompt.Definition.Name)
	}

	if prompt.Definition.Description == "" {
		t.Error("Description should not be empty")
	}

	// Verify arguments
	if len(prompt.Definition.Arguments) != 1 {
		t.Errorf("Expected 1 argument, got %d", len(prompt.Definition.Arguments))
	}

	// Check issue_description argument
	if prompt.Definition.Arguments[0].Name != "issue_description" {
		t.Errorf("Expected argument name 'issue_description', got %q",
			prompt.Definition.Arguments[0].Name)
	}

	if prompt.Definition.Arguments[0].Required {
		t.Error("issue_description should be optional")
	}

	// Test handler execution with argument
	argsWithDesc := map[string]string{
		"issue_description": "table not found",
	}
	result := prompt.Handler(argsWithDesc)

	if result.Description == "" {
		t.Error("Result description should not be empty")
	}

	if len(result.Messages) == 0 {
		t.Error("Result should have at least one message")
	}

	if result.Messages[0].Content.Text == "" {
		t.Error("Message text should not be empty")
	}

	// Test handler execution without argument
	argsEmpty := map[string]string{}
	resultNoArg := prompt.Handler(argsEmpty)

	if resultNoArg.Description == "" {
		t.Error("Result description should not be empty even without argument")
	}

	if len(resultNoArg.Messages) == 0 {
		t.Error("Result should have at least one message even without argument")
	}
}

func TestDesignSchemaPrompt(t *testing.T) {
	prompt := DesignSchema()

	// Verify prompt structure
	if prompt.Definition.Name != "design-schema" {
		t.Errorf("Expected name 'design-schema', got %q", prompt.Definition.Name)
	}

	if prompt.Definition.Description == "" {
		t.Error("Description should not be empty")
	}

	// Verify arguments
	if len(prompt.Definition.Arguments) != 3 {
		t.Errorf("Expected 3 arguments, got %d", len(prompt.Definition.Arguments))
	}

	// Check requirements argument
	var hasRequirements bool
	var hasUseCase bool
	var hasFullFeatured bool
	for _, arg := range prompt.Definition.Arguments {
		if arg.Name == "requirements" {
			hasRequirements = true
			if !arg.Required {
				t.Error("requirements should be required")
			}
		}
		if arg.Name == "use_case" {
			hasUseCase = true
			if arg.Required {
				t.Error("use_case should be optional")
			}
		}
		if arg.Name == "full_featured" {
			hasFullFeatured = true
			if arg.Required {
				t.Error("full_featured should be optional")
			}
		}
	}

	if !hasRequirements {
		t.Error("Missing requirements argument")
	}
	if !hasUseCase {
		t.Error("Missing use_case argument")
	}
	if !hasFullFeatured {
		t.Error("Missing full_featured argument")
	}

	// Test handler execution with argument
	argsWithReqs := map[string]string{
		"requirements": "User management system with roles and permissions",
		"use_case":     "oltp",
	}
	result := prompt.Handler(argsWithReqs)

	if result.Description == "" {
		t.Error("Result description should not be empty")
	}

	if len(result.Messages) == 0 {
		t.Error("Result should have at least one message")
	}

	if result.Messages[0].Role != "user" {
		t.Errorf("Expected first message role 'user', got %q", result.Messages[0].Role)
	}

	if result.Messages[0].Content.Text == "" {
		t.Error("Message text should not be empty")
	}

	// Test handler execution with minimal arguments
	argsMinimal := map[string]string{
		"requirements": "E-commerce product catalog",
	}
	resultMinimal := prompt.Handler(argsMinimal)

	if resultMinimal.Description == "" {
		t.Error("Result description should not be empty with minimal args")
	}

	if len(resultMinimal.Messages) == 0 {
		t.Error("Result should have at least one message with minimal args")
	}

	// Test handler execution without arguments (should use defaults)
	argsEmpty := map[string]string{}
	resultEmpty := prompt.Handler(argsEmpty)

	if len(resultEmpty.Messages) == 0 {
		t.Error("Handler should return messages even with empty args")
	}

	// Test full_featured=false (default) includes minimal mode guidance
	argsMinimalMode := map[string]string{
		"requirements": "Simple product catalog",
	}
	resultMinimalMode := prompt.Handler(argsMinimalMode)
	minimalText := resultMinimalMode.Messages[0].Content.Text
	if !strings.Contains(minimalText, "MINIMAL DESIGN MODE") {
		t.Error("Default mode should include MINIMAL DESIGN MODE guidance")
	}
	if strings.Contains(minimalText, "COMPREHENSIVE DESIGN MODE") {
		t.Error("Default mode should not include COMPREHENSIVE DESIGN MODE guidance")
	}
	// Verify strict column guidance is present
	if !strings.Contains(minimalText, "Do NOT add created_at, updated_at") {
		t.Error("Minimal mode should warn against timestamp fields")
	}
	if !strings.Contains(minimalText, "Do NOT duplicate relationship data") {
		t.Error("Minimal mode should warn against duplicate relationships")
	}

	// Test full_featured=true includes comprehensive mode guidance
	argsFullMode := map[string]string{
		"requirements":  "Complex e-commerce system",
		"full_featured": "true",
	}
	resultFullMode := prompt.Handler(argsFullMode)
	fullText := resultFullMode.Messages[0].Content.Text
	if !strings.Contains(fullText, "COMPREHENSIVE DESIGN MODE") {
		t.Error("full_featured=true should include COMPREHENSIVE DESIGN MODE guidance")
	}
	if strings.Contains(fullText, "MINIMAL DESIGN MODE") {
		t.Error("full_featured=true should not include MINIMAL DESIGN MODE guidance")
	}
}

func TestPromptArgumentVariations(t *testing.T) {
	prompt := SetupSemanticSearch()

	// Test with empty query_text
	argsEmpty := map[string]string{
		"query_text": "",
	}
	resultEmpty := prompt.Handler(argsEmpty)

	if len(resultEmpty.Messages) == 0 {
		t.Error("Handler should return messages even with empty query_text")
	}

	// Test with partial arguments
	argsPartial := map[string]string{
		"query_text": "test query",
	}
	resultPartial := prompt.Handler(argsPartial)

	if len(resultPartial.Messages) == 0 {
		t.Error("Handler should return messages with partial args")
	}

	// Test with all arguments (only query_text for this prompt)
	argsFull := map[string]string{
		"query_text": "test query",
	}
	resultFull := prompt.Handler(argsFull)

	if len(resultFull.Messages) == 0 {
		t.Error("Handler should return messages with full args")
	}

	// Verify prompt text is generated
	textFull := resultFull.Messages[0].Content.Text
	if len(textFull) == 0 {
		t.Error("Expected prompt text to be generated")
	}
}
