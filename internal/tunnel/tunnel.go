package tunnel

import (
	"fmt"
	"io"
	"log"
	"net"
	"sync"
)

type Tunnel struct {
	Port   int
	Domain string
	HTTPS  bool

	listener net.Listener
	done     chan struct{}
}

var (
	activeTunnels = make(map[string]*Tunnel)
	tunnelMutex   sync.RWMutex
)

func New(port int, domain string, https bool) *Tunnel {
	return &Tunnel{
		Port:   port,
		Domain: domain,
		HTTPS:  https,
		done:   make(chan struct{}),
	}
}

func (t *Tunnel) Start() error {
	// Register tunnel
	tunnelMutex.Lock()
	activeTunnels[t.Domain] = t
	tunnelMutex.Unlock()

	// Start the listener for the tunnel
	var err error
	addr := fmt.Sprintf(":%d", t.HTTPS && 443 || 80)
	t.listener, err = net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to start listener: %w", err)
	}

	log.Printf("Started tunnel at %s -> localhost:%d", t.Domain, t.Port)

	// handle connections
	go t.handleConnections()

	// Wait for shutdown
	<-t.done
	return nil
}

func (t *Tunnel) handleConnections() {
	for {
		conn, err := t.listener.Accept()
		if err != nil {
			select {
			case <-t.done:
				return
			default:
				log.Printf("Error accepting connection: %v", err)
				continue
			}
		}

		go t.handleConnection(conn)
	}
}

func (t *Tunnel) handleConnection(clientConn net.Conn) {
	defer clientConn.Close()

	// Connect to local service
	localConn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", t.Port))
	if err != nil {
		log.Printf("Failed to connect to local service: %v", err)
		return
	}
	defer localConn.Close()

	// Bidirectional copy
	go func() {
		io.Copy(localConn, clientConn)
	}()
	io.Copy(clientConn, localConn)
}

func List() []*Tunnel {
	tunnelMutex.RLock()
	defer tunnelMutex.RUnlock()

	tunnels := make([]*Tunnel, 0, len(activeTunnels))

	for _, t := range activeTunnels {
		tunnels = append(tunnels, t)
	}
	return tunnels
}
