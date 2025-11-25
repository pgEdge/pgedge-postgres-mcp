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

func TestAnalytics_RecordCompaction(t *testing.T) {
	analytics := NewAnalytics()

	info1 := CompactionInfo{
		OriginalCount:    10,
		CompactedCount:   5,
		TokensSaved:      1000,
		CompressionRatio: 0.5,
	}

	analytics.RecordCompaction(info1, 100*time.Millisecond)

	metrics := analytics.GetMetrics()

	if metrics.TotalCompactions != 1 {
		t.Errorf("TotalCompactions = %v, want 1", metrics.TotalCompactions)
	}

	if metrics.TotalMessagesIn != 10 {
		t.Errorf("TotalMessagesIn = %v, want 10", metrics.TotalMessagesIn)
	}

	if metrics.TotalMessagesOut != 5 {
		t.Errorf("TotalMessagesOut = %v, want 5", metrics.TotalMessagesOut)
	}

	if metrics.TotalTokensSaved != 1000 {
		t.Errorf("TotalTokensSaved = %v, want 1000", metrics.TotalTokensSaved)
	}

	if metrics.AverageDuration != 100*time.Millisecond {
		t.Errorf("AverageDuration = %v, want 100ms", metrics.AverageDuration)
	}
}

func TestAnalytics_MultipleCompactions(t *testing.T) {
	analytics := NewAnalytics()

	info1 := CompactionInfo{
		OriginalCount:    10,
		CompactedCount:   5,
		TokensSaved:      1000,
		CompressionRatio: 0.5,
	}

	info2 := CompactionInfo{
		OriginalCount:    20,
		CompactedCount:   10,
		TokensSaved:      2000,
		CompressionRatio: 0.5,
	}

	analytics.RecordCompaction(info1, 100*time.Millisecond)
	analytics.RecordCompaction(info2, 200*time.Millisecond)

	metrics := analytics.GetMetrics()

	if metrics.TotalCompactions != 2 {
		t.Errorf("TotalCompactions = %v, want 2", metrics.TotalCompactions)
	}

	if metrics.TotalMessagesIn != 30 {
		t.Errorf("TotalMessagesIn = %v, want 30", metrics.TotalMessagesIn)
	}

	if metrics.TotalMessagesOut != 15 {
		t.Errorf("TotalMessagesOut = %v, want 15", metrics.TotalMessagesOut)
	}

	if metrics.TotalTokensSaved != 3000 {
		t.Errorf("TotalTokensSaved = %v, want 3000", metrics.TotalTokensSaved)
	}

	expectedAvgDuration := 150 * time.Millisecond
	if metrics.AverageDuration != expectedAvgDuration {
		t.Errorf("AverageDuration = %v, want %v", metrics.AverageDuration, expectedAvgDuration)
	}

	expectedAvgCompression := 0.5
	if metrics.AverageCompression != expectedAvgCompression {
		t.Errorf("AverageCompression = %v, want %v", metrics.AverageCompression, expectedAvgCompression)
	}
}

