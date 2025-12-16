//go:build !windows

package chat

import (
	"context"
	"os"
	"syscall"
	"time"

	"golang.org/x/term"
)

// ListenForEscape monitors stdin for the Escape key and signals via the cancel
// function when detected. The function returns when either Escape is pressed,
// the done channel is closed, or the context is canceled. It puts the terminal
// into raw mode temporarily to detect key presses.
//
// Arrow keys and other special keys send escape sequences (ESC + more bytes),
// so we need to distinguish between a standalone Escape and escape sequences.
// We do this by waiting briefly after seeing ESC to check for following bytes.
func ListenForEscape(ctx context.Context, done <-chan struct{}, cancel context.CancelFunc) {
	// Save the current terminal state
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		// Can't enter raw mode - just return without key detection
		return
	}

	// Restore terminal state when done
	defer func() {
		_ = term.Restore(fd, oldState) //nolint:errcheck // Best effort restore
	}()

	buf := make([]byte, 1)

	// Poll for input using select() syscall with timeout
	// This avoids blocking reads that can't be interrupted
	// Use a short timeout (20ms) to respond quickly when done channel is closed
	for {
		// Check if we should stop
		select {
		case <-done:
			return
		case <-ctx.Done():
			return
		default:
		}

		// Use select() syscall to check if stdin has data (with 20ms timeout)
		// Short timeout ensures we respond quickly to done channel
		if !stdinHasData(fd, 20*time.Millisecond) {
			continue
		}

		// Data is available - read it
		n, err := syscall.Read(fd, buf)
		if err != nil || n == 0 {
			continue
		}

		if buf[0] == KeyEscape {
			// Got ESC - check if more bytes follow (escape sequence)
			// Wait briefly and check for following bytes
			if stdinHasData(fd, 50*time.Millisecond) {
				// More bytes followed - this is an escape sequence, consume them
				// Read up to 5 more bytes to consume the sequence
				seqBuf := make([]byte, 5)
				_, _ = syscall.Read(fd, seqBuf) //nolint:errcheck // Best effort consume
				// Continue listening - don't cancel
				continue
			}
			// No more bytes - this is a standalone Escape
			cancel()
			return
		}
		// Ignore other keys while waiting for LLM
	}
}
