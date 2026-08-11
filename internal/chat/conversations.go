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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	llmlib "github.com/pgEdge/pgedge-go-llm-lib/llm"
)

// ConversationSummary provides a lightweight view for listing
type ConversationSummary struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	Connection string    `json:"connection,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	Preview    string    `json:"preview"`
}

// Conversation represents a stored conversation
type Conversation struct {
	ID         string           `json:"id"`
	Username   string           `json:"username"`
	Title      string           `json:"title"`
	Provider   string           `json:"provider,omitempty"`
	Model      string           `json:"model,omitempty"`
	Connection string           `json:"connection,omitempty"`
	Messages   []llmlib.Message `json:"messages"`
	CreatedAt  time.Time        `json:"created_at"`
	UpdatedAt  time.Time        `json:"updated_at"`
}

// UnmarshalJSON implements legacy-format migration for the Messages
// field. Conversations saved by versions before the chat-LLMClient
// removal stored each message's Content as either a plain string or a
// typed slice with a "tool_result" content shape that differed from
// the library's BlockToolResult layout. We accept both forms and
// produce a slice of library-typed messages.
func (c *Conversation) UnmarshalJSON(data []byte) error {
	type rawMessage struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	}
	var raw struct {
		ID         string       `json:"id"`
		Username   string       `json:"username"`
		Title      string       `json:"title"`
		Provider   string       `json:"provider,omitempty"`
		Model      string       `json:"model,omitempty"`
		Connection string       `json:"connection,omitempty"`
		Messages   []rawMessage `json:"messages"`
		CreatedAt  time.Time    `json:"created_at"`
		UpdatedAt  time.Time    `json:"updated_at"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	c.ID = raw.ID
	c.Username = raw.Username
	c.Title = raw.Title
	c.Provider = raw.Provider
	c.Model = raw.Model
	c.Connection = raw.Connection
	c.CreatedAt = raw.CreatedAt
	c.UpdatedAt = raw.UpdatedAt
	if len(raw.Messages) == 0 {
		c.Messages = nil
		return nil
	}
	msgs := make([]llmlib.Message, 0, len(raw.Messages))
	for i, m := range raw.Messages {
		blocks, role, err := decodeMessageContent(m.Role, m.Content)
		if err != nil {
			return fmt.Errorf("messages[%d]: %w", i, err)
		}
		msgs = append(msgs, llmlib.Message{Role: role, Content: blocks})
	}
	c.Messages = msgs
	return nil
}

// decodeMessageContent translates a raw on-disk message content into
// the library's typed []llmlib.ContentBlock form, accepting both the
// current shape and the pre-PR5 legacy shape.
func decodeMessageContent(role string, raw json.RawMessage) ([]llmlib.ContentBlock, llmlib.Role, error) {
	r := llmlib.Role(role)
	if len(raw) == 0 || string(raw) == "null" {
		return nil, r, nil
	}
	// Legacy form: Content is a plain JSON string. Wrap as a single
	// text block.
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, r, fmt.Errorf("decode string content: %w", err)
		}
		return []llmlib.ContentBlock{{Type: llmlib.BlockText, Text: s}}, r, nil
	}
	// Either current or legacy shape: a JSON array of blocks. Decode
	// once into a permissive shape and translate per-block.
	type rawBlock struct {
		Type      string          `json:"type"`
		Text      string          `json:"text,omitempty"`
		ID        string          `json:"id,omitempty"`
		Name      string          `json:"name,omitempty"`
		Input     json.RawMessage `json:"input,omitempty"`
		ToolUseID string          `json:"tool_use_id,omitempty"`
		// Legacy tool_result carried Content (string or structured)
		// where the library uses Text.
		Content json.RawMessage `json:"content,omitempty"`
		IsError bool            `json:"is_error,omitempty"`
		// Library tool_use blocks nest the call under tool_use.
		ToolUse *llmlib.ToolUse `json:"tool_use,omitempty"`
	}
	var rb []rawBlock
	if err := json.Unmarshal(raw, &rb); err != nil {
		return nil, r, fmt.Errorf("decode content array: %w", err)
	}
	blocks := make([]llmlib.ContentBlock, 0, len(rb))
	containsToolResult := false
	for _, b := range rb {
		switch b.Type {
		case string(llmlib.BlockText), "":
			blocks = append(blocks, llmlib.ContentBlock{Type: llmlib.BlockText, Text: b.Text})
		case string(llmlib.BlockToolUse):
			tu := b.ToolUse
			if tu == nil {
				tu = &llmlib.ToolUse{ID: b.ID, Name: b.Name, Input: b.Input}
			}
			blocks = append(blocks, llmlib.ContentBlock{Type: llmlib.BlockToolUse, ToolUse: tu})
		case string(llmlib.BlockToolResult):
			containsToolResult = true
			text := b.Text
			if text == "" && len(b.Content) > 0 {
				text = coerceToolResultText(b.Content)
			}
			blocks = append(blocks, llmlib.ContentBlock{
				Type:      llmlib.BlockToolResult,
				ToolUseID: b.ToolUseID,
				Text:      text,
				IsError:   b.IsError,
			})
		default:
			// Unknown / image / document blocks are passed through
			// as text so a save/load round-trip never silently drops
			// data; the LLM may ignore them.
			blocks = append(blocks, llmlib.ContentBlock{Type: llmlib.BlockText, Text: b.Text})
		}
	}
	// A message that carries tool-result blocks must use RoleTool
	// for the library to route it correctly. Pre-PR5 saves used
	// "user" for tool-result messages; fix that on load.
	if containsToolResult {
		r = llmlib.RoleTool
	}
	return blocks, r, nil
}

