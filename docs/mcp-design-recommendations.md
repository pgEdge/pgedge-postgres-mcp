# MCP Tool and Resource Design Recommendations

**Date:** 2025-11-21
**Review Focus:** Tool descriptions, resource design, prompts, and token efficiency

## Executive Summary

After analyzing best practices from:

- MCP Tool Design: From APIs to AI-First Interfaces
- Writing Effective MCP Tools (official docs)
- Prompt Design Principles (Cursor/Priompt)
- Internal design guide (.claude/prompts-tools-resources-chat.md)

This document provides actionable recommendations to improve LLM decision-making
and minimize token usage while maintaining functionality.

## Current State Analysis

### Tools (5 visible, 1 hidden)

1. **query_database** - SQL query execution
2. **get_schema_info** - Schema discovery
3. **similarity_search** - Hybrid vector + BM25 search
4. **read_resource** - Resource access adapter
5. **generate_embedding** - Embedding generation
6. **authenticate_user** - Hidden authentication tool

### Resources (2)

1. **pg://database-schema** - Table listing
2. **pg://system-info** - PostgreSQL version info

### Prompts (0)

Currently no MCP prompts are implemented.

## Key Findings

### Strengths

1. **Good tool separation** - Tools are intent-based rather than API-based
2. **Token budget awareness** - similarity_search already has max_output_tokens
3. **Discovery optimization** - Resources use pull model correctly
4. **Clear naming** - Tool names are descriptive and unambiguous
5. **Read-only safety** - query_database enforces read-only transactions

### Areas for Improvement

1. **Tool descriptions lack decision criteria** - LLMs need explicit "use when"
   guidance
2. **Missing workflow prompts** - Common multi-step operations not guided
3. **No output format control** - Tools return fixed formats, can't adapt to
   context
4. **Resource overlap with tools** - read_resource duplicates native resources
5. **Token budget still too high** - Default 1000 tokens is good, but no
   summary mode
6. **No progressive disclosure** - Large results not paginated or truncatable

## Recommendations

### 1. Enhance Tool Descriptions with Decision Criteria

**Problem:** LLMs see all tools but lack explicit decision guidance.

**Current example (query_database):**
```go
Description: "Execute a SQL query against the PostgreSQL database..."
```

**Recommended structure:**
```go
Description: `Execute SQL queries for STRUCTURED, EXACT data retrieval.

<usecase>
Use query_database when you need:
- Exact matches by ID, status, date ranges, or specific column values
- Aggregations: COUNT, SUM, AVG, GROUP BY, HAVING
- Joins across tables using foreign keys
- Sorting or filtering by structured columns
- Transaction data, user records, system logs with known schema
</usecase>

<when_not_to_use>
DO NOT use for:
- Natural language content search → use similarity_search instead
- Finding topics, themes, or concepts in text → use similarity_search
- "Documents about X" queries → use similarity_search
</when_not_to_use>

<examples>
✓ "How many orders were placed last week?"
✓ "Show all users with status = 'active' and created > '2024-01-01'"
✓ "Average order value grouped by region"
✗ "Find documents about database performance" → similarity_search
</examples>

