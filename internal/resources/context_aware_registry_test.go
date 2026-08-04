/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package resources

import (
	"context"
	"os"
	"testing"

	"pgedge-postgres-mcp/internal/auth"
	conf "pgedge-postgres-mcp/internal/config"
	"pgedge-postgres-mcp/internal/database"
	"pgedge-postgres-mcp/internal/definitions"
	"pgedge-postgres-mcp/internal/mcp"
)

// skipIfNoDatabase skips the test if no test database connection is available.
// Tests that attempt to read resources may trigger database connection attempts.
func skipIfNoDatabase(t *testing.T) {
	if os.Getenv("TEST_PGEDGE_POSTGRES_CONNECTION_STRING") == "" {
		t.Skip("TEST_PGEDGE_POSTGRES_CONNECTION_STRING not set, skipping test that requires database")
	}
}

// Helper to create bool pointer
func boolPtr(b bool) *bool {
	return &b
}

func TestNewContextAwareRegistry(t *testing.T) {
	cm := database.NewClientManager([]conf.NamedDatabaseConfig{
		{Name: "db1", Host: "localhost", Port: 5432, Database: "test1"},
	})

	cfg := &conf.Config{
		Builtins: conf.BuiltinsConfig{
			Resources: conf.ResourcesConfig{
				SystemInfo: boolPtr(true),
			},
		},
	}

	registry := NewContextAwareRegistry(cm, false, nil, cfg)

	if registry == nil {
		t.Fatal("expected non-nil registry")
	}
	if registry.clientManager != cm {
		t.Error("expected client manager to be set")
	}
	if registry.authEnabled {
		t.Error("expected authEnabled to be false")
	}
}

func TestContextAwareRegistry_List(t *testing.T) {
	cm := database.NewClientManager([]conf.NamedDatabaseConfig{
		{Name: "db1", Host: "localhost", Port: 5432, Database: "test1"},
	})

	t.Run("with all resources enabled", func(t *testing.T) {
		cfg := &conf.Config{
			Builtins: conf.BuiltinsConfig{
				Resources: conf.ResourcesConfig{
					SystemInfo: boolPtr(true),
				},
			},
		}

		registry := NewContextAwareRegistry(cm, false, nil, cfg)
		resources := registry.List()

		// Should have built-in resource
		if len(resources) < 1 {
			t.Errorf("expected at least 1 resource, got %d", len(resources))
		}

		// Verify URIs
		found := make(map[string]bool)
		for _, r := range resources {
			found[r.URI] = true
		}
		if !found[URISystemInfo] {
			t.Error("expected URISystemInfo to be in list")
		}
	})

	t.Run("with system_info disabled", func(t *testing.T) {
		cfg := &conf.Config{
			Builtins: conf.BuiltinsConfig{
				Resources: conf.ResourcesConfig{
					SystemInfo: boolPtr(false),
				},
			},
		}

		registry := NewContextAwareRegistry(cm, false, nil, cfg)
		resources := registry.List()

		// Should not have system info
		found := make(map[string]bool)
		for _, r := range resources {
			found[r.URI] = true
		}
		if found[URISystemInfo] {
			t.Error("expected URISystemInfo to be disabled")
		}
	})
}

func TestContextAwareRegistry_List_DeterministicOrder(t *testing.T) {
	cm := database.NewClientManager([]conf.NamedDatabaseConfig{
		{Name: "db1", Host: "localhost", Port: 5432, Database: "test1"},
	})
	cfg := &conf.Config{
		Builtins: conf.BuiltinsConfig{
			Resources: conf.ResourcesConfig{
				SystemInfo: boolPtr(true),
			},
		},
	}

	registry := NewContextAwareRegistry(cm, false, nil, cfg)

	uris := []string{"pg://custom/zebra", "pg://custom/apple", "pg://custom/mango", "pg://custom/banana", "pg://custom/cherry"}
	for _, uri := range uris {
		def := definitions.ResourceDefinition{
			URI:      uri,
			Name:     uri,
			Type:     "static",
			MimeType: "application/json",
			Data:     map[string]interface{}{"key": "value"},
		}
		if err := registry.RegisterStatic(def); err != nil {
			t.Fatalf("RegisterStatic(%q) failed: %v", uri, err)
		}
	}

	first := registry.List()
	for i := 1; i < len(first); i++ {
		if first[i-1].URI > first[i].URI {
			t.Fatalf("List() is not sorted: %q appears before %q", first[i-1].URI, first[i].URI)
		}
	}

	// A single call cannot detect nondeterministic map iteration on its
	// own; call repeatedly and require every call to match the first.
	for i := 0; i < 50; i++ {
		next := registry.List()
		if len(next) != len(first) {
			t.Fatalf("List() length changed between calls: %d vs %d", len(first), len(next))
		}
		for j := range first {
			if first[j].URI != next[j].URI {
				t.Fatalf("List() order changed between calls at index %d: %q vs %q", j, first[j].URI, next[j].URI)
			}
		}
	}
}

