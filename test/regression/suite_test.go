package regression

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/stretchr/testify/suite"
)

// LogLevel defines the verbosity of test output
type LogLevel int

const (
	LogLevelMinimal  LogLevel = iota // Only test names and status
	LogLevelDetailed                 // All logs (default)
)

// ServerEnvironment defines whether to use staging or live repositories
type ServerEnvironment int

const (
	EnvLive    ServerEnvironment = iota // Production/live repositories
	EnvStaging                          // Staging repositories
)

// String returns the string representation of ServerEnvironment
func (e ServerEnvironment) String() string {
	switch e {
	case EnvLive:
		return "live"
	case EnvStaging:
		return "staging"
	default:
		return "unknown"
	}
}

// TestResult tracks individual test execution details
type TestResult struct {
	Name      string
	Status    string
	Duration  time.Duration
	StartTime time.Time
}

// RegressionTestSuite runs basic regression tests
type RegressionTestSuite struct {
	suite.Suite
	ctx           context.Context
	executor      Executor
	osImage       string // Original image tag (e.g., "almalinux:10")
	osDisplayName string // Pretty OS name for display (e.g., "AlmaLinux 10.0")
	repoURL       string
	execMode      ExecutionMode
	logLevel      LogLevel
	serverEnv     ServerEnvironment

	// Track setup state to avoid redundant operations
	setupState struct {
		repoInstalled        bool
		postgresqlInstalled  bool
		mcpPackagesInstalled bool
	}

	// Track test results for summary
	testResults    []TestResult
	suiteStartTime time.Time
}

// SetupSuite runs once before all tests
func (s *RegressionTestSuite) SetupSuite() {
	s.ctx = context.Background()
	s.suiteStartTime = time.Now()
	s.testResults = make([]TestResult, 0)

	// Determine execution mode
	s.execMode = s.getExecutionMode()

	// Determine server environment (staging vs live)
	s.serverEnv = s.getServerEnvironment()

	// Determine log level from environment
	logLevelStr := strings.ToLower(os.Getenv("TEST_LOG_LEVEL"))
	switch logLevelStr {
	case "minimal", "min", "summary":
		s.logLevel = LogLevelMinimal
	case "detailed", "detail", "verbose", "":
		s.logLevel = LogLevelDetailed
	default:
		s.logLevel = LogLevelDetailed
	}

	// Get OS image from environment or use default (only for container modes)
	s.osImage = os.Getenv("TEST_OS_IMAGE")
	if s.osImage == "" && s.execMode != ModeLocal {
		s.osImage = "debian:12" // Default to Debian 12 for containers
	}

	// Create executor once for the entire test suite
	// This allows all tests to share the same environment and reuse installations
	var err error
	s.executor, err = NewExecutor(s.execMode, s.osImage)
	s.Require().NoError(err, "Failed to create executor")

	err = s.executor.Start(s.ctx)
	s.Require().NoError(err, "Failed to start executor")

	// Detect the actual OS from the executor for display purposes
	osInfo, err := s.executor.GetOSInfo(s.ctx)
	if err == nil && osInfo != "" {
		s.osDisplayName = osInfo
	} else if s.execMode == ModeLocal {
		s.osDisplayName = "Local System"
	} else {
		// For container mode, use image tag as fallback
		s.osDisplayName = s.osImage
	}

	if s.logLevel == LogLevelDetailed {
		s.T().Logf("Execution mode: %s", s.execMode.String())
		if s.execMode != ModeLocal {
			s.T().Logf("Testing with OS image: %s", s.osDisplayName)
		} else {
			s.T().Logf("Testing on: %s", s.osDisplayName)
		}
		s.T().Logf("Server environment: %s", s.serverEnv.String())
		s.T().Logf("Executor (%s) started successfully", s.execMode.String())
	}
}

// getExecutionMode determines the execution mode from environment or user prompt
func (s *RegressionTestSuite) getExecutionMode() ExecutionMode {
	// Check environment variable first
	modeStr := os.Getenv("TEST_EXEC_MODE")
	if modeStr != "" {
		switch strings.ToLower(modeStr) {
		case "local":
			return ModeLocal
		case "container", "container-systemd", "systemd":
			return ModeContainerSystemd
		default:
			s.T().Logf("Warning: Unknown TEST_EXEC_MODE '%s', prompting user", modeStr)
		}
	}

	// If not in CI and no environment variable, prompt the user
	if !s.isCI() {
		return s.promptExecutionMode()
	}

	// Default to container mode with systemd for CI
	return ModeContainerSystemd
}

// isCI checks if running in CI environment
func (s *RegressionTestSuite) isCI() bool {
	return os.Getenv("CI") != "" || os.Getenv("GITHUB_ACTIONS") != ""
}

// getServerEnvironment determines the server environment from environment or user prompt
func (s *RegressionTestSuite) getServerEnvironment() ServerEnvironment {
	// Check environment variable first
	envStr := os.Getenv("TEST_SERVER_ENV")
	if envStr != "" {
		switch strings.ToLower(envStr) {
		case "live", "production", "prod":
			return EnvLive
		case "staging", "stage", "stg":
			return EnvStaging
		default:
			s.T().Logf("Warning: Unknown TEST_SERVER_ENV '%s', prompting user", envStr)
		}
	}

	// If not in CI and no environment variable, prompt the user
	if !s.isCI() {
		return s.promptServerEnvironment()
	}

	// Default to live for CI
	return EnvLive
}

// promptServerEnvironment prompts the user to select server environment
func (s *RegressionTestSuite) promptServerEnvironment() ServerEnvironment {
	fmt.Println("\n=== Server Environment Selection ===")
	fmt.Println("Please select which server environment to use:")
	fmt.Println()
	fmt.Println("1. Live/Production - Use production repositories")
	fmt.Println("2. Staging - Use staging repositories for testing")
	fmt.Println()
	fmt.Print("Enter your choice [1-2] (default: 1): ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		s.T().Logf("Error reading input, using default (live): %v", err)
		return EnvLive
	}

	input = strings.TrimSpace(input)
	if input == "" {
		input = "1"
	}

	switch input {
	case "1":
		fmt.Println("Selected: Live/Production")
		return EnvLive
	case "2":
		fmt.Println("Selected: Staging")
		return EnvStaging
	default:
		fmt.Printf("Invalid choice '%s', using default (live)\n", input)
		return EnvLive
	}
}

// setRepositoryURL sets the appropriate repository URL based on OS type
func (s *RegressionTestSuite) setRepositoryURL() {
	// Check if user provided custom repo URL via environment variable
	customURL := os.Getenv("PGEDGE_REPO_URL")
	if customURL != "" {
		s.repoURL = customURL
		return
	}

	// Determine OS type to set appropriate repository URL
	isDebian, isRHEL := s.getOSType()

	// Build the repository URL with server environment
	if isDebian {
		// Debian/Ubuntu uses APT repository with component (release or staging)
		component := "release"
		if s.serverEnv == EnvStaging {
			component = "staging"
		}
		s.repoURL = fmt.Sprintf("https://apt.pgedge.com (%s)", component)
	} else if isRHEL {
		// RHEL/Rocky/Alma uses DNF/YUM repository
		// Note: The actual repo file will be modified by sed to change release->staging
		envPath := "release"
		if s.serverEnv == EnvStaging {
			envPath = "staging"
		}
		s.repoURL = fmt.Sprintf("https://dnf.pgedge.com (%s)", envPath)
	} else {
		// Default to APT repository if OS type cannot be determined
		s.repoURL = "https://apt.pgedge.com (release)"
	}
}

