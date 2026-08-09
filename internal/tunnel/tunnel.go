package tunnel

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/v1truv1us/gotunnel/internal/cert"
	"github.com/v1truv1us/gotunnel/internal/dnsserver"
	"github.com/v1truv1us/gotunnel/internal/logging"
	"github.com/v1truv1us/gotunnel/internal/proxy"
)

const (
	defaultHostsFile = "/etc/hosts"
)

// For testing purposes - allow overriding the hosts file path
var hostsFile = defaultHostsFile

type Tunnel struct {
	Port        int    // Backend target port (where user's app runs)
	HTTPPort    int    // Tunnel HTTP listen port (default 80)
	HTTPSPort   int    // Tunnel HTTPS listen port (default 443) 
	Domain      string
	TargetIP    string
	HTTPS       bool
	server      *http.Server
	listener    net.Listener
	done        chan struct{}
	Cert        *tls.Certificate
}

type Manager struct {
	tunnels      map[string]*Tunnel
	mu           sync.RWMutex
	certManager  *cert.CertManager
	hostsBackup  string
	proxyManager *proxy.Manager
	logger       *logging.Logger
	useProxy     bool
}

func NewManager(certManager *cert.CertManager, logger *logging.Logger) *Manager {
	if logger == nil {
		logger, _ = logging.New(logging.DefaultConfig())
	}
	return NewManagerWithProxy(certManager, nil, false, logger)
}

func NewManagerWithProxy(certManager *cert.CertManager, proxyManager *proxy.Manager, useProxy bool, logger *logging.Logger) *Manager {
	if logger == nil {
		logger, _ = logging.New(logging.DefaultConfig())
	}
	// Initialize DNS server when creating a new manager
	if err := dnsserver.StartDNSServer(); err != nil {
		logger.Warn("Failed to initialize DNS server", "error", err)
	} else {
		logger.Info("DNS server initialized successfully")
	}

	return &Manager{
		tunnels:      make(map[string]*Tunnel),
		certManager:  certManager,
		proxyManager: proxyManager,
		useProxy:     useProxy,
		logger:       logger.WithComponent("tunnel"),
	}
}

// backupHostsFile creates a backup of the hosts file
func (m *Manager) backupHostsFile() error {
	content, err := os.ReadFile(hostsFile)
	if err != nil {
		return fmt.Errorf("failed to read hosts file: %w", err)
	}

	if err := os.WriteFile(m.hostsBackup, content, 0644); err != nil {
		return fmt.Errorf("failed to create hosts backup: %w", err)
	}

	return nil
}

// restoreHostsFile restores the hosts file from backup
func (m *Manager) restoreHostsFile() error {
	if m.hostsBackup == "" {
		return nil // No backup exists
	}

	content, err := os.ReadFile(m.hostsBackup)
	if err != nil {
		return fmt.Errorf("failed to read hosts backup: %w", err)
	}

	if err := os.WriteFile(hostsFile, content, 0644); err != nil {
		return fmt.Errorf("failed to restore hosts file: %w", err)
	}

	// Clean up backup file
	if err := os.Remove(m.hostsBackup); err != nil {
		log.Printf("Warning: Failed to remove backup file: %v", err)
	}

	return nil
}

// StartTunnelWithPorts starts a tunnel with custom listen ports (for testing)
func (m *Manager) StartTunnelWithPorts(ctx context.Context, backendPort int, domain string, https bool, httpPort, httpsPort int) error {
	// Set defaults if needed
	if httpsPort == 0 {
		httpsPort = 443
	}
	if httpPort == 0 {
		httpPort = 80
	}

	m.logger.WithContext(ctx).Info("Starting tunnel",
		"domain", domain,
		"backend_port", backendPort,
		"https", https,
		"http_port", httpPort,
		"https_port", httpsPort,
	)

	startTime := time.Now()
	err := m.startTunnelInternal(ctx, backendPort, domain, https, httpPort, httpsPort)
	
	if err != nil {
		m.logger.WithContext(ctx).TunnelError(domain, err, map[string]any{
			"backend_port": backendPort,
			"duration": time.Since(startTime),
		})
		return err
	}

	m.logger.WithContext(ctx).TunnelStarted(domain, backendPort, fmt.Sprintf("localhost:%d", backendPort))
	return nil
}

