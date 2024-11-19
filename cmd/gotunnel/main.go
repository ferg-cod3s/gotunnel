package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/johncferguson/gotunnel/internal/dnsserver"
	"github.com/johncferguson/gotunnel/internal/tunnel"
	"github.com/urfave/cli/v2"
)

var manager *tunnel.Manager

func main() {
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
			// Check if we have root privileges
			if os.Geteuid() != 0 {
				return fmt.Errorf("gotunnel requires root privileges to modify hosts file and bind to privileged ports. Please run with sudo")
			}

			// Initialize mDNS server
			if err := dnsserver.StartDNSServer(); err != nil {
				return fmt.Errorf("failed to start mDNS server: %w", err)
			}

			log.Println("Initializing tunnel manager...")
			manager = tunnel.NewManager()

			// Set up cleanup on program exit
			setupCleanup()

			return nil
		},
		Commands: []*cli.Command{
			{
				Name:  "start",
				Usage: "Start a new tunnel",
				Flags: []cli.Flag{
					&cli.IntFlag{
						Name:    "port",
						Aliases: []string{"p"},
						Value:   80,
						Usage:   "Local port to tunnel",
					},
					&cli.StringFlag{
						Name:    "domain",
						Aliases: []string{"d"},
						Usage:   "Domain name for the tunnel (will be suffixed with .local if not provided)",
					},
					&cli.BoolFlag{
						Name:    "https",
						Aliases: []string{"s"},
						Value:   true,
						Usage:   "Enable HTTPS (default: true)",
					},
					&cli.IntFlag{
						Name:  "https-port",
						Value: 443,
						Usage: "HTTPS port (default: 443)",
					},
				},
				Action: StartTunnel,
			},
			{
				Name:      "stop",
				Usage:     "Stop a tunnel",
				ArgsUsage: "[domain]",
				Action:    StopTunnel,
			},
			{
				Name:   "list",
				Usage:  "List active tunnels",
				Action: ListTunnels,
			},
			{
				Name:   "stop-all",
				Usage:  "Stop all tunnels",
				Action: StopAllTunnels,
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func setupCleanup() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		log.Println("\nShutting down...")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := manager.Stop(ctx); err != nil {
			log.Printf("Error during shutdown: %v", err)
			os.Exit(1)
		}

		log.Println("Shutdown complete")
		os.Exit(0)
	}()
}

func StartTunnel(c *cli.Context) error {
	domain := c.String("domain")
	if domain == "" {
		return fmt.Errorf("domain is required")
	}

	// Ensure domain has .local suffix
	if !strings.HasSuffix(domain, ".local") {
		domain = domain + ".local"
	}

	// Start the tunnel
	ctx := context.Background()
	err := manager.StartTunnel(ctx, c.Int("port"), domain, c.Bool("https"), c.Int("https-port"))
	if err != nil {
		return fmt.Errorf("failed to start tunnel: %w", err)
	}

	fmt.Printf("\nTunnel started successfully!\n")
	fmt.Printf("Local endpoint: http://localhost:%d\n", c.Int("port"))
	if c.Bool("https") {
		fmt.Printf("Access your service at: https://%s\n", domain)
	} else {
		fmt.Printf("Access your service at: http://%s\n", domain)
	}
	fmt.Printf("\nDomain is accessible:\n")
	fmt.Printf("- Locally via /etc/hosts: https://%s\n", domain)
	fmt.Printf("- On your network via mDNS: https://%s\n", domain)

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	return manager.StopTunnel(ctx, domain)
}

func StopTunnel(c *cli.Context) error {
	ctx := context.Background()
	domain := c.Args().Get(0)
	if domain == "" {
		return fmt.Errorf("domain is required")
	}
	return manager.StopTunnel(ctx, domain)
}

func StopAllTunnels(c *cli.Context) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	return manager.Stop(ctx)
}

func ListTunnels(c *cli.Context) error {
	tunnels := manager.ListTunnels()
	if len(tunnels) == 0 {
		fmt.Println("No active tunnels")
		return nil
	}

	fmt.Println("Active tunnels:")
	for _, t := range tunnels {
		fmt.Printf("  %s -> localhost:%d (HTTPS: %v)\n",
			t["domain"], t["port"], t["https"])
	}
	return nil
}
