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
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

// SlashCommand represents a parsed slash command
type SlashCommand struct {
	Command string
	Args    []string
}

// ParseSlashCommand parses a slash command from user input
func ParseSlashCommand(input string) *SlashCommand {
	if !strings.HasPrefix(input, "/") {
		return nil
	}

	// Remove the leading slash
	input = strings.TrimPrefix(input, "/")

	// Split into command and arguments, respecting quotes
	parts := parseQuotedArgs(input)
	if len(parts) == 0 {
		return nil
	}

	return &SlashCommand{
		Command: parts[0],
		Args:    parts[1:],
	}
}

// parseQuotedArgs splits a string into arguments, respecting quoted strings
func parseQuotedArgs(input string) []string {
	var args []string
	var current strings.Builder
	inQuote := false
	quoteChar := rune(0)

	// Convert to runes for proper Unicode handling
	runes := []rune(input)

	for i := 0; i < len(runes); i++ {
		r := runes[i]

		switch {
		case (r == '"' || r == '\'') && !inQuote:
			// Start of quoted string
			inQuote = true
			quoteChar = r
		case r == quoteChar && inQuote:
			// End of quoted string
			inQuote = false
			quoteChar = 0
		case r == ' ' && !inQuote:
			// Space outside quotes - end of argument
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		case r == '\\' && inQuote && i+1 < len(runes):
			// Escape sequence in quoted string
			next := runes[i+1]
			if next == quoteChar || next == '\\' {
				// Skip the backslash, include the escaped character
				current.WriteRune(next)
				i++ // Skip the next character since we've already processed it
			} else {
				// Not a valid escape sequence, include the backslash
				current.WriteRune(r)
			}
		default:
			// Regular character
			current.WriteRune(r)
		}
	}

	// Add the last argument if any
	if current.Len() > 0 {
		args = append(args, current.String())
	}

	return args
}

// HandleSlashCommand processes slash commands, returns true if handled
func (c *Client) HandleSlashCommand(ctx context.Context, cmd *SlashCommand) bool {
	if cmd == nil {
		return false
	}

	switch cmd.Command {
	case "help":
		c.printSlashHelp()
		return true

	case "clear":
		c.ui.ClearScreen()
		var serverVersion string
		if c.mcp != nil {
			_, serverVersion = c.mcp.GetServerInfo()
		}
		c.ui.PrintWelcome(ClientVersion, serverVersion)
		return true

	case "tools":
		c.ui.PrintSystemMessage(fmt.Sprintf("Available tools (%d):", len(c.tools)))
		// Sort tools alphabetically by name
		sortedTools := make([]struct{ Name, Desc string }, len(c.tools))
		for i, tool := range c.tools {
			sortedTools[i] = struct{ Name, Desc string }{tool.Name, getBriefDescription(tool.Description)}
		}
		sort.Slice(sortedTools, func(i, j int) bool {
			return sortedTools[i].Name < sortedTools[j].Name
		})
		for _, tool := range sortedTools {
			fmt.Printf("  - %s: %s\n", tool.Name, tool.Desc)
		}
		return true

	case "resources":
		c.ui.PrintSystemMessage(fmt.Sprintf("Available resources (%d):", len(c.resources)))
		// Sort resources alphabetically by name
		sortedResources := make([]struct{ Name, Desc string }, len(c.resources))
		for i, resource := range c.resources {
			sortedResources[i] = struct{ Name, Desc string }{resource.Name, resource.Description}
		}
		sort.Slice(sortedResources, func(i, j int) bool {
			return sortedResources[i].Name < sortedResources[j].Name
		})
		for _, resource := range sortedResources {
			fmt.Printf("  - %s: %s\n", resource.Name, resource.Desc)
		}
		return true

	case "prompts":
		c.ui.PrintSystemMessage(fmt.Sprintf("Available prompts (%d):", len(c.prompts)))
		// Sort prompts alphabetically by name
		sortedPrompts := make([]struct{ Name, Desc string }, len(c.prompts))
		for i, prompt := range c.prompts {
			sortedPrompts[i] = struct{ Name, Desc string }{prompt.Name, prompt.Description}
		}
		sort.Slice(sortedPrompts, func(i, j int) bool {
			return sortedPrompts[i].Name < sortedPrompts[j].Name
		})
		for _, prompt := range sortedPrompts {
			fmt.Printf("  - %s: %s\n", prompt.Name, prompt.Desc)
		}
		return true

	case "quit", "exit":
		c.ui.PrintSystemMessage("Goodbye!")
		os.Exit(0)
		return true

	case "set":
		return c.handleSetCommand(ctx, cmd.Args)

	case "show":
		return c.handleShowCommand(ctx, cmd.Args)

	case "list":
		return c.handleListCommand(ctx, cmd.Args)

	case "prompt":
		return c.handlePromptCommand(ctx, cmd.Args)

	case "history":
		return c.handleHistoryCommand(ctx, cmd.Args)

	case "new":
		return c.handleNewConversation(ctx)

	case "save":
		return c.handleSaveConversation(ctx)

	default:
		// Unknown slash command, let it be sent to LLM
		return false
	}
}

