/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package tools

import (
	"database/sql"
	"encoding/binary"
	"math"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
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
	if result.Section != "Introduction" {
		t.Errorf("Section = %q, want %q", result.Section, "Introduction")
	}
	if result.ProjectName != "PostgreSQL" {
		t.Errorf("ProjectName = %q, want %q", result.ProjectName, "PostgreSQL")
	}
	if result.ProjectVersion != "17" {
		t.Errorf("ProjectVersion = %q, want %q", result.ProjectVersion, "17")
	}
	if result.FilePath != "/docs/intro.md" {
		t.Errorf("FilePath = %q, want %q", result.FilePath, "/docs/intro.md")
	}
	if result.Similarity != 0.92 {
		t.Errorf("Similarity = %f, want %f", result.Similarity, 0.92)
	}
}

// testChunk describes a single row to insert into a temporary knowledgebase.
// A nil vector leaves the corresponding column NULL, mirroring a real
// knowledgebase where only the building provider's column is populated.
type testChunk struct {
	text           string
	title          string
	projectName    string
	projectVersion string
	openai         []float32
	voyage         []float32
	ollama         []float32
	gemini         []float32
}

// serialiseEmbedding encodes a vector the same way the knowledgebase builder
// does, as little endian float32 values.
func serialiseEmbedding(vector []float32) []byte {
	if vector == nil {
		return nil
	}
	buf := make([]byte, len(vector)*4)
	for i, v := range vector {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(v))
	}
	return buf
}

// newTestKB creates a temporary knowledgebase database and returns its path.
// When withGemini is false the chunks table omits the gemini_embedding
// column, reproducing a knowledgebase built before Gemini support landed.
// The database lives under the test's temporary directory, so the testing
// package removes it once the test finishes.
func newTestKB(t *testing.T, withGemini bool, chunks []testChunk) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "kb.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("failed to open test knowledgebase: %v", err)
	}
	defer db.Close()

	schema := `
        CREATE TABLE chunks (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            text TEXT NOT NULL,
            title TEXT,
            section TEXT,
            project_name TEXT NOT NULL,
            project_version TEXT NOT NULL,
            file_path TEXT,
            source_file_checksum TEXT,
            openai_embedding BLOB,
            voyage_embedding BLOB,
            ollama_embedding BLOB,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        )`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("failed to create chunks table: %v", err)
	}
	if withGemini {
		if _, err := db.Exec("ALTER TABLE chunks ADD COLUMN gemini_embedding BLOB"); err != nil {
			t.Fatalf("failed to add gemini_embedding column: %v", err)
		}
	}

	insert := `
        INSERT INTO chunks (text, title, section, project_name, project_version,
                            file_path, openai_embedding, voyage_embedding,
                            ollama_embedding)
        VALUES (?, ?, '', ?, ?, '', ?, ?, ?)`
	for _, chunk := range chunks {
		res, err := db.Exec(insert, chunk.text, chunk.title, chunk.projectName,
			chunk.projectVersion, serialiseEmbedding(chunk.openai),
			serialiseEmbedding(chunk.voyage), serialiseEmbedding(chunk.ollama))
		if err != nil {
			t.Fatalf("failed to insert chunk %q: %v", chunk.text, err)
		}
		if !withGemini || chunk.gemini == nil {
			continue
		}
		id, err := res.LastInsertId()
		if err != nil {
			t.Fatalf("failed to read chunk id: %v", err)
		}
		if _, err := db.Exec("UPDATE chunks SET gemini_embedding = ? WHERE id = ?",
			serialiseEmbedding(chunk.gemini), id); err != nil {
			t.Fatalf("failed to set gemini embedding: %v", err)
		}
	}

	return path
}

