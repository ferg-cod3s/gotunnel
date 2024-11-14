package tunnel

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"sync"
	"time"

	"github.com/johncferguson/gotunnel/internal/cert"
	"github.com/johncferguson/gotunnel/internal/mdns"
	"github.com/miekg/dns"
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
	mdns        *mdns.MDNSServer
	certManager *cert.CertManager
}

func NewManager() *Manager {
	m := &Manager{
		tunnels:     make(map[string]*Tunnel),
		mdns:        mdns.New(),
		certManager: cert.New("./certs"),
	}

	log.Println("Initializing Manager and loading existing tunnels")
	// Load existing tunnels
	// Verify mDNS registration by discovering services
	m.mdns.DiscoverServices()
	log.Println("Discovered mDNS services")

	// states, err := state.LoadTunnels()
	// if err != nil {
	// 	log.Printf("Error loading tunnel state: %v", err)
	// 	return m
	// }

	// Start existing tunnels
	// for _, t := range states {
	// 	if err := m.StartTunnel(t.Port, t.Domain, t.HTTPS); err != nil {
	// 		log.Printf("Error restoring tunnel %s: %v", t.Domain, err)
	// 	}
	// }

	return m
}

func (m *Manager) StartTunnel(ctx context.Context, port int, domain string, https bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	log.Printf("Attempting to start tunnel for domain: %s on port: %d (HTTPS: %v)", domain, port, https)

	// Prevent duplicate tunnels for the same domain
	if _, exists := m.tunnels[domain]; exists {
		log.Printf("Tunnel for domain %s already exists", domain)
		return fmt.Errorf("tunnel for domain %s already exists", domain)
	}

	domain = strings.TrimPrefix(strings.TrimSuffix(strings.ToLower(domain), ".go"), ".") + ".go"

	log.Println("registering domain")
	// Register the domain with mDNS
	ip, err := resolveHostname(domain)
	if err != nil {
		return fmt.Errorf("failed to resolve hostname %s: %v", domain, err)
	}

	// Create new tunnel instance
	tunnel := &Tunnel{
		Port:     port,
		Domain:   domain,
		TargetIP: ip,
		HTTPS:    https,
		done:     make(chan struct{}), // Channel for cleanup signaling
	}

	ctx, cancel := context.WithTimeout(ctx, 10 * time.Second)
    defer cancel()
	// Ensure the SSL/TLS certificate is available
	if https {
		log.Printf("Ensuring certificate for domain: %s.go", domain)
		cert, err := m.certManager.EnsureCert(domain + ".go")
		if err != nil {
			log.Printf("Failed to ensure certificate for domain %s: %v", domain, err)
			return fmt.Errorf("failed to ensure certificate: %w", err)
		}
		tunnel.Cert = cert
		tunnel.HTTPSPort = 8443

		listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.ParseIP("0.0.0.0"), Port: tunnel.HTTPSPort})
		if err != nil {
			return fmt.Errorf("failed to create listener: %w", err)
		}

		// Wrap the listener with TLS
		tlsListener := tls.NewListener(listener, &tls.Config{
			Certificates: []tls.Certificate{*tunnel.Cert},
		})

		// set the tunnel listeners
		tunnel.listener = tlsListener

		// Start accepting connections in a goroutine
		go func(ctx context.Context) {
			log.Println("accepting connections now")
			for {
				conn, err := tlsListener.Accept()
				if err != nil {
					log.Println("Error accepting connection:", err)
					continue
				}
				go handleConnection(ctx, conn, tunnel)
			}
		}(ctx)
		log.Println("TLS listener created successfully")
	}

	tunnel.server = &http.Server{
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{*tunnel.Cert}, // Load certificate
		},
	}
	// Start the HTTP server and set up the proxy
	if err := m.startTunnel(tunnel); err != nil {
		return fmt.Errorf("failed to start tunnel: %w", err)
	}

	// Add to internal map for tracking
	m.tunnels[domain] = tunnel

	// Persist tunnel configuration to disk
	// if err := m.saveTunnelState(); err != nil {
	// 	log.Printf("Error saving tunnel state: %v", err)
	// }

	log.Printf("Started tunnel: %s.local -> localhost:%d (HTTPS: %v)",
		domain, port, https)
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
		// Unregister from mDNS
		if err := m.mdns.UnregisterDomain(domain); err != nil {
			errs = append(errs, fmt.Errorf("failed to unregister mDNS for %s: %w", domain, err))
		}
	}

	// Clear the tunnels map
	m.tunnels = make(map[string]*Tunnel)

	// Save empty state
	// if err := m.saveTunnelState(); err != nil {
	// 	log.Printf("Error saving tunnel state: %v", err)
	// }

	// If there were any errors, return them combined
	if len(errs) > 0 {
		log.Printf("Errors occurred while stopping tunnels: %v", errs)
	}

	return nil
}

