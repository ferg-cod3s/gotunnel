package observability

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMetrics(t *testing.T) {
	config := DefaultConfig()
	config.SentryDSN = "" // Disable Sentry for testing

	provider, err := NewProvider(config)
	require.NoError(t, err)

	metrics, err := NewMetrics(provider)
	require.NoError(t, err)
	require.NotNil(t, metrics)

	// Cleanup
	ctx := context.Background()
	err = provider.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestTunnelMetrics(t *testing.T) {
	config := DefaultConfig()
	config.SentryDSN = "" // Disable Sentry for testing

	provider, err := NewProvider(config)
	require.NoError(t, err)

	metrics, err := NewMetrics(provider)
	require.NoError(t, err)

	ctx := context.Background()

	// Test tunnel creation metrics
	metrics.TunnelCreated(ctx, "test.local", 8080, true)

	// Test tunnel destruction metrics
	duration := time.Minute * 5
	metrics.TunnelDestroyed(ctx, "test.local", duration)

	// Cleanup
	err = provider.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestHTTPMetrics(t *testing.T) {
	config := DefaultConfig()
	config.SentryDSN = "" // Disable Sentry for testing

	provider, err := NewProvider(config)
	require.NoError(t, err)

	metrics, err := NewMetrics(provider)
	require.NoError(t, err)

	ctx := context.Background()

	// Test HTTP request metrics
	metrics.HTTPRequest(ctx, "GET", "/api/test", 200, 1024, 2048, time.Millisecond*50)
	metrics.HTTPRequest(ctx, "POST", "/api/create", 201, 512, 1024, time.Millisecond*100)
	metrics.HTTPRequest(ctx, "GET", "/api/error", 500, 256, 512, time.Millisecond*200)

	// Cleanup
	err = provider.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestCertificateMetrics(t *testing.T) {
	config := DefaultConfig()
	config.SentryDSN = "" // Disable Sentry for testing

	provider, err := NewProvider(config)
	require.NoError(t, err)

	metrics, err := NewMetrics(provider)
	require.NoError(t, err)

	ctx := context.Background()

	// Test certificate metrics
	metrics.CertificateGenerated(ctx, "test.local")
	metrics.CertificateExpiry(ctx, "test.local", 30.5) // 30.5 days until expiry

	// Cleanup
	err = provider.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestErrorMetrics(t *testing.T) {
	config := DefaultConfig()
	config.SentryDSN = "" // Disable Sentry for testing

	provider, err := NewProvider(config)
	require.NoError(t, err)

	metrics, err := NewMetrics(provider)
	require.NoError(t, err)

	ctx := context.Background()

	// Test error recording
	testErr := assert.AnError
	metrics.RecordError(ctx, "network_error", "tunnel_start", testErr)
	metrics.RecordError(ctx, "permission_error", "cert_install", testErr)
	metrics.RecordError(ctx, "validation_error", "config_load", testErr)

	// Cleanup
	err = provider.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestOperationTimer(t *testing.T) {
	config := DefaultConfig()
	config.SentryDSN = "" // Disable Sentry for testing
	config.LogLevel = slog.LevelDebug // Enable debug logging to see timer logs

	provider, err := NewProvider(config)
	require.NoError(t, err)

	metrics, err := NewMetrics(provider)
	require.NoError(t, err)

	ctx := context.Background()

	// Test successful operation
	timer := metrics.StartOperation(ctx, "test_operation")
	time.Sleep(time.Millisecond * 10) // Simulate work
	timer.End(nil)

	// Test failed operation
	timer = metrics.StartOperation(ctx, "failing_operation")
	time.Sleep(time.Millisecond * 5) // Simulate work
	timer.End(assert.AnError)

	// Cleanup
	err = provider.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestMemoryMetrics(t *testing.T) {
	config := DefaultConfig()
	config.SentryDSN = "" // Disable Sentry for testing

	provider, err := NewProvider(config)
	require.NoError(t, err)

	metrics, err := NewMetrics(provider)
	require.NoError(t, err)

	ctx := context.Background()

	// Test memory usage recording
	metrics.UpdateMemoryUsage(ctx, 1024*1024*64) // 64 MB

	// Cleanup
	err = provider.Shutdown(ctx)
	assert.NoError(t, err)
}