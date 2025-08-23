package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"go.opentelemetry.io/otel/trace"
)

// LogLevel represents the logging level
type LogLevel string

const (
	LevelDebug LogLevel = "debug"
	LevelInfo  LogLevel = "info"
	LevelWarn  LogLevel = "warn"
	LevelError LogLevel = "error"
)

// LogFormat represents the logging format
type LogFormat string

const (
	FormatJSON LogFormat = "json"
	FormatText LogFormat = "text"
)

// Config holds the logging configuration
type Config struct {
	Level      LogLevel  `yaml:"level" json:"level"`
	Format     LogFormat `yaml:"format" json:"format"`
	Output     string    `yaml:"output" json:"output"` // "stdout", "stderr", or file path
	AddSource  bool      `yaml:"add_source" json:"add_source"`
	TimeFormat string    `yaml:"time_format" json:"time_format"`
}

// DefaultConfig returns a default logging configuration
func DefaultConfig() *Config {
	return &Config{
		Level:      LevelInfo,
		Format:     FormatText,
		Output:     "stdout",
		AddSource:  false,
		TimeFormat: time.RFC3339,
	}
}

// Logger wraps slog.Logger with additional functionality
type Logger struct {
	*slog.Logger
	config *Config
}

// New creates a new logger with the given configuration
func New(config *Config) (*Logger, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// Set log level
	var level slog.Level
	switch config.Level {
	case LevelDebug:
		level = slog.LevelDebug
	case LevelInfo:
		level = slog.LevelInfo
	case LevelWarn:
		level = slog.LevelWarn
	case LevelError:
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	// Set output destination
	var output io.Writer
	switch config.Output {
	case "stdout", "":
		output = os.Stdout
	case "stderr":
		output = os.Stderr
	default:
		// File output
		if err := os.MkdirAll(filepath.Dir(config.Output), 0755); err != nil {
			return nil, fmt.Errorf("failed to create log directory: %w", err)
		}
		file, err := os.OpenFile(config.Output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}
		output = file
	}

	// Create handler options
	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: config.AddSource,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Customize time format
			if a.Key == slog.TimeKey && config.TimeFormat != "" {
				if t, ok := a.Value.Any().(time.Time); ok {
					return slog.String(a.Key, t.Format(config.TimeFormat))
				}
			}
			
			// Shorten source file paths
			if a.Key == slog.SourceKey {
				if source, ok := a.Value.Any().(*slog.Source); ok {
					// Get relative path from project root
					if idx := strings.LastIndex(source.File, "/gotunnel/"); idx != -1 {
						source.File = source.File[idx+10:] // Remove "/gotunnel/" prefix
					}
				}
			}
			
			return a
		},
	}

	// Create handler based on format
	var handler slog.Handler
	switch config.Format {
	case FormatJSON:
		handler = slog.NewJSONHandler(output, opts)
	case FormatText, "":
		handler = slog.NewTextHandler(output, opts)
	default:
		handler = slog.NewTextHandler(output, opts)
	}

	logger := slog.New(handler)

	return &Logger{
		Logger: logger,
		config: config,
	}, nil
}

// WithContext creates a new logger with context values
func (l *Logger) WithContext(ctx context.Context) *Logger {
	logger := l.Logger

	// Add trace information if available
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		logger = logger.With(
			slog.String("trace_id", span.SpanContext().TraceID().String()),
			slog.String("span_id", span.SpanContext().SpanID().String()),
		)
	}

	return &Logger{
		Logger: logger,
		config: l.config,
	}
}

// WithComponent creates a new logger with a component name
func (l *Logger) WithComponent(component string) *Logger {
	return &Logger{
		Logger: l.Logger.With(slog.String("component", component)),
		config: l.config,
	}
}

// WithFields creates a new logger with additional fields
func (l *Logger) WithFields(fields map[string]any) *Logger {
	args := make([]any, 0, len(fields)*2)
	for k, v := range fields {
		args = append(args, k, v)
	}
	
	return &Logger{
		Logger: l.Logger.With(args...),
		config: l.config,
	}
}

// WithError creates a new logger with an error field
func (l *Logger) WithError(err error) *Logger {
	if err == nil {
		return l
	}
	
	return &Logger{
		Logger: l.Logger.With(slog.String("error", err.Error())),
		config: l.config,
	}
}

// Tunnel-specific logging methods

// TunnelStarted logs when a tunnel is started
func (l *Logger) TunnelStarted(domain string, port int, target string) {
	l.Info("Tunnel started",
		slog.String("event", "tunnel_started"),
		slog.String("domain", domain),
		slog.Int("port", port),
		slog.String("target", target),
	)
}

