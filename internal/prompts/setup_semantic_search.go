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

// SetupSemanticSearch creates a prompt for semantic search preparation and execution
func SetupSemanticSearch() Prompt {
	return Prompt{
		Definition: mcp.Prompt{
			Name:        "setup-semantic-search",
			Description: "Guide to discover vector-enabled tables and perform semantic search efficiently. Optimized for token usage.",
			Arguments: []mcp.PromptArgument{
				{
					Name:        "query_text",
					Description: "The semantic search query to execute",
					Required:    true,
				},
			},
		},
		Handler: func(args map[string]string) mcp.PromptResult {
			queryText := args["query_text"]
			if queryText == "" {
				queryText = "[your search query]"
			}

			return mcp.PromptResult{
				Description: fmt.Sprintf("Semantic search setup and execution for: %q", queryText),
				Messages: []mcp.PromptMessage{
					{
						Role: "user",
						Content: mcp.ContentItem{
							Type: "text",
							Text: fmt.Sprintf(`I need to find documents or data related to: %q

<semantic_search_workflow>
Step 1: Discover Vector-Enabled Tables
- Call: get_schema_info(vector_tables_only=true)
- This returns ONLY tables that support semantic/vector search
- Review the table names and descriptions to understand what data is available
- Identify which table(s) are most relevant to your search query

Step 2: Select Target Table
- Choose the table whose name or description best matches your search intent
- Consider:
  * Table name indicates its content domain
  * Description explains what kind of documents/data it contains
  * Column names suggest the type of text content stored
- If multiple tables seem relevant, start with the most specific one

Step 3: Execute Initial Search (Summary Mode)
- Call: similarity_search(
    table_name="[selected_table]",
    query_text=%q,
    output_format="summary"
  )
- Summary mode returns concise results optimized for token usage
- Review the results to see if they match what you're looking for
- Check the relevance scores (higher scores = better matches)

Step 4: Evaluate Results
- Do the results answer the question?
- Is more detail needed from any specific result?
- Should you try a different table or refine the query?

Step 5: Get Full Details (Only if Needed)
- If summary results are promising but you need complete content:
  * Call similarity_search again with output_format="full"
  * This returns complete document text
  * Use only when user explicitly requests details
- If results aren't relevant:
  * Try a different vector-enabled table
  * Rephrase the search query
  * Consider if the data might not exist in this database
</semantic_search_workflow>

<token_optimization>
To manage token usage effectively:
- ALWAYS start with output_format="summary" (default)
- Only use output_format="full" when:
  * User explicitly requests complete content
  * Summary showed promising results but lacked needed details
- Use max_results parameter to limit the number of matches (default: 5)
- If you need many results, get summaries first, then fetch full content for specific items
</token_optimization>

<search_parameters_guide>
Required:
- table_name: The vector-enabled table to search (from get_schema_info)
- query_text: Your semantic search query

Optional:
- output_format: "summary" (default, recommended) or "full"
- max_results: Number of matches to return (default: 5)
- column_filter: JSON object to filter results by column values
  Example: {"document_type": "FAQ", "language": "en"}
</search_parameters_guide>

<common_scenarios>
Scenario: No vector-enabled tables found
- The database may not have semantic search capability set up
- Try using query_database with SQL LIKE or full-text search instead
- Suggest checking if embeddings need to be generated

Scenario: Results have low relevance scores (< 0.5)
- Query may be too specific or use different terminology
- Try broader or alternative phrasing
- Check if the data domain matches your search

Scenario: Too many results needed
- First call with max_results=10 and output_format="summary"
- Review summaries and identify most relevant ones
- Make targeted calls for full content of specific items

Scenario: Need to filter by metadata
- Use column_filter to narrow results by attributes
- Example: Only search FAQs, or only English documents
- Combines semantic similarity with exact metadata matching
</common_scenarios>

Begin the search now. Start with discovering available tables, then execute the search with summary output.`, queryText, queryText),
						},
					},
				},
			}
		},
	}
}