// printSlashHelp prints help for slash commands
func (c *Client) printSlashHelp() {
	help := `
Commands:
  /help                                Show this help message
  /clear                               Clear screen
  /tools                               List available MCP tools
  /resources                           List available MCP resources
  /prompts                             List available MCP prompts
  /quit, /exit                         Exit the chat client

Settings:
  /set status-messages <on|off>        Enable or disable status messages
  /set markdown <on|off>               Enable or disable markdown rendering
  /set debug <on|off>                  Enable or disable debug messages
  /set llm-provider <provider>         Set LLM provider (anthropic, openai, ollama)
  /set llm-model <model>               Set LLM model to use
  /set database <name>                 Select a database connection
  /show status-messages                Show current status messages setting
  /show markdown                       Show current markdown rendering setting
  /show debug                          Show current debug setting
  /show llm-provider                   Show current LLM provider
  /show llm-model                      Show current LLM model
  /show database                       Show current database connection
  /show settings                       Show all current settings
  /list models                         List available models from current LLM provider
  /list databases                      List available database connections

Prompts:
  /prompt <name> [arg=value ...]       Execute an MCP prompt with optional arguments
`

	// Add history commands only if running with authentication
	if c.conversations != nil {
		help += `
Conversation History (requires authentication):
  /new                                 Start a new conversation
  /save                                Save the current conversation
  /history                             List saved conversations
  /history load <id>                   Load a saved conversation
  /history rename <id> "new title"     Rename a saved conversation
  /history delete <id>                 Delete a saved conversation
  /history delete-all                  Delete all saved conversations
`
	}

	help += `
Examples:
  /set llm-provider openai
  /set llm-model gpt-4-turbo
  /set database mydb
  /list models
  /list databases
  /prompt explore-database
  /prompt setup-semantic-search query_text="product search"

Anything else you type will be sent to the LLM.
`
	fmt.Print(help)
}

// handleSetCommand handles /set commands
func (c *Client) handleSetCommand(ctx context.Context, args []string) bool {
	if len(args) < 2 {
		c.ui.PrintError("Usage: /set <setting> <value>")
		c.ui.PrintSystemMessage("Available settings: status-messages, markdown, debug, llm-provider, llm-model, database")
		return true
	}

	setting := args[0]
	value := args[1]

	switch setting {
	case "status-messages":
		return c.handleSetStatusMessages(value)

	case "markdown":
		return c.handleSetMarkdown(value)

	case "debug":
		return c.handleSetDebug(value)

	case "llm-provider":
		return c.handleSetLLMProvider(value)

	case "llm-model":
		return c.handleSetLLMModel(value)

	case "database":
		return c.handleSetDatabase(ctx, value)

	default:
		c.ui.PrintError(fmt.Sprintf("Unknown setting: %s", setting))
		c.ui.PrintSystemMessage("Available settings: status-messages, markdown, debug, llm-provider, llm-model, database")
		return true
	}
}

// handleSetStatusMessages handles setting status messages on/off
func (c *Client) handleSetStatusMessages(value string) bool {
	value = strings.ToLower(value)

	switch value {
	case "on", "true", "1", "yes":
		c.config.UI.DisplayStatusMessages = true
		c.ui.DisplayStatusMessages = true
		c.preferences.UI.DisplayStatusMessages = true
		c.ui.PrintSystemMessage("Status messages enabled")

	case "off", "false", "0", "no":
		c.config.UI.DisplayStatusMessages = false
		c.ui.DisplayStatusMessages = false
		c.preferences.UI.DisplayStatusMessages = false
		c.ui.PrintSystemMessage("Status messages disabled")

	default:
		c.ui.PrintError(fmt.Sprintf("Invalid value for status-messages: %s (use on or off)", value))
		return true
	}

	// Save preferences
	if err := SavePreferences(c.preferences); err != nil {
		c.ui.PrintError(fmt.Sprintf("Warning: Failed to save preferences: %v", err))
	}

	return true
}

