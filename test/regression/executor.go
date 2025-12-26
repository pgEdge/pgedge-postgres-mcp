package regression

import (
	"context"
	"fmt"
)

// ExecutionMode defines how tests should run
type ExecutionMode int

const (
	// ModeContainerSystemd runs tests in a systemd-enabled container
	ModeContainerSystemd ExecutionMode = iota
	// ModeLocal runs tests on the local machine
	ModeLocal
)

// String returns the string representation of the execution mode
func (m ExecutionMode) String() string {
	switch m {
	case ModeContainerSystemd:
		return "container-systemd"
	case ModeLocal:
		return "local"
	default:
		return "unknown"
	}
}

// Executor defines the interface for running commands
type Executor interface {
	// Start initializes the executor
	Start(ctx context.Context) error

	// Exec runs a command and returns output, exit code, and error
	Exec(ctx context.Context, cmd string) (string, int, error)

	// Cleanup performs cleanup operations
	Cleanup(ctx context.Context) error

	// GetLogs retrieves logs (for debugging)
	GetLogs(ctx context.Context) (string, error)

	// Mode returns the execution mode
	Mode() ExecutionMode
}

// NewExecutor creates an executor based on the execution mode
func NewExecutor(mode ExecutionMode, osImage string) (Executor, error) {
	switch mode {
	case ModeContainerSystemd:
		return NewContainerExecutor(osImage, true)
	case ModeLocal:
		return NewLocalExecutor()
	default:
		return nil, fmt.Errorf("unsupported execution mode: %v", mode)
	}
}
