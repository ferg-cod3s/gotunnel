package tunnel

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/johncferguson/gotunnel/internal/cert"
	"github.com/johncferguson/gotunnel/internal/dnsserver"
)

const (
	hostsFile = "/etc/hosts"
)

type Tunnel struct {
	Port      int
	HTTPSPort int
	Domain    string
	TargetIP  string
	HTTPS     bool
	server    *http.Server
	listener  net.Listener
	done      chan struct{}
	Cert      *tls.Certificate
}

type Manager struct {
	tunnels     map[string]*Tunnel
	mu          sync.RWMutex
	certManager *cert.CertManager
	hostsBackup string
}

func NewManager() *Manager {
	return &Manager{
		tunnels:     make(map[string]*Tunnel),
		certManager: cert.New("./certs"),
		hostsBackup: filepath.Join(os.TempDir(), fmt.Sprintf("hosts.backup.%d", time.Now().Unix())),
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

	log.Printf("Created hosts file backup at: %s", m.hostsBackup)
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

	log.Printf("Restored hosts file from backup: %s", m.hostsBackup)

	// Clean up backup file
	if err := os.Remove(m.hostsBackup); err != nil {
		log.Printf("Warning: Failed to remove backup file: %v", err)
	}

	return nil
}

func (m *Manager) StartTunnel(ctx context.Context, port int, domain string, https bool, httpsPort int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	log.Printf("Attempting to start tunnel for domain: %s on port: %d (HTTPS: %v)", domain, port, https)

	// Prevent duplicate tunnels for the same domain
	if _, exists := m.tunnels[domain]; exists {
		log.Printf("Tunnel for domain %s already exists", domain)
		return fmt.Errorf("tunnel for domain %s already exists", domain)
	}

	// Convert domain to .local if not already
	if !strings.HasSuffix(domain, ".local") {
		domain = domain + ".local"
	}

	// Create new tunnel instance
	tunnel := &Tunnel{
		Port:      port,
		HTTPSPort: httpsPort,
		Domain:    domain,
		TargetIP:  "127.0.0.1",
		HTTPS:     https,
		done:      make(chan struct{}), // Channel for cleanup signaling
	}

	// Ensure the SSL/TLS certificate is available
	if https {
		log.Printf("Ensuring certificate for domain: %s", domain)
		cert, err := m.certManager.EnsureCert(domain)
		if err != nil {
			log.Printf("Failed to ensure certificate for domain %s: %v", domain, err)
			return fmt.Errorf("failed to ensure certificate: %w", err)
		}
		tunnel.Cert = cert
	}

	log.Printf("Starting tunnel for domain: %s", domain)
	if err := m.startTunnel(tunnel); err != nil {
		return fmt.Errorf("failed to start tunnel: %w", err)
	}

	// Register the domain with mDNS
	servicePort := port
	if https {
		servicePort = httpsPort
	}
	if err := dnsserver.RegisterDomain(domain, servicePort); err != nil {
		// Stop the tunnel we just started since mDNS registration failed
		if stopErr := tunnel.stop(ctx); stopErr != nil {
			log.Printf("Warning: Failed to stop tunnel after mDNS registration failed: %v", stopErr)
		}
		return fmt.Errorf("failed to register domain with mDNS: %w", err)
	}

	// Add to internal map for tracking
	m.tunnels[domain] = tunnel

	// Create hosts file backup before first modification
	if len(m.tunnels) == 1 {
		if err := m.backupHostsFile(); err != nil {
			log.Printf("Warning: Failed to backup hosts file: %v", err)
		}
	}

	log.Printf("Started tunnel: %s -> localhost:%d (HTTPS: %v, HTTPS Port: %d)",
		domain, port, https, httpsPort)
	return nil
}

func (m *Manager) Stop(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	log.Println("Stopping all tunnels")
	var errs []error
	// Stop all tunnels
	for domain, tunnel := range m.tunnels {
		log.Printf("Stopping tunnel for domain: %s", domain)
		if err := tunnel.stop(ctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to stop tunnel %s: %w", domain, err))
		}
	}

	// Clear the tunnels map
	m.tunnels = make(map[string]*Tunnel)

	// Restore hosts file from backup
	if err := m.restoreHostsFile(); err != nil {
		errs = append(errs, fmt.Errorf("failed to restore hosts file: %w", err))
	}

	// If there were any errors, return them combined
	if len(errs) > 0 {
		log.Printf("Errors occurred while stopping tunnels: %v", errs)
		return fmt.Errorf("errors during shutdown: %v", errs)
	}

	return nil
}

func (m *Manager) StopTunnel(ctx context.Context, domain string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	tunnel, exists := m.tunnels[domain]
	if !exists {
		log.Printf("Tunnel for domain %s does not exist", domain)
		return fmt.Errorf("tunnel for domain %s does not exist", domain)
	}

	// Stop the tunnel
	if err := tunnel.stop(ctx); err != nil {
		log.Printf("Failed to stop tunnel for domain %s: %v", domain, err)
		return fmt.Errorf("failed to stop tunnel: %w", err)
	}

	// Unregister from mDNS
	if err := dnsserver.UnregisterDomain(domain); err != nil {
		log.Printf("Warning: Failed to unregister domain from mDNS: %v", err)
	}

	// Remove from tunnels map
	delete(m.tunnels, domain)
	log.Printf("Stopped tunnel: %s", domain)
	return nil
}

func (t *Tunnel) stop(ctx context.Context) error {
	if t.server != nil {
		if err := t.server.Shutdown(ctx); err != nil {
			log.Printf("Warning: error shutting down server: %v", err)
		}
	}

	if t.listener != nil {
		// Force close the listener to unblock any pending accepts
		if err := t.listener.Close(); err != nil {
			log.Printf("Warning: error closing listener: %v", err)
		}
		t.listener = nil
	}

	// Remove from hosts file
	if err := removeFromHostsFile(t.Domain); err != nil {
		log.Printf("Warning: Failed to remove from hosts file: %v", err)
	}

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

func handleConnection(ctx context.Context, clientConn net.Conn, tunnel *Tunnel) {
	defer clientConn.Close()
	log.Println("Handling new client connection")

	// Connect to the local application (with a timeout)
	dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	localConn, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(dialCtx, "tcp", fmt.Sprintf("localhost:%d", tunnel.Port))
	if err != nil {
		log.Println("Error connecting to local application:", err)
		return
	}
	defer localConn.Close()

	// Forward traffic (using the context for cancellation)
	go func() {
		// Use io.Copy with a context-aware mechanism:
		if _, err := io.Copy(localConn, clientConn); err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("Error copying from client to local app: %v", err)
		}
	}()

	if _, err := io.Copy(clientConn, localConn); err != nil && !errors.Is(err, context.Canceled) {
		log.Printf("Error copying from local app to client: %v", err)
	}
}

func (m *Manager) startTunnel(t *Tunnel) error {
	log.Printf("Starting tunnel for domain: %s", t.Domain)

	// Get the machine's network IP for the proxy
	ip := dnsserver.GetOutboundIP()
	t.TargetIP = ip.String()
	log.Printf("Using target IP for proxy: %s", t.TargetIP)

	// Update /etc/hosts file
	if err := updateHostsFile(t.Domain); err != nil {
		log.Printf("Warning: Failed to update hosts file: %v", err)
	}

	// Create reverse proxy
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			targetURL := fmt.Sprintf("http://%s:%d", t.TargetIP, t.Port)
			target, _ := url.Parse(targetURL)
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host
		},
	}

	t.server = &http.Server{
		Handler: proxy,
	}

	var err error

	if t.HTTPS {
		log.Printf("Starting HTTPS tunnel for domain %s on port %d", t.Domain, t.HTTPSPort)

		// Create TLS config
		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{*t.Cert},
			MinVersion:   tls.VersionTLS12,
			ServerName:   t.Domain,
			ClientAuth:   tls.NoClientCert, // Changed from RequestClientCert since we don't need client certs
			CipherSuites: []uint16{
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			},
			PreferServerCipherSuites: true,
			NextProtos:               []string{"h2", "http/1.1"}, // Added HTTP/2 support
		}

		// Create base TCP listener with reuse options
		config := &net.ListenConfig{
			Control: func(network, address string, c syscall.RawConn) error {
				var opErr error
				if err := c.Control(func(fd uintptr) {
					opErr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
				}); err != nil {
					return err
				}
				return opErr
			},
		}

		// Explicitly bind to all interfaces
		baseListener, err := config.Listen(context.Background(), "tcp", fmt.Sprintf("0.0.0.0:%d", t.HTTPSPort))
		if err != nil {
			return fmt.Errorf("failed to create base listener: %w", err)
		}

		// Wrap with TLS
		t.listener = tls.NewListener(baseListener, tlsConfig)
	} else {
		// Create regular HTTP listener on configured port with reuse options
		config := &net.ListenConfig{
			Control: func(network, address string, c syscall.RawConn) error {
				var opErr error
				if err := c.Control(func(fd uintptr) {
					opErr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
				}); err != nil {
					return err
				}
				return opErr
			},
		}

		// Explicitly bind to all interfaces
		t.listener, err = config.Listen(context.Background(), "tcp", fmt.Sprintf("0.0.0.0:%d", t.Port))
		if err != nil {
			return fmt.Errorf("failed to create listener: %w", err)
		}
	}

	// Start server in goroutine
	go func() {
		if err := t.server.Serve(t.listener); err != http.ErrServerClosed {
			log.Printf("Server error: %v", err)
		}
	}()

	return nil
}

