# gotunnel 🚇 

[![Go Version](https://img.shields.io/github/go-mod/go-version/johncferguson/gotunnel)](https://golang.org/)
[![Release](https://img.shields.io/github/v/release/johncferguson/gotunnel?include_prereleases)](https://github.com/johncferguson/gotunnel/releases)
[![CI/CD](https://github.com/johncferguson/gotunnel/workflows/CI%2FCD%20Pipeline/badge.svg)](https://github.com/johncferguson/gotunnel/actions)
[![Docker](https://img.shields.io/badge/docker-available-blue.svg)](https://ghcr.io/johncferguson/gotunnel)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Report Card](https://goreportcard.com/badge/github.com/johncferguson/gotunnel)](https://goreportcard.com/report/github.com/johncferguson/gotunnel)

**Create secure local tunnels for development without root privileges**

gotunnel provides secure HTTP/HTTPS tunnels for local development with built-in proxy capabilities, OpenTelemetry observability, and enterprise-friendly configuration options.

## ✨ Features

- 🔐 **No Root Required**: Works without administrator privileges
- 🌐 **Built-in HTTP Proxy**: No external dependencies needed
- 🏢 **Enterprise Ready**: Works with corporate firewalls and proxy settings
- 🔄 **Multiple Backends**: Support for nginx, Caddy, or built-in proxy
- 📊 **OpenTelemetry**: Full observability with traces, metrics, and logs
- 🖥️ **Cross-Platform**: Native support for macOS, Linux, and Windows
- 🐳 **Docker Ready**: Full containerization support with Compose
- 🔍 **Auto-Discovery**: mDNS support for network-wide access

## 🚀 Quick Start

### Installation

#### Package Managers

**Homebrew (macOS/Linux):**
```bash
brew tap ferg-cod3s/gotunnel
brew install gotunnel
```

**Scoop (Windows):**
```powershell
scoop bucket add ferg-cod3s https://github.com/ferg-cod3s/scoop-bucket
scoop install gotunnel
```

**APT (Debian/Ubuntu):**
```bash
curl -fsSL https://github.com/johncferguson/gotunnel/releases/latest/download/gotunnel_0.1.0-beta_amd64.deb -o gotunnel.deb
sudo dpkg -i gotunnel.deb
```

**AUR (Arch Linux):**
```bash
yay -S gotunnel
# or: paru -S gotunnel
```

#### Direct Installation

**Install Script (Unix):**
```bash
curl -sSL https://raw.githubusercontent.com/johncferguson/gotunnel/main/scripts/install.sh | bash
```

**Go Install:**
```bash
go install github.com/johncferguson/gotunnel/cmd/gotunnel@latest
```

**Docker:**
```bash
docker run --rm -p 80:80 -p 443:443 ghcr.io/johncferguson/gotunnel:latest
```

### Basic Usage

**Start a tunnel (no privileges required):**
```bash
# Tunnel your app running on port 3000
gotunnel --proxy=builtin --no-privilege-check start \
  --port 3000 --domain myapp --https=false
```

**Access your app:**
- Local: `http://localhost:3000`
- Tunnel: `http://myapp.local:8080` (with non-privileged ports)

**With HTTPS (default):**
```bash
gotunnel start --port 3000 --domain myapp
# Access at: https://myapp.local
```

**Multiple tunnels:**
```bash
# Terminal 1: Frontend
gotunnel start --port 3000 --domain frontend

# Terminal 2: API  
gotunnel start --port 8080 --domain api

# Terminal 3: Database Admin
gotunnel start --port 5432 --domain pgadmin
```

## 🏢 Enterprise Usage

### Custom Proxy Ports
```bash
# Use non-standard ports for corporate environments
gotunnel --proxy=builtin --proxy-http-port 8080 --proxy-https-port 8443 \
  start --port 3000 --domain myapp
```

### Configuration File
```bash
# Use configuration file (recommended for teams)
cp configs/gotunnel.example.yaml ~/.config/gotunnel/config.yaml
gotunnel start --port 3000 --domain myapp
```

### Generate Proxy Config Only
```bash
# Generate nginx/Caddy configuration without running proxy
gotunnel --proxy=config start --port 3000 --domain myapp
```

## 🐳 Docker Deployment

### Docker Compose (Recommended)

**Quick Start:**
```yaml
version: '3.8'
services:
  gotunnel:
    image: ghcr.io/johncferguson/gotunnel:latest
    ports:
      - "80:80"
      - "443:443"
    environment:
      - ENVIRONMENT=production
      - SENTRY_DSN=${SENTRY_DSN}
    volumes:
      - ./certs:/app/certs
      - ./config:/app/config
    restart: unless-stopped
```

**With Monitoring Stack:**
```bash
# Copy the provided docker-compose.yml
docker-compose --profile monitoring up -d

# Access services
open http://localhost:3000  # Grafana (admin/admin)
open http://localhost:9090  # Prometheus
```

### Kubernetes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gotunnel
spec:
  replicas: 1
  selector:
    matchLabels:
      app: gotunnel
  template:
    metadata:
      labels:
        app: gotunnel
    spec:
      containers:
      - name: gotunnel
        image: ghcr.io/johncferguson/gotunnel:latest
        ports:
        - containerPort: 80
        - containerPort: 443
        env:
        - name: ENVIRONMENT
          value: "production"
```

## ⚙️ Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `ENVIRONMENT` | Runtime environment | `development` |
| `SENTRY_DSN` | Sentry error tracking | - |
| `DEBUG` | Enable debug logging | `false` |
| `GOTUNNEL_PROXY` | Proxy mode | `auto` |
| `GOTUNNEL_PROXY_HTTP_PORT` | HTTP proxy port | `80` |
| `GOTUNNEL_PROXY_HTTPS_PORT` | HTTPS proxy port | `443` |

### Configuration File

Create `~/.config/gotunnel/config.yaml`:

```yaml
proxy:
  mode: "builtin"
  http_port: 8080
  https_port: 8443

observability:
  sentry:
    dsn: "${SENTRY_DSN}"
  logging:
    level: "info"
  tracing:
    enabled: true
    sample_rate: 1.0
```

## 📊 Observability

### Metrics

gotunnel exposes Prometheus metrics at `:9090/metrics`:

- `gotunnel_tunnels_active` - Number of active tunnels
- `gotunnel_requests_total` - Total HTTP requests processed
- `gotunnel_request_duration_seconds` - Request processing time
- `gotunnel_errors_total` - Total errors by type

### Tracing

Distributed tracing via OpenTelemetry:

```bash
# With OTLP endpoint
gotunnel --debug start --port 3000 --domain myapp
```

### Monitoring Stack

```bash
# Start with Prometheus + Grafana
docker-compose --profile monitoring up -d

# Access dashboards
open http://localhost:3000  # Grafana (admin/admin)
open http://localhost:9090  # Prometheus
```

## 📚 CLI Reference

### Global Flags

```bash
gotunnel [global options] command [command options] [arguments...]

GLOBAL OPTIONS:
   --no-privilege-check         Skip privilege check
   --sentry-dsn value           Sentry DSN for error tracking [$SENTRY_DSN]
   --environment value          Environment (development, staging, production) [$ENVIRONMENT]
   --debug                      Enable debug logging and tracing [$DEBUG]
   --proxy value                Proxy mode: builtin, nginx, caddy, auto, config, none [$GOTUNNEL_PROXY]
   --proxy-http-port value      HTTP port for proxy (default: 80) [$GOTUNNEL_PROXY_HTTP_PORT]
   --proxy-https-port value     HTTPS port for proxy (default: 443) [$GOTUNNEL_PROXY_HTTPS_PORT]
```

### Commands

```bash
gotunnel start --port 3000 --domain myapp    # Start tunnel
gotunnel stop myapp                           # Stop specific tunnel  
gotunnel list                                 # List active tunnels
gotunnel stop-all                            # Stop all tunnels
```

## 🛠️ Troubleshooting

### Common Issues

**"Permission denied" on ports 80/443:**
```bash
# Use non-privileged ports
gotunnel --proxy-http-port 8080 --proxy-https-port 8443 start --port 3000 --domain myapp
```

**"Domain not accessible":**
```bash
# Check /etc/hosts
cat /etc/hosts | grep myapp.local

# Check DNS resolution
dig myapp.local
nslookup myapp.local
```

**Corporate proxy issues:**
```bash
# Disable proxy auto-detection
gotunnel --proxy=builtin --no-privilege-check start --port 3000 --domain myapp
```

### Debug Mode

```bash
# Enable debug logging
gotunnel --debug start --port 3000 --domain myapp

# Check observability
curl http://localhost:9090/metrics
```

### System Service

**systemd (Linux):**
```bash
# Install as system service
sudo ./scripts/install.sh --service

# Control service
sudo systemctl start gotunnel
sudo systemctl enable gotunnel
sudo journalctl -u gotunnel -f
```

**launchd (macOS):**
```bash
# Install via Homebrew (includes service)
brew services start gotunnel
brew services stop gotunnel
```

## 🧪 Development

### Building from Source

```bash
git clone https://github.com/johncferguson/gotunnel.git
cd gotunnel
go mod tidy
go build ./cmd/gotunnel
```

### Running Tests

```bash
go test ./...                    # All tests
go test ./internal/tunnel -v     # Specific package
go test -race ./...              # Race detection
```

### Quality Checks

```bash
golangci-lint run               # Full linting
go fmt ./...                    # Format code
go vet ./...                    # Static analysis
```

### CI/CD Pipeline

The project uses GitHub Actions for:
- ✅ Multi-platform testing (Go 1.22, 1.23)
- ✅ Comprehensive linting with golangci-lint
- ✅ Security scanning with gosec and Trivy
- ✅ Multi-architecture builds (Linux, macOS, Windows - amd64/arm64)
- ✅ Docker image building and publishing
- ✅ Automated releases and package distribution
- ✅ Code signing for macOS binaries

## 🔒 Security

### Reporting Vulnerabilities

Please report security vulnerabilities via [GitHub Security Advisories](https://github.com/johncferguson/gotunnel/security/advisories/new).

### Security Features

- **Code Signing**: macOS binaries are signed with Apple Developer ID and notarized
- **No Root Required**: Core functionality works without administrator privileges
- **Automatic Certificate Management**: Self-signed certificates for HTTPS tunnels
- **Host File Safety**: Automatic backup and restoration of system hosts file
- **Network Isolation**: Docker containers with proper security boundaries
- **Security Scanning**: Automated vulnerability scanning in CI/CD pipeline
- **Encrypted Secrets**: All sensitive data handled through encrypted GitHub secrets

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Development Guidelines

- Follow Go best practices and idioms
- Add tests for new functionality
- Update documentation for user-facing changes
- Use conventional commits for clear history
- Ensure all CI checks pass

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🆘 Support

- 📚 [Documentation](https://gotunnel.dev)
- 🐛 [Issue Tracker](https://github.com/johncferguson/gotunnel/issues)
- 💬 [Discussions](https://github.com/johncferguson/gotunnel/discussions)
- 🔐 [Security](https://github.com/johncferguson/gotunnel/security)

## 🙏 Acknowledgments

- Built with [Go](https://golang.org/) and love ❤️
- Observability powered by [OpenTelemetry](https://opentelemetry.io/)
- Error tracking via [Sentry](https://sentry.io/)
- CLI framework by [urfave/cli](https://github.com/urfave/cli)

---

**gotunnel** - Making local development tunnels simple and secure 🚇