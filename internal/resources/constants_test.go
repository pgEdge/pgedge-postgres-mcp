/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package resources

import (
	"strings"
	"testing"
)

func TestDefaultQueryLimit(t *testing.T) {
	if DefaultQueryLimit != 100 {
		t.Errorf("expected DefaultQueryLimit to be 100, got %d", DefaultQueryLimit)
	}
}

func TestURIConstants(t *testing.T) {
	tests := []struct {
		name     string
		uri      string
		expected string
	}{
		{"URISystemInfo", URISystemInfo, "pg://system_info"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.uri != tt.expected {
				t.Errorf("expected %s = %q, got %q", tt.name, tt.expected, tt.uri)
			}
		})
	}
}

func TestURIFormat(t *testing.T) {
	// All resource URIs should follow pg:// scheme
	uris := []string{URISystemInfo}

	for _, uri := range uris {
		if !strings.HasPrefix(uri, "pg://") {
			t.Errorf("URI %q should have pg:// scheme", uri)
		}
	}
}
