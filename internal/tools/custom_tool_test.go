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
	"strings"
	"testing"

	"pgedge-postgres-mcp/internal/config"
	"pgedge-postgres-mcp/internal/database"
	"pgedge-postgres-mcp/internal/definitions"
)

func TestNewCustomToolExecutor(t *testing.T) {
	t.Run("with allowed languages", func(t *testing.T) {
		executor := NewCustomToolExecutor(nil, []string{"plpgsql", "plpython3u"})

		if executor == nil {
			t.Fatal("NewCustomToolExecutor returned nil")
		}

		if !executor.allowedPLLanguages["plpgsql"] {
			t.Error("plpgsql should be allowed")
		}
		if !executor.allowedPLLanguages["plpython3u"] {
			t.Error("plpython3u should be allowed")
		}
		if executor.allowedPLLanguages["plv8"] {
			t.Error("plv8 should not be allowed")
		}
	})

	t.Run("case insensitive languages", func(t *testing.T) {
		executor := NewCustomToolExecutor(nil, []string{"PLpgSQL", "PLPYTHON3U"})

		if !executor.allowedPLLanguages["plpgsql"] {
			t.Error("plpgsql (lowercase) should be allowed")
		}
		if !executor.allowedPLLanguages["plpython3u"] {
			t.Error("plpython3u (lowercase) should be allowed")
		}
	})

	t.Run("empty allowed languages", func(t *testing.T) {
		executor := NewCustomToolExecutor(nil, []string{})

		if len(executor.allowedPLLanguages) != 0 {
			t.Error("should have no allowed languages")
		}
	})

	t.Run("default timeout", func(t *testing.T) {
		executor := NewCustomToolExecutor(nil, []string{})

		if executor.defaultTimeout.Seconds() != 30 {
			t.Errorf("default timeout should be 30s, got %v", executor.defaultTimeout)
		}
	})
}

func TestCreateTool(t *testing.T) {
	executor := NewCustomToolExecutor(nil, []string{"plpgsql"})

	t.Run("SQL tool", func(t *testing.T) {
		def := definitions.ToolDefinition{
			Name:        "test_sql_tool",
			Description: "A test SQL tool",
			Type:        "sql",
			SQL:         "SELECT * FROM users WHERE id = $1",
			InputSchema: definitions.ToolInputSchema{
				Type: "object",
				Properties: map[string]definitions.ToolProperty{
					"user_id": {
						Type:        "integer",
						Description: "User ID",
					},
				},
				Required: []string{"user_id"},
			},
		}

		tool := executor.CreateTool(def)

		if tool.Definition.Name != "test_sql_tool" {
			t.Errorf("Name = %q, want %q", tool.Definition.Name, "test_sql_tool")
		}
		if tool.Definition.Description != "A test SQL tool" {
			t.Errorf("Description = %q, want %q", tool.Definition.Description, "A test SQL tool")
		}
		if tool.Handler == nil {
			t.Error("Handler should not be nil")
		}
	})

	t.Run("PL-DO tool", func(t *testing.T) {
		def := definitions.ToolDefinition{
			Name:        "test_pldo_tool",
			Description: "A test PL-DO tool",
			Type:        "pl-do",
			Language:    "plpgsql",
			Code:        "PERFORM set_config('mcp.tool_result', 'test', true);",
			InputSchema: definitions.ToolInputSchema{
				Type:       "object",
				Properties: map[string]definitions.ToolProperty{},
			},
		}

		tool := executor.CreateTool(def)

		if tool.Definition.Name != "test_pldo_tool" {
			t.Errorf("Name = %q, want %q", tool.Definition.Name, "test_pldo_tool")
		}
	})

	t.Run("PL-FUNC tool", func(t *testing.T) {
		def := definitions.ToolDefinition{
			Name:        "test_plfunc_tool",
			Description: "A test PL-FUNC tool",
			Type:        "pl-func",
			Language:    "plpgsql",
			Returns:     "jsonb",
			Code:        "BEGIN RETURN '{}'::jsonb; END;",
			InputSchema: definitions.ToolInputSchema{
				Type:       "object",
				Properties: map[string]definitions.ToolProperty{},
			},
		}

		tool := executor.CreateTool(def)

		if tool.Definition.Name != "test_plfunc_tool" {
			t.Errorf("Name = %q, want %q", tool.Definition.Name, "test_plfunc_tool")
		}
	})

	t.Run("empty description gets default", func(t *testing.T) {
		def := definitions.ToolDefinition{
			Name:        "no_desc_tool",
			Description: "",
			Type:        "sql",
			SQL:         "SELECT 1",
			InputSchema: definitions.ToolInputSchema{Type: "object"},
		}

		tool := executor.CreateTool(def)

		if !strings.Contains(tool.Definition.Description, "Custom sql tool") {
			t.Errorf("Description should contain 'Custom sql tool', got %q", tool.Definition.Description)
		}
	})
}

