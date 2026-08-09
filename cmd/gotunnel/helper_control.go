package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
)

const helperSocketPath = "/tmp/gotunnel-helper.sock"

// runControlServer starts an HTTP API on a Unix socket for hosts management.
// Called by the port-forwarder (running as root).
func runControlServer() {
	os.Remove(helperSocketPath)
	listener, err := net.Listen("unix", helperSocketPath)
	if err != nil {
		log.Printf("helper: control socket failed: %v", err)
		return
	}
	os.Chmod(helperSocketPath, 0666)

	mux := http.NewServeMux()
	mux.HandleFunc("/hosts/add", handleHostsAdd)
	mux.HandleFunc("/hosts/remove", handleHostsRemove)
	mux.HandleFunc("/hosts/list", handleHostsList)

	log.Println("helper: control API ready on", helperSocketPath)
	http.Serve(listener, mux)
}

func handleHostsAdd(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Domain string `json:"domain"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Domain == "" {
		http.Error(w, "domain required", http.StatusBadRequest)
		return
	}
	if err := addHostsEntry(req.Domain); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func handleHostsRemove(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Domain string `json:"domain"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Domain == "" {
		http.Error(w, "domain required", http.StatusBadRequest)
		return
	}
	if err := removeHostsEntry(req.Domain); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func handleHostsList(w http.ResponseWriter, r *http.Request) {
	content, err := os.ReadFile("/etc/hosts")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var entries []string
	for _, line := range strings.Split(string(content), "\n") {
		if strings.Contains(line, "# gotunnel") {
			entries = append(entries, strings.TrimSpace(line))
		}
	}
	json.NewEncoder(w).Encode(entries)
}

const hostsMarker = "# gotunnel"

func addHostsEntry(domain string) error {
	content, err := os.ReadFile("/etc/hosts")
	if err != nil {
		return fmt.Errorf("cannot read /etc/hosts: %w", err)
	}

	for _, line := range strings.Split(string(content), "\n") {
		if strings.Contains(line, domain) && strings.Contains(line, hostsMarker) {
			return nil
		}
	}

	newContent := string(content)
	if !strings.HasSuffix(newContent, "\n") {
		newContent += "\n"
	}
	newContent += fmt.Sprintf("127.0.0.1\t%s\t%s\n", domain, hostsMarker)

	return atomicWriteHosts(newContent)
}

func removeHostsEntry(domain string) error {
	content, err := os.ReadFile("/etc/hosts")
	if err != nil {
		return fmt.Errorf("cannot read /etc/hosts: %w", err)
	}

	var kept []string
	for _, line := range strings.Split(string(content), "\n") {
		if strings.Contains(line, domain) && strings.Contains(line, hostsMarker) {
			continue
		}
		kept = append(kept, line)
	}

	return atomicWriteHosts(strings.Join(kept, "\n"))
}

func atomicWriteHosts(content string) error {
	tmp := "/etc/hosts.gotunnel-tmp"
	if err := os.WriteFile(tmp, []byte(content), 0644); err != nil {
		return fmt.Errorf("cannot write temp file: %w", err)
	}
	return os.Rename(tmp, "/etc/hosts")
}