// TestContextAwareRegistry_List_DeterministicOrder_EqualURIs covers a case
// no current caller triggers: two custom resources registered under
// different keys whose Definitions both advertise the same URI.
// sort.Slice gives no ordering guarantee between elements that compare
// equal, so relying on URI alone would leave these two entries' relative
// order exactly as nondeterministic as before the fix. List() breaks the
// tie with the registration key, which is unique by construction.
func TestContextAwareRegistry_List_DeterministicOrder_EqualURIs(t *testing.T) {
	cm := database.NewClientManager([]conf.NamedDatabaseConfig{})
	cfg := &conf.Config{
		Builtins: conf.BuiltinsConfig{
			Resources: conf.ResourcesConfig{
				SystemInfo: boolPtr(false),
			},
		},
	}

	registry := NewContextAwareRegistry(cm, false, nil, cfg)
	// Descriptions are deliberately the inverse of key order ("key_a" <
	// "key_b", but its Description, "second", sorts after "first"), so a
	// tiebreak that accidentally used Description instead of the
	// registration key would produce the opposite -- and wrong -- order.
	registry.customResources["key_b"] = customResource{definition: mcp.Resource{URI: "dup", Description: "first"}}
	registry.customResources["key_a"] = customResource{definition: mcp.Resource{URI: "dup", Description: "second"}}
	registry.customResources["zzz"] = customResource{definition: mcp.Resource{URI: "zzz"}}

	descOrder := func(resources []mcp.Resource) []string {
		out := []string{}
		for _, resource := range resources {
			if resource.URI == "dup" {
				out = append(out, resource.Description)
			}
		}
		return out
	}

	want := []string{"second", "first"} // key_a, key_b
	for i := 0; i < 50; i++ {
		got := descOrder(registry.List())
		if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
			t.Fatalf("relative order of equal-URI entries: got %v, want %v (key-sorted)", got, want)
		}
	}
}

