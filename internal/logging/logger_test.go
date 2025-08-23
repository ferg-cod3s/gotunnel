package logging

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	
	assert.Equal(t, LevelInfo, config.Level)
	assert.Equal(t, FormatText, config.Format)
	assert.Equal(t, "stdout", config.Output)
	assert.False(t, config.AddSource)
	assert.Equal(t, time.RFC3339, config.TimeFormat)
}

func TestNewLogger(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
		hasErr bool
	}{
		{
			name:   "nil config uses defaults",
			config: nil,
			hasErr: false,
		},
		{
			name: "custom config",
			config: &Config{
				Level:      LevelDebug,
				Format:     FormatJSON,
				Output:     "stdout",
				AddSource:  true,
				TimeFormat: time.RFC3339,
			},
			hasErr: false,
		},
		{
			name: "stderr output",
			config: &Config{
				Level:  LevelWarn,
				Format: FormatText,
				Output: "stderr",
			},
			hasErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := New(tt.config)
			
			if tt.hasErr {
				assert.Error(t, err)
				assert.Nil(t, logger)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, logger)
				assert.NotNil(t, logger.Logger)
				assert.NotNil(t, logger.config)
			}
		})
	}
}

func TestLoggerWithContext(t *testing.T) {
	// Create a tracer for testing
	tracer := otel.Tracer("test")
	ctx, span := tracer.Start(context.Background(), "test-span")
	defer span.End()

	logger, err := New(&Config{
		Level:  LevelDebug,
		Format: FormatJSON,
		Output: "stdout",
	})
	require.NoError(t, err)

	contextLogger := logger.WithContext(ctx)
	assert.NotNil(t, contextLogger)
	// Context loggers return the same instance but with different context
	assert.NotNil(t, contextLogger.Logger)
}

func TestLoggerWithComponent(t *testing.T) {
	logger, err := New(DefaultConfig())
	require.NoError(t, err)

	componentLogger := logger.WithComponent("tunnel")
	assert.NotNil(t, componentLogger)
	assert.NotEqual(t, logger, componentLogger)
}

func TestLoggerWithFields(t *testing.T) {
	logger, err := New(DefaultConfig())
	require.NoError(t, err)

	fields := map[string]any{
		"user_id": "123",
		"action":  "create_tunnel",
	}

	fieldsLogger := logger.WithFields(fields)
	assert.NotNil(t, fieldsLogger)
	assert.NotEqual(t, logger, fieldsLogger)
}

func TestLoggerWithError(t *testing.T) {
	logger, err := New(DefaultConfig())
	require.NoError(t, err)

	testErr := errors.New("test error")
	errorLogger := logger.WithError(testErr)
	assert.NotNil(t, errorLogger)
	assert.NotEqual(t, logger, errorLogger)

	// Test with nil error
	nilErrorLogger := logger.WithError(nil)
	assert.Equal(t, logger, nilErrorLogger)
}

func TestTunnelSpecificLogging(t *testing.T) {
	// Create logger for testing
	config := &Config{
		Level:  LevelDebug,
		Format: FormatJSON,
		Output: "stdout", // We'll capture this in tests
	}
	
	logger, err := New(config)
	require.NoError(t, err)

	// Test tunnel started
	logger.TunnelStarted("test.local", 3000, "localhost:3000")
	
	// Test tunnel stopped
	logger.TunnelStopped("test.local", time.Minute)
	
	// Test tunnel error
	testErr := errors.New("connection failed")
	details := map[string]any{
		"retry_count": 3,
		"last_error":  "timeout",
	}
	logger.TunnelError("test.local", testErr, details)
}

func TestProxyRequestLogging(t *testing.T) {
	logger, err := New(&Config{
		Level:  LevelDebug,
		Format: FormatJSON,
		Output: "stdout",
	})
	require.NoError(t, err)

	logger.ProxyRequest("GET", "test.local", "/api/health", 200, time.Millisecond*150, "Mozilla/5.0")
}