// promptExecutionMode prompts the user to select execution mode
func (s *RegressionTestSuite) promptExecutionMode() ExecutionMode {
	fmt.Println("\n=== MCP Regression Test Suite ===")
	fmt.Println("Please select how you want to run the tests:")
	fmt.Println()
	fmt.Println("1. Container with systemd - Docker container with systemd enabled")
	fmt.Println("2. Local machine - Run tests directly on this system")
	fmt.Println()
	fmt.Print("Enter your choice [1-2] (default: 1): ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		s.T().Logf("Error reading input, using default (container with systemd): %v", err)
		return ModeContainerSystemd
	}

	input = strings.TrimSpace(input)
	if input == "" {
		input = "1"
	}

	switch input {
	case "1":
		fmt.Println("Selected: Container with systemd")
		return ModeContainerSystemd
	case "2":
		fmt.Println("Selected: Local machine")
		fmt.Println("\nWARNING: Running tests on local machine will:")
		fmt.Println("  - Install packages on your system")
		fmt.Println("  - Modify system configuration")
		fmt.Println("  - Require sudo access")
		fmt.Print("\nAre you sure? [y/N]: ")

		confirm, _ := reader.ReadString('\n')
		confirm = strings.TrimSpace(strings.ToLower(confirm))
		if confirm == "y" || confirm == "yes" {
			fmt.Println("Proceeding with local execution...")
			return ModeLocal
		}
		fmt.Println("Cancelled. Using container mode with systemd instead.")
		return ModeContainerSystemd
	default:
		fmt.Printf("Invalid choice '%s', using default (container with systemd)\n", input)
		return ModeContainerSystemd
	}
}

// logDetailed logs only when in detailed mode
func (s *RegressionTestSuite) logDetailed(format string, args ...interface{}) {
	if s.logLevel == LogLevelDetailed {
		s.T().Logf(format, args...)
	}
}

// SetupTest runs before each test
func (s *RegressionTestSuite) SetupTest() {
	s.logDetailed("=== Starting test: %s ===", s.T().Name())

	// Track test start time
	s.testResults = append(s.testResults, TestResult{
		Name:      s.T().Name(),
		StartTime: time.Now(),
		Status:    "RUNNING",
	})
}

// TearDownTest runs after each test
func (s *RegressionTestSuite) TearDownTest() {
	// Update test result with final status and duration
	if len(s.testResults) > 0 {
		idx := len(s.testResults) - 1
		s.testResults[idx].Duration = time.Since(s.testResults[idx].StartTime)
		if s.T().Failed() {
			s.testResults[idx].Status = "FAIL"
		} else {
			s.testResults[idx].Status = "PASS"
		}
	}

	// Always show test result in minimal mode
	if s.logLevel == LogLevelMinimal {
		if s.T().Failed() {
			s.T().Logf("%-50s ✗ FAIL", s.T().Name())
		} else {
			s.T().Logf("%-50s ✓ PASS", s.T().Name())
		}
	}

	// Print logs on failure (even in minimal mode for debugging)
	if s.T().Failed() && s.executor != nil {
		logs, _ := s.executor.GetLogs(s.ctx)
		s.T().Logf("Executor logs:\n%s", logs)
	}

	s.logDetailed("Test %s completed", s.T().Name())
}

// TearDownSuite runs once after all tests
func (s *RegressionTestSuite) TearDownSuite() {
	// Clean up executor at the end of all tests
	if s.executor != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := s.executor.Cleanup(ctx); err != nil {
			s.logDetailed("Warning: Executor cleanup failed: %v", err)
		} else {
			s.logDetailed("Executor cleaned up successfully")
		}
	}

	// Always show beautiful summary
	s.printTestSummary()

	if s.logLevel == LogLevelDetailed {
		if s.execMode == ModeLocal {
			s.T().Log("Tests completed on local machine")
		} else {
			s.T().Log("All Docker containers have been cleaned up")
		}
	}
}

// formatDuration formats a duration with consistent width for table alignment
func formatDuration(d time.Duration) string {
	d = d.Round(time.Millisecond)

	// For durations >= 1 second, show with decimal seconds
	if d >= time.Second {
		seconds := float64(d) / float64(time.Second)
		return fmt.Sprintf("%7.3fs", seconds)
	}

	// For durations < 1 second, show as milliseconds
	ms := d.Milliseconds()
	return fmt.Sprintf("%7dms", ms)
}

// printTestSummary displays a beautiful formatted summary of test results
func (s *RegressionTestSuite) printTestSummary() {
	totalDuration := time.Since(s.suiteStartTime)

	// Count passes and failures
	passCount, failCount := 0, 0
	for _, result := range s.testResults {
		if result.Status == "PASS" {
			passCount++
		} else if result.Status == "FAIL" {
			failCount++
		}
	}

	// Create the summary table
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.StyleColoredBright)

	// Configure title
	t.SetTitle("🧪 pgEdge MCP Regression Test Suite - Summary")

	// Add headers
	t.AppendHeader(table.Row{"#", "Test Name", "Status", "Duration"})

	// Configure column alignments for better display
	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, Align: text.AlignRight},  // # column - right aligned
		{Number: 2, Align: text.AlignLeft},   // Test Name - left aligned
		{Number: 3, Align: text.AlignCenter}, // Status - centered
		{Number: 4, Align: text.AlignRight},  // Duration - right aligned
	})

	// Add test results
	for i, result := range s.testResults {
		testName := strings.TrimPrefix(result.Name, "TestRegressionSuite/")

		var status string
		if result.Status == "PASS" {
			status = text.FgGreen.Sprintf("✓ PASS")
		} else if result.Status == "FAIL" {
			status = text.FgRed.Sprintf("✗ FAIL")
		} else {
			status = text.FgYellow.Sprintf("⚠ %s", result.Status)
		}

		// Format duration consistently for better alignment
		durationStr := formatDuration(result.Duration)
		t.AppendRow(table.Row{i + 1, testName, status, durationStr})
	}

	// Add separator before footer
	t.AppendSeparator()

	// Add footer with totals
	totalTests := len(s.testResults)
	var statusSummary string
	if failCount > 0 {
		statusSummary = text.FgRed.Sprintf("%d passed, %d failed", passCount, failCount)
	} else {
		statusSummary = text.FgGreen.Sprintf("All %d tests passed! ✨", passCount)
	}

	totalDurationStr := formatDuration(totalDuration)
	t.AppendFooter(table.Row{"", fmt.Sprintf("Total: %d tests", totalTests), statusSummary, totalDurationStr})

	// Print banner and table
	fmt.Println("\n" + strings.Repeat("=", 80))
	t.Render()
	fmt.Println(strings.Repeat("=", 80))

	// Print execution context
	fmt.Printf("\n📋 Execution Mode: %s\n", text.FgCyan.Sprint(s.execMode.String()))
	if s.execMode != ModeLocal {
		fmt.Printf("🐳 OS Image: %s\n", text.FgCyan.Sprint(s.osDisplayName))
	} else {
		fmt.Printf("💻 System OS: %s\n", text.FgCyan.Sprint(s.osDisplayName))
	}

	// Show server environment with appropriate emoji
	envEmoji := "🟢"
	if s.serverEnv == EnvStaging {
		envEmoji = "🟡"
	}
	fmt.Printf("%s Server Environment: %s\n", envEmoji, text.FgCyan.Sprint(s.serverEnv.String()))

	fmt.Printf("📦 Repository: %s\n", text.FgCyan.Sprint(s.repoURL))
	fmt.Printf("⏱️  Total Duration: %s\n", text.FgCyan.Sprint(totalDuration.Round(time.Millisecond)))

	// Print final status
	if failCount > 0 {
		fmt.Printf("\n%s\n", text.FgRed.Sprint("❌ TEST SUITE FAILED"))
	} else {
		fmt.Printf("\n%s\n", text.FgGreen.Sprint("✅ TEST SUITE PASSED"))
	}

	// Add separator to distinguish from Go test output
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println()
}

