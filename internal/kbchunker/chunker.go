/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
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

// Section represents a section of a document
type Section struct {
	Heading string
	Content string
	Level   int
}

// parseMarkdownSections parses markdown into sections
func parseMarkdownSections(markdown string) []Section {
	lines := strings.Split(markdown, "\n")
	var sections []Section
	var currentSection *Section

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check if line is a heading
		if strings.HasPrefix(trimmed, "#") {
			// Save previous section
			if currentSection != nil && currentSection.Content != "" {
				sections = append(sections, *currentSection)
			}

			// Start new section
			level := 0
			for _, r := range trimmed {
				if r == '#' {
					level++
				} else {
					break
				}
			}

			heading := strings.TrimSpace(trimmed[level:])
			currentSection = &Section{
				Heading: heading,
				Content: "",
				Level:   level,
			}
		} else if currentSection != nil {
			// Add content to current section
			currentSection.Content += line + "\n"
		} else {
			// Content before first heading - create a default section
			currentSection = &Section{
				Heading: "",
				Content: "",
				Level:   0,
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

// chunkSection breaks a section into chunks
func chunkSection(section Section, doc *kbtypes.Document) []*kbtypes.Chunk {
	content := strings.TrimSpace(section.Content)
	tokens := tokenize(content)

	// Skip sections with no content
	if len(tokens) == 0 && section.Heading == "" {
		return nil
	}

	// If section is small enough, return as single chunk
	if len(tokens) <= TargetChunkSize {
		text := content
		if section.Heading != "" {
			text = section.Heading + "\n\n" + content
		}

		// Skip if the final text is empty
		if strings.TrimSpace(text) == "" {
			return nil
		}

		return []*kbtypes.Chunk{
			{
				Text:           text,
				Title:          doc.Title,
				Section:        section.Heading,
				ProjectName:    doc.ProjectName,
				ProjectVersion: doc.ProjectVersion,
				FilePath:       doc.FilePath,
			},
		}
	}

	// Section is too large, split into multiple chunks with overlap
	var chunks []*kbtypes.Chunk
	start := 0

	for start < len(tokens) {
		end := start + TargetChunkSize
		if end > len(tokens) {
			end = len(tokens)
		}

		// Try to break at sentence boundary
		if end < len(tokens) {
			maxEnd := start + MaxChunkSize
			if maxEnd > len(tokens) {
				maxEnd = len(tokens)
			}
			end = findSentenceBoundary(tokens, end, maxEnd)
		}

		// Extract chunk tokens and convert back to text
		chunkTokens := tokens[start:end]
		chunkText := detokenize(chunkTokens)

		// Prepend section heading to chunk
		if section.Heading != "" {
			chunkText = section.Heading + "\n\n" + chunkText
		}

		chunks = append(chunks, &kbtypes.Chunk{
			Text:           chunkText,
			Title:          doc.Title,
			Section:        section.Heading,
			ProjectName:    doc.ProjectName,
			ProjectVersion: doc.ProjectVersion,
			FilePath:       doc.FilePath,
		})

		// If we've reached the end, we're done
		if end >= len(tokens) {
			break
		}

		// Move start position with overlap, ensuring we always make progress
		nextStart := end - OverlapSize
		if nextStart <= start {
			// Ensure we always advance by at least 1 token to avoid infinite loops
			nextStart = start + 1
		}
		start = nextStart
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
