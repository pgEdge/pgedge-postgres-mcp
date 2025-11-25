/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

//-------------------------------------------------------------------------
//
// pgEdge PostgreSQL MCP - Knowledgebase Deduplication Utility
//
// Portions copyright (c) 2025, pgEdge, Inc.
// This software is released under The PostgreSQL License
//
//-------------------------------------------------------------------------

package main

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <database-path>\n", os.Args[0])
		os.Exit(1)
	}

	dbPath := os.Args[1]

	fmt.Printf("Opening database: %s\n", dbPath)
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Get initial stats
	var totalBefore int
	err = db.QueryRow("SELECT COUNT(*) FROM chunks").Scan(&totalBefore)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error counting chunks: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Total chunks before deduplication: %d\n", totalBefore)

	// Find and remove duplicates
	// Keep the chunk with the lowest ID (oldest) for each unique combination
	fmt.Println("\nRemoving duplicate chunks...")
	result, err := db.Exec(`
		DELETE FROM chunks
		WHERE id NOT IN (
			SELECT MIN(id)
			FROM chunks
			GROUP BY text, title, section, project_name, project_version, file_path, source_file_checksum
		)
	`)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error removing duplicates: %v\n", err)
		os.Exit(1)
	}

	deleted, _ := result.RowsAffected()
	fmt.Printf("Removed %d duplicate chunks\n", deleted)

	// Get final stats
	var totalAfter int
	err = db.QueryRow("SELECT COUNT(*) FROM chunks").Scan(&totalAfter)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error counting chunks: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Total chunks after deduplication: %d\n", totalAfter)

	// Update source_files table with correct chunk counts
	fmt.Println("\nUpdating source_files table...")
	_, err = db.Exec(`
		UPDATE source_files
		SET num_chunks = (
			SELECT COUNT(*)
			FROM chunks
			WHERE chunks.source_file_checksum = source_files.checksum
			  AND chunks.project_name = source_files.project_name
			  AND chunks.project_version = source_files.project_version
		)
	`)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error updating source_files: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Updated source_files table")

	// Show final project statistics
	fmt.Println("\n=== Final Database Statistics ===")
	rows, err := db.Query(`
		SELECT project_name, project_version, COUNT(*) as chunks
		FROM chunks
		GROUP BY project_name, project_version
		ORDER BY project_name, project_version
	`)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting stats: %v\n", err)
		os.Exit(1)
	}
	defer rows.Close()

	for rows.Next() {
		var name, version string
		var count int
		if err := rows.Scan(&name, &version, &count); err != nil {
			fmt.Fprintf(os.Stderr, "Error reading stats: %v\n", err)
			continue
		}
		fmt.Printf("  %s %s: %d chunks\n", name, version, count)
	}

	fmt.Println("\n✓ Deduplication complete!")
}