// coerceToolResultText reduces a legacy tool_result Content field
// (which may be a string, an array of MCP content items, or some
// other structured value) to a single text string.
func coerceToolResultText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	// Plain string.
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			return s
		}
	}
	// Array of MCP content items: [{type,text}, ...].
	if raw[0] == '[' {
		var items []struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal(raw, &items); err == nil {
			parts := make([]string, 0, len(items))
			for _, it := range items {
				if it.Text != "" {
					parts = append(parts, it.Text)
				}
			}
			joined := ""
			for i, p := range parts {
				if i > 0 {
					joined += "\n"
				}
				joined += p
			}
			return joined
		}
	}
	// Fallback: return the raw JSON.
	return string(raw)
}

// ConversationsClient manages conversation history via the REST API
type ConversationsClient struct {
	baseURL string
	token   string
	client  *http.Client
}

// NewConversationsClient creates a new conversations client
func NewConversationsClient(baseURL, token string) *ConversationsClient {
	// Remove /mcp/v1 suffix if present to get base URL
	apiURL := baseURL
	if len(apiURL) > 7 && apiURL[len(apiURL)-7:] == "/mcp/v1" {
		apiURL = apiURL[:len(apiURL)-7]
	}
	return &ConversationsClient{
		baseURL: apiURL + "/api/conversations",
		token:   token,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// ListResponse represents the response from list endpoint
type ListResponse struct {
	Conversations []ConversationSummary `json:"conversations"`
}

// List returns all conversations for the current user
func (c *ConversationsClient) List(ctx context.Context) ([]ConversationSummary, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body) //nolint:errcheck // Best effort to read error body
		return nil, fmt.Errorf("request failed (%d): %s", resp.StatusCode, string(body))
	}

	var result ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Conversations, nil
}

// Get retrieves a specific conversation by ID
func (c *ConversationsClient) Get(ctx context.Context, id string) (*Conversation, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/"+id, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("conversation not found")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body) //nolint:errcheck // Best effort to read error body
		return nil, fmt.Errorf("request failed (%d): %s", resp.StatusCode, string(body))
	}

	var result Conversation
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// CreateRequest represents a request to create a conversation
type CreateConversationRequest struct {
	Provider   string           `json:"provider"`
	Model      string           `json:"model"`
	Connection string           `json:"connection"`
	Messages   []llmlib.Message `json:"messages"`
}

// Create creates a new conversation
func (c *ConversationsClient) Create(ctx context.Context, provider, model, connection string, messages []llmlib.Message) (*Conversation, error) {
	body := CreateConversationRequest{
		Provider:   provider,
		Model:      model,
		Connection: connection,
		Messages:   messages,
	}

	jsonData, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body) //nolint:errcheck // Best effort to read error body
		return nil, fmt.Errorf("request failed (%d): %s", resp.StatusCode, string(respBody))
	}

	var result Conversation
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// Update updates an existing conversation
func (c *ConversationsClient) Update(ctx context.Context, id, provider, model, connection string, messages []llmlib.Message) (*Conversation, error) {
	body := CreateConversationRequest{
		Provider:   provider,
		Model:      model,
		Connection: connection,
		Messages:   messages,
	}

	jsonData, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", c.baseURL+"/"+id, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("conversation not found")
	}
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body) //nolint:errcheck // Best effort to read error body
		return nil, fmt.Errorf("request failed (%d): %s", resp.StatusCode, string(respBody))
	}

	var result Conversation
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// RenameRequest represents a request to rename a conversation
type RenameConversationRequest struct {
	Title string `json:"title"`
}

// Rename renames a conversation
func (c *ConversationsClient) Rename(ctx context.Context, id, title string) error {
	body := RenameConversationRequest{Title: title}

	jsonData, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "PATCH", c.baseURL+"/"+id, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("conversation not found")
	}
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body) //nolint:errcheck // Best effort to read error body
		return fmt.Errorf("request failed (%d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// Delete deletes a conversation
func (c *ConversationsClient) Delete(ctx context.Context, id string) error {
	req, err := http.NewRequestWithContext(ctx, "DELETE", c.baseURL+"/"+id, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("conversation not found")
	}
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body) //nolint:errcheck // Best effort to read error body
		return fmt.Errorf("request failed (%d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// DeleteAll deletes all conversations for the current user
func (c *ConversationsClient) DeleteAll(ctx context.Context) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, "DELETE", c.baseURL+"?all=true", nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body) //nolint:errcheck // Best effort to read error body
		return 0, fmt.Errorf("request failed (%d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Success bool  `json:"success"`
		Deleted int64 `json:"deleted"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Deleted, nil
}
