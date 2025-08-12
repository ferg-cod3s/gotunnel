package observability

import (
	"context"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Metrics holds all the custom metrics for gotunnel
type Metrics struct {
	provider *Provider

	// Tunnel metrics
	tunnelCount     metric.Int64Counter
	tunnelDuration  metric.Float64Histogram
	activeTunnels   metric.Int64UpDownCounter

	// HTTP proxy metrics
	requestCount    metric.Int64Counter
	requestDuration metric.Float64Histogram
	requestSize     metric.Int64Histogram
	responseSize    metric.Int64Histogram

	// Certificate metrics
	certExpiry      metric.Float64Gauge
	certGeneration  metric.Int64Counter

	// Error metrics
	errorCount      metric.Int64Counter

	// System metrics
	memoryUsage     metric.Int64UpDownCounter
}

// NewMetrics creates a new metrics instance
func NewMetrics(provider *Provider) (*Metrics, error) {
	meter := provider.Meter()

	// Initialize all metrics
	tunnelCount, err := meter.Int64Counter(
		"gotunnel.tunnels.created",
		metric.WithDescription("Total number of tunnels created"),
	)
	if err != nil {
		return nil, err
	}

	tunnelDuration, err := meter.Float64Histogram(
		"gotunnel.tunnel.duration",
		metric.WithDescription("Duration of tunnel lifecycle in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	activeTunnels, err := meter.Int64UpDownCounter(
		"gotunnel.tunnels.active",
		metric.WithDescription("Number of currently active tunnels"),
	)
	if err != nil {
		return nil, err
	}

	requestCount, err := meter.Int64Counter(
		"gotunnel.http.requests.total",
		metric.WithDescription("Total number of HTTP requests proxied"),
	)
	if err != nil {
		return nil, err
	}

	requestDuration, err := meter.Float64Histogram(
		"gotunnel.http.request.duration",
		metric.WithDescription("Duration of HTTP request processing"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	requestSize, err := meter.Int64Histogram(
		"gotunnel.http.request.size",
		metric.WithDescription("Size of HTTP request body in bytes"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return nil, err
	}

	responseSize, err := meter.Int64Histogram(
		"gotunnel.http.response.size",
		metric.WithDescription("Size of HTTP response body in bytes"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return nil, err
	}

	certExpiry, err := meter.Float64Gauge(
		"gotunnel.certificate.expiry.days",
		metric.WithDescription("Days until certificate expiry"),
		metric.WithUnit("d"),
	)
	if err != nil {
		return nil, err
	}

	certGeneration, err := meter.Int64Counter(
		"gotunnel.certificates.generated",
		metric.WithDescription("Total number of certificates generated"),
	)
	if err != nil {
		return nil, err
	}

	errorCount, err := meter.Int64Counter(
		"gotunnel.errors.total",
		metric.WithDescription("Total number of errors by type"),
	)
	if err != nil {
		return nil, err
	}

	memoryUsage, err := meter.Int64UpDownCounter(
		"gotunnel.memory.usage",
		metric.WithDescription("Current memory usage in bytes"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return nil, err
	}

	return &Metrics{
		provider:        provider,
		tunnelCount:     tunnelCount,
		tunnelDuration:  tunnelDuration,
		activeTunnels:   activeTunnels,
		requestCount:    requestCount,
		requestDuration: requestDuration,
		requestSize:     requestSize,
		responseSize:    responseSize,
		certExpiry:      certExpiry,
		certGeneration:  certGeneration,
		errorCount:      errorCount,
		memoryUsage:     memoryUsage,
	}, nil
}

// Tunnel Metrics

func (m *Metrics) TunnelCreated(ctx context.Context, domain string, port int, https bool) {
	attrs := []attribute.KeyValue{
		attribute.String("domain", domain),
		attribute.Int("port", port),
		attribute.Bool("https", https),
	}

	m.tunnelCount.Add(ctx, 1, metric.WithAttributes(attrs...))
	m.activeTunnels.Add(ctx, 1, metric.WithAttributes(attrs...))

	// Log the event
	m.provider.Logger().InfoContext(ctx, "Tunnel created",
		slog.String("domain", domain),
		slog.Int("port", port),
		slog.Bool("https", https),
	)
}

func (m *Metrics) TunnelDestroyed(ctx context.Context, domain string, duration time.Duration) {
	attrs := []attribute.KeyValue{
		attribute.String("domain", domain),
	}

	m.tunnelDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
	m.activeTunnels.Add(ctx, -1, metric.WithAttributes(attrs...))

	// Log the event
	m.provider.Logger().InfoContext(ctx, "Tunnel destroyed",
		slog.String("domain", domain),
		slog.Duration("duration", duration),
	)
}

// HTTP Proxy Metrics

func (m *Metrics) HTTPRequest(ctx context.Context, method, path string, statusCode int, requestSize, responseSize int64, duration time.Duration) {
	attrs := []attribute.KeyValue{
		attribute.String("method", method),
		attribute.String("path", path),
		attribute.Int("status_code", statusCode),
	}

	m.requestCount.Add(ctx, 1, metric.WithAttributes(attrs...))
	m.requestDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
	
	if requestSize > 0 {
		m.requestSize.Record(ctx, requestSize, metric.WithAttributes(attrs...))
	}
	if responseSize > 0 {
		m.responseSize.Record(ctx, responseSize, metric.WithAttributes(attrs...))
	}
}

// Certificate Metrics

func (m *Metrics) CertificateGenerated(ctx context.Context, domain string) {
	attrs := []attribute.KeyValue{
		attribute.String("domain", domain),
	}

	m.certGeneration.Add(ctx, 1, metric.WithAttributes(attrs...))

	// Log the event
	m.provider.Logger().InfoContext(ctx, "Certificate generated",
		slog.String("domain", domain),
	)
}

func (m *Metrics) CertificateExpiry(ctx context.Context, domain string, daysUntilExpiry float64) {
	attrs := []attribute.KeyValue{
		attribute.String("domain", domain),
	}

	m.certExpiry.Record(ctx, daysUntilExpiry, metric.WithAttributes(attrs...))
}

// Error Metrics

func (m *Metrics) RecordError(ctx context.Context, errorType, operation string, err error) {
	attrs := []attribute.KeyValue{
		attribute.String("error_type", errorType),
		attribute.String("operation", operation),
	}

	m.errorCount.Add(ctx, 1, metric.WithAttributes(attrs...))

	// Send to Sentry
	m.provider.CaptureError(ctx, err, map[string]string{
		"error_type": errorType,
		"operation":  operation,
	})

	// Log the error
	m.provider.Logger().ErrorContext(ctx, "Operation failed",
		slog.String("error_type", errorType),
		slog.String("operation", operation),
		slog.Any("error", err),
	)
}

// System Metrics

func (m *Metrics) UpdateMemoryUsage(ctx context.Context, bytes int64) {
	// UpDownCounter uses Add instead of Set
	// We'd need to track previous value to calculate the delta
	// For now, just record the current value as a gauge-like measurement
	m.memoryUsage.Add(ctx, bytes)
}

// Helper for operation timing
type OperationTimer struct {
	metrics   *Metrics
	ctx       context.Context
	operation string
	startTime time.Time
}

func (m *Metrics) StartOperation(ctx context.Context, operation string) *OperationTimer {
	return &OperationTimer{
		metrics:   m,
		ctx:       ctx,
		operation: operation,
		startTime: time.Now(),
	}
}

func (timer *OperationTimer) End(err error) {
	duration := time.Since(timer.startTime)

	if err != nil {
		timer.metrics.RecordError(timer.ctx, "operation_error", timer.operation, err)
	}

	// Could add operation-specific duration metrics here
	timer.metrics.provider.Logger().DebugContext(timer.ctx, "Operation completed",
		slog.String("operation", timer.operation),
		slog.Duration("duration", duration),
		slog.Any("error", err),
	)
}