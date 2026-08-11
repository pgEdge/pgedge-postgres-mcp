#--------------------------------------------------------------------------
#
# pgEdge MCP Server — One-Command Installer for Windows
#
# Copyright (c) 2025 - 2026, pgEdge, Inc.
# This software is released under The PostgreSQL License
#
#--------------------------------------------------------------------------

# pgEdge MCP Server — one-command installer for Windows
#
# Usage (interactive, in PowerShell):
#   irm https://raw.githubusercontent.com/pgEdge/pgedge-postgres-mcp/main/examples/claude-try-it/install.ps1 | iex
#
# Usage (non-interactive, via Claude Code):
#   $s = irm https://raw.githubusercontent.com/pgEdge/pgedge-postgres-mcp/main/examples/claude-try-it/install.ps1; & ([scriptblock]::Create($s)) -Demo
#
# Usage (with flags, saved locally):
#   .\install.ps1 -Demo
#   .\install.ps1 -Detect
#   .\install.ps1 -Detect -DbPort 5433
#   .\install.ps1 -OwnDb -DbHost localhost -DbPort 5432 -DbName mydb -DbUser me -DbPass secret
#
# Pin a specific version (escape hatch — skips the latest-release lookup):
#   .\install.ps1 -Version v1.0.0
#   $env:PGEDGE_MCP_VERSION = "v1.0.0"; irm .../install.ps1 | iex   # env var equivalent
#
# What it does:
#   1. Downloads the pgEdge MCP Server binary for Windows (x86_64)
#   2. Helps you connect to a database (your own or a demo with sample data)
#   3. Configures Claude Code (.claude.json) and/or Claude Desktop

param(
    [switch]$Demo,
    [switch]$OwnDb,
    [switch]$InstallDocker,
    [switch]$Detect,
    [string]$DbHost,
    [string]$DbPort,
    [string]$DbName,
    [string]$DbUser,
    [string]$DbPass,
    [string]$Version
)

$ErrorActionPreference = 'Stop'
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

# --- Configuration --------------------------------------------------------

$InstallDir = Join-Path $env:USERPROFILE ".pgedge"
$BinDir     = Join-Path $InstallDir "bin"
$DemoDir    = Join-Path $InstallDir "demo"
$Repo       = "pgEdge/pgedge-postgres-mcp"
$DemoPort   = 5432

# --- State variables ------------------------------------------------------

$script:Version      = ""
$script:VersionNum   = ""
# Pinned version: -Version flag wins, else PGEDGE_MCP_VERSION env var, else empty.
$script:PinVersion   = if ($Version) { $Version } elseif ($env:PGEDGE_MCP_VERSION) { $env:PGEDGE_MCP_VERSION } else { "" }
$script:DbHost       = $DbHost
$script:DbPort       = $DbPort
$script:DbName       = $DbName
$script:DbUser       = $DbUser
$script:DbPass       = $DbPass
$script:DbConfigured = $false
$script:DemoPort     = $DemoPort
$script:AuthUser     = ""
$script:DetectedInstances = @()

# --- Helper functions -----------------------------------------------------

function Write-Info  { param([string]$Msg) Write-Host "  i  $Msg" }
function Write-Ok    { param([string]$Msg) Write-Host "  +  $Msg" -ForegroundColor Green }
function Write-Warn  { param([string]$Msg) Write-Host "  !  $Msg" -ForegroundColor Yellow }
function Write-Fail  { param([string]$Msg) Write-Host "  x  $Msg" -ForegroundColor Red; exit 1 }

function Test-Interactive {
    try {
        return [Environment]::UserInteractive -and -not [Console]::IsInputRedirected
    } catch {
        return $false
    }
}

function Read-Prompt {
    param([string]$Prompt, [string]$Default)
    if (-not (Test-Interactive)) { return $Default }
    $response = Read-Host -Prompt $Prompt
    if ([string]::IsNullOrWhiteSpace($response)) { return $Default }
    return $response
}

# --- Detect platform ------------------------------------------------------

function Get-Platform {
    if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") {
        Write-Fail "ARM64 Windows is not yet supported. Only x86_64 builds are available."
    }
    if (-not [System.Environment]::Is64BitOperatingSystem) {
        Write-Fail "32-bit Windows is not supported."
    }
    # Windows x86_64 only
    $script:OS   = "windows"
    $script:Arch = "x86_64"
    $script:Ext  = "zip"
}

# --- Get latest release version -------------------------------------------

