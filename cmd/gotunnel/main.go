package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/johncferguson/gotunnel/internal/cert"
	"github.com/johncferguson/gotunnel/internal/dnsserver"
	"github.com/johncferguson/gotunnel/internal/logging"
	"github.com/johncferguson/gotunnel/internal/observability"
	"github.com/johncferguson/gotunnel/internal/privilege"
	"github.com/johncferguson/gotunnel/internal/proxy"
	"github.com/johncferguson/gotunnel/internal/tunnel"
	"github.com/urfave/cli/v2"
	"go.opentelemetry.io/otel/attribute"
)

// Build-time variables (set by ldflags)
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

var (
	manager      *tunnel.Manager
	obsProvider  *observability.Provider
	metrics      *observability.Metrics
	proxyManager *proxy.Manager
)

func main() {
	app := &cli.App{
		Name:    "gotunnel",
		Usage:   "Create secure local tunnels for development",
		Version: fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date),
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "no-privilege-check",
				Value: false,
				Usage: "Skip privilege check",
			},
			&cli.StringFlag{
				Name:    "sentry-dsn",
				EnvVars: []string{"SENTRY_DSN"},
				Usage:   "Sentry DSN for error tracking and performance monitoring",
				Value:   "https://2df8619717cb8316ef83612d2ec29b95@sentry.fergify.work/11",
			},
			&cli.StringFlag{
				Name:    "environment",
				EnvVars: []string{"ENVIRONMENT"},
				Usage:   "Environment (development, staging, production)",
				Value:   "development",
			},
			&cli.BoolFlag{
				Name:    "debug",
				EnvVars: []string{"DEBUG"},
				Usage:   "Enable debug logging and tracing",
			},
			&cli.StringFlag{
				Name:    "proxy",
				EnvVars: []string{"GOTUNNEL_PROXY"},
				Usage:   "Proxy mode: builtin, nginx, caddy, auto, config, none",
				Value:   "auto",
			},
			&cli.IntFlag{
				Name:    "proxy-http-port",
				EnvVars: []string{"GOTUNNEL_PROXY_HTTP_PORT"},
				Usage:   "HTTP port for proxy (default: 80)",
				Value:   80,
			},
			&cli.IntFlag{
				Name:    "proxy-https-port",
				EnvVars: []string{"GOTUNNEL_PROXY_HTTPS_PORT"}, 
				Usage:   "HTTPS port for proxy (default: 443)",
				Value:   443,
			},
		},
		Before: func(c *cli.Context) error {
			// Configure logging
			logConfig := &logging.Config{
				Level:      logging.LevelInfo,
				Format:     logging.FormatText,
				Output:     "stdout",
				AddSource:  false,
				TimeFormat: time.RFC3339,
			}
			
			if c.Bool("debug") {
				logConfig.Level = logging.LevelDebug
				logConfig.AddSource = true
			}
			
			// Initialize observability first
			obsConfig := observability.Config{
				ServiceName:      "gotunnel",
				ServiceVersion:   version,
				Environment:      c.String("environment"),
				SentryDSN:        c.String("sentry-dsn"),
				TracesSampleRate: 1.0,
				LogLevel:         slog.LevelInfo,
				LogFormat:        "text",
				Debug:            c.Bool("debug"),
				Logging:          logConfig,
			}

			if obsConfig.Debug {
				obsConfig.LogLevel = slog.LevelDebug
				obsConfig.LogFormat = "text" // Keep text format for debug readability
			}

			var err error
			obsProvider, err = observability.NewProvider(obsConfig)
			if err != nil {
				return fmt.Errorf("failed to initialize observability: %w", err)
			}

			// Initialize metrics
			metrics, err = observability.NewMetrics(obsProvider)
			if err != nil {
				return fmt.Errorf("failed to initialize metrics: %w", err)
			}

			// Create a root context with tracing
			ctx := context.Background()
			ctx, span := obsProvider.StartSpan(ctx, "gotunnel.startup")
			defer span.End()

			obsProvider.Logger().WithContext(ctx).Info("Starting gotunnel",
				"version", obsConfig.ServiceVersion,
				"environment", obsConfig.Environment,
			)

			if !c.Bool("no-privilege-check") {
				if err := privilege.CheckPrivileges(); err != nil {
					metrics.RecordError(ctx, "privilege_check", "startup", err)
					return err
				}
			}

			// Create cert manager
			certManager := cert.New("./certs")
			
			// Initialize proxy if requested
			proxyModeStr := c.String("proxy")
			var useProxy bool
			
			if proxyModeStr != "none" {
				proxyConfig := proxy.ProxyConfig{
					Mode:        proxy.ProxyMode(proxyModeStr),
					HTTPPort:    c.Int("proxy-http-port"),
					HTTPSPort:   c.Int("proxy-https-port"),
					AutoInstall: false, // Don't auto-install external tools
				}
				
				// Auto-detect best proxy if mode is "auto"
				if proxyConfig.Mode == proxy.AutoProxy {
					available := proxy.DetectAvailableProxies()
					if len(available) > 0 {
						// Prefer builtin for reliability in enterprise environments  
						proxyConfig.Type = proxy.BuiltInProxyType
						proxyConfig.Mode = proxy.BuiltInProxy
						obsProvider.Logger().InfoContext(ctx, "Auto-selected built-in proxy for maximum compatibility")
					} else {
						proxyConfig.Mode = proxy.NoProxy
						obsProvider.Logger().WarnContext(ctx, "No proxy available, disabling proxy mode")
					}
				}
				
				if proxyConfig.Mode != proxy.NoProxy {
					proxyManager = proxy.NewManager(proxyConfig)
					useProxy = true
					
					obsProvider.Logger().InfoContext(ctx, "Proxy initialized",
						slog.String("mode", string(proxyConfig.Mode)),
						slog.Int("http_port", proxyConfig.HTTPPort),
						slog.Int("https_port", proxyConfig.HTTPSPort),
					)
				}
			}
			
			// Create tunnel manager with proxy integration
			if useProxy && proxyManager != nil {
				manager = tunnel.NewManagerWithProxy(certManager, proxyManager, true, obsProvider.Logger())
				
				// Start the proxy system
				if err := proxyManager.Start(); err != nil {
					obsProvider.Logger().WithContext(ctx).Error("Failed to start proxy", "error", err)
					metrics.RecordError(ctx, "proxy", "startup", err)
					// Don't fail completely, fall back to direct mode
					manager = tunnel.NewManager(certManager, obsProvider.Logger())
					proxyManager = nil
					obsProvider.Logger().WithContext(ctx).Warn("Falling back to direct tunnel mode")
				} else {
					obsProvider.Logger().WithContext(ctx).Info("Proxy system started successfully")
				}
			} else {
				manager = tunnel.NewManager(certManager, obsProvider.Logger())
			}

			// Set up DNS server
			go func() {
				if err := dnsserver.StartDNSServer(); err != nil {
					obsProvider.Logger().ErrorContext(ctx, "Failed to start DNS server", slog.Any("error", err))
					metrics.RecordError(ctx, "dns_server", "startup", err)
				}
			}()

			setupCleanup()
			
			span.SetAttributes(
				attribute.String("service.version", obsConfig.ServiceVersion),
				attribute.String("service.environment", obsConfig.Environment),
			)
			
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

		ctx := context.Background()
		if obsProvider != nil {
			ctx, span := obsProvider.StartSpan(ctx, "application.shutdown")
			defer span.End()

			obsProvider.Logger().InfoContext(ctx, "Shutting down application...")
		}

		shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		// Stop proxy manager first
		if proxyManager != nil {
			if err := proxyManager.Stop(); err != nil {
				if obsProvider != nil {
					obsProvider.Logger().ErrorContext(shutdownCtx, "Error stopping proxy manager", slog.Any("error", err))
					metrics.RecordError(shutdownCtx, "proxy_manager", "shutdown", err)
				} else {
					log.Printf("Error during proxy manager shutdown: %v", err)
				}
			}
		}

		// Stop tunnel manager
		if manager != nil {
			if err := manager.Stop(shutdownCtx); err != nil {
				if obsProvider != nil {
					obsProvider.Logger().ErrorContext(shutdownCtx, "Error stopping tunnel manager", slog.Any("error", err))
					metrics.RecordError(shutdownCtx, "tunnel_manager", "shutdown", err)
				} else {
					log.Printf("Error during tunnel manager shutdown: %v", err)
				}
			}
		}

		// Shutdown observability provider
		if obsProvider != nil {
			obsProvider.Logger().InfoContext(shutdownCtx, "Shutting down observability...")
			if err := obsProvider.Shutdown(shutdownCtx); err != nil {
				// Can't use obsProvider.Logger here since we're shutting it down
				log.Printf("Error during observability shutdown: %v", err)
			}
		}

		fmt.Println("Shutdown complete")
		os.Exit(0)
	}()
}

