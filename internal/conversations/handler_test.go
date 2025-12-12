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
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"pgedge-postgres-mcp/internal/auth"
)

// setupTestHandler creates a test handler with a store and user store
func setupTestHandler(t *testing.T) (*Handler, func(), string) {
	tempDir, err := os.MkdirTemp("", "conversations_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Create conversation store
	store, err := NewStore(tempDir)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to create store: %v", err)
	}

	// Initialize user store (no arguments)
	userStore := auth.InitializeUserStore()

	// Add a test user
	err = userStore.AddUser("testuser", "password123", "Test User")
	if err != nil {
		store.Close()
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to add test user: %v", err)
	}

	// Authenticate to get a session token
	sessionToken, _, err := userStore.AuthenticateUser("testuser", "password123", 0)
	if err != nil {
		store.Close()
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to authenticate user: %v", err)
	}

	handler := NewHandler(store, userStore)

	cleanup := func() {
		store.Close()
		os.RemoveAll(tempDir)
	}

	return handler, cleanup, sessionToken
}

func TestNewHandler(t *testing.T) {
	handler, cleanup, _ := setupTestHandler(t)
	defer cleanup()

	if handler == nil {
		t.Error("Expected non-nil handler")
	}
	if handler.store == nil {
		t.Error("Expected non-nil store")
	}
	if handler.userStore == nil {
		t.Error("Expected non-nil userStore")
	}
}