func TestConvertPropertyToMCP(t *testing.T) {
	t.Run("basic property", func(t *testing.T) {
		prop := definitions.ToolProperty{
			Type:        "string",
			Description: "A test property",
		}

		result := convertPropertyToMCP(prop)

		if result["type"] != "string" {
			t.Errorf("type = %v, want %v", result["type"], "string")
		}
		if result["description"] != "A test property" {
			t.Errorf("description = %v, want %v", result["description"], "A test property")
		}
	})

	t.Run("property with default", func(t *testing.T) {
		prop := definitions.ToolProperty{
			Type:    "integer",
			Default: 42,
		}

		result := convertPropertyToMCP(prop)

		if result["default"] != 42 {
			t.Errorf("default = %v, want %v", result["default"], 42)
		}
	})

	t.Run("property with enum", func(t *testing.T) {
		prop := definitions.ToolProperty{
			Type: "string",
			Enum: []string{"a", "b", "c"},
		}

		result := convertPropertyToMCP(prop)

		enum, ok := result["enum"].([]string)
		if !ok {
			t.Fatal("enum should be []string")
		}
		if len(enum) != 3 {
			t.Errorf("enum length = %d, want %d", len(enum), 3)
		}
	})

	t.Run("array property with items", func(t *testing.T) {
		prop := definitions.ToolProperty{
			Type: "array",
			Items: &definitions.ToolProperty{
				Type: "string",
			},
		}

		result := convertPropertyToMCP(prop)

		items, ok := result["items"].(map[string]interface{})
		if !ok {
			t.Fatal("items should be map[string]interface{}")
		}
		if items["type"] != "string" {
			t.Errorf("items.type = %v, want %v", items["type"], "string")
		}
	})

	t.Run("property without optional fields", func(t *testing.T) {
		prop := definitions.ToolProperty{
			Type: "boolean",
		}

		result := convertPropertyToMCP(prop)

		if _, ok := result["description"]; ok {
			t.Error("description should not be present when empty")
		}
		if _, ok := result["default"]; ok {
			t.Error("default should not be present when nil")
		}
		if _, ok := result["enum"]; ok {
			t.Error("enum should not be present when empty")
		}
	})
}

