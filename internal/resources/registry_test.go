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
	"errors"
	"testing"

	"pgedge-postgres-mcp/internal/mcp"
)

func TestNewRegistry(t *testing.T) {
	registry := NewRegistry()

	if registry == nil {
		t.Fatal("NewRegistry() returned nil")
	}

	if registry.resources == nil {
		t.Error("resources map is nil")
	}

	if len(registry.resources) != 0 {
		t.Errorf("resources map should be empty, got %d entries", len(registry.resources))
	}
}

func TestRegister(t *testing.T) {
	registry := NewRegistry()

	resource := Resource{
		Definition: mcp.Resource{
			URI:         "test://resource",
			Name:        "Test Resource",
			Description: "A test resource",
			MimeType:    "text/plain",
		},
		Handler: func() (mcp.ResourceContent, error) {
			return mcp.ResourceContent{}, nil
		},
	}

	registry.Register("test://resource", resource)

	if len(registry.resources) != 1 {
		t.Errorf("Expected 1 resource, got %d", len(registry.resources))
	}

	retrieved, exists := registry.resources["test://resource"]
	if !exists {
		t.Error("Resource 'test://resource' was not registered")
	}

	if retrieved.Definition.URI != "test://resource" {
		t.Errorf("Resource URI = %q, want %q", retrieved.Definition.URI, "test://resource")
	}
}

func TestGet(t *testing.T) {
	registry := NewRegistry()

	resource := Resource{
		Definition: mcp.Resource{
			URI:         "existing://resource",
			Name:        "Existing Resource",
			Description: "An existing resource",
			MimeType:    "text/plain",
		},
		Handler: func() (mcp.ResourceContent, error) {
			return mcp.ResourceContent{}, nil
		},
	}

	registry.Register("existing://resource", resource)

	t.Run("existing resource", func(t *testing.T) {
		retrieved, exists := registry.Get("existing://resource")
		if !exists {
			t.Error("Get() returned exists=false for existing resource")
		}
		if retrieved.Definition.URI != "existing://resource" {
			t.Errorf("Resource URI = %q, want %q", retrieved.Definition.URI, "existing://resource")
		}
	})

	t.Run("non-existent resource", func(t *testing.T) {
		_, exists := registry.Get("non://existent")
		if exists {
			t.Error("Get() returned exists=true for non-existent resource")
		}
	})
}

func TestList(t *testing.T) {
	registry := NewRegistry()

	t.Run("empty registry", func(t *testing.T) {
		resources := registry.List()
		if len(resources) != 0 {
			t.Errorf("List() returned %d resources, want 0", len(resources))
		}
	})

	t.Run("with resources", func(t *testing.T) {
		resource1 := Resource{
			Definition: mcp.Resource{
				URI:         "resource://one",
				Name:        "Resource One",
				Description: "First resource",
				MimeType:    "text/plain",
			},
			Handler: func() (mcp.ResourceContent, error) {
				return mcp.ResourceContent{}, nil
			},
		}

		resource2 := Resource{
			Definition: mcp.Resource{
				URI:         "resource://two",
				Name:        "Resource Two",
				Description: "Second resource",
				MimeType:    "application/json",
			},
			Handler: func() (mcp.ResourceContent, error) {
				return mcp.ResourceContent{}, nil
			},
		}

		registry.Register("resource://one", resource1)
		registry.Register("resource://two", resource2)

		resources := registry.List()
		if len(resources) != 2 {
			t.Errorf("List() returned %d resources, want 2", len(resources))
		}

		// Verify both resources are in the list
		uris := make(map[string]bool)
		for _, resource := range resources {
			uris[resource.URI] = true
		}

		if !uris["resource://one"] {
			t.Error("List() missing 'resource://one'")
		}
		if !uris["resource://two"] {
			t.Error("List() missing 'resource://two'")
		}
	})
}

