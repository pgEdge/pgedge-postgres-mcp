/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package conversations

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewStore(t *testing.T) {
	// Create a temporary directory for the test database
	tempDir, err := os.MkdirTemp("", "conversations_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Verify the database file was created
	dbPath := filepath.Join(tempDir, "conversations.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Errorf("Database file was not created at %s", dbPath)
	}
}

func TestCreateAndGetConversation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "conversations_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create a conversation
	messages := []Message{
		{Role: "user", Content: "Hello, how are you?"},
		{Role: "assistant", Content: "I'm doing well, thank you!"},
	}

	conv, err := store.Create("testuser", "anthropic", "claude-3-sonnet", "", messages)
	if err != nil {
		t.Fatalf("Failed to create conversation: %v", err)
	}

	// Verify the conversation was created
	if conv.ID == "" {
		t.Error("Conversation ID should not be empty")
	}
	if conv.Username != "testuser" {
		t.Errorf("Expected username 'testuser', got '%s'", conv.Username)
	}
	if conv.Provider != "anthropic" {
		t.Errorf("Expected provider 'anthropic', got '%s'", conv.Provider)
	}
	if conv.Model != "claude-3-sonnet" {
		t.Errorf("Expected model 'claude-3-sonnet', got '%s'", conv.Model)
	}
	if conv.Title != "Hello, how are you?" {
		t.Errorf("Expected title 'Hello, how are you?', got '%s'", conv.Title)
	}
	if len(conv.Messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(conv.Messages))
	}

	// Get the conversation
	retrieved, err := store.Get(conv.ID, "testuser")
	if err != nil {
		t.Fatalf("Failed to get conversation: %v", err)
	}

	if retrieved.ID != conv.ID {
		t.Errorf("Expected ID '%s', got '%s'", conv.ID, retrieved.ID)
	}
	if retrieved.Provider != conv.Provider {
		t.Errorf("Expected provider '%s', got '%s'", conv.Provider, retrieved.Provider)
	}
	if retrieved.Model != conv.Model {
		t.Errorf("Expected model '%s', got '%s'", conv.Model, retrieved.Model)
	}
}

func TestGetConversationWrongUser(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "conversations_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create a conversation for user1
	messages := []Message{{Role: "user", Content: "Test message"}}
	conv, err := store.Create("user1", "", "", "", messages)
	if err != nil {
		t.Fatalf("Failed to create conversation: %v", err)
	}

	// Try to get the conversation as user2
	_, err = store.Get(conv.ID, "user2")
	if err == nil {
		t.Error("Expected error when getting conversation as wrong user")
	}
}

func TestUpdateConversation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "conversations_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create a conversation
	messages := []Message{{Role: "user", Content: "Initial message"}}
	conv, err := store.Create("testuser", "openai", "gpt-4", "", messages)
	if err != nil {
		t.Fatalf("Failed to create conversation: %v", err)
	}

	// Update the conversation
	newMessages := []Message{
		{Role: "user", Content: "Initial message"},
		{Role: "assistant", Content: "Response"},
		{Role: "user", Content: "Follow up"},
	}

	updated, err := store.Update(conv.ID, "testuser", "anthropic", "claude-3-opus", "", newMessages)
	if err != nil {
		t.Fatalf("Failed to update conversation: %v", err)
	}

	if len(updated.Messages) != 3 {
		t.Errorf("Expected 3 messages, got %d", len(updated.Messages))
	}
	if updated.Provider != "anthropic" {
		t.Errorf("Expected provider 'anthropic', got '%s'", updated.Provider)
	}
	if updated.Model != "claude-3-opus" {
		t.Errorf("Expected model 'claude-3-opus', got '%s'", updated.Model)
	}
}

func TestUpdateConversationWrongUser(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "conversations_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create a conversation for user1
	messages := []Message{{Role: "user", Content: "Test message"}}
	conv, err := store.Create("user1", "", "", "", messages)
	if err != nil {
		t.Fatalf("Failed to create conversation: %v", err)
	}

	// Try to update the conversation as user2
	_, err = store.Update(conv.ID, "user2", "", "", "", messages)
	if err == nil {
		t.Error("Expected error when updating conversation as wrong user")
	}
}

