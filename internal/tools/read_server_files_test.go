/*-------------------------------------------------------------------------
 *
 * pgEdge Postgres MCP Server
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package tools

import (
	"strings"
	"testing"

	"pgedge-postgres-mcp/internal/database"
)

func TestReadServerLogTool(t *testing.T) {
	t.Run("tool definition", func(t *testing.T) {
		client := database.NewTestClient("postgres://localhost/test", make(map[string]database.TableInfo))
		tool := ReadServerLogTool(client)

		if tool.Definition.Name != "read_server_log" {
			t.Errorf("Expected tool name 'read_server_log', got '%s'", tool.Definition.Name)
		}

		if tool.Definition.Description == "" {
			t.Error("Tool description is empty")
		}

		// Verify input schema has lines parameter
		props := tool.Definition.InputSchema.Properties
		if props == nil {
			t.Fatal("InputSchema.Properties is nil")
		}

		if _, exists := props["lines"]; !exists {
			t.Error("Expected 'lines' parameter in input schema")
		}
	})

	t.Run("database not ready", func(t *testing.T) {
		client := database.NewTestClient("postgres://localhost/test", make(map[string]database.TableInfo))
		tool := ReadServerLogTool(client)

		response, err := tool.Handler(map[string]interface{}{})
		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}

		if !response.IsError {
			t.Error("Expected IsError=true when database not ready")
		}
	})

	t.Run("invalid lines parameter - too low", func(t *testing.T) {
		client := database.NewTestClient("postgres://localhost/test", make(map[string]database.TableInfo))
		tool := ReadServerLogTool(client)

		response, err := tool.Handler(map[string]interface{}{
			"lines": 0.0,
		})

		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}

		if !response.IsError {
			t.Error("Expected IsError=true for lines=0")
		}

		content := response.Content[0].Text
		if !strings.Contains(content, "between 1 and 10000") {
			t.Errorf("Expected validation error message, got: %s", content)
		}
	})

	t.Run("invalid lines parameter - too high", func(t *testing.T) {
		client := database.NewTestClient("postgres://localhost/test", make(map[string]database.TableInfo))
		tool := ReadServerLogTool(client)

		response, err := tool.Handler(map[string]interface{}{
			"lines": 15000.0,
		})

		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}

		if !response.IsError {
			t.Error("Expected IsError=true for lines=15000")
		}
	})

	t.Run("default lines parameter", func(t *testing.T) {
		client := database.NewTestClient("postgres://localhost/test", make(map[string]database.TableInfo))
		tool := ReadServerLogTool(client)

		// Should not error on validation with default parameter
		response, err := tool.Handler(map[string]interface{}{})

		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}

		// It will fail on database connection, but not on validation
		if response.IsError {
			content := response.Content[0].Text
			// Should not be a validation error
			if strings.Contains(content, "between 1 and 10000") {
				t.Error("Should not have validation error with default parameter")
			}
		}
	})
}

func TestReadPostgresqlConfTool(t *testing.T) {
	t.Run("tool definition", func(t *testing.T) {
		client := database.NewTestClient("postgres://localhost/test", make(map[string]database.TableInfo))
		tool := ReadPostgresqlConfTool(client)

		if tool.Definition.Name != "read_postgresql_conf" {
			t.Errorf("Expected tool name 'read_postgresql_conf', got '%s'", tool.Definition.Name)
		}

		if tool.Definition.Description == "" {
			t.Error("Tool description is empty")
		}

		if !strings.Contains(tool.Definition.Description, "include") {
			t.Error("Description should mention include directives")
		}
	})

	t.Run("database not ready", func(t *testing.T) {
		client := database.NewTestClient("postgres://localhost/test", make(map[string]database.TableInfo))
		tool := ReadPostgresqlConfTool(client)

		response, err := tool.Handler(map[string]interface{}{})
		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}

		if !response.IsError {
			t.Error("Expected IsError=true when database not ready")
		}

		content := response.Content[0].Text
		if !strings.Contains(content, "not ready") && !strings.Contains(content, "not available") {
			t.Errorf("Expected error message, got: %s", content)
		}
	})
}

func TestReadPgHbaConfTool(t *testing.T) {
	t.Run("tool definition", func(t *testing.T) {
		client := database.NewTestClient("postgres://localhost/test", make(map[string]database.TableInfo))
		tool := ReadPgHbaConfTool(client)

		if tool.Definition.Name != "read_pg_hba_conf" {
			t.Errorf("Expected tool name 'read_pg_hba_conf', got '%s'", tool.Definition.Name)
		}

		if tool.Definition.Description == "" {
			t.Error("Tool description is empty")
		}

		if !strings.Contains(tool.Definition.Description, "Host-Based Authentication") {
			t.Error("Description should mention Host-Based Authentication")
		}
	})

	t.Run("database not ready", func(t *testing.T) {
		client := database.NewTestClient("postgres://localhost/test", make(map[string]database.TableInfo))
		tool := ReadPgHbaConfTool(client)

		response, err := tool.Handler(map[string]interface{}{})
		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}

		if !response.IsError {
			t.Error("Expected IsError=true when database not ready")
		}
	})
}

func TestReadPgIdentConfTool(t *testing.T) {
	t.Run("tool definition", func(t *testing.T) {
		client := database.NewTestClient("postgres://localhost/test", make(map[string]database.TableInfo))
		tool := ReadPgIdentConfTool(client)

		if tool.Definition.Name != "read_pg_ident_conf" {
			t.Errorf("Expected tool name 'read_pg_ident_conf', got '%s'", tool.Definition.Name)
		}

		if tool.Definition.Description == "" {
			t.Error("Tool description is empty")
		}

		if !strings.Contains(tool.Definition.Description, "User Name Mapping") {
			t.Error("Description should mention User Name Mapping")
		}
	})

	t.Run("database not ready", func(t *testing.T) {
		client := database.NewTestClient("postgres://localhost/test", make(map[string]database.TableInfo))
		tool := ReadPgIdentConfTool(client)

		response, err := tool.Handler(map[string]interface{}{})
		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}

		if !response.IsError {
			t.Error("Expected IsError=true when database not ready")
		}
	})
}

func TestParseIncludeDirectives(t *testing.T) {
	t.Run("no includes", func(t *testing.T) {
		content := `
# PostgreSQL configuration file
max_connections = 100
shared_buffers = 128MB
`
		includes := parseIncludeDirectives(content, "/etc/postgresql")
		if len(includes) != 0 {
			t.Errorf("Expected 0 includes, got %d", len(includes))
		}
	})

	t.Run("single include", func(t *testing.T) {
		content := `
max_connections = 100
include = '/etc/postgresql/custom.conf'
shared_buffers = 128MB
`
		includes := parseIncludeDirectives(content, "/etc/postgresql")
		if len(includes) != 1 {
			t.Fatalf("Expected 1 include, got %d", len(includes))
		}
		if includes[0] != "/etc/postgresql/custom.conf" {
			t.Errorf("Expected '/etc/postgresql/custom.conf', got '%s'", includes[0])
		}
	})

	t.Run("multiple includes", func(t *testing.T) {
		content := `
max_connections = 100
include = '/etc/postgresql/custom.conf'
include_if_exists = '/etc/postgresql/optional.conf'
shared_buffers = 128MB
`
		includes := parseIncludeDirectives(content, "/etc/postgresql")
		if len(includes) != 2 {
			t.Fatalf("Expected 2 includes, got %d", len(includes))
		}
	})

	t.Run("commented include ignored", func(t *testing.T) {
		content := `
max_connections = 100
# include = '/etc/postgresql/custom.conf'
shared_buffers = 128MB
`
		includes := parseIncludeDirectives(content, "/etc/postgresql")
		if len(includes) != 0 {
			t.Errorf("Expected 0 includes (commented should be ignored), got %d", len(includes))
		}
	})

	t.Run("include with spaces", func(t *testing.T) {
		content := `
max_connections = 100
    include    =    '/etc/postgresql/custom.conf'
shared_buffers = 128MB
`
		includes := parseIncludeDirectives(content, "/etc/postgresql")
		if len(includes) != 1 {
			t.Fatalf("Expected 1 include, got %d", len(includes))
		}
		if includes[0] != "/etc/postgresql/custom.conf" {
			t.Errorf("Expected '/etc/postgresql/custom.conf', got '%s'", includes[0])
		}
	})
}
