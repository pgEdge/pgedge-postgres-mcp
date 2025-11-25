# pgEdge Natural Language Agent - Packaging Guide

This document provides guidance for creating native OS packages (RPM/DEB) for the
pgEdge Natural Language Agent. The project produces four separate packages:

1. **pgedge-nla-server** - MCP server with HTTP/HTTPS API
2. **pgedge-nla-cli** - Command-line chat client
3. **pgedge-nla-web** - Web UI with React frontend
4. **pgedge-nla-kb** - Pre-built knowledgebase database (optional)

## Prerequisites for Building

- Go 1.23 or higher
- Node.js 18 or higher (for Web UI)
- make
- git

## Build Process

### 1. MCP Server Package (`pgedge-nla-server`)

**Build Command:**
```bash
cd /path/to/pgedge-postgres-mcp
make clean-server
make build-server
```

**Binary Location:**
```
bin/pgedge-nla-server
```

**Files to Include:**

```
/usr/bin/pgedge-nla-server                  # Main server binary
/etc/pgedge/nla-server.yaml                 # Default configuration
/etc/pgedge/nla-server.env                  # Environment variables template
/usr/lib/systemd/system/pgedge-nla-server.service  # Systemd unit
/usr/share/doc/pgedge-nla-server/README.md  # Documentation
/usr/share/doc/pgedge-nla-server/LICENSE    # License file
/var/lib/pgedge/nla-server/                 # Data directory (create empty)
/var/log/pgedge/nla-server/                 # Log directory (create empty)
```

**Default Configuration File** (`/etc/pgedge/nla-server.yaml`):
```yaml
# pgEdge Natural Language Agent - Server Configuration
# See: https://github.com/pgEdge/pgedge-postgres-mcp/blob/main/docs/configuration.md

# Database connection settings
database:
  host: localhost
  port: 5432
  database: postgres
  user: postgres
  # Password can be set via PGEDGE_POSTGRES_PASSWORD environment variable
  sslmode: prefer

# HTTP server settings
http:
  enabled: true
  address: "localhost:8080"

  # TLS/HTTPS configuration (optional)
  tls:
    enabled: false
    # cert_file: /etc/pgedge/certs/server.crt
    # key_file: /etc/pgedge/certs/server.key

  # Authentication settings
  auth:
    enabled: true
    token_file: /var/lib/pgedge/nla-server/tokens.json
    user_file: /var/lib/pgedge/nla-server/users.json
    max_failed_attempts_before_lockout: 5  # Lock account after N failed attempts (0 = disabled)
    rate_limit_window_minutes: 15  # Time window for rate limiting
    rate_limit_max_attempts: 10  # Max failed attempts per IP per window

# LLM proxy settings (optional - for web UI)
llm:
  enabled: false
  # provider: anthropic  # anthropic, openai, or ollama
  # Set API keys via environment variables:
  # ANTHROPIC_API_KEY or OPENAI_API_KEY

# Knowledgebase database (for similarity_search tool)
# Requires pgedge-nla-kb package
knowledgebase:
  enabled: true
  database_path: /usr/share/pgedge/nla-kb/kb.db

# Custom prompts and resources (optional)
# custom_definitions_path: /etc/pgedge/custom-definitions.yaml
```

**Environment Variables File** (`/etc/pgedge/nla-server.env`):

This file can be used to set sensitive configuration that shouldn't be in the main
YAML config file. It's loaded by systemd via `EnvironmentFile=`.

```bash
# pgEdge Natural Language Agent - Environment Variables
# This file is sourced by systemd service

# Database password (if not using peer authentication)
# PGEDGE_POSTGRES_PASSWORD=your-secure-password

# LLM API Keys (for web UI LLM proxy)
# ANTHROPIC_API_KEY=your-anthropic-api-key
# OPENAI_API_KEY=your-openai-api-key

# Alternative: Full database connection string
# PGEDGE_POSTGRES_CONNECTION_STRING=postgres://user:password@host:port/database?sslmode=prefer

# Logging level (optional)
# PGEDGE_MCP_LOG_LEVEL=info
```

