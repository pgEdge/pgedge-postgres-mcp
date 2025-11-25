//-------------------------------------------------------------------------
//
// pgEdge PostgreSQL MCP - Knowledgebase Builder
//
// Portions copyright (c) 2025, pgEdge, Inc.
// This software is released under The PostgreSQL License
//
//-------------------------------------------------------------------------

package kbembed

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"pgedge-postgres-mcp/internal/kbconfig"
	"pgedge-postgres-mcp/internal/kbdatabase"
	"pgedge-postgres-mcp/internal/kbtypes"
)

const (
	maxRetries     = 5
	initialBackoff = 1 * time.Second
	maxBackoff     = 32 * time.Second
)

// EmbeddingGenerator generates embeddings using configured providers
type EmbeddingGenerator struct {
	config *kbconfig.Config
	client *http.Client
	db     *kbdatabase.Database
	dbMux  sync.Mutex // Protects database writes from concurrent providers
}

// NewEmbeddingGenerator creates a new embedding generator
func NewEmbeddingGenerator(config *kbconfig.Config, db *kbdatabase.Database) *EmbeddingGenerator {
	// Use longer timeout for Ollama (models may need initialization, slower processing)
	// OpenAI/Voyage typically respond in seconds, but Ollama can take much longer
	timeout := 5 * time.Minute

	return &EmbeddingGenerator{
		config: config,
		client: &http.Client{
			Timeout: timeout,
		},
		db: db,
	}
}

// GenerateEmbeddings generates embeddings for all chunks using all enabled providers in parallel
// Returns a map of provider names to errors (if any), but does not fail on individual provider errors
func (eg *EmbeddingGenerator) GenerateEmbeddings(chunks []*kbtypes.Chunk) map[string]error {
	fmt.Printf("\nGenerating embeddings for %d chunks...\n", len(chunks))

	var wg sync.WaitGroup
	type providerResult struct {
		name string
		err  error
	}
	resultChan := make(chan providerResult, 3)

	startTime := time.Now()

	// Generate embeddings for each provider in parallel
	if eg.config.Embeddings.OpenAI.Enabled {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fmt.Printf("Starting OpenAI embeddings...\n")
			providerStart := time.Now()
			if err := eg.generateOpenAIEmbeddings(chunks); err != nil {
				fmt.Printf("⚠️  OpenAI embeddings failed: %v\n", err)
				resultChan <- providerResult{"OpenAI", err}
				return
			}
			fmt.Printf("✓ OpenAI embeddings completed in %.2fs\n", time.Since(providerStart).Seconds())
			resultChan <- providerResult{"OpenAI", nil}
		}()
	}

	if eg.config.Embeddings.Voyage.Enabled {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fmt.Printf("Starting Voyage embeddings...\n")
			providerStart := time.Now()
			if err := eg.generateVoyageEmbeddings(chunks); err != nil {
				fmt.Printf("⚠️  Voyage embeddings failed: %v\n", err)
				resultChan <- providerResult{"Voyage", err}
				return
			}
			fmt.Printf("✓ Voyage embeddings completed in %.2fs\n", time.Since(providerStart).Seconds())
			resultChan <- providerResult{"Voyage", nil}
		}()
	}

	if eg.config.Embeddings.Ollama.Enabled {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fmt.Printf("Starting Ollama embeddings...\n")
			providerStart := time.Now()
			if err := eg.generateOllamaEmbeddings(chunks); err != nil {
				fmt.Printf("⚠️  Ollama embeddings failed: %v\n", err)
				resultChan <- providerResult{"Ollama", err}
				return
			}
			fmt.Printf("✓ Ollama embeddings completed in %.2fs\n", time.Since(providerStart).Seconds())
			resultChan <- providerResult{"Ollama", nil}
		}()
	}

	// Wait for all providers to complete
	wg.Wait()
	close(resultChan)

	// Collect results
	errors := make(map[string]error)
	for result := range resultChan {
		if result.err != nil {
			errors[result.name] = result.err
		}
	}

	fmt.Printf("\nAll embedding providers completed in %.2fs\n", time.Since(startTime).Seconds())

	return errors
}

