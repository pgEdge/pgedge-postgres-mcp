# Enabling or Disabling Built-in Features

You can selectively enable or disable built-in tools, resources, and prompts; all features are enabled by default. When a feature is disabled:

    - It is not advertised to the LLM in list operations
    - Attempts to use it return an error message

Within the `builtins` section of the configuration file, you can indicate if you would like the feature to be enabled (`true`) or disabled (`false`):

```yaml
builtins:
  tools:
    query_database: true        # Execute SQL queries
    get_schema_info: true       # Get schema information
    similarity_search: false    # Disable vector similarity search
    execute_explain: true       # Execute EXPLAIN queries
    generate_embedding: false   # Disable embedding generation
    search_knowledgebase: true  # Search documentation knowledgebase
  resources:
    system_info: true           # pg://system_info
  prompts:
    explore_database: true      # explore-database prompt
    setup_semantic_search: true # setup-semantic-search prompt
    diagnose_query_issue: true  # diagnose-query-issue prompt
    design_schema: true         # design-schema prompt
```

!!! Notes

    - The `read_resource` tool is always enabled as it is required for listing resources.
    - Features can also be disabled by other configuration settings (e.g., `search_knowledgebase` requires `knowledgebase.enabled: true`).
