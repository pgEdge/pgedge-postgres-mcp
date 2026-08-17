/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package tracing

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// parseJSONLFile reads a JSONL file and returns a slice of parsed entries.
func parseJSONLFile(t *testing.T, path string) []map[string]interface{} {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file %s: %v", path, err)
	}

	var entries []map[string]interface{}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("failed to parse JSON line %q: %v", line, err)
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scanner error: %v", err)
	}

	return entries
}

func TestInitializeEmpty(t *testing.T) {
	ResetForTesting()

	Initialize("", false)

	if IsEnabled() {
		t.Error("expected tracing to be disabled after Initialize(\"\")")
	}
}

func TestInitializeValid(t *testing.T) {
	ResetForTesting()

	tracePath := filepath.Join(t.TempDir(), "trace.jsonl")
	Initialize(tracePath, false)
	defer Close()

	if !IsEnabled() {
		t.Error("expected tracing to be enabled after Initialize with valid path")
	}

	if _, err := os.Stat(tracePath); os.IsNotExist(err) {
		t.Errorf("expected trace file to exist at %s", tracePath)
	}

	if got := GetFilePath(); got != tracePath {
		t.Errorf("GetFilePath() = %q, want %q", got, tracePath)
	}
}

func TestLogWhenDisabled(t *testing.T) {
	ResetForTesting()

	// These calls should not panic when tracing is disabled.
	LogToolCall("session-1", "tok", "req1", "test-tool", map[string]interface{}{"key": "val"})
	LogToolResult("session-1", "tok", "req1", "test-tool", "result-data", nil, 100*time.Millisecond)
	LogResourceRead("session-1", "tok", "req1", "resource://test")
	LogResourceResult("session-1", "tok", "req1", "resource://test", nil, nil, 50*time.Millisecond)
	LogPromptCall("session-1", "tok", "req1", "prompt-name", map[string]string{})
	LogPromptResult("session-1", "tok", "req1", "prompt-name", nil, nil, 75*time.Millisecond)
	LogDatabaseSwitch("session-1", "tok", "req1", "newdb", map[string]interface{}{})
	LogConfigReload("session-1", map[string]interface{}{})
	LogHTTPRequest("session-1", "tok", "req1", "GET", "/api/test", nil)
	LogHTTPResponse("session-1", "tok", "req1", "GET", "/api/test", 200, nil, 10*time.Millisecond)
	LogSessionStart("session-1", "tok", map[string]interface{}{})
	LogSessionEnd("session-1", "tok", map[string]interface{}{})
	LogError("session-1", "tok", "req1", "test-context", fmt.Errorf("test error"))
	LogUserPrompt("session-1", "tok", "req1", "hello")
	LogLLMResponse("session-1", "tok", "req1", "world", 200*time.Millisecond)
}

func TestLogToolCall(t *testing.T) {
	ResetForTesting()

	tracePath := filepath.Join(t.TempDir(), "trace.jsonl")
	Initialize(tracePath, false)

	params := map[string]interface{}{
		"query": "SELECT 1",
		"limit": 10,
	}
	LogToolCall("sess-abc", "tok-hash", "req-001", "query_database", params)

	Close()

	entries := parseJSONLFile(t, tracePath)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]

	if entry["type"] != "tool_call" {
		t.Errorf("type = %v, want \"tool_call\"", entry["type"])
	}
	if entry["name"] != "query_database" {
		t.Errorf("name = %v, want \"query_database\"", entry["name"])
	}
	if entry["session_id"] != "sess-abc" {
		t.Errorf("session_id = %v, want \"sess-abc\"", entry["session_id"])
	}
	if _, ok := entry["parameters"]; !ok {
		t.Error("expected \"parameters\" field to be present")
	}
	if _, ok := entry["timestamp"]; !ok {
		t.Error("expected \"timestamp\" field to be present")
	}
}