# Does the server binary asset exist for a given tag on this platform?
# Uses a 0-byte ranged GET (Range: bytes=0-0), matching install.sh: HEAD is
# unreliable against GitHub's asset redirect to S3, which can reject it. Rejects
# only on a definitive 404 — any other failure (transient network, a proxy
# quirk) is treated as "present" so we don't wrongly skip a good release and
# strand every Windows user.
function Test-AssetExists {
    param([string]$Tag)
    $num = $Tag.TrimStart('v')
    $asset = "pgedge-postgres-mcp-server_${num}_$($script:OS)_$($script:Arch).$($script:Ext)"
    $url = "https://github.com/$Repo/releases/download/$Tag/$asset"
    try {
        Invoke-WebRequest -Uri $url -Headers @{ Range = "bytes=0-0" } `
            -UseBasicParsing -ErrorAction Stop | Out-Null
        return $true
    } catch {
        if ($_.Exception.Response -and [int]$_.Exception.Response.StatusCode -eq 404) {
            return $false
        }
        return $true
    }
}

# Resolve the version to install.
#
# We deliberately do NOT trust GitHub's /releases/latest: this repo also
# publishes knowledge-base releases (tagged kb-YYYY-MM-DD) that carry only a
# kb.db asset and no platform binaries. Those are not flagged as prereleases,
# so /releases/latest can return one and the binary download 404s.
#
# Instead, walk the releases list newest-first and pick the first STABLE
# software release (v<digit>…, not draft, not prerelease) that actually has a
# server binary for this platform. A pinned version (-Version flag or
# PGEDGE_MCP_VERSION env var) overrides everything as an escape hatch.
function Get-LatestVersion {
    if ($script:PinVersion) {
        # Normalize to a v-prefixed tag so -Version 1.0.0 and -Version v1.0.0
        # both resolve to the v1.0.0 release tag (the leading v is optional).
        $script:VersionNum = $script:PinVersion.Trim().TrimStart('v', 'V')
        $script:Version = "v$($script:VersionNum)"
        Write-Info "Using pinned version $($script:Version)"
        return
    }

    try {
        $releases = Invoke-RestMethod "https://api.github.com/repos/$Repo/releases?per_page=30"
    } catch {
        Write-Fail "Could not fetch releases from GitHub: $_"
    }

    # Stable v* releases only, ordered by semantic version (highest first).
    # GitHub lists releases by date, not version, so a hotfix cut on an older
    # line after a newer release would otherwise be picked ahead of it.
    #
    # Filter structurally on the tag, matching install.sh: reject any tag with a
    # "-" (v1.0.0-beta3, -rc1…) rather than trusting GitHub's prerelease flag.
    # This repo does not set that flag reliably — its beta/alpha tags are all
    # published with prerelease=false — so the flag alone would let an RC slip
    # through whenever a release line has no stable counterpart yet.
    $candidates = @($releases |
        Where-Object { -not $_.draft -and -not $_.prerelease -and $_.tag_name -match '^v\d' -and $_.tag_name -notmatch '-' } |
        ForEach-Object { $_.tag_name } |
        Sort-Object { try { [version]($_.TrimStart('v')) } catch { [version]'0.0' } } -Descending)

    if ($candidates.Count -eq 0) {
        Write-Fail "No stable release (v*) found in the latest 30 releases"
    }

    foreach ($tag in $candidates) {
        if (Test-AssetExists -Tag $tag) {
            $script:Version = $tag
            $script:VersionNum = $tag.TrimStart('v')
            return
        }
    }

    Write-Fail "No stable release with a $($script:OS)/$($script:Arch) binary was found (checked: $($candidates -join ', '))"
}

# --- Download and install binary ------------------------------------------

function Install-Binary {
    $assetName = "pgedge-postgres-mcp-server_$($script:VersionNum)_$($script:OS)_$($script:Arch).$($script:Ext)"
    $url = "https://github.com/$Repo/releases/download/$($script:Version)/$assetName"
    $tmpDir = Join-Path ([System.IO.Path]::GetTempPath()) ([System.Guid]::NewGuid().ToString())
    New-Item -ItemType Directory -Path $tmpDir -Force | Out-Null

    Write-Info "Downloading pgEdge MCP Server $($script:Version) ($($script:OS)/$($script:Arch))..."

    try {
        Invoke-WebRequest -Uri $url -OutFile (Join-Path $tmpDir $assetName) -UseBasicParsing
    } catch {
        $code = if ($_.Exception.Response) { [int]$_.Exception.Response.StatusCode } else { 0 }
        # Write-Fail exits, so clean up the temp dir here before it does.
        Remove-Item $tmpDir -Recurse -Force -ErrorAction SilentlyContinue
        if ($code -eq 403 -or $code -eq 429) {
            Write-Fail "Download failed (HTTP $code): GitHub rate limit reached. Wait a few minutes and retry, or pin a version with -Version $($script:Version)."
        } elseif ($code -eq 404) {
            Write-Fail "Download failed (HTTP 404): no $($script:OS)/$($script:Arch) binary for $($script:Version) at $url"
        } else {
            $suffix = if ($code) { " (HTTP $code)" } else { "" }
            Write-Fail "Download failed$suffix. Check your internet connection."
        }
    }

    New-Item -ItemType Directory -Path $BinDir -Force | Out-Null

    $extractDir = Join-Path $tmpDir "extracted"
    Expand-Archive -Path (Join-Path $tmpDir $assetName) -DestinationPath $extractDir -Force

    # Find the binary in the extracted archive
    $binaryName = "pgedge-postgres-mcp.exe"
    $source = Get-ChildItem -Path $extractDir -Recurse -Filter $binaryName | Select-Object -First 1
    if (-not $source) {
        $source = Get-ChildItem -Path $extractDir -Recurse -Filter "pgedge-postgres-mcp*" | Select-Object -First 1
    }
    if (-not $source) { Write-Fail "Binary not found in archive" }

    Copy-Item $source.FullName (Join-Path $BinDir $binaryName) -Force
    Remove-Item $tmpDir -Recurse -Force
    Set-Content -Path (Join-Path $BinDir ".version") -Value $script:Version

    Write-Ok "Binary installed: $(Join-Path $BinDir $binaryName)"
}

# --- Docker detection and installation ------------------------------------

function Test-DockerDesktopInstalled {
    # Check standard install path
    if (Test-Path "C:\Program Files\Docker\Docker\Docker Desktop.exe") { return $true }
    # Check registry uninstall key (machine-wide and per-user installs)
    $reg = Get-ItemProperty "HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\Docker Desktop" -ErrorAction SilentlyContinue
    if ($reg) { return $true }
    $reg = Get-ItemProperty "HKCU:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\Docker Desktop" -ErrorAction SilentlyContinue
    if ($reg) { return $true }
    # Check CLI on PATH (original method, still useful)
    if (Get-Command docker -ErrorAction SilentlyContinue) { return $true }
    return $false
}

function Test-DockerRunning {
    # Named pipe is reliable and doesn't need docker CLI on PATH
    if (Test-Path "\\.\pipe\docker_engine") { return $true }
    # Fallback to docker info if CLI is on PATH
    if (-not (Get-Command docker -ErrorAction SilentlyContinue)) { return $false }
    $prevPref = $ErrorActionPreference
    $ErrorActionPreference = 'SilentlyContinue'
    try {
        $null = & docker info 2>$null
        return $LASTEXITCODE -eq 0
    } catch { return $false }
    finally { $ErrorActionPreference = $prevPref }
}

function Install-DockerDesktop {
    # If Docker is already installed but just not running, don't reinstall
    if (Test-DockerDesktopInstalled) {
        Write-Host ""
        Write-Warn "Docker is installed but not running."
        Write-Host ""
        Write-Host "  Please open Docker Desktop from the Start menu and wait"
        Write-Host "  for it to finish starting (the whale icon in the taskbar"
        Write-Host "  will stop animating when it's ready)."
        Write-Host ""
        Write-Host "  Then re-run this installer."
        Write-Host ""
        exit 0
    }

    Write-Host ""
    Write-Info "Installing Docker Desktop..."
    Write-Host ""

    if (Get-Command winget -ErrorAction SilentlyContinue) {
        Write-Info "Installing via winget (this may take several minutes)..."
        & winget install -e --id Docker.DockerDesktop --no-upgrade --accept-source-agreements --accept-package-agreements
        if ($LASTEXITCODE -ne 0) {
            Write-Fail "winget install failed (exit code $LASTEXITCODE). Please install Docker Desktop manually from https://www.docker.com/products/docker-desktop/"
        }
        Write-Host ""
        Write-Info "Docker Desktop installed. Please open Docker Desktop from"
        Write-Info "the Start menu, wait for it to start, then re-run this installer."
        exit 0
    } else {
        Write-Host ""
        Write-Host "  Docker Desktop needs to be installed manually."
        Write-Host ""
        Write-Host "  1. Download it from: https://www.docker.com/products/docker-desktop/"
        Write-Host "  2. Run the installer and follow the prompts"
        Write-Host "  3. Launch Docker Desktop and wait for it to start"
        Write-Host "  4. Re-run this installer"
        Write-Host ""
        exit 0
    }
}

# --- Database choice ------------------------------------------------------

function Select-Database {
    if ($Demo) {
        Start-DemoDatabase
        return
    }
    if ($OwnDb) {
        Set-OwnDatabase
        return
    }
    if ($InstallDocker) {
        Install-DockerDesktop
        Start-DemoDatabase
        return
    }
    if ($Detect) {
        $script:DetectedInstances = Find-PostgresInstance
        if ($script:DetectedInstances.Count -eq 0) {
            Write-Host ""
            Write-Host "DETECT_NO_INSTANCES"
            Write-Host "No reachable PostgreSQL instance found. Re-run with:"
            Write-Host "  -OwnDb -DbHost HOST -DbPort PORT -DbName DB -DbUser USER -DbPass PASS"
            $script:DbConfigured = $false
            return
        }
        Connect-ExistingAuto -TargetPort $DbPort
        return
    }

    # Detect instances
    $script:DetectedInstances = Find-PostgresInstance

    # Non-interactive — output choices for Claude
    if (-not (Test-Interactive)) {
        Write-Host ""
        Write-Host "DATABASE_CHOICE_NEEDED"
        Write-Host "The MCP server needs a PostgreSQL database to connect to."
        if ($script:DetectedInstances.Count -gt 0) {
            $ports = ($script:DetectedInstances | ForEach-Object { $_.Port }) -join ", "
            Write-Host ""
            Write-Host "Detected PostgreSQL on port(s): $ports"
            Write-Host ""
            Write-Host "Options:"
            Write-Host "  1. Connect to detected instance - re-run with: -Detect"
            Write-Host "  2. Demo database - re-run with: -Demo"
            Write-Host "  3. Your own database - re-run with: -OwnDb -DbHost HOST -DbPort PORT -DbName DB -DbUser USER -DbPass PASS"
        } else {
            Write-Host "Options:"
            Write-Host "  1. Demo database - re-run with: -Demo"
            Write-Host "  2. Your own database - re-run with: -OwnDb -DbHost HOST -DbPort PORT -DbName DB -DbUser USER -DbPass PASS"
        }
        Write-Host ""
        $script:DbConfigured = $false
        return
    }

    # Interactive menu
    Write-Host ""
    Write-Host "  The MCP server needs a PostgreSQL database to connect to."

    if ($script:DetectedInstances.Count -gt 0) {
        Write-Host ""
        Write-Host "  I found PostgreSQL running on:"
        foreach ($inst in $script:DetectedInstances) {
            if ($inst.Confirmed) {
                Write-Host "    * port $($inst.Port)"
            } else {
                Write-Host "    * port $($inst.Port) (likely PostgreSQL)"
            }
        }

        $defaultPort = $script:DetectedInstances[0].Port

        Write-Host ""
        Write-Host "  Which would you like?"
        Write-Host ""
        Write-Host "    1) Connect to an existing instance (port $defaultPort)"
        Write-Host "       I'll help you pick a database."
        Write-Host ""
        Write-Host "    2) Load a sample database (Northwind - customers, orders, products)"
        Write-Host "       Requires Docker. Runs on a non-conflicting port."
        Write-Host ""
        Write-Host "    3) Connect to a different PostgreSQL database"
        Write-Host "       You'll provide the connection details."
        Write-Host ""

        $promptText = "  Enter 1, 2, or 3"
        if ($script:DetectedInstances.Count -gt 1) {
            $promptText = "  Enter 1, 2, or 3 (or 1:<port> to pick a specific instance)"
        }
        $choice = Read-Prompt $promptText "1"

        $targetPort = ""
        if ($choice -match '^1:(\d+)$') {
            $targetPort = $Matches[1]
            $choice = "1"
        }

        switch ($choice) {
            "1" { Connect-ExistingInstance -TargetPort $targetPort }
            "2" { Start-DemoDatabase }
            "3" { Set-OwnDatabase }
            default {
                Write-Info "Defaulting to existing instance..."
                Connect-ExistingInstance -TargetPort $targetPort
            }
        }
    } else {
        Write-Host ""
        Write-Host "  Which would you like?"
        Write-Host ""
        Write-Host "    1) Load a sample database (Northwind - customers, orders, products)"
        Write-Host "       Requires Docker. Great for trying things out."
        Write-Host ""
        Write-Host "    2) Connect to my own PostgreSQL database"
        Write-Host "       You'll provide the connection details."
        Write-Host ""

        $choice = Read-Prompt "  Enter 1 or 2" "1"

        switch ($choice) {
            "1" { Start-DemoDatabase }
            "2" { Set-OwnDatabase }
            default {
                Write-Info "Defaulting to sample database..."
                Start-DemoDatabase
            }
        }
    }
}

# --- Demo database setup --------------------------------------------------

function Start-DemoDatabase {
    if (Test-DockerRunning) {
        Start-DemoPostgres
        return
    }

    # Docker installed but not running?
    if (Test-DockerDesktopInstalled) {
        Write-Host ""
        Write-Warn "Docker is installed but not running."
        Write-Host ""
        Write-Host "  Please open Docker Desktop from the Start menu and wait"
        Write-Host "  for it to finish starting, then re-run this installer."
        Write-Host ""

        if (-not (Test-Interactive)) {
            Write-Host "DOCKER_NOT_RUNNING"
            Write-Host "Start Docker Desktop, then re-run with: -Demo"
            $script:DbConfigured = $false
            return
        }

        Write-Host "  Options:"
        Write-Host ""
        Write-Host "    1) I'll start Docker Desktop and re-run this later"
        Write-Host "    2) Connect to my own database instead"
        Write-Host ""

        $choice = Read-Prompt "  Enter 1 or 2" "1"
        switch ($choice) {
            "2" { Set-OwnDatabase }
            default {
                Write-Host ""
                Write-Host "  Open Docker Desktop, wait for it to start, then re-run this installer."
                Write-Host ""
                $script:DbConfigured = $false
            }
        }
        return
    }

    # Docker not installed at all
    Write-Host ""
    Write-Warn "Docker is not installed."
    Write-Host ""
    Write-Host "  The sample database runs in a Docker container."
    Write-Host "  Docker Desktop is free and takes about 5 minutes to install."
    Write-Host ""

    if (-not (Test-Interactive)) {
        Write-Host "DOCKER_NOT_FOUND"
        Write-Host "To install Docker and set up the demo, re-run with: -InstallDocker"
        Write-Host "To skip the demo and use your own database, re-run with: -OwnDb -DbHost ... -DbPort ... -DbName ... -DbUser ... -DbPass ..."
        $script:DbConfigured = $false
        return
    }

    Write-Host "  Would you like me to install Docker for you?"
    Write-Host ""
    Write-Host "    1) Yes, install Docker"
    Write-Host "    2) No, I'll connect to my own database instead"
    Write-Host "    3) No, I'll install Docker myself and re-run this later"
    Write-Host ""

    $choice = Read-Prompt "  Enter 1, 2, or 3" "3"

    switch ($choice) {
        "1" { Install-DockerDesktop; Start-DemoPostgres }
        "2" { Set-OwnDatabase }
        default {
            Write-Host ""
            Write-Host "  To install Docker Desktop:"
            Write-Host "    https://www.docker.com/products/docker-desktop/"
            Write-Host ""
            Write-Host "  After installing, re-run this installer."
            Write-Host ""
            $script:DbConfigured = $false
        }
    }
}

# --- Find a free port -----------------------------------------------------

function Find-FreePort {
    foreach ($port in 5432, 5433, 5434, 5435, 5436) {
        $inUse = Get-NetTCPConnection -LocalPort $port `
            -State Listen -ErrorAction SilentlyContinue
        if (-not $inUse) {
            return $port
        }
    }
    # Last resort: let .NET pick an ephemeral port
    try {
        $listener = [System.Net.Sockets.TcpListener]::new(
            [System.Net.IPAddress]::Loopback, 0)
        $listener.Start()
        $port = $listener.LocalEndpoint.Port
        $listener.Stop()
        return $port
    } catch {
        return 0
    }
}