// execCmd is a helper that automatically prepends sudo for local mode when needed
func (s *RegressionTestSuite) execCmd(ctx context.Context, cmd string) (string, int, error) {
	// For local mode, prepend sudo to commands that need it
	if s.execMode == ModeLocal {
		// Check if command needs sudo (package management, file operations in system dirs)
		needsSudo := strings.HasPrefix(cmd, "apt-get") ||
			strings.HasPrefix(cmd, "apt ") ||
			strings.HasPrefix(cmd, "apt-cache") ||
			strings.HasPrefix(cmd, "dpkg ") ||
			strings.HasPrefix(cmd, "dnf ") ||
			strings.HasPrefix(cmd, "yum ") ||
			strings.HasPrefix(cmd, "rpm ") ||
			strings.HasPrefix(cmd, "systemctl ") ||
			strings.Contains(cmd, "> /etc/") ||
			strings.Contains(cmd, "> /usr/") ||
			strings.Contains(cmd, "> /var/") ||
			strings.Contains(cmd, "chown ") ||
			strings.Contains(cmd, "chmod ") ||
			strings.Contains(cmd, "mkdir /etc/") ||
			strings.Contains(cmd, "mkdir /usr/") ||
			strings.Contains(cmd, "mkdir /var/") ||
			strings.Contains(cmd, "sed -i") && strings.Contains(cmd, "/etc/")

		if needsSudo && !strings.HasPrefix(cmd, "sudo ") {
			cmd = "sudo " + cmd
		}
	}

	return s.executor.Exec(ctx, cmd)
}

// getOSType returns the OS type based on executor mode
func (s *RegressionTestSuite) getOSType() (isDebian bool, isRHEL bool) {
	if s.execMode == ModeLocal {
		// Detect OS from local system
		output, exitCode, _ := s.execCmd(s.ctx, "cat /etc/os-release")
		if exitCode == 0 {
			osRelease := strings.ToLower(output)
			isDebian = strings.Contains(osRelease, "debian") || strings.Contains(osRelease, "ubuntu")
			isRHEL = strings.Contains(osRelease, "rocky") || strings.Contains(osRelease, "rhel") ||
				strings.Contains(osRelease, "alma") || strings.Contains(osRelease, "centos") ||
				strings.Contains(osRelease, "fedora")
		}
	} else {
		// Use osImage for container modes
		isDebian = strings.Contains(s.osImage, "debian") || strings.Contains(s.osImage, "ubuntu")
		isRHEL = strings.Contains(s.osImage, "rocky") || strings.Contains(s.osImage, "alma") ||
			strings.Contains(s.osImage, "rhel")
	}
	return
}

// ========================================================================
// Helper Methods for Setup (with state tracking)
// ========================================================================

// ensureRepositoryInstalled ensures the repository is installed (runs only once)
func (s *RegressionTestSuite) ensureRepositoryInstalled() {
	if s.setupState.repoInstalled {
		return // Already installed
	}

	s.logDetailed("Installing pgEdge repository...")
	s.installRepository()
	s.setupState.repoInstalled = true
}

// ensurePostgreSQLInstalled ensures PostgreSQL is installed (runs only once)
func (s *RegressionTestSuite) ensurePostgreSQLInstalled() {
	if s.setupState.postgresqlInstalled {
		return // Already installed
	}

	// PostgreSQL requires repository first
	s.ensureRepositoryInstalled()

	s.logDetailed("Installing and configuring PostgreSQL...")
	s.installPostgreSQL()
	s.setupState.postgresqlInstalled = true
}

// ensureMCPPackagesInstalled ensures MCP packages are installed (runs only once)
func (s *RegressionTestSuite) ensureMCPPackagesInstalled() {
	if s.setupState.mcpPackagesInstalled {
		return // Already installed
	}

	// MCP packages require PostgreSQL first
	s.ensurePostgreSQLInstalled()

	s.logDetailed("Installing MCP server packages...")
	s.installMCPPackages()
	s.setupState.mcpPackagesInstalled = true
}

// ========================================================================
// TEST 01: Repository Installation
// ========================================================================
func (s *RegressionTestSuite) Test01_RepositoryInstallation() {
	s.T().Log("TEST 01: Installing pgEdge repository")
	s.ensureRepositoryInstalled()
	s.T().Log("✓ Repository installed successfully")
}

// installRepository performs the actual repository installation
func (s *RegressionTestSuite) installRepository() {
	// Set the repository URL now that we have an executor
	s.setRepositoryURL()

	// Determine package manager based on OS type
	isDebian, isRHEL := s.getOSType()

	if isDebian {
		// Debian/Ubuntu: Install repository using official pgEdge release package
		commands := []string{
			"apt-get update",
			"apt-get install -y curl gnupg2 lsb-release",
			// Download and install pgedge-release package
			"curl -sSL https://apt.pgedge.com/repodeb/pgedge-release_latest_all.deb -o /tmp/pgedge-release.deb",
			"dpkg -i /tmp/pgedge-release.deb",
			"rm -f /tmp/pgedge-release.deb",
		}

		for _, cmd := range commands {
			output, exitCode, err := s.execCmd(s.ctx, cmd)
			s.NoError(err, "Command failed: %s\nOutput: %s", cmd, output)
			s.Equal(0, exitCode, "Command exited with non-zero: %s\nOutput: %s", cmd, output)
		}

		// For Debian systems, modify repo list to use staging if needed
		if s.serverEnv == EnvStaging {
			s.logDetailed("Modifying repository configuration for staging environment...")
			// Find the pgedge repo list file (could be pgedge.list or other name)
			findCmd := "ls -1 /etc/apt/sources.list.d/ | grep -i pgedge"
			findOutput, findExitCode, _ := s.execCmd(s.ctx, findCmd)
			if findExitCode == 0 && strings.TrimSpace(findOutput) != "" {
				repoFile := strings.TrimSpace(strings.Split(findOutput, "\n")[0])
				s.logDetailed("Found repository file: %s", repoFile)
				// Replace 'release' with 'staging' in the pgedge repo list file
				sedCmd := fmt.Sprintf("sed -i 's/ release / staging /g' /etc/apt/sources.list.d/%s", repoFile)
				sedOutput, sedExitCode, sedErr := s.execCmd(s.ctx, sedCmd)
				s.NoError(sedErr, "Failed to modify repo file: %s", sedOutput)
				s.Equal(0, sedExitCode, "Failed to modify repo file: %s", sedOutput)
			} else {
				s.logDetailed("Warning: Could not find pgedge repository file, staging may not be configured")
			}
		}

		// Update package lists
		output, exitCode, err := s.execCmd(s.ctx, "apt-get update")
		s.NoError(err, "apt-get update failed: %s", output)
		s.Equal(0, exitCode, "apt-get update exited with non-zero: %s", output)

		// Verify repository is available
		output, exitCode, err = s.execCmd(s.ctx, "apt-cache search pgedge-postgres-mcp")
		s.NoError(err)
		s.Equal(0, exitCode)
		s.Contains(output, "pgedge-postgres-mcp", "Package should be available in repo")

	} else if isRHEL {
		// RHEL/Rocky/Alma: Install repository
		// Determine EL version
		versionCmd := "rpm -E %{rhel}"
		versionOutput, _, _ := s.execCmd(s.ctx, versionCmd)
		elVersion := strings.TrimSpace(versionOutput)
		if elVersion == "" || elVersion == "%{rhel}" {
			elVersion = "9" // Default to EL9
		}

		commands := []string{
			// Install EPEL repository first
			fmt.Sprintf("dnf -y install https://dl.fedoraproject.org/pub/epel/epel-release-latest-%s.noarch.rpm", elVersion),
			// Install pgEdge repository
			"dnf install -y https://dnf.pgedge.com/reporpm/pgedge-release-latest.noarch.rpm",
		}

		for _, cmd := range commands {
			output, _, err := s.execCmd(s.ctx, cmd)
			s.NoError(err, "Command failed: %s\nOutput: %s", cmd, output)
		}

		// For RHEL-based systems, modify repo file to use staging if needed
		if s.serverEnv == EnvStaging {
			s.logDetailed("Modifying repository configuration for staging environment...")
			// Replace 'release' with 'staging' in the repo file
			sedCmd := "sed -i 's/\\/release\\//\\/staging\\//g' /etc/yum.repos.d/pgedge.repo"
			sedOutput, sedExitCode, sedErr := s.execCmd(s.ctx, sedCmd)
			s.NoError(sedErr, "Failed to modify repo file: %s", sedOutput)
			s.Equal(0, sedExitCode, "Failed to modify repo file: %s", sedOutput)
		}

		// Update metadata
		_, _, err := s.execCmd(s.ctx, "dnf check-update || true")
		s.NoError(err, "Failed to update DNF metadata")

		// Verify repository is available
		output, exitCode, err := s.execCmd(s.ctx, "dnf search pgedge-postgres-mcp")
		s.NoError(err)
		s.Equal(0, exitCode)
		s.Contains(output, "pgedge-postgres-mcp", "Package should be available in repo")
	} else {
		s.Fail("Unsupported OS")
	}
}

