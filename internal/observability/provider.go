package observability

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/getsentry/sentry-go"
	sentryotel "github.com/getsentry/sentry-go/otel"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	ServiceName    = "gotunnel"
	ServiceVersion = "1.0.0" // TODO: Get from build-time ldflags
)

// Provider manages all observability concerns: logging, tracing, metrics, and error tracking
type Provider struct {
	logger       *slog.Logger
	tracer       trace.Tracer
	meter        metric.Meter
	tracerProvider *sdktrace.TracerProvider
	resource     *resource.Resource
	config       Config
}

// Config holds observability configuration
type Config struct {
	ServiceName      string
	ServiceVersion   string
	Environment      string
	SentryDSN        string
	TracesSampleRate float64
	LogLevel         slog.Level
	LogFormat        string // "json" or "text"
	Debug            bool   // Enable debug mode
}

// NewProvider creates a new observability provider with Sentry OpenTelemetry collection
func NewProvider(config Config) (*Provider, error) {
	// Set defaults
	if config.ServiceName == "" {
		config.ServiceName = ServiceName
	}
	if config.ServiceVersion == "" {
		config.ServiceVersion = ServiceVersion
	}
	if config.Environment == "" {
		config.Environment = "development"
	}
	if config.TracesSampleRate == 0 {
		config.TracesSampleRate = 1.0
	}
	if config.LogLevel == 0 {
		config.LogLevel = slog.LevelInfo
	}
	if config.LogFormat == "" {
		config.LogFormat = "text"
	}

	// Validate Sentry DSN if provided
	if config.SentryDSN != "" {
		// Sentry will validate the DSN internally
	}

	// Create resource with service information
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(config.ServiceName),
			semconv.ServiceVersionKey.String(config.ServiceVersion),
			semconv.DeploymentEnvironmentNameKey.String(config.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	provider := &Provider{
		resource: res,
		config:   config,
	}

	// Initialize Sentry
	if err := provider.initSentry(); err != nil {
		return nil, fmt.Errorf("failed to initialize Sentry: %w", err)
	}

	// Initialize OpenTelemetry tracing
	if err := provider.initTracing(); err != nil {
		return nil, fmt.Errorf("failed to initialize tracing: %w", err)
	}

	// Initialize structured logging
	provider.initLogging()

	// Initialize metrics (using global meter for now)
	provider.meter = otel.Meter(config.ServiceName)

	return provider, nil
}

// initSentry initializes Sentry with OpenTelemetry integration following Sentry docs
func (p *Provider) initSentry() error {
	if p.config.SentryDSN == "" {
		return nil // Sentry disabled
	}

	return sentry.Init(sentry.ClientOptions{
		Dsn:           p.config.SentryDSN,
		Environment:   p.config.Environment,
		Release:       p.config.ServiceVersion,
		Debug:         p.config.Debug,
		SampleRate:    1.0, // Error sampling rate
		TracesSampleRate: p.config.TracesSampleRate,
		EnableTracing:    true,
		AttachStacktrace: true,
		BeforeSend: func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
			// Add service context
			event.Tags["service"] = p.config.ServiceName
			event.Tags["version"] = p.config.ServiceVersion
			return event
		},
		// Sentry's OpenTelemetry integration is handled differently
		// The sentryotel package provides the integration automatically
	})
}

// initTracing initializes OpenTelemetry tracing with Sentry integration
func (p *Provider) initTracing() error {
	// Configure sampling
	sampler := sdktrace.AlwaysSample()
	if p.config.TracesSampleRate < 1.0 {
		sampler = sdktrace.TraceIDRatioBased(p.config.TracesSampleRate)
	}

	// Create tracer provider with Sentry's span processor
	p.tracerProvider = sdktrace.NewTracerProvider(
		sdktrace.WithResource(p.resource),
		sdktrace.WithSampler(sampler),
		// Add Sentry's span processor - this automatically sends spans to Sentry
		sdktrace.WithSpanProcessor(sentryotel.NewSentrySpanProcessor()),
	)

	// Set global tracer provider
	otel.SetTracerProvider(p.tracerProvider)

	// Set global text map propagator (for cross-service tracing)
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	// Create tracer
	p.tracer = p.tracerProvider.Tracer(p.config.ServiceName)

	return nil
}

