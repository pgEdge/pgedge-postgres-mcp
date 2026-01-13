/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package kbchunker

import (
	"strings"
	"testing"
)

func TestElementTypeString(t *testing.T) {
	tests := []struct {
		elementType ElementType
		expected    string
	}{
		{Paragraph, "paragraph"},
		{CodeBlock, "code_block"},
		{Table, "table"},
		{List, "list"},
		{Blockquote, "blockquote"},
		{ElementType(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.elementType.String()
			if result != tt.expected {
				t.Errorf("ElementType.String() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestParseStructuralElements_CodeBlock(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []ElementType
	}{
		{
			name: "simple code block",
			content: `Some text

` + "```" + `
code here
` + "```" + `

More text`,
			expected: []ElementType{Paragraph, CodeBlock, Paragraph},
		},
		{
			name:     "code block with language",
			content:  "```go\nfunc main() {\n}\n```",
			expected: []ElementType{CodeBlock},
		},
		{
			name:     "multiple code blocks",
			content:  "```\nfirst\n```\n\n```\nsecond\n```",
			expected: []ElementType{CodeBlock, CodeBlock},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			elements := parseStructuralElements(tt.content)
			if len(elements) != len(tt.expected) {
				t.Errorf("got %d elements, want %d", len(elements), len(tt.expected))
				for i, e := range elements {
					t.Logf("  element %d: %s = %q", i, e.Type.String(), e.Content)
				}
				return
			}
			for i, e := range elements {
				if e.Type != tt.expected[i] {
					t.Errorf("element %d: got type %s, want %s", i, e.Type.String(), tt.expected[i].String())
				}
			}
		})
	}
}

func TestParseStructuralElements_Table(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []ElementType
	}{
		{
			name: "simple table",
			content: `| Header 1 | Header 2 |
|----------|----------|
| Cell 1   | Cell 2   |`,
			expected: []ElementType{Table},
		},
		{
			name: "table with surrounding text",
			content: `Some intro text

| Col A | Col B |
|-------|-------|
| 1     | 2     |

Some outro text`,
			expected: []ElementType{Paragraph, Table, Paragraph},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			elements := parseStructuralElements(tt.content)
			if len(elements) != len(tt.expected) {
				t.Errorf("got %d elements, want %d", len(elements), len(tt.expected))
				for i, e := range elements {
					t.Logf("  element %d: %s = %q", i, e.Type.String(), e.Content)
				}
				return
			}
			for i, e := range elements {
				if e.Type != tt.expected[i] {
					t.Errorf("element %d: got type %s, want %s", i, e.Type.String(), tt.expected[i].String())
				}
			}
		})
	}
}

func TestParseStructuralElements_List(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []ElementType
	}{
		{
			name: "unordered list",
			content: `- Item 1
- Item 2
- Item 3`,
			expected: []ElementType{List},
		},
		{
			name: "ordered list",
			content: `1. First
2. Second
3. Third`,
			expected: []ElementType{List},
		},
		{
			name: "nested list",
			content: `- Parent
    - Child 1
    - Child 2
- Another parent`,
			expected: []ElementType{List},
		},
		{
			name: "list with surrounding text",
			content: `Introduction:

- Item A
- Item B

Conclusion`,
			expected: []ElementType{Paragraph, List, Paragraph},
		},
		{
			name: "mixed list markers",
			content: `* Star item
- Dash item
+ Plus item`,
			expected: []ElementType{List},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			elements := parseStructuralElements(tt.content)
			if len(elements) != len(tt.expected) {
				t.Errorf("got %d elements, want %d", len(elements), len(tt.expected))
				for i, e := range elements {
					t.Logf("  element %d: %s = %q", i, e.Type.String(), e.Content)
				}
				return
			}
			for i, e := range elements {
				if e.Type != tt.expected[i] {
					t.Errorf("element %d: got type %s, want %s", i, e.Type.String(), tt.expected[i].String())
				}
			}
		})
	}
}

func TestParseStructuralElements_Blockquote(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []ElementType
	}{
		{
			name: "simple blockquote",
			content: `> This is a quote
> spanning multiple lines`,
			expected: []ElementType{Blockquote},
		},
		{
			name: "blockquote with surrounding text",
			content: `Some text

> A famous quote

Attribution`,
			expected: []ElementType{Paragraph, Blockquote, Paragraph},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			elements := parseStructuralElements(tt.content)
			if len(elements) != len(tt.expected) {
				t.Errorf("got %d elements, want %d", len(elements), len(tt.expected))
				for i, e := range elements {
					t.Logf("  element %d: %s = %q", i, e.Type.String(), e.Content)
				}
				return
			}
			for i, e := range elements {
				if e.Type != tt.expected[i] {
					t.Errorf("element %d: got type %s, want %s", i, e.Type.String(), tt.expected[i].String())
				}
			}
		})
	}
}

