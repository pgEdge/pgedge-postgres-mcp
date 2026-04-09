/*-------------------------------------------------------------------------
*
 * pgEdge Natural Language Agent
*
* Copyright (c) 2025 - 2026, pgEdge, Inc.
* This software is released under The PostgreSQL License
*
*-------------------------------------------------------------------------
*/

package chat

import (
	"bytes"
	"context"
	crand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"pgedge-postgres-mcp/internal/embedding"
	"pgedge-postgres-mcp/internal/mcp"
)

// Message represents a chat message
type Message struct {
	Role         string                 `json:"role"`
	Content      interface{}            `json:"content"`
	CacheControl map[string]interface{} `json:"cache_control,omitempty"`
}

// ToolUse represents a tool invocation in a message
type ToolUse struct {
	Type  string                 `json:"type"`
	ID    string                 `json:"id"`
	Name  string                 `json:"name"`
	Input map[string]interface{} `json:"input"`
}

// TextContent represents text content in a message
type TextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// ToolResult represents the result of a tool execution
type ToolResult struct {
	Type      string      `json:"type"`
	ToolUseID string      `json:"tool_use_id"`
	Content   interface{} `json:"content"`
	IsError   bool        `json:"is_error,omitempty"`
}

// LLMResponse represents a response from the LLM
type LLMResponse struct {
	Content    []interface{} // Can be TextContent or ToolUse
	StopReason string
	TokenUsage *TokenUsage `json:"token_usage,omitempty"` // Optional token usage information (only when debug enabled)
}

// TokenUsage holds token usage information for debug purposes
type TokenUsage struct {
	Provider               string  `json:"provider"`
	PromptTokens           int     `json:"prompt_tokens,omitempty"`
	CompletionTokens       int     `json:"completion_tokens,omitempty"`
	TotalTokens            int     `json:"total_tokens,omitempty"`
	CacheCreationTokens    int     `json:"cache_creation_tokens,omitempty"`
	CacheReadTokens        int     `json:"cache_read_tokens,omitempty"`
	CacheSavingsPercentage float64 `json:"cache_savings_percentage,omitempty"`
}

// LLMClient provides a unified interface for different LLM providers
// readOnlySafetyPrompt is appended to the system prompt when the
// database connection is in read-only mode.  It instructs the LLM
// not to attempt any bypass of the read-only transaction setting.
const readOnlySafetyPrompt = `

CRITICAL SECURITY RULE: The database is in READ-ONLY mode. You must NEVER attempt to:
- Modify the transaction_read_only or default_transaction_read_only settings
- Use SET TRANSACTION READ WRITE or any variant
- Use set_config() to change transaction or session read-only settings
- Use DO blocks or PL/pgSQL to bypass read-only restrictions
- Execute any DDL (CREATE, DROP, ALTER) or DML (INSERT, UPDATE, DELETE) statements
Any attempt to bypass read-only mode is a security violation and will be rejected.`

type LLMClient interface {
	// Chat sends messages and available tools to the LLM and returns the response
	Chat(ctx context.Context, messages []Message, tools interface{}) (LLMResponse, error)

	// ListModels returns a list of available models from the provider
	ListModels(ctx context.Context) ([]string, error)

	// SetReadOnlyMode tells the LLM client whether the database
	// connection is in read-only mode so the system prompt can
	// include appropriate safety instructions.
	SetReadOnlyMode(readOnly bool)
}

// anthropicClient implements LLMClient for Anthropic Claude
type anthropicClient struct {
	apiKey        string
	baseURL       string
	model         string
	maxTokens     int
	temperature   float64
	debug         bool
	readOnly      bool
	customHeaders map[string]string
	client        *http.Client
}

// SetReadOnlyMode sets whether the database is in read-only mode.
func (c *anthropicClient) SetReadOnlyMode(readOnly bool) {
	c.readOnly = readOnly
}

// ValidateBaseURL validates and normalizes a base URL for API clients.
// Returns the normalized URL or an error if the URL is invalid.
func ValidateBaseURL(baseURL, providerName string) (string, error) {
	baseURL = strings.TrimSpace(baseURL)
	baseURL = strings.TrimSuffix(baseURL, "/")

	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid %s base URL: %w", providerName, err)
	}
	if parsedURL.Scheme != "https" && parsedURL.Scheme != "http" {
		return "", fmt.Errorf("%s base URL must use http or https scheme, got: %s", providerName, parsedURL.Scheme)
	}
	if parsedURL.Host == "" {
		return "", fmt.Errorf("%s base URL must include a host", providerName)
	}
	return baseURL, nil
}

// NewAnthropicClient creates a new Anthropic client
// baseURL can be empty to use the default (https://api.anthropic.com)
func NewAnthropicClient(apiKey, baseURL, model string, maxTokens int, temperature float64, customHeaders map[string]string, debug bool) (LLMClient, error) {
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	} else {
		var err error
		baseURL, err = ValidateBaseURL(baseURL, "Anthropic")
		if err != nil {
			return nil, err
		}
	}
	return &anthropicClient{
		apiKey:        apiKey,
		baseURL:       baseURL,
		model:         model,
		maxTokens:     maxTokens,
		temperature:   temperature,
		customHeaders: customHeaders,
		debug:         debug,
		client:        &http.Client{},
	}, nil
}

type anthropicRequest struct {
	Model       string                   `json:"model"`
	MaxTokens   int                      `json:"max_tokens"`
	Messages    []Message                `json:"messages"`
	Tools       []map[string]interface{} `json:"tools,omitempty"`
	Temperature float64                  `json:"temperature,omitempty"`
	System      []map[string]interface{} `json:"system,omitempty"` // Support for system messages with caching
}

type anthropicUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
}

type anthropicResponse struct {
	ID         string                   `json:"id"`
	Type       string                   `json:"type"`
	Role       string                   `json:"role"`
	Content    []map[string]interface{} `json:"content"`
	StopReason string                   `json:"stop_reason"`
	Usage      anthropicUsage           `json:"usage"`
}

