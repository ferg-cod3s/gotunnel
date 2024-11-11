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

	name := domain
	if len(name) > 6 && name[len(name)-6:] == ".local" {
		name = name[:len(name)-6]
	}

	log.Printf("Registering domain: %s", name)
	if _, exists := s.services[name]; exists {
		log.Printf("Service for domain %s is already registered, unregistering it first", name)
		if err := s.UnregisterDomain(name); err != nil {
			return fmt.Errorf("failed to unregister existing service: %w", err)
		}
	}

	server, err := zeroconf.Register(
		name,
		"_http._tcp",
		"local.",
		443,
		[]string{"path=/"},
		nil,
	)
	if err != nil {
		log.Printf("Failed to register mDNS service for domain %s: %v", name, err)
		return fmt.Errorf("failed to register mDNS service: %w", err)
	}

	s.services[name] = server
	log.Printf("Registered mDNS service: %s.local", name)
	return nil
}

func (s *MDNSServer) UnregisterDomain(domain string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	log.Printf("Unregistering domain: %s", domain)
	if server, exists := s.services[domain]; exists {
		server.Shutdown()
		delete(s.services, domain)
		log.Printf("Unregistered mDNS service: %s", domain)
	}
	return nil
}

func (s *MDNSServer) DiscoverServices() {
	log.Println("Discovering services...")
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
		log.Printf("Failed to browse HTTP services: %v", err)
	}
	time.Sleep(time.Second * 1)
	log.Println("Done discovering services")
}