func TestLogToolResult(t *testing.T) {
	t.Run("without error", func(t *testing.T) {
		ResetForTesting()

		tracePath := filepath.Join(t.TempDir(), "trace.jsonl")
		Initialize(tracePath, false)

		LogToolResult("sess-1", "tok", "req1", "query_database", "some result", nil, 250*time.Millisecond)

		Close()

		entries := parseJSONLFile(t, tracePath)
		if len(entries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(entries))
		}

		entry := entries[0]

		if entry["type"] != "tool_result" {
			t.Errorf("type = %v, want \"tool_result\"", entry["type"])
		}
		if entry["name"] != "query_database" {
			t.Errorf("name = %v, want \"query_database\"", entry["name"])
		}
		if _, ok := entry["error"]; ok {
			t.Error("expected no \"error\" field when err is nil")
		}

		durationMs, ok := entry["duration_ms"].(float64)
		if !ok {
			t.Fatalf("duration_ms is not a number: %T", entry["duration_ms"])
		}
		if int(durationMs) != 250 {
			t.Errorf("duration_ms = %v, want 250", durationMs)
		}
	})

	t.Run("with error", func(t *testing.T) {
		ResetForTesting()

		tracePath := filepath.Join(t.TempDir(), "trace.jsonl")
		Initialize(tracePath, false)

		testErr := fmt.Errorf("connection refused")
		LogToolResult("sess-2", "tok", "req2", "query_database", "", testErr, 100*time.Millisecond)

		Close()

		entries := parseJSONLFile(t, tracePath)
		if len(entries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(entries))
		}

		entry := entries[0]

		if entry["type"] != "tool_result" {
			t.Errorf("type = %v, want \"tool_result\"", entry["type"])
		}
		errField, ok := entry["error"].(string)
		if !ok {
			t.Fatalf("expected \"error\" field to be a string, got %T", entry["error"])
		}
		if errField != "connection refused" {
			t.Errorf("error = %q, want \"connection refused\"", errField)
		}
	})
}

func TestLogResourceReadResult(t *testing.T) {
	ResetForTesting()

	tracePath := filepath.Join(t.TempDir(), "trace.jsonl")
	Initialize(tracePath, false)

	LogResourceRead("sess-r", "tok", "req1", "resource://pg/tables")
	LogResourceResult("sess-r", "tok", "req1", "resource://pg/tables", nil, nil, 30*time.Millisecond)

	Close()

	entries := parseJSONLFile(t, tracePath)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	if entries[0]["type"] != "resource_read" {
		t.Errorf("first entry type = %v, want \"resource_read\"", entries[0]["type"])
	}
	if entries[1]["type"] != "resource_result" {
		t.Errorf("second entry type = %v, want \"resource_result\"", entries[1]["type"])
	}
}

func TestLogPromptCallResult(t *testing.T) {
	ResetForTesting()

	tracePath := filepath.Join(t.TempDir(), "trace.jsonl")
	Initialize(tracePath, false)

	LogPromptCall("sess-p", "tok", "req1", "explain_query", map[string]string{"sql": "SELECT 1"})
	LogPromptResult("sess-p", "tok", "req1", "explain_query", nil, nil, 60*time.Millisecond)

	Close()

	entries := parseJSONLFile(t, tracePath)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	if entries[0]["type"] != "prompt_call" {
		t.Errorf("first entry type = %v, want \"prompt_call\"", entries[0]["type"])
	}
	if entries[1]["type"] != "prompt_result" {
		t.Errorf("second entry type = %v, want \"prompt_result\"", entries[1]["type"])
	}
}

func TestLogDatabaseSwitch(t *testing.T) {
	ResetForTesting()

	tracePath := filepath.Join(t.TempDir(), "trace.jsonl")
	Initialize(tracePath, false)

	metadata := map[string]interface{}{
		"previous": "db1",
		"reason":   "user request",
	}
	LogDatabaseSwitch("sess-db", "tok", "req1", "db2", metadata)

	Close()

	entries := parseJSONLFile(t, tracePath)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry["type"] != "database_switch" {
		t.Errorf("type = %v, want \"database_switch\"", entry["type"])
	}
	if entry["name"] != "db2" {
		t.Errorf("name = %v, want \"db2\"", entry["name"])
	}
	if _, ok := entry["metadata"]; !ok {
		t.Error("expected \"metadata\" field to be present")
	}
}