// StartTunnel starts a tunnel with default ports (production use)
func (m *Manager) StartTunnel(ctx context.Context, backendPort int, domain string, https bool, httpsPort int) error {
	return m.StartTunnelWithPorts(ctx, backendPort, domain, https, 8080, httpsPort)
}

func (m *Manager) startTunnelInternal(ctx context.Context, backendPort int, domain string, https bool, httpPort, httpsPort int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Validate inputs
	if backendPort <= 0 || backendPort > 65535 {
		return fmt.Errorf("invalid backend port: %d", backendPort)
	}
	if domain == "" {
		return fmt.Errorf("domain cannot be empty")
	}
	if err := ValidateDomain(domain); err != nil {
		return fmt.Errorf("invalid domain: %w", err)
	}
	if httpPort <= 0 || httpPort > 65535 {
		return fmt.Errorf("invalid HTTP port: %d", httpPort)
	}
	if httpsPort <= 0 || httpsPort > 65535 {
		return fmt.Errorf("invalid HTTPS port: %d", httpsPort)
	}

	// Prevent duplicate tunnels for the same domain
	if _, exists := m.tunnels[domain]; exists {
		return fmt.Errorf("tunnel for domain %s already exists", domain)
	}

	// If using proxy, modify ports to avoid conflicts
	tunnelHTTPPort := httpPort
	tunnelHTTPSPort := httpsPort
	
	if m.useProxy && m.proxyManager != nil {
		// Use high ports for actual tunnel, proxy will handle 80/443
		// Start from 9080 to avoid conflicts with proxy on 8080
		tunnelHTTPPort = 9080 + len(m.tunnels)  // Dynamic port allocation  
		tunnelHTTPSPort = 9443 + len(m.tunnels)
		
		log.Printf("Using proxy mode: tunnel will run on ports %d/%d, accessible via proxy on %d/%d", 
			tunnelHTTPPort, tunnelHTTPSPort, httpPort, httpsPort)
	}

	// Ensure domain has .local suffix (RFC 6761: auto-resolves to 127.0.0.1, no /etc/hosts needed)
	if !strings.HasSuffix(domain, ".local") {
		domain = domain + ".local"
	}

	// Create new tunnel instance
	tunnel := &Tunnel{
		Port:      backendPort,      // Backend target port (where user's app runs)
		HTTPPort:  tunnelHTTPPort,   // Tunnel HTTP listen port (may be high port if using proxy)
		HTTPSPort: tunnelHTTPSPort,  // Tunnel HTTPS listen port (may be high port if using proxy)
		Domain:    domain,
		TargetIP:  "127.0.0.1",
		HTTPS:     https,
		done:      make(chan struct{}), // Initialize the done channel
	}

	// Ensure the SSL/TLS certificate is available
	if https {
		cert, err := m.certManager.EnsureCert(domain)
		if err != nil {
			return fmt.Errorf("failed to ensure certificate: %w", err)
		}
		tunnel.Cert = cert
	}

	if err := m.startTunnel(tunnel); err != nil {
		return fmt.Errorf("failed to start tunnel: %w", err)
	}

	// Add to internal map for tracking
	m.tunnels[domain] = tunnel

	// Register with proxy if using proxy mode
	if m.useProxy && m.proxyManager != nil {
		route := &proxy.Route{
			Domain:     domain,
			TargetHost: "127.0.0.1",
			TargetPort: tunnel.HTTPPort, // Proxy routes to tunnel's actual port
			HTTPS:      https,
		}
		
		if err := m.proxyManager.AddRoute(route); err != nil {
			log.Printf("Warning: Failed to register proxy route: %v", err)
		} else {
			log.Printf("✅ Registered proxy route: %s -> 127.0.0.1:%d", domain, tunnel.HTTPPort)
		}
	}

	// .local domains resolve to 127.0.0.1 via RFC 6761 — no /etc/hosts backup/modification needed

	return nil
}

func (m *Manager) Stop(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []error
	// Stop all tunnels
	for domain, tunnel := range m.tunnels {
		if err := tunnel.stop(ctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to stop tunnel %s: %w", domain, err))
		}
	}

	// Clear the tunnels map
	m.tunnels = make(map[string]*Tunnel)

	// .local domains don't modify /etc/hosts — no restore needed

	// If there were any errors, return them combined
	if len(errs) > 0 {
		return fmt.Errorf("errors during shutdown: %v", errs)
	}

	return nil
}

