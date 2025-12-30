CONFIG_FILE := .test.env
EXAMPLE_FILE := .test.env.example

# Try to load config file (don't error if missing)
-include $(CONFIG_FILE)

# Set defaults if not loaded from config file
TEST_EXEC_MODE ?= container-systemd
TEST_SERVER_ENV ?= live
TEST_OS_IMAGE ?= almalinux:10
TEST_LOG_LEVEL ?= minimal
TEST_TIMEOUT ?= 30m

# Export all variables to subprocesses
export

.PHONY: deps
deps:
	@echo "📦 Downloading Go module dependencies..."
	@go mod download 2>&1 | grep -q "." && echo "   Downloaded modules" || echo "   ✓ All dependencies cached"
	@echo "✓ Dependencies ready"

# Interactive configuration wizard
.PHONY: config Setup_configuration
Setup_configuration config:
	@echo "════════════════════════════════════════════════════════════════"
	@echo "  pgEdge MCP Regression Test Suite - Configuration Wizard"
	@echo "════════════════════════════════════════════════════════════════"
	@echo ""
	@echo "This wizard will create a .test.env configuration file."
	@echo "Press Enter to accept the default value shown in [brackets]."
	@echo ""
	@echo "────────────────────────────────────────────────────────────────"
	@echo "1. EXECUTION MODE"
	@echo "────────────────────────────────────────────────────────────────"
	@echo "Where should tests run?"
	@echo "  • local            - Run tests on your local machine (fast setup)"
	@echo "  • container-systemd - Run tests in Docker container (isolated, portable)"
	@echo ""
	@read -p "Execution mode [container-systemd]: " exec_mode; \
	exec_mode=$${exec_mode:-container-systemd}; \
	echo ""; \
	echo "────────────────────────────────────────────────────────────────"; \
	echo "2. SERVER ENVIRONMENT"; \
	echo "────────────────────────────────────────────────────────────────"; \
	echo "Which repository should be used?"; \
	echo "  • live    - Production repository (recommended, fast, reliable)"; \
	echo "  • staging - Staging repository (may be slow or timeout)"; \
	echo ""; \
	read -p "Server environment [live]: " server_env; \
	server_env=$${server_env:-live}; \
	echo ""; \
	if [ "$$exec_mode" = "container-systemd" ]; then \
		echo "────────────────────────────────────────────────────────────────"; \
		echo "3. CONTAINER OS IMAGE"; \
		echo "────────────────────────────────────────────────────────────────"; \
		echo "Which operating system should be tested? (container mode only)"; \
		echo "  • almalinux:10 - AlmaLinux 10 (RHEL-based, recommended)"; \
		echo "  • almalinux:9  - AlmaLinux 9"; \
		echo "  • ubuntu:24.04 - Ubuntu 24.04 (Debian-based)"; \
		echo "  • debian:12    - Debian 12"; \
		echo "  • rocky:9      - Rocky Linux 9"; \
		echo ""; \
		read -p "OS image [almalinux:10]: " os_image; \
		os_image=$${os_image:-almalinux:10}; \
	else \
		os_image="n/a"; \
	fi; \
	echo ""; \
	echo "────────────────────────────────────────────────────────────────"; \
	echo "4. LOG LEVEL"; \
	echo "────────────────────────────────────────────────────────────────"; \
	echo "How verbose should test output be?"; \
	echo "  • minimal  - Show only summary and failures (recommended)"; \
	echo "  • detailed - Show all test steps and command output (for debugging)"; \
	echo ""; \
	read -p "Log level [minimal]: " log_level; \
	log_level=$${log_level:-minimal}; \
	echo ""; \
	echo "────────────────────────────────────────────────────────────────"; \
	echo "5. TEST TIMEOUT"; \
	echo "────────────────────────────────────────────────────────────────"; \
	echo "Maximum time to allow for full test suite:"; \
	echo "  • 30m - 30 minutes (recommended for 'live' environment)"; \
	echo "  • 60m - 60 minutes (recommended for 'staging' environment)"; \
	echo "  • Format: 30m, 1h, 90m, etc."; \
	echo ""; \
	read -p "Timeout [30m]: " timeout; \
	timeout=$${timeout:-30m}; \
	echo ""; \
	echo "════════════════════════════════════════════════════════════════"; \
	echo "  Configuration Summary"; \
	echo "════════════════════════════════════════════════════════════════"; \
	echo "  Execution Mode:  $$exec_mode"; \
	echo "  Server Env:      $$server_env"; \
	if [ "$$exec_mode" = "container-systemd" ]; then \
		echo "  OS Image:        $$os_image"; \
	fi; \
	echo "  Log Level:       $$log_level"; \
	echo "  Timeout:         $$timeout"; \
	echo "════════════════════════════════════════════════════════════════"; \
	echo ""; \
	read -p "Save this configuration? [Y/n]: " confirm; \
	confirm=$${confirm:-Y}; \
	if [ "$$confirm" = "Y" ] || [ "$$confirm" = "y" ]; then \
		echo "# pgEdge MCP Regression Test Configuration" > $(CONFIG_FILE); \
		echo "# Generated by 'make config' on $$(date)" >> $(CONFIG_FILE); \
		echo "" >> $(CONFIG_FILE); \
		echo "TEST_EXEC_MODE=$$exec_mode" >> $(CONFIG_FILE); \
		echo "TEST_SERVER_ENV=$$server_env" >> $(CONFIG_FILE); \
		echo "TEST_OS_IMAGE=$$os_image" >> $(CONFIG_FILE); \
		echo "TEST_LOG_LEVEL=$$log_level" >> $(CONFIG_FILE); \
		echo "TEST_TIMEOUT=$$timeout" >> $(CONFIG_FILE); \
		echo ""; \
		echo "✓ Configuration saved to $(CONFIG_FILE)"; \
		echo ""; \
		echo "════════════════════════════════════════════════════════════════"; \
		echo "  Next Steps"; \
		echo "════════════════════════════════════════════════════════════════"; \
		echo "Run tests using any of these commands:"; \
		echo ""; \
		echo "  ./Execute_Regression_suite    (Simplest - no 'make' needed)"; \
		echo "  make test                      (Short alias)"; \
		echo "  make Execute_Regression_suite  (Full command)"; \
		echo "════════════════════════════════════════════════════════════════"; \
	else \
		echo ""; \
		echo "✗ Configuration not saved."; \
	fi