func (m *Manager) StopAll(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	log.Printf("Stopping all tunnels...")

	// Stop each tunnel
	for domain, _ := range m.tunnels {
		if err := m.StopTunnel(ctx, domain); err != nil {
			log.Printf("Error stopping tunnel %s: %v", domain, err)
		}
	}

	// Clear the tunnels map
	m.tunnels = make(map[string]*Tunnel)

	return nil
}

func (m *Manager) Close(ctx context.Context) error {
	return m.StopAll(ctx)
}

// updateHostsFile adds or updates an entry in /etc/hosts
func updateHostsFile(domain string) error {
	hostsFile := "/etc/hosts"

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
	hostsFile := "/etc/hosts"

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

// resolveHostname resolves a hostname, using the system DNS for .local domains
func resolveHostname(hostname string) (string, error) {
	if strings.HasSuffix(hostname, ".local") {
		// Resolve using system DNS for .local domains
		ips, err := net.LookupHost(hostname)
		if err != nil {
			return "", fmt.Errorf("failed to resolve hostname: %w", err)
		}
		if len(ips) > 0 {
			return ips[0], nil // Return the first IP address
		}
		return "", fmt.Errorf("hostname not found in system DNS")
	} else {
		// Resolve using system DNS for other domains
		ips, err := net.LookupHost(hostname)
		if err != nil {
			return "", fmt.Errorf("failed to resolve hostname: %w", err)
		}
		if len(ips) > 0 {
			return ips[0], nil // Return the first IP address
		}
		return "", fmt.Errorf("hostname not found in system DNS")
	}
}