**Systemd Unit File** (`/usr/lib/systemd/system/pgedge-nla-server.service`):
```ini
[Unit]
Description=pgEdge Natural Language Agent - MCP Server
Documentation=https://github.com/pgEdge/pgedge-postgres-mcp
After=network.target postgresql.service
Wants=postgresql.service

[Service]
Type=simple
User=pgedge
Group=pgedge
WorkingDirectory=/var/lib/pgedge/nla-server

# Main executable
ExecStart=/usr/bin/pgedge-nla-server -config /etc/pgedge/nla-server.yaml

# Environment
Environment="PGEDGE_POSTGRES_CONNECTION_STRING=postgres://postgres@localhost/postgres?sslmode=prefer"
EnvironmentFile=-/etc/pgedge/nla-server.env

# Security settings
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/pgedge/nla-server /var/log/pgedge/nla-server
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
RestrictRealtime=true
RestrictNamespaces=true
RestrictSUIDSGID=true
LockPersonality=true

# Resource limits
LimitNOFILE=65536
LimitNPROC=4096

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=pgedge-nla-server

# Restart policy
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
```

**Post-Install Script Actions:**
```bash
# Create pgedge user/group if not exists
if ! getent group pgedge >/dev/null; then
    groupadd -r pgedge
fi
if ! getent passwd pgedge >/dev/null; then
    useradd -r -g pgedge -d /var/lib/pgedge -s /sbin/nologin -c "pgEdge Services" pgedge
fi

# Set ownership
chown -R pgedge:pgedge /var/lib/pgedge/nla-server
chown -R pgedge:pgedge /var/log/pgedge/nla-server
chmod 750 /var/lib/pgedge/nla-server
chmod 750 /var/log/pgedge/nla-server

# Set binary permissions
chmod 755 /usr/bin/pgedge-nla-server

# Reload systemd
systemctl daemon-reload
```

**DO NOT INCLUDE:**
- Test files (`*_test.go`)
- Development configs (`.env`, `.env.*`)
- Build artifacts (`*.o`, `*.a`)
- Git metadata (`.git/`, `.gitignore`)
- CI/CD configs (`.github/`)
- Documentation source (`docs/`, `mkdocs.yml`)

---

### 2. CLI Client Package (`pgedge-nla-cli`)

**Build Command:**
```bash
cd /path/to/pgedge-postgres-mcp
make clean-client
make build-client
```

**Binary Location:**
```
bin/pgedge-nla-cli
```

**Files to Include:**

```
/usr/bin/pgedge-nla-cli                  # Main CLI binary
/etc/pgedge/nla-cli.yaml                    # Default configuration (optional)
/usr/share/doc/pgedge-nla-cli/README.md     # Documentation
/usr/share/doc/pgedge-nla-cli/LICENSE       # License file
/usr/share/man/man1/pgedge-pg-mcp-cli.1.gz  # Man page (if created)
```

**Default CLI Configuration** (`/etc/pgedge/nla-cli.yaml`):
```yaml
# pgEdge Natural Language Agent - CLI Client Configuration
# User-specific config: ~/.config/pgedge/cli-config.yaml

# MCP server connection
mcp_server:
  url: "http://localhost:8080/mcp/v1"
  # auth_token: ""  # Set via PGEDGE_MCP_TOKEN env var

# LLM provider settings
llm:
  provider: anthropic  # anthropic, openai, or ollama
  model: claude-sonnet-4-20250514
  # API keys set via environment:
  # ANTHROPIC_API_KEY or OPENAI_API_KEY

  # Optional: Ollama configuration
  # ollama:
  #   url: http://localhost:11434
  #   model: llama3

# UI preferences
ui:
  color: true
  markdown_rendering: true
  prompt_caching: true
```

**Post-Install Script Actions:**
```bash
# Set binary permissions
chmod 755 /usr/bin/pgedge-nla-cli

# Create config directory template
mkdir -p /etc/skel/.config/pgedge
chmod 755 /etc/skel/.config/pgedge
```

**DO NOT INCLUDE:**
- Test files
- Development dependencies
- Chat history files (`~/.config/pgedge/chat_*.json`)

---

### 3. Web UI Package (`pgedge-nla-web`)

**Build Commands:**
```bash
cd /path/to/pgedge-postgres-mcp/web

# Install dependencies (do NOT ship node_modules)
npm install

# Build production bundle
npm run build

# The production build will be in: web/dist/
```

**Production Build Location:**
```
web/dist/           # Contains minified HTML, CSS, JS, and assets
```