// handleSetMarkdown handles setting markdown rendering on/off
func (c *Client) handleSetMarkdown(value string) bool {
	value = strings.ToLower(value)

	switch value {
	case "on", "true", "1", "yes":
		c.config.UI.RenderMarkdown = true
		c.ui.RenderMarkdown = true
		c.preferences.UI.RenderMarkdown = true
		c.ui.PrintSystemMessage("Markdown rendering enabled")

	case "off", "false", "0", "no":
		c.config.UI.RenderMarkdown = false
		c.ui.RenderMarkdown = false
		c.preferences.UI.RenderMarkdown = false
		c.ui.PrintSystemMessage("Markdown rendering disabled")

	default:
		c.ui.PrintError(fmt.Sprintf("Invalid value for markdown: %s (use on or off)", value))
		return true
	}

	// Save preferences
	if err := SavePreferences(c.preferences); err != nil {
		c.ui.PrintError(fmt.Sprintf("Warning: Failed to save preferences: %v", err))
	}

	return true
}

// handleSetDebug handles setting debug mode on/off
func (c *Client) handleSetDebug(value string) bool {
	value = strings.ToLower(value)

	switch value {
	case "on", "true", "1", "yes":
		c.config.UI.Debug = true
		c.preferences.UI.Debug = true
		c.ui.PrintSystemMessage("Debug messages enabled")

	case "off", "false", "0", "no":
		c.config.UI.Debug = false
		c.preferences.UI.Debug = false
		c.ui.PrintSystemMessage("Debug messages disabled")

	default:
		c.ui.PrintError(fmt.Sprintf("Invalid value for debug: %s (use on or off)", value))
		return true
	}

	// Reinitialize LLM client with new debug setting
	if err := c.initializeLLM(); err != nil {
		c.ui.PrintError(fmt.Sprintf("Failed to reinitialize LLM: %v", err))
		return true
	}

	// Save preferences
	if err := SavePreferences(c.preferences); err != nil {
		c.ui.PrintError(fmt.Sprintf("Warning: Failed to save preferences: %v", err))
	}

	return true
}

// handleSetLLMProvider handles setting the LLM provider
func (c *Client) handleSetLLMProvider(provider string) bool {
	provider = strings.ToLower(provider)

	// Validate provider name
	validProviders := map[string]bool{
		"anthropic": true,
		"openai":    true,
		"ollama":    true,
	}

	if !validProviders[provider] {
		c.ui.PrintError(fmt.Sprintf("Invalid LLM provider: %s", provider))
		c.ui.PrintSystemMessage("Valid providers: anthropic, openai, ollama")
		return true
	}

	// Check if provider is configured
	if !c.config.IsProviderConfigured(provider) {
		c.ui.PrintError(fmt.Sprintf("Provider %s is not configured (missing API key or URL)", provider))
		return true
	}

	// Save current model for current provider before switching
	if c.config.LLM.Provider != "" && c.config.LLM.Model != "" {
		c.preferences.SetModelForProvider(c.config.LLM.Provider, c.config.LLM.Model)
	}

	// Update config to new provider
	c.config.LLM.Provider = provider

	// Clear model to trigger auto-selection in initializeLLM()
	c.config.LLM.Model = ""

	// Update preferences
	c.preferences.LastProvider = provider

	// Reinitialize LLM client (will auto-select model)
	if err := c.initializeLLM(); err != nil {
		c.ui.PrintError(fmt.Sprintf("Failed to initialize LLM: %v", err))
		return true
	}

	// Save preferences (model was already saved in initializeLLM)
	if err := SavePreferences(c.preferences); err != nil {
		c.ui.PrintError(fmt.Sprintf("Warning: Failed to save preferences: %v", err))
	}

	c.ui.PrintSystemMessage(fmt.Sprintf("LLM provider set to: %s (model: %s)", provider, c.config.LLM.Model))
	return true
}

