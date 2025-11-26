/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package config

import (
	"fmt"
	"os"
	"sync"
)

// ReloadableConfig wraps a Config with thread-safe access and reload capability
type ReloadableConfig struct {
	mu       sync.RWMutex
	config   *Config
	path     string
	cliFlags CLIFlags
	onReload []func(*Config)
}

// NewReloadableConfig creates a new reloadable configuration
func NewReloadableConfig(config *Config, path string, cliFlags CLIFlags) *ReloadableConfig {
	return &ReloadableConfig{
		config:   config,
		path:     path,
		cliFlags: cliFlags,
		onReload: make([]func(*Config), 0),
	}
}

// Get returns the current configuration (read-only access)
func (rc *ReloadableConfig) Get() *Config {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.config
}

// Reload reloads the configuration from the file
// Returns an error if the reload fails, but keeps the old config
func (rc *ReloadableConfig) Reload() error {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if rc.path == "" {
		return fmt.Errorf("no configuration file path set")
	}

	// Load the new configuration (LoadConfig applies CLI flags internally)
	newConfig, err := LoadConfig(rc.path, rc.cliFlags)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Validate the new configuration
	if err := validateConfig(newConfig); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Log what settings require restart (won't be applied)
	rc.logRestartRequiredSettings(newConfig)

	// Update the config
	oldConfig := rc.config
	rc.config = newConfig

	// Notify all registered callbacks
	for _, callback := range rc.onReload {
		callback(newConfig)
	}

	// Log successful reload
	fmt.Fprintf(os.Stderr, "Configuration reloaded successfully from %s\n", rc.path)
	fmt.Fprintf(os.Stderr, "  Databases: %d configured\n", len(newConfig.Databases))

	// Log if databases changed
	if len(oldConfig.Databases) != len(newConfig.Databases) {
		fmt.Fprintf(os.Stderr, "  Database count changed: %d -> %d\n",
			len(oldConfig.Databases), len(newConfig.Databases))
	}

	return nil
}

// logRestartRequiredSettings logs settings that changed but require a restart
func (rc *ReloadableConfig) logRestartRequiredSettings(newConfig *Config) {
	old := rc.config

	// HTTP mode changes require restart
	if old.HTTP.Enabled != newConfig.HTTP.Enabled {
		fmt.Fprintf(os.Stderr, "  WARNING: http.enabled changed - requires restart\n")
	}
	if old.HTTP.Address != newConfig.HTTP.Address {
		fmt.Fprintf(os.Stderr, "  WARNING: http.address changed - requires restart\n")
	}

	// TLS changes require restart
	if old.HTTP.TLS.Enabled != newConfig.HTTP.TLS.Enabled {
		fmt.Fprintf(os.Stderr, "  WARNING: http.tls.enabled changed - requires restart\n")
	}
	if old.HTTP.TLS.CertFile != newConfig.HTTP.TLS.CertFile {
		fmt.Fprintf(os.Stderr, "  WARNING: http.tls.cert_file changed - requires restart\n")
	}
	if old.HTTP.TLS.KeyFile != newConfig.HTTP.TLS.KeyFile {
		fmt.Fprintf(os.Stderr, "  WARNING: http.tls.key_file changed - requires restart\n")
	}

	// LLM/embedding provider changes are logged (may work but connections need reset)
	if old.LLM.Provider != newConfig.LLM.Provider {
		fmt.Fprintf(os.Stderr, "  NOTE: llm.provider changed to %s\n", newConfig.LLM.Provider)
	}
	if old.LLM.Model != newConfig.LLM.Model {
		fmt.Fprintf(os.Stderr, "  NOTE: llm.model changed to %s\n", newConfig.LLM.Model)
	}
	if old.Embedding.Provider != newConfig.Embedding.Provider {
		fmt.Fprintf(os.Stderr, "  NOTE: embedding.provider changed to %s\n", newConfig.Embedding.Provider)
	}
}

// OnReload registers a callback to be called when configuration is reloaded
// The callback receives the new configuration
func (rc *ReloadableConfig) OnReload(fn func(*Config)) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.onReload = append(rc.onReload, fn)
}

// GetPath returns the configuration file path
func (rc *ReloadableConfig) GetPath() string {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.path
}
