# MCP Server Regression Tests

Automated regression testing for pgEdge Postgres MCP Server package installation.

## Overview

This test suite validates:

1. ✅ Repository installation
2. ✅ PostgreSQL installation and configuration
3. ✅ MCP server package installation from repository
4. ✅ Installation validation (files, permissions, services)
5. ✅ Token management commands
6. ✅ User management commands

## Execution Modes

The test suite supports three execution modes:

1. **Container (Standard)** - Runs tests in a Docker container without
   systemd
2. **Container with systemd** - Runs tests in a systemd-enabled Docker
   container
3. **Local Machine** - Runs tests directly on your local system (requires
   sudo)

## Prerequisites

### For Container Modes

- Docker installed and running
- Go 1.21+ installed
- Internet connection (for pulling Docker images)

### For Local Machine Mode

- Go 1.21+ installed
- Sudo access (passwordless sudo recommended)
- Internet connection
- Supported OS: Debian, Ubuntu, Rocky Linux, AlmaLinux, RHEL

## Quick Start

### New Configuration System (Recommended)

The test suite now uses an interactive configuration wizard:

```bash
# Navigate to test directory
cd test/regression

# Step 1: Configure test settings (first time only)
./Setup_configuration

# Step 2: Run tests
./Execute_Regression_suite
```

Or use make commands (if you prefer):

```bash
make Setup_configuration  # Configure
make Execute_Regression_suite  # Run tests
```

Or use short aliases:

```bash
make config    # Same as Setup_configuration
make test      # Same as Execute_Regression_suite
```

The configuration wizard guides you through selecting:
- Execution mode (local vs container)
- Server environment (live vs staging)
- Container OS image (AlmaLinux, Ubuntu, Debian, etc.)
- Log level (minimal vs detailed)
- Test timeout

Configuration is saved to `.test.env` and reused for future runs.

### Legacy Method (Still Supported)

```bash
# Specify execution mode via environment variable
TEST_EXEC_MODE=container make test           # Standard container
TEST_EXEC_MODE=container-systemd make test   # Systemd container
TEST_EXEC_MODE=local make test               # Local machine
```

## Running Tests

### Test on Specific OS

```bash
# Debian 12
make test-debian

# Rocky Linux 9
make test-rocky

# Ubuntu 22.04
make test-ubuntu
```

### Test Individual Cases

```bash
# Run specific test
make test-one TEST=Test01_RepositoryInstallation
make test-one TEST=Test02_PostgreSQLSetup
make test-one TEST=Test03_MCPServerInstallation
make test-one TEST=Test04_InstallationValidation
make test-one TEST=Test05_TokenManagement
make test-one TEST=Test06_UserManagement
make test-one TEST=Test07_PackageFilesVerification
```

### Custom OS Image

```bash
# Test on custom Docker image (container mode)
TEST_OS_IMAGE=debian:11 go test -v -timeout 20m
TEST_OS_IMAGE=almalinux:9 go test -v -timeout 20m

# With systemd enabled
TEST_EXEC_MODE=container-systemd TEST_OS_IMAGE=debian:12 go test -v
-timeout 20m

# On local machine (ignores TEST_OS_IMAGE)
TEST_EXEC_MODE=local go test -v -timeout 20m
```

### Interactive Mode

When you run tests without setting `TEST_EXEC_MODE`, the suite will
prompt you:

```
=== MCP Regression Test Suite ===
Please select how you want to run the tests:

1. Container (Docker) - Standard container without systemd
2. Container with systemd - Docker container with systemd enabled
3. Local machine - Run tests directly on this system

Enter your choice [1-3] (default: 1):
```

**Note:** Local machine mode will show a warning and require confirmation
before proceeding.

## Test Workflow

### Container Mode

Each test:

1. **Pulls** Docker image (debian:12, rockylinux:9, etc.)
2. **Starts** clean container (with or without systemd)
3. **Installs** pgEdge repository
4. **Installs** MCP server package from repo
5. **Validates** installation
6. **Runs** test cases
7. **Removes** container

Total time per OS: ~5-10 minutes

### Local Machine Mode

Each test:

1. **Verifies** sudo access
2. **Detects** local OS and package manager
3. **Installs** basic dependencies (wget, curl, gnupg, ca-certificates)
4. **Installs** pgEdge repository on local system
5. **Installs** MCP server package from repo
6. **Validates** installation
7. **Runs** test cases
8. **Cleanup** (executor cleanup, packages remain installed)

**WARNING:** Local mode will install packages and modify system
configuration. It does NOT uninstall packages automatically.

**Dependencies Installed:** The test suite automatically installs basic
dependencies needed for package management:

- Debian/Ubuntu: `wget`, `gnupg`, `curl`, `ca-certificates`
- RHEL/Rocky/Alma: `wget`, `curl`, `ca-certificates`

