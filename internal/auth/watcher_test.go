/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package auth

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
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

// TestWatcherDetectsKubernetesSecretRotation is a regression test for
// issue #186.
//
// Kubernetes projects Secret/ConfigMap volumes with a double symlink
// indirection: the file the app opens (e.g. "tokens.yaml") is a symlink
// to "..data/tokens.yaml", and "..data" is itself a symlink to a hidden,
// timestamped directory. On rotation, kubelet populates a new timestamped
// directory, then atomically renames a temp symlink onto "..data".
//
// The literal watched filename ("tokens.yaml") never receives an event at
// all during this - only "..data" (and a transient temp name) do, and
// neither ever equals fw.filePath. Before the fix for #186, the watcher
// filtered on an exact event-name match and only handled Write/Create,
// so this rotation was silently missed and the file was never reloaded.
func TestWatcherDetectsKubernetesSecretRotation(t *testing.T) {
	tempDir := t.TempDir()

	// Set up the "..data_v1" target directory holding the real file.
	dataV1 := filepath.Join(tempDir, "..data_v1")
	if err := os.Mkdir(dataV1, 0700); err != nil {
		t.Fatalf("failed to create data-v1 dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataV1, "tokens.yaml"), []byte("v1"), 0600); err != nil {
		t.Fatalf("failed to write v1 file: %v", err)
	}

	// "..data" symlinks to the current version directory.
	dataLink := filepath.Join(tempDir, "..data")
	if err := os.Symlink(dataV1, dataLink); err != nil {
		t.Fatalf("failed to create ..data symlink: %v", err)
	}

	// The watched path is a symlink through "..data" - this never changes.
	watchedPath := filepath.Join(tempDir, "tokens.yaml")
	if err := os.Symlink(filepath.Join(dataLink, "tokens.yaml"), watchedPath); err != nil {
		t.Fatalf("failed to create tokens.yaml symlink: %v", err)
	}

	var mu sync.Mutex
	reloadCount := 0
	reloadFn := func() error {
		mu.Lock()
		defer mu.Unlock()
		reloadCount++
		return nil
	}

	watcher, err := NewFileWatcher(watchedPath, reloadFn)
	if err != nil {
		t.Fatalf("NewFileWatcher failed: %v", err)
	}
	defer watcher.Stop()
	watcher.Start()

	time.Sleep(50 * time.Millisecond)

	// Simulate a secret rotation: populate a new version directory, then
	// atomically swap "..data" to point at it via a rename - exactly how
	// kubelet updates a projected secret volume.
	dataV2 := filepath.Join(tempDir, "..data_v2")
	if err := os.Mkdir(dataV2, 0700); err != nil {
		t.Fatalf("failed to create data-v2 dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataV2, "tokens.yaml"), []byte("v2"), 0600); err != nil {
		t.Fatalf("failed to write v2 file: %v", err)
	}

	tmpLink := filepath.Join(tempDir, "..data_tmp")
	if err := os.Symlink(dataV2, tmpLink); err != nil {
		t.Fatalf("failed to create temp symlink: %v", err)
	}
	if err := os.Rename(tmpLink, dataLink); err != nil {
		t.Fatalf("failed to atomically swap ..data symlink: %v", err)
	}

	// Give the watcher's debounce + goroutine time to react.
	time.Sleep(300 * time.Millisecond)

	// Confirm the resolved content really did change on disk (sanity check
	// that this reproduces the rotation mechanism correctly).
	got, err := os.ReadFile(watchedPath)
	if err != nil {
		t.Fatalf("failed to read watched path after rotation: %v", err)
	}
	if string(got) != "v2" {
		t.Fatalf("expected resolved content to be v2 after rotation, got %q", got)
	}

	mu.Lock()
	count := reloadCount
	mu.Unlock()

	t.Logf("reload callback fired %d time(s) after a Kubernetes-style secret rotation", count)
	if count == 0 {
		t.Error("watcher never reloaded after the ..data symlink swap, even though the " +
			"file content behind the watched path changed on disk (regression of #186)")
	}
}

// TestWatcherStop_WaitsForInFlightReload is a regression test: Stop must
// not return while a debounce-triggered checkAndReload is still running,
// or could still run. Without waiting on checkWG, a reload already
// in flight (or about to fire) when Stop is called could invoke reloadFn
// after the caller believes the watcher is fully stopped and proceeds to
// tear down whatever reloadFn touches.
func TestWatcherStop_WaitsForInFlightReload(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.yaml")
	if err := os.WriteFile(testFile, []byte("initial"), 0600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	var inFlight int32
	var mu sync.Mutex
	reloadCount := 0
	reloadFn := func() error {
		atomic.StoreInt32(&inFlight, 1)
		time.Sleep(100 * time.Millisecond)
		mu.Lock()
		reloadCount++
		mu.Unlock()
		atomic.StoreInt32(&inFlight, 0)
		return nil
	}

	watcher, err := NewFileWatcher(testFile, reloadFn)
	if err != nil {
		t.Fatalf("NewFileWatcher failed: %v", err)
	}
	watcher.Start()

	if err := os.WriteFile(testFile, []byte("updated"), 0600); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Land Stop() while the debounced reload is in flight: the debounce
	// is 100ms and reloadFn sleeps a further 100ms, so calling Stop after
	// 150ms lands squarely inside reloadFn's own sleep.
	time.Sleep(150 * time.Millisecond)
	watcher.Stop()

	if atomic.LoadInt32(&inFlight) != 0 {
		t.Fatal("BUG: Stop() returned while a reload was still in flight")
	}

	mu.Lock()
	count := reloadCount
	mu.Unlock()
	if count == 0 {
		t.Fatal("expected the in-flight reload to have completed by the time Stop() returned")
	}
}

// TestWatcherStop_Idempotent is a regression test: Stop must be safe to
// call more than once. close(fw.done) panics on a second call unless Stop
// guards against it, and callers are not guaranteed to serialize their own
// calls (e.g. a defer watcher.Stop() alongside an explicit Stop() on an
// error path, or two goroutines racing to shut down).
func TestWatcherStop_Idempotent(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.yaml")
	if err := os.WriteFile(testFile, []byte("initial"), 0600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	watcher, err := NewFileWatcher(testFile, func() error { return nil })
	if err != nil {
		t.Fatalf("NewFileWatcher failed: %v", err)
	}
	watcher.Start()

	// Sequential double-call must not panic.
	watcher.Stop()
	watcher.Stop()

	// Concurrent calls on a second watcher must not panic either.
	watcher2, err := NewFileWatcher(testFile, func() error { return nil })
	if err != nil {
		t.Fatalf("NewFileWatcher failed: %v", err)
	}
	watcher2.Start()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			watcher2.Stop()
		}()
	}
	wg.Wait()
}
