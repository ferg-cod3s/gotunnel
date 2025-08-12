# TODO: Production Readiness Implementation

> **📋 Reference Documents:**
> - [docs/PRD.md](./PRD.md) - Product Requirements and Feature Definitions
> - [docs/DEPLOYMENT.md](./DEPLOYMENT.md) - Deployment Strategy and Observability Integration
> - [CLAUDE.md](../CLAUDE.md) - Development Guidelines

## Phase 1: Foundation & Observability (Week 1-2)

### 🔴 P0: OpenTelemetry + Sentry Integration
**Objective**: Implement comprehensive observability with OpenTelemetry and Sentry
**Success Criteria**: All operations traced, errors tracked, performance monitored

#### Tasks:
- [ ] **Add OpenTelemetry Dependencies**
  ```bash
  go get go.opentelemetry.io/otel@v1.24.0
  go get go.opentelemetry.io/otel/trace@v1.24.0
  go get go.opentelemetry.io/otel/metric@v1.24.0
  go get go.opentelemetry.io/otel/sdk@v1.24.0
  go get github.com/getsentry/sentry-go@v0.27.0
  ```
- [ ] **Create Observability Provider**
  - `internal/observability/provider.go` - Central telemetry manager
  - Resource detection (service name, version, environment)
  - Sentry initialization with OpenTelemetry integration
- [ ] **Implement Structured Logging**
  - Replace `log` package with `slog` + OpenTelemetry correlation
  - JSON format for production, text for development
  - Trace ID correlation in all log messages
- [ ] **Add Distributed Tracing**
  - Instrument tunnel lifecycle operations
  - Certificate generation and installation
  - DNS registration and mDNS operations
  - HTTP request proxying
- [ ] **Custom Metrics Integration**
  - Tunnel count and duration metrics
  - HTTP request metrics (count, latency, errors)
  - Certificate expiry monitoring
  - Error categorization and rates
- [ ] **Sentry Error Tracking**
  - Error categorization by operation type
  - Performance monitoring for key operations
  - Release tracking and commit attribution
  - User context (without PII) for debugging

#### Implementation Priority:
```go
// internal/observability/provider.go
type Provider struct {
    logger    *slog.Logger
    tracer    trace.Tracer
    meter     metric.Meter
    sentry    *sentry.Hub
    resource  *resource.Resource
}

// Key integration points:
// 1. cmd/gotunnel/main.go - Initialize provider
// 2. internal/tunnel/manager.go - Instrument tunnel operations
// 3. internal/cert/manager.go - Instrument certificate operations
// 4. internal/dnsserver/server.go - Instrument DNS operations
```

### 🔴 P0: Testing Infrastructure
**Current Issue**: Tests failing, port conflicts, privilege requirements
**Impact**: Cannot verify functionality, blocks CI/CD

#### Tasks:
- [ ] **Fix Test Port Management**
  - Implement dynamic port allocation for tests
  - Add port cleanup mechanisms
  - Use ephemeral ports for test isolation
- [ ] **Mock Privileged Operations**
  - Create interface abstractions for system operations
  - Mock hosts file modifications
  - Mock certificate installation
- [ ] **Test Environment Setup**
  - Add test-specific configuration
  - Create in-memory state management for tests
  - Add test utilities for tunnel lifecycle

#### Architectural Changes:
```go
// internal/system/interfaces.go
type HostsManager interface {
    AddEntry(domain, ip string) error
    RemoveEntry(domain string) error
    Backup() error
    Restore() error
}

type CertInstaller interface {
    InstallCA() error
    GenerateCert(domain string) (*tls.Certificate, error)
    CleanupCert(domain string) error
}

// internal/system/mock.go - for testing
type MockHostsManager struct{}
type MockCertInstaller struct{}
```

### 🔴 P0: Security & Privilege Model
**Current Issue**: Privilege checking disabled, security vulnerabilities
**Impact**: Installation failures, security risks

#### Tasks:
- [ ] **Implement Proper Privilege Detection**
  - Check for required permissions before operations
  - Provide clear error messages when privileges missing
  - Support elevation prompts
