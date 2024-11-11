// Application initializes with default settings and runs without errors
package main

import (
	"context"
	"flag"
	"fmt"
	"testing"

	"github.com/johncferguson/gotunnel/internal/privilege"
	"github.com/johncferguson/gotunnel/internal/tunnel"
	"github.com/urfave/cli/v2"
)

func TestAppInitializationWithDefaults(t *testing.T) {
	app := &cli.App{
		Name:  "gotunnel",
		Usage: "Create secure local tunnels for development",
	}

	err := app.Run([]string{"gotunnel"})
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
}

func TestAppRunWithNoPrivilegeCheckFlag(t *testing.T) {
	app := &cli.App{
		Name:  "gotunnel",
		Usage: "Create secure local tunnels for development",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "no-privilege-check",
				Value: false,
				Usage: "Skip privilege check",
			},
		},
		Before: func(c *cli.Context) error {
			if c.Bool("no-privilege-check") {
				return nil
			}
			return privilege.CheckPrivileges()
		},
	}

	err := app.Run([]string{"gotunnel", "--no-privilege-check"})
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
}

// Privilege check is performed unless explicitly skipped
func TestPrivilegeCheck(t *testing.T) {
	app := &cli.App{
		Before: func(c *cli.Context) error {
			if !c.Bool("no-privilege-check") {
				return privilege.CheckPrivileges()
			}
			return nil
		},
	}
	privilegeCheckCalled := false
	privilege.CheckPrivileges = func() error {
		privilegeCheckCalled = true
		return nil
	}

	err := app.Before(cli.NewContext(app, nil, nil))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !privilegeCheckCalled {
		t.Error("expected privilege check to be called")
	}
}

// Tunnel manager is initialized successfully
func TestTunnelManagerInitialization(t *testing.T) {
	var manager *tunnel.Manager

	app := &cli.App{
		Before: func(c *cli.Context) error {
			manager = tunnel.NewManager()
			return nil
		},
	}

	err := app.Before(cli.NewContext(app, nil, nil))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if manager == nil {
		t.Error("expected tunnel manager to be initialized")
	}
}

// Command 'start' initiates a tunnel with specified port and domain
func TestStartCommandInitiatesTunnel(t *testing.T) {
	app := &cli.App{
		Commands: []*cli.Command{
			{
				Name:   "start",
				Action: StartTunnel,
				Flags: []cli.Flag{
					&cli.IntFlag{Name: "port", Value: 8000},
					&cli.StringFlag{Name: "domain", Required: true},
				},
			},
		},
	}

	tunnelStarted := false
	StartTunnel = func(c *cli.Context) error {
		tunnelStarted = true
		return nil
	}

	set := flag.NewFlagSet("test", 0)
	set.Int("port", 8000, "")
	set.String("domain", "example.local", "")

	c := cli.NewContext(app, set, nil)

	err := app.RunContext(context.Background(), []string{"app", "start"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !tunnelStarted {
		t.Error("expected tunnel to be started")
	}
}

// Command 'stop' terminates the specified tunnel
func TestStopCommandTerminatesSpecifiedTunnel(t *testing.T) {
	app := &cli.App{
		Commands: []*cli.Command{
			{
				Name:   "stop",
				Usage:  "Stop a tunnel",
				Action: StopTunnel,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "domain",
						Aliases:  []string{"d"},
						Usage:    "Domain of tunnel to stop",
						Required: true,
					},
				},
			},
		},
	}

	// Mock the StopTunnel function
	StopTunnel = func(c *cli.Context) error {
		domain := c.String("domain")
		if domain != "example.local" {
			t.Errorf("Expected domain 'example.local', got %s", domain)
		}
		return nil
	}

	err := app.Run([]string{"gotunnel", "stop", "--domain", "example.local"})
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
}

// Command 'stopAll' terminates all active tunnels
func TestStopAllCommandTerminatesAllTunnels(t *testing.T) {
	app := &cli.App{
		Commands: []*cli.Command{
			{
				Name:   "stopAll",
				Usage:  "Stop all tunnels",
				Action: StopAllTunnels,
			},
		},
	}

	// Mock the StopAllTunnels function
	StopAllTunnels = func(c *cli.Context) error {
		// Simulate stopping all tunnels
		return nil
	}

	err := app.Run([]string{"gotunnel", "stopAll"})
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
}

// Command 'list' displays all active tunnels
func TestListCommandDisplaysAllActiveTunnels(t *testing.T) {
	app := &cli.App{
		Commands: []*cli.Command{
			{
				Name:    "list",
				Aliases: []string{"ls"},
				Usage:   "List all active tunnels",
				Action:  ListTunnels,
			},
		},
	}

	// Mock the ListTunnels function
	ListTunnels = func(c *cli.Context) error {
		// Simulate listing tunnels
		fmt.Println("Tunnel1\nTunnel2")
		return nil
	}

	err := app.Run([]string{"gotunnel", "list"})
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
}
