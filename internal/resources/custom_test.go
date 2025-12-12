/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package resources

import (
	"context"
	"testing"

	conf "pgedge-postgres-mcp/internal/config"
	"pgedge-postgres-mcp/internal/database"
	"pgedge-postgres-mcp/internal/definitions"
)

func TestRegisterSQL_ValidDefinition(t *testing.T) {
	cm := database.NewClientManager([]conf.NamedDatabaseConfig{
		{Name: "db1", Host: "localhost", Port: 5432, Database: "test1"},
	})
	cfg := &conf.Config{
		Builtins: conf.BuiltinsConfig{
			Resources: conf.ResourcesConfig{},
		},
	}

	registry := NewContextAwareRegistry(cm, false, nil, cfg)

	def := definitions.ResourceDefinition{
		URI:         "pg://custom/test",
		Name:        "Test Resource",
		Description: "A test SQL resource",
		Type:        "sql",
		MimeType:    "application/json",
		SQL:         "SELECT 1 AS test",
	}

	err := registry.RegisterSQL(def)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify resource is registered
	resources := registry.List()
	found := false
	for _, r := range resources {
		if r.URI == "pg://custom/test" {
			found = true
			if r.Name != "Test Resource" {
				t.Errorf("expected name 'Test Resource', got %q", r.Name)
			}
			break
		}
	}
	if !found {
		t.Error("expected custom resource to be registered")
	}
}

func TestRegisterSQL_WrongType(t *testing.T) {
	cm := database.NewClientManager([]conf.NamedDatabaseConfig{})
	cfg := &conf.Config{
		Builtins: conf.BuiltinsConfig{
			Resources: conf.ResourcesConfig{},
		},
	}

	registry := NewContextAwareRegistry(cm, false, nil, cfg)

	def := definitions.ResourceDefinition{
		URI:  "pg://custom/test",
		Type: "static", // Wrong type for RegisterSQL
		SQL:  "SELECT 1",
	}

	err := registry.RegisterSQL(def)
	if err == nil {
		t.Fatal("expected error for wrong type")
	}
	if err.Error() != "resource type must be 'sql', got: static" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRegisterSQL_MissingSQL(t *testing.T) {
	cm := database.NewClientManager([]conf.NamedDatabaseConfig{})
	cfg := &conf.Config{
		Builtins: conf.BuiltinsConfig{
			Resources: conf.ResourcesConfig{},
		},
	}

	registry := NewContextAwareRegistry(cm, false, nil, cfg)

	def := definitions.ResourceDefinition{
		URI:  "pg://custom/test",
		Type: "sql",
		SQL:  "", // Missing SQL
	}

	err := registry.RegisterSQL(def)
	if err == nil {
		t.Fatal("expected error for missing SQL")
	}
	if err.Error() != "SQL query is required for SQL resource" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRegisterStatic_ValidDefinition(t *testing.T) {
	cm := database.NewClientManager([]conf.NamedDatabaseConfig{})
	cfg := &conf.Config{
		Builtins: conf.BuiltinsConfig{
			Resources: conf.ResourcesConfig{},
		},
	}

	registry := NewContextAwareRegistry(cm, false, nil, cfg)

	def := definitions.ResourceDefinition{
		URI:         "pg://custom/static",
		Name:        "Static Resource",
		Description: "A test static resource",
		Type:        "static",
		MimeType:    "application/json",
		Data:        map[string]interface{}{"key": "value"},
	}

	err := registry.RegisterStatic(def)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify resource is registered
	resources := registry.List()
	found := false
	for _, r := range resources {
		if r.URI == "pg://custom/static" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected static resource to be registered")
	}
}

func TestRegisterStatic_WrongType(t *testing.T) {
	cm := database.NewClientManager([]conf.NamedDatabaseConfig{})
	cfg := &conf.Config{
		Builtins: conf.BuiltinsConfig{
			Resources: conf.ResourcesConfig{},
		},
	}

	registry := NewContextAwareRegistry(cm, false, nil, cfg)

	def := definitions.ResourceDefinition{
		URI:  "pg://custom/static",
		Type: "sql", // Wrong type for RegisterStatic
		Data: "test",
	}

	err := registry.RegisterStatic(def)
	if err == nil {
		t.Fatal("expected error for wrong type")
	}
	if err.Error() != "resource type must be 'static', got: sql" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRegisterStatic_MissingData(t *testing.T) {
	cm := database.NewClientManager([]conf.NamedDatabaseConfig{})
	cfg := &conf.Config{
		Builtins: conf.BuiltinsConfig{
			Resources: conf.ResourcesConfig{},
		},
	}

	registry := NewContextAwareRegistry(cm, false, nil, cfg)

	def := definitions.ResourceDefinition{
		URI:  "pg://custom/static",
		Type: "static",
		Data: nil, // Missing data
	}

	err := registry.RegisterStatic(def)
	if err == nil {
		t.Fatal("expected error for missing data")
	}
	if err.Error() != "data is required for static resource" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRegisterStatic_VariousDataTypes(t *testing.T) {
	tests := []struct {
		name string
		data interface{}
	}{
		{"string", "hello world"},
		{"int", 42},
		{"float", 3.14},
		{"bool", true},
		{"array", []interface{}{"a", "b", "c"}},
		{"object", map[string]interface{}{"nested": "value"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm := database.NewClientManager([]conf.NamedDatabaseConfig{})
			cfg := &conf.Config{
				Builtins: conf.BuiltinsConfig{
					Resources: conf.ResourcesConfig{},
				},
			}

			registry := NewContextAwareRegistry(cm, false, nil, cfg)

			def := definitions.ResourceDefinition{
				URI:      "pg://custom/" + tt.name,
				Type:     "static",
				MimeType: "application/json",
				Data:     tt.data,
			}

			err := registry.RegisterStatic(def)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Read the resource to verify handler works
			content, err := registry.Read(context.Background(), def.URI)
			if err != nil {
				t.Fatalf("unexpected error reading resource: %v", err)
			}

			if len(content.Contents) == 0 {
				t.Fatal("expected content")
			}
			if content.Contents[0].Text == "" {
				t.Error("expected non-empty text content")
			}
		})
	}
}

func TestReadCustomResource(t *testing.T) {
	cm := database.NewClientManager([]conf.NamedDatabaseConfig{})
	cfg := &conf.Config{
		Builtins: conf.BuiltinsConfig{
			Resources: conf.ResourcesConfig{},
		},
	}

	registry := NewContextAwareRegistry(cm, false, nil, cfg)

	// Register a static resource
	def := definitions.ResourceDefinition{
		URI:      "pg://custom/read-test",
		Name:     "Read Test",
		Type:     "static",
		MimeType: "application/json",
		Data:     map[string]interface{}{"test": "value"},
	}

	err := registry.RegisterStatic(def)
	if err != nil {
		t.Fatalf("failed to register: %v", err)
	}

	// Read the custom resource
	content, err := registry.Read(context.Background(), "pg://custom/read-test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if content.URI != "pg://custom/read-test" {
		t.Errorf("expected URI 'pg://custom/read-test', got %q", content.URI)
	}
	if len(content.Contents) == 0 {
		t.Fatal("expected content")
	}
}
