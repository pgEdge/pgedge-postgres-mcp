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
	"regexp"
	"strings"
)

// ElementType represents the type of a structural element in markdown
type ElementType int

const (
	// Paragraph is regular text content
	Paragraph ElementType = iota
	// CodeBlock is a fenced or indented code block
	CodeBlock
	// Table is a markdown table with | delimiters
	Table
	// List is an ordered or unordered list
	List
	// Blockquote is a > prefixed quote block
	Blockquote
)

// String returns the string representation of an ElementType
func (et ElementType) String() string {
	switch et {
	case Paragraph:
		return "paragraph"
	case CodeBlock:
		return "code_block"
	case Table:
		return "table"
	case List:
		return "list"
	case Blockquote:
		return "blockquote"
	default:
		return "unknown"
	}
}

// StructuralElement represents a markdown structural unit that should not
// be split during chunking
type StructuralElement struct {
	Type    ElementType
	Content string
}

// Regular expressions for element detection
var (
	codeBlockFenceRegex = regexp.MustCompile("^\\s*```")
	tableRowRegex       = regexp.MustCompile(`^\s*\|.*\|`)
	tableSeparatorRegex = regexp.MustCompile(`^\s*\|[-:| ]+\|`)
	listItemRegex       = regexp.MustCompile(`^(\s*)([-*+]|\d+\.)\s+`)
	blockquoteRegex     = regexp.MustCompile(`^\s*>\s*`)
)

// parseStructuralElements identifies structural units in markdown content.
// It parses the content and returns a slice of StructuralElements that
// represent code blocks, tables, lists, blockquotes, and paragraphs.
func parseStructuralElements(content string) []StructuralElement {
	lines := strings.Split(content, "\n")
	var elements []StructuralElement

	i := 0
	for i < len(lines) {
		line := lines[i]

		// Skip empty lines between elements
		if strings.TrimSpace(line) == "" {
			i++
			continue
		}

		// Code block (fenced with ```)
		if codeBlockFenceRegex.MatchString(line) {
			element, endIdx := parseCodeBlock(lines, i)
			if element.Content != "" {
				elements = append(elements, element)
			}
			i = endIdx + 1
			continue
		}

		// Table (starts with | and contains |)
		if tableRowRegex.MatchString(line) {
			element, endIdx := parseTable(lines, i)
			if element.Content != "" {
				elements = append(elements, element)
			}
			i = endIdx + 1
			continue
		}

		// List (starts with -, *, +, or number.)
		if listItemRegex.MatchString(line) {
			element, endIdx := parseList(lines, i)
			if element.Content != "" {
				elements = append(elements, element)
			}
			i = endIdx + 1
			continue
		}

		// Blockquote (starts with >)
		if blockquoteRegex.MatchString(line) {
			element, endIdx := parseBlockquote(lines, i)
			if element.Content != "" {
				elements = append(elements, element)
			}
			i = endIdx + 1
			continue
		}

		// Paragraph (default - regular text)
		element, endIdx := parseParagraph(lines, i)
		if element.Content != "" {
			elements = append(elements, element)
		}
		i = endIdx + 1
	}

	return elements
}

// parseCodeBlock extracts a fenced code block starting at index i.
// Returns the element and the index of the last line of the block.
func parseCodeBlock(lines []string, i int) (StructuralElement, int) {
	var content strings.Builder
	content.WriteString(lines[i])
	content.WriteString("\n")

	// Find the closing fence
	endIdx := i
	for j := i + 1; j < len(lines); j++ {
		content.WriteString(lines[j])
		content.WriteString("\n")
		endIdx = j

		// Check for closing fence (``` with optional language specifier)
		if codeBlockFenceRegex.MatchString(lines[j]) &&
			strings.TrimSpace(lines[j]) == "```" {
			break
		}
	}

	return StructuralElement{
		Type:    CodeBlock,
		Content: strings.TrimRight(content.String(), "\n"),
	}, endIdx
}

// parseTable extracts a markdown table starting at index i.
// Returns the element and the index of the last line of the table.
func parseTable(lines []string, i int) (StructuralElement, int) {
	var content strings.Builder
	content.WriteString(lines[i])
	content.WriteString("\n")

	endIdx := i
	for j := i + 1; j < len(lines); j++ {
		line := lines[j]

		// Continue if it's a table row or separator
		if tableRowRegex.MatchString(line) || tableSeparatorRegex.MatchString(line) {
			content.WriteString(line)
			content.WriteString("\n")
			endIdx = j
		} else {
			// Empty line or non-table content ends the table
			break
		}
	}

	return StructuralElement{
		Type:    Table,
		Content: strings.TrimRight(content.String(), "\n"),
	}, endIdx
}

