/*-------------------------------------------------------------------------
 *
 * UI components for MCP Chat Client
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
	"math/rand"
	"os"
	"strings"
	"time"

	"golang.org/x/term"
)

// Color codes for terminal output
const (
	ColorReset   = "\033[0m"
	ColorRed     = "\033[31m"
	ColorGreen   = "\033[32m"
	ColorYellow  = "\033[33m"
	ColorBlue    = "\033[34m"
	ColorMagenta = "\033[35m"
	ColorCyan    = "\033[36m"
	ColorGray    = "\033[90m"
	ColorBold    = "\033[1m"
)

// UI handles the user interface
type UI struct {
	noColor bool
}

// NewUI creates a new UI instance
func NewUI(noColor bool) *UI {
	return &UI{noColor: noColor}
}

// colorize applies color if colors are enabled
func (ui *UI) colorize(color, text string) string {
	if ui.noColor {
		return text
	}
	return color + text + ColorReset
}

// PrintWelcome prints the welcome message
// ASCII art credit: https://ascii.co.uk/art/elephant
func (ui *UI) PrintWelcome() {
	elephant := `
          _
   ______/ \-.   _           pgEdge Postgres MCP Chat Client
.-/     (    o\_//           Type 'quit' or 'exit' to leave, 'help' for commands
 |  ___  \_/\---'
 |_||  |_||
`
	fmt.Println(ui.colorize(ColorCyan, elephant))
}

// GetPrompt returns the prompt string for readline
func (ui *UI) GetPrompt() string {
	return ui.colorize(ColorGreen+ColorBold, "You: ")
}

// PrintUserInput prints the user's input prompt (deprecated, kept for compatibility)
func (ui *UI) PrintUserInput() {
	fmt.Print(ui.GetPrompt())
	// Ensure the prompt is immediately visible
	_ = os.Stdout.Sync() //nolint:errcheck // Best effort flush, not critical
}

// PrintAssistantResponse prints the assistant's response
func (ui *UI) PrintAssistantResponse(text string) {
	// Clear the thinking animation line and add blank line before response
	maxWidth := ui.getThinkingMaxWidth()
	fmt.Print("\r" + strings.Repeat(" ", maxWidth) + "\r\n\n")
	fmt.Print(ui.colorize(ColorBlue, "Assistant: ") + text + "\n")
}

// PrintSystemMessage prints a system message
func (ui *UI) PrintSystemMessage(text string) {
	fmt.Println(ui.colorize(ColorYellow, "System: ") + text)
}

// PrintError prints an error message
func (ui *UI) PrintError(text string) {
	// Clear any thinking animation line and add blank line before error
	maxWidth := ui.getThinkingMaxWidth()
	fmt.Print("\r" + strings.Repeat(" ", maxWidth) + "\r\n\n")
	fmt.Println(ui.colorize(ColorRed, "Error: ") + text)
}

// PrintToolExecution prints a tool execution message on the same line as the thinking animation
func (ui *UI) PrintToolExecution(toolName string, params map[string]interface{}) {
	message := fmt.Sprintf(" → Executing tool: %s", toolName)

	// For read_resource, show the URI being accessed
	if toolName == "read_resource" {
		if uri, ok := params["uri"].(string); ok && uri != "" {
			message = fmt.Sprintf(" → Executing tool: %s (%s)", toolName, uri)
		}
	}

	fmt.Print(ui.colorize(ColorMagenta, message+"\n"))
}

// PrintSeparator prints a separator line
func (ui *UI) PrintSeparator() {
	fmt.Println(ui.colorize(ColorGray, strings.Repeat("─", 80)))
}

// PostgreSQL/Elephant themed action words for animation
var elephantActions = []string{
	"Thinking with trunks",
	"Consulting the herd",
	"Stampeding through data",
	"Trumpeting queries",
	"Migrating thoughts",
	"Packing memories",
	"Charging through logic",
	"Bathing in wisdom",
	"Roaming the database",
	"Grazing on metadata",
	"Herding ideas",
	"Splashing in pools",
	"Foraging for answers",
	"Wandering savannah",
	"Dusting off schemas",
	"Pondering profoundly",
	"Remembering everything",
	"Trumpeting brilliance",
	"Stomping bugs",
	"Tusking through code",
}

// getThinkingMaxWidth calculates the maximum width needed for thinking animation
func (ui *UI) getThinkingMaxWidth() int {
	maxWidth := 40
	for _, action := range elephantActions {
		width := len(action) + 5 // frame + space + action + "..."
		if width > maxWidth {
			maxWidth = width
		}
	}
	return maxWidth
}

// ShowThinking displays an animated "thinking" indicator
func (ui *UI) ShowThinking(ctx context.Context, done chan struct{}) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	frameIndex := 0
	actionIndex := rand.Intn(len(elephantActions))
	actionChangeCounter := 0

	maxWidth := ui.getThinkingMaxWidth()

	fmt.Print("\r" + ui.colorize(ColorCyan, frames[frameIndex]) + " " + ui.colorize(ColorGray, elephantActions[actionIndex]) + "...")

	for {
		select {
		case <-done:
			// Just stop animating, don't clear (let caller decide what to print next)
			return
		case <-ctx.Done():
			// Just stop animating, don't clear (let caller decide what to print next)
			return
		case <-ticker.C:
			frameIndex = (frameIndex + 1) % len(frames)
			actionChangeCounter++

			// Change action text every 4 ticks (2 seconds)
			if actionChangeCounter >= 4 {
				actionIndex = rand.Intn(len(elephantActions))
				actionChangeCounter = 0
			}

			// Build the message and pad to maxWidth to clear any leftover characters
			msg := ui.colorize(ColorCyan, frames[frameIndex]) + " " + ui.colorize(ColorGray, elephantActions[actionIndex]) + "..."
			// Add padding spaces after the message
			padding := maxWidth - len(elephantActions[actionIndex]) - 5
			if padding > 0 {
				msg += strings.Repeat(" ", padding)
			}
			fmt.Print("\r" + msg)
		}
	}
}

// PromptForToken prompts the user to enter an authentication token
func (ui *UI) PromptForToken() string {
	fmt.Print(ui.colorize(ColorYellow, "Enter MCP server authentication token: "))
	var token string
	_, _ = fmt.Scanln(&token) //nolint:errcheck // User input, errors not actionable
	return strings.TrimSpace(token)
}

// PromptForUsername prompts the user to enter a username
// Returns an error if the input is interrupted (e.g., Ctrl+C)
func (ui *UI) PromptForUsername(ctx context.Context) (string, error) {
	fmt.Print(ui.colorize(ColorYellow, "Username: "))

	// Use a channel to get the result from the blocking read
	type result struct {
		username string
		err      error
	}
	resultChan := make(chan result, 1)

	go func() {
		var username string
		_, err := fmt.Scanln(&username)
		resultChan <- result{username: strings.TrimSpace(username), err: err}
	}()

	// Wait for either the input or context cancellation
	select {
	case <-ctx.Done():
		fmt.Println() // Ensure newline after cancellation
		return "", ctx.Err()
	case res := <-resultChan:
		if res.err != nil {
			fmt.Println() // Ensure newline after error
			return "", res.err
		}
		return res.username, nil
	}
}

// PromptForPassword prompts the user to enter a password (hidden input)
// Returns an error if the input is interrupted (e.g., Ctrl+C)
func (ui *UI) PromptForPassword(ctx context.Context) (string, error) {
	fmt.Print(ui.colorize(ColorYellow, "Password: "))

	// Use a channel to get the result from the blocking read
	type result struct {
		password string
		err      error
	}
	resultChan := make(chan result, 1)

	go func() {
		password, err := term.ReadPassword(int(os.Stdin.Fd()))
		resultChan <- result{password: strings.TrimSpace(string(password)), err: err}
	}()

	// Wait for either the input or context cancellation
	select {
	case <-ctx.Done():
		fmt.Println() // Print newline after cancellation
		return "", ctx.Err()
	case res := <-resultChan:
		fmt.Println() // Print newline after password input
		if res.err != nil {
			return "", res.err
		}
		return res.password, nil
	}
}

// PrintHelp prints the help message
func (ui *UI) PrintHelp() {
	help := `
Available commands:
  help     - Show this help message
  quit     - Exit the chat client
  exit     - Exit the chat client
  clear    - Clear the screen
  tools    - List available MCP tools
  resources - List available MCP resources

You can ask questions naturally, and the assistant will use available tools and resources to help you.
`
	fmt.Println(ui.colorize(ColorCyan, help))
}

// ClearScreen clears the terminal screen
func (ui *UI) ClearScreen() {
	fmt.Print("\033[H\033[2J")
}
