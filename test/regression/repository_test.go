package regression

import (
	"fmt"
	"strings"
)

// ========================================================================
// TEST 01: Repository Installation
// ========================================================================

// Test01_RepositoryInstallation tests pgEdge repository installation
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
			output, exitCode, err := s.execCmd(s.ctx, cmd)
			s.NoError(err, "Command failed: %s\nOutput: %s", cmd, output)
			s.Equal(0, exitCode, "Command exited with non-zero: %s\nOutput: %s", cmd, output)
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