// handleSetLLMModel handles setting the LLM model
func (c *Client) handleSetLLMModel(model string) bool {
	// Get available models to validate
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	availableModels, err := c.llm.ListModels(ctx)
	if err != nil {
		// If we can't validate, warn but allow the change
		c.ui.PrintSystemMessage(fmt.Sprintf(
			"Warning: Could not validate model (error: %v)", err))
	} else if !isModelAvailable(model, availableModels) {
		c.ui.PrintError(fmt.Sprintf(
			"Model '%s' not available from %s", model, c.config.LLM.Provider))
		c.ui.PrintSystemMessage("Use /list models to see available models")
		return true
	}

	// Update config
	c.config.LLM.Model = model

	// Save model preference for current provider
	c.preferences.SetModelForProvider(c.config.LLM.Provider, model)

	// Reinitialize LLM client with the new model
	if err := c.initializeLLM(); err != nil {
		c.ui.PrintError(fmt.Sprintf("Failed to initialize LLM: %v", err))
		return true
	}

	// Save preferences
	if err := SavePreferences(c.preferences); err != nil {
		c.ui.PrintError(fmt.Sprintf("Warning: Failed to save preferences: %v", err))
	}

	c.ui.PrintSystemMessage(fmt.Sprintf("LLM model set to: %s (provider: %s)", model, c.config.LLM.Provider))
	return true
}

// handleShowCommand handles /show commands
func (c *Client) handleShowCommand(ctx context.Context, args []string) bool {
	if len(args) < 1 {
		c.ui.PrintError("Usage: /show <setting>")
		c.ui.PrintSystemMessage("Available settings: status-messages, markdown, debug, llm-provider, llm-model, database, settings")
		return true
	}

	setting := args[0]

	switch setting {
	case "status-messages":
		status := "off"
		if c.config.UI.DisplayStatusMessages {
			status = "on"
		}
		c.ui.PrintSystemMessage(fmt.Sprintf("Status messages: %s", status))

	case "markdown":
		status := "off"
		if c.config.UI.RenderMarkdown {
			status = "on"
		}
		c.ui.PrintSystemMessage(fmt.Sprintf("Markdown rendering: %s", status))

	case "debug":
		status := "off"
		if c.config.UI.Debug {
			status = "on"
		}
		c.ui.PrintSystemMessage(fmt.Sprintf("Debug messages: %s", status))

	case "llm-provider":
		c.ui.PrintSystemMessage(fmt.Sprintf("LLM provider: %s", c.config.LLM.Provider))

	case "llm-model":
		c.ui.PrintSystemMessage(fmt.Sprintf("LLM model: %s", c.config.LLM.Model))

	case "database":
		return c.handleShowDatabase(ctx)

	case "settings":
		c.printAllSettings()

	default:
		c.ui.PrintError(fmt.Sprintf("Unknown setting: %s", setting))
		c.ui.PrintSystemMessage("Available settings: status-messages, markdown, debug, llm-provider, llm-model, database, settings")
	}

	return true
}

// printAllSettings prints all current settings
func (c *Client) printAllSettings() {
	fmt.Println("\nCurrent Settings:")
	fmt.Println("─────────────────────────────────────────────────")

	// UI Settings
	fmt.Println("\nUI:")
	statusMsg := "off"
	if c.config.UI.DisplayStatusMessages {
		statusMsg = "on"
	}
	fmt.Printf("  Status Messages:  %s\n", statusMsg)
	markdown := "off"
	if c.config.UI.RenderMarkdown {
		markdown = "on"
	}
	fmt.Printf("  Render Markdown:  %s\n", markdown)
	debug := "off"
	if c.config.UI.Debug {
		debug = "on"
	}
	fmt.Printf("  Debug Messages:   %s\n", debug)
	noColor := "no"
	if c.config.UI.NoColor {
		noColor = "yes"
	}
	fmt.Printf("  No Color:         %s\n", noColor)

	// LLM Settings
	fmt.Println("\nLLM:")
	fmt.Printf("  Provider:         %s\n", c.config.LLM.Provider)
	fmt.Printf("  Model:            %s\n", c.config.LLM.Model)
	fmt.Printf("  Max Tokens:       %d\n", c.config.LLM.MaxTokens)
	fmt.Printf("  Temperature:      %.2f\n", c.config.LLM.Temperature)

	// MCP Settings
	fmt.Println("\nMCP:")
	fmt.Printf("  Mode:             %s\n", c.config.MCP.Mode)
	if c.config.MCP.Mode == "http" {
		fmt.Printf("  URL:              %s\n", c.config.MCP.URL)
		fmt.Printf("  Auth Mode:        %s\n", c.config.MCP.AuthMode)
	} else {
		fmt.Printf("  Server Path:      %s\n", c.config.MCP.ServerPath)
	}

	fmt.Println("─────────────────────────────────────────────────")
}