func TestBuildSQLParams(t *testing.T) {
	executor := NewCustomToolExecutor(nil, []string{})

	t.Run("single parameter", func(t *testing.T) {
		schema := definitions.ToolInputSchema{
			Type: "object",
			Properties: map[string]definitions.ToolProperty{
				"id": {Type: "integer"},
			},
			Required: []string{"id"},
		}
		args := map[string]interface{}{"id": 42}
		sql := "SELECT * FROM users WHERE id = $1"

		params, err := executor.buildSQLParams(sql, schema, args)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(params) != 1 {
			t.Fatalf("params length = %d, want %d", len(params), 1)
		}
		if params[0] != 42 {
			t.Errorf("params[0] = %v, want %v", params[0], 42)
		}
	})

	t.Run("multiple parameters ordered by required", func(t *testing.T) {
		schema := definitions.ToolInputSchema{
			Type: "object",
			Properties: map[string]definitions.ToolProperty{
				"name": {Type: "string"},
				"id":   {Type: "integer"},
			},
			Required: []string{"id", "name"},
		}
		args := map[string]interface{}{"id": 42, "name": "test"}
		sql := "SELECT * FROM users WHERE id = $1 AND name = $2"

		params, err := executor.buildSQLParams(sql, schema, args)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(params) != 2 {
			t.Fatalf("params length = %d, want %d", len(params), 2)
		}
		// First required param is "id"
		if params[0] != 42 {
			t.Errorf("params[0] = %v, want %v", params[0], 42)
		}
		// Second required param is "name"
		if params[1] != "test" {
			t.Errorf("params[1] = %v, want %v", params[1], "test")
		}
	})

	t.Run("parameter with default value", func(t *testing.T) {
		schema := definitions.ToolInputSchema{
			Type: "object",
			Properties: map[string]definitions.ToolProperty{
				"status": {Type: "string", Default: "active"},
			},
			Required: []string{},
		}
		args := map[string]interface{}{} // No status provided
		sql := "SELECT * FROM users WHERE status = $1"

		params, err := executor.buildSQLParams(sql, schema, args)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(params) != 1 {
			t.Fatalf("params length = %d, want %d", len(params), 1)
		}
		if params[0] != "active" {
			t.Errorf("params[0] = %v, want %v", params[0], "active")
		}
	})

	t.Run("no parameters in SQL", func(t *testing.T) {
		schema := definitions.ToolInputSchema{
			Type: "object",
			Properties: map[string]definitions.ToolProperty{
				"unused": {Type: "string"},
			},
		}
		args := map[string]interface{}{}
		sql := "SELECT * FROM users"

		params, err := executor.buildSQLParams(sql, schema, args)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if params != nil {
			t.Errorf("params should be nil, got %v", params)
		}
	})

	t.Run("missing optional parameter is nil", func(t *testing.T) {
		schema := definitions.ToolInputSchema{
			Type: "object",
			Properties: map[string]definitions.ToolProperty{
				"id":     {Type: "integer"},
				"status": {Type: "string"},
			},
			Required: []string{"id"},
		}
		args := map[string]interface{}{"id": 42}
		sql := "SELECT * FROM users WHERE id = $1 AND ($2 IS NULL OR status = $2)"

		params, err := executor.buildSQLParams(sql, schema, args)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(params) != 2 {
			t.Fatalf("params length = %d, want %d", len(params), 2)
		}
		if params[0] != 42 {
			t.Errorf("params[0] = %v, want %v", params[0], 42)
		}
		if params[1] != nil {
			t.Errorf("params[1] = %v, want nil", params[1])
		}
	})
}

