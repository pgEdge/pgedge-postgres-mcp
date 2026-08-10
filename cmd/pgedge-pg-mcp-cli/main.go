/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Lanaguge Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
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

func main() {
	// Command line flags
	configFile := flag.String("config", "", "Path to configuration file")
	showVersion := flag.Bool("version", false, "Show version and exit")
	mcpMode := flag.String("mcp-mode", "", "MCP connection mode: stdio or http (default: stdio)")
	mcpURL := flag.String("mcp-url", "", "MCP server URL (for HTTP mode)")
	mcpServerPath := flag.String("mcp-server-path", "", "Path to MCP server binary (for stdio mode)")
	mcpServerConfig := flag.String("mcp-server-config", "", "Path to MCP server config file (for stdio mode)")
	mcpAuthMode := flag.String("mcp-auth-mode", "", "MCP authentication mode: none, token, or user (default: user)")
	mcpToken := flag.String("mcp-token", "", "MCP server authentication token (for token mode)")
	mcpUsername := flag.String("mcp-username", "", "MCP server username (for user mode)")
	mcpPassword := flag.String("mcp-password", "", "MCP server password (for user mode)")
	llmProvider := flag.String("llm-provider", "", "LLM provider: anthropic, openai, gemini, or ollama (default: anthropic)")
	llmModel := flag.String("llm-model", "", "LLM model to use")
	anthropicAPIKey := flag.String("anthropic-api-key", "", "API key for Anthropic")
	openaiAPIKey := flag.String("openai-api-key", "", "API key for OpenAI")
	geminiAPIKey := flag.String("gemini-api-key", "", "API key for Google Gemini")
	geminiAPIKeyFile := flag.String("gemini-api-key-file", "", "Path to a file containing the Google Gemini API key")
	ollamaURL := flag.String("ollama-url", "", "Ollama server URL (default: http://localhost:11434)")
	noColor := flag.Bool("no-color", false, "Disable colored output")

	flag.Parse()

	// Show version
	if *showVersion {
		fmt.Printf("pgEdge NLA CLI v%s\n", chat.ClientVersion)
		return
	}

	// Load configuration
	cfg, err := chat.LoadConfig(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	// Track which flags were explicitly set
	overrides := &chat.ConfigOverrides{
		ProviderSet: (*llmProvider != ""),
		ModelSet:    (*llmModel != ""),
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
	if *mcpServerConfig != "" {
		cfg.MCP.ServerConfigPath = *mcpServerConfig
	}
	if *mcpAuthMode != "" {
		cfg.MCP.AuthMode = *mcpAuthMode
	}
	if *mcpToken != "" {
		cfg.MCP.Token = *mcpToken
	}
	if *mcpUsername != "" {
		cfg.MCP.Username = *mcpUsername
	}
	if *mcpPassword != "" {
		cfg.MCP.Password = *mcpPassword
	}
	if *llmProvider != "" {
		cfg.LLM.Provider = *llmProvider
	}
	if *llmModel != "" {
		cfg.LLM.Model = *llmModel
	}
	if *anthropicAPIKey != "" {
		cfg.LLM.AnthropicAPIKey = *anthropicAPIKey
	}
	if *openaiAPIKey != "" {
		cfg.LLM.OpenAIAPIKey = *openaiAPIKey
	}
	// The key file is read here rather than by the loader, because the loader
	// resolves the files named in the configuration before any flag is seen.
	// A key given directly on the command line wins over one in a file.
	if *geminiAPIKeyFile != "" {
		cfg.LLM.GeminiAPIKeyFile = *geminiAPIKeyFile
		key, err := chat.ReadAPIKeyFile(*geminiAPIKeyFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading Gemini API key file: %v\n", err)
			os.Exit(1)
		}
		if key == "" {
			fmt.Fprintf(os.Stderr, "Gemini API key file %s is missing or empty\n", *geminiAPIKeyFile)
			os.Exit(1)
		}
		cfg.LLM.GeminiAPIKey = key
	}
	if *geminiAPIKey != "" {
		cfg.LLM.GeminiAPIKey = *geminiAPIKey
	}
	if *ollamaURL != "" {
		cfg.LLM.OllamaURL = *ollamaURL
	}
	if *noColor {
		cfg.UI.NoColor = true
	}

	// Re-register the credentials for redaction, since the flags above may
	// have supplied keys that LoadConfig never saw.
	cfg.RegisterSecrets()

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
	client, err := chat.NewClient(cfg, overrides)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating chat client: %v\n", err)
		os.Exit(1)
	}

	// Save preferences on exit (normal or interrupted)
	defer func() {
		if err := client.SavePreferences(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to save preferences: %v\n", err)
		}
	}()

	if err := client.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error running chat client: %v\n", err)
		os.Exit(1)
	}
}
