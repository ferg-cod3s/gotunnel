package relay

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Client connects to a relay server and forwards requests to a local port.
type Client struct {
	relayURL  string
	domain    string
	localPort int
	conn      *websocket.Conn
	writeMu   sync.Mutex
}

func NewClient(relayURL, domain string, localPort int) *Client {
	return &Client{
		relayURL:  relayURL,
		domain:    domain,
		localPort: localPort,
	}
}

func (c *Client) Run() error {
	for {
		if err := c.connectAndServe(); err != nil {
			log.Printf("relay disconnected: %v — reconnecting in 3s", err)
		}
		time.Sleep(3 * time.Second)
	}
}

func (c *Client) connectAndServe() error {
	wsURL := fmt.Sprintf("%s/ws?domain=%s", c.relayURL, c.domain)

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to relay: %w", err)
	}
	c.conn = conn
	defer conn.Close()

	log.Printf("connected to relay: %s (domain: %s)", c.relayURL, c.domain)

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return err
		}

		var req Request
		if err := json.Unmarshal(data, &req); err != nil {
			continue
		}
		go c.proxyRequest(&req)
	}
}

func (c *Client) proxyRequest(req *Request) {
	body, _ := base64.StdEncoding.DecodeString(req.BodyB64)

	target := fmt.Sprintf("http://localhost:%d%s", c.localPort, req.Path)
	httpReq, err := http.NewRequest(req.Method, target, bytes.NewReader(body))
	if err != nil {
		c.sendResponse(&Response{ID: req.ID, Status: 500, Error: err.Error()})
		return
	}

	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		c.sendResponse(&Response{ID: req.ID, Status: 502, Error: err.Error()})
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	c.sendResponse(&Response{
		ID:      req.ID,
		Status:  resp.StatusCode,
		Headers: flattenHeader(resp.Header),
		BodyB64: base64.StdEncoding.EncodeToString(respBody),
	})
}

func (c *Client) sendResponse(resp *Response) {
	data, _ := json.Marshal(resp)
	c.writeMu.Lock()
	c.conn.WriteMessage(websocket.TextMessage, data)
	c.writeMu.Unlock()
}
