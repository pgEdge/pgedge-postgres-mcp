//-------------------------------------------------------------------------
//
// pgEdge PostgreSQL MCP - Knowledgebase Builder
//
// Portions copyright (c) 2025, pgEdge, Inc.
// This software is released under The PostgreSQL License
//
//-------------------------------------------------------------------------

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
		Heading: "Test Section",
		Content: "This is a small section that fits in one chunk.",
		Level:   1,
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

	if !strings.Contains(chunks[0].Text, section.Content) {
		t.Error("Chunk should contain section content")
	}
}

func TestChunkSection_LargeSection(t *testing.T) {
	// Create a large section that requires multiple chunks
	largeContent := strings.Repeat("This is a sentence. ", 1000)

	section := Section{
		Heading: "Large Section",
		Content: largeContent,
		Level:   1,
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

	// Verify overlap between consecutive chunks
	if len(chunks) >= 2 {
		// Check if there's any overlapping content
		chunk1Words := strings.Fields(chunks[0].Text)
		chunk2Words := strings.Fields(chunks[1].Text)

		hasOverlap := false
		for _, word := range chunk1Words[len(chunk1Words)-50:] {
			for _, word2 := range chunk2Words[:50] {
				if word == word2 {
					hasOverlap = true
					break
				}
			}
			if hasOverlap {
				break
			}
		}

		if !hasOverlap {
			t.Error("Consecutive chunks should have overlapping content")
		}
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
			expectedCount: 6, // Hello, , world, !, How, are, you, ?
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
	// Create a document with known content
	content := strings.Repeat("Word ", TargetChunkSize*2)

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