**Files to Include:**

```
/usr/share/pgedge/nla-web/                  # Web root (web/dist/ contents)
  ├── index.html
  ├── assets/
  │   ├── index-*.js                        # Minified JavaScript bundles
  │   ├── index-*.css                       # Minified CSS
  │   └── *.woff2                           # Font files
  └── vite.svg                              # Favicon

/etc/nginx/sites-available/pgedge-nla-web   # Nginx config
/usr/lib/systemd/system/pgedge-nla-web.service  # Systemd unit (if using standalone server)
/usr/share/doc/pgedge-nla-web/README.md     # Documentation
/usr/share/doc/pgedge-nla-web/LICENSE       # License file
```

**Nginx Configuration** (`/etc/nginx/sites-available/pgedge-nla-web`):
```nginx
# pgEdge Natural Language Agent - Web UI Nginx Configuration
#
# Enable with: ln -s /etc/nginx/sites-available/pgedge-nla-web /etc/nginx/sites-enabled/
# Then: systemctl reload nginx

server {
    listen 80;
    server_name nla.example.com;

    # Redirect HTTP to HTTPS (recommended for production)
    # return 301 https://$server_name$request_uri;

    # Or serve directly over HTTP (development only)
    root /usr/share/pgedge/nla-web;
    index index.html;

    # Gzip compression
    gzip on;
    gzip_types text/plain text/css application/json application/javascript text/xml application/xml application/xml+rss text/javascript;
    gzip_vary on;

    # Security headers
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;
    add_header Referrer-Policy "strict-origin-when-cross-origin" always;

    # React Router - serve index.html for all routes
    location / {
        try_files $uri $uri/ /index.html;
    }

    # Proxy API requests to MCP server
    location /api/ {
        proxy_pass http://localhost:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded_Proto $scheme;

        # Timeouts for long-running LLM requests
        proxy_connect_timeout 90s;
        proxy_send_timeout 90s;
        proxy_read_timeout 90s;
    }

    # Proxy MCP endpoint
    location /mcp/ {
        proxy_pass http://localhost:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded_Proto $scheme;

        # Timeouts for long-running queries
        proxy_connect_timeout 90s;
        proxy_send_timeout 90s;
        proxy_read_timeout 90s;
    }

    # Cache static assets
    location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg|woff|woff2|ttf|eot)$ {
        expires 1y;
        add_header Cache-Control "public, immutable";
    }
}

# HTTPS configuration (uncomment and configure for production)
# server {
#     listen 443 ssl http2;
#     server_name nla.example.com;
#
#     ssl_certificate /etc/ssl/certs/nla.example.com.crt;
#     ssl_certificate_key /etc/ssl/private/nla.example.com.key;
#
#     # Modern SSL configuration
#     ssl_protocols TLSv1.2 TLSv1.3;
#     ssl_ciphers HIGH:!aNULL:!MD5;
#     ssl_prefer_server_ciphers on;
#
#     # ... rest of config same as HTTP block above ...
# }
```

**Alternative: Standalone Systemd Unit** (if not using Nginx):

**Note:** This is only needed if the web UI will be served by its own HTTP server
instead of Nginx. For most deployments, use Nginx as shown above.

```ini
[Unit]
Description=pgEdge Natural Language Agent - Web UI Server
Documentation=https://github.com/pgEdge/pgedge-postgres-mcp
After=network.target pgedge-nla-server.service
Requires=pgedge-nla-server.service

[Service]
Type=simple
User=pgedge
Group=pgedge
WorkingDirectory=/usr/share/pgedge/nla-web

# Serve using a simple HTTP server (for development)
# For production, use Nginx configuration above
ExecStart=/usr/bin/python3 -m http.server 3000

# Environment
Environment="CONFIG_FILE=/etc/pgedge/nla-web-config.json"

# Security
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadOnlyPaths=/usr/share/pgedge/nla-web

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=pgedge-nla-web

# Restart policy
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
```

