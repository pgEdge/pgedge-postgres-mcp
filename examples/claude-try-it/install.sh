#!/bin/bash

#--------------------------------------------------------------------------
#
# pgEdge MCP Server — One-Command Installer
#
# Copyright (c) 2025 - 2026, pgEdge, Inc.
# This software is released under The PostgreSQL License
#
#--------------------------------------------------------------------------

# pgEdge MCP Server — one-command installer
#
# Usage (interactive, in a terminal):
#   curl -fsSL https://raw.githubusercontent.com/pgEdge/pgedge-postgres-mcp/main/examples/claude-try-it/install.sh | bash
#
# Usage (non-interactive, via Claude Code):
#   curl -fsSL .../install.sh | bash -s -- --demo
#   curl -fsSL .../install.sh | bash -s -- --detect
#   curl -fsSL .../install.sh | bash -s -- --db-host=localhost --db-port=5432 --db-name=mydb --db-user=me --db-pass=secret
#
# Pin a specific version (escape hatch — skips the latest-release lookup):
#   curl -fsSL .../install.sh | bash -s -- --version=v1.0.0
#   curl -fsSL .../install.sh | PGEDGE_MCP_VERSION=v1.0.0 bash   # env var equivalent
#
# What it does:
#   1. Downloads the pgEdge MCP Server binary for your platform
#   2. Helps you connect to a database (your own or a demo with sample data)
#   3. Configures Claude Code (.claude.json) and/or Claude Desktop
#
set -eo pipefail

# ─── Configuration ───────────────────────────────────────────────────────────

INSTALL_DIR="$HOME/.pgedge"
BIN_DIR="$INSTALL_DIR/bin"
DEMO_DIR="$INSTALL_DIR/demo"
REPO="pgEdge/pgedge-postgres-mcp"
DEMO_PORT=5432

# ─── Parse flags (for non-interactive / Claude Code usage) ───────────────────

MODE=""
DB_HOST="" DB_PORT="" DB_NAME="" DB_USER="" DB_PASS=""
# Pinned version: --version=… flag wins, else PGEDGE_MCP_VERSION env var, else empty.
PIN_VERSION="${PGEDGE_MCP_VERSION:-}"

for arg in "$@"; do
  case "$arg" in
    --demo)          MODE="demo" ;;
    --own-db)        MODE="own" ;;
    --detect)        MODE="detect" ;;
    --db-host=*)     DB_HOST="${arg#*=}" ;;
    --db-port=*)     DB_PORT="${arg#*=}" ;;
    --db-name=*)     DB_NAME="${arg#*=}" ;;
    --db-user=*)     DB_USER="${arg#*=}" ;;
    --db-pass=*)     DB_PASS="${arg#*=}" ;;
    --version=*)     PIN_VERSION="${arg#*=}" ;;
    --install-docker) MODE="install-docker" ;;
  esac
done

# ─── Helper functions ────────────────────────────────────────────────────────

info()  { echo "  ℹ  $*"; }
ok()    { echo "  ✓  $*"; }
warn()  { echo "  ⚠  $*"; }
fail()  { echo "  ✗  $*" >&2; exit 1; }

# Read from /dev/tty if available (works even when script is piped from curl)
ask() {
  local prompt="$1" var="$2"
  if [ -t 0 ] || [ -e /dev/tty ]; then
    # shellcheck disable=SC2229
    read -r -p "$prompt" "$var" < /dev/tty
  else
    # Non-interactive — return empty (caller handles default)
    eval "$var=''"
  fi
}

# Like ask() but hides input (for passwords)
ask_secret() {
  local prompt="$1" var="$2"
  if [ -t 0 ] || [ -e /dev/tty ]; then
    # shellcheck disable=SC2229
    read -s -r -p "$prompt" "$var" < /dev/tty
    echo >&2  # newline after silent input
  else
    eval "$var=''"
  fi
}

has_tty() {
  [ -t 0 ] || [ -e /dev/tty ]
}

# ─── Detect platform ────────────────────────────────────────────────────────

detect_platform() {
  case "$(uname -s)" in
    Darwin) OS="darwin" ;;
    Linux)  OS="linux" ;;
    MINGW*|MSYS*|CYGWIN*) OS="windows" ;;
    *) fail "Unsupported operating system: $(uname -s)" ;;
  esac

  case "$(uname -m)" in
    x86_64|amd64)  ARCH="x86_64" ;;
    arm64|aarch64) ARCH="arm64" ;;
    *) fail "Unsupported architecture: $(uname -m)" ;;
  esac

  if [ "$OS" = "windows" ]; then EXT="zip"; else EXT="tar.gz"; fi
}

# ─── Get latest release version ─────────────────────────────────────────────

# Does the server binary asset exist for a given tag on this platform?
# Uses a 0-byte ranged GET (more reliable than HEAD against GitHub's asset
# redirect to S3, which can reject HEAD requests). Rejects only on a definitive
# 404 — any other outcome (a transient error, a proxy quirk) is treated as
# "present" so we don't wrongly skip a good release.
asset_exists_for_version() {
  local tag="$1" num="${1#v}"
  local asset="pgedge-postgres-mcp-server_${num}_${OS}_${ARCH}.${EXT}"
  local url="https://github.com/$REPO/releases/download/$tag/$asset"
  local code
  code=$(curl -sSL -r 0-0 -o /dev/null -w '%{http_code}' "$url" 2>/dev/null) || code=""
  [ "$code" != "404" ]
}

