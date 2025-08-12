package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/johncferguson/gotunnel/internal/cert"
	"github.com/johncferguson/gotunnel/internal/tunnel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	// Setup test environment
	tempDir, err := os.MkdirTemp("", "gotunnel-test-*")
	if err != nil {
		log.Printf("Failed to create temp directory: %v", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tempDir)

	os.Exit(m.Run())
}

func setupTestServer(t *testing.T) (*http.Server, int) {
	t.Helper()

	// Create a test HTTP server with a random available port
	listener, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port

	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "Hello from test server!")
		}),
	}

	go func() {
		if err := srv.Serve(listener); err != http.ErrServerClosed {
			t.Logf("HTTP server error: %v", err)
		}
	}()

	return srv, port
}

func setupTestServerWithCleanup(t *testing.T) (*http.Server, int, func()) {
	srv, port := setupTestServer(t)
	return srv, port, func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			t.Logf("Error shutting down test server: %v", err)
		}
	}
}

func setupTunnelManagerWithCleanup(t *testing.T) (*tunnel.Manager, func()) {
	// Create a temp directory for certs and hosts backup
	tmpDir, err := os.MkdirTemp("", "gotunnel-test-*")
	require.NoError(t, err)

	// Create cert manager with temp dir for certs
	certManager := cert.New(tmpDir)

	// Create tunnel manager with temp file for hosts backup
	manager := tunnel.NewManager(certManager)
	manager.SetHostsBackupDir(filepath.Join(tmpDir, "hosts.bak"))

	return manager, func() {
		// Cleanup
		os.RemoveAll(tmpDir)
	}
}

func TestTunnelCreation(t *testing.T) {
	// Remove the privilege check
	// if os.Getuid() != 0 {
	//     t.Skip("Skipping test - requires root privileges")
	// }

	// Create tunnel manager first
	manager, cleanup := setupTunnelManagerWithCleanup(t)
	defer cleanup()

	tests := []struct {
		name    string
		domain  string
		https   bool
		wantErr bool
	}{
		{
			name:    "Basic HTTP Tunnel",
			domain:  "test-http",
			https:   false,
			wantErr: false,
		},
		{
			name:    "HTTPS Tunnel",
			domain:  "test-https",
			https:   true,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip HTTPS tests if mkcert is not available
			if tt.https {
				if _, err := exec.LookPath("mkcert"); err != nil {
					t.Skip("Skipping HTTPS test - mkcert not available")
				}
			}

			// Create test server for this test case (this is our target)
			srv, targetPort := setupTestServer(t)
			defer func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := srv.Shutdown(ctx); err != nil {
					t.Logf("Error shutting down test server: %v", err)
				}
			}()

			// Get available port for tunnel to listen on
			tunnelListener, err := net.Listen("tcp", ":0")
			require.NoError(t, err)
			tunnelPort := tunnelListener.Addr().(*net.TCPAddr).Port
			tunnelListener.Close() // Close immediately to free the port

			var httpsPort int
			if tt.https {
				httpsListener, err := net.Listen("tcp", ":0")
				require.NoError(t, err)
				httpsPort = httpsListener.Addr().(*net.TCPAddr).Port
				httpsListener.Close() // Close immediately to free the port
			}

			// Start tunnel with custom ports for testing
			// targetPort = backend app port, tunnelPort/httpsPort = tunnel listen ports
			if tt.https {
				err = manager.StartTunnelWithPorts(context.Background(), targetPort, tt.domain, true, 0, httpsPort)
			} else {
				err = manager.StartTunnelWithPorts(context.Background(), targetPort, tt.domain, false, tunnelPort, 0)
			}
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Ensure tunnel is cleaned up
			defer func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := manager.StopTunnel(ctx, tt.domain+".local"); err != nil {
					t.Logf("Error stopping tunnel: %v", err)
				}
			}()

			// Give DNS time to propagate
			time.Sleep(2 * time.Second)

			// Test the tunnel by connecting to tunnelPort (HTTP) or httpsPort (HTTPS)
			protocol := "http"
			testPort := tunnelPort
			if tt.https {
				protocol = "https"
				testPort = httpsPort
			}

			client := &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: true,
					},
				},
				Timeout: 5 * time.Second,
			}

			// Connect directly to the tunnel listener for testing
			// In production, this would go through domain resolution
			var resp *http.Response
			var lastErr error
			for i := 0; i < 5; i++ {
				// Test by connecting directly to the tunnel port
				resp, err = client.Get(fmt.Sprintf("%s://127.0.0.1:%d", protocol, testPort))
				if err == nil {
					break
				}
				lastErr = err
				time.Sleep(1 * time.Second)
			}
			if lastErr != nil {
				t.Fatalf("Failed to connect after retries: %v", lastErr)
			}

			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.Equal(t, "Hello from test server!", string(body))
		})
	}
}

