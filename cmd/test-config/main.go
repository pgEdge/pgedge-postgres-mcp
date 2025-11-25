/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package main

import (
	"fmt"
	"os"

	"pgedge-postgres-mcp/internal/chat"
)

func main() {
	configPath := "bin/openai-test.yaml"

	fmt.Printf("Loading config from: %s\n", configPath)
	cfg, err := chat.LoadConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nConfig loaded successfully:\n")
	fmt.Printf("  LLM Provider: %s\n", cfg.LLM.Provider)
	fmt.Printf("  LLM Model: %s\n", cfg.LLM.Model)
	fmt.Printf("  LLM Max Tokens: %d\n", cfg.LLM.MaxTokens)
	fmt.Printf("  LLM Temperature: %.1f\n", cfg.LLM.Temperature)
	fmt.Printf("  MCP Mode: %s\n", cfg.MCP.Mode)
	fmt.Printf("  MCP Server Path: %s\n", cfg.MCP.ServerPath)

	fmt.Printf("\nValidating config...\n")
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Validation error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nConfig after validation:\n")
	fmt.Printf("  LLM Provider: %s\n", cfg.LLM.Provider)
	fmt.Printf("  LLM Model: %s\n", cfg.LLM.Model)
	fmt.Printf("  LLM Max Tokens: %d\n", cfg.LLM.MaxTokens)
	fmt.Printf("  LLM Temperature: %.1f\n", cfg.LLM.Temperature)
}
