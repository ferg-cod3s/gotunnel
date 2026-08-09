package proxy

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/v1truv1us/gotunnel/internal/cert"
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
	Mode        ProxyMode          `yaml:"mode" json:"mode"`
	Type        ProxyType          `yaml:"type" json:"type"`
	HTTPPort    int                `yaml:"http_port" json:"http_port"`
	HTTPSPort   int                `yaml:"https_port" json:"https_port"`
	AutoInstall bool               `yaml:"auto_install" json:"auto_install"`
	ConfigPath  string             `yaml:"config_path" json:"config_path"`
	CertManager *cert.CertManager  `yaml:"-" json:"-"`
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
	config      ProxyConfig
	routes      map[string]*Route
	server      *http.Server
	httpsServer *http.Server
	listener    net.Listener
	httpsListener net.Listener
	actualPort  int
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewManager creates a new proxy manager
func NewManager(config ProxyConfig) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	
	// Set defaults
	if config.HTTPPort == 0 {
		config.HTTPPort = 8080
	}
	if config.HTTPSPort == 0 {
		config.HTTPSPort = 8443
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

// startBuiltInProxy starts the built-in HTTP/HTTPS proxy server
func (m *Manager) startBuiltInProxy() error {
	httpPort := m.config.HTTPPort
	if httpPort == 0 {
		httpPort = 8080
	}
	httpsPort := m.config.HTTPSPort
	if httpsPort == 0 {
		httpsPort = 8443
	}

	handler := &httputil.ReverseProxy{
		Director:     m.proxyDirector,
		ErrorHandler: m.proxyErrorHandler,
	}

	// HTTP listener
	m.server = &http.Server{
		Addr:              fmt.Sprintf(":%d", httpPort),
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	listener, err := net.Listen("tcp", m.server.Addr)
	if err != nil {
		return fmt.Errorf("failed to create proxy listener on port %d: %w", httpPort, err)
	}
	m.listener = listener

	if tcpListener, ok := listener.(*net.TCPListener); ok {
		m.actualPort = tcpListener.Addr().(*net.TCPAddr).Port
	} else {
		m.actualPort = httpPort
	}

	go func() {
		if err := m.server.Serve(m.listener); err != nil && err != http.ErrServerClosed {
			fmt.Printf("⚠️  Proxy HTTP error: %v\n", err)
		}
	}()

	fmt.Printf("✅ Proxy started on port %d (HTTP)\n", httpPort)

	// HTTPS listener with SNI-based cert selection
	if m.config.CertManager != nil {
		tlsConfig := &tls.Config{
			GetCertificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
				sni := hello.ServerName
				if sni != "" {
					if cert, err := m.config.CertManager.EnsureCert(sni); err == nil {
						return cert, nil
					}
				}
				// Fall back to first route's cert (e.g. when accessing by IP)
				m.mu.RLock()
				for _, route := range m.routes {
					if route.Domain != "" {
						d := route.Domain
						m.mu.RUnlock()
						return m.config.CertManager.EnsureCert(d)
					}
				}
				m.mu.RUnlock()
				return nil, fmt.Errorf("no certificate available")
			},
			MinVersion: tls.VersionTLS12,
		}

		httpsLn, err := tls.Listen("tcp", fmt.Sprintf(":%d", httpsPort), tlsConfig)
		if err != nil {
			fmt.Printf("⚠️  Failed to start HTTPS on port %d: %v\n", httpsPort, err)
		} else {
			m.httpsListener = httpsLn
			m.httpsServer = &http.Server{
				Handler:     handler,
				IdleTimeout: 60 * time.Second,
			}
			go func() {
				if err := m.httpsServer.Serve(m.httpsListener); err != nil && err != http.ErrServerClosed {
					fmt.Printf("⚠️  Proxy HTTPS error: %v\n", err)
				}
			}()
			fmt.Printf("✅ Proxy started on port %d (HTTPS)\n", httpsPort)
		}
	}

	return nil
}

// proxyDirector handles routing logic for the reverse proxy
func (m *Manager) proxyDirector(req *http.Request) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	host := strings.Split(req.Host, ":")[0] // Remove port from host header
	route, exists := m.routes[host]

	if !exists {
		// Fall back to first available route (e.g. when accessing by IP from phone)
		for _, r := range m.routes {
			route = r
			exists = true
			break
		}
	}

	if !exists {
		// No routes at all — return 404 via ErrorHandler
		req.URL = nil
		return
	}

	// Set up the proxy target — always HTTP to tunnel (proxy terminates TLS)
	scheme := "http"

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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if m.server != nil {
		m.server.Shutdown(ctx)
	}
	if m.httpsServer != nil {
		m.httpsServer.Shutdown(ctx)
	}
	if m.listener != nil {
		m.listener.Close()
	}
	if m.httpsListener != nil {
		m.httpsListener.Close()
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