func TestContextAwareRegistry_Read_DisabledResource(t *testing.T) {
	cm := database.NewClientManager([]conf.NamedDatabaseConfig{
		{Name: "db1", Host: "localhost", Port: 5432, Database: "test1"},
	})

	cfg := &conf.Config{
		Builtins: conf.BuiltinsConfig{
			Resources: conf.ResourcesConfig{
				SystemInfo: boolPtr(false),
			},
		},
	}

	registry := NewContextAwareRegistry(cm, false, nil, cfg)

	// Reading disabled resource should return error content
	content, err := registry.Read(context.Background(), URISystemInfo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check the content indicates resource is not available
	if len(content.Contents) == 0 {
		t.Fatal("expected content")
	}
	if content.Contents[0].Text == "" {
		t.Error("expected error message in content")
	}
}

func TestContextAwareRegistry_Read_NotFound(t *testing.T) {
	skipIfNoDatabase(t)

	cm := database.NewClientManager([]conf.NamedDatabaseConfig{
		{Name: "db1", Host: "localhost", Port: 5432, Database: "test1"},
	})

	cfg := &conf.Config{
		Builtins: conf.BuiltinsConfig{
			Resources: conf.ResourcesConfig{
				SystemInfo: boolPtr(true),
			},
		},
	}

	registry := NewContextAwareRegistry(cm, false, nil, cfg)

	// Reading non-existent resource should return not found content
	content, err := registry.Read(context.Background(), "pg://nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check the content indicates resource not found
	if len(content.Contents) == 0 {
		t.Fatal("expected content")
	}
	if content.Contents[0].Text != "Resource not found: pg://nonexistent" {
		t.Errorf("unexpected content: %s", content.Contents[0].Text)
	}
}

func TestContextAwareRegistry_Read_AuthRequired(t *testing.T) {
	cm := database.NewClientManager([]conf.NamedDatabaseConfig{
		{Name: "db1", Host: "localhost", Port: 5432, Database: "test1"},
	})

	cfg := &conf.Config{
		Builtins: conf.BuiltinsConfig{
			Resources: conf.ResourcesConfig{
				SystemInfo: boolPtr(true),
			},
		},
	}

	// Auth enabled but no token in context
	registry := NewContextAwareRegistry(cm, true, nil, cfg)

	// Reading without token should return error
	content, err := registry.Read(context.Background(), URISystemInfo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have error content about missing token
	if len(content.Contents) == 0 {
		t.Fatal("expected content")
	}
	if content.Contents[0].Text == "" {
		t.Error("expected error message")
	}
}

func TestContextAwareRegistry_Read_WithToken(t *testing.T) {
	skipIfNoDatabase(t)

	cm := database.NewClientManager([]conf.NamedDatabaseConfig{
		{Name: "db1", Host: "localhost", Port: 5432, Database: "test1"},
	})

	cfg := &conf.Config{
		Builtins: conf.BuiltinsConfig{
			Resources: conf.ResourcesConfig{
				SystemInfo: boolPtr(true),
			},
		},
	}

	registry := NewContextAwareRegistry(cm, true, nil, cfg)

	// Add token to context
	ctx := context.WithValue(context.Background(), auth.TokenHashContextKey, "test-token-hash")

	// Reading with token - will fail because no DB connection, but exercises the code path
	content, err := registry.Read(ctx, URISystemInfo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have content (either success or error about DB connection)
	if len(content.Contents) == 0 {
		t.Fatal("expected content")
	}
}

func TestContextAwareRegistry_GetClient_AuthDisabled(t *testing.T) {
	cm := database.NewClientManager([]conf.NamedDatabaseConfig{
		{Name: "db1", Host: "localhost", Port: 5432, Database: "test1"},
	})

	cfg := &conf.Config{
		Builtins: conf.BuiltinsConfig{
			Resources: conf.ResourcesConfig{
				SystemInfo: boolPtr(true),
			},
		},
	}

	registry := NewContextAwareRegistry(cm, false, nil, cfg)

	// When auth is disabled, getClient uses "default" key
	// This exercises the code path - it may return an error or a client
	// depending on ClientManager implementation
	_, _ = registry.getClient(context.Background())
	// Test passes if no panic occurs - we're just testing the code path
}

func TestContextAwareRegistry_GetClient_AuthEnabled_NoToken(t *testing.T) {
	cm := database.NewClientManager([]conf.NamedDatabaseConfig{
		{Name: "db1", Host: "localhost", Port: 5432, Database: "test1"},
	})

	cfg := &conf.Config{}

	registry := NewContextAwareRegistry(cm, true, nil, cfg)

	_, err := registry.getClient(context.Background())
	if err == nil {
		t.Fatal("expected error for missing token")
	}
	if err.Error() != "no authentication token found in request context" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestContextAwareRegistry_DefaultNilConfig(t *testing.T) {
	cm := database.NewClientManager([]conf.NamedDatabaseConfig{
		{Name: "db1", Host: "localhost", Port: 5432, Database: "test1"},
	})

	// With nil values (defaults to enabled)
	cfg := &conf.Config{
		Builtins: conf.BuiltinsConfig{
			Resources: conf.ResourcesConfig{}, // All nil = all enabled
		},
	}

	registry := NewContextAwareRegistry(cm, false, nil, cfg)
	resources := registry.List()

	// Should have built-in resource since nil defaults to enabled
	if len(resources) < 1 {
		t.Errorf("expected at least 1 resource with default config, got %d", len(resources))
	}
}
