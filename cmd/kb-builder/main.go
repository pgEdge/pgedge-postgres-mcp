/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"pgedge-postgres-mcp/internal/kbchunker"
	"pgedge-postgres-mcp/internal/kbconfig"
	"pgedge-postgres-mcp/internal/kbconverter"
	"pgedge-postgres-mcp/internal/kbdatabase"
	"pgedge-postgres-mcp/internal/kbembed"
	"pgedge-postgres-mcp/internal/kbsource"
	"pgedge-postgres-mcp/internal/kbtypes"
)

var (
	configFile           string
	databasePath         string
	skipUpdates          bool
	addMissingEmbeddings bool
	clearEmbeddings      string
)

var rootCmd = &cobra.Command{
	Use:   "kb-builder",
	Short: "pgEdge Knowledgebase Builder - Build searchable documentation databases",
	Long: `kb-builder processes documentation from various sources (Git repos, local paths)
and builds a searchable SQLite database with vector embeddings for use with the
pgEdge PostgreSQL MCP server.

The tool converts documents from multiple formats (Markdown, HTML, RST, SGML),
chunks them intelligently, generates embeddings using multiple providers (OpenAI,
Voyage, Ollama), and stores everything in an optimized SQLite database.`,
	RunE: run,
}

func init() {
	rootCmd.Flags().StringVarP(&configFile, "config", "c", "pgedge-nla-kb-builder.yaml",
		"Path to configuration file")
	rootCmd.Flags().StringVarP(&databasePath, "database", "d", "",
		"Path to output SQLite database (overrides config file)")
	rootCmd.Flags().BoolVar(&skipUpdates, "skip-updates", false,
		"Skip git pull updates for existing repositories")
	rootCmd.Flags().BoolVar(&addMissingEmbeddings, "add-missing-embeddings", false,
		"Add missing embeddings to existing database instead of rebuilding")
	rootCmd.Flags().StringVar(&clearEmbeddings, "clear-embeddings", "",
		"Clear embeddings for specified provider (openai, voyage, or ollama)")
}