func TestHandleList(t *testing.T) {
	handler, cleanup, token := setupTestHandler(t)
	defer cleanup()

	// First create a conversation
	messages := []Message{{Role: "user", Content: "Test message"}}
	_, err := handler.store.Create("testuser", "anthropic", "claude-3", "", messages)
	if err != nil {
		t.Fatalf("Failed to create conversation: %v", err)
	}

	// Create request
	req := httptest.NewRequest("GET", "/api/conversations", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	// Create response recorder
	rr := httptest.NewRecorder()

	// Handle request
	handler.HandleList(rr, req)

	// Check status code
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	// Parse response
	var response struct {
		Conversations []ConversationSummary `json:"conversations"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(response.Conversations) != 1 {
		t.Errorf("Expected 1 conversation, got %d", len(response.Conversations))
	}
}

func TestHandleList_Unauthorized(t *testing.T) {
	handler, cleanup, _ := setupTestHandler(t)
	defer cleanup()

	// Create request without token
	req := httptest.NewRequest("GET", "/api/conversations", nil)
	rr := httptest.NewRecorder()

	handler.HandleList(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestHandleList_WrongMethod(t *testing.T) {
	handler, cleanup, token := setupTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest("POST", "/api/conversations", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	handler.HandleList(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, rr.Code)
	}
}

func TestHandleCreate(t *testing.T) {
	handler, cleanup, token := setupTestHandler(t)
	defer cleanup()

	// Create request body
	createReq := CreateRequest{
		Provider:   "anthropic",
		Model:      "claude-3",
		Connection: "testdb",
		Messages:   []Message{{Role: "user", Content: "Hello"}},
	}
	body, _ := json.Marshal(createReq)

	req := httptest.NewRequest("POST", "/api/conversations", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.HandleCreate(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}

	// Parse response
	var conv Conversation
	if err := json.NewDecoder(rr.Body).Decode(&conv); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if conv.ID == "" {
		t.Error("Expected non-empty conversation ID")
	}
	if conv.Provider != "anthropic" {
		t.Errorf("Expected provider 'anthropic', got %q", conv.Provider)
	}
}

func TestHandleCreate_EmptyMessages(t *testing.T) {
	handler, cleanup, token := setupTestHandler(t)
	defer cleanup()

	createReq := CreateRequest{
		Provider: "anthropic",
		Messages: []Message{}, // Empty messages
	}
	body, _ := json.Marshal(createReq)

	req := httptest.NewRequest("POST", "/api/conversations", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.HandleCreate(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestHandleCreate_InvalidBody(t *testing.T) {
	handler, cleanup, token := setupTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest("POST", "/api/conversations", bytes.NewBufferString("invalid json"))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.HandleCreate(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestHandleGet(t *testing.T) {
	handler, cleanup, token := setupTestHandler(t)
	defer cleanup()

	// Create a conversation first
	messages := []Message{{Role: "user", Content: "Test message"}}
	conv, err := handler.store.Create("testuser", "anthropic", "claude-3", "", messages)
	if err != nil {
		t.Fatalf("Failed to create conversation: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/conversations/"+conv.ID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	handler.HandleGet(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var retrieved Conversation
	if err := json.NewDecoder(rr.Body).Decode(&retrieved); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if retrieved.ID != conv.ID {
		t.Errorf("Expected ID %q, got %q", conv.ID, retrieved.ID)
	}
}

func TestHandleGet_NotFound(t *testing.T) {
	handler, cleanup, token := setupTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/conversations/nonexistent", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	handler.HandleGet(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestHandleGet_MissingID(t *testing.T) {
	handler, cleanup, token := setupTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/conversations/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	handler.HandleGet(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestHandleUpdate(t *testing.T) {
	handler, cleanup, token := setupTestHandler(t)
	defer cleanup()

	// Create a conversation first
	messages := []Message{{Role: "user", Content: "Original"}}
	conv, err := handler.store.Create("testuser", "anthropic", "claude-3", "", messages)
	if err != nil {
		t.Fatalf("Failed to create conversation: %v", err)
	}

	// Update the conversation
	updateReq := UpdateRequest{
		Provider:   "openai",
		Model:      "gpt-4",
		Connection: "newdb",
		Messages:   []Message{{Role: "user", Content: "Updated"}},
	}
	body, _ := json.Marshal(updateReq)

	req := httptest.NewRequest("PUT", "/api/conversations/"+conv.ID, bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.HandleUpdate(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var updated Conversation
	if err := json.NewDecoder(rr.Body).Decode(&updated); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if updated.Provider != "openai" {
		t.Errorf("Expected provider 'openai', got %q", updated.Provider)
	}
}

func TestHandleUpdate_NotFound(t *testing.T) {
	handler, cleanup, token := setupTestHandler(t)
	defer cleanup()

	updateReq := UpdateRequest{
		Messages: []Message{{Role: "user", Content: "Test"}},
	}
	body, _ := json.Marshal(updateReq)

	req := httptest.NewRequest("PUT", "/api/conversations/nonexistent", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.HandleUpdate(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestHandleRename(t *testing.T) {
	handler, cleanup, token := setupTestHandler(t)
	defer cleanup()

	// Create a conversation first
	messages := []Message{{Role: "user", Content: "Original title"}}
	conv, err := handler.store.Create("testuser", "anthropic", "claude-3", "", messages)
	if err != nil {
		t.Fatalf("Failed to create conversation: %v", err)
	}

	// Rename the conversation
	renameReq := RenameRequest{Title: "New Title"}
	body, _ := json.Marshal(renameReq)

	req := httptest.NewRequest("PATCH", "/api/conversations/"+conv.ID, bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.HandleRename(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	// Verify the rename
	retrieved, err := handler.store.Get(conv.ID, "testuser")
	if err != nil {
		t.Fatalf("Failed to get conversation: %v", err)
	}
	if retrieved.Title != "New Title" {
		t.Errorf("Expected title 'New Title', got %q", retrieved.Title)
	}
}

func TestHandleRename_EmptyTitle(t *testing.T) {
	handler, cleanup, token := setupTestHandler(t)
	defer cleanup()

	// Create a conversation first
	messages := []Message{{Role: "user", Content: "Test"}}
	conv, err := handler.store.Create("testuser", "anthropic", "claude-3", "", messages)
	if err != nil {
		t.Fatalf("Failed to create conversation: %v", err)
	}

	// Try to rename with empty title
	renameReq := RenameRequest{Title: ""}
	body, _ := json.Marshal(renameReq)

	req := httptest.NewRequest("PATCH", "/api/conversations/"+conv.ID, bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.HandleRename(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestHandleDelete(t *testing.T) {
	handler, cleanup, token := setupTestHandler(t)
	defer cleanup()

	// Create a conversation first
	messages := []Message{{Role: "user", Content: "Test"}}
	conv, err := handler.store.Create("testuser", "anthropic", "claude-3", "", messages)
	if err != nil {
		t.Fatalf("Failed to create conversation: %v", err)
	}

	req := httptest.NewRequest("DELETE", "/api/conversations/"+conv.ID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	handler.HandleDelete(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	// Verify it's deleted
	_, err = handler.store.Get(conv.ID, "testuser")
	if err == nil {
		t.Error("Expected conversation to be deleted")
	}
}

func TestHandleDelete_NotFound(t *testing.T) {
	handler, cleanup, token := setupTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest("DELETE", "/api/conversations/nonexistent", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	handler.HandleDelete(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestHandleDeleteAll(t *testing.T) {
	handler, cleanup, token := setupTestHandler(t)
	defer cleanup()

	// Create multiple conversations
	for i := 0; i < 3; i++ {
		messages := []Message{{Role: "user", Content: "Test"}}
		_, err := handler.store.Create("testuser", "", "", "", messages)
		if err != nil {
			t.Fatalf("Failed to create conversation: %v", err)
		}
	}

	req := httptest.NewRequest("DELETE", "/api/conversations?all=true", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	handler.HandleDeleteAll(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	// Parse response
	var response struct {
		Success bool  `json:"success"`
		Deleted int64 `json:"deleted"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("Expected success to be true")
	}
	if response.Deleted != 3 {
		t.Errorf("Expected 3 deleted, got %d", response.Deleted)
	}
}

func TestExtractUsername_MissingHeader(t *testing.T) {
	handler, cleanup, _ := setupTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/conversations", nil)
	// No Authorization header

	_, err := handler.extractUsername(req)
	if err == nil {
		t.Error("Expected error for missing Authorization header")
	}
}

func TestExtractUsername_InvalidFormat(t *testing.T) {
	handler, cleanup, _ := setupTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/conversations", nil)
	req.Header.Set("Authorization", "Basic invalid") // Wrong format

	_, err := handler.extractUsername(req)
	if err == nil {
		t.Error("Expected error for invalid Authorization format")
	}
}

func TestExtractUsername_InvalidToken(t *testing.T) {
	handler, cleanup, _ := setupTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/conversations", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")

	_, err := handler.extractUsername(req)
	if err == nil {
		t.Error("Expected error for invalid token")
	}
}

