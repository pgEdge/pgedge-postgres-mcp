# Knowledgebase Directory

This directory is used during Docker image builds to include a pre-built
knowledgebase database in the container image.

## Usage

To build an image with a knowledgebase included:

1. Build or download a knowledgebase database file
2. Place it at `kb/kb.db` in this directory
3. Build the Docker image normally

```bash
# Example: Build the image with your local KB
docker build -f Dockerfile.server -t mcp-server:with-kb .
```

Alternatively, use the `KB_SOURCE` build argument to download from a URL:

```bash
docker build -f Dockerfile.server \
    --build-arg KB_SOURCE=https://example.com/kb.db \
    -t mcp-server:with-kb .
```

## Notes

- The `kb.db` file is not committed to version control (see `.gitignore`)
- If no `kb.db` file is present, the base image is built without a
  knowledgebase
- See the [KB Builder documentation](../docs/guide/knowledgebase.md) for
  instructions on building a knowledgebase
