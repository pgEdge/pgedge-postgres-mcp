/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package compactor

import (
	"testing"
	"time"
)

func TestCompactionCache_Basic(t *testing.T) {
	cache := NewCompactionCache(0) // No TTL

	messages := []Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there"},
	}

	result := CompactResponse{
		Messages: []Message{{Role: "user", Content: "Hello"}},
		CompactionInfo: CompactionInfo{
			OriginalCount:  2,
			CompactedCount: 1,
			DroppedCount:   1,
		},
	}

	// Store in cache
	cache.Set(messages, 1000, 10, result)

	// Retrieve from cache
	cached, found := cache.Get(messages, 1000, 10)
	if !found {
		t.Fatal("Expected to find cached entry")
	}

	if len(cached.CompactedMsgs) != 1 {
		t.Errorf("Cached messages = %v, want 1 message", len(cached.CompactedMsgs))
	}

	if cached.CompactionInfo.OriginalCount != 2 {
		t.Errorf("OriginalCount = %v, want 2", cached.CompactionInfo.OriginalCount)
	}
}

func TestCompactionCache_DifferentParams(t *testing.T) {
	cache := NewCompactionCache(0)

	messages := []Message{
		{Role: "user", Content: "Hello"},
	}

	result1 := CompactResponse{
		Messages: []Message{{Role: "user", Content: "Result 1"}},
	}

	result2 := CompactResponse{
		Messages: []Message{{Role: "user", Content: "Result 2"}},
	}

	// Store with different parameters
	cache.Set(messages, 1000, 10, result1)
	cache.Set(messages, 2000, 10, result2)

	// Retrieve should get correct result based on parameters
	cached1, found1 := cache.Get(messages, 1000, 10)
	if !found1 {
		t.Fatal("Expected to find cached entry 1")
	}

	cached2, found2 := cache.Get(messages, 2000, 10)
	if !found2 {
		t.Fatal("Expected to find cached entry 2")
	}

	if len(cached1.CompactedMsgs) != 1 || cached1.CompactedMsgs[0].Content != "Result 1" {
		t.Errorf("Got wrong cached result 1")
	}

	if len(cached2.CompactedMsgs) != 1 || cached2.CompactedMsgs[0].Content != "Result 2" {
		t.Errorf("Got wrong cached result 2")
	}
}

func TestCompactionCache_NotFound(t *testing.T) {
	cache := NewCompactionCache(0)

	messages := []Message{
		{Role: "user", Content: "Hello"},
	}

	// Try to get from empty cache
	_, found := cache.Get(messages, 1000, 10)
	if found {
		t.Error("Expected not to find entry in empty cache")
	}
}

func TestCompactionCache_TTL(t *testing.T) {
	cache := NewCompactionCache(100 * time.Millisecond)

	messages := []Message{
		{Role: "user", Content: "Hello"},
	}

	result := CompactResponse{
		Messages: []Message{{Role: "user", Content: "Hello"}},
	}

	// Store in cache
	cache.Set(messages, 1000, 10, result)

	// Should find immediately
	_, found := cache.Get(messages, 1000, 10)
	if !found {
		t.Error("Expected to find fresh cached entry")
	}

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Should not find after expiration
	_, found = cache.Get(messages, 1000, 10)
	if found {
		t.Error("Expected not to find expired entry")
	}
}

func TestCompactionCache_Clear(t *testing.T) {
	cache := NewCompactionCache(0)

	messages := []Message{
		{Role: "user", Content: "Hello"},
	}

	result := CompactResponse{
		Messages: []Message{{Role: "user", Content: "Hello"}},
	}

	// Store in cache
	cache.Set(messages, 1000, 10, result)

	// Should find before clear
	_, found := cache.Get(messages, 1000, 10)
	if !found {
		t.Error("Expected to find cached entry before clear")
	}

	// Clear cache
	cache.Clear()

	// Should not find after clear
	_, found = cache.Get(messages, 1000, 10)
	if found {
		t.Error("Expected not to find entry after clear")
	}
}

func TestCompactionCache_GenerateKey(t *testing.T) {
	cache := NewCompactionCache(0)

	messages1 := []Message{
		{Role: "user", Content: "Hello"},
	}

	messages2 := []Message{
		{Role: "user", Content: "Hello"},
	}

	messages3 := []Message{
		{Role: "user", Content: "Different"},
	}

	// Same messages should produce same key
	key1 := cache.generateKey(messages1, 1000, 10)
	key2 := cache.generateKey(messages2, 1000, 10)
	if key1 != key2 {
		t.Error("Expected same key for identical messages")
	}

	// Different messages should produce different key
	key3 := cache.generateKey(messages3, 1000, 10)
	if key1 == key3 {
		t.Error("Expected different key for different messages")
	}

	// Different params should produce different key
	key4 := cache.generateKey(messages1, 2000, 10)
	if key1 == key4 {
		t.Error("Expected different key for different maxTokens")
	}

	key5 := cache.generateKey(messages1, 1000, 20)
	if key1 == key5 {
		t.Error("Expected different key for different recentWindow")
	}
}

func TestCompactionCache_Size(t *testing.T) {
	cache := NewCompactionCache(0)

	if cache.Size() != 0 {
		t.Errorf("Empty cache size = %v, want 0", cache.Size())
	}

	messages := []Message{
		{Role: "user", Content: "Hello"},
	}

	result := CompactResponse{
		Messages: []Message{{Role: "user", Content: "Hello"}},
	}

	cache.Set(messages, 1000, 10, result)

	if cache.Size() != 1 {
		t.Errorf("Cache size after one insert = %v, want 1", cache.Size())
	}

	cache.Set(messages, 2000, 10, result) // Different params = different entry

	if cache.Size() != 2 {
		t.Errorf("Cache size after two inserts = %v, want 2", cache.Size())
	}

	cache.Clear()

	if cache.Size() != 0 {
		t.Errorf("Cache size after clear = %v, want 0", cache.Size())
	}
}
