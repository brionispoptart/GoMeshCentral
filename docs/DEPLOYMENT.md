# Deployment Guide

## Quick Start - Linux Server

### Prerequisites
- Linux server (Ubuntu 20.04+ recommended)
- No dependencies needed (Go binaries are self-contained)

### Installation Steps

1. **Download the server binary**
   ```bash
   # Replace with the latest version from releases
   wget https://github.com/brionispoptart/GoMeshCentral/releases/download/v1.0.0/server-linux
   chmod +x server-linux
   ```

2. **Download the agent binary** (for same machine testing)
   ```bash
   wget https://github.com/brionispoptart/GoMeshCentral/releases/download/v1.0.0/agent-linux
   chmod +x agent-linux
   ```

3. **Create data directory**
   ```bash
   mkdir -p data
   ```

4. **Set environment variables**
   ```bash
   export GMC_LISTEN_ADDR=:8080
   export GMC_JWT_SECRET=your-secret-key-here
   export GMC_BOOTSTRAP_ADMIN_USER=admin
   export GMC_BOOTSTRAP_ADMIN_PASS=admin123
   ```

5. **Run the server**
   ```bash
   ./server-linux
   ```

   You should see:
   ```
   server listening on :8080
   ```

6. **In another terminal, run the agent**
   ```bash
   export GMC_LISTEN_ADDR=:8080
   export GMC_JWT_SECRET=your-secret-key-here
   export GMC_BOOTSTRAP_ADMIN_USER=admin
   export GMC_BOOTSTRAP_ADMIN_PASS=admin123
   
   ./agent-linux -server localhost:8080
   ```

7. **Access the dashboard**
   ```
   http://localhost:8080/client
   Username: admin
   Password: admin123
   ```

## Production Deployment

### Systemd Service (Recommended)

1. **Create service file**
   ```bash
   sudo nano /etc/systemd/system/gomeshcentral-server.service
   ```

2. **Add this content**
   ```ini
   [Unit]
   Description=GoMeshCentral Server
   After=network.target

   [Service]
   Type=simple
   User=gomesh
   WorkingDirectory=/opt/gomeshcentral
   Environment="GMC_LISTEN_ADDR=:8080"
   Environment="GMC_JWT_SECRET=your-secret-here"
   Environment="GMC_BOOTSTRAP_ADMIN_USER=admin"
   Environment="GMC_BOOTSTRAP_ADMIN_PASS=secure-password"
   Environment="GMC_DB_PATH=/var/lib/gomeshcentral/gomeshcentral.db"
   ExecStart=/opt/gomeshcentral/server-linux
   Restart=always
   RestartSec=10

   [Install]
   WantedBy=multi-user.target
   ```

3. **Enable and start**
   ```bash
   sudo systemctl daemon-reload
   sudo systemctl enable gomeshcentral-server
   sudo systemctl start gomeshcentral-server
   sudo systemctl status gomeshcentral-server
   ```

### Cloudflare Tunnel Setup

1. **Install cloudflared**
   ```bash
   curl https://pkg.cloudflare.com/cloudflare-release.key | sudo gpg --yes --dearmor --output /usr/share/keyrings/cloudflare-archive-keyring.gpg
   echo 'deb [signed-by=/usr/share/keyrings/cloudflare-archive-keyring.gpg] https://pkg.cloudflare.com/linux focal main' | sudo tee /etc/apt/sources.list.d/cloudflare-main.list
   sudo apt-get update
   sudo apt-get install cloudflared
   ```

2. **Authenticate**
   ```bash
   cloudflared tunnel login
   ```

3. **Create tunnel config** `~/.cloudflared/config.yml`
   ```yaml
   tunnel: my-gomesh-tunnel
   credentials-file: /home/user/.cloudflared/UUID.json

   ingress:
     - hostname: app.example.com
       service: http://localhost:8080
     - service: http_status:404
   ```

4. **Run tunnel**
   ```bash
   cloudflared tunnel run
   ```

5. **DNS setup** (in Cloudflare dashboard)
   - Create CNAME record: `app.example.com` → `UUID.cfargotunnel.com`

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `GMC_LISTEN_ADDR` | `:8080` | Server listen address |
| `GMC_AGENT_PUBLIC_ADDR` | `localhost:8080` | Agent connection address (use when behind reverse proxy) |
| `GMC_JWT_SECRET` | `dev-secret` | JWT signing secret (MUST change in production) |
| `GMC_DB_PATH` | `data/gomeshcentral.db` | SQLite database path |
| `GMC_BOOTSTRAP_ADMIN_USER` | `admin` | Initial admin username |
| `GMC_BOOTSTRAP_ADMIN_PASS` | `admin123` | Initial admin password |

## Health Check

```bash
curl http://localhost:8080/health
```

Expected response:
```json
{"status":"ok"}
```

## Database Backup

```bash
cp data/gomeshcentral.db data/gomeshcentral.db.backup
```

## Logs

Check systemd logs:
```bash
sudo journalctl -u gomeshcentral-server -f
```

## Troubleshooting

### Port already in use
```bash
lsof -i :8080
kill -9 <PID>
```

### Database locked
```bash
# Restart the service
sudo systemctl restart gomeshcentral-server
```

### Agent won't connect
- Check firewall rules
- Verify `GMC_AGENT_PUBLIC_ADDR` matches server's public address
- Check logs for connection errors

## Monitoring

Monitor process health:
```bash
systemctl is-active gomeshcentral-server
systemctl is-enabled gomeshcentral-server
```

Monitor resources:
```bash
ps aux | grep server-linux
```