func TestWrapPLDOCode(t *testing.T) {
	executor := NewCustomToolExecutor(nil, []string{})
	args := map[string]interface{}{"key": "value", "num": 42}

	t.Run("plpgsql wrapping", func(t *testing.T) {
		code := "result := '{}'::jsonb;"
		wrapped := executor.wrapPLDOCode("plpgsql", code, args, "$mcp_args_test$")

		if !strings.Contains(wrapped, "args jsonb") {
			t.Error("should declare args variable")
		}
		if !strings.Contains(wrapped, "result jsonb") {
			t.Error("should declare result variable")
		}
		// JSON is embedded as Go-escaped string (%q), so check for escaped quotes
		if !strings.Contains(wrapped, "key") {
			t.Error("should contain serialized args with key")
		}
		if !strings.Contains(wrapped, mcpResultConfigKey) {
			t.Error("should reference mcp.tool_result config key")
		}
	})

	t.Run("plpython3u wrapping", func(t *testing.T) {
		code := "mcp_return({'result': args['key']})"
		wrapped := executor.wrapPLDOCode("plpython3u", code, args, "$mcp_args_test$")

		if !strings.Contains(wrapped, "import json") {
			t.Error("should import json")
		}
		if !strings.Contains(wrapped, "def mcp_return") {
			t.Error("should define mcp_return function")
		}
		if !strings.Contains(wrapped, "set_config") {
			t.Error("should use set_config for result")
		}
		// JSON is embedded as Go-escaped string (%q), so check for escaped quotes
		if !strings.Contains(wrapped, "key") {
			t.Error("should contain serialized args with key")
		}
	})

	t.Run("plv8 wrapping", func(t *testing.T) {
		code := "mcp_return({result: args.key});"
		wrapped := executor.wrapPLDOCode("plv8", code, args, "$mcp_args_test$")

		if !strings.Contains(wrapped, "var args") {
			t.Error("should declare args variable")
		}
		if !strings.Contains(wrapped, "function mcp_return") {
			t.Error("should define mcp_return function")
		}
		if !strings.Contains(wrapped, "set_config") {
			t.Error("should use set_config for result")
		}
	})

	t.Run("plperl trusted wrapping", func(t *testing.T) {
		code := "mcp_return({result => $args->{'key'}});"
		wrapped := executor.wrapPLDOCode("plperl", code, args, "$mcp_args_test$")

		if strings.Contains(wrapped, "use JSON") {
			t.Error("trusted plperl must not use JSON module")
		}
		if !strings.Contains(wrapped, "spi_exec_query") {
			t.Error("should use SPI to parse JSON args")
		}
		if !strings.Contains(wrapped, "jsonb_each_text") {
			t.Error("should use jsonb_each_text for arg parsing")
		}
		if !strings.Contains(wrapped, "sub mcp_return") {
			t.Error("should define mcp_return function")
		}
		if !strings.Contains(wrapped, "sub _to_json") {
			t.Error("should define _to_json helper for trusted plperl")
		}
		if !strings.Contains(wrapped, "set_config") {
			t.Error("should use set_config for result")
		}
	})

	t.Run("plperlu untrusted wrapping", func(t *testing.T) {
		code := "mcp_return({result => $args->{'key'}});"
		wrapped := executor.wrapPLDOCode("plperlu", code, args, "$mcp_args_test$")

		if !strings.Contains(wrapped, "use JSON") {
			t.Error("untrusted plperlu should use JSON module")
		}
		if !strings.Contains(wrapped, "decode_json") {
			t.Error("should decode JSON args")
		}
		if !strings.Contains(wrapped, "sub mcp_return") {
			t.Error("should define mcp_return function")
		}
		if !strings.Contains(wrapped, "set_config") {
			t.Error("should use set_config for result")
		}
	})

	t.Run("unknown language fallback", func(t *testing.T) {
		code := "SOME CODE"
		wrapped := executor.wrapPLDOCode("unknown", code, args, "$mcp_args_test$")

		if !strings.Contains(wrapped, "-- args:") {
			t.Error("should have args comment")
		}
		if !strings.Contains(wrapped, "SOME CODE") {
			t.Error("should contain original code")
		}
	})
}

func TestWrapPLFuncCode(t *testing.T) {
	executor := NewCustomToolExecutor(nil, []string{})
	args := map[string]interface{}{"key": "value"}

	t.Run("plpgsql passes through", func(t *testing.T) {
		code := "BEGIN RETURN 1; END;"
		wrapped := executor.wrapPLFuncCode("plpgsql", code, args)

		// plpgsql code should pass through unchanged
		if wrapped != code {
			t.Errorf("plpgsql code should pass through, got %q", wrapped)
		}
	})

	t.Run("plpython3u wrapping", func(t *testing.T) {
		code := "return args['key']"
		wrapped := executor.wrapPLFuncCode("plpython3u", code, args)

		if !strings.Contains(wrapped, "import json") {
			t.Error("should import json")
		}
		if !strings.Contains(wrapped, "global args") {
			t.Error("should declare global args to avoid UnboundLocalError")
		}
		if !strings.Contains(wrapped, "json.loads(args)") {
			t.Error("should parse args from JSON")
		}
	})

	t.Run("plv8 wrapping", func(t *testing.T) {
		code := "return args.key;"
		wrapped := executor.wrapPLFuncCode("plv8", code, args)

		if !strings.Contains(wrapped, "JSON.parse") {
			t.Error("should parse args from JSON")
		}
	})

	t.Run("plperl trusted wrapping", func(t *testing.T) {
		code := "return $args->{'key'};"
		wrapped := executor.wrapPLFuncCode("plperl", code, args)

		if strings.Contains(wrapped, "use JSON") {
			t.Error("trusted plperl must not use JSON module")
		}
		if !strings.Contains(wrapped, "spi_exec_query") {
			t.Error("should use SPI to parse JSON args")
		}
		if !strings.Contains(wrapped, "jsonb_each_text") {
			t.Error("should use jsonb_each_text for arg parsing")
		}
		if !strings.Contains(wrapped, "$args_json = $_[0]") {
			t.Error("should read args from function parameter")
		}
	})

	t.Run("plperlu untrusted wrapping", func(t *testing.T) {
		code := "return $args->{'key'};"
		wrapped := executor.wrapPLFuncCode("plperlu", code, args)

		if !strings.Contains(wrapped, "use JSON") {
			t.Error("untrusted plperlu should use JSON module")
		}
		if !strings.Contains(wrapped, "decode_json") {
			t.Error("should decode JSON args")
		}
	})
}

