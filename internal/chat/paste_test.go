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
	"errors"
	"io"
	"testing"

	"github.com/chzyer/readline"
)

// scriptedReader returns each scripted result in turn, then io.EOF forever.
type scriptedReader struct {
	lines []string
	errs  []error
}

func (r *scriptedReader) Readline() (string, error) {
	if len(r.lines) == 0 {
		return "", io.EOF
	}
	line, err := r.lines[0], r.errs[0]
	r.lines, r.errs = r.lines[1:], r.errs[1:]
	return line, err
}

// script builds a scriptedReader from a sequence of lines (strings) and
// terminal events (errors), in the order readline would report them.
func script(steps ...any) *scriptedReader {
	r := &scriptedReader{}
	for _, s := range steps {
		switch v := s.(type) {
		case string:
			r.lines = append(r.lines, v)
			r.errs = append(r.errs, nil)
		case error:
			r.lines = append(r.lines, "")
			r.errs = append(r.errs, v)
		default:
			panic("script steps must be strings or errors")
		}
	}
	return r
}

func TestCollectPastedInput(t *testing.T) {
	boom := errors.New("terminal went away")

	tests := []struct {
		name        string
		reader      *scriptedReader
		wantText    string
		wantAborted bool
		wantErr     error
	}{
		{
			name: "multi-line SQL joined with newlines on Ctrl+D",
			reader: script(
				"SELECT empno,",
				"       ename",
				"FROM emp",
				"WHERE deptno = 10;",
				io.EOF,
			),
			wantText: "SELECT empno,\n       ename\nFROM emp\nWHERE deptno = 10;",
		},
		{
			name:     "leading and trailing blank lines are trimmed, inner ones kept",
			reader:   script("", "first", "", "second", "", io.EOF),
			wantText: "first\n\nsecond",
		},
		{
			name:     "indentation on the first and last lines is preserved",
			reader:   script("  ", "    def f():", "        return 1  ", "\t", io.EOF),
			wantText: "    def f():\n        return 1  ",
		},
		{
			name:     "lines starting with a slash are content, not commands",
			reader:   script("/help me write this", "/quit is not a command here", io.EOF),
			wantText: "/help me write this\n/quit is not a command here",
		},
		{
			name:     "Ctrl+D with nothing pasted yields empty text",
			reader:   script(io.EOF),
			wantText: "",
		},
		{
			name:        "Ctrl+C discards collected lines and reports abort",
			reader:      script("SELECT 1;", "SELECT 2;", readline.ErrInterrupt),
			wantAborted: true,
		},
		{
			name:    "other errors are returned",
			reader:  script("SELECT 1;", boom),
			wantErr: boom,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, aborted, err := collectPastedInput(tt.reader)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if aborted != tt.wantAborted {
				t.Errorf("aborted = %v, want %v", aborted, tt.wantAborted)
			}
			if text != tt.wantText {
				t.Errorf("text = %q, want %q", text, tt.wantText)
			}
		})
	}
}

func TestGetContinuationPrompt(t *testing.T) {
	ui := NewUI(true, false)
	if got := ui.GetContinuationPrompt(); got != "...: " {
		t.Errorf("GetContinuationPrompt() = %q, want %q", got, "...: ")
	}
	if got, want := len(ui.GetContinuationPrompt()), len(ui.GetPrompt()); got != want {
		t.Errorf("continuation prompt width %d does not match main prompt width %d", got, want)
	}
}