# Resolve the version to install.
#
# We deliberately do NOT trust GitHub's /releases/latest: this repo also
# publishes knowledge-base releases (tagged kb-YYYY-MM-DD) that carry only a
# kb.db asset and no platform binaries. Those are not flagged as prereleases,
# so /releases/latest can return one and the binary download 404s.
#
# Instead, walk the releases list newest-first and pick the first STABLE
# software release (tag matches v<digit>… with no prerelease "-" suffix) that
# actually has a server binary for this platform. A pinned version (--version=
# flag or PGEDGE_MCP_VERSION env var) overrides everything as an escape hatch
# (e.g. --version=v1.0.0).
get_latest_version() {
  if [ -n "$PIN_VERSION" ]; then
    # Normalize to a v-prefixed tag so --version=1.0.0 and --version=v1.0.0
    # both resolve to the v1.0.0 release tag (the leading v is optional).
    VERSION_NUM="${PIN_VERSION#[vV]}"
    VERSION="v${VERSION_NUM}"
    info "Using pinned version $VERSION"
    return
  fi

  local response
  response=$(curl -fsSL "https://api.github.com/repos/$REPO/releases?per_page=30") \
    || fail "Could not fetch releases from GitHub (network error or rate limit)"

  # Collect stable v* tags, newest first (GitHub returns newest first).
  local -a candidates=()
  local tag
  while IFS= read -r tag; do
    [ -z "$tag" ] && continue
    case "$tag" in *-*) continue ;; esac   # skip prereleases (v1.0.0-beta3, -test1…)
    candidates+=("$tag")
  done <<< "$(echo "$response" | grep -oE '"tag_name": *"v[0-9][^"]*"' | cut -d'"' -f4)"

  [ ${#candidates[@]} -eq 0 ] \
    && fail "No stable release (v*) found in the latest 30 releases"

  # Order by semantic version, highest first. GitHub lists releases by date,
  # not version, so a hotfix cut on an older line after a newer release would
  # otherwise be picked ahead of it. Falls back to list (date) order if this
  # platform's sort lacks -V.
  if echo | sort -V >/dev/null 2>&1; then
    local -a sorted=()
    while IFS= read -r tag; do
      [ -n "$tag" ] && sorted+=("$tag")
    done <<< "$(printf '%s\n' "${candidates[@]}" | sort -V -r)"
    candidates=("${sorted[@]}")
  fi

  # Pick the highest version that actually ships a binary for this platform.
  for tag in "${candidates[@]}"; do
    if asset_exists_for_version "$tag"; then
      VERSION="$tag"
      VERSION_NUM="${VERSION#v}"
      return
    fi
  done

  fail "No stable release with a ${OS}/${ARCH} binary was found (checked: ${candidates[*]})"
}

# ─── Download and install binary ────────────────────────────────────────────

download_binary() {
  local asset_name="pgedge-postgres-mcp-server_${VERSION_NUM}_${OS}_${ARCH}.${EXT}"
  local url="https://github.com/$REPO/releases/download/$VERSION/$asset_name"
  local tmp_dir
  tmp_dir=$(mktemp -d)

  info "Downloading pgEdge MCP Server $VERSION ($OS/$ARCH)..."

  local http_code
  http_code=$(curl -sSL -o "$tmp_dir/$asset_name" -w '%{http_code}' "$url" 2>/dev/null) \
    || http_code="000"
  case "$http_code" in
    2??) : ;;  # success
    403|429) rm -rf "$tmp_dir"
             fail "Download failed (HTTP $http_code): GitHub rate limit reached. Wait a few minutes and retry, or pin a version with --version=$VERSION." ;;
    404)     rm -rf "$tmp_dir"
             fail "Download failed (HTTP 404): no $OS/$ARCH binary for $VERSION at $url" ;;
    *)       rm -rf "$tmp_dir"
             fail "Download failed (HTTP ${http_code:-000}). Check your internet connection." ;;
  esac

  mkdir -p "$BIN_DIR"

  if [ "$EXT" = "zip" ]; then
    unzip -qo "$tmp_dir/$asset_name" -d "$tmp_dir/extracted"
  else
    mkdir -p "$tmp_dir/extracted"
    tar xzf "$tmp_dir/$asset_name" -C "$tmp_dir/extracted"
  fi

  # Find the binary in the extracted archive (may be in a subdirectory)
  local binary
  binary=$(find "$tmp_dir/extracted" -name "pgedge-postgres-mcp" -type f | head -1)
  if [ -z "$binary" ]; then
    binary=$(find "$tmp_dir/extracted" -name "pgedge-postgres-mcp*" -type f | head -1)
  fi
  [ -z "$binary" ] && fail "Binary not found in archive"

  cp "$binary" "$BIN_DIR/pgedge-postgres-mcp"
  chmod +x "$BIN_DIR/pgedge-postgres-mcp"
  rm -rf "$tmp_dir"

  echo "$VERSION" > "$BIN_DIR/.version"
  ok "Binary installed: $BIN_DIR/pgedge-postgres-mcp"
}

# ─── Docker detection and installation ──────────────────────────────────────

docker_installed() {
  command -v docker &>/dev/null
}

docker_running() {
  docker info >/dev/null 2>&1
}

install_docker() {
  echo ""
  info "Installing Docker..."
  echo ""

  case "$OS" in
    darwin)
      if command -v brew &>/dev/null; then
        info "Installing Docker Desktop via Homebrew (this may take a few minutes)..."
        brew install --cask docker
        info "Docker Desktop installed. Please open Docker Desktop from"
        info "your Applications folder, wait for it to start, then re-run"
        info "this installer."
        exit 0
      else
        echo ""
        echo "  Docker Desktop needs to be installed manually on macOS."
        echo ""
        echo "  1. Download it from: https://www.docker.com/products/docker-desktop/"
        echo "  2. Open the .dmg and drag Docker to Applications"
        echo "  3. Launch Docker Desktop and wait for it to start"
        echo "  4. Re-run this installer"
        echo ""
        exit 0
      fi
      ;;
    linux)
      info "Installing Docker Engine..."
      curl -fsSL https://get.docker.com | sh || true
      if docker_installed && docker_running; then
        ok "Docker installed successfully"
      else
        warn "Docker installed but may need a logout/login to take effect."
        warn "Try: sudo usermod -aG docker \$USER && newgrp docker"
        warn "Then re-run this installer."
        exit 0
      fi
      ;;
    *)
      echo "  Please install Docker Desktop from: https://www.docker.com/products/docker-desktop/"
      exit 0
      ;;
  esac
}

# ─── Database choice ────────────────────────────────────────────────────────

