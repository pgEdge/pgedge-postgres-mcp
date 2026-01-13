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

	"pgedge-postgres-mcp/internal/kbtypes"
)

func TestChunkDocument(t *testing.T) {
	doc := &kbtypes.Document{
		Title:          "Test Document",
		Content:        "# Section 1\n\nThis is content for section 1.\n\n# Section 2\n\nThis is content for section 2.",
		ProjectName:    "Test",
		ProjectVersion: "1.0",
		FilePath:       "test.md",
	}

	chunks, err := ChunkDocument(doc)
	if err != nil {
		t.Fatalf("ChunkDocument failed: %v", err)
	}

	if len(chunks) == 0 {
		t.Fatal("Expected chunks but got none")
	}

	// Verify chunk structure
	for i, chunk := range chunks {
		if chunk.ProjectName != "Test" {
			t.Errorf("Chunk %d: wrong project name", i)
		}
		if chunk.ProjectVersion != "1.0" {
			t.Errorf("Chunk %d: wrong project version", i)
		}
		if chunk.Text == "" {
			t.Errorf("Chunk %d: empty text", i)
		}
	}
}

func TestParseMarkdownSections(t *testing.T) {
	tests := []struct {
		name           string
		markdown       string
		expectedCount  int
		expectedLevels []int
	}{
		{
			name:           "simple sections",
			markdown:       "# Level 1\n\nContent\n\n## Level 2\n\nMore content",
			expectedCount:  2,
			expectedLevels: []int{1, 2},
		},
		{
			name:           "no headings",
			markdown:       "Just some content without headings",
			expectedCount:  1,
			expectedLevels: []int{0},
		},
		{
			name:           "multiple same level",
			markdown:       "# First\n\nContent 1\n\n# Second\n\nContent 2\n\n# Third\n\nContent 3",
			expectedCount:  3,
			expectedLevels: []int{1, 1, 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sections := parseMarkdownSections(tt.markdown)

			if len(sections) != tt.expectedCount {
				t.Errorf("Expected %d sections, got %d", tt.expectedCount, len(sections))
			}

			for i, section := range sections {
				if i < len(tt.expectedLevels) && section.Level != tt.expectedLevels[i] {
					t.Errorf("Section %d: expected level %d, got %d", i, tt.expectedLevels[i], section.Level)
				}
			}
		})
	}
}

func TestChunkSection_SmallSection(t *testing.T) {
	section := Section{
		Heading:     "Test Section",
		HeadingPath: []string{"Test Section"},
		Content:     "This is a small section that fits in one chunk.",
		Level:       1,
	}

	doc := &kbtypes.Document{
		Title:          "Test",
		ProjectName:    "Test",
		ProjectVersion: "1.0",
	}

	chunks := chunkSection(section, doc)

	if len(chunks) != 1 {
		t.Errorf("Small section should produce 1 chunk, got %d", len(chunks))
	}

	if !strings.Contains(chunks[0].Text, section.Heading) {
		t.Error("Chunk should contain section heading")
	}

	// Content is trimmed before chunking, so check for trimmed version
	if !strings.Contains(chunks[0].Text, strings.TrimSpace(section.Content)) {
		t.Error("Chunk should contain section content")
	}
}

func TestChunkSection_LargeSection(t *testing.T) {
	// Create a large section that requires multiple chunks
	largeContent := strings.Repeat("This is a sentence. ", 1000)

	section := Section{
		Heading:     "Large Section",
		HeadingPath: []string{"Large Section"},
		Content:     largeContent,
		Level:       1,
	}

	doc := &kbtypes.Document{
		Title:          "Test",
		ProjectName:    "Test",
		ProjectVersion: "1.0",
	}

	chunks := chunkSection(section, doc)

	if len(chunks) <= 1 {
		t.Error("Large section should produce multiple chunks")
	}

	// Verify all chunks are within size limits
	for i, chunk := range chunks {
		words := len(strings.Fields(chunk.Text))
		if words > MaxChunkSize+50 { // Allow buffer for heading
			t.Errorf("Chunk %d exceeds max word size: %d words", i, words)
		}
		if len(chunk.Text) > MaxChunkChars+100 { // Allow buffer for heading
			t.Errorf("Chunk %d exceeds max char size: %d chars", i, len(chunk.Text))
		}
	}
}