## Test Cases

### Test 01: Repository Installation

- Adds pgEdge package repository
- Installs EPEL repository (RHEL/Rocky)
- Verifies repository configuration
- Confirms packages are available

### Test 02: PostgreSQL Installation and Configuration

- Installs pgEdge PostgreSQL 16 packages
- Initializes PostgreSQL database
- Starts PostgreSQL service
- Sets postgres user password
- Creates MCP database

### Test 03: MCP Server Package Installation

- Installs `pgedge-postgres-mcp` from repository
- Installs `pgedge-nla-cli` package
- Installs `pgedge-nla-web` package
- Installs `pgedge-postgres-mcp-kb` package
- Updates `postgres-mcp.yaml` configuration
- Updates `postgres-mcp.env` configuration

### Test 04: Installation Validation

- Binary exists at `/usr/bin/pgedge-postgres-mcp`
- Config directory exists at `/etc/pgedge`
- Config files exist (`postgres-mcp.yaml`, `postgres-mcp.env`)
- Systemd service file installed at `/usr/lib/systemd/system/`
- Data directory exists at `/var/lib/pgedge/postgres-mcp`

### Test 05: Token Management

- Creates API token with `add-token` command
- Lists tokens with `list-tokens` command
- Verifies token file created correctly
- Validates token format

### Test 06: User Management

- Creates user with `add-user` command
- Lists users with `list-users` command
- Verifies user file created correctly
- Checks file permissions

### Test 07: Package Files and Permissions Verification

- Verifies all binaries in `/usr/bin`:
  - `pgedge-postgres-mcp` (755, root:root)
  - `pgedge-nla-kb-builder` (755, root:root)
  - `pgedge-nla-cli` (755, root:root)
- Verifies systemd service file:
  - `/usr/lib/systemd/system/pgedge-postgres-mcp.service` (644, root:root)
- Verifies `/usr/share/pgedge/nla-web` directory:
  - Directory exists with 755 permissions (root:root)
  - Lists all files inside the directory
- Verifies configuration files in `/etc/pgedge`:
  - `postgres-mcp.env` (644, root:root)
  - `pgedge-nla-kb-builder.yaml` (644, root:root)
  - `nla-cli.yaml` (644, root:root)
  - `postgres-mcp.yaml` (644, root:root)
- Verifies data directory:
  - `/var/lib/pgedge/postgres-mcp` (755, pgedge:pgedge)
- Verifies log directories:
  - `/var/log/pgedge` (755)
  - `/var/log/pgedge/postgres-mcp` (755, pgedge:pgedge)
  - `/var/log/pgedge/nla-web` (755, pgedge:pgedge)
  - Lists log files inside postgres-mcp and nla-web directories

## Configuration

### Using the Configuration Wizard (Recommended)

Run the interactive wizard:

```bash
./Setup_configuration
```

Or using make:

```bash
make Setup_configuration  # Full command
make config              # Short alias
```

This creates a `.test.env` file with your preferences. You can:
- View current config: `./Display_configuration` (or `make Display_configuration` or `make show-config`)
- Reconfigure anytime: `./Setup_configuration` (or `make Setup_configuration` or `make config`)
- Edit manually: `vim .test.env`

### Manual Configuration

Copy the template and edit:

```bash
cp .test.env.example .test.env
vim .test.env
```

Example `.test.env`:

```bash
TEST_EXEC_MODE=container-systemd
TEST_SERVER_ENV=live
TEST_OS_IMAGE=almalinux:10
TEST_LOG_LEVEL=minimal
TEST_TIMEOUT=30m
```

### Environment Variables (Legacy)

```bash
# Execution mode (optional, will prompt if not set)
export TEST_EXEC_MODE=container          # Standard container
export TEST_EXEC_MODE=container-systemd  # Systemd-enabled container
export TEST_EXEC_MODE=local              # Local machine

# Server environment
export TEST_SERVER_ENV=live              # Production (recommended)
export TEST_SERVER_ENV=staging           # Staging (may timeout)

# Docker image to test (for container modes only)
export TEST_OS_IMAGE=almalinux:10

# Log level
export TEST_LOG_LEVEL=minimal            # Summary only
export TEST_LOG_LEVEL=detailed           # Full output

# Test timeout
export TEST_TIMEOUT=30m

# CI mode (disables interactive prompts, defaults to container mode)
export CI=true
```

### Quick Override Targets

Use saved config but temporarily override specific settings:

```bash
make test-local      # Override to local mode
make test-staging    # Override to staging environment
make test-detailed   # Override to detailed logging
```

## Cleanup

```bash
# Remove any orphaned test containers
make clean

# Or manually
docker ps -a | grep mcp-test | awk '{print $1}' | xargs docker rm -f
```