choose_database() {
  # Flag-based modes (non-interactive)
  if [ "$MODE" = "demo" ]; then
    setup_demo_database
    return
  fi

  if [ "$MODE" = "own" ]; then
    setup_own_database
    return
  fi

  if [ "$MODE" = "install-docker" ]; then
    install_docker
    setup_demo_database
    return
  fi

  if [ "$MODE" = "detect" ]; then
    detect_postgres_instances
    if [ ${#DETECTED_PORTS[@]} -eq 0 ]; then
      echo ""
      echo "DETECT_NO_INSTANCES"
      echo "No reachable PostgreSQL instance found. Re-run with:"
      echo "  --own-db --db-host=HOST --db-port=PORT --db-name=DB --db-user=USER --db-pass=PASS"
      DB_CONFIGURED=false
      return
    fi
    # Auto-connect to first instance, first database
    local target_port=""
    [ -n "$DB_PORT" ] && target_port="$DB_PORT"
    connect_existing_auto "$target_port"
    return
  fi

  # --- Detect existing instances ---
  detect_postgres_instances

  # Non-interactive without flags — output choices for Claude
  if ! has_tty; then
    echo ""
    echo "DATABASE_CHOICE_NEEDED"
    echo "The MCP server needs a PostgreSQL database to connect to."
    if [ ${#DETECTED_PORTS[@]} -gt 0 ]; then
      echo ""
      echo "Detected PostgreSQL on port(s): ${DETECTED_PORTS[*]}"
      echo ""
      echo "Options:"
      echo "  1. Connect to detected instance — re-run with: --detect"
      echo "  2. Demo database — re-run with: --demo"
      echo "  3. Your own database — re-run with: --own-db --db-host=HOST --db-port=PORT --db-name=DB --db-user=USER --db-pass=PASS"
    else
      echo "Options:"
      echo "  1. Demo database — re-run with: --demo"
      echo "  2. Your own database — re-run with: --own-db --db-host=HOST --db-port=PORT --db-name=DB --db-user=USER --db-pass=PASS"
    fi
    echo ""
    DB_CONFIGURED=false
    return
  fi

  # --- Interactive menu ---
  echo ""
  echo "  The MCP server needs a PostgreSQL database to connect to."

  if [ ${#DETECTED_PORTS[@]} -gt 0 ]; then
    echo ""
    echo "  I found PostgreSQL running on:"
    for i in "${!DETECTED_PORTS[@]}"; do
      local port="${DETECTED_PORTS[$i]}"
      local confirmed="${DETECTED_CONFIRMED[$i]}"
      if [ "$confirmed" = "true" ]; then
        echo "    * port $port"
      else
        echo "    * port $port (likely PostgreSQL)"
      fi
    done

    echo ""
    echo "  Which would you like?"
    echo ""
    echo "    1) Connect to an existing instance (port ${DETECTED_PORTS[0]})"
    echo "       I'll help you pick a database."
    echo ""
    echo "    2) Load a sample database (Northwind — customers, orders, products)"
    echo "       Requires Docker. Runs on a non-conflicting port."
    echo ""
    echo "    3) Connect to a different PostgreSQL database"
    echo "       You'll provide the connection details."
    echo ""

    local choice
    if [ ${#DETECTED_PORTS[@]} -gt 1 ]; then
      ask "  Enter 1, 2, or 3 (or 1:<port> to pick a specific instance): " choice
    else
      ask "  Enter 1, 2, or 3: " choice
    fi

    # Parse "1:port" syntax
    local target_port=""
    case "$choice" in
      1:*)
        target_port="${choice#1:}"
        choice="1"
        ;;
    esac

    case "$choice" in
      1) connect_existing_instance "$target_port" ;;
      2) setup_demo_database ;;
      3) setup_own_database ;;
      *) info "Defaulting to existing instance..."
         connect_existing_instance "$target_port" ;;
    esac
  else
    # No instances detected — show original two-option menu
    echo ""
    echo "  Which would you like?"
    echo ""
    echo "    1) Load a sample database (Northwind — customers, orders, products)"
    echo "       Requires Docker. Great for trying things out."
    echo ""
    echo "    2) Connect to my own PostgreSQL database"
    echo "       You'll provide the connection details."
    echo ""

    local choice
    ask "  Enter 1 or 2: " choice

    case "$choice" in
      1) setup_demo_database ;;
      2) setup_own_database ;;
      *) info "Defaulting to sample database..."
         setup_demo_database ;;
    esac
  fi
}

# ─── Demo database setup ────────────────────────────────────────────────────

setup_demo_database() {
  if docker_installed && docker_running; then
    start_demo_postgres
    return
  fi

  # Docker installed but not running
  if docker_installed; then
    echo ""
    warn "Docker is installed but not running."
    echo ""
    echo "  Please start Docker Desktop and wait for it to finish starting,"
    echo "  then re-run this installer."
    echo ""

    if ! has_tty; then
      echo "DOCKER_NOT_RUNNING"
      echo "Start Docker Desktop, then re-run with: --demo"
      DB_CONFIGURED=false
      return
    fi

    echo "  Options:"
    echo ""
    echo "    1) I'll start Docker Desktop and re-run this later"
    echo "    2) Connect to my own database instead"
    echo ""

    local choice
    ask "  Enter 1 or 2: " choice

    case "$choice" in
      2) setup_own_database ;;
      *)
        echo ""
        echo "  Start Docker Desktop, wait for it to finish starting,"
        echo "  then re-run this installer."
        echo ""
        DB_CONFIGURED=false
        ;;
    esac
    return
  fi

  # Docker not installed at all
  echo ""
  warn "Docker is not installed."
  echo ""
  echo "  The sample database runs in a Docker container."
  echo "  Docker Desktop is free and takes about 5 minutes to install."
  echo ""

  if ! has_tty; then
    echo "DOCKER_NOT_FOUND"
    echo "To install Docker and set up the demo, re-run with: --install-docker"
    echo "To skip the demo and use your own database, re-run with: --own-db --db-host=... --db-port=... --db-name=... --db-user=... --db-pass=..."
    DB_CONFIGURED=false
    return
  fi

  echo "  Would you like me to install Docker for you?"
  echo ""
  echo "    1) Yes, install Docker"
  echo "    2) No, I'll connect to my own database instead"
  echo "    3) No, I'll install Docker myself and re-run this later"
  echo ""

  local choice
  ask "  Enter 1, 2, or 3: " choice

  case "$choice" in
    1) install_docker; start_demo_postgres ;;
    2) setup_own_database ;;
    *)
      echo ""
      echo "  To install Docker Desktop:"
      echo "    https://www.docker.com/products/docker-desktop/"
      echo ""
      echo "  After installing, re-run this command:"
      echo "    curl -fsSL https://raw.githubusercontent.com/pgEdge/pgedge-postgres-mcp/main/examples/claude-try-it/install.sh | bash"
      echo ""
      DB_CONFIGURED=false
      ;;
  esac
}