func TestLogConfigReload(t *testing.T) {
	ResetForTesting()

	tracePath := filepath.Join(t.TempDir(), "trace.jsonl")
	Initialize(tracePath, false)

	metadata := map[string]interface{}{
		"source": "file_watcher",
		"path":   "/etc/config.yaml",
	}
	LogConfigReload("sess-cfg", metadata)

	Close()

	entries := parseJSONLFile(t, tracePath)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry["type"] != "config_reload" {
		t.Errorf("type = %v, want \"config_reload\"", entry["type"])
	}
	if _, ok := entry["metadata"]; !ok {
		t.Error("expected \"metadata\" field to be present")
	}
}

func TestLogHTTPRequestResponse(t *testing.T) {
	ResetForTesting()

	tracePath := filepath.Join(t.TempDir(), "trace.jsonl")
	Initialize(tracePath, false)

	LogHTTPRequest("sess-http", "tok", "req1", "POST", "/api/v1/query", nil)
	LogHTTPResponse("sess-http", "tok", "req1", "POST", "/api/v1/query", 200, nil, 45*time.Millisecond)

	Close()

	entries := parseJSONLFile(t, tracePath)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// Verify HTTP request entry.
	reqEntry := entries[0]
	if reqEntry["type"] != "http_request" {
		t.Errorf("request type = %v, want \"http_request\"", reqEntry["type"])
	}

	// The name field contains "method path" (e.g., "POST /api/v1/query").
	nameStr, ok := reqEntry["name"].(string)
	if !ok {
		t.Fatalf("expected name field to be a string, got %T", reqEntry["name"])
	}
	if !strings.Contains(nameStr, "POST") {
		t.Error("expected method \"POST\" in http_request name field")
	}
	if !strings.Contains(nameStr, "/api/v1/query") {
		t.Error("expected path \"/api/v1/query\" in http_request name field")
	}

	// Verify HTTP response entry.
	respEntry := entries[1]
	if respEntry["type"] != "http_response" {
		t.Errorf("response type = %v, want \"http_response\"", respEntry["type"])
	}

	// Check that status code 200 is present somewhere in the entry.
	foundStatus := false
	for key, val := range respEntry {
		if valNum, ok := val.(float64); ok && int(valNum) == 200 {
			foundStatus = true
			break
		}
		if key == "metadata" {
			if meta, ok := val.(map[string]interface{}); ok {
				if sc, ok := meta["status_code"].(float64); ok && int(sc) == 200 {
					foundStatus = true
				}
				if sc, ok := meta["status"].(float64); ok && int(sc) == 200 {
					foundStatus = true
				}
			}
		}
	}
	if !foundStatus {
		t.Error("expected status code 200 in http_response entry metadata")
	}
}

func TestLogSessionStartEnd(t *testing.T) {
	ResetForTesting()

	tracePath := filepath.Join(t.TempDir(), "trace.jsonl")
	Initialize(tracePath, false)

	startMeta := map[string]interface{}{
		"transport": "stdio",
		"client":    "test-client",
	}
	endMeta := map[string]interface{}{
		"reason": "client disconnect",
	}

	LogSessionStart("sess-life", "tok", startMeta)
	LogSessionEnd("sess-life", "tok", endMeta)

	Close()

	entries := parseJSONLFile(t, tracePath)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	if entries[0]["type"] != "session_start" {
		t.Errorf("first entry type = %v, want \"session_start\"", entries[0]["type"])
	}
	if _, ok := entries[0]["metadata"]; !ok {
		t.Error("expected \"metadata\" field in session_start entry")
	}

	if entries[1]["type"] != "session_end" {
		t.Errorf("second entry type = %v, want \"session_end\"", entries[1]["type"])
	}
	if _, ok := entries[1]["metadata"]; !ok {
		t.Error("expected \"metadata\" field in session_end entry")
	}
}

