# Distributed Deployment

A distributed deployment runs multiple pgEdge Postgres MCP Server
instances behind a load balancer. This architecture provides high
availability, horizontal scaling, and fault tolerance for
production environments.

The MCP server uses file-based configuration for authentication
and a SQLite file for the knowledge base. Distributed deployments
must ensure that all instances access consistent copies of these
files. This guide covers architecture patterns, load balancer
configuration, and orchestration strategies for multi-instance
deployments.

## Architecture Patterns

Two primary patterns support distributed MCP server deployments.
The shared filesystem pattern offers simplicity and strong
consistency. The object storage pattern provides flexibility for
environments without shared volumes.

### Shared Filesystem Pattern

The shared filesystem pattern mounts authentication files and the
knowledge base database on a shared network volume. All instances
read from the same files on the shared volume.

The following diagram illustrates this architecture:

```text
+--------------+ +--------------+ +--------------+
| MCP Server   | | MCP Server   | | MCP Server   |
| Instance 1   | | Instance 2   | | Instance 3   |
+------+-------+ +------+-------+ +------+-------+
       |                |                |
       +--------+-------+--------+-------+
                |
       +--------+--------+
       |  Shared Volume  |
       |  (NFS/EFS/GCS)  |
       |  - tokens.yaml  |
       |  - users.yaml   |
       |  - kb.db        |
       +-----------------+
```

This pattern works well with managed cloud storage services.
Amazon EFS, Google Cloud Filestore, and Azure Files all provide
compatible shared volumes. The server uses `fsnotify` to detect
file changes; the auto-reload feature picks up updates on shared
volumes without a restart.

The shared filesystem pattern offers the following advantages:

- All instances read the same authentication files.
- Configuration changes propagate to every instance.
- The setup requires minimal additional infrastructure.
- Cloud-managed storage handles replication and durability.

### Object Storage Pattern

The object storage pattern stores authentication and knowledge
base files in a cloud object store. This approach suits
deployments that lack shared filesystem support.

Use Amazon S3, Google Cloud Storage, or Azure Blob Storage to
host the configuration files. Init containers or scheduled cron
jobs synchronize files from the object store to local disk before
the server starts.

This pattern introduces eventual consistency as a trade-off.
Updates to authentication files take effect only after the next
synchronization cycle. The synchronization interval determines
the maximum delay between an update and its propagation.

The following steps describe the object storage workflow:

1. Upload the updated `tokens.yaml` file to the object store.
2. Each instance pulls the file on its synchronization schedule.
3. The server detects the local file change and reloads.

## Load Balancer Configuration

Bearer token authentication in the MCP server is stateless. Any
instance can validate a token independently; sticky sessions are
not required. A standard round-robin or least-connections load
balancer distributes traffic across all instances.

### nginx Configuration

The following `nginx` configuration distributes requests across
three MCP server instances:

```nginx
upstream mcp_backend {
    server mcp-1:8080;
    server mcp-2:8080;
    server mcp-3:8080;
}

server {
    listen 443 ssl;
    server_name mcp.example.com;

    ssl_certificate /etc/ssl/certs/mcp.crt;
    ssl_certificate_key /etc/ssl/private/mcp.key;

    location / {
        proxy_pass http://mcp_backend;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $remote_addr;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location /health {
        proxy_pass http://mcp_backend;
    }
}
```

The `upstream` block defines the pool of backend servers. The
`/health` endpoint enables load balancer health checks without
authentication.

Both forwarding headers are set to `$remote_addr` so that they replace
any value the caller sent; `$proxy_add_x_forwarded_for` would append to
it instead. Each backend then needs to read the client address from the
header, trusting the load balancer that fronts it:

```yaml
http:
  client_ip:
    source: header
    header: X-Real-IP
    trusted_proxies:
      - 192.0.2.0/24        # The subnet the load balancers run in
```

