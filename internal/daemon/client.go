package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"
)

// Client talks to the daemon API over a Unix socket.
type Client struct {
	socketPath string
	http       *http.Client
}

func NewClient() *Client {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return net.DialTimeout("unix", SocketPath(), 2*time.Second)
		},
	}
	return &Client{
		socketPath: SocketPath(),
		http:       &http.Client{
			Transport: transport,
			Timeout:   5 * time.Second,
		},
	}
}

// IsRunning checks whether the daemon is reachable.
func (c *Client) IsRunning() bool {
	resp, err := c.http.Get("http://unix/api/health")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}

// Start sends a start-tunnel request to the daemon.
func (c *Client) Start(req StartRequest) (*TunnelInfo, error) {
	body, _ := json.Marshal(req)
	resp, err := c.http.Post("http://unix/api/start", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("daemon not reachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("%s", errResp.Error)
	}

	var info TunnelInfo
	json.NewDecoder(resp.Body).Decode(&info)
	return &info, nil
}

// Stop sends a stop-tunnel request to the daemon.
func (c *Client) Stop(domain string) error {
	body, _ := json.Marshal(map[string]string{"domain": domain})
	resp, err := c.http.Post("http://unix/api/stop", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("daemon not reachable: %w", err)
	}
	resp.Body.Close()
	return nil
}

// StopAll sends a stop-all request to the daemon.
func (c *Client) StopAll() error {
	resp, err := c.http.Post("http://unix/api/stop-all", "application/json", nil)
	if err != nil {
		return fmt.Errorf("daemon not reachable: %w", err)
	}
	resp.Body.Close()
	return nil
}

// List returns the active tunnels from the daemon.
func (c *Client) List() ([]TunnelInfo, error) {
	resp, err := c.http.Get("http://unix/api/tunnels")
	if err != nil {
		return nil, fmt.Errorf("daemon not reachable: %w", err)
	}
	defer resp.Body.Close()

	var tunnels []TunnelInfo
	json.NewDecoder(resp.Body).Decode(&tunnels)
	return tunnels, nil
}
