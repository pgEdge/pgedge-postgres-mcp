package regression

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

// ContainerExecutor runs tests in a Docker container
type ContainerExecutor struct {
	cli         *client.Client
	containerID string
	image       string
	name        string
	systemd     bool
	mode        ExecutionMode
}

// NewContainerExecutor creates a new container-based executor
func NewContainerExecutor(image string, enableSystemd bool) (*ContainerExecutor, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	return &ContainerExecutor{
		cli:     cli,
		image:   image,
		name:    fmt.Sprintf("mcp-test-%d", time.Now().Unix()),
		systemd: enableSystemd,
		mode:    ModeContainerSystemd,
	}, nil
}

// Start pulls image and starts container
func (c *ContainerExecutor) Start(ctx context.Context) error {
	// Pull image
	reader, err := c.cli.ImagePull(ctx, c.image, types.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}
	io.Copy(io.Discard, reader)
	reader.Close()

	// Configure container based on systemd requirement
	var cmd []string
	var mounts []mount.Mount

	if c.systemd {
		// Systemd-enabled container configuration
		// Use systemd as init process
		cmd = []string{"/sbin/init"}

		// Mount cgroup for systemd (WRITABLE for systemd to work properly)
		mounts = []mount.Mount{
			{
				Type:     mount.TypeBind,
				Source:   "/sys/fs/cgroup",
				Target:   "/sys/fs/cgroup",
				ReadOnly: false, // Must be writable for systemd
			},
		}
	} else {
		// Regular container with sleep
		cmd = []string{"/bin/bash", "-c", "sleep infinity"}
	}

	// Create container
	resp, err := c.cli.ContainerCreate(ctx,
		&container.Config{
			Image: c.image,
			Cmd:   cmd,
			Tty:   true,
		},
		&container.HostConfig{
			Privileged: true, // For systemd and package installation
			Mounts:     mounts,
			Tmpfs: map[string]string{
				"/run":      "",
				"/run/lock": "",
			},
		},
		nil, nil, c.name)
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	c.containerID = resp.ID

	// Start container
	if err := c.cli.ContainerStart(ctx, c.containerID, types.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	// Wait for container to be ready
	if c.systemd {
		// Wait longer for systemd to initialize
		time.Sleep(5 * time.Second)

		// Verify systemd is running
		output, exitCode, err := c.Exec(ctx, "systemctl --version")
		if err != nil || exitCode != 0 {
			return fmt.Errorf("systemd not running in container: %v (output: %s)", err, output)
		}
	} else {
		time.Sleep(2 * time.Second)
	}

	return nil
}

// Exec runs a command in the container
func (c *ContainerExecutor) Exec(ctx context.Context, cmd string) (string, int, error) {
	execConfig := types.ExecConfig{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          []string{"/bin/bash", "-c", cmd},
	}

	execID, err := c.cli.ContainerExecCreate(ctx, c.containerID, execConfig)
	if err != nil {
		return "", 0, fmt.Errorf("failed to create exec: %w", err)
	}

	resp, err := c.cli.ContainerExecAttach(ctx, execID.ID, types.ExecStartCheck{})
	if err != nil {
		return "", 0, fmt.Errorf("failed to attach exec: %w", err)
	}
	defer resp.Close()

	// Docker multiplexes stdout and stderr with 8-byte headers
	// Use stdcopy to properly demultiplex the streams
	var stdout, stderr bytes.Buffer
	_, err = stdcopy.StdCopy(&stdout, &stderr, resp.Reader)
	if err != nil {
		return "", 0, fmt.Errorf("failed to read output: %w", err)
	}

	// Combine stdout and stderr
	output := stdout.String() + stderr.String()

	// Get exit code
	inspect, err := c.cli.ContainerExecInspect(ctx, execID.ID)
	if err != nil {
		return output, 0, err
	}

	return output, inspect.ExitCode, nil
}

// Cleanup stops and removes the container
func (c *ContainerExecutor) Cleanup(ctx context.Context) error {
	if c.containerID == "" {
		return nil
	}

	timeout := 5
	if c.systemd {
		// Give systemd more time to shutdown gracefully
		timeout = 10
	}

	if err := c.cli.ContainerStop(ctx, c.containerID, container.StopOptions{Timeout: &timeout}); err != nil {
		// Ignore stop errors
	}

	return c.cli.ContainerRemove(ctx, c.containerID, types.ContainerRemoveOptions{Force: true})
}

// GetLogs retrieves container logs
func (c *ContainerExecutor) GetLogs(ctx context.Context) (string, error) {
	if c.containerID == "" {
		return "", fmt.Errorf("no container running")
	}

	reader, err := c.cli.ContainerLogs(ctx, c.containerID, types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       "100",
	})
	if err != nil {
		return "", err
	}
	defer reader.Close()

	logs, err := io.ReadAll(reader)
	return string(logs), err
}

// Mode returns the execution mode
func (c *ContainerExecutor) Mode() ExecutionMode {
	return c.mode
}

// IsSystemdEnabled returns true if systemd is enabled for this container
func (c *ContainerExecutor) IsSystemdEnabled() bool {
	return c.systemd
}

// GetOSInfo returns OS information from the container
func (c *ContainerExecutor) GetOSInfo(ctx context.Context) (string, error) {
	output, exitCode, err := c.Exec(ctx, "cat /etc/os-release")
	if err != nil || exitCode != 0 {
		return "", fmt.Errorf("failed to get OS info: %v (exit code: %d)", err, exitCode)
	}

	// Parse output to get OS name
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			return strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), "\""), nil
		}
	}

	return strings.TrimSpace(output), nil
}
