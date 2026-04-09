/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

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
	defaultMaxRetries = 5
	initialBackoff    = 1 * time.Second
	maxBackoff        = 60 * time.Second
)

// EmbeddingGenerator generates embeddings using configured providers
type EmbeddingGenerator struct {
	config     *kbconfig.Config
	client     *http.Client
	db         *kbdatabase.Database
	dbMux      sync.Mutex // Protects database writes from concurrent providers
	maxRetries int        // Maximum number of retries for transient errors (0 = unlimited)
}

// NewEmbeddingGenerator creates a new embedding generator.
// maxRetries controls how many times transient API errors are retried.
// Pass -1 to use the default (5), or 0 to retry indefinitely.
func NewEmbeddingGenerator(config *kbconfig.Config, db *kbdatabase.Database, maxRetries int) *EmbeddingGenerator {
	// Use longer timeout for Ollama (models may need initialization, slower processing)
	// OpenAI/Voyage typically respond in seconds, but Ollama can take much longer
	timeout := 5 * time.Minute

	if maxRetries < 0 {
		maxRetries = defaultMaxRetries
	}

	return &EmbeddingGenerator{
		config: config,
		client: &http.Client{
			Timeout: timeout,
		},
		db:         db,
		maxRetries: maxRetries,
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
	resultChan := make(chan providerResult, 4)

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

	if eg.config.Embeddings.Gemini.Enabled {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fmt.Printf("Starting Gemini embeddings...\n")
			providerStart := time.Now()
			if err := eg.generateGeminiEmbeddings(chunks); err != nil {
				fmt.Printf("⚠️  Gemini embeddings failed: %v\n", err)
				resultChan <- providerResult{"Gemini", err}
				return
			}
			fmt.Printf("✓ Gemini embeddings completed in %.2fs\n", time.Since(providerStart).Seconds())
			resultChan <- providerResult{"Gemini", nil}
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

// retryWithBackoff executes a function with exponential backoff retry logic.
// When eg.maxRetries is 0, the function retries indefinitely until success
// or a non-retryable error occurs.
func (eg *EmbeddingGenerator) retryWithBackoff(operation string, fn func() (*http.Response, error)) (*http.Response, error) {
	var lastErr error
	backoff := initialBackoff
	limit := eg.maxRetries
	unlimited := limit == 0

	for attempt := 0; unlimited || attempt <= limit; attempt++ {
		if attempt > 0 {
			if unlimited {
				fmt.Printf("  Retry %d for %s after %.1fs...\n", attempt, operation, backoff.Seconds())
			} else {
				fmt.Printf("  Retry %d/%d for %s after %.1fs...\n", attempt, limit, operation, backoff.Seconds())
			}
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

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				body = []byte("(could not read response body)")
			}
			resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))

			// Context length errors are deterministic; retrying
			// the same input will never succeed.
			if strings.Contains(string(body), ollamaContextLengthError) {
				return nil, lastErr
			}

			if resp.StatusCode == 429 && (unlimited || attempt < limit) {
				fmt.Printf("  Rate limited, will retry...\n")
			}
			continue
		}

		// Non-retryable error
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			body = []byte("(could not read response body)")
		}
		resp.Body.Close()
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil, fmt.Errorf("failed after %d retries: %w", limit, lastErr)
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
			if strings.TrimSpace(chunk.Text) != "" {
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
		resp, err := eg.retryWithBackoff(operation, func() (*http.Response, error) {
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

		// Parse response
		var embResp openAIEmbeddingResponse
		if err := json.NewDecoder(resp.Body).Decode(&embResp); err != nil {
			resp.Body.Close()
			return fmt.Errorf("failed to decode response: %w", err)
		}
		resp.Body.Close()

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
			if strings.TrimSpace(chunk.Text) != "" {
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
		resp, err := eg.retryWithBackoff(operation, func() (*http.Response, error) {
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

		// Parse response
		var embResp voyageEmbeddingResponse
		if err := json.NewDecoder(resp.Body).Decode(&embResp); err != nil {
			resp.Body.Close()
			return fmt.Errorf("failed to decode response: %w", err)
		}
		resp.Body.Close()

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

// ollamaContextLengthError is the error substring returned by Ollama
// when the input exceeds the model's context window.
const ollamaContextLengthError = "the input length exceeds the context length"

// isContextLengthError checks whether an error from retryWithBackoff
// contains the Ollama context-length error message.
func isContextLengthError(err error) bool {
	return err != nil && strings.Contains(err.Error(), ollamaContextLengthError)
}

// truncateAtWordBoundary truncates text to approximately the given
// fraction of its original length, cutting at a word boundary so
// that no word is split in the middle.  The fraction must be between
// 0 (exclusive) and 1 (inclusive).
func truncateAtWordBoundary(text string, fraction float64) string {
	if fraction >= 1.0 {
		return text
	}
	targetLen := int(float64(len(text)) * fraction)
	if targetLen <= 0 {
		return ""
	}
	if targetLen >= len(text) {
		return text
	}

	// Walk backwards from targetLen to find a space.
	cut := targetLen
	for cut > 0 && text[cut] != ' ' {
		cut--
	}
	// If we walked all the way back, fall back to the hard limit so
	// we still make progress.
	if cut == 0 {
		cut = targetLen
	}
	return strings.TrimSpace(text[:cut])
}

// Ollama API structures
type ollamaEmbeddingRequest struct {
	Model   string                 `json:"model"`
	Prompt  string                 `json:"prompt"`
	Options map[string]interface{} `json:"options,omitempty"`
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
		if len(chunk.OllamaEmbedding) == 0 && strings.TrimSpace(chunk.Text) != "" {
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
		promptText := chunk.Text

		embedding, err := eg.ollamaEmbedSingle(
			endpoint, config.Model, config.ContextLength, promptText,
			fmt.Sprintf("Ollama chunk %d/%d", i+1, len(chunksToProcess)),
		)

		// If the initial attempt failed with a context-length error,
		// retry with progressively truncated text.
		if isContextLengthError(err) {
			fmt.Printf("  Ollama: Chunk %d/%d exceeds context length (%d chars, %d words); trying truncation\n",
				i+1, len(chunksToProcess), len(chunk.Text), len(strings.Fields(chunk.Text)))

			truncated := false
			for _, fraction := range []float64{0.75, 0.50, 0.25} {
				shortened := truncateAtWordBoundary(promptText, fraction)
				if shortened == "" {
					break
				}
				fmt.Printf("    Trying %.0f%% truncation (%d chars)...\n", fraction*100, len(shortened))

				embedding, err = eg.ollamaEmbedSingle(
					endpoint, config.Model, config.ContextLength, shortened,
					fmt.Sprintf("Ollama chunk %d/%d (%.0f%%)", i+1, len(chunksToProcess), fraction*100),
				)
				if err == nil {
					truncated = true
					break
				}
				if !isContextLengthError(err) {
					// A different error occurred; stop trying.
					break
				}
			}

			if err != nil {
				// All truncation attempts failed -- skip this chunk.
				fmt.Printf("  WARNING: Skipping chunk %d/%d after all truncation attempts failed\n", i+1, len(chunksToProcess))
				fmt.Printf("     File: %s\n", chunk.FilePath)
				fmt.Printf("     Section: %s\n", chunk.Section)
				fmt.Printf("     Chars: %d, Words: %d\n", len(chunk.Text), len(strings.Fields(chunk.Text)))
				continue
			}
			if truncated {
				fmt.Printf("    Succeeded with truncated text for chunk %d/%d\n", i+1, len(chunksToProcess))
			}
		} else if err != nil {
			// Non-context-length error -- fail as before.
			fmt.Printf("\n  Failed chunk details:\n")
			fmt.Printf("     File: %s\n", chunk.FilePath)
			fmt.Printf("     Section: %s\n", chunk.Section)
			fmt.Printf("     Chars: %d, Words: %d\n", len(chunk.Text), len(strings.Fields(chunk.Text)))
			fmt.Printf("     Preview: %.200s...\n", chunk.Text)
			return fmt.Errorf("failed to make request: %w", err)
		}

		chunk.OllamaEmbedding = embedding
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

// ollamaEmbedSingle sends a single embedding request to Ollama and
// returns the resulting embedding vector.  It uses retryWithBackoff
// for transient errors.
func (eg *EmbeddingGenerator) ollamaEmbedSingle(
	endpoint, model string, contextLength int, text, operation string,
) ([]float32, error) {

	reqBody := ollamaEmbeddingRequest{
		Model:  model,
		Prompt: text,
		Options: map[string]interface{}{
			"num_ctx": contextLength,
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := eg.retryWithBackoff(operation, func() (*http.Response, error) {
		req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonData))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		return eg.client.Do(req)
	})
	if err != nil {
		return nil, err
	}

	var embResp ollamaEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embResp); err != nil {
		resp.Body.Close()
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	resp.Body.Close()

	return embResp.Embedding, nil
}

// Gemini API structures
type geminiEmbedBatchRequest struct {
	Model   string `json:"model"`
	Content struct {
		Parts []struct {
			Text string `json:"text"`
		} `json:"parts"`
	} `json:"content"`
}

type geminiEmbedBatchResponse struct {
	Embedding struct {
		Values []float32 `json:"values"`
	} `json:"embedding"`
}

// generateGeminiEmbeddings generates embeddings using Google Gemini
func (eg *EmbeddingGenerator) generateGeminiEmbeddings(chunks []*kbtypes.Chunk) error {
	config := eg.config.Embeddings.Gemini
	endpoint := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:embedContent?key=%s",
		config.Model, config.APIKey)

	var chunksToProcess []*kbtypes.Chunk
	for _, chunk := range chunks {
		if len(chunk.GeminiEmbedding) == 0 && strings.TrimSpace(chunk.Text) != "" {
			chunksToProcess = append(chunksToProcess, chunk)
		}
	}

	if len(chunksToProcess) == 0 {
		fmt.Printf("  Gemini: All chunks already have embeddings, skipping\n")
		return nil
	}

	if len(chunksToProcess) < len(chunks) {
		fmt.Printf("  Gemini: Processing %d chunks (%d already have Gemini embeddings)\n",
			len(chunksToProcess), len(chunks)-len(chunksToProcess))
	} else {
		fmt.Printf("  Gemini: Processing %d chunks\n", len(chunksToProcess))
	}

	const saveInterval = 50
	var pendingSave []*kbtypes.Chunk

	for i, chunk := range chunksToProcess {
		reqBody := geminiEmbedBatchRequest{
			Model: "models/" + config.Model,
		}
		reqBody.Content.Parts = []struct {
			Text string `json:"text"`
		}{{Text: chunk.Text}}

		jsonData, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("failed to marshal request: %w", err)
		}

		operation := fmt.Sprintf("Gemini chunk %d/%d", i+1, len(chunksToProcess))
		resp, err := eg.retryWithBackoff(operation, func() (*http.Response, error) {
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

		var embResp geminiEmbedBatchResponse
		if err := json.NewDecoder(resp.Body).Decode(&embResp); err != nil {
			resp.Body.Close()
			return fmt.Errorf("failed to decode response: %w", err)
		}
		resp.Body.Close()

		chunk.GeminiEmbedding = embResp.Embedding.Values
		pendingSave = append(pendingSave, chunk)

		if len(pendingSave) >= saveInterval || i == len(chunksToProcess)-1 {
			if eg.db != nil && len(pendingSave) > 0 && pendingSave[0].ID != 0 {
				eg.dbMux.Lock()
				if err := eg.db.UpdateGeminiEmbeddings(pendingSave); err != nil {
					eg.dbMux.Unlock()
					return fmt.Errorf("failed to save chunks to database: %w", err)
				}
				eg.dbMux.Unlock()
				pendingSave = nil
			}
		}

		if (i+1)%10 == 0 || i == len(chunksToProcess)-1 {
			fmt.Printf("  Gemini: Processed %d/%d chunks\n", i+1, len(chunksToProcess))
		}
	}

	return nil
}
