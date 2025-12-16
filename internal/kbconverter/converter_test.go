/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package kbconverter

import (
	"strings"
	"testing"

	"pgedge-postgres-mcp/internal/kbtypes"
)

func TestDetectDocumentType(t *testing.T) {
	tests := []struct {
		filename string
		expected kbtypes.DocumentType
	}{
		{"test.html", kbtypes.TypeHTML},
		{"test.htm", kbtypes.TypeHTML},
		{"test.md", kbtypes.TypeMarkdown},
		{"test.rst", kbtypes.TypeReStructuredText},
		{"test.sgml", kbtypes.TypeSGML},
		{"test.sgm", kbtypes.TypeSGML},
		{"test.xml", kbtypes.TypeSGML},
		{"test.txt", kbtypes.TypeUnknown},
		{"test", kbtypes.TypeUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := DetectDocumentType(tt.filename)
			if result != tt.expected {
				t.Errorf("DetectDocumentType(%q) = %v, expected %v", tt.filename, result, tt.expected)
			}
		})
	}
}

func TestConvertHTML(t *testing.T) {
	html := `
<!DOCTYPE html>
<html>
<head><title>Test Document</title></head>
<body>
<h1>Main Heading</h1>
<p>This is a paragraph.</p>
<h2>Sub Heading</h2>
<p>Another paragraph with <strong>bold</strong> text.</p>
</body>
</html>
`

	markdown, title, err := convertHTML([]byte(html))
	if err != nil {
		t.Fatalf("convertHTML failed: %v", err)
	}

	if title != "Test Document" {
		t.Errorf("Expected title 'Test Document', got '%s'", title)
	}

	if !strings.Contains(markdown, "# Test Document") {
		t.Error("Markdown should contain title as H1")
	}

	if !strings.Contains(markdown, "paragraph") {
		t.Error("Markdown should contain content")
	}
}

func TestProcessMarkdown(t *testing.T) {
	markdown := `# Test Title

This is content.

## Section 1

More content here.
`

	result, title, err := processMarkdown([]byte(markdown))
	if err != nil {
		t.Fatalf("processMarkdown failed: %v", err)
	}

	if title != "Test Title" {
		t.Errorf("Expected title 'Test Title', got '%s'", title)
	}

	if result != markdown {
		t.Error("Markdown should be unchanged")
	}
}

