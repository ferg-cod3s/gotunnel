# Deployment Strategy: gotunnel

## Overview

This document outlines the comprehensive deployment strategy for gotunnel, covering environments, distribution channels, observability integration, and operational procedures.

## Environment Strategy

### 1. Development Environment
**Purpose**: Local development and testing
**Infrastructure**: Developer machines
**Observability**: Console logging + local Jaeger/Sentry

```yaml
# config/development.yaml
environment: development
observability:
  logging:
    level: debug
    format: text
  tracing:
    enabled: true
    exporters:
      - console
      - jaeger:
          endpoint: http://localhost:14268
  sentry:
    enabled: true
    dsn: ${SENTRY_DSN_DEV}
    environment: development
    sample_rate: 1.0
```

### 2. Staging Environment
**Purpose**: Integration testing and validation
**Infrastructure**: CI/CD runners, test infrastructure
**Observability**: Full telemetry to staging Sentry project

```yaml
# config/staging.yaml
environment: staging
observability:
  logging:
    level: info
    format: json
  tracing:
    enabled: true
    sample_rate: 0.1
    exporters:
      - otlp:
          endpoint: https://api.sentry.io/api/0/projects/gotunnel/gotunnel-staging/envelope/
  sentry:
    enabled: true
    dsn: ${SENTRY_DSN_STAGING}
    environment: staging
    sample_rate: 0.2
```

### 3. Production Environment
**Purpose**: End-user distribution
**Infrastructure**: User machines, containers, CI/CD
**Observability**: Full telemetry with sampling

```yaml
# config/production.yaml
environment: production
observability:
  logging:
    level: info
    format: json
  tracing:
    enabled: true
    sample_rate: 0.01
    exporters:
      - sentry
  sentry:
    enabled: true
    dsn: ${SENTRY_DSN_PROD}
    environment: production
    sample_rate: 0.05
    performance_sample_rate: 0.1
```

## Distribution Channels

### 1. GitHub Releases (Primary)
**Target**: All platforms, direct downloads
**Artifacts**: Platform-specific binaries with checksums
**Automation**: GitHub Actions workflow

```
gotunnel-v1.0.0-darwin-amd64
gotunnel-v1.0.0-darwin-arm64
gotunnel-v1.0.0-linux-amd64
gotunnel-v1.0.0-linux-arm64
gotunnel-v1.0.0-windows-amd64.exe
checksums.txt
checksums.txt.sig
```

### 2. Package Managers
**Homebrew (macOS)**:
```ruby
# Formula/gotunnel.rb
class Gotunnel < Formula
  desc "Secure local tunnels for development"
  homepage "https://github.com/johncferguson/gotunnel"
  url "https://github.com/johncferguson/gotunnel/archive/v1.0.0.tar.gz"
  sha256 "..."
  
  depends_on "go" => :build
  depends_on "mkcert"
  
  def install
    system "go", "build", "-o", bin/"gotunnel", "./cmd/gotunnel"
  end
end
```

**Scoop (Windows)**:
```json
{
    "version": "1.0.0",
    "description": "Secure local tunnels for development",
    "homepage": "https://github.com/johncferguson/gotunnel",
    "license": "MIT",
    "url": "https://github.com/johncferguson/gotunnel/releases/download/v1.0.0/gotunnel-v1.0.0-windows-amd64.exe",
    "hash": "...",
    "bin": "gotunnel.exe"
}
```

### 3. Container Images
**Docker Hub**: Multi-architecture images
**GitHub Container Registry**: Enterprise distribution

```dockerfile
# Dockerfile
FROM alpine:3.18
RUN apk add --no-cache ca-certificates
COPY gotunnel /usr/local/bin/
EXPOSE 80 443
ENTRYPOINT ["/usr/local/bin/gotunnel"]
```

### 4. Installation Scripts
**One-line installer**:
```bash
curl -fsSL https://get.gotunnel.dev | sh
```

## OpenTelemetry + Sentry Integration Architecture