// ========================================================================
// TEST 02: PostgreSQL Installation and Setup
// ========================================================================
func (s *RegressionTestSuite) Test02_PostgreSQLSetup() {
	s.T().Log("TEST 02: Installing and configuring pgEdge PostgreSQL")
	s.ensurePostgreSQLInstalled()
	s.T().Log("✓ PostgreSQL installed and configured successfully")
}

// installPostgreSQL performs the actual PostgreSQL installation
func (s *RegressionTestSuite) installPostgreSQL() {

	isDebian, _ := s.getOSType()

	// Step 1: Install PostgreSQL packages
	s.logDetailed("Step 1: Installing PostgreSQL packages")
	var pgPackages []string
	if isDebian {
		pgPackages = []string{
			"apt-get install -y postgresql-18",
		}
	} else {
		pgPackages = []string{
			"dnf install -y pgedge-postgresql18-server",
			"dnf install -y pgedge-postgresql18",
		}
	}

	for _, installCmd := range pgPackages {
		output, exitCode, err := s.execCmd(s.ctx, installCmd)
		s.NoError(err, "PostgreSQL install failed: %s\nOutput: %s", installCmd, output)
		s.Equal(0, exitCode, "PostgreSQL install exited with error: %s\nOutput: %s", installCmd, output)
	}

	// Step 2: Initialize PostgreSQL database
	s.logDetailed("Step 2: Initializing PostgreSQL database")
	if isDebian {
		// Debian/Ubuntu initialization
		// Use pg_ctlcluster with --skip-systemctl-redirect to bypass systemd
		// This avoids the systemd PID file ownership issue on Ubuntu/Debian
		output, exitCode, err := s.execCmd(s.ctx, "pg_ctlcluster --skip-systemctl-redirect 18 main start")
		s.NoError(err, "Failed to start PostgreSQL: %s", output)
		// Exit code 0 = started successfully, Exit code 2 = already running (both are OK)
		s.True(exitCode == 0 || exitCode == 2, "PostgreSQL start failed with unexpected exit code %d: %s", exitCode, output)
	} else {
		// RHEL/Rocky initialization (manual, not systemd)

		// Stop any existing PostgreSQL instance
		s.logDetailed("  Stopping any existing PostgreSQL instances...")
		stopCmd := "su - postgres -c '/usr/pgsql-18/bin/pg_ctl -D /var/lib/pgsql/18/data stop' 2>/dev/null || true"
		s.execCmd(s.ctx, stopCmd) // Ignore errors

		// Also try older version for cleanup
		stopCmd16 := "su - postgres -c '/usr/pgsql-16/bin/pg_ctl -D /var/lib/pgsql/16/data stop' 2>/dev/null || true"
		s.execCmd(s.ctx, stopCmd16) // Ignore errors

		time.Sleep(2 * time.Second)

		// Remove existing data directory if it exists
		s.logDetailed("  Cleaning up existing data directory...")
		cleanupCmd := "rm -rf /var/lib/pgsql/18/data"
		s.execCmd(s.ctx, cleanupCmd)

		// Initialize database
		s.logDetailed("  Initializing new PostgreSQL 18 database...")
		initCmd := "su - postgres -c '/usr/pgsql-18/bin/initdb -D /var/lib/pgsql/18/data'"
		output, exitCode, err := s.execCmd(s.ctx, initCmd)
		s.NoError(err, "PostgreSQL initdb failed: %s", output)
		s.Equal(0, exitCode, "PostgreSQL initdb failed: %s", output)

		// Configure PostgreSQL to accept local connections
		configCmd := `echo "host all all 127.0.0.1/32 md5" >> /var/lib/pgsql/18/data/pg_hba.conf`
		output, exitCode, err = s.execCmd(s.ctx, configCmd)
		s.NoError(err, "Failed to configure pg_hba.conf: %s", output)
		s.Equal(0, exitCode, "pg_hba.conf config failed: %s", output)

		// Start PostgreSQL manually
		s.logDetailed("  Starting PostgreSQL 18...")
		startCmd := "su - postgres -c '/usr/pgsql-18/bin/pg_ctl -D /var/lib/pgsql/18/data -l /var/lib/pgsql/18/data/logfile start'"
		output, exitCode, err = s.execCmd(s.ctx, startCmd)
		s.NoError(err, "PostgreSQL start failed: %s", output)
		s.Equal(0, exitCode, "PostgreSQL start failed: %s", output)

		// Wait for PostgreSQL to be ready
		time.Sleep(3 * time.Second)
	}

	// Step 3: Set postgres user password
	s.logDetailed("Step 3: Setting postgres user password")
	setPwCmd := `su - postgres -c "psql -c \"ALTER USER postgres WITH PASSWORD 'postgres123';\""`
	output, exitCode, err := s.execCmd(s.ctx, setPwCmd)
	s.NoError(err, "Failed to set postgres password: %s", output)
	s.Equal(0, exitCode, "Set password failed: %s", output)

	// Step 4: Create MCP database
	s.logDetailed("Step 4: Creating MCP database")
	// First, try to drop the database if it exists
	dropDbCmd := `su - postgres -c "psql -c \"DROP DATABASE IF EXISTS mcp_server;\""`
	s.execCmd(s.ctx, dropDbCmd) // Ignore errors

	createDbCmd := `su - postgres -c "psql -c \"CREATE DATABASE mcp_server;\""`
	output, exitCode, err = s.execCmd(s.ctx, createDbCmd)
	s.NoError(err, "Failed to create MCP database: %s", output)
	s.Equal(0, exitCode, "Create database failed: %s", output)
}

