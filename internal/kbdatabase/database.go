//-------------------------------------------------------------------------
//
// pgEdge PostgreSQL MCP - Knowledgebase Builder
//
// Portions copyright (c) 2025, pgEdge, Inc.
// This software is released under The PostgreSQL License
//
//-------------------------------------------------------------------------

package kbdatabase

import (
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"pgedge-postgres-mcp/internal/kbtypes"
)

// Database represents the knowledgebase database
type Database struct {
	db *sql.DB
}

// Open opens or creates the knowledgebase database
func Open(path string) (*Database, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	d := &Database{db: db}

	// Create schema
	if err := d.createSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create schema: %w", err)
	}

	return d, nil
}

// Close closes the database
func (d *Database) Close() error {
	return d.db.Close()
}

// createSchema creates the database schema
func (d *Database) createSchema() error {
	schema := `
    CREATE TABLE IF NOT EXISTS chunks (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        text TEXT NOT NULL,
        title TEXT,
        section TEXT,
        project_name TEXT NOT NULL,
        project_version TEXT NOT NULL,
        file_path TEXT,
        source_file_checksum TEXT,

        -- Embeddings from different providers (stored as BLOB)
        openai_embedding BLOB,
        voyage_embedding BLOB,
        ollama_embedding BLOB,

        -- Metadata
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );

    -- Table to track processed source files
    CREATE TABLE IF NOT EXISTS source_files (
        checksum TEXT NOT NULL,
        file_path TEXT NOT NULL,
        project_name TEXT NOT NULL,
        project_version TEXT NOT NULL,
        doc_type TEXT,
        num_chunks INTEGER DEFAULT 0,
        processed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

        PRIMARY KEY (checksum, project_name, project_version)
    );

    -- Indexes for fast filtering
    CREATE INDEX IF NOT EXISTS idx_project ON chunks(project_name, project_version);
    CREATE INDEX IF NOT EXISTS idx_title ON chunks(title);
    CREATE INDEX IF NOT EXISTS idx_section ON chunks(section);
    CREATE INDEX IF NOT EXISTS idx_source_checksum ON chunks(source_file_checksum);
    CREATE INDEX IF NOT EXISTS idx_source_files_checksum ON source_files(checksum);
    `

	_, err := d.db.Exec(schema)
	return err
}

