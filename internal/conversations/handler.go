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
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"pgedge-postgres-mcp/internal/auth"
)

// Handler handles conversation API requests
type Handler struct {
	store     *Store
	userStore *auth.UserStore
}

// NewHandler creates a new conversation handler
func NewHandler(store *Store, userStore *auth.UserStore) *Handler {
	return &Handler{
		store:     store,
		userStore: userStore,
	}
}

// extractUsername extracts the username from the session token
func (h *Handler) extractUsername(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", fmt.Errorf("missing Authorization header")
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == authHeader {
		return "", fmt.Errorf("invalid Authorization header format")
	}

	username, err := h.userStore.ValidateSessionToken(token)
	if err != nil {
		return "", fmt.Errorf("invalid or expired session")
	}

	return username, nil
}

// sendJSON sends a JSON response
func sendJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	//nolint:errcheck // Encoding a simple map should never fail
	json.NewEncoder(w).Encode(data)
}

// sendError sends an error response
func sendError(w http.ResponseWriter, status int, message string) {
	sendJSON(w, status, map[string]string{"error": message})
}

// HandleList handles GET /api/conversations
func (h *Handler) HandleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	username, err := h.extractUsername(r)
	if err != nil {
		sendError(w, http.StatusUnauthorized, err.Error())
		return
	}

	// Parse pagination parameters
	limit := 50
	offset := 0

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	conversations, err := h.store.List(username, limit, offset)
	if err != nil {
		sendError(w, http.StatusInternalServerError, "Failed to list conversations")
		return
	}

	// Return empty array instead of null
	if conversations == nil {
		conversations = []ConversationSummary{}
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"conversations": conversations,
	})
}

// HandleGet handles GET /api/conversations/{id}
func (h *Handler) HandleGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	username, err := h.extractUsername(r)
	if err != nil {
		sendError(w, http.StatusUnauthorized, err.Error())
		return
	}

	// Extract ID from path
	id := strings.TrimPrefix(r.URL.Path, "/api/conversations/")
	if id == "" {
		sendError(w, http.StatusBadRequest, "Conversation ID required")
		return
	}

	conv, err := h.store.Get(id, username)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			sendError(w, http.StatusNotFound, "Conversation not found")
		} else {
			sendError(w, http.StatusInternalServerError, "Failed to get conversation")
		}
		return
	}

	sendJSON(w, http.StatusOK, conv)
}

// CreateRequest represents a request to create a conversation
type CreateRequest struct {
	Provider   string    `json:"provider"`
	Model      string    `json:"model"`
	Connection string    `json:"connection"`
	Messages   []Message `json:"messages"`
}

// HandleCreate handles POST /api/conversations
func (h *Handler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	username, err := h.extractUsername(r)
	if err != nil {
		sendError(w, http.StatusUnauthorized, err.Error())
		return
	}

	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if len(req.Messages) == 0 {
		sendError(w, http.StatusBadRequest, "Messages required")
		return
	}

	conv, err := h.store.Create(username, req.Provider, req.Model, req.Connection, req.Messages)
	if err != nil {
		sendError(w, http.StatusInternalServerError, "Failed to create conversation")
		return
	}

	sendJSON(w, http.StatusCreated, conv)
}

// UpdateRequest represents a request to update a conversation
type UpdateRequest struct {
	Provider   string    `json:"provider"`
	Model      string    `json:"model"`
	Connection string    `json:"connection"`
	Messages   []Message `json:"messages"`
}

// HandleUpdate handles PUT /api/conversations/{id}
func (h *Handler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	username, err := h.extractUsername(r)
	if err != nil {
		sendError(w, http.StatusUnauthorized, err.Error())
		return
	}

	// Extract ID from path
	id := strings.TrimPrefix(r.URL.Path, "/api/conversations/")
	if id == "" {
		sendError(w, http.StatusBadRequest, "Conversation ID required")
		return
	}

	var req UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	conv, err := h.store.Update(id, username, req.Provider, req.Model, req.Connection, req.Messages)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			sendError(w, http.StatusNotFound, "Conversation not found")
		} else if strings.Contains(err.Error(), "access denied") {
			sendError(w, http.StatusForbidden, "Access denied")
		} else {
			sendError(w, http.StatusInternalServerError, "Failed to update conversation")
		}
		return
	}

	sendJSON(w, http.StatusOK, conv)
}

// RenameRequest represents a request to rename a conversation
type RenameRequest struct {
	Title string `json:"title"`
}

// HandleRename handles PATCH /api/conversations/{id}
func (h *Handler) HandleRename(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	username, err := h.extractUsername(r)
	if err != nil {
		sendError(w, http.StatusUnauthorized, err.Error())
		return
	}

	// Extract ID from path
	id := strings.TrimPrefix(r.URL.Path, "/api/conversations/")
	if id == "" {
		sendError(w, http.StatusBadRequest, "Conversation ID required")
		return
	}

	var req RenameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Title == "" {
		sendError(w, http.StatusBadRequest, "Title required")
		return
	}

	err = h.store.Rename(id, username, req.Title)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "access denied") {
			sendError(w, http.StatusNotFound, "Conversation not found")
		} else {
			sendError(w, http.StatusInternalServerError, "Failed to rename conversation")
		}
		return
	}

	sendJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// HandleDelete handles DELETE /api/conversations/{id}
func (h *Handler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	username, err := h.extractUsername(r)
	if err != nil {
		sendError(w, http.StatusUnauthorized, err.Error())
		return
	}

	// Extract ID from path
	id := strings.TrimPrefix(r.URL.Path, "/api/conversations/")
	if id == "" {
		sendError(w, http.StatusBadRequest, "Conversation ID required")
		return
	}

	err = h.store.Delete(id, username)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "access denied") {
			sendError(w, http.StatusNotFound, "Conversation not found")
		} else {
			sendError(w, http.StatusInternalServerError, "Failed to delete conversation")
		}
		return
	}

	sendJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// HandleDeleteAll handles DELETE /api/conversations (with query param all=true)
func (h *Handler) HandleDeleteAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	username, err := h.extractUsername(r)
	if err != nil {
		sendError(w, http.StatusUnauthorized, err.Error())
		return
	}

	count, err := h.store.DeleteAll(username)
	if err != nil {
		sendError(w, http.StatusInternalServerError, "Failed to delete conversations")
		return
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"deleted": count,
	})
}

// RegisterRoutes registers conversation routes with the given mux
func (h *Handler) RegisterRoutes(mux *http.ServeMux, authWrapper func(http.HandlerFunc) http.HandlerFunc) {
	// List conversations
	mux.HandleFunc("/api/conversations", authWrapper(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			h.HandleList(w, r)
		case http.MethodPost:
			h.HandleCreate(w, r)
		case http.MethodDelete:
			// Delete all conversations
			if r.URL.Query().Get("all") == "true" {
				h.HandleDeleteAll(w, r)
			} else {
				sendError(w, http.StatusBadRequest, "Use ?all=true to delete all conversations")
			}
		default:
			sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
		}
	}))

	// Single conversation operations
	mux.HandleFunc("/api/conversations/", authWrapper(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			h.HandleGet(w, r)
		case http.MethodPut:
			h.HandleUpdate(w, r)
		case http.MethodPatch:
			h.HandleRename(w, r)
		case http.MethodDelete:
			h.HandleDelete(w, r)
		default:
			sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
		}
	}))
}