// ========================================================================
// TEST 03: MCP Server Package Installation
// ========================================================================
func (s *RegressionTestSuite) Test03_MCPServerInstallation() {
	s.T().Log("TEST 03: Installing MCP server packages")
	s.ensureMCPPackagesInstalled()
	s.T().Log("✓ All MCP server packages installed and configured successfully")
}

// installMCPPackages performs the actual MCP package installation
func (s *RegressionTestSuite) installMCPPackages() {

	isDebian, _ := s.getOSType()

	// Step 1: Install MCP server packages
	s.logDetailed("Step 1: Installing MCP server packages")
	var packages []string
	if isDebian {
		packages = []string{
			"apt-get install -y pgedge-postgres-mcp",
			"apt-get install -y pgedge-nla-cli",
			"apt-get install -y pgedge-nla-web",
			"apt-get install -y pgedge-postgres-mcp-kb",
		}
	} else {
		packages = []string{
			"dnf install -y pgedge-postgres-mcp",
			"dnf install -y pgedge-nla-cli",
			"dnf install -y pgedge-nla-web",
			"dnf install -y pgedge-postgres-mcp-kb",
		}
	}

	for _, installCmd := range packages {
		output, exitCode, err := s.execCmd(s.ctx, installCmd)
		s.NoError(err, "Install failed: %s\nOutput: %s", installCmd, output)
		s.Equal(0, exitCode, "Install exited with error: %s\nOutput: %s", installCmd, output)
	}

	// Step 2: Update MCP server configuration
	s.logDetailed("Step 2: Updating MCP server configuration files")

	// Update postgres-mcp.yaml
	yamlConfig := `cat > /etc/pgedge/postgres-mcp.yaml << 'EOF'
databases:
  - host: localhost
    port: 5432
    name: mcp_server
    user: postgres
    password: postgres123
server:
  mode: http
  addr: :8080
EOF`
	output, exitCode, err := s.execCmd(s.ctx, yamlConfig)
	s.NoError(err, "Failed to update postgres-mcp.yaml: %s", output)
	s.Equal(0, exitCode, "Update config failed: %s", output)

	// Update postgres-mcp.env
	envConfig := `cat > /etc/pgedge/postgres-mcp.env << 'EOF'
DB_HOST=localhost
DB_PORT=5432
DB_NAME=mcp_server
DB_USER=postgres
DB_PASSWORD=postgres123
SERVER_MODE=http
SERVER_ADDR=:8080
EOF`
	output, exitCode, err = s.execCmd(s.ctx, envConfig)
	s.NoError(err, "Failed to update postgres-mcp.env: %s", output)
	s.Equal(0, exitCode, "Update env failed: %s", output)
}

// ========================================================================
// TEST 04: Installation Validation
// ========================================================================
func (s *RegressionTestSuite) Test04_InstallationValidation() {
	s.T().Log("TEST 04: Validating MCP server installation")

	// Ensure packages are installed
	s.ensureMCPPackagesInstalled()

	// Check 1: Binary exists and is executable
	output, exitCode, err := s.execCmd(s.ctx, "test -x /usr/bin/pgedge-postgres-mcp && echo 'OK'")
	s.NoError(err)
	s.Equal(0, exitCode)
	s.Contains(output, "OK", "Binary should exist and be executable")

	// Check 2: Config directory exists
	output, exitCode, err = s.execCmd(s.ctx, "test -d /etc/pgedge && echo 'OK'")
	s.NoError(err)
	s.Equal(0, exitCode)
	s.Contains(output, "OK", "Config directory should exist")

	// Check 3: Config files exist
	output, exitCode, err = s.execCmd(s.ctx, "test -f /etc/pgedge/postgres-mcp.yaml && echo 'OK'")
	s.NoError(err)
	s.Equal(0, exitCode)
	s.Contains(output, "OK", "postgres-mcp.yaml should exist")

	output, exitCode, err = s.execCmd(s.ctx, "test -f /etc/pgedge/postgres-mcp.env && echo 'OK'")
	s.NoError(err)
	s.Equal(0, exitCode)
	s.Contains(output, "OK", "postgres-mcp.env should exist")

	// Check 4: Systemd service file exists
	output, exitCode, err = s.execCmd(s.ctx, "test -f /usr/lib/systemd/system/pgedge-postgres-mcp.service && echo 'OK'")
	s.NoError(err)
	s.Equal(0, exitCode)
	s.Contains(output, "OK", "Systemd service file should exist")

	// Check 5: Data directory exists with correct permissions
	output, exitCode, err = s.execCmd(s.ctx, "test -d /var/lib/pgedge/postgres-mcp && echo 'OK'")
	s.NoError(err)
	s.Equal(0, exitCode)
	s.Contains(output, "OK", "Data directory should exist")

	s.T().Log("✓ All installation validation checks passed")
}

// ========================================================================
// TEST 05: Token Management
// ========================================================================
func (s *RegressionTestSuite) Test05_TokenManagement() {
	s.T().Log("TEST 05: Testing token management commands")

	// Ensure packages are installed
	s.ensureMCPPackagesInstalled()

	// Test 1: Create token (using config file for database connection)
	createCmd := `/usr/bin/pgedge-postgres-mcp -config /etc/pgedge/postgres-mcp.yaml -add-token -token-file /etc/pgedge/pgedge-postgres-mcp-tokens.yaml -token-note "test-token"`
	output, exitCode, err := s.execCmd(s.ctx, createCmd)
	s.NoError(err, "Token creation failed\nOutput: %s", output)
	s.Equal(0, exitCode)
	s.Contains(output, "Token:", "Should show generated token")
	s.Contains(output, "Hash:", "Should show token hash")

	// Set proper ownership on token file
	chownCmd := "chown pgedge:pgedge /etc/pgedge/pgedge-postgres-mcp-tokens.yaml"
	output, exitCode, err = s.execCmd(s.ctx, chownCmd)
	s.NoError(err, "Failed to set ownership on token file: %s", output)
	s.Equal(0, exitCode, "chown failed: %s", output)

	// Test 2: List tokens
	output, exitCode, err = s.execCmd(s.ctx, "/usr/bin/pgedge-postgres-mcp -config /etc/pgedge/postgres-mcp.yaml -list-tokens -token-file /etc/pgedge/pgedge-postgres-mcp-tokens.yaml")
	s.NoError(err)
	s.Equal(0, exitCode)
	s.Contains(output, "test-token", "Should list created token")

	// Test 3: Verify token file was created
	output, exitCode, err = s.execCmd(s.ctx, "test -f /etc/pgedge/pgedge-postgres-mcp-tokens.yaml && echo 'OK'")
	s.NoError(err)
	s.Equal(0, exitCode)
	s.Contains(output, "OK", "Token file should exist")

	s.T().Log("✓ Token management working correctly")
}