# Check if config exists, if not, prompt user
.PHONY: check-config
check-config:
	@if [ ! -f $(CONFIG_FILE) ]; then \
		echo "════════════════════════════════════════════════════════════════"; \
		echo "  ⚠️  Configuration Required"; \
		echo "════════════════════════════════════════════════════════════════"; \
		echo ""; \
		echo "No configuration file found: $(CONFIG_FILE)"; \
		echo ""; \
		echo "Please run the configuration wizard:"; \
		echo ""; \
		echo "  ./Setup_configuration    (Simplest - no 'make' needed)"; \
		echo "  make config              (Short alias)"; \
		echo "  make Setup_configuration (Full command)"; \
		echo ""; \
		echo "This will guide you through interactive configuration."; \
		echo "════════════════════════════════════════════════════════════════"; \
		exit 1; \
	fi

.PHONY: test Execute_Regression_suite
Execute_Regression_suite test: check-config deps
	@echo ""
	@echo "🧪 Running regression tests..."
	@echo "   Mode: $(TEST_EXEC_MODE)"
	@echo "   Server: $(TEST_SERVER_ENV)"
	@if [ "$(TEST_EXEC_MODE)" = "local" ]; then \
		echo "   OS: System (auto-detect)"; \
	else \
		echo "   OS: $(TEST_OS_IMAGE)"; \
	fi
	@echo "   Log: $(TEST_LOG_LEVEL)"
	@echo ""
	@go test -run TestRegressionSuite -timeout $(TEST_TIMEOUT)

