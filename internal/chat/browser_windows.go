//go:build windows

/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package chat

import (
	"os"
	"os/exec"
	"path/filepath"
)

// openBrowser opens rawURL in the user's default browser. The URL is
// passed as a single argument to rundll32, never through a shell, so it
// cannot be interpreted as anything other than a literal argument, and
// rundll32 itself is named by its absolute path under the system
// directory rather than looked up on PATH, which on Windows includes
// the current directory.
func openBrowser(rawURL string) error {
	systemRoot := os.Getenv("SystemRoot")
	if systemRoot == "" {
		systemRoot = `C:\Windows`
	}
	rundll32 := filepath.Join(systemRoot, "System32", "rundll32.exe")
	return exec.Command(rundll32, "url.dll,FileProtocolHandler", rawURL).Start()
}
