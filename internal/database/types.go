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
	ColumnName       string
	DataType         string
	IsNullable       string
	Description      string
	IsPrimaryKey     bool   // True if this column is part of the primary key
	IsUnique         bool   // True if this column has a unique constraint (excluding PK)
	ForeignKeyRef    string // Reference in format "schema.table.column" if FK, empty otherwise
	IsIndexed        bool   // True if this column is part of any index
	IsIdentity       string // Identity generation: "" (none), "a" (ALWAYS), "d" (BY DEFAULT)
	DefaultValue     string // Default value expression if any, empty otherwise
	IsVectorColumn   bool   // True if this is a pgvector column
	VectorDimensions int    // Number of dimensions for vector columns (0 if not a vector)
}
