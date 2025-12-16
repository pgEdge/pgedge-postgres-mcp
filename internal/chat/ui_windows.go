//go:build windows

package chat

import (
	"context"
	"time"
)

// ListenForEscape stub for Windows - Escape key cancellation not supported
func ListenForEscape(ctx context.Context, done <-chan struct{}, cancel context.CancelFunc) {
	// Windows doesn't support the Escape key cancellation feature
	// Just wait for done or context cancellation
	select {
	case <-done:
		return
	case <-ctx.Done():
		return
	}
}

// stdinHasData checks if stdin has data available within the timeout
// Windows implementation - returns false (not supported on Windows)
func stdinHasData(fd int, timeout time.Duration) bool {
	// Windows doesn't support select() syscall on stdin
	// The Escape key feature is not available on Windows
	// Return false to avoid blocking
	_ = fd
	_ = timeout
	return false
}
