.PHONY: test test-debian test-rocky test-ubuntu test-all clean help
.PHONY: test-local test-systemd test-container

# Default test
test: test-debian

# Test on Debian 12 (container mode)
test-debian:
	@echo "=== Testing on Debian 12 ==="
	TEST_OS_IMAGE=debian:12 go test -v -timeout 20m

# Test on Rocky Linux (detects local version or uses container)
test-rocky:
	@echo "=== Testing on Rocky Linux ==="
	@if [ "$(TEST_EXEC_MODE)" = "local" ]; then \
		TEST_EXEC_MODE=local go test -v -timeout 20m; \
	else \
		TEST_OS_IMAGE=rockylinux:9 go test -v -timeout 20m; \
	fi

# Test on Ubuntu 22.04 (container mode)
test-ubuntu:
	@echo "=== Testing on Ubuntu 22.04 ==="
	TEST_OS_IMAGE=ubuntu:22.04 go test -v -timeout 20m

# Test on all platforms (container mode)
test-all: test-debian test-rocky test-ubuntu

# Test with systemd-enabled container
test-systemd:
	@echo "=== Testing with systemd-enabled container ==="
	TEST_EXEC_MODE=container-systemd TEST_OS_IMAGE=debian:12 go test -v -timeout 20m

# Test on local machine
test-local:
	@echo "=== Testing on local machine ==="
	@echo "WARNING: This will install packages on your local system!"
	@echo "Press Ctrl+C to cancel, or Enter to continue..."
	@read dummy
	TEST_EXEC_MODE=local go test -v -timeout 20m

# Test in standard container mode (explicit)
test-container:
	@echo "=== Testing in standard container mode ==="
	TEST_EXEC_MODE=container TEST_OS_IMAGE=debian:12 go test -v -timeout 20m

# Run specific test
test-one:
	@echo "=== Running specific test ==="
	@echo "Usage: make test-one TEST=Test01_RepositoryInstallation"
	TEST_OS_IMAGE=debian:12 go test -v -run $(TEST) -timeout 10m

# Clean up any orphaned containers
clean:
	@echo "Cleaning up test containers..."
	docker ps -a | grep mcp-test | awk '{print $$1}' | xargs -r docker rm -f || true

# Show help
help:
	@echo "Available targets:"
	@echo ""
	@echo "Execution modes:"
	@echo "  make test-container  - Run in standard container (default)"
	@echo "  make test-systemd    - Run in systemd-enabled container"
	@echo "  make test-local      - Run on local machine (requires sudo)"
	@echo ""
	@echo "Platform-specific tests (container mode):"
	@echo "  make test            - Run tests on Debian 12 (default)"
	@echo "  make test-debian     - Run tests on Debian 12"
	@echo "  make test-rocky      - Run tests on Rocky Linux 9"
	@echo "  make test-ubuntu     - Run tests on Ubuntu 22.04"
	@echo "  make test-all        - Run tests on all platforms"
	@echo ""
	@echo "Other targets:"
	@echo "  make test-one        - Run specific test (set TEST=TestName)"
	@echo "  make clean           - Clean up orphaned containers"
	@echo "  make help            - Show this help message"
	@echo ""
	@echo "Environment variables:"
	@echo "  TEST_EXEC_MODE       - Execution mode: container, container-systemd, local"
	@echo "  TEST_OS_IMAGE        - Docker image to test (e.g., debian:12)"
	@echo "  TEST_LOG_LEVEL       - Log verbosity: minimal (summary only) or detailed (default)"
	@echo "  PGEDGE_REPO_URL      - Repository URL for packages"
	@echo "  CI                   - Set to disable interactive prompts"
	@echo ""
	@echo "Examples:"
	@echo "  make test-systemd                                    # Systemd container"
	@echo "  TEST_OS_IMAGE=ubuntu:22.04 make test-systemd         # Ubuntu with systemd"
	@echo "  TEST_EXEC_MODE=local make test                       # Local execution"
	@echo "  TEST_LOG_LEVEL=minimal make test-rocky               # Summary output only"