type anthropicErrorResponse struct {
	Type  string `json:"type"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// extractAnthropicErrorMessage parses Anthropic's error response to get a user-friendly message
func extractAnthropicErrorMessage(statusCode int, body []byte) string {
	var errResp anthropicErrorResponse
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error.Message != "" {
		// Successfully parsed error response, return the message
		return fmt.Sprintf("API error (%d): %s", statusCode, errResp.Error.Message)
	}
	// Fallback to raw body if parsing fails
	return fmt.Sprintf("API error (%d): %s", statusCode, string(body))
}

func (c *anthropicClient) Chat(ctx context.Context, messages []Message, tools interface{}) (LLMResponse, error) {
	startTime := time.Now()
	operation := "chat"
	url := c.baseURL + "/v1/messages"

	embedding.LogLLMCallDetails("anthropic", c.model, operation, url, len(messages))

	// Convert interface{} tools to []mcp.Tool via JSON
	var mcpTools []mcp.Tool
	if tools != nil {
		toolsJSON, err := json.Marshal(tools)
		if err != nil {
			return LLMResponse{}, fmt.Errorf("failed to marshal tools: %w", err)
		}
		if err := json.Unmarshal(toolsJSON, &mcpTools); err != nil {
			return LLMResponse{}, fmt.Errorf("failed to unmarshal tools: %w", err)
		}
	}

	// Convert MCP tools to Anthropic format with caching
	anthropicTools := make([]map[string]interface{}, 0, len(mcpTools))
	for i, tool := range mcpTools {
		toolDef := map[string]interface{}{
			"name":         tool.Name,
			"description":  tool.Description,
			"input_schema": tool.InputSchema,
		}

		// Add cache_control to the last tool definition to cache all tools
		// This caches the entire tools array (must be on the last item)
		if i == len(mcpTools)-1 {
			toolDef["cache_control"] = map[string]interface{}{
				"type": "ephemeral",
			}
		}

		anthropicTools = append(anthropicTools, toolDef)
	}

	// Create system message for better UX
	systemContent := `You are a helpful PostgreSQL database assistant with expert knowledge on PostgreSQL and products from pgEdge with access to MCP tools.

When executing tools:
- Be concise and direct
- Show results without explaining your methodology unless specifically asked
- Base responses ONLY on actual tool results - never make up or guess data
- Format results clearly for the user
- Only use tools when necessary to answer the question`

	if c.readOnly {
		systemContent += readOnlySafetyPrompt
	}

	systemMessage := []map[string]interface{}{
		{
			"type": "text",
			"text": systemContent,
		},
	}

	req := anthropicRequest{
		Model:       c.model,
		MaxTokens:   c.maxTokens,
		Messages:    messages,
		Tools:       anthropicTools,
		Temperature: c.temperature,
		System:      systemMessage,
	}

	reqData, err := json.Marshal(req)
	if err != nil {
		return LLMResponse{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	embedding.LogLLMRequestTrace("anthropic", c.model, operation, string(reqData))

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqData))
	if err != nil {
		return LLMResponse{}, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("anthropic-beta", "prompt-caching-2024-07-31")
	for k, v := range c.customHeaders {
		httpReq.Header.Set(k, v)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		embedding.LogConnectionError("anthropic", url, err)
		duration := time.Since(startTime)
		embedding.LogLLMCall("anthropic", c.model, operation, 0, 0, duration, err)
		return LLMResponse{}, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			duration := time.Since(startTime)
			readErr := fmt.Errorf("API error %d (failed to read body: %w)", resp.StatusCode, err)
			embedding.LogLLMCall("anthropic", c.model, operation, 0, 0, duration, readErr)
			return LLMResponse{}, readErr
		}

		// Check if this is a rate limit error
		if resp.StatusCode == 429 {
			embedding.LogRateLimitError("anthropic", c.model, resp.StatusCode, string(body))
		}

		// Extract user-friendly error message from Anthropic's error response
		userFriendlyMsg := extractAnthropicErrorMessage(resp.StatusCode, body)

		duration := time.Since(startTime)
		apiErr := fmt.Errorf("%s", userFriendlyMsg)
		embedding.LogLLMCall("anthropic", c.model, operation, 0, 0, duration, apiErr)
		return LLMResponse{}, apiErr
	}

	var anthropicResp anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&anthropicResp); err != nil {
		duration := time.Since(startTime)
		embedding.LogLLMCall("anthropic", c.model, operation, 0, 0, duration, err)
		return LLMResponse{}, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert response content to typed structs
	content := make([]interface{}, 0, len(anthropicResp.Content))
	for _, item := range anthropicResp.Content {
		itemType, ok := item["type"].(string)
		if !ok {
			continue
		}
		switch itemType {
		case "text":
			text, ok := item["text"].(string)
			if !ok {
				continue
			}
			content = append(content, TextContent{
				Type: "text",
				Text: text,
			})
		case "tool_use":
			id, ok := item["id"].(string)
			if !ok {
				continue
			}
			name, ok := item["name"].(string)
			if !ok {
				continue
			}
			input, ok := item["input"].(map[string]interface{})
			if !ok {
				input = make(map[string]interface{})
			}
			content = append(content, ToolUse{
				Type:  "tool_use",
				ID:    id,
				Name:  name,
				Input: input,
			})
		}
	}

	duration := time.Since(startTime)
	embedding.LogLLMResponseTrace("anthropic", c.model, operation, resp.StatusCode, anthropicResp.StopReason)
	embedding.LogLLMCall("anthropic", c.model, operation, anthropicResp.Usage.InputTokens, anthropicResp.Usage.OutputTokens, duration, nil)

	// Build token usage for debug
	var tokenUsage *TokenUsage
	if c.debug {
		totalInput := anthropicResp.Usage.InputTokens + anthropicResp.Usage.CacheReadInputTokens
		savePercent := 0.0
		if totalInput > 0 {
			savePercent = float64(anthropicResp.Usage.CacheReadInputTokens) / float64(totalInput) * 100
		}

		tokenUsage = &TokenUsage{
			Provider:               "anthropic",
			PromptTokens:           anthropicResp.Usage.InputTokens,
			CompletionTokens:       anthropicResp.Usage.OutputTokens,
			TotalTokens:            anthropicResp.Usage.InputTokens + anthropicResp.Usage.OutputTokens,
			CacheCreationTokens:    anthropicResp.Usage.CacheCreationInputTokens,
			CacheReadTokens:        anthropicResp.Usage.CacheReadInputTokens,
			CacheSavingsPercentage: savePercent,
		}

		// Log to stderr for CLI (use \r\n to clear spinner line first)
		if anthropicResp.Usage.CacheCreationInputTokens > 0 || anthropicResp.Usage.CacheReadInputTokens > 0 {
			fmt.Fprintf(os.Stderr, "\r\n[LLM] [DEBUG] Anthropic - Prompt Cache: Created %d tokens, Read %d tokens (saved ~%.0f%% on input)\n",
				anthropicResp.Usage.CacheCreationInputTokens,
				anthropicResp.Usage.CacheReadInputTokens,
				savePercent,
			)
			fmt.Fprintf(os.Stderr, "\r[LLM] [DEBUG] Anthropic - Tokens: Input %d, Output %d, Total %d\n",
				anthropicResp.Usage.InputTokens,
				anthropicResp.Usage.OutputTokens,
				anthropicResp.Usage.InputTokens+anthropicResp.Usage.OutputTokens,
			)
		} else {
			fmt.Fprintf(os.Stderr, "\r\n[LLM] [DEBUG] Anthropic - Tokens: Input %d, Output %d, Total %d\n",
				anthropicResp.Usage.InputTokens,
				anthropicResp.Usage.OutputTokens,
				anthropicResp.Usage.InputTokens+anthropicResp.Usage.OutputTokens,
			)
		}
	}

	return LLMResponse{
		Content:    content,
		StopReason: anthropicResp.StopReason,
		TokenUsage: tokenUsage,
	}, nil
}

// ListModels returns available Anthropic Claude models from the API
func (c *anthropicClient) ListModels(ctx context.Context) ([]string, error) {
	url := c.baseURL + "/v1/models"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	for k, v := range c.customHeaders {
		req.Header.Set(k, v)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body) //nolint:errcheck // Error response body read is best effort
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	// Parse response: {"data": [{"id": "claude-3-opus-20240229", "type": "model", ...}, ...]}
	var response struct {
		Data []struct {
			ID   string `json:"id"`
			Type string `json:"type"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	models := make([]string, 0, len(response.Data))
	for _, model := range response.Data {
		// Only include models (not other types if any)
		if model.Type == "model" {
			models = append(models, model.ID)
		}
	}

	return models, nil
}

// ollamaClient implements LLMClient for Ollama
type ollamaClient struct {
	baseURL       string
	model         string
	debug         bool
	readOnly      bool
	customHeaders map[string]string
	client        *http.Client
}

// SetReadOnlyMode sets whether the database is in read-only mode.
func (c *ollamaClient) SetReadOnlyMode(readOnly bool) {
	c.readOnly = readOnly
}

// NewOllamaClient creates a new Ollama client
func NewOllamaClient(baseURL, model string, customHeaders map[string]string, debug bool) LLMClient {
	return &ollamaClient{
		baseURL:       baseURL,
		model:         model,
		customHeaders: customHeaders,
		debug:         debug,
		client:        &http.Client{},
	}
}

type ollamaMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
	ToolName  string           `json:"tool_name,omitempty"`
}

type ollamaToolCall struct {
	Function ollamaToolCallFunction `json:"function"`
}

type ollamaToolCallFunction struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