func TestExecuteSQLTool_NoClient(t *testing.T) {
	executor := NewCustomToolExecutor(nil, []string{})

	def := definitions.ToolDefinition{
		Name: "test_tool",
		Type: "sql",
		SQL:  "SELECT 1",
		InputSchema: definitions.ToolInputSchema{
			Type: "object",
		},
	}

	tool := executor.CreateTool(def)
	response, err := tool.Handler(map[string]interface{}{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !response.IsError {
		t.Error("expected error response when no database client")
	}
	if len(response.Content) == 0 || !strings.Contains(response.Content[0].Text, "database client not available") {
		t.Error("expected 'database client not available' error message")
	}
}

func TestExecutePLDOTool_NoClient(t *testing.T) {
	executor := NewCustomToolExecutor(nil, []string{"plpgsql"})

	def := definitions.ToolDefinition{
		Name:     "test_tool",
		Type:     "pl-do",
		Language: "plpgsql",
		Code:     "NULL;",
		InputSchema: definitions.ToolInputSchema{
			Type: "object",
		},
	}

	tool := executor.CreateTool(def)
	response, err := tool.Handler(map[string]interface{}{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !response.IsError {
		t.Error("expected error response when no database client")
	}
	if len(response.Content) == 0 || !strings.Contains(response.Content[0].Text, "database client not available") {
		t.Error("expected 'database client not available' error message")
	}
}

func TestExecutePLDOTool_LanguageNotAllowed(t *testing.T) {
	executor := NewCustomToolExecutor(nil, []string{"plpgsql"}) // Only plpgsql allowed

	def := definitions.ToolDefinition{
		Name:     "test_tool",
		Type:     "pl-do",
		Language: "plpython3u", // Not allowed
		Code:     "pass",
		InputSchema: definitions.ToolInputSchema{
			Type: "object",
		},
	}

	tool := executor.CreateTool(def)
	response, err := tool.Handler(map[string]interface{}{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !response.IsError {
		t.Error("expected error response when language not allowed")
	}
	if len(response.Content) == 0 || !strings.Contains(response.Content[0].Text, "not allowed") {
		t.Error("expected 'not allowed' error message")
	}
}

func TestExecutePLFuncTool_NoClient(t *testing.T) {
	executor := NewCustomToolExecutor(nil, []string{"plpgsql"})

	def := definitions.ToolDefinition{
		Name:     "test_tool",
		Type:     "pl-func",
		Language: "plpgsql",
		Returns:  "integer",
		Code:     "BEGIN RETURN 1; END;",
		InputSchema: definitions.ToolInputSchema{
			Type: "object",
		},
	}

	tool := executor.CreateTool(def)
	response, err := tool.Handler(map[string]interface{}{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !response.IsError {
		t.Error("expected error response when no database client")
	}
}

func TestExecutePLFuncTool_LanguageNotAllowed(t *testing.T) {
	executor := NewCustomToolExecutor(nil, []string{"plpgsql"})

	def := definitions.ToolDefinition{
		Name:     "test_tool",
		Type:     "pl-func",
		Language: "plv8", // Not allowed
		Returns:  "jsonb",
		Code:     "return {};",
		InputSchema: definitions.ToolInputSchema{
			Type: "object",
		},
	}

	tool := executor.CreateTool(def)
	response, err := tool.Handler(map[string]interface{}{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !response.IsError {
		t.Error("expected error response when language not allowed")
	}
}

func TestExecuteTool_WildcardLanguage(t *testing.T) {
	// When "*" is in allowed languages, any language should be allowed
	executor := NewCustomToolExecutor(nil, []string{"*"})

	def := definitions.ToolDefinition{
		Name:     "test_tool",
		Type:     "pl-do",
		Language: "plpython3u",
		Code:     "pass",
		InputSchema: definitions.ToolInputSchema{
			Type: "object",
		},
	}

	tool := executor.CreateTool(def)
	response, err := tool.Handler(map[string]interface{}{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should not fail with "language not allowed" - will fail with "database client not available"
	if response.IsError && strings.Contains(response.Content[0].Text, "not allowed") {
		t.Error("wildcard should allow any language")
	}
}

func TestExecuteTool_UnsupportedType(t *testing.T) {
	executor := NewCustomToolExecutor(nil, []string{})

	def := definitions.ToolDefinition{
		Name: "test_tool",
		Type: "unsupported",
		InputSchema: definitions.ToolInputSchema{
			Type: "object",
		},
	}

	tool := executor.CreateTool(def)
	response, err := tool.Handler(map[string]interface{}{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !response.IsError {
		t.Error("expected error response for unsupported type")
	}
	if len(response.Content) == 0 || !strings.Contains(response.Content[0].Text, "unsupported tool type") {
		t.Error("expected 'unsupported tool type' error message")
	}
}

func createComplexToolDef() definitions.ToolDefinition {
	return definitions.ToolDefinition{
		Name:        "complex_tool",
		Description: "Tool with complex schema",
		Type:        "sql",
		SQL:         "SELECT 1",
		InputSchema: definitions.ToolInputSchema{
			Type: "object",
			Properties: map[string]definitions.ToolProperty{
				"name":   {Type: "string", Description: "Name parameter"},
				"count":  {Type: "integer", Description: "Count parameter", Default: 10},
				"status": {Type: "string", Enum: []string{"active", "inactive"}},
				"tags":   {Type: "array", Items: &definitions.ToolProperty{Type: "string"}},
			},
			Required: []string{"name"},
		},
	}
}

func TestInputSchemaConversion(t *testing.T) {
	executor := NewCustomToolExecutor(nil, []string{})
	tool := executor.CreateTool(createComplexToolDef())

	t.Run("schema type and required", func(t *testing.T) {
		if tool.Definition.InputSchema.Type != "object" {
			t.Errorf("InputSchema.Type = %q, want %q", tool.Definition.InputSchema.Type, "object")
		}
		if len(tool.Definition.InputSchema.Required) != 1 || tool.Definition.InputSchema.Required[0] != "name" {
			t.Errorf("InputSchema.Required = %v, want [name]", tool.Definition.InputSchema.Required)
		}
	})

	t.Run("name property", func(t *testing.T) {
		props := tool.Definition.InputSchema.Properties
		nameProp, ok := props["name"].(map[string]interface{})
		if !ok {
			t.Fatal("name property should exist")
		}
		if nameProp["type"] != "string" {
			t.Errorf("name.type = %v, want string", nameProp["type"])
		}
	})

	t.Run("count property with default", func(t *testing.T) {
		props := tool.Definition.InputSchema.Properties
		countProp, ok := props["count"].(map[string]interface{})
		if !ok {
			t.Fatal("count property should exist")
		}
		if countProp["default"] != 10 {
			t.Errorf("count.default = %v, want 10", countProp["default"])
		}
	})

	t.Run("status property with enum", func(t *testing.T) {
		props := tool.Definition.InputSchema.Properties
		statusProp, ok := props["status"].(map[string]interface{})
		if !ok {
			t.Fatal("status property should exist")
		}
		enum, ok := statusProp["enum"].([]string)
		if !ok || len(enum) != 2 {
			t.Errorf("status.enum = %v, want [active inactive]", statusProp["enum"])
		}
	})

	t.Run("tags property with items", func(t *testing.T) {
		props := tool.Definition.InputSchema.Properties
		tagsProp, ok := props["tags"].(map[string]interface{})
		if !ok {
			t.Fatal("tags property should exist")
		}
		items, ok := tagsProp["items"].(map[string]interface{})
		if !ok {
			t.Fatal("tags.items should exist")
		}
		if items["type"] != "string" {
			t.Errorf("tags.items.type = %v, want string", items["type"])
		}
	})
}

// TestNewDollarQuoteTags covers the delimiters used to wrap custom tool code.
// They must be unpredictable, because tool arguments are interpolated between
// them: a fixed delimiter appearing in an argument value would end the quoted
// section early and the remainder of that value would be parsed as SQL.
func TestNewDollarQuoteTags(t *testing.T) {
	bodyTag, argsTag := newDollarQuoteTags()

	if bodyTag == argsTag {
		t.Error("body and args delimiters must differ, otherwise the inner one closes the outer")
	}
	for _, tag := range []string{bodyTag, argsTag} {
		if !strings.HasPrefix(tag, "$") || !strings.HasSuffix(tag, "$") {
			t.Errorf("tag %q is not dollar-quoted", tag)
		}
		if len(tag) < 10 {
			t.Errorf("tag %q is too short to be unguessable", tag)
		}
	}

	otherBody, otherArgs := newDollarQuoteTags()
	if otherBody == bodyTag || otherArgs == argsTag {
		t.Error("delimiters must be generated fresh for each invocation")
	}
}

// TestWrapPLDOCodeResistsDelimiterEscape is a regression test for a read-only
// bypass. The wrapper previously used the fixed delimiters $mcp_custom_tool$
// and $mcp_args$, and JSON encoding does not escape a dollar sign, so an
// argument containing either literal closed the quoting and had the rest of
// its value executed. Because the resulting statement is run through Exec with
// no bind parameters, and pgx then uses the simple query protocol, which
// accepts several statements at once, that was enough to escape read-only mode
// altogether.
func TestWrapPLDOCodeResistsDelimiterEscape(t *testing.T) {
	executor := NewCustomToolExecutor(nil, []string{"plpgsql"})

	hostile := map[string]interface{}{
		"name": "$mcp_args$::jsonb; END; $mcp_custom_tool$ LANGUAGE plpgsql; " +
			"CREATE TABLE pwn(i int); --",
	}

	bodyTag, argsTag := newDollarQuoteTags()
	wrapped := executor.wrapPLDOCode("plpgsql", "PERFORM 1;", hostile, argsTag)

	// The generated delimiters must appear exactly twice: as the opening and
	// closing pair. A third occurrence would mean the payload introduced one.
	if got := strings.Count(wrapped, argsTag); got != 2 {
		t.Errorf("args delimiter appears %d times, want exactly 2; quoting is unbalanced", got)
	}
	if strings.Contains(wrapped, bodyTag) {
		t.Error("body delimiter must not appear inside the wrapped body")
	}

	// The historic fixed delimiters may appear, but only as inert text.
	if !strings.Contains(wrapped, "$mcp_args$") {
		t.Error("expected the payload to survive as ordinary data")
	}
}

// TestExecutePLFuncToolRequiresWrites checks that a pl-func tool is refused up
// front on a read-only connection. It creates and drops a temporary function,
// so it cannot work in a read-only transaction; failing partway through with a
// permissions error invited turning read-only mode off as the remedy.
func TestExecutePLFuncToolRequiresWrites(t *testing.T) {
	client := database.NewTestClientWithConfig(
		"postgres://localhost/test",
		map[string]database.TableInfo{},
		&config.NamedDatabaseConfig{Name: "test", AllowWrites: false},
	)

	executor := NewCustomToolExecutor(client, []string{"plpgsql"})
	def := definitions.ToolDefinition{
		Name:     "example",
		Type:     "pl-func",
		Language: "plpgsql",
		Returns:  "jsonb",
		Code:     "BEGIN RETURN '{}'::jsonb; END;",
	}

	resp, err := executor.executePLFuncTool(context.Background(), def, map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.IsError {
		t.Fatal("pl-func tool should be refused on a read-only connection")
	}

	if len(resp.Content) == 0 || !strings.Contains(resp.Content[0].Text, "allow_writes") {
		t.Errorf("refusal should name the setting that governs it, got: %+v", resp.Content)
	}
}

// TestCustomToolMayWrite covers the readOnlyHint advertised for custom tools.
// Clients use the hint to decide whether a call needs the user's confirmation,
// so an unrecognised statement must be assumed to write: being wrong that way
// costs a needless prompt, whereas the opposite silently skips the gate.
func TestCustomToolMayWrite(t *testing.T) {
	tests := []struct {
		name        string
		def         definitions.ToolDefinition
		allowWrites bool
		want        bool
	}{
		{
			name:        "read-only connection can never write",
			def:         definitions.ToolDefinition{Type: "pl-func", Code: "..."},
			allowWrites: false,
			want:        false,
		},
		{
			name:        "plain SELECT is a read",
			def:         definitions.ToolDefinition{Type: "sql", SQL: "SELECT * FROM orders WHERE id = $1"},
			allowWrites: true,
			want:        false,
		},
		{
			name:        "SHOW is a read",
			def:         definitions.ToolDefinition{Type: "sql", SQL: "SHOW server_version"},
			allowWrites: true,
			want:        false,
		},
		{
			name:        "leading whitespace and mixed case do not matter",
			def:         definitions.ToolDefinition{Type: "sql", SQL: "\n  select 1"},
			allowWrites: true,
			want:        false,
		},
		{
			name:        "INSERT is a write",
			def:         definitions.ToolDefinition{Type: "sql", SQL: "INSERT INTO audit VALUES ($1)"},
			allowWrites: true,
			want:        true,
		},
		{
			// A data-modifying CTE is a write dressed as a query, which is why
			// WITH is not in the read-only prefix set.
			name:        "data-modifying CTE is a write",
			def:         definitions.ToolDefinition{Type: "sql", SQL: "WITH d AS (DELETE FROM t RETURNING *) SELECT * FROM d"},
			allowWrites: true,
			want:        true,
		},
		{
			// The keyword match is word-bounded, so a table whose name merely
			// contains a keyword does not make the statement a write.
			name:        "SELECT from a table named after a keyword is still a read",
			def:         definitions.ToolDefinition{Type: "sql", SQL: "SELECT * FROM my_update_log"},
			allowWrites: true,
			want:        false,
		},
		{
			name:        "pl-do runs arbitrary code so may write",
			def:         definitions.ToolDefinition{Type: "pl-do", Language: "plpgsql", Code: "PERFORM 1;"},
			allowWrites: true,
			want:        true,
		},
		{
			name:        "pl-func may write",
			def:         definitions.ToolDefinition{Type: "pl-func", Language: "plpgsql", Code: "BEGIN RETURN 1; END;"},
			allowWrites: true,
			want:        true,
		},
		{
			name:        "unknown type is assumed to write",
			def:         definitions.ToolDefinition{Type: "something-new"},
			allowWrites: true,
			want:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := customToolMayWrite(tt.def, tt.allowWrites); got != tt.want {
				t.Errorf("customToolMayWrite() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestCreateToolAnnotations checks that the hint reaches the tool definition,
// since that is what clients actually read.
func TestCreateToolAnnotations(t *testing.T) {
	// With no database client the executor cannot permit writes, so every
	// custom tool is advertised as read-only.
	executor := NewCustomToolExecutor(nil, []string{"plpgsql"})

	tool := executor.CreateTool(definitions.ToolDefinition{
		Name:     "example",
		Type:     "pl-do",
		Language: "plpgsql",
		Code:     "PERFORM 1;",
	})

	if tool.Definition.Annotations == nil {
		t.Fatal("custom tools should advertise annotations")
	}
	if tool.Definition.Annotations.ReadOnlyHint == nil {
		t.Fatal("readOnlyHint should be set")
	}
	if !*tool.Definition.Annotations.ReadOnlyHint {
		t.Error("a tool on a connection that cannot write should be advertised read-only")
	}
}