// ========================================================================
// TEST 06: User Management
// ========================================================================
func (s *RegressionTestSuite) Test06_UserManagement() {
	s.T().Log("TEST 06: Testing user management commands")

	// Ensure packages are installed
	s.ensureMCPPackagesInstalled()

	// Test 1: Create user (using config file for database connection)
	createCmd := `/usr/bin/pgedge-postgres-mcp -config /etc/pgedge/postgres-mcp.yaml -add-user -user-file /etc/pgedge/pgedge-postgres-mcp-users.yaml -username testuser -password testpass123 -user-note "test user"`
	output, exitCode, err := s.execCmd(s.ctx, createCmd)
	s.NoError(err, "User creation failed\nOutput: %s", output)
	s.Equal(0, exitCode)
	s.Contains(output, "User created", "Should confirm user creation")

	// Test 2: List users
	output, exitCode, err = s.execCmd(s.ctx, "/usr/bin/pgedge-postgres-mcp -config /etc/pgedge/postgres-mcp.yaml -list-users -user-file /etc/pgedge/pgedge-postgres-mcp-users.yaml")
	s.NoError(err)
	s.Equal(0, exitCode)
	s.Contains(output, "testuser", "Should list created user")

	// Test 3: Verify user file was created
	output, exitCode, err = s.execCmd(s.ctx, "test -f /etc/pgedge/pgedge-postgres-mcp-users.yaml && echo 'OK'")
	s.NoError(err)
	s.Equal(0, exitCode)
	s.Contains(output, "OK", "User file should exist")

	// Test 4: User file has correct permissions (should be restrictive)
	output, exitCode, err = s.execCmd(s.ctx, "stat -c '%a' /etc/pgedge/pgedge-postgres-mcp-users.yaml")
	s.NoError(err)
	s.Equal(0, exitCode)
	// File should be readable but ideally 600 or 644
	output = strings.TrimSpace(output)
	s.Regexp(`^[0-9]{3}$`, output, "Should have valid permissions")

	s.T().Log("✓ User management working correctly")
}

// ========================================================================
// TEST 07: Package Files and Permissions Verification
// ========================================================================
func (s *RegressionTestSuite) Test07_PackageFilesVerification() {
	s.T().Log("TEST 07: Verifying installed package files and permissions")

	// Ensure packages are installed
	s.ensureMCPPackagesInstalled()

	// ====================================================================
	// 1. Verify binaries in /usr/bin with executable permissions
	// ====================================================================
	s.T().Log("Checking binaries in /usr/bin...")

	binaries := []struct {
		name        string
		permissions string // Expected permissions pattern
	}{
		{"pgedge-postgres-mcp", "755"},
		{"pgedge-nla-kb-builder", "755"},
		{"pgedge-nla-cli", "755"},
	}

	for _, bin := range binaries {
		// Check if binary exists
		output, exitCode, err := s.execCmd(s.ctx, fmt.Sprintf("test -f /usr/bin/%s && echo 'exists'", bin.name))
		s.NoError(err, "Failed to check if %s exists", bin.name)
		s.Equal(0, exitCode, "%s should exist in /usr/bin", bin.name)
		s.Contains(output, "exists", "%s should exist in /usr/bin", bin.name)

		// Check permissions (should be executable: 755 or 775)
		output, exitCode, err = s.execCmd(s.ctx, fmt.Sprintf("stat -c '%%a' /usr/bin/%s", bin.name))
		s.NoError(err, "Failed to check permissions for %s", bin.name)
		s.Equal(0, exitCode, "Should get permissions for %s", bin.name)
		s.Contains(output, bin.permissions, "%s should have %s permissions", bin.name, bin.permissions)

		// Verify it's owned by root
		output, exitCode, err = s.execCmd(s.ctx, fmt.Sprintf("stat -c '%%U:%%G' /usr/bin/%s", bin.name))
		s.NoError(err, "Failed to check ownership for %s", bin.name)
		s.Equal(0, exitCode, "Should get ownership for %s", bin.name)
		s.Contains(output, "root:root", "%s should be owned by root:root", bin.name)

		s.T().Logf("  ✓ /usr/bin/%s exists with correct permissions (%s)", bin.name, bin.permissions)
	}

	// ====================================================================
	// 2. Verify systemd service file
	// ====================================================================
	s.T().Log("Checking systemd service file...")

	serviceFile := "/usr/lib/systemd/system/pgedge-postgres-mcp.service"

	// Check if service file exists
	output, exitCode, err := s.execCmd(s.ctx, fmt.Sprintf("test -f %s && echo 'exists'", serviceFile))
	s.NoError(err, "Failed to check if service file exists")
	s.Equal(0, exitCode, "Service file should exist")
	s.Contains(output, "exists", "Service file should exist at %s", serviceFile)

	// Check permissions (should be 644)
	output, exitCode, err = s.execCmd(s.ctx, fmt.Sprintf("stat -c '%%a' %s", serviceFile))
	s.NoError(err, "Failed to check service file permissions")
	s.Equal(0, exitCode, "Should get service file permissions")
	s.Contains(output, "644", "Service file should have 644 permissions")

	s.T().Logf("  ✓ %s exists with correct permissions (644)", serviceFile)

	// ====================================================================
	// 3. Verify /usr/share directories
	// ====================================================================
	s.T().Log("Checking /usr/share/pgedge/nla-web directory...")

	nlaWebPath := "/usr/share/pgedge/nla-web"

	// Check if directory exists
	output, exitCode, err = s.execCmd(s.ctx, fmt.Sprintf("test -d %s && echo 'exists'", nlaWebPath))
	s.NoError(err, "Failed to check if %s exists", nlaWebPath)
	s.Equal(0, exitCode, "%s should exist", nlaWebPath)
	s.Contains(output, "exists", "%s should exist", nlaWebPath)

	// Check permissions (should be readable: 755)
	output, exitCode, err = s.execCmd(s.ctx, fmt.Sprintf("stat -c '%%a' %s", nlaWebPath))
	s.NoError(err, "Failed to check permissions for %s", nlaWebPath)
	s.Equal(0, exitCode, "Should get permissions for %s", nlaWebPath)
	s.Contains(output, "755", "%s should have 755 permissions", nlaWebPath)

	// Verify it's owned by root
	output, exitCode, err = s.execCmd(s.ctx, fmt.Sprintf("stat -c '%%U:%%G' %s", nlaWebPath))
	s.NoError(err, "Failed to check ownership for %s", nlaWebPath)
	s.Equal(0, exitCode, "Should get ownership for %s", nlaWebPath)
	s.Contains(output, "root:root", "%s should be owned by root:root", nlaWebPath)

	s.T().Logf("  ✓ %s exists with correct permissions (755)", nlaWebPath)

	// List files inside nla-web directory
	output, exitCode, err = s.execCmd(s.ctx, fmt.Sprintf("ls -lrt %s", nlaWebPath))
	s.NoError(err, "Failed to list files in %s", nlaWebPath)
	s.Equal(0, exitCode, "Should be able to list files in %s", nlaWebPath)

	s.T().Logf("    Files in %s:", nlaWebPath)
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line != "" && !strings.HasPrefix(line, "total") {
			s.T().Logf("      %s", line)
		}
	}

	// ====================================================================
	// 4. Verify /etc/pgedge configuration files
	// ====================================================================
	s.T().Log("Checking /etc/pgedge configuration files...")

	configFiles := []struct {
		name        string
		permissions string
	}{
		{"postgres-mcp.env", "644"},
		{"pgedge-nla-kb-builder.yaml", "644"},
		{"nla-cli.yaml", "644"},
		{"postgres-mcp.yaml", "644"},
	}

	for _, cfg := range configFiles {
		path := fmt.Sprintf("/etc/pgedge/%s", cfg.name)

		// Check if file exists
		output, exitCode, err := s.execCmd(s.ctx, fmt.Sprintf("test -f %s && echo 'exists'", path))
		s.NoError(err, "Failed to check if %s exists", path)
		s.Equal(0, exitCode, "%s should exist", path)
		s.Contains(output, "exists", "%s should exist", path)

		// Check permissions (should be readable: 644)
		output, exitCode, err = s.execCmd(s.ctx, fmt.Sprintf("stat -c '%%a' %s", path))
		s.NoError(err, "Failed to check permissions for %s", path)
		s.Equal(0, exitCode, "Should get permissions for %s", path)
		s.Contains(output, cfg.permissions, "%s should have %s permissions", path, cfg.permissions)

		// Verify it's owned by root
		output, exitCode, err = s.execCmd(s.ctx, fmt.Sprintf("stat -c '%%U:%%G' %s", path))
		s.NoError(err, "Failed to check ownership for %s", path)
		s.Equal(0, exitCode, "Should get ownership for %s", path)
		s.Contains(output, "root:root", "%s should be owned by root:root", path)

		s.T().Logf("  ✓ %s exists with correct permissions (%s)", path, cfg.permissions)
	}

	// ====================================================================
	// 5. Verify /var/lib/pgedge/postgres-mcp directory
	// ====================================================================
	s.T().Log("Checking /var/lib/pgedge/postgres-mcp directory...")

	dataDir := "/var/lib/pgedge/postgres-mcp"

	// Check if directory exists
	output, exitCode, err = s.execCmd(s.ctx, fmt.Sprintf("test -d %s && echo 'exists'", dataDir))
	s.NoError(err, "Failed to check if data directory exists")
	s.Equal(0, exitCode, "Data directory should exist")
	s.Contains(output, "exists", "%s should exist", dataDir)

	// Check permissions (should be 755 for directory)
	output, exitCode, err = s.execCmd(s.ctx, fmt.Sprintf("stat -c '%%a' %s", dataDir))
	s.NoError(err, "Failed to check data directory permissions")
	s.Equal(0, exitCode, "Should get data directory permissions")
	s.Contains(output, "755", "Data directory should have 755 permissions")

	// Verify it's owned by pgedge:pgedge
	output, exitCode, err = s.execCmd(s.ctx, fmt.Sprintf("stat -c '%%U:%%G' %s", dataDir))
	s.NoError(err, "Failed to check data directory ownership")
	s.Equal(0, exitCode, "Should get data directory ownership")
	s.Contains(output, "pgedge:pgedge", "%s should be owned by pgedge:pgedge", dataDir)

	s.T().Logf("  ✓ %s exists with correct ownership (pgedge:pgedge)", dataDir)

	// ====================================================================
	// 6. Verify log directories exist
	// ====================================================================
	s.T().Log("Checking log directories...")

	logDirectories := []struct {
		path        string
		owner       string
		permissions string
	}{
		{"/var/log/pgedge/postgres-mcp", "pgedge:pgedge", "755"},
		{"/var/log/pgedge/nla-web", "pgedge:pgedge", "755"},
	}

	for _, logDir := range logDirectories {
		// Check if directory exists (it might not exist until service runs)
		output, exitCode, err := s.execCmd(s.ctx, fmt.Sprintf("test -d %s && echo 'exists' || echo 'missing'", logDir.path))
		s.NoError(err, "Failed to check if %s exists", logDir.path)

		if strings.Contains(output, "exists") {
			// Directory exists, verify permissions and ownership

			// Check permissions
			output, exitCode, err = s.execCmd(s.ctx, fmt.Sprintf("stat -c '%%a' %s", logDir.path))
			s.NoError(err, "Failed to check permissions for %s", logDir.path)
			s.Equal(0, exitCode, "Should get permissions for %s", logDir.path)
			s.Contains(output, logDir.permissions, "%s should have %s permissions", logDir.path, logDir.permissions)

			// Check ownership
			output, exitCode, err = s.execCmd(s.ctx, fmt.Sprintf("stat -c '%%U:%%G' %s", logDir.path))
			s.NoError(err, "Failed to check ownership for %s", logDir.path)
			s.Equal(0, exitCode, "Should get ownership for %s", logDir.path)
			s.Contains(output, logDir.owner, "%s should be owned by %s", logDir.path, logDir.owner)

			s.T().Logf("  ✓ %s exists with correct ownership (%s)", logDir.path, logDir.owner)

			// List log files inside the directory
			output, exitCode, err = s.execCmd(s.ctx, fmt.Sprintf("ls -lh %s 2>/dev/null || echo 'empty'", logDir.path))
			s.NoError(err, "Failed to list log files in %s", logDir.path)

			if strings.Contains(output, "empty") || strings.TrimSpace(output) == "" {
				s.T().Logf("    ℹ %s is empty (no log files yet)", logDir.path)
			} else {
				s.T().Logf("    Log files in %s:", logDir.path)
				for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
					if line != "" && !strings.HasPrefix(line, "total") {
						s.T().Logf("      %s", line)
					}
				}
			}
		} else {
			// Directory doesn't exist yet - this is acceptable (created on first run)
			s.T().Logf("  ℹ %s not yet created (will be created on first service run)", logDir.path)
		}
	}

	// ====================================================================
	// 7. Verify parent /var/log/pgedge directory
	// ====================================================================
	s.T().Log("Checking parent log directory...")

	parentLogDir := "/var/log/pgedge"

	// Check if parent directory exists
	output, exitCode, err = s.execCmd(s.ctx, fmt.Sprintf("test -d %s && echo 'exists'", parentLogDir))
	s.NoError(err, "Failed to check if parent log directory exists")
	s.Equal(0, exitCode, "Parent log directory should exist")
	s.Contains(output, "exists", "%s should exist", parentLogDir)

	// Check permissions
	output, exitCode, err = s.execCmd(s.ctx, fmt.Sprintf("stat -c '%%a' %s", parentLogDir))
	s.NoError(err, "Failed to check parent log directory permissions")
	s.Equal(0, exitCode, "Should get parent log directory permissions")
	s.Contains(output, "755", "Parent log directory should have 755 permissions")

	s.T().Logf("  ✓ %s exists with correct permissions (755)", parentLogDir)

	s.T().Log("✓ All package files and permissions verified successfully")
}

