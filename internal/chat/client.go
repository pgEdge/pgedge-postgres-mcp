/*-------------------------------------------------------------------------
 *
 * Chat Client - Main agentic chat loop
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package chat

import (
    "context"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "strings"

    "github.com/chzyer/readline"
    "pgedge-postgres-mcp/internal/mcp"
)

// Client is the main chat client
type Client struct {
    config   *Config
    ui       *UI
    mcp      MCPClient
    llm      LLMClient
    messages []Message
    tools    []mcp.Tool
}

// NewClient creates a new chat client
func NewClient(cfg *Config) (*Client, error) {
    return &Client{
        config:   cfg,
        ui:       NewUI(cfg.UI.NoColor),
        messages: []Message{},
    }, nil
}

// Run starts the chat client
func (c *Client) Run(ctx context.Context) error {
    // Connect to MCP server
    if err := c.connectToMCP(ctx); err != nil {
        return fmt.Errorf("failed to connect to MCP server: %w", err)
    }
    defer c.mcp.Close()

    // Initialize MCP connection
    if err := c.mcp.Initialize(ctx); err != nil {
        return fmt.Errorf("failed to initialize MCP connection: %w", err)
    }

    // Get available tools
    tools, err := c.mcp.ListTools(ctx)
    if err != nil {
        return fmt.Errorf("failed to list tools: %w", err)
    }
    c.tools = tools

    // Initialize LLM client
    if err := c.initializeLLM(); err != nil {
        return fmt.Errorf("failed to initialize LLM: %w", err)
    }

    // Print welcome message
    c.ui.PrintWelcome()
    c.ui.PrintSystemMessage(fmt.Sprintf("Connected to MCP server (%d tools available)", len(c.tools)))
    c.ui.PrintSystemMessage(fmt.Sprintf("Using LLM: %s (%s)", c.config.LLM.Provider, c.config.LLM.Model))
    c.ui.PrintSeparator()

    // Start chat loop
    return c.chatLoop(ctx)
}

// connectToMCP establishes connection to the MCP server
func (c *Client) connectToMCP(ctx context.Context) error {
    if c.config.MCP.Mode == "http" {
        // HTTP mode
        token := c.config.MCP.Token
        if token == "" {
            // Prompt for token
            token = c.ui.PromptForToken()
            if token == "" {
                return fmt.Errorf("authentication token is required for HTTP mode")
            }
        }

        url := c.config.MCP.URL
        if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
            if c.config.MCP.TLS {
                url = "https://" + url
            } else {
                url = "http://" + url
            }
        }

        // Ensure URL ends with /mcp/v1
        if !strings.HasSuffix(url, "/mcp/v1") {
            if strings.HasSuffix(url, "/") {
                url += "mcp/v1"
            } else {
                url += "/mcp/v1"
            }
        }

        c.mcp = NewHTTPClient(url, token)
    } else {
        // Stdio mode
        mcpClient, err := NewStdioClient(c.config.MCP.ServerPath)
        if err != nil {
            return err
        }
        c.mcp = mcpClient
    }

    return nil
}

// initializeLLM creates the LLM client
func (c *Client) initializeLLM() error {
    if c.config.LLM.Provider == "anthropic" {
        c.llm = NewAnthropicClient(
            c.config.LLM.APIKey,
            c.config.LLM.Model,
            c.config.LLM.MaxTokens,
            c.config.LLM.Temperature,
        )
    } else if c.config.LLM.Provider == "ollama" {
        c.llm = NewOllamaClient(
            c.config.LLM.OllamaURL,
            c.config.LLM.Model,
        )
    } else {
        return fmt.Errorf("unsupported LLM provider: %s", c.config.LLM.Provider)
    }

    return nil
}

// chatLoop runs the interactive chat loop
func (c *Client) chatLoop(ctx context.Context) error {
    // Set up readline with history
    homeDir, err := os.UserHomeDir()
    if err != nil {
        return fmt.Errorf("failed to get home directory: %w", err)
    }
    historyFile := filepath.Join(homeDir, ".pgedge-postgres-mcp-chat-history")

    // Configure readline with custom prompt
    rl, err := readline.NewEx(&readline.Config{
        Prompt:                 c.ui.GetPrompt(),
        HistoryFile:            historyFile,
        HistoryLimit:           1000,
        DisableAutoSaveHistory: false,
        InterruptPrompt:        "^C",
        EOFPrompt:              "exit",
    })
    if err != nil {
        return fmt.Errorf("failed to initialize readline: %w", err)
    }
    defer rl.Close()

    // Monitor context cancellation in a goroutine
    go func() {
        <-ctx.Done()
        rl.Close() // Closing readline will cause Readline() to return an error
    }()

    // Main readline loop
    for {
        // This blocks until user provides input
        line, err := rl.Readline()

        if err != nil {
            // Handle various exit conditions
            if err == readline.ErrInterrupt || err == io.EOF {
                fmt.Println()
                c.ui.PrintSystemMessage("Goodbye!")
                return nil
            }
            // Check if context was canceled
            if ctx.Err() != nil {
                fmt.Println()
                c.ui.PrintSystemMessage("Goodbye!")
                return nil
            }
            return fmt.Errorf("readline error: %w", err)
        }

        userInput := strings.TrimSpace(line)
        if userInput == "" {
            continue
        }

        // Handle special commands
        if c.handleCommand(ctx, userInput) {
            continue
        }

        // Check for quit command
        if userInput == "quit" || userInput == "exit" {
            c.ui.PrintSystemMessage("Goodbye!")
            return nil
        }

        // Process the query
        if err := c.processQuery(ctx, userInput); err != nil {
            c.ui.PrintError(err.Error())
        }

        c.ui.PrintSeparator()
        // Readline will automatically display the prompt on the next iteration
    }
}

// handleCommand handles special commands, returns true if command was handled
func (c *Client) handleCommand(ctx context.Context, input string) bool {
    switch input {
    case "help":
        c.ui.PrintHelp()
        return true

    case "clear":
        c.ui.ClearScreen()
        c.ui.PrintWelcome()
        return true

    case "tools":
        c.ui.PrintSystemMessage(fmt.Sprintf("Available tools (%d):", len(c.tools)))
        for _, tool := range c.tools {
            fmt.Printf("  - %s: %s\n", tool.Name, tool.Description)
        }
        return true

    case "resources":
        resources, err := c.mcp.ListResources(ctx)
        if err != nil {
            c.ui.PrintError(fmt.Sprintf("Failed to list resources: %v", err))
            return true
        }
        c.ui.PrintSystemMessage(fmt.Sprintf("Available resources (%d):", len(resources)))
        for _, resource := range resources {
            fmt.Printf("  - %s: %s\n", resource.Name, resource.Description)
        }
        return true

    default:
        return false
    }
}

// processQuery processes a user query through the agentic loop
func (c *Client) processQuery(ctx context.Context, query string) error {
    // Add user message to conversation history
    c.messages = append(c.messages, Message{
        Role:    "user",
        Content: query,
    })

    // Start thinking animation
    thinkingDone := make(chan struct{})
    go c.ui.ShowThinking(ctx, thinkingDone)

    // Agentic loop (max 10 iterations to prevent infinite loops)
    for iteration := 0; iteration < 10; iteration++ {
        // Get response from LLM
        response, err := c.llm.Chat(ctx, c.messages, c.tools)
        if err != nil {
            close(thinkingDone)
            return fmt.Errorf("LLM error: %w", err)
        }

        // Check if LLM wants to use tools
        if response.StopReason == "tool_use" {
            // Extract tool uses and text content
            var toolUses []ToolUse
            var textParts []string

            for _, item := range response.Content {
                switch v := item.(type) {
                case ToolUse:
                    toolUses = append(toolUses, v)
                case TextContent:
                    _ = append(textParts, v.Text) // Not used in this context, just checking type
                }
            }

            // Add assistant's message to history
            c.messages = append(c.messages, Message{
                Role:    "assistant",
                Content: response.Content,
            })

            // Execute all tool calls
            toolResults := []ToolResult{}
            for _, toolUse := range toolUses {
                close(thinkingDone)
                c.ui.PrintToolExecution(toolUse.Name)
                thinkingDone = make(chan struct{})
                go c.ui.ShowThinking(ctx, thinkingDone)

                result, err := c.mcp.CallTool(ctx, toolUse.Name, toolUse.Input)
                if err != nil {
                    toolResults = append(toolResults, ToolResult{
                        Type:      "tool_result",
                        ToolUseID: toolUse.ID,
                        Content:   fmt.Sprintf("Error: %v", err),
                        IsError:   true,
                    })
                } else {
                    toolResults = append(toolResults, ToolResult{
                        Type:      "tool_result",
                        ToolUseID: toolUse.ID,
                        Content:   result.Content,
                        IsError:   result.IsError,
                    })
                }
            }

            // Add tool results to conversation
            c.messages = append(c.messages, Message{
                Role:    "user",
                Content: toolResults,
            })

            // Continue the loop to get final response
            continue
        }

        // Got final response
        close(thinkingDone)

        // Extract and display text content
        var textParts []string
        for _, item := range response.Content {
            if text, ok := item.(TextContent); ok {
                textParts = append(textParts, text.Text)
            }
        }

        finalText := strings.Join(textParts, "\n")
        c.ui.PrintAssistantResponse(finalText)

        // Add assistant's response to history
        c.messages = append(c.messages, Message{
            Role:    "assistant",
            Content: finalText,
        })

        return nil
    }

    close(thinkingDone)
    return fmt.Errorf("reached maximum number of tool calls (10)")
}