**Post-Install Script Actions:**
```bash
# Set ownership
chown -R root:root /usr/share/pgedge/nla-web
chmod 755 /usr/share/pgedge/nla-web
find /usr/share/pgedge/nla-web -type f -exec chmod 644 {} \;
find /usr/share/pgedge/nla-web -type d -exec chmod 755 {} \;

# Note: Do NOT enable/start the service automatically
# Users should configure Nginx or their preferred web server
echo "Web UI installed to /usr/share/pgedge/nla-web"
echo "Configure Nginx: ln -s /etc/nginx/sites-available/pgedge-nla-web /etc/nginx/sites-enabled/"
```

**DO NOT INCLUDE:**
- Source files (`web/src/`)
- Node modules (`web/node_modules/`)
- Development configs (`web/vite.config.ts`, `web/tsconfig.json`)
- Development server files
- `.env` files

---

### 4. Knowledgebase Database Package (`pgedge-nla-kb`)

**Build Commands:**
```bash
cd /path/to/pgedge-postgres-mcp

# Build the kb-builder tool
make build-kb-builder

# Generate the knowledgebase database
# This requires valid API keys for embedding providers (OpenAI, Voyage, AND Ollama)
# Note that Ollama may need to run on a g4dn.xlarge instance on AWS to keep up.
# The example config below should be maintained with the standard list of repos.
./bin/pgedge-nla-kb-builder -config examples/pgedge-nla-kb-builder.yaml

# The generated database will be: kb.db
```

**Database Location:**
```
kb.db  # SQLite database with embeddings
```

**Files to Include:**

```
/usr/share/pgedge/nla-kb/kb.db              # Pre-built knowledgebase database
/usr/share/doc/pgedge-nla-kb/README.md      # Documentation
/usr/share/doc/pgedge-nla-kb/LICENSE        # License file
/usr/share/doc/pgedge-nla-kb/VERSION        # Database version/build info
```

**VERSION File Content:**
```
PGEDGE_NLA_KB_VERSION=1.0.0
BUILD_DATE=2025-01-15
POSTGRES_DOCS_VERSION=17.0
PGEDGE_DOCS_VERSION=24.12
EMBEDDING_PROVIDER=openai
EMBEDDING_MODEL=text-embedding-3-small
TOTAL_CHUNKS=12543
```

**Post-Install Script Actions:**
```bash
# Set ownership and permissions
chown -R root:root /usr/share/pgedge/nla-kb
chmod 755 /usr/share/pgedge/nla-kb
chmod 644 /usr/share/pgedge/nla-kb/kb.db

# Verify database integrity (optional)
if command -v sqlite3 >/dev/null 2>&1; then
    sqlite3 /usr/share/pgedge/nla-kb/kb.db "PRAGMA integrity_check;" >/dev/null 2>&1 || {
        echo "Warning: Knowledgebase database may be corrupted"
    }
fi
```

**Package Notes:**

- This package is **optional** and only needed if using the `similarity_search` tool
- The database must be regenerated when source documentation is updated
- Requires setting `knowledgebase.enabled: true` in server config
- Database is read-only at runtime (no write operations)
- Can be updated independently of other packages

**DO NOT INCLUDE:**
- kb-builder binary (this is a build-time tool only)
- Source documentation files
- Build configuration files
- Temporary build artifacts

---

## Package Dependencies

### MCP Server (`pgedge-nla-server`)
**Runtime Dependencies:**
- libc (glibc or musl)
- systemd (for service management)

**Recommended:**
- postgresql-server (any version 11+)

### CLI Client (`pgedge-nla-cli`)
**Runtime Dependencies:**
- libc (glibc or musl)
- terminal emulator with UTF-8 support

### Web UI (`pgedge-nla-web`)
**Runtime Dependencies:**
- nginx (or any web server)
- pgedge-nla-server (for API backend)

### Knowledgebase (`pgedge-nla-kb`)
**Runtime Dependencies:**
- None (standalone SQLite database)

**Optional for:**
- pgedge-nla-server (enables similarity_search tool)

---

## Package Metadata

### Common Metadata
```
License: PostgreSQL License
Homepage: https://github.com/pgEdge/pgedge-postgres-mcp
Maintainer: pgEdge, Inc. <support@pgedge.com>
```

### Package Descriptions

**pgedge-nla-server:**
```
Summary: pgEdge Natural Language Agent - MCP Server
Description: Model Context Protocol (MCP) server that enables natural language
 queries against PostgreSQL databases. Provides HTTP/HTTPS API with read-only
 query execution, schema analysis, and semantic search capabilities.
```

