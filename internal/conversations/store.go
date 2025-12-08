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
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite" // Pure Go SQLite driver
)

// Message represents a single message in a conversation
type Message struct {
	Role      string                   `json:"role"`
	Content   interface{}              `json:"content"`
	Timestamp string                   `json:"timestamp,omitempty"`
	Provider  string                   `json:"provider,omitempty"`
	Model     string                   `json:"model,omitempty"`
	Activity  []map[string]interface{} `json:"activity,omitempty"`
	IsError   bool                     `json:"isError,omitempty"`
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

// ConversationSummary provides a lightweight view for listing
type ConversationSummary struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	Connection string    `json:"connection,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	Preview    string    `json:"preview"`
}

// Store manages conversation persistence using SQLite
type Store struct {
	db   *sql.DB
	mu   sync.RWMutex
	path string
}

// NewStore creates a new conversation store
func NewStore(dataDir string) (*Store, error) {
	// Ensure data directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	dbPath := filepath.Join(dataDir, "conversations.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable WAL mode for better concurrent access
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	store := &Store{
		db:   db,
		path: dbPath,
	}

	// Initialize schema
	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return store, nil
}

// initSchema creates the necessary tables
func (s *Store) initSchema() error {
	schema := `
    CREATE TABLE IF NOT EXISTS conversations (
        id TEXT PRIMARY KEY,
        username TEXT NOT NULL,
        title TEXT NOT NULL,
        provider TEXT DEFAULT '',
        model TEXT DEFAULT '',
        connection TEXT DEFAULT '',
        messages TEXT NOT NULL,
        created_at DATETIME NOT NULL,
        updated_at DATETIME NOT NULL
    );

    CREATE INDEX IF NOT EXISTS idx_conversations_username
        ON conversations(username);

    CREATE INDEX IF NOT EXISTS idx_conversations_updated_at
        ON conversations(updated_at DESC);
    `

	_, err := s.db.Exec(schema)
	if err != nil {
		return err
	}

	// Migration: Add provider and model columns if they don't exist
	// SQLite doesn't support IF NOT EXISTS for ALTER TABLE, so we check first
	var count int
	err = s.db.QueryRow(`
        SELECT COUNT(*) FROM pragma_table_info('conversations')
        WHERE name = 'provider'
    `).Scan(&count)
	if err != nil {
		return err
	}

	if count == 0 {
		// Add provider column
		if _, err := s.db.Exec(`ALTER TABLE conversations ADD COLUMN provider TEXT DEFAULT ''`); err != nil {
			return fmt.Errorf("failed to add provider column: %w", err)
		}
		// Add model column
		if _, err := s.db.Exec(`ALTER TABLE conversations ADD COLUMN model TEXT DEFAULT ''`); err != nil {
			return fmt.Errorf("failed to add model column: %w", err)
		}
	}

	// Migration: Add connection column if it doesn't exist
	err = s.db.QueryRow(`
        SELECT COUNT(*) FROM pragma_table_info('conversations')
        WHERE name = 'connection'
    `).Scan(&count)
	if err != nil {
		return err
	}

	if count == 0 {
		if _, err := s.db.Exec(`ALTER TABLE conversations ADD COLUMN connection TEXT DEFAULT ''`); err != nil {
			return fmt.Errorf("failed to add connection column: %w", err)
		}
	}

	return nil
}

// Close closes the database connection
func (s *Store) Close() error {
	return s.db.Close()
}

// generateID creates a unique conversation ID
func generateID() string {
	return fmt.Sprintf("conv_%d", time.Now().UnixNano())
}

// generateTitle creates a title from the first user message
func generateTitle(messages []Message) string {
	for _, msg := range messages {
		if msg.Role != "user" {
			continue
		}

		content := ""
		switch c := msg.Content.(type) {
		case string:
			content = c
		default:
			// For non-string content, use a default
			content = "New conversation"
		}
		// Truncate to reasonable length
		if len(content) > 50 {
			return content[:47] + "..."
		}
		if content == "" {
			return "New conversation"
		}
		return content
	}
	return "New conversation"
}

// Create creates a new conversation
func (s *Store) Create(username, provider, model, connection string, messages []Message) (*Conversation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	conv := &Conversation{
		ID:         generateID(),
		Username:   username,
		Title:      generateTitle(messages),
		Provider:   provider,
		Model:      model,
		Connection: connection,
		Messages:   messages,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}

	messagesJSON, err := json.Marshal(messages)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal messages: %w", err)
	}

	_, err = s.db.Exec(
		`INSERT INTO conversations (id, username, title, provider, model, connection, messages, created_at, updated_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		conv.ID, conv.Username, conv.Title, conv.Provider, conv.Model, conv.Connection, string(messagesJSON),
		conv.CreatedAt, conv.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert conversation: %w", err)
	}

	return conv, nil
}