type ollamaTool struct {
	Type     string             `json:"type"`
	Function ollamaToolFunction `json:"function"`
}

type ollamaToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

type ollamaRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Tools    []ollamaTool    `json:"tools,omitempty"`
	Stream   bool            `json:"stream"`
}

type ollamaResponse struct {
	Model   string        `json:"model"`
	Message ollamaMessage `json:"message"`
	Done    bool          `json:"done"`
}

// toolCallRequest represents a tool call parsed from Ollama's response
type toolCallRequest struct {
	Tool      string                 `json:"tool"`
	Arguments map[string]interface{} `json:"arguments"`
}

type ollamaErrorResponse struct {
	Error string `json:"error"`
}

// extractOllamaErrorMessage parses Ollama's error response to get a user-friendly message
func extractOllamaErrorMessage(statusCode int, body []byte) string {
	var errResp ollamaErrorResponse
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error != "" {
		// Successfully parsed error response, return the message
		return fmt.Sprintf("Ollama error (%d): %s", statusCode, errResp.Error)
	}
	// Fallback to raw body if parsing fails
	bodyStr := string(body)
	if len(bodyStr) > 200 {
		bodyStr = bodyStr[:200] + "..."
	}
	return fmt.Sprintf("Ollama error (%d): %s", statusCode, bodyStr)
}

// extractJSONFromText attempts to extract a JSON object from text that may contain
// additional explanation or commentary around the JSON
func extractJSONFromText(text string) string {
	// Find the first '{' and last '}' to extract the JSON object
	firstBrace := strings.Index(text, "{")
	if firstBrace == -1 {
		return ""
	}

	// Find the matching closing brace by counting braces
	braceCount := 0
	lastBrace := -1
	for i := firstBrace; i < len(text); i++ {
		if text[i] == '{' {
			braceCount++
		} else if text[i] == '}' {
			braceCount--
			if braceCount == 0 {
				lastBrace = i
				break
			}
		}
	}

	if lastBrace == -1 {
		return ""
	}

	return text[firstBrace : lastBrace+1]
}

