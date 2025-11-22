/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package database

import (
	"testing"
)

func TestParseQueryForConnection(t *testing.T) {
	tests := []struct {
		name                     string
		query                    string
		expectedCleanedQuery     string
		expectedConnectionString string
		expectedSetAsDefault     bool
	}{
		{
			name:                     "Simple query without connection",
			query:                    "Show me all users",
			expectedCleanedQuery:     "Show me all users",
			expectedConnectionString: "",
			expectedSetAsDefault:     false,
		},
		{
			name:                     "Query with 'at' connection string",
			query:                    "Show users at postgres://localhost/mydb",
			expectedCleanedQuery:     "Show users",
			expectedConnectionString: "postgres://localhost/mydb",
			expectedSetAsDefault:     false,
		},
		{
			name:                     "Query with 'from' connection string",
			query:                    "Get all tables from postgres://host:5432/testdb",
			expectedCleanedQuery:     "Get all tables",
			expectedConnectionString: "postgres://host:5432/testdb",
			expectedSetAsDefault:     false,
		},
		{
			name:                     "Query with 'on' connection string",
			query:                    "List databases on postgresql://server/postgres",
			expectedCleanedQuery:     "List databases",
			expectedConnectionString: "postgresql://server/postgres",
			expectedSetAsDefault:     false,
		},
		{
			name:                     "Set default database command",
			query:                    "set default database to postgres://localhost/newdb",
			expectedCleanedQuery:     "",
			expectedConnectionString: "postgres://localhost/newdb",
			expectedSetAsDefault:     true,
		},
		{
			name:                     "Use database command",
			query:                    "use database postgres://host/db",
			expectedCleanedQuery:     "",
			expectedConnectionString: "postgres://host/db",
			expectedSetAsDefault:     true,
		},
		{
			name:                     "Switch to database command",
			query:                    "switch to postgres://myhost/mydb",
			expectedCleanedQuery:     "",
			expectedConnectionString: "postgres://myhost/mydb",
			expectedSetAsDefault:     true,
		},
		{
			name:                     "Database prefix pattern",
			query:                    "database postgres://localhost/test show all tables",
			expectedCleanedQuery:     "show all tables",
			expectedConnectionString: "postgres://localhost/test",
			expectedSetAsDefault:     false,
		},
		{
			name:                     "DB prefix pattern",
			query:                    "db postgres://localhost/mydb get schema info",
			expectedCleanedQuery:     "get schema info",
			expectedConnectionString: "postgres://localhost/mydb",
			expectedSetAsDefault:     false,
		},
		{
			name:                     "Connection string with user and password",
			query:                    "Show tables at postgres://user:pass@localhost:5432/db",
			expectedCleanedQuery:     "Show tables",
			expectedConnectionString: "postgres://user:pass@localhost:5432/db",
			expectedSetAsDefault:     false,
		},
		{
			name:                     "Case insensitive patterns",
			query:                    "SHOW USERS FROM postgres://localhost/db",
			expectedCleanedQuery:     "SHOW USERS",
			expectedConnectionString: "postgres://localhost/db",
			expectedSetAsDefault:     false,
		},
		{
			name:                     "Query with 'for' connection string",
			query:                    "Get statistics for postgres://host/db",
			expectedCleanedQuery:     "Get statistics",
			expectedConnectionString: "postgres://host/db",
			expectedSetAsDefault:     false,
		},
		{
			name:                     "Query with 'in' connection string",
			query:                    "Count rows in postgres://host/db",
			expectedCleanedQuery:     "Count rows",
			expectedConnectionString: "postgres://host/db",
			expectedSetAsDefault:     false,
		},
		{
			name:                     "PostgreSQL prefix variant",
			query:                    "Show data from postgresql://localhost/mydb",
			expectedCleanedQuery:     "Show data",
			expectedConnectionString: "postgresql://localhost/mydb",
			expectedSetAsDefault:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseQueryForConnection(tt.query)

			if result.CleanedQuery != tt.expectedCleanedQuery {
				t.Errorf("CleanedQuery = %q, want %q", result.CleanedQuery, tt.expectedCleanedQuery)
			}

			if result.ConnectionString != tt.expectedConnectionString {
				t.Errorf("ConnectionString = %q, want %q", result.ConnectionString, tt.expectedConnectionString)
			}

			if result.SetAsDefault != tt.expectedSetAsDefault {
				t.Errorf("SetAsDefault = %v, want %v", result.SetAsDefault, tt.expectedSetAsDefault)
			}
		})
	}
}

func TestIsSetDefaultCommand(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected bool
	}{
		{
			name:     "set default prefix",
			query:    "set default database to postgres://localhost/db",
			expected: true,
		},
		{
			name:     "use database prefix",
			query:    "use database postgres://localhost/db",
			expected: true,
		},
		{
			name:     "switch to database prefix",
			query:    "switch to database postgres://localhost/db",
			expected: true,
		},
		{
			name:     "change database to prefix",
			query:    "change database to postgres://localhost/db",
			expected: true,
		},
		{
			name:     "case insensitive - uppercase",
			query:    "SET DEFAULT database to postgres://localhost/db",
			expected: true,
		},
		{
			name:     "case insensitive - mixed case",
			query:    "Use Database postgres://localhost/db",
			expected: true,
		},
		{
			name:     "with leading whitespace",
			query:    "  set default database to postgres://localhost/db",
			expected: true,
		},
		{
			name:     "regular query",
			query:    "show all tables",
			expected: false,
		},
		{
			name:     "query containing 'set' but not a command",
			query:    "get the dataset from table",
			expected: false,
		},
		{
			name:     "query containing 'use' but not a command",
			query:    "what can I use to query this",
			expected: false,
		},
		{
			name:     "empty query",
			query:    "",
			expected: false,
		},
		{
			name:     "whitespace only",
			query:    "   ",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsSetDefaultCommand(tt.query)
			if result != tt.expected {
				t.Errorf("IsSetDefaultCommand(%q) = %v, want %v", tt.query, result, tt.expected)
			}
		})
	}
}
