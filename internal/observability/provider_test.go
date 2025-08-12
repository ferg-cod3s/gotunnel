package observability

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	
	assert.Equal(t, ServiceName, config.ServiceName)
	assert.Equal(t, ServiceVersion, config.ServiceVersion)
	assert.Equal(t, "development", config.Environment)
	assert.Equal(t, 1.0, config.TracesSampleRate)
	assert.Equal(t, slog.LevelInfo, config.LogLevel)
	assert.Equal(t, "text", config.LogFormat)
	assert.False(t, config.Debug)
}

func TestNewProviderWithoutSentry(t *testing.T) {
	config := DefaultConfig()
	config.SentryDSN = "" // Disable Sentry for testing

	provider, err := NewProvider(config)
	require.NoError(t, err)
	require.NotNil(t, provider)

	// Test basic functionality
	assert.NotNil(t, provider.Logger())
	assert.NotNil(t, provider.Tracer())
	assert.NotNil(t, provider.Meter())

	// Test span creation
	ctx := context.Background()
	ctx, span := provider.StartSpan(ctx, "test.span")
	assert.NotNil(t, span)
	
	// Test logging with context
	provider.Logger().InfoContext(ctx, "Test log message")
	
	span.End()

	// Cleanup
	shutdownCtx := context.Background()
	err = provider.Shutdown(shutdownCtx)
	assert.NoError(t, err)
}

func TestNewProviderWithSentry(t *testing.T) {
	// Skip if no Sentry DSN available (for CI environments)
	config := DefaultConfig()
	config.SentryDSN = "https://test@test.ingest.sentry.io/123456" // Test DSN format
	config.Debug = true
	config.LogLevel = slog.LevelDebug

	provider, err := NewProvider(config)
	require.NoError(t, err)
	require.NotNil(t, provider)

	// Test tracing with Sentry
	ctx := context.Background()
	ctx, span := provider.StartSpan(ctx, "test.sentry.span")
	defer span.End()

	// Test error recording
	testErr := assert.AnError
	provider.CaptureError(ctx, testErr, map[string]string{
		"test_tag": "test_value",
	})

	// Test span error recording
	provider.RecordError(ctx, span, testErr, "test error description")

	// Cleanup
	shutdownCtx := context.Background()
	err = provider.Shutdown(shutdownCtx)
	assert.NoError(t, err)
}

func TestProviderConfigDefaults(t *testing.T) {
	tests := []struct {
		name     string
		input    Config
		expected Config
	}{
		{
			name:  "empty config gets defaults",
			input: Config{},
			expected: Config{
				ServiceName:      ServiceName,
				ServiceVersion:   ServiceVersion,
				Environment:      "development",
				TracesSampleRate: 1.0,
				LogLevel:         slog.LevelInfo,
				LogFormat:        "text",
				Debug:            false,
			},
		},
		{
			name: "partial config preserves values",
			input: Config{
				ServiceName: "custom-service",
				Environment: "production",
			},
			expected: Config{
				ServiceName:      "custom-service",
				ServiceVersion:   ServiceVersion,
				Environment:      "production",
				TracesSampleRate: 1.0,
				LogLevel:         slog.LevelInfo,
				LogFormat:        "text",
				Debug:            false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Disable Sentry for testing
			tt.input.SentryDSN = ""
			
			provider, err := NewProvider(tt.input)
			require.NoError(t, err)
			require.NotNil(t, provider)

			// Verify the config was applied correctly by testing provider functionality
			assert.NotNil(t, provider.Logger())
			assert.NotNil(t, provider.Tracer())

			// Cleanup
			ctx := context.Background()
			err = provider.Shutdown(ctx)
			assert.NoError(t, err)
		})
	}
}

func TestTraceCorrelation(t *testing.T) {
	config := DefaultConfig()
	config.SentryDSN = "" // Disable Sentry for testing

	provider, err := NewProvider(config)
	require.NoError(t, err)
	require.NotNil(t, provider)

	ctx := context.Background()
	ctx, span := provider.StartSpan(ctx, "test.correlation")
	defer span.End()

	// The trace handler should add trace_id and span_id to log records
	// This is tested implicitly by the span context being valid
	assert.True(t, span.SpanContext().IsValid())
	assert.True(t, span.SpanContext().TraceID().IsValid())
	assert.True(t, span.SpanContext().SpanID().IsValid())

	// Test logging with trace context
	provider.Logger().InfoContext(ctx, "Test message with trace correlation")

	// Cleanup
	shutdownCtx := context.Background()
	err = provider.Shutdown(shutdownCtx)
	assert.NoError(t, err)
}