func TestLogError(t *testing.T) {
	ResetForTesting()

	tracePath := filepath.Join(t.TempDir(), "trace.jsonl")
	Initialize(tracePath, false)

	LogError("sess-err", "tok", "req1", "database_query", fmt.Errorf("relation \"foo\" does not exist"))

	Close()

	entries := parseJSONLFile(t, tracePath)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry["type"] != "error" {
		t.Errorf("type = %v, want \"error\"", entry["type"])
	}
	if entry["name"] != "database_query" {
		t.Errorf("name = %v, want \"database_query\"", entry["name"])
	}
	errField, ok := entry["error"].(string)
	if !ok {
		t.Fatalf("expected \"error\" field to be a string, got %T", entry["error"])
	}
	if errField != "relation \"foo\" does not exist" {
		t.Errorf("error = %q, want %q", errField, "relation \"foo\" does not exist")
	}
}

func TestLogUserPromptAndLLMResponse(t *testing.T) {
	ResetForTesting()

	tracePath := filepath.Join(t.TempDir(), "trace.jsonl")
	Initialize(tracePath, false)

	LogUserPrompt("sess-llm", "tok", "req1", "What tables exist in the database?")
	LogLLMResponse("sess-llm", "tok", "req1", "The database contains the following tables...", 1500*time.Millisecond)

	Close()

	entries := parseJSONLFile(t, tracePath)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	if entries[0]["type"] != "user_prompt" {
		t.Errorf("first entry type = %v, want \"user_prompt\"", entries[0]["type"])
	}

	if entries[1]["type"] != "llm_response" {
		t.Errorf("second entry type = %v, want \"llm_response\"", entries[1]["type"])
	}

	// Verify duration is present in the LLM response entry.
	durationMs, ok := entries[1]["duration_ms"].(float64)
	if !ok {
		t.Fatalf("duration_ms is not a number in llm_response: %T", entries[1]["duration_ms"])
	}
	if int(durationMs) != 1500 {
		t.Errorf("duration_ms = %v, want 1500", durationMs)
	}
}

func TestGenerateSessionID(t *testing.T) {
	id1 := GenerateSessionID()
	id2 := GenerateSessionID()

	if !strings.HasPrefix(id1, "stdio-") {
		t.Errorf("session ID %q does not start with \"stdio-\"", id1)
	}
	if !strings.HasPrefix(id2, "stdio-") {
		t.Errorf("session ID %q does not start with \"stdio-\"", id2)
	}
	if id1 == id2 {
		t.Errorf("two calls to GenerateSessionID produced the same value: %q", id1)
	}
}

func TestGenerateRequestID(t *testing.T) {
	id1 := GenerateRequestID()
	id2 := GenerateRequestID()

	if id1 == "" {
		t.Error("GenerateRequestID returned an empty string")
	}
	if id2 == "" {
		t.Error("GenerateRequestID returned an empty string")
	}
	if id1 == id2 {
		t.Errorf("two calls to GenerateRequestID produced the same value: %q", id1)
	}
}

func TestDurationSerialization(t *testing.T) {
	ResetForTesting()

	tracePath := filepath.Join(t.TempDir(), "trace.jsonl")
	Initialize(tracePath, false)

	// 1.5 seconds should serialize as duration_ms=1500 (integer).
	LogToolResult("sess-dur", "tok", "req1", "slow_tool", "done", nil, 1500*time.Millisecond)

	Close()

	entries := parseJSONLFile(t, tracePath)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]
	durationMs, ok := entry["duration_ms"].(float64)
	if !ok {
		t.Fatalf("duration_ms is not a number: %T", entry["duration_ms"])
	}

	// Verify the value is 1500 and is an integer (no fractional part).
	if durationMs != 1500 {
		t.Errorf("duration_ms = %v, want 1500", durationMs)
	}
	if durationMs != float64(int(durationMs)) {
		t.Errorf("duration_ms = %v is not an integer value", durationMs)
	}
}

