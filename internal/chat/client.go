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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"pgedge-postgres-mcp/internal/mcp"

	"github.com/chzyer/readline"
)

// Client is the main chat client
type Client struct {
	config                *Config
	ui                    *UI
	mcp                   MCPClient
	llm                   LLMClient
	messages              []Message
	tools                 []mcp.Tool
	resources             []mcp.Resource
	prompts               []mcp.Prompt
	preferences           *Preferences
	conversations         *ConversationsClient
	currentConversationID string
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

	// Apply UI preferences from saved prefs
	cfg.UI.DisplayStatusMessages = prefs.UI.DisplayStatusMessages
	cfg.UI.RenderMarkdown = prefs.UI.RenderMarkdown
	cfg.UI.Debug = prefs.UI.Debug
	// Color preference (inverted: Color=true means NoColor=false)
	// Only apply if not already set by environment variable NO_COLOR
	if os.Getenv("NO_COLOR") == "" {
		cfg.UI.NoColor = !prefs.UI.Color
	}

	// === PROVIDER SELECTION LOGIC ===
	// Priority: flags > saved provider (if configured) > first configured provider
	if !overrides.ProviderSet {
		// Check if saved provider is configured
		if prefs.LastProvider != "" && cfg.IsProviderConfigured(prefs.LastProvider) {
			cfg.LLM.Provider = prefs.LastProvider
		} else {
			// Use first configured provider (anthropic > openai > ollama)
			configuredProviders := cfg.GetConfiguredProviders()
			if len(configuredProviders) == 0 {
				return nil, fmt.Errorf("no LLM provider configured (set API key for anthropic, openai, or ollama URL)")
			}
			cfg.LLM.Provider = configuredProviders[0]
		}
	}

	// Update prefs with actual provider being used
	prefs.LastProvider = cfg.LLM.Provider

	// === MODEL SELECTION ===
	// If model not set via flag, clear it so initializeLLM() will auto-select
	// based on saved preferences and available models from the provider
	if !overrides.ModelSet {
		cfg.LLM.Model = ""
	}

	ui := NewUI(cfg.UI.NoColor, cfg.UI.RenderMarkdown)
	ui.DisplayStatusMessages = cfg.UI.DisplayStatusMessages
	return &Client{
		config:      cfg,
		ui:          ui,
		messages:    []Message{},
		preferences: prefs,
	}, nil
}

// sanitizeTerminal ensures the terminal is in a sane state.
// This fixes issues if a previous run exited without restoring terminal settings
// (e.g., if the program crashed while in raw mode).
func (c *Client) sanitizeTerminal() {
	// Use stty sane to reset terminal to a sensible state
	// This is a no-op if terminal is already in a good state
	cmd := exec.Command("stty", "sane")
	cmd.Stdin = os.Stdin
	_ = cmd.Run() //nolint:errcheck // Best-effort terminal reset, errors are expected on non-TTY
}

// Run starts the chat client
func (c *Client) Run(ctx context.Context) error {
	// Ensure terminal is in a sane state at startup
	// This fixes issues if a previous run exited without restoring terminal settings
	c.sanitizeTerminal()

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

	// Restore saved database preference for this server
	c.restoreDatabasePreference(ctx)

	// Initialize LLM client
	if err := c.initializeLLM(); err != nil {
		return fmt.Errorf("failed to initialize LLM: %w", err)
	}

	// Print welcome message with version info
	serverName, serverVersion := c.mcp.GetServerInfo()
	c.ui.PrintWelcome(ClientVersion, serverVersion)
	c.ui.PrintSystemMessage(fmt.Sprintf("Connected to %s (%d tools, %d resources, %d prompts)", serverName, len(c.tools), len(c.resources), len(c.prompts)))
	c.ui.PrintSystemMessage(fmt.Sprintf("Using LLM: %s (%s)", c.config.LLM.Provider, c.config.LLM.Model))

	// Display current database
	if databases, current, err := c.mcp.ListDatabases(ctx); err == nil && len(databases) > 0 {
		c.ui.PrintSystemMessage(fmt.Sprintf("Database: %s", current))
	}

	c.ui.PrintSeparator()

	// Start chat loop
	return c.chatLoop(ctx)
}

