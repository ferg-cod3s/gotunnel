package mdns

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	server := New()
	assert.NotNil(t, server)
	assert.NotNil(t, server.services)
}

func TestRegisterAndUnregisterDomain(t *testing.T) {
	server := New()
	domain := "test-service.local"

	// Test registration
	err := server.RegisterDomain(domain)
	require.NoError(t, err)

	// Verify service is registered
	server.mu.RLock()
	service, exists := server.services[domain]
	server.mu.RUnlock()
	assert.True(t, exists)
	assert.NotNil(t, service)

	// Test unregistration
	err = server.UnregisterDomain(domain)
	require.NoError(t, err)

	// Verify service is unregistered
	server.mu.RLock()
	_, exists = server.services[domain]
	server.mu.RUnlock()
	assert.False(t, exists)
}

func TestRegisterDomainValidation(t *testing.T) {
	server := New()
	tests := []struct {
		name    string
		domain  string
		wantErr bool
	}{
		{
			name:    "Valid domain with .local",
			domain:  "test-service.local",
			wantErr: false,
		},
		{
			name:    "Valid domain without .local",
			domain:  "test-service",
			wantErr: false,
		},
		{
			name:    "Empty domain",
			domain:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := server.RegisterDomain(tt.domain)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if err == nil {
					// Clean up
					_ = server.UnregisterDomain(tt.domain)
				}
			}
		})
	}
}

func TestDiscoverServices(t *testing.T) {
	server := New()
	domain := "test-discover.local"

	// Register a service
	err := server.RegisterDomain(domain)
	require.NoError(t, err)
	defer server.UnregisterDomain(domain)

	// Give some time for the service to be advertised
	time.Sleep(100 * time.Millisecond)

	// Test discovery
	go server.DiscoverServices()

	// Give some time for discovery
	time.Sleep(500 * time.Millisecond)
}

func TestConcurrentOperations(t *testing.T) {
	server := New()
	const numServices = 5
	errCh := make(chan error, numServices*2) // For both register and unregister operations

	// Register services concurrently
	for i := 0; i < numServices; i++ {
		go func(i int) {
			domain := fmt.Sprintf("test-concurrent-%d.local", i)
			errCh <- server.RegisterDomain(domain)
		}(i)
	}

	// Wait for registrations
	for i := 0; i < numServices; i++ {
		err := <-errCh
		assert.NoError(t, err)
	}

	// Verify all services are registered
	server.mu.RLock()
	assert.Equal(t, numServices, len(server.services))
	server.mu.RUnlock()

	// Unregister services concurrently
	for i := 0; i < numServices; i++ {
		go func(i int) {
			domain := fmt.Sprintf("test-concurrent-%d.local", i)
			errCh <- server.UnregisterDomain(domain)
		}(i)
	}

	// Wait for unregistrations
	for i := 0; i < numServices; i++ {
		err := <-errCh
		assert.NoError(t, err)
	}

	// Verify all services are unregistered
	server.mu.RLock()
	assert.Equal(t, 0, len(server.services))
	server.mu.RUnlock()
}
