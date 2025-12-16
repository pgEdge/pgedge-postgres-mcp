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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewConversationsClient(t *testing.T) {
	tests := []struct {
		name            string
		baseURL         string
		token           string
		expectedBaseURL string
	}{
		{
			name:            "with mcp/v1 suffix",
			baseURL:         "http://localhost:8080/mcp/v1",
			token:           "test-token",
			expectedBaseURL: "http://localhost:8080/api/conversations",
		},
		{
			name:            "without mcp/v1 suffix",
			baseURL:         "http://localhost:8080",
			token:           "test-token",
			expectedBaseURL: "http://localhost:8080/api/conversations",
		},
		{
			name:            "empty token",
			baseURL:         "http://localhost:8080",
			token:           "",
			expectedBaseURL: "http://localhost:8080/api/conversations",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewConversationsClient(tt.baseURL, tt.token)
			if client == nil {
				t.Fatal("Expected non-nil client")
			}
			if client.baseURL != tt.expectedBaseURL {
				t.Errorf("baseURL = %q, want %q", client.baseURL, tt.expectedBaseURL)
			}
			if client.token != tt.token {
				t.Errorf("token = %q, want %q", client.token, tt.token)
			}
		})
	}
}

func TestConversationsClient_List(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET, got %s", r.Method)
		}

		// Check authorization header
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token" {
			t.Errorf("Expected Authorization header 'Bearer test-token', got %q", auth)
		}

		response := ListResponse{
			Conversations: []ConversationSummary{
				{
					ID:        "conv_123",
					Title:     "Test Conversation",
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
					Preview:   "Hello, how can I help?",
				},
				{
					ID:         "conv_456",
					Title:      "Another Conversation",
					Connection: "mydb",
					CreatedAt:  time.Now(),
					UpdatedAt:  time.Now(),
					Preview:    "What's new?",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewConversationsClient(server.URL, "test-token")
	conversations, err := client.List(context.Background())
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(conversations) != 2 {
		t.Errorf("Expected 2 conversations, got %d", len(conversations))
	}
	if conversations[0].ID != "conv_123" {
		t.Errorf("Expected first conversation ID 'conv_123', got %q", conversations[0].ID)
	}
	if conversations[1].Connection != "mydb" {
		t.Errorf("Expected second conversation connection 'mydb', got %q", conversations[1].Connection)
	}
}

func TestConversationsClient_List_Error(t *testing.T) {
	// Create a mock server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal server error"))
	}))
	defer server.Close()

	client := NewConversationsClient(server.URL, "test-token")
	_, err := client.List(context.Background())
	if err == nil {
		t.Error("Expected error for server error response")
	}
}

func TestConversationsClient_Get(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET, got %s", r.Method)
		}

		// Check path includes conversation ID
		expectedPath := "/api/conversations/conv_123"
		if r.URL.Path != expectedPath {
			t.Errorf("Expected path %q, got %q", expectedPath, r.URL.Path)
		}

		response := Conversation{
			ID:       "conv_123",
			Username: "testuser",
			Title:    "Test Conversation",
			Provider: "anthropic",
			Model:    "claude-3-opus",
			Messages: []Message{
				{Role: "user", Content: "Hello"},
				{Role: "assistant", Content: "Hi there!"},
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewConversationsClient(server.URL, "test-token")
	conv, err := client.Get(context.Background(), "conv_123")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if conv.ID != "conv_123" {
		t.Errorf("Expected ID 'conv_123', got %q", conv.ID)
	}
	if conv.Provider != "anthropic" {
		t.Errorf("Expected Provider 'anthropic', got %q", conv.Provider)
	}
	if len(conv.Messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(conv.Messages))
	}
}

func TestConversationsClient_Get_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewConversationsClient(server.URL, "test-token")
	_, err := client.Get(context.Background(), "nonexistent")
	if err == nil {
		t.Error("Expected error for not found")
	}
	if err.Error() != "conversation not found" {
		t.Errorf("Expected 'conversation not found' error, got: %v", err)
	}
}

func TestConversationsClient_Create(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}

		// Parse request body
		var req CreateConversationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}

		if req.Provider != "anthropic" {
			t.Errorf("Expected provider 'anthropic', got %q", req.Provider)
		}
		if req.Model != "claude-3-opus" {
			t.Errorf("Expected model 'claude-3-opus', got %q", req.Model)
		}
		if len(req.Messages) != 2 {
			t.Errorf("Expected 2 messages, got %d", len(req.Messages))
		}

		response := Conversation{
			ID:         "conv_new",
			Title:      "New Conversation",
			Provider:   req.Provider,
			Model:      req.Model,
			Connection: req.Connection,
			Messages:   req.Messages,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}

		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewConversationsClient(server.URL, "test-token")
	messages := []Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi!"},
	}

	conv, err := client.Create(context.Background(), "anthropic", "claude-3-opus", "mydb", messages)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if conv.ID != "conv_new" {
		t.Errorf("Expected ID 'conv_new', got %q", conv.ID)
	}
}