func (c *ollamaClient) Chat(ctx context.Context, messages []Message, tools interface{}) (LLMResponse, error) {
	startTime := time.Now()
	operation := "chat"
	url := c.baseURL + "/api/chat"

	embedding.LogLLMCallDetails("ollama", c.model, operation, url, len(messages))

	// Convert interface{} tools to []mcp.Tool via JSON
	var mcpTools []mcp.Tool
	if tools != nil {
		toolsJSON, err := json.Marshal(tools)
		if err != nil {
			return LLMResponse{}, fmt.Errorf("failed to marshal tools: %w", err)
		}
		if err := json.Unmarshal(toolsJSON, &mcpTools); err != nil {
			return LLMResponse{}, fmt.Errorf("failed to unmarshal tools: %w", err)
		}
	}

	// Convert MCP tools to Ollama's native tool format
	var ollamaTools []ollamaTool
	for _, tool := range mcpTools {
		params := make(map[string]interface{})
		if tool.InputSchema.Type != "" {
			params["type"] = tool.InputSchema.Type
		}
		if tool.InputSchema.Properties != nil {
			params["properties"] = tool.InputSchema.Properties
		}
		if tool.InputSchema.Required != nil {
			params["required"] = tool.InputSchema.Required
		}
		ollamaTools = append(ollamaTools, ollamaTool{
			Type: "function",
			Function: ollamaToolFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  params,
			},
		})
	}

	// Build system message - use simpler prompt when native tools are
	// available since tool definitions are passed via the API; fall back
	// to text-based tool descriptions when no native tools are present.
	systemContent := `You are a helpful PostgreSQL database assistant with expert knowledge on PostgreSQL and products from pgEdge.

When executing tools:
- Be concise and direct
- Show results without explaining your methodology unless specifically asked
- Base responses ONLY on actual tool results - never make up or guess data
- Format results clearly for the user
- Only use tools when necessary to answer the question`
	if len(ollamaTools) > 0 {
		systemContent += "\n- When you receive tool results, use them to provide a clear answer to the user's question"
	}

	if c.readOnly {
		systemContent += readOnlySafetyPrompt
	}

	// Convert messages to Ollama format
	ollamaMessages := []ollamaMessage{
		{
			Role:    "system",
			Content: systemContent,
		},
	}

	// Build a map of tool use IDs to tool names for correlating
	// tool results with their original tool calls.
	toolNameByID := make(map[string]string)
	for _, msg := range messages {
		if items, ok := msg.Content.([]interface{}); ok && msg.Role == "assistant" {
			for _, item := range items {
				switch v := item.(type) {
				case ToolUse:
					toolNameByID[v.ID] = v.Name
				default:
					if itemMap, ok := item.(map[string]interface{}); ok {
						if itemType, ok2 := itemMap["type"].(string); ok2 && itemType == "tool_use" {
							if id, ok1 := itemMap["id"].(string); ok1 {
								if name, ok2 := itemMap["name"].(string); ok2 {
									toolNameByID[id] = name
								}
							}
						}
					}
				}
			}
		}
	}

	for _, msg := range messages {
		switch content := msg.Content.(type) {
		case string:
			ollamaMessages = append(ollamaMessages, ollamaMessage{
				Role:    msg.Role,
				Content: content,
			})
		case []ToolResult:
			// Handle []ToolResult directly (from client.go tool execution)
			for _, v := range content {
				contentStr := toolResultContentString(v.Content)
				if contentStr == "" {
					contentStr = "{}"
				}
				ollamaMessages = append(ollamaMessages, ollamaMessage{
					Role:     "tool",
					Content:  contentStr,
					ToolName: toolNameByID[v.ToolUseID],
				})
			}
		case []interface{}:
			// Check if this is an assistant message with tool calls
			if msg.Role == "assistant" {
				var toolCalls []ollamaToolCall
				var textContent string
				for _, item := range content {
					switch v := item.(type) {
					case ToolUse:
						toolCalls = append(toolCalls, ollamaToolCall{
							Function: ollamaToolCallFunction{
								Name:      v.Name,
								Arguments: v.Input,
							},
						})
					case TextContent:
						if textContent != "" {
							textContent += " "
						}
						textContent += v.Text
					default:
						itemMap, ok := item.(map[string]interface{})
						if !ok {
							continue
						}
						itemType, ok2 := itemMap["type"].(string)
						if !ok2 {
							continue
						}
						switch itemType {
						case "tool_use":
							name, ok1 := itemMap["name"].(string)
							if !ok1 {
								continue
							}
							input, ok2 := itemMap["input"].(map[string]interface{})
							if !ok2 || input == nil {
								input = map[string]interface{}{}
							}
							toolCalls = append(toolCalls, ollamaToolCall{
								Function: ollamaToolCallFunction{
									Name:      name,
									Arguments: input,
								},
							})
						case "text":
							if text, ok := itemMap["text"].(string); ok {
								if textContent != "" {
									textContent += " "
								}
								textContent += text
							}
						}
					}
				}
				if len(toolCalls) > 0 {
					ollamaMessages = append(ollamaMessages, ollamaMessage{
						Role:      "assistant",
						Content:   textContent,
						ToolCalls: toolCalls,
					})
				} else {
					ollamaMessages = append(ollamaMessages, ollamaMessage{
						Role:    "assistant",
						Content: textContent,
					})
				}
			} else {
				// Handle tool results - convert to role: "tool"
				for _, item := range content {
					switch v := item.(type) {
					case ToolResult:
						contentStr := toolResultContentString(v.Content)
						if contentStr == "" {
							contentStr = "{}"
						}
						ollamaMessages = append(ollamaMessages, ollamaMessage{
							Role:     "tool",
							Content:  contentStr,
							ToolName: toolNameByID[v.ToolUseID],
						})
					default:
						itemMap, ok := item.(map[string]interface{})
						if !ok {
							continue
						}
						itemType, ok2 := itemMap["type"].(string)
						if !ok2 || itemType != "tool_result" {
							continue
						}
						toolUseID, ok := itemMap["tool_use_id"].(string)
						if !ok {
							continue
						}
						contentStr := extractTextFromContent(itemMap["content"])
						if contentStr == "" {
							contentStr = "{}"
						}
						ollamaMessages = append(ollamaMessages, ollamaMessage{
							Role:     "tool",
							Content:  contentStr,
							ToolName: toolNameByID[toolUseID],
						})
					}
				}
			}
		}
	}

	req := ollamaRequest{
		Model:    c.model,
		Messages: ollamaMessages,
		Tools:    ollamaTools,
		Stream:   false,
	}

	reqData, err := json.Marshal(req)
	if err != nil {
		return LLMResponse{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqData))
	if err != nil {
		return LLMResponse{}, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	for k, v := range c.customHeaders {
		httpReq.Header.Set(k, v)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		embedding.LogConnectionError("ollama", url, err)
		duration := time.Since(startTime)
		embedding.LogLLMCall("ollama", c.model, operation, 0, 0, duration, err)
		return LLMResponse{}, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			duration := time.Since(startTime)
			readErr := fmt.Errorf("API error %d (failed to read body: %w)", resp.StatusCode, err)
			embedding.LogLLMCall("ollama", c.model, operation, 0, 0, duration, readErr)
			return LLMResponse{}, readErr
		}

		// Extract user-friendly error message from Ollama's error response
		userFriendlyMsg := extractOllamaErrorMessage(resp.StatusCode, body)

		duration := time.Since(startTime)
		apiErr := fmt.Errorf("%s", userFriendlyMsg)
		embedding.LogLLMCall("ollama", c.model, operation, 0, 0, duration, apiErr)
		return LLMResponse{}, apiErr
	}

	var ollamaResp ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		duration := time.Since(startTime)
		embedding.LogLLMCall("ollama", c.model, operation, 0, 0, duration, err)
		return LLMResponse{}, fmt.Errorf("failed to decode response: %w", err)
	}

	// Build token usage for debug (Ollama doesn't provide counts)
	var tokenUsage *TokenUsage
	if c.debug {
		tokenUsage = &TokenUsage{
			Provider: "ollama",
		}
	}

	// Check for native tool calls first
	if len(ollamaResp.Message.ToolCalls) > 0 {
		duration := time.Since(startTime)
		embedding.LogLLMResponseTrace("ollama", c.model, operation, resp.StatusCode, "tool_use")
		embedding.LogLLMCall("ollama", c.model, operation, 0, 0, duration, nil)

		if c.debug {
			fmt.Fprintf(os.Stderr, "\r\n[LLM] [DEBUG] Ollama - Response: tool_use (native tool calling, Ollama does not provide token counts)\n")
		}

		msgIndex := len(messages)
		var toolUses []interface{}
		for i, tc := range ollamaResp.Message.ToolCalls {
			toolUses = append(toolUses, ToolUse{
				Type:  "tool_use",
				ID:    fmt.Sprintf("ollama-tool-%d-%d", msgIndex, i+1),
				Name:  tc.Function.Name,
				Input: tc.Function.Arguments,
			})
		}
		return LLMResponse{
			Content:    toolUses,
			StopReason: "tool_use",
			TokenUsage: tokenUsage,
		}, nil
	}

	// Fallback: try to parse text content as a tool call JSON
	// This handles models that don't support native tool calling
	fallbackID := fmt.Sprintf("ollama-tool-%d-1", len(messages))
	textContent := ollamaResp.Message.Content

	// First try direct parsing (if the model responded with raw JSON)
	var toolCall toolCallRequest
	if err := json.Unmarshal([]byte(strings.TrimSpace(textContent)), &toolCall); err == nil && toolCall.Tool != "" {
		duration := time.Since(startTime)
		embedding.LogLLMResponseTrace("ollama", c.model, operation, resp.StatusCode, "tool_use")
		embedding.LogLLMCall("ollama", c.model, operation, 0, 0, duration, nil)

		if c.debug {
			fmt.Fprintf(os.Stderr, "\r\n[LLM] [DEBUG] Ollama - Response: tool_use (text fallback, Ollama does not provide token counts)\n")
		}

		return LLMResponse{
			Content: []interface{}{
				ToolUse{
					Type:  "tool_use",
					ID:    fallbackID,
					Name:  toolCall.Tool,
					Input: toolCall.Arguments,
				},
			},
			StopReason: "tool_use",
			TokenUsage: tokenUsage,
		}, nil
	}

	// If direct parsing failed, try to extract JSON from surrounding text
	// This handles cases where the model adds explanation around the JSON
	if extractedJSON := extractJSONFromText(textContent); extractedJSON != "" {
		if err := json.Unmarshal([]byte(extractedJSON), &toolCall); err == nil && toolCall.Tool != "" {
			duration := time.Since(startTime)
			embedding.LogLLMResponseTrace("ollama", c.model, operation, resp.StatusCode, "tool_use")
			embedding.LogLLMCall("ollama", c.model, operation, 0, 0, duration, nil)

			if c.debug {
				fmt.Fprintf(os.Stderr, "\r\n[LLM] [DEBUG] Ollama - Response: tool_use (text extraction fallback, Ollama does not provide token counts)\n")
			}

			return LLMResponse{
				Content: []interface{}{
					ToolUse{
						Type:  "tool_use",
						ID:    fallbackID,
						Name:  toolCall.Tool,
						Input: toolCall.Arguments,
					},
				},
				StopReason: "tool_use",
				TokenUsage: tokenUsage,
			}, nil
		}
	}

	// It's a text response
	duration := time.Since(startTime)
	embedding.LogLLMResponseTrace("ollama", c.model, operation, resp.StatusCode, "end_turn")
	embedding.LogLLMCall("ollama", c.model, operation, 0, 0, duration, nil)

	if c.debug {
		fmt.Fprintf(os.Stderr, "\r\n[LLM] [DEBUG] Ollama - Response: end_turn (Ollama does not provide token counts)\n")
	}

	return LLMResponse{
		Content: []interface{}{
			TextContent{
				Type: "text",
				Text: textContent,
			},
		},
		StopReason: "end_turn",
		TokenUsage: tokenUsage,
	}, nil
}

// ListModels returns available models from the Ollama server
func (c *ollamaClient) ListModels(ctx context.Context) ([]string, error) {
	url := c.baseURL + "/api/tags"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for k, v := range c.customHeaders {
		req.Header.Set(k, v)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body) //nolint:errcheck // Error response body read is best effort
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	// Parse response: {"models": [{"name": "llama3", ...}, ...]}
	var response struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	models := make([]string, 0, len(response.Models))
	for _, model := range response.Models {
		models = append(models, model.Name)
	}

	return models, nil
}

// geminiClient implements LLMClient for Google Gemini models
type geminiClient struct {
	apiKey        string
	baseURL       string
	model         string
	maxTokens     int
	temperature   float64
	debug         bool
	readOnly      bool
	customHeaders map[string]string
	client        *http.Client
}

// SetReadOnlyMode sets whether the database is in read-only mode.
func (c *geminiClient) SetReadOnlyMode(readOnly bool) {
	c.readOnly = readOnly
}

// NewGeminiClient creates a new Gemini client
// baseURL can be empty to use the default (https://generativelanguage.googleapis.com)
func NewGeminiClient(apiKey, baseURL, model string, maxTokens int, temperature float64, customHeaders map[string]string, debug bool) (LLMClient, error) {
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com"
	} else {
		var err error
		baseURL, err = ValidateBaseURL(baseURL, "Gemini")
		if err != nil {
			return nil, err
		}
	}
	return &geminiClient{
		apiKey:        apiKey,
		baseURL:       baseURL,
		model:         model,
		maxTokens:     maxTokens,
		temperature:   temperature,
		customHeaders: customHeaders,
		debug:         debug,
		client:        &http.Client{},
	}, nil
}

