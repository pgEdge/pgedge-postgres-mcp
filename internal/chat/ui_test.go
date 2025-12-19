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
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func TestUI_Colorize_WithColor(t *testing.T) {
	ui := NewUI(false, false) // Enable colors, disable markdown

	colored := ui.colorize(ColorRed, "test")
	expected := ColorRed + "test" + ColorReset

	if colored != expected {
		t.Errorf("Expected '%s', got '%s'", expected, colored)
	}
}

func TestUI_Colorize_NoColor(t *testing.T) {
	ui := NewUI(true, false) // Disable colors

	colored := ui.colorize(ColorRed, "test")

	if colored != "test" {
		t.Errorf("Expected 'test', got '%s'", colored)
	}
}

func TestUI_GetPrompt_WithColor(t *testing.T) {
	ui := NewUI(false, false)

	prompt := ui.GetPrompt()
	expected := ColorGreen + ColorBold + "You: " + ColorReset

	if prompt != expected {
		t.Errorf("Expected '%s', got '%s'", expected, prompt)
	}
}

func TestUI_GetPrompt_NoColor(t *testing.T) {
	ui := NewUI(true, false)

	prompt := ui.GetPrompt()

	if prompt != "You: " {
		t.Errorf("Expected 'You: ', got '%s'", prompt)
	}
}

func TestUI_PrintWelcome(t *testing.T) {
	ui := NewUI(true, false)

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	ui.PrintWelcome("1.0.0-test", "1.0.0-server")

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Check for key elements in welcome message
	if !strings.Contains(output, "pgEdge Natural Language Agent") {
		t.Error("Welcome message should contain 'pgEdge Natural Language Agent'")
	}

	if !strings.Contains(output, "quit") {
		t.Error("Welcome message should mention 'quit' command")
	}

	if !strings.Contains(output, "1.0.0-test") {
		t.Error("Welcome message should contain client version")
	}

	if !strings.Contains(output, "1.0.0-server") {
		t.Error("Welcome message should contain server version")
	}
}

func TestUI_PrintAssistantResponse(t *testing.T) {
	ui := NewUI(true, false)

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	ui.PrintAssistantResponse("Test response")

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Check for assistant label and content
	if !strings.Contains(output, "Assistant:") {
		t.Error("Output should contain 'Assistant:'")
	}

	if !strings.Contains(output, "Test response") {
		t.Error("Output should contain 'Test response'")
	}
}

func TestUI_PrintSystemMessage(t *testing.T) {
	ui := NewUI(true, false)

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	ui.PrintSystemMessage("Test system message")

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Check for system label and content
	if !strings.Contains(output, "System:") {
		t.Error("Output should contain 'System:'")
	}

	if !strings.Contains(output, "Test system message") {
		t.Error("Output should contain 'Test system message'")
	}
}

func TestUI_PrintError(t *testing.T) {
	ui := NewUI(true, false)

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	ui.PrintError("Test error message")

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Check for error label and content
	if !strings.Contains(output, "Error:") {
		t.Error("Output should contain 'Error:'")
	}

	if !strings.Contains(output, "Test error message") {
		t.Error("Output should contain 'Test error message'")
	}
}

func TestUI_PrintToolExecution(t *testing.T) {
	ui := NewUI(true, false)

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	ui.PrintToolExecution("test_tool", map[string]interface{}{})

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Check for tool execution message
	if !strings.Contains(output, "Executing tool:") {
		t.Error("Output should contain 'Executing tool:'")
	}

	if !strings.Contains(output, "test_tool") {
		t.Error("Output should contain 'test_tool'")
	}
}

func TestUI_PrintToolExecution_WithURI(t *testing.T) {
	ui := NewUI(true, false)

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	ui.PrintToolExecution("read_resource", map[string]interface{}{
		"uri": "pg://system_info",
	})

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Check for tool execution message with URI
	if !strings.Contains(output, "Executing tool:") {
		t.Error("Output should contain 'Executing tool:'")
	}

	if !strings.Contains(output, "read_resource") {
		t.Error("Output should contain 'read_resource'")
	}

	if !strings.Contains(output, "pg://system_info") {
		t.Error("Output should contain 'pg://system_info'")
	}
}

