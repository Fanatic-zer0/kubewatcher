# Kubewatcher - Kubernetes Change Tracker

A lightweight, read-only Kubernetes change tracking tool for SREs and DevOps teams who want change visibility without the overhead of Grafana/Prometheus.

## Features

- 🔍 **Real-time Monitoring**: Watches Deployments, ConfigMaps, and Secrets for changes
- 📊 **Change Tracking**: Records ADD, MODIFY, and DELETE events with diffs
- 🐳 **Image Tracking**: Automatically detects container image changes
- 🔐 **Security First**: Never stores or displays secret values
- 💾 **SQLite Storage**: Simple, self-contained database
- 🎨 **Modern Web UI**: Clean interface with filtering and search
- ⏱️ **Timeline View**: See full change history for any resource
- 📈 **Statistics**: Changes per hour, top modified apps, recent images
- 🔄 **Auto-refresh**: Updates every 10 seconds

## Architecture

```
Kubernetes API Server
         ↓ watch (client-go informers)
     ┌──────────────┐
     │ Go Service   │
     │              │
     │   ┌────────┐ │ HTTP API    ┌─────────────┐
     │   │SQLite  │ │ ──────────→ │ HTML/JS UI  │
     │   │events.db│ │             │ (localhost) │
     │   └────────┘ │             └─────────────┘
     └──────────────┘
```

## Quick Start

### Local Development

1. **Prerequisites**:
   - Go 1.21+
   - Access to a Kubernetes cluster
   - kubectl configured

2. **Install dependencies**:
   ```bash
   cd k8watch
   go mod download
   ```

3. **Run**:
   ```bash
   go run cmd/k8watch/main.go
   ```

4. **Access UI**:
   Open http://localhost:8080

### Docker

1. **Build and run with Docker Compose**:
   ```bash
   docker-compose up -d
   ```

2. **Access UI**:
   Open http://localhost:8080

3. **View logs**:
   ```bash
   docker-compose logs -f
   ```

4. **Stop**:
   ```bash
   docker-compose down
   ```

### Custom Configuration

```bash
# Custom kubeconfig
./k8watch --kubeconfig /path/to/kubeconfig

# Custom database path
./k8watch --db /path/to/events.db

# Custom server address
./k8watch --addr :9090
```

## Usage

### Web UI

The web interface provides three main tabs:

1. **Deployments**: Track deployment changes, image updates, and rollouts
2. **ConfigMaps**: Monitor configuration changes (keys only, not values)
3. **Secrets**: Track secret modifications (keys only, never values)

### Filtering

- **Namespace**: Filter by Kubernetes namespace
- **Name**: Search for specific resource names
- **Action**: Filter by ADDED, MODIFIED, or DELETED events
- **Time Range**: View changes within specific time periods

### Timeline View

Click on any event to see the complete change timeline for that resource, including:
- All historical changes
- Image version changes
- Detailed diffs
- Timestamps and actions

### Statistics Dashboard

- Total changes tracked
- Changes in last 24 hours
- Average changes per hour
- Top 10 most modified applications
- Recent container images deployed

## API Endpoints

### Get Events
```bash
GET /api/events?kind=Deployment&namespace=default&limit=100
```

### Get Timeline
```bash
GET /api/timeline/{namespace}/{kind}/{name}
```

### Get Statistics
```bash
GET /api/stats
```

## Security Considerations

- **Read-Only**: K8Watch only reads from Kubernetes, never writes
- **Secret Protection**: Secret values are NEVER stored or displayed
- **ConfigMap Security**: ConfigMap values are not stored in the database
- **Local Only**: Designed to run locally or in a private network
- **No Authentication**: Add a reverse proxy (nginx/traefik) if exposing publicly

## Database Schema

```sql
CREATE TABLE change_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp DATETIME NOT NULL,
    namespace TEXT NOT NULL,
    kind TEXT NOT NULL,
    name TEXT NOT NULL,
    action TEXT NOT NULL,
    diff TEXT,
    metadata TEXT,
    image_before TEXT,
    image_after TEXT
);
```

## Performance

- **Memory**: ~50-100MB for typical workloads
- **CPU**: Minimal (<1% on idle)
- **Database**: ~1KB per event, ~1MB per 1000 events
- **Watch Connections**: 3 (Deployments, ConfigMaps, Secrets)

## Use Cases

✅ **Perfect for:**
- Small to medium Kubernetes clusters
- Development and staging environments
- Quick change visibility without setup overhead
- Debugging deployment issues
- Audit trail for configuration changes
- CI/CD integration monitoring

❌ **Not designed for:**
- Large-scale production clusters (1000+ deployments)
- Long-term audit compliance (use dedicated audit solutions)
- Real-time alerting (no notification system)
- Metric collection (use Prometheus for that)

## Troubleshooting

### "Failed to create config"
- Ensure kubeconfig is valid: `kubectl cluster-info`
- Check kubeconfig path: `--kubeconfig ~/.kube/config`

### "Permission denied" errors
- Verify RBAC permissions for reading resources
- K8Watch needs `get`, `list`, and `watch` on deployments, configmaps, and secrets

### Database locked errors
- Only run one instance per database file
- Check file permissions on events.db

### Events not appearing
- Check that resources are actually changing
- Verify namespace filters
- Review logs for watch errors

## Development

### Project Structure
```
k8watch/
├── cmd/k8watch/          # Main application entry point
├── internal/
│   ├── watcher/          # Kubernetes watchers
│   ├── storage/          # SQLite database layer
│   ├── api/              # HTTP API server
│   └── diff/             # Diff computation
├── web/                  # Frontend files
│   ├── index.html
│   ├── app.js
│   └── styles.css
├── Dockerfile
├── docker-compose.yml
└── README.md
```

### Building from Source
```bash
# Build binary
go build -o k8watch ./cmd/k8watch

# Build Docker image
docker build -t k8watch:latest .
```

### Running Tests
```bash
go test ./...
```

## Comparison with Other Tools

| Feature | K8Watch | Kubernetes Dashboard | Argo CD | Grafana+Prometheus |
|---------|---------|---------------------|---------|-------------------|
| Setup Time | 1 minute | 5-10 minutes | 30+ minutes | Hours |
| Change History | ✅ | ❌ | Limited | ❌ |
| Diffs | ✅ | ❌ | ✅ | ❌ |
| Image Tracking | ✅ | ✅ | ✅ | Requires config |
| Resource Usage | Very Low | Low | Medium | High |
| Dependencies | None | None | Many | Many |

## Contributing

Contributions are welcome! Please feel free to submit issues or pull requests.

## License

MIT License - see LICENSE file for details

## Support

For issues, questions, or suggestions, please open an issue on GitHub.

---

**Built for SREs, by SREs** 🛠️
