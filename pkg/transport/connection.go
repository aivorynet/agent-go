// Package transport provides WebSocket connection to AIVory backend.
package transport

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ilscipio/aivory-monitor/agent-go/pkg/capture"
)

// Connection represents a WebSocket connection to the AIVory backend.
type Connection struct {
	url       string
	apiKey    string
	debug     bool
	conn      *websocket.Conn
	connected bool
	authenticated bool
	mu        sync.RWMutex

	reconnectAttempts    int
	maxReconnectAttempts int
	reconnectDelay       time.Duration

	messageQueue chan []byte
	done         chan struct{}
}

// Message represents a WebSocket message.
type Message struct {
	Type      string      `json:"type"`
	Payload   interface{} `json:"payload"`
	Timestamp int64       `json:"timestamp"`
}

// NewConnection creates a new connection.
func NewConnection(url, apiKey string, debug bool) *Connection {
	return &Connection{
		url:                  url,
		apiKey:               apiKey,
		debug:                debug,
		maxReconnectAttempts: 10,
		reconnectDelay:       time.Second,
		messageQueue:         make(chan []byte, 100),
		done:                 make(chan struct{}),
	}
}

// Connect establishes the WebSocket connection.
func (c *Connection) Connect(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.done:
			return
		default:
		}

		err := c.connect()
		if err != nil {
			if c.debug {
				log.Printf("[AIVory Monitor] Connection error: %v", err)
			}

			c.reconnectAttempts++
			if c.reconnectAttempts > c.maxReconnectAttempts {
				log.Println("[AIVory Monitor] Max reconnect attempts reached")
				return
			}

			delay := c.reconnectDelay * time.Duration(1<<uint(c.reconnectAttempts-1))
			if delay > 60*time.Second {
				delay = 60 * time.Second
			}

			if c.debug {
				log.Printf("[AIVory Monitor] Reconnecting in %v (attempt %d)", delay, c.reconnectAttempts)
			}

			time.Sleep(delay)
			continue
		}

		c.reconnectAttempts = 0
		c.runMessageLoop()
	}
}

// Disconnect closes the connection.
func (c *Connection) Disconnect() {
	close(c.done)

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}

	c.connected = false
	c.authenticated = false
}

// SendException sends an exception capture to the backend.
func (c *Connection) SendException(exc *capture.ExceptionCapture) {
	c.send("exception", exc)
}

// IsConnected returns true if connected and authenticated.
func (c *Connection) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected && c.authenticated
}

func (c *Connection) connect() error {
	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+c.apiKey)

	if c.debug {
		log.Printf("[AIVory Monitor] Connecting to %s", c.url)
	}

	conn, _, err := websocket.DefaultDialer.Dial(c.url, headers)
	if err != nil {
		return err
	}

	c.mu.Lock()
	c.conn = conn
	c.connected = true
	c.mu.Unlock()

	if c.debug {
		log.Println("[AIVory Monitor] WebSocket connected")
	}

	// Authenticate
	c.authenticate()

	return nil
}

func (c *Connection) authenticate() {
	hostname := ""
	// Get hostname (simplified)

	payload := map[string]interface{}{
		"api_key":       c.apiKey,
		"agent_version": "1.0.0",
		"hostname":      hostname,
		"runtime":       "go",
	}

	c.sendDirect("register", payload)
}

func (c *Connection) runMessageLoop() {
	// Start heartbeat
	heartbeatTicker := time.NewTicker(30 * time.Second)
	defer heartbeatTicker.Stop()

	// Read messages
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		for {
			_, message, err := c.conn.ReadMessage()
			if err != nil {
				if c.debug && !websocket.IsCloseError(err, websocket.CloseNormalClosure) {
					log.Printf("[AIVory Monitor] Read error: %v", err)
				}
				return
			}
			c.handleMessage(message)
		}
	}()

	// Main loop
	for {
		select {
		case <-c.done:
			return
		case <-readDone:
			c.mu.Lock()
			c.connected = false
			c.authenticated = false
			c.mu.Unlock()
			return
		case <-heartbeatTicker.C:
			if c.authenticated {
				c.send("heartbeat", map[string]interface{}{
					"timestamp": time.Now().UnixMilli(),
				})
			}
		case msg := <-c.messageQueue:
			c.mu.RLock()
			if c.conn != nil && c.connected && c.authenticated {
				c.conn.WriteMessage(websocket.TextMessage, msg)
			}
			c.mu.RUnlock()
		}
	}
}

func (c *Connection) handleMessage(data []byte) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		if c.debug {
			log.Printf("[AIVory Monitor] Error parsing message: %v", err)
		}
		return
	}

	if c.debug {
		log.Printf("[AIVory Monitor] Received: %s", msg.Type)
	}

	switch msg.Type {
	case "registered":
		c.handleRegistered()
	case "error":
		c.handleError(msg.Payload)
	default:
		if c.debug {
			log.Printf("[AIVory Monitor] Unhandled message type: %s", msg.Type)
		}
	}
}

func (c *Connection) handleRegistered() {
	c.mu.Lock()
	c.authenticated = true
	c.mu.Unlock()

	if c.debug {
		log.Println("[AIVory Monitor] Agent registered")
	}
}

func (c *Connection) handleError(payload interface{}) {
	payloadMap, ok := payload.(map[string]interface{})
	if !ok {
		return
	}

	code, _ := payloadMap["code"].(string)
	message, _ := payloadMap["message"].(string)

	log.Printf("[AIVory Monitor] Backend error: %s - %s", code, message)

	if code == "auth_error" || code == "invalid_api_key" {
		log.Println("[AIVory Monitor] Authentication failed, disabling reconnect")
		c.maxReconnectAttempts = 0
		c.Disconnect()
	}
}

func (c *Connection) send(msgType string, payload interface{}) {
	msg := Message{
		Type:      msgType,
		Payload:   payload,
		Timestamp: time.Now().UnixMilli(),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		if c.debug {
			log.Printf("[AIVory Monitor] Error marshaling message: %v", err)
		}
		return
	}

	c.mu.RLock()
	connected := c.connected && c.authenticated
	c.mu.RUnlock()

	if connected {
		select {
		case c.messageQueue <- data:
		default:
			// Queue full, drop oldest
			select {
			case <-c.messageQueue:
			default:
			}
			c.messageQueue <- data
		}
	}
}

func (c *Connection) sendDirect(msgType string, payload interface{}) {
	msg := Message{
		Type:      msgType,
		Payload:   payload,
		Timestamp: time.Now().UnixMilli(),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn != nil {
		conn.WriteMessage(websocket.TextMessage, data)
	}
}