### 1. Telemetry Stack
```go
// internal/observability/provider.go
type ObservabilityProvider struct {
    logger       *slog.Logger
    tracer       trace.Tracer
    meter        metric.Meter
    sentryHub    *sentry.Hub
    resource     *resource.Resource
}

func NewProvider(config *Config) (*ObservabilityProvider, error) {
    // Initialize resource detection
    res := resource.NewWithAttributes(
        semconv.SchemaURL,
        semconv.ServiceName("gotunnel"),
        semconv.ServiceVersion(version),
        semconv.DeploymentEnvironment(config.Environment),
    )
    
    // Initialize Sentry
    err := sentry.Init(sentry.Options{
        Dsn:              config.Sentry.DSN,
        Environment:      config.Environment,
        Release:          version,
        SampleRate:       config.Sentry.SampleRate,
        TracesSampleRate: config.Sentry.TracesSampleRate,
        Integrations: []sentry.Integration{
            sentry.NewOtelTracingIntegration(),
        },
    })
    
    // Initialize OpenTelemetry
    tp := newTraceProvider(config, res)
    mp := newMeterProvider(config, res)
    
    return &ObservabilityProvider{
        logger:    newLogger(config),
        tracer:    tp.Tracer("gotunnel"),
        meter:     mp.Meter("gotunnel"),
        sentryHub: sentry.CurrentHub(),
        resource:  res,
    }
}
```

### 2. Sentry-Specific Integration
```go
// internal/observability/sentry.go
type SentryIntegration struct {
    hub *sentry.Hub
}

func (s *SentryIntegration) CaptureError(ctx context.Context, err error, tags map[string]string) {
    s.hub.WithScope(func(scope *sentry.Scope) {
        // Add OpenTelemetry trace context
        if span := trace.SpanFromContext(ctx); span != nil {
            traceID := span.SpanContext().TraceID().String()
            scope.SetTag("trace_id", traceID)
        }
        
        // Add custom tags
        for k, v := range tags {
            scope.SetTag(k, v)
        }
        
        s.hub.CaptureException(err)
    })
}

func (s *SentryIntegration) CaptureMessage(ctx context.Context, message string, level sentry.Level) {
    s.hub.WithScope(func(scope *sentry.Scope) {
        if span := trace.SpanFromContext(ctx); span != nil {
            traceID := span.SpanContext().TraceID().String()
            scope.SetTag("trace_id", traceID)
        }
        
        s.hub.CaptureMessage(message)
    })
}
```

### 3. Key Telemetry Signals

#### Traces (Sentry Performance Monitoring)
```go
// Tunnel lifecycle tracing
func (m *Manager) StartTunnel(ctx context.Context, config TunnelConfig) error {
    ctx, span := m.tracer.Start(ctx, "tunnel.start",
        trace.WithAttributes(
            attribute.String("tunnel.domain", config.Domain),
            attribute.Int("tunnel.port", config.Port),
            attribute.Bool("tunnel.https", config.HTTPS),
        ),
    )
    defer span.End()
    
    // Sentry transaction
    transaction := sentry.StartTransaction(ctx, "tunnel.start")
    defer transaction.Finish()
    
    // Implementation...
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
        sentry.CaptureException(err)
        return err
    }
    
    return nil
}
```

#### Metrics (Custom Sentry Metrics)
```go
// Performance and usage metrics
type Metrics struct {
    tunnelCount       metric.Int64Counter
    tunnelDuration    metric.Float64Histogram
    requestCount      metric.Int64Counter
    errorRate         metric.Float64Gauge
}

func (m *Metrics) RecordTunnelCreated(ctx context.Context, domain string) {
    m.tunnelCount.Add(ctx, 1,
        metric.WithAttributes(
            attribute.String("domain", domain),
        ),
    )
    
    // Also send to Sentry as custom metric
    sentry.WithScope(func(scope *sentry.Scope) {
        scope.SetTag("domain", domain)
        sentry.CaptureMessage("Tunnel created")
    })
}
```

#### Error Tracking
```go
// Error categorization and reporting
type ErrorTracker struct {
    sentry *SentryIntegration
}

func (et *ErrorTracker) ReportError(ctx context.Context, err error, category string) {
    tags := map[string]string{
        "error.category": category,
        "error.type":     reflect.TypeOf(err).String(),
    }
    
    // Extract additional context
    if span := trace.SpanFromContext(ctx); span != nil {
        tags["operation"] = span.Name()
    }
    
    et.sentry.CaptureError(ctx, err, tags)
}
```

