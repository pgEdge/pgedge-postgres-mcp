/*-------------------------------------------------------------------------
 *
 * pgEdge Postgres MCP Server
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package database

// TableInfo contains information about a database table or view
type TableInfo struct {
	SchemaName  string
	TableName   string
	TableType   string // 'TABLE', 'VIEW', or 'MATERIALIZED VIEW'
	Description string
	Columns     []ColumnInfo
}

// ColumnInfo contains information about a database column
type ColumnInfo struct {
	ColumnName  string
	DataType    string
	IsNullable  string
	Description string
}