func TestTunnelManagement(t *testing.T) {
	// Skip if not running as root since we need to modify hosts file
	if os.Getuid() != 0 {
		t.Skip("Skipping test - requires root privileges")
	}

	// Create temp directory for test
	tempDir, err := os.MkdirTemp("", "gotunnel-mgmt-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create cert manager with temp dir for certs
	certManager := cert.New(tempDir)

	// Create tunnel manager with temp file for hosts backup
	manager := tunnel.NewManager(certManager)
	manager.SetHostsBackupDir(filepath.Join(tempDir, "hosts.bak"))

	// Test tunnel management operations
	ctx := context.Background()

	// Create multiple test servers and tunnels
	var servers []*http.Server
	var ports []int
	for i := 0; i < 3; i++ {
		srv, port := setupTestServer(t)
		servers = append(servers, srv)
		ports = append(ports, port)
	}

	// Cleanup test servers
	defer func() {
		for _, srv := range servers {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := srv.Shutdown(ctx); err != nil {
				t.Logf("Error shutting down test server: %v", err)
			}
		}
	}()

	// Start multiple tunnels
	domains := []string{"test1", "test2", "test3"}
	for i, domain := range domains {
		err := manager.StartTunnel(ctx, ports[i], domain, false, 0)
		require.NoError(t, err)
	}

	// Test tunnel listing
	tunnels := manager.ListTunnels()
	assert.Len(t, tunnels, len(domains))
	for _, domain := range domains {
		found := false
		for _, tunnel := range tunnels {
			if tunnel["domain"].(string) == domain+".local" {
				found = true
				break
			}
		}
		assert.True(t, found, "Tunnel for domain %s not found", domain)
	}

	// Test individual tunnel stopping
	err = manager.StopTunnel(ctx, domains[0]+".local")
	require.NoError(t, err)
	tunnels = manager.ListTunnels()
	assert.Len(t, tunnels, len(domains)-1)

	// Test stopping all tunnels
	err = manager.Stop(ctx)
	require.NoError(t, err)
	tunnels = manager.ListTunnels()
	assert.Len(t, tunnels, 0)
}

func TestErrorHandling(t *testing.T) {
	// Skip if not running as root since we need to modify hosts file
	if os.Getuid() != 0 {
		t.Skip("Skipping test - requires root privileges")
	}

	tempDir, err := os.MkdirTemp("", "gotunnel-error-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	certManager := cert.New(tempDir)
	manager := tunnel.NewManager(certManager)
	manager.SetHostsBackupDir(filepath.Join(tempDir, "hosts.bak"))

	tests := []struct {
		name    string
		setup   func(t *testing.T) error
		wantErr string
	}{
		{
			name: "Invalid Port",
			setup: func(t *testing.T) error {
				return manager.StartTunnel(context.Background(), -1, "test", false, 0)
			},
			wantErr: "invalid port",
		},
		{
			name: "Empty Domain",
			setup: func(t *testing.T) error {
				return manager.StartTunnel(context.Background(), 8080, "", false, 0)
			},
			wantErr: "empty domain",
		},
		{
			name: "Stop Non-existent Tunnel",
			setup: func(t *testing.T) error {
				return manager.StopTunnel(context.Background(), "nonexistent.local")
			},
			wantErr: "tunnel not found",
		},
		{
			name: "Invalid Hosts Backup Path",
			setup: func(t *testing.T) error {
				manager.SetHostsBackupDir("/nonexistent/dir/hosts.bak")
				return manager.StartTunnel(context.Background(), 8080, "test", false, 0)
			},
			wantErr: "failed to backup hosts file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.setup(t)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