func TestUI_PrintSeparator(t *testing.T) {
	ui := NewUI(true, false)

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	ui.PrintSeparator()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Check for separator line (should contain dashes/lines)
	if !strings.Contains(output, "─") {
		t.Error("Separator should contain line characters")
	}
}

func TestUI_ShowThinking(t *testing.T) {
	ui := NewUI(true, false)

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	ctx := context.Background()
	done := make(chan struct{})

	// Start the thinking animation
	go ui.ShowThinking(ctx, done)

	// Let it run for a short time
	time.Sleep(100 * time.Millisecond)

	// Stop the animation
	close(done)

	// Give it time to stop
	time.Sleep(50 * time.Millisecond)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// The output should contain some of the thinking frames or actions
	// We just verify something was printed
	if len(output) == 0 {
		t.Error("Expected some output from thinking animation")
	}
}

func TestUI_ShowThinking_ContextCancellation(t *testing.T) {
	ui := NewUI(true, false)

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	// Start the thinking animation
	go ui.ShowThinking(ctx, done)

	// Let it run for a short time
	time.Sleep(100 * time.Millisecond)

	// Cancel the context
	cancel()

	// Give it time to stop
	time.Sleep(50 * time.Millisecond)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)

	// The animation should have stopped - we just verify it doesn't hang
	// If we get here, the test passes
}

func TestUI_PrintHelp(t *testing.T) {
	ui := NewUI(true, false)

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	ui.PrintHelp()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Check for key help commands
	commands := []string{"help", "quit", "exit", "clear", "tools", "resources"}
	for _, cmd := range commands {
		if !strings.Contains(output, cmd) {
			t.Errorf("Help output should contain '%s' command", cmd)
		}
	}
}

func TestUI_ClearScreen(t *testing.T) {
	ui := NewUI(true, false)

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	ui.ClearScreen()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Clear screen should output ANSI escape codes
	if !strings.Contains(output, "\033") {
		t.Error("ClearScreen should output ANSI escape codes")
	}
}

func TestUI_GetThinkingMaxWidth(t *testing.T) {
	ui := NewUI(true, false)

	maxWidth := ui.getThinkingMaxWidth()

	// Verify it's a reasonable width
	if maxWidth < 40 {
		t.Errorf("Expected max width to be at least 40, got %d", maxWidth)
	}

	// Verify it's at least as wide as the longest action
	for _, action := range elephantActions {
		expectedWidth := len(action) + 5 // frame + space + action + "..."
		if maxWidth < expectedWidth {
			t.Errorf("Max width %d is less than required for action '%s' (%d)", maxWidth, action, expectedWidth)
		}
	}
}

func TestUI_PromptForToken(t *testing.T) {
	ui := NewUI(true, false)

	// This test is tricky because it reads from stdin
	// We'll simulate input by providing a fake stdin
	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r

	// Write test token to the pipe
	go func() {
		w.Write([]byte("test-token-123\n"))
		w.Close()
	}()

	// Capture stdout
	oldStdout := os.Stdout
	rOut, wOut, _ := os.Pipe()
	os.Stdout = wOut

	token := ui.PromptForToken()

	wOut.Close()
	os.Stdout = oldStdout
	os.Stdin = oldStdin

	var buf bytes.Buffer
	io.Copy(&buf, rOut)

	// Verify the token was read
	if token != "test-token-123" {
		t.Errorf("Expected token 'test-token-123', got '%s'", token)
	}

	// Verify prompt was displayed
	output := buf.String()
	if !strings.Contains(output, "token") {
		t.Error("Prompt should mention 'token'")
	}
}

func TestUI_ElephantActions(t *testing.T) {
	// Verify elephant actions list is populated
	if len(elephantActions) == 0 {
		t.Error("elephantActions should not be empty")
	}

	// Verify all actions are non-empty strings
	for i, action := range elephantActions {
		if action == "" {
			t.Errorf("elephantActions[%d] should not be empty", i)
		}
	}

	// Verify some expected elephant-themed words are present
	allActions := strings.Join(elephantActions, " ")
	elephantWords := []string{"trunk", "herd", "elephant"}

	foundAny := false
	for _, word := range elephantWords {
		if strings.Contains(strings.ToLower(allActions), strings.ToLower(word)) {
			foundAny = true
			break
		}
	}

	if !foundAny {
		t.Error("Expected some elephant-themed words in actions")
	}
}

