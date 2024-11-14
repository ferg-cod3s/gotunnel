package dnsserver

import (
	"fmt"
	"log"
	"net"
	"os"
	"strings"

	"github.com/miekg/dns"
)

// hosts represents a custom hosts file entry
type hosts struct {
	hostname string
	ip       net.IP
}

// loadHostsFile reads and parses the custom hosts file
func loadHostsFile(filename string) ([]hosts, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read hosts file: %w", err)
	}

	var entries []hosts
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue // Skip empty lines and comments
		}

		fields := strings.Fields(line)
		if len(fields) >= 2 {
			ip := net.ParseIP(fields[0])
			if ip != nil {
				entries = append(entries, hosts{hostname: fields[1], ip: ip})
			}
		}
	}

	return entries, nil
}

// handleDNSRequest handles incoming DNS queries
func handleDNSRequest(w dns.ResponseWriter, r *dns.Msg, hostsFileEntries []hosts) {
	m := new(dns.Msg)
	m.SetReply(r)
	m.RecursionAvailable = true

	for _, q := range r.Question {
		if strings.HasSuffix(q.Name, ".go.") && q.Qtype == dns.TypeA { // Check for .go TLD
			for _, entry := range hostsFileEntries {
				if entry.hostname == q.Name {
					rr, err := dns.NewRR(fmt.Sprintf("%s A %s", q.Name, entry.ip.String()))
					if err != nil {
						log.Println("Error creating DNS record:", err)
						return
					}
					m.Answer = append(m.Answer, rr)
					break // Found matching entry in hosts file
				}
			}
		} else {
			// Forward other requests to upstream DNS server
			in, err := dns.Exchange(r, "8.8.8.8:53") // Use Google Public DNS as upstream
			if err != nil {
				log.Println("Error forwarding DNS request:", err)
				return
			}
			m = in
		}
	}

	w.WriteMsg(m)
}

// StartDNSServer starts the custom DNS server
func StartDNSServer(hostsFile string, listenAddr string) error {
	hostsFileEntries, err := loadHostsFile(hostsFile)
	if err != nil {
		return err
	}

	dns.HandleFunc(".", func(w dns.ResponseWriter, r *dns.Msg) {
		handleDNSRequest(w, r, hostsFileEntries)
	})
	server := &dns.Server{Addr: listenAddr, Net: "udp"}
	log.Printf("Starting DNS server on %s\n", listenAddr)
	return server.ListenAndServe()
}