// TunnelStopped logs when a tunnel is stopped
func (l *Logger) TunnelStopped(domain string, duration time.Duration) {
	l.Info("Tunnel stopped",
		slog.String("event", "tunnel_stopped"),
		slog.String("domain", domain),
		slog.Duration("duration", duration),
	)
}

// TunnelError logs tunnel-related errors
func (l *Logger) TunnelError(domain string, err error, details map[string]any) {
	args := []any{
		slog.String("event", "tunnel_error"),
		slog.String("domain", domain),
		slog.String("error", err.Error()),
	}
	
	for k, v := range details {
		args = append(args, k, v)
	}
	
	l.Error("Tunnel error occurred", args...)
}

// ProxyRequest logs HTTP proxy requests
func (l *Logger) ProxyRequest(method, host, path string, statusCode int, duration time.Duration, userAgent string) {
	l.Debug("Proxy request",
		slog.String("event", "proxy_request"),
		slog.String("method", method),
		slog.String("host", host),
		slog.String("path", path),
		slog.Int("status_code", statusCode),
		slog.Duration("duration", duration),
		slog.String("user_agent", userAgent),
	)
}

// CertificateGenerated logs certificate generation
func (l *Logger) CertificateGenerated(domain string, expiresAt time.Time) {
	l.Info("Certificate generated",
		slog.String("event", "certificate_generated"),
		slog.String("domain", domain),
		slog.Time("expires_at", expiresAt),
	)
}

// CertificateError logs certificate-related errors
func (l *Logger) CertificateError(domain string, err error) {
	l.Error("Certificate error",
		slog.String("event", "certificate_error"),
		slog.String("domain", domain),
		slog.String("error", err.Error()),
	)
}

// DNSRegistered logs when a domain is registered with DNS
func (l *Logger) DNSRegistered(domain string, ip string) {
	l.Info("DNS domain registered",
		slog.String("event", "dns_registered"),
		slog.String("domain", domain),
		slog.String("ip", ip),
	)
}

// DNSUnregistered logs when a domain is unregistered from DNS
func (l *Logger) DNSUnregistered(domain string) {
	l.Info("DNS domain unregistered",
		slog.String("event", "dns_unregistered"),
		slog.String("domain", domain),
	)
}

// ServiceStarted logs when a service component starts
func (l *Logger) ServiceStarted(service string, details map[string]any) {
	args := []any{
		slog.String("event", "service_started"),
		slog.String("service", service),
	}
	
	for k, v := range details {
		args = append(args, k, v)
	}
	
	l.Info("Service started", args...)
}

// ServiceStopped logs when a service component stops
func (l *Logger) ServiceStopped(service string, duration time.Duration) {
	l.Info("Service stopped",
		slog.String("event", "service_stopped"),
		slog.String("service", service),
		slog.Duration("duration", duration),
	)
}

// Audit logs security-relevant events
func (l *Logger) Audit(action string, user string, resource string, success bool, details map[string]any) {
	args := []any{
		slog.String("event", "audit"),
		slog.String("action", action),
		slog.String("user", user),
		slog.String("resource", resource),
		slog.Bool("success", success),
	}
	
	for k, v := range details {
		args = append(args, k, v)
	}
	
	l.Info("Audit event", args...)
}

// Performance logs performance metrics
func (l *Logger) Performance(operation string, duration time.Duration, details map[string]any) {
	args := []any{
		slog.String("event", "performance"),
		slog.String("operation", operation),
		slog.Duration("duration", duration),
	}
	
	for k, v := range details {
		args = append(args, k, v)
	}
	
	l.Debug("Performance metric", args...)
}

// Helper functions

// GetCaller returns information about the calling function
func GetCaller(skip int) (file string, line int, funcName string) {
	pc, file, line, ok := runtime.Caller(skip + 1)
	if !ok {
		return "unknown", 0, "unknown"
	}
	
	funcName = runtime.FuncForPC(pc).Name()
	if lastSlash := strings.LastIndex(funcName, "/"); lastSlash >= 0 {
		funcName = funcName[lastSlash+1:]
	}
	
	if lastSlash := strings.LastIndex(file, "/"); lastSlash >= 0 {
		file = file[lastSlash+1:]
	}
	
	return file, line, funcName
}

// Fatal logs a fatal error and exits the program
func (l *Logger) Fatal(msg string, args ...any) {
	l.Error(msg, args...)
	os.Exit(1)
}

// Panic logs an error and panics
func (l *Logger) Panic(msg string, args ...any) {
	l.Error(msg, args...)
	panic(msg)
}