// handleListCommand handles /list commands
func (c *Client) handleListCommand(ctx context.Context, args []string) bool {
	if len(args) < 1 {
		c.ui.PrintError("Usage: /list <what>")
		c.ui.PrintSystemMessage("Available: models, databases")
		return true
	}

	what := args[0]

	switch what {
	case "models":
		return c.listModels(ctx)

	case "databases":
		return c.handleListDatabases(ctx)

	default:
		c.ui.PrintError(fmt.Sprintf("Unknown list target: %s", what))
		c.ui.PrintSystemMessage("Available: models, databases")
	}

	return true
}

// listModels lists available models from the current LLM provider
func (c *Client) listModels(ctx context.Context) bool {
	models, err := c.llm.ListModels(ctx)
	if err != nil {
		c.ui.PrintError(fmt.Sprintf("Failed to list models: %v", err))
		return true
	}

	if len(models) == 0 {
		c.ui.PrintSystemMessage("No models available")
		return true
	}

	c.ui.PrintSystemMessage(fmt.Sprintf("Available models from %s (%d):", c.config.LLM.Provider, len(models)))
	for _, model := range models {
		if model == c.config.LLM.Model {
			fmt.Printf("  * %s (current)\n", model)
		} else {
			fmt.Printf("    %s\n", model)
		}
	}

	return true
}

// handlePromptCommand handles /prompt commands
func (c *Client) handlePromptCommand(ctx context.Context, args []string) bool {
	if len(args) < 1 {
		c.ui.PrintError("Usage: /prompt <name> [arg=value ...]")
		c.ui.PrintSystemMessage("Use 'prompts' command to list available prompts")
		return true
	}

	promptName := args[0]

	// Parse arguments in key=value format
	promptArgs := make(map[string]string)
	for _, arg := range args[1:] {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			// Quotes are already removed by parseQuotedArgs
			promptArgs[key] = value
		} else {
			c.ui.PrintError(fmt.Sprintf("Invalid argument format: %s (expected key=value)", arg))
			return true
		}
	}

	// Execute the prompt
	c.ui.PrintSystemMessage(fmt.Sprintf("Executing prompt: %s", promptName))

	result, err := c.mcp.GetPrompt(ctx, promptName, promptArgs)
	if err != nil {
		c.ui.PrintError(fmt.Sprintf("Failed to execute prompt: %v", err))
		return true
	}

	// Display the prompt description if available
	if result.Description != "" {
		c.ui.PrintSystemMessage(result.Description)
	}

	// Add prompt messages to conversation history
	// The prompt result contains messages that guide the LLM through a workflow
	for _, msg := range result.Messages {
		if msg.Role == "user" {
			// Add user message from prompt
			c.messages = append(c.messages, Message{
				Role:    "user",
				Content: msg.Content.Text,
			})
		} else if msg.Role == "assistant" {
			// Add assistant message from prompt (less common but supported)
			c.messages = append(c.messages, Message{
				Role:    "assistant",
				Content: msg.Content.Text,
			})
		}
	}

	c.ui.PrintSystemMessage("Prompt loaded. Starting workflow execution...")
	c.ui.PrintSeparator()

	// Automatically process the prompt through the LLM
	// This triggers the agentic loop with the loaded prompt instructions
	if err := c.processQuery(ctx, ""); err != nil {
		c.ui.PrintError(err.Error())
	}

	return true
}

