/*-------------------------------------------------------------------------
 *
 * pgEdge Postgres MCP Server
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package config

import (
    "fmt"
    "os"
    "path/filepath"

    "pgedge-postgres-mcp/internal/auth"

    "gopkg.in/yaml.v3"
)

// Preferences represents user preferences that can be modified at runtime
// This is separate from the main configuration file for security reasons
type Preferences struct {
    // Saved database connections (used when auth is disabled)
    // When auth is enabled, connections are stored per-token in the token file
    Connections *auth.SavedConnectionStore `yaml:"connections,omitempty"`
}

// LoadPreferences loads user preferences from a YAML file
func LoadPreferences(path string) (*Preferences, error) {
    // If file doesn't exist, return empty preferences
    if _, err := os.Stat(path); os.IsNotExist(err) {
        return &Preferences{
            Connections: auth.NewSavedConnectionStore(),
        }, nil
    }

    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("failed to read preferences file: %w", err)
    }

    var prefs Preferences
    if err := yaml.Unmarshal(data, &prefs); err != nil {
        return nil, fmt.Errorf("failed to parse preferences file: %w", err)
    }

    // Ensure connections is initialized
    if prefs.Connections == nil {
        prefs.Connections = auth.NewSavedConnectionStore()
    }

    return &prefs, nil
}

// SavePreferences saves user preferences to a YAML file
func SavePreferences(path string, prefs *Preferences) error {
    data, err := yaml.Marshal(prefs)
    if err != nil {
        return fmt.Errorf("failed to marshal preferences: %w", err)
    }

    // Create directory if it doesn't exist
    dir := filepath.Dir(path)
    if err := os.MkdirAll(dir, 0755); err != nil {
        return fmt.Errorf("failed to create directory: %w", err)
    }

    // Write with appropriate permissions
    if err := os.WriteFile(path, data, 0644); err != nil {
        return fmt.Errorf("failed to write preferences file: %w", err)
    }

    return nil
}

// GetDefaultPreferencesPath returns the default preferences file path (same directory as binary)
func GetDefaultPreferencesPath(binaryPath string) string {
    dir := filepath.Dir(binaryPath)
    return filepath.Join(dir, "pgedge-postgres-mcp-prefs.yaml")
}
