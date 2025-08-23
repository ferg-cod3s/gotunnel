# gotunnel API Reference

## Table of Contents

- [CLI Reference](#cli-reference)
  - [Global Options](#global-options)
  - [Commands](#commands)
  - [Environment Variables](#environment-variables)
- [Configuration API](#configuration-api)
  - [Configuration File](#configuration-file)
  - [Configuration Schema](#configuration-schema)
- [Programmatic API](#programmatic-api)
  - [Go Package API](#go-package-api)
  - [REST API (Future)](#rest-api-future)
- [Proxy Backends](#proxy-backends)
- [Observability API](#observability-api)
- [Examples](#examples)

## CLI Reference

### Synopsis

```bash
gotunnel [global options] command [command options] [arguments...]
```

### Global Options

| Option | Short | Default | Environment Variable | Description |
|--------|-------|---------|---------------------|-------------|
| `--no-privilege-check` | | `false` | `GOTUNNEL_NO_PRIVILEGE_CHECK` | Skip privilege check for operations |
| `--sentry-dsn` | | | `SENTRY_DSN` | Sentry DSN for error tracking |
| `--environment` | `-e` | `development` | `ENVIRONMENT` | Environment (development, staging, production) |
| `--debug` | `-d` | `false` | `DEBUG` | Enable debug logging and tracing |
| `--proxy` | `-p` | `auto` | `GOTUNNEL_PROXY` | Proxy mode: builtin, nginx, caddy, auto, config, none |
| `--proxy-http-port` | | `80` | `GOTUNNEL_PROXY_HTTP_PORT` | HTTP port for proxy |
| `--proxy-https-port` | | `443` | `GOTUNNEL_PROXY_HTTPS_PORT` | HTTPS port for proxy |
| `--config` | `-c` | | `GOTUNNEL_CONFIG` | Path to configuration file |
| `--log-level` | | `info` | `GOTUNNEL_LOG_LEVEL` | Log level (trace, debug, info, warn, error) |
| `--log-format` | | `text` | `GOTUNNEL_LOG_FORMAT` | Log format (text, json) |
| `--help` | `-h` | | | Show help |
| `--version` | `-v` | | | Print version |

### Commands

#### `start` - Start a tunnel

Create and start a new tunnel for a local service.

```bash
gotunnel start [options]
```

**Options:**

| Option | Short | Default | Description |
|--------|-------|---------|-------------|
| `--port` | `-p` | Required | Local port to tunnel |
| `--domain` | `-d` | Required | Domain name for the tunnel |
| `--https` | `-s` | `true` | Enable HTTPS |
| `--host` | `-H` | `localhost` | Local host to tunnel |
| `--backend` | `-b` | | Backend URL (overrides host/port) |
| `--headers` | | | Custom headers (JSON format) |
| `--basic-auth` | | | Enable basic auth (user:pass) |
| `--timeout` | `-t` | `30s` | Request timeout |
| `--name` | `-n` | | Tunnel name (defaults to domain) |

**Examples:**

```bash
# Basic tunnel
gotunnel start --port 3000 --domain myapp

# With custom host
gotunnel start --port 8080 --domain api --host 192.168.1.100

# HTTP only
gotunnel start --port 3000 --domain myapp --https=false

# With basic auth
gotunnel start --port 3000 --domain secure --basic-auth admin:password

# Custom headers
gotunnel start --port 3000 --domain api --headers '{"X-API-Key": "secret"}'
```

#### `stop` - Stop a tunnel

Stop and remove an existing tunnel.

```bash
gotunnel stop <tunnel-name|domain>
```

**Arguments:**
- `tunnel-name|domain` - Name or domain of the tunnel to stop

**Examples:**

```bash
# Stop by domain
gotunnel stop myapp.local

# Stop by name
gotunnel stop frontend-tunnel
```

#### `list` - List active tunnels

Display all currently active tunnels.

```bash
gotunnel list [options]
```

**Options:**

| Option | Short | Default | Description |
|--------|-------|---------|-------------|
| `--format` | `-f` | `table` | Output format (table, json, yaml) |
| `--verbose` | `-v` | `false` | Show detailed information |

**Examples:**

```bash
# List all tunnels
gotunnel list

# JSON output
gotunnel list --format json

# Detailed view
gotunnel list --verbose
```

#### `stop-all` - Stop all tunnels

Stop and remove all active tunnels.

```bash
gotunnel stop-all [options]
```

**Options:**

| Option | Short | Default | Description |
|--------|-------|---------|-------------|
| `--force` | `-f` | `false` | Force stop without confirmation |

**Examples:**

```bash
# Stop all with confirmation
gotunnel stop-all

# Force stop all
gotunnel stop-all --force
```

#### `status` - Check tunnel status

Get detailed status of a specific tunnel.

```bash
gotunnel status <tunnel-name|domain>
```

**Examples:**

```bash
# Check status
gotunnel status myapp.local
```

#### `config` - Configuration management

Manage gotunnel configuration.

```bash
gotunnel config <subcommand> [options]
```

**Subcommands:**

- `init` - Initialize configuration file
- `validate` - Validate configuration file
- `show` - Display current configuration
- `set` - Set configuration value
- `get` - Get configuration value

**Examples:**

```bash
# Initialize config
gotunnel config init

# Validate config
gotunnel config validate --file ./gotunnel.yaml

# Show current config
gotunnel config show

# Set a value
gotunnel config set proxy.mode builtin

# Get a value
gotunnel config get proxy.http_port
```

### Environment Variables

All environment variables are prefixed with `GOTUNNEL_` (except standard ones like `SENTRY_DSN`).

```bash
# Core configuration
export GOTUNNEL_PROXY=builtin
export GOTUNNEL_PROXY_HTTP_PORT=8080
export GOTUNNEL_PROXY_HTTPS_PORT=8443
export GOTUNNEL_CONFIG=/etc/gotunnel/config.yaml

# Logging
export GOTUNNEL_LOG_LEVEL=debug
export GOTUNNEL_LOG_FORMAT=json

# Observability
export SENTRY_DSN=https://key@sentry.io/project
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
export OTEL_SERVICE_NAME=gotunnel
export OTEL_RESOURCE_ATTRIBUTES="environment=production"

# Runtime
export ENVIRONMENT=production
export DEBUG=true
```

## Configuration API

### Configuration File

gotunnel supports YAML and JSON configuration files.

**Default locations (in order of precedence):**
1. `./gotunnel.yaml` or `./gotunnel.json`
2. `~/.config/gotunnel/config.yaml` or `~/.config/gotunnel/config.json`
3. `/etc/gotunnel/config.yaml` or `/etc/gotunnel/config.json`

### Configuration Schema

```yaml
# gotunnel.yaml
version: "1.0"

# Server configuration
server:
  host: "0.0.0.0"
  admin_port: 9999
  metrics_port: 9090
  health_port: 8888

# Proxy configuration
proxy:
  mode: "builtin"  # builtin, nginx, caddy, auto, config, none
  http_port: 80
  https_port: 443
  config_path: "/etc/nginx/sites-enabled"
  
  # Builtin proxy settings
  builtin:
    max_connections: 1000
    idle_timeout: "60s"
    read_timeout: "30s"
    write_timeout: "30s"
    max_header_bytes: 1048576
    
  # Nginx settings
  nginx:
    binary: "/usr/sbin/nginx"
    config_template: "/path/to/template.conf"
    reload_command: "nginx -s reload"
    
  # Caddy settings
  caddy:
    binary: "/usr/local/bin/caddy"
    config_adapter: "caddyfile"
    admin_endpoint: "http://localhost:2019"

# Certificate configuration
certificates:
  provider: "self-signed"  # self-signed, letsencrypt, custom
  storage: "~/.gotunnel/certs"
  ca_cert: "~/.gotunnel/ca.crt"
  ca_key: "~/.gotunnel/ca.key"
  
  # Let's Encrypt settings (future)
  letsencrypt:
    email: "admin@example.com"
    staging: false
    dns_provider: "cloudflare"
    
  # Self-signed settings
  self_signed:
    validity_days: 365
    key_size: 2048
    organization: "gotunnel Development"
    country: "US"

# DNS configuration
dns:
  enable_mdns: true
  mdns_domain: ".local"
  update_hosts: true
  hosts_file: "/etc/hosts"
  
  # mDNS settings
  mdns:
    service_name: "_gotunnel._tcp"
    port: 5353
    ttl: 120
    interfaces: []  # Empty means all interfaces

# Observability configuration
observability:
  # Logging
  logging:
    level: "info"  # trace, debug, info, warn, error
    format: "json"  # text, json
    output: "stdout"  # stdout, stderr, file
    file: "/var/log/gotunnel.log"
    sampling: false
    
  # Metrics
  metrics:
    enabled: true
    endpoint: "/metrics"
    interval: "30s"
    exporters:
      - prometheus:
          endpoint: ":9090/metrics"
      - otlp:
          endpoint: "http://localhost:4318"
          
  # Tracing
  tracing:
    enabled: true
    sampler: "always"  # always, never, ratio
    sample_rate: 1.0
    exporters:
      - otlp:
          endpoint: "http://localhost:4318"
      - jaeger:
          endpoint: "http://localhost:14268"
          
  # Error tracking
  sentry:
    dsn: "${SENTRY_DSN}"
    environment: "${ENVIRONMENT}"
    sample_rate: 1.0
    traces_sample_rate: 0.1
    attach_stacktrace: true
    
  # Health checks
  health:
    enabled: true
    endpoint: "/health"
    readiness_endpoint: "/ready"
    liveness_endpoint: "/alive"

# Tunnel defaults
tunnel_defaults:
  https: true
  timeout: "30s"
  max_idle_conns: 100
  idle_conn_timeout: "90s"
  tls_handshake_timeout: "10s"
  expect_continue_timeout: "1s"
  
# Security configuration
security:
  enable_auth: false
  auth_type: "basic"  # basic, oauth, jwt
  
  # Basic auth
  basic_auth:
    users:
      - username: "admin"
        password_hash: "$2a$10$..."  # bcrypt hash
        
  # Rate limiting
  rate_limiting:
    enabled: false
    requests_per_minute: 60
    burst: 10
    
  # IP filtering
  ip_filtering:
    enabled: false
    allow: []
    deny: []

# Predefined tunnels
tunnels:
  - name: "frontend"
    domain: "app"
    port: 3000
    https: true
    auto_start: true
    
  - name: "api"
    domain: "api"
    port: 8080
    https: true
    headers:
      X-API-Version: "v1"
    auto_start: false
    
  - name: "database"
    domain: "db"
    port: 5432
    https: false
    host: "192.168.1.100"
    auto_start: false
```

## Programmatic API

### Go Package API

gotunnel can be used as a Go library for embedding tunnel functionality.

```go
import "github.com/johncferguson/gotunnel/pkg/tunnel"

// Create tunnel manager
manager, err := tunnel.NewManager(tunnel.Config{
    Proxy: tunnel.ProxyConfig{
        Mode: "builtin",
        HTTPPort: 8080,
        HTTPSPort: 8443,
    },
})

// Start a tunnel
t, err := manager.CreateTunnel(tunnel.TunnelConfig{
    Name:   "myapp",
    Domain: "myapp.local",
    Port:   3000,
    HTTPS:  true,
})

// Get tunnel status
status := t.Status()
fmt.Printf("Tunnel %s is %s\n", status.Name, status.State)

// Stop tunnel
err = manager.StopTunnel("myapp")

// List all tunnels
tunnels := manager.ListTunnels()
for _, t := range tunnels {
    fmt.Printf("%s -> %s\n", t.Domain, t.Backend)
}

// Cleanup
manager.Shutdown()
```

### REST API (Future)

A REST API is planned for future releases:

```bash
# Start tunnel
POST /api/v1/tunnels
{
  "name": "myapp",
  "domain": "myapp.local",
  "port": 3000,
  "https": true
}

# List tunnels
GET /api/v1/tunnels

# Get tunnel details
GET /api/v1/tunnels/{name}

# Stop tunnel
DELETE /api/v1/tunnels/{name}

# Health check
GET /api/v1/health

# Metrics
GET /api/v1/metrics
```

## Proxy Backends

### Builtin Proxy

Native Go HTTP/HTTPS reverse proxy.

**Features:**
- Zero dependencies
- Connection pooling
- Header manipulation
- Request/response modification
- WebSocket support

**Configuration:**
```yaml
proxy:
  mode: "builtin"
  builtin:
    max_connections: 1000
    idle_timeout: "60s"
```

### Nginx Backend

Uses nginx as the proxy engine.

**Requirements:**
- nginx installed on system
- Write access to nginx config directory

**Configuration:**
```yaml
proxy:
  mode: "nginx"
  nginx:
    binary: "/usr/sbin/nginx"
    config_path: "/etc/nginx/sites-enabled"
```

### Caddy Backend

Uses Caddy server as the proxy engine.

**Requirements:**
- Caddy v2 installed
- Caddy admin API enabled

**Configuration:**
```yaml
proxy:
  mode: "caddy"
  caddy:
    admin_endpoint: "http://localhost:2019"
```

### Config Mode

Generates configuration files without starting proxy.

**Use cases:**
- Integration with existing proxy setups
- Custom proxy configurations
- Debugging and testing

**Example:**
```bash
gotunnel --proxy=config start --port 3000 --domain myapp
# Outputs nginx/caddy config to stdout
```

## Observability API

### Metrics Endpoint

Prometheus-compatible metrics available at `/metrics`:

```prometheus
# Tunnel metrics
gotunnel_tunnels_active{} 3
gotunnel_tunnels_total{} 10
gotunnel_tunnel_duration_seconds{tunnel="myapp"} 3600

# Request metrics
gotunnel_requests_total{tunnel="myapp",method="GET",status="200"} 1234
gotunnel_request_duration_seconds{tunnel="myapp",quantile="0.99"} 0.05

# Error metrics
gotunnel_errors_total{tunnel="myapp",type="proxy"} 5

# Certificate metrics
gotunnel_certificate_expiry_days{domain="myapp.local"} 364

# System metrics
gotunnel_process_cpu_seconds_total{} 123.45
gotunnel_process_memory_bytes{} 67890123
```

### Health Endpoints

```bash
# Liveness check
GET /health/alive
200 OK
{
  "status": "alive",
  "timestamp": "2024-08-13T10:00:00Z"
}

# Readiness check
GET /health/ready
200 OK
{
  "status": "ready",
  "tunnels": 3,
  "proxy": "running",
  "timestamp": "2024-08-13T10:00:00Z"
}

# Detailed health
GET /health
200 OK
{
  "status": "healthy",
  "version": "0.1.0-beta",
  "uptime": "1h30m",
  "tunnels": {
    "active": 3,
    "total": 10
  },
  "proxy": {
    "type": "builtin",
    "status": "running",
    "ports": [80, 443]
  },
  "certificates": {
    "valid": 3,
    "expiring_soon": 0
  }
}
```

### OpenTelemetry Integration

```yaml
# Enable OTLP export
observability:
  tracing:
    exporters:
      - otlp:
          endpoint: "http://collector:4318"
          headers:
            api-key: "${OTLP_API_KEY}"
```

## Examples

### Basic Development Setup

```bash
# Start frontend tunnel
gotunnel start --port 3000 --domain frontend

# Start API tunnel
gotunnel start --port 8080 --domain api

# Start database admin tunnel
gotunnel start --port 5432 --domain pgadmin
```

### Production Configuration

```yaml
# /etc/gotunnel/config.yaml
version: "1.0"

proxy:
  mode: "builtin"
  http_port: 80
  https_port: 443

certificates:
  provider: "letsencrypt"
  letsencrypt:
    email: "admin@company.com"

observability:
  logging:
    level: "info"
    format: "json"
  metrics:
    enabled: true
  tracing:
    enabled: true
    sampler: "ratio"
    sample_rate: 0.1

security:
  rate_limiting:
    enabled: true
    requests_per_minute: 100

tunnels:
  - name: "production-app"
    domain: "app.company.local"
    port: 3000
    https: true
    auto_start: true
```

### Docker Compose Integration

```yaml
version: '3.8'

services:
  gotunnel:
    image: ghcr.io/johncferguson/gotunnel:latest
    command: start --port 3000 --domain app
    environment:
      - GOTUNNEL_PROXY=builtin
      - GOTUNNEL_PROXY_HTTP_PORT=8080
      - GOTUNNEL_PROXY_HTTPS_PORT=8443
    ports:
      - "8080:8080"
      - "8443:8443"
    volumes:
      - ./config:/etc/gotunnel
      
  app:
    image: myapp:latest
    ports:
      - "3000:3000"
```

### Kubernetes ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: gotunnel-config
data:
  config.yaml: |
    version: "1.0"
    proxy:
      mode: "builtin"
    tunnels:
      - name: "app"
        domain: "app.cluster.local"
        port: 8080
        auto_start: true
```

### Scripted Automation

```bash
#!/bin/bash
# start-dev-env.sh

# Start all development tunnels
gotunnel start --port 3000 --domain frontend &
gotunnel start --port 8080 --domain api &
gotunnel start --port 5432 --domain database &

# Wait for user input
echo "Development tunnels started. Press Enter to stop..."
read

# Cleanup
gotunnel stop-all --force
```

---

*This API documentation is version 0.1.0-beta and subject to change. For the latest updates, see the [GitHub repository](https://github.com/johncferguson/gotunnel).*
