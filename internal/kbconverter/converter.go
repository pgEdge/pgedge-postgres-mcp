/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

//-------------------------------------------------------------------------
//
// pgEdge Docloader
//
// Portions copyright (c) 2025, pgEdge, Inc.
// This software is released under The PostgreSQL License
//
//-------------------------------------------------------------------------

package kbconverter

import (
	"bufio"
	"errors"
	"fmt"
	"html"
	"io"
	"path/filepath"
	"regexp"
	"strings"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/PuerkitoBio/goquery"

	"pgedge-postgres-mcp/internal/kbtypes"
)

var (
	// ErrUnsupportedFormat is returned when a file format is not supported
	ErrUnsupportedFormat = errors.New("unsupported document format")
)

// DetectDocumentType detects the document type from file extension
func DetectDocumentType(filename string) kbtypes.DocumentType {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".html", ".htm":
		return kbtypes.TypeHTML
	case ".md":
		return kbtypes.TypeMarkdown
	case ".rst":
		return kbtypes.TypeReStructuredText
	case ".sgml", ".sgm", ".xml":
		return kbtypes.TypeSGML
	default:
		return kbtypes.TypeUnknown
	}
}

// Convert converts a document to markdown based on its type
func Convert(content []byte, docType kbtypes.DocumentType) (markdown string, title string, err error) {
	switch docType {
	case kbtypes.TypeHTML:
		return convertHTML(content)
	case kbtypes.TypeMarkdown:
		return processMarkdown(content)
	case kbtypes.TypeReStructuredText:
		return convertRST(content)
	case kbtypes.TypeSGML:
		return convertSGML(content)
	default:
		return "", "", ErrUnsupportedFormat
	}
}

// convertHTML converts HTML to Markdown and extracts the title
func convertHTML(content []byte) (string, string, error) {
	converter := md.NewConverter("", true, nil)

	// Add custom rule to shift heading levels down by one
	// (since we use the <title> as H1, all other headings should be shifted)
	converter.AddRules(md.Rule{
		Filter: []string{"h1", "h2", "h3", "h4", "h5", "h6"},
		Replacement: func(content string, selec *goquery.Selection, opt *md.Options) *string {
			// Shift each heading level down by one
			level := 2 // h1 becomes h2 (##)
			switch selec.Nodes[0].Data {
			case "h1":
				level = 2
			case "h2":
				level = 3
			case "h3":
				level = 4
			case "h4":
				level = 5
			case "h5":
				level = 6
			case "h6":
				level = 6 // h6 stays at max level (######)
			}

			result := strings.Repeat("#", level) + " " + content
			return &result
		},
	})

	markdown, err := converter.ConvertBytes(content)
	if err != nil {
		return "", "", fmt.Errorf("failed to convert HTML: %w", err)
	}

	// Extract title from HTML
	title := extractHTMLTitle(content)

	// Prepend title as H1 heading if we have one
	markdownStr := string(markdown)
	if title != "" {
		// The html-to-markdown library includes the title as plain text at the start
		// We need to replace it with a proper markdown heading
		markdownStr = strings.TrimSpace(markdownStr)

		// Check if the markdown starts with the title (without HTML entities decoded)
		// The library decodes entities in the output but we extract title from raw HTML
		if strings.HasPrefix(markdownStr, title) {
			// Remove the plain title and replace with heading
			markdownStr = strings.TrimPrefix(markdownStr, title)
			markdownStr = strings.TrimSpace(markdownStr)
		}

		// Add title as H1 heading
		markdownStr = "# " + title + "\n\n" + markdownStr
	}

	return markdownStr, title, nil
}

// extractHTMLTitle extracts the title from HTML <title> tag
func extractHTMLTitle(content []byte) string {
	titleRe := regexp.MustCompile(`(?i)<title[^>]*>([^<]+)</title>`)
	matches := titleRe.FindSubmatch(content)
	if len(matches) > 1 {
		// Decode HTML entities (e.g., &#8212; -> —)
		title := html.UnescapeString(string(matches[1]))
		return strings.TrimSpace(title)
	}
	return ""
}

// processMarkdown processes Markdown and extracts the title
func processMarkdown(content []byte) (string, string, error) {
	// Markdown is already in the target format
	markdown := string(content)

	// Extract title from first # heading
	title := extractMarkdownTitle(markdown)

	return markdown, title, nil
}

// extractMarkdownTitle extracts the title from the first # heading
func extractMarkdownTitle(content string) string {
	scanner := bufio.NewScanner(strings.NewReader(content))
	inMetadata := false
	metadataDelimiterCount := 0

	for scanner.Scan() {
		line := scanner.Text()

		// Skip YAML front matter
		if line == "---" {
			metadataDelimiterCount++
			if metadataDelimiterCount == 1 {
				inMetadata = true
				continue
			} else if metadataDelimiterCount == 2 {
				inMetadata = false
				continue
			}
		}

		if inMetadata {
			continue
		}

		// Look for first # heading
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
		}
	}

	return ""
}