func TestZeroDurationNotOmitted(t *testing.T) {
	ResetForTesting()

	tracePath := filepath.Join(t.TempDir(), "trace.jsonl")
	Initialize(tracePath, false)

	// A zero-duration result must still include duration_ms: 0
	// so that "not measured" (nil, omitted) and "instant" (0) are
	// distinguishable.
	LogToolResult("sess-zero", "tok", "req1", "fast_tool", "ok", nil, 0)

	Close()

	entries := parseJSONLFile(t, tracePath)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]
	raw, exists := entry["duration_ms"]
	if !exists {
		t.Fatal("duration_ms field is missing; 0ms should still be serialized")
	}

	durationMs, ok := raw.(float64)
	if !ok {
		t.Fatalf("duration_ms is not a number: %T", raw)
	}
	if durationMs != 0 {
		t.Errorf("duration_ms = %v, want 0", durationMs)
	}
}

func TestHTMLEscapingDisabled(t *testing.T) {
	ResetForTesting()

	tracePath := filepath.Join(t.TempDir(), "trace.jsonl")
	Initialize(tracePath, false)

	params := map[string]interface{}{
		"input": "<script>alert(1)</script>",
	}
	LogToolCall("sess-xss", "tok", "req1", "test_tool", params)

	Close()

	// Read the raw file content to check for escaped characters.
	raw, err := os.ReadFile(tracePath)
	if err != nil {
		t.Fatalf("failed to read trace file: %v", err)
	}

	content := string(raw)
	if strings.Contains(content, `\u003c`) {
		t.Error("found escaped \\u003c; expected raw < character")
	}
	if strings.Contains(content, `\u003e`) {
		t.Error("found escaped \\u003e; expected raw > character")
	}
	if !strings.Contains(content, "<script>alert(1)</script>") {
		t.Error("expected raw HTML characters in trace output")
	}
}