# ─── Find a free port ────────────────────────────────────────────────────────

port_in_use() {
  local p="$1"
  if command -v lsof &>/dev/null; then
    lsof -i ":$p" >/dev/null 2>&1
  elif command -v ss &>/dev/null; then
    ss -tlnH "sport = :$p" 2>/dev/null | grep -q .
  else
    (echo >/dev/tcp/localhost/"$p") 2>/dev/null
  fi
}

find_free_port() {
  # Try preferred ports in order: 5432, 5433, 5434, 5435, 5436
  for port in 5432 5433 5434 5435 5436; do
    if ! port_in_use "$port"; then
      echo "$port"
      return
    fi
  done
  # Last resort: let the OS pick
  python3 -c "import socket; s=socket.socket(); s.bind(('',0)); print(s.getsockname()[1]); s.close()" 2>/dev/null \
    || echo "0"
}

# ─── Detect running Postgres instances ─────────────────────────────────

# Populates two parallel arrays:
#   DETECTED_PORTS[]    — port numbers with listeners
#   DETECTED_CONFIRMED[] — "true" if pg_isready confirmed Postgres
detect_postgres_instances() {
  DETECTED_PORTS=()
  DETECTED_CONFIRMED=()

  local has_pgready=false
  command -v pg_isready &>/dev/null && has_pgready=true

  for port in 5432 5433 5434 5435 5436; do
    local listening=false confirmed=false

    if $has_pgready; then
      if pg_isready -h localhost -p "$port" -t 2 >/dev/null 2>&1; then
        listening=true
        confirmed=true
      fi
    fi

    if ! $listening; then
      if command -v lsof &>/dev/null; then
        if lsof -iTCP:"$port" -sTCP:LISTEN -P -n >/dev/null 2>&1; then
          listening=true
        fi
      elif command -v ss &>/dev/null; then
        if ss -tlnH "sport = :$port" 2>/dev/null | grep -q .; then
          listening=true
        fi
      fi
    fi

    if $listening; then
      DETECTED_PORTS+=("$port")
      DETECTED_CONFIRMED+=("$confirmed")
    fi
  done
}

# ─── Try passwordless auth against a Postgres instance ─────────────────

# Sets AUTH_USER on success, returns 1 on failure.
# Requires psql on PATH.
try_passwordless_auth() {
  local port="$1"
  AUTH_USER=""

  if ! command -v psql &>/dev/null; then
    return 1
  fi

  # Try 'postgres' user first (most common superuser)
  if PGPASSWORD="" psql -h localhost -p "$port" -U postgres -d postgres \
       -w -c "SELECT 1" >/dev/null 2>&1; then
    AUTH_USER="postgres"
    return 0
  fi

  # Try current OS username
  local os_user
  os_user="$(whoami)"
  if [ "$os_user" != "postgres" ]; then
    if PGPASSWORD="" psql -h localhost -p "$port" -U "$os_user" -d postgres \
         -w -c "SELECT 1" >/dev/null 2>&1; then
      AUTH_USER="$os_user"
      return 0
    fi
  fi

  return 1
}

# ─── List user databases on a Postgres instance ────────────────────────

# Requires psql. Prints database names, one per line.
# Filters out template and system databases.
list_databases() {
  local port="$1" user="$2" password="${3:-}"

  PGPASSWORD="$password" psql -h localhost -p "$port" -U "$user" -d postgres \
    -w -t -A -c "
    SELECT datname FROM pg_database
    WHERE datistemplate = false
      AND datname NOT IN ('postgres')
    ORDER BY datname
  " 2>/dev/null
}

# ─── Connect to an existing Postgres instance ──────────────────────────

connect_existing_instance() {
  local target_port="${1:-}"

  # Build arrays of instance state
  local -a inst_ports=()
  local -a inst_confirmed=()
  local -a inst_users=()
  local -a inst_passwords=()
  local -a inst_authed=()
  local -a all_db_names=()
  local -a all_db_ports=()
  local -a all_db_users=()
  local -a all_db_passwords=()

  local has_psql=false
  command -v psql &>/dev/null && has_psql=true

  for i in "${!DETECTED_PORTS[@]}"; do
    local port="${DETECTED_PORTS[$i]}"
    local confirmed="${DETECTED_CONFIRMED[$i]}"

    # If user specified a port, skip others
    if [ -n "$target_port" ] && [ "$port" != "$target_port" ]; then
      continue
    fi

    inst_ports+=("$port")
    inst_confirmed+=("$confirmed")

    if $has_psql && try_passwordless_auth "$port"; then
      inst_users+=("$AUTH_USER")
      inst_passwords+=("")
      inst_authed+=("true")

      # List databases on this instance
      local dbs
      dbs=$(list_databases "$port" "$AUTH_USER" "")
      if [ -n "$dbs" ]; then
        while IFS= read -r db; do
          all_db_names+=("$db")
          all_db_ports+=("$port")
          all_db_users+=("$AUTH_USER")
          all_db_passwords+=("")
        done <<< "$dbs"
      fi
    else
      inst_users+=("")
      inst_passwords+=("")
      inst_authed+=("false")
    fi
  done

  if [ ${#inst_ports[@]} -eq 0 ]; then
    warn "No instances to connect to."
    DB_CONFIGURED=false
    return
  fi

  # --- Present the combined menu ---

  echo ""
  local option_num=1
  local -a option_type=()
  local -a option_data=()

  for i in "${!inst_ports[@]}"; do
    local port="${inst_ports[$i]}"
    local user="${inst_users[$i]}"
    local authed="${inst_authed[$i]}"

    if [ "$authed" = "true" ]; then
      local label="PostgreSQL"
      [ "${inst_confirmed[$i]:-false}" = "true" ] \
        || label="service (likely PostgreSQL)"
      echo "    Port $port ($label) — connected as '$user'"

      # Show databases for this instance
      local found_db=false
      for j in "${!all_db_names[@]}"; do
        if [ "${all_db_ports[$j]}" = "$port" ]; then
          echo "      $option_num) ${all_db_names[$j]}"
          option_type+=("db")
          option_data+=("$j")
          option_num=$((option_num + 1))
          found_db=true
        fi
      done

      if ! $found_db; then
        echo "      (no user databases found)"
      fi
    else
      if $has_psql; then
        echo "    Port $port — authentication required"
        echo "      $option_num) Enter credentials for this instance"
        option_type+=("auth")
        option_data+=("$i")
        option_num=$((option_num + 1))
      else
        echo "    Port $port — psql not installed, cannot list databases"
        echo "      $option_num) Enter connection details for port $port"
        option_type+=("manualPort")
        option_data+=("$port")
        option_num=$((option_num + 1))
      fi
    fi

    echo ""
  done

  echo "    Other options:"
  echo "      $option_num) Start a demo database instead (Docker, Northwind)"
  option_type+=("demo")
  option_data+=("")
  option_num=$((option_num + 1))

  echo "      $option_num) Enter connection details manually"
  option_type+=("manual")
  option_data+=("")
  echo ""

  local choice
  ask "  Enter a number: " choice
  choice="${choice:-1}"

  # Validate choice is a number in range
  if ! [[ "$choice" =~ ^[0-9]+$ ]] || [ "$choice" -lt 1 ] \
       || [ "$choice" -gt "${#option_type[@]}" ]; then
    warn "Invalid choice. Defaulting to demo database."
    setup_demo_database
    return
  fi

  local idx=$((choice - 1))
  case "${option_type[$idx]}" in
    db)
      local db_idx="${option_data[$idx]}"
      DB_HOST="localhost"
      DB_PORT="${all_db_ports[$db_idx]}"
      DB_NAME="${all_db_names[$db_idx]}"
      DB_USER="${all_db_users[$db_idx]}"
      DB_PASS="${all_db_passwords[$db_idx]}"
      DB_CONFIGURED=true
      ok "Using database: $DB_NAME on localhost:$DB_PORT ($DB_USER)"
      ;;
    auth)
      local inst_idx="${option_data[$idx]}"
      local port="${inst_ports[$inst_idx]}"
      prompt_credentials_and_list "$port"
      ;;
    manualPort)
      DB_HOST="localhost"
      DB_PORT="${option_data[$idx]}"
      setup_own_database
      ;;
    demo)
      setup_demo_database
      ;;
    manual)
      setup_own_database
      ;;
  esac
}