func TestConversationsClient_Update(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("Expected PUT, got %s", r.Method)
		}

		expectedPath := "/api/conversations/conv_123"
		if r.URL.Path != expectedPath {
			t.Errorf("Expected path %q, got %q", expectedPath, r.URL.Path)
		}

		response := Conversation{
			ID:        "conv_123",
			Title:     "Updated Conversation",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewConversationsClient(server.URL, "test-token")
	messages := []Message{
		{Role: "user", Content: "Updated message"},
	}

	conv, err := client.Update(context.Background(), "conv_123", "anthropic", "claude-3-opus", "mydb", messages)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	if conv.ID != "conv_123" {
		t.Errorf("Expected ID 'conv_123', got %q", conv.ID)
	}
}

func TestConversationsClient_Update_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewConversationsClient(server.URL, "test-token")
	_, err := client.Update(context.Background(), "nonexistent", "anthropic", "model", "", nil)
	if err == nil {
		t.Error("Expected error for not found")
	}
}

func TestConversationsClient_Rename(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" {
			t.Errorf("Expected PATCH, got %s", r.Method)
		}

		expectedPath := "/api/conversations/conv_123"
		if r.URL.Path != expectedPath {
			t.Errorf("Expected path %q, got %q", expectedPath, r.URL.Path)
		}

		var req RenameConversationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}

		if req.Title != "New Title" {
			t.Errorf("Expected title 'New Title', got %q", req.Title)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewConversationsClient(server.URL, "test-token")
	err := client.Rename(context.Background(), "conv_123", "New Title")
	if err != nil {
		t.Fatalf("Rename failed: %v", err)
	}
}

func TestConversationsClient_Rename_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewConversationsClient(server.URL, "test-token")
	err := client.Rename(context.Background(), "nonexistent", "Title")
	if err == nil {
		t.Error("Expected error for not found")
	}
}

func TestConversationsClient_Delete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("Expected DELETE, got %s", r.Method)
		}

		expectedPath := "/api/conversations/conv_123"
		if r.URL.Path != expectedPath {
			t.Errorf("Expected path %q, got %q", expectedPath, r.URL.Path)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewConversationsClient(server.URL, "test-token")
	err := client.Delete(context.Background(), "conv_123")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
}

func TestConversationsClient_Delete_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewConversationsClient(server.URL, "test-token")
	err := client.Delete(context.Background(), "nonexistent")
	if err == nil {
		t.Error("Expected error for not found")
	}
}

func TestConversationsClient_DeleteAll(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("Expected DELETE, got %s", r.Method)
		}

		// Check query parameter
		if r.URL.Query().Get("all") != "true" {
			t.Error("Expected 'all=true' query parameter")
		}

		response := struct {
			Success bool  `json:"success"`
			Deleted int64 `json:"deleted"`
		}{
			Success: true,
			Deleted: 5,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewConversationsClient(server.URL, "test-token")
	deleted, err := client.DeleteAll(context.Background())
	if err != nil {
		t.Fatalf("DeleteAll failed: %v", err)
	}

	if deleted != 5 {
		t.Errorf("Expected 5 deleted, got %d", deleted)
	}
}

func TestConversationsClient_DeleteAll_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Database error"))
	}))
	defer server.Close()

	client := NewConversationsClient(server.URL, "test-token")
	_, err := client.DeleteAll(context.Background())
	if err == nil {
		t.Error("Expected error for server error")
	}
}

func TestConversationStructs(t *testing.T) {
	// Test that structs can be marshaled and unmarshaled correctly
	now := time.Now().UTC().Truncate(time.Second)

	conv := Conversation{
		ID:         "conv_123",
		Username:   "testuser",
		Title:      "Test Conversation",
		Provider:   "anthropic",
		Model:      "claude-3-opus",
		Connection: "mydb",
		Messages: []Message{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi!"},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	data, err := json.Marshal(conv)
	if err != nil {
		t.Fatalf("Failed to marshal conversation: %v", err)
	}

	var unmarshaled Conversation
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal conversation: %v", err)
	}

	if unmarshaled.ID != conv.ID {
		t.Errorf("ID mismatch: got %q, want %q", unmarshaled.ID, conv.ID)
	}
	if unmarshaled.Provider != conv.Provider {
		t.Errorf("Provider mismatch: got %q, want %q", unmarshaled.Provider, conv.Provider)
	}
	if len(unmarshaled.Messages) != len(conv.Messages) {
		t.Errorf("Messages count mismatch: got %d, want %d", len(unmarshaled.Messages), len(conv.Messages))
	}
}

func TestConversationSummaryJSON(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)

	summary := ConversationSummary{
		ID:         "conv_123",
		Title:      "Test Summary",
		Connection: "mydb",
		CreatedAt:  now,
		UpdatedAt:  now,
		Preview:    "Preview text...",
	}

	data, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("Failed to marshal summary: %v", err)
	}

	var unmarshaled ConversationSummary
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal summary: %v", err)
	}

	if unmarshaled.ID != summary.ID {
		t.Errorf("ID mismatch: got %q, want %q", unmarshaled.ID, summary.ID)
	}
	if unmarshaled.Preview != summary.Preview {
		t.Errorf("Preview mismatch: got %q, want %q", unmarshaled.Preview, summary.Preview)
	}
}
