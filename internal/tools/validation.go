/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package tools

import (
	"fmt"

	"pgedge-postgres-mcp/internal/mcp"
)

// ValidateStringParam validates and extracts a required string parameter from args
// Returns the string value and a ToolResponse error if validation fails
func ValidateStringParam(args map[string]interface{}, name string) (string, *mcp.ToolResponse) {
	value, ok := args[name].(string)
	if !ok || value == "" {
		resp, err := mcp.NewToolError(fmt.Sprintf("Missing or invalid '%s' argument", name))
		if err != nil {
			return "", &resp
		}
		return "", &resp
	}
	return value, nil
}

// ValidateOptionalStringParam validates and extracts an optional string parameter
// Returns the string value (or defaultValue if not present) and no error
func ValidateOptionalStringParam(args map[string]interface{}, name string, defaultValue string) string {
	value, ok := args[name].(string)
	if !ok {
		return defaultValue
	}
	return value
}

// ValidateNumberParam validates and extracts a required number parameter from args
// Returns the float64 value and a ToolResponse error if validation fails
func ValidateNumberParam(args map[string]interface{}, name string) (float64, *mcp.ToolResponse) {
	value, ok := args[name].(float64)
	if !ok {
		resp, err := mcp.NewToolError(fmt.Sprintf("Error: %s must be a number", name))
		if err != nil {
			return 0, &resp
		}
		return 0, &resp
	}
	return value, nil
}

// ValidateOptionalNumberParam validates and extracts an optional number parameter
// Returns the float64 value (or defaultValue if not present) and no error
func ValidateOptionalNumberParam(args map[string]interface{}, name string, defaultValue float64) float64 {
	value, ok := args[name].(float64)
	if !ok {
		return defaultValue
	}
	return value
}

// ValidateBoolParam validates and extracts an optional boolean parameter
// Returns the bool value (or defaultValue if not present)
func ValidateBoolParam(args map[string]interface{}, name string, defaultValue bool) bool {
	value, ok := args[name].(bool)
	if !ok {
		return defaultValue
	}
	return value
}

// ValidatePositiveNumber checks if a number is greater than zero
// Returns a ToolResponse error if validation fails, nil otherwise
func ValidatePositiveNumber(value float64, name string) *mcp.ToolResponse {
	if value <= 0 {
		resp, err := mcp.NewToolError(fmt.Sprintf("Error: %s must be greater than 0", name))
		if err != nil {
			return &resp
		}
		return &resp
	}
	return nil
}
