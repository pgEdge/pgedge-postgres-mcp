/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package tools

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"

	"pgedge-postgres-mcp/internal/config"
	"pgedge-postgres-mcp/internal/database"
)

// testDBConfig parses TEST_PGEDGE_POSTGRES_CONNECTION_STRING into a
// NamedDatabaseConfig for tests that need a genuinely reachable database,
// skipping the test if the env var isn't set. Mirrors the helper of the
// same name in internal/database/client_manager_test.go.
func testDBConfig(t *testing.T) *config.NamedDatabaseConfig {
	t.Helper()
	connStr := os.Getenv("TEST_PGEDGE_POSTGRES_CONNECTION_STRING")
	if connStr == "" {
		t.Skip("TEST_PGEDGE_POSTGRES_CONNECTION_STRING not set, skipping live database test")
	}

	u, err := url.Parse(connStr)
	if err != nil {
		t.Fatalf("Failed to parse TEST_PGEDGE_POSTGRES_CONNECTION_STRING: %v", err)
	}

	host := u.Hostname()
	if host == "" {
		host = "localhost"
	}
	port := 5432
	if u.Port() != "" {
		p, err := strconv.Atoi(u.Port())
		if err != nil {
			t.Fatalf("invalid port %q in TEST_PGEDGE_POSTGRES_CONNECTION_STRING: %v", u.Port(), err)
		}
		port = p
	}
	dbName := "postgres"
	if len(u.Path) > 1 {
		dbName = u.Path[1:]
	}
	user := ""
	password := ""
	if u.User != nil {
		user = u.User.Username()
		password, _ = u.User.Password()
	}
	sslMode := u.Query().Get("sslmode")

	trueVal := true
	return &config.NamedDatabaseConfig{
		Name:              "testdb",
		Host:              host,
		Port:              port,
		Database:          dbName,
		User:              user,
		Password:          password,
		SSLMode:           sslMode,
		AllowWrites:       true,
		AllowLLMSwitching: &trueVal,
	}
}

// Helper to create a test config with databases
func createTestDatabaseConfigs() []config.NamedDatabaseConfig {
	trueVal := true
	falseVal := false
	return []config.NamedDatabaseConfig{
		{
			Name:              "db1",
			Host:              "localhost",
			Port:              5432,
			Database:          "testdb1",
			User:              "user1",
			AllowWrites:       false,
			AllowLLMSwitching: &trueVal,
		},
		{
			Name:              "db2",
			Host:              "remotehost",
			Port:              5433,
			Database:          "testdb2",
			User:              "user2",
			AllowWrites:       true,
			AllowLLMSwitching: &trueVal,
		},
		{
			Name:              "db3-no-llm",
			Host:              "localhost",
			Port:              5432,
			Database:          "testdb3",
			User:              "user3",
			AllowWrites:       false,
			AllowLLMSwitching: &falseVal, // LLM switching disabled
		},
	}
}

// TestListDatabaseConnectionsTool_Basic tests basic listing functionality
func TestListDatabaseConnectionsTool_Basic(t *testing.T) {
	databases := createTestDatabaseConfigs()
	cfg := &config.Config{Databases: databases}
	clientManager := database.NewClientManager(databases)
	defer clientManager.CloseAll()

	tool := ListDatabaseConnectionsTool(clientManager, nil, cfg)

	// Verify tool definition
	if tool.Definition.Name != "list_database_connections" {
		t.Errorf("Expected tool name 'list_database_connections', got %q", tool.Definition.Name)
	}

	// Execute the tool
	args := map[string]interface{}{
		"__context": context.Background(),
	}
	response, err := tool.Handler(args)
	if err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}

	if response.IsError {
		t.Fatalf("Expected success response, got error: %v", response.Content)
	}

	// Parse response
	var result struct {
		Databases []struct {
			Name        string `json:"name"`
			Database    string `json:"database"`
			Host        string `json:"host"`
			Port        int    `json:"port"`
			AllowWrites bool   `json:"allow_writes"`
			Status      string `json:"status"`
		} `json:"databases"`
		Current string `json:"current"`
	}
	if err := json.Unmarshal([]byte(response.Content[0].Text), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Should have 2 databases (db3-no-llm is filtered out due to allow_llm_switching: false)
	if len(result.Databases) != 2 {
		t.Errorf("Expected 2 databases, got %d", len(result.Databases))
	}

	// Verify first database has all expected fields
	found := false
	for _, db := range result.Databases {
		if db.Name == "db1" {
			found = true
			if db.Database != "testdb1" {
				t.Errorf("Expected database 'testdb1', got %q", db.Database)
			}
			if db.Host != "localhost" {
				t.Errorf("Expected host 'localhost', got %q", db.Host)
			}
			if db.Port != 5432 {
				t.Errorf("Expected port 5432, got %d", db.Port)
			}
			if db.AllowWrites != false {
				t.Errorf("Expected allow_writes false, got %v", db.AllowWrites)
			}
			if db.Status == "" {
				t.Error("Expected status field to be present")
			}
		}
	}
	if !found {
		t.Error("Database 'db1' not found in response")
	}
}

