/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"pgedge-postgres-mcp/internal/auth"
	"pgedge-postgres-mcp/internal/config"
	"pgedge-postgres-mcp/internal/database"
)

// Helper to create a ClientManager with test databases
func createTestClientManager() *database.ClientManager {
	databases := []config.NamedDatabaseConfig{
		{
			Name:     "testdb1",
			Host:     "localhost",
			Port:     5432,
			Database: "db1",
			User:     "user1",
			SSLMode:  "disable",
		},
		{
			Name:     "testdb2",
			Host:     "localhost",
			Port:     5433,
			Database: "db2",
			User:     "user2",
			SSLMode:  "require",
		},
	}
	return database.NewClientManager(databases)
}

func TestNewDatabaseHandler(t *testing.T) {
	cm := createTestClientManager()
	handler := NewDatabaseHandler(cm, nil, false, true)

	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
	if handler.clientManager != cm {
		t.Error("expected client manager to be set")
	}
	if handler.isSTDIO {
		t.Error("expected isSTDIO to be false")
	}
	if !handler.authEnabled {
		t.Error("expected authEnabled to be true")
	}
}

func TestHandleListDatabases_Success(t *testing.T) {
	cm := createTestClientManager()
	handler := NewDatabaseHandler(cm, nil, false, false)

	req := httptest.NewRequest(http.MethodGet, "/api/databases", nil)
	w := httptest.NewRecorder()

	handler.HandleListDatabases(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response ListDatabasesResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Databases) != 2 {
		t.Errorf("expected 2 databases, got %d", len(response.Databases))
	}

	// Verify database info
	found := make(map[string]bool)
	for _, db := range response.Databases {
		found[db.Name] = true
	}
	if !found["testdb1"] || !found["testdb2"] {
		t.Error("expected both testdb1 and testdb2 in response")
	}
}

