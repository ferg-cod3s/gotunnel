package dnsserver

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartDNSServer(t *testing.T) {
	err := StartDNSServer()
	require.NoError(t, err)
	defer Shutdown()

	// Verify server is initialized
	serverMu.Lock()
	assert.NotNil(t, globalServer)
	serverMu.Unlock()
}

func TestRegisterAndUnregisterDomain(t *testing.T) {
	err := StartDNSServer()
	require.NoError(t, err)
	defer Shutdown()

	domain := "test-service.local"
	port := 8080

	// Test registration
	err = RegisterDomain(domain, port)
	require.NoError(t, err)

	// Verify domain is registered
	serverMu.Lock()
	entry, exists := globalServer.entries[domain]
	serverMu.Unlock()
	assert.True(t, exists)
	assert.Equal(t, port, entry.port)
	assert.Equal(t, domain, entry.domain)

	// Test unregistration
	err = UnregisterDomain(domain)
	require.NoError(t, err)

	// Verify domain is unregistered
	serverMu.Lock()
	_, exists = globalServer.entries[domain]
	serverMu.Unlock()
	assert.False(t, exists)
}

func TestGetOutboundIP(t *testing.T) {
	ip := GetOutboundIP()
	assert.NotNil(t, ip)
	assert.True(t, ip.IsGlobalUnicast() || ip.IsLoopback(), "IP should be either global unicast or loopback")
}

func TestConcurrentRegistration(t *testing.T) {
	err := StartDNSServer()
	require.NoError(t, err)
	defer Shutdown()

	const numDomains = 5
	errCh := make(chan error, numDomains)
	doneCh := make(chan struct{})

	// Register multiple domains concurrently
	for i := 0; i < numDomains; i++ {
		go func(i int) {
			domain := fmt.Sprintf("test-service-%d.local", i)
			err := RegisterDomain(domain, 8080+i)
			errCh <- err
		}(i)
	}

	// Wait for all registrations
	go func() {
		for i := 0; i < numDomains; i++ {
			err := <-errCh
			assert.NoError(t, err)
		}
		close(doneCh)
	}()

	select {
	case <-doneCh:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for concurrent registrations")
	}

	// Verify all domains are registered
	serverMu.Lock()
	assert.Equal(t, numDomains, len(globalServer.entries))
	serverMu.Unlock()
}

func TestShutdown(t *testing.T) {
	err := StartDNSServer()
	require.NoError(t, err)

	// Register a domain
	err = RegisterDomain("test-shutdown.local", 8080)
	require.NoError(t, err)

	// Shutdown
	err = Shutdown()
	require.NoError(t, err)

	// Verify server is cleaned up
	serverMu.Lock()
	assert.Nil(t, globalServer)
	serverMu.Unlock()
}