# ─── Prompt for credentials and list databases ─────────────────────────

prompt_credentials_and_list() {
  local port="$1"
  local attempts=0

  while [ $attempts -lt 2 ]; do
    echo ""
    echo "  Connection to port $port requires authentication."
    echo ""

    local user pass
    ask "  Username [postgres]: " user
    user="${user:-postgres}"
    ask_secret "  Password: " pass

    if PGPASSWORD="$pass" psql -h localhost -p "$port" -U "$user" -d postgres \
         -w -c "SELECT 1" >/dev/null 2>&1; then
      ok "Connected to port $port as '$user'"

      local dbs
      dbs=$(list_databases "$port" "$user" "$pass")
      if [ -n "$dbs" ]; then
        echo ""
        echo "  Databases on port $port:"
        local num=1
        local -a db_arr=()
        while IFS= read -r db; do
          echo "    $num) $db"
          db_arr+=("$db")
          num=$((num + 1))
        done <<< "$dbs"
        echo ""

        local db_choice
        ask "  Enter a number (or type a database name): " db_choice

        # Check if it's a number
        if [[ "$db_choice" =~ ^[0-9]+$ ]] \
             && [ "$db_choice" -ge 1 ] \
             && [ "$db_choice" -le "${#db_arr[@]}" ]; then
          DB_NAME="${db_arr[$((db_choice - 1))]}"
        else
          DB_NAME="$db_choice"
        fi
      else
        echo ""
        echo "  No user databases found on port $port."
        echo ""
        ask "  Database name: " DB_NAME
        if [ -z "$DB_NAME" ]; then
          warn "Database name is required."
          DB_CONFIGURED=false
          return
        fi
      fi

      DB_HOST="localhost"
      DB_PORT="$port"
      DB_USER="$user"
      DB_PASS="$pass"
      DB_CONFIGURED=true
      ok "Using database: $DB_NAME on localhost:$port ($user)"
      return
    fi

    warn "Authentication failed."
    attempts=$((attempts + 1))
  done

  warn "Could not connect to port $port. Try the manual option."
  echo ""
  setup_own_database
}

# ─── Auto-connect (non-interactive --detect mode) ──────────────────────

connect_existing_auto() {
  local target_port="${1:-}"
  local has_psql=false
  command -v psql &>/dev/null && has_psql=true
  local matched_target=true
  [ -n "$target_port" ] && matched_target=false

  for i in "${!DETECTED_PORTS[@]}"; do
    local port="${DETECTED_PORTS[$i]}"
    if [ -n "$target_port" ] && [ "$port" != "$target_port" ]; then
      continue
    fi
    [ -n "$target_port" ] && matched_target=true

    if $has_psql && try_passwordless_auth "$port"; then
      if [ -n "$DB_NAME" ]; then
        local dbs
        dbs=$(list_databases "$port" "$AUTH_USER" "")
        if echo "$dbs" | grep -Fqx -- "$DB_NAME"; then
          DB_HOST="localhost"; DB_PORT="$port"
          DB_USER="$AUTH_USER"; DB_PASS=""
          DB_CONFIGURED=true
          ok "Using database: $DB_NAME on localhost:$port ($AUTH_USER)"
          return
        fi
        warn "Database '$DB_NAME' not found on port $port"
        continue
      fi

      local first_db
      first_db=$(list_databases "$port" "$AUTH_USER" "" | head -1)
      if [ -n "$first_db" ]; then
        DB_HOST="localhost"; DB_PORT="$port"
        DB_NAME="$first_db"; DB_USER="$AUTH_USER"; DB_PASS=""
        DB_CONFIGURED=true
        ok "Auto-detected: $DB_NAME on localhost:$port ($AUTH_USER)"
        return
      fi
    fi
  done

  echo ""
  if ! $has_psql; then
    echo "DETECT_PSQL_MISSING"
    echo "psql is required for --detect auto-connection."
  elif ! $matched_target; then
    echo "DETECT_PORT_NOT_FOUND"
    echo "No detected PostgreSQL instance on port $target_port."
  else
    echo "DETECT_AUTH_FAILED"
    echo "Could not authenticate to any detected instance."
  fi
  echo "Re-run with explicit credentials:"
  echo "  --own-db --db-host=HOST --db-port=PORT --db-name=DB --db-user=USER --db-pass=PASS"
  DB_CONFIGURED=false
}

