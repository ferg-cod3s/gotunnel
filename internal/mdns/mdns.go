package mdns

import (
	"fmt"
	"log"
	"sync"

	"github.com/grandcat/zeroconf"
)

type MDNSServer struct {
	services map[string]*zeroconf.Server
	mu       sync.RWMutex
}

func New() *MDNSServer {
	return &MDNSServer{
		services: make(map[string]*zeroconf.Server),
	}
}

func (s *MDNSServer) RegisterDomain(domain string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove .local suffix if present
	name := domain
	if len(name) > 6 && name[len(name)-6:] == ".local" {
		name = name[:len(name)-6]
	}

	server, err := zeroconf.Register(
		name,
		"_https._tcp",
		"local.",
		443, // Default HTTP port
		[]string{"path=/"},
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to register mDNS service: %w", err)
	}

	s.services[domain] = server
	log.Printf("Registered mDNS service: %s.local", name)
	return nil
}

func (s *MDNSServer) UnregisterDomain(domain string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if server, exists := s.services[domain]; exists {
		server.Shutdown()
		delete(s.services, domain)
		log.Printf("Unregistered mDNS service: %s", domain)
	}
	return nil
}

func (s *MDNSServer) Start() error {
	// mDNS doesn't need explicit start
	return nil
}

func (s *MDNSServer) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for domain, server := range s.services {
		server.Shutdown()
		delete(s.services, domain)
	}
	return nil
}