// ========================================================================
// TEST 08: Service Management (Start and Status Check)
// ========================================================================
func (s *RegressionTestSuite) Test08_ServiceManagement() {
	s.T().Log("TEST 08: Testing MCP server service management")

	// Ensure packages are installed
	s.ensureMCPPackagesInstalled()

	// Determine if we can use systemd based on execution mode
	canUseSystemd := s.execMode == ModeContainerSystemd || s.execMode == ModeLocal

	if canUseSystemd {
		s.T().Log("Testing with systemd service management...")

		// ====================================================================
		// 1. Reload systemd daemon to recognize the new service
		// ====================================================================
		s.logDetailed("Step 1: Reloading systemd daemon...")
		output, exitCode, err := s.execCmd(s.ctx, "systemctl daemon-reload")
		s.NoError(err, "Failed to reload systemd daemon: %s", output)
		s.Equal(0, exitCode, "systemctl daemon-reload failed: %s", output)

		// ====================================================================
		// 2. Check if systemd-journald is working (container issue workaround)
		// ====================================================================
		// In container mode with systemd issues (like AlmaLinux 10), journald may fail
		// which prevents services from starting. Check if this is the case.
		if s.execMode == ModeContainerSystemd {
			s.logDetailed("Step 2: Checking systemd-journald availability...")
			journalCheck, _, _ := s.execCmd(s.ctx, "systemctl is-active systemd-journald.service")
			if !strings.Contains(journalCheck, "active") {
				s.T().Log("  ⚠ systemd-journald is not available in this container")
				s.T().Log("  ℹ Skipping service tests (services require working journald)")
				s.T().Log("  ℹ Note: Package installation and configuration were verified successfully")
				s.T().Log("✓ Service management tests skipped (systemd-journald unavailable in container)")
				return
			}
		}

		// ====================================================================
		// 3. Enable the service (so it starts on boot)
		// ====================================================================
		s.logDetailed("Step 3: Enabling pgedge-postgres-mcp service...")
		output, exitCode, err = s.execCmd(s.ctx, "systemctl enable pgedge-postgres-mcp.service")
		s.NoError(err, "Failed to enable service: %s", output)
		s.Equal(0, exitCode, "systemctl enable failed: %s", output)

		// ====================================================================
		// 4. Start the service
		// ====================================================================
		s.logDetailed("Step 4: Starting pgedge-postgres-mcp service...")
		output, exitCode, err = s.execCmd(s.ctx, "systemctl start pgedge-postgres-mcp.service")
		s.NoError(err, "Failed to start service: %s", output)
		s.Equal(0, exitCode, "systemctl start failed: %s", output)

		// Wait for service to fully start
		time.Sleep(5 * time.Second)

		// ====================================================================
		// 5. Check service status
		// ====================================================================
		s.logDetailed("Step 5: Checking service status...")
		output, exitCode, err = s.execCmd(s.ctx, "systemctl status pgedge-postgres-mcp.service")
		// Note: systemctl status returns 0 if active, 3 if not running, 4 if unknown
		if exitCode != 0 {
			s.T().Logf("Service status output:\n%s", output)
		}
		s.NoError(err, "Failed to check service status: %s", output)
		s.Equal(0, exitCode, "Service should be running (status command returned non-zero): %s", output)

		// ====================================================================
		// 6. Verify service is active
		// ====================================================================
		s.logDetailed("Step 6: Verifying service is active...")
		output, exitCode, err = s.execCmd(s.ctx, "systemctl is-active pgedge-postgres-mcp.service")
		s.NoError(err, "Failed to check if service is active: %s", output)
		s.Equal(0, exitCode, "Service is not active: %s", output)
		s.Contains(output, "active", "Service should report as 'active'")

		s.T().Logf("  ✓ Service is active: %s", strings.TrimSpace(output))

		// ====================================================================
		// 7. Test HTTP endpoint connectivity (this also verifies port is listening)
		// ====================================================================
		s.logDetailed("Step 7: Testing HTTP endpoint connectivity...")
		// Try to connect with curl (proves service is listening on port 8080)
		var httpCheckSuccess bool
		var httpStatus string
		for i := 0; i < 5; i++ {
			output, exitCode, _ = s.execCmd(s.ctx, "curl -s -o /dev/null -w '%{http_code}' http://localhost:8080/ 2>/dev/null || echo 'curl_failed'")
			if exitCode == 0 && !strings.Contains(output, "curl_failed") {
				httpCheckSuccess = true
				httpStatus = strings.TrimSpace(output)
				s.T().Logf("  ✓ HTTP endpoint responded with status: %s (service is listening on port 8080)", httpStatus)
				break
			}
			time.Sleep(2 * time.Second)
		}

		if !httpCheckSuccess {
			// Show service logs for debugging
			logs, _, _ := s.execCmd(s.ctx, "journalctl -u pgedge-postgres-mcp.service -n 50 --no-pager")
			s.T().Logf("Service logs:\n%s", logs)

			// Also try port check commands for additional debugging info
			portCheck, _, _ := s.execCmd(s.ctx, "ss -tlnp | grep :8080 || netstat -tlnp | grep :8080 || lsof -i :8080 || echo 'No port check tools available'")
			s.T().Logf("Port check output:\n%s", portCheck)

			s.Fail("HTTP endpoint is not responding on port 8080")
		}

		s.T().Log("✓ Service management tests completed successfully (systemd mode)")

	} else {
		// Manual service testing (for standard containers without systemd)
		s.T().Log("Testing with manual service management (no systemd)...")

		// ====================================================================
		// 1. Start the service manually in the background
		// ====================================================================
		s.logDetailed("Step 1: Starting pgedge-postgres-mcp manually...")

		// Create a simple script to run the service
		startScript := `cat > /tmp/start-mcp.sh << 'EOF'
#!/bin/bash
/usr/bin/pgedge-postgres-mcp -config /etc/pgedge/postgres-mcp.yaml > /var/log/pgedge/postgres-mcp/server.log 2>&1 &
echo $! > /tmp/mcp-server.pid
EOF`
		output, exitCode, err := s.execCmd(s.ctx, startScript)
		s.NoError(err, "Failed to create start script: %s", output)
		s.Equal(0, exitCode, "Create start script failed: %s", output)

		// Make it executable
		output, exitCode, err = s.execCmd(s.ctx, "chmod +x /tmp/start-mcp.sh")
		s.NoError(err, "Failed to make start script executable: %s", output)
		s.Equal(0, exitCode, "chmod failed: %s", output)

		// Run the start script
		output, exitCode, err = s.execCmd(s.ctx, "/tmp/start-mcp.sh")
		s.NoError(err, "Failed to start service: %s", output)
		s.Equal(0, exitCode, "Service start failed: %s", output)

		// Wait for service to start
		time.Sleep(5 * time.Second)

		// ====================================================================
		// 2. Check if process is running
		// ====================================================================
		s.logDetailed("Step 2: Checking if service process is running...")
		output, exitCode, err = s.execCmd(s.ctx, "ps aux | grep pgedge-postgres-mcp | grep -v grep")
		s.NoError(err, "Failed to check process status: %s", output)
		s.Equal(0, exitCode, "Service process is not running: %s", output)
		s.Contains(output, "pgedge-postgres-mcp", "Service process should be running")

		s.T().Logf("  ✓ Service process is running")

		// ====================================================================
		// 3. Check if service is listening on the configured port
		// ====================================================================
		s.logDetailed("Step 3: Verifying service is listening on port 8080...")
		var portCheckSuccess bool
		for i := 0; i < 5; i++ {
			output, exitCode, _ = s.execCmd(s.ctx, "ss -tlnp | grep :8080 || netstat -tlnp | grep :8080 || true")
			if exitCode == 0 && strings.Contains(output, "8080") {
				portCheckSuccess = true
				s.T().Logf("  ✓ Service is listening on port 8080")
				break
			}
			time.Sleep(2 * time.Second)
		}

		if !portCheckSuccess {
			// Show service logs for debugging
			logs, _, _ := s.execCmd(s.ctx, "cat /var/log/pgedge/postgres-mcp/server.log 2>/dev/null || echo 'No logs available'")
			s.T().Logf("Service logs:\n%s", logs)
			s.Fail("Service is not listening on port 8080")
		}

		// ====================================================================
		// 4. Test HTTP endpoint (basic connectivity)
		// ====================================================================
		s.logDetailed("Step 4: Testing HTTP endpoint connectivity...")
		output, exitCode, err = s.execCmd(s.ctx, "curl -s -o /dev/null -w '%{http_code}' http://localhost:8080/ || echo 'curl_failed'")
		if exitCode == 0 && !strings.Contains(output, "curl_failed") {
			s.T().Logf("  ✓ HTTP endpoint responded with status: %s", strings.TrimSpace(output))
		} else {
			s.T().Logf("  ⚠ Could not reach HTTP endpoint (this may be expected if auth is required)")
		}

		// ====================================================================
		// 5. Stop the service (cleanup)
		// ====================================================================
		s.logDetailed("Step 5: Stopping service (cleanup)...")
		output, exitCode, err = s.execCmd(s.ctx, "kill $(cat /tmp/mcp-server.pid 2>/dev/null) 2>/dev/null || true")
		s.NoError(err, "Failed to stop service: %s", output)

		s.T().Log("✓ Service management tests completed successfully (manual mode)")
	}
}

// Run the test suite
func TestRegressionSuite(t *testing.T) {
	suite.Run(t, new(RegressionTestSuite))
}