// DatabaseInfo represents a database connection in API responses
type DatabaseInfo struct {
	Name     string `json:"name"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Database string `json:"database"`
	User     string `json:"user"`
	SSLMode  string `json:"sslmode"`
}

// ListDatabasesResponse is the response from GET /api/databases
type ListDatabasesResponse struct {
	Databases []DatabaseInfo `json:"databases"`
	Current   string         `json:"current"`
}

// SelectDatabaseRequest is the request body for POST /api/databases/select
type SelectDatabaseRequest struct {
	Name string `json:"name"`
}

// SelectDatabaseResponse is the response from POST /api/databases/select
type SelectDatabaseResponse struct {
	Success bool   `json:"success"`
	Current string `json:"current,omitempty"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// getServerKey returns a unique identifier for the current server connection
// Used for storing per-server preferences like selected database
func (c *Client) getServerKey() string {
	if c.config.MCP.Mode == "http" {
		// For HTTP mode, hash the server URL
		hash := sha256.Sum256([]byte(c.config.MCP.URL))
		return hex.EncodeToString(hash[:8]) // First 8 bytes = 16 hex chars
	}
	// For STDIO mode, use "local" or hash of binary path
	if c.config.MCP.ServerPath != "" {
		hash := sha256.Sum256([]byte(c.config.MCP.ServerPath))
		return "local-" + hex.EncodeToString(hash[:4])
	}
	return "local"
}

// handleListDatabases handles /list databases command - lists available databases
func (c *Client) handleListDatabases(ctx context.Context) bool {
	// Use the MCPClient interface method (works for both HTTP and STDIO modes)
	databases, current, err := c.mcp.ListDatabases(ctx)
	if err != nil {
		c.ui.PrintError(fmt.Sprintf("Failed to list databases: %v", err))
		return true
	}

	if len(databases) == 0 {
		c.ui.PrintSystemMessage("No databases available")
		return true
	}

	c.ui.PrintSystemMessage(fmt.Sprintf("Available databases (%d):", len(databases)))
	for _, db := range databases {
		currentMarker := ""
		if db.Name == current {
			currentMarker = " (current)"
		}
		fmt.Printf("  %s%s - %s@%s:%d/%s\n",
			db.Name, currentMarker, db.User, db.Host, db.Port, db.Database)
	}

	return true
}

// handleShowDatabase handles /show database command - shows current database
func (c *Client) handleShowDatabase(ctx context.Context) bool {
	// Use the MCPClient interface method (works for both HTTP and STDIO modes)
	_, current, err := c.mcp.ListDatabases(ctx)
	if err != nil {
		c.ui.PrintError(fmt.Sprintf("Failed to get current database: %v", err))
		return true
	}

	if current == "" {
		c.ui.PrintSystemMessage("No database currently selected")
	} else {
		c.ui.PrintSystemMessage(fmt.Sprintf("Current database: %s", current))
	}

	return true
}

// handleSetDatabase handles /set database <name> command - selects a database
func (c *Client) handleSetDatabase(ctx context.Context, dbName string) bool {
	// Use the MCPClient interface method (works for both HTTP and STDIO modes)
	if err := c.mcp.SelectDatabase(ctx, dbName); err != nil {
		c.ui.PrintError(fmt.Sprintf("Failed to select database: %v", err))
		return true
	}

	// Save the preference for this server
	serverKey := c.getServerKey()
	c.preferences.SetDatabaseForServer(serverKey, dbName)
	if err := SavePreferences(c.preferences); err != nil {
		c.ui.PrintError(fmt.Sprintf("Warning: Failed to save preference: %v", err))
	}

	c.ui.PrintSystemMessage(fmt.Sprintf("Database switched to: %s", dbName))

	// Refresh tools since they may be database-specific
	if err := c.refreshCapabilities(ctx); err != nil {
		c.ui.PrintError(fmt.Sprintf("Warning: Failed to refresh capabilities: %v", err))
	}

	return true
}

// refreshCapabilities refreshes tools, resources, and prompts from the server
func (c *Client) refreshCapabilities(ctx context.Context) error {
	tools, err := c.mcp.ListTools(ctx)
	if err != nil {
		return fmt.Errorf("failed to list tools: %w", err)
	}
	c.tools = tools

	resources, err := c.mcp.ListResources(ctx)
	if err != nil {
		return fmt.Errorf("failed to list resources: %w", err)
	}
	c.resources = resources

	prompts, err := c.mcp.ListPrompts(ctx)
	if err != nil {
		return fmt.Errorf("failed to list prompts: %w", err)
	}
	c.prompts = prompts

	return nil
}

// handleHistoryCommand handles /history commands for conversation management
func (c *Client) handleHistoryCommand(ctx context.Context, args []string) bool {
	// Check if conversations are available (HTTP mode with authentication)
	if c.conversations == nil {
		c.ui.PrintError("Conversation history is only available when running with authentication (HTTP mode)")
		return true
	}

	// No args - list conversations
	if len(args) == 0 {
		return c.listConversations(ctx)
	}

	subcommand := args[0]

	switch subcommand {
	case "list":
		return c.listConversations(ctx)

	case "load":
		if len(args) < 2 {
			c.ui.PrintError("Usage: /history load <conversation-id>")
			return true
		}
		return c.loadConversation(ctx, args[1])

	case "rename":
		if len(args) < 3 {
			c.ui.PrintError("Usage: /history rename <conversation-id> \"new title\"")
			return true
		}
		// Join remaining args as the title (in case it wasn't quoted)
		title := strings.Join(args[2:], " ")
		return c.renameConversation(ctx, args[1], title)

	case "delete":
		if len(args) < 2 {
			c.ui.PrintError("Usage: /history delete <conversation-id>")
			return true
		}
		return c.deleteConversation(ctx, args[1])

	case "delete-all":
		return c.deleteAllConversations(ctx)

	default:
		c.ui.PrintError(fmt.Sprintf("Unknown history subcommand: %s", subcommand))
		c.ui.PrintSystemMessage("Available: list, load, rename, delete, delete-all")
		return true
	}
}

// listConversations lists all saved conversations
func (c *Client) listConversations(ctx context.Context) bool {
	conversations, err := c.conversations.List(ctx)
	if err != nil {
		c.ui.PrintError(fmt.Sprintf("Failed to list conversations: %v", err))
		return true
	}

	if len(conversations) == 0 {
		c.ui.PrintSystemMessage("No saved conversations")
		return true
	}

	c.ui.PrintSystemMessage(fmt.Sprintf("Saved conversations (%d):", len(conversations)))
	fmt.Println()

	for _, conv := range conversations {
		// Format the date
		dateStr := conv.UpdatedAt.Local().Format("Jan 02, 15:04")

		// Mark current conversation
		current := ""
		if conv.ID == c.currentConversationID {
			current = " (current)"
		}

		// Show connection if available
		connection := ""
		if conv.Connection != "" {
			connection = fmt.Sprintf(" [%s]", conv.Connection)
		}

		fmt.Printf("  %s%s%s\n", conv.ID, current, connection)
		fmt.Printf("    Title: %s\n", conv.Title)
		fmt.Printf("    Updated: %s\n", dateStr)
		if conv.Preview != "" {
			preview := conv.Preview
			if len(preview) > 60 {
				preview = preview[:57] + "..."
			}
			fmt.Printf("    Preview: %s\n", preview)
		}
		fmt.Println()
	}

	return true
}

// loadConversation loads a saved conversation
func (c *Client) loadConversation(ctx context.Context, id string) bool {
	conv, err := c.conversations.Get(ctx, id)
	if err != nil {
		c.ui.PrintError(fmt.Sprintf("Failed to load conversation: %v", err))
		return true
	}

	// Convert stored messages to client messages
	c.messages = make([]Message, 0, len(conv.Messages))
	for _, msg := range conv.Messages {
		c.messages = append(c.messages, Message{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	// Update current conversation ID
	c.currentConversationID = conv.ID

	// Restore provider and model if they were saved
	if conv.Provider != "" && c.config.IsProviderConfigured(conv.Provider) {
		if conv.Provider != c.config.LLM.Provider {
			c.config.LLM.Provider = conv.Provider
			c.config.LLM.Model = conv.Model
			if err := c.initializeLLM(); err != nil {
				c.ui.PrintError(fmt.Sprintf("Warning: Failed to restore LLM provider: %v", err))
			}
		} else if conv.Model != "" && conv.Model != c.config.LLM.Model {
			c.config.LLM.Model = conv.Model
			if err := c.initializeLLM(); err != nil {
				c.ui.PrintError(fmt.Sprintf("Warning: Failed to restore LLM model: %v", err))
			}
		}
	}

	// Restore database connection if different
	if conv.Connection != "" {
		if _, current, err := c.mcp.ListDatabases(ctx); err == nil {
			if current != conv.Connection {
				if err := c.mcp.SelectDatabase(ctx, conv.Connection); err != nil {
					c.ui.PrintError(fmt.Sprintf("Warning: Failed to restore database connection: %v", err))
				} else {
					// Refresh capabilities after database change
					if err := c.refreshCapabilities(ctx); err != nil {
						c.ui.PrintError(fmt.Sprintf("Warning: Failed to refresh capabilities: %v", err))
					}
				}
			}
		}
	}

	c.ui.PrintSystemMessage(fmt.Sprintf("Loaded conversation: %s", conv.Title))
	c.ui.PrintSystemMessage(fmt.Sprintf("Messages: %d, Provider: %s, Model: %s",
		len(c.messages), c.config.LLM.Provider, c.config.LLM.Model))

	// Show current database connection
	if _, current, err := c.mcp.ListDatabases(ctx); err == nil && current != "" {
		c.ui.PrintSystemMessage(fmt.Sprintf("Database: %s", current))
	}

	// Replay the conversation history with muted colors
	if len(c.messages) > 0 {
		fmt.Println()
		c.ui.PrintHistorySeparator("Conversation History")
		fmt.Println()

		for _, msg := range c.messages {
			// Extract text content from the message
			var text string
			switch content := msg.Content.(type) {
			case string:
				text = content
			default:
				// Skip non-text messages (tool calls, tool results, etc.)
				continue
			}

			if text == "" {
				continue
			}

			switch msg.Role {
			case "user":
				c.ui.PrintHistoricUserMessage(text)
			case "assistant":
				c.ui.PrintHistoricAssistantMessage(text)
			}
		}

		fmt.Println()
		c.ui.PrintHistorySeparator("End of History")
		fmt.Println()
	}

	return true
}

// renameConversation renames a saved conversation
func (c *Client) renameConversation(ctx context.Context, id, title string) bool {
	if err := c.conversations.Rename(ctx, id, title); err != nil {
		c.ui.PrintError(fmt.Sprintf("Failed to rename conversation: %v", err))
		return true
	}

	c.ui.PrintSystemMessage(fmt.Sprintf("Conversation renamed to: %s", title))
	return true
}

// deleteConversation deletes a saved conversation
func (c *Client) deleteConversation(ctx context.Context, id string) bool {
	if err := c.conversations.Delete(ctx, id); err != nil {
		c.ui.PrintError(fmt.Sprintf("Failed to delete conversation: %v", err))
		return true
	}

	// Clear current conversation ID if we deleted the current one
	if id == c.currentConversationID {
		c.currentConversationID = ""
	}

	c.ui.PrintSystemMessage("Conversation deleted")
	return true
}

// deleteAllConversations deletes all saved conversations
func (c *Client) deleteAllConversations(ctx context.Context) bool {
	count, err := c.conversations.DeleteAll(ctx)
	if err != nil {
		c.ui.PrintError(fmt.Sprintf("Failed to delete conversations: %v", err))
		return true
	}

	c.currentConversationID = ""
	c.ui.PrintSystemMessage(fmt.Sprintf("Deleted %d conversation(s)", count))
	return true
}

// handleNewConversation starts a new conversation
func (c *Client) handleNewConversation(ctx context.Context) bool {
	// Check if conversations are available (HTTP mode with authentication)
	if c.conversations == nil {
		c.ui.PrintError("Conversation history is only available when running with authentication (HTTP mode)")
		return true
	}

	// Clear current conversation
	c.messages = []Message{}
	c.currentConversationID = ""

	c.ui.PrintSystemMessage("Started new conversation")
	return true
}

// handleSaveConversation saves the current conversation
func (c *Client) handleSaveConversation(ctx context.Context) bool {
	// Check if conversations are available (HTTP mode with authentication)
	if c.conversations == nil {
		c.ui.PrintError("Conversation history is only available when running with authentication (HTTP mode)")
		return true
	}

	if len(c.messages) == 0 {
		c.ui.PrintError("No messages to save")
		return true
	}

	// Get current database connection
	connection := ""
	if _, current, err := c.mcp.ListDatabases(ctx); err == nil {
		connection = current
	}

	var conv *Conversation
	var err error

	if c.currentConversationID != "" {
		// Update existing conversation
		conv, err = c.conversations.Update(ctx, c.currentConversationID,
			c.config.LLM.Provider, c.config.LLM.Model, connection, c.messages)
	} else {
		// Create new conversation
		conv, err = c.conversations.Create(ctx,
			c.config.LLM.Provider, c.config.LLM.Model, connection, c.messages)
	}

	if err != nil {
		c.ui.PrintError(fmt.Sprintf("Failed to save conversation: %v", err))
		return true
	}

	c.currentConversationID = conv.ID
	c.ui.PrintSystemMessage(fmt.Sprintf("Conversation saved: %s (ID: %s)", conv.Title, conv.ID))
	return true
}
