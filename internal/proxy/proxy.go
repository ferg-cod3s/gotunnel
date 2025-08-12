package proxy

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/johncferguson/gotunnel/internal/privilege"
)

// ProxyMode defines how the proxy should operate
type ProxyMode string

const (
	NoProxy      ProxyMode = "none"     // User manages routing manually
	BuiltInProxy ProxyMode = "builtin"  // Use gotunnel's built-in proxy
	NginxProxy   ProxyMode = "nginx"    // Auto-configure nginx
	CaddyProxy   ProxyMode = "caddy"    // Auto-configure caddy
	AutoProxy    ProxyMode = "auto"     // Auto-detect best option
	ConfigOnly   ProxyMode = "config"   // Generate config files only
)

// ProxyType represents different proxy implementations
type ProxyType string

const (
	BuiltInProxyType ProxyType = "builtin"
	NginxProxyType   ProxyType = "nginx"  
	CaddyProxyType   ProxyType = "caddy"
	TraefikProxyType ProxyType = "traefik"
)

// ProxyConfig holds configuration for the proxy system
type ProxyConfig struct {
	Mode        ProxyMode `yaml:"mode" json:"mode"`
	Type        ProxyType `yaml:"type" json:"type"`
	HTTPPort    int       `yaml:"http_port" json:"http_port"`
	HTTPSPort   int       `yaml:"https_port" json:"https_port"`
	AutoInstall bool      `yaml:"auto_install" json:"auto_install"`
	ConfigPath  string    `yaml:"config_path" json:"config_path"`
}

// Route represents a proxy route mapping
type Route struct {
	Domain     string `json:"domain"`
	TargetHost string `json:"target_host"`
	TargetPort int    `json:"target_port"`
	HTTPS      bool   `json:"https"`
}

// Manager handles proxy operations and routing
type Manager struct {
	config     ProxyConfig
	routes     map[string]*Route // domain -> route mapping
	server     *http.Server
	listener   net.Listener
	actualPort int              // The actual port being used (important for port 0)
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewManager creates a new proxy manager
func NewManager(config ProxyConfig) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	
	// Set defaults
	if config.HTTPPort == 0 {
		config.HTTPPort = 80
	}
	if config.HTTPSPort == 0 {
		config.HTTPSPort = 443
	}
	if config.Mode == "" {
		config.Mode = AutoProxy
	}

	return &Manager{
		config: config,
		routes: make(map[string]*Route),
		ctx:    ctx,
		cancel: cancel,
	}
}

// DetectAvailableProxies scans the system for available proxy software
func DetectAvailableProxies() []ProxyType {
	var proxies []ProxyType

	// Check for various proxy types
	if commandExists("nginx") {
		proxies = append(proxies, NginxProxyType)
	}
	if commandExists("caddy") {
		proxies = append(proxies, CaddyProxyType)
	}
	if commandExists("traefik") {
		proxies = append(proxies, TraefikProxyType)
	}

	// Built-in is always available
	proxies = append(proxies, BuiltInProxyType)

	return proxies
}

// Start initializes and starts the proxy system
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch m.config.Mode {
	case BuiltInProxy, AutoProxy:
		return m.startBuiltInProxy()
	case NginxProxy:
		return m.startNginxProxy()
	case CaddyProxy:
		return m.startCaddyProxy()
	case ConfigOnly:
		return m.generateConfigFiles()
	case NoProxy:
		return nil // No proxy needed
	default:
		return fmt.Errorf("unsupported proxy mode: %s", m.config.Mode)
	}
}