func TestFilePermissions(t *testing.T) {
	ResetForTesting()

	tracePath := filepath.Join(t.TempDir(), "trace.jsonl")
	Initialize(tracePath, false)
	defer Close()

	info, err := os.Stat(tracePath)
	if err != nil {
		t.Fatalf("failed to stat trace file: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("file permissions = %04o, want 0600", perm)
	}
}

func TestConcurrentWrites(t *testing.T) {
	ResetForTesting()

	tracePath := filepath.Join(t.TempDir(), "trace.jsonl")
	Initialize(tracePath, false)

	const goroutineCount = 100
	var wg sync.WaitGroup
	wg.Add(goroutineCount)

	for i := 0; i < goroutineCount; i++ {
		go func(idx int) {
			defer wg.Done()
			toolName := fmt.Sprintf("tool_%d", idx)
			params := map[string]interface{}{
				"index": idx,
			}
			LogToolCall("sess-concurrent", "tok", fmt.Sprintf("req-%d", idx), toolName, params)
		}(i)
	}

	wg.Wait()
	Close()

	entries := parseJSONLFile(t, tracePath)
	if len(entries) != goroutineCount {
		t.Errorf("expected %d entries, got %d", goroutineCount, len(entries))
	}

	// Verify every entry is valid JSON with the correct type.
	for i, entry := range entries {
		if entry["type"] != "tool_call" {
			t.Errorf("entry %d: type = %v, want \"tool_call\"", i, entry["type"])
		}
	}
}

func TestResetForTesting(t *testing.T) {
	ResetForTesting()

	// Initialize and verify enabled.
	tracePath1 := filepath.Join(t.TempDir(), "trace1.jsonl")
	Initialize(tracePath1, false)

	if !IsEnabled() {
		t.Error("expected tracing to be enabled after Initialize")
	}

	Close()

	// Reset and verify disabled.
	ResetForTesting()

	if IsEnabled() {
		t.Error("expected tracing to be disabled after ResetForTesting")
	}

	// Initialize again with a new path and verify enabled.
	tracePath2 := filepath.Join(t.TempDir(), "trace2.jsonl")
	Initialize(tracePath2, false)
	defer Close()

	if !IsEnabled() {
		t.Error("expected tracing to be enabled after second Initialize")
	}

	if got := GetFilePath(); got != tracePath2 {
		t.Errorf("GetFilePath() = %q, want %q", got, tracePath2)
	}
}

func TestContextHelpers(t *testing.T) {
	t.Run("WithRequestID round-trip", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithRequestID(ctx, "req-123")

		got := GetRequestIDFromContext(ctx)
		if got != "req-123" {
			t.Errorf("GetRequestIDFromContext() = %q, want \"req-123\"", got)
		}
	})

	t.Run("WithSessionID round-trip", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithSessionID(ctx, "sess-456")

		got := GetSessionIDFromContext(ctx)
		if got != "sess-456" {
			t.Errorf("GetSessionIDFromContext() = %q, want \"sess-456\"", got)
		}
	})

	t.Run("empty context returns empty string for request ID", func(t *testing.T) {
		ctx := context.Background()

		got := GetRequestIDFromContext(ctx)
		if got != "" {
			t.Errorf("GetRequestIDFromContext(empty) = %q, want \"\"", got)
		}
	})

	t.Run("empty context returns empty string for session ID", func(t *testing.T) {
		ctx := context.Background()

		got := GetSessionIDFromContext(ctx)
		if got != "" {
			t.Errorf("GetSessionIDFromContext(empty) = %q, want \"\"", got)
		}
	})

	t.Run("both IDs in same context", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithRequestID(ctx, "req-789")
		ctx = WithSessionID(ctx, "sess-012")

		reqID := GetRequestIDFromContext(ctx)
		sessID := GetSessionIDFromContext(ctx)

		if reqID != "req-789" {
			t.Errorf("GetRequestIDFromContext() = %q, want \"req-789\"", reqID)
		}
		if sessID != "sess-012" {
			t.Errorf("GetSessionIDFromContext() = %q, want \"sess-012\"", sessID)
		}
	})
}

func TestSanitizeParams(t *testing.T) {
	t.Run("nil params", func(t *testing.T) {
		result := sanitizeParams(nil)
		if result != nil {
			t.Errorf("sanitizeParams(nil) = %v, want nil", result)
		}
	})

	t.Run("no sensitive keys", func(t *testing.T) {
		params := map[string]interface{}{
			"query": "SELECT 1",
			"name":  "test_tool",
		}
		result := sanitizeParams(params)
		if result["query"] != "SELECT 1" || result["name"] != "test_tool" {
			t.Errorf("sanitizeParams should not redact non-sensitive keys")
		}
	})

	t.Run("redacts sensitive keys", func(t *testing.T) {
		params := map[string]interface{}{
			"username": "admin",
			"password": "secret123",
			"query":    "SELECT 1",
		}
		result := sanitizeParams(params)
		if result["password"] != "[REDACTED]" {
			t.Errorf("expected password to be redacted, got %v", result["password"])
		}
		if result["username"] != "admin" {
			t.Errorf("expected username to be preserved, got %v", result["username"])
		}
		if result["query"] != "SELECT 1" {
			t.Errorf("expected query to be preserved, got %v", result["query"])
		}
	})

	t.Run("case insensitive", func(t *testing.T) {
		params := map[string]interface{}{
			"Password":      "secret",
			"SESSION_TOKEN": "tok123",
			"API_KEY":       "key456",
		}
		result := sanitizeParams(params)
		// Note: sensitiveParamKeys uses ToLower, so case-insensitive
		if result["Password"] != "[REDACTED]" {
			t.Errorf("expected Password to be redacted, got %v", result["Password"])
		}
	})
}

