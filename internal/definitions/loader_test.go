/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package definitions

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefinitions_YAML(t *testing.T) {
	content := `
prompts:
  - name: test-prompt
    description: Test prompt
    arguments:
      - name: arg1
        description: First argument
        required: true
    messages:
      - role: user
        content:
          type: text
          text: "Test {{arg1}}"

resources:
  - uri: custom://test
    name: Test Resource
    type: static
    data: "test value"
`

	tmpFile := createTempFile(t, "test-*.yaml", content)
	defer os.Remove(tmpFile)

	defs, err := LoadDefinitions(tmpFile)
	if err != nil {
		t.Fatalf("Failed to load definitions: %v", err)
	}

	if len(defs.Prompts) != 1 {
		t.Errorf("Expected 1 prompt, got %d", len(defs.Prompts))
	}

	if len(defs.Resources) != 1 {
		t.Errorf("Expected 1 resource, got %d", len(defs.Resources))
	}
}

func TestLoadDefinitions_InvalidYAML(t *testing.T) {
	content := `
prompts:
  - name: test
    invalid: : yaml
`
	tmpFile := createTempFile(t, "test-*.yaml", content)
	defer os.Remove(tmpFile)

	_, err := LoadDefinitions(tmpFile)
	if err == nil {
		t.Error("Expected error for invalid YAML, got nil")
	}
}

func TestLoadDefinitions_UnsupportedFormat(t *testing.T) {
	content := `test content`
	tmpFile := createTempFile(t, "test-*.txt", content)
	defer os.Remove(tmpFile)

	_, err := LoadDefinitions(tmpFile)
	if err == nil {
		t.Error("Expected error for unsupported format, got nil")
	}
}

func TestLoadDefinitions_FileNotFound(t *testing.T) {
	_, err := LoadDefinitions("/nonexistent/file.json")
	if err == nil {
		t.Error("Expected error for nonexistent file, got nil")
	}
}

func TestLoadDefinitions_EmptyFile(t *testing.T) {
	content := ``
	tmpFile := createTempFile(t, "test-*.yaml", content)
	defer os.Remove(tmpFile)

	defs, err := LoadDefinitions(tmpFile)
	if err != nil {
		t.Fatalf("Failed to load empty definitions: %v", err)
	}

	if len(defs.Prompts) != 0 {
		t.Errorf("Expected 0 prompts, got %d", len(defs.Prompts))
	}

	if len(defs.Resources) != 0 {
		t.Errorf("Expected 0 resources, got %d", len(defs.Resources))
	}
}

func createTempFile(t *testing.T, pattern string, content string) string {
	t.Helper()
	tmpFile, err := os.CreateTemp("", pattern)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}
	return tmpFile.Name()
}

func TestLoadDefinitions_ComplexData(t *testing.T) {
	content := `
resources:
  - uri: custom://scalar
    name: Scalar
    type: static
    data: "string value"
  - uri: custom://array
    name: Array
    type: static
    data: ["a", "b", "c"]
  - uri: custom://2d-array
    name: 2D Array
    type: static
    data: [["a", "b"], ["c", "d"]]
  - uri: custom://object
    name: Object
    type: static
    data:
      key: "value"
      num: 42
`

	tmpFile := createTempFile(t, "test-*.yaml", content)
	defer os.Remove(tmpFile)

	defs, err := LoadDefinitions(tmpFile)
	if err != nil {
		t.Fatalf("Failed to load definitions: %v", err)
	}

	if len(defs.Resources) != 4 {
		t.Errorf("Expected 4 resources, got %d", len(defs.Resources))
	}
}

func TestLoadDefinitions_SQLResource(t *testing.T) {
	content := `
resources:
  - uri: custom://users
    name: Users
    type: sql
    sql: "SELECT * FROM users"
`

	tmpFile := createTempFile(t, "test-*.yaml", content)
	defer os.Remove(tmpFile)

	defs, err := LoadDefinitions(tmpFile)
	if err != nil {
		t.Fatalf("Failed to load definitions: %v", err)
	}

	if len(defs.Resources) != 1 {
		t.Errorf("Expected 1 resource, got %d", len(defs.Resources))
	}

	if defs.Resources[0].Type != "sql" {
		t.Errorf("Expected SQL resource, got %s", defs.Resources[0].Type)
	}

	if defs.Resources[0].SQL != "SELECT * FROM users" {
		t.Errorf("Expected SQL query, got %s", defs.Resources[0].SQL)
	}
}

func TestLoadDefinitions_WithMimeType(t *testing.T) {
	content := `
resources:
  - uri: custom://test
    name: Test
    type: static
    mimeType: "text/plain"
    data: "test"
`

	tmpFile := createTempFile(t, "test-*.yaml", content)
	defer os.Remove(tmpFile)

	defs, err := LoadDefinitions(tmpFile)
	if err != nil {
		t.Fatalf("Failed to load definitions: %v", err)
	}

	if defs.Resources[0].MimeType != "text/plain" {
		t.Errorf("Expected mimeType 'text/plain', got '%s'", defs.Resources[0].MimeType)
	}
}

func TestLoadDefinitions_YMLExtension(t *testing.T) {
	content := `
resources:
  - uri: custom://test
    name: Test
    type: static
    data: "test"
`

	tmpFile := createTempFile(t, "test-*.yml", content)
	defer os.Remove(tmpFile)

	_, err := LoadDefinitions(tmpFile)
	if err != nil {
		t.Fatalf("Failed to load .yml file: %v", err)
	}
}

func TestLoadDefinitions_RelativePath(t *testing.T) {
	// Create a temp directory and file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.yaml")
	content := `prompts: []
resources: []`

	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	_, err := LoadDefinitions(tmpFile)
	if err != nil {
		t.Fatalf("Failed to load definitions from relative path: %v", err)
	}
}