# --- Detect running Postgres instances -----------------------------------

# Returns an array of [PSCustomObject]@{ Port; Confirmed }
function Find-PostgresInstance {
    $results = @()
    $hasPgReady = [bool](Get-Command pg_isready `
        -ErrorAction SilentlyContinue)

    foreach ($port in 5432, 5433, 5434, 5435, 5436) {
        $listening = $false
        $confirmed = $false

        if ($hasPgReady) {
            $prevPref = $ErrorActionPreference
            $ErrorActionPreference = 'SilentlyContinue'
            try {
                & pg_isready -h localhost -p $port -t 2 2>$null | Out-Null
                if ($LASTEXITCODE -eq 0) {
                    $listening = $true
                    $confirmed = $true
                }
            } catch {}
            finally { $ErrorActionPreference = $prevPref }
        }

        if (-not $listening) {
            $conn = Get-NetTCPConnection -LocalPort $port `
                -State Listen -ErrorAction SilentlyContinue
            if ($conn) {
                $listening = $true
            }
        }

        if ($listening) {
            $results += [PSCustomObject]@{
                Port = $port
                Confirmed = $confirmed
            }
        }
    }

    return $results
}

# --- Try passwordless auth -----------------------------------------------

function Test-PasswordlessAuth {
    param([int]$Port)
    $script:AuthUser = ""

    if (-not (Get-Command psql -ErrorAction SilentlyContinue)) {
        return $false
    }

    $prevPref = $ErrorActionPreference
    $prevPgPassword = $env:PGPASSWORD
    try {
        $env:PGPASSWORD = ""
        $ErrorActionPreference = 'SilentlyContinue'

        & psql -h localhost -p $Port -U postgres -d postgres -w -c "SELECT 1" 2>$null | Out-Null
        if ($LASTEXITCODE -eq 0) {
            $script:AuthUser = "postgres"
            return $true
        }

        $osUser = $env:USERNAME
        if ($osUser -ne "postgres") {
            & psql -h localhost -p $Port -U $osUser -d postgres -w -c "SELECT 1" 2>$null | Out-Null
            if ($LASTEXITCODE -eq 0) {
                $script:AuthUser = $osUser
                return $true
            }
        }

        return $false
    } catch {
        return $false
    } finally {
        $env:PGPASSWORD = $prevPgPassword
        $ErrorActionPreference = $prevPref
    }
}