func TestSanitizeResult(t *testing.T) {
	t.Run("non-map result passes through", func(t *testing.T) {
		result := sanitizeResult("plain string")
		if result != "plain string" {
			t.Errorf("expected string to pass through, got %v", result)
		}
	})

	t.Run("map result with sensitive keys", func(t *testing.T) {
		input := map[string]interface{}{
			"success":       true,
			"session_token": "tok-abc-123",
			"message":       "ok",
		}
		result := sanitizeResult(input)
		m := result.(map[string]interface{})
		if m["session_token"] != "[REDACTED]" {
			t.Errorf("expected session_token to be redacted, got %v", m["session_token"])
		}
		if m["success"] != true || m["message"] != "ok" {
			t.Errorf("expected non-sensitive fields to be preserved")
		}
	})
}

func TestSanitizationInLogToolCall(t *testing.T) {
	ResetForTesting()

	tracePath := filepath.Join(t.TempDir(), "trace.jsonl")
	Initialize(tracePath, false)
	defer Close()

	LogToolCall("sess1", "hash1", "req1", "authenticate_user", map[string]interface{}{
		"username": "admin",
		"password": "supersecret",
	})

	Close()

	entries := parseJSONLFile(t, tracePath)
	if len(entries) == 0 {
		t.Fatal("expected at least one entry")
	}

	params, ok := entries[0]["parameters"].(map[string]interface{})
	if !ok {
		t.Fatal("expected parameters in entry")
	}
	if params["password"] != "[REDACTED]" {
		t.Errorf("expected password to be redacted in trace, got %v", params["password"])
	}
	if params["username"] != "admin" {
		t.Errorf("expected username to be preserved in trace, got %v", params["username"])
	}
}

// assertNoParametersOrResult fails the test if any entry carries a
// "parameters" or "result" field, as metadata-only mode requires.
func assertNoParametersOrResult(t *testing.T, entries []map[string]interface{}) {
	t.Helper()
	for _, entry := range entries {
		if _, ok := entry["parameters"]; ok {
			t.Errorf("entry %v: expected no \"parameters\" field in metadata-only mode", entry["type"])
		}
		if _, ok := entry["result"]; ok {
			t.Errorf("entry %v: expected no \"result\" field in metadata-only mode", entry["type"])
		}
	}
}

func TestMetadataOnlyOmitsParametersAndResult(t *testing.T) {
	ResetForTesting()

	tracePath := filepath.Join(t.TempDir(), "trace.jsonl")
	Initialize(tracePath, true)
	defer Close()

	LogToolCall("sess-meta", "tok-hash", "req-meta", "query_database", map[string]interface{}{
		"query": "SELECT * FROM customers",
	})
	LogToolResult("sess-meta", "tok-hash", "req-meta", "query_database", []map[string]interface{}{
		{"id": 1, "name": "Jane Doe"},
	}, nil, 42*time.Millisecond)
	LogResourceResult("sess-meta", "tok-hash", "req-meta", "resource://pg/tables", "sensitive rows", nil, 10*time.Millisecond)
	LogPromptResult("sess-meta", "tok-hash", "req-meta", "explain_query", "sensitive explanation", nil, 5*time.Millisecond)

	Close()

	entries := parseJSONLFile(t, tracePath)
	if len(entries) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(entries))
	}

	assertNoParametersOrResult(t, entries)

	toolCall := entries[0]
	if toolCall["type"] != "tool_call" {
		t.Errorf("type = %v, want \"tool_call\"", toolCall["type"])
	}
	if toolCall["name"] != "query_database" {
		t.Errorf("name = %v, want \"query_database\"", toolCall["name"])
	}
	if toolCall["session_id"] != "sess-meta" {
		t.Errorf("session_id = %v, want \"sess-meta\"", toolCall["session_id"])
	}

	toolResult := entries[1]
	durationMs, ok := toolResult["duration_ms"].(float64)
	if !ok || int(durationMs) != 42 {
		t.Errorf("duration_ms = %v, want 42", toolResult["duration_ms"])
	}
}