// startBuiltInProxy starts the built-in HTTP proxy server
func (m *Manager) startBuiltInProxy() error {
	// Check if we can bind to privileged ports
	canBindPrivileged := privilege.HasRootPrivileges()
	
	httpPort := m.config.HTTPPort
	if httpPort == 0 {
		// Port 0 means use any available port (testing/dynamic allocation)
		// Keep it as 0 for dynamic allocation
	} else if !canBindPrivileged && httpPort < 1024 {
		// Fall back to high port and warn user
		httpPort = 8080
		fmt.Printf("⚠️  Cannot bind to port %d without privileges. Using port %d instead.\n", m.config.HTTPPort, httpPort)
		fmt.Printf("💡 Access your tunnels via: http://yourapp.local:%d\n", httpPort)
		fmt.Printf("💡 Or run with sudo for port 80 access: sudo gotunnel ...\n\n")
	}

	// Create the reverse proxy handler
	handler := &httputil.ReverseProxy{
		Director: m.proxyDirector,
		ErrorHandler: m.proxyErrorHandler,
	}

	// Create HTTP server
	m.server = &http.Server{
		Addr:              fmt.Sprintf(":%d", httpPort),
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Create listener
	listener, err := net.Listen("tcp", m.server.Addr)
	if err != nil {
		return fmt.Errorf("failed to create proxy listener on port %d: %w", httpPort, err)
	}
	m.listener = listener
	
	// Store the actual port (important for port 0)
	if tcpListener, ok := listener.(*net.TCPListener); ok {
		m.actualPort = tcpListener.Addr().(*net.TCPAddr).Port
	} else {
		m.actualPort = httpPort
	}

	// Start server in background
	go func() {
		if err := m.server.Serve(m.listener); err != nil && err != http.ErrServerClosed {
			fmt.Printf("⚠️  Proxy server error: %v\n", err)
		}
	}()

	fmt.Printf("✅ Built-in proxy started on port %d\n", httpPort)
	return nil
}

// proxyDirector handles routing logic for the reverse proxy
func (m *Manager) proxyDirector(req *http.Request) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	host := strings.Split(req.Host, ":")[0] // Remove port from host header
	route, exists := m.routes[host]
	
	if !exists {
		// Default behavior - return 404 will be handled by ErrorHandler
		req.URL = nil
		return
	}

	// Set up the proxy target
	scheme := "http"
	if route.HTTPS {
		scheme = "https"
	}

	target := &url.URL{
		Scheme: scheme,
		Host:   fmt.Sprintf("%s:%d", route.TargetHost, route.TargetPort),
	}

	// Update the request
	req.URL.Scheme = target.Scheme
	req.URL.Host = target.Host
	req.Host = target.Host

	// Add proxy headers
	req.Header.Set("X-Forwarded-For", getClientIP(req))
	req.Header.Set("X-Forwarded-Proto", scheme)
	req.Header.Set("X-Forwarded-Host", host)
}

// proxyErrorHandler handles proxy errors (like 404 for unknown routes)
func (m *Manager) proxyErrorHandler(w http.ResponseWriter, r *http.Request, err error) {
	host := strings.Split(r.Host, ":")[0]
	
	if r.URL == nil {
		// No route found
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><title>Tunnel Not Found</title></head>
<body>
<h1>🚇 gotunnel - Route Not Found</h1>
<p>No tunnel configured for <strong>%s</strong></p>
<p>Available routes:</p>
<ul>`, host)

		m.mu.RLock()
		for domain := range m.routes {
			fmt.Fprintf(w, "<li>%s</li>", domain)
		}
		m.mu.RUnlock()

		fmt.Fprint(w, `</ul>
<p><em>Configure a tunnel with: <code>gotunnel start [name] --port [port]</code></em></p>
</body>
</html>`)
		return
	}

	// Other proxy errors
	w.WriteHeader(http.StatusBadGateway)
	fmt.Fprintf(w, "Proxy Error: %v", err)
}

// AddRoute adds a new route to the proxy
func (m *Manager) AddRoute(route *Route) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Normalize domain (remove .local suffix if present for storage)
	domain := route.Domain
	if strings.HasSuffix(domain, ".local") {
		domain = strings.TrimSuffix(domain, ".local")
	}

	m.routes[domain+".local"] = route
	m.routes[domain] = route // Support both with and without .local

	fmt.Printf("🔗 Added proxy route: %s -> %s:%d\n", route.Domain, route.TargetHost, route.TargetPort)
	return nil
}

// RemoveRoute removes a route from the proxy
func (m *Manager) RemoveRoute(domain string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Remove both variations
	delete(m.routes, domain)
	if strings.HasSuffix(domain, ".local") {
		delete(m.routes, strings.TrimSuffix(domain, ".local"))
	} else {
		delete(m.routes, domain+".local")
	}

	fmt.Printf("🗑️  Removed proxy route: %s\n", domain)
	return nil
}

// ListRoutes returns all configured routes
func (m *Manager) ListRoutes() map[string]*Route {
	m.mu.RLock()
	defer m.mu.RUnlock()

	routes := make(map[string]*Route)
	for k, v := range m.routes {
		routes[k] = v
	}
	return routes
}

// Stop shuts down the proxy system
func (m *Manager) Stop() error {
	m.cancel()

	if m.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		if err := m.server.Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to shutdown proxy server: %w", err)
		}
	}

	if m.listener != nil {
		m.listener.Close()
	}

	fmt.Println("✅ Proxy stopped")
	return nil
}

// Helper functions

func commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

func getClientIP(req *http.Request) string {
	// Try X-Forwarded-For first
	if xff := req.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.Split(xff, ",")[0]
	}
	
	// Try X-Real-IP
	if xri := req.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	
	// Fall back to remote address
	ip, _, _ := net.SplitHostPort(req.RemoteAddr)
	return ip
}