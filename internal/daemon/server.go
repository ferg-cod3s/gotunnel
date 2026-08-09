package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/v1truv1us/gotunnel/internal/tunnel"
)

func SocketPath() string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("gotunnel-%d.sock", os.Getuid()))
}

type TunnelInfo struct {
	Domain    string `json:"domain"`
	Port      int    `json:"port"`
	URL       string `json:"url"`
	HTTPS     bool   `json:"https"`
	HTTPSPort int    `json:"https_port"`
	HTTPPort  int    `json:"http_port"`
}

type StartRequest struct {
	Port      int    `json:"port"`
	Domain    string `json:"domain"`
	HTTPS     bool   `json:"https"`
	HTTPSPort int    `json:"https_port"`
}

type Server struct {
	manager      *tunnel.Manager
	listener     net.Listener
	server       *http.Server
	mu           sync.Mutex
	tunnels      map[string]*TunnelInfo
	helperActive bool
	helper       *HelperClient
}

// checkHelperActive returns true if the privileged port forwarder is running.
func checkHelperActive() bool {
	conn, err := net.DialTimeout("tcp", "127.0.0.1:80", 200*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// urlFor builds the access URL, omitting the port if the helper is active.
func (s *Server) urlFor(domain string, https bool, httpsPort int) string {
	if s.helperActive {
		if https {
			return fmt.Sprintf("https://%s", domain)
		}
		return fmt.Sprintf("http://%s", domain)
	}
	if https {
		return fmt.Sprintf("https://%s:%d", domain, httpsPort)
	}
	return fmt.Sprintf("http://%s:8080", domain)
}

func NewServer(manager *tunnel.Manager) (*Server, error) {
	socketPath := SocketPath()
	os.MkdirAll(filepath.Dir(socketPath), 0755)
	os.Remove(socketPath)

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s: %w", socketPath, err)
	}
	os.Chmod(socketPath, 0600)

	s := &Server{
		manager:      manager,
		listener:     listener,
		tunnels:      make(map[string]*TunnelInfo),
		helperActive: checkHelperActive(),
		helper:       NewHelperClient(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/tunnels", s.handleTunnels)
	mux.HandleFunc("/api/start", s.handleStart)
	mux.HandleFunc("/api/stop", s.handleStop)
	mux.HandleFunc("/api/stop-all", s.handleStopAll)

	s.server = &http.Server{Handler: mux}
	return s, nil
}

func (s *Server) Start() {
	go s.server.Serve(s.listener)
}

func (s *Server) Stop() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.server.Shutdown(ctx)
	s.listener.Close()
	os.Remove(SocketPath())
}

func (s *Server) Tunnels() []TunnelInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]TunnelInfo, 0, len(s.tunnels))
	for _, t := range s.tunnels {
		result = append(result, *t)
	}
	return result
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleTunnels(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(s.Tunnels())
}

func (s *Server) handleStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req StartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid request: %v", err), http.StatusBadRequest)
		return
	}

	if req.Port <= 0 {
		req.Port = 8080
	}
	if req.HTTPSPort <= 0 {
		req.HTTPSPort = 8443
	}

	domain := req.Domain
	if domain == "" {
		domain = fmt.Sprintf("app-%d", req.Port)
	}
	if !strings.HasSuffix(domain, ".local") {
		domain = domain + ".local"
	}

	ctx := context.Background()
	if err := s.manager.StartTunnel(ctx, req.Port, domain, req.HTTPS, req.HTTPSPort); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Ask helper to add /etc/hosts entry (best-effort)
	if s.helper != nil {
		if err := s.helper.AddHost(domain); err == nil {
			s.mu.Lock()
			s.helperActive = true
			s.mu.Unlock()
		}
	}

	url := s.urlFor(domain, req.HTTPS, req.HTTPSPort)

	info := TunnelInfo{
		Domain:    domain,
		Port:      req.Port,
		URL:       url,
		HTTPS:     req.HTTPS,
		HTTPSPort: req.HTTPSPort,
		HTTPPort:  8080,
	}

	s.mu.Lock()
	s.tunnels[domain] = &info
	s.mu.Unlock()

	json.NewEncoder(w).Encode(info)
}

func (s *Server) handleStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Domain string `json:"domain"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid request: %v", err), http.StatusBadRequest)
		return
	}

	domain := req.Domain
	if !strings.HasSuffix(domain, ".local") {
		domain = domain + ".local"
	}

	ctx := context.Background()
	if err := s.manager.StopTunnel(ctx, domain); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Ask helper to remove /etc/hosts entry
	if s.helper != nil {
		s.helper.RemoveHost(domain)
	}

	s.mu.Lock()
	delete(s.tunnels, domain)
	s.mu.Unlock()

	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleStopAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := context.Background()
	s.manager.Stop(ctx)

	s.mu.Lock()
	s.tunnels = make(map[string]*TunnelInfo)
	s.mu.Unlock()

	w.WriteHeader(http.StatusOK)
}

func (s *Server) StopAllTunnels() {
	ctx := context.Background()

	s.mu.Lock()
	domains := make([]string, 0, len(s.tunnels))
	for d := range s.tunnels {
		domains = append(domains, d)
	}
	s.mu.Unlock()

	for _, d := range domains {
		s.manager.StopTunnel(ctx, d)
		if s.helper != nil {
			s.helper.RemoveHost(d)
		}
	}

	s.mu.Lock()
	s.tunnels = make(map[string]*TunnelInfo)
	s.mu.Unlock()
}

func (s *Server) StopTunnel(domain string) error {
	ctx := context.Background()
	if err := s.manager.StopTunnel(ctx, domain); err != nil {
		return err
	}
	if s.helper != nil {
		s.helper.RemoveHost(domain)
	}
	s.mu.Lock()
	delete(s.tunnels, domain)
	s.mu.Unlock()
	return nil
}
