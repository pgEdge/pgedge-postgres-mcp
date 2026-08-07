# Quick Start

Get the pgEdge Postgres MCP Server running with your
preferred client in minutes.

## Prerequisites

Every setup requires the following:

- A PostgreSQL 14+ database that is running and accessible.
- An LLM API key from
  [Anthropic](https://console.anthropic.com/),
  [OpenAI](https://platform.openai.com/), or a local
  [Ollama](https://ollama.ai/) installation.
- The MCP server binary, Docker image, or package (see
  below).

### Obtaining the MCP Server

=== "pgEdge Packages"

    Install the MCP server from the
    [pgEdge Enterprise](https://docs.pgedge.com/enterprise/)
    package repository. First, configure the repository for
    your platform, then install the package.

    **Debian / Ubuntu:**

    ```bash
    sudo apt-get update
    sudo apt-get install -y curl gnupg2 lsb-release

    sudo curl -sSL \
        https://apt.pgedge.com/repodeb/pgedge-release_latest_all.deb \
        -o /tmp/pgedge-release.deb
    sudo dpkg -i /tmp/pgedge-release.deb
    rm -f /tmp/pgedge-release.deb

    sudo apt-get update
    sudo apt-get install -y pgedge-postgres-mcp
    ```

    **RHEL / Rocky / Alma Linux:**

    ```bash
    sudo dnf install -y \
        https://dnf.pgedge.com/reporpm/pgedge-release-latest.noarch.rpm

    sudo dnf install -y pgedge-postgres-mcp
    ```

    The binary is installed at
    `/usr/bin/pgedge-postgres-mcp`. The default
    configuration file is at
    `/etc/pgedge/postgres-mcp.yaml`.

=== "GitHub Release"

    Download the latest release for your platform from the
    [GitHub Releases](https://github.com/pgEdge/pgedge-postgres-mcp/releases)
    page. Extract the archive and note the path to the
    `pgedge-postgres-mcp` binary.

    ```bash
    # Example for Linux amd64
    tar xzf pgedge-postgres-mcp_linux_amd64.tar.gz
    chmod +x pgedge-postgres-mcp
    ```

=== "Docker"

    !!! warning

        Docker Compose deployments use HTTP mode and
        require `PGEDGE_HTTP_ENABLED=true` in the
        `.env` file. For stdio-based clients, you can
        either use a local binary or run the Docker
        image directly with the client-specific
        `docker run -i --rm` examples below.

    Pull the Docker image from the
    [GitHub Container Registry](https://github.com/orgs/pgEdge/packages):

    ```bash
    docker pull ghcr.io/pgedge/postgres-mcp:latest
    ```

=== "Build from Source"

    Clone the
    [repository](https://github.com/pgEdge/pgedge-postgres-mcp)
    and build the binary:

    ```bash
    git clone \
        https://github.com/pgEdge/pgedge-postgres-mcp.git
    cd pgedge-postgres-mcp
    make build
    ```

    The binary is created at `bin/pgedge-postgres-mcp`.

### Creating a Configuration File

The MCP server reads settings from a YAML configuration
file. Create a file named `postgres-mcp.yaml` and place it
in the same directory as the binary, or specify its
location with the `-config` flag. For pgEdge packages, the
default location is `/etc/pgedge/postgres-mcp.yaml`. For
builds from source, place the file at
`bin/postgres-mcp.yaml` alongside the compiled binary.

> **Note:** For pgEdge package installations, edit the
> existing `/etc/pgedge/postgres-mcp.yaml` file. Comment
> out or remove unused options for a quick setup. Set the
> `user_file` path to `/etc/pgedge/postgres-mcp-users.yaml`
> so user credentials are stored in the same directory as
> the configuration.

The following example shows a minimal configuration with
all available sections. Uncomment and edit the sections you
need. For a full reference with detailed comments, see
the [Server Configuration Example](../reference/config-examples/server.md)
and the [Configuration Guide](configuration.md).

```yaml
# Database connections (at least one is required)
databases:
    - name: "mydb"
      host: "localhost"
      port: 5432
      database: "mydb"
      user: "myuser"
      password: "mypass"
      sslmode: "prefer"
      # allow_writes: false
      # allow_llm_switching: true
      # allowed_pl_languages: []
      # available_to_users: []
      # pool_max_conns: 4
      # pool_min_conns: 0
      # pool_max_conn_idle_time: "30m"

# HTTP server (enable for Web UI, CLI HTTP mode,
# or API access)
# http:
#     enabled: true
#     address: ":8080"
#     tls:
#         enabled: false
#         cert_file: ""
#         key_file: ""
#         chain_file: ""
#     auth:
#         enabled: true
#         user_file: "./postgres-mcp-users.yaml"
#         max_failed_attempts_before_lockout: 0
#         rate_limit_window_minutes: 15
#         rate_limit_max_attempts: 10

# LLM proxy (required for the Web UI only)
# llm:
#     enabled: false
#     provider: "anthropic"
#     model: "claude-sonnet-4-5"
#     anthropic_api_key_file: "~/.anthropic-api-key"
#     # anthropic_base_url: ""
#     # openai_api_key_file: "~/.openai-api-key"
#     # openai_base_url: ""
#     # ollama_url: "http://localhost:11434"
#     # max_tokens: 4096

# Embedding generation (for the generate_embedding tool)
# embedding:
#     enabled: false
#     provider: "ollama"
#     model: "nomic-embed-text"
#     # openai_api_key_file: "~/.openai-api-key"
#     # openai_base_url: ""
#     # voyage_api_key_file: "~/.voyage-api-key"
#     # voyage_base_url: ""
#     # ollama_url: "http://localhost:11434"

# Knowledgebase search
# knowledgebase:
#     enabled: false
#     database_path: ""
#     embedding_provider: "ollama"
#     embedding_model: "nomic-embed-text"
#     # embedding_voyage_api_key_file: ""
#     # embedding_voyage_base_url: ""
#     # embedding_openai_api_key_file: ""
#     # embedding_openai_base_url: ""
#     # embedding_ollama_url: "http://localhost:11434"

# Built-in features (all enabled by default)
# builtins:
#     tools:
#         query_database: true
#         get_schema_info: true
#         similarity_search: true
#         execute_explain: true
#         generate_embedding: true
#         search_knowledgebase: true
#         count_rows: true
#         llm_connection_selection: false
#     resources:
#         system_info: true
#     prompts:
#         explore_database: true
#         setup_semantic_search: true
#         diagnose_query_issue: true
#         design_schema: true

# Other options
# secret_file: ""
# trace_file: ""
# custom_definitions_path: ""
# data_dir: ""
```

## Choosing a Client

Choose a client from the table below and follow the steps
in the corresponding section.

| Client | Transport | Best For |
|--------|-----------|----------|
| [CLI (Stdio)](#cli-stdio) | Stdio | Local single-user development |
| [CLI (HTTP)](#cli-http) | HTTP | Multi-user or remote access |
| [Web UI](#web-ui) | HTTP | Browser-based chat interface |
| [Claude Code](#claude-code) | Stdio | Anthropic CLI agent |
| [Claude Desktop](#claude-desktop) | Stdio | Anthropic desktop app |
| [Cursor](#cursor) | Stdio | AI code editor |
| [Windsurf](#windsurf) | Stdio | Codeium code editor |
| [VS Code Copilot](#vs-code-copilot) | Stdio | GitHub Copilot agent |

---

## CLI (Stdio)

The CLI client connects to the MCP server as a local
subprocess. This mode is ideal for single-user development.

=== "pgEdge Packages"

    Install the CLI client from the pgEdge repository
    (configured in [Prerequisites](#obtaining-the-mcp-server)):

    ```bash
    # Debian/Ubuntu: sudo apt-get install -y pgedge-nla-cli
    # RHEL/Rocky:    sudo dnf install -y pgedge-nla-cli
    ```

    Set your LLM API key and start the CLI:

    ```bash
    export PGEDGE_ANTHROPIC_API_KEY=sk-ant-...

    pgedge-nla-cli \
        -mcp-mode stdio \
        -mcp-server-path /usr/bin/pgedge-postgres-mcp \
        -mcp-server-config /etc/pgedge/postgres-mcp.yaml
    ```

=== "GitHub Release"

    Download the latest CLI release for your platform from the
    [GitHub Releases](https://github.com/pgEdge/pgedge-postgres-mcp/releases)
    page. Extract the archive and add the `pgedge-nla-cli` binary
    to your path.

    ```bash
    # Example for Linux amd64
    tar xzf pgedge-postgres-mcp-cli_linux_amd64.tar.gz
    chmod +x pgedge-nla-cli
    ```

    Set your LLM API key and start the CLI, pointing it at
    the server binary and your configuration file:

    ```bash
    export PGEDGE_ANTHROPIC_API_KEY=sk-ant-...

    ./pgedge-nla-cli \
        -mcp-mode stdio \
        -mcp-server-path ./pgedge-postgres-mcp \
        -mcp-server-config ./postgres-mcp.yaml
    ```

=== "Docker"

    The CLI client launches the MCP server as a local
    subprocess and requires a binary on disk. Use
    **pgEdge Packages**, **GitHub Release**, or
    **Build from Source** to obtain the binary.

=== "Build from Source"

    Build both the server and CLI client, then run:

    ```bash
    make build
    export PGEDGE_ANTHROPIC_API_KEY=sk-ant-...

    ./bin/pgedge-nla-cli \
        -mcp-mode stdio \
        -mcp-server-path ./bin/pgedge-postgres-mcp \
        -mcp-server-config ./bin/postgres-mcp.yaml
    ```

    By default, the CLI looks for the server binary and
    `postgres-mcp.yaml` in the `bin/` directory.

**Verify the setup** by typing a question at the prompt:

```
You: What tables are in my database?
```

The client should list your database tables. For full CLI
documentation, see the
[CLI Client Guide](cli-client.md).

---

## CLI (HTTP)

The CLI client connects to a running MCP server over HTTP.
This mode supports multiple concurrent users and remote
access. Authentication is enabled by default, so you must
create a user account before starting the server.

=== "pgEdge Packages"

    Install the CLI client from the pgEdge repository
    (configured in [Prerequisites](#obtaining-the-mcp-server)):

    ```bash
    # Debian/Ubuntu: sudo apt-get install -y pgedge-nla-cli
    # RHEL/Rocky:    sudo dnf install -y pgedge-nla-cli
    ```

    Create a user account, start the server, then connect
    the CLI:

    ```bash
    # Create a user account (runs and exits)
    sudo pgedge-postgres-mcp \
        -config /etc/pgedge/postgres-mcp.yaml \
        -add-user \
        -username admin -password secret123

    # Set ownership for the user file
    sudo chown pgedge:pgedge /etc/pgedge/postgres-mcp-users.yaml

    # Start the server in HTTP mode
    pgedge-postgres-mcp \
        -config /etc/pgedge/postgres-mcp.yaml \
        -http -addr :8080 &

    # Connect the CLI
    export PGEDGE_ANTHROPIC_API_KEY=sk-ant-...

    pgedge-nla-cli \
        -mcp-mode http \
        -mcp-url http://localhost:8080
    ```

    The CLI prompts for your username and password at
    startup.

=== "GitHub Release"

    Download the latest CLI release for your platform from the
    [GitHub Releases](https://github.com/pgEdge/pgedge-postgres-mcp/releases)
    page. Extract the archive and add the `pgedge-nla-cli` binary
    to your path.

    ```bash
    # Example for Linux amd64
    tar xzf pgedge-postgres-mcp-cli_linux_amd64.tar.gz
    chmod +x pgedge-nla-cli
    ```

    Create a user account, start the server, then connect
    the CLI:

    ```bash
    # Create a user account (runs and exits)
    ./pgedge-postgres-mcp \
        -config ./postgres-mcp.yaml \
        -add-user \
        -username admin -password secret123

    # Set permissions for the user file
    chmod 640 ./postgres-mcp-users.yaml

    # Start the server in HTTP mode
    ./pgedge-postgres-mcp \
        -config ./postgres-mcp.yaml \
        -http -addr :8080 &

    # Connect the CLI
    export PGEDGE_ANTHROPIC_API_KEY=sk-ant-...

    ./pgedge-nla-cli \
        -mcp-mode http \
        -mcp-url http://localhost:8080
    ```

    The CLI prompts for your username and password at
    startup.

=== "Docker"

    Clone the repository and use Docker Compose to start
    the server:

    ```bash
    git clone \
        https://github.com/pgEdge/pgedge-postgres-mcp.git
    cd pgedge-postgres-mcp
    cp .env.example .env
    ```

    Edit `.env` and set your database credentials, an LLM
    API key, and user accounts:

    - `PGEDGE_DB_HOST`, `PGEDGE_DB_PORT`,
      `PGEDGE_DB_NAME`, `PGEDGE_DB_USER`,
      `PGEDGE_DB_PASSWORD`
    - `PGEDGE_ANTHROPIC_API_KEY` or
      `PGEDGE_OPENAI_API_KEY`
    - `INIT_USERS=admin:secret123`

    Start the containers:

    ```bash
    docker compose up -d
    ```

    The CLI client is not included in the Docker image.
    Download the `pgedge-nla-cli` binary for your platform
    from the
    [GitHub Releases](https://github.com/pgEdge/pgedge-postgres-mcp/releases)
    page, then connect to the running server:

    ```bash
    export PGEDGE_ANTHROPIC_API_KEY=sk-ant-...

    ./pgedge-nla-cli \
        -mcp-mode http \
        -mcp-url http://localhost:8080
    ```

    The CLI prompts for your username and password at
    startup.

=== "Build from Source"

    Build the binaries, create a user account, start the
    server, then connect the CLI:

    ```bash
    make build

    # Create a user account (runs and exits)
    ./bin/pgedge-postgres-mcp \
        -config ./bin/postgres-mcp.yaml \
        -add-user \
        -username admin -password secret123

    # Set permissions for the user file
    chmod 640 ./postgres-mcp-users.yaml

    # Start the server in HTTP mode
    ./bin/pgedge-postgres-mcp \
        -config ./bin/postgres-mcp.yaml \
        -http -addr :8080 &

    # Connect the CLI
    export PGEDGE_ANTHROPIC_API_KEY=sk-ant-...

    ./bin/pgedge-nla-cli \
        -mcp-mode http \
        -mcp-url http://localhost:8080
    ```

    The CLI prompts for your username and password at
    startup.

**Verify the setup** by typing a question at the prompt:

```
You: What tables are in my database?
```

For full CLI documentation, see the
[CLI Client Guide](cli-client.md).

---

## Web UI

The web client provides a browser-based chat interface for
querying your database with natural language. The Web UI
requires the LLM proxy, so you must uncomment and
configure the `llm` section in your configuration file
before starting the server. Authentication is enabled by
default, so you must also create a user account.

=== "pgEdge Packages"

    Install the Web client from the pgEdge repository
    (configured in
    [Prerequisites](#obtaining-the-mcp-server)):

    ```bash
    # Debian/Ubuntu: sudo apt-get install -y pgedge-nla-web
    # RHEL/Rocky:    sudo dnf install -y pgedge-nla-web
    ```

    Create a user account, then start the server in HTTP
    mode:

    ```bash
    # Create a user account (runs and exits)
    sudo pgedge-postgres-mcp \
        -config /etc/pgedge/postgres-mcp.yaml \
        -add-user \
        -username admin -password secret123

    # Set ownership for the user file
    sudo chown pgedge:pgedge /etc/pgedge/postgres-mcp-users.yaml

    # Start the MCP server (choose one method)
    # Option 1: Using systemctl (recommended)
    sudo systemctl start pgedge-postgres-mcp.service

    # Option 2: Manual start
    # pgedge-postgres-mcp \
    #     -config /etc/pgedge/postgres-mcp.yaml \
    #     -http -addr :8080 &

    # Start nginx
    sudo systemctl start nginx.service
    ```

    > **Note:** On RHEL/Rocky with SELinux enabled, run
    > `sudo setsebool -P httpd_can_network_connect 1`
    > before starting nginx to allow the proxy connection.

    Open `http://localhost:8081` in your browser and log
    in with the credentials you created.


=== "GitHub Release"

    Create a user account, then start the server in HTTP
    mode:

    ```bash
    # Create a user account (runs and exits)
    ./pgedge-postgres-mcp \
        -config ./postgres-mcp.yaml \
        -add-user \
        -username admin -password secret123

    # Set permissions for the user file
    chmod 640 ./postgres-mcp-users.yaml

    # Start the server in HTTP mode
    ./pgedge-postgres-mcp \
        -config ./postgres-mcp.yaml \
        -http -addr :8080
    ```

    Verify the server is running:

    ```bash
    curl http://localhost:8080/health
    ```

    The standalone binary provides only the MCP API; there
    is no built-in web interface. Use the CLI client or
    Docker for the Web GUI.

=== "Docker"

    Clone the repository and use Docker Compose:

    ```bash
    git clone \
        https://github.com/pgEdge/pgedge-postgres-mcp.git
    cd pgedge-postgres-mcp
    cp .env.example .env
    ```

    Edit `.env` and set the following variables:

    - `PGEDGE_DB_HOST`, `PGEDGE_DB_PORT`,
      `PGEDGE_DB_NAME`, `PGEDGE_DB_USER`,
      `PGEDGE_DB_PASSWORD`
    - `PGEDGE_ANTHROPIC_API_KEY` or
      `PGEDGE_OPENAI_API_KEY`
    - `INIT_USERS=admin:password123`

    Start the containers:

    ```bash
    docker compose up -d
    ```

    Open `http://localhost:8081` in your browser and log in
    with the credentials you configured. Docker Compose
    maps the web client to port 8081 and the MCP API to
    port 8080.

=== "Build from Source"

    Build the server, create a user, and start:

    ```bash
    make build

    # Create a user account (runs and exits)
    ./bin/pgedge-postgres-mcp \
        -config ./bin/postgres-mcp.yaml \
        -add-user \
        -username admin -password secret123

    # Set permissions for the user file
    chmod 640 ./postgres-mcp-users.yaml

    # Start the server in HTTP mode
    ./bin/pgedge-postgres-mcp \
        -config ./bin/postgres-mcp.yaml \
        -http -addr :8080
    ```

    Verify the server is running:

    ```bash
    curl http://localhost:8080/health
    ```

    The standalone binary provides only the MCP API; there
    is no built-in web interface. Use the CLI client or
    Docker for the Web GUI.

**Verify the setup** by asking "What tables are in my
database?" using the CLI client or Docker web interface.

For full Web UI documentation, see the
[Web Client Guide](web-client.md).

---

## Claude Code

Claude Code connects to the MCP server using the stdio
transport. Create a `.mcp.json` file in your project root.

=== "pgEdge Packages"

    Create `.mcp.json` in your project directory with the
    following content:

    ```json
    {
      "mcpServers": {
        "pgedge": {
          "command": "/usr/bin/pgedge-postgres-mcp",
          "args": [
            "-config",
            "/etc/pgedge/postgres-mcp.yaml"
          ]
        }
      }
    }
    ```

=== "GitHub Release"

    Create `.mcp.json` in your project directory with the
    following content:

    ```json
    {
      "mcpServers": {
        "pgedge": {
          "command": "/path/to/pgedge-postgres-mcp",
          "args": [
            "-config",
            "/path/to/postgres-mcp.yaml"
          ]
        }
      }
    }
    ```

    Replace the paths with the absolute paths to the
    downloaded binary and your configuration file.

=== "Docker"

    The Docker image supports stdio mode by default.
    Use `docker` as the command and pass database
    connection details as environment variables:

    ```json
    {
      "mcpServers": {
        "pgedge": {
          "command": "docker",
          "args": [
            "run", "-i", "--rm",
            "--add-host", "host.docker.internal:host-gateway",
            "-e", "PGEDGE_DB_HOST=host.docker.internal",
            "-e", "PGEDGE_DB_PORT=5432",
            "-e", "PGEDGE_DB_NAME=mydb",
            "-e", "PGEDGE_DB_USER=myuser",
            "-e", "PGEDGE_DB_PASSWORD=mypass",
            "ghcr.io/pgedge/postgres-mcp:latest"
          ]
        }
      }
    }
    ```

    Replace the database connection values with your
    own. Use `host.docker.internal` to connect to a
    database running on the host machine.

=== "Build from Source"

    Build the server and create `.mcp.json` in your project
    directory:

    ```bash
    cd /path/to/pgedge-postgres-mcp
    make build
    ```

    ```json
    {
      "mcpServers": {
        "pgedge": {
          "command": "/path/to/pgedge-postgres-mcp/bin/pgedge-postgres-mcp",
          "args": [
            "-config",
            "/path/to/pgedge-postgres-mcp/bin/postgres-mcp.yaml"
          ]
        }
      }
    }
    ```

    Replace `/path/to/pgedge-postgres-mcp` with the
    absolute path to your cloned repository.

**Verify the setup** by asking Claude Code to list your
database tables. Claude Code detects the `.mcp.json` file
and starts the MCP server automatically.

---

## Claude Desktop

Claude Desktop connects to the MCP server using the stdio
transport. Edit the Claude Desktop configuration file to
add the server.

The configuration file location depends on your operating
system:

- **macOS:**
  `~/Library/Application Support/Claude/claude_desktop_config.json`
- **Linux:**
  `~/.config/Claude/claude_desktop_config.json`
- **Windows:**
  `%APPDATA%\Claude\claude_desktop_config.json`

=== "pgEdge Packages"

    Add the following entry to the `mcpServers` property in
    the configuration file:

    ```json
    {
      "mcpServers": {
        "pgedge": {
          "command": "/usr/bin/pgedge-postgres-mcp",
          "args": [
            "-config",
            "/etc/pgedge/postgres-mcp.yaml"
          ]
        }
      }
    }
    ```

=== "GitHub Release"

    Add the following entry to the `mcpServers` property in
    the configuration file:

    ```json
    {
      "mcpServers": {
        "pgedge": {
          "command": "/path/to/pgedge-postgres-mcp",
          "args": [
            "-config",
            "/path/to/postgres-mcp.yaml"
          ]
        }
      }
    }
    ```

    Replace the paths with the absolute paths to the
    downloaded binary and your configuration file.

=== "Docker"

    The Docker image supports stdio mode by default.
    Use `docker` as the command and pass database
    connection details as environment variables:

    ```json
    {
      "mcpServers": {
        "pgedge": {
          "command": "docker",
          "args": [
            "run", "-i", "--rm",
            "--add-host", "host.docker.internal:host-gateway",
            "-e", "PGEDGE_DB_HOST=host.docker.internal",
            "-e", "PGEDGE_DB_PORT=5432",
            "-e", "PGEDGE_DB_NAME=mydb",
            "-e", "PGEDGE_DB_USER=myuser",
            "-e", "PGEDGE_DB_PASSWORD=mypass",
            "ghcr.io/pgedge/postgres-mcp:latest"
          ]
        }
      }
    }
    ```

    Replace the database connection values with your
    own. Use `host.docker.internal` to connect to a
    database running on the host machine.

=== "Build from Source"

    Build the server binary first:

    ```bash
    cd /path/to/pgedge-postgres-mcp
    make build
    ```

    Add the following entry to the `mcpServers` property:

    ```json
    {
      "mcpServers": {
        "pgedge": {
          "command": "/path/to/pgedge-postgres-mcp/bin/pgedge-postgres-mcp",
          "args": [
            "-config",
            "/path/to/pgedge-postgres-mcp/bin/postgres-mcp.yaml"
          ]
        }
      }
    }
    ```

Restart Claude Desktop after saving the configuration file.

**Verify the setup** by asking Claude "What tables are in
my database?" The MCP server tools should appear in the
Claude Desktop tool list.

For full configuration options, see the
[Claude Desktop Guide](claude_desktop.md).

---

## Cursor

Cursor connects to the MCP server using the stdio
transport. Edit the Cursor MCP configuration file to add
the server.

The configuration file is located at `~/.cursor/mcp.json`.

=== "pgEdge Packages"

    Add the following entry to the `mcpServers` property:

    ```json
    {
      "mcpServers": {
        "pgedge": {
          "command": "/usr/bin/pgedge-postgres-mcp",
          "args": [
            "-config",
            "/etc/pgedge/postgres-mcp.yaml"
          ]
        }
      }
    }
    ```

=== "GitHub Release"

    Add the following entry to the `mcpServers` property:

    ```json
    {
      "mcpServers": {
        "pgedge": {
          "command": "/path/to/pgedge-postgres-mcp",
          "args": [
            "-config",
            "/path/to/postgres-mcp.yaml"
          ]
        }
      }
    }
    ```

    Replace the paths with the absolute paths to the
    downloaded binary and your configuration file.

=== "Docker"

    The Docker image supports stdio mode by default.
    Use `docker` as the command and pass database
    connection details as environment variables:

    ```json
    {
      "mcpServers": {
        "pgedge": {
          "command": "docker",
          "args": [
            "run", "-i", "--rm",
            "--add-host", "host.docker.internal:host-gateway",
            "-e", "PGEDGE_DB_HOST=host.docker.internal",
            "-e", "PGEDGE_DB_PORT=5432",
            "-e", "PGEDGE_DB_NAME=mydb",
            "-e", "PGEDGE_DB_USER=myuser",
            "-e", "PGEDGE_DB_PASSWORD=mypass",
            "ghcr.io/pgedge/postgres-mcp:latest"
          ]
        }
      }
    }
    ```

    Replace the database connection values with your
    own. Use `host.docker.internal` to connect to a
    database running on the host machine.

=== "Build from Source"

    Build the server binary first:

    ```bash
    cd /path/to/pgedge-postgres-mcp
    make build
    ```

    Add the following entry to the `mcpServers` property:

    ```json
    {
      "mcpServers": {
        "pgedge": {
          "command": "/path/to/pgedge-postgres-mcp/bin/pgedge-postgres-mcp",
          "args": [
            "-config",
            "/path/to/pgedge-postgres-mcp/bin/postgres-mcp.yaml"
          ]
        }
      }
    }
    ```

Restart Cursor after saving the configuration file.

**Verify the setup** by asking Cursor to list your
database tables. The pgEdge MCP tools should appear in the
available tools list.

---

## Windsurf

Windsurf connects to the MCP server using the stdio
transport. Edit the Windsurf MCP configuration file to add
the server.

The configuration file is located at
`~/.codeium/windsurf/mcp_config.json`.

=== "pgEdge Packages"

    Add the following entry to the `mcpServers` property:

    ```json
    {
      "mcpServers": {
        "pgedge": {
          "command": "/usr/bin/pgedge-postgres-mcp",
          "args": [
            "-config",
            "/etc/pgedge/postgres-mcp.yaml"
          ]
        }
      }
    }
    ```

=== "GitHub Release"

    Add the following entry to the `mcpServers` property:

    ```json
    {
      "mcpServers": {
        "pgedge": {
          "command": "/path/to/pgedge-postgres-mcp",
          "args": [
            "-config",
            "/path/to/postgres-mcp.yaml"
          ]
        }
      }
    }
    ```

    Replace the paths with the absolute paths to the
    downloaded binary and your configuration file.

=== "Docker"

    The Docker image supports stdio mode by default.
    Use `docker` as the command and pass database
    connection details as environment variables:

    ```json
    {
      "mcpServers": {
        "pgedge": {
          "command": "docker",
          "args": [
            "run", "-i", "--rm",
            "--add-host", "host.docker.internal:host-gateway",
            "-e", "PGEDGE_DB_HOST=host.docker.internal",
            "-e", "PGEDGE_DB_PORT=5432",
            "-e", "PGEDGE_DB_NAME=mydb",
            "-e", "PGEDGE_DB_USER=myuser",
            "-e", "PGEDGE_DB_PASSWORD=mypass",
            "ghcr.io/pgedge/postgres-mcp:latest"
          ]
        }
      }
    }
    ```

    Replace the database connection values with your
    own. Use `host.docker.internal` to connect to a
    database running on the host machine.

=== "Build from Source"

    Build the server binary first:

    ```bash
    cd /path/to/pgedge-postgres-mcp
    make build
    ```

    Add the following entry to the `mcpServers` property:

    ```json
    {
      "mcpServers": {
        "pgedge": {
          "command": "/path/to/pgedge-postgres-mcp/bin/pgedge-postgres-mcp",
          "args": [
            "-config",
            "/path/to/pgedge-postgres-mcp/bin/postgres-mcp.yaml"
          ]
        }
      }
    }
    ```

Restart Windsurf after saving the configuration file.

**Verify the setup** by asking Windsurf to list your
database tables.

---

## VS Code Copilot

VS Code with GitHub Copilot connects to the MCP server
using the stdio transport. Create a `.vscode/mcp.json`
file in your project root.

!!! note

    VS Code uses the `servers` key instead of
    `mcpServers`.

=== "pgEdge Packages"

    Create `.vscode/mcp.json` in your project directory:

    ```json
    {
      "servers": {
        "pgedge": {
          "command": "/usr/bin/pgedge-postgres-mcp",
          "args": [
            "-config",
            "/etc/pgedge/postgres-mcp.yaml"
          ]
        }
      }
    }
    ```

=== "GitHub Release"

    Create `.vscode/mcp.json` in your project directory:

    ```json
    {
      "servers": {
        "pgedge": {
          "command": "/path/to/pgedge-postgres-mcp",
          "args": [
            "-config",
            "/path/to/postgres-mcp.yaml"
          ]
        }
      }
    }
    ```

    Replace the paths with the absolute paths to the
    downloaded binary and your configuration file.

=== "Docker"

    The Docker image supports stdio mode by default.
    Use `docker` as the command and pass database
    connection details as environment variables:

    ```json
    {
      "servers": {
        "pgedge": {
          "command": "docker",
          "args": [
            "run", "-i", "--rm",
            "--add-host", "host.docker.internal:host-gateway",
            "-e", "PGEDGE_DB_HOST=host.docker.internal",
            "-e", "PGEDGE_DB_PORT=5432",
            "-e", "PGEDGE_DB_NAME=mydb",
            "-e", "PGEDGE_DB_USER=myuser",
            "-e", "PGEDGE_DB_PASSWORD=mypass",
            "ghcr.io/pgedge/postgres-mcp:latest"
          ]
        }
      }
    }
    ```

    Replace the database connection values with your
    own. Use `host.docker.internal` to connect to a
    database running on the host machine.

=== "Build from Source"

    Build the server binary first:

    ```bash
    cd /path/to/pgedge-postgres-mcp
    make build
    ```

    Create `.vscode/mcp.json` in your project directory:

    ```json
    {
      "servers": {
        "pgedge": {
          "command": "/path/to/pgedge-postgres-mcp/bin/pgedge-postgres-mcp",
          "args": [
            "-config",
            "/path/to/pgedge-postgres-mcp/bin/postgres-mcp.yaml"
          ]
        }
      }
    }
    ```

Restart VS Code after saving the configuration file.

**Verify the setup** by asking Copilot to list your
database tables. The pgEdge MCP tools should appear in the
Copilot agent tools list.

---

## Next Steps

After verifying your setup, explore the following
resources:

- [Configuration Guide](configuration.md) - All
  configuration options and advanced settings.
- [Authentication Guide](authentication.md) - Set up
  users, tokens, and access control.
- [Security Checklist](security.md) - Best practices for
  production deployments.
- [Tools Reference](../reference/tools.md) - Available
  MCP tools for database interaction.
- [Troubleshooting](troubleshooting.md) - Solutions for
  common issues.