func (m *Manager) StopTunnel(ctx context.Context, domain string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	tunnel, exists := m.tunnels[domain]
	if !exists {
		return fmt.Errorf("tunnel for domain %s does not exist", domain)
	}

	// Stop the tunnel
	if err := tunnel.stop(ctx); err != nil {
		return fmt.Errorf("failed to stop tunnel: %w", err)
	}

	// Remove from hosts file (only if not using proxy mode)
	// .local domains resolve via RFC 6761 — no /etc/hosts cleanup needed

	// Remove from proxy if using proxy mode
	if m.useProxy && m.proxyManager != nil {
		if err := m.proxyManager.RemoveRoute(domain); err != nil {
			log.Printf("Warning: Failed to remove proxy route: %v", err)
		} else {
			log.Printf("🗑️  Removed proxy route: %s", domain)
		}
	}

	// Unregister from mDNS
	if err := dnsserver.UnregisterDomain(domain); err != nil {
		log.Printf("Warning: Failed to unregister domain from mDNS: %v", err)
	}

	// Remove from tunnels map
	delete(m.tunnels, domain)
	return nil
}

func (t *Tunnel) stop(ctx context.Context) error {
	if t.server != nil {
		// Server shutdown should gracefully close the listener
		if err := t.server.Shutdown(ctx); err != nil {
			// If graceful shutdown fails, force close the listener
			if t.listener != nil {
				t.listener.Close()
			}
			return fmt.Errorf("error shutting down server: %w", err)
		}
		t.server = nil
	} else if t.listener != nil {
		// Only close listener directly if server wasn't running
		if err := t.listener.Close(); err != nil {
			return fmt.Errorf("error closing listener: %w", err)
		}
	}
	t.listener = nil

	// Remove from hosts file (will be handled by manager for proxy mode)
	// Note: This is called from manager which handles proxy mode appropriately

	close(t.done)
	return nil
}

func (m *Manager) ListTunnels() []map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tunnelList := make([]map[string]interface{}, 0, len(m.tunnels))
	for domain, tunnel := range m.tunnels {
		tunnelInfo := map[string]interface{}{
			"domain": domain,
			"port":   tunnel.Port,
			"https":  tunnel.HTTPS,
		}
		tunnelList = append(tunnelList, tunnelInfo)
	}

	return tunnelList
}


func (m *Manager) startTunnel(t *Tunnel) error {
	// Get the machine's network IP for the proxy
	ip := dnsserver.GetOutboundIP()
	t.TargetIP = ip.String()

	// .local resolves to 127.0.0.1 via RFC 6761 — no /etc/hosts modification needed

	// Register domain with DNS server (use tunnel listen port, not backend port)
	listenPort := t.HTTPPort
	if t.HTTPS {
		listenPort = t.HTTPSPort
	}
	if err := dnsserver.RegisterDomain(t.Domain, listenPort); err != nil {
		return fmt.Errorf("failed to register domain: %w", err)
	}

	// Create reverse proxy
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			targetURL := fmt.Sprintf("http://localhost:%d", t.Port)
			target, _ := url.Parse(targetURL)
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host
		},
	}

	// Create the listener before the server
	var err error
	var baseListener net.Listener

	// Create listener with reuse options
	config := &net.ListenConfig{
		Control: setSocketOptions,
	}

	// Create server first with proper configuration
	t.server = &http.Server{
		Handler: proxy,
	}

	// Initialize done channel
	t.done = make(chan struct{})

	// Bind listener: proxy mode uses HTTP (proxy handles TLS); direct mode handles TLS itself
	if m.useProxy {
		// Proxy handles TLS — tunnel uses HTTP internally
		baseListener, err = config.Listen(context.Background(), "tcp", fmt.Sprintf("0.0.0.0:%d", t.HTTPPort))
		if err != nil {
			return fmt.Errorf("failed to create tunnel listener: %w", err)
		}
		t.listener = baseListener
	} else if t.HTTPS {
		// Direct mode — tunnel handles its own TLS
		baseListener, err = config.Listen(context.Background(), "tcp", fmt.Sprintf("0.0.0.0:%d", t.HTTPSPort))
		if err != nil {
			return fmt.Errorf("failed to create HTTPS listener: %w", err)
		}

		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{*t.Cert},
			MinVersion:   tls.VersionTLS12,
			ServerName:   t.Domain,
			ClientAuth:   tls.NoClientCert,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			},
			NextProtos: []string{"h2", "http/1.1"},
		}

		t.listener = tls.NewListener(baseListener, tlsConfig)
	} else {
		baseListener, err = config.Listen(context.Background(), "tcp", fmt.Sprintf("0.0.0.0:%d", t.HTTPPort))
		if err != nil {
			return fmt.Errorf("failed to create HTTP listener: %w", err)
		}
		t.listener = baseListener
	}

	// Start server in goroutine with proper error handling
	serverErrChan := make(chan error, 1)
	go func() {
		if err := t.server.Serve(t.listener); err != nil && err != http.ErrServerClosed {
			log.Printf("Server error: %v", err)
			serverErrChan <- err
		}
		close(serverErrChan)
	}()

	// Wait a short time to catch immediate startup errors
	select {
	case err := <-serverErrChan:
		if err != nil {
			return fmt.Errorf("server startup error: %w", err)
		}
	case <-time.After(100 * time.Millisecond):
		// Server started successfully
	}

	return nil
}

