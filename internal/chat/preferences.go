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
	"strings"

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
		},
		ProviderModels: map[string]string{
			"anthropic": "claude-sonnet-4-20250514",
			"openai":    "gpt-5.1",
			"ollama":    "", // Will be determined dynamically from available models
		},
		LastProvider: "anthropic",
	}
}

// getKnownAnthropicModels returns the list of known Anthropic models
func getKnownAnthropicModels() []string {
	return []string{
		"claude-sonnet-4-20250514",
		"claude-3-7-sonnet-20250219",
		"claude-3-5-sonnet-20241022",
		"claude-3-5-sonnet-20240620",
		"claude-3-opus-20240229",
		"claude-3-sonnet-20240229",
		"claude-3-haiku-20240307",
	}
}

// getPreferredOllamaModels returns the user's preferred Ollama models in priority order
// These should all support tool calling
func getPreferredOllamaModels() []string {
	return []string{
		"gpt-oss:20b",
		"gemma:latest",
		"qwen3-coder:30b",
		"llama3.1:latest",
	}
}

// isValidModelForProvider checks if a model is valid for a given provider
// For Anthropic: checks against known static list
// For OpenAI: assumes all models starting with "gpt", "o1-", or "o3-" are valid
// For Ollama: we can't validate without querying the server
func isValidModelForProvider(provider, model string) bool {
	if model == "" {
		return false
	}

	switch provider {
	case "anthropic":
		knownModels := getKnownAnthropicModels()
		for _, known := range knownModels {
			if model == known {
				return true
			}
		}
		return false

	case "openai":
		// OpenAI models: gpt-*, o1-*, o3-*
		return strings.HasPrefix(model, "gpt-") ||
			strings.HasPrefix(model, "gpt") ||
			strings.HasPrefix(model, "o1-") ||
			strings.HasPrefix(model, "o3-")

	case "ollama":
		// We can't validate Ollama models without querying the server
		// Accept any non-empty model name
		return true

	default:
		return false
	}
}

// sanitizePreferences validates and fixes corrupted preference data
// Returns a sanitized copy of the preferences
func sanitizePreferences(prefs *Preferences) *Preferences {
	defaults := getDefaultPreferences()

	// Ensure provider_models map exists
	if prefs.ProviderModels == nil {
		prefs.ProviderModels = make(map[string]string)
	}

	// Validate each provider's model
	for _, provider := range []string{"anthropic", "openai", "ollama"} {
		if savedModel, exists := prefs.ProviderModels[provider]; exists {
			// Validate the saved model for this provider
			if !isValidModelForProvider(provider, savedModel) {
				// Invalid model for this provider - use default
				fmt.Fprintf(os.Stderr, "Warning: Invalid model '%s' for provider '%s', using default\n",
					savedModel, provider)
				prefs.ProviderModels[provider] = defaults.ProviderModels[provider]
			}
		} else {
			// No saved model for this provider - use default
			prefs.ProviderModels[provider] = defaults.ProviderModels[provider]
		}
	}

	// Validate LastProvider
	validProviders := map[string]bool{
		"anthropic": true,
		"openai":    true,
		"ollama":    true,
	}
	if !validProviders[prefs.LastProvider] {
		// Invalid provider - use default
		prefs.LastProvider = defaults.LastProvider
	}

	// Ensure the saved provider has a valid model
	if prefs.LastProvider != "" {
		if model := prefs.ProviderModels[prefs.LastProvider]; model == "" {
			prefs.ProviderModels[prefs.LastProvider] = defaults.ProviderModels[prefs.LastProvider]
		}
	}

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