All queries run in read-only transactions. Connection switching: 'SELECT *
FROM table at postgres://...' or 'set default database to postgres://...'.`
```

**Rationale:** Research shows structured descriptions with XML-style tags help
LLMs make better tool choices without consuming more tokens (descriptions are
cached).

### 2. Improve similarity_search Description

**Current:** Good but lacks clear distinction from query_database.

**Recommended:**
```go
Description: `Semantic search for NATURAL LANGUAGE and CONCEPT-BASED queries.

<usecase>
Use similarity_search when you need:
- Finding content by meaning, not exact keywords
- "Documents about X" or "Similar to Y" queries
- Topic/theme discovery in unstructured text
- When user language may differ from stored text
- Searching Wikipedia articles, documentation, support tickets
</usecase>

<technical_details>
- Hybrid: Vector similarity (pgvector) + BM25 lexical ranking
- MMR diversity filtering (λ parameter: 0.0=diverse, 1.0=relevant)
- Automatic chunking with token budgets
- Smart column weighting (title vs content)
</technical_details>

<when_not_to_use>
DO NOT use for:
- Structured filters (status, dates, IDs) → use query_database
- Aggregations or joins → use query_database
- Exact keyword matching → query_database with LIKE/ILIKE
</when_not_to_use>

<important>
ALWAYS call get_schema_info with vector_tables_only=true FIRST if you don't
know the table name. This tool will fail without a valid table name.
</important>

<examples>
✓ "Find tickets about connection timeouts"
✓ "Documents similar to ID 123"
✓ "Wikipedia articles related to quantum computing"
✗ "Count tickets by status" → query_database
✗ "Users created last week" → query_database
</examples>

Default budget: 1000 tokens (~10 chunks). Increase max_output_tokens if more
context needed, but beware rate limits.`
```

### 3. Add Summary Mode Parameter

**Problem:** No way to do lightweight exploration before deep dive.

**Recommended addition to similarity_search InputSchema:**
```go
"output_format": map[string]interface{}{
    "type": "string",
    "enum": []string{"full", "summary", "ids_only"},
    "default": "full",
    "description": "Output format: 'full'=complete chunks (default),
                    'summary'=titles+snippets only (~50 tokens total),
                    'ids_only'=just IDs for progressive disclosure",
}
```

**Implementation:**
- **full**: Current behavior (~1000 tokens)
- **summary**: Return only titles/snippets (~50 tokens, 10x more results fit)
- **ids_only**: Just row IDs for follow-up queries (~10 tokens)

**Benefits:**
- LLM can scan 10x more results in summary mode
- Progressive disclosure: overview → details only when needed
- Reduces rate limit issues for exploration queries

### 4. Add Result Limiting to query_database

**Problem:** Large result sets consume excessive tokens.

**Recommended addition:**
```go
"limit": map[string]interface{}{
    "type": "integer",
    "default": 100,
    "minimum": 1,
    "maximum": 1000,
    "description": "Maximum rows to return (default: 100, max: 1000).
                    Automatically appended to query if not present.",
}
```

**Implementation:**
```go
// In QueryDatabaseTool handler
sqlQuery := strings.TrimSpace(queryCtx.CleanedQuery)

