# pgEdge Natural Language Agent - Container Deployment Guide

This document provides guidance for deploying the pgEdge Natural Language Agent
using Docker containers and Kubernetes (Helm).

## Table of Contents

- [Docker Images](#docker-images)
  - [Image Variants](#image-variants)
- [Docker Compose Deployment](#docker-compose-deployment)
- [Kubernetes Deployment (Helm)](#kubernetes-deployment-helm-chart)
- [Container Registry](#container-registry)
- [Security Best Practices](#security-best-practices)
- [Monitoring and Troubleshooting](#monitoring-and-troubleshooting)

---

## Docker Images

The pgEdge Natural Language Agent provides pre-built Dockerfiles for all
components.

### Available Dockerfiles

The following Dockerfiles are available in the repository root:

- **[Dockerfile.server](Dockerfile.server)** - MCP server with PostgreSQL
  tools
- **[Dockerfile.cli](Dockerfile.cli)** - Command-line chat client
- **[Dockerfile.web](Dockerfile.web)** - React-based web UI with Nginx

All images use multi-stage builds for minimal size and run as non-root users
for security.

### Building Container Images

```bash
# Build all images
docker build -f Dockerfile.server -t ghcr.io/pgedge/mcp-server:latest .
docker build -f Dockerfile.cli -t ghcr.io/pgedge/nla-cli:latest .
docker build -f Dockerfile.web -t ghcr.io/pgedge/nla-web:latest .

# Tag for versioning
VERSION=$(git describe --tags --always)
docker tag ghcr.io/pgedge/mcp-server:latest ghcr.io/pgedge/mcp-server:${VERSION}
docker tag ghcr.io/pgedge/nla-cli:latest ghcr.io/pgedge/nla-cli:${VERSION}
docker tag ghcr.io/pgedge/nla-web:latest ghcr.io/pgedge/nla-web:${VERSION}

# Build with BuildKit for better performance
DOCKER_BUILDKIT=1 docker build -f Dockerfile.server -t ghcr.io/pgedge/mcp-server:latest .
```

### Image Variants

The MCP server image is available in two variants:

#### Base Image (without Knowledgebase)

The base image contains only the MCP server without a pre-built knowledgebase
database. Use this variant when:

- You want the smallest possible image (~50MB)
- You will provide your own knowledgebase via volume mount
- You don't need knowledgebase search functionality

```bash
docker pull ghcr.io/pgedge/mcp-server:latest
```

#### Image with Knowledgebase

The `-with-kb` variant includes a pre-built knowledgebase database with
documentation for PostgreSQL, pgEdge products, and related tools. Use this
when:

- You want knowledgebase search available out-of-the-box
- You prefer simplicity over image size (~300-500MB)
- You're setting up a quick demo or development environment

```bash
docker pull ghcr.io/pgedge/mcp-server:latest-with-kb
```

#### Available Tags

| Tag | Description |
|-----|-------------|
| `latest` | Latest base image (no KB) |
| `latest-with-kb` | Latest image with pre-built KB |
| `v1.0.0` | Version 1.0.0 base image |
| `v1.0.0-with-kb` | Version 1.0.0 with KB |

#### Building Images with Knowledgebase

You can build your own image with a custom knowledgebase using the
`KB_SOURCE` build argument:

**From a local file:**

```bash
# First, build or obtain your KB database
./bin/pgedge-nla-kb-builder -c my-config.yaml

# Place the database in the kb/ directory
cp pgedge-nla-kb.db kb/kb.db

# Build the image (automatically includes kb/kb.db if present)
docker build -f Dockerfile.server -t mcp-server:custom-kb .
```

**From a URL:**

```bash
docker build -f Dockerfile.server \
    --build-arg KB_SOURCE=https://example.com/path/to/kb.db \
    -t mcp-server:custom-kb .
```

#### Using Knowledgebase in Containers

When using the `-with-kb` image, enable knowledgebase search with:

```bash
docker run -d \
  -e PGEDGE_KB_ENABLED=true \
  -e PGEDGE_KB_EMBEDDING_PROVIDER=voyage \
  -e PGEDGE_KB_VOYAGE_API_KEY=${VOYAGE_API_KEY} \
  ghcr.io/pgedge/mcp-server:latest-with-kb
```

**Note:** Even with the built-in KB, you still need an embedding provider API
key for similarity search queries.

When using the base image with a custom KB, mount it as a volume:

```bash
docker run -d \
  -v ./my-kb.db:/usr/share/pgedge/nla-kb/kb.db:ro \
  -e PGEDGE_KB_ENABLED=true \
  -e PGEDGE_KB_DATABASE_PATH=/usr/share/pgedge/nla-kb/kb.db \
  -e PGEDGE_KB_EMBEDDING_PROVIDER=voyage \
  -e PGEDGE_KB_VOYAGE_API_KEY=${VOYAGE_API_KEY} \
  ghcr.io/pgedge/mcp-server:latest
```

---

## Docker Compose Deployment

### Development Deployment

The repository includes a [docker-compose.yml](docker-compose.yml) file for
local development. This configuration builds images from source and is suitable
for testing and development.

```bash
# Start the stack
docker-compose up -d

# View logs
docker-compose logs -f

# Stop the stack
docker-compose down
```

### Production Deployment

For production deployments, use the example configuration at
[examples/docker-compose.production.yml](examples/docker-compose.production.yml).

This configuration:

- Uses pre-built images from GitHub Container Registry
- Includes resource limits and health checks
- Provides proper logging configuration
- Uses production-ready restart policies

**Usage:**

```bash
# Copy environment example
cp examples/.env.example .env
# Edit .env with your values

# Start production stack
docker-compose -f examples/docker-compose.production.yml up -d

# View logs
docker-compose -f examples/docker-compose.production.yml logs -f

# Stop stack
docker-compose -f examples/docker-compose.production.yml down
```

**Required Environment Variables:**

See [examples/.env.example](examples/.env.example) for all configuration
options. At minimum, you need:

- `POSTGRES_PASSWORD` - PostgreSQL password
- `ANTHROPIC_API_KEY` or `OPENAI_API_KEY` - LLM provider API key

---

## Kubernetes Deployment (Helm Chart)

A complete Helm chart is available at
[examples/helm/pgedge-nla/](examples/helm/pgedge-nla/).

### Chart Features

- **High Availability**: Supports multiple replicas with pod anti-affinity
- **Autoscaling**: Horizontal Pod Autoscaler (HPA) support
- **Security**: Pod security contexts, read-only root filesystems
- **Ingress**: Optional ingress with TLS support
- **Persistence**: StatefulSet support with persistent volumes

### Quick Start

```bash
# Install from local chart
helm install pgedge-nla examples/helm/pgedge-nla \
  --namespace pgedge \
  --create-namespace \
  --set secrets.postgresPassword="your-secure-password" \
  --set secrets.anthropicApiKey="your-api-key"

# Install with production values
helm install pgedge-nla examples/helm/pgedge-nla \
  --namespace pgedge \
  --create-namespace \
  -f examples/helm/pgedge-nla/values-production.yaml

# Upgrade
helm upgrade pgedge-nla examples/helm/pgedge-nla \
  --namespace pgedge \
  -f values.yaml

# Uninstall
helm uninstall pgedge-nla --namespace pgedge
```

### Configuration

The chart includes two values files:

- **[values.yaml](examples/helm/pgedge-nla/values.yaml)** - Default
  configuration for development/testing
- **[values-production.yaml](examples/helm/pgedge-nla/values-production.yaml)**
  - Production-ready configuration with:
  - Higher replica counts
  - Resource limits and autoscaling
  - Pod anti-affinity for high availability
  - Ingress with TLS

### Key Configuration Options

```yaml
server:
  replicaCount: 2 # Number of server replicas
  resources:
    limits:
      cpu: 1000m
      memory: 1Gi
  autoscaling:
    enabled: false # Set to true for HPA
  knowledgebase:
    enabled: true # Enable similarity search
    existingPvc: pgedge-nla-kb # PVC with knowledgebase

ingress:
  enabled: true
  className: nginx
  hosts:
    - host: nla.example.com
  tls:
    - secretName: pgedge-nla-tls
      hosts:
        - nla.example.com
```

See the [Helm chart README](examples/helm/pgedge-nla/README.md) for complete
documentation.

---

## Container Registry

Images are published to GitHub Container Registry (GHCR).

### Pulling Images

```bash
# Pull latest images
docker pull ghcr.io/pgedge/mcp-server:latest
docker pull ghcr.io/pgedge/nla-web:latest
docker pull ghcr.io/pgedge/nla-cli:latest

# Pull specific version
docker pull ghcr.io/pgedge/mcp-server:1.0.0
```

### Publishing Images

```bash
# Login to GHCR
echo $GITHUB_TOKEN | docker login ghcr.io -u USERNAME --password-stdin

# Tag images
docker tag pgedge/mcp-server:latest ghcr.io/pgedge/mcp-server:latest
docker tag pgedge/mcp-server:latest ghcr.io/pgedge/mcp-server:${VERSION}

# Push images
docker push ghcr.io/pgedge/mcp-server:latest
docker push ghcr.io/pgedge/mcp-server:${VERSION}

# Push all tags
docker push ghcr.io/pgedge/mcp-server --all-tags
```

### Using GitHub Actions

For automated builds and publishing, use GitHub Actions:

```yaml
name: Build and Publish Images

on:
  push:
    tags:
      - 'v*'

jobs:
  build:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write

    steps:
      - uses: actions/checkout@v4

      - name: Login to GHCR
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Build and push
        uses: docker/build-push-action@v5
        with:
          context: .
          file: Dockerfile.server
          push: true
          tags: |
            ghcr.io/pgedge/mcp-server:latest
            ghcr.io/pgedge/mcp-server:${{ github.ref_name }}
```

---

## Security Best Practices

### Image Security

- **Run as non-root**: All images run as unprivileged users (UID 1000)
- **Read-only root filesystem**: Containers use read-only root filesystems
  where possible
- **No new privileges**: `allowPrivilegeEscalation: false` in Kubernetes
- **Drop all capabilities**: Minimal Linux capabilities required

### Scanning Images

```bash
# Scan with Trivy
trivy image ghcr.io/pgedge/mcp-server:latest

# Scan with Docker Scout (if available)
docker scout cves ghcr.io/pgedge/mcp-server:latest

# Scan for high/critical vulnerabilities only
trivy image --severity HIGH,CRITICAL ghcr.io/pgedge/mcp-server:latest
```

### Secret Management

**Never commit secrets to version control.** Use one of these approaches:

**Docker Compose:**

```bash
# Use environment files (not committed to git)
docker-compose --env-file .env up -d
```

**Kubernetes:**

```bash
# Use kubectl to create secrets
kubectl create secret generic pgedge-secrets \
  --from-literal=postgres-password='your-password' \
  --from-literal=anthropic-api-key='your-key' \
  --namespace pgedge

# Or use external secret managers
# - AWS Secrets Manager
# - HashiCorp Vault
# - Google Secret Manager
```

### Network Policies

For Kubernetes, restrict network traffic with NetworkPolicies:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: pgedge-mcp-server
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/component: server
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - from:
        - podSelector:
            matchLabels:
              app.kubernetes.io/component: web
      ports:
        - protocol: TCP
          port: 8080
  egress:
    - to:
        - podSelector:
            matchLabels:
              app: postgres
      ports:
        - protocol: TCP
          port: 5432
```

---

## Monitoring and Troubleshooting

### Viewing Logs

**Docker Compose:**

```bash
# All services
docker-compose logs -f

# Specific service
docker-compose logs -f pgedge-mcp-server

# Last 100 lines
docker-compose logs --tail=100 pgedge-mcp-server
```

**Kubernetes:**

```bash
# Server logs
kubectl logs -f deployment/pgedge-mcp-server -n pgedge

# Web UI logs
kubectl logs -f deployment/pgedge-nla-web -n pgedge

# Previous container logs (after crash)
kubectl logs deployment/pgedge-mcp-server -n pgedge --previous

# All pods with label
kubectl logs -l app.kubernetes.io/component=server -n pgedge --tail=100
```

### Health Checks

**Docker:**

```bash
# Check container health status
docker ps

# Inspect health check
docker inspect pgedge-mcp-server | jq '.[0].State.Health'

# Manual health check
curl http://localhost:8080/health
```

**Kubernetes:**

```bash
# Check pod status
kubectl get pods -n pgedge

# Describe pod (includes events)
kubectl describe pod pgedge-mcp-server-xxx -n pgedge

# Port forward and test
kubectl port-forward svc/pgedge-mcp-server 8080:8080 -n pgedge
curl http://localhost:8080/health
```

### Debug Container

**Docker:**

```bash
# Execute shell in running container
docker exec -it pgedge-mcp-server sh

# Check processes
docker exec pgedge-mcp-server ps aux

# Check network connectivity
docker exec pgedge-mcp-server wget -O- http://postgres:5432
```

**Kubernetes:**

```bash
# Execute shell in pod
kubectl exec -it deployment/pgedge-mcp-server -n pgedge -- sh

# Debug with ephemeral container (Kubernetes 1.23+)
kubectl debug -it pgedge-mcp-server-xxx -n pgedge --image=alpine --target=server

# Check connectivity to postgres
kubectl exec deployment/pgedge-mcp-server -n pgedge -- \
  wget -qO- http://postgres-postgresql:5432 || echo "Cannot connect"
```

### Common Issues

**Server won't start:**

```bash
# Check logs for errors
kubectl logs deployment/pgedge-mcp-server -n pgedge | grep -i error

# Verify database connection
kubectl exec deployment/pgedge-mcp-server -n pgedge -- \
  env | grep POSTGRES

# Check if config is mounted
kubectl exec deployment/pgedge-mcp-server -n pgedge -- \
  cat /etc/pgedge/mcp-server.yaml
```

**Database connection issues:**

```bash
# Test PostgreSQL connectivity
kubectl run -it --rm debug --image=postgres:17-alpine -- \
  psql postgresql://postgres:password@postgres-postgresql:5432/postgres

# Check DNS resolution
kubectl exec deployment/pgedge-mcp-server -n pgedge -- \
  nslookup postgres-postgresql
```

**Resource constraints:**

```bash
# Check resource usage
kubectl top pods -n pgedge

# Describe pod to see events
kubectl describe pod pgedge-mcp-server-xxx -n pgedge | grep -A 5 Events

# Increase resources in values.yaml and upgrade
helm upgrade pgedge-nla examples/helm/pgedge-nla -f values.yaml
```
