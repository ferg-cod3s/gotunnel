package main

import (
	"encoding/binary"
	"log"
	"net"
	"strings"
	"time"

	"github.com/v1truv1us/gotunnel/internal/dnsserver"
)

// runDNSServer listens on port 53 and resolves *.local to the Mac's LAN IP.
// Non-.local queries are forwarded to 8.8.8.8 so normal browsing still works.
// Run as root via the privileged helper.
func runDNSServer() {
	lanIP := dnsserver.GetOutboundIP()
	addr := &net.UDPAddr{Port: 53}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Printf("DNS: failed to bind port 53: %v", err)
		return
	}
	defer conn.Close()

	log.Println("DNS: serving *.local ->", lanIP.String(), "on :53")

	buf := make([]byte, 1024)
	for {
		n, remote, err := conn.ReadFromUDP(buf)
		if err != nil {
			continue
		}

		query := buf[:n]
		domain := extractDomain(query)

		if strings.HasSuffix(domain, ".local") {
			resp := buildDNSResponse(query, lanIP)
			if resp != nil {
				conn.WriteToUDP(resp, remote)
			}
		} else {
			forwardDNS(query, remote, conn)
		}
	}
}

func extractDomain(query []byte) string {
	if len(query) < 13 {
		return ""
	}
	pos := 12
	var labels []string
	for pos < len(query) {
		labelLen := int(query[pos])
		if labelLen == 0 {
			break
		}
		pos++
		if pos+labelLen > len(query) {
			return ""
		}
		labels = append(labels, string(query[pos:pos+labelLen]))
		pos += labelLen
	}
	return strings.Join(labels, ".")
}

func buildDNSResponse(query []byte, ip net.IP) []byte {
	if len(query) < 12 || ip == nil {
		return nil
	}
	ip4 := ip.To4()
	if ip4 == nil {
		return nil
	}

	resp := make([]byte, 0, len(query)+20)

	header := make([]byte, 12)
	copy(header, query[:12])
	header[2] = 0x84
	header[3] = 0x00
	binary.BigEndian.PutUint16(header[6:8], 1)
	resp = append(resp, header...)

	pos := 12
	qdcount := binary.BigEndian.Uint16(query[4:6])
	for i := 0; i < int(qdcount) && pos < len(query); i++ {
		for pos < len(query) {
			l := int(query[pos])
			if l == 0 {
				pos++
				break
			}
			pos += l + 1
		}
		pos += 4
	}
	resp = append(resp, query[12:pos]...)

	answer := []byte{
		0xC0, 0x0C,
		0x00, 0x01,
		0x00, 0x01,
		0x00, 0x00, 0x00, 0x3C,
		0x00, 0x04,
	}
	resp = append(resp, answer...)
	resp = append(resp, ip4...)

	return resp
}

func forwardDNS(query []byte, remote *net.UDPAddr, listenConn *net.UDPConn) {
	upstream, err := net.DialTimeout("udp", "8.8.8.8:53", 2*time.Second)
	if err != nil {
		return
	}
	defer upstream.Close()

	upstream.SetDeadline(time.Now().Add(3 * time.Second))
	if _, err := upstream.Write(query); err != nil {
		return
	}

	respBuf := make([]byte, 1024)
	n, err := upstream.Read(respBuf)
	if err != nil {
		return
	}

	listenConn.WriteToUDP(respBuf[:n], remote)
}
