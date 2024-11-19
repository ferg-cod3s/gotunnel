package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
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
		fmt.Printf("Failed to create temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tempDir)

	os.Exit(m.Run())
}

func setupTestServer() (*http.Server, error) {
	// Create a test HTTP server
	srv := &http.Server{
		Addr: ":8080",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "Hello from test server!")
		}),
	}

	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			fmt.Printf("HTTP server error: %v\n", err)
		}
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)
	return srv, nil
}

func TestTunnelCreation(t *testing.T) {
	// Create temp directory for test
	tempDir, err := os.MkdirTemp("", "gotunnel-tunnel-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Setup test server
	srv, err := setupTestServer()
	require.NoError(t, err)
	defer srv.Shutdown(context.Background())

	tests := []struct {
		name    string
		domain  string
		port    int
		https   bool
		wantErr bool
	}{
		{
			name:    "Basic HTTP Tunnel",
			domain:  "test-http",
			port:    8080,
			https:   false,
			wantErr: false,
		},
		{
			name:    "HTTPS Tunnel",
			domain:  "test-https",
			port:    8080,
			https:   true,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create tunnel manager
			certManager := cert.New(filepath.Join(tempDir, "certs"))
			manager := tunnel.NewManager(certManager)

			// Start tunnel
			ctx := context.Background()
			err := manager.StartTunnel(ctx, tt.port, tt.domain, tt.https, tt.port+1000)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			defer manager.StopTunnel(ctx, tt.domain)

			// Test tunnel connection
			protocol := "http"
			if tt.https {
				protocol = "https"
			}
			resp, err := http.Get(fmt.Sprintf("%s://%s.local:%d", protocol, tt.domain, tt.port))
			require.NoError(t, err)
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.Contains(t, string(body), "Hello from test server!")
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
	domains := []string{"test1", "test2", "test3"}
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
