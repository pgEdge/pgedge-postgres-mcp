#!/bin/sh
set -eu

# ----------------------------
# Config
# ----------------------------
BASE_URL="https://downloads.pgedge.com/quickstart/mcp"
FILES="docker-compose.yml .env.example"
WORKDIR="/tmp/pgedge-download.$$"

# ----------------------------
# Pretty printing (portable)
# ----------------------------
if [ -t 1 ] && command -v tput >/dev/null 2>&1; then
  BOLD="$(tput bold || true)"
  DIM="$(tput dim || true)"
  RED="$(tput setaf 1 || true)"
  GREEN="$(tput setaf 2 || true)"
  YELLOW="$(tput setaf 3 || true)"
  CYAN="$(tput setaf 6 || true)"
  RESET="$(tput sgr0 || true)"
else
  BOLD=""; DIM=""; RED=""; GREEN=""; YELLOW=""; CYAN=""; RESET=""
fi

info()  { printf "%sℹ%s  %s\n" "$CYAN" "$RESET" "$*"; }
ok()    { printf "%s✓%s  %s\n" "$GREEN" "$RESET" "$*"; }
warn()  { printf "%s!%s  %s\n" "$YELLOW" "$RESET" "$*"; }
err()   { printf "%s✗%s  %s\n" "$RED" "$RESET" "$*"; }

die() { err "$*"; exit 1; }

# ----------------------------
# Dependencies
# ----------------------------
need_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "Missing required command: $1"
}

# download helper
download() {
  url="$1"
  out="$2"

  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" -o "$out"
  elif command -v wget >/dev/null 2>&1; then
    wget -q "$url" -O "$out"
  else
    die "Need curl or wget to download files."
  fi
}

# Secure prompt (hidden input)
prompt_secret() {
  label="$1"

  printf "%s%s%s\n" "$BOLD" "$label" "$RESET" >&2
  printf "%s(Leave blank to skip)%s\n" "$DIM" "$RESET" >&2
  printf "%s› %s" "$DIM" "$RESET" >&2

  stty -echo 2>/dev/null || true
  IFS= read -r value || value=""
  stty echo 2>/dev/null || true
  printf "\n" >&2

  printf "%s" "$value"
}

# Update or append KEY=VALUE in .env
set_env_kv() {
  file="$1"
  key="$2"
  val="$3"

  [ -n "$val" ] || return 0

  if grep -q "^${key}=" "$file" 2>/dev/null; then
    tmp="${file}.tmp.$$"
    grep -v "^${key}=" "$file" > "$tmp"
    printf '%s="%s"\n' "$key" "$val" >> "$tmp"
    mv "$tmp" "$file"
  else
    printf '%s="%s"\n' "$key" "$val" >> "$file"
  fi
}

# ----------------------------
# Main
# ----------------------------
info "Creating workspace: $WORKDIR"
mkdir -p "$WORKDIR"

info "Downloading files"
for f in $FILES; do
  info "→ $f"
  download "$BASE_URL/$f" "$WORKDIR/$f"
done
ok "Downloads complete"

printf "\n%spgEdge AI Toolkit Demo setup%s\n" "$BOLD" "$RESET"
printf "%sYou need to specify an API key for Anthropic or OpenAI (or both)%s\n\n" "$DIM" "$RESET"

ANTHROPIC_KEY=$(prompt_secret "Anthropic API key")
OPENAI_KEY=$(prompt_secret "OpenAI API key")

if [ -z "$ANTHROPIC_KEY" ] && [ -z "$OPENAI_KEY" ]; then
  die "You must provide at least one API key (Anthropic or OpenAI)."
fi

ENV_EXAMPLE="$WORKDIR/.env.example"
ENV_FILE="$WORKDIR/.env"

cp "$ENV_EXAMPLE" "$ENV_FILE"
set_env_kv "$ENV_FILE" "PGEDGE_ANTHROPIC_API_KEY" "$ANTHROPIC_KEY"
set_env_kv "$ENV_FILE" "PGEDGE_OPENAI_API_KEY" "$OPENAI_KEY"
ok "Wrote .env"

need_cmd docker
if docker compose version >/dev/null 2>&1; then
  COMPOSE="docker compose"
elif command -v docker-compose >/dev/null 2>&1; then
  COMPOSE="docker-compose"
else
  die "Docker Compose not found."
fi

info "Starting services"
(
  cd "$WORKDIR"
  $COMPOSE up -d
)

info "Waiting for services to be healthy (this may take up to 60 seconds)..."
(
  cd "$WORKDIR"
  timeout=60
  while [ $timeout -gt 0 ]; do
    # Check if all services are healthy
    if $COMPOSE ps --format json 2>/dev/null | grep -q '"Health":"healthy"' || \
       $COMPOSE ps | grep -q "(healthy)"; then
      # Give it a moment to stabilize
      sleep 2
      break
    fi
    sleep 2
    timeout=$((timeout - 2))
  done

  if [ $timeout -le 0 ]; then
    warn "Timeout waiting for services. Check status with: cd $WORKDIR && docker compose ps"
  fi
)

ok "Services are ready"

printf "\n%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n" "$CYAN" "$RESET"
printf "%s  pgEdge AI Toolkit Demo is running!%s\n" "$BOLD$GREEN" "$RESET"
printf "%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n\n" "$CYAN" "$RESET"

printf "%sWeb Client Interface:%s\n" "$BOLD" "$RESET"
printf "  %shttp://localhost:8081%s\n" "$CYAN" "$RESET"
printf "  Login: %sdemo%s / %sdemo123%s\n\n" "$YELLOW" "$RESET" "$YELLOW" "$RESET"

printf "%sPostgreSQL Database:%s\n" "$BOLD" "$RESET"
printf "  Database: %snorthwind%s\n" "$CYAN" "$RESET"
printf "  User: %sdemo%s / %sdemo123%s\n" "$YELLOW" "$RESET" "$YELLOW" "$RESET"
printf "  Connect: %sdocker exec -it pgedge-quickstart-db psql -U demo -d northwind%s\n\n" "$CYAN" "$RESET"

printf "%sMCP Server API:%s\n" "$BOLD" "$RESET"
printf "  %shttp://localhost:8080%s\n" "$CYAN" "$RESET"
printf "  Bearer Token: %sdemo-token-12345%s\n\n" "$YELLOW" "$RESET"

printf "%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n\n" "$CYAN" "$RESET"

printf "%sWorkspace:%s %s\n" "$DIM" "$RESET" "$WORKDIR"
printf "%sTo stop:%s cd %s && docker compose down -v\n\n" "$DIM" "$RESET" "$WORKDIR"

printf "For more information: %shttps://github.com/pgEdge/pgedge-nla%s\n\n" "$CYAN" "$RESET"