# ─── Clean up old demo containers ─────────────────────────────────────────

cleanup_old_demos() {
  local old
  for old in $(docker ps -a --filter "name=pgedge-demo-" \
               --format '{{.Names}}' 2>/dev/null); do
    info "Removing old demo container: $old"
    docker stop "$old" 2>/dev/null || true
    docker rm -v "$old" 2>/dev/null || true
  done
}

# ─── Start demo Postgres container ──────────────────────────────────────────

start_demo_postgres() {
  # Clean up any old demo containers from previous installs
  cleanup_old_demos

  # Generate a unique container name for this run
  CONTAINER_NAME="pgedge-demo-$(date +%s)"

  # Find a free port
  DEMO_PORT=$(find_free_port)
  if [ "$DEMO_PORT" = "0" ]; then
    warn "Could not find a free port for the demo database."
    DB_CONFIGURED=false
    return
  fi

  if [ "$DEMO_PORT" != "5432" ]; then
    info "Port 5432 is in use (probably an existing Postgres instance)."
    info "Using port $DEMO_PORT for the demo database instead."
  fi

  mkdir -p "$DEMO_DIR"

  # Write docker-compose with port and container name substituted via bash
  # (avoids sed cross-platform issues)
  COMPOSE_CONTENT=$(cat << 'COMPOSE'
services:
  postgres:
    image: ghcr.io/pgedge/pgedge-postgres:17-spock5-standard
    container_name: PGEDGE_CONTAINER_NAME
    command: postgres -c listen_addresses='*' -c shared_preload_libraries='pg_stat_statements'
    environment:
      POSTGRES_USER: demo
      POSTGRES_PASSWORD: demo123
      POSTGRES_DB: northwind
    volumes:
      - pgdata:/var/lib/postgresql/data
    configs:
      - source: load-northwind
        target: /docker-entrypoint-initdb.d/01-load-northwind.sh
        mode: 0755
      - source: enable-extensions
        target: /docker-entrypoint-initdb.d/02-enable-extensions.sh
        mode: 0755
    ports:
      - "PGEDGE_HOST_PORT:5432"
    restart: unless-stopped
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U demo -d northwind"]
      interval: 5s
      timeout: 5s
      retries: 10
      start_period: 30s

volumes:
  pgdata:
    driver: local

configs:
  load-northwind:
    content: |-
      #!/usr/bin/env bash
      set -e
      echo "Loading Northwind dataset..."
      curl -fsSL -o /tmp/northwind.sql https://downloads.pgedge.com/platform/examples/northwind/northwind.sql
      psql -v ON_ERROR_STOP=1 --username "$$POSTGRES_USER" --dbname "$$POSTGRES_DB" -f /tmp/northwind.sql
      rm -f /tmp/northwind.sql
      echo "Northwind dataset loaded"
  enable-extensions:
    content: |-
      #!/usr/bin/env bash
      set -e
      psql -v ON_ERROR_STOP=1 --username "$$POSTGRES_USER" --dbname "$$POSTGRES_DB" \
        -c "CREATE EXTENSION IF NOT EXISTS pg_stat_statements;"
      echo "Extensions enabled"
COMPOSE
)
  local composed="${COMPOSE_CONTENT//PGEDGE_HOST_PORT/$DEMO_PORT}"
  echo "${composed//PGEDGE_CONTAINER_NAME/$CONTAINER_NAME}" > "$DEMO_DIR/docker-compose.yml"

  echo ""
  info "Starting demo Postgres ($CONTAINER_NAME) with Northwind sample data on port $DEMO_PORT..."
  info "(first run downloads the image — this may take a minute)"
  echo ""

  docker compose -f "$DEMO_DIR/docker-compose.yml" up -d 2>/dev/null \
    || docker-compose -f "$DEMO_DIR/docker-compose.yml" up -d 2>/dev/null \
    || { warn "Failed to start demo database."; DB_CONFIGURED=false; return; }

  info "Waiting for database to be ready..."
  for _ in $(seq 1 24); do
    if docker exec "$CONTAINER_NAME" pg_isready -U demo -d northwind >/dev/null 2>&1; then
      ok "Demo database ready (northwind on localhost:$DEMO_PORT, container: $CONTAINER_NAME)"
      DB_HOST="localhost"; DB_PORT="$DEMO_PORT"; DB_NAME="northwind"
      DB_USER="demo"; DB_PASS="demo123"; DB_CONFIGURED=true
      return
    fi
    sleep 5
  done

  warn "Database is still starting. It may need another minute."
  DB_HOST="localhost"; DB_PORT="$DEMO_PORT"; DB_NAME="northwind"
  DB_USER="demo"; DB_PASS="demo123"; DB_CONFIGURED=true
}

# ─── Database connection test ───────────────────────────────────────────────

test_db_connection() {
  local host="$1" port="$2"
  # Try pg_isready first (most reliable)
  if command -v pg_isready &>/dev/null; then
    if pg_isready -h "$host" -p "$port" -t 3 >/dev/null 2>&1; then
      return 0
    fi
    return 1
  fi
  # Fallback: TCP connect via /dev/tcp (bash built-in)
  if (echo >/dev/tcp/"$host"/"$port") 2>/dev/null; then
    return 0
  fi
  # Fallback: nc/netcat
  if command -v nc &>/dev/null; then
    if nc -z -w 3 "$host" "$port" 2>/dev/null; then
      return 0
    fi
    return 1
  fi
  # No way to test — assume OK
  return 0
}

verify_own_db_connection() {
  info "Testing connection to $DB_HOST:$DB_PORT..."
  if test_db_connection "$DB_HOST" "$DB_PORT"; then
    ok "Connection to $DB_HOST:$DB_PORT succeeded"
    return
  fi

  echo ""
  warn "Could not reach $DB_HOST:$DB_PORT (TCP connection failed)"
  echo ""

  if ! has_tty; then
    warn "Continuing anyway — verify your connection details are correct."
    return
  fi

  echo "  What would you like to do?"
  echo ""
  echo "    1) Re-enter connection details"
  echo "    2) Continue anyway (I'll fix it later)"
  echo ""

  local choice
  ask "  Enter 1 or 2: " choice

  case "$choice" in
    1)
      # Clear previous values so setup_own_database re-prompts
      DB_HOST="" DB_PORT="" DB_NAME="" DB_USER="" DB_PASS=""
      setup_own_database
      return
      ;;
    *) warn "Continuing — you can update ~/.claude.json later with the correct details." ;;
  esac
}