type geminiRequest struct {
	Contents          []geminiContent         `json:"contents"`
	Tools             []geminiToolDeclaration `json:"tools,omitempty"`
	SystemInstruction *geminiContent          `json:"systemInstruction,omitempty"`
	GenerationConfig  *geminiGenerationConfig `json:"generationConfig,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text             string                  `json:"text,omitempty"`
	FunctionCall     *geminiFunctionCall     `json:"functionCall,omitempty"`
	FunctionResponse *geminiFunctionResponse `json:"functionResponse,omitempty"`
}

type geminiFunctionCall struct {
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args"`
}

type geminiFunctionResponse struct {
	Name     string                 `json:"name"`
	Response map[string]interface{} `json:"response"`
}

type geminiToolDeclaration struct {
	FunctionDeclarations []geminiFunction `json:"functionDeclarations"`
}

type geminiFunction struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters,omitempty"`
}

type geminiGenerationConfig struct {
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
	Temperature     float64 `json:"temperature,omitempty"`
}

type geminiResponse struct {
	Candidates    []geminiCandidate    `json:"candidates"`
	UsageMetadata *geminiUsageMetadata `json:"usageMetadata,omitempty"`
}

type geminiCandidate struct {
	Content      geminiContent `json:"content"`
	FinishReason string        `json:"finishReason"`
}

type geminiUsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

type geminiErrorResponse struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

// extractGeminiErrorMessage parses Gemini's error response to get a user-friendly message
func extractGeminiErrorMessage(statusCode int, body []byte) string {
	var errResp geminiErrorResponse
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error.Message != "" {
		return fmt.Sprintf("API error (%d): %s", statusCode, errResp.Error.Message)
	}
	return fmt.Sprintf("API error (%d): %s", statusCode, string(body))
}