**pgedge-nla-cli:**
```
Summary: pgEdge Natural Language Agent - CLI Client
Description: Command-line chat client for interacting with PostgreSQL databases
 using natural language. Connects to pgEdge MCP Server and supports multiple
 LLM providers (Anthropic Claude, OpenAI, Ollama).
```

**pgedge-nla-web:**
```
Summary: pgEdge Natural Language Agent - Web UI
Description: Modern React-based web interface for the pgEdge Natural Language
 Agent. Provides an intuitive UI for natural language database queries,
 monitoring, and administration.
```

**pgedge-nla-kb:**
```
Summary: pgEdge Natural Language Agent - Knowledgebase Database
Description: Pre-built knowledgebase database containing embeddings for
 PostgreSQL and pgEdge documentation. Enables semantic search capabilities
 in the Natural Language Agent server. Optional package updated independently.
```

---

## File Permissions Reference

```
# Executables
/usr/bin/*                          0755 root:root

# Configuration files
/etc/pgedge/*.yaml                  0644 root:root
/etc/nginx/sites-available/*        0644 root:root

# Systemd units
/usr/lib/systemd/system/*           0644 root:root

# Data directories
/var/lib/pgedge/nla-server/         0750 pgedge:pgedge

# Knowledgebase
/usr/share/pgedge/nla-kb/           0755 root:root
/usr/share/pgedge/nla-kb/kb.db      0644 root:root

# Log directories
/var/log/pgedge/nla-server/         0750 pgedge:pgedge

# Web files
/usr/share/pgedge/nla-web/          0755 root:root
/usr/share/pgedge/nla-web/*         0644 root:root (files)
/usr/share/pgedge/nla-web/*/        0755 root:root (dirs)

# Documentation
/usr/share/doc/*/                   0755 root:root
/usr/share/doc/*/*                  0644 root:root
```

---

## Production Build Verification

Before packaging, verify builds are production-ready:

```bash
# 1. Verify binaries are stripped and optimized
file bin/pgedge-nla-server
# Should show: "ELF 64-bit LSB executable ... stripped"

# 2. Verify web build is minified
ls -lh web/dist/assets/
# JS files should be small (minified and gzipped)

# 3. Check for debug symbols (should be removed)
nm bin/pgedge-nla-server | grep -i debug
# Should return nothing

# 4. Verify no test files in distribution
find bin/ -name "*test*"
# Should return nothing
```

---

## Installation Layout Summary

```
/etc/pgedge/
├── nla-server.yaml                 # Server config
├── nla-server.env                  # Server environment (optional)
├── nla-cli.yaml                    # CLI config (optional)
└── custom-definitions.yaml         # Custom prompts/resources (optional)

/usr/bin/
├── pgedge-pg-mcp-svr              # Server binary
└── pgedge-pg-mcp-cli              # CLI binary

/usr/lib/systemd/system/
├── pgedge-nla-server.service      # Server systemd unit
└── pgedge-nla-web.service         # Web UI systemd unit (optional)

/usr/share/pgedge/
├── nla-kb/                        # Knowledgebase (optional package)
│   └── kb.db                      # Pre-built knowledgebase database
└── nla-web/                       # Web UI files
    ├── index.html
    └── assets/

/etc/nginx/sites-available/
└── pgedge-nla-web                 # Nginx config

/var/lib/pgedge/
└── nla-server/                    # Server data/state
    ├── tokens.json                # Created by admin
    └── users.json                 # Created by admin

/var/log/pgedge/
└── nla-server/                    # Server logs (if file logging enabled)

/usr/share/doc/
├── pgedge-nla-server/
│   ├── README.md
│   └── LICENSE
├── pgedge-nla-cli/
│   ├── README.md
│   └── LICENSE
├── pgedge-nla-web/
│   ├── README.md
│   └── LICENSE
└── pgedge-nla-kb/
    ├── README.md
    ├── LICENSE
    └── VERSION
```

---

## Quick Start Commands (Post-Install)

Include these in package documentation:

### 1. Server Setup
```bash
# Create initial admin token
sudo -u pgedge pgedge-pg-mcp-svr -add-token -token-note "Admin token"

# Configure database connection
sudo vim /etc/pgedge/nla-server.yaml

# Start service
sudo systemctl enable pgedge-nla-server
sudo systemctl start pgedge-nla-server
sudo systemctl status pgedge-nla-server
```