// retryWithBackoff executes a function with exponential backoff retry logic
func retryWithBackoff(operation string, fn func() (*http.Response, error)) (*http.Response, error) {
	var lastErr error
	backoff := initialBackoff

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			fmt.Printf("  Retry %d/%d for %s after %.1fs...\n", attempt, maxRetries, operation, backoff.Seconds())
			time.Sleep(backoff)

			// Exponential backoff with jitter
			backoff = time.Duration(float64(backoff) * 2)
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}

		resp, err := fn()
		if err != nil {
			lastErr = err
			continue
		}

		// Check HTTP status
		if resp.StatusCode == http.StatusOK {
			return resp, nil
		}

		// Handle retryable errors
		if resp.StatusCode == 429 || // Rate limited
			resp.StatusCode == 500 || // Server error
			resp.StatusCode == 502 || // Bad gateway
			resp.StatusCode == 503 || // Service unavailable
			resp.StatusCode == 504 { // Gateway timeout

			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))

			if resp.StatusCode == 429 && attempt < maxRetries {
				fmt.Printf("  Rate limited, will retry...\n")
			}
			continue
		}

		// Non-retryable error
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil, fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

// OpenAI API request/response structures
type openAIEmbeddingRequest struct {
	Input      []string `json:"input"`
	Model      string   `json:"model"`
	Dimensions int      `json:"dimensions,omitempty"`
}

type openAIEmbeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

// generateOpenAIEmbeddings generates embeddings using OpenAI
func (eg *EmbeddingGenerator) generateOpenAIEmbeddings(chunks []*kbtypes.Chunk) error {
	const batchSize = 100 // OpenAI allows up to 2048, but we'll be conservative
	config := eg.config.Embeddings.OpenAI

	// Filter chunks that need OpenAI embeddings
	var chunksToProcess []*kbtypes.Chunk
	for _, chunk := range chunks {
		if len(chunk.OpenAIEmbedding) == 0 {
			chunksToProcess = append(chunksToProcess, chunk)
		}
	}

	if len(chunksToProcess) == 0 {
		fmt.Printf("  OpenAI: All chunks already have embeddings, skipping\n")
		return nil
	}

	if len(chunksToProcess) < len(chunks) {
		fmt.Printf("  OpenAI: Processing %d chunks (%d already have OpenAI embeddings)\n",
			len(chunksToProcess), len(chunks)-len(chunksToProcess))
	} else {
		fmt.Printf("  OpenAI: Processing %d chunks\n", len(chunksToProcess))
	}

	for i := 0; i < len(chunksToProcess); i += batchSize {
		end := i + batchSize
		if end > len(chunksToProcess) {
			end = len(chunksToProcess)
		}

		batch := chunksToProcess[i:end]

		// Filter out chunks with empty text and build text array
		var validChunks []*kbtypes.Chunk
		var texts []string
		for _, chunk := range batch {
			if len(strings.TrimSpace(chunk.Text)) > 0 {
				validChunks = append(validChunks, chunk)
				texts = append(texts, chunk.Text)
			}
		}

		// Skip if no valid chunks in this batch
		if len(texts) == 0 {
			fmt.Printf("  OpenAI: Skipped batch %d-%d (all empty)\n", i+1, end)
			continue
		}

		batch = validChunks

		// Create request
		reqBody := openAIEmbeddingRequest{
			Input:      texts,
			Model:      config.Model,
			Dimensions: config.Dimensions,
		}

		jsonData, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("failed to marshal request: %w", err)
		}

		// Make API request with retry logic
		operation := fmt.Sprintf("OpenAI batch %d-%d", i+1, end)
		resp, err := retryWithBackoff(operation, func() (*http.Response, error) {
			req, err := http.NewRequest("POST", "https://api.openai.com/v1/embeddings", bytes.NewBuffer(jsonData))
			if err != nil {
				return nil, err
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+config.APIKey)
			return eg.client.Do(req)
		})
		if err != nil {
			return fmt.Errorf("failed to make request: %w", err)
		}
		defer resp.Body.Close()

		// Parse response
		var embResp openAIEmbeddingResponse
		if err := json.NewDecoder(resp.Body).Decode(&embResp); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}

		// Assign embeddings to chunks
		if len(embResp.Data) != len(batch) {
			return fmt.Errorf("expected %d embeddings, got %d", len(batch), len(embResp.Data))
		}

		for j, chunk := range batch {
			chunk.OpenAIEmbedding = embResp.Data[j].Embedding
		}

		// Save progress to database after each batch (only for existing chunks with IDs)
		if eg.db != nil && len(batch) > 0 && batch[0].ID != 0 {
			eg.dbMux.Lock()
			if err := eg.db.UpdateOpenAIEmbeddings(batch); err != nil {
				eg.dbMux.Unlock()
				return fmt.Errorf("failed to save batch to database: %w", err)
			}
			eg.dbMux.Unlock()
		}

		fmt.Printf("  OpenAI: Processed %d/%d chunks\n", end, len(chunksToProcess))
	}

	return nil
}

