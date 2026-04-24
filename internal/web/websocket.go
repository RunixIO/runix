package web

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

const maxWSConnections = 256

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true // not a CORS request (e.g., wscat, curl)
		}
		// Allow same-origin only.
		return originMatchesHost(r)
	},
}

// originMatchesHost checks if the Origin header matches the request Host.
func originMatchesHost(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	host := r.Host
	// Normalize: Origin is scheme://host, request.Host is host[:port].
	// Strip scheme from origin for comparison.
	originHost := origin
	if idx := strings.Index(origin, "://"); idx >= 0 {
		originHost = origin[idx+3:]
	}
	// Remove trailing slash if present.
	originHost = strings.TrimRight(originHost, "/")
	return strings.EqualFold(originHost, host)
}

// WSHub manages WebSocket client connections.
type WSHub struct {
	mu         sync.RWMutex
	clients    map[*wsClient]bool
	register   chan *wsClient
	unregister chan *wsClient
	broadcast  chan []byte
	stop       chan struct{}
}

// ClientCount returns the current number of connected clients.
func (h *WSHub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// wsClient represents a single WebSocket connection.
type wsClient struct {
	hub  *WSHub
	conn *websocket.Conn
	send chan []byte
}

// NewWSHub creates a new WebSocket hub.
func NewWSHub() *WSHub {
	return &WSHub{
		clients:    make(map[*wsClient]bool),
		register:   make(chan *wsClient),
		unregister: make(chan *wsClient, maxWSConnections),
		broadcast:  make(chan []byte, 64),
		stop:       make(chan struct{}),
	}
}

// Run starts the hub's event loop. Blocks until Stop is called.
func (h *WSHub) Run() {
	for {
		select {
		case <-h.stop:
			// Close all client connections on shutdown.
			h.mu.Lock()
			for client := range h.clients {
				close(client.send)
				delete(h.clients, client)
			}
			h.mu.Unlock()
			return

		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Debug().Int("clients", len(h.clients)).Msg("ws client connected")

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			log.Debug().Int("clients", len(h.clients)).Msg("ws client disconnected")

		case msg := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- msg:
				default:
					// Buffer full, drop client.
					go func(c *wsClient) {
						h.unregister <- c
					}(client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Broadcast sends a message to all connected clients.
func (h *WSHub) Broadcast(v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		log.Error().Err(err).Msg("failed to marshal ws broadcast")
		return
	}
	select {
	case h.broadcast <- data:
	default:
		log.Warn().Msg("ws broadcast channel full, dropping message")
	}
}

// Stop shuts down the hub, closing all client connections.
func (h *WSHub) Stop() {
	close(h.stop)
}

// handleWebSocket upgrades an HTTP connection to WebSocket.
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	count := s.hub.ClientCount()
	if count >= maxWSConnections {
		http.Error(w, "too many WebSocket connections", http.StatusServiceUnavailable)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error().Err(err).Msg("ws upgrade failed")
		return
	}

	client := &wsClient{
		hub:  s.hub,
		conn: conn,
		send: make(chan []byte, 64),
	}

	s.hub.register <- client

	// Send initial process list.
	procs := s.supervisor.List()
	initMsg, _ := json.Marshal(ProcessListMessage{
		Type:      "process_list",
		Processes: summarizeProcesses(procs),
		Timestamp: time.Now().Unix(),
	})
	select {
	case client.send <- initMsg:
	default:
	}

	go client.writePump()
	go client.readPump()
}

// readPump reads messages from the client (pings/commands).
func (c *wsClient) readPump() {
	defer func() {
		c.hub.unregister <- c
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(4096)
	_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

// writePump writes messages to the client.
func (c *wsClient) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
