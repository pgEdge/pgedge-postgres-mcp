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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// Client handles interactions with LLM APIs (Anthropic or Ollama)
type Client struct {
	provider string // "anthropic" or "ollama"
	apiKey   string // Only for Anthropic
	baseURL  string
	model    string
}

// NewClient creates a new LLM client with the specified provider
func NewClient(provider, apiKey, baseURL, model string) *Client {
	return &Client{
		provider: provider,
		apiKey:   apiKey,
		baseURL:  baseURL,
		model:    model,
	}
}

// IsConfigured returns whether the client is properly configured
func (c *Client) IsConfigured() bool {
	switch c.provider {
	case "anthropic":
		return c.apiKey != ""
	case "ollama":
		return c.baseURL != "" && c.model != ""
	default:
		return false
	}
}

// ConvertNLToSQL converts a natural language query to SQL using the configured LLM
func (c *Client) ConvertNLToSQL(nlQuery, schemaContext string) (string, error) {
	if !c.IsConfigured() {
		return "", fmt.Errorf("LLM client not configured")
	}

	switch c.provider {
	case "anthropic":
		return c.convertWithAnthropic(nlQuery, schemaContext)
	case "ollama":
		return c.convertWithOllama(nlQuery, schemaContext)
	default:
		return "", fmt.Errorf("unsupported LLM provider: %s", c.provider)
	}
}

