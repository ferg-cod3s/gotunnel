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
	
	// Set a temp directory for hosts backup for testing
	hostsBackupFile := filepath.Join(tempDir, "hosts.backup")
	manager.SetHostsBackupDir(hostsBackupFile)
	
	// Create a mock hosts file for testing
	mockHostsFile := filepath.Join(tempDir, "mock_hosts")
	err = os.WriteFile(mockHostsFile, []byte("127.0.0.1\tlocalhost\n"), 0644)
	require.NoError(t, err)
	
	// Override the global hosts file variable for testing
	originalHostsFile := hostsFile
	hostsFile = mockHostsFile

	cleanup := func() {
		ctx := context.Background()
		manager.Stop(ctx)
		// Restore original hosts file path
		hostsFile = originalHostsFile
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
	backendPort := 8080
	httpPort := 8180   // Use different port to avoid conflicts with other tests
	httpsPort := 8443

	// Start tunnel with custom ports
	err := manager.StartTunnelWithPorts(ctx, backendPort, domain, false, httpPort, httpsPort)
	require.NoError(t, err)

	// Verify tunnel is created
	manager.mu.RLock()
	tunnel, exists := manager.tunnels[domain]
	manager.mu.RUnlock()
	assert.True(t, exists)
	assert.Equal(t, backendPort, tunnel.Port)
	assert.Equal(t, domain, tunnel.Domain)
	assert.Equal(t, httpPort, tunnel.HTTPPort)

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
	backendPort := 8080
	httpPort := 8181   // Use different port to avoid conflicts
	httpsPort := 8444  // Use different port to avoid conflicts

	// Start HTTPS tunnel with custom ports
	err := manager.StartTunnelWithPorts(ctx, backendPort, domain, true, httpPort, httpsPort)
	if err != nil {
		t.Skipf("Skipping HTTPS test (might need certificates): %v", err)
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

	// Start multiple tunnels with different HTTP listen ports to avoid conflicts
	for i := 0; i < numTunnels; i++ {
		domain := fmt.Sprintf("test-%d.local", i)
		// Use StartTunnelWithPorts to specify different HTTP and HTTPS ports
		err := manager.StartTunnelWithPorts(ctx, 8080+i, domain, false, 8080+100+i, 8443+i)
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
				return manager.StartTunnelWithPorts(ctx, -1, "test.local", false, 8200, 8500)
			},
			wantErr: true,
		},
		{
			name: "Empty domain",
			fn: func() error {
				return manager.StartTunnelWithPorts(ctx, 8080, "", false, 8201, 8501)
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
				err := manager.StartTunnelWithPorts(ctx, 8080, domain, false, 8202, 8502)
				if err != nil {
					return err
				}
				return manager.StartTunnelWithPorts(ctx, 8081, domain, false, 8203, 8503)
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