func TestUI_ColorConstants(t *testing.T) {
	// Verify color constants are defined
	colors := map[string]string{
		"Reset":   ColorReset,
		"Red":     ColorRed,
		"Green":   ColorGreen,
		"Yellow":  ColorYellow,
		"Blue":    ColorBlue,
		"Magenta": ColorMagenta,
		"Cyan":    ColorCyan,
		"Gray":    ColorGray,
		"Bold":    ColorBold,
	}

	for name, color := range colors {
		if color == "" {
			t.Errorf("Color constant %s should not be empty", name)
		}

		// All color codes should start with ANSI escape sequence
		if !strings.HasPrefix(color, "\033[") {
			t.Errorf("Color constant %s should start with ANSI escape sequence", name)
		}
	}
}

func TestUI_PrintUserInput(t *testing.T) {
	ui := NewUI(true, false)

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	ui.PrintUserInput()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Should print the prompt
	if !strings.Contains(output, "You:") {
		t.Error("PrintUserInput should output 'You:' prompt")
	}
}

func TestUI_SetNoColor(t *testing.T) {
	tests := []struct {
		name           string
		initialNoColor bool
		setNoColor     bool
		expectNoColor  bool
	}{
		{
			name:           "enable colors (set noColor to false)",
			initialNoColor: true,
			setNoColor:     false,
			expectNoColor:  false,
		},
		{
			name:           "disable colors (set noColor to true)",
			initialNoColor: false,
			setNoColor:     true,
			expectNoColor:  true,
		},
		{
			name:           "keep colors disabled",
			initialNoColor: true,
			setNoColor:     true,
			expectNoColor:  true,
		},
		{
			name:           "keep colors enabled",
			initialNoColor: false,
			setNoColor:     false,
			expectNoColor:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ui := NewUI(tt.initialNoColor, false)

			// Verify initial state
			if ui.IsNoColor() != tt.initialNoColor {
				t.Errorf("Initial noColor state expected %v, got %v", tt.initialNoColor, ui.IsNoColor())
			}

			// Change the state
			ui.SetNoColor(tt.setNoColor)

			// Verify new state
			if ui.IsNoColor() != tt.expectNoColor {
				t.Errorf("After SetNoColor(%v), expected IsNoColor() = %v, got %v",
					tt.setNoColor, tt.expectNoColor, ui.IsNoColor())
			}
		})
	}
}

func TestUI_IsNoColor(t *testing.T) {
	// Test with colors enabled (noColor = false)
	uiWithColor := NewUI(false, false)
	if uiWithColor.IsNoColor() {
		t.Error("IsNoColor() should return false when colors are enabled")
	}

	// Test with colors disabled (noColor = true)
	uiNoColor := NewUI(true, false)
	if !uiNoColor.IsNoColor() {
		t.Error("IsNoColor() should return true when colors are disabled")
	}
}

func TestUI_SetNoColor_AffectsOutput(t *testing.T) {
	ui := NewUI(false, false) // Start with colors enabled

	// Verify colored output
	coloredOutput := ui.colorize(ColorRed, "test")
	if coloredOutput == "test" {
		t.Error("With colors enabled, colorize should add color codes")
	}
	if !strings.Contains(coloredOutput, ColorRed) {
		t.Error("Colored output should contain ColorRed code")
	}

	// Disable colors
	ui.SetNoColor(true)

	// Verify plain output
	plainOutput := ui.colorize(ColorRed, "test")
	if plainOutput != "test" {
		t.Errorf("With colors disabled, colorize should return plain text, got '%s'", plainOutput)
	}

	// Re-enable colors
	ui.SetNoColor(false)

	// Verify colored output again
	coloredAgain := ui.colorize(ColorRed, "test")
	if coloredAgain == "test" {
		t.Error("After re-enabling colors, colorize should add color codes")
	}
}