// TestListDatabaseConnectionsTool_FiltersLLMSwitching tests allow_llm_switching filtering
func TestListDatabaseConnectionsTool_FiltersLLMSwitching(t *testing.T) {
	databases := createTestDatabaseConfigs()
	cfg := &config.Config{Databases: databases}
	clientManager := database.NewClientManager(databases)
	defer clientManager.CloseAll()

	tool := ListDatabaseConnectionsTool(clientManager, nil, cfg)

	args := map[string]interface{}{
		"__context": context.Background(),
	}
	response, err := tool.Handler(args)
	if err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}

	var result struct {
		Databases []struct {
			Name string `json:"name"`
		} `json:"databases"`
	}
	if err := json.Unmarshal([]byte(response.Content[0].Text), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// db3-no-llm should NOT be in the list
	for _, db := range result.Databases {
		if db.Name == "db3-no-llm" {
			t.Error("Database with allow_llm_switching: false should not be listed")
		}
	}
}

// TestListDatabaseConnectionsTool_EmptyConfig tests with no databases configured
func TestListDatabaseConnectionsTool_EmptyConfig(t *testing.T) {
	cfg := &config.Config{Databases: []config.NamedDatabaseConfig{}}
	clientManager := database.NewClientManager(nil)
	defer clientManager.CloseAll()

	tool := ListDatabaseConnectionsTool(clientManager, nil, cfg)

	args := map[string]interface{}{
		"__context": context.Background(),
	}
	response, err := tool.Handler(args)
	if err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}

	var result struct {
		Databases []interface{} `json:"databases"`
		Current   string        `json:"current"`
	}
	if err := json.Unmarshal([]byte(response.Content[0].Text), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(result.Databases) != 0 {
		t.Errorf("Expected 0 databases, got %d", len(result.Databases))
	}
	if result.Current != "" {
		t.Errorf("Expected empty current database, got %q", result.Current)
	}
}

