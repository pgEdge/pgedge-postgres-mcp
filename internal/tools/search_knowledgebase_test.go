/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package tools

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestDeserializeEmbedding(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want []float32
	}{
		{
			name: "valid embedding",
			data: func() []byte {
				buf := make([]byte, 12) // 3 float32s
				binary.LittleEndian.PutUint32(buf[0:], math.Float32bits(1.0))
				binary.LittleEndian.PutUint32(buf[4:], math.Float32bits(2.0))
				binary.LittleEndian.PutUint32(buf[8:], math.Float32bits(3.0))
				return buf
			}(),
			want: []float32{1.0, 2.0, 3.0},
		},
		{
			name: "empty data",
			data: []byte{},
			want: nil,
		},
		{
			name: "nil data",
			data: nil,
			want: nil,
		},
		{
			name: "invalid length not multiple of 4",
			data: []byte{1, 2, 3},
			want: nil,
		},
		{
			name: "single float",
			data: func() []byte {
				buf := make([]byte, 4)
				binary.LittleEndian.PutUint32(buf, math.Float32bits(0.5))
				return buf
			}(),
			want: []float32{0.5},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deserializeEmbedding(tt.data)
			if len(got) != len(tt.want) {
				t.Errorf("deserializeEmbedding() returned %d elements, want %d", len(got), len(tt.want))
				return
			}
			for i, v := range got {
				if v != tt.want[i] {
					t.Errorf("deserializeEmbedding()[%d] = %f, want %f", i, v, tt.want[i])
				}
			}
		})
	}
}

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name string
		a    []float32
		b    []float32
		want float64
	}{
		{
			name: "identical vectors",
			a:    []float32{1.0, 0.0, 0.0},
			b:    []float32{1.0, 0.0, 0.0},
			want: 1.0,
		},
		{
			name: "orthogonal vectors",
			a:    []float32{1.0, 0.0, 0.0},
			b:    []float32{0.0, 1.0, 0.0},
			want: 0.0,
		},
		{
			name: "opposite vectors",
			a:    []float32{1.0, 0.0, 0.0},
			b:    []float32{-1.0, 0.0, 0.0},
			want: -1.0,
		},
		{
			name: "same direction different magnitude",
			a:    []float32{1.0, 2.0, 3.0},
			b:    []float32{2.0, 4.0, 6.0},
			want: 1.0,
		},
		{
			name: "different lengths returns 0",
			a:    []float32{1.0, 2.0},
			b:    []float32{1.0, 2.0, 3.0},
			want: 0.0,
		},
		{
			name: "zero vector a",
			a:    []float32{0.0, 0.0, 0.0},
			b:    []float32{1.0, 2.0, 3.0},
			want: 0.0,
		},
		{
			name: "zero vector b",
			a:    []float32{1.0, 2.0, 3.0},
			b:    []float32{0.0, 0.0, 0.0},
			want: 0.0,
		},
		{
			name: "empty vectors",
			a:    []float32{},
			b:    []float32{},
			want: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cosineSimilarity(tt.a, tt.b)
			if math.Abs(got-tt.want) > 1e-6 {
				t.Errorf("cosineSimilarity() = %f, want %f", got, tt.want)
			}
		})
	}
}

func TestFormatKBResults(t *testing.T) {
	tests := []struct {
		name            string
		results         []KBSearchResult
		query           string
		projectNames    []string
		projectVersions []string
		wantContains    []string
	}{
		{
			name: "basic results",
			results: []KBSearchResult{
				{
					Text:           "Test content",
					Title:          "Test Title",
					Section:        "Section 1",
					ProjectName:    "PostgreSQL",
					ProjectVersion: "17",
					Similarity:     0.95,
				},
			},
			query:           "test query",
			projectNames:    nil,
			projectVersions: nil,
			wantContains: []string{
				`"test query"`,
				"Test content",
				"Test Title",
				"PostgreSQL",
				"0.950",
			},
		},
		{
			name: "with project filter",
			results: []KBSearchResult{
				{
					Text:        "Content",
					ProjectName: "pgEdge",
					Similarity:  0.85,
				},
			},
			query:           "search",
			projectNames:    []string{"pgEdge"},
			projectVersions: nil,
			wantContains: []string{
				"Filter - Projects: pgEdge",
			},
		},
		{
			name: "with version filter",
			results: []KBSearchResult{
				{
					Text:           "Content",
					ProjectName:    "PostgreSQL",
					ProjectVersion: "16",
					Similarity:     0.90,
				},
			},
			query:           "search",
			projectNames:    []string{"PostgreSQL"},
			projectVersions: []string{"16"},
			wantContains: []string{
				"Filter - Projects: PostgreSQL",
				"Versions: 16",
			},
		},
		{
			name:            "empty results",
			results:         []KBSearchResult{},
			query:           "nothing",
			projectNames:    nil,
			projectVersions: nil,
			wantContains: []string{
				"Found 0 relevant chunks",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatKBResults(tt.results, tt.query, tt.projectNames, tt.projectVersions)
			for _, want := range tt.wantContains {
				if !containsString(got, want) {
					t.Errorf("formatKBResults() missing %q in output:\n%s", want, got)
				}
			}
		})
	}
}

func TestKBSearchResultStruct(t *testing.T) {
	result := KBSearchResult{
		Text:           "Sample documentation text",
		Title:          "Getting Started",
		Section:        "Introduction",
		ProjectName:    "PostgreSQL",
		ProjectVersion: "17",
		FilePath:       "/docs/intro.md",
		Similarity:     0.92,
	}

	if result.Text != "Sample documentation text" {
		t.Errorf("Text = %q, want %q", result.Text, "Sample documentation text")
	}
	if result.Title != "Getting Started" {
		t.Errorf("Title = %q, want %q", result.Title, "Getting Started")
	}
	if result.ProjectName != "PostgreSQL" {
		t.Errorf("ProjectName = %q, want %q", result.ProjectName, "PostgreSQL")
	}
	if result.ProjectVersion != "17" {
		t.Errorf("ProjectVersion = %q, want %q", result.ProjectVersion, "17")
	}
	if result.Similarity != 0.92 {
		t.Errorf("Similarity = %f, want %f", result.Similarity, 0.92)
	}
}

// containsString checks if the string contains the substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || findInString(s, substr))
}

func findInString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
