package main

import (
	"fmt"
	"log"
	"os"

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
			// Check privileges
			if !c.Bool("no-privilege-check") {
				if err := privilege.CheckPrivileges(); err != nil {
					return err
				}
			}

			// Initialize tunnel manager
			manager = tunnel.NewManager()
			return nil // Remove the StartTunnel() call here
		},
		// After: func(c *cli.Context) error {
		// 	if manager != nil {
		// 		if err := manager.Stop(); err != nil {
		// 			log.Printf("Error stopping tunnels: %v", err)
		// 			return err
		// 		}
		// 	}
		// 	return nil
		// },
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
						Name:  "https",
						Value: true,
						Usage: "Enable HTTPS",
					},
				},
				Action: startTunnel,
			},
			{
				Name:   "stop",
				Usage:  "Stop a tunnel",
				Action: stopTunnel,
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
				Action: stopAllTunnels,
			},
			{
				Name:    "list",
				Aliases: []string{"ls"},
				Usage:   "List all active tunnels",
				Action:  listTunnels,
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func startTunnel(c *cli.Context) error {
	return manager.StartTunnel(
		c.Int("port"),
		c.String("domain"),
		c.Bool("https"),
	)
}

func stopTunnel(c *cli.Context) error {
	return manager.StopTunnel(c.String("domain"))
}

func stopAllTunnels(c *cli.Context) error {
	return manager.Stop()
}

func listTunnels(c *cli.Context) error {
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