# --- List user databases -------------------------------------------------

function Get-UserDatabases {
    param([int]$Port, [string]$User, [string]$Password)
    $prevPref = $ErrorActionPreference
    $prevPgPassword = $env:PGPASSWORD
    try {
        $env:PGPASSWORD = $Password
        $ErrorActionPreference = 'SilentlyContinue'
        $output = & psql -h localhost -p $Port -U $User -d postgres -w -t -A -c @"
SELECT datname FROM pg_database
WHERE datistemplate = false
  AND datname NOT IN ('postgres')
ORDER BY datname
"@ 2>$null
        if ($LASTEXITCODE -eq 0 -and $output) {
            return ($output | Where-Object {
                -not [string]::IsNullOrWhiteSpace($_)
            })
        }
    } catch {}
    finally {
        $env:PGPASSWORD = $prevPgPassword
        $ErrorActionPreference = $prevPref
    }
    return @()
}

# --- Connect to existing instance ----------------------------------------

function Connect-ExistingInstance {
    param([string]$TargetPort)

    $hasPsql = [bool](Get-Command psql -ErrorAction SilentlyContinue)
    $instances = $script:DetectedInstances

    # Build combined database list
    $options = @()

    foreach ($inst in $instances) {
        if ($TargetPort -and $inst.Port -ne [int]$TargetPort) { continue }

        if ($hasPsql -and (Test-PasswordlessAuth -Port $inst.Port)) {
            $dbs = @(Get-UserDatabases -Port $inst.Port `
                -User $script:AuthUser -Password "")
            $label = if ($inst.Confirmed) { "PostgreSQL" } else { "service (likely PostgreSQL)" }
            Write-Host ""
            Write-Host "    Port $($inst.Port) ($label) — connected as '$($script:AuthUser)'"
            if ($dbs.Count -gt 0) {
                foreach ($db in $dbs) {
                    $options += [PSCustomObject]@{
                        Type = "db"; Port = $inst.Port
                        User = $script:AuthUser; Password = ""
                        DbName = $db; InstIndex = $null
                    }
                    Write-Host "      $($options.Count)) $db"
                }
            } else {
                Write-Host "      (no user databases found)"
            }
        } else {
            Write-Host ""
            if ($hasPsql) {
                Write-Host "    Port $($inst.Port) — authentication required"
                $options += [PSCustomObject]@{
                    Type = "auth"; Port = $inst.Port
                    User = ""; Password = ""
                    DbName = ""; InstIndex = $null
                }
                Write-Host "      $($options.Count)) Enter credentials for this instance"
            } else {
                Write-Host "    Port $($inst.Port) — psql not installed, cannot list databases"
                $options += [PSCustomObject]@{
                    Type = "manualPort"; Port = $inst.Port
                    User = ""; Password = ""
                    DbName = ""; InstIndex = $null
                }
                Write-Host "      $($options.Count)) Enter connection details for port $($inst.Port)"
            }
        }
    }

    if ($options.Count -eq 0) {
        Write-Warn "No instances to connect to."
        $script:DbConfigured = $false
        return
    }

    Write-Host ""
    Write-Host "    Other options:"
    $options += [PSCustomObject]@{
        Type = "demo"; Port = 0; User = ""; Password = ""
        DbName = ""; InstIndex = $null
    }
    Write-Host "      $($options.Count)) Start a demo database instead (Docker, Northwind)"
    $options += [PSCustomObject]@{
        Type = "manual"; Port = 0; User = ""; Password = ""
        DbName = ""; InstIndex = $null
    }
    Write-Host "      $($options.Count)) Enter connection details manually"
    Write-Host ""

    $choice = Read-Prompt "  Enter a number" "1"

    if ($choice -match '^\d+$') {
        $idx = [int]$choice - 1
    } else {
        Write-Info "Defaulting to demo database..."
        Start-DemoDatabase
        return
    }

    if ($idx -lt 0 -or $idx -ge $options.Count) {
        Write-Warn "Invalid choice. Defaulting to demo database."
        Start-DemoDatabase
        return
    }

    $selected = $options[$idx]
    switch ($selected.Type) {
        "db" {
            $script:DbHost = "localhost"
            $script:DbPort = "$($selected.Port)"
            $script:DbName = $selected.DbName
            $script:DbUser = $selected.User
            $script:DbPass = $selected.Password
            $script:DbConfigured = $true
            Write-Ok "Using database: $($selected.DbName) on localhost:$($selected.Port) ($($selected.User))"
        }
        "auth" {
            Invoke-CredentialPrompt -Port $selected.Port
        }
        "manualPort" {
            $script:DbHost = "localhost"
            $script:DbPort = "$($selected.Port)"
            Set-OwnDatabase
        }
        "demo" {
            Start-DemoDatabase
        }
        "manual" {
            Set-OwnDatabase
        }
    }
}

# --- Prompt for credentials and list databases ----------------------------

function Invoke-CredentialPrompt {
    param([int]$Port)
    $attempts = 0

    while ($attempts -lt 2) {
        Write-Host ""
        Write-Host "  Connection to port $Port requires authentication."
        Write-Host ""

        $user = Read-Prompt "  Username [postgres]" "postgres"

        if (Test-Interactive) {
            if ($PSVersionTable.PSVersion.Major -ge 7) {
                $pass = Read-Host -Prompt "  Password" -MaskInput
            } else {
                $secure = Read-Host -Prompt "  Password" -AsSecureString
                $bstr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secure)
                try {
                    $pass = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($bstr)
                } finally {
                    [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($bstr)
                }
            }
        } else {
            $pass = ""
        }

        $prevPref = $ErrorActionPreference
        $prevPgPassword = $env:PGPASSWORD
        try {
            $env:PGPASSWORD = $pass
            $ErrorActionPreference = 'SilentlyContinue'
            & psql -h localhost -p $Port -U $user -d postgres -w -c "SELECT 1" 2>$null | Out-Null
            if ($LASTEXITCODE -eq 0) {
                Write-Ok "Connected to port $Port as '$user'"

                $dbs = @(Get-UserDatabases -Port $Port -User $user -Password $pass)
                if ($dbs.Count -gt 0) {
                    Write-Host ""
                    Write-Host "  Databases on port ${Port}:"
                    for ($i = 0; $i -lt $dbs.Count; $i++) {
                        Write-Host "    $($i + 1)) $($dbs[$i])"
                    }
                    Write-Host ""

                    $dbChoice = Read-Prompt "  Enter a number (or type a database name)" "1"
                    if ($dbChoice -match '^\d+$') {
                        $dbIdx = [int]$dbChoice - 1
                        if ($dbIdx -ge 0 -and $dbIdx -lt $dbs.Count) {
                            $script:DbName = $dbs[$dbIdx]
                        } else {
                            $script:DbName = $dbChoice
                        }
                    } else {
                        $script:DbName = $dbChoice
                    }
                } else {
                    Write-Host ""
                    Write-Host "  No user databases found on port $Port."
                    Write-Host ""
                    $script:DbName = Read-Prompt "  Database name" ""
                    if (-not $script:DbName) {
                        Write-Warn "Database name is required."
                        $script:DbConfigured = $false
                        return
                    }
                }

                $script:DbHost = "localhost"
                $script:DbPort = "$Port"
                $script:DbUser = $user
                $script:DbPass = $pass
                $script:DbConfigured = $true
                Write-Ok "Using database: $($script:DbName) on localhost:$Port ($user)"
                return
            }
        } catch {}
        finally {
            $env:PGPASSWORD = $prevPgPassword
            $ErrorActionPreference = $prevPref
        }

        Write-Warn "Authentication failed."
        $attempts++
    }

    Write-Warn "Could not connect to port $Port. Try the manual option."
    Write-Host ""
    Set-OwnDatabase
}

# --- Auto-connect (non-interactive -Detect mode) --------------------------

function Connect-ExistingAuto {
    param([string]$TargetPort)

    $hasPsql = [bool](Get-Command psql -ErrorAction SilentlyContinue)
    $matchedTarget = if ([string]::IsNullOrWhiteSpace($TargetPort)) {
        $true } else { $false }

    foreach ($inst in $script:DetectedInstances) {
        if ($TargetPort -and $inst.Port -ne [int]$TargetPort) {
            continue
        }
        if ($TargetPort) { $matchedTarget = $true }

        if ($hasPsql -and (Test-PasswordlessAuth -Port $inst.Port)) {
            if ($script:DbName) {
                $dbs = @(Get-UserDatabases -Port $inst.Port `
                    -User $script:AuthUser -Password "")
                if ($dbs -ccontains $script:DbName) {
                    $script:DbHost = "localhost"
                    $script:DbPort = "$($inst.Port)"
                    $script:DbUser = $script:AuthUser
                    $script:DbPass = ""
                    $script:DbConfigured = $true
                    Write-Ok "Using database: $($script:DbName) on localhost:$($inst.Port) ($($script:AuthUser))"
                    return
                }
                Write-Warn "Database '$($script:DbName)' not found on port $($inst.Port)"
                continue
            }

            $dbs = @(Get-UserDatabases -Port $inst.Port `
                -User $script:AuthUser -Password "")
            if ($dbs.Count -gt 0) {
                $script:DbHost = "localhost"
                $script:DbPort = "$($inst.Port)"
                $script:DbName = $dbs[0]
                $script:DbUser = $script:AuthUser
                $script:DbPass = ""
                $script:DbConfigured = $true
                Write-Ok "Auto-detected: $($dbs[0]) on localhost:$($inst.Port) ($($script:AuthUser))"
                return
            }
        }
    }

    Write-Host ""
    if (-not $hasPsql) {
        Write-Host "DETECT_PSQL_MISSING"
        Write-Host "psql is required for -Detect auto-connection."
    } elseif (-not $matchedTarget) {
        Write-Host "DETECT_PORT_NOT_FOUND"
        Write-Host "No detected PostgreSQL instance on port $TargetPort."
    } else {
        Write-Host "DETECT_AUTH_FAILED"
        Write-Host "Could not authenticate to any detected instance."
    }
    Write-Host "Re-run with explicit credentials:"
    Write-Host "  -OwnDb -DbHost HOST -DbPort PORT -DbName DB -DbUser USER -DbPass PASS"
    $script:DbConfigured = $false
}

# --- Remove old demo containers ------------------------------------------

function Remove-OldDemos {
    try {
        $old = & docker ps -a --filter "name=pgedge-demo-" --format '{{.Names}}' 2>$null
    } catch { return }
    if (-not $old) { return }
    foreach ($name in $old) {
        if ([string]::IsNullOrWhiteSpace($name)) { continue }
        Write-Info "Removing old demo container: $name"
        & docker stop $name 2>$null | Out-Null
        & docker rm -v $name 2>$null | Out-Null
    }
}

# --- Start demo Postgres container ----------------------------------------

function Start-DemoPostgres {
    # Clean up any old demo containers from previous installs
    Remove-OldDemos

    # Generate a unique container name for this run
    $ContainerName = "pgedge-demo-$([DateTimeOffset]::UtcNow.ToUnixTimeSeconds())"

    $script:DemoPort = Find-FreePort
    if ($script:DemoPort -eq 0) {
        Write-Warn "Could not find a free port for the demo database."
        $script:DbConfigured = $false
        return
    }

    if ($script:DemoPort -ne 5432) {
        Write-Info "Port 5432 is in use (probably an existing Postgres instance)."
        Write-Info "Using port $($script:DemoPort) for the demo database instead."
    }

    New-Item -ItemType Directory -Path $DemoDir -Force | Out-Null

    # Write docker-compose.yml
    # Note: $$POSTGRES_USER inside the configs content is docker-compose syntax
    # (double-dollar escapes the dollar sign). Port is interpolated directly.
    $composeContent = @"
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
      - "$($script:DemoPort):5432"
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
      psql -v ON_ERROR_STOP=1 --username "`$`$POSTGRES_USER" --dbname "`$`$POSTGRES_DB" -f /tmp/northwind.sql
      rm -f /tmp/northwind.sql
      echo "Northwind dataset loaded"
  enable-extensions:
    content: |-
      #!/usr/bin/env bash
      set -e
      psql -v ON_ERROR_STOP=1 --username "`$`$POSTGRES_USER" --dbname "`$`$POSTGRES_DB" \
        -c "CREATE EXTENSION IF NOT EXISTS pg_stat_statements;"
      echo "Extensions enabled"
"@

    $composeContent = $composeContent -replace 'PGEDGE_CONTAINER_NAME', $ContainerName
    $composeContent | Set-Content (Join-Path $DemoDir "docker-compose.yml") -Encoding UTF8

    Write-Host ""
    Write-Info "Starting demo Postgres ($ContainerName) with Northwind sample data on port $($script:DemoPort)..."
    Write-Info "(first run downloads the image - this may take a minute)"
    Write-Host ""

    $composeFile = Join-Path $DemoDir "docker-compose.yml"
    $started = $false
    try {
        & docker compose -f $composeFile up -d 2>$null
        if ($LASTEXITCODE -eq 0) { $started = $true }
    } catch {}

    if (-not $started) {
        try {
            & docker-compose -f $composeFile up -d 2>$null
            if ($LASTEXITCODE -eq 0) { $started = $true }
        } catch {}
    }

    if (-not $started) {
        Write-Warn "Failed to start demo database."
        $script:DbConfigured = $false
        return
    }

    Write-Info "Waiting for database to be ready..."
    for ($i = 0; $i -lt 24; $i++) {
        try {
            & docker exec $ContainerName pg_isready -U demo -d northwind 2>$null | Out-Null
            if ($LASTEXITCODE -eq 0) {
                Write-Ok "Demo database ready (northwind on localhost:$($script:DemoPort), container: $ContainerName)"
                $script:DbHost = "localhost"; $script:DbPort = "$($script:DemoPort)"
                $script:DbName = "northwind"; $script:DbUser = "demo"
                $script:DbPass = "demo123"; $script:DbConfigured = $true
                return
            }
        } catch {}
        Start-Sleep -Seconds 5
    }

    Write-Warn "Database is still starting. It may need another minute."
    $script:DbHost = "localhost"; $script:DbPort = "$($script:DemoPort)"
    $script:DbName = "northwind"; $script:DbUser = "demo"
    $script:DbPass = "demo123"; $script:DbConfigured = $true
}

# --- Database connection test ---------------------------------------------

function Test-DbConnection {
    param([string]$Host_, [int]$Port)
    Write-Info "Testing connection to ${Host_}:${Port}..."
    try {
        $tcp = New-Object System.Net.Sockets.TcpClient
        $completed = $tcp.ConnectAsync($Host_, $Port).Wait(3000)
        if ($completed -and $tcp.Connected) {
            $tcp.Close()
            Write-Ok "Connection to ${Host_}:${Port} succeeded"
            return $true
        }
        $tcp.Close()
    } catch {
        try { if ($tcp) { $tcp.Close() } } catch {}
    }
    return $false
}

function Confirm-OwnDbConnection {
    if (Test-DbConnection $script:DbHost ([int]$script:DbPort)) { return }

    Write-Host ""
    Write-Warn "Could not reach $($script:DbHost):$($script:DbPort) (TCP connection failed)"
    Write-Host ""

    if (-not (Test-Interactive)) {
        Write-Warn "Continuing anyway — verify your connection details are correct."
        return
    }

    Write-Host "  What would you like to do?"
    Write-Host ""
    Write-Host "    1) Re-enter connection details"
    Write-Host "    2) Continue anyway (I'll fix it later)"
    Write-Host ""

    $choice = Read-Prompt "  Enter 1 or 2" "2"
    switch ($choice) {
        "1" {
            # Clear previous values so Set-OwnDatabase re-prompts
            $script:DbHost = ""; $script:DbPort = ""; $script:DbName = ""
            $script:DbUser = ""; $script:DbPass = ""
            Set-OwnDatabase
            return
        }
        default { Write-Warn "Continuing — you can update ~/.claude.json later with the correct details." }
    }
}

# --- Own database setup ---------------------------------------------------

function Set-OwnDatabase {
    # If connection details were provided via flags
    if ($script:DbHost -and $script:DbName -and $script:DbUser) {
        if (-not $script:DbPort) { $script:DbPort = "5432" }
        $script:DbConfigured = $true
        Write-Ok "Using database: $($script:DbName) on $($script:DbHost):$($script:DbPort)"
        Confirm-OwnDbConnection
        return
    }

    if (-not (Test-Interactive)) {
        Write-Host ""
        Write-Host "OWN_DATABASE_CHOSEN"
        Write-Host "Re-run with connection details:"
        Write-Host "  -OwnDb -DbHost HOST -DbPort PORT -DbName DB -DbUser USER -DbPass PASS"
        $script:DbConfigured = $false
        return
    }

    Write-Host ""
    Write-Host "  Enter your PostgreSQL connection details:"
    Write-Host ""

    $defaultHost = if ($script:DbHost) { $script:DbHost } else { "localhost" }
    $defaultPort = if ($script:DbPort) { $script:DbPort } else { "5432" }
    $script:DbHost = Read-Prompt "  Host [$defaultHost]" $defaultHost
    $script:DbPort = Read-Prompt "  Port [$defaultPort]" $defaultPort
    $script:DbName = Read-Prompt "  Database name" ""
    if (-not $script:DbName) { Write-Warn "Database name is required."; $script:DbConfigured = $false; return }
    $script:DbUser = Read-Prompt "  Username" ""
    if (-not $script:DbUser) { Write-Warn "Username is required."; $script:DbConfigured = $false; return }
    if (Test-Interactive) {
        if ($PSVersionTable.PSVersion.Major -ge 7) {
            $script:DbPass = Read-Host -Prompt "  Password" -MaskInput
        } else {
            $secure = Read-Host -Prompt "  Password" -AsSecureString
            $bstr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secure)
            try {
                $script:DbPass = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($bstr)
            } finally {
                [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($bstr)
            }
        }
    } else {
        $script:DbPass = ""
    }

    $script:DbConfigured = $true
    Write-Ok "Using database: $($script:DbName) on $($script:DbHost):$($script:DbPort)"
    Confirm-OwnDbConnection
}

# --- Configure Claude Code ------------------------------------------------

function Set-ClaudeCodeConfig {
    $mcpJson = Join-Path $env:USERPROFILE ".claude.json"
    $binaryPath = (Join-Path $BinDir "pgedge-postgres-mcp.exe").Replace('\', '/')

    $pgHost = if ($script:DbHost) { $script:DbHost } else { "localhost" }
    $pgPort = if ($script:DbPort) { $script:DbPort } else { "5432" }
    $pgDb   = if ($script:DbName) { $script:DbName } else { "your_database" }
    $pgUser = if ($script:DbUser) { $script:DbUser } else { "your_user" }
    $pgPass = if ($script:DbPass) { $script:DbPass } else { "your_password" }

    if (Test-Path $mcpJson) {
        try {
            $raw = Get-Content $mcpJson -Raw
            $existing = $raw | ConvertFrom-Json

            # Convert PSCustomObject to allow adding properties
            if (-not $existing.mcpServers) {
                $existing | Add-Member -NotePropertyName "mcpServers" -NotePropertyValue (New-Object PSObject) -Force
            }

            # Add or overwrite the pgedge entry
            $pgedgeObj = New-Object PSObject
            $pgedgeObj | Add-Member -NotePropertyName "command" -NotePropertyValue $binaryPath
            $envObj = New-Object PSObject
            $envObj | Add-Member -NotePropertyName "PGHOST"     -NotePropertyValue $pgHost
            $envObj | Add-Member -NotePropertyName "PGPORT"     -NotePropertyValue $pgPort
            $envObj | Add-Member -NotePropertyName "PGDATABASE" -NotePropertyValue $pgDb
            $envObj | Add-Member -NotePropertyName "PGUSER"     -NotePropertyValue $pgUser
            $envObj | Add-Member -NotePropertyName "PGPASSWORD" -NotePropertyValue $pgPass
            $pgedgeObj | Add-Member -NotePropertyName "env" -NotePropertyValue $envObj

            if ($existing.mcpServers.PSObject.Properties["pgedge"]) {
                $existing.mcpServers.pgedge = $pgedgeObj
            } else {
                $existing.mcpServers | Add-Member -NotePropertyName "pgedge" -NotePropertyValue $pgedgeObj
            }

            $existing | ConvertTo-Json -Depth 10 | Set-Content $mcpJson -Encoding UTF8
            Write-Ok "Claude Code: configured in ~/.claude.json (available in all projects)"
            return
        } catch {
            Write-Warn "~/.claude.json contains invalid JSON — backing up to .claude.json.bak"
            Rename-Item $mcpJson "$mcpJson.bak" -Force
            # Fall through to write fresh config
        }
    }

    # Write fresh config
    $config = New-Object PSObject
    $mcpServers = New-Object PSObject
    $pgedgeObj = New-Object PSObject
    $pgedgeObj | Add-Member -NotePropertyName "command" -NotePropertyValue $binaryPath
    $envObj = New-Object PSObject
    $envObj | Add-Member -NotePropertyName "PGHOST"     -NotePropertyValue $pgHost
    $envObj | Add-Member -NotePropertyName "PGPORT"     -NotePropertyValue $pgPort
    $envObj | Add-Member -NotePropertyName "PGDATABASE" -NotePropertyValue $pgDb
    $envObj | Add-Member -NotePropertyName "PGUSER"     -NotePropertyValue $pgUser
    $envObj | Add-Member -NotePropertyName "PGPASSWORD" -NotePropertyValue $pgPass
    $pgedgeObj | Add-Member -NotePropertyName "env" -NotePropertyValue $envObj
    $mcpServers | Add-Member -NotePropertyName "pgedge" -NotePropertyValue $pgedgeObj
    $config | Add-Member -NotePropertyName "mcpServers" -NotePropertyValue $mcpServers

    $config | ConvertTo-Json -Depth 10 | Set-Content $mcpJson -Encoding UTF8
    Write-Ok "Claude Code: configured in ~/.claude.json (available in all projects)"
}

# --- Configure Claude Desktop ---------------------------------------------

function Set-ClaudeDesktopConfig {
    $binaryPath = (Join-Path $BinDir "pgedge-postgres-mcp.exe").Replace('\', '/')
    $configFile = Join-Path $env:APPDATA "Claude\claude_desktop_config.json"
    $configDir = Split-Path $configFile

    if (-not (Test-Path $configDir)) {
        Write-Info "Claude Desktop not detected - skipping config"
        return
    }

    $pgHost = if ($script:DbHost) { $script:DbHost } else { "localhost" }
    $pgPort = if ($script:DbPort) { $script:DbPort } else { "5432" }
    $pgDb   = if ($script:DbName) { $script:DbName } else { "your_database" }
    $pgUser = if ($script:DbUser) { $script:DbUser } else { "your_user" }
    $pgPass = if ($script:DbPass) { $script:DbPass } else { "your_password" }

    # Build the pgedge entry as PSCustomObject (PS 5.1 compatible)
    $pgedgeObj = New-Object PSObject
    $pgedgeObj | Add-Member -NotePropertyName "command" -NotePropertyValue $binaryPath
    $envObj = New-Object PSObject
    $envObj | Add-Member -NotePropertyName "PGHOST"     -NotePropertyValue $pgHost
    $envObj | Add-Member -NotePropertyName "PGPORT"     -NotePropertyValue $pgPort
    $envObj | Add-Member -NotePropertyName "PGDATABASE" -NotePropertyValue $pgDb
    $envObj | Add-Member -NotePropertyName "PGUSER"     -NotePropertyValue $pgUser
    $envObj | Add-Member -NotePropertyName "PGPASSWORD" -NotePropertyValue $pgPass
    $pgedgeObj | Add-Member -NotePropertyName "env" -NotePropertyValue $envObj

    $config = $null
    if (Test-Path $configFile) {
        try {
            $raw = Get-Content $configFile -Raw
            $config = $raw | ConvertFrom-Json
        } catch {
            Write-Warn "Claude Desktop config ($configFile) contains invalid JSON — backing up to $configFile.bak"
            Rename-Item $configFile "$configFile.bak" -Force
        }
    }

    if (-not $config) {
        $config = New-Object PSObject
    }

    if (-not $config.mcpServers) {
        $config | Add-Member -NotePropertyName "mcpServers" -NotePropertyValue (New-Object PSObject) -Force
    }

    if ($config.mcpServers.PSObject.Properties["pgedge"]) {
        $config.mcpServers.pgedge = $pgedgeObj
    } else {
        $config.mcpServers | Add-Member -NotePropertyName "pgedge" -NotePropertyValue $pgedgeObj
    }

    New-Item -ItemType Directory -Path $configDir -Force | Out-Null
    $config | ConvertTo-Json -Depth 10 | Set-Content $configFile -Encoding UTF8

    Write-Ok "Claude Desktop: configured (restart Claude Desktop to activate)"
}

# --- Summary --------------------------------------------------------------

function Write-UpdateSummary {
    Write-Host (([string][char]0x2550) * 54)
    Write-Host "  Update complete!"
    Write-Host (([string][char]0x2550) * 54)
    Write-Host ""
    Write-Host "  Your existing database configuration is unchanged."
    Write-Host ""
    Write-Host "  Claude Code:    start a new conversation to use"
    Write-Host "                  the updated server"
    Write-Host "  Claude Desktop: restart the app to pick up the"
    Write-Host "                  new version"
    Write-Host ""
    Write-Host (([string][char]0x2550) * 54)
    Write-Host ""
}

function Write-Summary {
    Write-Host ""
    Write-Host (([string][char]0x2550) * 54) # horizontal double line
    Write-Host "  Installation complete!"
    Write-Host (([string][char]0x2550) * 54)
    Write-Host ""
    Write-Host "  Binary:   $(Join-Path $BinDir 'pgedge-postgres-mcp.exe')"

    if ($script:DbConfigured) {
        Write-Host "  Database: $($script:DbName) on $($script:DbHost):$($script:DbPort) ($($script:DbUser))"
        Write-Host ""
        Write-Host "  Try asking Claude:"
        Write-Host '    "What tables are in my database?"'
        Write-Host '    "Show me the top 10 products by sales"'
        Write-Host '    "Which customers have placed more than 5 orders?"'
    } else {
        Write-Host "  Database: not yet configured"
        Write-Host ""
        Write-Host "  To configure later, edit:"
        Write-Host "    Claude Code:    ~/.claude.json"
        Write-Host "    Claude Desktop: $env:APPDATA\Claude\claude_desktop_config.json"
    }

    Write-Host ""
    Write-Host "  Claude Code:    ready - start a new conversation"
    Write-Host "  Claude Desktop: restart the app, then start chatting"
    Write-Host ""
    Write-Host (([string][char]0x2550) * 54)
    Write-Host ""
}

# --- Stop stale MCP server processes --------------------------------------

function Stop-StaleProcesses {
    $procs = Get-Process -Name 'pgedge-postgres-mcp' -ErrorAction SilentlyContinue
    if ($procs) {
        $count = @($procs).Count
        Write-Info "Stopping $count running MCP server process(es)..."
        $procs | Stop-Process -Force -ErrorAction SilentlyContinue
        Start-Sleep -Seconds 1
    }
}

# --- Check for existing installation -------------------------------------

function Test-ExistingInstall {
    $explicitReconfigure = $Detect -or $Demo -or $OwnDb -or $InstallDocker
    $binaryExt = if ($script:OS -eq "windows") { ".exe" } else { "" }
    $binary = Join-Path $BinDir "pgedge-postgres-mcp$binaryExt"
    if (-not (Test-Path $binary)) { return $false }

    $versionFile = Join-Path $BinDir ".version"
    $installedVersion = ""
    if (Test-Path $versionFile) {
        $raw = Get-Content $versionFile -Raw
        if ($null -ne $raw) { $installedVersion = $raw.Trim() }
    }

    if ($installedVersion -eq $script:Version) {
        Write-Host ""
        Write-Ok "pgEdge MCP Server $($script:Version) is already up to date."
        Write-Host ""
        if ($explicitReconfigure) {
            Select-Database
            if ($script:DbConfigured) {
                Write-Host ""
                Set-ClaudeCodeConfig
                Set-ClaudeDesktopConfig
                Write-Summary
            }
        } elseif (Test-Interactive) {
            $reconfigure = Read-Prompt "  Want to reconfigure the database connection? (y/n)" "n"
            if ($reconfigure -match '^[Yy]') {
                Write-Host ""
                Select-Database
                if ($script:DbConfigured) {
                    Write-Host ""
                    Set-ClaudeCodeConfig
                    Set-ClaudeDesktopConfig
                    Write-Summary
                }
            } else {
                Write-Host ""
                Write-Info "Nothing to do."
                Write-Host ""
                Write-Host "  Claude Code:    start a new conversation"
                Write-Host "  Claude Desktop: restart the app, then start chatting"
                Write-Host ""
            }
        } else {
            Write-Info "Already up to date. Nothing to do."
        }
        return $true
    }

    Write-Host ""
    if ($installedVersion) {
        Write-Info "pgEdge MCP Server $installedVersion is installed."
    } else {
        Write-Info "pgEdge MCP Server (unknown version) is installed."
    }
    Write-Ok "A newer version ($($script:Version)) is available."
    Write-Host ""
    if (Test-Interactive) {
        $update = Read-Prompt "  Update? (y/n)" "y"
        if ($update -match '^[Yy]') {
            Stop-StaleProcesses
            Install-Binary
            Write-Ok "Updated to $($script:Version)."
            Write-Host ""
            if (-not $explicitReconfigure) { Write-UpdateSummary }
        } else {
            Write-Host ""
            Write-Info "Skipping update. Exiting."
            return $true
        }
    } else {
        Stop-StaleProcesses
        Install-Binary
        Write-Ok "Updated to $($script:Version)."
        Write-Host ""
        if (-not $explicitReconfigure) { Write-UpdateSummary }
    }

    if ($explicitReconfigure) {
        Select-Database
        if ($script:DbConfigured) {
            Write-Host ""
            Set-ClaudeCodeConfig
            Set-ClaudeDesktopConfig
            Write-Summary
        }
    }
    return $true
}

# --- Main -----------------------------------------------------------------

function Main {
    Write-Host ""
    Write-Host (([string][char]0x2550) * 54)
    Write-Host "  pgEdge MCP Server - Installer"
    Write-Host (([string][char]0x2550) * 54)
    Write-Host ""
    Write-Host "  This will install the pgEdge MCP Server so you can"
    Write-Host "  query PostgreSQL databases using natural language"
    Write-Host "  in Claude Code or Claude Desktop."
    Write-Host ""

    $script:DbConfigured = $false

    Get-Platform
    Get-LatestVersion

    if (Test-ExistingInstall) { return }

    Install-Binary

    Write-Host ""
    Select-Database

    if ($script:DbConfigured) {
        Write-Host ""
        Set-ClaudeCodeConfig
        Set-ClaudeDesktopConfig
        Write-Summary
    }
}

Main
