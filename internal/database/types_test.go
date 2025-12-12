/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package database

import (
	"encoding/json"
	"testing"
)

func TestTableInfo_Struct(t *testing.T) {
	info := TableInfo{
		SchemaName:  "public",
		TableName:   "users",
		TableType:   "TABLE",
		Description: "User accounts table",
		Columns: []ColumnInfo{
			{
				ColumnName:       "id",
				DataType:         "integer",
				IsNullable:       "NO",
				Description:      "Primary key",
				IsVectorColumn:   false,
				VectorDimensions: 0,
			},
			{
				ColumnName:       "name",
				DataType:         "varchar(255)",
				IsNullable:       "YES",
				Description:      "User name",
				IsVectorColumn:   false,
				VectorDimensions: 0,
			},
		},
	}

	if info.SchemaName != "public" {
		t.Errorf("expected schema 'public', got %q", info.SchemaName)
	}
	if info.TableName != "users" {
		t.Errorf("expected table 'users', got %q", info.TableName)
	}
	if info.TableType != "TABLE" {
		t.Errorf("expected type 'TABLE', got %q", info.TableType)
	}
	if len(info.Columns) != 2 {
		t.Errorf("expected 2 columns, got %d", len(info.Columns))
	}
}

func TestTableInfo_JSON(t *testing.T) {
	info := TableInfo{
		SchemaName:  "schema1",
		TableName:   "table1",
		TableType:   "VIEW",
		Description: "A view",
		Columns: []ColumnInfo{
			{ColumnName: "col1", DataType: "text"},
		},
	}

	// Marshal to JSON
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Unmarshal back
	var decoded TableInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.SchemaName != info.SchemaName {
		t.Errorf("expected schema %q, got %q", info.SchemaName, decoded.SchemaName)
	}
	if decoded.TableType != "VIEW" {
		t.Errorf("expected type 'VIEW', got %q", decoded.TableType)
	}
}

func TestColumnInfo_Struct(t *testing.T) {
	col := ColumnInfo{
		ColumnName:       "embedding",
		DataType:         "vector(1536)",
		IsNullable:       "YES",
		Description:      "OpenAI embedding vector",
		IsVectorColumn:   true,
		VectorDimensions: 1536,
	}

	if col.ColumnName != "embedding" {
		t.Errorf("expected column name 'embedding', got %q", col.ColumnName)
	}
	if !col.IsVectorColumn {
		t.Error("expected IsVectorColumn to be true")
	}
	if col.VectorDimensions != 1536 {
		t.Errorf("expected 1536 dimensions, got %d", col.VectorDimensions)
	}
}

func TestColumnInfo_JSON(t *testing.T) {
	col := ColumnInfo{
		ColumnName:       "test_col",
		DataType:         "integer",
		IsNullable:       "NO",
		Description:      "Test column",
		IsVectorColumn:   false,
		VectorDimensions: 0,
	}

	data, err := json.Marshal(col)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded ColumnInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.ColumnName != col.ColumnName {
		t.Errorf("expected column name %q, got %q", col.ColumnName, decoded.ColumnName)
	}
	if decoded.DataType != col.DataType {
		t.Errorf("expected data type %q, got %q", col.DataType, decoded.DataType)
	}
}

func TestTableInfo_MaterializedView(t *testing.T) {
	info := TableInfo{
		SchemaName:  "analytics",
		TableName:   "daily_stats",
		TableType:   "MATERIALIZED VIEW",
		Description: "Daily aggregated statistics",
		Columns:     []ColumnInfo{},
	}

	if info.TableType != "MATERIALIZED VIEW" {
		t.Errorf("expected type 'MATERIALIZED VIEW', got %q", info.TableType)
	}
}

func TestTableInfo_EmptyColumns(t *testing.T) {
	info := TableInfo{
		SchemaName: "public",
		TableName:  "empty_table",
		TableType:  "TABLE",
		Columns:    []ColumnInfo{},
	}

	if len(info.Columns) != 0 {
		t.Errorf("expected 0 columns, got %d", len(info.Columns))
	}

	// Verify it can be marshaled/unmarshaled with empty columns
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded TableInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
}

func TestColumnInfo_VectorTypes(t *testing.T) {
	tests := []struct {
		name       string
		dataType   string
		isVector   bool
		dimensions int
	}{
		{"regular integer", "integer", false, 0},
		{"regular text", "text", false, 0},
		{"vector 1536", "vector(1536)", true, 1536},
		{"vector 384", "vector(384)", true, 384},
		{"vector 768", "vector(768)", true, 768},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			col := ColumnInfo{
				ColumnName:       "test",
				DataType:         tt.dataType,
				IsVectorColumn:   tt.isVector,
				VectorDimensions: tt.dimensions,
			}

			if col.IsVectorColumn != tt.isVector {
				t.Errorf("expected IsVectorColumn=%v, got %v", tt.isVector, col.IsVectorColumn)
			}
			if col.VectorDimensions != tt.dimensions {
				t.Errorf("expected dimensions=%d, got %d", tt.dimensions, col.VectorDimensions)
			}
		})
	}
}

func TestColumnInfo_NullableValues(t *testing.T) {
	tests := []struct {
		name     string
		nullable string
	}{
		{"YES", "YES"},
		{"NO", "NO"},
		{"empty", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			col := ColumnInfo{
				ColumnName: "test",
				IsNullable: tt.nullable,
			}

			if col.IsNullable != tt.nullable {
				t.Errorf("expected IsNullable=%q, got %q", tt.nullable, col.IsNullable)
			}
		})
	}
}
