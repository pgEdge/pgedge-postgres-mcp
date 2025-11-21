/*-------------------------------------------------------------------------
 *
 * User Preferences for MCP Chat Client
 *
 * Copyright (c) 2025, pgEdge, Inc.
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
    UI             UIPreferences            `yaml:"ui"`
    ProviderModels map[string]string        `yaml:"provider_models"`
    LastProvider   string                   `yaml:"last_provider"`
}

// UIPreferences holds UI-related preferences
type UIPreferences struct {
    DisplayStatusMessages bool `yaml:"display_status_messages"`
    RenderMarkdown        bool `yaml:"render_markdown"`
}

// GetPreferencesPath returns the path to the user preferences file
func GetPreferencesPath() string {
    return filepath.Join(os.Getenv("HOME"), ".pgedge-pg-mcp-cli-prefs")
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

    // Ensure provider_models map is initialized
    if prefs.ProviderModels == nil {
        prefs.ProviderModels = make(map[string]string)
    }

    // Fill in any missing default models
    defaults := getDefaultPreferences()
    for provider, model := range defaults.ProviderModels {
        if _, exists := prefs.ProviderModels[provider]; !exists {
            prefs.ProviderModels[provider] = model
        }
    }

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
        },
        ProviderModels: map[string]string{
            "anthropic": "claude-sonnet-4-20250514",
            "openai":    "gpt-5-main",
            "ollama":    "llama3",
        },
        LastProvider: "anthropic",
    }
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
