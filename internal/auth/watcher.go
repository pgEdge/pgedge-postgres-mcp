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
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
)

// FileWatcher watches a file for changes and triggers a reload callback.
//
// It watches the file's parent directory rather than the file itself, and
// reacts to any filesystem event in that directory rather than requiring
// an exact match on the watched path or a specific event type. This is
// deliberate: when a file is delivered via an atomically-swapped symlink,
// as with a Kubernetes-projected Secret or ConfigMap volume, or any tool
// that renames a new version into place, the events that fire are
// Create/Rename/Remove on a different directory entry entirely (for
// example Kubernetes' own "..data" symlink), never Write/Create on the
// watched filename itself.
//
// To decide whether a reload is actually warranted, the watcher
// re-resolves and hashes the watched path's content after a debounce
// following any directory event, and invokes the reload callback only when
// that resolved content has actually changed. This both catches changes
// that never touch the watched filename directly and avoids reloading on
// unrelated activity elsewhere in the same directory.
type FileWatcher struct {
	watcher  *fsnotify.Watcher
	filePath string
	reloadFn func() error
	done     chan bool

	// started records whether Start has been called. Stop only waits on
	// watchExited when it has: watch() is what closes watchExited, so if
	// Start was never called (a watcher can be constructed and stopped
	// without ever starting, e.g. in tests), nothing would ever close it.
	started atomic.Bool

	// watchExited is closed by watch() right before it returns. Stop waits
	// on this before waiting on checkWG: only watch() ever calls
	// checkWG.Add, so once watch() has exited, no further Add can race
	// with Stop's Wait (sync.WaitGroup forbids Add and Wait running
	// concurrently while the counter could be transitioning through
	// zero - closing fw.done alone does not guarantee watch() has
	// stopped processing a buffered event before Stop reaches Wait).
	watchExited chan struct{}

	// checkWG tracks every scheduled-but-not-yet-resolved checkAndReload
	// call (whether it goes on to run or is cleanly cancelled), so Stop
	// can wait for all of them before returning. Without this, a debounce
	// timer that has already fired - or fires concurrently with Stop -
	// could still invoke reloadFn after the caller believes the watcher
	// is fully stopped.
	checkWG sync.WaitGroup

	// stopOnce makes Stop idempotent: close(fw.done) below panics on a
	// second call, and callers are not guaranteed to serialize their own
	// calls to Stop (e.g. TokenStore/UserStore.StopWatching check-then-nil
	// their watcher field with no lock, so two concurrent StopWatching
	// calls could both reach Stop on the same *FileWatcher).
	stopOnce sync.Once

	// hashMu guards lastHash/hasHash, since a new debounce timer can start
	// before an in-flight one's callback has finished (Stop does not wait
	// for an already-fired timer's goroutine to complete).
	hashMu   sync.Mutex
	lastHash [sha256.Size]byte
	hasHash  bool
}

// NewFileWatcher creates a new file watcher.
func NewFileWatcher(filePath string, reloadFn func() error) (*FileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create watcher: %w", err)
	}

	fw := &FileWatcher{
		watcher:     watcher,
		filePath:    filePath,
		reloadFn:    reloadFn,
		done:        make(chan bool),
		watchExited: make(chan struct{}),
	}

	// Watch the directory containing the file, not the file itself.
	// Editors delete and recreate on save, and orchestrators that project
	// mounted secrets swap an internal symlink rather than touching this
	// path directly - both only ever produce events on the directory.
	dir := filepath.Dir(filePath)
	if err := watcher.Add(dir); err != nil {
		watcher.Close()
		return nil, fmt.Errorf("failed to watch directory %s: %w", dir, err)
	}

	// Record a baseline so the first real change is detected as a change,
	// not silently treated as "no different from before we were watching".
	if hash, ok := hashFile(filePath); ok {
		fw.lastHash = hash
		fw.hasHash = true
	}

	return fw, nil
}

// Start begins watching for file changes.
func (fw *FileWatcher) Start() {
	fw.started.Store(true)
	go fw.watch()
}