// parseList extracts a list (ordered or unordered) starting at index i.
// Handles nested lists by tracking indentation.
// Returns the element and the index of the last line of the list.
func parseList(lines []string, i int) (StructuralElement, int) {
	var content strings.Builder
	content.WriteString(lines[i])
	content.WriteString("\n")

	// Get the base indentation of the first list item
	baseIndent := getIndentation(lines[i])

	endIdx := i
	for j := i + 1; j < len(lines); j++ {
		line := lines[j]
		trimmed := strings.TrimSpace(line)

		// Empty line might end the list
		if trimmed == "" {
			// Look ahead to see if list continues
			if j+1 < len(lines) {
				nextLine := lines[j+1]
				nextIndent := getIndentation(nextLine)
				// If next non-empty line is a list item at same or greater indent
				if listItemRegex.MatchString(nextLine) && nextIndent >= baseIndent {
					content.WriteString(line)
					content.WriteString("\n")
					endIdx = j
					continue
				}
			}
			break
		}

		currentIndent := getIndentation(line)

		// Check if it's a list item (at any nesting level)
		if listItemRegex.MatchString(line) && currentIndent >= baseIndent {
			content.WriteString(line)
			content.WriteString("\n")
			endIdx = j
			continue
		}

		// Check if it's continuation content (indented more than base)
		if currentIndent > baseIndent {
			content.WriteString(line)
			content.WriteString("\n")
			endIdx = j
			continue
		}

		// Otherwise, the list has ended
		break
	}

	return StructuralElement{
		Type:    List,
		Content: strings.TrimRight(content.String(), "\n"),
	}, endIdx
}

// parseBlockquote extracts a blockquote starting at index i.
// Returns the element and the index of the last line of the blockquote.
func parseBlockquote(lines []string, i int) (StructuralElement, int) {
	var content strings.Builder
	content.WriteString(lines[i])
	content.WriteString("\n")

	endIdx := i
	for j := i + 1; j < len(lines); j++ {
		line := lines[j]

		// Continue if it's a blockquote line
		if blockquoteRegex.MatchString(line) {
			content.WriteString(line)
			content.WriteString("\n")
			endIdx = j
		} else if strings.TrimSpace(line) == "" {
			// Empty line might continue or end blockquote
			// Look ahead to see if blockquote continues
			if j+1 < len(lines) && blockquoteRegex.MatchString(lines[j+1]) {
				content.WriteString(line)
				content.WriteString("\n")
				endIdx = j
				continue
			}
			break
		} else {
			// Non-blockquote content, stop
			break
		}
	}

	return StructuralElement{
		Type:    Blockquote,
		Content: strings.TrimRight(content.String(), "\n"),
	}, endIdx
}

// parseParagraph extracts a paragraph starting at index i.
// A paragraph continues until an empty line or a structural element is found.
// Returns the element and the index of the last line of the paragraph.
func parseParagraph(lines []string, i int) (StructuralElement, int) {
	var content strings.Builder
	content.WriteString(lines[i])
	content.WriteString("\n")

	endIdx := i
	for j := i + 1; j < len(lines); j++ {
		line := lines[j]

		// Empty line ends the paragraph
		if strings.TrimSpace(line) == "" {
			break
		}

		// Check if this line starts a new structural element
		if codeBlockFenceRegex.MatchString(line) ||
			tableRowRegex.MatchString(line) ||
			listItemRegex.MatchString(line) ||
			blockquoteRegex.MatchString(line) {
			break
		}

		content.WriteString(line)
		content.WriteString("\n")
		endIdx = j
	}

	return StructuralElement{
		Type:    Paragraph,
		Content: strings.TrimRight(content.String(), "\n"),
	}, endIdx
}

// getIndentation returns the number of leading spaces/tabs in a line
func getIndentation(line string) int {
	count := 0
	for _, r := range line {
		if r == ' ' {
			count++
		} else if r == '\t' {
			count += 4 // Treat tab as 4 spaces
		} else {
			break
		}
	}
	return count
}