// convertRST converts reStructuredText to Markdown
func convertRST(content []byte) (string, string, error) {
	// Basic RST to Markdown conversion
	// This is a simplified implementation
	text := string(content)
	title := extractRSTTitle(text)

	// Convert RST headings to Markdown
	markdown := convertRSTHeadings(text)

	// Convert RST images to Markdown
	markdown = convertRSTImages(markdown)

	return markdown, title, nil
}

// extractRSTTitle extracts the title from reStructuredText
func extractRSTTitle(content string) string {
	lines := strings.Split(content, "\n")

	for i := 0; i < len(lines)-1; i++ {
		current := strings.TrimSpace(lines[i])
		next := strings.TrimSpace(lines[i+1])

		// Skip RST directives, anchors, and labels (.. name: or .. _name:)
		if strings.HasPrefix(current, "..") && strings.HasSuffix(current, ":") {
			continue
		}

		// Check for overline+underline pattern (heading with line above and below)
		if i+2 < len(lines) && isUnderline(current) {
			text := strings.TrimSpace(lines[i+1])
			underline := strings.TrimSpace(lines[i+2])

			// Make sure the text line is not a directive either
			if text != "" && current == underline && isUnderline(underline) &&
				!(strings.HasPrefix(text, "..") && strings.HasSuffix(text, ":")) {
				// This is a heading with overline and underline - likely the title
				return cleanHeadingText(text)
			}
		}

		// Check for underline-only pattern (=, -, ~, etc.)
		if current != "" && next != "" {
			char := next[0]
			if (char == '=' || char == '-' || char == '~' || char == '#' || char == '*') &&
				strings.Count(next, string(char)) == len(next) &&
				len(next) >= len(current) {
				return cleanHeadingText(current)
			}
		}
	}

	return ""
}

// convertRSTHeadings converts RST-style headings to Markdown
func convertRSTHeadings(content string) string {
	lines := strings.Split(content, "\n")
	var result []string

	// Track heading patterns in order of appearance
	headingPatterns := make(map[string]int)
	nextLevel := 1

	i := 0
	for i < len(lines) {
		// Skip RST directives, anchors, and labels (.. name: or .. _name:)
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, "..") && strings.HasSuffix(trimmed, ":") {
			i++
			continue
		}

		current := lines[i]
		currentTrim := strings.TrimSpace(current)

		// Check for heading with overline and underline
		if i+2 < len(lines) && isUnderline(currentTrim) {
			text := strings.TrimSpace(lines[i+1])
			underline := strings.TrimSpace(lines[i+2])

			if text != "" && currentTrim == underline && isUnderline(underline) {
				// This is a heading with overline and underline
				pattern := string(currentTrim[0]) + "o" // 'o' for overline
				level := getOrAssignLevel(pattern, headingPatterns, &nextLevel)
				cleanText := cleanHeadingText(text)
				result = append(result, strings.Repeat("#", level)+" "+cleanText)
				i += 3
				continue
			}
		}

		// Check for heading with just underline
		if i+1 < len(lines) && currentTrim != "" {
			next := strings.TrimSpace(lines[i+1])
			if isUnderline(next) && len(next) >= len(currentTrim) {
				// This is a heading with just underline
				pattern := string(next[0]) + "u" // 'u' for underline only
				level := getOrAssignLevel(pattern, headingPatterns, &nextLevel)
				cleanText := cleanHeadingText(currentTrim)
				result = append(result, strings.Repeat("#", level)+" "+cleanText)
				i += 2
				continue
			}
		}

		result = append(result, current)
		i++
	}

	return strings.Join(result, "\n")
}

// isUnderline checks if a line is a valid RST underline (all same punctuation)
func isUnderline(line string) bool {
	if line == "" {
		return false
	}

	// Check if all characters are the same punctuation
	char := line[0]
	if !isPunctuation(char) {
		return false
	}

	for _, c := range line {
		if byte(c) != char {
			return false
		}
	}

	return true
}

// isPunctuation checks if a character is a valid RST heading punctuation
func isPunctuation(c byte) bool {
	// Common RST heading characters
	punctuation := "!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~"
	return strings.ContainsRune(punctuation, rune(c))
}

