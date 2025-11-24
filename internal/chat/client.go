/*-------------------------------------------------------------------------
 *
 * Chat Client - Main agentic chat loop
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"pgedge-postgres-mcp/internal/mcp"

	"github.com/chzyer/readline"
)

// Client is the main chat client
type Client struct {
	config      *Config
	ui          *UI
	mcp         MCPClient
	llm         LLMClient
	messages    []Message
	tools       []mcp.Tool
	resources   []mcp.Resource
	prompts     []mcp.Prompt
	preferences *Preferences
}

// NewClient creates a new chat client
func NewClient(cfg *Config, overrides *ConfigOverrides) (*Client, error) {
	// Load user preferences
	prefs, err := LoadPreferences()
	if err != nil {
		// Log error but don't fail - use defaults
		fmt.Fprintf(os.Stderr, "Warning: Failed to load preferences: %v\n", err)
		prefs = getDefaultPreferences()
	}

	// Apply preferences to config (unless explicitly overridden by command-line flags)
	// Priority: flags > preferences > env vars > config > defaults
	cfg.UI.DisplayStatusMessages = prefs.UI.DisplayStatusMessages
	cfg.UI.RenderMarkdown = prefs.UI.RenderMarkdown
	cfg.UI.Debug = prefs.UI.Debug

	// Apply preferred provider if not explicitly set via flag
	if !overrides.ProviderSet && prefs.LastProvider != "" {
		cfg.LLM.Provider = prefs.LastProvider
	}

	// Apply preferred model for the current provider if not explicitly set via flag
	if !overrides.ModelSet {
		if preferredModel := prefs.GetModelForProvider(cfg.LLM.Provider); preferredModel != "" {
			cfg.LLM.Model = preferredModel
		}
	}

	// Update last provider
	prefs.LastProvider = cfg.LLM.Provider

	ui := NewUI(cfg.UI.NoColor, cfg.UI.RenderMarkdown)
	ui.DisplayStatusMessages = cfg.UI.DisplayStatusMessages
	return &Client{
		config:      cfg,
		ui:          ui,
		messages:    []Message{},
		preferences: prefs,
	}, nil
}

// Run starts the chat client
func (c *Client) Run(ctx context.Context) error {
	// Connect to MCP server
	if err := c.connectToMCP(ctx); err != nil {
		return fmt.Errorf("failed to connect to MCP server: %w", err)
	}
	defer c.mcp.Close()

	// Initialize MCP connection
	if err := c.mcp.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize MCP connection: %w", err)
	}

	// Get available tools
	tools, err := c.mcp.ListTools(ctx)
	if err != nil {
		return fmt.Errorf("failed to list tools: %w", err)
	}
	c.tools = tools

	// Get available resources
	resources, err := c.mcp.ListResources(ctx)
	if err != nil {
		// Don't fail if resources are not supported by the server
		// Just log the error and continue
		if c.config.UI.Debug {
			fmt.Fprintf(os.Stderr, "Warning: Failed to list resources: %v\n", err)
		}
		c.resources = []mcp.Resource{}
	} else {
		c.resources = resources
	}

	// Get available prompts
	prompts, err := c.mcp.ListPrompts(ctx)
	if err != nil {
		// Don't fail if prompts are not supported by the server
		// Just log the error and continue
		if c.config.UI.Debug {
			fmt.Fprintf(os.Stderr, "Warning: Failed to list prompts: %v\n", err)
		}
		c.prompts = []mcp.Prompt{}
	} else {
		c.prompts = prompts
	}

	// Initialize LLM client
	if err := c.initializeLLM(); err != nil {
		return fmt.Errorf("failed to initialize LLM: %w", err)
	}

	// Print welcome message
	c.ui.PrintWelcome()
	c.ui.PrintSystemMessage(fmt.Sprintf("Connected to MCP server (%d tools, %d resources, %d prompts available)", len(c.tools), len(c.resources), len(c.prompts)))
	c.ui.PrintSystemMessage(fmt.Sprintf("Using LLM: %s (%s)", c.config.LLM.Provider, c.config.LLM.Model))
	c.ui.PrintSeparator()

	// Start chat loop
	return c.chatLoop(ctx)
}

// connectToMCP establishes connection to the MCP server
func (c *Client) connectToMCP(ctx context.Context) error {
	if c.config.MCP.Mode == "http" {
		// HTTP mode
		var token string

		if c.config.MCP.AuthMode == "user" {
			// User authentication mode
			username := c.config.MCP.Username
			password := c.config.MCP.Password

			// Prompt for username if not provided
			if username == "" {
				var err error
				username, err = c.ui.PromptForUsername(ctx)
				if err != nil {
					// User interrupted (Ctrl+C) or other input error
					return fmt.Errorf("authentication canceled")
				}
				if username == "" {
					return fmt.Errorf("username is required for user authentication")
				}
			}

			// Prompt for password if not provided
			if password == "" {
				var err error
				password, err = c.ui.PromptForPassword(ctx)
				if err != nil {
					// User interrupted (Ctrl+C) or other input error
					return fmt.Errorf("authentication canceled")
				}
				if password == "" {
					return fmt.Errorf("password is required for user authentication")
				}
			}

			// Authenticate and get session token
			sessionToken, err := c.authenticateUser(ctx, username, password)
			if err != nil {
				return fmt.Errorf("authentication failed: %w", err)
			}
			token = sessionToken
		} else {
			// Token authentication mode
			token = c.config.MCP.Token
			if token == "" {
				// Prompt for token
				token = c.ui.PromptForToken()
				if token == "" {
					return fmt.Errorf("authentication token is required for HTTP mode")
				}
			}
		}

		url := c.config.MCP.URL
		if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
			if c.config.MCP.TLS {
				url = "https://" + url
			} else {
				url = "http://" + url
			}
		}

		// Ensure URL ends with /mcp/v1
		if !strings.HasSuffix(url, "/mcp/v1") {
			if strings.HasSuffix(url, "/") {
				url += "mcp/v1"
			} else {
				url += "/mcp/v1"
			}
		}

		c.mcp = NewHTTPClient(url, token)
	} else {
		// Stdio mode
		mcpClient, err := NewStdioClient(c.config.MCP.ServerPath, c.config.MCP.ServerConfigPath)
		if err != nil {
			return err
		}
		c.mcp = mcpClient
	}

	return nil
}

// authenticateUser authenticates with username/password and returns a session token
func (c *Client) authenticateUser(ctx context.Context, username, password string) (string, error) {
	// Construct the URL for authentication (without /mcp/v1 suffix)
	baseURL := c.config.MCP.URL
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		if c.config.MCP.TLS {
			baseURL = "https://" + baseURL
		} else {
			baseURL = "http://" + baseURL
		}
	}

	// Ensure URL ends with /mcp/v1
	if !strings.HasSuffix(baseURL, "/mcp/v1") {
		if strings.HasSuffix(baseURL, "/") {
			baseURL += "mcp/v1"
		} else {
			baseURL += "/mcp/v1"
		}
	}

	// Create a temporary HTTP client without authentication to call authenticate_user
	tempClient := NewHTTPClient(baseURL, "")

	// Call authenticate_user tool
	args := map[string]interface{}{
		"username": username,
		"password": password,
	}

	response, err := tempClient.CallTool(ctx, "authenticate_user", args)
	if err != nil {
		return "", err
	}

	// Check for errors in response
	if response.IsError {
		if len(response.Content) > 0 {
			return "", fmt.Errorf("%v", response.Content[0].Text)
		}
		return "", fmt.Errorf("authentication failed")
	}

	// Parse the response to extract session token
	if len(response.Content) == 0 {
		return "", fmt.Errorf("empty response from authentication")
	}

	// The response is JSON: {"success": true, "session_token": "...", "expires_at": "...", "message": "..."}
	var authResult struct {
		Success      bool   `json:"success"`
		SessionToken string `json:"session_token"`
		ExpiresAt    string `json:"expires_at"`
		Message      string `json:"message"`
	}

	// Parse JSON from text content
	if err := json.Unmarshal([]byte(response.Content[0].Text), &authResult); err != nil {
		return "", fmt.Errorf("failed to parse authentication response: %w", err)
	}

	if !authResult.Success || authResult.SessionToken == "" {
		return "", fmt.Errorf("authentication failed: %s", authResult.Message)
	}

	return authResult.SessionToken, nil
}

// initializeLLM creates the LLM client
func (c *Client) initializeLLM() error {
	if c.config.LLM.Provider == "anthropic" {
		c.llm = NewAnthropicClient(
			c.config.LLM.AnthropicAPIKey,
			c.config.LLM.Model,
			c.config.LLM.MaxTokens,
			c.config.LLM.Temperature,
			c.config.UI.Debug,
		)
	} else if c.config.LLM.Provider == "openai" {
		c.llm = NewOpenAIClient(
			c.config.LLM.OpenAIAPIKey,
			c.config.LLM.Model,
			c.config.LLM.MaxTokens,
			c.config.LLM.Temperature,
			c.config.UI.Debug,
		)
	} else if c.config.LLM.Provider == "ollama" {
		c.llm = NewOllamaClient(
			c.config.LLM.OllamaURL,
			c.config.LLM.Model,
			c.config.UI.Debug,
		)
	} else {
		return fmt.Errorf("unsupported LLM provider: %s", c.config.LLM.Provider)
	}

	return nil
}

// PrefixCompleter implements readline.AutoCompleter for prefix-based history
type PrefixCompleter struct {
}

// Do implements the AutoCompleter interface for prefix-based history completion
func (pc *PrefixCompleter) Do(line []rune, pos int) (newLine [][]rune, length int) {
	// Get current line text
	lineStr := string(line[:pos])

	// If line is empty, don't suggest anything
	if lineStr == "" {
		return nil, 0
	}

	// This is called for Tab completion - we don't want to interfere with that
	// We only want to filter history on up/down arrows, which readline handles differently
	return nil, 0
}

// chatLoop runs the interactive chat loop
func (c *Client) chatLoop(ctx context.Context) error {
	// Use history file from config
	historyFile := c.config.HistoryFile

	// Configure readline with custom prompt
	rl, err := readline.NewEx(&readline.Config{
		Prompt:                 c.ui.GetPrompt(),
		HistoryFile:            historyFile,
		HistoryLimit:           1000,
		DisableAutoSaveHistory: false,
		InterruptPrompt:        "^C",
		EOFPrompt:              "exit",
		HistorySearchFold:      true, // Enable case-insensitive history search
		// Unfortunately, chzyer/readline doesn't support prefix-based history filtering
		// on up/down arrows natively. Users can use Ctrl+R for reverse search.
	})
	if err != nil {
		return fmt.Errorf("failed to initialize readline: %w", err)
	}
	defer rl.Close()

	// Monitor context cancellation in a goroutine
	go func() {
		<-ctx.Done()
		rl.Close() // Closing readline will cause Readline() to return an error
	}()

	// Main readline loop
	for {
		// This blocks until user provides input
		line, err := rl.Readline()

		if err != nil {
			// Handle various exit conditions
			if err == readline.ErrInterrupt || err == io.EOF {
				fmt.Println()
				c.ui.PrintSystemMessage("Goodbye!")
				return nil
			}
			// Check if context was canceled
			if ctx.Err() != nil {
				fmt.Println()
				c.ui.PrintSystemMessage("Goodbye!")
				return nil
			}
			return fmt.Errorf("readline error: %w", err)
		}

		userInput := strings.TrimSpace(line)
		if userInput == "" {
			continue
		}

		// Check for slash commands first (e.g., /help, /set, /show, /list)
		if cmd := ParseSlashCommand(userInput); cmd != nil {
			if c.HandleSlashCommand(ctx, cmd) {
				continue // Command was handled
			}
			// If HandleSlashCommand returns false, command is unknown
			// Fall through to send to LLM
		}

		// Handle special commands (help, clear, tools, resources)
		if c.handleCommand(ctx, userInput) {
			continue
		}

		// Check for quit command
		if userInput == "quit" || userInput == "exit" {
			c.ui.PrintSystemMessage("Goodbye!")
			return nil
		}

		// Process the query
		if err := c.processQuery(ctx, userInput); err != nil {
			c.ui.PrintError(err.Error())
		}

		c.ui.PrintSeparator()
		// Readline will automatically display the prompt on the next iteration
	}
}

// handleCommand handles special commands, returns true if command was handled
func (c *Client) handleCommand(ctx context.Context, input string) bool {
	switch input {
	case "help":
		c.ui.PrintHelp()
		return true

	case "clear":
		c.ui.ClearScreen()
		c.ui.PrintWelcome()
		return true

	case "tools":
		c.ui.PrintSystemMessage(fmt.Sprintf("Available tools (%d):", len(c.tools)))
		for _, tool := range c.tools {
			fmt.Printf("  - %s: %s\n", tool.Name, tool.Description)
		}
		return true

	case "resources":
		c.ui.PrintSystemMessage(fmt.Sprintf("Available resources (%d):", len(c.resources)))
		for _, resource := range c.resources {
			fmt.Printf("  - %s: %s\n", resource.Name, resource.Description)
		}
		return true

	case "prompts":
		c.ui.PrintSystemMessage(fmt.Sprintf("Available prompts (%d):", len(c.prompts)))
		for _, prompt := range c.prompts {
			fmt.Printf("  - %s: %s\n", prompt.Name, prompt.Description)
			if len(prompt.Arguments) > 0 {
				fmt.Printf("    Arguments:\n")
				for _, arg := range prompt.Arguments {
					required := ""
					if arg.Required {
						required = " (required)"
					}
					fmt.Printf("      - %s: %s%s\n", arg.Name, arg.Description, required)
				}
			}
		}
		return true

	default:
		return false
	}
}

// CompactionRequest represents a request to compact chat history.
type CompactionRequest struct {
	Messages     []Message `json:"messages"`
	MaxTokens    int       `json:"max_tokens,omitempty"`
	RecentWindow int       `json:"recent_window,omitempty"`
	KeepAnchors  bool      `json:"keep_anchors"`
}

// CompactionResponse contains the compacted messages and statistics.
type CompactionResponse struct {
	Messages       []Message      `json:"messages"`
	TokenEstimate  int            `json:"token_estimate"`
	CompactionInfo CompactionInfo `json:"compaction_info"`
}

// CompactionInfo provides statistics about the compaction operation.
type CompactionInfo struct {
	OriginalCount    int     `json:"original_count"`
	CompactedCount   int     `json:"compacted_count"`
	DroppedCount     int     `json:"dropped_count"`
	TokensSaved      int     `json:"tokens_saved"`
	CompressionRatio float64 `json:"compression_ratio"`
}

// compactMessages reduces the message history to prevent token overflow.
// It tries to use the server-side smart compaction if available in HTTP mode,
// falling back to local basic compaction if needed.
func (c *Client) compactMessages(messages []Message) []Message {
	const maxRecentMessages = 10
	const maxTokens = 100000

	const minMessagesForCompaction = 30 // Don't compact unless we have at least 30 messages
	const minSavingsThreshold = 5       // Only compact if we can save at least 5 messages

	// If we have fewer messages than the minimum threshold, return all
	if len(messages) < minMessagesForCompaction {
		return messages
	}

	// Estimate if compaction would be worthwhile
	// With recentWindow=10 and keepAnchors=true, we keep at least: 1 (first) + 10 (recent) = 11
	// So we need at least 11 + minSavingsThreshold messages to make it worthwhile
	if len(messages) < (11 + minSavingsThreshold) {
		return messages
	}

	// Try server-side smart compaction if in HTTP mode
	if compacted, ok := c.tryServerCompaction(messages, maxTokens, maxRecentMessages, minSavingsThreshold); ok {
		return compacted
	}

	// Fall back to local basic compaction
	localCompacted := c.localCompactMessages(messages, maxRecentMessages)
	messagesSaved := len(messages) - len(localCompacted)

	// Only use local compaction if it actually saves enough messages
	if messagesSaved < minSavingsThreshold {
		if c.config.UI.Debug {
			fmt.Fprintf(os.Stderr, "[DEBUG] Local compaction skipped - only saved %d messages (threshold: %d)\n",
				messagesSaved, minSavingsThreshold)
		}
		return messages
	}

	return localCompacted
}

// tryServerCompaction attempts to use the server's smart compaction endpoint.
func (c *Client) tryServerCompaction(messages []Message, maxTokens, recentWindow, minSavingsThreshold int) ([]Message, bool) {
	// Only available in HTTP mode
	httpClient, ok := c.mcp.(*httpClient)
	if !ok {
		return nil, false
	}

	// Build compaction request
	reqBody := CompactionRequest{
		Messages:     messages,
		MaxTokens:    maxTokens,
		RecentWindow: recentWindow,
		KeepAnchors:  true,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		if c.config.UI.Debug {
			fmt.Fprintf(os.Stderr, "[DEBUG] Failed to marshal compaction request: %v\n", err)
		}
		return nil, false
	}

	// Call the compaction endpoint
	req, err := http.NewRequest("POST", httpClient.url+"/api/chat/compact", bytes.NewBuffer(jsonData))
	if err != nil {
		if c.config.UI.Debug {
			fmt.Fprintf(os.Stderr, "[DEBUG] Failed to create compaction request: %v\n", err)
		}
		return nil, false
	}

	req.Header.Set("Content-Type", "application/json")
	if httpClient.token != "" {
		req.Header.Set("Authorization", "Bearer "+httpClient.token)
	}

	resp, err := httpClient.client.Do(req)
	if err != nil {
		if c.config.UI.Debug {
			fmt.Fprintf(os.Stderr, "[DEBUG] Compaction request failed: %v\n", err)
		}
		return nil, false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if c.config.UI.Debug {
			fmt.Fprintf(os.Stderr, "[DEBUG] Compaction returned status %d\n", resp.StatusCode)
		}
		return nil, false
	}

	// Parse response
	var compactResp CompactionResponse
	if err := json.NewDecoder(resp.Body).Decode(&compactResp); err != nil {
		if c.config.UI.Debug {
			fmt.Fprintf(os.Stderr, "[DEBUG] Failed to decode compaction response: %v\n", err)
		}
		return nil, false
	}

	// Check if compaction actually saved enough messages
	info := compactResp.CompactionInfo
	messagesSaved := info.OriginalCount - info.CompactedCount
	if messagesSaved < minSavingsThreshold {
		if c.config.UI.Debug {
			fmt.Fprintf(os.Stderr, "[DEBUG] Server compaction skipped - only saved %d messages (threshold: %d)\n",
				messagesSaved, minSavingsThreshold)
		}
		return nil, false
	}

	// Show compaction status to user (only when actually using it)
	fmt.Fprintf(os.Stderr, "Compacting chat history...\n")

	if c.config.UI.Debug {
		fmt.Fprintf(os.Stderr, "[DEBUG] Server compaction: %d -> %d messages (dropped %d, saved %d tokens, ratio %.2f)\n",
			info.OriginalCount, info.CompactedCount, info.DroppedCount,
			info.TokensSaved, info.CompressionRatio)
	}

	return compactResp.Messages, true
}

// localCompactMessages performs basic local compaction.
// Strategy: Keep the first user message and the last N messages.
// This preserves the original query context while maintaining recent conversation flow.
func (c *Client) localCompactMessages(messages []Message, maxRecentMessages int) []Message {
	compacted := make([]Message, 0, maxRecentMessages+1)

	// Keep the first user message (original query)
	if len(messages) > 0 && messages[0].Role == "user" {
		compacted = append(compacted, messages[0])
	}

	// Keep the last N messages
	startIdx := len(messages) - maxRecentMessages
	if startIdx < 1 {
		startIdx = 1 // Skip first message since we already added it
	}
	compacted = append(compacted, messages[startIdx:]...)

	if c.config.UI.Debug {
		fmt.Fprintf(os.Stderr, "[DEBUG] Local compaction: %d -> %d (kept first + last %d)\n",
			len(messages), len(compacted), maxRecentMessages)
	}

	return compacted
}

func (c *Client) processQuery(ctx context.Context, query string) error {
	const maxAgenticLoops = 50 // Maximum iterations to prevent infinite loops

	// Add user message to conversation history (skip if empty, used for prompts)
	if query != "" {
		c.messages = append(c.messages, Message{
			Role:    "user",
			Content: query,
		})
	}

	// Start thinking animation
	thinkingDone := make(chan struct{})
	go c.ui.ShowThinking(ctx, thinkingDone)

	// Agentic loop (allow up to maxAgenticLoops iterations for complex queries)
	for iteration := 0; iteration < maxAgenticLoops; iteration++ {
		// Compact message history to prevent token overflow
		compactedMessages := c.compactMessages(c.messages)

		// Get response from LLM with compacted history
		response, err := c.llm.Chat(ctx, compactedMessages, c.tools)
		if err != nil {
			close(thinkingDone)
			return fmt.Errorf("LLM error: %w", err)
		}

		// Check if LLM wants to use tools
		if response.StopReason == "tool_use" {
			// Extract tool uses and text content
			var toolUses []ToolUse
			var textParts []string

			for _, item := range response.Content {
				switch v := item.(type) {
				case ToolUse:
					toolUses = append(toolUses, v)
				case TextContent:
					_ = append(textParts, v.Text) // Not used in this context, just checking type
				}
			}

			// Add assistant's message to history
			c.messages = append(c.messages, Message{
				Role:    "assistant",
				Content: response.Content,
			})

			// Execute all tool calls
			toolResults := []ToolResult{}
			for _, toolUse := range toolUses {
				close(thinkingDone)
				// Give the thinking animation goroutine time to clear the line
				time.Sleep(50 * time.Millisecond)
				c.ui.PrintToolExecution(toolUse.Name, toolUse.Input)
				thinkingDone = make(chan struct{})
				go c.ui.ShowThinking(ctx, thinkingDone)

				result, err := c.mcp.CallTool(ctx, toolUse.Name, toolUse.Input)
				if err != nil {
					toolResults = append(toolResults, ToolResult{
						Type:      "tool_result",
						ToolUseID: toolUse.ID,
						Content:   fmt.Sprintf("Error: %v", err),
						IsError:   true,
					})
				} else {
					toolResults = append(toolResults, ToolResult{
						Type:      "tool_result",
						ToolUseID: toolUse.ID,
						Content:   result.Content,
						IsError:   result.IsError,
					})

					// Refresh tool list after successful manage_connections operation
					// This ensures we get the updated tool list when database connection changes
					if toolUse.Name == "manage_connections" && !result.IsError {
						if newTools, err := c.mcp.ListTools(ctx); err == nil {
							c.tools = newTools
						}
					}
				}
			}

			// Add tool results to conversation
			c.messages = append(c.messages, Message{
				Role:    "user",
				Content: toolResults,
			})

			// Continue the loop to get final response
			continue
		}

		// Got final response
		close(thinkingDone)

		// Extract and display text content
		var textParts []string
		for _, item := range response.Content {
			if text, ok := item.(TextContent); ok {
				textParts = append(textParts, text.Text)
			}
		}

		finalText := strings.Join(textParts, "\n")
		c.ui.PrintAssistantResponse(finalText)

		// Add assistant's response to history
		c.messages = append(c.messages, Message{
			Role:    "assistant",
			Content: finalText,
		})

		return nil
	}

	close(thinkingDone)
	return fmt.Errorf("reached maximum number of tool calls (%d)", maxAgenticLoops)
}

// SavePreferences saves the current preferences to disk
func (c *Client) SavePreferences() error {
	if c.preferences == nil {
		return nil
	}

	// Update preferences with current provider and model
	c.preferences.LastProvider = c.config.LLM.Provider
	c.preferences.SetModelForProvider(c.config.LLM.Provider, c.config.LLM.Model)

	return SavePreferences(c.preferences)
}