func TestChunkSection_HighCharacterRatio(t *testing.T) {
	// Create content with high character-to-word ratio (simulating technical XML content)
	// Create sentences with long words to exceed MaxChunkChars
	longWord := strings.Repeat("abcdefghij", 30) // 300 char "word"
	// Create multiple sentences with long words to ensure we exceed char limit
	largeContent := strings.Repeat(longWord+" word. ", 20) // ~6400 chars

	section := Section{
		Heading:     "Technical Section",
		HeadingPath: []string{"Technical Section"},
		Content:     largeContent,
		Level:       1,
	}

	doc := &kbtypes.Document{
		Title:          "Test",
		ProjectName:    "Test",
		ProjectVersion: "1.0",
	}

	chunks := chunkSection(section, doc)

	// Should produce multiple chunks due to character limit
	if len(chunks) <= 1 {
		t.Logf("Got %d chunks with total content length %d", len(chunks), len(largeContent))
		t.Error("High char-ratio content should produce multiple chunks")
	}

	// Verify chunks are being created and content is preserved
	totalContent := ""
	for _, chunk := range chunks {
		// Extract content after heading
		content := chunk.Text
		if idx := strings.Index(content, "\n\n"); idx >= 0 {
			content = content[idx+2:]
		}
		totalContent += content
	}

	if !strings.Contains(totalContent, longWord) {
		t.Error("Long word content should be preserved in chunks")
	}
}

func TestTokenize(t *testing.T) {
	tests := []struct {
		name          string
		text          string
		expectedCount int
	}{
		{
			name:          "simple text",
			text:          "Hello world",
			expectedCount: 2,
		},
		{
			name:          "empty text",
			text:          "",
			expectedCount: 0,
		},
		{
			name:          "text with punctuation",
			text:          "Hello, world! How are you?",
			expectedCount: 5, // "Hello,", "world!", "How", "are", "you?" (Fields splits on whitespace)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := tokenize(tt.text)
			if len(tokens) != tt.expectedCount {
				t.Errorf("Expected %d tokens, got %d", tt.expectedCount, len(tokens))
			}
		})
	}
}

func TestDetokenize(t *testing.T) {
	tokens := []string{"Hello", "world", "this", "is", "a", "test"}
	result := detokenize(tokens)

	expected := "Hello world this is a test"
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

func TestFindSentenceBoundary(t *testing.T) {
	tokens := []string{
		"This", "is", "a", "sentence.", "This", "is", "another", "sentence.",
		"And", "this", "is", "a", "third", "one.",
	}

	// Should find boundary at position after "sentence."
	boundary := findSentenceBoundary(tokens, 6, 10)

	// Should be at or near a sentence end
	if boundary < 0 || boundary >= len(tokens) {
		t.Error("Boundary should be within token range")
	}
}

func TestIsSentenceEnd(t *testing.T) {
	tests := []struct {
		char     rune
		expected bool
	}{
		{'.', true},
		{'!', true},
		{'?', true},
		{'\n', true},
		{',', false},
		{'a', false},
		{' ', false},
	}

	for _, tt := range tests {
		t.Run(string(tt.char), func(t *testing.T) {
			result := isSentenceEnd(tt.char)
			if result != tt.expected {
				t.Errorf("isSentenceEnd('%c') = %v, expected %v", tt.char, result, tt.expected)
			}
		})
	}
}

func TestEstimateTokenCount(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		minCount int
		maxCount int
	}{
		{
			name:     "simple sentence",
			text:     "This is a simple sentence.",
			minCount: 4,
			maxCount: 8,
		},
		{
			name:     "empty string",
			text:     "",
			minCount: 0,
			maxCount: 0,
		},
		{
			name:     "with punctuation",
			text:     "Hello, world! How are you?",
			minCount: 5,
			maxCount: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count := EstimateTokenCount(tt.text)
			if count < tt.minCount || count > tt.maxCount {
				t.Errorf("EstimateTokenCount(%q) = %d, expected between %d and %d",
					tt.text, count, tt.minCount, tt.maxCount)
			}
		})
	}
}

func TestChunkWithOverlap(t *testing.T) {
	// Create a document with known content - use sentences to trigger proper splitting
	content := strings.Repeat("This is a test sentence. ", TargetChunkSize*3)

	doc := &kbtypes.Document{
		Title:          "Test",
		Content:        "# Test\n\n" + content,
		ProjectName:    "Test",
		ProjectVersion: "1.0",
	}

	chunks, err := ChunkDocument(doc)
	if err != nil {
		t.Fatalf("ChunkDocument failed: %v", err)
	}

	if len(chunks) < 2 {
		t.Logf("Got %d chunks for content with %d words", len(chunks), len(strings.Fields(content)))
		t.Error("Should create multiple chunks for large content")
	}

	// Check that chunks include section heading
	for i, chunk := range chunks {
		if !strings.Contains(chunk.Text, "Test") {
			t.Errorf("Chunk %d should contain section heading", i)
		}
		if chunk.Section != "Test" {
			t.Errorf("Chunk %d should have section set to 'Test'", i)
		}
	}
}

