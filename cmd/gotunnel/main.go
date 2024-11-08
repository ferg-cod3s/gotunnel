package main

import (
	"log"
	"os"

	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:  "gotunnel",
		Usage: "Create secure local tunnels for development",
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
			},
			{
				Name:   "list",
				Usage:  "List",
				Action: listTunnels,
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func startTunnel(c *cli.Context) error {
	t := tunnel.New(
		c.Int("port"),
		c.String("domain"),
		c.Bool("https"),
	)

	return t.Start()
}

func listTunnels(c *cli.Context) error {
	tunnels := tunnel.List()
	if len(tunnels) == 0 {
		log.Println("No active tunnels")
		return nil
	}

	for _, t := range tunnels {
		log.Printf("%s -> localhost:%d", t.Domain, t.Port)
	}
	return nil
}