### 2. Web UI Setup (with Nginx)
```bash
# Enable Nginx site
sudo ln -s /etc/nginx/sites-available/pgedge-nla-web /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl reload nginx

# Access at: http://localhost/
```

### 3. CLI Usage
```bash
# Set API token
export PGEDGE_MCP_TOKEN="your-token-here"

# Start chat
pgedge-pg-mcp-cli
```

---

## Notes for Packagers

1. **Static Binaries**: All Go binaries are statically linked and have no external
   library dependencies beyond libc.

2. **Web Assets**: The web UI must be built during package creation. DO NOT ship
   source files or node_modules.

3. **Service User**: All services run as the `pgedge` user for security. Create
   this user during package installation.

4. **Default Configs**: Ship sane defaults that work out-of-box for testing,
   but require configuration for production (especially auth tokens).

5. **Documentation**: Include links to online documentation in package
   descriptions and README files.

6. **Systemd Integration**: Enable but don't start services by default. Users
   should configure first.

7. **Logging**: Default to systemd journal. File logging is optional and
   configured in config files.

8. **Upgrades**: Preserve configuration files during upgrades. Mark config
   files as `%config(noreplace)` in RPM or as conffiles in DEB.

9. **SELinux/AppArmor**: See security configuration sections below for
   required policy modules and profiles.

---

## SELinux Configuration (RHEL/CentOS/Fedora)

For systems with SELinux enabled, you need to provide a custom policy module to allow
the MCP server to function properly.

**SELinux Policy Module** (`pgedge-nla-server.te`):

```te
policy_module(pgedge-nla-server, 1.0.0)

require {
    type unconfined_t;
    type unreserved_port_t;
    type postgresql_port_t;
    type postgresql_t;
    type postgresql_exec_t;
    type init_t;
    type syslogd_t;
    type syslogd_var_run_t;
    class tcp_socket { name_bind name_connect };
    class process { setrlimit };
    class capability { setgid setuid };
    class file { read write open getattr };
    class sock_file { write };
    class unix_stream_socket { connectto };
}

# Define custom type for MCP server
type pgedge_mcp_t;
type pgedge_mcp_exec_t;
init_daemon_domain(pgedge_mcp_t, pgedge_mcp_exec_t)

# Allow binding to HTTP ports
allow pgedge_mcp_t self:tcp_socket { name_bind name_connect };
allow pgedge_mcp_t unreserved_port_t:tcp_socket { name_bind };

# Allow connecting to PostgreSQL
allow pgedge_mcp_t postgresql_port_t:tcp_socket { name_connect };
allow pgedge_mcp_t postgresql_t:unix_stream_socket { connectto };

# Allow reading/writing to data directories
files_read_etc_files(pgedge_mcp_t)
files_search_var_lib(pgedge_mcp_t)
allow pgedge_mcp_t var_lib_t:dir { search read write add_name remove_name };
allow pgedge_mcp_t var_lib_t:file { create read write open getattr unlink };

# Allow logging
logging_send_syslog_msg(pgedge_mcp_t)
allow pgedge_mcp_t syslogd_var_run_t:sock_file { write };
allow pgedge_mcp_t syslogd_t:unix_stream_socket { connectto };

# Allow reading knowledgebase database
files_read_usr_files(pgedge_mcp_t)

# Allow setuid/setgid (for systemd User= directive)
allow pgedge_mcp_t self:capability { setgid setuid };
allow pgedge_mcp_t self:process { setrlimit };
```

**Build and Install SELinux Module:**

```bash
# Compile the policy
checkmodule -M -m -o pgedge-nla-server.mod pgedge-nla-server.te
semodule_package -o pgedge-nla-server.pp -m pgedge-nla-server.mod

# Install the module
semodule -i pgedge-nla-server.pp

# Label the binary
semanage fcontext -a -t pgedge_mcp_exec_t '/usr/bin/pgedge-nla-server'
restorecon -v /usr/bin/pgedge-nla-server

# Label data directories
semanage fcontext -a -t var_lib_t '/var/lib/pgedge/nla-server(/.*)?'
restorecon -Rv /var/lib/pgedge/nla-server
```