// Stop stops watching for file changes. It blocks until any
// already-scheduled or in-flight checkAndReload call has resolved, so no
// reloadFn invocation can happen after Stop returns. Safe to call more
// than once, including concurrently; only the first call does anything,
// and every call - including redundant ones - returns only once that
// first call has fully completed.
func (fw *FileWatcher) Stop() {
	fw.stopOnce.Do(func() {
		close(fw.done)
		fw.watcher.Close()
		if fw.started.Load() {
			<-fw.watchExited
		}
		fw.checkWG.Wait()
	})
}

// watch monitors directory events and triggers a content check, debounced,
// whenever one occurs.
func (fw *FileWatcher) watch() {
	// Signal Stop that no further checkWG.Add call can happen, however
	// this goroutine exits - watch is the only caller of Add.
	defer close(fw.watchExited)

	// Debounce timer to avoid a check per event for rapid changes.
	var debounceTimer *time.Timer
	debounceDuration := 100 * time.Millisecond

	for {
		select {
		case _, ok := <-fw.watcher.Events:
			if !ok {
				return
			}

			// Any event in the watched directory - on any name, of any
			// type - could mean the content resolved by fw.filePath has
			// changed: a direct write, an editor's delete-and-recreate, or
			// a symlink swap on an entirely different directory entry.
			// checkAndReload re-resolves and hashes fw.filePath itself, so
			// unrelated activity is filtered out there instead of here by
			// matching the event's name or type.
			if debounceTimer != nil && debounceTimer.Stop() {
				// Successfully cancelled before firing: the callback below
				// will never run, so its Add(1) needs a matching Done here.
				fw.checkWG.Done()
			}
			fw.checkWG.Add(1)
			debounceTimer = time.AfterFunc(debounceDuration, func() {
				defer fw.checkWG.Done()
				fw.checkAndReload()
			})

		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("[AUTH] Watcher error for %s: %v", fw.filePath, err)

		case <-fw.done:
			if debounceTimer != nil && debounceTimer.Stop() {
				fw.checkWG.Done()
			}
			return
		}
	}
}

// checkAndReload re-resolves and hashes the watched path, invoking the
// reload callback only if the content differs from the last check.
//
// The whole check-compare-update sequence, including the reload callback
// itself, runs under hashMu. A new debounce timer can start before an
// already-fired one's goroutine has finished, so without this lock two
// overlapping calls could both observe the old hash as "changed" and race
// on updating it; holding the lock for the full call also means two
// reloads never run concurrently against each other.
func (fw *FileWatcher) checkAndReload() {
	fw.hashMu.Lock()
	defer fw.hashMu.Unlock()

	hash, ok := hashFile(fw.filePath)
	if !ok {
		// The path is transiently missing or unreadable, e.g. mid-swap.
		// Nothing to reload yet; a later event will trigger another check.
		return
	}

	if fw.hasHash && hash == fw.lastHash {
		return
	}

	// Record the hash only after a successful reload. If reloadFn fails,
	// leaving lastHash unchanged means the next directory event retries the
	// same content rather than treating it as already handled - otherwise a
	// transient failure (e.g. a one-off read error on otherwise-valid new
	// content) would strand the store on stale data until the file changed
	// again, which is exactly the missed-update class of bug this watcher
	// exists to prevent.
	if err := fw.reloadFn(); err != nil {
		log.Printf("[AUTH] Failed to reload %s: %v", fw.filePath, err)
		return
	}
	fw.lastHash = hash
	fw.hasHash = true
	log.Printf("[AUTH] Reloaded %s", fw.filePath)
}

// hashFile returns the SHA-256 hash of the file at path, transparently
// following any symlinks along the way. ok is false if the file could not
// be opened or read.
func hashFile(path string) (hash [sha256.Size]byte, ok bool) {
	f, err := os.Open(path)
	if err != nil {
		return hash, false
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return hash, false
	}
	copy(hash[:], h.Sum(nil))
	return hash, true
}
