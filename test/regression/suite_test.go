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
	// Always show as seconds with 3 decimal places for consistency
	seconds := float64(d) / float64(time.Second)
	return fmt.Sprintf("%.3fs", seconds)
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

	// Use ColoredBright - has best color coverage despite small edge gaps
	t.SetStyle(table.StyleColoredBright)

	// Fix footer visibility by customizing colors
	// StyleColoredBright uses BgCyan+FgBlack which has poor visibility
	// Change to BgHiCyan+FgBlack for better contrast
	style := t.Style()
	style.Color.Footer = text.Colors{text.BgHiCyan, text.FgBlack}
	t.SetStyle(*style)

	// Configure title
	t.SetTitle("🧪 pgEdge MCP Regression Test Suite - Summary")

	// Add headers
	t.AppendHeader(table.Row{"#", "Test Name", "Status", "Duration"})

	// Configure column alignments for better display
	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, Align: text.AlignRight},  // # column - right aligned
		{Number: 2, Align: text.AlignLeft},   // Test Name - left aligned
		{Number: 3, Align: text.AlignLeft},   // Status - left aligned
		{Number: 4, Align: text.AlignRight},  // Duration - right aligned
	})

	// Add test results
	for i, result := range s.testResults {
		testName := strings.TrimPrefix(result.Name, "TestRegressionSuite/")

		var status string
		// Use simpler status format in CI to avoid rendering issues
		if os.Getenv("CI") != "" {
			if result.Status == "PASS" {
				status = "✓ PASS"
			} else if result.Status == "FAIL" {
				status = "✗ FAIL"
			} else {
				status = fmt.Sprintf("⚠ %s", result.Status)
			}
		} else {
			if result.Status == "PASS" {
				status = text.FgGreen.Sprintf("✓ PASS")
			} else if result.Status == "FAIL" {
				status = text.FgRed.Sprintf("✗ FAIL")
			} else {
				status = text.FgYellow.Sprintf("⚠ %s", result.Status)
			}
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
	// Don't add manual ANSI colors - table style handles footer coloring
	if failCount > 0 {
		statusSummary = fmt.Sprintf("%d/%d PASSED", passCount, totalTests)
	} else {
		statusSummary = fmt.Sprintf("%d/%d PASSED ✨", passCount, totalTests)
	}

	totalDurationStr := formatDuration(totalDuration)
	t.AppendFooter(table.Row{"", fmt.Sprintf("TOTAL: %d tests", totalTests), statusSummary, totalDurationStr})

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

// Run the test suite
func TestRegressionSuite(t *testing.T) {
	suite.Run(t, new(RegressionTestSuite))
}
