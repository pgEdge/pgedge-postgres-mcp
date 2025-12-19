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
type LLMClient interface {
	// Chat sends messages and available tools to the LLM and returns the response
	Chat(ctx context.Context, messages []Message, tools interface{}) (LLMResponse, error)

	// ListModels returns a list of available models from the provider
	ListModels(ctx context.Context) ([]string, error)
}

// anthropicClient implements LLMClient for Anthropic Claude
type anthropicClient struct {
	apiKey      string
	model       string
	maxTokens   int
	temperature float64
	debug       bool
	client      *http.Client
}

// NewAnthropicClient creates a new Anthropic client
func NewAnthropicClient(apiKey, model string, maxTokens int, temperature float64, debug bool) LLMClient {
	return &anthropicClient{
		apiKey:      apiKey,
		model:       model,
		maxTokens:   maxTokens,
		temperature: temperature,
		debug:       debug,
		client:      &http.Client{},
	}
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
	url := "https://api.anthropic.com/v1/messages"

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
	url := "https://api.anthropic.com/v1/models"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

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
	baseURL string
	model   string
	debug   bool
	client  *http.Client
}

// NewOllamaClient creates a new Ollama client
func NewOllamaClient(baseURL, model string, debug bool) LLMClient {
	return &ollamaClient{
		baseURL: baseURL,
		model:   model,
		debug:   debug,
		client:  &http.Client{},
	}
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
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

	// Format tools for Ollama
	toolsContext := c.formatToolsForOllama(mcpTools)

	// Create system message with tool information
	systemMessage := fmt.Sprintf(`You are a helpful PostgreSQL database assistant with expert knowledge on PostgreSQL and products from pgEdge. You have access to the following tools:

%s

IMPORTANT INSTRUCTIONS:
1. When you need to use a tool, respond with ONLY a JSON object - no other text before or after:
{
    "tool": "tool_name",
    "arguments": {
        "param1": "value1",
        "param2": "value2"
    }
}

2. After calling a tool, you will receive actual results from the database.
3. You MUST base your response ONLY on the actual tool results provided - never make up or guess data.
4. If you receive tool results, format them clearly for the user.
5. Only use tools when necessary to answer the user's question.
6. Be concise and direct - show results without explaining your methodology unless specifically asked.`, toolsContext)

	// Convert messages to Ollama format
	ollamaMessages := []ollamaMessage{
		{
			Role:    "system",
			Content: systemMessage,
		},
	}

	for _, msg := range messages {
		switch content := msg.Content.(type) {
		case string:
			ollamaMessages = append(ollamaMessages, ollamaMessage{
				Role:    msg.Role,
				Content: content,
			})
		case []interface{}:
			// Handle tool results
			var parts []string
			for _, item := range content {
				if tr, ok := item.(ToolResult); ok {
					contentStr := ""
					switch c := tr.Content.(type) {
					case string:
						contentStr = c
					case []mcp.ContentItem:
						var texts []string
						for _, ci := range c {
							texts = append(texts, ci.Text)
						}
						contentStr = strings.Join(texts, "\n")
					default:
						data, err := json.Marshal(c)
						if err != nil {
							contentStr = fmt.Sprintf("%v", c)
						} else {
							contentStr = string(data)
						}
					}
					parts = append(parts, fmt.Sprintf("Tool result:\n%s", contentStr))
				}
			}
			if len(parts) > 0 {
				ollamaMessages = append(ollamaMessages, ollamaMessage{
					Role:    msg.Role,
					Content: strings.Join(parts, "\n\n"),
				})
			}
		}
	}

	req := ollamaRequest{
		Model:    c.model,
		Messages: ollamaMessages,
		Stream:   false,
	}

	reqData, err := json.Marshal(req)
	if err != nil {
		return LLMResponse{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/chat", bytes.NewBuffer(reqData))
	if err != nil {
		return LLMResponse{}, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

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

	content := ollamaResp.Message.Content

	// Try to parse as tool call
	// First try direct parsing (if the model behaved correctly)
	var toolCall toolCallRequest
	if err := json.Unmarshal([]byte(strings.TrimSpace(content)), &toolCall); err == nil && toolCall.Tool != "" {
		// It's a tool call
		duration := time.Since(startTime)
		embedding.LogLLMResponseTrace("ollama", c.model, operation, resp.StatusCode, "tool_use")
		embedding.LogLLMCall("ollama", c.model, operation, 0, 0, duration, nil) // Ollama doesn't provide token counts

		// Build token usage for debug (Ollama doesn't provide counts)
		var tokenUsage *TokenUsage
		if c.debug {
			tokenUsage = &TokenUsage{
				Provider: "ollama",
			}

			// Log to stderr for CLI
			fmt.Fprintf(os.Stderr, "\r\n[LLM] [DEBUG] Ollama - Response: tool_use (Ollama does not provide token counts)\n")
		}

		return LLMResponse{
			Content: []interface{}{
				ToolUse{
					Type:  "tool_use",
					ID:    "ollama-tool-1", // Ollama doesn't provide IDs, so we generate one
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
	if extractedJSON := extractJSONFromText(content); extractedJSON != "" {
		if err := json.Unmarshal([]byte(extractedJSON), &toolCall); err == nil && toolCall.Tool != "" {
			// Successfully extracted and parsed tool call
			duration := time.Since(startTime)
			embedding.LogLLMResponseTrace("ollama", c.model, operation, resp.StatusCode, "tool_use")
			embedding.LogLLMCall("ollama", c.model, operation, 0, 0, duration, nil)

			// Build token usage for debug
			var tokenUsage *TokenUsage
			if c.debug {
				tokenUsage = &TokenUsage{
					Provider: "ollama",
				}

				// Log to stderr for CLI
				fmt.Fprintf(os.Stderr, "\r\n[LLM] [DEBUG] Ollama - Response: tool_use (Ollama does not provide token counts)\n")
			}

			return LLMResponse{
				Content: []interface{}{
					ToolUse{
						Type:  "tool_use",
						ID:    "ollama-tool-1",
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
	embedding.LogLLMCall("ollama", c.model, operation, 0, 0, duration, nil) // Ollama doesn't provide token counts

	// Build token usage for debug (Ollama doesn't provide counts)
	var tokenUsage *TokenUsage
	if c.debug {
		tokenUsage = &TokenUsage{
			Provider: "ollama",
		}

		// Log to stderr for CLI
		fmt.Fprintf(os.Stderr, "\r\n[LLM] [DEBUG] Ollama - Response: end_turn (Ollama does not provide token counts)\n")
	}

	return LLMResponse{
		Content: []interface{}{
			TextContent{
				Type: "text",
				Text: content,
			},
		},
		StopReason: "end_turn",
		TokenUsage: tokenUsage,
	}, nil
}

func (c *ollamaClient) formatToolsForOllama(tools []mcp.Tool) string {
	var toolDescriptions []string
	for _, tool := range tools {
		toolDesc := fmt.Sprintf("- %s: %s", tool.Name, tool.Description)

		// Add parameter info if available
		if len(tool.InputSchema.Properties) > 0 {
			var params []string
			for paramName, paramInfo := range tool.InputSchema.Properties {
				paramMap, ok := paramInfo.(map[string]interface{})
				if !ok {
					continue
				}
				paramType, _ := paramMap["type"].(string)        //nolint:errcheck // Optional field, default to empty
				paramDesc, _ := paramMap["description"].(string) //nolint:errcheck // Optional field, default to empty
				if paramType == "" {
					paramType = "any"
				}
				params = append(params, fmt.Sprintf("%s (%s): %s", paramName, paramType, paramDesc))
			}
			if len(params) > 0 {
				toolDesc += "\n  Parameters:\n    " + strings.Join(params, "\n    ")
			}
		}

		toolDescriptions = append(toolDescriptions, toolDesc)
	}

	return strings.Join(toolDescriptions, "\n")
}

// ListModels returns available models from the Ollama server
func (c *ollamaClient) ListModels(ctx context.Context) ([]string, error) {
	url := c.baseURL + "/api/tags"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
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

// openaiClient implements LLMClient for OpenAI GPT models
type openaiClient struct {
	apiKey      string
	model       string
	maxTokens   int
	temperature float64
	debug       bool
	client      *http.Client
}

// NewOpenAIClient creates a new OpenAI client
func NewOpenAIClient(apiKey, model string, maxTokens int, temperature float64, debug bool) LLMClient {
	return &openaiClient{
		apiKey:      apiKey,
		model:       model,
		maxTokens:   maxTokens,
		temperature: temperature,
		debug:       debug,
		client:      &http.Client{},
	}
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
	url := "https://api.openai.com/v1/chat/completions"

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
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

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
	url := "https://api.openai.com/v1/models"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)

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
