package regression

import (
	"fmt"
	"time"
)

// ========================================================================
// TEST 02: PostgreSQL Installation and Setup
// ========================================================================

// Test02_PostgreSQLSetup tests PostgreSQL installation and configuration
func (s *RegressionTestSuite) Test02_PostgreSQLSetup() {
	s.T().Log("TEST 02: Installing and configuring pgEdge PostgreSQL")
	s.ensurePostgreSQLInstalled()
	s.T().Log("✓ PostgreSQL installed and configured successfully")
}

// installPostgreSQL performs the actual PostgreSQL installation
func (s *RegressionTestSuite) installPostgreSQL() {

	isDebian, _ := s.getOSType()

	// Step 1: Install PostgreSQL packages
	s.logDetailed("Step 1: Installing PostgreSQL %s packages", s.pgVersion)
	var pgPackages []string
	if isDebian {
		pgPackages = []string{
			fmt.Sprintf("apt-get install -y postgresql-%s", s.pgVersion),
		}
	} else {
		pgPackages = []string{
			fmt.Sprintf("dnf install -y pgedge-postgresql%s-server", s.pgVersion),
			fmt.Sprintf("dnf install -y pgedge-postgresql%s", s.pgVersion),
		}
	}

	for _, installCmd := range pgPackages {
		output, exitCode, err := s.execCmd(s.ctx, installCmd)
		s.NoError(err, "PostgreSQL install failed: %s\nOutput: %s", installCmd, output)
		s.Equal(0, exitCode, "PostgreSQL install exited with error: %s\nOutput: %s", installCmd, output)
	}

	// Step 2: Initialize PostgreSQL database
	s.logDetailed("Step 2: Initializing PostgreSQL %s database", s.pgVersion)
	if isDebian {
		// Debian/Ubuntu initialization
		// Use pg_ctlcluster with --skip-systemctl-redirect to bypass systemd
		// This avoids the systemd PID file ownership issue on Ubuntu/Debian
		startCmd := fmt.Sprintf("pg_ctlcluster --skip-systemctl-redirect %s main start", s.pgVersion)
		output, exitCode, err := s.execCmd(s.ctx, startCmd)
		s.NoError(err, "Failed to start PostgreSQL: %s", output)
		// Exit code 0 = started successfully, Exit code 2 = already running (both are OK)
		s.True(exitCode == 0 || exitCode == 2, "PostgreSQL start failed with unexpected exit code %d: %s", exitCode, output)
	} else {
		// RHEL/Rocky initialization (manual, not systemd)

		// Stop any existing PostgreSQL instances for cleanup
		s.logDetailed("  Stopping any existing PostgreSQL instances...")
		for _, ver := range []string{"16", "17", "18"} {
			stopCmd := fmt.Sprintf("su - postgres -c '/usr/pgsql-%s/bin/pg_ctl -D /var/lib/pgsql/%s/data stop' 2>/dev/null || true", ver, ver)
			s.execCmd(s.ctx, stopCmd) // Ignore errors
		}

		time.Sleep(2 * time.Second)

		// Remove existing data directory if it exists
		s.logDetailed("  Cleaning up existing data directory...")
		cleanupCmd := fmt.Sprintf("rm -rf /var/lib/pgsql/%s/data", s.pgVersion)
		s.execCmd(s.ctx, cleanupCmd)

		// Initialize database
		s.logDetailed("  Initializing new PostgreSQL %s database...", s.pgVersion)
		initCmd := fmt.Sprintf("su - postgres -c '/usr/pgsql-%s/bin/initdb -D /var/lib/pgsql/%s/data'", s.pgVersion, s.pgVersion)
		output, exitCode, err := s.execCmd(s.ctx, initCmd)
		s.NoError(err, "PostgreSQL initdb failed: %s", output)
		s.Equal(0, exitCode, "PostgreSQL initdb failed: %s", output)

		// Configure PostgreSQL to accept local connections
		configCmd := fmt.Sprintf(`echo "host all all 127.0.0.1/32 md5" >> /var/lib/pgsql/%s/data/pg_hba.conf`, s.pgVersion)
		output, exitCode, err = s.execCmd(s.ctx, configCmd)
		s.NoError(err, "Failed to configure pg_hba.conf: %s", output)
		s.Equal(0, exitCode, "pg_hba.conf config failed: %s", output)

		// Start PostgreSQL manually
		s.logDetailed("  Starting PostgreSQL %s...", s.pgVersion)
		startCmd := fmt.Sprintf("su - postgres -c '/usr/pgsql-%s/bin/pg_ctl -D /var/lib/pgsql/%s/data -l /var/lib/pgsql/%s/data/logfile start'", s.pgVersion, s.pgVersion, s.pgVersion)
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
