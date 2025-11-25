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

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadDefinitions loads prompt and resource definitions from a YAML file
func LoadDefinitions(path string) (*Definitions, error) {
	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read definitions file: %w", err)
	}

	// Determine format based on file extension
	ext := strings.ToLower(filepath.Ext(path))
	var defs Definitions

	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &defs); err != nil {
			return nil, fmt.Errorf("failed to parse YAML: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported file format: %s (expected .yaml or .yml)", ext)
	}

	// Validate definitions
	if err := ValidateDefinitions(&defs); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	return &defs, nil
}