func TestRenameConversation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "conversations_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create a conversation
	messages := []Message{{Role: "user", Content: "Original title"}}
	conv, err := store.Create("testuser", "", "", "", messages)
	if err != nil {
		t.Fatalf("Failed to create conversation: %v", err)
	}

	// Rename the conversation
	err = store.Rename(conv.ID, "testuser", "New Title")
	if err != nil {
		t.Fatalf("Failed to rename conversation: %v", err)
	}

	// Verify the rename
	retrieved, err := store.Get(conv.ID, "testuser")
	if err != nil {
		t.Fatalf("Failed to get conversation: %v", err)
	}

	if retrieved.Title != "New Title" {
		t.Errorf("Expected title 'New Title', got '%s'", retrieved.Title)
	}
}

func TestRenameConversationWrongUser(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "conversations_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create a conversation for user1
	messages := []Message{{Role: "user", Content: "Test message"}}
	conv, err := store.Create("user1", "", "", "", messages)
	if err != nil {
		t.Fatalf("Failed to create conversation: %v", err)
	}

	// Try to rename as user2
	err = store.Rename(conv.ID, "user2", "Hacked Title")
	if err == nil {
		t.Error("Expected error when renaming conversation as wrong user")
	}
}

func TestListConversations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "conversations_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create multiple conversations for testuser
	for i := 0; i < 3; i++ {
		messages := []Message{{Role: "user", Content: "Message " + string(rune('A'+i))}}
		_, err := store.Create("testuser", "", "", "", messages)
		if err != nil {
			t.Fatalf("Failed to create conversation %d: %v", i, err)
		}
	}

	// Create a conversation for another user
	messages := []Message{{Role: "user", Content: "Other user message"}}
	_, err = store.Create("otheruser", "", "", "", messages)
	if err != nil {
		t.Fatalf("Failed to create conversation for otheruser: %v", err)
	}

	// List conversations for testuser
	list, err := store.List("testuser", 50, 0)
	if err != nil {
		t.Fatalf("Failed to list conversations: %v", err)
	}

	if len(list) != 3 {
		t.Errorf("Expected 3 conversations, got %d", len(list))
	}

	// List conversations for otheruser
	list, err = store.List("otheruser", 50, 0)
	if err != nil {
		t.Fatalf("Failed to list conversations: %v", err)
	}

	if len(list) != 1 {
		t.Errorf("Expected 1 conversation, got %d", len(list))
	}
}

