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
