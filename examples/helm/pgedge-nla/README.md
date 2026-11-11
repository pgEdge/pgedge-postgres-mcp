# pgEdge Natural Language Agent - Helm Chart

This Helm chart deploys the pgEdge Natural Language Agent on Kubernetes.

## Prerequisites

- Kubernetes 1.19+
- Helm 3.0+
- PostgreSQL database (external or in-cluster)
- Optional: cert-manager for TLS certificates

## Installation

```bash
# Install from local chart
helm install pgedge-nla ./pgedge-nla \
  --namespace pgedge \
  --create-namespace \
  --set secrets.postgresPassword="your-secure-password"

# Install with production values
helm install pgedge-nla ./pgedge-nla \
  --namespace pgedge \
  --create-namespace \
  -f values-production.yaml \
  --set secrets.postgresPassword="your-secure-password"

# Upgrade
helm upgrade pgedge-nla ./pgedge-nla \
  --namespace pgedge \
  -f values-production.yaml \
  --set secrets.postgresPassword="your-secure-password"

# Uninstall
helm uninstall pgedge-nla --namespace pgedge
```

Install one release per namespace. The web client's nginx proxies to a
Service named `postgres-mcp` on port 8080, so the chart always gives the
server's Service that name, and a second release in the same namespace
would collide with it.

## Configuration

See [values.yaml](values.yaml) for all configuration options.

### Key Configuration Options

- `secrets.postgresPassword`: PostgreSQL password (required)
- `server.database`: PostgreSQL host, port, database, user and SSL mode
- `server.llm`: Enable the LLM proxy for the web client's chat, with the
  API key in `secrets.anthropicApiKey`, `secrets.openaiApiKey` or
  `secrets.geminiApiKey`; any provider other than Anthropic needs
  `server.llm.model`, and Ollama takes `server.llm.ollamaUrl`
- `server.allowedOrigins`: Extra browser origins to accept; each ingress
  host is allowed already, and any other origin except loopback gets 403
- `server.persistence`: The data volume for users, tokens and sessions
- `server.clientIp.trustedProxies`: Your ingress controller's addresses,
  so that login rate limiting sees real client addresses
- `web.replicaCount`: Number of web client replicas (default: 2)
- `ingress.enabled`: Enable ingress for external access

### Users and Tokens

The server starts with no users and no API tokens, and keeps both in
`users.json` and `tokens.json` on its data volume. An init container
creates both files empty on first install, so the server watches them and
loads changes without a restart. Add users and tokens with the server binary inside the running pod; `-it` lets
it prompt for the password:

```bash
kubectl exec -it -n pgedge deploy/pgedge-nla-server -- \
  /app/pgedge-postgres-mcp -add-user -username alice \
  -user-file /var/lib/pgedge/mcp-server/users.json

kubectl exec -it -n pgedge deploy/pgedge-nla-server -- \
  /app/pgedge-postgres-mcp -add-token -token-note "reporting service" \
  -token-file /var/lib/pgedge/mcp-server/tokens.json
```

### Single Server Replica

The server always runs as one replica, with the `Recreate` update strategy.
It keeps users, tokens, password-login sessions and OAuth state in its data
directory, which two instances must never share, and the data volume is
`ReadWriteOnce`. `Recreate` stops the old pod before its replacement mounts
the volume, so an upgrade causes a brief outage. The web client is stateless
and scales with `web.replicaCount`.

## Production Deployment

For production deployments, use the provided `values-production.yaml` which
includes:

- Larger server resources and data volume
- More web client replicas, with pod anti-affinity
- Ingress with TLS

See [values-production.yaml](values-production.yaml) for details.