// getOrAssignLevel gets or assigns a heading level for a pattern
func getOrAssignLevel(pattern string, patterns map[string]int, nextLevel *int) int {
	if level, exists := patterns[pattern]; exists {
		return level
	}

	level := *nextLevel
	patterns[pattern] = level
	*nextLevel++

	// Cap at level 6 (max Markdown heading level)
	if *nextLevel > 6 {
		*nextLevel = 6
	}

	return level
}

// cleanHeadingText removes RST directives and extra formatting from heading text
func cleanHeadingText(text string) string {
	// Remove inline directives like :index:, :ref:, etc.
	// Pattern: `text`:directive:
	re := regexp.MustCompile("`([^`]+)`:[a-zA-Z]+:")
	text = re.ReplaceAllString(text, "$1")

	// Remove just the directive part if no backticks
	// Pattern: :directive:
	re2 := regexp.MustCompile(":[a-zA-Z]+:")
	text = re2.ReplaceAllString(text, "")

	return strings.TrimSpace(text)
}

// convertRSTImages converts RST image directives to Markdown format
func convertRSTImages(content string) string {
	lines := strings.Split(content, "\n")
	var result []string

	i := 0
	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Check for image or figure directive
		if strings.HasPrefix(trimmed, ".. image::") || strings.HasPrefix(trimmed, ".. figure::") {
			// Extract image path
			parts := strings.SplitN(trimmed, "::", 2)
			if len(parts) == 2 {
				imagePath := strings.TrimSpace(parts[1])
				altText := ""

				// Look ahead for :alt: option
				j := i + 1
				for j < len(lines) {
					nextLine := strings.TrimSpace(lines[j])

					// Stop if we hit a non-indented line or empty line after options
					if nextLine == "" {
						break
					}
					if !strings.HasPrefix(lines[j], "   ") && !strings.HasPrefix(lines[j], "\t") {
						break
					}

					// Extract alt text
					if strings.HasPrefix(nextLine, ":alt:") {
						altParts := strings.SplitN(nextLine, ":alt:", 2)
						if len(altParts) == 2 {
							altText = strings.TrimSpace(altParts[1])
						}
					}
					j++
				}

				// Convert to Markdown format
				markdownImage := fmt.Sprintf("![%s](%s)", altText, imagePath)
				result = append(result, markdownImage)
				result = append(result, "")

				// Skip the directive and its options
				i = j
				continue
			}
		}

		result = append(result, line)
		i++
	}

	return strings.Join(result, "\n")
}

// IsSupported returns true if the file type is supported
func IsSupported(filename string) bool {
	docType := DetectDocumentType(filename)
	return docType != kbtypes.TypeUnknown
}

// GetSupportedExtensions returns a list of supported file extensions
func GetSupportedExtensions() []string {
	return []string{".html", ".htm", ".md", ".rst", ".sgml", ".sgm", ".xml"}
}

// ReadAll reads all content from a reader
func ReadAll(r io.Reader) ([]byte, error) {
	return io.ReadAll(r)
}

// convertSGML converts SGML (DocBook) to Markdown
// This is a simplified converter for PostgreSQL-style DocBook SGML
func convertSGML(content []byte) (string, string, error) {
	text := string(content)
	title := extractSGMLTitle(text)

	// Convert SGML tags to Markdown
	markdown := convertSGMLTags(text)

	return markdown, title, nil
}

// extractSGMLTitle extracts the title from SGML content
func extractSGMLTitle(content string) string {
	// Look for <title> tag
	titleRe := regexp.MustCompile(`(?i)<title[^>]*>([^<]+)</title>`)
	matches := titleRe.FindStringSubmatch(content)
	if len(matches) > 1 {
		title := html.UnescapeString(matches[1])
		return strings.TrimSpace(title)
	}

	// Try <refentrytitle> (for reference pages)
	refTitleRe := regexp.MustCompile(`(?i)<refentrytitle[^>]*>([^<]+)</refentrytitle>`)
	matches = refTitleRe.FindStringSubmatch(content)
	if len(matches) > 1 {
		title := html.UnescapeString(matches[1])
		return strings.TrimSpace(title)
	}

	return ""
}

