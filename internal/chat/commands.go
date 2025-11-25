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
	"fmt"
	"strings"
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

	case "set":
		return c.handleSetCommand(cmd.Args)

	case "show":
		return c.handleShowCommand(cmd.Args)

	case "list":
		return c.handleListCommand(ctx, cmd.Args)

	case "prompt":
		return c.handlePromptCommand(ctx, cmd.Args)

	default:
		// Unknown slash command, let it be sent to LLM
		return false
	}
}

// printSlashHelp prints help for slash commands
func (c *Client) printSlashHelp() {
	help := `
Slash Commands:
  /help                                Show this help message
  /set status-messages <on|off>        Enable or disable status messages
  /set markdown <on|off>               Enable or disable markdown rendering
  /set debug <on|off>                  Enable or disable debug messages
  /set llm-provider <provider>         Set LLM provider (anthropic, openai, ollama)
  /set llm-model <model>               Set LLM model to use
  /show status-messages                Show current status messages setting
  /show markdown                       Show current markdown rendering setting
  /show debug                          Show current debug setting
  /show llm-provider                   Show current LLM provider
  /show llm-model                      Show current LLM model
  /show settings                       Show all current settings
  /list models                         List available models from current LLM provider
  /prompt <name> [arg=value ...]       Execute an MCP prompt with optional arguments

Other Commands:
  help                                 Show general help
  clear                                Clear screen
  tools                                List available MCP tools
  resources                            List available MCP resources
  prompts                              List available MCP prompts
  quit, exit                           Exit the chat client

Examples:
  /set status-messages off
  /set markdown on
  /set debug on
  /set llm-provider openai
  /set llm-model gpt-4-turbo
  /list models
  /show settings
  /prompt explore-database
  /prompt setup-semantic-search query_text="product search"
`
	fmt.Print(help)
}

// handleSetCommand handles /set commands
func (c *Client) handleSetCommand(args []string) bool {
	if len(args) < 2 {
		c.ui.PrintError("Usage: /set <setting> <value>")
		c.ui.PrintSystemMessage("Available settings: status-messages, markdown, debug, llm-provider, llm-model")
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

	default:
		c.ui.PrintError(fmt.Sprintf("Unknown setting: %s", setting))
		c.ui.PrintSystemMessage("Available settings: status-messages, markdown, debug, llm-provider, llm-model")
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

	// Validate provider
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

	// Update config
	c.config.LLM.Provider = provider

	// Auto-switch to preferred model for this provider
	if preferredModel := c.preferences.GetModelForProvider(provider); preferredModel != "" {
		c.config.LLM.Model = preferredModel
	}

	// Update preferences
	c.preferences.LastProvider = provider

	// Reinitialize LLM client
	if err := c.initializeLLM(); err != nil {
		c.ui.PrintError(fmt.Sprintf("Failed to initialize LLM: %v", err))
		return true
	}

	// Save preferences
	if err := SavePreferences(c.preferences); err != nil {
		c.ui.PrintError(fmt.Sprintf("Warning: Failed to save preferences: %v", err))
	}

	c.ui.PrintSystemMessage(fmt.Sprintf("LLM provider set to: %s (model: %s)", provider, c.config.LLM.Model))
	return true
}

// handleSetLLMModel handles setting the LLM model
func (c *Client) handleSetLLMModel(model string) bool {
	// Update config
	c.config.LLM.Model = model

	// Save model preference for current provider
	c.preferences.SetModelForProvider(c.config.LLM.Provider, model)

	// Reinitialize LLM client
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
func (c *Client) handleShowCommand(args []string) bool {
	if len(args) < 1 {
		c.ui.PrintError("Usage: /show <setting>")
		c.ui.PrintSystemMessage("Available settings: status-messages, markdown, debug, llm-provider, llm-model, settings")
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

	case "settings":
		c.printAllSettings()

	default:
		c.ui.PrintError(fmt.Sprintf("Unknown setting: %s", setting))
		c.ui.PrintSystemMessage("Available settings: status-messages, markdown, debug, llm-provider, llm-model, settings")
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
		c.ui.PrintSystemMessage("Available: models")
		return true
	}

	what := args[0]

	switch what {
	case "models":
		return c.listModels(ctx)

	default:
		c.ui.PrintError(fmt.Sprintf("Unknown list target: %s", what))
		c.ui.PrintSystemMessage("Available: models")
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
