# pgEdge Natural Language Agent - Helm Chart

This Helm chart deploys the pgEdge Natural Language Agent on Kubernetes.

## Prerequisites

- Kubernetes 1.21+, which labels each namespace with
  `kubernetes.io/metadata.name` for the network policies to select
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

`helm uninstall` keeps the server's data volume, so users, tokens and
conversation history survive a reinstall. Delete the claim yourself to
discard them:

```bash
kubectl delete pvc -n pgedge pgedge-nla-server-data
```

The chart deploys the server and web client images whose tag matches its
`appVersion`, adding `-with-kb` to the server's tag when the built-in
knowledgebase is enabled. Set `server.image.tag` or `web.image.tag` only to
pin a different build.

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
- `server.knowledgebase`: The documentation knowledgebase, off by
  default; enabling it needs `server.knowledgebase.embedding.provider`
- `server.persistence`: The data volume for users, tokens and sessions
- `server.clientIp.trustedProxies`: The networks the ingress controller
  and web pods connect from, so that login rate limiting sees real client
  addresses; see Client Addresses below
- `networkPolicy.enabled`: Restrict which pods may reach the web client and
  the server (default: true)
- `web.replicaCount`: Number of web client replicas (default: 2)
- `ingress.enabled`: Enable ingress for external access

### Users and Tokens

The server starts with no users and no API tokens, and keeps both in
`users.json` and `tokens.json` on its data volume. An init container
creates both files empty on first install, so the server watches them and
loads changes without a restart. Add users and tokens with the server
binary inside the running pod; `-it` lets it prompt for the password:

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

### Client Addresses

A browser request passes through two proxies before it reaches the server:
the ingress controller, then a web pod's nginx. The server's peer on every
such connection is therefore a web pod, and whilst
`server.clientIp.trustedProxies` is empty the server counts every browser
login against that pod's address. One person failing to log in
repeatedly can then lock everyone out of the web client for the rate limit
window.

The ingress controller sets `X-Forwarded-For` to the caller's address, and
the web pod's nginx appends the controller's, which the chart has the
server read once `trustedProxies` is set. The server reads the header from the
right, skipping each address in a trusted network, and uses the first
address it does not trust as the caller. Set `trustedProxies` to the
network both proxies connect from, which is usually the cluster's pod
network:

```bash
helm upgrade pgedge-nla ./pgedge-nla --namespace pgedge \
  --reuse-values \
  --set 'server.clientIp.trustedProxies[0]=10.244.0.0/16'
```

Replace `10.244.0.0/16` with your cluster's pod CIDR. If the ingress
controller runs with host networking, add the nodes' addresses too, since
it then connects from those rather than from the pod network. Keep the
network policies below enabled whilst you trust the pod network, so that
no other pod can reach the server with a forged header.

If a load balancer that terminates HTTP sits in front of the ingress
controller, the controller sees the load balancer rather than the caller,
and ignores the load balancer's `X-Forwarded-For` by default. For
ingress-nginx, set `use-forwarded-headers: "true"` and `proxy-real-ip-cidr`
to the load balancer's addresses in the controller's ConfigMap, so that it
passes the caller's address on.

### Network Policies

With `networkPolicy.enabled`, the chart admits traffic to the web pods only
from the ingress controller, and to the server only from the web pods.
`networkPolicy.ingressController` selects the controller's pods; its
default matches an `ingress-nginx` install in the `ingress-nginx`
namespace, so change it for any other controller. A controller with host
networking matches no pod selector, so replace both selectors with an
`ipBlock` covering its nodes; Helm merges your values into the defaults, so
set the selectors to `null` to remove them:

```yaml
networkPolicy:
  ingressController:
    namespaceSelector: null
    podSelector: null
    ipBlock:
      cidr: 192.0.2.0/24
```

Network policies do not reliably govern traffic from pods that use host
networking, which most plugins treat as traffic from the node itself. Once
`trustedProxies` includes node addresses, any host-network pod on those
nodes could reach the server with a forged `X-Forwarded-For`, so restrict
who can run host-network pods in the cluster.

The policies need a network plugin that enforces them, and have no effect
otherwise. `kubectl exec` and `kubectl port-forward` still work, because
both reach the pod through the kubelet rather than over the pod network.

### Knowledgebase

The knowledgebase is off by default. Searching it embeds each query, so the
server needs the embedding provider and model that built the knowledgebase;
the server's own default is Ollama on localhost, which no pod provides. Set
`server.knowledgebase.embedding.provider` to `voyage`, `openai`, `gemini`
or `ollama`, with its API key in `secrets.voyageApiKey`,
`secrets.openaiApiKey` or `secrets.geminiApiKey`. Ollama needs
`server.knowledgebase.embedding.ollamaUrl` instead of a key.

`server.knowledgebase.useBuiltIn` uses the knowledgebase in the server's
`-with-kb` image; set it to `false` to mount `kb.db` from the claim named
in `server.knowledgebase.existingPvc`.

## Production Deployment

For production deployments, use the provided `values-production.yaml` which
includes:

- Larger server resources and data volume
- More web client replicas, with pod anti-affinity
- Ingress with TLS

See [values-production.yaml](values-production.yaml) for details.