func TestExtractMarkdownTitle(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "simple title",
			content:  "# My Title\n\nContent",
			expected: "My Title",
		},
		{
			name:     "with front matter",
			content:  "---\ntitle: Something\n---\n\n# Real Title\n\nContent",
			expected: "Real Title",
		},
		{
			name:     "no title",
			content:  "Just content without a heading",
			expected: "",
		},
		{
			name:     "multiple headings",
			content:  "# First Title\n\n## Second Title",
			expected: "First Title",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractMarkdownTitle(tt.content)
			if result != tt.expected {
				t.Errorf("extractMarkdownTitle() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestConvertRST(t *testing.T) {
	rst := `
Test Document
=============

This is a paragraph.

Section 1
---------

Content for section 1.
`

	markdown, title, err := convertRST([]byte(rst))
	if err != nil {
		t.Fatalf("convertRST failed: %v", err)
	}

	if title == "" {
		t.Error("Should extract title from RST")
	}

	if !strings.Contains(markdown, "Test Document") {
		t.Error("Markdown should contain document title")
	}

	if !strings.Contains(markdown, "Section 1") {
		t.Error("Markdown should contain section heading")
	}
}

func TestExtractRSTTitle(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name: "underline style",
			content: `My Title
========

Content here.`,
			expected: "My Title",
		},
		{
			name: "overline and underline",
			content: `========
My Title
========

Content here.`,
			expected: "My Title",
		},
		{
			name: "no title",
			content: `Just some content
without proper heading markers`,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractRSTTitle(tt.content)
			if result != tt.expected {
				t.Errorf("extractRSTTitle() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestConvertSGML(t *testing.T) {
	sgml := `
<!DOCTYPE book PUBLIC "-//OASIS//DTD DocBook V4.2//EN">
<book>
<title>Test Document</title>
<chapter>
<title>Chapter 1</title>
<para>This is a paragraph.</para>
<sect1>
<title>Section 1.1</title>
<para>Section content.</para>
</sect1>
</chapter>
</book>
`

	markdown, title, err := convertSGML([]byte(sgml))
	if err != nil {
		t.Fatalf("convertSGML failed: %v", err)
	}

	if title != "Test Document" {
		t.Errorf("Expected title 'Test Document', got '%s'", title)
	}

	if !strings.Contains(markdown, "Chapter 1") {
		t.Error("Markdown should contain chapter heading")
	}

	if !strings.Contains(markdown, "Section 1.1") {
		t.Error("Markdown should contain section heading")
	}

	if !strings.Contains(markdown, "paragraph") {
		t.Error("Markdown should contain content")
	}
}

func TestIsSupported(t *testing.T) {
	tests := []struct {
		filename string
		expected bool
	}{
		{"test.html", true},
		{"test.md", true},
		{"test.rst", true},
		{"test.sgml", true},
		{"test.txt", false},
		{"test.pdf", false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := IsSupported(tt.filename)
			if result != tt.expected {
				t.Errorf("IsSupported(%q) = %v, expected %v", tt.filename, result, tt.expected)
			}
		})
	}
}

func TestConvertUnsupportedFormat(t *testing.T) {
	_, _, err := Convert([]byte("content"), kbtypes.TypeUnknown)
	if err != ErrUnsupportedFormat {
		t.Errorf("Expected ErrUnsupportedFormat, got %v", err)
	}
}

func TestGetSupportedExtensions(t *testing.T) {
	extensions := GetSupportedExtensions()

	expectedExtensions := []string{".html", ".htm", ".md", ".rst", ".sgml", ".sgm", ".xml"}

	if len(extensions) != len(expectedExtensions) {
		t.Errorf("Expected %d extensions, got %d", len(expectedExtensions), len(extensions))
	}

	// Check that all expected extensions are present
	extMap := make(map[string]bool)
	for _, ext := range extensions {
		extMap[ext] = true
	}

	for _, expected := range expectedExtensions {
		if !extMap[expected] {
			t.Errorf("Extension %s not found in supported extensions", expected)
		}
	}
}

func TestCleanMarkdownForRAG(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "strip image with alt text",
			input:    "Here is an image: ![diagram of flow](images/flow.png) in the text.",
			expected: "Here is an image: diagram of flow in the text.",
		},
		{
			name:     "strip image without alt text",
			input:    "Here is an image: ![](images/empty.png) gone.",
			expected: "Here is an image: gone.",
		},
		{
			name:     "strip link keep text",
			input:    "Check the [PostgreSQL docs](https://postgresql.org/docs) for more.",
			expected: "Check the PostgreSQL docs for more.",
		},
		{
			name:     "strip empty link",
			input:    "Empty link [](https://example.com) removed.",
			expected: "Empty link removed.",
		},
		{
			name:     "strip reference-style link definitions",
			input:    "Some text.\n\n[link1]: https://example.com\n[link2]: https://other.com \"Title\"\n\nMore text.",
			expected: "Some text.\n\nMore text.",
		},
		{
			name:     "collapse multiple spaces",
			input:    "Too   many    spaces    here.",
			expected: "Too many spaces here.",
		},
		{
			name:     "preserve code indentation",
			input:    "Code:\n\n    if x > 0:\n        print(x)\n\nDone.",
			expected: "Code:\n\n    if x > 0:\n        print(x)\n\nDone.",
		},
		{
			name:     "collapse excessive newlines",
			input:    "First paragraph.\n\n\n\n\nSecond paragraph.",
			expected: "First paragraph.\n\nSecond paragraph.",
		},
		{
			name:     "remove trailing whitespace",
			input:    "Line with trailing spaces   \nAnother line  ",
			expected: "Line with trailing spaces\nAnother line",
		},
		{
			name:     "trim document whitespace",
			input:    "\n\n  Content here  \n\n",
			expected: "Content here",
		},
		{
			name:     "preserve headings and formatting",
			input:    "# Title\n\nSome **bold** and *italic* text.\n\n## Section\n\nMore content.",
			expected: "# Title\n\nSome **bold** and *italic* text.\n\n## Section\n\nMore content.",
		},
		{
			name:     "preserve code blocks",
			input:    "Example:\n\n```python\ndef foo():\n    pass\n```\n\nDone.",
			expected: "Example:\n\n```python\ndef foo():\n    pass\n```\n\nDone.",
		},
		{
			name:     "complex document",
			input:    "# Guide\n\nSee the ![icon](img/icon.png) and read [the docs](http://example.com).\n\n\n\nNext section with   extra spaces.",
			expected: "# Guide\n\nSee the icon and read the docs.\n\nNext section with extra spaces.",
		},
		{
			name:     "simplify ASCII table borders",
			input:    "+------------------------------+---------------------------------------------------+\n| Column 1                     | Description                                       |\n+------------------------------+---------------------------------------------------+",
			expected: "+-+-+\n| Column 1 | Description |\n+-+-+",
		},
		{
			name:     "simplify RST separator lines",
			input:    "Header\n======\n\nContent\n------",
			expected: "Header\n---\n\nContent\n---",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CleanMarkdownForRAG(tt.input)
			if result != tt.expected {
				t.Errorf("CleanMarkdownForRAG():\ngot:      %q\nexpected: %q", result, tt.expected)
			}
		})
	}
}

func TestConvertAppliesCleanup(t *testing.T) {
	// Test that Convert applies the RAG cleanup
	markdown := `# Test

Check [this link](http://example.com) and ![image](img.png).
`

	result, _, err := Convert([]byte(markdown), kbtypes.TypeMarkdown)
	if err != nil {
		t.Fatalf("Convert failed: %v", err)
	}

	// Links should be stripped
	if strings.Contains(result, "http://example.com") {
		t.Error("Convert should strip URLs from links")
	}
	if !strings.Contains(result, "this link") {
		t.Error("Convert should preserve link text")
	}

	// Images should be stripped
	if strings.Contains(result, "img.png") {
		t.Error("Convert should strip image paths")
	}
	if !strings.Contains(result, "image") {
		t.Error("Convert should preserve image alt text")
	}
}
