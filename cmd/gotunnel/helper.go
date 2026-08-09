package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
)

const helperTargetPath = "/usr/local/bin/gotunnel-helper"

// RunPortForward listens on privileged ports and forwards to the daemon.
// Run as root via launchd/systemd. Blocks forever.
func RunPortForward() error {
	go tcpForward(":80", "127.0.0.1:8080")
	go tcpForward(":443", "127.0.0.1:8443")
	go runControlServer()
	go runDNSServer()

	log.Println("gotunnel port-forwarder: :80->8080, :443->8443, DNS :53 (*.local)")
	select {}
}

func tcpForward(listenAddr, targetAddr string) {
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalf("port-forward: failed to listen on %s: %v", listenAddr, err)
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		go handleForward(conn, targetAddr)
	}
}

func handleForward(client net.Conn, target string) {
	defer client.Close()
	upstream, err := net.Dial("tcp", target)
	if err != nil {
		return
	}
	defer upstream.Close()

	done := make(chan struct{}, 2)
	go func() {
		io.Copy(upstream, client)
		done <- struct{}{}
	}()
	go func() {
		io.Copy(client, upstream)
		done <- struct{}{}
	}()
	<-done
}

// copySelfToHelper copies the current binary to the helper target path.
func copySelfToHelper() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine executable path: %w", err)
	}
	data, err := os.ReadFile(exe)
	if err != nil {
		return fmt.Errorf("cannot read own binary: %w", err)
	}
	if err := os.WriteFile(helperTargetPath, data, 0755); err != nil {
		return fmt.Errorf("cannot write to %s: %w", helperTargetPath, err)
	}
	return nil
}
