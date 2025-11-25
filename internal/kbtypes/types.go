/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package kbtypes

// DocumentType represents the type of a document
type DocumentType int

const (
	// TypeUnknown represents an unknown or unsupported document type
	TypeUnknown DocumentType = iota
	// TypeMarkdown represents a Markdown document
	TypeMarkdown
	// TypeHTML represents an HTML document
	TypeHTML
	// TypeReStructuredText represents a reStructuredText document
	TypeReStructuredText
	// TypeSGML represents an SGML document
	TypeSGML
)

// String returns the string representation of a DocumentType
func (dt DocumentType) String() string {
	switch dt {
	case TypeMarkdown:
		return "Markdown"
	case TypeHTML:
		return "HTML"
	case TypeReStructuredText:
		return "reStructuredText"
	case TypeSGML:
		return "SGML"
	default:
		return "Unknown"
	}
}

// Document represents a processed document
type Document struct {
	Title          string
	Content        string // Markdown content
	SourceContent  []byte // Original content
	FilePath       string
	ProjectName    string
	ProjectVersion string
	DocType        DocumentType
}

// Chunk represents a chunk of a document with embeddings
type Chunk struct {
	ID                 int // Database ID (populated when retrieved from DB)
	Text               string
	Title              string // Document title
	Section            string // Section heading
	ProjectName        string
	ProjectVersion     string
	FilePath           string
	SourceFileChecksum string // SHA256 checksum of source file

	// Embeddings from different providers
	OpenAIEmbedding []float32
	VoyageEmbedding []float32
	OllamaEmbedding []float32
}
