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
	"pgedge-postgres-mcp/internal/mcp"
)

// ExploreDatabase creates a prompt for systematic database exploration
func ExploreDatabase() Prompt {
	return Prompt{
		Definition: mcp.Prompt{
			Name:        "explore-database",
			Description: "Multi-step workflow to systematically explore an unfamiliar database and understand its structure, capabilities, and available data.",
			Arguments:   []mcp.PromptArgument{},
		},
		Handler: func(args map[string]string) mcp.PromptResult {
			return mcp.PromptResult{
				Description: "Systematic database exploration workflow",
				Messages: []mcp.PromptMessage{
					{
						Role: "user",
						Content: mcp.ContentItem{
							Type: "text",
							Text: `I need to explore this PostgreSQL database systematically to understand what data is available and how it's organized.

<exploration_workflow>
Step 1: Get System Information
- Call read_resource(uri="pg://system-info") OR use native resources/read
- Understand: database name, PostgreSQL version, connection details
- This helps identify which database you're connected to

Step 2: Quick Table Overview
- Call get_schema_info() with NO parameters for comprehensive view
- OR use schema_name="public" if you only want the main schema
- Note: This may return significant data - use filtering if database is large

Step 3: Analyze Schema Structure
- Identify tables of interest based on:
  * Table names that suggest their purpose
  * Table descriptions from pg_description
  * Number and types of columns
- Look for patterns: transaction tables, lookup tables, junction tables

Step 4: Identify Special Capabilities
- Check for vector columns (pgvector): indicates semantic search capability
- Check for JSONB columns: indicates flexible/document storage
- Check for foreign keys: understand relationships between tables

Step 5: Sample Data (if needed)
- For key tables, query small samples: query_database(query="SELECT * FROM table_name", limit=5)
- This helps understand data patterns and content types

Step 6: Summarize Findings
- Database purpose and domain (e.g., e-commerce, CRM, documentation)
- Key tables and their relationships
- Available search capabilities (SQL, semantic, full-text)
- Suggested queries or analyzes for this data
</exploration_workflow>

<rate_limit_management>
- Use get_schema_info(schema_name="public") to reduce token usage
- Use limit=5 for sample queries
- Cache results - don't re-query the same information
- If exploring large database, filter by schema first
</rate_limit_management>

<early_exit_conditions>
Stop exploration if:
- No tables found: might be wrong database or empty database
- Permission denied: limited access prevents exploration
- Specific data sought but not found: suggest checking other databases
</early_exit_conditions>

Begin the exploration now. Be systematic and summarize findings clearly.`,
						},
					},
				},
			}
		},
	}
}