func (m *Manager) startTunnel(t *Tunnel) error {
	log.Printf("Starting tunnel for domain: %s", t.Domain)
	// Create reverse proxy
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "https"
			req.URL.Host = fmt.Sprintf("localhost:%d", t.Port)
		},
	}

	// Create server
	t.server = &http.Server{
		Handler: proxy,
	}

	// Create listener
	var err error
	if t.HTTPS {
		log.Printf("Starting HTTPS tunnel for domain %s on port 443", t.Domain)
		// Generate or load certificate
		cert, err := m.certManager.EnsureCert(t.Domain + ".local")
		if err != nil {
			return fmt.Errorf("failed to ensure certificate: %w", err)
		}

		// Create TLS config
		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{*cert},
		}

		// Create TLS listener
		t.listener, err = tls.Listen("tcp", ":443", tlsConfig)
		if err != nil {
			return fmt.Errorf("failed to create TLS listener: %w", err)
		}
	} else {
		// Create regular HTTP listener
		t.listener, err = net.Listen("tcp", ":80")
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

func (m *Manager) StopTunnel(ctx context.Context, domain string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	log.Printf("Attempting to stop tunnel for domain: %s", domain)
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
	if err := m.mdns.UnregisterDomain(domain); err != nil {
		log.Printf("Failed to unregister mDNS for domain %s: %v", domain, err)
		return fmt.Errorf("failed to unregister mDNS: %w", err)
	}

	// Remove from tunnels map
	delete(m.tunnels, domain)
	log.Printf("Stopped tunnel: %s", domain)
	// if err := m.saveTunnelState(); err != nil {
	// 	log.Printf("Error saving tunnel state: %v", err)
	// }
	return nil
}

func (t *Tunnel) stop(ctx context.Context) error {
	log.Printf("Stopping tunnel for domain: %s", t.Domain)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if t.server != nil {
		if err := t.server.Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to shutdown server: %w", err)
		}
	}

	if t.listener != nil {
		if err := t.listener.Close(); err != nil {
			if ne, ok := err.(*net.OpError); ok && ne.Op == "close" {
				log.Printf("Listener for domain %s is already closed", t.Domain)
			} else {
				return fmt.Errorf("failed to close listener: %w", err)
			}
		}
	}

	// Ensure the Done channel is closed after stopping
	close(t.done) // Ensure this is called to signal completion
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

// resolveHostname resolves a hostname, using the custom DNS server for .go domains
func resolveHostname(hostname string) (string, error) {
	if strings.HasSuffix(hostname, ".go") {
		// Resolve using custom DNS server
		m := new(dns.Msg)
		m.SetQuestion(dns.Fqdn(hostname), dns.TypeA)
		in, err := dns.Exchange(m, "127.0.0.1:5353") // Your DNS server address
		if err != nil {
			return "", fmt.Errorf("failed to resolve hostname: %w", err)
		}
		if len(in.Answer) > 0 {
			if a, ok := in.Answer[0].(*dns.A); ok {
				return a.A.String(), nil // Return the IP address
			}
		}
		return "", fmt.Errorf("hostname not found in custom DNS")
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
