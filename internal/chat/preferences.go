/*-------------------------------------------------------------------------
*
 * pgEdge Natural Language Agent
*
* Portions copyright (c) 2025, pgEdge, Inc.
* This software is released under The PostgreSQL License
*
*-------------------------------------------------------------------------
*/

package chat

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Preferences holds user preferences that persist across sessions
type Preferences struct {
	UI              UIPreferences     `yaml:"ui"`
	ProviderModels  map[string]string `yaml:"provider_models"`
	LastProvider    string            `yaml:"last_provider"`
	ServerDatabases map[string]string `yaml:"server_databases,omitempty"` // server key -> database name
}

// UIPreferences holds UI-related preferences
type UIPreferences struct {
	DisplayStatusMessages bool `yaml:"display_status_messages"`
	RenderMarkdown        bool `yaml:"render_markdown"`
	Debug                 bool `yaml:"debug"`
	Color                 bool `yaml:"color"`
}

// GetPreferencesPath returns the path to the user preferences file
func GetPreferencesPath() string {
	return filepath.Join(os.Getenv("HOME"), ".pgedge-nla-cli-prefs")
}

// LoadPreferences loads user preferences from the preferences file
// Returns default preferences if file doesn't exist
func LoadPreferences() (*Preferences, error) {
	path := GetPreferencesPath()

	// If file doesn't exist, return defaults
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return getDefaultPreferences(), nil
	}

	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read preferences file: %w", err)
	}

	// Parse YAML
	prefs := &Preferences{}
	if err := yaml.Unmarshal(data, prefs); err != nil {
		return nil, fmt.Errorf("failed to parse preferences file: %w", err)
	}

	// Sanitize and validate loaded preferences
	prefs = sanitizePreferences(prefs)

	return prefs, nil
}

// SavePreferences saves user preferences to the preferences file
func SavePreferences(prefs *Preferences) error {
	path := GetPreferencesPath()

	// Marshal to YAML
	data, err := yaml.Marshal(prefs)
	if err != nil {
		return fmt.Errorf("failed to marshal preferences: %w", err)
	}

	// Write to temporary file first for atomic write
	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write preferences file: %w", err)
	}

	// Rename to final location (atomic on Unix)
	if err := os.Rename(tempPath, path); err != nil {
		os.Remove(tempPath) // Clean up temp file
		return fmt.Errorf("failed to save preferences file: %w", err)
	}

	return nil
}

// getDefaultPreferences returns default preferences
func getDefaultPreferences() *Preferences {
	return &Preferences{
		UI: UIPreferences{
			DisplayStatusMessages: true,
			RenderMarkdown:        true,
			Debug:                 false,
			Color:                 true,
		},
		ProviderModels: map[string]string{
			"anthropic": "claude-sonnet-4-5-20250929",
			"openai":    "gpt-4o",
			"ollama":    "qwen3-coder:latest",
		},
		LastProvider: "anthropic",
	}
}

// sanitizePreferences validates and fixes corrupted preference data
// Only validates structure, not model validity (done at runtime in initializeLLM)
func sanitizePreferences(prefs *Preferences) *Preferences {
	// Ensure provider_models map exists
	if prefs.ProviderModels == nil {
		prefs.ProviderModels = make(map[string]string)
	}

	// Validate LastProvider is a known provider name
	validProviders := map[string]bool{
		"anthropic": true,
		"openai":    true,
		"ollama":    true,
	}
	if !validProviders[prefs.LastProvider] {
		// Invalid provider - use default
		defaults := getDefaultPreferences()
		prefs.LastProvider = defaults.LastProvider
	}

	// Don't validate models here - that requires API access
	// Model validation happens at runtime in initializeLLM()

	return prefs
}

// GetModelForProvider returns the preferred model for a provider
func (p *Preferences) GetModelForProvider(provider string) string {
	if model, exists := p.ProviderModels[provider]; exists {
		return model
	}

	// Fall back to defaults
	defaults := getDefaultPreferences()
	if model, exists := defaults.ProviderModels[provider]; exists {
		return model
	}

	return ""
}

// SetModelForProvider sets the preferred model for a provider
func (p *Preferences) SetModelForProvider(provider, model string) {
	if p.ProviderModels == nil {
		p.ProviderModels = make(map[string]string)
	}
	p.ProviderModels[provider] = model
}

// GetDatabaseForServer returns the preferred database for a server
func (p *Preferences) GetDatabaseForServer(serverKey string) string {
	if p.ServerDatabases == nil {
		return ""
	}
	return p.ServerDatabases[serverKey]
}

// SetDatabaseForServer sets the preferred database for a server
func (p *Preferences) SetDatabaseForServer(serverKey, database string) {
	if p.ServerDatabases == nil {
		p.ServerDatabases = make(map[string]string)
	}
	if database == "" {
		delete(p.ServerDatabases, serverKey)
	} else {
		p.ServerDatabases[serverKey] = database
	}
}