func (m *Manager) StopAll(ctx context.Context) error {
	m.mu.Lock()
	domains := make([]string, 0, len(m.tunnels))
	for d := range m.tunnels {
		domains = append(domains, d)
	}
	m.mu.Unlock()

	for _, d := range domains {
		if err := m.StopTunnel(ctx, d); err != nil {
			return fmt.Errorf("error stopping tunnel %s: %w", d, err)
		}
	}

	return nil
}

func (m *Manager) Close(ctx context.Context) error {
	if err := m.StopAll(ctx); err != nil {
		return fmt.Errorf("failed to stop all tunnels: %w", err)
	}

	// Shutdown DNS server when closing manager
	if err := dnsserver.Shutdown(); err != nil {
		log.Printf("Warning: Failed to shutdown DNS server: %v", err)
	}

	return nil
}

func (m *Manager) SetHostsBackupDir(dir string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hostsBackup = dir
}

// CleanupStaleDomain removes orphaned /etc/hosts and mDNS entries for a domain
// whose owning process is no longer running. Safe to call even if nothing is
// registered. Does not modify the in-memory tunnels map.
func (m *Manager) CleanupStaleDomain(domain string) {
	if !strings.HasSuffix(domain, ".local") {
		domain = domain + ".local"
	}
	if err := removeFromHostsFile(domain); err != nil {
		log.Printf("Warning: Failed to clean stale hosts entry for %s: %v", domain, err)
	}
	if err := dnsserver.UnregisterDomain(domain); err != nil {
		log.Printf("Warning: Failed to clean stale mDNS entry for %s: %v", domain, err)
	}
}

// updateHostsFile adds or updates an entry in /etc/hosts
func updateHostsFile(domain string) error {

	// Read current hosts file
	content, err := os.ReadFile(hostsFile)
	if err != nil {
		return fmt.Errorf("failed to read hosts file: %w", err)
	}

	// Check if entry already exists
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, domain) {
			// Entry already exists
			return nil
		}
	}

	// Add new entry
	entry := fmt.Sprintf("\n127.0.0.1\t%s\n", domain)
	if err := os.WriteFile(hostsFile, []byte(string(content)+entry), 0644); err != nil {
		return fmt.Errorf("failed to update hosts file: %w", err)
	}

	return nil
}

// removeFromHostsFile removes an entry from /etc/hosts
func removeFromHostsFile(domain string) error {

	// Read current hosts file
	content, err := os.ReadFile(hostsFile)
	if err != nil {
		return fmt.Errorf("failed to read hosts file: %w", err)
	}

	var newLines []string
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, domain) {
			newLines = append(newLines, line)
		}
	}

	// Write back the file without the domain
	if err := os.WriteFile(hostsFile, []byte(strings.Join(newLines, "\n")+"\n"), 0644); err != nil {
		return fmt.Errorf("failed to update hosts file: %w", err)
	}

	return nil
}

