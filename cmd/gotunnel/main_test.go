package main

import (
	// "bytes"
	// "context"
	// "fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"unsafe"

	"github.com/johncferguson/gotunnel/internal/tunnel" // Import your tunnel package
	"github.com/urfave/cli/v2"
)

// Mock tunnel manager for testing
type MockTunnelManager struct {
	tunnels map[string]map[string]interface{}
	started bool
	stopped bool
	errors  []error
}

func (m *MockTunnelManager) StartTunnel(port int, domain string, https bool) error {
	if len(m.errors) > 0 {
		err := m.errors[0]
		m.errors = m.errors[1:]
		return err
	}
	m.tunnels[domain] = map[string]interface{}{
		"domain": domain,
		"port":   port,
		"https":  https,
	}
	m.started = true
	return nil
}

func (m *MockTunnelManager) StopTunnel(domain string) error {
	if len(m.errors) > 0 {
		err := m.errors[0]
		m.errors = m.errors[1:]
		return err
	}
	delete(m.tunnels, domain)
	return nil
}

func (m *MockTunnelManager) Stop() error {
	if len(m.errors) > 0 {
		err := m.errors[0]
		m.errors = m.errors[1:]
		return err
	}

	m.tunnels = make(map[string]map[string]interface{})
	m.stopped = true
	return nil
}

func (m *MockTunnelManager) ListTunnels() []map[string]interface{} {
	var tunnels []map[string]interface{}
	for _, t := range m.tunnels {
		tunnels = append(tunnels, t)
	}
	return tunnels
}

var originalResolveHostname = resolveHostname

func mockResolveHostname(hostname string) (string, error) {
	return "127.0.0.1", nil // Return a mock IP
}

func TestStartTunnelCommand(t *testing.T) {
	mockManager := &MockTunnelManager{tunnels: make(map[string]map[string]interface{})}

	// Correct way to use the mock
	manager = (*tunnel.Manager)(unsafe.Pointer(mockManager)) // Type assertion with unsafe.Pointer

	resolveHostname = mockResolveHostname // Assign the mock to the variable

	defer func() { resolveHostname = originalResolveHostname }()

	app := cli.NewApp()
	app.Commands = []*cli.Command{
		{
			Name:   "start",
			Action: StartTunnel,
			Flags: []cli.Flag{
				&cli.IntFlag{Name: "port", Value: 8000},
				&cli.StringFlag{Name: "domain", Required: true},
				&cli.BoolFlag{Name: "https", Value: true},
			},
		},
	}

	// Redirect standard output to a buffer to capture the output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Run the start command
	err := app.Run([]string{"gotunnel", "start", "--domain", "example.com"})
	if err != nil {
		t.Fatal(err)
	}

	// Restore standard output
	w.Close()
	os.Stdout = oldStdout
	out, _ := io.ReadAll(r)

	if !mockManager.started {
		t.Error("StartTunnel was not called.")
	}

	expectedOutput := "Tunnel started successfully"
	if !strings.Contains(string(out), expectedOutput) {
		t.Errorf("Expected output to contain %q, but got %q", expectedOutput, string(out))
	}

	tunnel, exists := mockManager.tunnels["example.com"]

	if !exists {
		t.Error("Tunnel not found in manager after starting")
	}

	if tunnel["port"].(int) != 8000 {
		t.Error("Invalid port setting for the new tunnel")
	}

	if !tunnel["https"].(bool) {
		t.Error("Invalid https setting for the new tunnel")
	}
}

func TestStopTunnelCommand(t *testing.T) {
	mockManager := &MockTunnelManager{tunnels: make(map[string]map[string]interface{})}
	manager = (*tunnel.Manager)(unsafe.Pointer(mockManager)) // Type assertion with unsafe.Pointer

	mockManager.tunnels["test.com"] = map[string]interface{}{
		"server": &http.Server{}, // Initialize the server in the mock
	}

	app := cli.NewApp()
	app.Commands = []*cli.Command{
		{
			Name:   "stop",
			Action: StopTunnel,
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "domain", Required: true},
			},
		},
	}

	err := app.Run([]string{"gotunnel", "stop", "--domain", "test.com"}) // Change here
	if err != nil {
		t.Error(err)
	}

	if _, exists := mockManager.tunnels["test.com"]; exists {
		t.Error("Tunnel was not stopped correctly")
	}
}
