/*-------------------------------------------------------------------------
 *
 * pgEdge Postgres MCP Server
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package test

import (
	"encoding/json"
	"os"
	"testing"
)

// TestMCPCompliance verifies that the MCP server properly advertises
// all capabilities, tools, and resources according to the MCP specification
func TestMCPCompliance(t *testing.T) {
	connString := os.Getenv("TEST_PGEDGE_POSTGRES_CONNECTION_STRING")
	if connString == "" {
		connString = "postgres://localhost/postgres?sslmode=disable"
	}

	apiKey := os.Getenv("TEST_ANTHROPIC_API_KEY")
	if apiKey == "" {
		apiKey = "dummy-key-for-testing"
	}

	server, err := StartMCPServer(t, connString, apiKey)
	if err != nil {
		t.Fatalf("Failed to start MCP server: %v", err)
	}
	defer func() { _ = server.Close() }()

	t.Run("AdvertiseCapabilities", func(t *testing.T) {
		testAdvertiseCapabilities(t, server)
	})

	t.Run("ToolsHaveValidSchemas", func(t *testing.T) {
		testToolsHaveValidSchemas(t, server)
	})

	t.Run("ResourcesHaveValidMetadata", func(t *testing.T) {
		testResourcesHaveValidMetadata(t, server)
	})

	t.Run("ToolsMatchCapabilities", func(t *testing.T) {
		testToolsMatchCapabilities(t, server)
	})

	t.Run("ResourcesMatchCapabilities", func(t *testing.T) {
		testResourcesMatchCapabilities(t, server)
	})
}

func testAdvertiseCapabilities(t *testing.T, server *MCPServer) {
	params := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]interface{}{
			"roots": map[string]interface{}{
				"listChanged": true,
			},
		},
		"clientInfo": map[string]interface{}{
			"name":    "test-client",
			"version": "1.0.0",
		},
	}

	resp, err := server.SendRequest("initialize", params)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("Initialize returned error: %s", resp.Error.Message)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to parse initialize result: %v", err)
	}

	// Verify capabilities are present
	capabilities, ok := result["capabilities"].(map[string]interface{})
	if !ok {
		t.Fatal("capabilities not found in initialize response")
	}

	// Verify tools capability is advertised
	if _, ok := capabilities["tools"]; !ok {
		t.Error("tools capability not advertised in initialize response")
	}

	// Verify resources capability is advertised
	if _, ok := capabilities["resources"]; !ok {
		t.Error("resources capability not advertised in initialize response")
	}

	// Verify protocol version
	protocolVersion, ok := result["protocolVersion"].(string)
	if !ok {
		t.Error("protocolVersion not found in initialize response")
	} else if protocolVersion != "2024-11-05" {
		t.Errorf("Expected protocolVersion '2024-11-05', got '%s'", protocolVersion)
	}

	// Verify serverInfo
	serverInfo, ok := result["serverInfo"].(map[string]interface{})
	if !ok {
		t.Error("serverInfo not found in initialize response")
	} else {
		if name, ok := serverInfo["name"].(string); !ok || name == "" {
			t.Error("serverInfo.name is missing or empty")
		}
		if version, ok := serverInfo["version"].(string); !ok || version == "" {
			t.Error("serverInfo.version is missing or empty")
		}
	}

	t.Log("✓ Server properly advertises all capabilities")
}

func testToolsHaveValidSchemas(t *testing.T, server *MCPServer) {
	resp, err := server.SendRequest("tools/list", nil)
	if err != nil {
		t.Fatalf("tools/list failed: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("tools/list returned error: %s", resp.Error.Message)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to parse tools/list result: %v", err)
	}

	tools, ok := result["tools"].([]interface{})
	if !ok {
		t.Fatal("tools array not found in result")
	}

	if len(tools) == 0 {
		t.Fatal("No tools returned")
	}

	// Verify each tool has required fields
	for i, tool := range tools {
		toolMap, ok := tool.(map[string]interface{})
		if !ok {
			t.Errorf("Tool %d is not a valid object", i)
			continue
		}

		// Check name
		name, ok := toolMap["name"].(string)
		if !ok || name == "" {
			t.Errorf("Tool %d: name is missing or empty", i)
		}

		// Check description
		description, ok := toolMap["description"].(string)
		if !ok || description == "" {
			t.Errorf("Tool %d (%s): description is missing or empty", i, name)
		}

		// Check inputSchema
		inputSchema, ok := toolMap["inputSchema"].(map[string]interface{})
		if !ok {
			t.Errorf("Tool %d (%s): inputSchema is missing", i, name)
			continue
		}

		// Verify inputSchema has type
		schemaType, ok := inputSchema["type"].(string)
		if !ok || schemaType == "" {
			t.Errorf("Tool %d (%s): inputSchema.type is missing or empty", i, name)
		}

		// Verify inputSchema has properties
		properties, ok := inputSchema["properties"].(map[string]interface{})
		if !ok {
			t.Errorf("Tool %d (%s): inputSchema.properties is missing", i, name)
		} else if len(properties) == 0 {
			// It's okay to have no properties for some tools
			t.Logf("Tool %s has no input properties", name)
		}
	}

	t.Logf("✓ All %d tools have valid schemas", len(tools))
}

func testResourcesHaveValidMetadata(t *testing.T, server *MCPServer) {
	resp, err := server.SendRequest("resources/list", nil)
	if err != nil {
		t.Fatalf("resources/list failed: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("resources/list returned error: %s", resp.Error.Message)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to parse resources/list result: %v", err)
	}

	resources, ok := result["resources"].([]interface{})
	if !ok {
		t.Fatal("resources array not found in result")
	}

	if len(resources) == 0 {
		t.Fatal("No resources returned")
	}

	// Verify each resource has required fields
	for i, resource := range resources {
		resMap, ok := resource.(map[string]interface{})
		if !ok {
			t.Errorf("Resource %d is not a valid object", i)
			continue
		}

		// Check uri (required)
		uri, ok := resMap["uri"].(string)
		if !ok || uri == "" {
			t.Errorf("Resource %d: uri is missing or empty", i)
		}

		// Check name (required)
		name, ok := resMap["name"].(string)
		if !ok || name == "" {
			t.Errorf("Resource %d (%s): name is missing or empty", i, uri)
		}

		// Check description (optional but recommended)
		if description, ok := resMap["description"].(string); ok {
			if description == "" {
				t.Logf("Resource %s (%s): description is empty", uri, name)
			}
		} else {
			t.Logf("Resource %s (%s): description is missing", uri, name)
		}

		// Check mimeType (optional but recommended)
		if mimeType, ok := resMap["mimeType"].(string); ok {
			if mimeType == "" {
				t.Errorf("Resource %s (%s): mimeType is empty", uri, name)
			}
		} else {
			t.Logf("Resource %s (%s): mimeType is missing", uri, name)
		}
	}

	t.Logf("✓ All %d resources have valid metadata", len(resources))
}

func testToolsMatchCapabilities(t *testing.T, server *MCPServer) {
	// Initialize and get capabilities
	initResp, err := server.SendRequest("initialize", map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
		"clientInfo": map[string]interface{}{
			"name":    "test-client",
			"version": "1.0.0",
		},
	})
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	var initResult map[string]interface{}
	if err := json.Unmarshal(initResp.Result, &initResult); err != nil {
		t.Fatalf("Failed to parse initialize result: %v", err)
	}

	capabilities, _ := initResult["capabilities"].(map[string]interface{})

	// Get tools list
	toolsResp, err := server.SendRequest("tools/list", nil)
	if err != nil {
		t.Fatalf("tools/list failed: %v", err)
	}

	var toolsResult map[string]interface{}
	if err := json.Unmarshal(toolsResp.Result, &toolsResult); err != nil {
		t.Fatalf("Failed to parse tools/list result: %v", err)
	}

	tools, _ := toolsResult["tools"].([]interface{})

	// If tools capability is advertised, there should be tools
	if _, hasTools := capabilities["tools"]; hasTools {
		if len(tools) == 0 {
			t.Error("Server advertises tools capability but returns no tools")
		} else {
			t.Logf("✓ Server advertises tools capability and provides %d tools", len(tools))
		}
	} else {
		if len(tools) > 0 {
			t.Error("Server returns tools but doesn't advertise tools capability")
		}
	}
}

func testResourcesMatchCapabilities(t *testing.T, server *MCPServer) {
	// Initialize and get capabilities
	initResp, err := server.SendRequest("initialize", map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
		"clientInfo": map[string]interface{}{
			"name":    "test-client",
			"version": "1.0.0",
		},
	})
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	var initResult map[string]interface{}
	if err := json.Unmarshal(initResp.Result, &initResult); err != nil {
		t.Fatalf("Failed to parse initialize result: %v", err)
	}

	capabilities, _ := initResult["capabilities"].(map[string]interface{})

	// Get resources list
	resourcesResp, err := server.SendRequest("resources/list", nil)
	if err != nil {
		t.Fatalf("resources/list failed: %v", err)
	}

	var resourcesResult map[string]interface{}
	if err := json.Unmarshal(resourcesResp.Result, &resourcesResult); err != nil {
		t.Fatalf("Failed to parse resources/list result: %v", err)
	}

	resources, _ := resourcesResult["resources"].([]interface{})

	// If resources capability is advertised, there should be resources
	if _, hasResources := capabilities["resources"]; hasResources {
		if len(resources) == 0 {
			t.Error("Server advertises resources capability but returns no resources")
		} else {
			t.Logf("✓ Server advertises resources capability and provides %d resources", len(resources))
		}
	} else {
		if len(resources) > 0 {
			t.Error("Server returns resources but doesn't advertise resources capability")
		}
	}
}
