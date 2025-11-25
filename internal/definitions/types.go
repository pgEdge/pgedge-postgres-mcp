/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package definitions

// Definitions contains user-defined prompts and resources loaded from a file
type Definitions struct {
	Prompts   []PromptDefinition   `json:"prompts,omitempty" yaml:"prompts,omitempty"`
	Resources []ResourceDefinition `json:"resources,omitempty" yaml:"resources,omitempty"`
}

// PromptDefinition defines a user-defined prompt with templates
type PromptDefinition struct {
	Name        string        `json:"name" yaml:"name"`
	Description string        `json:"description,omitempty" yaml:"description,omitempty"`
	Arguments   []ArgumentDef `json:"arguments,omitempty" yaml:"arguments,omitempty"`
	Messages    []MessageDef  `json:"messages" yaml:"messages"`
}

// ArgumentDef defines a prompt argument
type ArgumentDef struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	Required    bool   `json:"required" yaml:"required"`
}

// MessageDef defines a message in a prompt
type MessageDef struct {
	Role    string     `json:"role" yaml:"role"`
	Content ContentDef `json:"content" yaml:"content"`
}

// ContentDef defines message content (text, image, or resource)
type ContentDef struct {
	Type     string `json:"type" yaml:"type"`
	Text     string `json:"text,omitempty" yaml:"text,omitempty"`
	Data     string `json:"data,omitempty" yaml:"data,omitempty"`
	MimeType string `json:"mimeType,omitempty" yaml:"mimeType,omitempty"`
	URI      string `json:"uri,omitempty" yaml:"uri,omitempty"`
}

// ResourceDefinition defines a user-defined resource
type ResourceDefinition struct {
	URI         string      `json:"uri" yaml:"uri"`
	Name        string      `json:"name" yaml:"name"`
	Description string      `json:"description,omitempty" yaml:"description,omitempty"`
	MimeType    string      `json:"mimeType,omitempty" yaml:"mimeType,omitempty"`
	Type        string      `json:"type" yaml:"type"` // "sql" or "static"
	SQL         string      `json:"sql,omitempty" yaml:"sql,omitempty"`
	Data        interface{} `json:"data,omitempty" yaml:"data,omitempty"`
}
