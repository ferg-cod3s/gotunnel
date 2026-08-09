package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"time"
)

const helperSocketPath = "/tmp/gotunnel-helper.sock"

// HelperClient talks to the privileged helper for /etc/hosts management.
type HelperClient struct {
	http *http.Client
}

func NewHelperClient() *HelperClient {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return net.DialTimeout("unix", helperSocketPath, 1*time.Second)
		},
	}
	return &HelperClient{
		http: &http.Client{Transport: transport, Timeout: 3 * time.Second},
	}
}

// IsRunning checks whether the privileged helper is reachable.
func (c *HelperClient) IsRunning() bool {
	resp, err := c.http.Get("http://helper/hosts/list")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}

// AddHost asks the helper to add a domain to /etc/hosts.
func (c *HelperClient) AddHost(domain string) error {
	body, _ := json.Marshal(map[string]string{"domain": domain})
	resp, err := c.http.Post("http://helper/hosts/add", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// RemoveHost asks the helper to remove a domain from /etc/hosts.
func (c *HelperClient) RemoveHost(domain string) error {
	body, _ := json.Marshal(map[string]string{"domain": domain})
	resp, err := c.http.Post("http://helper/hosts/remove", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