## CI/CD Pipeline Integration

### 1. Build Pipeline
```yaml
# .github/workflows/build.yml
name: Build and Release
on:
  push:
    tags: ['v*']

jobs:
  build:
    strategy:
      matrix:
        os: [ubuntu-latest, macos-latest, windows-latest]
        arch: [amd64, arm64]
    
    steps:
      - uses: actions/checkout@v4
      
      - name: Setup Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'
          
      - name: Setup Sentry CLI
        run: |
          curl -sL https://sentry.io/get-cli/ | bash
          
      - name: Build Binary
        run: |
          GOOS=${{ matrix.os == 'ubuntu-latest' && 'linux' || matrix.os == 'macos-latest' && 'darwin' || 'windows' }} \
          GOARCH=${{ matrix.arch }} \
          go build -ldflags="-X main.version=${GITHUB_REF#refs/tags/} -X main.sentryDsn=${{ secrets.SENTRY_DSN }}" \
          -o gotunnel-${GITHUB_REF#refs/tags/}-${{ matrix.os }}-${{ matrix.arch }} \
          ./cmd/gotunnel
          
      - name: Create Sentry Release
        run: |
          sentry-cli releases new ${GITHUB_REF#refs/tags/}
          sentry-cli releases set-commits ${GITHUB_REF#refs/tags/} --auto
          sentry-cli releases files ${GITHUB_REF#refs/tags/} upload-sourcemaps .
          sentry-cli releases finalize ${GITHUB_REF#refs/tags/}
        env:
          SENTRY_AUTH_TOKEN: ${{ secrets.SENTRY_AUTH_TOKEN }}
          SENTRY_ORG: gotunnel
          SENTRY_PROJECT: gotunnel
```

### 2. Testing with Observability
```yaml
# .github/workflows/test.yml
  test:
    runs-on: ubuntu-latest
    steps:
      - name: Run Tests with Coverage
        run: |
          go test -v -race -coverprofile=coverage.out ./...
          
      - name: Upload Coverage to Sentry
        run: |
          sentry-cli upload-coverage coverage.out
        env:
          SENTRY_AUTH_TOKEN: ${{ secrets.SENTRY_AUTH_TOKEN }}
```

## Operational Procedures

### 1. Release Management
1. **Version Tagging**: Semantic versioning (v1.0.0)
2. **Changelog**: Automated generation from commit messages
3. **Sentry Release**: Automatic release creation and commit tracking
4. **Rollback**: Quick rollback via GitHub release deletion

### 2. Monitoring & Alerting
**Sentry Alerts**:
- Error rate > 1% for 5 minutes
- New error types introduced
- Performance regression > 100ms p95
- Release health < 95%

**Custom Dashboards**:
- Tunnel creation success rate
- Average tunnel duration
- Geographic usage distribution
- Platform breakdown

### 3. Incident Response
1. **Detection**: Sentry alerts + monitoring
2. **Triage**: Error categorization and impact assessment
3. **Resolution**: Fix deployment via GitHub Actions
4. **Post-mortem**: Sentry issue analysis and improvement planning

## Security Considerations

### 1. Telemetry Privacy
- **No PII**: Never log domains, IPs, or user information
- **Sanitized Errors**: Remove sensitive data from error messages
- **Opt-out**: Allow users to disable telemetry

### 2. Build Security
- **Signed Binaries**: GPG signature verification
- **SLSA Compliance**: Supply chain security attestation
- **Vulnerability Scanning**: Automated security scanning in CI

### 3. Distribution Security
- **Checksums**: SHA256 verification for all downloads
- **HTTPS Only**: Secure distribution channels
- **Package Verification**: Signature verification for package managers

## Cost Optimization

### 1. Sentry Usage
- **Sampling**: Production sampling rates to control costs
- **Data Retention**: 30-day retention for most data
- **Alert Throttling**: Prevent alert spam

### 2. Infrastructure
- **Serverless CI/CD**: GitHub Actions pay-per-use
- **CDN Distribution**: GitHub releases for binary hosting
- **Container Registry**: GitHub Container Registry for enterprise

This deployment strategy ensures gotunnel is production-ready with comprehensive observability, reliable distribution, and operational excellence.