// Auto-inject LIMIT if not present and limit parameter provided
if limit, ok := args["limit"].(float64); ok {
    if !strings.Contains(strings.ToUpper(sqlQuery), "LIMIT") {
        sqlQuery = fmt.Sprintf("%s LIMIT %d", sqlQuery, int(limit))
    }
}
```

**Benefits:**
- Prevents accidental full table scans
- Gives LLM control over result size
- Clear guidance in description

### 5. Implement MCP Prompts for Common Workflows

**Problem:** Multi-step operations require LLM to figure out the sequence.

**Recommended prompts:**

#### a) Explore Database Prompt

```go
// In new file: internal/prompts/explore_database.go
func ExploreDatabase() Prompt {
    return Prompt{
        Name: "explore-database",
        Description: "Multi-step workflow to explore an unfamiliar database
                      and understand its structure",
        Arguments: []PromptArgument{},
        Handler: func(args map[string]string) PromptResult {
            return PromptResult{
                Messages: []PromptMessage{
                    {
                        Role: "user",
                        Content: `I need to explore this database systematically:

1. Call get_schema_info() with no parameters to see all tables
2. Identify tables of interest based on names and descriptions
3. For key tables, call get_schema_info(schema_name="...") for details
4. Look for:
   - Primary/foreign key relationships
   - Vector columns (for semantic search capability)
   - Interesting text columns for analysis
5. Summarize the database structure and suggest relevant queries

Begin exploration now.`,
                    },
                },
            }
        },
    }
}
```

#### b) Semantic Search Setup Prompt

```go
func SemanticSearchSetup() Prompt {
    return Prompt{
        Name: "setup-semantic-search",
        Description: "Guide to discover vector-enabled tables and perform
                      semantic search",
        Arguments: []PromptArgument{
            {
                Name: "query_text",
                Description: "The search query to execute",
                Required: true,
            },
        },
        Handler: func(args map[string]string) PromptResult {
            queryText := args["query_text"]
            return PromptResult{
                Messages: []PromptMessage{
                    {
                        Role: "user",
                        Content: fmt.Sprintf(`Execute semantic search workflow:

Query: %q

Step 1: Call get_schema_info(vector_tables_only=true) to find tables with
        vector columns

Step 2: Examine the output to identify the most relevant table for this query
        Consider:
        - Table names and descriptions
        - Column names that suggest content type
        - Vector column presence

Step 3: Call similarity_search with:
        - table_name: [selected table]
        - query_text: %q
        - output_format: "summary" (for initial exploration)

Step 4: If results look promising, call again with output_format="full" for
        detailed content

Step 5: Summarize findings and suggest follow-up queries if needed

Execute this workflow now.`, queryText, queryText),
                    },
                },
            }
        },
    }
}
```

#### c) Query Performance Analysis Prompt

```go
func AnalyzeQueryPerformance() Prompt {
    return Prompt{
        Name: "analyze-query-performance",
        Description: "Multi-step analysis of slow query performance",
        Arguments: []PromptArgument{
            {
                Name: "slow_query",
                Description: "The SQL query that's running slowly",
                Required: true,
            },
        },
        Handler: func(args map[string]string) PromptResult {
            slowQuery := args["slow_query"]
            return PromptResult{
                Messages: []PromptMessage{
                    {
                        Role: "user",
                        Content: fmt.Sprintf(`Analyze query performance:

Query: %s

Step 1: Execute EXPLAIN (ANALYZE, BUFFERS) on the query to see execution plan
Step 2: Identify performance bottlenecks:
        - Sequential scans on large tables
        - Missing indexes on filter/join columns
        - High buffer usage
        - Nested loop joins on large datasets
Step 3: Check table statistics with pg_stat_user_tables
Step 4: Suggest specific improvements:
        - Index creation statements
        - Query rewrites for better performance
        - VACUUM/ANALYZE recommendations
Step 5: Estimate improvement impact

Execute analysis now.`, slowQuery),
                    },
                },
            }
        },
    }
}
```

**Benefits:**
- Guides LLMs through proven workflows
- Reduces trial-and-error token consumption
- Teaches best practices for tool sequencing
- Users can invoke with simple prompt name

### 6. Consolidate or Remove read_resource Tool

**Problem:** Duplication between read_resource tool and native resources.

**Analysis:**
- Native MCP resources/read endpoint provides same functionality
- read_resource tool adds backward compatibility but creates confusion
- Resources are already properly implemented with pull model

**Recommendation:** Keep but add clear guidance:

```go
Description: `Read MCP resources (database schema, system info) via tool
              interface.

<important>
This tool provides backward compatibility. Modern MCP clients should use the
native resources/read endpoint instead.
</important>

<usecase>
Use read_resource when:
- Client doesn't support native resources/read
- You need resource content as tool output
- Building tool-only workflows
</usecase>

<available_resources>
- pg://database-schema: Lightweight table listing (names only)
- pg://system-info: PostgreSQL version and platform details
</available_resources>

<alternative>
For detailed schema info, prefer get_schema_info tool - it provides column
details, constraints, and descriptions that resources don't include.
</alternative>`
```

### 7. Enhance Resource Descriptions

**Current resources are good but could be more explicit about differences
from tools.**

#### pg://database-schema

**Current:**
```go
Description: "Returns a lightweight overview of all tables..."
```

**Recommended:**
```go
Description: `Lightweight table listing: schema names, table names, and owners
              only.

<usecase>
Use this resource for:
- Quick overview of database structure
- Finding all schemas and tables
- Checking table ownership
</usecase>

<limitations>
Does NOT include:
- Column details (use get_schema_info tool instead)
- Data types, constraints, indexes
- Table descriptions from pg_description
</limitations>

<when_to_use_tools>
For detailed schema exploration, use get_schema_info tool which provides:
- All columns with data types
- Primary/foreign key constraints
- Nullable/not-null information
- pg_description comments
- Vector column detection
</when_to_use_tools>

This resource is best for initial discovery; tools for detailed analysis.`
```

#### pg://system-info

**Recommended:**
```go
Description: `PostgreSQL server metadata: version, OS, architecture,
              connection details.

<usecase>
Use for:
- Version compatibility checks
- Platform verification
- Connection debugging
- System architecture discovery
</usecase>

<provided_info>
- PostgreSQL version (major.minor.patch)
- Operating system (Linux, Darwin, Windows)
- CPU architecture (x86_64, aarch64)
- Compiler used for build
- Current database and user
- Connection host and port
</provided_info>

<caching>
This resource is highly cacheable - system info rarely changes.
</caching>`
```

### 8. Implement Error Messages as Teaching Tools

**Current:** Errors are informative but could guide next steps.

**Example enhancement for similarity_search:**

```go
// When table not found
if err != nil {
    return mcp.NewToolError(fmt.Sprintf(
        `Table '%s' not found in metadata.

<next_steps>
1. Call get_schema_info(vector_tables_only=true) to discover available tables
   with vector columns
2. Review the output to find the correct table name
3. Retry similarity_search with the correct table name
</next_steps>

<common_mistakes>
- Forgot schema prefix: Try 'public.%s' instead of '%s'
- Table exists but has no vector columns: Check get_schema_info output
- Metadata not loaded: Wait for database initialization to complete
</common_mistakes>

Error details: %v`, tableName, tableName, tableName, err))
}

