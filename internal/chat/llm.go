/*-------------------------------------------------------------------------
 *
 * LLM Client for Chat Client
 *
 * Copyright (c) 2025, pgEdge, Inc.
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
    "strings"

    "pgedge-postgres-mcp/internal/mcp"
)

// Message represents a chat message
type Message struct {
    Role    string      `json:"role"`
    Content interface{} `json:"content"`
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
}

// LLMClient provides a unified interface for different LLM providers
type LLMClient interface {
    // Chat sends messages and available tools to the LLM and returns the response
    Chat(ctx context.Context, messages []Message, tools []mcp.Tool) (LLMResponse, error)
}

// anthropicClient implements LLMClient for Anthropic Claude
type anthropicClient struct {
    apiKey      string
    model       string
    maxTokens   int
    temperature float64
    client      *http.Client
}

// NewAnthropicClient creates a new Anthropic client
func NewAnthropicClient(apiKey, model string, maxTokens int, temperature float64) LLMClient {
    return &anthropicClient{
        apiKey:      apiKey,
        model:       model,
        maxTokens:   maxTokens,
        temperature: temperature,
        client:      &http.Client{},
    }
}

type anthropicRequest struct {
    Model       string                   `json:"model"`
    MaxTokens   int                      `json:"max_tokens"`
    Messages    []Message                `json:"messages"`
    Tools       []map[string]interface{} `json:"tools,omitempty"`
    Temperature float64                  `json:"temperature,omitempty"`
}

type anthropicResponse struct {
    ID         string                   `json:"id"`
    Type       string                   `json:"type"`
    Role       string                   `json:"role"`
    Content    []map[string]interface{} `json:"content"`
    StopReason string                   `json:"stop_reason"`
}

func (c *anthropicClient) Chat(ctx context.Context, messages []Message, tools []mcp.Tool) (LLMResponse, error) {
    // Convert MCP tools to Anthropic format
    anthropicTools := make([]map[string]interface{}, 0, len(tools))
    for _, tool := range tools {
        anthropicTools = append(anthropicTools, map[string]interface{}{
            "name":         tool.Name,
            "description":  tool.Description,
            "input_schema": tool.InputSchema,
        })
    }

    req := anthropicRequest{
        Model:       c.model,
        MaxTokens:   c.maxTokens,
        Messages:    messages,
        Tools:       anthropicTools,
        Temperature: c.temperature,
    }

    reqData, err := json.Marshal(req)
    if err != nil {
        return LLMResponse{}, fmt.Errorf("failed to marshal request: %w", err)
    }

    httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(reqData))
    if err != nil {
        return LLMResponse{}, fmt.Errorf("failed to create request: %w", err)
    }

    httpReq.Header.Set("Content-Type", "application/json")
    httpReq.Header.Set("x-api-key", c.apiKey)
    httpReq.Header.Set("anthropic-version", "2023-06-01")

    resp, err := c.client.Do(httpReq)
    if err != nil {
        return LLMResponse{}, fmt.Errorf("failed to send request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, err := io.ReadAll(resp.Body)
        if err != nil {
            return LLMResponse{}, fmt.Errorf("API error %d (failed to read body: %w)", resp.StatusCode, err)
        }
        return LLMResponse{}, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
    }

    var anthropicResp anthropicResponse
    if err := json.NewDecoder(resp.Body).Decode(&anthropicResp); err != nil {
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

    return LLMResponse{
        Content:    content,
        StopReason: anthropicResp.StopReason,
    }, nil
}

// ollamaClient implements LLMClient for Ollama
type ollamaClient struct {
    baseURL string
    model   string
    client  *http.Client
}

// NewOllamaClient creates a new Ollama client
func NewOllamaClient(baseURL, model string) LLMClient {
    return &ollamaClient{
        baseURL: baseURL,
        model:   model,
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

func (c *ollamaClient) Chat(ctx context.Context, messages []Message, tools []mcp.Tool) (LLMResponse, error) {
    // Format tools for Ollama
    toolsContext := c.formatToolsForOllama(tools)

    // Create system message with tool information
    systemMessage := fmt.Sprintf(`You are a helpful PostgreSQL database assistant. You have access to the following tools:

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
5. Only use tools when necessary to answer the user's question.`, toolsContext)

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
        return LLMResponse{}, fmt.Errorf("failed to send request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, err := io.ReadAll(resp.Body)
        if err != nil {
            return LLMResponse{}, fmt.Errorf("API error %d (failed to read body: %w)", resp.StatusCode, err)
        }
        return LLMResponse{}, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
    }

    var ollamaResp ollamaResponse
    if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
        return LLMResponse{}, fmt.Errorf("failed to decode response: %w", err)
    }

    content := ollamaResp.Message.Content

    // Try to parse as tool call
    var toolCall toolCallRequest
    if err := json.Unmarshal([]byte(strings.TrimSpace(content)), &toolCall); err == nil && toolCall.Tool != "" {
        // It's a tool call
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
        }, nil
    }

    // It's a text response
    return LLMResponse{
        Content: []interface{}{
            TextContent{
                Type: "text",
                Text: content,
            },
        },
        StopReason: "end_turn",
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
                paramType, _ := paramMap["type"].(string)       //nolint:errcheck // Optional field, default to empty
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
