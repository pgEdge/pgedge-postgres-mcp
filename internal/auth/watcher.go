/*-------------------------------------------------------------------------
 *
 * pgEdge Postgres MCP Server
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package auth

import (
    "fmt"
    "log"
    "path/filepath"
    "time"

    "github.com/fsnotify/fsnotify"
)

// FileWatcher watches a file for changes and triggers a reload callback
type FileWatcher struct {
    watcher  *fsnotify.Watcher
    filePath string
    reloadFn func() error
    done     chan bool
}

// NewFileWatcher creates a new file watcher
func NewFileWatcher(filePath string, reloadFn func() error) (*FileWatcher, error) {
    watcher, err := fsnotify.NewWatcher()
    if err != nil {
        return nil, fmt.Errorf("failed to create watcher: %w", err)
    }

    fw := &FileWatcher{
        watcher:  watcher,
        filePath: filePath,
        reloadFn: reloadFn,
        done:     make(chan bool),
    }

    // Watch the directory containing the file (not the file itself)
    // This is because editors often delete and recreate files on save
    dir := filepath.Dir(filePath)
    if err := watcher.Add(dir); err != nil {
        watcher.Close()
        return nil, fmt.Errorf("failed to watch directory %s: %w", dir, err)
    }

    return fw, nil
}

// Start begins watching for file changes
func (fw *FileWatcher) Start() {
    go fw.watch()
}

// Stop stops watching for file changes
func (fw *FileWatcher) Stop() {
    close(fw.done)
    fw.watcher.Close()
}

// watch monitors file events and triggers reloads
func (fw *FileWatcher) watch() {
    // Debounce timer to avoid multiple reloads for rapid changes
    var debounceTimer *time.Timer
    debounceDuration := 100 * time.Millisecond

    for {
        select {
        case event, ok := <-fw.watcher.Events:
            if !ok {
                return
            }

            // Only process events for our specific file
            if event.Name != fw.filePath {
                continue
            }

            // Handle write and create events (editors may delete and recreate)
            if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
                // Reset or create debounce timer
                if debounceTimer != nil {
                    debounceTimer.Stop()
                }
                debounceTimer = time.AfterFunc(debounceDuration, func() {
                    if err := fw.reloadFn(); err != nil {
                        log.Printf("[AUTH] Failed to reload %s: %v", fw.filePath, err)
                    } else {
                        log.Printf("[AUTH] Reloaded %s", fw.filePath)
                    }
                })
            }

        case err, ok := <-fw.watcher.Errors:
            if !ok {
                return
            }
            log.Printf("[AUTH] Watcher error for %s: %v", fw.filePath, err)

        case <-fw.done:
            if debounceTimer != nil {
                debounceTimer.Stop()
            }
            return
        }
    }
}
