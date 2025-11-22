/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package auth

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestNewFileWatcher tests basic watcher creation
func TestNewFileWatcher(t *testing.T) {
	// Create temporary directory and file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.yaml")

	// Create test file
	if err := os.WriteFile(testFile, []byte("test"), 0600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test successful watcher creation
	reloadFn := func() error {
		return nil
	}

	watcher, err := NewFileWatcher(testFile, reloadFn)
	if err != nil {
		t.Fatalf("NewFileWatcher failed: %v", err)
	}
	defer watcher.Stop()

	// Verify watcher was created
	if watcher == nil {
		t.Fatal("Expected non-nil watcher")
	}
	if watcher.filePath != testFile {
		t.Errorf("Expected filePath %s, got %s", testFile, watcher.filePath)
	}
}

// TestNewFileWatcherInvalidDirectory tests error handling for invalid directory
func TestNewFileWatcherInvalidDirectory(t *testing.T) {
	reloadFn := func() error { return nil }

	// Try to watch file in non-existent directory
	invalidPath := "/nonexistent/directory/file.yaml"
	_, err := NewFileWatcher(invalidPath, reloadFn)
	if err == nil {
		t.Fatal("Expected error for invalid directory, got nil")
	}
}

// TestWatcherReloadOnWrite tests that file writes trigger reload
func TestWatcherReloadOnWrite(t *testing.T) {
	// Create temporary directory and file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.yaml")

	// Create test file
	if err := os.WriteFile(testFile, []byte("initial"), 0600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Track reload calls
	var mu sync.Mutex
	reloadCount := 0
	reloadFn := func() error {
		mu.Lock()
		defer mu.Unlock()
		reloadCount++
		return nil
	}

	// Create and start watcher
	watcher, err := NewFileWatcher(testFile, reloadFn)
	if err != nil {
		t.Fatalf("NewFileWatcher failed: %v", err)
	}
	defer watcher.Stop()

	watcher.Start()

	// Write to file
	if err := os.WriteFile(testFile, []byte("updated"), 0600); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Wait for debounce timer and reload
	time.Sleep(200 * time.Millisecond)

	// Verify reload was called
	mu.Lock()
	count := reloadCount
	mu.Unlock()

	if count == 0 {
		t.Error("Expected reload to be called after write")
	}
}

// TestWatcherReloadOnCreate tests that file creation triggers reload
func TestWatcherReloadOnCreate(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.yaml")

	// Create initial file
	if err := os.WriteFile(testFile, []byte("initial"), 0600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Track reload calls
	var mu sync.Mutex
	reloadCount := 0
	reloadFn := func() error {
		mu.Lock()
		defer mu.Unlock()
		reloadCount++
		return nil
	}

	// Create and start watcher
	watcher, err := NewFileWatcher(testFile, reloadFn)
	if err != nil {
		t.Fatalf("NewFileWatcher failed: %v", err)
	}
	defer watcher.Stop()

	watcher.Start()

	// Wait a moment for watcher to initialize
	time.Sleep(50 * time.Millisecond)

	// Simulate editor behavior: delete and recreate file
	if err := os.Remove(testFile); err != nil {
		t.Fatalf("Failed to remove test file: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	if err := os.WriteFile(testFile, []byte("recreated"), 0600); err != nil {
		t.Fatalf("Failed to recreate test file: %v", err)
	}

	// Wait for debounce timer and reload
	time.Sleep(200 * time.Millisecond)

	// Verify reload was called
	mu.Lock()
	count := reloadCount
	mu.Unlock()

	if count == 0 {
		t.Error("Expected reload to be called after file recreation")
	}
}

// TestWatcherDebouncing tests that rapid changes are debounced
func TestWatcherDebouncing(t *testing.T) {
	// Create temporary directory and file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.yaml")

	// Create test file
	if err := os.WriteFile(testFile, []byte("initial"), 0600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Track reload calls
	var mu sync.Mutex
	reloadCount := 0
	reloadFn := func() error {
		mu.Lock()
		defer mu.Unlock()
		reloadCount++
		return nil
	}

	// Create and start watcher
	watcher, err := NewFileWatcher(testFile, reloadFn)
	if err != nil {
		t.Fatalf("NewFileWatcher failed: %v", err)
	}
	defer watcher.Stop()

	watcher.Start()

	// Make rapid consecutive writes (within debounce window)
	for i := 0; i < 5; i++ {
		if err := os.WriteFile(testFile, []byte("rapid update"), 0600); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}
		time.Sleep(20 * time.Millisecond) // Less than 100ms debounce
	}

	// Wait for debounce timer to complete
	time.Sleep(200 * time.Millisecond)

	// Verify reload was called only once (or very few times) due to debouncing
	mu.Lock()
	count := reloadCount
	mu.Unlock()

	if count == 0 {
		t.Error("Expected at least one reload call")
	}
	if count > 2 {
		t.Errorf("Expected debouncing to limit reloads, got %d calls for 5 writes", count)
	}
}

// TestWatcherStop tests that watcher cleanup works correctly
func TestWatcherStop(t *testing.T) {
	// Create temporary directory and file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.yaml")

	// Create test file
	if err := os.WriteFile(testFile, []byte("initial"), 0600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Track reload calls
	var mu sync.Mutex
	reloadCount := 0
	reloadFn := func() error {
		mu.Lock()
		defer mu.Unlock()
		reloadCount++
		return nil
	}

	// Create and start watcher
	watcher, err := NewFileWatcher(testFile, reloadFn)
	if err != nil {
		t.Fatalf("NewFileWatcher failed: %v", err)
	}

	watcher.Start()

	// Wait for watcher to initialize
	time.Sleep(50 * time.Millisecond)

	// Stop watcher
	watcher.Stop()

	// Wait for stop to take effect
	time.Sleep(50 * time.Millisecond)

	// Write to file after stopping
	if err := os.WriteFile(testFile, []byte("after stop"), 0600); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Wait to see if reload is called
	time.Sleep(200 * time.Millisecond)

	// Verify reload was NOT called after stop
	mu.Lock()
	count := reloadCount
	mu.Unlock()

	if count > 0 {
		t.Errorf("Expected no reloads after Stop(), got %d", count)
	}
}

// TestWatcherIgnoresOtherFiles tests that only the target file triggers reload
func TestWatcherIgnoresOtherFiles(t *testing.T) {
	// Create temporary directory and file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.yaml")
	otherFile := filepath.Join(tempDir, "other.yaml")

	// Create test files
	if err := os.WriteFile(testFile, []byte("test"), 0600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	if err := os.WriteFile(otherFile, []byte("other"), 0600); err != nil {
		t.Fatalf("Failed to create other file: %v", err)
	}

	// Track reload calls
	var mu sync.Mutex
	reloadCount := 0
	reloadFn := func() error {
		mu.Lock()
		defer mu.Unlock()
		reloadCount++
		return nil
	}

	// Create and start watcher
	watcher, err := NewFileWatcher(testFile, reloadFn)
	if err != nil {
		t.Fatalf("NewFileWatcher failed: %v", err)
	}
	defer watcher.Stop()

	watcher.Start()

	// Write to OTHER file (should not trigger reload)
	if err := os.WriteFile(otherFile, []byte("updated other"), 0600); err != nil {
		t.Fatalf("Failed to write other file: %v", err)
	}

	// Wait for potential reload
	time.Sleep(200 * time.Millisecond)

	// Verify reload was NOT called
	mu.Lock()
	countAfterOther := reloadCount
	mu.Unlock()

	if countAfterOther > 0 {
		t.Errorf("Expected no reload for other file, got %d calls", countAfterOther)
	}

	// Now write to TARGET file (should trigger reload)
	if err := os.WriteFile(testFile, []byte("updated test"), 0600); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Wait for reload
	time.Sleep(200 * time.Millisecond)

	// Verify reload WAS called
	mu.Lock()
	countAfterTest := reloadCount
	mu.Unlock()

	if countAfterTest == 0 {
		t.Error("Expected reload after target file write")
	}
}

// TestWatcherReloadError tests that reload errors are handled gracefully
func TestWatcherReloadError(t *testing.T) {
	// Create temporary directory and file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.yaml")

	// Create test file
	if err := os.WriteFile(testFile, []byte("initial"), 0600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create reload function that returns error
	reloadFn := func() error {
		return os.ErrPermission
	}

	// Create and start watcher
	watcher, err := NewFileWatcher(testFile, reloadFn)
	if err != nil {
		t.Fatalf("NewFileWatcher failed: %v", err)
	}
	defer watcher.Stop()

	watcher.Start()

	// Write to file (reload will error, but watcher should continue)
	if err := os.WriteFile(testFile, []byte("updated"), 0600); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Wait for reload attempt
	time.Sleep(200 * time.Millisecond)

	// If we reach here without panic/crash, the error was handled gracefully
	// (The error is logged but doesn't crash the watcher)
}

// TestWatcherConcurrentAccess tests thread safety with concurrent file updates
func TestWatcherConcurrentAccess(t *testing.T) {
	// Create temporary directory and file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.yaml")

	// Create test file
	if err := os.WriteFile(testFile, []byte("initial"), 0600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Track reload calls with mutex
	var mu sync.Mutex
	reloadCount := 0
	reloadFn := func() error {
		mu.Lock()
		defer mu.Unlock()
		reloadCount++
		// Simulate some work
		time.Sleep(10 * time.Millisecond)
		return nil
	}

	// Create and start watcher
	watcher, err := NewFileWatcher(testFile, reloadFn)
	if err != nil {
		t.Fatalf("NewFileWatcher failed: %v", err)
	}
	defer watcher.Stop()

	watcher.Start()

	// Launch concurrent writers
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 2; j++ {
				if err := os.WriteFile(testFile, []byte("concurrent"), 0600); err != nil {
					t.Logf("Write failed: %v", err)
				}
				time.Sleep(50 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()

	// Wait for all reloads to complete
	time.Sleep(300 * time.Millisecond)

	// Just verify no panics occurred - exact count is non-deterministic
	// due to debouncing and timing
	mu.Lock()
	count := reloadCount
	mu.Unlock()

	if count == 0 {
		t.Error("Expected at least one reload from concurrent writes")
	}
}
