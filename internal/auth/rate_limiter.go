/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package auth

import (
	"sync"
	"time"
)

// RateLimiter tracks failed authentication attempts per IP address
// Uses a sliding window approach to limit brute force attacks
type RateLimiter struct {
	mu              sync.RWMutex
	attempts        map[string][]time.Time // IP address -> timestamps of failed attempts
	windowDuration  time.Duration          // Time window for rate limiting
	maxAttempts     int                    // Maximum attempts allowed per window
	cleanupInterval time.Duration          // How often to clean up old entries
	stopCleanup     chan bool              // Channel to signal cleanup goroutine to stop
}

// NewRateLimiter creates a new rate limiter with the specified parameters
// cleanupInterval specifies how often to clean up old entries (0 = default of 1 minute)
func NewRateLimiter(windowMinutes int, maxAttempts int, cleanupInterval ...time.Duration) *RateLimiter {
	cleanup := time.Minute // default to 1 minute
	if len(cleanupInterval) > 0 && cleanupInterval[0] > 0 {
		cleanup = cleanupInterval[0]
	}

	rl := &RateLimiter{
		attempts:        make(map[string][]time.Time),
		windowDuration:  time.Duration(windowMinutes) * time.Minute,
		maxAttempts:     maxAttempts,
		cleanupInterval: cleanup,
		stopCleanup:     make(chan bool),
	}

	// Start background cleanup goroutine
	go rl.cleanupLoop()

	return rl
}

// IsAllowed checks if an IP address is allowed to make an authentication attempt
// Returns true if the IP has not exceeded the rate limit
func (rl *RateLimiter) IsAllowed(ipAddress string) bool {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	attempts, exists := rl.attempts[ipAddress]
	if !exists {
		return true
	}

	// Count attempts within the time window
	cutoff := time.Now().Add(-rl.windowDuration)
	validAttempts := 0
	for _, timestamp := range attempts {
		if timestamp.After(cutoff) {
			validAttempts++
		}
	}

	return validAttempts < rl.maxAttempts
}

// RecordFailedAttempt records a failed authentication attempt for an IP address
func (rl *RateLimiter) RecordFailedAttempt(ipAddress string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	if rl.attempts[ipAddress] == nil {
		rl.attempts[ipAddress] = []time.Time{now}
	} else {
		rl.attempts[ipAddress] = append(rl.attempts[ipAddress], now)
	}
}

// Reset clears all failed attempts for an IP address
// This can be called after a successful authentication
func (rl *RateLimiter) Reset(ipAddress string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	delete(rl.attempts, ipAddress)
}

// cleanupLoop periodically removes old entries that are outside the time window
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.cleanup()
		case <-rl.stopCleanup:
			return
		}
	}
}

// cleanup removes old attempts that are outside the time window
func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := time.Now().Add(-rl.windowDuration)

	for ip, attempts := range rl.attempts {
		// Filter out old attempts
		validAttempts := make([]time.Time, 0)
		for _, timestamp := range attempts {
			if timestamp.After(cutoff) {
				validAttempts = append(validAttempts, timestamp)
			}
		}

		// Update or remove entry
		if len(validAttempts) > 0 {
			rl.attempts[ip] = validAttempts
		} else {
			delete(rl.attempts, ip)
		}
	}
}

// Stop stops the cleanup goroutine
// Should be called when shutting down the server
func (rl *RateLimiter) Stop() {
	close(rl.stopCleanup)
}

// GetRemainingAttempts returns the number of attempts remaining for an IP address
// Returns -1 if unlimited (no attempts recorded yet)
func (rl *RateLimiter) GetRemainingAttempts(ipAddress string) int {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	attempts, exists := rl.attempts[ipAddress]
	if !exists {
		return rl.maxAttempts
	}

	// Count attempts within the time window
	cutoff := time.Now().Add(-rl.windowDuration)
	validAttempts := 0
	for _, timestamp := range attempts {
		if timestamp.After(cutoff) {
			validAttempts++
		}
	}

	remaining := rl.maxAttempts - validAttempts
	if remaining < 0 {
		return 0
	}
	return remaining
}
