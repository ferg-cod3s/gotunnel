package relay

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin:     func(r *http.Request) bool { return true },
	ReadBufferSize:  32768,
	WriteBufferSize: 32768,
}

// tunnel represents a connected tunnel client.
type tunnel struct {
	conn    *websocket.Conn
	pending sync.Map // request ID → chan Response
}

// Server is the relay server that routes browser traffic to tunnel clients.
type Server struct {
	domain  string
	mu      sync.RWMutex
	tunnels map[string]*tunnel
}

func NewServer(domain string) *Server {
	return &Server{
		domain:  domain,
		tunnels: make(map[string]*tunnel),
	}
}

// HandleTunnel is the WebSocket endpoint that tunnel clients connect to.
func (s *Server) HandleTunnel(w http.ResponseWriter, r *http.Request) {
	subdomain := r.URL.Query().Get("domain")
	if subdomain == "" {
		http.Error(w, "domain query param required", http.StatusBadRequest)
		return
	}
	subdomain = strings.ToLower(subdomain)

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	t := &tunnel{conn: conn}

	s.mu.Lock()
	s.tunnels[subdomain] = t
	s.mu.Unlock()

	log.Printf("tunnel connected: %s", subdomain)

	// Read loop: dispatch responses to waiting handlers
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var resp Response
		if err := json.Unmarshal(data, &resp); err != nil {
			continue
		}
		if ch, ok := t.pending.LoadAndDelete(resp.ID); ok {
			ch.(chan Response) <- resp
		}
	}

	s.mu.Lock()
	delete(s.tunnels, subdomain)
	s.mu.Unlock()
	log.Printf("tunnel disconnected: %s", subdomain)
}

// HandleHTTP routes browser requests to the appropriate tunnel.
func (s *Server) HandleHTTP(w http.ResponseWriter, r *http.Request) {
	subdomain := s.subdomainFromRequest(r)

	s.mu.RLock()
	t, ok := s.tunnels[subdomain]
	s.mu.RUnlock()

	if !ok {
		http.Error(w, "no tunnel for "+subdomain, http.StatusBadGateway)
		return
	}

	// Read request body
	body, _ := io.ReadAll(r.Body)
	r.Body.Close()

	reqID := uuid.New().String()
	req := Request{
		ID:      reqID,
		Method:  r.Method,
		Path:    r.URL.RequestURI(),
		Headers: flattenHeader(r.Header),
		BodyB64: base64.StdEncoding.EncodeToString(body),
	}

	// Register response channel
	respCh := make(chan Response, 1)
	t.pending.Store(reqID, respCh)
	defer t.pending.Delete(reqID)

	// Send request to tunnel client
	data, _ := json.Marshal(req)
	if err := t.conn.WriteMessage(websocket.TextMessage, data); err != nil {
		http.Error(w, "tunnel write failed", http.StatusBadGateway)
		return
	}

	// Wait for response
	select {
	case resp := <-respCh:
		for k, v := range resp.Headers {
			w.Header().Set(k, v)
		}
		w.WriteHeader(resp.Status)
		if resp.BodyB64 != "" {
			body, _ := base64.StdEncoding.DecodeString(resp.BodyB64)
			w.Write(body)
		}
	case <-time.After(30 * time.Second):
		http.Error(w, "tunnel timeout", http.StatusGatewayTimeout)
	}
}

func (s *Server) extractSubdomain(host string) string {
	host = strings.Split(host, ":")[0]
	host = strings.ToLower(host)
	if s.domain != "" {
		return strings.TrimSuffix(host, "."+s.domain)
	}
	return host
}

// subdomainFromRequest extracts the tunnel subdomain from an HTTP request,
// checking X-Forwarded-Host first (set by Coolify/Traefik/nginx proxies).
func (s *Server) subdomainFromRequest(r *http.Request) string {
	host := r.Host
	if xfh := r.Header.Get("X-Forwarded-Host"); xfh != "" {
		host = xfh
	}
	return s.extractSubdomain(host)
}

// TunnelList returns info about connected tunnels for monitoring.
type TunnelInfo struct {
	Domain string `json:"domain"`
}

func (s *Server) HandleTunnelsList(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	infos := make([]TunnelInfo, 0, len(s.tunnels))
	for d := range s.tunnels {
		infos = append(infos, TunnelInfo{Domain: d})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"count":   len(infos),
		"tunnels": infos,
	})
}

func flattenHeader(h http.Header) map[string]string {
	flat := make(map[string]string)
	for k := range h {
		flat[k] = h.Get(k)
	}
	return flat
}
