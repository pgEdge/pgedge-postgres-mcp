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
	"unicode"

	"pgedge-postgres-mcp/internal/kbtypes"
)

const (
	// TargetChunkSize is the target number of words per chunk.
	// Note: This is word count, not LLM tokens. Technical content with long
	// terms (like MULE_INTERNAL, EUC_JIS_2004) can tokenize to 3-4x more
	// LLM tokens. For nomic-embed-text (8192 token limit), 250 words of
	// dense technical content is a safe maximum.
	TargetChunkSize = 250
	// MaxChunkSize is the maximum number of words per chunk
	MaxChunkSize = 300
	// OverlapSize is the number of words to overlap between chunks
	OverlapSize = 50
	// MaxChunkChars is the maximum characters per chunk.
	// This is a safety limit for content with high char-to-word ratios
	// (like XML/SGML with verbose technical terms). For 8192 token limit,
	// assuming ~2 chars per token for technical content, 3000 chars is safe.
	MaxChunkChars = 3000
)

// ChunkDocument breaks a document into chunks with overlap
func ChunkDocument(doc *kbtypes.Document) ([]*kbtypes.Chunk, error) {
	// Parse the markdown into sections
	sections := parseMarkdownSections(doc.Content)

	var chunks []*kbtypes.Chunk

	for _, section := range sections {
		sectionChunks := chunkSection(section, doc)
		chunks = append(chunks, sectionChunks...)
	}

	return chunks, nil
}

// Section represents a section of a document with its heading hierarchy
type Section struct {
	Heading     string
	HeadingPath []string // Full heading hierarchy (e.g., ["API", "Auth", "OAuth"])
	Content     string
	Level       int
}

// parseMarkdownSections parses markdown into sections with heading hierarchy tracking.
// It maintains a stack of headings to build the full heading path for each section.
func parseMarkdownSections(markdown string) []Section {
	lines := strings.Split(markdown, "\n")
	var sections []Section
	var currentSection *Section

	// Heading stack for building hierarchy (levels 1-6, index 0 unused)
	headingStack := make([]string, 7)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check if line is a heading
		if strings.HasPrefix(trimmed, "#") {
			// Save previous section
			if currentSection != nil && currentSection.Content != "" {
				sections = append(sections, *currentSection)
			}

			// Count heading level
			level := 0
			for _, r := range trimmed {
				if r == '#' {
					level++
				} else {
					break
				}
			}
			if level > 6 {
				level = 6 // Cap at h6
			}

			heading := strings.TrimSpace(trimmed[level:])

			// Update heading stack: set current level and clear deeper levels
			headingStack[level] = heading
			for i := level + 1; i < len(headingStack); i++ {
				headingStack[i] = ""
			}

			// Build heading path from stack
			var headingPath []string
			for i := 1; i <= level; i++ {
				if headingStack[i] != "" {
					headingPath = append(headingPath, headingStack[i])
				}
			}

			currentSection = &Section{
				Heading:     heading,
				HeadingPath: headingPath,
				Content:     "",
				Level:       level,
			}
		} else if currentSection != nil {
			// Add content to current section
			currentSection.Content += line + "\n"
		} else {
			// Content before first heading - create a default section
			currentSection = &Section{
				Heading:     "",
				HeadingPath: nil,
				Content:     "",
				Level:       0,
			}
			currentSection.Content += line + "\n"
		}
	}

	// Add final section
	if currentSection != nil && currentSection.Content != "" {
		sections = append(sections, *currentSection)
	}

	return sections
}

// chunkSection breaks a section into chunks using a hybrid two-pass algorithm.
// Pass 1: Split at semantic boundaries (code blocks, tables, lists stay intact)
// Pass 2: Merge undersized chunks to improve embedding quality
func chunkSection(section Section, doc *kbtypes.Document) []*kbtypes.Chunk {
	content := strings.TrimSpace(section.Content)

	// Skip sections with no content
	if content == "" && section.Heading == "" {
		return nil
	}

	cfg := DefaultChunkConfig()

	// Parse structural elements (code blocks, tables, lists, paragraphs)
	elements := parseStructuralElements(content)

	// If no elements parsed, fall back to treating content as single paragraph
	if len(elements) == 0 && content != "" {
		elements = []StructuralElement{{Type: Paragraph, Content: content}}
	}

	// Pass 1: Split at semantic boundaries
	rawChunks := splitAtSemanticBoundaries(elements, cfg)

	// Pass 2: Merge undersized chunks
	mergedChunks := mergeUndersizedChunks(rawChunks, cfg.MinSize, cfg.MaxSize, cfg.MaxChars)

	// Convert to final chunks with metadata
	var chunks []*kbtypes.Chunk
	for _, raw := range mergedChunks {
		text := raw.Text

		// Add heading context
		if section.Heading != "" {
			// Use full heading path for better context if available
			if len(section.HeadingPath) > 1 {
				headingContext := strings.Join(section.HeadingPath, " > ")
				text = headingContext + "\n\n" + text
			} else {
				text = section.Heading + "\n\n" + text
			}
		}

		// Skip if the final text is empty
		if strings.TrimSpace(text) == "" {
			continue
		}

		chunks = append(chunks, &kbtypes.Chunk{
			Text:           text,
			Title:          doc.Title,
			Section:        section.Heading,
			HeadingPath:    section.HeadingPath,
			ElementTypes:   raw.ElementTypes,
			ProjectName:    doc.ProjectName,
			ProjectVersion: doc.ProjectVersion,
			FilePath:       doc.FilePath,
		})
	}

	return chunks
}

// tokenize splits text into tokens (simple whitespace tokenization)
func tokenize(text string) []string {
	// Split on whitespace
	fields := strings.Fields(text)
	return fields
}

// detokenize joins tokens back into text
func detokenize(tokens []string) string {
	return strings.Join(tokens, " ")
}

// findSentenceBoundary finds the nearest sentence boundary before maxEnd
func findSentenceBoundary(tokens []string, preferredEnd, maxEnd int) int {
	// Look backwards from preferredEnd for sentence-ending punctuation
	for i := preferredEnd - 1; i >= preferredEnd-50 && i >= 0; i-- {
		token := tokens[i]
		if token != "" {
			lastChar := rune(token[len(token)-1])
			if isSentenceEnd(lastChar) {
				return i + 1
			}
		}
	}

	// No sentence boundary found, check if we can extend to maxEnd
	if maxEnd > preferredEnd {
		for i := preferredEnd; i < maxEnd && i < len(tokens); i++ {
			token := tokens[i]
			if token != "" {
				lastChar := rune(token[len(token)-1])
				if isSentenceEnd(lastChar) {
					return i + 1
				}
			}
		}
	}

	// No good boundary found, return preferred end
	return preferredEnd
}

// isSentenceEnd checks if a character typically ends a sentence
func isSentenceEnd(r rune) bool {
	return r == '.' || r == '!' || r == '?' || r == '\n'
}

// EstimateTokenCount estimates the number of tokens in a string
// This is a rough approximation: actual token count depends on the tokenizer
func EstimateTokenCount(text string) int {
	// Count words and punctuation as rough token estimate
	count := 0
	inWord := false

	for _, r := range text {
		if unicode.IsSpace(r) {
			if inWord {
				count++
				inWord = false
			}
		} else if unicode.IsPunct(r) {
			if inWord {
				count++ // End of word
				inWord = false
			}
			count++ // Punctuation is often a token
		} else {
			inWord = true
		}
	}

	if inWord {
		count++
	}

	return count
}
