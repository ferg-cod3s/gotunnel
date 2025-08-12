package proxy

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	config := ProxyConfig{
		Mode:      BuiltInProxy,
		HTTPPort:  8080,
		HTTPSPort: 8443,
	}

	manager := NewManager(config)
	assert.NotNil(t, manager)
	assert.Equal(t, BuiltInProxy, manager.config.Mode)
	assert.Equal(t, 8080, manager.config.HTTPPort)
	assert.Equal(t, 8443, manager.config.HTTPSPort)
	assert.NotNil(t, manager.routes)
}

func TestDetectAvailableProxies(t *testing.T) {
	proxies := DetectAvailableProxies()
	
	// Should always include built-in
	assert.Contains(t, proxies, BuiltInProxyType)
	
	// May include others depending on system
	t.Logf("Available proxies: %v", proxies)
}

func TestAddAndRemoveRoute(t *testing.T) {
	manager := NewManager(ProxyConfig{Mode: BuiltInProxy})

	route := &Route{
		Domain:     "test.local",
		TargetHost: "127.0.0.1",
		TargetPort: 3000,
		HTTPS:      false,
	}

	// Add route
	err := manager.AddRoute(route)
	require.NoError(t, err)

	// Verify route exists
	routes := manager.ListRoutes()
	assert.Contains(t, routes, "test.local")
	assert.Contains(t, routes, "test") // Should support both variations

	// Remove route
	err = manager.RemoveRoute("test.local")
	require.NoError(t, err)

	// Verify route removed
	routes = manager.ListRoutes()
	assert.NotContains(t, routes, "test.local")
	assert.NotContains(t, routes, "test")
}

func TestBuiltInProxyRouting(t *testing.T) {
	// Create a test backend server
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello from backend! Host: %s", r.Host)
	}))
	defer backend.Close()

	// Parse backend URL
	backendHost := strings.TrimPrefix(backend.URL, "http://")
	parts := strings.Split(backendHost, ":")
	require.Len(t, parts, 2)
	
	// Create proxy manager with high port (no privileges needed)
	// Use a random high port to avoid conflicts
	config := ProxyConfig{
		Mode:     BuiltInProxy,
		HTTPPort: 15000 + (int(time.Now().UnixNano()) % 1000), // Random port 15000-15999
	}
	manager := NewManager(config)

	// Add route
	route := &Route{
		Domain:     "test.local",
		TargetHost: parts[0],
		TargetPort: mustParseInt(parts[1]),
		HTTPS:      false,
	}
	err := manager.AddRoute(route)
	require.NoError(t, err)

	// Start proxy
	err = manager.Start()
	require.NoError(t, err)
	defer manager.Stop()

	// Give proxy time to start
	time.Sleep(100 * time.Millisecond)

	// Test proxy request using configured port
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("GET", fmt.Sprintf("http://localhost:%d", config.HTTPPort), nil)
	require.NoError(t, err)
	req.Host = "test.local" // Set Host header for routing

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), "Hello from backend!")
}

func TestBuiltInProxyNotFound(t *testing.T) {
	// Create proxy manager with high port
	config := ProxyConfig{
		Mode:     BuiltInProxy,
		HTTPPort: 9081, // Use different port
	}
	manager := NewManager(config)

	// Start proxy without any routes
	err := manager.Start()
	require.NoError(t, err)
	defer manager.Stop()

	// Give proxy time to start
	time.Sleep(100 * time.Millisecond)

	// Test request to unknown route
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("GET", "http://localhost:9081", nil)
	require.NoError(t, err)
	req.Host = "unknown.local"

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.Contains(t, string(body), "Route Not Found")
	assert.Contains(t, string(body), "unknown.local")
}

func TestProxyLifecycle(t *testing.T) {
	config := ProxyConfig{
		Mode:     BuiltInProxy,
		HTTPPort: 9082,
	}
	manager := NewManager(config)

	// Start proxy
	err := manager.Start()
	require.NoError(t, err)

	// Verify server is running
	assert.NotNil(t, manager.server)
	assert.NotNil(t, manager.listener)

	// Stop proxy
	err = manager.Stop()
	require.NoError(t, err)

	// Give time for shutdown
	time.Sleep(100 * time.Millisecond)

	// Verify connection fails after shutdown
	client := &http.Client{Timeout: 1 * time.Second}
	_, err = client.Get("http://localhost:9082")
	assert.Error(t, err) // Should fail to connect
}

func TestConfigOnlyMode(t *testing.T) {
	config := ProxyConfig{
		Mode: ConfigOnly,
	}
	manager := NewManager(config)

	// Add some test routes
	routes := []*Route{
		{
			Domain:     "app1.local",
			TargetHost: "127.0.0.1",
			TargetPort: 3000,
			HTTPS:      false,
		},
		{
			Domain:     "app2.local", 
			TargetHost: "127.0.0.1",
			TargetPort: 3001,
			HTTPS:      true,
		},
	}

	for _, route := range routes {
		err := manager.AddRoute(route)
		require.NoError(t, err)
	}

	// Test config generation
	err := manager.Start()
	require.NoError(t, err)

	// Should not create server in config-only mode
	assert.Nil(t, manager.server)
	assert.Nil(t, manager.listener)
}

func TestNoProxyMode(t *testing.T) {
	config := ProxyConfig{
		Mode: NoProxy,
	}
	manager := NewManager(config)

	// Start should succeed but do nothing
	err := manager.Start()
	require.NoError(t, err)

	// Should not create server
	assert.Nil(t, manager.server)
	assert.Nil(t, manager.listener)

	// Stop should also succeed
	err = manager.Stop()
	require.NoError(t, err)
}

func TestRouteNormalization(t *testing.T) {
	manager := NewManager(ProxyConfig{Mode: NoProxy})

	tests := []struct {
		input    string
		expected []string // Should be accessible via these domains
	}{
		{
			input:    "test.local",
			expected: []string{"test.local", "test"},
		},
		{
			input:    "app",
			expected: []string{"app.local", "app"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			route := &Route{
				Domain:     tt.input,
				TargetHost: "127.0.0.1", 
				TargetPort: 3000,
			}

			err := manager.AddRoute(route)
			require.NoError(t, err)

			routes := manager.ListRoutes()
			for _, expectedDomain := range tt.expected {
				assert.Contains(t, routes, expectedDomain, "Route should be accessible via %s", expectedDomain)
			}

			// Clean up for next test
			manager.RemoveRoute(tt.input)
		})
	}
}

// Helper function for tests
func mustParseInt(s string) int {
	if s == "80" {
		return 80
	} else if s == "443" {
		return 443
	}
	// For test backend ports, parse from the test server URL
	// This is a simplified parser for test purposes
	switch s {
	case "80":
		return 80
	case "443":
		return 443
	default:
		// For httptest server ports, we'll just return a reasonable default
		// In real code you'd want proper parsing
		if len(s) == 4 || len(s) == 5 {
			// Assume it's a valid port number from httptest
			var port int
			fmt.Sscanf(s, "%d", &port)
			return port
		}
		return 8080
	}
}