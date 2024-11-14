package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/johncferguson/gotunnel/internal/privilege"
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
			log.Println("Checking privileges...")
			if !c.Bool("no-privilege-check") {
				if err := privilege.CheckPrivileges(); err != nil {
					log.Printf("Privilege check failed: %v", err)
					return err
				}
				log.Println("Privilege check passed.")
			}

			log.Println("Initializing tunnel manager...")
			manager = tunnel.NewManager()
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
						Value:   8000,
						Usage:   "Local port to tunnel",
					},
					&cli.StringFlag{
						Name:     "domain",
						Aliases:  []string{"d"},
						Usage:    "Desired .local domain",
						Required: true,
					},
					&cli.BoolFlag{
						Name:    "https",
						Aliases: []string{"t"},
						Value:   true,
						Usage:   "Enable HTTPS",
					},
				},
				Action: StartTunnel,
			},
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
			{
				Name:   "stopAll",
				Usage:  "Stop all tunnels",
				Action: StopAllTunnels,
			},
			{
				Name:    "list",
				Aliases: []string{"ls"},
				Usage:   "List all active tunnels",
				Action:  ListTunnels,
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func StartTunnel(c *cli.Context) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Start the tunnel using parameters from CLI flags
	// This creates the HTTP server, sets up the proxy, and registers mDNS
	if err := manager.StartTunnel(
		ctx,
		c.Int("port"),      // Local port to forward traffic to
		c.String("domain"), // Domain name for the tunnel (e.g., myapp)
		c.Bool("https"),    // Whether to use HTTPS
	); err != nil {
		return err
	}

	log.Println("Tunnel started successfully")

	// Create a channel that will never receive a value
	// This is used to keep the program running indefinitely
	forever := make(chan struct{})

	// Create a channel for OS signals
	// Buffer size of 1 means it can hold one signal without blocking
	sigChan := make(chan os.Signal, 1)

	// Register for SIGINT (Ctrl+C) and SIGTERM (graceful shutdown) signals
	// When either signal is received, it will be sent to sigChan
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start a goroutine to handle shutdown
	// This runs concurrently with the main thread
	go func() {
		// Wait here until a signal is received
		<-sigChan

		// Once a signal is received, begin shutdown
		log.Println("Shutting down...")

		// Stop all tunnels, unregister mDNS, and cleanup
		if err := manager.Stop(ctx); err != nil {
			log.Printf("Error stopping tunnels: %v", err)
		}

		// Exit the program with status 0 (success)
		os.Exit(0)
	}()

	// Block the main thread forever
	// This prevents the program from exiting until a signal is received
	// and the shutdown goroutine calls os.Exit()
	<-forever

	// This return will never be reached due to os.Exit() in the goroutine
	return nil
}

func StopTunnel(c *cli.Context) error {
	ctx := context.Background()
	return manager.StopTunnel(ctx, c.String("domain"))
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