- [ ] **Secure Hosts File Management**
  - Atomic file operations with backup/restore
  - Validation of entries before modification
  - Cleanup on application termination
- [ ] **Certificate Security**
  - Validate certificate paths and permissions
  - Secure key storage
  - Certificate rotation support

#### Architectural Changes:
```go
// internal/privilege/manager.go
type PrivilegeManager struct {
    required []Permission
}

type Permission int
const (
    BindPrivilegedPorts Permission = iota
    ModifyHostsFile
    InstallCertificates
)

func (pm *PrivilegeManager) Check() error
func (pm *PrivilegeManager) RequestElevation() error
func (pm *PrivilegeManager) CanPerform(perm Permission) bool
```

### 🔴 P0: CI/CD Pipeline
**Current Issue**: No automated testing/building/releasing
**Impact**: Manual releases, no quality gates

#### Tasks:
- [ ] **GitHub Actions Workflow**
  - Multi-platform build matrix (Linux, macOS, Windows)
  - Automated testing with proper test isolation
  - Security scanning (gosec, vulnerability checks)
- [ ] **Release Automation**
  - Semantic versioning
  - GitHub releases with binaries
  - Changelog generation
- [ ] **Quality Gates**
  - Test coverage reporting
  - Linting (golangci-lint)
  - Dependency vulnerability scanning

#### Files to Create:
```
.github/
├── workflows/
│   ├── test.yml
│   ├── build.yml
│   └── release.yml
├── dependabot.yml
└── SECURITY.md
```

## Phase 2: Core Improvements (Week 3-4)

### 🟡 P1: Configuration Management
**Current Issue**: Hardcoded values, CLI-only configuration
**Impact**: Poor user experience, inflexible deployment

#### Tasks:
- [ ] **Configuration File Support**
  - YAML/JSON configuration files
  - XDG Base Directory compliance
  - Environment variable overrides
- [ ] **Validation & Defaults**
  - Configuration schema validation
  - Sensible defaults for all platforms
  - Configuration migration support

#### Architectural Changes:
```go
// internal/config/config.go
type Config struct {
    Server   ServerConfig   `yaml:"server"`
    Security SecurityConfig `yaml:"security"`
    Logging  LoggingConfig  `yaml:"logging"`
    Tunnels  []TunnelConfig `yaml:"tunnels"`
}

// Support multiple config sources with precedence:
// 1. CLI flags (highest)
// 2. Environment variables
// 3. Config file
// 4. Defaults (lowest)
```

### 🟡 P1: Enhanced Logging & OpenTelemetry Observability
**Current Issue**: Basic logging, no telemetry, poor error reporting
**Impact**: Difficult debugging, no production insights, poor user experience

#### Tasks:
- [ ] **Structured Logging with OpenTelemetry**
  - Replace standard log with slog + OpenTelemetry integration
  - JSON output for machine parsing
  - Configurable log levels (trace, debug, info, warn, error)
  - Correlation IDs for request tracing
  - Log sampling for high-volume scenarios
- [ ] **OpenTelemetry Metrics & Tracing**
  - OTEL metrics for tunnel operations (count, duration, errors)
  - Distributed tracing for tunnel lifecycle
  - Custom meters for performance monitoring
  - Resource detection and service metadata
- [ ] **Telemetry Exporters**
  - OTLP exporter for OpenTelemetry Collector
  - Prometheus metrics endpoint (/metrics)
  - Jaeger tracing support
  - Console exporter for development