**Post-Install Actions** (add to package post-install script):

```bash
# Install SELinux policy if SELinux is enabled
if [ -x /usr/sbin/selinuxenabled ] && /usr/sbin/selinuxenabled; then
    # Install policy module
    /usr/sbin/semodule -i /usr/share/selinux/packages/pgedge-nla-server.pp 2>/dev/null || true

    # Set file contexts
    /usr/sbin/semanage fcontext -a -t pgedge_mcp_exec_t '/usr/bin/pgedge-nla-server' 2>/dev/null || true
    /usr/sbin/restorecon -v /usr/bin/pgedge-nla-server 2>/dev/null || true

    /usr/sbin/semanage fcontext -a -t var_lib_t '/var/lib/pgedge/nla-server(/.*)?'  2>/dev/null || true
    /usr/sbin/restorecon -Rv /var/lib/pgedge/nla-server 2>/dev/null || true
fi
```

**Include in Package:**
```
/usr/share/selinux/packages/pgedge-nla-server.pp
/usr/share/selinux/devel/include/contrib/pgedge-nla-server.if
```

---

## AppArmor Configuration (Ubuntu/Debian)

For systems with AppArmor enabled, provide a profile to confine the MCP server.

**AppArmor Profile** (`/etc/apparmor.d/usr.bin.pgedge-pg-mcp-svr`):

```apparmor
#include <tunables/global>

/usr/bin/pgedge-nla-server {
  #include <abstractions/base>
  #include <abstractions/nameservice>
  #include <abstractions/openssl>

  # Binary execution
  /usr/bin/pgedge-nla-server mr,

  # Configuration files
  /etc/pgedge/** r,

  # Data directories
  /var/lib/pgedge/nla-server/** rw,
  /usr/share/pgedge/nla-server/** r,

  # Log directories
  /var/log/pgedge/nla-server/** rw,

  # Temporary files
  /tmp/** rw,
  owner /tmp/** rw,

  # Network access
  network inet stream,
  network inet6 stream,
  network inet dgram,
  network inet6 dgram,

  # PostgreSQL socket
  /var/run/postgresql/.s.PGSQL.* rw,
  /run/postgresql/.s.PGSQL.* rw,

  # System files
  /proc/sys/kernel/random/boot_id r,
  /sys/kernel/mm/transparent_hugepage/hpage_pmd_size r,
  /proc/*/stat r,
  /proc/*/status r,

  # Required for Go runtime
  /proc/sys/net/core/somaxconn r,
  @{PROC}/sys/vm/overcommit_memory r,

  # Systemd journal
  /run/systemd/journal/socket w,
  /run/systemd/journal/stdout rw,

  # Capability restrictions
  capability setuid,
  capability setgid,
  capability net_bind_service,

  # Deny sensitive operations
  deny /proc/kcore r,
  deny /boot/** r,
  deny /sys/firmware/** r,
  deny ptrace,
  deny mount,
  deny umount,

  # Signal handling
  signal (receive) set=(term, kill) peer=unconfined,
}
```

**Install AppArmor Profile** (add to package post-install script):

```bash
# Install AppArmor profile if AppArmor is enabled
if [ -x /sbin/apparmor_parser ] && [ -d /etc/apparmor.d ]; then
    # Load the profile
    /sbin/apparmor_parser -r /etc/apparmor.d/usr.bin.pgedge-pg-mcp-svr 2>/dev/null || true

    # Enable the profile on boot
    if [ -d /etc/apparmor.d/force-complain ]; then
        ln -sf /etc/apparmor.d/usr.bin.pgedge-pg-mcp-svr \
               /etc/apparmor.d/force-complain/ 2>/dev/null || true
    fi
fi
```

**Test AppArmor Profile:**

```bash
# Load in complain mode first (logs violations but doesn't block)
aa-complain /usr/bin/pgedge-nla-server

# Test the service
systemctl start pgedge-nla-server
systemctl status pgedge-nla-server

# Check for violations
aa-logprof

# Once satisfied, switch to enforce mode
aa-enforce /usr/bin/pgedge-nla-server
```

**Include in Package:**
```
/etc/apparmor.d/usr.bin.pgedge-pg-mcp-svr
```

**Package Dependencies** (for Ubuntu/Debian):
```
Suggests: apparmor-utils
```