func TestParseMarkdownSections_HeadingHierarchy(t *testing.T) {
	markdown := `# API Reference

Introduction to the API.

## Authentication

How to authenticate.

### OAuth

OAuth flow details.

### API Keys

API key usage.

## Endpoints

Available endpoints.`

	sections := parseMarkdownSections(markdown)

	// Expected sections with their heading paths
	expectedPaths := [][]string{
		{"API Reference"},
		{"API Reference", "Authentication"},
		{"API Reference", "Authentication", "OAuth"},
		{"API Reference", "Authentication", "API Keys"},
		{"API Reference", "Endpoints"},
	}

	if len(sections) != len(expectedPaths) {
		t.Errorf("Expected %d sections, got %d", len(expectedPaths), len(sections))
		for i, s := range sections {
			t.Logf("Section %d: %q, path: %v", i, s.Heading, s.HeadingPath)
		}
		return
	}

	for i, section := range sections {
		if len(section.HeadingPath) != len(expectedPaths[i]) {
			t.Errorf("Section %d: expected path len %d, got %d",
				i, len(expectedPaths[i]), len(section.HeadingPath))
			continue
		}
		for j, heading := range section.HeadingPath {
			if heading != expectedPaths[i][j] {
				t.Errorf("Section %d path[%d]: expected %q, got %q",
					i, j, expectedPaths[i][j], heading)
			}
		}
	}
}

func TestChunkDocument_PreservesCodeBlocks(t *testing.T) {
	doc := &kbtypes.Document{
		Title:          "Code Example",
		Content:        "# Code\n\nHere is some code:\n\n```go\nfunc main() {\n    fmt.Println(\"Hello\")\n}\n```\n\nMore text here.",
		ProjectName:    "Test",
		ProjectVersion: "1.0",
		FilePath:       "test.md",
	}

	chunks, err := ChunkDocument(doc)
	if err != nil {
		t.Fatalf("ChunkDocument failed: %v", err)
	}

	// The code block should be preserved intact
	foundCodeBlock := false
	for _, chunk := range chunks {
		if strings.Contains(chunk.Text, "func main()") &&
			strings.Contains(chunk.Text, "fmt.Println") {
			foundCodeBlock = true
			// Verify the code block wasn't split
			if !strings.Contains(chunk.Text, "```go") {
				t.Error("Code block should include opening fence")
			}
		}
	}

	if !foundCodeBlock {
		t.Error("Code block content not found in any chunk")
	}
}

func TestChunkDocument_HeadingPathInChunk(t *testing.T) {
	doc := &kbtypes.Document{
		Title:          "Test Doc",
		Content:        "# Parent\n\n## Child\n\nSome content here.",
		ProjectName:    "Test",
		ProjectVersion: "1.0",
		FilePath:       "test.md",
	}

	chunks, err := ChunkDocument(doc)
	if err != nil {
		t.Fatalf("ChunkDocument failed: %v", err)
	}

	// Find the chunk with "Child" section
	for _, chunk := range chunks {
		if chunk.Section == "Child" {
			// Verify heading path is set
			if len(chunk.HeadingPath) != 2 {
				t.Errorf("Expected HeadingPath len 2, got %d", len(chunk.HeadingPath))
			}
			if len(chunk.HeadingPath) >= 2 {
				if chunk.HeadingPath[0] != "Parent" {
					t.Errorf("HeadingPath[0] should be 'Parent', got %q", chunk.HeadingPath[0])
				}
				if chunk.HeadingPath[1] != "Child" {
					t.Errorf("HeadingPath[1] should be 'Child', got %q", chunk.HeadingPath[1])
				}
			}
			// Verify the text includes the full heading path
			if !strings.Contains(chunk.Text, "Parent > Child") {
				t.Error("Chunk text should include full heading path")
			}
		}
	}
}

func TestChunkDocument_ElementTypesTracked(t *testing.T) {
	doc := &kbtypes.Document{
		Title:          "Mixed Content",
		Content:        "# Section\n\nParagraph text.\n\n```\ncode\n```\n\n| A | B |\n|---|---|\n| 1 | 2 |",
		ProjectName:    "Test",
		ProjectVersion: "1.0",
		FilePath:       "test.md",
	}

	chunks, err := ChunkDocument(doc)
	if err != nil {
		t.Fatalf("ChunkDocument failed: %v", err)
	}

	// Verify that element types are tracked
	foundElementTypes := false
	for _, chunk := range chunks {
		if len(chunk.ElementTypes) > 0 {
			foundElementTypes = true
		}
	}

	if !foundElementTypes {
		t.Error("Expected element types to be tracked in chunks")
	}
}
