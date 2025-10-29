/*-------------------------------------------------------------------------
 *
 * pgEdge Postgres MCP Server
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package llm

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestNewClient(t *testing.T) {
	// Save original env vars
	originalAPIKey := os.Getenv("ANTHROPIC_API_KEY")
	originalModel := os.Getenv("ANTHROPIC_MODEL")
	defer func() {
		os.Setenv("ANTHROPIC_API_KEY", originalAPIKey)
		os.Setenv("ANTHROPIC_MODEL", originalModel)
	}()

	t.Run("default model when not set", func(t *testing.T) {
		os.Unsetenv("ANTHROPIC_MODEL")
		os.Setenv("ANTHROPIC_API_KEY", "test-key")

		client := NewClient()
		if client == nil {
			t.Fatal("NewClient() returned nil")
		}
		if client.model != "claude-sonnet-4-5" {
			t.Errorf("model = %q, want %q", client.model, "claude-sonnet-4-5")
		}
		if client.apiKey != "test-key" {
			t.Errorf("apiKey = %q, want %q", client.apiKey, "test-key")
		}
		if client.baseURL != "https://api.anthropic.com/v1" {
			t.Errorf("baseURL = %q, want %q", client.baseURL, "https://api.anthropic.com/v1")
		}
	})

	t.Run("custom model from env", func(t *testing.T) {
		os.Setenv("ANTHROPIC_MODEL", "claude-3-opus-20240229")
		os.Setenv("ANTHROPIC_API_KEY", "test-key-2")

		client := NewClient()
		if client.model != "claude-3-opus-20240229" {
			t.Errorf("model = %q, want %q", client.model, "claude-3-opus-20240229")
		}
	})

	t.Run("no api key", func(t *testing.T) {
		os.Unsetenv("ANTHROPIC_API_KEY")

		client := NewClient()
		if client.apiKey != "" {
			t.Errorf("apiKey = %q, want empty string", client.apiKey)
		}
	})
}

func TestIsConfigured(t *testing.T) {
	tests := []struct {
		name     string
		apiKey   string
		expected bool
	}{
		{
			name:     "with api key",
			apiKey:   "sk-ant-test-key",
			expected: true,
		},
		{
			name:     "without api key",
			apiKey:   "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{
				apiKey: tt.apiKey,
			}

			result := client.IsConfigured()
			if result != tt.expected {
				t.Errorf("IsConfigured() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestConvertNLToSQL_NotConfigured(t *testing.T) {
	client := &Client{
		apiKey: "",
	}

	_, err := client.ConvertNLToSQL("show all users", "schema context")
	if err == nil {
		t.Error("ConvertNLToSQL() expected error when not configured, got nil")
	}
	if !strings.Contains(err.Error(), "ANTHROPIC_API_KEY not set") {
		t.Errorf("ConvertNLToSQL() error = %v, want error containing 'ANTHROPIC_API_KEY not set'", err)
	}
}

func TestConvertNLToSQL_Success(t *testing.T) {
	// Create a mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and headers
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("x-api-key") != "test-api-key" {
			t.Errorf("Expected x-api-key test-api-key, got %s", r.Header.Get("x-api-key"))
		}
		if r.Header.Get("anthropic-version") != "2023-06-01" {
			t.Errorf("Expected anthropic-version 2023-06-01, got %s", r.Header.Get("anthropic-version"))
		}

		// Send mock response
		response := claudeResponse{
			ID:   "msg_123",
			Type: "message",
			Role: "assistant",
			Content: []claudeContentBlock{
				{
					Type: "text",
					Text: "SELECT * FROM users WHERE active = true",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	client := &Client{
		apiKey:  "test-api-key",
		baseURL: server.URL,
		model:   "claude-sonnet-4-5",
	}

	result, err := client.ConvertNLToSQL("show active users", "public.users (TABLE)\n  Columns:\n    - id (integer)\n    - active (boolean)")
	if err != nil {
		t.Fatalf("ConvertNLToSQL() unexpected error: %v", err)
	}

	expected := "SELECT * FROM users WHERE active = true"
	if result != expected {
		t.Errorf("ConvertNLToSQL() = %q, want %q", result, expected)
	}
}

func TestConvertNLToSQL_CleanupSQL(t *testing.T) {
	tests := []struct {
		name         string
		responseText string
		expected     string
	}{
		{
			name:         "plain SQL",
			responseText: "SELECT * FROM users",
			expected:     "SELECT * FROM users",
		},
		{
			name:         "SQL with trailing semicolon",
			responseText: "SELECT * FROM users;",
			expected:     "SELECT * FROM users",
		},
		{
			name:         "SQL with markdown code block",
			responseText: "```sql\nSELECT * FROM users\n```",
			expected:     "SELECT * FROM users",
		},
		{
			name:         "SQL with generic code block",
			responseText: "```\nSELECT * FROM users\n```",
			expected:     "SELECT * FROM users",
		},
		{
			name:         "SQL with whitespace",
			responseText: "  SELECT * FROM users  ",
			expected:     "SELECT * FROM users",
		},
		{
			name:         "SQL with code block and semicolon",
			responseText: "```sql\nSELECT * FROM users;\n```",
			expected:     "SELECT * FROM users",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock HTTP server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				response := claudeResponse{
					ID:   "msg_123",
					Type: "message",
					Role: "assistant",
					Content: []claudeContentBlock{
						{
							Type: "text",
							Text: tt.responseText,
						},
					},
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				if err := json.NewEncoder(w).Encode(response); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
			}))
			defer server.Close()

			client := &Client{
				apiKey:  "test-api-key",
				baseURL: server.URL,
				model:   "claude-sonnet-4-5",
			}

			result, err := client.ConvertNLToSQL("test query", "schema")
			if err != nil {
				t.Fatalf("ConvertNLToSQL() unexpected error: %v", err)
			}

			if result != tt.expected {
				t.Errorf("ConvertNLToSQL() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestConvertNLToSQL_APIError(t *testing.T) {
	// Create a mock HTTP server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		if _, err := w.Write([]byte(`{"error": {"type": "invalid_request_error", "message": "Invalid request"}}`)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	client := &Client{
		apiKey:  "test-api-key",
		baseURL: server.URL,
		model:   "claude-sonnet-4-5",
	}

	_, err := client.ConvertNLToSQL("show users", "schema")
	if err == nil {
		t.Error("ConvertNLToSQL() expected error for API error response, got nil")
	}
	if !strings.Contains(err.Error(), "API returned status 400") {
		t.Errorf("ConvertNLToSQL() error = %v, want error containing 'API returned status 400'", err)
	}
}

func TestConvertNLToSQL_EmptyResponse(t *testing.T) {
	// Create a mock HTTP server that returns empty content
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := claudeResponse{
			ID:      "msg_123",
			Type:    "message",
			Role:    "assistant",
			Content: []claudeContentBlock{},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	client := &Client{
		apiKey:  "test-api-key",
		baseURL: server.URL,
		model:   "claude-sonnet-4-5",
	}

	_, err := client.ConvertNLToSQL("show users", "schema")
	if err == nil {
		t.Error("ConvertNLToSQL() expected error for empty response, got nil")
	}
	if !strings.Contains(err.Error(), "no content in response") {
		t.Errorf("ConvertNLToSQL() error = %v, want error containing 'no content in response'", err)
	}
}

func TestCleanSQL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain SQL",
			input:    "SELECT * FROM users WHERE active = true",
			expected: "SELECT * FROM users WHERE active = true",
		},
		{
			name:     "SQL with trailing semicolon",
			input:    "SELECT * FROM users;",
			expected: "SELECT * FROM users",
		},
		{
			name:     "SQL with markdown code block",
			input:    "```sql\nSELECT * FROM users\n```",
			expected: "SELECT * FROM users",
		},
		{
			name:     "SQL with generic code block",
			input:    "```\nSELECT * FROM users\n```",
			expected: "SELECT * FROM users",
		},
		{
			name:     "SQL with explanatory text before",
			input:    "Here's the SQL query you requested:\n\nSELECT * FROM users WHERE id = 1",
			expected: "SELECT * FROM users WHERE id = 1",
		},
		{
			name:     "SQL with explanatory text after",
			input:    "SELECT * FROM users\n\nThis query will return all users from the database.",
			expected: "SELECT * FROM users",
		},
		{
			name:     "SQL with single-line comments",
			input:    "-- This is a comment\nSELECT * FROM users\n-- Another comment",
			expected: "SELECT * FROM users",
		},
		{
			name:     "SQL with inline comments",
			input:    "SELECT * FROM users -- get all users\nWHERE active = true",
			expected: "SELECT * FROM users WHERE active = true",
		},
		{
			name:     "SQL with multi-line comments",
			input:    "/* This is a\n   multi-line comment */\nSELECT * FROM users",
			expected: "SELECT * FROM users",
		},
		{
			name:     "SQL with embedded multi-line comment",
			input:    "SELECT * FROM /* comment */ users WHERE id = 1",
			expected: "SELECT * FROM users WHERE id = 1",
		},
		{
			name:     "complex SQL with multiple clauses",
			input:    "SELECT u.name, u.email FROM users u WHERE u.active = true ORDER BY u.name",
			expected: "SELECT u.name, u.email FROM users u WHERE u.active = true ORDER BY u.name",
		},
		{
			name:     "SQL with markdown and explanatory text",
			input:    "Here's your query:\n```sql\nSELECT * FROM users WHERE age > 18\n```\nThis will find all adult users.",
			expected: "SELECT * FROM users WHERE age > 18",
		},
		{
			name:     "INSERT statement",
			input:    "INSERT INTO users (name, email) VALUES ('John', 'john@example.com')",
			expected: "INSERT INTO users (name, email) VALUES ('John', 'john@example.com')",
		},
		{
			name:     "UPDATE statement",
			input:    "UPDATE users SET active = false WHERE last_login < '2023-01-01'",
			expected: "UPDATE users SET active = false WHERE last_login < '2023-01-01'",
		},
		{
			name:     "DELETE statement",
			input:    "DELETE FROM users WHERE id = 123",
			expected: "DELETE FROM users WHERE id = 123",
		},
		{
			name:     "WITH (CTE) statement",
			input:    "WITH active_users AS (SELECT * FROM users WHERE active = true) SELECT * FROM active_users",
			expected: "WITH active_users AS (SELECT * FROM users WHERE active = true) SELECT * FROM active_users",
		},
		{
			name:     "SQL with extra whitespace",
			input:    "SELECT   *   FROM    users    WHERE    id = 1",
			expected: "SELECT * FROM users WHERE id = 1",
		},
		{
			name:     "SQL with newlines",
			input:    "SELECT *\nFROM users\nWHERE active = true\nORDER BY name",
			expected: "SELECT * FROM users WHERE active = true ORDER BY name",
		},
		{
			name:     "Empty input",
			input:    "",
			expected: "",
		},
		{
			name:     "Only comments",
			input:    "-- Just a comment\n/* Another comment */",
			expected: "",
		},
		{
			name:     "Only explanatory text",
			input:    "This is just some text explaining what we're going to do.",
			expected: "",
		},
		{
			name:     "SQL with EXPLAIN",
			input:    "EXPLAIN SELECT * FROM users WHERE id = 1",
			expected: "EXPLAIN SELECT * FROM users WHERE id = 1",
		},
		{
			name:     "SQL with ANALYZE",
			input:    "ANALYZE users",
			expected: "ANALYZE users",
		},
		{
			name:     "Multiple statements - only first returned",
			input:    "SELECT * FROM users;\nSELECT * FROM orders;",
			expected: "SELECT * FROM users",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleanSQL(tt.input)
			if result != tt.expected {
				t.Errorf("cleanSQL() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestConvertNLToSQL_WithCleanSQL(t *testing.T) {
	// Test that ConvertNLToSQL properly uses cleanSQL
	tests := []struct {
		name         string
		responseText string
		expected     string
	}{
		{
			name:         "response with explanation",
			responseText: "Here's the query:\n```sql\nSELECT * FROM users\n```",
			expected:     "SELECT * FROM users",
		},
		{
			name:         "response with comments",
			responseText: "-- Get all users\nSELECT * FROM users",
			expected:     "SELECT * FROM users",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				response := claudeResponse{
					ID:   "msg_123",
					Type: "message",
					Role: "assistant",
					Content: []claudeContentBlock{
						{
							Type: "text",
							Text: tt.responseText,
						},
					},
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				if err := json.NewEncoder(w).Encode(response); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
			}))
			defer server.Close()

			client := &Client{
				apiKey:  "test-api-key",
				baseURL: server.URL,
				model:   "claude-sonnet-4-5",
			}

			result, err := client.ConvertNLToSQL("test query", "schema")
			if err != nil {
				t.Fatalf("ConvertNLToSQL() unexpected error: %v", err)
			}

			if result != tt.expected {
				t.Errorf("ConvertNLToSQL() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestConvertNLToSQL_NoValidSQL(t *testing.T) {
	// Test that ConvertNLToSQL returns error when no valid SQL found
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := claudeResponse{
			ID:   "msg_123",
			Type: "message",
			Role: "assistant",
			Content: []claudeContentBlock{
				{
					Type: "text",
					Text: "I cannot generate SQL for that query because the schema is insufficient.",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	client := &Client{
		apiKey:  "test-api-key",
		baseURL: server.URL,
		model:   "claude-sonnet-4-5",
	}

	_, err := client.ConvertNLToSQL("test query", "schema")
	if err == nil {
		t.Error("ConvertNLToSQL() expected error when no valid SQL found, got nil")
	}
	if !strings.Contains(err.Error(), "no valid SQL found") {
		t.Errorf("ConvertNLToSQL() error = %v, want error containing 'no valid SQL found'", err)
	}
}
