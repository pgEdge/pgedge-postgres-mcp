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
	"strings"
)

// redirectURIAllowed reports whether uri is permitted for use as an OAuth
// redirect target, given the configured allowed list. Matching is exact
// (scheme and host compared case-insensitively; everything else,
// including the path in its percent-encoded form, byte-for-byte), except
// that a loopback host (127.0.0.1, [::1] or localhost) may use any port
// so long as its scheme, host and path match an allowed entry with the
// same host. A URI carrying
// userinfo (e.g. "http://attacker@127.0.0.1/callback") is always rejected,
// since a browser silently drops it and it exists only to mislead a
// reviewer of the URI. So is one containing "#", even with an empty
// fragment, since RFC 6749 section 3.1.2 forbids fragments outright.
func redirectURIAllowed(uri string, allowed []string) bool {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme == "" || u.Host == "" || u.User != nil || strings.Contains(uri, "#") {
		return false
	}

	for _, a := range allowed {
		au, err := url.Parse(a)
		if err != nil {
			continue
		}
		if strings.EqualFold(u.Scheme, au.Scheme) && strings.EqualFold(u.Host, au.Host) &&
			u.EscapedPath() == au.EscapedPath() && u.RawQuery == au.RawQuery {
			return true
		}
		if isLoopbackHostname(u.Hostname()) && strings.EqualFold(u.Hostname(), au.Hostname()) &&
			strings.EqualFold(u.Scheme, au.Scheme) && u.EscapedPath() == au.EscapedPath() && u.RawQuery == "" && au.RawQuery == "" {
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