// When no vector columns found
if len(vectorCols) == 0 {
    return mcp.NewToolError(fmt.Sprintf(
        `Table '%s' has no vector columns - similarity_search requires pgvector.

<solution>
Use query_database instead for structured queries on this table:
  query_database(query="SELECT * FROM %s WHERE ...")

Or use get_schema_info to find tables with vector columns:
  get_schema_info(vector_tables_only=true)
</solution>

<about_vector_columns>
Vector columns are created with pgvector extension:
  CREATE TABLE items (embedding vector(1536));

This table doesn't have any vector-type columns.
</about_vector_columns>`, tableName, tableName))
}
```

**Benefits:**
- Reduces back-and-forth trial and error
- Teaches LLMs the correct workflow
- Minimizes wasted tool calls
- Improves user experience

### 9. Add Monitoring and Logging Recommendations

**Problem:** Hard to diagnose rate limit issues without visibility.

**Recommended additions:**

```go
// In similarity_search handler, after results
import "pgedge-postgres-mcp/internal/logging"

logging.Info(
    "similarity_search_executed",
    "table", tableName,
    "query_length", len(queryText),
    "results", len(finalChunks),
    "total_tokens", totalTokens,
    "token_budget", searchCfg.MaxOutputTokens,
    "top_n", searchCfg.TopN,
    "lambda", searchCfg.Lambda,
)
```

**Also track in query_database:**

```go
logging.Info(
    "query_database_executed",
    "query_length", len(sqlQuery),
    "rows_returned", len(results),
    "estimated_tokens", len(string(resultsJSON))/4, // rough estimate
)
```

**Benefits:**
- Identify which tool calls consume most tokens
- Detect rate limit causes
- Optimize default parameters based on real usage
- Track query patterns for improvements

### 10. Parameter Defaults Review

**Current defaults are good, but worth documenting rationale:**

| Tool | Parameter | Current | Recommendation | Rationale |
|------|-----------|---------|----------------|-----------|
| similarity_search | max_output_tokens | 1000 | Keep 1000 | Down from 2500, good balance |
| similarity_search | top_n | 10 | Keep 10 | Good for most queries |
| similarity_search | chunk_size_tokens | 100 | Keep 100 | ~1 paragraph |
| similarity_search | lambda | 0.6 | Keep 0.6 | 60% relevance, 40% diversity |
| query_database | limit | none | Add 100 default | Prevent large result sets |

### 11. Consider Adding Tools

**Potential new tools based on common patterns:**

#### a) execute_explain (Query Analysis)

```go
Name: "execute_explain",
Description: `Execute EXPLAIN ANALYZE on a query to diagnose performance.

<usecase>
Use when:
- Query runs slowly
- Investigating performance issues
- Planning index creation
- Understanding query execution
</usecase>

Returns execution plan with timing, row counts, and buffer usage.`
```

#### b) list_vector_tables (Convenience Tool)

```go
Name: "list_vector_tables",
Description: `Quick way to list all tables with pgvector columns.

<usecase>
Use before similarity_search when you don't know the table name.
Equivalent to: get_schema_info(vector_tables_only=true)
</usecase>

<advantage>
Simpler interface - no parameters needed for common case.
</advantage>`
```

**Recommendation:** Start with prompts (lower cost) before adding tools. Tools
increase API surface area and maintenance burden.

## Token Efficiency Summary

### Current Token Costs (Estimated)

**Per conversation:**
- Tool definitions: ~800 tokens (5 tools, cached)
- Resource definitions: ~100 tokens (2 resources, cached)
- Total static overhead: ~900 tokens (cached)

**Per tool call:**
- query_database result: 500-10,000+ tokens (depends on rows)
- similarity_search result: 1,000-3,000 tokens (with budget)
- get_schema_info: 1,000-5,000 tokens (depends on tables)

### Recommended Improvements Impact

| Change | Token Savings | Implementation Effort |
|--------|---------------|----------------------|
| Add summary mode to similarity_search | 90% on exploration | Low (1 parameter, format logic) |
| Add limit to query_database | Up to 95% on large queries | Low (auto-inject LIMIT) |
| Enhanced descriptions | -5% static (more text) +30% on avoided mistakes | Medium (rewrite descriptions) |
| MCP prompts | Variable (guides correct tool use) | Medium (3 new prompts) |
| Progressive disclosure (ids_only) | 99% on initial scan | Medium (new format) |

### Net Impact

- **Static cost:** +100 tokens (better descriptions, cached)
- **Dynamic savings:** 30-90% per tool call (better decisions, summary modes)
- **Overall:** Significant net savings from reduced trial-and-error

## Implementation Priority

### Phase 1: High Impact, Low Effort (Week 1)

1. Enhance tool descriptions with structured format (done)
2. Add limit parameter to query_database (done)
3. Add summary mode to similarity_search (done)
4. Update resource descriptions (done)

**Estimated impact:** 40% token reduction on common queries

### Phase 2: Workflow Optimization (Week 2)

1. Implement 3 core MCP prompts (done)
2. Add prompt registry and handlers (done)
3. Enhance error messages with next steps (done)
4. Document prompt usage (done)

**Estimated impact:** 25% reduction in trial-and-error

### Phase 3: Advanced Features (Week 3-4)

1. Add progressive disclosure (ids_only mode)
2. Implement monitoring and logging
3. Add execute_explain tool
4. Create comprehensive testing suite

**Estimated impact:** 15% additional optimization

## Testing Recommendations

### Before/After Comparison

Test common queries with current vs. improved design:

```
Test Case 1: "Find documents about database performance"
- Current: May try query_database first, fail, then similarity_search
- Improved: Descriptions guide to similarity_search immediately
- Savings: ~2,000 tokens (one avoided tool call)

