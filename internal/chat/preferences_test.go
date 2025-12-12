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
	"os"
	"path/filepath"
	"testing"
)

func TestGetDefaultPreferences(t *testing.T) {
	prefs := getDefaultPreferences()

	// Check UI defaults
	if !prefs.UI.DisplayStatusMessages {
		t.Error("Expected DisplayStatusMessages to be true by default")
	}
	if !prefs.UI.RenderMarkdown {
		t.Error("Expected RenderMarkdown to be true by default")
	}
	if prefs.UI.Debug {
		t.Error("Expected Debug to be false by default")
	}
	if !prefs.UI.Color {
		t.Error("Expected Color to be true by default")
	}

	// Check provider defaults
	if prefs.LastProvider != "anthropic" {
		t.Errorf("Expected LastProvider to be 'anthropic', got %q", prefs.LastProvider)
	}

	// Check model defaults
	if prefs.ProviderModels["anthropic"] != "claude-sonnet-4-20250514" {
		t.Errorf("Expected anthropic model to be 'claude-sonnet-4-20250514', got %q", prefs.ProviderModels["anthropic"])
	}
	if prefs.ProviderModels["openai"] != "gpt-5.1" {
		t.Errorf("Expected openai model to be 'gpt-5.1', got %q", prefs.ProviderModels["openai"])
	}
	if prefs.ProviderModels["ollama"] != "qwen3-coder:latest" {
		t.Errorf("Expected ollama model to be 'qwen3-coder:latest', got %q", prefs.ProviderModels["ollama"])
	}
}

func TestSanitizePreferences(t *testing.T) {
	tests := []struct {
		name  string
		prefs *Preferences
		check func(*Preferences, *testing.T)
	}{
		{
			name: "nil provider models map",
			prefs: &Preferences{
				ProviderModels: nil,
				LastProvider:   "anthropic",
			},
			check: func(p *Preferences, t *testing.T) {
				if p.ProviderModels == nil {
					t.Error("Expected ProviderModels to be initialized")
				}
			},
		},
		{
			name: "invalid last provider",
			prefs: &Preferences{
				ProviderModels: make(map[string]string),
				LastProvider:   "invalid",
			},
			check: func(p *Preferences, t *testing.T) {
				if p.LastProvider != "anthropic" {
					t.Errorf("Expected LastProvider to be reset to 'anthropic', got %q", p.LastProvider)
				}
			},
		},
		{
			name: "valid preferences",
			prefs: &Preferences{
				ProviderModels: map[string]string{
					"anthropic": "claude-3-opus",
				},
				LastProvider: "openai",
			},
			check: func(p *Preferences, t *testing.T) {
				if p.LastProvider != "openai" {
					t.Errorf("Expected LastProvider to remain 'openai', got %q", p.LastProvider)
				}
				if p.ProviderModels["anthropic"] != "claude-3-opus" {
					t.Errorf("Expected model to remain 'claude-3-opus', got %q", p.ProviderModels["anthropic"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizePreferences(tt.prefs)
			tt.check(result, t)
		})
	}
}

func TestPreferencesGetModelForProvider(t *testing.T) {
	prefs := &Preferences{
		ProviderModels: map[string]string{
			"anthropic": "claude-3-opus",
			"openai":    "gpt-4-turbo",
		},
	}

	tests := []struct {
		provider string
		expected string
	}{
		{"anthropic", "claude-3-opus"},
		{"openai", "gpt-4-turbo"},
		{"ollama", "qwen3-coder:latest"}, // Falls back to defaults
		{"unknown", ""},                  // Not in defaults either
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			got := prefs.GetModelForProvider(tt.provider)
			if got != tt.expected {
				t.Errorf("GetModelForProvider(%q) = %q, want %q", tt.provider, got, tt.expected)
			}
		})
	}
}

func TestPreferencesSetModelForProvider(t *testing.T) {
	// Test with nil map
	prefs := &Preferences{
		ProviderModels: nil,
	}

	prefs.SetModelForProvider("anthropic", "claude-3-opus")

	if prefs.ProviderModels == nil {
		t.Error("Expected ProviderModels to be initialized")
	}
	if prefs.ProviderModels["anthropic"] != "claude-3-opus" {
		t.Errorf("Expected model 'claude-3-opus', got %q", prefs.ProviderModels["anthropic"])
	}

	// Test with existing map
	prefs.SetModelForProvider("openai", "gpt-4")
	if prefs.ProviderModels["openai"] != "gpt-4" {
		t.Errorf("Expected model 'gpt-4', got %q", prefs.ProviderModels["openai"])
	}
}

func TestPreferencesGetDatabaseForServer(t *testing.T) {
	tests := []struct {
		name            string
		serverDatabases map[string]string
		serverKey       string
		expected        string
	}{
		{
			name:            "nil map",
			serverDatabases: nil,
			serverKey:       "server1",
			expected:        "",
		},
		{
			name:            "key exists",
			serverDatabases: map[string]string{"server1": "mydb"},
			serverKey:       "server1",
			expected:        "mydb",
		},
		{
			name:            "key not found",
			serverDatabases: map[string]string{"server1": "mydb"},
			serverKey:       "server2",
			expected:        "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefs := &Preferences{
				ServerDatabases: tt.serverDatabases,
			}
			got := prefs.GetDatabaseForServer(tt.serverKey)
			if got != tt.expected {
				t.Errorf("GetDatabaseForServer(%q) = %q, want %q", tt.serverKey, got, tt.expected)
			}
		})
	}
}