// Update updates an existing conversation
func (s *Store) Update(id, username, provider, model, connection string, messages []Message) (*Conversation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Verify ownership
	var existingUsername string
	err := s.db.QueryRow(
		"SELECT username FROM conversations WHERE id = ?", id,
	).Scan(&existingUsername)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("conversation not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query conversation: %w", err)
	}
	if existingUsername != username {
		return nil, fmt.Errorf("access denied")
	}

	messagesJSON, err := json.Marshal(messages)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal messages: %w", err)
	}

	title := generateTitle(messages)
	updatedAt := time.Now().UTC()

	_, err = s.db.Exec(
		`UPDATE conversations
         SET title = ?, provider = ?, model = ?, connection = ?, messages = ?, updated_at = ?
         WHERE id = ? AND username = ?`,
		title, provider, model, connection, string(messagesJSON), updatedAt, id, username,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update conversation: %w", err)
	}

	// Fetch updated conversation
	return s.getUnlocked(id, username)
}

// Get retrieves a conversation by ID
func (s *Store) Get(id, username string) (*Conversation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.getUnlocked(id, username)
}

// getUnlocked retrieves a conversation without acquiring a lock (caller must hold lock)
func (s *Store) getUnlocked(id, username string) (*Conversation, error) {
	var conv Conversation
	var messagesJSON string

	err := s.db.QueryRow(
		`SELECT id, username, title, provider, model, connection, messages, created_at, updated_at
         FROM conversations
         WHERE id = ? AND username = ?`,
		id, username,
	).Scan(&conv.ID, &conv.Username, &conv.Title, &conv.Provider, &conv.Model, &conv.Connection,
		&messagesJSON, &conv.CreatedAt, &conv.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("conversation not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query conversation: %w", err)
	}

	if err := json.Unmarshal([]byte(messagesJSON), &conv.Messages); err != nil {
		return nil, fmt.Errorf("failed to unmarshal messages: %w", err)
	}

	return &conv, nil
}

// List lists all conversations for a user
func (s *Store) List(username string, limit, offset int) ([]ConversationSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	rows, err := s.db.Query(
		`SELECT id, title, connection, messages, created_at, updated_at
         FROM conversations
         WHERE username = ?
         ORDER BY updated_at DESC
         LIMIT ? OFFSET ?`,
		username, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query conversations: %w", err)
	}
	defer rows.Close()

	var summaries []ConversationSummary
	for rows.Next() {
		var summary ConversationSummary
		var messagesJSON string

		if err := rows.Scan(&summary.ID, &summary.Title, &summary.Connection, &messagesJSON,
			&summary.CreatedAt, &summary.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Extract preview from first user message
		var messages []Message
		if err := json.Unmarshal([]byte(messagesJSON), &messages); err == nil {
			for _, msg := range messages {
				if msg.Role == "user" {
					if content, ok := msg.Content.(string); ok {
						if len(content) > 100 {
							summary.Preview = content[:97] + "..."
						} else {
							summary.Preview = content
						}
						break
					}
				}
			}
		}

		summaries = append(summaries, summary)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return summaries, nil
}

// Rename renames a conversation
func (s *Store) Rename(id, username, title string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	result, err := s.db.Exec(
		`UPDATE conversations SET title = ?, updated_at = ?
         WHERE id = ? AND username = ?`,
		title, time.Now().UTC(), id, username,
	)
	if err != nil {
		return fmt.Errorf("failed to rename conversation: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("conversation not found or access denied")
	}

	return nil
}

// Delete deletes a conversation
func (s *Store) Delete(id, username string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	result, err := s.db.Exec(
		"DELETE FROM conversations WHERE id = ? AND username = ?",
		id, username,
	)
	if err != nil {
		return fmt.Errorf("failed to delete conversation: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("conversation not found or access denied")
	}

	return nil
}

// DeleteAll deletes all conversations for a user
func (s *Store) DeleteAll(username string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	result, err := s.db.Exec(
		"DELETE FROM conversations WHERE username = ?",
		username,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to delete conversations: %w", err)
	}

	return result.RowsAffected()
}
