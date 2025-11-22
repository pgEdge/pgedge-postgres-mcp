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

// DiagnoseQueryIssue creates a prompt for diagnosing why queries aren't working
func DiagnoseQueryIssue() Prompt {
	return Prompt{
		Definition: mcp.Prompt{
			Name:        "diagnose-query-issue",
			Description: "Systematic diagnosis of why queries are failing or returning unexpected results. Helps identify connection, schema, or data issues.",
			Arguments: []mcp.PromptArgument{
				{
					Name:        "issue_description",
					Description: "Description of the problem (e.g., 'table not found', 'no results', 'wrong database')",
					Required:    false,
				},
			},
		},
		Handler: func(args map[string]string) mcp.PromptResult {
			issueDesc := args["issue_description"]
			if issueDesc == "" {
				issueDesc = "query not working as expected"
			}

			return mcp.PromptResult{
				Description: fmt.Sprintf("Diagnosing issue: %s", issueDesc),
				Messages: []mcp.PromptMessage{
					{
						Role: "user",
						Content: mcp.ContentItem{
							Type: "text",
							Text: fmt.Sprintf(`Diagnose why this query issue is occurring: %s

<diagnostic_workflow>
Step 1: Verify Database Connection
- Call: read_resource(uri="pg://system-info")
- Confirm:
  * Which database you're connected to
  * PostgreSQL version
  * Connection status
- Question: Is this the expected database?

Step 2: Check Schema Availability
- Call: get_schema_info(schema_name="[target_schema]")
- OR: get_schema_info() for all schemas
- Verify:
  * Does the target table/schema exist?
  * Are there any tables at all?
  * Is the schema name spelled correctly?

Step 3: Inspect Table Structure (if table exists)
- Review column names and types
- Check for required columns
- Verify data types match query expectations
- Look for constraints that might affect queries

Step 4: Sample Data Check
- Call: query_database(query="SELECT * FROM [table]", limit=3)
- Verify:
  * Table has data
  * Data format matches expectations
  * Column values are what you expect

Step 5: Common Issues Checklist
□ Wrong database connected
  → Solution: Use 'set default database to postgres://...' to switch

□ Table name misspelled or wrong schema
  → Solution: Check get_schema_info output for correct name

□ No vector columns (for similarity_search)
  → Solution: Use query_database instead, or check different table

□ Embedding generation disabled (for similarity_search)
  → Solution: Contact administrator to enable embedding service

□ Empty result set (query works but no matching data)
  → Solution: Adjust query criteria or check data availability

□ Rate limit exceeded
  → Solution: Wait 60 seconds, use more targeted queries with filters

□ Permission denied
  → Solution: Check user permissions with administrator

Step 6: Propose Solutions
Based on diagnosis, suggest:
- Correct database to connect to
- Proper table/column names to use
- Alternative query approaches
- Whether data exists for this query
</diagnostic_workflow>

<quick_checks>
Most common issues (check these first):
1. Wrong database: Check system-info, switch if needed
2. Table doesn't exist: Run get_schema_info to see what's available
3. No data in table: Sample with limit=1 to verify
4. Looking for semantic search in non-vector table: Check vector_tables_only
</quick_checks>

Begin diagnosis now. Be systematic and explain findings clearly.`, issueDesc),
						},
					},
				},
			}
		},
	}
}