// generateToolID creates a unique identifier for tool calls
func generateToolID() string {
	b := make([]byte, 12)
	_, err := crand.Read(b)
	if err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func (c *geminiClient) Chat(ctx context.Context, messages []Message, tools interface{}) (LLMResponse, error) {
	startTime := time.Now()
	operation := "chat"
	requestURL := c.baseURL + "/v1beta/models/" + c.model + ":generateContent?key=" + c.apiKey

	// Log the URL without the API key for security
	safeURL := c.baseURL + "/v1beta/models/" + c.model + ":generateContent?key=***"
	embedding.LogLLMCallDetails("gemini", c.model, operation, safeURL, len(messages))

	// Convert interface{} tools to []mcp.Tool via JSON
	var mcpTools []mcp.Tool
	if tools != nil {
		toolsJSON, err := json.Marshal(tools)
		if err != nil {
			return LLMResponse{}, fmt.Errorf("failed to marshal tools: %w", err)
		}
		if err := json.Unmarshal(toolsJSON, &mcpTools); err != nil {
			return LLMResponse{}, fmt.Errorf("failed to unmarshal tools: %w", err)
		}
	}

	// Convert MCP tools to Gemini's functionDeclarations format
	var geminiTools []geminiToolDeclaration
	if len(mcpTools) > 0 {
		var functions []geminiFunction
		for _, tool := range mcpTools {
			functions = append(functions, geminiFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.InputSchema,
			})
		}
		geminiTools = []geminiToolDeclaration{
			{FunctionDeclarations: functions},
		}
	}

	// Build system instruction
	systemContent := `You are a helpful PostgreSQL database assistant with expert knowledge on PostgreSQL and products from pgEdge with access to MCP tools.

When executing tools:
- Be concise and direct
- Show results without explaining your methodology unless specifically asked
- Base responses ONLY on actual tool results - never make up or guess data
- Format results clearly for the user
- Only use tools when necessary to answer the question`

	if c.readOnly {
		systemContent += readOnlySafetyPrompt
	}

	systemInstruction := &geminiContent{
		Parts: []geminiPart{
			{Text: systemContent},
		},
	}

	// Build a map of tool use IDs to tool names for correlating
	// tool results with their original tool calls.
	toolNameByID := make(map[string]string)
	for _, msg := range messages {
		if items, ok := msg.Content.([]interface{}); ok && msg.Role == "assistant" {
			for _, item := range items {
				switch v := item.(type) {
				case ToolUse:
					toolNameByID[v.ID] = v.Name
				default:
					if itemMap, ok := item.(map[string]interface{}); ok {
						if itemType, ok2 := itemMap["type"].(string); ok2 && itemType == "tool_use" {
							if id, ok1 := itemMap["id"].(string); ok1 {
								if name, ok2 := itemMap["name"].(string); ok2 {
									toolNameByID[id] = name
								}
							}
						}
					}
				}
			}
		}
	}

	// Convert messages to Gemini format
	var contents []geminiContent
	for _, msg := range messages {
		switch content := msg.Content.(type) {
		case string:
			role := msg.Role
			if role == "assistant" {
				role = "model"
			}
			contents = append(contents, geminiContent{
				Role:  role,
				Parts: []geminiPart{{Text: content}},
			})
		case []ToolResult:
			// Handle []ToolResult directly (from client.go tool execution)
			var parts []geminiPart
			for _, v := range content {
				contentStr := toolResultContentString(v.Content)
				if contentStr == "" {
					contentStr = "{}"
				}
				toolName := toolNameByID[v.ToolUseID]
				parts = append(parts, geminiPart{
					FunctionResponse: &geminiFunctionResponse{
						Name: toolName,
						Response: map[string]interface{}{
							"result": contentStr,
						},
					},
				})
			}
			if len(parts) > 0 {
				contents = append(contents, geminiContent{
					Role:  "user",
					Parts: parts,
				})
			}
		case []interface{}:
			if msg.Role == "assistant" {
				// Handle assistant messages with text and tool_use
				var parts []geminiPart
				for _, item := range content {
					switch v := item.(type) {
					case TextContent:
						parts = append(parts, geminiPart{Text: v.Text})
					case ToolUse:
						parts = append(parts, geminiPart{
							FunctionCall: &geminiFunctionCall{
								Name: v.Name,
								Args: v.Input,
							},
						})
					default:
						itemMap, ok := item.(map[string]interface{})
						if !ok {
							continue
						}
						itemType, ok2 := itemMap["type"].(string)
						if !ok2 {
							continue
						}
						switch itemType {
						case "text":
							if text, ok := itemMap["text"].(string); ok {
								parts = append(parts, geminiPart{Text: text})
							}
						case "tool_use":
							name, ok1 := itemMap["name"].(string)
							if !ok1 {
								continue
							}
							input, ok2 := itemMap["input"].(map[string]interface{})
							if !ok2 || input == nil {
								input = map[string]interface{}{}
							}
							parts = append(parts, geminiPart{
								FunctionCall: &geminiFunctionCall{
									Name: name,
									Args: input,
								},
							})
						}
					}
				}
				if len(parts) > 0 {
					contents = append(contents, geminiContent{
						Role:  "model",
						Parts: parts,
					})
				}
			} else {
				// Handle user messages with tool_result content
				var parts []geminiPart
				for _, item := range content {
					switch v := item.(type) {
					case ToolResult:
						contentStr := toolResultContentString(v.Content)
						if contentStr == "" {
							contentStr = "{}"
						}
						toolName := toolNameByID[v.ToolUseID]
						parts = append(parts, geminiPart{
							FunctionResponse: &geminiFunctionResponse{
								Name: toolName,
								Response: map[string]interface{}{
									"result": contentStr,
								},
							},
						})
					default:
						itemMap, ok := item.(map[string]interface{})
						if !ok {
							continue
						}
						itemType, ok2 := itemMap["type"].(string)
						if !ok2 {
							continue
						}
						if itemType == "tool_result" {
							toolUseID, ok := itemMap["tool_use_id"].(string)
							if !ok {
								continue
							}
							resultContent := itemMap["content"]
							contentStr := extractTextFromContent(resultContent)
							if contentStr == "" {
								contentStr = "{}"
							}
							toolName := toolNameByID[toolUseID]
							parts = append(parts, geminiPart{
								FunctionResponse: &geminiFunctionResponse{
									Name: toolName,
									Response: map[string]interface{}{
										"result": contentStr,
									},
								},
							})
						}
					}
				}
				if len(parts) > 0 {
					contents = append(contents, geminiContent{
						Role:  "user",
						Parts: parts,
					})
				}
			}
		}
	}

	// Build request
	geminiReq := geminiRequest{
		Contents:          contents,
		Tools:             geminiTools,
		SystemInstruction: systemInstruction,
		GenerationConfig: &geminiGenerationConfig{
			MaxOutputTokens: c.maxTokens,
			Temperature:     c.temperature,
		},
	}

	reqData, err := json.Marshal(geminiReq)
	if err != nil {
		return LLMResponse{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	embedding.LogLLMRequestTrace("gemini", c.model, operation, string(reqData))

	httpReq, err := http.NewRequestWithContext(ctx, "POST", requestURL, bytes.NewBuffer(reqData))
	if err != nil {
		return LLMResponse{}, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	for k, v := range c.customHeaders {
		httpReq.Header.Set(k, v)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		embedding.LogConnectionError("gemini", safeURL, err)
		duration := time.Since(startTime)
		embedding.LogLLMCall("gemini", c.model, operation, 0, 0, duration, err)
		return LLMResponse{}, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			duration := time.Since(startTime)
			readErr := fmt.Errorf("API error %d (failed to read body: %w)", resp.StatusCode, err)
			embedding.LogLLMCall("gemini", c.model, operation, 0, 0, duration, readErr)
			return LLMResponse{}, readErr
		}

		// Check if this is a rate limit error
		if resp.StatusCode == 429 {
			embedding.LogRateLimitError("gemini", c.model, resp.StatusCode, string(body))
		}

		// Extract user-friendly error message from Gemini's error response
		userFriendlyMsg := extractGeminiErrorMessage(resp.StatusCode, body)

		duration := time.Since(startTime)
		apiErr := fmt.Errorf("%s", userFriendlyMsg)
		embedding.LogLLMCall("gemini", c.model, operation, 0, 0, duration, apiErr)
		return LLMResponse{}, apiErr
	}

	var geminiResp geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
		duration := time.Since(startTime)
		embedding.LogLLMCall("gemini", c.model, operation, 0, 0, duration, err)
		return LLMResponse{}, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(geminiResp.Candidates) == 0 {
		duration := time.Since(startTime)
		err := fmt.Errorf("no candidates in response")
		embedding.LogLLMCall("gemini", c.model, operation, 0, 0, duration, err)
		return LLMResponse{}, err
	}

	candidate := geminiResp.Candidates[0]

	// Convert response content to typed structs
	var responseContent []interface{}
	hasToolUse := false
	for _, part := range candidate.Content.Parts {
		if part.FunctionCall != nil {
			hasToolUse = true
			responseContent = append(responseContent, ToolUse{
				Type:  "tool_use",
				ID:    generateToolID(),
				Name:  part.FunctionCall.Name,
				Input: part.FunctionCall.Args,
			})
		} else if part.Text != "" {
			responseContent = append(responseContent, TextContent{
				Type: "text",
				Text: part.Text,
			})
		}
	}

	// Determine stop reason
	stopReason := "end_turn"
	if hasToolUse {
		stopReason = "tool_use"
	} else {
		switch candidate.FinishReason {
		case "STOP":
			stopReason = "end_turn"
		case "MAX_TOKENS":
			stopReason = "max_tokens"
		}
	}

	// Extract token usage
	promptTokens := 0
	completionTokens := 0
	totalTokens := 0
	if geminiResp.UsageMetadata != nil {
		promptTokens = geminiResp.UsageMetadata.PromptTokenCount
		completionTokens = geminiResp.UsageMetadata.CandidatesTokenCount
		totalTokens = geminiResp.UsageMetadata.TotalTokenCount
	}

	duration := time.Since(startTime)
	embedding.LogLLMResponseTrace("gemini", c.model, operation, resp.StatusCode, stopReason)
	embedding.LogLLMCall("gemini", c.model, operation, promptTokens, completionTokens, duration, nil)

	// Build token usage for debug
	var tokenUsage *TokenUsage
	if c.debug {
		tokenUsage = &TokenUsage{
			Provider:         "gemini",
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      totalTokens,
		}

		// Log to stderr for CLI
		fmt.Fprintf(os.Stderr, "\r\n[LLM] [DEBUG] Gemini - Tokens: Prompt %d, Completion %d, Total %d\n",
			promptTokens,
			completionTokens,
			totalTokens,
		)
	}

	return LLMResponse{
		Content:    responseContent,
		StopReason: stopReason,
		TokenUsage: tokenUsage,
	}, nil
}

// ListModels returns available models from Gemini
// Filters to models that support generateContent
func (c *geminiClient) ListModels(ctx context.Context) ([]string, error) {
	requestURL := c.baseURL + "/v1beta/models?key=" + c.apiKey

	req, err := http.NewRequestWithContext(ctx, "GET", requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for k, v := range c.customHeaders {
		req.Header.Set(k, v)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body) //nolint:errcheck // Error response body read is best effort
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	// Parse response: {"models": [{"name": "models/gemini-pro", "supportedGenerationMethods": [...], ...}, ...]}
	var response struct {
		Models []struct {
			Name                       string   `json:"name"`
			SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	models := make([]string, 0, len(response.Models))
	for _, model := range response.Models {
		// Only include models that support generateContent
		supportsGenerate := false
		for _, method := range model.SupportedGenerationMethods {
			if method == "generateContent" {
				supportsGenerate = true
				break
			}
		}
		if !supportsGenerate {
			continue
		}

		// Strip "models/" prefix from model names
		name := model.Name
		name = strings.TrimPrefix(name, "models/")
		models = append(models, name)
	}

	return models, nil
}

// openaiClient implements LLMClient for OpenAI GPT models
type openaiClient struct {
	apiKey        string
	baseURL       string
	model         string
	maxTokens     int
	temperature   float64
	debug         bool
	readOnly      bool
	customHeaders map[string]string
	client        *http.Client
}

// SetReadOnlyMode sets whether the database is in read-only mode.
func (c *openaiClient) SetReadOnlyMode(readOnly bool) {
	c.readOnly = readOnly
}

// NewOpenAIClient creates a new OpenAI client
// baseURL can be empty to use the default (https://api.openai.com)
func NewOpenAIClient(apiKey, baseURL, model string, maxTokens int, temperature float64, customHeaders map[string]string, debug bool) (LLMClient, error) {
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	} else {
		var err error
		baseURL, err = ValidateBaseURL(baseURL, "OpenAI")
		if err != nil {
			return nil, err
		}
	}
	return &openaiClient{
		apiKey:        apiKey,
		baseURL:       baseURL,
		model:         model,
		maxTokens:     maxTokens,
		temperature:   temperature,
		customHeaders: customHeaders,
		debug:         debug,
		client:        &http.Client{},
	}, nil
}

type openaiMessage struct {
	Role       string      `json:"role"`
	Content    interface{} `json:"content,omitempty"`
	ToolCalls  interface{} `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
}

type openaiRequest struct {
	Model               string          `json:"model"`
	Messages            []openaiMessage `json:"messages"`
	Tools               interface{}     `json:"tools,omitempty"`
	MaxTokens           int             `json:"max_tokens,omitempty"`
	MaxCompletionTokens int             `json:"max_completion_tokens,omitempty"`
	Temperature         float64         `json:"temperature,omitempty"`
}

type openaiUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type openaiChoice struct {
	Index        int           `json:"index"`
	Message      openaiMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

type openaiResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []openaiChoice `json:"choices"`
	Usage   openaiUsage    `json:"usage"`
}

type openaiErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// extractOpenAIErrorMessage parses OpenAI's error response to get a user-friendly message
func extractOpenAIErrorMessage(statusCode int, body []byte) string {
	var errResp openaiErrorResponse
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error.Message != "" {
		// Successfully parsed error response, return the message
		return fmt.Sprintf("API error (%d): %s", statusCode, errResp.Error.Message)
	}
	// Fallback to raw body if parsing fails
	return fmt.Sprintf("API error (%d): %s", statusCode, string(body))
}

// toolResultContentString extracts a string from a ToolResult's
// typed Content field (string, []mcp.ContentItem, or other).
func toolResultContentString(content interface{}) string {
	switch c := content.(type) {
	case string:
		return c
	case []mcp.ContentItem:
		var texts []string
		for _, ci := range c {
			texts = append(texts, ci.Text)
		}
		return strings.Join(texts, "\n")
	default:
		data, err := json.Marshal(c)
		if err != nil {
			return fmt.Sprintf("%v", c)
		}
		return string(data)
	}
}

// extractTextFromContent extracts text from tool result content
// Content can be: string, []byte, array of text blocks, or other structures
func extractTextFromContent(content interface{}) string {
	switch c := content.(type) {
	case string:
		return c
	case []byte:
		return string(c)
	case []interface{}:
		// Content is an array of blocks - extract text from each
		var texts []string
		for _, block := range c {
			if blockMap, ok := block.(map[string]interface{}); ok {
				if blockType, ok := blockMap["type"].(string); ok && blockType == "text" {
					if text, ok := blockMap["text"].(string); ok {
						texts = append(texts, text)
					}
				}
			}
		}
		if len(texts) > 0 {
			return strings.Join(texts, "\n")
		}
	}
	// Default: serialize to JSON
	if jsonBytes, err := json.Marshal(content); err == nil {
		return string(jsonBytes)
	}
	return fmt.Sprintf("%v", content)
}

func (c *openaiClient) Chat(ctx context.Context, messages []Message, tools interface{}) (LLMResponse, error) {
	startTime := time.Now()
	operation := "chat"
	url := c.baseURL + "/v1/chat/completions"

	embedding.LogLLMCallDetails("openai", c.model, operation, url, len(messages))

	// Convert interface{} tools to []mcp.Tool via JSON
	var mcpTools []mcp.Tool
	if tools != nil {
		toolsJSON, err := json.Marshal(tools)
		if err != nil {
			return LLMResponse{}, fmt.Errorf("failed to marshal tools: %w", err)
		}
		if err := json.Unmarshal(toolsJSON, &mcpTools); err != nil {
			return LLMResponse{}, fmt.Errorf("failed to unmarshal tools: %w", err)
		}
	}

	// Convert MCP tools to OpenAI format
	var openaiTools []map[string]interface{}
	if len(mcpTools) > 0 {
		for _, tool := range mcpTools {
			openaiTools = append(openaiTools, map[string]interface{}{
				"type": "function",
				"function": map[string]interface{}{
					"name":        tool.Name,
					"description": tool.Description,
					"parameters":  tool.InputSchema,
				},
			})
		}
	}

	// Convert messages to OpenAI format
	// Start with system message
	systemContent := `You are a helpful PostgreSQL database assistant with expert knowledge on PostgreSQL and products from pgEdge with access to MCP tools.

When executing tools:
- Be concise and direct
- Show results without explaining your methodology unless specifically asked
- Base responses ONLY on actual tool results - never make up or guess data
- Format results clearly for the user
- Only use tools when necessary to answer the question`

	if c.readOnly {
		systemContent += readOnlySafetyPrompt
	}

	openaiMessages := make([]openaiMessage, 0, len(messages)+1)
	openaiMessages = append(openaiMessages, openaiMessage{
		Role:    "system",
		Content: systemContent,
	})

	for _, msg := range messages {
		openaiMsg := openaiMessage{
			Role: msg.Role,
		}

		// Handle different content types
		switch content := msg.Content.(type) {
		case string:
			openaiMsg.Content = content
		case []ToolResult:
			// Handle []ToolResult directly
			for _, v := range content {
				contentStr := extractTextFromContent(v.Content)
				if contentStr == "" {
					contentStr = "{}"
				}
				openaiMessages = append(openaiMessages, openaiMessage{
					Role:       "tool",
					Content:    contentStr,
					ToolCallID: v.ToolUseID,
				})
			}
			// Don't add the parent message
			continue
		case []interface{}:
			// Handle complex content (text, tool use, and tool results)
			var toolCalls []map[string]interface{}
			for _, item := range content {
				// Handle typed structs (when messages are passed directly)
				switch v := item.(type) {
				case TextContent:
					openaiMsg.Content = v.Text
				case ToolUse:
					// Convert ToolUse to OpenAI tool_calls format
					argsJSON, err := json.Marshal(v.Input)
					if err != nil {
						argsJSON = []byte("{}")
					}
					toolCalls = append(toolCalls, map[string]interface{}{
						"id":   v.ID,
						"type": "function",
						"function": map[string]interface{}{
							"name":      v.Name,
							"arguments": string(argsJSON),
						},
					})
				case ToolResult:
					// ToolResult - send as separate tool message
					// Extract text from result content
					contentStr := extractTextFromContent(v.Content)
					if contentStr == "" {
						contentStr = "{}"
					}

					openaiMessages = append(openaiMessages, openaiMessage{
						Role:       "tool",
						Content:    contentStr,
						ToolCallID: v.ToolUseID,
					})
				default:
					// Handle map[string]interface{} (when items are unmarshaled from JSON)
					itemMap, ok := item.(map[string]interface{})
					if !ok {
						continue
					}

					itemType, ok := itemMap["type"].(string)
					if !ok {
						continue
					}
					switch itemType {
					case "text":
						// TextContent
						if text, ok := itemMap["text"].(string); ok {
							openaiMsg.Content = text
						}
					case "tool_use":
						// ToolUse - convert to OpenAI tool_calls format
						id, ok1 := itemMap["id"].(string)
						name, ok2 := itemMap["name"].(string)
						input, ok3 := itemMap["input"].(map[string]interface{})
						if !ok1 || !ok2 || !ok3 {
							continue
						}

						argsJSON, err := json.Marshal(input)
						if err != nil {
							argsJSON = []byte("{}")
						}
						toolCalls = append(toolCalls, map[string]interface{}{
							"id":   id,
							"type": "function",
							"function": map[string]interface{}{
								"name":      name,
								"arguments": string(argsJSON),
							},
						})
					case "tool_result":
						// ToolResult - send as separate tool message
						toolUseID, ok := itemMap["tool_use_id"].(string)
						if !ok {
							continue
						}
						resultContent := itemMap["content"]

						// Extract text from result content
						contentStr := extractTextFromContent(resultContent)
						if contentStr == "" {
							contentStr = "{}"
						}

						openaiMessages = append(openaiMessages, openaiMessage{
							Role:       "tool",
							Content:    contentStr,
							ToolCallID: toolUseID,
						})
					}
				}
			}
			// If we found tool calls, set them on the message
			if len(toolCalls) > 0 {
				openaiMsg.ToolCalls = toolCalls
			}
		}

		// Only add the message if it has content or tool calls
		// Skip empty assistant messages (shouldn't happen, but be safe)
		if openaiMsg.Content != nil || openaiMsg.ToolCalls != nil {
			openaiMessages = append(openaiMessages, openaiMsg)
		}
	}

	// Build request
	reqData := openaiRequest{
		Model:    c.model,
		Messages: openaiMessages,
	}

	// Use max_completion_tokens for newer models (gpt-5, o1-*, etc.)
	// Use max_tokens for older models (gpt-4, gpt-3.5, etc.)
	// GPT-5 and o-series models don't support custom temperature (only default of 1)
	isNewModel := strings.HasPrefix(c.model, "gpt-5") || strings.HasPrefix(c.model, "o1-") || strings.HasPrefix(c.model, "o3-")

	if isNewModel {
		reqData.MaxCompletionTokens = c.maxTokens
		// GPT-5 only supports temperature=1 (default), so don't set it
	} else {
		reqData.MaxTokens = c.maxTokens
		reqData.Temperature = c.temperature
	}

	if len(openaiTools) > 0 {
		reqData.Tools = openaiTools
	}

	reqJSON, err := json.Marshal(reqData)
	if err != nil {
		duration := time.Since(startTime)
		embedding.LogLLMCall("openai", c.model, operation, 0, 0, duration, err)
		return LLMResponse{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	embedding.LogLLMRequestTrace("openai", c.model, operation, string(reqJSON))

	// Make request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqJSON))
	if err != nil {
		duration := time.Since(startTime)
		embedding.LogLLMCall("openai", c.model, operation, 0, 0, duration, err)
		return LLMResponse{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	for k, v := range c.customHeaders {
		req.Header.Set(k, v)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		duration := time.Since(startTime)
		embedding.LogConnectionError("openai", url, err)
		embedding.LogLLMCall("openai", c.model, operation, 0, 0, duration, err)
		return LLMResponse{}, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		duration := time.Since(startTime)
		readErr := fmt.Errorf("failed to read response body: %w", err)
		embedding.LogLLMCall("openai", c.model, operation, 0, 0, duration, readErr)
		return LLMResponse{}, readErr
	}

	// Check for errors
	if resp.StatusCode != http.StatusOK {
		// Check if this is a rate limit error
		if resp.StatusCode == 429 {
			embedding.LogRateLimitError("openai", c.model, resp.StatusCode, string(body))
		}

		// Extract user-friendly error message from OpenAI's error response
		userFriendlyMsg := extractOpenAIErrorMessage(resp.StatusCode, body)

		duration := time.Since(startTime)
		apiErr := fmt.Errorf("%s", userFriendlyMsg)
		embedding.LogLLMCall("openai", c.model, operation, 0, 0, duration, apiErr)
		return LLMResponse{}, apiErr
	}

	var openaiResp openaiResponse
	if err := json.Unmarshal(body, &openaiResp); err != nil {
		duration := time.Since(startTime)
		embedding.LogLLMCall("openai", c.model, operation, 0, 0, duration, err)
		return LLMResponse{}, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(openaiResp.Choices) == 0 {
		duration := time.Since(startTime)
		err := fmt.Errorf("no choices in response")
		embedding.LogLLMCall("openai", c.model, operation, 0, 0, duration, err)
		return LLMResponse{}, err
	}

	choice := openaiResp.Choices[0]
	duration := time.Since(startTime)

	// Check if there are tool calls
	if choice.Message.ToolCalls != nil {
		toolCalls, ok := choice.Message.ToolCalls.([]interface{})
		if ok && len(toolCalls) > 0 {
			embedding.LogLLMResponseTrace("openai", c.model, operation, resp.StatusCode, "tool_calls")
			embedding.LogLLMCall("openai", c.model, operation, openaiResp.Usage.PromptTokens, openaiResp.Usage.CompletionTokens, duration, nil)

			// Build token usage for debug
			var tokenUsage *TokenUsage
			if c.debug {
				tokenUsage = &TokenUsage{
					Provider:         "openai",
					PromptTokens:     openaiResp.Usage.PromptTokens,
					CompletionTokens: openaiResp.Usage.CompletionTokens,
					TotalTokens:      openaiResp.Usage.TotalTokens,
				}

				// Log to stderr for CLI
				fmt.Fprintf(os.Stderr, "\r\n[LLM] [DEBUG] OpenAI - Tokens: Prompt %d, Completion %d, Total %d\n",
					openaiResp.Usage.PromptTokens,
					openaiResp.Usage.CompletionTokens,
					openaiResp.Usage.TotalTokens,
				)
			}

			// Convert tool calls to our format
			content := make([]interface{}, 0, len(toolCalls))
			for _, tc := range toolCalls {
				toolCall, ok := tc.(map[string]interface{})
				if !ok {
					continue
				}

				function, ok := toolCall["function"].(map[string]interface{})
				if !ok {
					continue
				}

				name, ok := function["name"].(string)
				if !ok {
					continue
				}
				argsStr, ok := function["arguments"].(string)
				if !ok {
					continue
				}

				var args map[string]interface{}
				if err := json.Unmarshal([]byte(argsStr), &args); err != nil {
					args = map[string]interface{}{}
				}

				id, ok := toolCall["id"].(string)
				if !ok {
					continue
				}

				content = append(content, ToolUse{
					Type:  "tool_use",
					ID:    id,
					Name:  name,
					Input: args,
				})
			}

			return LLMResponse{
				Content:    content,
				StopReason: "tool_use",
				TokenUsage: tokenUsage,
			}, nil
		}
	}

	// It's a text response
	messageContent := ""
	if choice.Message.Content != nil {
		if contentStr, ok := choice.Message.Content.(string); ok {
			messageContent = contentStr
		}
	}

	embedding.LogLLMResponseTrace("openai", c.model, operation, resp.StatusCode, choice.FinishReason)
	embedding.LogLLMCall("openai", c.model, operation, openaiResp.Usage.PromptTokens, openaiResp.Usage.CompletionTokens, duration, nil)

	// Build token usage for debug
	var tokenUsage *TokenUsage
	if c.debug {
		tokenUsage = &TokenUsage{
			Provider:         "openai",
			PromptTokens:     openaiResp.Usage.PromptTokens,
			CompletionTokens: openaiResp.Usage.CompletionTokens,
			TotalTokens:      openaiResp.Usage.TotalTokens,
		}

		// Log to stderr for CLI
		fmt.Fprintf(os.Stderr, "\r\n[LLM] [DEBUG] OpenAI - Tokens: Prompt %d, Completion %d, Total %d\n",
			openaiResp.Usage.PromptTokens,
			openaiResp.Usage.CompletionTokens,
			openaiResp.Usage.TotalTokens,
		)
	}

	return LLMResponse{
		Content: []interface{}{
			TextContent{
				Type: "text",
				Text: messageContent,
			},
		},
		StopReason: "end_turn",
		TokenUsage: tokenUsage,
	}, nil
}

// ListModels returns available models from OpenAI
// Filters out embedding, audio, and image models
func (c *openaiClient) ListModels(ctx context.Context) ([]string, error) {
	url := c.baseURL + "/v1/models"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	for k, v := range c.customHeaders {
		req.Header.Set(k, v)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body) //nolint:errcheck // Error response body read is best effort
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	// Parse response: {"data": [{"id": "gpt-5-main", ...}, ...]}
	var response struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	models := make([]string, 0, len(response.Data))
	for _, model := range response.Data {
		id := model.ID

		// Exclude embedding models
		if strings.Contains(id, "embedding") {
			continue
		}

		// Exclude audio/speech models
		if strings.Contains(id, "whisper") ||
			strings.Contains(id, "tts") ||
			strings.Contains(id, "audio") {
			continue
		}

		// Exclude image models
		if strings.Contains(id, "dall-e") {
			continue
		}

		// Include only chat-capable models (gpt-*, o1-*, o3-*)
		if strings.Contains(id, "gpt") ||
			strings.HasPrefix(id, "o1-") ||
			strings.HasPrefix(id, "o3-") {
			models = append(models, id)
		}
	}

	return models, nil
}
