package compactor

import (
	"sync"
	"time"
)

// Analytics tracks compaction metrics and statistics
type Analytics struct {
	metrics CompactionMetrics
	mu      sync.RWMutex
}

// NewAnalytics creates a new analytics tracker
func NewAnalytics() *Analytics {
	return &Analytics{
		metrics: CompactionMetrics{},
	}
}

// RecordCompaction records a compaction operation
func (a *Analytics) RecordCompaction(info CompactionInfo, duration time.Duration) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.metrics.TotalCompactions++
	a.metrics.TotalMessagesIn += int64(info.OriginalCount)
	a.metrics.TotalMessagesOut += int64(info.CompactedCount)
	a.metrics.TotalTokensSaved += int64(info.TokensSaved)
	a.metrics.TotalDuration += duration
	a.metrics.LastCompactionTime = time.Now()

	// Update averages
	if a.metrics.TotalCompactions > 0 {
		a.metrics.AverageCompression = float64(a.metrics.TotalMessagesOut) / float64(a.metrics.TotalMessagesIn)
		a.metrics.AverageDuration = a.metrics.TotalDuration / time.Duration(a.metrics.TotalCompactions)
	}
}

// GetMetrics returns a copy of current metrics
func (a *Analytics) GetMetrics() CompactionMetrics {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.metrics
}

// Reset clears all metrics
func (a *Analytics) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.metrics = CompactionMetrics{}
}

// GetSummary returns a human-readable summary of metrics
func (a *Analytics) GetSummary() map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.metrics.TotalCompactions == 0 {
		return map[string]interface{}{
			"status": "no compactions recorded",
		}
	}

	return map[string]interface{}{
		"total_compactions":   a.metrics.TotalCompactions,
		"total_messages_in":   a.metrics.TotalMessagesIn,
		"total_messages_out":  a.metrics.TotalMessagesOut,
		"total_tokens_saved":  a.metrics.TotalTokensSaved,
		"average_compression": a.metrics.AverageCompression,
		"average_duration_ms": a.metrics.AverageDuration.Milliseconds(),
		"last_compaction":     a.metrics.LastCompactionTime.Format(time.RFC3339),
	}
}

// GetEfficiencyReport generates an efficiency report
func (a *Analytics) GetEfficiencyReport() EfficiencyReport {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.metrics.TotalCompactions == 0 {
		return EfficiencyReport{
			HasData: false,
		}
	}

	avgMessagesDropped := float64(a.metrics.TotalMessagesIn-a.metrics.TotalMessagesOut) / float64(a.metrics.TotalCompactions)
	avgTokensSaved := float64(a.metrics.TotalTokensSaved) / float64(a.metrics.TotalCompactions)

	return EfficiencyReport{
		HasData:                true,
		TotalCompactions:       a.metrics.TotalCompactions,
		AverageCompression:     a.metrics.AverageCompression,
		AverageMessagesDropped: avgMessagesDropped,
		AverageTokensSaved:     avgTokensSaved,
		AverageDuration:        a.metrics.AverageDuration,
		TotalTokensSaved:       a.metrics.TotalTokensSaved,
	}
}

// EfficiencyReport provides detailed efficiency metrics
type EfficiencyReport struct {
	HasData                bool
	TotalCompactions       int64
	AverageCompression     float64
	AverageMessagesDropped float64
	AverageTokensSaved     float64
	AverageDuration        time.Duration
	TotalTokensSaved       int64
}