func StartTunnel(c *cli.Context) error {
	ctx := context.Background()
	ctx, span := obsProvider.StartSpan(ctx, "tunnel.start")
	defer span.End()

	domain := c.String("domain")
	if domain == "" {
		err := fmt.Errorf("domain is required")
		obsProvider.RecordError(ctx, span, err, "domain parameter missing")
		return err
	}

	// Ensure domain has .local suffix
	if !strings.HasSuffix(domain, ".local") {
		domain = domain + ".local"
	}

	port := c.Int("port")
	https := c.Bool("https")
	httpsPort := c.Int("https-port")

	// Add span attributes
	span.SetAttributes(
		attribute.String("tunnel.domain", domain),
		attribute.Int("tunnel.port", port),
		attribute.Bool("tunnel.https", https),
		attribute.Int("tunnel.https_port", httpsPort),
	)

	// Log the tunnel start attempt
	obsProvider.Logger().InfoContext(ctx, "Starting tunnel",
		slog.String("domain", domain),
		slog.Int("port", port),
		slog.Bool("https", https),
		slog.Int("https_port", httpsPort),
	)

	// Record tunnel creation metric
	metrics.TunnelCreated(ctx, domain, port, https)

	// Start the tunnel
	timer := metrics.StartOperation(ctx, "tunnel_start")
	err := manager.StartTunnel(ctx, port, domain, https, httpsPort)
	timer.End(err)

	if err != nil {
		errMsg := fmt.Errorf("failed to start tunnel: %w", err)
		obsProvider.RecordError(ctx, span, err, "tunnel start failed")
		return errMsg
	}

	obsProvider.Logger().InfoContext(ctx, "Tunnel started successfully",
		slog.String("domain", domain),
		slog.Int("port", port),
	)

	// Print success information
	fmt.Printf("\nTunnel started successfully!\n")
	fmt.Printf("Local endpoint: http://localhost:%d\n", port)
	if https {
		fmt.Printf("Access your service at: https://%s\n", domain)
	} else {
		fmt.Printf("Access your service at: http://%s\n", domain)
	}
	fmt.Printf("\nDomain is accessible:\n")
	fmt.Printf("- Locally via /etc/hosts: https://%s\n", domain)
	fmt.Printf("- On your network via mDNS: https://%s\n", domain)

	// Track tunnel start time for duration calculation
	startTime := time.Now()

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	obsProvider.Logger().InfoContext(ctx, "Received shutdown signal, stopping tunnel",
		slog.String("domain", domain),
	)

	// Stop tunnel with proper tracing
	stopCtx, stopSpan := obsProvider.StartSpan(ctx, "tunnel.stop")
	defer stopSpan.End()

	stopTimer := metrics.StartOperation(stopCtx, "tunnel_stop")
	err = manager.StopTunnel(stopCtx, domain)
	stopTimer.End(err)

	// Record tunnel duration
	duration := time.Since(startTime)
	metrics.TunnelDestroyed(stopCtx, domain, duration)

	if err != nil {
		obsProvider.RecordError(stopCtx, stopSpan, err, "tunnel stop failed")
		return err
	}

	obsProvider.Logger().InfoContext(stopCtx, "Tunnel stopped successfully",
		slog.String("domain", domain),
		slog.Duration("total_duration", duration),
	)

	return nil
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