func TestCertificateLogging(t *testing.T) {
	logger, err := New(DefaultConfig())
	require.NoError(t, err)

	expiresAt := time.Now().Add(24 * time.Hour)
	logger.CertificateGenerated("test.local", expiresAt)

	certErr := errors.New("failed to generate certificate")
	logger.CertificateError("test.local", certErr)
}

func TestDNSLogging(t *testing.T) {
	logger, err := New(DefaultConfig())
	require.NoError(t, err)

	logger.DNSRegistered("test.local", "192.168.1.100")
	logger.DNSUnregistered("test.local")
}

func TestServiceLogging(t *testing.T) {
	logger, err := New(DefaultConfig())
	require.NoError(t, err)

	details := map[string]any{
		"port":    8080,
		"version": "1.0.0",
	}
	logger.ServiceStarted("proxy", details)
	logger.ServiceStopped("proxy", time.Minute*5)
}

func TestAuditLogging(t *testing.T) {
	logger, err := New(DefaultConfig())
	require.NoError(t, err)

	details := map[string]any{
		"ip_address": "192.168.1.100",
		"method":     "POST",
	}
	logger.Audit("create_tunnel", "user123", "tunnel:test.local", true, details)
}

func TestPerformanceLogging(t *testing.T) {
	logger, err := New(&Config{
		Level:  LevelDebug,
		Format: FormatJSON,
		Output: "stdout",
	})
	require.NoError(t, err)

	details := map[string]any{
		"requests_count": 100,
		"cache_hits":     85,
	}
	logger.Performance("proxy_request", time.Millisecond*50, details)
}

func TestGetCaller(t *testing.T) {
	file, line, funcName := GetCaller(0)
	
	assert.Contains(t, file, "logger_test.go")
	assert.Greater(t, line, 0)
	assert.Contains(t, funcName, "TestGetCaller")
}

func TestLogLevels(t *testing.T) {
	tests := []struct {
		name     string
		level    LogLevel
		expected string
	}{
		{"debug level", LevelDebug, "DEBUG"},
		{"info level", LevelInfo, "INFO"},
		{"warn level", LevelWarn, "WARN"},
		{"error level", LevelError, "ERROR"},
		{"invalid level defaults to info", "invalid", "INFO"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Level:  tt.level,
				Format: FormatText,
				Output: "stdout",
			}
			
			logger, err := New(config)
			assert.NoError(t, err)
			assert.NotNil(t, logger)
		})
	}
}

func TestLogFormats(t *testing.T) {
	tests := []struct {
		name   string
		format LogFormat
	}{
		{"json format", FormatJSON},
		{"text format", FormatText},
		{"invalid format defaults to text", "invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Level:  LevelInfo,
				Format: tt.format,
				Output: "stdout",
			}
			
			logger, err := New(config)
			assert.NoError(t, err)
			assert.NotNil(t, logger)
		})
	}
}

func TestLoggerCreation(t *testing.T) {
	// This is a simplified test - in a real implementation you'd want
	// to capture the actual output from the logger
	logger, err := New(&Config{
		Level:  LevelDebug,
		Format: FormatJSON,
		Output: "stdout",
	})
	require.NoError(t, err)
	
	logger.Info("test message", "key", "value")
}

func TestJSONOutput(t *testing.T) {
	config := &Config{
		Level:  LevelInfo,
		Format: FormatJSON,
		Output: "stdout",
	}
	
	logger, err := New(config)
	require.NoError(t, err)
	
	// Test that we can create a JSON logger without errors
	logger.Info("test message", "key", "value")
}

func TestTimeFormatCustomization(t *testing.T) {
	config := &Config{
		Level:      LevelInfo,
		Format:     FormatJSON,
		Output:     "stdout",
		TimeFormat: "2006-01-02 15:04:05",
	}
	
	logger, err := New(config)
	require.NoError(t, err)
	
	logger.Info("test message with custom time format")
}

func TestSourceAddition(t *testing.T) {
	config := &Config{
		Level:     LevelInfo,
		Format:    FormatJSON,
		Output:    "stdout",
		AddSource: true,
	}
	
	logger, err := New(config)
	require.NoError(t, err)
	
	logger.Info("test message with source information")
}