func TestHandleList_WithPagination(t *testing.T) {
	handler, cleanup, token := setupTestHandler(t)
	defer cleanup()

	// Create 5 conversations
	for i := 0; i < 5; i++ {
		messages := []Message{{Role: "user", Content: "Test"}}
		_, err := handler.store.Create("testuser", "", "", "", messages)
		if err != nil {
			t.Fatalf("Failed to create conversation: %v", err)
		}
	}

	// Request with limit=2
	req := httptest.NewRequest("GET", "/api/conversations?limit=2&offset=0", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	handler.HandleList(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var response struct {
		Conversations []ConversationSummary `json:"conversations"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(response.Conversations) != 2 {
		t.Errorf("Expected 2 conversations, got %d", len(response.Conversations))
	}
}

func TestSendJSON(t *testing.T) {
	rr := httptest.NewRecorder()

	data := map[string]string{"message": "test"}
	sendJSON(rr, http.StatusOK, data)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	contentType := rr.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got %q", contentType)
	}

	var response map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["message"] != "test" {
		t.Errorf("Expected message 'test', got %q", response["message"])
	}
}

func TestSendError(t *testing.T) {
	rr := httptest.NewRecorder()

	sendError(rr, http.StatusBadRequest, "test error")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["error"] != "test error" {
		t.Errorf("Expected error 'test error', got %q", response["error"])
	}
}
