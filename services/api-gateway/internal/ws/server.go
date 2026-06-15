package ws

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/auth"
	"github.com/gorilla/websocket"
)

// Config holds WebSocket security configuration.
type Config struct {
	AllowedOrigins []string
	JWTSecret      string
}

func originAllowed(r *http.Request, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	for _, o := range allowed {
		if o == origin {
			return true
		}
	}
	return false
}

func authenticateUpgrade(r *http.Request, jwtSecret string) (string, error) {
	if jwtSecret == "" {
		return "", nil
	}
	// Prefer token from query parameter for WebSocket clients.
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		auth := r.Header.Get("Authorization")
		if strings.HasPrefix(auth, "Bearer ") {
			tokenStr = auth[7:]
		}
	}
	if tokenStr == "" {
		return "", fmt.Errorf("missing token")
	}

	claims, err := auth.ParseJWT(tokenStr, jwtSecret)
	if err != nil {
		return "", err
	}
	return claims.Subject, nil
}

// WSMessage is the envelope broadcast to WebSocket clients.
type WSMessage struct {
	Topic   string          `json:"topic"`
	UserID  string          `json:"user_id,omitempty"`
	Payload json.RawMessage `json:"payload"`
}

// Client is a single WebSocket connection.
type Client struct {
	hub    *Hub
	conn   *websocket.Conn
	send   chan []byte
	topics map[string]bool
	userID string
	mu     sync.RWMutex
}

// Hub maintains the set of active clients and broadcasts messages.
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
}

// NewHub creates a new Hub.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Run starts the Hub event loop.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
		case message := <-h.broadcast:
			h.broadcastToClients(message)
		}
	}
}

// Broadcast sends a message to the hub broadcast channel.
func (h *Hub) Broadcast(message []byte) {
	select {
	case h.broadcast <- message:
	default:
		log.Println("hub broadcast channel full, dropping message")
	}
}

// BroadcastChannel returns the hub's broadcast channel for testing purposes.
func (h *Hub) BroadcastChannel() <-chan []byte {
	return h.broadcast
}

func (h *Hub) broadcastToClients(message []byte) {
	var wsMsg WSMessage
	if err := json.Unmarshal(message, &wsMsg); err != nil {
		// Fallback: broadcast raw message to all clients.
		for client := range h.clients {
			select {
			case client.send <- message:
			default:
				close(client.send)
				delete(h.clients, client)
			}
		}
		return
	}

	for client := range h.clients {
		client.mu.RLock()
		subscribed := client.topics[wsMsg.Topic]
		userID := client.userID
		client.mu.RUnlock()

		if !subscribed {
			continue
		}
		if wsMsg.UserID != "" && userID != "" && userID != wsMsg.UserID {
			continue
		}
		select {
		case client.send <- message:
		default:
			close(client.send)
			delete(h.clients, client)
		}
	}
}

// SubscribeMessage is sent by a client to subscribe to topics.
type SubscribeMessage struct {
	Action string   `json:"action"`
	Topics []string `json:"topics"`
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(512 * 1024)
	_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("websocket error: %v", err)
			}
			break
		}

		var msg SubscribeMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}
		if msg.Action == "subscribe" {
			c.mu.Lock()
			for _, t := range msg.Topics {
				c.topics[t] = true
			}
			c.mu.Unlock()
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			_, _ = w.Write(message)
			if err := w.Close(); err != nil {
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

// ServeWs handles WebSocket requests from clients.
func ServeWs(hub *Hub, w http.ResponseWriter, r *http.Request, cfg Config) {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return originAllowed(r, cfg.AllowedOrigins)
		},
	}

	userID, err := authenticateUpgrade(r, cfg.JWTSecret)
	if err != nil {
		log.Printf("websocket auth error: %v", err)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade error: %v", err)
		return
	}
	client := &Client{
		hub:    hub,
		conn:   conn,
		send:   make(chan []byte, 256),
		topics: make(map[string]bool),
		userID: userID,
	}
	client.hub.register <- client

	go client.writePump()
	go client.readPump()
}