# Convenience targets for quick overrides
.PHONY: test-local
test-local: check-config deps
	@echo ""
	@echo "🧪 Running regression tests (local mode override)..."
	@TEST_EXEC_MODE=local go test -run TestRegressionSuite -timeout $(TEST_TIMEOUT)

.PHONY: test-staging
test-staging: check-config deps
	@echo ""
	@echo "🧪 Running regression tests (staging environment override)..."
	@TEST_SERVER_ENV=staging go test -run TestRegressionSuite -timeout $(TEST_TIMEOUT)

.PHONY: test-detailed
test-detailed: check-config deps
	@echo ""
	@echo "🧪 Running regression tests (detailed logging override)..."
	@TEST_LOG_LEVEL=detailed go test -run TestRegressionSuite -timeout $(TEST_TIMEOUT)

# Show current configuration
.PHONY: show-config Display_configuration
Display_configuration show-config:
	@if [ -f $(CONFIG_FILE) ]; then \
		echo "════════════════════════════════════════════════════════════════"; \
		echo "  Current Configuration ($(CONFIG_FILE))"; \
		echo "════════════════════════════════════════════════════════════════"; \
		cat $(CONFIG_FILE) | grep "^TEST_" | sed 's/TEST_/  /'; \
		echo "════════════════════════════════════════════════════════════════"; \
		echo ""; \
		echo "To modify: make config"; \
		echo "To edit manually: vim $(CONFIG_FILE)"; \
	else \
		echo "════════════════════════════════════════════════════════════════"; \
		echo "  No Configuration File Found"; \
		echo "════════════════════════════════════════════════════════════════"; \
		echo ""; \
		echo "Run: make config"; \
	fi

.PHONY: clean
clean:
	@echo "🧹 Cleaning test cache and containers..."
	@go clean -testcache
	@docker ps -a --filter "name=mcp-test-" -q | xargs -r docker rm -f 2>/dev/null || true
	@echo "✓ Cleaned"

.PHONY: help
help:
	@echo "════════════════════════════════════════════════════════════════"
	@echo "  pgEdge MCP Regression Test Suite - Available Commands"
	@echo "════════════════════════════════════════════════════════════════"
	@echo ""
	@echo "Direct Commands (No 'make' needed):"
	@echo "  ./Setup_configuration      - Configure test settings (required first time)"
	@echo "  ./Execute_Regression_suite - Run regression tests"
	@echo "  ./Display_configuration    - Show current configuration"
	@echo ""
	@echo "Make Commands (Alternative):"
	@echo "  make Setup_configuration   - Same as ./Setup_configuration"
	@echo "  make Execute_Regression_suite - Same as ./Execute_Regression_suite"
	@echo "  make Display_configuration - Same as ./Display_configuration"
	@echo ""
	@echo "Short Aliases:"
	@echo "  make config                - Same as ./Setup_configuration"
	@echo "  make test                  - Same as ./Execute_Regression_suite"
	@echo "  make show-config           - Same as ./Display_configuration"
	@echo ""
	@echo "Quick Overrides:"
	@echo "  make test-local            - Run tests on local machine (override)"
	@echo "  make test-staging          - Run tests against staging server (override)"
	@echo "  make test-detailed         - Run tests with detailed logging (override)"
	@echo ""
	@echo "Maintenance:"
	@echo "  make deps                  - Download Go module dependencies"
	@echo "  make clean                 - Clean test cache and containers"
	@echo "  make help                  - Show this help"
	@echo ""
	@echo "════════════════════════════════════════════════════════════════"
	@echo ""
	@echo "Quick Start (Simplest):"
	@echo "  1. ./Setup_configuration     # Configure test settings"
	@echo "  2. ./Execute_Regression_suite # Run tests"
	@echo ""
	@echo "Quick Start (Using make):"
	@echo "  1. make config               # Configure test settings"
	@echo "  2. make test                 # Run tests"
	@echo ""
	@echo "════════════════════════════════════════════════════════════════"
