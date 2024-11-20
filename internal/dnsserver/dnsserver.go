package dnsserver

import (
	"fmt"
	"log"
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
	ifaces, err := net.Interfaces()
	if err != nil {
		log.Printf("Warning: Failed to get network interfaces: %v", err)
		return net.ParseIP("127.0.0.1")
	}

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
	// log.Printf("Using hostname: %s", host)

	// Remove .local suffix if present for service name
	serviceName := strings.TrimSuffix(domain, ".local")
	// log.Printf("Service name: %s", serviceName)

	// Get the machine's network IP
	ip := GetOutboundIP()
	// log.Printf("Advertising service on IP: %s", ip.String())

	// Configure mDNS service
	service, err := mdns.NewMDNSService(
		serviceName,   // Instance name
		"_https._tcp", // Service
		"",            // Domain (empty for .local)
		host,          // Host name
		port,          // Port
		[]net.IP{ip},  // Use the network IP instead of localhost
		[]string{
			"version=1",
			fmt.Sprintf("ip=%s", ip.String()),
			fmt.Sprintf("port=%d", port),
		}, // TXT records with more info
	)
	if err != nil {
		// return fmt.Errorf("failed to create mDNS service: %w", err)
	}

	// Create the mDNS server with more debugging
	config := &mdns.Config{
		Zone:              service,
		LogEmptyResponses: true,
	}

	server, err := mdns.NewServer(config)
	if err != nil {
		return fmt.Errorf("failed to start mDNS server: %w", err)
	}

	// Store the entry
	globalServer.entries[domain] = &ServiceEntry{
		domain: domain,
		port:   port,
		server: server,
	}

	// log.Printf("Successfully registered domain %s with mDNS on port %d", domain, port)
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
