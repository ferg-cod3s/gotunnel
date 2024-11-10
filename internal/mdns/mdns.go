package mdns

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

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

	// Check if the domain is already registered
	if _, exists := s.services[domain]; exists {
		log.Printf("Service for domain %s is already registered", domain)
		return nil // or return an error if you want to enforce uniqueness
	}

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

func (s *MDNSServer) DiscoverServices() {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		log.Printf("Failed to create resolver: %v", err)
		return
	}

	entries := make(chan *zeroconf.ServiceEntry)
	go func() {
		for entry := range entries {
			log.Printf("Found service: %s.%s.%s:%d",
				entry.Instance,
				entry.Service,
				entry.Domain,
				entry.Port)
		}
	}()

	ctx := context.Background()
	err = resolver.Browse(ctx, "_http._tcp", "local.", entries)
	if err != nil {
		log.Printf("Failed to browse: %v", err)
	}
	err = resolver.Browse(ctx, "_https._tcp", "local.", entries)
	if err != nil {
		log.Printf("Failed to browse: %v", err)
	}
	// Wait a bit to collect responses
	time.Sleep(time.Second * 1)
}