// convertSGMLTags converts SGML tags to Markdown
func convertSGMLTags(content string) string {
	// Remove SGML comments using simpler string operations
	// The regex <!--[\s\S]*?--> can cause catastrophic backtracking on large files
	for {
		start := strings.Index(content, "<!--")
		if start == -1 {
			break
		}
		end := strings.Index(content[start:], "-->")
		if end == -1 {
			break
		}
		content = content[:start] + content[start+end+3:]
	}

	// Remove DOCTYPE and SGML declarations
	content = regexp.MustCompile(`(?i)<!DOCTYPE[^>]*>`).ReplaceAllString(content, "")
	content = regexp.MustCompile(`(?i)<!ENTITY[^>]*>`).ReplaceAllString(content, "")

	// Convert headings (sect1, sect2, chapter, etc.)
	content = convertSGMLHeadings(content)

	// Convert paragraphs
	content = regexp.MustCompile(`(?i)<para[^>]*>`).ReplaceAllString(content, "\n")
	content = regexp.MustCompile(`(?i)</para>`).ReplaceAllString(content, "\n")

	// Convert emphasis tags - use non-greedy but limit to reasonable content
	// Limit inner content to avoid backtracking on malformed tags
	content = regexp.MustCompile(`(?i)<emphasis[^>]*>([^<]*)</emphasis>`).ReplaceAllString(content, "*$1*")
	content = regexp.MustCompile(`(?i)<literal[^>]*>([^<]*)</literal>`).ReplaceAllString(content, "`$1`")
	content = regexp.MustCompile(`(?i)<command[^>]*>([^<]*)</command>`).ReplaceAllString(content, "`$1`")
	content = regexp.MustCompile(`(?i)<filename[^>]*>([^<]*)</filename>`).ReplaceAllString(content, "`$1`")
	content = regexp.MustCompile(`(?i)<function[^>]*>([^<]*)</function>`).ReplaceAllString(content, "`$1`")
	content = regexp.MustCompile(`(?i)<type[^>]*>([^<]*)</type>`).ReplaceAllString(content, "`$1`")

	// Convert lists
	content = regexp.MustCompile(`(?i)<itemizedlist[^>]*>`).ReplaceAllString(content, "\n")
	content = regexp.MustCompile(`(?i)</itemizedlist>`).ReplaceAllString(content, "\n")
	content = regexp.MustCompile(`(?i)<listitem[^>]*>`).ReplaceAllString(content, "\n- ")
	content = regexp.MustCompile(`(?i)</listitem>`).ReplaceAllString(content, "")

	// Convert code blocks using string operations to avoid catastrophic backtracking
	// Track position to avoid reprocessing the same content
	pos := 0
	for {
		// Search from current position
		searchContent := strings.ToLower(content[pos:])
		start := strings.Index(searchContent, "<programlisting")
		if start == -1 {
			break
		}
		start += pos // Adjust to absolute position

		// Find the end of the opening tag
		tagEnd := strings.Index(content[start:], ">")
		if tagEnd == -1 {
			break
		}
		tagEnd += start

		// Find the closing tag
		closeTag := strings.Index(strings.ToLower(content[tagEnd:]), "</programlisting>")
		if closeTag == -1 {
			break
		}
		closeTag += tagEnd

		// Extract and format the code
		code := strings.TrimSpace(content[tagEnd+1 : closeTag])
		replacement := "\n```\n" + code + "\n```\n"
		content = content[:start] + replacement + content[closeTag+16:]

		// Move position past the replacement to avoid reprocessing
		pos = start + len(replacement)
	}

	// Remove remaining tags
	tagRe := regexp.MustCompile(`<[^>]+>`)
	content = tagRe.ReplaceAllString(content, "")

	// Clean up whitespace
	content = regexp.MustCompile(`\n{3,}`).ReplaceAllString(content, "\n\n")
	content = strings.TrimSpace(content)

	return content
}

// convertSGMLHeadings converts SGML heading tags to Markdown
func convertSGMLHeadings(content string) string {
	// Map of SGML heading tags to Markdown levels
	headingTags := map[string]int{
		"chapter":        1,
		"sect1":          2,
		"sect2":          3,
		"sect3":          4,
		"sect4":          5,
		"refsect1":       2,
		"refsect2":       3,
		"refsynopsisdiv": 2,
	}

	for tag, level := range headingTags {
		// Find <tag><title>...</title> patterns
		// Limit title content and whitespace to avoid backtracking - titles shouldn't contain tags
		// Limit whitespace matching to prevent catastrophic backtracking
		pattern := fmt.Sprintf(`(?i)<%s[^>]*>[\s\n]{0,100}<title[^>]*>([^<]*)</title>`, tag)
		re := regexp.MustCompile(pattern)
		content = re.ReplaceAllStringFunc(content, func(match string) string {
			titleRe := regexp.MustCompile(`(?i)<title[^>]*>([^<]*)</title>`)
			titleMatch := titleRe.FindStringSubmatch(match)
			if len(titleMatch) > 1 {
				title := html.UnescapeString(titleMatch[1])
				title = strings.TrimSpace(title)
				return "\n" + strings.Repeat("#", level) + " " + title + "\n"
			}
			return match
		})

		// Remove closing tags
		closingTag := fmt.Sprintf(`(?i)</%s>`, tag)
		content = regexp.MustCompile(closingTag).ReplaceAllString(content, "")
	}

	return content
}
