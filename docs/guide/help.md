# Using Online Help

You can include the `-help` keyword at the command line to retreive an online list of command options:

```bash
bin/pgedge-postgres-mcp -help
```

You can append the following command-line options to the executable to manage server behavior or retrieve detailed configuration and usage information.

```bash
Usage of bin/pgedge-postgres-mcp:
  -add-token
    	Add a new API token
  -add-user
    	Add a new user
  -addr string
    	HTTP server address
  -cert string
    	Path to TLS certificate file
  -chain string
    	Path to TLS certificate chain file (optional)
  -config string
    	Path to configuration file (default "/Users/dpage/git/pgedge-nla/bin/pgedge-postgres-mcp.yaml")
  -db-host string
    	Database host
  -db-name string
    	Database name
  -db-password string
    	Database password
  -db-port int
    	Database port
  -db-sslmode string
    	Database SSL mode (disable, require, verify-ca, verify-full)
  -db-user string
    	Database user
  -debug
    	Enable debug logging (logs HTTP requests/responses)
  -delete-user
    	Delete a user
  -disable-user
    	Disable a user account
  -enable-user
    	Enable a user account
  -http
    	Enable HTTP transport mode (default: stdio)
  -key string
    	Path to TLS key file
  -list-tokens
    	List all API tokens
  -list-users
    	List all users
  -no-auth
    	Disable API token authentication in HTTP mode
  -password string
    	Password for user management commands (prompted if not provided)
  -remove-token string
    	Remove an API token by ID or hash prefix
  -tls
    	Enable TLS/HTTPS (requires -http)
  -token-database string
    	Bind token to specific database name (used with -add-token, empty = first configured database)
  -token-expiry string
    	Token expiry duration: '30d', '1y', '2w', '12h', 'never' (used with -add-token)
  -token-file string
    	Path to API token file
  -token-note string
    	Annotation for the new token (used with -add-token)
  -update-user
    	Update an existing user
  -user-file string
    	Path to user file
  -user-note string
    	Annotation for the new user (used with -add-user)
  -username string
    	Username for user management commands
```