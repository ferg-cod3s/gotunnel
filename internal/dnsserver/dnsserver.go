package dnsserver

import (
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/hashicorp/mdns"
)

type Server struct {
	mu      sync.RWMutex
	entries map[string]*ServiceEntry
}

type ServiceEntry struct {
	domain string
	ip     net.IP
	port   int
	server *mdns.Server
}

var (
	globalServer *Server
	serverMu     sync.Mutex
)

// StartDNSServer initializes the DNS server
func StartDNSServer() error {
	serverMu.Lock()
	defer serverMu.Unlock()

	if globalServer != nil {
		return nil
	}

	globalServer = &Server{
		entries: make(map[string]*ServiceEntry),
	}

	// log.Printf("mDNS server initialized")
	return nil
}

// getOutboundIP gets the preferred outbound IP of this machine
func GetOutboundIP() net.IP {
	// Create a temporary UDP connection to determine the preferred outbound IP
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return net.ParseIP("127.0.0.1")
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP
}

// RegisterDomain adds a new domain to the DNS server and advertises it via get
func RegisterDomain(domain string, port int) error {
	if globalServer == nil {
		return fmt.Errorf("DNS server not initialized")
	}

	globalServer.mu.Lock()
	defer globalServer.mu.Unlock()

	host := domain

	// Make sure hostname is a proper FQDN
	host = strings.TrimSuffix(host, ".")      // Remove any trailing dot
	host = strings.TrimSuffix(host, ".local") // Remove .local if present
	host = host + ".local."                   // Add .local. to make it a proper FQDN

	// Remove .local suffix if present for service name
	serviceName := strings.TrimSuffix(domain, ".local")

	// Get the machine's network IP
	ip := GetOutboundIP()

	// Determine service type based on port
	serviceType := "_http._tcp"
	if port == 8443 || port > 1024 { // Assume HTTPS for port 8443 or high ports
		serviceType = "_https._tcp"
	}

	// Configure mDNS service
	service, err := mdns.NewMDNSService(
		serviceName,  // Instance name
		serviceType,  // Service type (_http._tcp or _https._tcp)
		"",           // Domain (empty for .local)
		host,         // Host name
		port,         // Port
		[]net.IP{ip}, // Use the network IP instead of localhost
		[]string{
			"version=1",
			fmt.Sprintf("ip=%s", ip.String()),
			fmt.Sprintf("port=%d", port),
		}, // TXT records with more info
	)
	if err != nil {
		return fmt.Errorf("failed to create mDNS service: %w", err)
	}

	// Create the mDNS server
	server, err := mdns.NewServer(&mdns.Config{Zone: service})
	if err != nil {
		return fmt.Errorf("failed to create mDNS server: %w", err)
	}

	// Store the entry
	globalServer.entries[domain] = &ServiceEntry{
		domain: domain,
		ip:     ip,
		port:   port,
		server: server,
	}

	return nil
}

// UnregisterDomain removes a domain from the DNS server
func UnregisterDomain(domain string) error {
	if globalServer == nil {
		// return fmt.Errorf("DNS server not initialized")
	}

	globalServer.mu.Lock()
	defer globalServer.mu.Unlock()

	entry, exists := globalServer.entries[domain]
	if !exists {
		return nil
	}

	// Shutdown the mDNS server for this domain
	if entry.server != nil {
		entry.server.Shutdown()
	}

	delete(globalServer.entries, domain)
	// log.Printf("Unregistered domain %s from mDNS", domain)
	return nil
}

// Shutdown cleans up the DNS server
func Shutdown() error {
	if globalServer == nil {
		return nil
	}

	globalServer.mu.Lock()
	defer globalServer.mu.Unlock()

	// Shutdown all mDNS servers
	for domain, entry := range globalServer.entries {
		if entry.server != nil {
			entry.server.Shutdown()
		}
		delete(globalServer.entries, domain)
	}

	globalServer = nil
	// log.Printf("DNS server shut down")
	return nil
}