func TestPreferencesSetDatabaseForServer(t *testing.T) {
	// Test with nil map
	prefs := &Preferences{
		ServerDatabases: nil,
	}

	prefs.SetDatabaseForServer("server1", "mydb")

	if prefs.ServerDatabases == nil {
		t.Error("Expected ServerDatabases to be initialized")
	}
	if prefs.ServerDatabases["server1"] != "mydb" {
		t.Errorf("Expected database 'mydb', got %q", prefs.ServerDatabases["server1"])
	}

	// Test setting empty string (should delete)
	prefs.SetDatabaseForServer("server1", "")
	if _, exists := prefs.ServerDatabases["server1"]; exists {
		t.Error("Expected server1 to be deleted when setting empty string")
	}

	// Test setting another server
	prefs.SetDatabaseForServer("server2", "otherdb")
	if prefs.ServerDatabases["server2"] != "otherdb" {
		t.Errorf("Expected database 'otherdb', got %q", prefs.ServerDatabases["server2"])
	}
}

func TestLoadPreferences_NonExistentFile(t *testing.T) {
	// Save original HOME and restore after test
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)

	// Set HOME to temp directory
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)

	// Load preferences (should return defaults since file doesn't exist)
	prefs, err := LoadPreferences()
	if err != nil {
		t.Fatalf("LoadPreferences failed: %v", err)
	}

	// Should return default preferences
	if prefs.LastProvider != "anthropic" {
		t.Errorf("Expected default LastProvider 'anthropic', got %q", prefs.LastProvider)
	}
}

func TestSaveAndLoadPreferences(t *testing.T) {
	// Save original HOME and restore after test
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)

	// Set HOME to temp directory
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)

	// Create preferences to save
	prefs := &Preferences{
		UI: UIPreferences{
			DisplayStatusMessages: false,
			RenderMarkdown:        false,
			Debug:                 true,
			Color:                 false,
		},
		ProviderModels: map[string]string{
			"anthropic": "claude-3-opus",
			"openai":    "gpt-4-turbo",
		},
		LastProvider: "openai",
		ServerDatabases: map[string]string{
			"server1": "testdb",
		},
	}

	// Save preferences
	if err := SavePreferences(prefs); err != nil {
		t.Fatalf("SavePreferences failed: %v", err)
	}

	// Verify file was created
	prefsPath := filepath.Join(tmpDir, ".pgedge-nla-cli-prefs")
	if _, err := os.Stat(prefsPath); os.IsNotExist(err) {
		t.Fatal("Preferences file was not created")
	}

	// Load preferences back
	loadedPrefs, err := LoadPreferences()
	if err != nil {
		t.Fatalf("LoadPreferences failed: %v", err)
	}

	// Verify loaded values
	if loadedPrefs.UI.DisplayStatusMessages != false {
		t.Error("Expected DisplayStatusMessages to be false")
	}
	if loadedPrefs.UI.RenderMarkdown != false {
		t.Error("Expected RenderMarkdown to be false")
	}
	if loadedPrefs.UI.Debug != true {
		t.Error("Expected Debug to be true")
	}
	if loadedPrefs.UI.Color != false {
		t.Error("Expected Color to be false")
	}
	if loadedPrefs.LastProvider != "openai" {
		t.Errorf("Expected LastProvider 'openai', got %q", loadedPrefs.LastProvider)
	}
	if loadedPrefs.ProviderModels["anthropic"] != "claude-3-opus" {
		t.Errorf("Expected anthropic model 'claude-3-opus', got %q", loadedPrefs.ProviderModels["anthropic"])
	}
	if loadedPrefs.ServerDatabases["server1"] != "testdb" {
		t.Errorf("Expected server1 database 'testdb', got %q", loadedPrefs.ServerDatabases["server1"])
	}
}

func TestLoadPreferences_InvalidYAML(t *testing.T) {
	// Save original HOME and restore after test
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)

	// Set HOME to temp directory
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)

	// Create invalid YAML file
	prefsPath := filepath.Join(tmpDir, ".pgedge-nla-cli-prefs")
	if err := os.WriteFile(prefsPath, []byte("invalid: yaml: content: ["), 0600); err != nil {
		t.Fatalf("Failed to write invalid YAML: %v", err)
	}

	// Load should fail
	_, err := LoadPreferences()
	if err == nil {
		t.Error("Expected error loading invalid YAML")
	}
}

func TestGetPreferencesPath(t *testing.T) {
	// Save original HOME
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)

	// Set a known HOME
	os.Setenv("HOME", "/test/home")

	path := GetPreferencesPath()
	expected := "/test/home/.pgedge-nla-cli-prefs"
	if path != expected {
		t.Errorf("GetPreferencesPath() = %q, want %q", path, expected)
	}
}