// InsertChunks inserts chunks into the database and records source file metadata
func (d *Database) InsertChunks(chunks []*kbtypes.Chunk) error {
	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
        INSERT INTO chunks (
            text, title, section, project_name, project_version, file_path, source_file_checksum,
            openai_embedding, voyage_embedding, ollama_embedding
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	// Track source files and their chunk counts
	sourceFileChunks := make(map[string]int) // checksum+project+version -> count

	for i, chunk := range chunks {
		// Serialize embeddings to BLOB
		var openaiBlob, voyageBlob, ollamaBlob []byte

		if len(chunk.OpenAIEmbedding) > 0 {
			openaiBlob = serializeEmbedding(chunk.OpenAIEmbedding)
		}
		if len(chunk.VoyageEmbedding) > 0 {
			voyageBlob = serializeEmbedding(chunk.VoyageEmbedding)
		}
		if len(chunk.OllamaEmbedding) > 0 {
			ollamaBlob = serializeEmbedding(chunk.OllamaEmbedding)
		}

		_, err := stmt.Exec(
			chunk.Text,
			chunk.Title,
			chunk.Section,
			chunk.ProjectName,
			chunk.ProjectVersion,
			chunk.FilePath,
			chunk.SourceFileChecksum,
			openaiBlob,
			voyageBlob,
			ollamaBlob,
		)
		if err != nil {
			return fmt.Errorf("failed to insert chunk %d: %w", i, err)
		}

		// Track source file
		if chunk.SourceFileChecksum != "" {
			key := chunk.SourceFileChecksum + "|" + chunk.ProjectName + "|" + chunk.ProjectVersion
			sourceFileChunks[key]++
		}
	}

	// Insert or update source file records
	sourceStmt, err := tx.Prepare(`
        INSERT INTO source_files (checksum, file_path, project_name, project_version, num_chunks)
        VALUES (?, ?, ?, ?, ?)
        ON CONFLICT(checksum, project_name, project_version)
        DO UPDATE SET num_chunks = num_chunks + ?, processed_at = CURRENT_TIMESTAMP
    `)
	if err != nil {
		return fmt.Errorf("failed to prepare source file statement: %w", err)
	}
	defer sourceStmt.Close()

	// Get a sample chunk for each source file to extract metadata
	sourceFileInfo := make(map[string]*kbtypes.Chunk)
	for _, chunk := range chunks {
		if chunk.SourceFileChecksum != "" {
			key := chunk.SourceFileChecksum + "|" + chunk.ProjectName + "|" + chunk.ProjectVersion
			if _, exists := sourceFileInfo[key]; !exists {
				sourceFileInfo[key] = chunk
			}
		}
	}

	for key, count := range sourceFileChunks {
		chunk := sourceFileInfo[key]
		_, err := sourceStmt.Exec(
			chunk.SourceFileChecksum,
			chunk.FilePath,
			chunk.ProjectName,
			chunk.ProjectVersion,
			count,
			count, // For the UPDATE clause
		)
		if err != nil {
			return fmt.Errorf("failed to insert source file record: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// serializeEmbedding converts float32 slice to bytes
func serializeEmbedding(embedding []float32) []byte {
	buf := make([]byte, len(embedding)*4)
	for i, v := range embedding {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(v))
	}
	return buf
}

// deserializeEmbedding converts bytes back to float32 slice
func deserializeEmbedding(data []byte) []float32 {
	if len(data) == 0 || len(data)%4 != 0 {
		return nil
	}

	embedding := make([]float32, len(data)/4)
	for i := range embedding {
		bits := binary.LittleEndian.Uint32(data[i*4:])
		embedding[i] = math.Float32frombits(bits)
	}
	return embedding
}

// GetStats returns statistics about the database
func (d *Database) GetStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Total chunks
	var totalChunks int
	err := d.db.QueryRow("SELECT COUNT(*) FROM chunks").Scan(&totalChunks)
	if err != nil {
		return nil, err
	}
	stats["total_chunks"] = totalChunks

	// Chunks by project
	rows, err := d.db.Query(`
        SELECT project_name, project_version, COUNT(*)
        FROM chunks
        GROUP BY project_name, project_version
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	projects := make([]map[string]interface{}, 0)
	for rows.Next() {
		var name, version string
		var count int
		if err := rows.Scan(&name, &version, &count); err != nil {
			return nil, err
		}
		projects = append(projects, map[string]interface{}{
			"name":    name,
			"version": version,
			"chunks":  count,
		})
	}
	stats["projects"] = projects

	return stats, nil
}

// SearchChunks performs a simple text search (for testing without vector search)
func (d *Database) SearchChunks(query string, limit int) ([]*kbtypes.Chunk, error) {
	rows, err := d.db.Query(`
        SELECT id, text, title, section, project_name, project_version, file_path,
               openai_embedding, voyage_embedding, ollama_embedding
        FROM chunks
        WHERE text LIKE ?
        LIMIT ?
    `, "%"+query+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chunks []*kbtypes.Chunk
	for rows.Next() {
		var id int
		var text, title, section, projectName, projectVersion, filePath string
		var openaiBlob, voyageBlob, ollamaBlob []byte

		err := rows.Scan(&id, &text, &title, &section, &projectName, &projectVersion,
			&filePath, &openaiBlob, &voyageBlob, &ollamaBlob)
		if err != nil {
			return nil, err
		}

		chunk := &kbtypes.Chunk{
			Text:            text,
			Title:           title,
			Section:         section,
			ProjectName:     projectName,
			ProjectVersion:  projectVersion,
			FilePath:        filePath,
			OpenAIEmbedding: deserializeEmbedding(openaiBlob),
			VoyageEmbedding: deserializeEmbedding(voyageBlob),
			OllamaEmbedding: deserializeEmbedding(ollamaBlob),
		}

		chunks = append(chunks, chunk)
	}

	return chunks, nil
}

// GetAllChunks retrieves all chunks from the database
func (d *Database) GetAllChunks() ([]*kbtypes.Chunk, error) {
	rows, err := d.db.Query(`
        SELECT id, text, title, section, project_name, project_version, file_path,
               openai_embedding, voyage_embedding, ollama_embedding
        FROM chunks
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chunks []*kbtypes.Chunk
	for rows.Next() {
		var id int
		var text, title, section, projectName, projectVersion, filePath string
		var openaiBlob, voyageBlob, ollamaBlob []byte

		err := rows.Scan(&id, &text, &title, &section, &projectName, &projectVersion,
			&filePath, &openaiBlob, &voyageBlob, &ollamaBlob)
		if err != nil {
			return nil, err
		}

		chunk := &kbtypes.Chunk{
			ID:              id,
			Text:            text,
			Title:           title,
			Section:         section,
			ProjectName:     projectName,
			ProjectVersion:  projectVersion,
			FilePath:        filePath,
			OpenAIEmbedding: deserializeEmbedding(openaiBlob),
			VoyageEmbedding: deserializeEmbedding(voyageBlob),
			OllamaEmbedding: deserializeEmbedding(ollamaBlob),
		}

		chunks = append(chunks, chunk)
	}

	return chunks, rows.Err()
}

// UpdateChunkEmbeddings updates embeddings for existing chunks
func (d *Database) UpdateChunkEmbeddings(chunks []*kbtypes.Chunk) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
        UPDATE chunks
        SET openai_embedding = ?,
            voyage_embedding = ?,
            ollama_embedding = ?
        WHERE id = ?
    `)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, chunk := range chunks {
		openaiBlob := serializeEmbedding(chunk.OpenAIEmbedding)
		voyageBlob := serializeEmbedding(chunk.VoyageEmbedding)
		ollamaBlob := serializeEmbedding(chunk.OllamaEmbedding)

		_, err := stmt.Exec(openaiBlob, voyageBlob, ollamaBlob, chunk.ID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// UpdateOpenAIEmbeddings updates only OpenAI embeddings for existing chunks
func (d *Database) UpdateOpenAIEmbeddings(chunks []*kbtypes.Chunk) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`UPDATE chunks SET openai_embedding = ? WHERE id = ?`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, chunk := range chunks {
		blob := serializeEmbedding(chunk.OpenAIEmbedding)
		if _, err := stmt.Exec(blob, chunk.ID); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// UpdateVoyageEmbeddings updates only Voyage embeddings for existing chunks
func (d *Database) UpdateVoyageEmbeddings(chunks []*kbtypes.Chunk) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`UPDATE chunks SET voyage_embedding = ? WHERE id = ?`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, chunk := range chunks {
		blob := serializeEmbedding(chunk.VoyageEmbedding)
		if _, err := stmt.Exec(blob, chunk.ID); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// UpdateOllamaEmbeddings updates only Ollama embeddings for existing chunks
func (d *Database) UpdateOllamaEmbeddings(chunks []*kbtypes.Chunk) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`UPDATE chunks SET ollama_embedding = ? WHERE id = ?`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, chunk := range chunks {
		blob := serializeEmbedding(chunk.OllamaEmbedding)
		if _, err := stmt.Exec(blob, chunk.ID); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// ClearEmbeddings clears all embeddings for a specific provider
func (d *Database) ClearEmbeddings(provider string) (int64, error) {
	var column string
	switch strings.ToLower(provider) {
	case "openai":
		column = "openai_embedding"
	case "voyage":
		column = "voyage_embedding"
	case "ollama":
		column = "ollama_embedding"
	default:
		return 0, fmt.Errorf("invalid provider: %s (must be openai, voyage, or ollama)", provider)
	}

	query := fmt.Sprintf("UPDATE chunks SET %s = NULL", column)
	result, err := d.db.Exec(query)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// FileNeedsProcessing checks if a file needs processing based on its checksum
// Returns true if the file is new or changed, false if already processed
func (d *Database) FileNeedsProcessing(checksum, projectName, projectVersion string) (bool, error) {
	var count int
	err := d.db.QueryRow(`
        SELECT COUNT(*) FROM source_files
        WHERE checksum = ? AND project_name = ? AND project_version = ?
    `, checksum, projectName, projectVersion).Scan(&count)

	if err != nil {
		return false, err
	}

	return count == 0, nil
}

// GetChunksForChecksum retrieves existing chunks for a given checksum from a different project/version
// This enables deduplication across versions
func (d *Database) GetChunksForChecksum(checksum string) ([]*kbtypes.Chunk, error) {
	rows, err := d.db.Query(`
        SELECT text, title, section, file_path,
               openai_embedding, voyage_embedding, ollama_embedding
        FROM chunks
        WHERE source_file_checksum = ?
        LIMIT 1000
    `, checksum)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chunks []*kbtypes.Chunk
	for rows.Next() {
		var text, title, section, filePath string
		var openaiBlob, voyageBlob, ollamaBlob []byte

		err := rows.Scan(&text, &title, &section, &filePath,
			&openaiBlob, &voyageBlob, &ollamaBlob)
		if err != nil {
			return nil, err
		}

		chunk := &kbtypes.Chunk{
			Text:               text,
			Title:              title,
			Section:            section,
			FilePath:           filePath,
			SourceFileChecksum: checksum,
			OpenAIEmbedding:    deserializeEmbedding(openaiBlob),
			VoyageEmbedding:    deserializeEmbedding(voyageBlob),
			OllamaEmbedding:    deserializeEmbedding(ollamaBlob),
		}

		chunks = append(chunks, chunk)
	}

	return chunks, rows.Err()
}

// DeleteChunksForFile deletes chunks associated with a specific file (used when file changes)
func (d *Database) DeleteChunksForFile(checksum, projectName, projectVersion string) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete chunks
	_, err = tx.Exec(`
        DELETE FROM chunks
        WHERE source_file_checksum = ? AND project_name = ? AND project_version = ?
    `, checksum, projectName, projectVersion)
	if err != nil {
		return err
	}

	// Delete source file record
	_, err = tx.Exec(`
        DELETE FROM source_files
        WHERE checksum = ? AND project_name = ? AND project_version = ?
    `, checksum, projectName, projectVersion)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// CleanupStaleChunks removes chunks for files that no longer exist in the current processing run
// Takes a list of checksums that were seen in the current run, and deletes any chunks for
// the project/version that don't match these checksums
func (d *Database) CleanupStaleChunks(projectName, projectVersion string, validChecksums []string) error {
	if len(validChecksums) == 0 {
		// If no checksums provided, don't delete anything (safety check)
		return nil
	}

	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Build a parameterized query with placeholders for all checksums
	placeholders := make([]string, len(validChecksums))
	args := make([]interface{}, 0, len(validChecksums)+2)
	args = append(args, projectName, projectVersion)

	for i := range validChecksums {
		placeholders[i] = "?"
		args = append(args, validChecksums[i])
	}

	placeholderList := "(" + joinStrings(placeholders, ",") + ")"

	// Delete chunks that aren't in the valid checksums list
	query := `
        DELETE FROM chunks
        WHERE project_name = ? AND project_version = ?
        AND source_file_checksum NOT IN ` + placeholderList

	result, err := tx.Exec(query, args...)
	if err != nil {
		return err
	}

	rowsAffected, _ := result.RowsAffected()

	// Delete source file records that aren't in the valid checksums list
	query = `
        DELETE FROM source_files
        WHERE project_name = ? AND project_version = ?
        AND checksum NOT IN ` + placeholderList

	_, err = tx.Exec(query, args...)
	if err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	if rowsAffected > 0 {
		fmt.Printf("  Cleaned up %d stale chunks from previous runs\n", rowsAffected)
	}

	return nil
}

// joinStrings is a helper to join strings with a separator
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}