func TestList_DeterministicOrder(t *testing.T) {
	registry := NewRegistry()

	uris := []string{"resource://zebra", "resource://apple", "resource://mango", "resource://banana", "resource://cherry"}
	for _, uri := range uris {
		registry.Register(uri, Resource{
			Definition: mcp.Resource{URI: uri},
			Handler: func() (mcp.ResourceContent, error) {
				return mcp.ResourceContent{}, nil
			},
		})
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

// TestList_DeterministicOrder_EqualURIs covers a case no current caller
// triggers: two registrations under different keys whose Definitions both
// advertise the same URI. sort.Slice gives no ordering guarantee between
// elements that compare equal, so relying on URI alone would leave these
// two entries' relative order exactly as nondeterministic as before the
// fix. List() breaks the tie with the registration key, which is unique
// by construction.
func TestList_DeterministicOrder_EqualURIs(t *testing.T) {
	registry := NewRegistry()
	// Descriptions are deliberately the inverse of key order ("key_a" <
	// "key_b", but its Description, "second", sorts after "first"), so a
	// tiebreak that accidentally used Description instead of the
	// registration key would produce the opposite -- and wrong -- order.
	registry.resources["key_b"] = Resource{Definition: mcp.Resource{URI: "dup", Description: "first"}}
	registry.resources["key_a"] = Resource{Definition: mcp.Resource{URI: "dup", Description: "second"}}
	registry.resources["zzz"] = Resource{Definition: mcp.Resource{URI: "zzz"}}

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

func TestRead(t *testing.T) {
	registry := NewRegistry()

	t.Run("successful read", func(t *testing.T) {
		callCount := 0
		resource := Resource{
			Definition: mcp.Resource{
				URI:         "counter://resource",
				Name:        "Counter Resource",
				Description: "Counts reads",
				MimeType:    "text/plain",
			},
			Handler: func() (mcp.ResourceContent, error) {
				callCount++
				return mcp.ResourceContent{
					URI:      "counter://resource",
					MimeType: "text/plain",
					Contents: []mcp.ContentItem{
						{
							Type: "text",
							Text: "Resource content",
						},
					},
				}, nil
			},
		}

		registry.Register("counter://resource", resource)

		content, err := registry.Read(context.Background(), "counter://resource")
		if err != nil {
			t.Errorf("Read() unexpected error: %v", err)
		}

		if callCount != 1 {
			t.Errorf("Handler was called %d times, want 1", callCount)
		}

		if content.URI != "counter://resource" {
			t.Errorf("Content URI = %q, want %q", content.URI, "counter://resource")
		}

		if len(content.Contents) != 1 {
			t.Errorf("Content has %d items, want 1", len(content.Contents))
		}

		if content.Contents[0].Text != "Resource content" {
			t.Errorf("Content text = %q, want %q", content.Contents[0].Text, "Resource content")
		}
	})

	t.Run("non-existent resource", func(t *testing.T) {
		// A protocol-level error (mcp.ErrResourceNotFound), not a
		// disguised success: an unknown resource URI is a JSON-RPC
		// error per the MCP spec (-32002 for legacy clients, -32602 for
		// modern -- see resourceNotFoundCode in internal/mcp), and this
		// registry's Read must let the MCP layer distinguish that case
		// from an actual resource whose content happens to describe an
		// error, which handleResourceRead/handleResourceReadHTTP do by
		// checking errors.Is(err, mcp.ErrResourceNotFound).
		_, err := registry.Read(context.Background(), "non://existent")
		if !errors.Is(err, mcp.ErrResourceNotFound) {
			t.Errorf("Read() error = %v, want mcp.ErrResourceNotFound", err)
		}
	})

	t.Run("resource with JSON mime type", func(t *testing.T) {
		resource := Resource{
			Definition: mcp.Resource{
				URI:         "json://data",
				Name:        "JSON Data",
				Description: "Returns JSON data",
				MimeType:    "application/json",
			},
			Handler: func() (mcp.ResourceContent, error) {
				return mcp.ResourceContent{
					URI:      "json://data",
					MimeType: "application/json",
					Contents: []mcp.ContentItem{
						{
							Type: "text",
							Text: `{"key": "value"}`,
						},
					},
				}, nil
			},
		}

		registry.Register("json://data", resource)

		content, err := registry.Read(context.Background(), "json://data")
		if err != nil {
			t.Errorf("Read() unexpected error: %v", err)
		}

		if content.MimeType != "application/json" {
			t.Errorf("Content MimeType = %q, want %q", content.MimeType, "application/json")
		}

		if content.Contents[0].Text != `{"key": "value"}` {
			t.Errorf("Content text = %q, want %q", content.Contents[0].Text, `{"key": "value"}`)
		}
	})

	t.Run("multiple registrations overwrite", func(t *testing.T) {
		version := 1
		resource1 := Resource{
			Definition: mcp.Resource{
				URI:         "versioned://resource",
				Name:        "Version 1",
				Description: "First version",
				MimeType:    "text/plain",
			},
			Handler: func() (mcp.ResourceContent, error) {
				return mcp.ResourceContent{
					URI: "versioned://resource",
					Contents: []mcp.ContentItem{
						{
							Type: "text",
							Text: "Version 1",
						},
					},
				}, nil
			},
		}

		registry.Register("versioned://resource", resource1)

		// Register again with different handler
		resource2 := Resource{
			Definition: mcp.Resource{
				URI:         "versioned://resource",
				Name:        "Version 2",
				Description: "Second version",
				MimeType:    "text/plain",
			},
			Handler: func() (mcp.ResourceContent, error) {
				version = 2
				return mcp.ResourceContent{
					URI: "versioned://resource",
					Contents: []mcp.ContentItem{
						{
							Type: "text",
							Text: "Version 2",
						},
					},
				}, nil
			},
		}

		registry.Register("versioned://resource", resource2)

		content, err := registry.Read(context.Background(), "versioned://resource")
		if err != nil {
			t.Errorf("Read() unexpected error: %v", err)
		}

		if version != 2 {
			t.Errorf("Version = %d, want 2 (latest registration)", version)
		}

		if content.Contents[0].Text != "Version 2" {
			t.Errorf("Content text = %q, want %q", content.Contents[0].Text, "Version 2")
		}
	})

	t.Run("resource with multiple content items", func(t *testing.T) {
		resource := Resource{
			Definition: mcp.Resource{
				URI:         "multi://content",
				Name:        "Multi Content",
				Description: "Returns multiple content items",
				MimeType:    "text/plain",
			},
			Handler: func() (mcp.ResourceContent, error) {
				return mcp.ResourceContent{
					URI: "multi://content",
					Contents: []mcp.ContentItem{
						{
							Type: "text",
							Text: "First item",
						},
						{
							Type: "text",
							Text: "Second item",
						},
						{
							Type: "text",
							Text: "Third item",
						},
					},
				}, nil
			},
		}

		registry.Register("multi://content", resource)

		content, err := registry.Read(context.Background(), "multi://content")
		if err != nil {
			t.Errorf("Read() unexpected error: %v", err)
		}

		if len(content.Contents) != 3 {
			t.Errorf("Content has %d items, want 3", len(content.Contents))
		}

		expectedTexts := []string{"First item", "Second item", "Third item"}
		for i, expected := range expectedTexts {
			if content.Contents[i].Text != expected {
				t.Errorf("Content[%d].Text = %q, want %q", i, content.Contents[i].Text, expected)
			}
		}
	})
}