Every backend sees the load balancer as its peer, so without this
setting all login rate limiting collapses onto one address. For
details, see [Client Address
Resolution](../guide/configuration.md#client-address-resolution).

### AWS Application Load Balancer

The following JSON configures health checks for an AWS
Application Load Balancer target group:

```json
{
    "HealthCheckPath": "/health",
    "HealthCheckIntervalSeconds": 30,
    "HealthyThresholdCount": 2,
    "UnhealthyThresholdCount": 3,
    "HealthCheckTimeoutSeconds": 5
}
```

The load balancer marks an instance as unhealthy after three
consecutive failed checks. Two successful checks restore the
instance to the healthy state.

## Docker Compose Multi-Instance Deployment

Docker Compose provides a straightforward way to run multiple
MCP server instances on a single host. The following example
deploys three instances behind an `nginx` reverse proxy.

In the following `docker-compose.yml` file, three MCP server
containers share a configuration volume:

```yaml
services:
    nginx:
        image: nginx:alpine
        ports:
            - "443:443"
        volumes:
            - ./nginx.conf:/etc/nginx/nginx.conf:ro
            - ./certs:/etc/ssl:ro
        depends_on:
            - mcp-1
            - mcp-2
            - mcp-3

    mcp-1:
        image: pgedge/postgres-mcp:latest
        env_file: .env
        volumes:
            - shared-config:/app/config:ro

    mcp-2:
        image: pgedge/postgres-mcp:latest
        env_file: .env
        volumes:
            - shared-config:/app/config:ro

    mcp-3:
        image: pgedge/postgres-mcp:latest
        env_file: .env
        volumes:
            - shared-config:/app/config:ro

volumes:
    shared-config:
        driver: local
        driver_opts:
            type: nfs
            o: addr=nfs-server,rw
            device: ":/exports/mcp-config"
```

The `shared-config` volume mounts an NFS share that contains
the authentication YAML files and the knowledge base database.
All three instances read from the same volume. Replace
`nfs-server` and the device path with your NFS server address
and export path.

Start the deployment with the following command:

```bash
docker-compose up -d
```

## Kubernetes Deployment

Kubernetes provides robust orchestration for distributed MCP
server deployments. The project includes Helm charts for
streamlined installation. This section covers key configuration
patterns for Kubernetes environments.

### Authentication Files with ConfigMap

Store authentication configuration in a Kubernetes ConfigMap or
Secret. The following ConfigMap example defines an API token
configuration:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
    name: mcp-auth
data:
    tokens.yaml: |
        tokens:
            - token: "hashed-token-here"
              name: "api-client"
```

Mount the ConfigMap as a volume in the MCP server pod. Use a
Kubernetes Secret instead of a ConfigMap for sensitive token
data.

### Knowledge Base with Init Containers

The knowledge base is a SQLite database file that the server
reads at startup. Use an init container to download the file
before the main container starts.

In the following example, the init container downloads the
knowledge base from a remote URL:

```yaml
initContainers:
    - name: kb-loader
      image: busybox
      command:
          - wget
          - "-O"
          - /data/kb.db
          - "https://storage.example.com/kb.db"
      volumeMounts:
          - name: kb-data
            mountPath: /data
```

A PersistentVolumeClaim offers an alternative for environments
that require durable storage. The init container writes the
knowledge base file to the persistent volume on first
deployment.

### Helm Persistence and auth.db

The server stores password-login sessions and OAuth state in `auth.db`,
which lives in the data directory. The Helm chart mounts a `data` volume at
`/var/lib/pgedge/mcp-server`. When `server.persistence.enabled` is `true`,
that volume is a PersistentVolumeClaim; otherwise, it is an `emptyDir`
volume that Kubernetes deletes when the pod is replaced.

The current chart does not place the data directory on that volume. With
the chart defaults, the image entrypoint cannot create `/app/data` on the
read-only root filesystem, so the server does not start.

If a pod runs with a writable `/app/data` outside the data volume, then
`auth.db` does not survive pod replacement, even with persistence enabled.
Every pod replacement then signs all users out and forces dynamically
registered OAuth clients to register again.

Do not point several replicas at one shared data directory. Each replica
needs its own `auth.db`, because sharing the file is unsupported. A
second server on the same data directory refuses to start, because the
first holds a lock on the directory.

### Horizontal Pod Autoscaler

A Horizontal Pod Autoscaler scales the number of MCP server
pods based on resource utilization. The following configuration
scales between two and ten replicas:

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
    name: mcp-server-hpa
spec:
    scaleTargetRef:
        apiVersion: apps/v1
        kind: Deployment
        name: mcp-server
    minReplicas: 2
    maxReplicas: 10
    metrics:
        - type: Resource
          resource:
              name: cpu
              target:
                  type: Utilization
                  averageUtilization: 70
```

The autoscaler adds pods when average CPU utilization exceeds
70 percent. Set `minReplicas` to at least two for high
availability.

## Knowledge Base Synchronization

The knowledge base is a SQLite file that the MCP server reads
in read-only mode. Multiple instances can safely read the same
file concurrently. Build the knowledge base on a single node or
in a CI/CD pipeline; then distribute the file to all instances.

The following strategies distribute the knowledge base:

- Store the file on a shared volume that all instances mount.
- Upload the file to S3 and download with init containers.
- Publish the file as a CI/CD build artifact.
- Use a sidecar container to periodically fetch updates.

Rebuild the knowledge base periodically to incorporate new
documentation. After rebuilding, replace the file on shared
storage or trigger a new deployment. The server detects the
updated file and reloads the knowledge base automatically.

## Authentication in a Distributed Context

All instances must read the same token and user YAML files.
Consistent authentication data ensures that a token validated
on one instance works on every other instance.

The MCP server uses `fsnotify` to monitor authentication files.
When files change on shared storage, all instances detect the
modification and reload credentials automatically. This reload
happens within 100 milliseconds of the file change.

The following considerations apply to distributed
authentication:

- User session tokens validate against the shared user store.
- API tokens work identically across all instances.
- The auto-reload feature propagates credential changes
  without a restart.
- Sessions remain valid during a credential reload, except those of users
  who were removed, disabled or given a new password.
- A password change also ends the user's OAuth grants.
- Each instance keeps sessions and OAuth state in its own `auth.db` file.
- Two instances must never share one data directory; this is unsupported.
- A second instance on the same data directory refuses to start.

### Rate Limiting Considerations

The built-in rate limiter operates per instance. Each instance
tracks failed authentication attempts independently. A
distributed deployment does not share rate-limiting state
across instances.

For centralized rate limiting, configure the reverse proxy to
enforce request limits. An `nginx` rate-limiting directive or a
cloud load balancer throttling policy provides consistent
enforcement across all instances.

## Monitoring and Health Checks

Each MCP server instance exposes a `/health` endpoint for
health monitoring. Configure the load balancer to poll this
endpoint and remove unhealthy instances from the pool.

The following practices support effective monitoring:

- Use the `/health` endpoint for load balancer health checks.
- Aggregate logs from all instances with a centralized system.
- Monitor per-instance CPU, memory, and request metrics.
- Set up alerts for instance failures and high error rates.

Centralized logging platforms such as the ELK stack, AWS
CloudWatch, or Google Cloud Logging collect and index logs
from all instances. Tag each log entry with the instance
identifier to trace requests across the deployment.

## Best Practices

The following recommendations apply to distributed MCP server
deployments:

- Start with two instances and scale based on observed load.
- Use a shared filesystem for simplicity in cloud deployments.
- Build and test knowledge base updates in a staging
  environment before production.
- Use centralized logging with ELK, CloudWatch, or
  Stackdriver.
- Implement graceful shutdown for zero-downtime deployments.
- Pin container image versions for reproducible deployments.
- Store authentication files in Kubernetes Secrets rather than
  ConfigMaps.
- Configure health check intervals based on your availability
  requirements.
- Rotate API tokens on a regular schedule across all
  instances.
- Test failover scenarios to verify high availability.

## See Also

The following resources provide additional context for
distributed deployments:

- [Deploying on Docker](../guide/deploy_docker.md) covers
  single-instance Docker deployment.
- [Authentication Guide](../guide/authentication.md) explains
  token and user authentication in detail.
- [Knowledgebase Configuration](knowledgebase.md) describes
  how to configure and use the knowledge base.
- [Security Checklist](../guide/security.md) provides security
  best practices for production deployments.