// Voyage API structures
type voyageEmbeddingRequest struct {
	Input []string `json:"input"`
	Model string   `json:"model"`
}

type voyageEmbeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

// generateVoyageEmbeddings generates embeddings using Voyage AI
func (eg *EmbeddingGenerator) generateVoyageEmbeddings(chunks []*kbtypes.Chunk) error {
	const batchSize = 100
	config := eg.config.Embeddings.Voyage

	// Filter chunks that need Voyage embeddings
	var chunksToProcess []*kbtypes.Chunk
	for _, chunk := range chunks {
		if len(chunk.VoyageEmbedding) == 0 {
			chunksToProcess = append(chunksToProcess, chunk)
		}
	}

	if len(chunksToProcess) == 0 {
		fmt.Printf("  Voyage: All chunks already have embeddings, skipping\n")
		return nil
	}

	if len(chunksToProcess) < len(chunks) {
		fmt.Printf("  Voyage: Processing %d chunks (%d already have Voyage embeddings)\n",
			len(chunksToProcess), len(chunks)-len(chunksToProcess))
	} else {
		fmt.Printf("  Voyage: Processing %d chunks\n", len(chunksToProcess))
	}

	for i := 0; i < len(chunksToProcess); i += batchSize {
		end := i + batchSize
		if end > len(chunksToProcess) {
			end = len(chunksToProcess)
		}

		batch := chunksToProcess[i:end]

		// Filter out chunks with empty text and build text array
		var validChunks []*kbtypes.Chunk
		var texts []string
		for _, chunk := range batch {
			if len(strings.TrimSpace(chunk.Text)) > 0 {
				validChunks = append(validChunks, chunk)
				texts = append(texts, chunk.Text)
			}
		}

		// Skip if no valid chunks in this batch
		if len(texts) == 0 {
			fmt.Printf("  Voyage: Skipped batch %d-%d (all empty)\n", i+1, end)
			continue
		}

		batch = validChunks

		// Create request
		reqBody := voyageEmbeddingRequest{
			Input: texts,
			Model: config.Model,
		}

		jsonData, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("failed to marshal request: %w", err)
		}

		// Make API request with retry logic
		operation := fmt.Sprintf("Voyage batch %d-%d", i+1, end)
		resp, err := retryWithBackoff(operation, func() (*http.Response, error) {
			req, err := http.NewRequest("POST", "https://api.voyageai.com/v1/embeddings", bytes.NewBuffer(jsonData))
			if err != nil {
				return nil, err
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+config.APIKey)
			return eg.client.Do(req)
		})
		if err != nil {
			return fmt.Errorf("failed to make request: %w", err)
		}
		defer resp.Body.Close()

		// Parse response
		var embResp voyageEmbeddingResponse
		if err := json.NewDecoder(resp.Body).Decode(&embResp); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}

		// Assign embeddings to chunks
		if len(embResp.Data) != len(batch) {
			return fmt.Errorf("expected %d embeddings, got %d", len(batch), len(embResp.Data))
		}

		for j, chunk := range batch {
			chunk.VoyageEmbedding = embResp.Data[j].Embedding
		}

		// Save progress to database after each batch (only for existing chunks with IDs)
		if eg.db != nil && len(batch) > 0 && batch[0].ID != 0 {
			eg.dbMux.Lock()
			if err := eg.db.UpdateVoyageEmbeddings(batch); err != nil {
				eg.dbMux.Unlock()
				return fmt.Errorf("failed to save batch to database: %w", err)
			}
			eg.dbMux.Unlock()
		}

		fmt.Printf("  Voyage: Processed %d/%d chunks\n", end, len(chunksToProcess))
	}

	return nil
}

