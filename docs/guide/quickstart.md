# pgEdge MCP Server and AI Toolkit Quickstart

In this Quickstart, we'll walk you through getting started with the MCP server. This guide creates a:

- **PostgreSQL 17** - pgEdge PostgreSQL distribution
- **Northwind Dataset** - Classic demo database with orders, customers, products
- **pgEdge MCP Server** - Natural language interface to your database
- **pgEdge Web UI** - Modern chat interface for querying with natural language
- **Pre-configured** - Demo credentials work out of the box

The Northwind database is a classic SQL Server sample database containing:

- **13 Tables**: `Categories`, `Customers`, `Employees`, `Orders`, `Products`, `Shippers`, `Suppliers`, etc.
- **~1000 Rows**: Realistic business data for testing and demos
- **1 Schema**: `northwind` (keeps your `public` schema clean)

The dataset is perfect for testing natural language queries, joins, aggregations, and analytics, and is installed with the Quickstart.


## Prerequisites

Before running the deployment steps:

- Install Docker Desktop
- Obtain an LLM API key (Anthropic or OpenAI)

After meeting the prerequisites, you're ready to install the AI Toolkit.  There are two installation options:

* a [One-Step Quickstart](#one-step-quickstart) that uses default settings to launch the demo.
* a [Three-Step Quickstart](#three-step-quickstart) that allows you to configure settings before launching the demo.


## One-Step QuickStart

The single command option is the fastest way to get started.  Execute the following command:

`/bin/sh -c "$(curl -fsSL https://downloads.pgedge.com/quickstart/mcp/pgedge-ait-demo.sh)`

This command will:

- Download `docker-compose.yml` and `.env.example` from the same location.
- Prompt you for your API key(s) securely.
- Start all services automatically.
- Display connection details when ready.

!!! note 

    The installer creates a temporary workspace in `/tmp` and runs the demo from that location.

Sample output from running the `demo` script:

```bash
$ /bin/sh -c "$(curl -fsSL https://downloads.pgedge.com/quickstart/mcp/pgedge-ait-demo.sh)"
ℹ  Creating workspace: /tmp/pgedge-download.28085
ℹ  Downloading files
ℹ  → docker-compose.yml
ℹ  → .env.example
✓  Downloads complete

pgEdge AI Toolkit Demo setup
You need to specify an API key for Anthropic or OpenAI (or both)

Anthropic API key
(Leave blank to skip)
›
OpenAI API key
(Leave blank to skip)
›
✓  Wrote .env
ℹ  Starting services
[+] Running 22/22
 ✔ web-client Pulled                                                                                                                                                                                          15.4s
 ✔ postgres Pulled                                                                                                                                                                                            17.5s
 ✔ postgres-mcp Pulled                                                                                                                                                                                         8.7s
[+] Running 6/6
 ✔ Network pgedge-download28085_pgedge-quickstart  Created                                                                                                                                                     0.0s
 ✔ Volume pgedge-download28085_postgres-data       Created                                                                                                                                                     0.0s
 ✔ Volume pgedge-download28085_mcp-data            Created                                                                                                                                                     0.0s
 ✔ Container pgedge-quickstart-db                  Healthy                                                                                                                                                     6.4s
 ✔ Container pgedge-quickstart-mcp                 Healthy                                                                                                                                                    11.2s
 ✔ Container pgedge-quickstart-web                 Started                                                                                                                                                    11.3s
ℹ  Waiting for services to be healthy (this may take up to 60 seconds)...
✓  Services are ready

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  pgEdge AI Toolkit Demo is running!
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Web Client Interface:
  http://localhost:8081
  Login: demo / demo123

PostgreSQL Database:
  Host: localhost:5432
  Database: northwind
  User: demo / demo123
  Connect: PGPASSWORD=demo123 psql -h localhost -p 5432 -U demo -d northwind

MCP Server API:
  http://localhost:8080
  Bearer Token: demo-token-12345

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Workspace: /tmp/pgedge-download.28085
To stop: cd /tmp/pgedge-download.28085 && docker compose down -v

For more information: https://github.com/pgEdge/pgedge-nla
```

Then, navigate to the address of the MCP Server (`http://localhost:8081`) and use these queries to test the server:

- `What tables are in the database?`
- `Show me the top 10 products by sales`
- `Which customers have placed more than 5 orders?`
- `Analyze order trends by month`


## Three-Step Quickstart

For a more traditional setup, you can:

1. Make a working directory:

```bash
mkdir ~/pgEdge-ait-demo
~/pgEdge-ait-demo
```

2. Download the demo artifacts:

```bash
curl -fsSLO https://downloads.pgedge.com/quickstart/mcp/docker-compose.yml
curl -fsSLO https://downloads.pgedge.com/quickstart/mcp/.env.example
```

3. Configure your API key

```bash
cp .env.example .env
```

Then, edit `.env` and add `PGEDGE_ANTHROPIC_API_KEY` and/or `PGEDGE_OPENAI_API_KEY`.

4. Use the following command to start the Docker container.

```bash
docker compose up
```

During deployment:

    1. PostgreSQL starts and downloads the Northwind dataset (~230KB)
    2. The Northwind dataset loads (13 tables, ~1000 rows)
    3. The MCP Server connects and analyzes your schema
    4. The Web UI starts and connects to MCP Server

Once all services are healthy, you can access them as follows (~60 seconds):

```bash
Web Client Interface:
  http://localhost:8081
  Login: demo / demo123

PostgreSQL Database:
  Host: localhost:5432
  Database: northwind
  User: demo / demo123
  Connect: PGPASSWORD=demo123 psql -h localhost -p 5432 -U demo -d northwind

MCP Server API:
  http://localhost:8080
  Bearer Token: demo-token-12345
```

Then, you can navigate to the address of the MCP Server (`http://localhost:8080`) and use these queries to test the server:

- `What tables are in the database?`
- `Show me the top 10 products by sales`
- `Which customers have placed more than 5 orders?`
- `Analyze order trends by month`


## Managing the Service and Reviewing Log Files

Use the following commands to stop the server:

Stop (retains data):

`docker compose down`

Stop and remove volumes (creating a fresh start):

`docker compose down -v`

Use the following command to view the log files for all services

`docker compose logs -f`

Or review the log file for a specific service:

```bash
docker compose logs -f postgres
docker compose logs -f postgres-mcp
docker compose logs -f web-client
bash