func TestAnalytics_GetEfficiencyReport(t *testing.T) {
	analytics := NewAnalytics()

	info1 := CompactionInfo{
		OriginalCount:    10,
		CompactedCount:   5,
		TokensSaved:      1000,
		CompressionRatio: 0.5,
	}

	info2 := CompactionInfo{
		OriginalCount:    20,
		CompactedCount:   15,
		TokensSaved:      500,
		CompressionRatio: 0.75,
	}

	analytics.RecordCompaction(info1, 100*time.Millisecond)
	analytics.RecordCompaction(info2, 200*time.Millisecond)

	report := analytics.GetEfficiencyReport()

	if report.TotalCompactions != 2 {
		t.Errorf("TotalCompactions = %v, want 2", report.TotalCompactions)
	}

	expectedAvgMessagesDropped := float64(10) / float64(2) // (5 + 5) / 2
	if report.AverageMessagesDropped != expectedAvgMessagesDropped {
		t.Errorf("AverageMessagesDropped = %v, want %v", report.AverageMessagesDropped, expectedAvgMessagesDropped)
	}

	expectedAvgTokensSaved := float64(1500) / float64(2) // (1000 + 500) / 2
	if report.AverageTokensSaved != expectedAvgTokensSaved {
		t.Errorf("AverageTokensSaved = %v, want %v", report.AverageTokensSaved, expectedAvgTokensSaved)
	}

	// AverageCompression = TotalMessagesOut / TotalMessagesIn = 20 / 30 = 0.666...
	expectedAvgCompression := 20.0 / 30.0
	if report.AverageCompression != expectedAvgCompression {
		t.Errorf("AverageCompression = %v, want %v", report.AverageCompression, expectedAvgCompression)
	}
}

func TestAnalytics_Reset(t *testing.T) {
	analytics := NewAnalytics()

	info := CompactionInfo{
		OriginalCount:    10,
		CompactedCount:   5,
		TokensSaved:      1000,
		CompressionRatio: 0.5,
	}

	analytics.RecordCompaction(info, 100*time.Millisecond)

	metrics := analytics.GetMetrics()
	if metrics.TotalCompactions != 1 {
		t.Fatal("Expected 1 compaction before reset")
	}

	analytics.Reset()

	metrics = analytics.GetMetrics()
	if metrics.TotalCompactions != 0 {
		t.Errorf("TotalCompactions after reset = %v, want 0", metrics.TotalCompactions)
	}
	if metrics.TotalMessagesIn != 0 {
		t.Errorf("TotalMessagesIn after reset = %v, want 0", metrics.TotalMessagesIn)
	}
	if metrics.TotalTokensSaved != 0 {
		t.Errorf("TotalTokensSaved after reset = %v, want 0", metrics.TotalTokensSaved)
	}
}

func TestAnalytics_LastCompactionTime(t *testing.T) {
	analytics := NewAnalytics()

	before := time.Now()

	info := CompactionInfo{
		OriginalCount:  10,
		CompactedCount: 5,
	}

	analytics.RecordCompaction(info, 100*time.Millisecond)

	after := time.Now()

	metrics := analytics.GetMetrics()

	if metrics.LastCompactionTime.Before(before) || metrics.LastCompactionTime.After(after) {
		t.Errorf("LastCompactionTime is outside expected range")
	}
}

func TestAnalytics_ThreadSafety(t *testing.T) {
	analytics := NewAnalytics()

	done := make(chan bool)

	// Simulate concurrent compactions
	for i := 0; i < 10; i++ {
		go func() {
			info := CompactionInfo{
				OriginalCount:  10,
				CompactedCount: 5,
				TokensSaved:    100,
			}
			analytics.RecordCompaction(info, 10*time.Millisecond)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	metrics := analytics.GetMetrics()

	if metrics.TotalCompactions != 10 {
		t.Errorf("TotalCompactions = %v, want 10", metrics.TotalCompactions)
	}

	if metrics.TotalTokensSaved != 1000 {
		t.Errorf("TotalTokensSaved = %v, want 1000", metrics.TotalTokensSaved)
	}
}

func TestAnalytics_EmptyMetrics(t *testing.T) {
	analytics := NewAnalytics()

	metrics := analytics.GetMetrics()

	if metrics.TotalCompactions != 0 {
		t.Errorf("Empty TotalCompactions = %v, want 0", metrics.TotalCompactions)
	}

	if metrics.AverageCompression != 0 {
		t.Errorf("Empty AverageCompression = %v, want 0", metrics.AverageCompression)
	}

	report := analytics.GetEfficiencyReport()

	if report.TotalCompactions != 0 {
		t.Errorf("Empty report TotalCompactions = %v, want 0", report.TotalCompactions)
	}
}
