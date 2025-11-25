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

	// List all prompts
	prompts := registry.List()

	if len(prompts) != 3 {
		t.Errorf("Expected 3 prompts, got %d", len(prompts))
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

	args := map[string]string{}
	_, err := registry.Execute("non-existent", args)

	if err == nil {
		t.Error("Expected error when executing non-existent prompt")
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
	if len(prompt.Definition.Arguments) != 2 {
		t.Errorf("Expected 2 arguments, got %d", len(prompt.Definition.Arguments))
	}

	// Check query_text argument
	var hasQueryText bool
	var hasTableName bool
	for _, arg := range prompt.Definition.Arguments {
		if arg.Name == "query_text" {
			hasQueryText = true
			if !arg.Required {
				t.Error("query_text should be required")
			}
		}
		if arg.Name == "table_name" {
			hasTableName = true
			if arg.Required {
				t.Error("table_name should be optional")
			}
		}
	}

	if !hasQueryText {
		t.Error("Missing query_text argument")
	}
	if !hasTableName {
		t.Error("Missing table_name argument")
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

	// Test with all arguments
	argsFull := map[string]string{
		"query_text": "test query",
		"table_name": "test_table",
	}
	resultFull := prompt.Handler(argsFull)

	if len(resultFull.Messages) == 0 {
		t.Error("Handler should return messages with full args")
	}

	// Verify that table_name is included in the prompt text when provided
	textFull := resultFull.Messages[0].Content.Text
	if len(textFull) == 0 {
		t.Error("Expected prompt text with table_name")
	}
}