Test Case 2: "Show me all tables"
- Current: Might use resource or get_schema_info
- Improved: Clear guidance that resource is lighter
- Savings: ~4,000 tokens (resource vs full schema)

Test Case 3: "Count tickets by status"
- Current: Might try similarity_search first
- Improved: Examples show this is query_database territory
- Savings: ~1,500 tokens (one avoided tool call)
```

### Metrics to Track

1. **Tool selection accuracy:** % of queries that pick the right tool first try
2. **Average tokens per query:** Before vs after optimization
3. **Rate limit frequency:** Hits per hour/day
4. **User satisfaction:** Query success rate

## Conclusion

The current tool design is solid with good separation of concerns and token
awareness. The recommended improvements focus on:

1. **Better LLM guidance** through structured descriptions
2. **Flexible output formats** for different use cases
3. **Workflow prompts** for complex multi-step operations
4. **Progressive disclosure** to minimize initial token cost

These changes follow industry best practices from Anthropic, MCP official
docs, and real-world AI interface design. The phased approach allows for
incremental validation of improvements.

**Key insight:** Tool descriptions are cached and cost almost nothing, while
tool results are dynamic and expensive. Invest in verbose descriptions to save
on execution costs.

## Next Steps

1. Review and approve this plan
2. Prioritize Phase 1 changes
3. Implement and test incrementally
4. Measure token usage before/after
5. Iterate based on real-world usage patterns

## References

- [MCP Tool Design: From APIs to AI-First](https://useai.substack.com/p/mcp-tool-design-from-apis-to-ai-first)
- [Writing Effective MCP Tools](https://modelcontextprotocol.info/docs/tutorials/writing-effective-tools/)
- [Prompt Design Principles](https://cursor.com/blog/prompt-design)
- Internal: [.claude/prompts-tools-resources-chat.md](https://github.com/pgEdge/pgedge-mcp/blob/main/.claude/prompts-tools-resources-chat.md)