// convertWithAnthropic uses Anthropic's Claude API
func (c *Client) convertWithAnthropic(nlQuery, schemaContext string) (string, error) {
	prompt := fmt.Sprintf(`You are a PostgreSQL expert. Given the following database schema and a natural language query, generate a SQL query that answers the question.

Database Schema:
%s

Natural Language Query: %s

Requirements:
1. Generate ONLY the SQL query, no explanations or markdown formatting
2. Use proper PostgreSQL syntax
3. Consider the column descriptions and table relationships
4. Use appropriate JOINs when needed
5. Include proper WHERE clauses, GROUP BY, ORDER BY as needed
6. Use meaningful column aliases
7. Make the query efficient and optimized
8. Do NOT include semicolons at the end
9. Return ONLY the SQL query text, nothing else

SQL Query:`, schemaContext, nlQuery)

	reqBody := claudeRequest{
		Model:     c.model,
		MaxTokens: 2048,
		Messages: []claudeMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL+"/messages", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: Failed to close HTTP response body: %v\n", err)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var claudeResp claudeResponse
	if err := json.Unmarshal(body, &claudeResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(claudeResp.Content) == 0 {
		return "", fmt.Errorf("no content in response")
	}

	sqlQuery := strings.TrimSpace(claudeResp.Content[0].Text)

	// Clean and sanitize the SQL query
	sqlQuery = cleanSQL(sqlQuery)
	if sqlQuery == "" {
		return "", fmt.Errorf("no valid SQL found in response")
	}

	return sqlQuery, nil
}

// convertWithOllama uses Ollama's OpenAI-compatible API
func (c *Client) convertWithOllama(nlQuery, schemaContext string) (string, error) {
	prompt := fmt.Sprintf(`You are a PostgreSQL expert. Given the following database schema and a natural language query, generate a SQL query that answers the question.

Database Schema:
%s

Natural Language Query: %s

Requirements:
1. Generate ONLY the SQL query, no explanations or markdown formatting
2. Use proper PostgreSQL syntax
3. Consider the column descriptions and table relationships
4. Use appropriate JOINs when needed
5. Include proper WHERE clauses, GROUP BY, ORDER BY as needed
6. Use meaningful column aliases
7. Make the query efficient and optimized
8. Do NOT include semicolons at the end
9. Return ONLY the SQL query text, nothing else

SQL Query:`, schemaContext, nlQuery)

	reqBody := ollamaRequest{
		Model: c.model,
		Messages: []ollamaMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Stream: false,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL+"/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: Failed to close HTTP response body: %v\n", err)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var ollamaResp ollamaResponse
	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(ollamaResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	sqlQuery := strings.TrimSpace(ollamaResp.Choices[0].Message.Content)

	// Clean and sanitize the SQL query
	sqlQuery = cleanSQL(sqlQuery)
	if sqlQuery == "" {
		return "", fmt.Errorf("no valid SQL found in response")
	}

	return sqlQuery, nil
}

// cleanSQL removes markdown formatting, comments, and explanatory text from SQL
func cleanSQL(input string) string {
	// Remove markdown code blocks
	input = strings.TrimSpace(input)

	// Remove ```sql or ``` at the beginning
	if after, found := strings.CutPrefix(input, "```sql"); found {
		input = after
	} else if after, found := strings.CutPrefix(input, "```"); found {
		input = after
	}

	// Remove ``` at the end
	input = strings.TrimSuffix(input, "```")
	input = strings.TrimSpace(input)

	// Remove multi-line comments /* ... */ (handle them first, before splitting lines)
	for {
		start := strings.Index(input, "/*")
		if start == -1 {
			break
		}
		end := strings.Index(input[start:], "*/")
		if end == -1 {
			break
		}
		end += start + 2 // +2 for the */
		input = input[:start] + " " + input[end:]
	}

	// Split into lines and process
	lines := strings.Split(input, "\n")
	var sqlLines []string
	foundSQL := false
	hitSemicolon := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip empty lines
		if line == "" {
			continue
		}

		// Skip single-line comments
		if strings.HasPrefix(line, "--") {
			continue
		}

		// Remove inline comments
		if idx := strings.Index(line, "--"); idx > 0 {
			line = strings.TrimSpace(line[:idx])
		}

		// Check if line contains a semicolon (end of statement)
		if strings.Contains(line, ";") {
			// Add the part before the semicolon
			parts := strings.SplitN(line, ";", 2)
			line = strings.TrimSpace(parts[0])
			hitSemicolon = true
		}

		// Check if this line looks like SQL (starts with common SQL keywords)
		upperLine := strings.ToUpper(line)
		isSQLStart := strings.HasPrefix(upperLine, "SELECT") ||
			strings.HasPrefix(upperLine, "INSERT") ||
			strings.HasPrefix(upperLine, "UPDATE") ||
			strings.HasPrefix(upperLine, "DELETE") ||
			strings.HasPrefix(upperLine, "WITH") ||
			strings.HasPrefix(upperLine, "CREATE") ||
			strings.HasPrefix(upperLine, "ALTER") ||
			strings.HasPrefix(upperLine, "DROP") ||
			strings.HasPrefix(upperLine, "EXPLAIN") ||
			strings.HasPrefix(upperLine, "ANALYZE")

		// Once we find SQL, keep adding lines
		if isSQLStart {
			foundSQL = true
		}

		// If we've found SQL and this line has content, add it
		if foundSQL && line != "" {
			// Check if this line could be part of SQL (contains typical SQL patterns)
			// or if it's explanatory text
			upperLine := strings.ToUpper(line)

			// If line looks like explanatory text (after SQL started), stop
			if !isSQLStart && (strings.HasPrefix(upperLine, "THIS ") ||
				strings.HasPrefix(upperLine, "THE ") ||
				strings.HasPrefix(upperLine, "WILL ") ||
				strings.HasPrefix(upperLine, "RETURNS ") ||
				strings.HasPrefix(upperLine, "NOTE:") ||
				strings.HasPrefix(upperLine, "EXPLANATION:")) {
				break
			}

			sqlLines = append(sqlLines, line)
		}

		// Stop if we hit a semicolon (end of first statement)
		if hitSemicolon {
			break
		}
	}

	// Join the SQL lines
	result := strings.Join(sqlLines, " ")
	result = strings.TrimSpace(result)

	// Remove any trailing ``` that might have been included
	result = strings.TrimSuffix(result, "```")
	result = strings.TrimSpace(result)

	// Normalize whitespace - replace multiple spaces with single space
	result = strings.Join(strings.Fields(result), " ")

	return result
}

// Internal types for Claude API
type claudeRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	Messages  []claudeMessage `json:"messages"`
}

type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type claudeResponse struct {
	ID      string               `json:"id"`
	Type    string               `json:"type"`
	Role    string               `json:"role"`
	Content []claudeContentBlock `json:"content"`
}

type claudeContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Internal types for Ollama API (OpenAI-compatible)
type ollamaRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []ollamaChoice `json:"choices"`
}

type ollamaChoice struct {
	Index        int           `json:"index"`
	Message      ollamaMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}
