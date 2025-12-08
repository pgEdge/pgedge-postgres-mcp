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
	"time"
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
	ID         string    `json:"id"`
	Username   string    `json:"username"`
	Title      string    `json:"title"`
	Provider   string    `json:"provider,omitempty"`
	Model      string    `json:"model,omitempty"`
	Connection string    `json:"connection,omitempty"`
	Messages   []Message `json:"messages"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
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
	Provider   string    `json:"provider"`
	Model      string    `json:"model"`
	Connection string    `json:"connection"`
	Messages   []Message `json:"messages"`
}

// Create creates a new conversation
func (c *ConversationsClient) Create(ctx context.Context, provider, model, connection string, messages []Message) (*Conversation, error) {
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
func (c *ConversationsClient) Update(ctx context.Context, id, provider, model, connection string, messages []Message) (*Conversation, error) {
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
