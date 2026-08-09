package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/v1truv1us/gotunnel/internal/relay"
)

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	addr := flag.String("addr", getenv("RELAY_ADDR", ":8080"), "Listen address")
	domain := flag.String("domain", getenv("RELAY_DOMAIN", ""), "Base domain for tunnels (e.g., tunnel.example.com)")
	flag.Parse()

	server := relay.NewServer(*domain)

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", server.HandleTunnel)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})
	mux.HandleFunc("/api/tunnels", server.HandleTunnelsList)
	mux.HandleFunc("/", server.HandleHTTP)

	srv := &http.Server{Addr: *addr, Handler: mux}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("shutting down relay...")
		srv.Close()
	}()

	log.Printf("gotunnel relay server")
	log.Printf("  listen:  %s", *addr)
	log.Printf("  domain:  %s", *domain)
	log.Printf("  endpoints: /ws (tunnel) | /* (proxy) | /health | /api/tunnels")

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
