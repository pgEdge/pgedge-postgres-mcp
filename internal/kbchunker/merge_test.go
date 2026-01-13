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

func TestDefaultChunkConfig(t *testing.T) {
	cfg := DefaultChunkConfig()

	if cfg.TargetSize != TargetChunkSize {
		t.Errorf("TargetSize = %d, want %d", cfg.TargetSize, TargetChunkSize)
	}
	if cfg.MaxSize != MaxChunkSize {
		t.Errorf("MaxSize = %d, want %d", cfg.MaxSize, MaxChunkSize)
	}
	if cfg.MinSize != 100 {
		t.Errorf("MinSize = %d, want 100", cfg.MinSize)
	}
	if cfg.MaxChars != MaxChunkChars {
		t.Errorf("MaxChars = %d, want %d", cfg.MaxChars, MaxChunkChars)
	}
	if !cfg.PreserveCode {
		t.Error("PreserveCode should be true by default")
	}
	if !cfg.PreserveTables {
		t.Error("PreserveTables should be true by default")
	}
}

func TestMergeUndersizedChunks_MergesSmallChunks(t *testing.T) {
	chunks := []RawChunk{
		{Text: "First small chunk.", ElementTypes: []string{"paragraph"}},
		{Text: "Second small chunk.", ElementTypes: []string{"paragraph"}},
	}

	merged := mergeUndersizedChunks(chunks, 100, 300, 3000)

	if len(merged) != 1 {
		t.Errorf("expected 1 merged chunk, got %d", len(merged))
		return
	}

	if !strings.Contains(merged[0].Text, "First small chunk") {
		t.Error("merged chunk should contain first chunk")
	}
	if !strings.Contains(merged[0].Text, "Second small chunk") {
		t.Error("merged chunk should contain second chunk")
	}
	if len(merged[0].ElementTypes) != 2 {
		t.Errorf("expected 2 element types, got %d", len(merged[0].ElementTypes))
	}
}

func TestMergeUndersizedChunks_RespectsMaxSize(t *testing.T) {
	// Create chunks that together exceed max size
	chunk1Text := strings.Repeat("word ", 200) // 200 words
	chunk2Text := strings.Repeat("word ", 150) // 150 words

	chunks := []RawChunk{
		{Text: chunk1Text, ElementTypes: []string{"paragraph"}},
		{Text: chunk2Text, ElementTypes: []string{"paragraph"}},
	}

	merged := mergeUndersizedChunks(chunks, 100, 300, 10000)

	// Should not merge because combined size (350) exceeds maxSize (300)
	if len(merged) != 2 {
		t.Errorf("expected 2 chunks (not merged due to size), got %d", len(merged))
	}
}

func TestMergeUndersizedChunks_RespectsMaxChars(t *testing.T) {
	// Create chunks that together exceed max chars
	chunk1Text := strings.Repeat("a", 2000)
	chunk2Text := strings.Repeat("b", 2000)

	chunks := []RawChunk{
		{Text: chunk1Text, ElementTypes: []string{"paragraph"}},
		{Text: chunk2Text, ElementTypes: []string{"paragraph"}},
	}

	merged := mergeUndersizedChunks(chunks, 1, 1000, 3000)

	// Should not merge because combined chars (4000+) exceeds maxChars (3000)
	if len(merged) != 2 {
		t.Errorf("expected 2 chunks (not merged due to chars), got %d", len(merged))
	}
}

func TestMergeUndersizedChunks_LeavesLargeChunksAlone(t *testing.T) {
	// Both chunks are above minSize, so no merging should happen
	largeText1 := strings.Repeat("word ", 150) // 150 words, above minSize
	largeText2 := strings.Repeat("word ", 120) // 120 words, above minSize

	chunks := []RawChunk{
		{Text: largeText1, ElementTypes: []string{"paragraph"}},
		{Text: largeText2, ElementTypes: []string{"paragraph"}},
	}

	merged := mergeUndersizedChunks(chunks, 100, 300, 10000)

	// Both chunks are above minSize, shouldn't merge
	if len(merged) != 2 {
		t.Errorf("expected 2 chunks (both above minSize), got %d", len(merged))
	}
}

func TestMergeUndersizedChunks_SingleChunk(t *testing.T) {
	chunks := []RawChunk{
		{Text: "Only one chunk", ElementTypes: []string{"paragraph"}},
	}

	merged := mergeUndersizedChunks(chunks, 100, 300, 3000)

	if len(merged) != 1 {
		t.Errorf("expected 1 chunk, got %d", len(merged))
	}
}

func TestMergeUndersizedChunks_EmptyInput(t *testing.T) {
	merged := mergeUndersizedChunks(nil, 100, 300, 3000)

	if len(merged) != 0 {
		t.Errorf("expected 0 chunks, got %d", len(merged))
	}
}

func TestMergeTrailingUndersized(t *testing.T) {
	chunks := []RawChunk{
		{Text: strings.Repeat("word ", 150), ElementTypes: []string{"paragraph"}},
		{Text: "tiny", ElementTypes: []string{"paragraph"}}, // undersized
	}

	merged := mergeTrailingUndersized(chunks, 100, 300, 10000)

	if len(merged) != 1 {
		t.Errorf("expected 1 merged chunk, got %d", len(merged))
	}
}