## Troubleshooting

### Container fails to start

```bash
# Check Docker is running
docker ps

# Check image exists
docker images | grep debian
```

### Tests timeout

```bash
# Increase timeout
go test -v -timeout 30m
```

### See container logs on failure

Test automatically prints container logs when a test fails.

### Manual debugging

```bash
# Start container manually
docker run -it --rm debian:12 /bin/bash

# Run commands from test manually
apt-get update
apt-get install -y pgedge-postgres-mcp
```

## Adding New Tests

```go
func (s *RegressionTestSuite) Test06_YourNewTest() {
    s.T().Log("TEST 06: Your test description")

    // Install package first
    s.Test02_PackageInstallation()

    // Your test logic
    output, exitCode, err := s.container.Exec(s.ctx, "your-command")
    s.NoError(err)
    s.Equal(0, exitCode)

    s.T().Log("✓ Test passed")
}
```

## CI/CD Integration

```yaml
# .github/workflows/regression.yml
name: Regression Tests
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      - name: Run regression tests
        run: cd test/regression && make test-all
```

## Expected Output

```
=== RUN   TestRegressionSuite
=== RUN   TestRegressionSuite/Test01_RepositoryInstallation
    suite_test.go:XX: TEST 01: Installing pgEdge repository
    suite_test.go:XX: ✓ Repository installed successfully
=== RUN   TestRegressionSuite/Test02_PackageInstallation
    suite_test.go:XX: TEST 02: Installing MCP server package
    suite_test.go:XX: ✓ Package installed successfully
=== RUN   TestRegressionSuite/Test03_InstallationValidation
    suite_test.go:XX: TEST 03: Validating MCP server installation
    suite_test.go:XX: ✓ All installation validation checks passed
=== RUN   TestRegressionSuite/Test04_TokenManagement
    suite_test.go:XX: TEST 04: Testing token management commands
    suite_test.go:XX: ✓ Token management working correctly
=== RUN   TestRegressionSuite/Test05_UserManagement
    suite_test.go:XX: TEST 05: Testing user management commands
    suite_test.go:XX: ✓ User management working correctly
=== RUN   TestRegressionSuite/Test06_UserManagement
    suite_test.go:XX: TEST 06: Testing user management commands
    suite_test.go:XX: ✓ User management working correctly
=== RUN   TestRegressionSuite/Test07_PackageFilesVerification
    suite_test.go:XX: TEST 07: Verifying installed package files and permissions
    suite_test.go:XX: ✓ All package files and permissions verified successfully
--- PASS: TestRegressionSuite (145.67s)
    --- PASS: TestRegressionSuite/Test01_RepositoryInstallation (23.12s)
    --- PASS: TestRegressionSuite/Test02_PostgreSQLSetup (28.34s)
    --- PASS: TestRegressionSuite/Test03_MCPServerInstallation (31.45s)
    --- PASS: TestRegressionSuite/Test04_InstallationValidation (22.11s)
    --- PASS: TestRegressionSuite/Test05_TokenManagement (18.23s)
    --- PASS: TestRegressionSuite/Test06_UserManagement (12.34s)
    --- PASS: TestRegressionSuite/Test07_PackageFilesVerification (10.08s)
PASS
ok      pgedge-postgres-mcp/test/regression    145.678s
```

## File Structure

```
test/regression/
├── executor.go             # Executor interface and factory
├── container_executor.go   # Docker container executor
├── local_executor.go       # Local machine executor
├── docker_helper.go        # Legacy Docker helper (deprecated)
├── suite_test.go           # 7 comprehensive test cases
├── Makefile                # Test execution commands
├── go.mod                  # Go module definition
├── go.sum                  # Go dependency checksums
└── README.md               # This file
```

## Execution Mode Details

### Container Mode (Standard)

- Uses Docker without systemd
- Suitable for most package installation tests
- Does not support systemd service management tests
- Faster startup time (~2 seconds)

### Container Mode (Systemd)

- Uses Docker with systemd enabled
- Supports full systemd service testing
- Requires privileged container
- Slower startup time (~5 seconds)
- Ideal for testing service management

### Local Machine Mode

- Runs directly on the host system
- No Docker required
- Requires sudo access (passwordless sudo recommended)
- **WARNING:** Modifies system configuration
- Useful for testing on actual VMs or physical hardware
- Packages remain installed after tests complete
- Automatically installs required dependencies
- All system commands are automatically run with `sudo` when needed

## Next Steps

To expand the test suite, you can add:

- Service management tests (systemd start/stop/restart)
- HTTP mode functionality tests
- Database connection tests
- Configuration file validation
- Package upgrade/downgrade tests
- Package removal tests
- Security/permission tests
- Performance benchmarks

See the "Adding New Tests" section above for examples.
