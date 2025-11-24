package compactor

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// CacheEntry represents a cached compaction result
type CacheEntry struct {
	Key            string
	CompactedMsgs  []Message
	CompactionInfo CompactionInfo
	Summary        *Summary
	CreatedAt      time.Time
	ExpiresAt      time.Time
}

// CompactionCache provides in-memory caching of compaction results
type CompactionCache struct {
	entries map[string]*CacheEntry
	mu      sync.RWMutex
	ttl     time.Duration
}

// NewCompactionCache creates a new compaction cache
func NewCompactionCache(ttl time.Duration) *CompactionCache {
	cache := &CompactionCache{
		entries: make(map[string]*CacheEntry),
		ttl:     ttl,
	}

	// Start background cleanup goroutine if TTL is set
	if ttl > 0 {
		go cache.cleanupExpired()
	}

	return cache
}

// Get retrieves a cached compaction result
func (c *CompactionCache) Get(messages []Message, maxTokens, recentWindow int) (*CacheEntry, bool) {
	key := c.generateKey(messages, maxTokens, recentWindow)

	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.entries[key]
	if !exists {
		return nil, false
	}

	// Check if expired
	if c.ttl > 0 && time.Now().After(entry.ExpiresAt) {
		return nil, false
	}

	return entry, true
}

// Set stores a compaction result in cache
func (c *CompactionCache) Set(messages []Message, maxTokens, recentWindow int, result CompactResponse) {
	key := c.generateKey(messages, maxTokens, recentWindow)

	entry := &CacheEntry{
		Key:            key,
		CompactedMsgs:  result.Messages,
		CompactionInfo: result.CompactionInfo,
		Summary:        result.Summary,
		CreatedAt:      time.Now(),
	}

	if c.ttl > 0 {
		entry.ExpiresAt = time.Now().Add(c.ttl)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = entry
}

// Clear removes all entries from cache
func (c *CompactionCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*CacheEntry)
}

// Size returns the number of cached entries
func (c *CompactionCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.entries)
}

// generateKey creates a cache key from messages and config
func (c *CompactionCache) generateKey(messages []Message, maxTokens, recentWindow int) string {
	// Create a hash of the messages + config
	h := sha256.New()

	// Hash messages
	for _, msg := range messages {
		msgJSON, _ := json.Marshal(msg)
		h.Write(msgJSON)
	}

	// Hash config
	config := fmt.Sprintf("%d:%d", maxTokens, recentWindow)
	h.Write([]byte(config))

	return fmt.Sprintf("%x", h.Sum(nil))
}

// cleanupExpired removes expired entries periodically
func (c *CompactionCache) cleanupExpired() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()

		c.mu.Lock()
		for key, entry := range c.entries {
			if now.After(entry.ExpiresAt) {
				delete(c.entries, key)
			}
		}
		c.mu.Unlock()
	}
}

// GetStats returns cache statistics
func (c *CompactionCache) GetStats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var oldestEntry, newestEntry time.Time
	expiredCount := 0
	now := time.Now()

	for _, entry := range c.entries {
		if oldestEntry.IsZero() || entry.CreatedAt.Before(oldestEntry) {
			oldestEntry = entry.CreatedAt
		}
		if newestEntry.IsZero() || entry.CreatedAt.After(newestEntry) {
			newestEntry = entry.CreatedAt
		}
		if c.ttl > 0 && now.After(entry.ExpiresAt) {
			expiredCount++
		}
	}

	return map[string]interface{}{
		"total_entries": len(c.entries),
		"expired_count": expiredCount,
		"oldest_entry":  oldestEntry,
		"newest_entry":  newestEntry,
		"cache_ttl_sec": c.ttl.Seconds(),
	}
}
