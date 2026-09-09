/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package oauth

import (
	"net"
	"net/url"
)

// redirectURIAllowed reports whether uri is permitted for use as an OAuth
// redirect target, given the configured allowed list. Matching is exact,
// except that a loopback host (127.0.0.1, [::1] or localhost) may use any
// port so long as its scheme, host and path match an allowed entry with
// the same host.
func redirectURIAllowed(uri string, allowed []string) bool {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme == "" || u.Host == "" || u.Fragment != "" {
		return false
	}

	for _, a := range allowed {
		if a == uri {
			return true
		}
		au, err := url.Parse(a)
		if err != nil {
			continue
		}
		if isLoopbackHostname(u.Hostname()) && u.Hostname() == au.Hostname() &&
			u.Scheme == au.Scheme && u.Path == au.Path && u.RawQuery == "" && au.RawQuery == "" {
			return true
		}
	}
	return false
}

// isLoopbackHostname reports whether h names the local loopback address,
// either as "localhost" or as a loopback IP literal.
func isLoopbackHostname(h string) bool {
	if h == "localhost" {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}
