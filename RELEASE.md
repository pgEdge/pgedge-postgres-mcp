# Release Process

This document describes how to create releases for the pgEdge Natural
Language Agent using GoReleaser.

## Prerequisites

- Go 1.24 or higher
- Node.js 20 or higher
- GoReleaser installed (`go install github.com/goreleaser/goreleaser@latest`)
- GitHub token with repo write permissions (for creating releases)

## Release Artifacts

Each release creates the following artifacts:

### 1. Server Binaries (`pgedge-nla-server_*`)

- Linux: amd64, arm64
- macOS: amd64, arm64
- Windows: amd64

Architecture-specific archives containing:

- `pgedge-mcp-server` binary
- Documentation (README.md, LICENCE.md, docs/)
- Example configuration files

### 2. CLI Client Binaries (`pgedge-nla-cli_*`)

- Linux: amd64, arm64
- macOS: amd64, arm64
- Windows: amd64

Architecture-specific archives containing:

- `pgedge-nla-cli` binary
- Documentation (README.md, LICENCE.md)
- CLI client usage guide

### 3. Web UI (`pgedge-nla-web_*_noarch`)

Platform-independent archive containing:

- Pre-built static web assets (from `web/dist/`)
- Web UI documentation
- License

This is a `noarch` package as it contains only static HTML/CSS/JS
files.

### 4. KB Builder (`pgedge-nla-kb-builder_*`)

- Linux: amd64, arm64
- macOS: amd64, arm64

Architecture-specific archives containing:

- `pgedge-nla-kb-builder` binary (CGO enabled for SQLite)
- Example configuration
- KB source files

## Testing Locally

Before creating a release, test the build process locally:

```bash
# Run the test script
./test-goreleaser.sh
```

This will:

1. Build the Web UI
2. Run all tests
3. Create a snapshot build without publishing
4. Generate all release artifacts in `dist/`

Verify the generated archives:

```bash
# Extract and test server binary
tar -xzf dist/pgedge-nla-server_*_linux_x86_64.tar.gz
cd pgedge-nla-server_*_linux_x86_64
./pgedge-mcp-server --help

# Extract and test CLI binary
tar -xzf dist/pgedge-nla-cli_*_linux_x86_64.tar.gz
cd pgedge-nla-cli_*_linux_x86_64
./pgedge-nla-cli --help

# Extract and verify web UI
tar -xzf dist/pgedge-nla-web_*_noarch.tar.gz
ls -la web/dist/
```

## Creating a Release

### 1. Prepare the Release

Update version information and changelog:

```bash
# Review changes since last release
git log $(git describe --tags --abbrev=0)..HEAD --oneline

# Update documentation if needed
vim README.md
vim docs/index.md
```

### 2. Create and Push Tag

```bash
# Create an annotated tag
git tag -a v1.0.0 -m "Release v1.0.0"

# Push the tag to trigger release workflow
git push origin v1.0.0
```

### 3. Automated Release Process

When you push a tag starting with `v`, the GitHub Actions workflow
(`.github/workflows/release.yml`) will automatically:

1. Check out the code
2. Set up Go and Node.js
3. Install dependencies
4. Build the Web UI
5. Run tests
6. Execute GoReleaser to:
   - Build binaries for all platforms
   - Create archives
   - Generate checksums
   - Create GitHub release with changelog
   - Upload all artifacts

### 4. Monitor the Release

Check the GitHub Actions workflow:

```
https://github.com/pgEdge/pgedge-nla/actions
```

Once complete, verify the release:

```
https://github.com/pgEdge/pgedge-nla/releases
```

## Release Workflow Details

### GitHub Actions Workflow

The `.github/workflows/release.yml` workflow:

- **Trigger**: Push of tags matching `v*`
- **Permissions**: Writes to releases and packages
- **Steps**:
  1. Checkout with full history
  2. Set up Go 1.24
  3. Set up Node.js 20
  4. Build Web UI
  5. Run tests
  6. Execute GoReleaser

### GoReleaser Configuration

The `.goreleaser.yaml` configuration:

- **Builds**: Three separate binaries (server, CLI, kb-builder)
- **Archives**: Platform-specific tarballs/zips
- **Checksums**: SHA256 for all artifacts
- **Changelog**: Auto-generated from conventional commits
- **Source**: Includes source archive

## Version Numbering

Follow semantic versioning (semver):

- **Major** (v1.0.0 → v2.0.0): Breaking changes
- **Minor** (v1.0.0 → v1.1.0): New features, backwards-compatible
- **Patch** (v1.0.0 → v1.0.1): Bug fixes, backwards-compatible

## Changelog Format

Use conventional commit format for automatic changelog generation:

```bash
feat: Add new similarity search feature
fix: Resolve authentication token expiry issue
sec: Update dependencies to address CVE-2024-xxxxx
docs: Update deployment guide
test: Add integration tests for CLI client
chore: Update CI/CD configuration
```

## Post-Release Tasks

After a successful release:

1. **Verify Downloads**: Test downloading and extracting artifacts
2. **Update Documentation**: Ensure docs reflect the new version
3. **Announce**: Update README badges, notify users
4. **Monitor**: Watch for issues from users

## Troubleshooting

### Build Fails in CI

- Check GitHub Actions logs
- Verify all tests pass locally
- Ensure Web UI builds successfully

### GoReleaser Errors

```bash
# Validate configuration
goreleaser check

# Test locally first
./test-goreleaser.sh
```

### Missing Artifacts

- Verify `.goreleaser.yaml` includes all necessary files
- Check that Web UI `dist/` directory exists
- Ensure example configs are present

### CGO Issues (KB Builder)

The kb-builder requires CGO for SQLite:

- Linux: Uses system gcc
- macOS: Uses Xcode command-line tools
- Cross-compilation may require additional setup

## Manual Release (Fallback)

If automated release fails, you can release manually:

```bash
# Set GitHub token
export GITHUB_TOKEN="your-github-token"

# Run goreleaser manually
goreleaser release --clean
```

## Related Documentation

- [GoReleaser Documentation](https://goreleaser.com/)
- [GitHub Actions Documentation](https://docs.github.com/en/actions)
- [Semantic Versioning](https://semver.org/)
- [Conventional Commits](https://www.conventionalcommits.org/)
