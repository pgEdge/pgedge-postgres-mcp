# Managing your Services
# Adding the Deployment to your Systemd Service

For Linux production deployments.

### Create a Service File

`/etc/systemd/system/pgedge-mcp-server.service`:

```ini
[Unit]
Description=pgEdge Natural Language Agent
After=network.target postgresql.service

[Service]
Type=simple
User=pgedge
Group=pgedge
WorkingDirectory=/opt/pgedge
ExecStart=/opt/pgedge/bin/pgedge-mcp-server -config /etc/pgedge/config.yaml
Restart=always
RestartSec=10

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true

[Install]
WantedBy=multi-user.target
```

### Enable and Start

```bash
sudo systemctl daemon-reload
sudo systemctl enable pgedge-mcp-server
sudo systemctl start pgedge-mcp-server
sudo systemctl status pgedge-mcp-server
```

### View Logs

```bash
journalctl -u pgedge-mcp-server -f
```

---

## Configuring a Reverse Proxy

For production, run behind nginx with TLS termination.

### Nginx Configuration

```nginx
server {
    listen 443 ssl http2;
    server_name mcp.example.com;

    ssl_certificate /etc/letsencrypt/live/mcp.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/mcp.example.com/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}

server {
    listen 80;
    server_name mcp.example.com;
    return 301 https://$host$request_uri;
}
```

---

## Health Checks

All deployment methods expose a health endpoint:

```bash
curl http://localhost:8080/health
```

Response:
```json
{"status": "ok", "server": "pgedge-mcp-server", "version": "1.0.0"}
```

---
