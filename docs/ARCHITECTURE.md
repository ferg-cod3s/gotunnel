# gotunnel Architecture

## Table of Contents

- [Overview](#overview)
- [System Architecture](#system-architecture)
- [Core Components](#core-components)
- [Data Flow](#data-flow)
- [Security Architecture](#security-architecture)
- [Deployment Architecture](#deployment-architecture)
- [Technology Stack](#technology-stack)
- [Design Patterns](#design-patterns)
- [Future Architecture](#future-architecture)

## Overview

gotunnel is a secure local tunneling solution designed to expose local development servers to the network without requiring root privileges. The architecture prioritizes security, simplicity, and cross-platform compatibility.

### Design Principles

1. **No Root Required**: Core functionality without elevated privileges
2. **Security First**: Defense in depth with multiple security layers
3. **Platform Agnostic**: Consistent behavior across operating systems
4. **Modular Design**: Loosely coupled components with clear interfaces
5. **Observable**: Built-in telemetry and monitoring capabilities
6. **Fail Safe**: Graceful degradation and automatic cleanup

## System Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                         Client Layer                         │
├───────────────┬───────────────┬───────────────┬─────────────┤
│      CLI      │   Config File  │  Environment  │     API     │
└───────┬───────┴───────┬───────┴───────┬───────┴─────────────┘
        │               │               │
        └───────────────┼───────────────┘
                        │
┌───────────────────────▼───────────────────────────────────────┐
│                    Application Core                           │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────┐       │
│  │   Tunnel    │  │ Certificate  │  │     DNS      │       │
│  │   Manager   │  │   Manager    │  │   Manager    │       │
│  └─────────────┘  └──────────────┘  └──────────────┘       │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────┐       │
│  │    Proxy    │  │    Config    │  │ Observability│       │
│  │   Engine    │  │   Manager    │  │   Manager    │       │
│  └─────────────┘  └──────────────┘  └──────────────┘       │
└───────────────────────────────────────────────────────────┘
                        │
┌───────────────────────▼───────────────────────────────────────┐
│                    System Layer                               │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────┐       │
│  │  Host File  │  │  Certificate │  │   Network    │       │
│  │  Management │  │     Store    │  │  Interfaces  │       │
│  └─────────────┘  └──────────────┘  └──────────────┘       │
└───────────────────────────────────────────────────────────┘
```

## Core Components

### 1. Tunnel Manager (`internal/tunnel/`)

**Responsibility**: Lifecycle management of tunnels

```go
type TunnelManager struct {
    tunnels    map[string]*Tunnel
    proxy      ProxyEngine
    cert       CertificateManager
    dns        DNSManager
    config     *Config
    telemetry  *TelemetryManager
}
```

**Key Functions**:
- Create, start, stop, and destroy tunnels
- Manage tunnel state and health
- Coordinate with other managers
- Handle concurrent tunnel operations
- Resource cleanup and recovery

### 2. Certificate Manager (`internal/cert/`)

**Responsibility**: TLS certificate generation and management

```go
type CertificateManager struct {
    store      CertificateStore
    generator  *CertGenerator
    validator  *CertValidator
    rootCA     *x509.Certificate
}
```

**Key Functions**:
- Generate self-signed certificates
- Store certificates securely
- Validate certificate chains
- Monitor certificate expiry
- Platform-specific certificate installation

### 3. DNS Manager (`internal/dns/`)

**Responsibility**: DNS registration and discovery

```go
type DNSManager struct {
    mdns       *MDNSServer
    hosts      HostsFileManager
    resolver   *Resolver
}
```

**Key Functions**:
- mDNS service advertisement
- Local hosts file management
- DNS resolution and caching
- Service discovery
- Network interface monitoring

### 4. Proxy Engine (`internal/proxy/`)

**Responsibility**: HTTP/HTTPS request routing

```go
type ProxyEngine interface {
    Start(config ProxyConfig) error
    Stop() error
    AddRoute(domain string, backend string) error
    RemoveRoute(domain string) error
}

// Implementations
type BuiltinProxy struct {}  // Native Go implementation
type NginxProxy struct {}    // Nginx backend
type CaddyProxy struct {}    // Caddy backend
```

**Key Functions**:
- HTTP/HTTPS reverse proxy
- Request routing and load balancing
- Header manipulation
- TLS termination
- Connection pooling

### 5. Configuration Manager (`internal/config/`)

**Responsibility**: Configuration loading and validation

```go
type ConfigManager struct {
    sources    []ConfigSource
    validator  *ConfigValidator
    defaults   *DefaultConfig
}
```

**Configuration Hierarchy**:
1. CLI flags (highest priority)
2. Environment variables
3. Configuration file
4. Default values (lowest priority)

### 6. Observability Manager (`internal/observability/`)

**Responsibility**: Logging, metrics, and tracing

```go
type ObservabilityManager struct {
    logger     *slog.Logger
    tracer     trace.Tracer
    meter      metric.Meter
    sentry     *sentry.Client
}
```

**Telemetry Signals**:
- **Logs**: Structured logging with slog
- **Metrics**: Prometheus-compatible metrics
- **Traces**: OpenTelemetry distributed tracing
- **Errors**: Sentry error tracking

## Data Flow

### Tunnel Creation Flow

```
User Request → CLI Parser → Config Validation
                              ↓
                    Tunnel Manager.Create()
                              ↓
                    ┌─────────┴──────────┐
                    ↓                    ↓
            Certificate.Generate()  DNS.Register()
                    ↓                    ↓
            Certificate.Install()   Hosts.Update()
                    ↓                    ↓
                    └─────────┬──────────┘
                              ↓
                       Proxy.AddRoute()
                              ↓
                       Tunnel.Start()
                              ↓
                        User Response
```

### HTTP Request Flow

```
External Request (http://app.local)
            ↓
    DNS Resolution (mDNS/hosts)
            ↓
    Proxy Engine (port 80/443)
            ↓
    Route Lookup (domain → backend)
            ↓
    TLS Termination (if HTTPS)
            ↓
    Backend Connection (localhost:port)
            ↓
    Response Processing
            ↓
    Client Response
```

## Security Architecture

### Defense in Depth

```
┌──────────────────────────────────────────┐
│          Layer 1: Input Validation       │
│  • Command injection prevention          │
│  • Path traversal protection            │
│  • Input sanitization                   │
└──────────────────────────────────────────┘
                    ↓
┌──────────────────────────────────────────┐
│         Layer 2: Process Isolation       │
│  • No shared state between tunnels      │
│  • Resource limits                      │
│  • Graceful degradation                 │
└──────────────────────────────────────────┘
                    ↓
┌──────────────────────────────────────────┐
│        Layer 3: Network Security         │
│  • TLS encryption                       │
│  • Certificate validation               │
│  • Rate limiting                        │
└──────────────────────────────────────────┘
                    ↓
┌──────────────────────────────────────────┐
│        Layer 4: System Protection        │
│  • Minimal privileges                   │
│  • Secure file operations               │
│  • Audit logging                        │
└──────────────────────────────────────────┘
```

### Privilege Model

```
Standard User Privileges:
• Create tunnels on non-privileged ports (>1024)
• Generate self-signed certificates
• Use mDNS for discovery
• Modify user-level configuration

Optional Elevated Privileges:
• Bind to ports 80/443
• Modify system hosts file
• Install CA certificates system-wide
• Access system certificate stores
```

## Deployment Architecture

### Local Development

```
┌─────────────┐     ┌──────────────┐     ┌──────────────┐
│   gotunnel  │────▶│ Local Proxy  │────▶│   Dev App    │
│     CLI     │     │  (port 8080) │     │ (port 3000)  │
└─────────────┘     └──────────────┘     └──────────────┘
```

### Docker Deployment

```
┌──────────────────────────────────────────┐
│            Docker Host                    │
│  ┌────────────────────────────────────┐  │
│  │       gotunnel Container           │  │
│  │  ┌──────────┐  ┌──────────────┐  │  │
│  │  │ gotunnel │  │ Proxy Engine │  │  │
│  │  └──────────┘  └──────────────┘  │  │
│  └────────────────────────────────────┘  │
│  ┌────────────────────────────────────┐  │
│  │      Application Container         │  │
│  └────────────────────────────────────┘  │
└──────────────────────────────────────────┘
```

### Kubernetes Deployment

```yaml
apiVersion: v1
kind: Service
metadata:
  name: gotunnel
spec:
  type: LoadBalancer
  ports:
    - port: 80
    - port: 443
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gotunnel
spec:
  template:
    spec:
      containers:
      - name: gotunnel
        image: ghcr.io/johncferguson/gotunnel
      - name: app
        image: myapp:latest
```

## Technology Stack

### Core Technologies

| Component | Technology | Rationale |
|-----------|-----------|-----------|
| Language | Go 1.22+ | Performance, simplicity, cross-platform |
| CLI | urfave/cli | Feature-rich, well-maintained |
| mDNS | grandcat/zeroconf | Cross-platform mDNS support |
| Testing | testify | Comprehensive assertion library |
| Observability | OpenTelemetry | Vendor-neutral telemetry |
| Error Tracking | Sentry | Production error monitoring |

### Platform-Specific

| Platform | Certificate Store | Hosts File | Service |
|----------|------------------|------------|---------|
| macOS | Keychain | /etc/hosts | launchd |
| Linux | System CA bundle | /etc/hosts | systemd |
| Windows | Windows Certificate Store | C:\Windows\System32\drivers\etc\hosts | Windows Service |

## Design Patterns

### 1. Strategy Pattern
Used for proxy engine selection (builtin, nginx, caddy)

### 2. Factory Pattern
Certificate and tunnel creation with platform-specific implementations

### 3. Observer Pattern
Event-driven updates for tunnel state changes

### 4. Repository Pattern
Abstract storage layer for certificates and configuration

### 5. Dependency Injection
All managers receive dependencies through constructors

### 6. Interface Segregation
Small, focused interfaces for each component

## Future Architecture

### Planned Enhancements

#### 1. Plugin System
```go
type Plugin interface {
    Name() string
    Init(config PluginConfig) error
    OnTunnelCreate(tunnel *Tunnel) error
    OnRequest(req *http.Request) error
}
```

#### 2. Distributed Mode
```
┌──────────┐     ┌──────────┐     ┌──────────┐
│  Agent 1 │────▶│  Control │◀────│  Agent 2 │
└──────────┘     │   Plane  │     └──────────┘
                 └──────────┘
```

#### 3. Service Mesh Integration
- Envoy proxy support
- Istio compatibility
- Service discovery integration

#### 4. Advanced Routing
```yaml
routes:
  - domain: api.local
    backends:
      - url: http://localhost:3000
        weight: 80
      - url: http://localhost:3001
        weight: 20
    healthcheck:
      path: /health
      interval: 30s
```

### Performance Optimizations

1. **Connection Pooling**: Reuse backend connections
2. **Caching Layer**: Cache DNS resolutions and certificates
3. **Zero-Copy Proxying**: Direct memory transfers
4. **Async I/O**: Non-blocking operations throughout

### Scalability Considerations

- Horizontal scaling with multiple proxy instances
- State synchronization via distributed cache
- Load balancing across tunnel endpoints
- Auto-scaling based on traffic patterns

## Architecture Decision Records

### ADR-001: Go as Primary Language
**Status**: Accepted
**Context**: Need cross-platform support with good performance
**Decision**: Use Go for its simplicity, performance, and deployment story
**Consequences**: Single binary distribution, excellent cross-compilation

### ADR-002: No External Dependencies for Core
**Status**: Accepted
**Context**: Minimize deployment complexity
**Decision**: Core functionality uses only Go standard library where possible
**Consequences**: Larger binary but simpler deployment

### ADR-003: Plugin Architecture
**Status**: Proposed
**Context**: Need extensibility without modifying core
**Decision**: Implement plugin system using Go plugins or WASM
**Consequences**: More complex but highly extensible

---

*This architecture document is maintained alongside the codebase and updated as the system evolves.*
