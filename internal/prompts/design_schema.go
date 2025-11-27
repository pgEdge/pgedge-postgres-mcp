/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package prompts

import (
	"fmt"

	"pgedge-postgres-mcp/internal/mcp"
)

// DesignSchema creates a prompt for designing PostgreSQL database schemas
func DesignSchema() Prompt {
	return Prompt{
		Definition: mcp.Prompt{
			Name:        "design-schema",
			Description: "Design a PostgreSQL database schema based on requirements. Uses best practices, appropriate normalization, and PostgreSQL extensions where beneficial.",
			Arguments: []mcp.PromptArgument{
				{
					Name:        "requirements",
					Description: "Description of the application requirements and data needs",
					Required:    true,
				},
				{
					Name:        "use_case",
					Description: "Primary use case: oltp, olap, hybrid, or general (default: general)",
					Required:    false,
				},
				{
					Name:        "full_featured",
					Description: "If true, design a comprehensive production-ready schema. If false (default), design minimal schema meeting only stated requirements.",
					Required:    false,
				},
			},
		},
		Handler: func(args map[string]string) mcp.PromptResult {
			requirements := args["requirements"]
			if requirements == "" {
				requirements = "[describe your data requirements]"
			}

			useCase := args["use_case"]
			if useCase == "" {
				useCase = "general"
			}

			fullFeatured := args["full_featured"] == "true"

			// Scope guidance based on full_featured parameter
			var scopeGuidance string
			if fullFeatured {
				scopeGuidance = `<scope_guidance>
COMPREHENSIVE DESIGN MODE: Design a production-ready schema with:
- All entities needed for a complete, real-world application
- Supporting tables (audit logs, user preferences, etc.)
- Future-proofing considerations
- Comprehensive constraints and indexes
</scope_guidance>`
			} else {
				scopeGuidance = `<scope_guidance>
MINIMAL DESIGN MODE (default): Design the ABSOLUTE MINIMUM schema that meets requirements:

TABLES:
- ONLY create tables explicitly mentioned in requirements
- Do NOT add supporting tables (user accounts, audit logs, favorites, settings, etc.)
- Junction/bridge tables are OK only when many-to-many relationships are explicitly stated

COLUMNS - BE EXTREMELY STRICT:
- Do NOT add created_at, updated_at, or any timestamp fields unless explicitly requested
- Do NOT add metadata columns (status, flags, counts, etc.) unless explicitly requested
- Do NOT add descriptive columns (notes, description) unless explicitly requested
- Do NOT add quantity/amount columns unless explicitly requested
- Do NOT add measurement columns (prep_time, servings, duration, etc.) unless explicitly requested
- If a piece of information would logically be in a description/notes field, do NOT create a separate column for it
- Primary key + foreign keys + explicitly requested attributes ONLY

RELATIONSHIPS:
- Use ONE relationship mechanism per association (foreign keys OR arrays, never both)
- Prefer simple foreign keys over arrays
- Do NOT add triggers to maintain denormalized data
- Do NOT duplicate relationship data in multiple forms

EXTENSIONS:
- Prefer simpler solutions: pg_trgm for text search over pgvector unless semantic search is required
- Avoid over-engineering: if LIKE or full-text search suffices, don't use vector embeddings

VALIDATION:
- Before finalizing, review EVERY column and ask: "Was this explicitly requested?"
- If the answer is no, REMOVE IT
- When in doubt, leave it out - the user can request additions later
</scope_guidance>`
			}

			return mcp.PromptResult{
				Description: fmt.Sprintf("Database schema design for: %s (use case: %s)", requirements, useCase),
				Messages: []mcp.PromptMessage{
					{
						Role: "user",
						Content: mcp.ContentItem{
							Type: "text",
							Text: fmt.Sprintf(`Design a PostgreSQL database schema for the following requirements:

<requirements>
%s
</requirements>

<use_case>%s</use_case>

%s

<schema_design_workflow>
Step 1: Research Best Practices (if knowledgebase available)
- Call: search_knowledgebase(query="PostgreSQL schema design best practices")
- Call: search_knowledgebase(query="PostgreSQL data types for [relevant domain]")
- Look for guidance on:
  * Appropriate data types for the domain
  * Indexing strategies for the use case
  * Relevant PostgreSQL extensions
- If knowledgebase is unavailable, proceed with built-in knowledge

Step 2: Analyze Requirements
- Identify core entities (nouns in the requirements)
- Identify relationships between entities
- Identify attributes for each entity
- Note any special requirements:
  * High-volume reads or writes
  * Full-text search needs
  * Geographic/spatial data
  * Time-series data
  * JSON/document storage needs
  * Vector/semantic search needs

Step 3: Choose Appropriate Data Types
- For primary keys:
  * GENERATED ALWAYS AS IDENTITY for single-database auto-increment (preferred)
  * UUID only when: (a) application provides IDs, or (b) distributed system needs
  * Avoid SERIAL (deprecated in favor of IDENTITY)
- Use specific types over generic ones:
  * TIMESTAMPTZ for timestamps (not TIMESTAMP)
  * NUMERIC for money/precision (not FLOAT)
  * TEXT for variable strings (not VARCHAR without limit)
  * JSONB for semi-structured data (not JSON)
  * Arrays for small, fixed collections
- Consider PostgreSQL-specific types:
  * INET/CIDR for IP addresses
  * DATERANGE/TSRANGE for ranges
  * POINT/GEOMETRY for spatial data (PostGIS)
  * TSVECTOR for full-text search
  * VECTOR for embeddings (pgvector)

Step 4: Apply Normalization Appropriately
<normalization_guidance use_case="%s">
For OLTP (transactional):
- Normalize to 3NF to reduce redundancy
- Use foreign keys for referential integrity
- Favor many small tables over few large ones
- Prioritize write performance and data consistency

For OLAP (analytical):
- Consider denormalization for query performance
- Use materialized views for aggregations
- Star/snowflake schemas may be appropriate
- Prioritize read performance over write efficiency

For Hybrid/General:
- Start with normalized design (3NF)
- Identify hot paths that need optimization
- Denormalize selectively with materialized views
- Balance read/write performance
</normalization_guidance>

Step 5: Design Indexes
- Primary keys (automatic B-tree index)
- Foreign keys (add explicit indexes)
- Columns used in WHERE clauses
- Columns used in ORDER BY
- Consider partial indexes for filtered queries
- Consider expression indexes for computed values
- GIN indexes for JSONB, arrays, full-text
- GiST indexes for geometric/range types
- HNSW/IVFFlat for vector similarity (pgvector)

Step 6: Consider PostgreSQL Extensions
<extension_selection_guidance>
CRITICAL: Choose the SIMPLEST extension that meets the requirement:
- For basic text search (finding substrings, fuzzy matching): use pg_trgm
- For full-text search (natural language queries): use built-in tsvector/tsquery
- For semantic/AI search (meaning-based similarity): use vector (pgvector)
- For geographic data: use postgis
- For case-insensitive text: use citext

Do NOT use advanced extensions when simpler ones suffice:
- Don't use pgvector for simple text matching - use pg_trgm or LIKE
- Don't use PostGIS for simple coordinates - use POINT type
- Don't add extensions speculatively - only when clearly needed
</extension_selection_guidance>

Research and recommend extensions where appropriate:
- vector: Vector similarity search (only for semantic/AI search)
- postgis: Geographic/spatial data
- pg_trgm: Fuzzy text matching (prefer this for simple text search)
- btree_gin/btree_gist: Multi-column indexes
- pg_stat_statements: Query performance analysis
- timescaledb: Time-series data (if applicable)
- citext: Case-insensitive text

CRITICAL: Before writing any CREATE EXTENSION statement, you MUST verify the
exact extension name. Extension names often differ from project/common names:
- pgvector project -> CREATE EXTENSION vector;
- PostGIS project -> CREATE EXTENSION postgis;
- Many others have similar discrepancies

For EVERY extension you plan to use:
1. Search the knowledgebase: search_knowledgebase(query="PostgreSQL [extension] CREATE EXTENSION")
2. If knowledgebase unavailable, state that the extension name should be verified
3. Never assume the extension name matches the project name

Step 7: Add Constraints and Defaults
- NOT NULL where data is required
- CHECK constraints for valid ranges/values
- UNIQUE constraints for business keys
- DEFAULT values (CURRENT_TIMESTAMP, gen_random_uuid())
- EXCLUDE constraints for non-overlapping ranges

Step 8: Plan for Scale and Maintenance
- Consider table partitioning for large tables
- Plan for archival/retention policies
- Add created_at/updated_at timestamps
- Consider soft deletes vs hard deletes
- Plan index maintenance strategy
</schema_design_workflow>

<output_format>
Provide the schema design as:

1. **Entity-Relationship Summary**
   - List entities and their relationships
   - Explain key design decisions

2. **SQL Schema Definition**
   - Complete CREATE TABLE statements
   - CREATE INDEX statements
   - Any required CREATE EXTENSION statements
   - Comments explaining design choices

3. **Recommended Extensions**
   - List any PostgreSQL extensions that would benefit this schema
   - Explain why each is recommended

4. **Sample Queries**
   - Show 2-3 common queries the schema supports efficiently
   - Explain index usage

5. **Scaling Considerations**
   - Identify potential bottlenecks
   - Suggest partitioning strategy if applicable
   - Note any future optimization opportunities
</output_format>

<anti_patterns_to_avoid>
- Don't use SERIAL; prefer GENERATED ALWAYS AS IDENTITY for auto-incrementing keys
- Don't generate UUIDs in the database unless required; use IDENTITY for single-database
  systems. UUID is appropriate when the application provides the ID or for distributed systems
- Don't use CHAR(n); use TEXT or VARCHAR
- Don't store money as FLOAT; use NUMERIC or integer cents
- Don't use TIMESTAMP; use TIMESTAMPTZ
- Don't create indexes on every column
- Don't over-normalize if queries always join the same tables
- Don't under-normalize if data consistency is critical
- Don't use EAV (Entity-Attribute-Value) pattern unless truly necessary
- Don't store file contents in database; store paths/URLs
- Don't assume extension names; always verify via knowledgebase before using CREATE EXTENSION
- Don't over-engineer: add only what is explicitly required
- Don't add tables or columns "just in case" - keep schema minimal
- Don't use advanced extensions (pgvector) when simpler ones (pg_trgm, LIKE) suffice
- Don't add timestamp fields (created_at, updated_at) in minimal mode unless requested
- Don't duplicate relationships (e.g., both array columns AND foreign keys for the same data)
- Don't use triggers to maintain denormalized/redundant data
- Don't add columns for data that belongs in a description/notes field
</anti_patterns_to_avoid>

Begin the schema design now. Start by checking the knowledgebase for relevant best practices, then proceed through the workflow systematically. Remember to follow the scope guidance above.`, requirements, useCase, scopeGuidance, useCase),
						},
					},
				},
			}
		},
	}
}
