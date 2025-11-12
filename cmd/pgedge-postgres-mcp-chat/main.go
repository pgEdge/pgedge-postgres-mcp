/*-------------------------------------------------------------------------
 *
 * pgEdge Postgres MCP Chat Client
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"pgedge-postgres-mcp/internal/chat"
)

const (
	version = "1.0.0-alpha1"
)

func main() {
	// Command line flags
	configFile := flag.String("config", "", "Path to configuration file")
	showVersion := flag.Bool("version", false, "Show version and exit")
	mcpMode := flag.String("mcp-mode", "", "MCP connection mode: stdio or http (default: stdio)")
	mcpURL := flag.String("mcp-url", "", "MCP server URL (for HTTP mode)")
	mcpServerPath := flag.String("mcp-server-path", "", "Path to MCP server binary (for stdio mode)")
	llmProvider := flag.String("llm-provider", "", "LLM provider: anthropic or ollama (default: anthropic)")
	llmModel := flag.String("llm-model", "", "LLM model to use")
	apiKey := flag.String("api-key", "", "API key for LLM provider")
	ollamaURL := flag.String("ollama-url", "", "Ollama server URL (default: http://localhost:11434)")
	noColor := flag.Bool("no-color", false, "Disable colored output")

	flag.Parse()

	// Show version
	if *showVersion {
		fmt.Printf("pgEdge Postgres MCP Chat Client v%s\n", version)
		return
	}

	// Load configuration
	cfg, err := chat.LoadConfig(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	// Override config with command line flags
	if *mcpMode != "" {
		cfg.MCP.Mode = *mcpMode
	}
	if *mcpURL != "" {
		cfg.MCP.URL = *mcpURL
	}
	if *mcpServerPath != "" {
		cfg.MCP.ServerPath = *mcpServerPath
	}
	if *llmProvider != "" {
		cfg.LLM.Provider = *llmProvider
	}
	if *llmModel != "" {
		cfg.LLM.Model = *llmModel
	}
	if *apiKey != "" {
		cfg.LLM.APIKey = *apiKey
	}
	if *ollamaURL != "" {
		cfg.LLM.OllamaURL = *ollamaURL
	}
	if *noColor {
		cfg.UI.NoColor = true
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		os.Exit(1)
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\n\nReceived interrupt signal. Shutting down...")
		cancel()
	}()

	// Create and run chat client
	client, err := chat.NewClient(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating chat client: %v\n", err)
		os.Exit(1)
	}

	if err := client.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error running chat client: %v\n", err)
		os.Exit(1)
	}
}
