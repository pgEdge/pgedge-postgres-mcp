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
	startTime := time.Now()

	// Check if image exists locally first
	fmt.Printf("🔍 Checking for image %s...\n", c.image)
	_, _, err := c.cli.ImageInspectWithRaw(ctx, c.image)
	if err != nil {
		// Image doesn't exist locally, pull it
		fmt.Printf("⬇️  Pulling image %s...\n", c.image)
		pullStart := time.Now()
		reader, err := c.cli.ImagePull(ctx, c.image, types.ImagePullOptions{})
		if err != nil {
			return fmt.Errorf("failed to pull image: %w", err)
		}
		io.Copy(io.Discard, reader)
		reader.Close()
		pullDuration := time.Since(pullStart)
		fmt.Printf("   ✓ Image pull completed in %.1fs\n", pullDuration.Seconds())
	} else {
		fmt.Printf("   ✓ Image found in local cache\n")
	}

	// For systemd mode, we need a two-stage process:
	// 1. Start container with bash to check/install systemd
	// 2. Stop and restart with /sbin/init or /lib/systemd/systemd
	if c.systemd {
		err := c.startWithSystemd(ctx)
		totalDuration := time.Since(startTime)
		fmt.Printf("⏱️  Total container startup time: %.1fs\n\n", totalDuration.Seconds())
		return err
	}

	// Regular container with sleep
	cmd := []string{"/bin/bash", "-c", "sleep infinity"}

	// Create container
	resp, err := c.cli.ContainerCreate(ctx,
		&container.Config{
			Image: c.image,
			Cmd:   cmd,
			Tty:   true,
		},
		&container.HostConfig{
			Privileged: true, // For package installation
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

	time.Sleep(2 * time.Second)
	return nil
}

// startWithSystemd handles the special case of systemd-enabled containers
func (c *ContainerExecutor) startWithSystemd(ctx context.Context) error {
	// Mount cgroup for systemd (WRITABLE for systemd to work properly)
	mounts := []mount.Mount{
		{
			Type:     mount.TypeBind,
			Source:   "/sys/fs/cgroup",
			Target:   "/sys/fs/cgroup",
			ReadOnly: false, // Must be writable for systemd
		},
	}

	// First, start with bash to check and install systemd if needed
	fmt.Printf("📦 Creating temporary container...\n")
	createStart := time.Now()
	resp, err := c.cli.ContainerCreate(ctx,
		&container.Config{
			Image: c.image,
			Cmd:   []string{"/bin/bash", "-c", "sleep infinity"},
			Tty:   true,
		},
		&container.HostConfig{
			Privileged: true,
			Mounts:     mounts,
			Tmpfs: map[string]string{
				"/run":      "",
				"/run/lock": "",
			},
			// Add DNS servers for network resolution
			DNS: []string{"8.8.8.8", "8.8.4.4", "1.1.1.1"},
		},
		nil, nil, c.name)
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	c.containerID = resp.ID
	fmt.Printf("   ✓ Container created in %.1fs\n", time.Since(createStart).Seconds())

	// Start container
	startStart := time.Now()
	if err := c.cli.ContainerStart(ctx, c.containerID, types.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	time.Sleep(2 * time.Second)
	fmt.Printf("   ✓ Container started in %.1fs\n", time.Since(startStart).Seconds())

	fmt.Printf("🔍 Checking for systemd...\n")
	checkStart := time.Now()

	// Check if systemd is installed
	output, exitCode, _ := c.Exec(ctx, "test -f /sbin/init || test -f /usr/sbin/init || test -f /lib/systemd/systemd")
	systemdExists := (exitCode == 0)
	fmt.Printf("   ✓ systemd check completed in %.1fs\n", time.Since(checkStart).Seconds())

	if !systemdExists {
		fmt.Printf("⚠️  systemd not found - installing...\n")
		installStart := time.Now()

		// Install systemd based on OS
		// Detect OS type
		osOutput, _, err := c.Exec(ctx, "cat /etc/os-release")
		if err != nil {
			return fmt.Errorf("failed to detect OS: %w", err)
		}

		var installCmd string
		if strings.Contains(strings.ToLower(osOutput), "ubuntu") || strings.Contains(strings.ToLower(osOutput), "debian") {
			// Debian/Ubuntu - ultra-optimized for speed
			// Use fastest mirrors and minimal updates
			installCmd = `(
apt-get update -qq -o Acquire::http::Timeout=5 -o Acquire::Retries=1 || true
) && \
DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
-o Dpkg::Options::="--force-confdef" \
-o Dpkg::Options::="--force-confold" \
-o Acquire::http::Timeout=5 \
-o Acquire::Retries=1 \
systemd systemd-sysv 2>&1 | grep -v "^Get:" | grep -v "^Fetched" || true && \
apt-get clean && \
rm -rf /var/lib/apt/lists/*`
		} else {
			// RHEL/AlmaLinux/Rocky
			installCmd = "yum install -y systemd || dnf install -y systemd"
		}

		_, exitCode, err = c.Exec(ctx, installCmd)
		if err != nil || exitCode != 0 {
			return fmt.Errorf("failed to install systemd: %v (exit code: %d)", err, exitCode)
		}
		fmt.Printf("   ✓ systemd installed in %.1fs\n", time.Since(installStart).Seconds())
	} else {
		fmt.Printf("   ✓ systemd already present\n")
	}

	// Determine which init path is actually available in this container
	// Try multiple possible paths
	fmt.Printf("🔍 Detecting init binary path...\n")
	detectStart := time.Now()
	initPath := ""
	possiblePaths := []string{
		"/lib/systemd/systemd",
		"/usr/lib/systemd/systemd",
		"/sbin/init",
		"/usr/sbin/init",
	}

	for _, path := range possiblePaths {
		_, exitCode, _ := c.Exec(ctx, fmt.Sprintf("test -f %s", path))
		if exitCode == 0 {
			initPath = path
			break
		}
	}

	if initPath == "" {
		return fmt.Errorf("no systemd init binary found after installation")
	}
	fmt.Printf("   ✓ Found init at %s (%.1fs)\n", initPath, time.Since(detectStart).Seconds())

	// If systemd was installed, commit the container to preserve it
	var finalImage string
	if !systemdExists {
		fmt.Printf("📦 Committing container with systemd...\n")
		commitStart := time.Now()

		// Commit the container to create a new image with systemd installed
		commitResp, err := c.cli.ContainerCommit(ctx, c.containerID, types.ContainerCommitOptions{
			Reference: c.image + "-systemd",
		})
		if err != nil {
			return fmt.Errorf("failed to commit container with systemd: %w", err)
		}
		finalImage = commitResp.ID
		fmt.Printf("   ✓ Commit completed in %.1fs\n", time.Since(commitStart).Seconds())
	} else {
		// systemd already present, use original image directly
		finalImage = c.image
	}

	// Stop and remove temporary container
	stopStart := time.Now()
	// Use shorter timeout (2s) for temp container since it's just running sleep
	timeout := 2
	if err := c.cli.ContainerStop(ctx, c.containerID, container.StopOptions{Timeout: &timeout}); err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}

	// Remove the container
	if err := c.cli.ContainerRemove(ctx, c.containerID, types.ContainerRemoveOptions{}); err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}
	fmt.Printf("   ✓ Temp container cleaned up in %.1fs\n", time.Since(stopStart).Seconds())

	fmt.Printf("🚀 Starting container with systemd (using %s)...\n", initPath)
	finalStart := time.Now()

	// Create new container with systemd as init
	resp, err = c.cli.ContainerCreate(ctx,
		&container.Config{
			Image: finalImage,
			Cmd:   []string{initPath},
			Tty:   true,
			// Stop signal for systemd
			StopSignal: "SIGRTMIN+3",
		},
		&container.HostConfig{
			Privileged: true,
			Mounts:     mounts,
			Tmpfs: map[string]string{
				"/run":           "",
				"/run/lock":      "",
				"/tmp":           "",
				"/var/lib/journal": "",
			},
			// Add DNS servers for network resolution
			DNS: []string{"8.8.8.8", "8.8.4.4", "1.1.1.1"},
			// CgroupnsMode for systemd compatibility
			CgroupnsMode: "host",
		},
		nil, nil, c.name)
	if err != nil {
		return fmt.Errorf("failed to recreate container with systemd: %w", err)
	}

	c.containerID = resp.ID

	// Start container with systemd
	if err := c.cli.ContainerStart(ctx, c.containerID, types.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("failed to start container with systemd: %w", err)
	}
	fmt.Printf("   ✓ Final container started in %.1fs\n", time.Since(finalStart).Seconds())

	// Wait for systemd to initialize
	fmt.Printf("⏳ Waiting for systemd to initialize...\n")
	initStart := time.Now()
	time.Sleep(5 * time.Second)

	// Verify systemd is running
	output, exitCode, err = c.Exec(ctx, "systemctl --version")
	if err != nil || exitCode != 0 {
		return fmt.Errorf("systemd not running in container: %v (output: %s)", err, output)
	}
	fmt.Printf("   ✓ systemd initialized in %.1fs\n", time.Since(initStart).Seconds())

	fmt.Printf("✅ Container ready with systemd\n\n")

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
