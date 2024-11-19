package tunnel

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/johncferguson/gotunnel/internal/cert"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestManager(t *testing.T) (*Manager, string, func()) {
	tempDir, err := os.MkdirTemp("", "tunnel-test-*")
	require.NoError(t, err)

	certManager := cert.New(filepath.Join(tempDir, "certs"))
	manager := NewManager(certManager)

	cleanup := func() {
		ctx := context.Background()
		manager.Stop(ctx)
		os.RemoveAll(tempDir)
	}

	return manager, tempDir, cleanup
}

func setupTestServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello, tunnel!")
	}))
}

func TestNewManager(t *testing.T) {
	manager, _, cleanup := setupTestManager(t)
	defer cleanup()

	assert.NotNil(t, manager)
	assert.NotNil(t, manager.tunnels)
	assert.NotNil(t, manager.certManager)
}

func TestStartAndStopTunnel(t *testing.T) {
	manager, _, cleanup := setupTestManager(t)
	defer cleanup()

	// Start a test HTTP server
	testServer := setupTestServer()
	defer testServer.Close()

	ctx := context.Background()
	domain := "test-tunnel.local"
	port := 8080
	httpsPort := 8443

	// Start tunnel
	err := manager.StartTunnel(ctx, port, domain, false, httpsPort)
	require.NoError(t, err)

	// Verify tunnel is created
	manager.mu.RLock()
	tunnel, exists := manager.tunnels[domain]
	manager.mu.RUnlock()
	assert.True(t, exists)
	assert.Equal(t, port, tunnel.Port)
	assert.Equal(t, domain, tunnel.Domain)

	// Stop tunnel
	err = manager.StopTunnel(ctx, domain)
	require.NoError(t, err)

	// Verify tunnel is removed
	manager.mu.RLock()
	_, exists = manager.tunnels[domain]
	manager.mu.RUnlock()
	assert.False(t, exists)
}

func TestHTTPSTunnel(t *testing.T) {
	manager, _, cleanup := setupTestManager(t)
	defer cleanup()

	ctx := context.Background()
	domain := "test-https.local"
	port := 8080
	httpsPort := 8443

	// Start HTTPS tunnel
	err := manager.StartTunnel(ctx, port, domain, true, httpsPort)
	if err != nil {
		t.Skipf("Skipping HTTPS test (might need privileges): %v", err)
		return
	}
	defer manager.StopTunnel(ctx, domain)

	// Verify HTTPS tunnel
	manager.mu.RLock()
	tunnel := manager.tunnels[domain]
	manager.mu.RUnlock()
	assert.True(t, tunnel.HTTPS)
	assert.Equal(t, httpsPort, tunnel.HTTPSPort)
}

func TestMultipleTunnels(t *testing.T) {
	manager, _, cleanup := setupTestManager(t)
	defer cleanup()

	ctx := context.Background()
	numTunnels := 3

	// Start multiple tunnels
	for i := 0; i < numTunnels; i++ {
		domain := fmt.Sprintf("test-%d.local", i)
		err := manager.StartTunnel(ctx, 8080+i, domain, false, 8443+i)
		require.NoError(t, err)
	}

	// Verify all tunnels are created
	tunnels := manager.ListTunnels()
	assert.Len(t, tunnels, numTunnels)

	// Stop all tunnels
	err := manager.Stop(ctx)
	require.NoError(t, err)

	// Verify all tunnels are stopped
	tunnels = manager.ListTunnels()
	assert.Len(t, tunnels, 0)
}

func TestErrorCases(t *testing.T) {
	manager, _, cleanup := setupTestManager(t)
	defer cleanup()

	ctx := context.Background()
	tests := []struct {
		name    string
		fn      func() error
		wantErr bool
	}{
		{
			name: "Invalid port",
			fn: func() error {
				return manager.StartTunnel(ctx, -1, "test.local", false, 8443)
			},
			wantErr: true,
		},
		{
			name: "Empty domain",
			fn: func() error {
				return manager.StartTunnel(ctx, 8080, "", false, 8443)
			},
			wantErr: true,
		},
		{
			name: "Stop non-existent tunnel",
			fn: func() error {
				return manager.StopTunnel(ctx, "nonexistent.local")
			},
			wantErr: true,
		},
		{
			name: "Duplicate tunnel",
			fn: func() error {
				domain := "duplicate.local"
				err := manager.StartTunnel(ctx, 8080, domain, false, 8443)
				if err != nil {
					return err
				}
				return manager.StartTunnel(ctx, 8081, domain, false, 8444)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
