/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyAPIKeyFlags(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "provider-key")
	if err := os.WriteFile(keyPath, []byte("key-from-file\n"), 0600); err != nil {
		t.Fatalf("Failed to write the key file: %v", err)
	}
	emptyPath := filepath.Join(tmpDir, "empty-key")
	if err := os.WriteFile(emptyPath, []byte("   \n"), 0600); err != nil {
		t.Fatalf("Failed to write the empty key file: %v", err)
	}
	missingPath := filepath.Join(tmpDir, "no-such-file")
	// Reading a directory always fails, whoever the test runs as, which a
	// mode 0000 file would not guarantee under root.
	unreadablePath := filepath.Join(tmpDir, "a-directory")
	if err := os.Mkdir(unreadablePath, 0755); err != nil {
		t.Fatalf("Failed to create the directory: %v", err)
	}

	tests := []struct {
		name        string
		key         string
		keyFile     string
		existingKey string
		wantKey     string
		wantKeyFile string
		wantErr     string
	}{
		{
			name:        "neither flag leaves the configuration alone",
			existingKey: "configured-key",
			wantKey:     "configured-key",
		},
		{
			name:        "a direct key overrides the configuration",
			key:         "flag-key",
			existingKey: "configured-key",
			wantKey:     "flag-key",
		},
		{
			name:        "a key file is read and recorded",
			keyFile:     keyPath,
			existingKey: "configured-key",
			wantKey:     "key-from-file",
			wantKeyFile: keyPath,
		},
		{
			name:        "a direct key beats a key file",
			key:         "flag-key",
			keyFile:     keyPath,
			wantKey:     "flag-key",
			wantKeyFile: keyPath,
		},
		{
			name:        "a missing key file is an error rather than a silent fallback",
			keyFile:     missingPath,
			existingKey: "configured-key",
			wantKeyFile: missingPath,
			wantErr:     "Gemini API key file " + missingPath + " is missing or empty",
		},
		{
			name:        "an unreadable key file is an error",
			keyFile:     unreadablePath,
			wantKeyFile: unreadablePath,
			wantErr:     "error reading Gemini API key file",
		},
		{
			name:        "an empty key file is an error",
			keyFile:     emptyPath,
			wantKeyFile: emptyPath,
			wantErr:     "Gemini API key file " + emptyPath + " is missing or empty",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			key := tc.existingKey
			keyFile := ""

			err := applyAPIKeyFlags("Gemini", tc.key, tc.keyFile, &key, &keyFile)

			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("Expected an error containing %q, got nil", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("Expected an error containing %q, got %q", tc.wantErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("applyAPIKeyFlags failed: %v", err)
			}
			if key != tc.wantKey {
				t.Errorf("Expected the key '%s', got '%s'", tc.wantKey, key)
			}
			if keyFile != tc.wantKeyFile {
				t.Errorf("Expected the key file '%s', got '%s'", tc.wantKeyFile, keyFile)
			}
		})
	}
}

// The provider label must reach the error text, since the three providers
// share one helper and an unlabelled message would be useless.
func TestApplyAPIKeyFlagsNamesTheProvider(t *testing.T) {
	unreadablePath := filepath.Join(t.TempDir(), "a-directory")
	if err := os.Mkdir(unreadablePath, 0755); err != nil {
		t.Fatalf("Failed to create the directory: %v", err)
	}

	for _, provider := range []string{"Anthropic", "OpenAI", "Gemini"} {
		key, keyFile := "", ""
		err := applyAPIKeyFlags(provider, "", unreadablePath, &key, &keyFile)
		if err == nil {
			t.Fatalf("Expected an error for provider %s, got nil", provider)
		}
		if !strings.Contains(err.Error(), provider) {
			t.Errorf("Expected the error to name %s, got %q", provider, err.Error())
		}
	}
}
