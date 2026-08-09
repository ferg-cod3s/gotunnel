package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/v1truv1us/gotunnel/internal/cert"
	"github.com/v1truv1us/gotunnel/internal/daemon"
	"github.com/v1truv1us/gotunnel/internal/dnsserver"
	"github.com/v1truv1us/gotunnel/internal/logging"
	"github.com/v1truv1us/gotunnel/internal/relay"
	"github.com/v1truv1us/gotunnel/internal/observability"
	"github.com/v1truv1us/gotunnel/internal/privilege"
	"github.com/v1truv1us/gotunnel/internal/proxy"
	"github.com/v1truv1us/gotunnel/internal/state"
	"github.com/v1truv1us/gotunnel/internal/tunnel"
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
		EnvVars: []string{"GOTUNNEL_SENTRY_DSN", "SENTRY_DSN"},
		Usage:   "Sentry DSN for error tracking (opt-in; disabled if unset)",
		Value:   "",
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
				Usage:   "HTTP port for proxy (default: 8080)",
				Value:   8080,
			},
			&cli.IntFlag{
				Name:    "proxy-https-port",
				EnvVars: []string{"GOTUNNEL_PROXY_HTTPS_PORT"}, 
				Usage:   "HTTPS port for proxy (default: 8443)",
				Value:   8443,
			},
		},
		Before: func(c *cli.Context) error {
			// Skip heavy setup when the daemon handles the command
			cmdName := c.Args().First()
			lightweightCmds := map[string]bool{
				"start": true, "stop": true, "list": true,
				"stop-all": true, "install-helper": true,
				"uninstall-helper": true, "port-forward": true,
				"expose": true,
			}
			if cmdName != "daemon" && cmdName != "" && lightweightCmds[cmdName] {
				client := daemon.NewClient()
				if client.IsRunning() || cmdName == "install-helper" || cmdName == "uninstall-helper" || cmdName == "port-forward" {
					return nil
				}
				// Auto-start daemon for start command
				if cmdName == "start" {
					if err := autoStartDaemon(); err == nil {
						return nil
					}
				}
			}

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

			// Create cert manager (use user-writable cache directory)
			cacheDir, _ := os.UserCacheDir()
			certsDir := filepath.Join(cacheDir, "gotunnel", "certs")
			os.MkdirAll(certsDir, 0755)
			certManager := cert.New(certsDir)
			
			// Initialize proxy if requested
			proxyModeStr := c.String("proxy")
			var useProxy bool
			
			if proxyModeStr != "none" {
				proxyConfig := proxy.ProxyConfig{
					Mode:        proxy.ProxyMode(proxyModeStr),
					HTTPPort:    c.Int("proxy-http-port"),
					HTTPSPort:   c.Int("proxy-https-port"),
					AutoInstall: false,
					CertManager: certManager,
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

		// Set a default hosts backup path (for legacy .local mode)
		hd, _ := os.UserHomeDir()
		gotunnelDir := filepath.Join(hd, ".gotunnel")
		os.MkdirAll(gotunnelDir, 0755)
		manager.SetHostsBackupDir(filepath.Join(gotunnelDir, "hosts.backup"))

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
						Value:   8080,
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
						Value: 8443,
						Usage: "HTTPS port (default: 8443)",
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
			{
				Name:   "daemon",
				Usage:  "Start the gotunnel daemon with menu bar icon",
				Action: RunDaemon,
			},
			{
				Name:   "install-helper",
				Usage:  "Install privileged port forwarder (80->8080, 443->8443). Requires sudo.",
				Action: func(c *cli.Context) error { return InstallHelper() },
			},
			{
				Name:   "uninstall-helper",
				Usage:  "Remove the privileged port forwarder. Requires sudo.",
				Action: func(c *cli.Context) error { return UninstallHelper() },
			},
			{
				Name:   "port-forward",
				Usage:  "Run the port forwarder (internal, launched by launchd/systemd)",
				Hidden: true,
				Action: func(c *cli.Context) error { return RunPortForward() },
			},
			{
				Name:      "expose",
				Usage:     "Expose a local port to the internet via a relay server",
				Flags: []cli.Flag{
					&cli.IntFlag{
						Name:    "port",
						Aliases: []string{"p"},
						Usage:   "Local port to expose",
						Value:   8080,
					},
					&cli.StringFlag{
						Name:    "domain",
						Aliases: []string{"d"},
						Usage:   "Subdomain for the tunnel URL",
					},
					&cli.StringFlag{
						Name:    "relay",
						Aliases: []string{"r"},
						Usage:   "Relay server URL (e.g., tunnel.example.com)",
					},
				},
				Action: ExposeTunnel,
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

		// Clear persisted tunnel state on graceful shutdown
		state.ClearTunnels()

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
	// Use daemon if running
	if dc := daemonClient(); dc != nil {
		return startViaDaemon(c, dc)
	}

	ctx := context.Background()
	ctx, span := obsProvider.StartSpan(ctx, "tunnel.start")
	defer span.End()

	domain := c.String("domain")
	if domain == "" {
		err := fmt.Errorf("domain is required")
		obsProvider.RecordError(ctx, span, err, "domain parameter missing")
		return err
	}

	if err := tunnel.ValidateDomain(domain); err != nil {
		obsProvider.RecordError(ctx, span, err, "domain validation failed")
		return err
	}

	// Ensure domain has .local suffix (RFC 6761: *.local resolves to 127.0.0.1)
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

	// Clean up any stale state entry for this domain (dead owner process)
	if existing, _ := state.LoadTunnels(); existing != nil {
		for _, t := range existing {
			if t.Domain == domain && !pidAlive(t.PID) {
				manager.CleanupStaleDomain(domain)
				state.RemoveTunnel(domain)
				break
			}
		}
	}

	// Start the tunnel
	timer := metrics.StartOperation(ctx, "tunnel_start")
	err := manager.StartTunnel(ctx, port, domain, https, httpsPort)
	timer.End(err)

	if err != nil {
		errMsg := fmt.Errorf("failed to start tunnel: %w", err)
		obsProvider.RecordError(ctx, span, err, "tunnel start failed")
		return errMsg
	}

	// Persist tunnel state so other processes can stop/list it
	if perr := state.AddTunnel(state.TunnelState{
		Domain:    domain,
		Port:      port,
		HTTPPort:  8080,
		HTTPSPort: httpsPort,
		HTTPS:     https,
		PID:       os.Getpid(),
		ProxyMode: c.String("proxy"),
	}); perr != nil {
		obsProvider.Logger().WarnContext(ctx, "Failed to persist tunnel state", "error", perr)
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
	state.RemoveTunnel(domain)

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
	domain := c.Args().Get(0)
	if domain == "" {
		return fmt.Errorf("domain is required")
	}

	// Use daemon if running
	if dc := daemonClient(); dc != nil {
		if err := dc.Stop(domain); err != nil {
			return err
		}
		fmt.Printf("Stopped tunnel %s\n", domain)
		return nil
	}

	ctx := context.Background()
	if !strings.HasSuffix(domain, ".local") {
		domain = domain + ".local"
	}

	// In-process stop: domain is owned by this process's manager
	for _, t := range manager.ListTunnels() {
		if t["domain"] == domain {
			if err := manager.StopTunnel(ctx, domain); err != nil {
				return err
			}
			state.RemoveTunnel(domain)
			fmt.Printf("Stopped tunnel %s\n", domain)
			return nil
		}
	}

	// Cross-process stop: signal the owning process
	tunnels, _ := state.LoadTunnels()
	var entry *state.TunnelState
	for i := range tunnels {
		if tunnels[i].Domain == domain {
			entry = &tunnels[i]
			break
		}
	}
	if entry == nil {
		return fmt.Errorf("no tunnel for domain %s", domain)
	}

	if entry.PID == os.Getpid() {
		if err := manager.StopTunnel(ctx, domain); err != nil {
			return err
		}
		state.RemoveTunnel(domain)
		return nil
	}

	if !pidAlive(entry.PID) {
		// Stale entry: owner is dead, clean up orphaned hosts/DNS
		manager.CleanupStaleDomain(domain)
		state.RemoveTunnel(domain)
		fmt.Printf("Removed stale tunnel %s (process %d not running)\n", domain, entry.PID)
		return nil
	}

	proc, err := os.FindProcess(entry.PID)
	if err != nil {
		return fmt.Errorf("failed to find process %d: %w", entry.PID, err)
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to signal process %d: %w", entry.PID, err)
	}

	// Wait for the owner to remove its own state entry
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if remaining, _ := state.LoadTunnels(); !containsDomain(remaining, domain) {
			fmt.Printf("Stopped tunnel %s\n", domain)
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Timeout: force clean up locally
	manager.CleanupStaleDomain(domain)
	state.RemoveTunnel(domain)
	fmt.Printf("Stopped tunnel %s (force-cleaned after timeout)\n", domain)
	return nil
}

func StopAllTunnels(c *cli.Context) error {
	// Use daemon if running
	if dc := daemonClient(); dc != nil {
		if err := dc.StopAll(); err != nil {
			return err
		}
		fmt.Println("Stopped all tunnels")
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Stop in-process tunnels
	manager.Stop(ctx)

	// Cross-process: signal every persisted tunnel
	tunnels, _ := state.LoadTunnels()
	for _, t := range tunnels {
		if t.PID == os.Getpid() {
			continue
		}
		if !pidAlive(t.PID) {
			manager.CleanupStaleDomain(t.Domain)
			continue
		}
		if proc, err := os.FindProcess(t.PID); err == nil {
			proc.Signal(syscall.SIGTERM)
		}
	}
	state.ClearTunnels()
	fmt.Println("Stopped all tunnels")
	return nil
}

func ListTunnels(c *cli.Context) error {
	// Use daemon if running
	if dc := daemonClient(); dc != nil {
		tunnels, err := dc.List()
		if err != nil {
			return err
		}
		if len(tunnels) == 0 {
			fmt.Println("No active tunnels")
			return nil
		}
		fmt.Println("Active tunnels:")
		for _, t := range tunnels {
			fmt.Printf("  ● %s -> localhost:%d\n", t.URL, t.Port)
		}
		return nil
	}

	// Merge in-process and persisted tunnels
	type tunnelInfo struct {
		domain string
		port   int
		https  bool
		pid    int
		alive  bool
	}
	seen := make(map[string]bool)
	var info []tunnelInfo

	for _, t := range manager.ListTunnels() {
		domain, _ := t["domain"].(string)
		port, _ := t["port"].(int)
		https, _ := t["https"].(bool)
		info = append(info, tunnelInfo{domain, port, https, os.Getpid(), true})
		seen[domain] = true
	}

	persisted, _ := state.LoadTunnels()
	for _, t := range persisted {
		if seen[t.Domain] {
			continue
		}
		info = append(info, tunnelInfo{t.Domain, t.Port, t.HTTPS, t.PID, pidAlive(t.PID)})
	}

	if len(info) == 0 {
		fmt.Println("No active tunnels")
		return nil
	}

	fmt.Println("Active tunnels:")
	for _, t := range info {
		status := "running"
		if !t.alive {
			status = "dead"
		}
		fmt.Printf("  %s -> localhost:%d (HTTPS: %v) [%s, pid %d]\n",
			t.domain, t.port, t.https, status, t.pid)
	}
	return nil
}

// pidAlive reports whether a process with the given PID is currently running.
func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal 0 probes process existence without delivering a signal.
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		return false
	}
	return true
}

// containsDomain reports whether a slice of TunnelState contains the given domain.
func containsDomain(tunnels []state.TunnelState, domain string) bool {
	for _, t := range tunnels {
		if t.Domain == domain {
			return true
		}
	}
	return false
}

// daemonClient returns a client if the daemon is running, nil otherwise.
func daemonClient() *daemon.Client {
	c := daemon.NewClient()
	if c.IsRunning() {
		return c
	}
	return nil
}

// RunDaemon starts the persistent daemon with menu bar icon.
func RunDaemon(c *cli.Context) error {
	daemonServer, err := daemon.NewServer(manager)
	if err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}
	daemonServer.Start()

	fmt.Println("gotunnel daemon running.")
	fmt.Println("Menu bar icon active — click 🚇 to manage tunnels.")
	fmt.Println("Run 'gotunnel start -p <port> -d <name>' to create tunnels.")

	// Block on menu bar until user clicks Quit
	daemon.RunMenuBar(daemonServer)

	daemonServer.Stop()
	fmt.Println("Daemon stopped.")
	return nil
}

// startViaDaemon sends a start request to the daemon and returns immediately.
func startViaDaemon(c *cli.Context, dc *daemon.Client) error {
	info, err := dc.Start(daemon.StartRequest{
		Port:      c.Int("port"),
		Domain:    c.String("domain"),
		HTTPS:     c.Bool("https"),
		HTTPSPort: c.Int("https-port"),
	})
	if err != nil {
		return err
	}

	fmt.Printf("\nTunnel started via daemon!\n")
	fmt.Printf("Backend: localhost:%d\n", info.Port)
	fmt.Printf("Access:  %s\n", info.URL)
	return nil
}

// autoStartDaemon launches the daemon in the background and waits for it to come up.
func autoStartDaemon() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}

	cmd := exec.Command(exe, "daemon")
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Env = os.Environ()

	if err := cmd.Start(); err != nil {
		return err
	}
	cmd.Process.Release()

	// Wait for daemon socket (max 5 seconds)
	for i := 0; i < 50; i++ {
		time.Sleep(100 * time.Millisecond)
		client := daemon.NewClient()
		if client.IsRunning() {
			return nil
		}
	}

	return fmt.Errorf("daemon did not start within 5 seconds")
}

// ExposeTunnel connects to a relay server and exposes a local port publicly.
func ExposeTunnel(c *cli.Context) error {
	localPort := c.Int("port")
	if localPort <= 0 {
		return fmt.Errorf("invalid port: %d", localPort)
	}

	domain := c.String("domain")
	if domain == "" {
		return fmt.Errorf("--domain is required (e.g., -d myapp)")
	}

	relayURL := c.String("relay")
	if relayURL == "" {
		return fmt.Errorf("--relay is required (e.g., --relay tunnel.example.com)")
	}

	// Normalize relay URL to wss://
	if !strings.HasPrefix(relayURL, "ws://") && !strings.HasPrefix(relayURL, "wss://") {
		relayURL = "wss://" + relayURL
	}

	// Derive the public URL
	publicURL := fmt.Sprintf("https://%s.%s", domain, strings.TrimPrefix(strings.TrimPrefix(relayURL, "wss://"), "ws://"))

	fmt.Printf("\n")
	fmt.Printf("  Local:   http://localhost:%d\n", localPort)
	fmt.Printf("  Public:  %s\n", publicURL)
	fmt.Printf("\nPress Ctrl+C to stop.\n\n")

	client := relay.NewClient(relayURL, domain, localPort)
	return client.Run()
}