func main() {
	// Let cobra handle errors and exit codes
	// Usage is shown for flag parse errors, but suppressed for runtime errors (via cmd.SilenceUsage in run())
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	// Suppress usage for runtime errors (flags have already been parsed by this point)
	cmd.SilenceUsage = true

	// Load configuration
	if configFile == "" {
		// Use default config file in binary directory
		exePath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("failed to get executable path: %w", err)
		}
		configFile = filepath.Join(filepath.Dir(exePath), "pgedge-nla-kb-builder.yaml")
	}

	fmt.Printf("Loading configuration from: %s\n", configFile)
	config, err := kbconfig.Load(configFile)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Override database path if specified on command line
	if databasePath != "" {
		config.DatabasePath = databasePath
	}

	// If --add-missing-embeddings is specified, run that and exit
	if addMissingEmbeddings {
		return runAddMissingEmbeddings(config)
	}

	// If --clear-embeddings is specified, run that and exit
	if clearEmbeddings != "" {
		return runClearEmbeddings(config, clearEmbeddings)
	}

	fmt.Printf("Output database: %s\n", config.DatabasePath)
	fmt.Printf("Doc source path: %s\n", config.DocSourcePath)
	fmt.Printf("Number of sources: %d\n", len(config.Sources))

	// Validate embedding providers
	enabledProviders := []string{}
	if config.Embeddings.OpenAI.Enabled {
		enabledProviders = append(enabledProviders, "OpenAI")
	}
	if config.Embeddings.Voyage.Enabled {
		enabledProviders = append(enabledProviders, "Voyage")
	}
	if config.Embeddings.Ollama.Enabled {
		enabledProviders = append(enabledProviders, "Ollama")
	}
	fmt.Printf("Enabled embedding providers: %v\n", enabledProviders)

	// Open database early for checksum checking
	db, err := kbdatabase.Open(config.DatabasePath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Fetch all documentation sources
	fmt.Println("\n=== Fetching Documentation Sources ===")
	if skipUpdates {
		fmt.Println("Note: Skipping git pull updates for existing repositories")
	}
	sources, err := kbsource.FetchAll(config, skipUpdates)
	if err != nil {
		return fmt.Errorf("failed to fetch sources: %w", err)
	}

	// Process all documents (with incremental processing)
	fmt.Println("\n=== Processing Documents ===")
	allChunks, err := processAllDocuments(sources, db)
	if err != nil {
		return fmt.Errorf("failed to process documents: %w", err)
	}

	fmt.Printf("\nTotal chunks created/reused: %d\n", len(allChunks))

	// Generate embeddings
	fmt.Println("\n=== Generating Embeddings ===")
	embedGen := kbembed.NewEmbeddingGenerator(config, db)
	embeddingErrors := embedGen.GenerateEmbeddings(allChunks)

	// Report any embedding failures
	if len(embeddingErrors) > 0 {
		fmt.Println("\n⚠️  Warning: Some embedding providers failed:")
		for provider, err := range embeddingErrors {
			fmt.Printf("  - %s: %v\n", provider, err)
		}
		fmt.Println("\nContinuing with partial embeddings. Use --add-missing-embeddings later to complete them.")
	}

	// Store in database
	fmt.Println("\n=== Storing in Database ===")
	if err := db.InsertChunks(allChunks); err != nil {
		return fmt.Errorf("failed to insert chunks: %w", err)
	}

	// Print stats
	fmt.Println("\n=== Database Statistics ===")
	stats, err := db.GetStats()
	if err != nil {
		return fmt.Errorf("failed to get stats: %w", err)
	}

	fmt.Printf("Total chunks: %v\n", stats["total_chunks"])
	fmt.Println("Projects:")
	for _, project := range stats["projects"].([]map[string]interface{}) {
		fmt.Printf("  - %s %s: %d chunks\n",
			project["name"], project["version"], project["chunks"])
	}

	fmt.Printf("\n✓ Knowledgebase successfully built: %s\n", config.DatabasePath)

	return nil
}