func TestMetadataOnlyRedactsErrorText(t *testing.T) {
	ResetForTesting()

	tracePath := filepath.Join(t.TempDir(), "trace.jsonl")
	Initialize(tracePath, true)
	defer Close()

	testErr := fmt.Errorf(`duplicate key value violates unique constraint "users_email_key" (Key (email)=(jane@example.com) already exists)`)
	LogToolResult("sess-err", "tok-hash", "req-err", "query_database", nil, testErr, 5*time.Millisecond)

	Close()

	entries := parseJSONLFile(t, tracePath)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	errField, ok := entries[0]["error"].(string)
	if !ok {
		t.Fatalf("expected \"error\" field to be a string, got %T", entries[0]["error"])
	}
	if errField != metadataOnlyErrorPlaceholder {
		t.Errorf("error = %q, want generic placeholder %q", errField, metadataOnlyErrorPlaceholder)
	}
	if strings.Contains(errField, "jane@example.com") {
		t.Error("expected error text to be redacted, but it leaked the underlying error message")
	}
}

func TestMetadataOnlyFalsePreservesParametersAndResult(t *testing.T) {
	ResetForTesting()

	tracePath := filepath.Join(t.TempDir(), "trace.jsonl")
	Initialize(tracePath, false)
	defer Close()

	LogToolCall("sess-full", "tok-hash", "req-full", "query_database", map[string]interface{}{
		"query": "SELECT 1",
	})

	Close()

	entries := parseJSONLFile(t, tracePath)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if _, ok := entries[0]["parameters"]; !ok {
		t.Error("expected \"parameters\" field to be present when metadata-only is disabled")
	}
}

// TestSetMetadataOnlyTakesEffectWithoutRestart is the regression test
// for the scenario a config reload is supposed to support: an operator
// turns metadata-only mode on (or off) on an already-running server,
// without restarting it. Before SetMetadataOnly existed, the tracer's
// metadataOnly flag was set once in Initialize and never touched
// again, so an entry logged after a config reload that flipped
// trace_metadata_only would silently keep using the tracer's original
// setting -- the reload would report success while quietly leaving the
// old behavior in place.
func TestSetMetadataOnlyTakesEffectWithoutRestart(t *testing.T) {
	ResetForTesting()

	tracePath := filepath.Join(t.TempDir(), "trace.jsonl")
	Initialize(tracePath, false)
	defer Close()

	LogToolCall("sess-a", "tok", "req1", "query_database", map[string]interface{}{
		"query": "SELECT 1",
	})

	SetMetadataOnly(true)

	LogToolCall("sess-b", "tok", "req2", "query_database", map[string]interface{}{
		"query": "SELECT 2",
	})

	SetMetadataOnly(false)

	LogToolCall("sess-c", "tok", "req3", "query_database", map[string]interface{}{
		"query": "SELECT 3",
	})

	Close()

	entries := parseJSONLFile(t, tracePath)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if _, ok := entries[0]["parameters"]; !ok {
		t.Error("entry logged before SetMetadataOnly(true): expected parameters to be present")
	}
	if _, ok := entries[1]["parameters"]; ok {
		t.Error("entry logged after SetMetadataOnly(true): expected parameters to be omitted")
	}
	if _, ok := entries[2]["parameters"]; !ok {
		t.Error("entry logged after SetMetadataOnly(false): expected parameters to be present again")
	}
}

// TestSetMetadataOnlyNoopWhenTracingNotInitialized confirms
// SetMetadataOnly doesn't panic when called before (or without) tracing
// having been initialized, matching the other tracing functions' pattern
// of tolerating a nil instance.
func TestSetMetadataOnlyNoopWhenTracingNotInitialized(t *testing.T) {
	ResetForTesting()
	SetMetadataOnly(true)
}
