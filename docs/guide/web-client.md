# Web Client

The pgEdge Natural Language Agent includes a web-based chat interface for
interacting with your PostgreSQL database using natural language queries.

## Logging In

Navigate to the web interface URL (typically `http://localhost:8080` for native
deployments or `http://localhost:8081` for Docker).

![Login Page](../img/screenshots/01-login-page.png)

Enter your username and password to access the chat interface. User accounts
are created either:

- Via the `-add-user` command line flag (native deployment)
- Via the `INIT_USERS` environment variable (Docker deployment)

See [Authentication](authentication.md) for details on user management.

## Main Interface

After logging in, you'll see the main chat interface:

![Main Interface](../img/screenshots/02-main-interface-empty.png)

The interface consists of:

- **Header** - Contains the pgEdge logo, theme toggle, help button, and user
  menu
- **Connection Status** - Shows your database connection status (click to
  expand details)
- **Provider/Model Selection** - Choose your LLM provider and model
- **Chat Area** - Displays conversation history
- **Message Input** - Type your natural language queries here

## Selecting a Provider and Model

### Provider Selection

Click the Provider dropdown to choose your LLM provider:

![Provider Selection](../img/screenshots/03-provider-dropdown.png)

Available providers depend on your server configuration:

- **Anthropic Claude** - Requires `PGEDGE_ANTHROPIC_API_KEY`
- **OpenAI** - Requires `PGEDGE_OPENAI_API_KEY`
- **Ollama** - Requires local Ollama installation

### Model Selection

After selecting a provider, choose a specific model:

![Model Selection](../img/screenshots/04-model-dropdown.png)

Different models offer varying capabilities and performance characteristics.
See your provider's documentation for model details.

## Querying Your Database

Type a natural language query in the message input and press Enter or click
the send button:

![Query Input](../img/screenshots/05-query-typed.png)

The LLM will analyze your query, interact with your database using the
available tools, and return a response:

![Query Response](../img/screenshots/06-query-response-light.png)

### What You Can Ask

- **Schema exploration** - "What tables are in the database?"
- **Data queries** - "Show me the top 10 customers by order count"
- **Relationships** - "How are the orders and products tables related?"
- **Analysis** - "What's the average order value by month?"
- **Semantic search** - "Find documents about authentication" (requires
  pgEdge Vectorizer)

## Dark Mode

Toggle between light and dark themes using the theme button in the header:

![Dark Mode](../img/screenshots/07-query-response-dark.png)

Your theme preference is saved and persists across sessions.

## Getting Help

Click the help icon in the header to open the help panel:

![Help Panel](../img/screenshots/08-help-panel.png)

The help panel provides:

- Getting started tips
- Chat interface usage
- Available prompts
- Settings explanations
- Keyboard shortcuts

## Preferences

Click the settings icon (gear) to access preferences:

![Preferences](../img/screenshots/09-preferences-popover.png)

Available options:

- **Show Activity** - Display tool calls and resource reads in the
  conversation
- **Render Markdown** - Format responses with markdown styling
- **Debug Messages** - Show system-level debug information

Preferences are saved locally and persist across sessions.

## Using Prompts

Prompts are guided workflows for common tasks. Click the brain icon next to
the message input to access them:

![Prompt Selection](../img/screenshots/10-prompt-popover-initial.png)

### Selecting a Prompt

Open the prompt dropdown to see available prompts:

![Prompt List](../img/screenshots/11-prompt-list-expanded.png)

### Configuring Arguments

Some prompts require arguments. Fill in the required fields:

![Prompt Arguments](../img/screenshots/12-prompt-selected.png)

Click "Execute Prompt" to run the workflow. The LLM will follow the prompt's
guided steps to complete the task.

### Available Prompts

| Prompt | Description |
|--------|-------------|
| `explore-database` | Systematically explore database structure |
| `setup-semantic-search` | Configure semantic search on vector tables |
| `diagnose-query-issue` | Debug problems with queries |

See [Prompts Reference](../reference/prompts.md) for complete documentation.

## User Menu

Click your avatar in the header to access the user menu:

![User Menu](../img/screenshots/14-user-menu.png)

Options include:

- View current username
- Log out

## Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| `Enter` | Send message |
| `Shift+Enter` | New line in message |
| `↑` / `↓` | Navigate query history |
| `Escape` | Close dialogs/panels |

## Saving Conversations

Click the download icon next to the message input to export your conversation
as a Markdown file. This is useful for:

- Documenting database exploration sessions
- Sharing query results with team members
- Keeping records of analysis work

## Tips for Effective Queries

1. **Be specific** - "Show customers from California" works better than
   "Show some customers"

2. **Reference tables by name** - If you know the table name, include it:
   "Query the orders table for last month's sales"

3. **Ask follow-up questions** - The LLM maintains conversation context, so
   you can refine queries: "Now filter that to only show orders over $100"

4. **Use prompts for complex tasks** - For multi-step operations like setting
   up semantic search, use the built-in prompts

5. **Check activity** - Enable "Show Activity" in preferences to see which
   tools and resources the LLM is using

## Troubleshooting

### Connection Issues

If you see a red connection indicator:

1. Check that the MCP server is running
2. Verify database credentials in the server configuration
3. Check network connectivity to the database host

### Slow Responses

- Try a faster model (e.g., `claude-sonnet` instead of `claude-opus`)
- Enable response streaming in server configuration
- Check your LLM provider's rate limits

### Authentication Errors

- Verify your username and password
- Check that the user exists (use `-list-users` on the server)
- Ensure authentication is enabled in server configuration

See [Troubleshooting](troubleshooting.md) for more help.