# ─── Own database setup ─────────────────────────────────────────────────────

setup_own_database() {
  # If connection details were provided via flags
  if [ -n "$DB_HOST" ] && [ -n "$DB_NAME" ] && [ -n "$DB_USER" ]; then
    DB_PORT="${DB_PORT:-5432}"
    DB_CONFIGURED=true
    ok "Using database: $DB_NAME on $DB_HOST:$DB_PORT"
    verify_own_db_connection
    return
  fi

  if ! has_tty; then
    echo ""
    echo "OWN_DATABASE_CHOSEN"
    echo "Re-run with connection details:"
    echo "  --own-db --db-host=HOST --db-port=PORT --db-name=DB --db-user=USER --db-pass=PASS"
    DB_CONFIGURED=false
    return
  fi

  echo ""
  echo "  Enter your PostgreSQL connection details:"
  echo ""

  local default_host="${DB_HOST:-localhost}"
  ask "  Host [$default_host]: " DB_HOST
  DB_HOST="${DB_HOST:-$default_host}"

  local default_port="${DB_PORT:-5432}"
  ask "  Port [$default_port]: " DB_PORT
  DB_PORT="${DB_PORT:-$default_port}"

  ask "  Database name: " DB_NAME
  [ -z "$DB_NAME" ] && { warn "Database name is required."; DB_CONFIGURED=false; return; }

  ask "  Username: " DB_USER
  [ -z "$DB_USER" ] && { warn "Username is required."; DB_CONFIGURED=false; return; }

  ask_secret "  Password: " DB_PASS

  DB_CONFIGURED=true
  ok "Using database: $DB_NAME on $DB_HOST:$DB_PORT"
  verify_own_db_connection
}

# ─── JSON helper ───────────────────────────────────────────────────────────

# Escape a string for safe embedding in JSON (handles \, ", control chars)
json_escape() {
  local s="$1"
  s="${s//\\/\\\\}"    # \ → \\  (must be first)
  s="${s//\"/\\\"}"    # " → \"
  s="${s//$'\n'/\\n}"  # newline → \n
  s="${s//$'\r'/\\r}"  # carriage return → \r
  s="${s//$'\t'/\\t}"  # tab → \t
  printf '%s' "$s"
}

# Write pgedge MCP config into a JSON file using python3 (safe for all values).
# Usage: write_mcp_config <config_file> <binary_path> [merge]
# If "merge" is passed and the file exists, merges into existing mcpServers.
write_mcp_config() {
  local config_file="$1" binary_path="$2" merge="${3:-}"

  if command -v python3 &>/dev/null; then
    # Pass values via environment to avoid any shell/python injection
    _MCP_FILE="$config_file" \
    _MCP_CMD="$binary_path" \
    _MCP_HOST="${DB_HOST:-localhost}" \
    _MCP_PORT="${DB_PORT:-5432}" \
    _MCP_DB="${DB_NAME:-your_database}" \
    _MCP_USER="${DB_USER:-your_user}" \
    _MCP_PASS="${DB_PASS:-your_password}" \
    _MCP_MERGE="$merge" \
    python3 -c '
import json, os, shutil, sys

config_file = os.environ["_MCP_FILE"]
merge = os.environ.get("_MCP_MERGE") == "merge"

config = {}
if merge:
    try:
        with open(config_file) as f:
            config = json.load(f)
    except FileNotFoundError:
        pass
    except (json.JSONDecodeError, ValueError) as e:
        backup = config_file + ".bak"
        shutil.copy2(config_file, backup)
        print(f"Warning: invalid JSON in {config_file}; backed up to {backup}", file=sys.stderr)

if "mcpServers" not in config:
    config["mcpServers"] = {}

config["mcpServers"]["pgedge"] = {
    "command": os.environ["_MCP_CMD"],
    "env": {
        "PGHOST":     os.environ["_MCP_HOST"],
        "PGPORT":     os.environ["_MCP_PORT"],
        "PGDATABASE": os.environ["_MCP_DB"],
        "PGUSER":     os.environ["_MCP_USER"],
        "PGPASSWORD": os.environ["_MCP_PASS"],
    }
}

with open(config_file, "w") as f:
    json.dump(config, f, indent=2)
os.chmod(config_file, 0o600)
' && return 0
    # python3 failed — fall through to manual JSON fallback
  fi

  # Fallback: no python3 — build JSON with escaped values
  if [ "$merge" = "merge" ] && [ -f "$config_file" ]; then
    warn "python3 not found — cannot safely merge into existing $config_file."
    warn "Install python3 and re-run to update this config."
    return 1
  fi

  local j_cmd j_host j_port j_db j_user j_pass
  j_cmd=$(json_escape "$binary_path")
  j_host=$(json_escape "${DB_HOST:-localhost}")
  j_port=$(json_escape "${DB_PORT:-5432}")
  j_db=$(json_escape "${DB_NAME:-your_database}")
  j_user=$(json_escape "${DB_USER:-your_user}")
  j_pass=$(json_escape "${DB_PASS:-your_password}")

  printf '{\n  "mcpServers": {\n    "pgedge": {\n      "command": "%s",\n      "env": {\n        "PGHOST": "%s",\n        "PGPORT": "%s",\n        "PGDATABASE": "%s",\n        "PGUSER": "%s",\n        "PGPASSWORD": "%s"\n      }\n    }\n  }\n}\n' \
    "$j_cmd" "$j_host" "$j_port" "$j_db" "$j_user" "$j_pass" > "$config_file"
  chmod 600 "$config_file"
  return 0
}

# ─── Configure Claude Code ──────────────────────────────────────────────────

configure_claude_code() {
  local mcp_json="$HOME/.claude.json"
  local binary_path="$BIN_DIR/pgedge-postgres-mcp"

  # Always merge — user-level config may have other MCP servers
  if write_mcp_config "$mcp_json" "$binary_path" "merge"; then
    ok "Claude Code: configured in ~/.claude.json (available in all projects)"
  else
    warn "Could not write $mcp_json"
  fi
}

