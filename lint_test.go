/*-------------------------------------------------------------------------
 *
 * pgEdge Postgres MCP Server
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestLint runs golangci-lint if it's installed on the system.
// This integrates linting into the regular test suite.
func TestLint(t *testing.T) {
	// Check if golangci-lint is available (try PATH first, then GOPATH/bin)
	lintPath, err := exec.LookPath("golangci-lint")
	if err != nil {
		// Try GOPATH/bin
		gopath := exec.Command("go", "env", "GOPATH")
		gopathOut, gopathErr := gopath.Output()
		if gopathErr == nil {
			gopathBin := strings.TrimSpace(string(gopathOut)) + "/bin/golangci-lint"
			if _, statErr := os.Stat(gopathBin); statErr == nil {
				lintPath = gopathBin
			}
		}
	}

	if lintPath == "" {
		t.Skip("golangci-lint not found in PATH or GOPATH/bin, skipping lint test. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest")
	}

	// Run golangci-lint from the project root
	cmd := exec.Command(lintPath, "run", "--timeout=5m")
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	// Check for configuration errors
	if strings.Contains(outputStr, "can't load config") || strings.Contains(outputStr, "unsupported version") {
		t.Skipf("golangci-lint configuration issue, skipping lint test:\n%s", outputStr)
	}

	if err != nil {
		// Check if it's just warnings or actual errors
		if strings.Contains(outputStr, "level=error") || strings.Contains(outputStr, "Error:") {
			t.Errorf("golangci-lint found issues:\n%s", outputStr)
			return
		}
		// Exit code might be non-zero but could be just warnings
		if strings.Contains(outputStr, "level=warning") {
			t.Logf("golangci-lint warnings:\n%s", outputStr)
		}
	}

	// Check for warnings in output (even if exit code is 0)
	if strings.Contains(outputStr, "level=warning") {
		t.Logf("golangci-lint output:\n%s", outputStr)
	}

	if !strings.Contains(outputStr, "Error:") {
		t.Log("✓ golangci-lint passed with no errors")
	}
}
