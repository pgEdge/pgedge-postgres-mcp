/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

// Package sourceheader carries no code. It exists for the test below, which
// checks that every source file in the repository carries the copyright
// notice the project requires, in the shape it requires, so that a missing or
// misaligned header is caught here rather than noticed by eye some releases
// later. Twelve files had drifted to an unaligned banner and four had no
// notice at all before this test was written.
//
// The original pass covered only Go and JavaScript. A follow-up review found
// the same gap in eighteen more files the extension list did not reach at
// all: shell scripts, the two example chatbots' Python, the Windows
// installer's PowerShell, and four Debian maintainer scripts named by
// suffix rather than a conventional extension. All eighteen got the notice
// and their languages joined checkedExtensions below, on the same reasoning
// as the original fix: a gap nothing enforces is a gap that reopens.
package sourceheader

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// headerLines is how far into a file the notice must appear. It is generous
// because a banner may legitimately carry other content first, such as the
// usage text in web/scripts/take-screenshots.js.
const headerLines = 30

// copyrightLine and licenceLine are the two statements every source file must
// make. The year range is deliberately not matched, so that a file's own range
// need not be edited here.
const (
	copyrightLine = "Copyright (c)"
	licenceLine   = "This software is released under The PostgreSQL License"
)

// checkedExtensions are the source languages the rule covers. The Debian
// maintainer scripts are named "<package>.preinst" and so on rather than
// carrying a language extension, but filepath.Ext still isolates the
// suffix correctly, so they need no special-casing beyond being listed here.
var checkedExtensions = map[string]bool{
	".go": true, ".js": true, ".jsx": true, ".mjs": true,
	".sh": true, ".py": true, ".ps1": true,
	".preinst": true, ".postinst": true, ".prerm": true, ".postrm": true,
}

// skippedDirs are not ours to head: dependencies, build output, and anything
// in a dot-directory, which covers the git metadata and any worktrees kept
// inside the repository.
var skippedDirs = map[string]bool{
	"node_modules": true, "vendor": true, "bin": true, "dist": true,
	"third_party": true, "coverage": true,
}

// exemptFiles are configuration rather than source, which the rule excludes.
// Keep this list short, and prefer heading a file to exempting it.
var exemptFiles = map[string]bool{
	"web/vite.config.js":     true,
	"web/vitest.config.js":   true,
	"web/eslint.config.js":   true,
	"web/postcss.config.js":  true,
	"web/tailwind.config.js": true,
}

// repoRoot walks up from the test's working directory to the directory holding
// go.mod, so the test does not care where it is run from.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to determine the working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("Failed to find go.mod above the working directory")
		}
		dir = parent
	}
}

// sourceFiles returns every file the header rule applies to, as paths relative
// to the repository root and using forward slashes.
func sourceFiles(t *testing.T, root string) []string {
	t.Helper()
	var found []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		name := info.Name()
		if info.IsDir() {
			if skippedDirs[name] || (strings.HasPrefix(name, ".") && path != root) {
				return filepath.SkipDir
			}
			return nil
		}
		if !checkedExtensions[filepath.Ext(name)] || strings.HasSuffix(name, ".pb.go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if exemptFiles[rel] {
			return nil
		}
		found = append(found, rel)
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to walk %s: %v", root, err)
	}
	return found
}

// TestEverySourceFileCarriesTheNotice checks the notice is present. A file may
// use either the /* */ banner or // line comments, since the required style
// follows the language, but it must say both things.
func TestEverySourceFileCarriesTheNotice(t *testing.T) {
	root := repoRoot(t)
	files := sourceFiles(t, root)
	if len(files) == 0 {
		t.Fatal("Found no source files to check, so the walk is wrong")
	}

	for _, rel := range files {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Errorf("%s: failed to read: %v", rel, err)
			continue
		}
		lines := strings.Split(string(data), "\n")
		if len(lines) > headerLines {
			lines = lines[:headerLines]
		}
		head := strings.Join(lines, "\n")
		if !strings.Contains(head, copyrightLine) {
			t.Errorf("%s: no %q in the first %d lines; add the project header",
				rel, copyrightLine, headerLines)
		}
		if !strings.Contains(head, licenceLine) {
			t.Errorf("%s: no licence line in the first %d lines; add the project header",
				rel, headerLines)
		}
	}
}

// TestBannerHeadersAreAligned checks the shape of the /* */ form. Every line
// inside the banner has to start with " *", which is what drifted: a header
// whose continuation lines begin at column one still compiles, and gofmt has
// no opinion on it, so nothing else catches it.
func TestBannerHeadersAreAligned(t *testing.T) {
	root := repoRoot(t)

	for _, rel := range sourceFiles(t, root) {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Errorf("%s: failed to read: %v", rel, err)
			continue
		}
		lines := strings.Split(string(data), "\n")

		// Find the banner, which may sit below a build constraint.
		start := -1
		for i, line := range lines {
			if i >= headerLines {
				break
			}
			if strings.HasPrefix(line, "/*---") {
				start = i
				break
			}
		}
		if start == -1 {
			continue // Not the banner form; the notice test still applies.
		}

		for i := start + 1; i < len(lines); i++ {
			line := lines[i]
			if !strings.HasPrefix(line, " *") {
				t.Errorf("%s:%d: banner line must start with \" *\", got %q",
					rel, i+1, line)
			}
			if strings.HasSuffix(strings.TrimSpace(line), "*/") {
				break
			}
		}
	}
}