# ─── Configure Claude Desktop ───────────────────────────────────────────────

configure_claude_desktop() {
  local config_file binary_path="$BIN_DIR/pgedge-postgres-mcp"

  case "$OS" in
    darwin) config_file="$HOME/Library/Application Support/Claude/claude_desktop_config.json" ;;
    linux)  config_file="$HOME/.config/Claude/claude_desktop_config.json" ;;
    *)      return ;;
  esac

  local config_dir
  config_dir=$(dirname "$config_file")
  if [ ! -d "$config_dir" ]; then
    info "Claude Desktop not detected — skipping config"
    return
  fi

  # Always merge — Claude Desktop config may have other MCP servers
  if write_mcp_config "$config_file" "$binary_path" "merge"; then
    ok "Claude Desktop: configured (restart Claude Desktop to activate)"
  else
    warn "Could not write Claude Desktop config"
  fi
}

# ─── Summary ─────────────────────────────────────────────────────────────────

print_update_summary() {
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "  Update complete!"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo ""
  echo "  Your existing database configuration is unchanged."
  echo ""
  echo "  Claude Code:    start a new conversation to use"
  echo "                  the updated server"
  echo "  Claude Desktop: restart the app to pick up the"
  echo "                  new version"
  echo ""
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo ""
}

print_summary() {
  echo ""
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "  Installation complete!"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo ""
  echo "  Binary:   $BIN_DIR/pgedge-postgres-mcp"

  if [ "$DB_CONFIGURED" = true ]; then
    echo "  Database: $DB_NAME on $DB_HOST:$DB_PORT ($DB_USER)"
    echo ""
    echo "  Try asking Claude:"
    echo "    \"What tables are in my database?\""
    echo "    \"Show me the top 10 products by sales\""
    echo "    \"Which customers have placed more than 5 orders?\""
  else
    echo "  Database: not yet configured"
    echo ""
    local desktop_config_path
    if [ "$OS" = "linux" ]; then
      # shellcheck disable=SC2088  # intentional literal ~ for display
      desktop_config_path="~/.config/Claude/claude_desktop_config.json"
    else
      # shellcheck disable=SC2088  # intentional literal ~ for display
      desktop_config_path="~/Library/Application Support/Claude/claude_desktop_config.json"
    fi
    echo "  To configure later, edit:"
    echo "    Claude Code:    ~/.claude.json"
    echo "    Claude Desktop: $desktop_config_path"
  fi

  echo ""
  echo "  Claude Code:    ready — start a new conversation"
  echo "  Claude Desktop: restart the app, then start chatting"
  echo ""
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo ""
}

# ─── Stop stale MCP server processes ─────────────────────────────────────────

stop_stale_processes() {
  local count
  count=$(pgrep -fc '(^|/)pgedge-postgres-mcp( |$)' 2>/dev/null || true)
  if [ "$count" -gt 0 ] 2>/dev/null; then
    info "Stopping $count running MCP server process(es)..."
    pkill -f '(^|/)pgedge-postgres-mcp( |$)' 2>/dev/null || true
    sleep 1
  fi
}

# ─── Check for existing installation ────────────────────────────────────────

check_existing_install() {
  local binary="$BIN_DIR/pgedge-postgres-mcp"
  [ -f "$binary" ] || return 1

  local explicit_reconfigure=false
  [ -n "$MODE" ] && explicit_reconfigure=true

  local installed_version=""
  if [ -f "$BIN_DIR/.version" ]; then
    installed_version=$(cat "$BIN_DIR/.version")
  fi

  if [ "$installed_version" = "$VERSION" ]; then
    echo ""
    ok "pgEdge MCP Server $VERSION is already up to date."
    echo ""
    if $explicit_reconfigure; then
      choose_database
      if [ "$DB_CONFIGURED" = true ]; then
        echo ""
        configure_claude_code
        configure_claude_desktop
        print_summary
      fi
    elif has_tty; then
      local reconfigure
      ask "  Want to reconfigure the database connection? (y/n): " reconfigure
      reconfigure="${reconfigure:-n}"
      case "$reconfigure" in
        [Yy]*)
          echo ""
          choose_database
          if [ "$DB_CONFIGURED" = true ]; then
            echo ""
            configure_claude_code
            configure_claude_desktop
            print_summary
          fi
          ;;
        *)
          echo ""
          info "Nothing to do."
          echo ""
          echo "  Claude Code:    start a new conversation"
          echo "  Claude Desktop: restart the app, then start chatting"
          echo ""
          ;;
      esac
    else
      info "Already up to date. Nothing to do."
    fi
    exit 0
  fi

  echo ""
  info "pgEdge MCP Server ${installed_version:-unknown} is installed."
  ok "A newer version ($VERSION) is available."
  echo ""
  if has_tty; then
    local update
    ask "  Update? (y/n): " update
    update="${update:-y}"
    case "$update" in
      [Yy]*)
        stop_stale_processes
        download_binary
        ok "Updated to $VERSION."
        if ! $explicit_reconfigure; then
          echo ""
          print_update_summary
        fi
        ;;
      *)
        echo ""
        info "Skipping update. Exiting."
        exit 0
        ;;
    esac
  else
    stop_stale_processes
    download_binary
    ok "Updated to $VERSION."
    if ! $explicit_reconfigure; then
      echo ""
      print_update_summary
    fi
  fi

  if $explicit_reconfigure; then
    choose_database
    if [ "$DB_CONFIGURED" = true ]; then
      echo ""
      configure_claude_code
      configure_claude_desktop
      print_summary
    fi
  fi
  exit 0
}

# ─── Main ────────────────────────────────────────────────────────────────────

main() {
  echo ""
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "  pgEdge MCP Server — Installer"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo ""
  echo "  This will install the pgEdge MCP Server so you can"
  echo "  query PostgreSQL databases using natural language"
  echo "  in Claude Code or Claude Desktop."
  echo ""

  DB_CONFIGURED=false

  detect_platform
  get_latest_version
  check_existing_install || true
  download_binary

  echo ""
  choose_database

  if [ "$DB_CONFIGURED" = true ]; then
    echo ""
    configure_claude_code
    configure_claude_desktop

    print_summary
  fi
}

main "$@"