// initLogging initializes structured logging with OpenTelemetry correlation
func (p *Provider) initLogging() {
	var handler slog.Handler

	opts := &slog.HandlerOptions{
		Level: p.config.LogLevel,
		AddSource: p.config.Environment == "development",
	}

	switch p.config.LogFormat {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, opts)
	default:
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	// Wrap with trace correlation
	p.logger = slog.New(&traceHandler{
		handler: handler,
	})
}

// traceHandler wraps slog.Handler to add trace correlation
type traceHandler struct {
	handler slog.Handler
}

func (h *traceHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

func (h *traceHandler) Handle(ctx context.Context, record slog.Record) error {
	// Add trace ID if available
	if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
		record.AddAttrs(
			slog.String("trace_id", span.SpanContext().TraceID().String()),
			slog.String("span_id", span.SpanContext().SpanID().String()),
		)
	}

	return h.handler.Handle(ctx, record)
}

func (h *traceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &traceHandler{handler: h.handler.WithAttrs(attrs)}
}

func (h *traceHandler) WithGroup(name string) slog.Handler {
	return &traceHandler{handler: h.handler.WithGroup(name)}
}

// Logger returns the structured logger
func (p *Provider) Logger() *slog.Logger {
	return p.logger
}

// Tracer returns the OpenTelemetry tracer
func (p *Provider) Tracer() trace.Tracer {
	return p.tracer
}

// Meter returns the OpenTelemetry meter
func (p *Provider) Meter() metric.Meter {
	return p.meter
}

// StartSpan starts a new trace span
func (p *Provider) StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return p.tracer.Start(ctx, name, opts...)
}

// LogWithSpan logs a message with trace context
func (p *Provider) LogWithSpan(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) {
	p.logger.LogAttrs(ctx, level, msg, attrs...)
}

// CaptureError sends an error to Sentry with trace correlation
func (p *Provider) CaptureError(ctx context.Context, err error, tags map[string]string) {
	if p.config.SentryDSN == "" {
		return // Sentry not configured
	}

	sentry.WithScope(func(scope *sentry.Scope) {
		// Add trace context
		if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
			scope.SetTag("trace_id", span.SpanContext().TraceID().String())
			scope.SetTag("span_id", span.SpanContext().SpanID().String())
		}

		// Add custom tags
		for k, v := range tags {
			scope.SetTag(k, v)
		}

		sentry.CaptureException(err)
	})
}

// Shutdown gracefully shuts down the observability provider
func (p *Provider) Shutdown(ctx context.Context) error {
	var errs []error

	// Shutdown Sentry only if it was configured
	if p.config.SentryDSN != "" {
		if !sentry.Flush(2 * time.Second) {
			errs = append(errs, fmt.Errorf("sentry flush timeout"))
		}
	}

	// Shutdown OpenTelemetry
	if p.tracerProvider != nil {
		if err := p.tracerProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("tracer provider shutdown: %w", err))
		}
	}

	// Return combined errors
	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}

	return nil
}

// Helper function to create a default configuration
func DefaultConfig() Config {
	return Config{
		ServiceName:      ServiceName,
		ServiceVersion:   ServiceVersion,
		Environment:      "development",
		TracesSampleRate: 1.0,
		LogLevel:         slog.LevelInfo,
		LogFormat:        "text",
		Debug:            false,
	}
}

// Helper functions for common operations

// WithSpanAttributes adds attributes to the current span
func WithSpanAttributes(span trace.Span, attrs ...attribute.KeyValue) {
	span.SetAttributes(attrs...)
}

// RecordError records an error in the current span and sends to Sentry
func (p *Provider) RecordError(ctx context.Context, span trace.Span, err error, description string) {
	span.RecordError(err)
	span.SetStatus(codes.Error, description)
	
	p.CaptureError(ctx, err, map[string]string{
		"span_name": span.SpanContext().TraceID().String(),
		"error_description": description,
	})
}