func runAddMissingEmbeddings(config *kbconfig.Config) error {
	fmt.Printf("Adding missing embeddings to: %s\n\n", config.DatabasePath)

	// Open database
	db, err := kbdatabase.Open(config.DatabasePath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Get all chunks from database
	fmt.Println("Loading existing chunks from database...")
	chunks, err := db.GetAllChunks()
	if err != nil {
		return fmt.Errorf("failed to load chunks: %w", err)
	}
	fmt.Printf("Loaded %d chunks\n", len(chunks))

	// Filter to only chunks missing embeddings for enabled providers
	var chunksNeedingEmbeddings []*kbtypes.Chunk
	for _, chunk := range chunks {
		needsEmbedding := false

		if config.Embeddings.OpenAI.Enabled && len(chunk.OpenAIEmbedding) == 0 {
			needsEmbedding = true
		}
		if config.Embeddings.Voyage.Enabled && len(chunk.VoyageEmbedding) == 0 {
			needsEmbedding = true
		}
		if config.Embeddings.Ollama.Enabled && len(chunk.OllamaEmbedding) == 0 {
			needsEmbedding = true
		}

		if needsEmbedding {
			chunksNeedingEmbeddings = append(chunksNeedingEmbeddings, chunk)
		}
	}

	if len(chunksNeedingEmbeddings) == 0 {
		fmt.Println("\n✓ All chunks already have embeddings for enabled providers")
		return nil
	}

	fmt.Printf("\nFound %d chunks with missing embeddings\n", len(chunksNeedingEmbeddings))

	// Generate missing embeddings
	fmt.Println("\n=== Generating Missing Embeddings ===")
	embedGen := kbembed.NewEmbeddingGenerator(config, db)
	embeddingErrors := embedGen.GenerateEmbeddings(chunksNeedingEmbeddings)

	// Report any failures
	if len(embeddingErrors) > 0 {
		fmt.Println("\n⚠️  Warning: Some embedding providers failed:")
		for provider, err := range embeddingErrors {
			fmt.Printf("  - %s: %v\n", provider, err)
		}
	}

	// Note: Embeddings are saved incrementally during generation, no final update needed

	fmt.Printf("\n✓ Successfully updated embeddings in: %s\n", config.DatabasePath)

	// Print final stats
	stats, err := db.GetStats()
	if err != nil {
		return fmt.Errorf("failed to get stats: %w", err)
	}

	fmt.Printf("\nTotal chunks: %v\n", stats["total_chunks"])

	return nil
}

func runClearEmbeddings(config *kbconfig.Config, provider string) error {
	fmt.Printf("Clearing %s embeddings from: %s\n\n", provider, config.DatabasePath)

	// Open database
	db, err := kbdatabase.Open(config.DatabasePath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Clear embeddings for the specified provider
	rowsAffected, err := db.ClearEmbeddings(provider)
	if err != nil {
		return fmt.Errorf("failed to clear embeddings: %w", err)
	}

	fmt.Printf("✓ Successfully cleared %s embeddings from %d chunks\n", provider, rowsAffected)

	return nil
}

func processAllDocuments(sources []kbsource.SourceInfo, db *kbdatabase.Database) ([]*kbtypes.Chunk, error) {
	var allChunks []*kbtypes.Chunk

	for _, source := range sources {
		fmt.Printf("\nProcessing %s %s...\n", source.Source.ProjectName, source.Source.ProjectVersion)

		chunks, err := processSource(source, db)
		if err != nil {
			return nil, fmt.Errorf("failed to process source %s: %w", source.Source.ProjectName, err)
		}

		fmt.Printf("  Created/reused %d chunks\n", len(chunks))
		allChunks = append(allChunks, chunks...)
	}

	return allChunks, nil
}

func processSource(source kbsource.SourceInfo, db *kbdatabase.Database) ([]*kbtypes.Chunk, error) {
	var chunks []*kbtypes.Chunk
	var validChecksums []string

	// First pass: count supported files
	fmt.Printf("  Scanning for supported files...\n")
	var supportedFiles []string
	err := filepath.WalkDir(source.BasePath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && kbconverter.IsSupported(path) {
			supportedFiles = append(supportedFiles, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	fmt.Printf("  Found %d supported files\n", len(supportedFiles))

	// Second pass: process files with progress
	processedCount := 0
	for _, path := range supportedFiles {
		processedCount++

		// Show progress every file, but with relative path for readability
		relPath, _ := filepath.Rel(source.BasePath, path)
		if relPath == "" {
			relPath = filepath.Base(path)
		}

		startTime := time.Now()
		fmt.Printf("  [%d/%d] Processing: %s", processedCount, len(supportedFiles), relPath)

		// Process the file (with checksum-based incremental processing)
		fileChunks, skipped, checksum, err := processFile(path, source, db)
		elapsed := time.Since(startTime)

		if err != nil {
			fmt.Printf(" - ERROR (%.2fs): %v\n", elapsed.Seconds(), err)
			continue // Continue processing other files
		}

		// Track this checksum as valid for cleanup
		if checksum != "" {
			validChecksums = append(validChecksums, checksum)
		}

		if skipped {
			fmt.Printf(" - skipped (unchanged)\n")
		} else if len(fileChunks) > 0 {
			fmt.Printf(" - %d chunks (%.2fs)\n", len(fileChunks), elapsed.Seconds())
		} else {
			fmt.Printf(" - 0 chunks (%.2fs)\n", elapsed.Seconds())
		}

		// Add chunks to the collection
		if len(fileChunks) > 0 {
			chunks = append(chunks, fileChunks...)
		}
	}

	// Cleanup stale chunks from previous runs (files that no longer exist)
	if len(validChecksums) > 0 {
		fmt.Printf("  Cleaning up stale data...\n")
		if err := db.CleanupStaleChunks(source.Source.ProjectName, source.Source.ProjectVersion, validChecksums); err != nil {
			return nil, fmt.Errorf("failed to cleanup stale chunks: %w", err)
		}
	}

	return chunks, nil
}

func processFile(filePath string, source kbsource.SourceInfo, db *kbdatabase.Database) ([]*kbtypes.Chunk, bool, string, error) {
	stepStart := time.Now()

	// Read file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, false, "", fmt.Errorf("failed to read file: %w", err)
	}
	readTime := time.Since(stepStart)

	// Compute checksum
	hash := sha256.Sum256(content)
	checksum := hex.EncodeToString(hash[:])

	// Check if this file needs processing for this project/version
	needsProcessing, err := db.FileNeedsProcessing(checksum, source.Source.ProjectName, source.Source.ProjectVersion)
	if err != nil {
		return nil, false, checksum, fmt.Errorf("failed to check if file needs processing: %w", err)
	}

	// If file doesn't need processing (already processed for this project/version), skip it
	if !needsProcessing {
		// Return empty chunks - they're already in the database, no need to re-insert
		return nil, true, checksum, nil
	}

	// Check if this file exists in another version (deduplication)
	existingChunks, err := db.GetChunksForChecksum(checksum)
	if err != nil {
		return nil, false, checksum, fmt.Errorf("failed to check for existing chunks: %w", err)
	}

	// If chunks exist for this checksum in another version, clone them with new project/version
	if len(existingChunks) > 0 {
		var chunks []*kbtypes.Chunk
		for _, existingChunk := range existingChunks {
			chunk := &kbtypes.Chunk{
				Text:               existingChunk.Text,
				Title:              existingChunk.Title,
				Section:            existingChunk.Section,
				ProjectName:        source.Source.ProjectName,
				ProjectVersion:     source.Source.ProjectVersion,
				FilePath:           filePath,
				SourceFileChecksum: checksum,
				OpenAIEmbedding:    existingChunk.OpenAIEmbedding,
				VoyageEmbedding:    existingChunk.VoyageEmbedding,
				OllamaEmbedding:    existingChunk.OllamaEmbedding,
			}
			chunks = append(chunks, chunk)
		}
		return chunks, false, checksum, nil
	}

	// File needs processing - process it from scratch
	// Detect document type
	stepStart = time.Now()
	docType := kbconverter.DetectDocumentType(filePath)
	detectTime := time.Since(stepStart)

	// Convert to markdown
	stepStart = time.Now()
	markdown, title, err := kbconverter.Convert(content, docType)
	if err != nil {
		return nil, false, checksum, fmt.Errorf("failed to convert document (read: %.2fs, detect: %.2fs, convert: %.2fs): %w",
			readTime.Seconds(), detectTime.Seconds(), time.Since(stepStart).Seconds(), err)
	}
	convertTime := time.Since(stepStart)

	// Create document
	doc := &kbtypes.Document{
		Title:          title,
		Content:        markdown,
		SourceContent:  content,
		FilePath:       filePath,
		ProjectName:    source.Source.ProjectName,
		ProjectVersion: source.Source.ProjectVersion,
		DocType:        docType,
	}

	// Chunk the document
	stepStart = time.Now()
	chunks, err := kbchunker.ChunkDocument(doc)
	if err != nil {
		return nil, false, checksum, fmt.Errorf("failed to chunk document (read: %.2fs, detect: %.2fs, convert: %.2fs, chunk: %.2fs): %w",
			readTime.Seconds(), detectTime.Seconds(), convertTime.Seconds(), time.Since(stepStart).Seconds(), err)
	}
	chunkTime := time.Since(stepStart)

	// Set checksum on all chunks
	for _, chunk := range chunks {
		chunk.SourceFileChecksum = checksum
	}

	// Log timing breakdown if file took more than 1 second
	totalTime := readTime + detectTime + convertTime + chunkTime
	if totalTime.Seconds() > 1.0 {
		fmt.Printf("           [Slow file - read: %.2fs, detect: %.2fs, convert: %.2fs, chunk: %.2fs]\n",
			readTime.Seconds(), detectTime.Seconds(), convertTime.Seconds(), chunkTime.Seconds())
	}

	return chunks, false, checksum, nil
}