func TestHandleListDatabases_MethodNotAllowed(t *testing.T) {
	cm := createTestClientManager()
	handler := NewDatabaseHandler(cm, nil, false, false)

	req := httptest.NewRequest(http.MethodPost, "/api/databases", nil)
	w := httptest.NewRecorder()

	handler.HandleListDatabases(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
	if allow := w.Header().Get("Allow"); allow != http.MethodGet {
		t.Errorf("expected Allow header %q, got %q", http.MethodGet, allow)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("response body is not valid JSON: %v (body: %q)", err, w.Body.String())
	}
	if body["error"] != "Method not allowed" {
		t.Errorf("expected error 'Method not allowed', got %q", body["error"])
	}
}

func TestHandleListDatabases_WithTokenHash(t *testing.T) {
	cm := createTestClientManager()
	handler := NewDatabaseHandler(cm, nil, false, true)

	// Set a current database for a token
	tokenHash := "test-token-hash"
	_ = cm.SetCurrentDatabase(tokenHash, "testdb2")

	req := httptest.NewRequest(http.MethodGet, "/api/databases", nil)
	ctx := context.WithValue(req.Context(), auth.TokenHashContextKey, tokenHash)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.HandleListDatabases(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response ListDatabasesResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Current != "testdb2" {
		t.Errorf("expected current database 'testdb2', got %q", response.Current)
	}
}

func TestHandleListDatabases_DefaultCurrent(t *testing.T) {
	cm := createTestClientManager()
	handler := NewDatabaseHandler(cm, nil, false, false)

	req := httptest.NewRequest(http.MethodGet, "/api/databases", nil)
	w := httptest.NewRecorder()

	handler.HandleListDatabases(w, req)

	var response ListDatabasesResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should return default (first) database when no token
	if response.Current != "testdb1" {
		t.Errorf("expected current database 'testdb1' (default), got %q", response.Current)
	}
}

func TestHandleSelectDatabase_Success(t *testing.T) {
	cm := createTestClientManager()
	handler := NewDatabaseHandler(cm, nil, false, false)

	tokenHash := "test-token-hash"
	body := SelectDatabaseRequest{Name: "testdb2"}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/databases/select", bytes.NewReader(bodyBytes))
	ctx := context.WithValue(req.Context(), auth.TokenHashContextKey, tokenHash)
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.HandleSelectDatabase(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response SelectDatabaseResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !response.Success {
		t.Errorf("expected success=true, got false: %s", response.Error)
	}
	if response.Current != "testdb2" {
		t.Errorf("expected current='testdb2', got %q", response.Current)
	}
}

func TestHandleSelectDatabase_MethodNotAllowed(t *testing.T) {
	cm := createTestClientManager()
	handler := NewDatabaseHandler(cm, nil, false, false)

	req := httptest.NewRequest(http.MethodGet, "/api/databases/select", nil)
	w := httptest.NewRecorder()

	handler.HandleSelectDatabase(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
	if allow := w.Header().Get("Allow"); allow != http.MethodPost {
		t.Errorf("expected Allow header %q, got %q", http.MethodPost, allow)
	}

	bodyBytes := w.Body.Bytes()

	// Confirm the "success" key is actually present in the raw JSON (not
	// just its Go zero-value default), so this test can't pass against a
	// bare {"error": "..."} shape that omits the field entirely -
	// matching the documented SelectDatabaseResponse contract for every
	// other error on this endpoint (400, 404, 403).
	var raw map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &raw); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if _, ok := raw["success"]; !ok {
		t.Fatal("expected response to include a \"success\" field, matching SelectDatabaseResponse")
	}

	var response SelectDatabaseResponse
	if err := json.Unmarshal(bodyBytes, &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Success {
		t.Error("expected success=false")
	}
	if response.Error != "Method not allowed" {
		t.Errorf("expected error 'Method not allowed', got %q", response.Error)
	}
}

func TestHandleSelectDatabase_InvalidBody(t *testing.T) {
	cm := createTestClientManager()
	handler := NewDatabaseHandler(cm, nil, false, false)

	req := httptest.NewRequest(http.MethodPost, "/api/databases/select",
		bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.HandleSelectDatabase(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}

	var response SelectDatabaseResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Success {
		t.Error("expected success=false")
	}
	if response.Error != "Invalid request body" {
		t.Errorf("expected error 'Invalid request body', got %q", response.Error)
	}
}

func TestHandleSelectDatabase_OversizedBody(t *testing.T) {
	cm := createTestClientManager()
	handler := NewDatabaseHandler(cm, nil, false, false)

	// Must be syntactically valid JSON so json.Decoder keeps reading
	// (buffering the string token) until it exceeds the body-size cap,
	// rather than failing fast on a syntax error before the cap is hit.
	huge := bytes.Repeat([]byte("x"), maxRequestBodySize+1)
	oversized := append(append([]byte(`{"name":"`), huge...), []byte(`"}`)...)
	req := httptest.NewRequest(http.MethodPost, "/api/databases/select",
		bytes.NewReader(oversized))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.HandleSelectDatabase(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected status 413, got %d", w.Code)
	}

	var response SelectDatabaseResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Success {
		t.Error("expected success=false")
	}
	if response.Error != "Request body too large" {
		t.Errorf("expected error 'Request body too large', got %q", response.Error)
	}
}

func TestHandleSelectDatabase_EmptyName(t *testing.T) {
	cm := createTestClientManager()
	handler := NewDatabaseHandler(cm, nil, false, false)

	body := SelectDatabaseRequest{Name: ""}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/databases/select",
		bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.HandleSelectDatabase(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}

	var response SelectDatabaseResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Success {
		t.Error("expected success=false")
	}
	if response.Error != "Database name is required" {
		t.Errorf("expected error 'Database name is required', got %q", response.Error)
	}
}

func TestHandleSelectDatabase_NotFound(t *testing.T) {
	cm := createTestClientManager()
	handler := NewDatabaseHandler(cm, nil, false, false)

	body := SelectDatabaseRequest{Name: "nonexistent"}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/databases/select",
		bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.HandleSelectDatabase(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}

	var response SelectDatabaseResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Success {
		t.Error("expected success=false")
	}
	if response.Error != "Database not found" {
		t.Errorf("expected error 'Database not found', got %q", response.Error)
	}
}

func TestHandleSelectDatabase_NoTokenHash(t *testing.T) {
	cm := createTestClientManager()
	handler := NewDatabaseHandler(cm, nil, false, false)

	body := SelectDatabaseRequest{Name: "testdb1"}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/databases/select",
		bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.HandleSelectDatabase(w, req)

	// Should still succeed but not store the selection (no token)
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response SelectDatabaseResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("expected success=true")
	}
}

func TestHandleListDatabases_WithAccessChecker(t *testing.T) {
	cm := createTestClientManager()

	// Create a mock access checker that only allows testdb1
	checker := auth.NewDatabaseAccessChecker(nil, true, false)

	handler := NewDatabaseHandler(cm, checker, false, true)

	req := httptest.NewRequest(http.MethodGet, "/api/databases", nil)
	// Add username to context for access checking
	ctx := context.WithValue(req.Context(), auth.UsernameContextKey, "testuser")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.HandleListDatabases(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response ListDatabasesResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// With a real access checker (no config), all databases should be returned
	// since there's no restriction configured
	if len(response.Databases) != 2 {
		t.Errorf("expected 2 databases (no restrictions), got %d", len(response.Databases))
	}
}

func TestDatabaseInfoStruct(t *testing.T) {
	info := DatabaseInfo{
		Name:     "test",
		Host:     "localhost",
		Port:     5432,
		Database: "testdb",
		User:     "user",
		SSLMode:  "disable",
	}

	// Marshal to JSON and back
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded DatabaseInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Name != info.Name {
		t.Errorf("expected name %q, got %q", info.Name, decoded.Name)
	}
	if decoded.Port != info.Port {
		t.Errorf("expected port %d, got %d", info.Port, decoded.Port)
	}
}

func TestDatabaseInfoStruct_MultiHost(t *testing.T) {
	info := DatabaseInfo{
		Name:     "multihost",
		Host:     "node1.example.com",
		Port:     5432,
		Database: "mydb",
		User:     "postgres",
		SSLMode:  "require",
		Hosts: []HostInfo{
			{Host: "node1.example.com", Port: 5432},
			{Host: "node2.example.com", Port: 5432},
		},
		TargetSessionAttrs: "read-write",
	}

	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded DatabaseInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(decoded.Hosts) != 2 {
		t.Fatalf("expected 2 hosts, got %d", len(decoded.Hosts))
	}
	if decoded.Hosts[0].Host != "node1.example.com" {
		t.Errorf("expected first host 'node1.example.com', got %q", decoded.Hosts[0].Host)
	}
	if decoded.Hosts[1].Host != "node2.example.com" {
		t.Errorf("expected second host 'node2.example.com', got %q", decoded.Hosts[1].Host)
	}
	if decoded.TargetSessionAttrs != "read-write" {
		t.Errorf("expected target_session_attrs 'read-write', got %q", decoded.TargetSessionAttrs)
	}
	// Backward compatibility: Host/Port from first configured host
	if decoded.Host != "node1.example.com" {
		t.Errorf("expected Host 'node1.example.com', got %q", decoded.Host)
	}
}

func TestDatabaseInfoStruct_SingleHost_OmitsMultiHostFields(t *testing.T) {
	info := DatabaseInfo{
		Name:     "singlehost",
		Host:     "localhost",
		Port:     5432,
		Database: "testdb",
		User:     "user",
		SSLMode:  "disable",
	}

	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Verify that hosts and target_session_attrs are omitted from JSON
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}
	if _, exists := raw["hosts"]; exists {
		t.Error("expected 'hosts' to be omitted from JSON for single-host config")
	}
	if _, exists := raw["target_session_attrs"]; exists {
		t.Error("expected 'target_session_attrs' to be omitted from JSON for single-host config")
	}
}

func TestHandleListDatabases_MultiHost(t *testing.T) {
	databases := []config.NamedDatabaseConfig{
		{
			Name:     "singledb",
			Host:     "localhost",
			Port:     5432,
			Database: "db1",
			User:     "user1",
			SSLMode:  "disable",
		},
		{
			Name:     "multidb",
			Host:     "node1.example.com",
			Port:     5432,
			Database: "db2",
			User:     "user2",
			SSLMode:  "require",
			Hosts: []config.HostEntry{
				{Host: "node1.example.com", Port: 5432},
				{Host: "node2.example.com", Port: 5433},
			},
			TargetSessionAttrs: "prefer-standby",
		},
	}
	cm := database.NewClientManager(databases)
	handler := NewDatabaseHandler(cm, nil, false, false)

	req := httptest.NewRequest(http.MethodGet, "/api/databases", nil)
	w := httptest.NewRecorder()

	handler.HandleListDatabases(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var response ListDatabasesResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Databases) != 2 {
		t.Fatalf("expected 2 databases, got %d", len(response.Databases))
	}

	// Find the multi-host database
	var multiDB DatabaseInfo
	for _, db := range response.Databases {
		if db.Name == "multidb" {
			multiDB = db
			break
		}
	}

	if len(multiDB.Hosts) != 2 {
		t.Fatalf("expected 2 hosts for multidb, got %d", len(multiDB.Hosts))
	}
	if multiDB.Hosts[0].Host != "node1.example.com" || multiDB.Hosts[0].Port != 5432 {
		t.Errorf("unexpected first host: %+v", multiDB.Hosts[0])
	}
	if multiDB.Hosts[1].Host != "node2.example.com" || multiDB.Hosts[1].Port != 5433 {
		t.Errorf("unexpected second host: %+v", multiDB.Hosts[1])
	}
	if multiDB.TargetSessionAttrs != "prefer-standby" {
		t.Errorf("expected target_session_attrs 'prefer-standby', got %q", multiDB.TargetSessionAttrs)
	}
	// Backward compatibility
	if multiDB.Host != "node1.example.com" {
		t.Errorf("expected Host from first entry, got %q", multiDB.Host)
	}
	if multiDB.Port != 5432 {
		t.Errorf("expected Port from first entry, got %d", multiDB.Port)
	}

	// Verify single-host database has no hosts field
	var singleDB DatabaseInfo
	for _, db := range response.Databases {
		if db.Name == "singledb" {
			singleDB = db
			break
		}
	}
	if len(singleDB.Hosts) != 0 {
		t.Errorf("expected no hosts for singledb, got %d", len(singleDB.Hosts))
	}
	if singleDB.TargetSessionAttrs != "" {
		t.Errorf("expected empty target_session_attrs for singledb, got %q", singleDB.TargetSessionAttrs)
	}
}

func TestSelectDatabaseResponseStruct(t *testing.T) {
	// Test success response
	success := SelectDatabaseResponse{
		Success: true,
		Current: "testdb",
		Message: "Selected successfully",
	}

	data, _ := json.Marshal(success)
	var decoded SelectDatabaseResponse
	json.Unmarshal(data, &decoded)

	if !decoded.Success {
		t.Error("expected Success=true")
	}
	if decoded.Current != "testdb" {
		t.Errorf("expected Current='testdb', got %q", decoded.Current)
	}

	// Test error response
	errorResp := SelectDatabaseResponse{
		Success: false,
		Error:   "Something went wrong",
	}

	data, _ = json.Marshal(errorResp)
	json.Unmarshal(data, &decoded)

	if decoded.Success {
		t.Error("expected Success=false")
	}
	if decoded.Error != "Something went wrong" {
		t.Errorf("expected Error='Something went wrong', got %q", decoded.Error)
	}
}