// TestSelectDatabaseConnectionTool_Success tests successful database
// switching against a live database, verifying both the response fields
// and that the connection is actually established: after a successful
// switch, list_database_connections must report the database as
// "connected" without any other tool having run first. Regression test
// for issue #256, where the switch only updated the current-database
// pointer and left the target "unavailable" until an unrelated query
// happened to connect it.
func TestSelectDatabaseConnectionTool_Success(t *testing.T) {
	dbConfig := testDBConfig(t)

	databases := []config.NamedDatabaseConfig{*dbConfig}
	cfg := &config.Config{Databases: databases}
	clientManager := database.NewClientManager(databases)
	defer clientManager.CloseAll()

	tool := SelectDatabaseConnectionTool(clientManager, nil, cfg)

	// Verify tool definition
	if tool.Definition.Name != "select_database_connection" {
		t.Errorf("Expected tool name 'select_database_connection', got %q", tool.Definition.Name)
	}

	args := map[string]interface{}{
		"__context": context.Background(),
		"name":      dbConfig.Name,
	}
	response, err := tool.Handler(args)
	if err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}

	if response.IsError {
		t.Fatalf("Expected success response, got error: %v", response.Content)
	}

	var result struct {
		Success     bool   `json:"success"`
		Message     string `json:"message"`
		Current     string `json:"current"`
		Database    string `json:"database"`
		Host        string `json:"host"`
		AllowWrites bool   `json:"allow_writes"`
	}
	if err := json.Unmarshal([]byte(response.Content[0].Text), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if !result.Success {
		t.Error("Expected success=true")
	}
	if result.Current != dbConfig.Name {
		t.Errorf("Expected current=%q, got %q", dbConfig.Name, result.Current)
	}
	if result.Database != dbConfig.Database {
		t.Errorf("Expected database=%q, got %q", dbConfig.Database, result.Database)
	}
	if result.Host != dbConfig.Host {
		t.Errorf("Expected host=%q, got %q", dbConfig.Host, result.Host)
	}
	if result.AllowWrites != true {
		t.Error("Expected allow_writes=true")
	}

	if !clientManager.IsConnected("default", dbConfig.Name) {
		t.Error("Expected the switched-to database to be connected immediately after a successful switch")
	}

	listTool := ListDatabaseConnectionsTool(clientManager, nil, cfg)
	listResponse, err := listTool.Handler(map[string]interface{}{"__context": context.Background()})
	if err != nil {
		t.Fatalf("list_database_connections returned error: %v", err)
	}
	var listResult struct {
		Databases []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"databases"`
	}
	if err := json.Unmarshal([]byte(listResponse.Content[0].Text), &listResult); err != nil {
		t.Fatalf("Failed to parse list response: %v", err)
	}
	if len(listResult.Databases) != 1 || listResult.Databases[0].Status != "connected" {
		t.Errorf("Expected %q to show status 'connected' after switching, got %+v", dbConfig.Name, listResult.Databases)
	}
}

// TestSelectDatabaseConnectionTool_ConnectFailure tests that switching to a
// configured but unreachable database surfaces a connection error
// immediately, rather than reporting success and leaving the failure for
// whatever tool call happens to use the connection next.
func TestSelectDatabaseConnectionTool_ConnectFailure(t *testing.T) {
	databases := createTestDatabaseConfigs()
	cfg := &config.Config{Databases: databases}
	clientManager := database.NewClientManager(databases)
	defer clientManager.CloseAll()

	tool := SelectDatabaseConnectionTool(clientManager, nil, cfg)

	args := map[string]interface{}{
		"__context": context.Background(),
		"name":      "db2",
	}
	response, err := tool.Handler(args)
	if err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}

	if !response.IsError {
		t.Fatal("Expected error response for a switch to an unreachable database")
	}
	if !strings.Contains(response.Content[0].Text, "failed to connect") {
		t.Errorf("Expected a connect-failure message, got: %s", response.Content[0].Text)
	}
}

// TestSelectDatabaseConnectionTool_MissingName tests missing name parameter
func TestSelectDatabaseConnectionTool_MissingName(t *testing.T) {
	databases := createTestDatabaseConfigs()
	cfg := &config.Config{Databases: databases}
	clientManager := database.NewClientManager(databases)
	defer clientManager.CloseAll()

	tool := SelectDatabaseConnectionTool(clientManager, nil, cfg)

	args := map[string]interface{}{
		"__context": context.Background(),
	}
	response, err := tool.Handler(args)
	if err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}

	if !response.IsError {
		t.Error("Expected error response for missing name parameter")
	}

	if len(response.Content) > 0 && !strings.Contains(response.Content[0].Text, "Missing or invalid 'name' parameter") {
		t.Errorf("Expected 'Missing or invalid' error, got: %s", response.Content[0].Text)
	}
}

// TestSelectDatabaseConnectionTool_DatabaseNotFound tests non-existent database
func TestSelectDatabaseConnectionTool_DatabaseNotFound(t *testing.T) {
	databases := createTestDatabaseConfigs()
	cfg := &config.Config{Databases: databases}
	clientManager := database.NewClientManager(databases)
	defer clientManager.CloseAll()

	tool := SelectDatabaseConnectionTool(clientManager, nil, cfg)

	args := map[string]interface{}{
		"__context": context.Background(),
		"name":      "nonexistent-db",
	}
	response, err := tool.Handler(args)
	if err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}

	if !response.IsError {
		t.Error("Expected error response for non-existent database")
	}

	// Should use "Access denied" message (not "not found") to prevent information disclosure
	if len(response.Content) > 0 && !strings.Contains(response.Content[0].Text, "Access denied") {
		t.Errorf("Expected 'Access denied' error to prevent info disclosure, got: %s", response.Content[0].Text)
	}
}

// TestSelectDatabaseConnectionTool_LLMSwitchingDisabled tests allow_llm_switching: false
func TestSelectDatabaseConnectionTool_LLMSwitchingDisabled(t *testing.T) {
	databases := createTestDatabaseConfigs()
	cfg := &config.Config{Databases: databases}
	clientManager := database.NewClientManager(databases)
	defer clientManager.CloseAll()

	tool := SelectDatabaseConnectionTool(clientManager, nil, cfg)

	args := map[string]interface{}{
		"__context": context.Background(),
		"name":      "db3-no-llm", // This database has allow_llm_switching: false
	}
	response, err := tool.Handler(args)
	if err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}

	if !response.IsError {
		t.Error("Expected error response for database with LLM switching disabled")
	}

	if len(response.Content) > 0 && !strings.Contains(response.Content[0].Text, "Access denied") {
		t.Errorf("Expected 'Access denied' error, got: %s", response.Content[0].Text)
	}
}

// TestExtractContextFromArgs tests context extraction helper
func TestExtractContextFromArgs(t *testing.T) {
	t.Run("with context", func(t *testing.T) {
		type testKey string
		ctx := context.WithValue(context.Background(), testKey("test"), "value")
		args := map[string]interface{}{
			"__context": ctx,
		}
		extracted := extractContextFromArgs(args)
		if extracted.Value(testKey("test")) != "value" {
			t.Error("Context not properly extracted")
		}
	})

	t.Run("without context", func(t *testing.T) {
		args := map[string]interface{}{}
		extracted := extractContextFromArgs(args)
		if extracted == nil {
			t.Error("Expected background context, got nil")
		}
	})

	t.Run("with invalid context type", func(t *testing.T) {
		args := map[string]interface{}{
			"__context": "not-a-context",
		}
		extracted := extractContextFromArgs(args)
		if extracted == nil {
			t.Error("Expected background context, got nil")
		}
	})
}

// TestListDatabaseConnectionsTool_HidesInaccessibleCurrent tests that current db is hidden if inaccessible
func TestListDatabaseConnectionsTool_HidesInaccessibleCurrent(t *testing.T) {
	// Create config with one LLM-accessible and one non-accessible database
	trueVal := true
	falseVal := false
	databases := []config.NamedDatabaseConfig{
		{
			Name:              "accessible-db",
			Host:              "localhost",
			Port:              5432,
			Database:          "accessible",
			User:              "user1",
			AllowLLMSwitching: &trueVal,
		},
		{
			Name:              "hidden-db",
			Host:              "localhost",
			Port:              5432,
			Database:          "hidden",
			User:              "user1",
			AllowLLMSwitching: &falseVal,
		},
	}
	cfg := &config.Config{Databases: databases}
	clientManager := database.NewClientManager(databases)
	defer clientManager.CloseAll()

	// Set current database to the hidden one
	clientManager.SetCurrentDatabase("default", "hidden-db")

	tool := ListDatabaseConnectionsTool(clientManager, nil, cfg)

	args := map[string]interface{}{
		"__context": context.Background(),
	}
	response, err := tool.Handler(args)
	if err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}

	var result struct {
		Current string `json:"current"`
	}
	if err := json.Unmarshal([]byte(response.Content[0].Text), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Current should NOT be "hidden-db" since it's not accessible to LLM
	// It should be empty to avoid misrepresenting the actual session state
	if result.Current == "hidden-db" {
		t.Error("Current database should not reveal inaccessible database name")
	}
	if result.Current != "" {
		t.Errorf("Current should be empty when inaccessible to LLM, got %q", result.Current)
	}
}

// TestListDatabaseConnectionsTool_IncludesStatus tests that status field is present
func TestListDatabaseConnectionsTool_IncludesStatus(t *testing.T) {
	databases := createTestDatabaseConfigs()
	cfg := &config.Config{Databases: databases}
	clientManager := database.NewClientManager(databases)
	defer clientManager.CloseAll()

	tool := ListDatabaseConnectionsTool(clientManager, nil, cfg)

	args := map[string]interface{}{
		"__context": context.Background(),
	}
	response, err := tool.Handler(args)
	if err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}

	var result struct {
		Databases []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"databases"`
	}
	if err := json.Unmarshal([]byte(response.Content[0].Text), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// All databases should have status "unavailable" since no actual
	// connections were established in this unit test
	for _, db := range result.Databases {
		if db.Status == "" {
			t.Errorf("Database %q missing status field", db.Name)
		}
		if db.Status != "unavailable" {
			t.Errorf("Expected status 'unavailable' for %q (no connection), got %q", db.Name, db.Status)
		}
	}
}

// TestListDatabaseConnectionsTool_StatusReflectsConnection tests that status
// transitions between connected and unavailable based on actual client state
func TestListDatabaseConnectionsTool_StatusReflectsConnection(t *testing.T) {
	trueVal := true
	databases := []config.NamedDatabaseConfig{
		{
			Name:              "db1",
			Host:              "localhost",
			Port:              5432,
			Database:          "testdb1",
			User:              "user1",
			AllowLLMSwitching: &trueVal,
		},
		{
			Name:              "db2",
			Host:              "remotehost",
			Port:              5433,
			Database:          "testdb2",
			User:              "user2",
			AllowLLMSwitching: &trueVal,
		},
	}
	cfg := &config.Config{Databases: databases}
	clientManager := database.NewClientManager(databases)
	defer clientManager.CloseAll()

	// Simulate db1 connected at startup, db2 unreachable
	client := database.NewClient(nil)
	if err := clientManager.SetClientForDatabase("default", "db1", client); err != nil {
		t.Fatalf("SetClientForDatabase returned error: %v", err)
	}

	tool := ListDatabaseConnectionsTool(clientManager, nil, cfg)
	args := map[string]interface{}{
		"__context": context.Background(),
	}
	response, err := tool.Handler(args)
	if err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}

	var result struct {
		Databases []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"databases"`
	}
	if err := json.Unmarshal([]byte(response.Content[0].Text), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	statusMap := make(map[string]string)
	for _, db := range result.Databases {
		statusMap[db.Name] = db.Status
	}

	if statusMap["db1"] != "connected" {
		t.Errorf("Expected db1 status 'connected', got %q", statusMap["db1"])
	}
	if statusMap["db2"] != "unavailable" {
		t.Errorf("Expected db2 status 'unavailable', got %q", statusMap["db2"])
	}

	// Close db1 to simulate a dropped connection
	client.Close()

	response, err = tool.Handler(args)
	if err != nil {
		t.Fatalf("Handler returned error after close: %v", err)
	}

	if err := json.Unmarshal([]byte(response.Content[0].Text), &result); err != nil {
		t.Fatalf("Failed to parse response after close: %v", err)
	}

	statusMap = make(map[string]string)
	for _, db := range result.Databases {
		statusMap[db.Name] = db.Status
	}

	if statusMap["db1"] != "unavailable" {
		t.Errorf("Expected db1 status 'unavailable' after close, got %q", statusMap["db1"])
	}
}

// TestListDatabaseConnectionsTool_ToolDescription tests that tool description includes port field
func TestListDatabaseConnectionsTool_ToolDescription(t *testing.T) {
	clientManager := database.NewClientManager(nil)
	defer clientManager.CloseAll()
	cfg := &config.Config{}

	tool := ListDatabaseConnectionsTool(clientManager, nil, cfg)

	// Verify tool description mentions port
	if !strings.Contains(tool.Definition.Description, "port") {
		t.Error("Tool description should mention 'port' field")
	}

	// Verify tool description mentions status
	if !strings.Contains(tool.Definition.Description, "status") {
		t.Error("Tool description should mention 'status' field")
	}
}

// TestSelectDatabaseConnectionTool_EmptyNameString tests empty name string
func TestSelectDatabaseConnectionTool_EmptyNameString(t *testing.T) {
	databases := createTestDatabaseConfigs()
	cfg := &config.Config{Databases: databases}
	clientManager := database.NewClientManager(databases)
	defer clientManager.CloseAll()

	tool := SelectDatabaseConnectionTool(clientManager, nil, cfg)

	args := map[string]interface{}{
		"__context": context.Background(),
		"name":      "",
	}
	response, err := tool.Handler(args)
	if err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}

	if !response.IsError {
		t.Error("Expected error response for empty name")
	}

	if len(response.Content) > 0 && !strings.Contains(response.Content[0].Text, "Missing or invalid") {
		t.Errorf("Expected 'Missing or invalid' error, got: %s", response.Content[0].Text)
	}
}

// TestBuildDatabaseListResponse_MultiHost tests multi-host config in list response
func TestBuildDatabaseListResponse_MultiHost(t *testing.T) {
	databases := []config.NamedDatabaseConfig{
		{
			Name:     "single",
			Database: "mydb",
			Host:     "db.example.com",
			Port:     5432,
		},
		{
			Name:     "cluster",
			Database: "mydb",
			Hosts: []config.HostEntry{
				{Host: "primary.example.com", Port: 5432},
				{Host: "replica.example.com", Port: 5433},
			},
			TargetSessionAttrs: "read-write",
		},
	}

	resp, err := buildDatabaseListResponse(databases, "single", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Parse the response JSON
	var result map[string]interface{}
	if jsonErr := json.Unmarshal([]byte(resp.Content[0].Text), &result); jsonErr != nil {
		t.Fatalf("failed to parse response: %v", jsonErr)
	}

	dbList := result["databases"].([]interface{})
	if len(dbList) != 2 {
		t.Fatalf("expected 2 databases, got %d", len(dbList))
	}

	// Single-host entry
	single := dbList[0].(map[string]interface{})
	if single["host"] != "db.example.com" {
		t.Errorf("single-host: expected host db.example.com, got %v", single["host"])
	}
	if _, hasHosts := single["hosts"]; hasHosts {
		t.Error("single-host: should not have hosts array")
	}

	// Multi-host entry
	cluster := dbList[1].(map[string]interface{})
	if cluster["host"] != "primary.example.com" {
		t.Errorf("multi-host: expected primary host, got %v", cluster["host"])
	}
	hosts, ok := cluster["hosts"]
	if !ok {
		t.Fatal("multi-host: expected hosts array in response")
	}
	hostList := hosts.([]interface{})
	if len(hostList) != 2 {
		t.Errorf("multi-host: expected 2 hosts, got %d", len(hostList))
	}
	if cluster["target_session_attrs"] != "read-write" {
		t.Errorf("expected target_session_attrs read-write, got %v", cluster["target_session_attrs"])
	}
}

// TestPopulateHostFields tests the shared host field population helper
func TestPopulateHostFields(t *testing.T) {
	t.Run("single host", func(t *testing.T) {
		entry := map[string]interface{}{}
		cfg := &config.NamedDatabaseConfig{
			Host: "db.example.com",
			Port: 5432,
		}
		populateHostFields(entry, cfg)

		if entry["host"] != "db.example.com" {
			t.Errorf("expected host db.example.com, got %v", entry["host"])
		}
		if entry["port"] != 5432 {
			t.Errorf("expected port 5432, got %v", entry["port"])
		}
		if _, ok := entry["hosts"]; ok {
			t.Error("single-host should not have hosts array")
		}
		if _, ok := entry["target_session_attrs"]; ok {
			t.Error("single-host should not have target_session_attrs")
		}
	})

	t.Run("multi host without target_session_attrs", func(t *testing.T) {
		entry := map[string]interface{}{}
		cfg := &config.NamedDatabaseConfig{
			Hosts: []config.HostEntry{
				{Host: "primary.example.com", Port: 5432},
				{Host: "replica.example.com", Port: 5433},
			},
		}
		populateHostFields(entry, cfg)

		if entry["host"] != "primary.example.com" {
			t.Errorf("expected first host, got %v", entry["host"])
		}
		if entry["port"] != 5432 {
			t.Errorf("expected first port, got %v", entry["port"])
		}
		hostsList := entry["hosts"].([]map[string]interface{})
		if len(hostsList) != 2 {
			t.Fatalf("expected 2 hosts, got %d", len(hostsList))
		}
		if hostsList[1]["host"] != "replica.example.com" {
			t.Errorf("expected second host replica.example.com, got %v", hostsList[1]["host"])
		}
		if _, ok := entry["target_session_attrs"]; ok {
			t.Error("should not have target_session_attrs when empty")
		}
	})

	t.Run("multi host with target_session_attrs", func(t *testing.T) {
		entry := map[string]interface{}{}
		cfg := &config.NamedDatabaseConfig{
			Hosts: []config.HostEntry{
				{Host: "primary.example.com", Port: 5432},
			},
			TargetSessionAttrs: "read-write",
		}
		populateHostFields(entry, cfg)

		if entry["target_session_attrs"] != "read-write" {
			t.Errorf("expected target_session_attrs read-write, got %v", entry["target_session_attrs"])
		}
	})
}

// TestSelectDatabaseConnectionTool_MultiHost tests multi-host fields in the
// select response, against a live database so the eager connect performed
// by select_database_connection succeeds.
func TestSelectDatabaseConnectionTool_MultiHost(t *testing.T) {
	dbConfig := testDBConfig(t)
	trueVal := true
	databases := []config.NamedDatabaseConfig{
		{
			Name:     "cluster",
			Database: dbConfig.Database,
			User:     dbConfig.User,
			Password: dbConfig.Password,
			SSLMode:  dbConfig.SSLMode,
			Hosts: []config.HostEntry{
				{Host: dbConfig.Host, Port: dbConfig.Port},
				{Host: dbConfig.Host, Port: dbConfig.Port},
			},
			TargetSessionAttrs: "read-write",
			AllowLLMSwitching:  &trueVal,
		},
	}
	cfg := &config.Config{Databases: databases}
	clientManager := database.NewClientManager(databases)
	defer clientManager.CloseAll()

	tool := SelectDatabaseConnectionTool(clientManager, nil, cfg)

	args := map[string]interface{}{
		"__context": context.Background(),
		"name":      "cluster",
	}
	response, err := tool.Handler(args)
	if err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}
	if response.IsError {
		t.Fatalf("Expected success, got error: %v", response.Content)
	}

	var result map[string]interface{}
	if jsonErr := json.Unmarshal([]byte(response.Content[0].Text), &result); jsonErr != nil {
		t.Fatalf("failed to parse response: %v", jsonErr)
	}

	if result["host"] != dbConfig.Host {
		t.Errorf("expected host %v, got %v", dbConfig.Host, result["host"])
	}
	hosts, ok := result["hosts"]
	if !ok {
		t.Fatal("expected hosts array in select response")
	}
	hostList := hosts.([]interface{})
	if len(hostList) != 2 {
		t.Errorf("expected 2 hosts, got %d", len(hostList))
	}
	if result["target_session_attrs"] != "read-write" {
		t.Errorf("expected target_session_attrs read-write, got %v", result["target_session_attrs"])
	}
}

// TestListDatabaseConnectionsTool_DefaultsToNil tests allow_llm_switching defaulting to true when nil
func TestListDatabaseConnectionsTool_DefaultsToNil(t *testing.T) {
	// Create database without explicit allow_llm_switching (should default to true)
	databases := []config.NamedDatabaseConfig{
		{
			Name:     "default-db",
			Host:     "localhost",
			Port:     5432,
			Database: "default",
			User:     "user1",
			// AllowLLMSwitching not set - should default to true
		},
	}
	cfg := &config.Config{Databases: databases}
	clientManager := database.NewClientManager(databases)
	defer clientManager.CloseAll()

	tool := ListDatabaseConnectionsTool(clientManager, nil, cfg)

	args := map[string]interface{}{
		"__context": context.Background(),
	}
	response, err := tool.Handler(args)
	if err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}

	var result struct {
		Databases []struct {
			Name string `json:"name"`
		} `json:"databases"`
	}
	if err := json.Unmarshal([]byte(response.Content[0].Text), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Database with nil AllowLLMSwitching should be included (defaults to true)
	if len(result.Databases) != 1 {
		t.Errorf("Expected 1 database (nil defaults to allowed), got %d", len(result.Databases))
	}
	if len(result.Databases) > 0 && result.Databases[0].Name != "default-db" {
		t.Errorf("Expected 'default-db', got %q", result.Databases[0].Name)
	}
}
