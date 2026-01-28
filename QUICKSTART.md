# K8Watch Quick Start Guide

## What is K8Watch?

K8Watch is a **lightweight, read-only Kubernetes change tracking tool** that watches your deployments, configmaps, and secrets in real-time and provides a modern web interface to view change history, diffs, and statistics.

Perfect for SREs/DevOps teams who want visibility without the complexity of Grafana/Prometheus.

## Installation

### Option 1: Run Locally (Recommended for Development)

```bash
# Navigate to the project
cd /Users/dheeryad/k8watch

# Install dependencies
go mod download

# Run the application
go run cmd/k8watch/main.go

# Or use the quick start script
./start.sh

# Or use make
make start
```

Access the UI at: **http://localhost:8080**

### Option 2: Docker (Recommended for Production)

```bash
# Using Docker Compose (easiest)
cd /Users/dheeryad/k8watch
docker-compose up -d

# Or build and run manually
docker build -t k8watch:latest .
docker run -d \
  -p 8080:8080 \
  -v ~/.kube/config:/root/.kube/config:ro \
  -v $(pwd)/data:/data \
  k8watch:latest
```

Access the UI at: **http://localhost:8080**

### Option 3: Build Binary

```bash
cd /Users/dheeryad/k8watch
go build -o k8watch ./cmd/k8watch

# Run with custom options
./k8watch --kubeconfig ~/.kube/config --db ./events.db --addr :8080
```

## First Time Setup

1. **Check Kubernetes Connection**
   ```bash
   kubectl cluster-info
   ```

2. **Start K8Watch**
   ```bash
   cd /Users/dheeryad/k8watch
   go run cmd/k8watch/main.go
   ```

3. **Open Web UI**
   - Navigate to http://localhost:8080
   - You should see the dashboard with three tabs: Deployments, ConfigMaps, Secrets

4. **Test It**
   - Make a change to a deployment: `kubectl set image deployment/myapp myapp=nginx:latest`
   - Watch it appear in K8Watch within seconds!

## Command Line Options

```bash
./k8watch [options]

Options:
  --kubeconfig string   Path to kubeconfig file (default: ~/.kube/config)
  --db string          Path to SQLite database (default: ./events.db)
  --addr string        HTTP server address (default: :8080)
```

## Features Overview

### 📊 Dashboard Tabs

1. **Deployments Tab**
   - Track deployment changes
   - See image version updates
   - View rollout history

2. **ConfigMaps Tab**
   - Monitor config changes
   - See which keys changed (values hidden for security)
   - Track configuration updates

3. **Secrets Tab**
   - Track secret modifications
   - See which keys changed (values NEVER shown)
   - Audit secret updates

### 🔍 Filtering

- **Namespace**: Filter by Kubernetes namespace
- **Name**: Search for specific resources
- **Action**: Filter by ADDED, MODIFIED, DELETED
- Auto-refresh every 10 seconds

### 📈 Statistics

- Total changes tracked
- Changes in last 24 hours
- Changes per hour
- Top 10 most modified apps
- Recent container images

### ⏱️ Timeline View

Click any event to see:
- Complete change history
- Image version changes
- Full diffs (for deployments)
- Timestamps and metadata

## Common Use Cases

### 1. Track Deployment Rollouts
```
Filter by: Kind=Deployment, Action=MODIFIED
Look for: Image changes in the timeline
```

### 2. Debug Configuration Issues
```
Filter by: Kind=ConfigMap, Namespace=production
Look for: Recent MODIFIED events
```

### 3. Audit Secret Changes
```
Filter by: Kind=Secret
Look for: Any changes in the last 24h
```

### 4. Monitor Specific App
```
Search by: Name="myapp"
Look for: All changes across all resource types
```

## Architecture

```
┌─────────────────────┐
│ Kubernetes Cluster  │
│                     │
│ - Deployments       │
│ - ConfigMaps        │
│ - Secrets           │
└──────────┬──────────┘
           │ Watch API
           ▼
    ┌──────────────┐
    │   K8Watch    │
    │              │
    │  ┌────────┐  │
    │  │ SQLite │  │  HTTP
    │  └────────┘  ├────────► Web Browser
    │              │          (localhost:8080)
    └──────────────┘
```

## Troubleshooting

### "Failed to create config"
```bash
# Verify kubeconfig works
kubectl cluster-info

# Try specifying kubeconfig explicitly
./k8watch --kubeconfig ~/.kube/config
```

### "Database is locked"
```bash
# Only one instance can access the database
# Stop other instances or use different DB
./k8watch --db /tmp/events-new.db
```

### "No events appearing"
```bash
# Check logs for watch errors
# Verify RBAC permissions
kubectl auth can-i list deployments --all-namespaces
kubectl auth can-i watch deployments --all-namespaces
```

### Port already in use
```bash
# Use different port
./k8watch --addr :9090
```

## Next Steps

1. **Deploy to Production**: Use Docker Compose with persistent volumes
2. **Add Reverse Proxy**: Use nginx/traefik for authentication
3. **Backup Database**: Regularly backup events.db for history
4. **Monitor Logs**: Set up log rotation for production use

## Security Notes

✅ **What K8Watch DOES:**
- Read-only access to Kubernetes
- Store event metadata and diffs
- Show key names for configs/secrets

❌ **What K8Watch NEVER does:**
- Write to Kubernetes
- Store secret/config values
- Expose sensitive data in UI
- Require authentication (add your own!)

## Resources

- **Source Code**: /Users/dheeryad/k8watch
- **Database**: ./events.db (SQLite)
- **Web UI**: ./web/
- **Documentation**: README.md

## Quick Commands

```bash
# Start
make start

# Stop (if running in background)
pkill k8watch

# View logs
tail -f /var/log/k8watch.log

# Check database
sqlite3 events.db "SELECT COUNT(*) FROM change_events;"

# Clean up
make clean
```

## Support

For issues or questions:
1. Check logs for error messages
2. Verify Kubernetes connectivity
3. Review troubleshooting section above

---

**Happy Tracking! 📊**
