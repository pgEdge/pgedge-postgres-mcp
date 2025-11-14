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

	"gopkg.in/yaml.v3"
)

// Preferences represents user preferences that can be modified at runtime
// This is separate from the main configuration file for security reasons
type Preferences struct {
	// Reserved for future user preferences
	// Database connection is now configured via config file, environment, or CLI flags
}

// LoadPreferences loads user preferences from a YAML file
func LoadPreferences(path string) (*Preferences, error) {
	// If file doesn't exist, return empty preferences
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &Preferences{}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read preferences file: %w", err)
	}

	var prefs Preferences
	if err := yaml.Unmarshal(data, &prefs); err != nil {
		return nil, fmt.Errorf("failed to parse preferences file: %w", err)
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
