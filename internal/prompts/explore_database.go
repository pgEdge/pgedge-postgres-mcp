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

<fresh_exploration_required>
CRITICAL: You MUST make fresh tool calls for this exploration. Do NOT rely on or
reference any previous database exploration results from this conversation.

Why this matters:
- The user may have switched to a different database connection
- Database schemas and tables can change at any time
- Data content changes constantly
- Server statistics and system info are time-sensitive

Even if you explored a database earlier in this conversation, treat this as a
completely new exploration. The current database state is unknown until you
query it fresh.
</fresh_exploration_required>

<critical_rate_limit_warning>
IMPORTANT: Each tool call uses ~8,000-10,000 tokens. Rate limits are typically
30,000 tokens per minute. To complete exploration without hitting rate limits:
- MINIMIZE tool calls - extract maximum information from each call
- NEVER call the same tool twice with the same parameters
- Combine information gathering into as few calls as possible
- Skip sample queries unless specifically requested by user
</critical_rate_limit_warning>

<efficient_exploration_workflow>
Step 1: Get Database Overview (ONE call only)
- Call get_schema_info(schema_name="public") for focused view
- This single call provides: all tables, columns, types, descriptions,
  vector columns, and relationships
- Extract ALL insights from this one response before making any other calls

Step 2: Analyze Results (NO tool calls needed)
From the get_schema_info response, identify:
- Table purposes from names and descriptions
- Vector columns (pgvector) for semantic search capability
- JSONB columns for flexible storage
- Relationships from foreign keys
- Data patterns (transaction tables, lookup tables, junction tables)

Step 3: Summarize Findings (NO tool calls needed)
Provide summary including:
- Database purpose and domain
- Key tables and their relationships
- Available search capabilities
- Suggested use cases

Step 4: Sample Data (ONLY if user requests)
- Only query sample data if user explicitly asks to see examples
- Use limit=3 maximum
- Combine multiple sample queries into ONE call if possible
</efficient_exploration_workflow>

<tool_call_budget>
Target: Complete exploration in 2-3 tool calls maximum
- Call 1: get_schema_info(schema_name="public")
- Call 2: (optional) read_resource for system-info if needed
- Call 3: (optional) sample query ONLY if user requests

Do NOT make calls for:
- Multiple get_schema_info with different parameters
- Sample queries from multiple tables
- Checking extensions (infer from column types instead)
</tool_call_budget>

<early_exit_conditions>
Stop exploration if:
- No tables found: inform user database may be empty
- Permission denied: inform user of limited access
</early_exit_conditions>

Begin the exploration now. Be efficient with tool calls and provide a comprehensive summary from minimal calls.`,
						},
					},
				},
			}
		},
	}
}
