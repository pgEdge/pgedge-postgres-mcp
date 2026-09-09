//go:build linux

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

import "os/exec"

// openBrowser opens rawURL in the user's default browser. The URL is
// passed as a single argument to xdg-open, never through a shell, so it
// cannot be interpreted as anything other than a literal argument.
func openBrowser(rawURL string) error {
	return exec.Command("xdg-open", rawURL).Start()
}
