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
	"github.com/johncferguson/gotunnel/internal/dnsserver"
	"github.com/johncferguson/gotunnel/internal/tunnel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	// Create a context with timeout for the entire test suite
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Setup test environment
	tempDir, err := os.MkdirTemp("", "gotunnel-test-*")
	if err != nil {
		log.Printf("Failed to create temp directory: %v", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tempDir)

	// Run cleanup after tests finish
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 2*time.Second)
		defer shutdownCancel()
		if err := dnsserver.Shutdown(); err != nil {
			log.Printf("Error shutting down DNS server: %v", err)
		}
	}()

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

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)
	return srv, port
}

func setupTestServerWithCleanup(t *testing.T) (*http.Server, int, func()) {
	srv, port := setupTestServer(t)
	return srv, port, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			t.Logf("Error shutting down test server: %v", err)
		}
	}
}

func setupTunnelManagerWithCleanup(t *testing.T) (*tunnel.Manager, func()) {
	certManager := cert.New(os.TempDir())
	manager := tunnel.NewManager(certManager)
	return manager, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := manager.Close(ctx); err != nil {
			t.Logf("Error closing tunnel manager: %v", err)
		}
		// Ensure DNS server is cleaned up
		if err := dnsserver.Shutdown(); err != nil {
			t.Logf("Error shutting down DNS server: %v", err)
		}
	}
}

func TestTunnelCreation(t *testing.T) {
	// Create test server with random port
	_, port, cleanupSrv := setupTestServerWithCleanup(t)
	defer cleanupSrv()

	// Create tunnel manager
	manager, cleanupManager := setupTunnelManagerWithCleanup(t)
	defer cleanupManager()

	tests := []struct {
		name      string
		domain    string
		port      int
		httpsPort int
		https     bool
		wantErr   bool
	}{
		{
			name:      "Basic HTTP Tunnel",
			domain:    "test-http",
			port:      port,
			httpsPort: port + 1,
			https:     false,
			wantErr:   false,
		},
		{
			name:      "HTTPS Tunnel",
			domain:    "test-https",
			port:      port + 2,
			httpsPort: port + 3,
			https:     true,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip HTTPS tests if mkcert is not available or we don't have permissions
			if tt.https {
				if _, err := exec.LookPath("mkcert"); err != nil || os.Getuid() != 0 {
					t.Skip("Skipping HTTPS test - mkcert not available or not running as root")
				}
			}

			ctx := context.Background()

			err := manager.StartTunnel(ctx, tt.port, tt.domain, tt.https, tt.httpsPort)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			defer func() {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				if err := manager.StopTunnel(ctx, tt.domain+".local"); err != nil {
					t.Logf("Error stopping tunnel: %v", err)
				}
			}()

			// Give DNS time to propagate (reduced from 500ms)
			time.Sleep(100 * time.Millisecond)

			// Test the tunnel...
			protocol := "http"
			testPort := tt.port
			if tt.https {
				protocol = "https"
				testPort = tt.httpsPort
			}

			client := &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: true,
					},
				},
				Timeout: 2 * time.Second, // Add timeout to prevent hanging
			}
			resp, err := client.Get(fmt.Sprintf("%s://%s.local:%d", protocol, tt.domain, testPort))
			require.NoError(t, err)
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.Equal(t, "Hello from test server!", string(body))
		})
	}
}

func TestTunnelManagement(t *testing.T) {
	// Create temp directory for test
	tempDir, err := os.MkdirTemp("", "gotunnel-mgmt-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	certManager := cert.New(filepath.Join(tempDir, "certs"))
	manager := tunnel.NewManager(certManager)
	ctx := context.Background()

	// Test multiple tunnel creation
	domains := []string{"test1.local", "test2.local", "test3.local"}
	for i, domain := range domains {
		err := manager.StartTunnel(ctx, 8080+i, domain, false, 9080+i)
		require.NoError(t, err)
	}

	// Test tunnel listing
	tunnels := manager.ListTunnels()
	assert.Len(t, tunnels, len(domains))

	// Test individual tunnel stopping
	err = manager.StopTunnel(ctx, domains[0])
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
	tempDir, err := os.MkdirTemp("", "gotunnel-error-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	certManager := cert.New(filepath.Join(tempDir, "certs"))
	manager := tunnel.NewManager(certManager)
	ctx := context.Background()

	tests := []struct {
		name    string
		fn      func() error
		wantErr bool
	}{
		{
			name: "Invalid Port",
			fn: func() error {
				return manager.StartTunnel(ctx, -1, "test", false, 9080)
			},
			wantErr: true,
		},
		{
			name: "Empty Domain",
			fn: func() error {
				return manager.StartTunnel(ctx, 8080, "", false, 9080)
			},
			wantErr: true,
		},
		{
			name: "Stop Non-existent Tunnel",
			fn: func() error {
				return manager.StopTunnel(ctx, "nonexistent")
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