func TestParseStructuralElements_Paragraph(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []ElementType
	}{
		{
			name:     "single paragraph",
			content:  "This is a simple paragraph of text.",
			expected: []ElementType{Paragraph},
		},
		{
			name: "multiple paragraphs",
			content: `First paragraph.

Second paragraph.

Third paragraph.`,
			expected: []ElementType{Paragraph, Paragraph, Paragraph},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			elements := parseStructuralElements(tt.content)
			if len(elements) != len(tt.expected) {
				t.Errorf("got %d elements, want %d", len(elements), len(tt.expected))
				for i, e := range elements {
					t.Logf("  element %d: %s = %q", i, e.Type.String(), e.Content)
				}
				return
			}
			for i, e := range elements {
				if e.Type != tt.expected[i] {
					t.Errorf("element %d: got type %s, want %s", i, e.Type.String(), tt.expected[i].String())
				}
			}
		})
	}
}

func TestParseStructuralElements_Mixed(t *testing.T) {
	// Note: This tests parseStructuralElements which operates on section CONTENT
	// (after headings have been extracted by parseMarkdownSections).
	// So we test without markdown headings here.
	content := `This is an introductory paragraph explaining the topic.

Here is some code:

` + "```go" + `
func hello() {
    fmt.Println("Hello, World!")
}
` + "```" + `

| Name  | Value |
|-------|-------|
| foo   | 1     |
| bar   | 2     |

- Feature one
- Feature two
    - Sub-feature A
    - Sub-feature B

> Note: This is important!

Final thoughts.`

	elements := parseStructuralElements(content)

	// Expected sequence of element types
	expectedTypes := []ElementType{
		Paragraph,  // "This is an introductory paragraph..."
		Paragraph,  // "Here is some code:"
		CodeBlock,  // The go code
		Table,      // The data table
		List,       // Features list
		Blockquote, // The note
		Paragraph,  // "Final thoughts."
	}

	if len(elements) != len(expectedTypes) {
		t.Errorf("got %d elements, want %d", len(elements), len(expectedTypes))
		for i, e := range elements {
			t.Logf("  element %d: %s = %q", i, e.Type.String(), truncate(e.Content, 50))
		}
		return
	}

	for i, e := range elements {
		if e.Type != expectedTypes[i] {
			t.Errorf("element %d: got type %s, want %s",
				i, e.Type.String(), expectedTypes[i].String())
		}
	}
}

func TestParseCodeBlock_Preserves_Content(t *testing.T) {
	content := "```python\ndef greet(name):\n    return f\"Hello, {name}!\"\n```"
	elements := parseStructuralElements(content)

	if len(elements) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elements))
	}

	if elements[0].Type != CodeBlock {
		t.Errorf("expected CodeBlock, got %s", elements[0].Type.String())
	}

	// The content should include the fences
	if !strings.Contains(elements[0].Content, "```python") {
		t.Error("code block should include opening fence with language")
	}
	if !strings.Contains(elements[0].Content, "def greet") {
		t.Error("code block should include the code")
	}
}

func TestParseTable_Preserves_Structure(t *testing.T) {
	content := `| Column A | Column B | Column C |
|----------|----------|----------|
| Value 1  | Value 2  | Value 3  |
| Value 4  | Value 5  | Value 6  |`

	elements := parseStructuralElements(content)

	if len(elements) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elements))
	}

	if elements[0].Type != Table {
		t.Errorf("expected Table, got %s", elements[0].Type.String())
	}

	// Count the rows
	rows := strings.Split(elements[0].Content, "\n")
	if len(rows) != 4 {
		t.Errorf("expected 4 table rows, got %d", len(rows))
	}
}

func TestParseList_Preserves_Nesting(t *testing.T) {
	content := `- Parent item 1
    - Child item 1a
    - Child item 1b
        - Grandchild
- Parent item 2`

	elements := parseStructuralElements(content)

	if len(elements) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elements))
	}

	if elements[0].Type != List {
		t.Errorf("expected List, got %s", elements[0].Type.String())
	}

	// All items should be in one list
	if !strings.Contains(elements[0].Content, "Grandchild") {
		t.Error("nested list should include grandchild item")
	}
	if !strings.Contains(elements[0].Content, "Parent item 2") {
		t.Error("list should include all parent items")
	}
}

func TestGetIndentation(t *testing.T) {
	tests := []struct {
		line     string
		expected int
	}{
		{"no indent", 0},
		{"  two spaces", 2},
		{"    four spaces", 4},
		{"\tone tab", 4},
		{"  \t  mixed", 8},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			result := getIndentation(tt.line)
			if result != tt.expected {
				t.Errorf("getIndentation(%q) = %d, want %d", tt.line, result, tt.expected)
			}
		})
	}
}

func TestParseStructuralElements_EmptyContent(t *testing.T) {
	elements := parseStructuralElements("")
	if len(elements) != 0 {
		t.Errorf("expected 0 elements for empty content, got %d", len(elements))
	}
}

func TestParseStructuralElements_OnlyWhitespace(t *testing.T) {
	elements := parseStructuralElements("   \n\n   \n")
	if len(elements) != 0 {
		t.Errorf("expected 0 elements for whitespace-only content, got %d", len(elements))
	}
}

// truncate is a helper for logging
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