// connectToMCP establishes connection to the MCP server
func (c *Client) connectToMCP(ctx context.Context) error {
	if c.config.MCP.Mode == "http" {
		// HTTP mode
		var token string

		if c.config.MCP.AuthMode == "none" {
			// No authentication - connect without a token
			// Used when server has auth disabled
			token = ""
		} else if c.config.MCP.AuthMode == "user" {
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
			// Token authentication mode (default for non-"none", non-"user")
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
		// Initialize conversations client for HTTP mode with authentication
		c.conversations = NewConversationsClient(url, token)
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

// initializeLLM creates the LLM client with model validation and auto-selection
func (c *Client) initializeLLM() error {
	provider := c.config.LLM.Provider

	// Create a temporary client to query available models
	var tempClient LLMClient
	switch provider {
	case "anthropic":
		tempClient = NewAnthropicClient(
			c.config.LLM.AnthropicAPIKey, "", 0, 0, false)
	case "openai":
		tempClient = NewOpenAIClient(
			c.config.LLM.OpenAIAPIKey, "", 0, 0, false)
	case "ollama":
		tempClient = NewOllamaClient(
			c.config.LLM.OllamaURL, "", false)
	default:
		return fmt.Errorf("unsupported LLM provider: %s", provider)
	}

	// Get available models from the provider
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	availableModels, err := tempClient.ListModels(ctx)
	if err != nil {
		// If we can't list models, log warning but continue with defaults
		if c.config.UI.Debug {
			fmt.Fprintf(os.Stderr, "Warning: Failed to list models from %s: %v\n", provider, err)
		}
		availableModels = nil
	}

	// Select the best model to use
	selection := c.selectModel(provider, availableModels)
	c.config.LLM.Model = selection.model

	// Log if we used a family match (newer version of saved model)
	if selection.usedFamilyMatch && c.config.UI.Debug {
		savedModel := c.preferences.GetModelForProvider(provider)
		fmt.Fprintf(os.Stderr, "[DEBUG] Model updated: %s → %s (newer version available)\n",
			savedModel, selection.model)
	}

	// Only save preferences if:
	// 1. There was no saved preference (first time using this provider)
	// 2. We used a family match (update to newer version)
	// Do NOT save if we fell back to default/first available - that would corrupt user's preference
	shouldSave := !selection.hadSavedPref || selection.usedFamilyMatch
	if shouldSave {
		c.preferences.SetModelForProvider(provider, selection.model)
		if err := SavePreferences(c.preferences); err != nil {
			if c.config.UI.Debug {
				fmt.Fprintf(os.Stderr, "Warning: Failed to save preferences: %v\n", err)
			}
		}
	}

	// Create the actual LLM client with the selected model
	switch provider {
	case "anthropic":
		c.llm = NewAnthropicClient(
			c.config.LLM.AnthropicAPIKey,
			c.config.LLM.Model,
			c.config.LLM.MaxTokens,
			c.config.LLM.Temperature,
			c.config.UI.Debug,
		)
	case "openai":
		c.llm = NewOpenAIClient(
			c.config.LLM.OpenAIAPIKey,
			c.config.LLM.Model,
			c.config.LLM.MaxTokens,
			c.config.LLM.Temperature,
			c.config.UI.Debug,
		)
	case "ollama":
		c.llm = NewOllamaClient(
			c.config.LLM.OllamaURL,
			c.config.LLM.Model,
			c.config.UI.Debug,
		)
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

		// Check for slash commands (all CLI commands start with /)
		if cmd := ParseSlashCommand(userInput); cmd != nil {
			if c.HandleSlashCommand(ctx, cmd) {
				continue // Command was handled
			}
			// Unknown slash command - inform user
			c.ui.PrintError(fmt.Sprintf("Unknown command: /%s (type /help for available commands)", cmd.Command))
			continue
		}

		// Everything else goes to the LLM
		if err := c.processQuery(ctx, userInput); err != nil {
			c.ui.PrintError(err.Error())
		}

		c.ui.PrintSeparator()
		// Readline will automatically display the prompt on the next iteration
	}
}

// getBriefDescription extracts the first line or sentence from a description
func getBriefDescription(desc string) string {
	// Split by newlines and take first non-empty line
	lines := strings.Split(desc, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			// If line ends with period, return it
			if strings.HasSuffix(line, ".") {
				return line
			}
			// Otherwise, find first sentence (period followed by space or end)
			if idx := strings.Index(line, ". "); idx != -1 {
				return line[:idx+1]
			}
			// No period found, return the whole line
			return line
		}
	}
	return desc
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

// estimateTokens estimates the number of tokens in a string.
// Uses a rough heuristic of ~3.5 characters per token.
func estimateTokens(text string) int {
	if text == "" {
		return 0
	}
	// Rough heuristic: ~4 characters per token for English, ~3 for code/JSON
	// Use 3.5 as a middle ground to be conservative
	return (len(text) + 2) / 3 // Rounds up, slightly more conservative than /3.5
}

// estimateTotalTokens estimates the total tokens in a message array.
func estimateTotalTokens(messages []Message) int {
	total := 0
	for _, msg := range messages {
		switch content := msg.Content.(type) {
		case string:
			total += estimateTokens(content)
		case []interface{}:
			// Handle tool_use and tool_result arrays
			for _, item := range content {
				if m, ok := item.(map[string]interface{}); ok {
					if text, ok := m["text"].(string); ok {
						total += estimateTokens(text)
					}
					if input, ok := m["input"]; ok {
						if jsonBytes, err := json.Marshal(input); err == nil {
							total += estimateTokens(string(jsonBytes))
						}
					}
					if c, ok := m["content"]; ok {
						if text, ok := c.(string); ok {
							total += estimateTokens(text)
						}
					}
				}
			}
		case []ToolResult:
			for _, tr := range content {
				switch c := tr.Content.(type) {
				case []mcp.ContentItem:
					for _, item := range c {
						total += estimateTokens(item.Text)
					}
				case string:
					total += estimateTokens(c)
				}
			}
		}
		// Add overhead for message structure (~10 tokens per message)
		total += 10
	}
	return total
}

// compactMessages reduces the message history to prevent token overflow.
// It tries to use the server-side smart compaction if available in HTTP mode,
// falling back to local basic compaction if needed.
func (c *Client) compactMessages(messages []Message) []Message {
	const maxRecentMessages = 10
	const maxTokens = 100000
	// Compact if estimated tokens exceed this threshold.
	// Note: Anthropic rate limits are typically 30k-60k input tokens/minute cumulative.
	// Setting lower allows multiple requests within the rate limit window.
	const tokenCompactionThreshold = 15000

	const minMessagesForCompaction = 15 // Don't compact unless we have at least 15 messages
	const minSavingsThreshold = 5       // Only compact if we can save at least 5 messages

	// Estimate total tokens in the conversation
	estimatedTokens := estimateTotalTokens(messages)

	// Check if we should compact based on token count OR message count
	shouldCompactByTokens := estimatedTokens > tokenCompactionThreshold
	shouldCompactByMessages := len(messages) >= minMessagesForCompaction

	// If neither threshold is met, skip compaction
	if !shouldCompactByTokens && !shouldCompactByMessages {
		return messages
	}

	// Log why we're compacting (for debugging)
	if c.config.UI.Debug {
		if shouldCompactByTokens {
			fmt.Fprintf(os.Stderr, "[DEBUG] Compaction triggered by token count: ~%d tokens (threshold: %d)\n",
				estimatedTokens, tokenCompactionThreshold)
		} else {
			fmt.Fprintf(os.Stderr, "[DEBUG] Compaction triggered by message count: %d messages (threshold: %d)\n",
				len(messages), minMessagesForCompaction)
		}
	}

	// Estimate if compaction would be worthwhile (only for message-based trigger)
	// With recentWindow=10 and keepAnchors=true, we keep at least: 1 (first) + 10 (recent) = 11
	// So we need at least 11 + minSavingsThreshold messages to make it worthwhile
	// For token-based trigger, always proceed since we need to reduce tokens
	if !shouldCompactByTokens && len(messages) < (11+minSavingsThreshold) {
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
// IMPORTANT: Ensures tool_use/tool_result message pairs are kept together to avoid
// API errors from orphaned tool references.
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

	// Ensure we don't break tool_use/tool_result pairs
	// If the first message we're keeping contains tool_results, we must also
	// keep the preceding assistant message that contains the tool_use blocks
	startIdx = c.adjustStartForToolPairs(messages, startIdx)

	compacted = append(compacted, messages[startIdx:]...)

	if c.config.UI.Debug {
		fmt.Fprintf(os.Stderr, "[DEBUG] Local compaction: %d -> %d (kept first + last %d)\n",
			len(messages), len(compacted), maxRecentMessages)
	}

	return compacted
}

// adjustStartForToolPairs adjusts the start index to ensure tool_use/tool_result
// message pairs are kept together. If the message at startIdx contains tool_results,
// we need to include the preceding assistant message with tool_use blocks.
func (c *Client) adjustStartForToolPairs(messages []Message, startIdx int) int {
	if startIdx <= 1 || startIdx >= len(messages) {
		return startIdx
	}

	// Check if the message at startIdx is a user message with tool_results
	msg := messages[startIdx]
	if msg.Role != "user" {
		return startIdx
	}

	// Check if this message contains tool_result blocks
	if c.hasToolResults(msg) {
		// Include the preceding assistant message (which should have tool_use)
		if startIdx > 1 {
			startIdx--
		}
	}

	return startIdx
}

// hasToolResults checks if a message contains tool_result blocks.
func (c *Client) hasToolResults(msg Message) bool {
	content, ok := msg.Content.([]ToolResult)
	if ok && len(content) > 0 {
		return true
	}

	// Also check for []interface{} format (from JSON unmarshaling)
	if contentSlice, ok := msg.Content.([]interface{}); ok {
		for _, item := range contentSlice {
			if itemMap, ok := item.(map[string]interface{}); ok {
				if itemType, ok := itemMap["type"].(string); ok && itemType == "tool_result" {
					return true
				}
			}
		}
	}

	return false
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

	// Create a cancellable context for this request
	// This allows the user to cancel with Escape key
	reqCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Start thinking animation
	thinkingDone := make(chan struct{})
	go c.ui.ShowThinking(reqCtx, thinkingDone)

	// Start listening for Escape key to cancel the request
	go ListenForEscape(ctx, thinkingDone, cancel)

	// Agentic loop (allow up to maxAgenticLoops iterations for complex queries)
	for iteration := 0; iteration < maxAgenticLoops; iteration++ {
		// Compact message history to prevent token overflow
		compactedMessages := c.compactMessages(c.messages)

		// Get response from LLM with compacted history
		response, err := c.llm.Chat(reqCtx, compactedMessages, c.tools)
		if err != nil {
			close(thinkingDone)
			// Wait for ListenForEscape to restore terminal from raw mode
			time.Sleep(50 * time.Millisecond)
			// Check if this was a user cancellation (Escape key)
			if reqCtx.Err() == context.Canceled && ctx.Err() == nil {
				// User canceled with Escape - keep the query in history
				// but don't save the Escape keypress
				c.ui.PrintCanceled()
				return nil // Return without error to continue the chat loop
			}
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
				go c.ui.ShowThinking(reqCtx, thinkingDone)
				// Start new Escape listener for this tool execution
				go ListenForEscape(ctx, thinkingDone, cancel)

				result, err := c.mcp.CallTool(reqCtx, toolUse.Name, toolUse.Input)
				if err != nil {
					// Check if this was a user cancellation (Escape key)
					if reqCtx.Err() == context.Canceled && ctx.Err() == nil {
						close(thinkingDone)
						// User canceled with Escape - keep the query in history
						// but don't save the Escape keypress
						c.ui.PrintCanceled()
						return nil
					}
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
						if newTools, err := c.mcp.ListTools(reqCtx); err == nil {
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
		// Wait for ListenForEscape to restore terminal from raw mode
		time.Sleep(50 * time.Millisecond)

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
	// Wait for ListenForEscape to restore terminal from raw mode
	time.Sleep(50 * time.Millisecond)
	return fmt.Errorf("reached maximum number of tool calls (%d)", maxAgenticLoops)
}

// SavePreferences saves the current preferences to disk
func (c *Client) SavePreferences() error {
	if c.preferences == nil {
		return nil
	}

	// Just save preferences as-is. The /set commands already update both
	// c.preferences and c.config, and save immediately. We don't want to
	// overwrite c.preferences.LastProvider from c.config here because
	// c.config may have been loaded from file with different values.
	return SavePreferences(c.preferences)
}

// modelSelectionResult contains the result of model selection
type modelSelectionResult struct {
	model           string
	fromSavedPref   bool // true if selected from saved preference (exact or family match)
	hadSavedPref    bool // true if there was a saved preference for this provider
	usedFamilyMatch bool // true if a newer version in the same family was selected
}

// selectModel determines the best model to use based on:
// 1. Command-line flag (if set via config)
// 2. Saved preference - exact match
// 3. Saved preference - family match (e.g., claude-opus-4-5-20251101 → claude-opus-4-5-20251217)
// 4. Default for provider (if available)
// 5. First available model from provider's list
func (c *Client) selectModel(provider string, availableModels []string) modelSelectionResult {
	debug := c.config.UI.Debug

	// If model was already set (via flag), use it (trust the user)
	if c.config.LLM.Model != "" {
		if debug {
			fmt.Fprintf(os.Stderr, "[DEBUG] Model set via flag: %s\n", c.config.LLM.Model)
		}
		return modelSelectionResult{model: c.config.LLM.Model, fromSavedPref: false, hadSavedPref: false}
	}

	// Check saved preference for this provider
	savedModel := c.preferences.GetModelForProvider(provider)
	if debug {
		fmt.Fprintf(os.Stderr, "[DEBUG] Saved model preference for %s: %q\n", provider, savedModel)
		if len(availableModels) > 0 {
			fmt.Fprintf(os.Stderr, "[DEBUG] Available models (%d): %v\n", len(availableModels), availableModels)
		} else {
			fmt.Fprintf(os.Stderr, "[DEBUG] No available models list (API call may have failed)\n")
		}
	}

	if savedModel != "" {
		// Try exact match first
		if isModelAvailable(savedModel, availableModels) {
			if debug {
				fmt.Fprintf(os.Stderr, "[DEBUG] Using saved model (exact match): %s\n", savedModel)
			}
			return modelSelectionResult{model: savedModel, fromSavedPref: true, hadSavedPref: true}
		}

		if debug {
			fmt.Fprintf(os.Stderr, "[DEBUG] Saved model %q not in available models, trying family match\n", savedModel)
		}

		// Try family match (e.g., claude-opus-4-5-* when saved is claude-opus-4-5-20251101)
		// This handles Anthropic releasing newer versions of the same model
		if familyMatch := findModelFamilyMatch(savedModel, availableModels); familyMatch != "" {
			if debug {
				fmt.Fprintf(os.Stderr, "[DEBUG] Family match found: %s → %s\n", savedModel, familyMatch)
			}
			return modelSelectionResult{
				model:           familyMatch,
				fromSavedPref:   true,
				hadSavedPref:    true,
				usedFamilyMatch: true,
			}
		}

		if debug {
			family := extractModelFamily(savedModel)
			fmt.Fprintf(os.Stderr, "[DEBUG] No family match found for %q (family: %q)\n", savedModel, family)
		}

		// Saved preference exists but couldn't be matched
		// Fall through to defaults, but remember we had a saved pref
	}

	hadSaved := savedModel != ""

	// Use default for provider
	defaultModel := getDefaultModelForProvider(provider)
	if isModelAvailable(defaultModel, availableModels) {
		if debug && hadSaved {
			fmt.Fprintf(os.Stderr, "[DEBUG] Falling back to provider default: %s (saved preference %q not available)\n",
				defaultModel, savedModel)
		}
		return modelSelectionResult{model: defaultModel, fromSavedPref: false, hadSavedPref: hadSaved}
	}

	// Fall back to first available model
	if len(availableModels) > 0 {
		if debug {
			fmt.Fprintf(os.Stderr, "[DEBUG] Falling back to first available model: %s (default %q also not available)\n",
				availableModels[0], defaultModel)
		}
		return modelSelectionResult{model: availableModels[0], fromSavedPref: false, hadSavedPref: hadSaved}
	}

	// Last resort: use default even if not validated
	if debug {
		fmt.Fprintf(os.Stderr, "[DEBUG] No models available, using default: %s\n", defaultModel)
	}
	return modelSelectionResult{model: defaultModel, fromSavedPref: false, hadSavedPref: hadSaved}
}

// findModelFamilyMatch finds a model in availableModels that matches the family of savedModel.
// Family matching: "claude-opus-4-5-20251101" matches "claude-opus-4-5-*"
// Returns the latest (by date suffix) matching model, or empty string if no match.
func findModelFamilyMatch(savedModel string, availableModels []string) string {
	if len(availableModels) == 0 {
		return ""
	}

	// Extract family prefix (everything before the last date segment)
	// e.g., "claude-opus-4-5-20251101" → "claude-opus-4-5-"
	family := extractModelFamily(savedModel)
	if family == "" {
		return ""
	}

	// Find all models with the SAME family (exact family match, not prefix)
	// e.g., claude-opus-4-5- should not match claude-opus-4- or claude-opus-4-5-1-
	var matches []string
	for _, m := range availableModels {
		modelFamily := extractModelFamily(m)
		if modelFamily == family {
			matches = append(matches, m)
		}
	}

	if len(matches) == 0 {
		return ""
	}

	// Return the latest version (highest date suffix)
	// Models are typically returned sorted, but sort to be safe
	sort.Strings(matches)
	return matches[len(matches)-1]
}

// extractModelFamily extracts the model family prefix from a model ID.
// Returns the prefix including trailing hyphen, or empty string if not parseable.
// Examples:
//   - "claude-opus-4-5-20251101" → "claude-opus-4-5-"
//   - "claude-sonnet-4-20250514" → "claude-sonnet-4-"
//   - "gpt-4o-mini" → "" (no date suffix pattern)
func extractModelFamily(model string) string {
	// Look for a date suffix pattern: -YYYYMMDD at the end
	// The date is 8 digits after a hyphen
	if len(model) < 9 {
		return ""
	}

	// Check if last 8 chars are digits (date)
	suffix := model[len(model)-8:]
	for _, c := range suffix {
		if c < '0' || c > '9' {
			return "" // Not a date suffix
		}
	}

	// Check there's a hyphen before the date
	if len(model) < 10 || model[len(model)-9] != '-' {
		return ""
	}

	// Return everything up to and including the hyphen before the date
	return model[:len(model)-8]
}

// isModelAvailable checks if model is in the available list
// Returns true if availableModels is nil (couldn't fetch) for graceful degradation
func isModelAvailable(model string, availableModels []string) bool {
	if availableModels == nil {
		return true // Can't validate, assume available
	}
	for _, m := range availableModels {
		if m == model {
			return true
		}
	}
	return false
}

// getDefaultModelForProvider returns the default model for a provider
func getDefaultModelForProvider(provider string) string {
	switch provider {
	case "anthropic":
		return "claude-sonnet-4-5-20250929"
	case "openai":
		return "gpt-4o"
	case "ollama":
		return "qwen3-coder:latest"
	default:
		return ""
	}
}
