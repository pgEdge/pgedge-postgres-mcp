/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// The pl-do wrappers for PL/Perl embed the caller's tool arguments as a Perl
// string literal in the generated DO block. That literal must never be
// interpolated: in a double-quoted Perl string, $ and @ are live, so a value
// such as @{[ ... ]} would run as Perl inside the database (SPI access under
// plperl, operating system commands under plperlu). These tests call real
// Perl pl-do tools with such values and check that each one comes back
// exactly as sent and that its side effect never happened.

// perlInjectedSetting is the custom setting that the hostile payloads below
// would set if they were evaluated. The payloads set it transaction-locally,
// and the tool reads it back within the same DO block, so the check does not
// depend on which pooled connection serves the call.
const perlInjectedSetting = "mcp_test.injected"

// perlArgsToolCode reads the setting the payloads would have set and returns
// it alongside the argument value it was given. The same code is valid under
// both plperl and plperlu, since each wrapper defines $args and mcp_return.
const perlArgsToolCode = `my $r = spi_exec_query(q(SELECT coalesce(current_setting('` +
	perlInjectedSetting + `', true), 'no') AS v));
mcp_return({ value => $args->{value}, injected => $r->{rows}[0]{v} });`

// perlHostileValues are argument values that run Perl, or are otherwise
// altered, if the literal they are embedded in is interpolated.
func perlHostileValues() []string {
	setInjected := fmt.Sprintf(
		"spi_exec_query(q(SELECT set_config('%s', 'yes', true)))", perlInjectedSetting)
	return []string{
		"@{[ " + setInjected + " ]}",
		`${\ ` + setInjected + ` }`,
		"@{[ print STDERR qq(INJECTED) ]}",
		"$0",
		"@ARGV",
		`it's a \'quoted\' value with \\ backslashes\`,
	}
}

// ensurePerlLanguage creates the named PL/Perl extension if it is available,
// and reports whether it can be used. An extension the test created is
// dropped again on cleanup, so the database is left as it was found.
func ensurePerlLanguage(t *testing.T, ctx context.Context, pool *pgxpool.Pool, lang string) bool {
	t.Helper()

	var existed bool
	if err := pool.QueryRow(ctx,
		"SELECT EXISTS (SELECT 1 FROM pg_extension WHERE extname = $1)", lang).Scan(&existed); err != nil {
		t.Fatalf("failed to check for extension %s: %v", lang, err)
	}
	if existed {
		return true
	}

	// lang is one of two fixed names chosen by the test, never external
	// input, so it is safe to quote it as an identifier here.
	if _, err := pool.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS "+quoteIdentifier(lang)); err != nil {
		t.Logf("%s is not available on the test database: %v", lang, err)
		return false
	}
	t.Cleanup(func() {
		//nolint:errcheck // best-effort cleanup
		_, _ = pool.Exec(context.Background(), "DROP EXTENSION IF EXISTS "+quoteIdentifier(lang))
	})
	return true
}

// writePerlArgsConfig writes a definitions file with one pl-do tool per
// language, and a read-only server configuration that allows those
// languages, and returns the config path.
func writePerlArgsConfig(t *testing.T, connString string, langs []string) string {
	t.Helper()

	dir := t.TempDir()

	var defs strings.Builder
	defs.WriteString("tools:\n")
	for _, lang := range langs {
		fmt.Fprintf(&defs, `    - name: perl_args_%[1]s
      description: Returns its argument from a %[1]s DO block
      type: pl-do
      language: %[1]s
      input_schema:
          type: object
          properties:
              value:
                  type: string
                  description: Value to return
          required: [value]
      code: |
%[2]s
`, lang, indentLines(perlArgsToolCode, "          "))
	}

	defsPath := filepath.Join(dir, "definitions.yaml")
	if err := os.WriteFile(defsPath, []byte(defs.String()), 0o600); err != nil {
		t.Fatalf("failed to write definitions: %v", err)
	}

	cfg, err := pgxpool.ParseConfig(connString)
	if err != nil {
		t.Fatalf("failed to parse connection string: %v", err)
	}
	conn := cfg.ConnConfig

	port := int(conn.Port)
	if port == 0 {
		port = 5432
	}

	langsJSON, err := json.Marshal(langs)
	if err != nil {
		t.Fatalf("failed to marshal languages: %v", err)
	}

	configYAML := fmt.Sprintf(`
custom_definitions_path: %q

databases:
    - name: "test"
      host: %q
      port: %d
      database: %q
      user: %q
      password: %q
      sslmode: "disable"
      allow_writes: false
      allowed_pl_languages: %s
`, defsPath, conn.Host, port, conn.Database, conn.User, conn.Password, langsJSON)

	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(configYAML), 0o600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	return configPath
}

// indentLines prefixes every line of s with indent.
func indentLines(s, indent string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = indent + line
	}
	return strings.Join(lines, "\n")
}

// callToolWithArgs invokes a tool with the given arguments and returns its
// text content along with whether the server reported it as an error.
func callToolWithArgs(t *testing.T, server *MCPServer, name string, args map[string]interface{}) (string, bool) {
	t.Helper()

	resp, err := server.SendRequest("tools/call", map[string]interface{}{
		"name":      name,
		"arguments": args,
	})
	if err != nil {
		t.Fatalf("failed to call %s: %v", name, err)
	}
	if resp.Error != nil {
		return resp.Error.Message, true
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to unmarshal result for %s: %v", name, err)
	}

	var text strings.Builder
	for _, item := range result.Content {
		text.WriteString(item.Text)
	}
	return text.String(), result.IsError
}

func TestPerlPLDOArgsAreNotInterpolated(t *testing.T) {
	connString := os.Getenv("TEST_PGEDGE_POSTGRES_CONNECTION_STRING")
	if connString == "" {
		t.Skip("TEST_PGEDGE_POSTGRES_CONNECTION_STRING not set")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	var langs []string
	for _, lang := range []string{"plperl", "plperlu"} {
		if ensurePerlLanguage(t, ctx, pool, lang) {
			langs = append(langs, lang)
		}
	}
	if len(langs) == 0 {
		t.Skip("neither plperl nor plperlu is available on the test database")
	}

	server := startMCPServerWithConfig(t, writePerlArgsConfig(t, connString, langs))
	defer func() { _ = server.Close() }()

	for _, lang := range langs {
		values := perlHostileValues()
		if lang == "plperl" {
			// Only the trusted wrapper is checked with non-ASCII text: the
			// plperlu wrapper hands the literal to JSON::decode_json, which
			// expects UTF-8 bytes, and that is a separate matter from
			// interpolation.
			values = append(values, "unicode: café 日本")
		}

		for i, value := range values {
			t.Run(fmt.Sprintf("%s value %d", lang, i), func(t *testing.T) {
				text, isErr := callToolWithArgs(t, server, "perl_args_"+lang,
					map[string]interface{}{"value": value})
				if isErr {
					t.Fatalf("tool call failed for value %q: %s", value, text)
				}

				var got struct {
					Value    string `json:"value"`
					Injected string `json:"injected"`
				}
				if err := json.Unmarshal([]byte(text), &got); err != nil {
					t.Fatalf("tool result is not the expected JSON object: %v (got: %s)", err, text)
				}

				if got.Value != value {
					t.Errorf("argument was altered on the way into Perl:\n got: %q\nwant: %q", got.Value, value)
				}
				if got.Injected != "no" {
					t.Errorf("argument value was evaluated as Perl: %s was set to %q",
						perlInjectedSetting, got.Injected)
				}
			})
		}
	}
}