func TestListConversationsPagination(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "conversations_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create 5 conversations
	for i := 0; i < 5; i++ {
		messages := []Message{{Role: "user", Content: "Message " + string(rune('A'+i))}}
		_, err := store.Create("testuser", "", "", "", messages)
		if err != nil {
			t.Fatalf("Failed to create conversation %d: %v", i, err)
		}
	}

	// Get first 2
	list, err := store.List("testuser", 2, 0)
	if err != nil {
		t.Fatalf("Failed to list conversations: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("Expected 2 conversations, got %d", len(list))
	}

	// Get next 2
	list, err = store.List("testuser", 2, 2)
	if err != nil {
		t.Fatalf("Failed to list conversations: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("Expected 2 conversations, got %d", len(list))
	}

	// Get last 1
	list, err = store.List("testuser", 2, 4)
	if err != nil {
		t.Fatalf("Failed to list conversations: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("Expected 1 conversation, got %d", len(list))
	}
}

func TestDeleteConversation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "conversations_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create a conversation
	messages := []Message{{Role: "user", Content: "Test message"}}
	conv, err := store.Create("testuser", "", "", "", messages)
	if err != nil {
		t.Fatalf("Failed to create conversation: %v", err)
	}

	// Delete the conversation
	err = store.Delete(conv.ID, "testuser")
	if err != nil {
		t.Fatalf("Failed to delete conversation: %v", err)
	}

	// Verify it's deleted
	_, err = store.Get(conv.ID, "testuser")
	if err == nil {
		t.Error("Expected error when getting deleted conversation")
	}
}

func TestDeleteConversationWrongUser(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "conversations_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create a conversation for user1
	messages := []Message{{Role: "user", Content: "Test message"}}
	conv, err := store.Create("user1", "", "", "", messages)
	if err != nil {
		t.Fatalf("Failed to create conversation: %v", err)
	}

	// Try to delete as user2
	err = store.Delete(conv.ID, "user2")
	if err == nil {
		t.Error("Expected error when deleting conversation as wrong user")
	}

	// Verify the conversation still exists
	_, err = store.Get(conv.ID, "user1")
	if err != nil {
		t.Error("Conversation should still exist after failed delete attempt")
	}
}

func TestDeleteAllConversations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "conversations_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create multiple conversations for testuser
	for i := 0; i < 3; i++ {
		messages := []Message{{Role: "user", Content: "Message " + string(rune('A'+i))}}
		_, err := store.Create("testuser", "", "", "", messages)
		if err != nil {
			t.Fatalf("Failed to create conversation %d: %v", i, err)
		}
	}

	// Create a conversation for another user
	messages := []Message{{Role: "user", Content: "Other user message"}}
	_, err = store.Create("otheruser", "", "", "", messages)
	if err != nil {
		t.Fatalf("Failed to create conversation for otheruser: %v", err)
	}

	// Delete all conversations for testuser
	count, err := store.DeleteAll("testuser")
	if err != nil {
		t.Fatalf("Failed to delete all conversations: %v", err)
	}

	if count != 3 {
		t.Errorf("Expected 3 deleted, got %d", count)
	}

	// Verify testuser has no conversations
	list, err := store.List("testuser", 50, 0)
	if err != nil {
		t.Fatalf("Failed to list conversations: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("Expected 0 conversations for testuser, got %d", len(list))
	}

	// Verify otheruser still has their conversation
	list, err = store.List("otheruser", 50, 0)
	if err != nil {
		t.Fatalf("Failed to list conversations: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("Expected 1 conversation for otheruser, got %d", len(list))
	}
}

func TestGenerateTitleFromMessages(t *testing.T) {
	tests := []struct {
		name     string
		messages []Message
		expected string
	}{
		{
			name:     "empty messages",
			messages: []Message{},
			expected: "New conversation",
		},
		{
			name: "only assistant message",
			messages: []Message{
				{Role: "assistant", Content: "Hello!"},
			},
			expected: "New conversation",
		},
		{
			name: "short user message",
			messages: []Message{
				{Role: "user", Content: "Hello"},
			},
			expected: "Hello",
		},
		{
			name: "long user message gets truncated",
			messages: []Message{
				{Role: "user", Content: "This is a very long message that should be truncated to a reasonable length for the title"},
			},
			expected: "This is a very long message that should be trun...",
		},
		{
			name: "empty user message content",
			messages: []Message{
				{Role: "user", Content: ""},
			},
			expected: "New conversation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateTitle(tt.messages)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestSchemaMigration(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "conversations_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a store (this runs the migration)
	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	store.Close()

	// Open the store again (migration should be idempotent)
	store, err = NewStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to reopen store: %v", err)
	}
	defer store.Close()

	// Verify we can create conversations with provider/model
	messages := []Message{{Role: "user", Content: "Test"}}
	conv, err := store.Create("testuser", "anthropic", "claude-3-sonnet", "", messages)
	if err != nil {
		t.Fatalf("Failed to create conversation: %v", err)
	}

	if conv.Provider != "anthropic" {
		t.Errorf("Expected provider 'anthropic', got '%s'", conv.Provider)
	}
	if conv.Model != "claude-3-sonnet" {
		t.Errorf("Expected model 'claude-3-sonnet', got '%s'", conv.Model)
	}
}