// Ollama API structures
type ollamaEmbeddingRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type ollamaEmbeddingResponse struct {
	Embedding []float32 `json:"embedding"`
}

// generateOllamaEmbeddings generates embeddings using Ollama
func (eg *EmbeddingGenerator) generateOllamaEmbeddings(chunks []*kbtypes.Chunk) error {
	config := eg.config.Embeddings.Ollama
	endpoint := config.Endpoint + "/api/embeddings"

	// Filter chunks that need Ollama embeddings
	var chunksToProcess []*kbtypes.Chunk
	for _, chunk := range chunks {
		if len(chunk.OllamaEmbedding) == 0 && len(strings.TrimSpace(chunk.Text)) > 0 {
			chunksToProcess = append(chunksToProcess, chunk)
		}
	}

	if len(chunksToProcess) == 0 {
		fmt.Printf("  Ollama: All chunks already have embeddings, skipping\n")
		return nil
	}

	if len(chunksToProcess) < len(chunks) {
		fmt.Printf("  Ollama: Processing %d chunks (%d already have Ollama embeddings)\n",
			len(chunksToProcess), len(chunks)-len(chunksToProcess))
	} else {
		fmt.Printf("  Ollama: Processing %d chunks\n", len(chunksToProcess))
	}

	// Ollama processes one at a time, save every 50 chunks
	const saveInterval = 50
	var pendingSave []*kbtypes.Chunk

	for i, chunk := range chunksToProcess {

		reqBody := ollamaEmbeddingRequest{
			Model:  config.Model,
			Prompt: chunk.Text,
		}

		jsonData, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("failed to marshal request: %w", err)
		}

		// Make API request with retry logic
		operation := fmt.Sprintf("Ollama chunk %d/%d", i+1, len(chunksToProcess))
		resp, err := retryWithBackoff(operation, func() (*http.Response, error) {
			req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonData))
			if err != nil {
				return nil, err
			}
			req.Header.Set("Content-Type", "application/json")
			return eg.client.Do(req)
		})
		if err != nil {
			return fmt.Errorf("failed to make request: %w", err)
		}
		defer resp.Body.Close()

		var embResp ollamaEmbeddingResponse
		if err := json.NewDecoder(resp.Body).Decode(&embResp); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}

		chunk.OllamaEmbedding = embResp.Embedding
		pendingSave = append(pendingSave, chunk)

		// Save progress every saveInterval chunks or on last chunk (only for existing chunks with IDs)
		if len(pendingSave) >= saveInterval || i == len(chunksToProcess)-1 {
			if eg.db != nil && len(pendingSave) > 0 && pendingSave[0].ID != 0 {
				eg.dbMux.Lock()
				if err := eg.db.UpdateOllamaEmbeddings(pendingSave); err != nil {
					eg.dbMux.Unlock()
					return fmt.Errorf("failed to save chunks to database: %w", err)
				}
				eg.dbMux.Unlock()
				pendingSave = nil
			}
		}

		if (i+1)%10 == 0 || i == len(chunksToProcess)-1 {
			fmt.Printf("  Ollama: Processed %d/%d chunks\n", i+1, len(chunksToProcess))
		}
	}

	return nil
}