- [ ] **Health & Diagnostics**
  - Health check endpoint (/health, /ready)
  - Pprof endpoints for profiling (/debug/pprof/*)
  - Configuration dump endpoint (/debug/config)
- [ ] **Better Error Handling**
  - Contextual error messages with tracing
  - User-friendly error formatting
  - Error codes for programmatic handling
  - Error attribution and categorization

#### Architectural Changes:
```go
// internal/observability/telemetry.go
type TelemetryManager struct {
    logger     *slog.Logger
    tracer     trace.Tracer
    meter      metric.Meter
    provider   *sdktrace.TracerProvider
}

func NewTelemetryManager(config *Config) (*TelemetryManager, error)
func (tm *TelemetryManager) StartSpan(ctx context.Context, name string) (context.Context, trace.Span)
func (tm *TelemetryManager) RecordMetric(name string, value float64, attrs ...attribute.KeyValue)
func (tm *TelemetryManager) Shutdown(ctx context.Context) error

// internal/observability/logger.go
type Logger interface {
    WithContext(ctx context.Context) Logger
    WithFields(fields ...slog.Attr) Logger
    Debug(msg string, args ...any)
    Info(msg string, args ...any)
    Warn(msg string, args ...any)
    Error(msg string, args ...any)
}

// internal/observability/metrics.go
type Metrics struct {
    tunnelCount     metric.Int64Counter
    tunnelDuration  metric.Float64Histogram
    errorCount      metric.Int64Counter
    activeConnections metric.Int64UpDownCounter
}

func (m *Metrics) IncrementTunnels(ctx context.Context, attrs ...attribute.KeyValue)
func (m *Metrics) RecordTunnelDuration(ctx context.Context, duration time.Duration, attrs ...attribute.KeyValue)
func (m *Metrics) RecordError(ctx context.Context, errorType string, attrs ...attribute.KeyValue)

// internal/observability/tracing.go
const (
    TraceName = "gotunnel"
    SpanTunnelStart = "tunnel.start"
    SpanTunnelStop = "tunnel.stop"
    SpanCertGenerate = "cert.generate"
    SpanDNSRegister = "dns.register"
)
```

#### Dependencies to Add:
```go
// go.mod additions
require (
    go.opentelemetry.io/otel v1.24.0
    go.opentelemetry.io/otel/trace v1.24.0
    go.opentelemetry.io/otel/metric v1.24.0
    go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.24.0
    go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp v1.24.0
    go.opentelemetry.io/otel/exporters/prometheus v0.46.0
    go.opentelemetry.io/otel/exporters/jaeger v1.24.0
    go.opentelemetry.io/otel/sdk v1.24.0
    go.opentelemetry.io/otel/sdk/metric v1.24.0
    go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.49.0
)
```

## Phase 3: Distribution & UX (Week 5-6)

### 🟡 P1: Installation & Distribution
**Current Issue**: Manual binary distribution only
**Impact**: Poor adoption, difficult installation

#### Tasks:
- [ ] **Package Managers**
  - Homebrew formula for macOS
  - Scoop manifest for Windows
  - APT/YUM packages for Linux
- [ ] **Container Support**
  - Docker image with minimal base
  - Docker Compose examples
  - Kubernetes manifests
- [ ] **Installation Scripts**
  - One-line install script
  - Dependency auto-installation (mkcert)
  - Uninstall scripts

#### Files to Create:
```
packaging/
├── homebrew/
│   └── gotunnel.rb
├── scoop/
│   └── gotunnel.json
├── docker/
│   ├── Dockerfile
│   └── docker-compose.yml
└── scripts/
    ├── install.sh
    └── uninstall.sh
```

### 🟡 P1: Cross-Platform Polish
**Current Issue**: Unix-centric implementation
**Impact**: Poor Windows experience

#### Tasks:
- [ ] **Windows-Specific Improvements**
  - Windows service support
  - Better PowerShell integration
  - Windows-specific certificate handling
- [ ] **macOS Enhancements**
  - Keychain integration
  - LaunchAgent support
  - macOS-specific networking
- [ ] **Platform Detection & Adaptation**
  - Runtime platform detection
  - Platform-specific default configurations
  - Graceful feature degradation

## Phase 4: Advanced Features (Week 7-8)

### 🟢 P2: Performance & Scalability
**Current Issue**: Basic implementation, no optimization
**Impact**: Resource usage, concurrent tunnel limits

#### Tasks:
- [ ] **Connection Management**
  - Connection pooling
  - Keep-alive mechanisms
  - Resource cleanup improvements
- [ ] **Performance Optimization**
  - Memory usage profiling
  - CPU usage optimization
  - Network buffer tuning
- [ ] **Concurrent Tunnel Management**
  - Improved tunnel lifecycle management
  - Better resource isolation
  - Tunnel health monitoring

### 🟢 P2: Advanced Configuration
**Current Issue**: Basic configuration options
**Impact**: Limited deployment flexibility

#### Tasks:
- [ ] **Advanced Tunnel Options**
  - Custom headers and routing
  - Load balancing between multiple backends
  - SSL/TLS configuration options
- [ ] **Profile Management**
  - Named configuration profiles
  - Profile switching
  - Template-based configurations
- [ ] **Integration Support**
  - Webhook notifications
  - External service discovery
  - API for programmatic control

## Implementation Strategy

### Dependencies & Order:
1. **Phase 1 must complete first** - Foundation for everything else
2. **Testing Infrastructure enables** → All subsequent development
3. **Security Model enables** → Distribution and production use
4. **CI/CD Pipeline enables** → Automated quality and releases

### Risk Mitigation:
- **Breaking Changes**: Maintain backward compatibility in CLI interface
- **Platform Support**: Implement platform-specific code behind interfaces
- **Dependencies**: Minimize external dependencies, prefer standard library
- **Performance**: Profile before and after major changes

### Success Metrics:
- [ ] All tests pass in CI
- [ ] 80%+ test coverage
- [ ] Automated releases working
- [ ] Package manager installation working
- [ ] Zero security vulnerabilities in scans
- [ ] Documentation complete and tested

## Architecture Principles

### 1. Interface-Driven Design
- Abstract system operations behind interfaces
- Enable testing with mocks
- Support multiple implementations per platform

### 2. Configuration Layers
- CLI → Environment → File → Defaults
- Validation at load time
- Clear precedence rules

### 3. Graceful Degradation
- Work without elevated privileges where possible
- Provide clear error messages when features unavailable
- Progressive enhancement based on available permissions

### 4. Platform Abstraction
- Common interfaces for platform-specific operations
- Runtime detection and adaptation
- Consistent user experience across platforms

### 5. OpenTelemetry-First Observability
- All operations instrumented with traces, metrics, and logs
- Context propagation throughout the application
- Auto-correlation between telemetry signals
- Vendor-neutral observability backends

## OpenTelemetry Integration Strategy

### Telemetry Configuration
```yaml
# gotunnel.yaml
observability:
  service_name: "gotunnel"
  service_version: "${VERSION}"
  environment: "production"
  
  logging:
    level: "info"
    format: "json"  # or "text" for development
    sampling: false
    
  tracing:
    enabled: true
    sampler: "parent_based_trace_id_ratio"
    sample_rate: 1.0
    exporters:
      - otlp:
          endpoint: "http://localhost:4318"
          headers:
            x-api-key: "${OTEL_API_KEY}"
      - jaeger:
          endpoint: "http://localhost:14268"
          
  metrics:
    enabled: true
    interval: "30s"
    exporters:
      - prometheus:
          endpoint: ":9090/metrics"
      - otlp:
          endpoint: "http://localhost:4318"
```

### Key Telemetry Signals

#### Traces
- `tunnel.lifecycle` - Complete tunnel creation/destruction
- `cert.operations` - Certificate generation and installation
- `dns.operations` - mDNS registration and discovery
- `hosts.operations` - Hosts file modifications
- `http.proxy` - Individual HTTP requests through tunnel

#### Metrics
- `tunnels.active` (gauge) - Current number of active tunnels
- `tunnels.total` (counter) - Total tunnels created
- `tunnel.duration` (histogram) - Tunnel lifetime duration
- `requests.total` (counter) - HTTP requests proxied
- `requests.duration` (histogram) - Request processing time
- `errors.total` (counter) - Errors by type and component
- `certificates.expiry` (gauge) - Days until certificate expiration

#### Logs (Structured with slog)
- Correlation with trace IDs
- Structured fields for filtering and analysis
- Different log levels for different environments
- Security-aware (no sensitive data logged)

### Deployment Integration
- **Docker**: Pre-configured with OpenTelemetry Collector sidecar
- **Kubernetes**: ServiceMonitor for Prometheus, OpenTelemetry Operator integration
- **Local Development**: Console exporters and local Jaeger/Prometheus
- **Production**: OTLP exporters to centralized observability platform