func TestSplitAtSemanticBoundaries_KeepsSmallElementsTogether(t *testing.T) {
	elements := []StructuralElement{
		{Type: Paragraph, Content: "First paragraph."},
		{Type: Paragraph, Content: "Second paragraph."},
	}

	cfg := DefaultChunkConfig()
	chunks := splitAtSemanticBoundaries(elements, cfg)

	// Both small paragraphs should be in one chunk
	if len(chunks) != 1 {
		t.Errorf("expected 1 chunk, got %d", len(chunks))
		return
	}

	if !strings.Contains(chunks[0].Text, "First paragraph") {
		t.Error("chunk should contain first paragraph")
	}
	if !strings.Contains(chunks[0].Text, "Second paragraph") {
		t.Error("chunk should contain second paragraph")
	}
}

func TestSplitAtSemanticBoundaries_SplitsAtTarget(t *testing.T) {
	// Create elements that together exceed target size
	elem1 := StructuralElement{
		Type:    Paragraph,
		Content: strings.Repeat("word ", 200),
	}
	elem2 := StructuralElement{
		Type:    Paragraph,
		Content: strings.Repeat("word ", 200),
	}

	elements := []StructuralElement{elem1, elem2}

	cfg := DefaultChunkConfig()
	chunks := splitAtSemanticBoundaries(elements, cfg)

	// Should split because combined exceeds target
	if len(chunks) != 2 {
		t.Errorf("expected 2 chunks, got %d", len(chunks))
	}
}

func TestSplitOversizedElement_Paragraph(t *testing.T) {
	largeContent := strings.Repeat("This is a sentence. ", 100)

	cfg := DefaultChunkConfig()
	chunks := splitParagraphAtSentences(largeContent, cfg)

	if len(chunks) < 2 {
		t.Errorf("expected multiple chunks, got %d", len(chunks))
	}

	// Verify all chunks are within limits
	for i, chunk := range chunks {
		if wordCount(chunk.Text) > cfg.MaxSize {
			t.Errorf("chunk %d exceeds max size: %d words", i, wordCount(chunk.Text))
		}
		if len(chunk.Text) > cfg.MaxChars {
			t.Errorf("chunk %d exceeds max chars: %d chars", i, len(chunk.Text))
		}
	}
}

func TestSplitCodeBlockAtLines(t *testing.T) {
	// Create a large code block that exceeds MaxChunkChars (3000)
	// Each line is about 50 chars, so 100 lines = 5000 chars
	var lines []string
	lines = append(lines, "```go")
	for i := 0; i < 100; i++ {
		lines = append(lines, "fmt.Println(\"This is a longer line of code that takes up more space\")")
	}
	lines = append(lines, "```")
	content := strings.Join(lines, "\n")

	cfg := DefaultChunkConfig()
	chunks := splitCodeBlockAtLines(content, cfg)

	// Verify we get at least one chunk
	if len(chunks) == 0 {
		t.Fatal("expected at least one chunk")
	}

	// Each chunk should have code block fences
	for i, chunk := range chunks {
		if !strings.Contains(chunk.Text, "```") {
			t.Errorf("chunk %d should contain code fences", i)
		}
	}

	// Verify content is preserved across all chunks
	allContent := ""
	for _, chunk := range chunks {
		allContent += chunk.Text
	}
	if !strings.Contains(allContent, "fmt.Println") {
		t.Error("code content should be preserved")
	}
}

func TestSplitTableAtRows(t *testing.T) {
	// Create a large table
	var lines []string
	lines = append(lines, "| Header 1 | Header 2 |")
	lines = append(lines, "|----------|----------|")
	for i := 0; i < 100; i++ {
		lines = append(lines, "| Value A  | Value B  |")
	}
	content := strings.Join(lines, "\n")

	cfg := DefaultChunkConfig()
	chunks := splitTableAtRows(content, cfg)

	if len(chunks) < 2 {
		t.Errorf("expected multiple chunks, got %d", len(chunks))
	}

	// Each chunk should include the header
	for i, chunk := range chunks {
		if !strings.Contains(chunk.Text, "Header 1") {
			t.Errorf("chunk %d should contain table header", i)
		}
	}
}

func TestSplitIntoSentences(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected int
	}{
		{
			name:     "simple sentences",
			text:     "First sentence. Second sentence. Third sentence.",
			expected: 3,
		},
		{
			name:     "with exclamation",
			text:     "Hello! How are you? I'm fine.",
			expected: 3,
		},
		{
			name:     "single sentence",
			text:     "Just one sentence.",
			expected: 1,
		},
		{
			name:     "no ending punctuation",
			text:     "No ending punctuation",
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sentences := splitIntoSentences(tt.text)
			if len(sentences) != tt.expected {
				t.Errorf("expected %d sentences, got %d: %v", tt.expected, len(sentences), sentences)
			}
		})
	}
}

func TestWordCount(t *testing.T) {
	tests := []struct {
		text     string
		expected int
	}{
		{"", 0},
		{"one", 1},
		{"one two three", 3},
		{"  spaced  out  ", 2},
		{"newlines\nwork\ntoo", 3},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			result := wordCount(tt.text)
			if result != tt.expected {
				t.Errorf("wordCount(%q) = %d, want %d", tt.text, result, tt.expected)
			}
		})
	}
}

func TestSplitListAtItems(t *testing.T) {
	// Create a large list
	var lines []string
	for i := 0; i < 100; i++ {
		lines = append(lines, "- This is a list item with some content that takes up space")
	}
	content := strings.Join(lines, "\n")

	cfg := DefaultChunkConfig()
	chunks := splitListAtItems(content, cfg)

	if len(chunks) < 2 {
		t.Errorf("expected multiple chunks, got %d", len(chunks))
	}

	// All chunks should be lists
	for i, chunk := range chunks {
		if len(chunk.ElementTypes) == 0 || chunk.ElementTypes[0] != "list" {
			t.Errorf("chunk %d should be a list", i)
		}
	}
}