func TestSearchKBProviderSelection(t *testing.T) {
	// A three dimensional query, so that four dimensional vectors from a
	// different provider are recognisably incompatible.
	query := []float32{1, 0, 0}

	geminiChunks := []testChunk{
		{text: "gemini match", title: "Match", projectName: "PostgreSQL",
			projectVersion: "17", gemini: []float32{1, 0, 0}},
		{text: "gemini other", title: "Other", projectName: "PostgreSQL",
			projectVersion: "17", gemini: []float32{0, 1, 0}},
	}
	openaiChunks := []testChunk{
		{text: "openai match", title: "Match", projectName: "PostgreSQL",
			projectVersion: "17", openai: []float32{1, 0, 0}},
		{text: "openai other", title: "Other", projectName: "PostgreSQL",
			projectVersion: "17", openai: []float32{0, 1, 0}},
	}
	voyageChunks := []testChunk{
		{text: "voyage match", title: "Match", projectName: "pgEdge",
			projectVersion: "1", voyage: []float32{1, 0, 0}},
	}
	ollamaChunks := []testChunk{
		{text: "ollama match", title: "Match", projectName: "pgEdge",
			projectVersion: "1", ollama: []float32{1, 0, 0}},
	}
	// A Gemini built knowledgebase where one chunk only ever received
	// OpenAI vectors of a different width, which must not be ranked.
	mixedChunks := []testChunk{
		{text: "gemini match", title: "Match", projectName: "PostgreSQL",
			projectVersion: "17", gemini: []float32{1, 0, 0}},
		{text: "wrong width", title: "Wrong", projectName: "PostgreSQL",
			projectVersion: "17", openai: []float32{1, 0, 0, 0}},
	}

	tests := []struct {
		name        string
		withGemini  bool
		chunks      []testChunk
		provider    string
		projects    []string
		wantErr     string
		wantTexts   []string
		wantTopSim  float64
		checkTopSim bool
	}{
		{
			name:        "gemini provider reads gemini column",
			withGemini:  true,
			chunks:      geminiChunks,
			provider:    "gemini",
			wantTexts:   []string{"gemini match", "gemini other"},
			wantTopSim:  1.0,
			checkTopSim: true,
		},
		{
			name:        "openai provider on legacy schema",
			withGemini:  false,
			chunks:      openaiChunks,
			provider:    "openai",
			wantTexts:   []string{"openai match", "openai other"},
			wantTopSim:  1.0,
			checkTopSim: true,
		},
		{
			name:       "voyage provider on legacy schema",
			withGemini: false,
			chunks:     voyageChunks,
			provider:   "voyage",
			wantTexts:  []string{"voyage match"},
		},
		{
			name:       "ollama provider on legacy schema",
			withGemini: false,
			chunks:     ollamaChunks,
			provider:   "ollama",
			wantTexts:  []string{"ollama match"},
		},
		{
			name:       "gemini provider on legacy schema reports rebuild",
			withGemini: false,
			chunks:     openaiChunks,
			provider:   "gemini",
			wantErr:    "contains no gemini embeddings",
		},
		{
			name:       "gemini provider against openai only database",
			withGemini: true,
			chunks:     openaiChunks,
			provider:   "gemini",
			wantErr:    "contains no gemini embeddings",
		},
		{
			name:       "gemini provider skips mismatched fallback vectors",
			withGemini: true,
			chunks:     mixedChunks,
			provider:   "gemini",
			wantTexts:  []string{"gemini match"},
		},
		{
			name:       "mismatched fallback width reports rebuild",
			withGemini: false,
			chunks: []testChunk{
				{text: "wrong width", projectName: "PostgreSQL",
					projectVersion: "17", openai: []float32{1, 0, 0, 0}},
			},
			provider: "ollama",
			wantErr:  "contains no ollama embeddings",
		},
		{
			name:       "existing fallback across providers is preserved",
			withGemini: false,
			chunks:     openaiChunks,
			provider:   "voyage",
			wantTexts:  []string{"openai match", "openai other"},
		},
		{
			name:       "filters matching nothing return no results",
			withGemini: true,
			chunks:     geminiChunks,
			provider:   "gemini",
			projects:   []string{"Nonexistent"},
			wantTexts:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := newTestKB(t, tt.withGemini, tt.chunks)

			results, err := searchKB(path, query, tt.projects, nil, 5, tt.provider)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("searchKB() returned %d results, want error containing %q",
						len(results), tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("searchKB() error = %q, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("searchKB() unexpected error: %v", err)
			}

			if len(results) != len(tt.wantTexts) {
				t.Fatalf("searchKB() returned %d results, want %d: %+v",
					len(results), len(tt.wantTexts), results)
			}
			for i, want := range tt.wantTexts {
				if results[i].Text != want {
					t.Errorf("result %d text = %q, want %q", i, results[i].Text, want)
				}
			}
			if tt.checkTopSim && len(results) > 0 {
				if math.Abs(results[0].Similarity-tt.wantTopSim) > 1e-6 {
					t.Errorf("top similarity = %f, want %f", results[0].Similarity, tt.wantTopSim)
				}
			}
		})
	}
}

func TestKBHasGeminiColumn(t *testing.T) {
	tests := []struct {
		name       string
		withGemini bool
		want       bool
	}{
		{name: "modern schema", withGemini: true, want: true},
		{name: "legacy schema", withGemini: false, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := newTestKB(t, tt.withGemini, nil)
			db, err := sql.Open("sqlite", path)
			if err != nil {
				t.Fatalf("failed to open test knowledgebase: %v", err)
			}
			defer db.Close()

			got, err := kbHasGeminiColumn(db)
			if err != nil {
				t.Fatalf("kbHasGeminiColumn() unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("kbHasGeminiColumn() = %v, want %v", got, tt.want)
			}
		})
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
