/*-------------------------------------------------------------------------
 *
 * pgEdge Postgres MCP Server
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package tools

import (
	"strings"
	"testing"
	"time"

	"pgedge-postgres-mcp/internal/database"
)

func TestAnalyzeBloatTool(t *testing.T) {
	t.Run("tool definition", func(t *testing.T) {
		client := database.NewClient()
		tool := AnalyzeBloatTool(client)

		if tool.Definition.Name != "analyze_bloat" {
			t.Errorf("Expected name 'analyze_bloat', got %s", tool.Definition.Name)
		}

		if tool.Definition.Description == "" {
			t.Error("Description should not be empty")
		}

		if tool.Definition.InputSchema.Type != "object" {
			t.Errorf("Expected input schema type 'object', got %s", tool.Definition.InputSchema.Type)
		}

		// Check properties exist
		props := tool.Definition.InputSchema.Properties
		if _, ok := props["schema_name"]; !ok {
			t.Error("'schema_name' property should exist")
		}
		if _, ok := props["table_name"]; !ok {
			t.Error("'table_name' property should exist")
		}
		if _, ok := props["min_dead_tuple_percent"]; !ok {
			t.Error("'min_dead_tuple_percent' property should exist")
		}
		if _, ok := props["include_indexes"]; !ok {
			t.Error("'include_indexes' property should exist")
		}

		// No required fields
		if len(tool.Definition.InputSchema.Required) != 0 {
			t.Errorf("Expected 0 required fields, got %d", len(tool.Definition.InputSchema.Required))
		}
	})

	t.Run("database not ready", func(t *testing.T) {
		client := database.NewClient()
		// Don't add any connections - database is not ready

		tool := AnalyzeBloatTool(client)
		response, err := tool.Handler(map[string]interface{}{})

		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}
		if !response.IsError {
			t.Error("Expected IsError=true when database not ready")
		}
		content := response.Content[0].Text
		if !strings.Contains(content, "Database is still initializing") {
			t.Errorf("Expected database not ready message, got: %s", content)
		}
	})

	t.Run("table_name without schema_name", func(t *testing.T) {
		client := database.NewTestClient("postgres://localhost/test", make(map[string]database.TableInfo))
		tool := AnalyzeBloatTool(client)

		response, err := tool.Handler(map[string]interface{}{
			"table_name": "users",
		})

		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}
		if !response.IsError {
			t.Error("Expected IsError=true when table_name provided without schema_name")
		}
		content := response.Content[0].Text
		if !strings.Contains(content, "table_name requires schema_name") {
			t.Errorf("Expected validation error, got: %s", content)
		}
	})

	t.Run("invalid min_dead_tuple_percent - negative", func(t *testing.T) {
		client := database.NewTestClient("postgres://localhost/test", make(map[string]database.TableInfo))
		tool := AnalyzeBloatTool(client)

		response, err := tool.Handler(map[string]interface{}{
			"min_dead_tuple_percent": -5.0,
		})

		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}
		if !response.IsError {
			t.Error("Expected IsError=true for negative min_dead_tuple_percent")
		}
		content := response.Content[0].Text
		if !strings.Contains(content, "must be between 0 and 100") {
			t.Errorf("Expected range validation error, got: %s", content)
		}
	})

	t.Run("invalid min_dead_tuple_percent - over 100", func(t *testing.T) {
		client := database.NewTestClient("postgres://localhost/test", make(map[string]database.TableInfo))
		tool := AnalyzeBloatTool(client)

		response, err := tool.Handler(map[string]interface{}{
			"min_dead_tuple_percent": 150.0,
		})

		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}
		if !response.IsError {
			t.Error("Expected IsError=true for min_dead_tuple_percent > 100")
		}
		content := response.Content[0].Text
		if !strings.Contains(content, "must be between 0 and 100") {
			t.Errorf("Expected range error, got: %s", content)
		}
	})

	t.Run("valid min_dead_tuple_percent - boundary values", func(t *testing.T) {
		client := database.NewTestClient("postgres://localhost/test", make(map[string]database.TableInfo))
		tool := AnalyzeBloatTool(client)

		testCases := []struct {
			name  string
			value float64
		}{
			{"zero", 0.0},
			{"boundary", 0.1},
			{"normal", 5.0},
			{"high", 50.0},
			{"max", 100.0},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				response, err := tool.Handler(map[string]interface{}{
					"min_dead_tuple_percent": tc.value,
				})

				if err != nil {
					t.Errorf("Handler returned error: %v", err)
				}

				// Should not fail validation (will fail on query which is expected without real DB)
				content := response.Content[0].Text
				if strings.Contains(content, "must be") || strings.Contains(content, "positive number") {
					t.Errorf("Should not fail validation for value %.1f, got: %s", tc.value, content)
				}
			})
		}
	})

	t.Run("optional parameters default correctly", func(t *testing.T) {
		client := database.NewTestClient("postgres://localhost/test", make(map[string]database.TableInfo))
		tool := AnalyzeBloatTool(client)

		// Call with no parameters - should use defaults
		response, err := tool.Handler(map[string]interface{}{})

		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}

		// Should not error on parameter validation (will error on query)
		content := response.Content[0].Text
		if strings.Contains(content, "requires schema_name") ||
			strings.Contains(content, "must be") {
			t.Errorf("Should not fail with default parameters, got: %s", content)
		}
	})

	t.Run("invalid parameter types", func(t *testing.T) {
		client := database.NewTestClient("postgres://localhost/test", make(map[string]database.TableInfo))
		tool := AnalyzeBloatTool(client)

		testCases := []struct {
			name   string
			params map[string]interface{}
		}{
			{
				"schema_name not string",
				map[string]interface{}{"schema_name": 123},
			},
			{
				"table_name not string",
				map[string]interface{}{"table_name": 456},
			},
			{
				"min_dead_tuple_percent not number",
				map[string]interface{}{"min_dead_tuple_percent": "not a number"},
			},
			{
				"include_indexes not boolean",
				map[string]interface{}{"include_indexes": "yes"},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Should handle type mismatches gracefully with defaults
				_, err := tool.Handler(tc.params)
				if err != nil {
					t.Errorf("Handler should not return error for type mismatch: %v", err)
				}
			})
		}
	})
}

func TestGenerateRecommendations(t *testing.T) {
	now := time.Now()
	lastWeek := now.Add(-7 * 24 * time.Hour)
	lastMonth := now.Add(-30 * 24 * time.Hour)

	t.Run("high dead tuple percentage", func(t *testing.T) {
		bloat := TableBloat{
			DeadTuplePercent: 25.0,
			ModsSinceAnalyze: 100,
		}

		recs := generateRecommendations(bloat)

		if len(recs) == 0 {
			t.Fatal("Expected recommendations")
		}

		found := false
		for _, rec := range recs {
			if strings.Contains(rec, "VACUUM") && strings.Contains(rec, "URGENT") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected urgent VACUUM recommendation for 25%% bloat, got: %v", recs)
		}
	})

	t.Run("very high dead tuple percentage suggests VACUUM FULL", func(t *testing.T) {
		bloat := TableBloat{
			DeadTuplePercent: 60.0,
			ModsSinceAnalyze: 100,
		}

		recs := generateRecommendations(bloat)

		found := false
		for _, rec := range recs {
			if strings.Contains(rec, "VACUUM FULL") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected VACUUM FULL recommendation for 60%% bloat, got: %v", recs)
		}
	})

	t.Run("moderate dead tuple percentage", func(t *testing.T) {
		bloat := TableBloat{
			DeadTuplePercent: 12.0,
			ModsSinceAnalyze: 100,
		}

		recs := generateRecommendations(bloat)

		found := false
		for _, rec := range recs {
			if strings.Contains(rec, "VACUUM") && strings.Contains(rec, "soon") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected moderate VACUUM recommendation for 12%% bloat, got: %v", recs)
		}
	})

	t.Run("low bloat with adequate maintenance", func(t *testing.T) {
		recent := now.Add(-1 * time.Hour)
		bloat := TableBloat{
			DeadTuplePercent: 3.0,
			ModsSinceAnalyze: 50,
			LastVacuum:       &recent,
			LastAnalyze:      &recent,
		}

		recs := generateRecommendations(bloat)

		found := false
		for _, rec := range recs {
			if strings.Contains(rec, "adequate") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected adequate maintenance message for low bloat, got: %v", recs)
		}
	})

	t.Run("high modifications since analyze", func(t *testing.T) {
		bloat := TableBloat{
			DeadTuplePercent: 5.0,
			ModsSinceAnalyze: 5000,
		}

		recs := generateRecommendations(bloat)

		found := false
		for _, rec := range recs {
			if strings.Contains(rec, "ANALYZE") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected ANALYZE recommendation for high modifications, got: %v", recs)
		}
	})

	t.Run("never vacuumed", func(t *testing.T) {
		bloat := TableBloat{
			DeadTuplePercent: 10.0,
			LastVacuum:       nil,
			LastAutovacuum:   nil,
		}

		recs := generateRecommendations(bloat)

		found := false
		for _, rec := range recs {
			if strings.Contains(rec, "never been vacuumed") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected never vacuumed warning, got: %v", recs)
		}
	})

	t.Run("never analyzed", func(t *testing.T) {
		bloat := TableBloat{
			DeadTuplePercent: 5.0,
			LastAnalyze:      nil,
			LastAutoanalyze:  nil,
		}

		recs := generateRecommendations(bloat)

		found := false
		for _, rec := range recs {
			if strings.Contains(rec, "never been analyzed") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected never analyzed warning, got: %v", recs)
		}
	})

	t.Run("old vacuum with high activity", func(t *testing.T) {
		bloat := TableBloat{
			DeadTuplePercent: 8.0,
			LastVacuum:       &lastMonth,
			Updates:          10000,
			Deletes:          5000,
		}

		recs := generateRecommendations(bloat)

		found := false
		for _, rec := range recs {
			if strings.Contains(rec, "last vacuumed") && strings.Contains(rec, "write activity") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected old vacuum warning with write activity, got: %v", recs)
		}
	})

	t.Run("old analyze with modifications", func(t *testing.T) {
		bloat := TableBloat{
			DeadTuplePercent: 5.0,
			LastAnalyze:      &lastWeek,
			ModsSinceAnalyze: 2000,
		}

		recs := generateRecommendations(bloat)

		found := false
		for _, rec := range recs {
			if strings.Contains(rec, "last analyzed") && strings.Contains(rec, "modifications") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected old analyze warning, got: %v", recs)
		}
	})

	t.Run("multiple recommendations", func(t *testing.T) {
		bloat := TableBloat{
			DeadTuplePercent: 25.0,
			ModsSinceAnalyze: 5000,
			LastVacuum:       nil,
			LastAnalyze:      nil,
		}

		recs := generateRecommendations(bloat)

		// Should have multiple recommendations
		if len(recs) < 3 {
			t.Errorf("Expected at least 3 recommendations for problematic table, got %d: %v", len(recs), recs)
		}

		// Check for key recommendations
		hasVacuum := false
		hasAnalyze := false
		hasNeverVacuumed := false

		for _, rec := range recs {
			if strings.Contains(rec, "VACUUM") {
				hasVacuum = true
			}
			if strings.Contains(rec, "ANALYZE") {
				hasAnalyze = true
			}
			if strings.Contains(rec, "never") {
				hasNeverVacuumed = true
			}
		}

		if !hasVacuum {
			t.Error("Expected VACUUM recommendation")
		}
		if !hasAnalyze {
			t.Error("Expected ANALYZE recommendation")
		}
		if !hasNeverVacuumed {
			t.Error("Expected never vacuumed warning")
		}
	})
}

func TestTableBloatStruct(t *testing.T) {
	t.Run("struct fields", func(t *testing.T) {
		now := time.Now()
		bloat := TableBloat{
			SchemaName:       "public",
			TableName:        "users",
			LiveTuples:       1000,
			DeadTuples:       100,
			DeadTuplePercent: 9.09,
			TotalSize:        "1 GB",
			TotalSizeBytes:   1073741824,
			LastVacuum:       &now,
			ModsSinceAnalyze: 500,
			Recommendations:  []string{"Test recommendation"},
		}

		if bloat.SchemaName != "public" {
			t.Errorf("SchemaName = %s, want public", bloat.SchemaName)
		}
		if bloat.TableName != "users" {
			t.Errorf("TableName = %s, want users", bloat.TableName)
		}
		if bloat.LiveTuples != 1000 {
			t.Errorf("LiveTuples = %d, want 1000", bloat.LiveTuples)
		}
		if bloat.DeadTuples != 100 {
			t.Errorf("DeadTuples = %d, want 100", bloat.DeadTuples)
		}
		if bloat.DeadTuplePercent != 9.09 {
			t.Errorf("DeadTuplePercent = %.2f, want 9.09", bloat.DeadTuplePercent)
		}
		if len(bloat.Recommendations) != 1 {
			t.Errorf("Recommendations length = %d, want 1", len(bloat.Recommendations))
		}
	})
}
