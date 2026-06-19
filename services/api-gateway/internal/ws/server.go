package ws

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/auth"
	"github.com/olahol/melody"
)

// Config holds WebSocket security configuration.
type Config struct {
	AllowedOrigins []string
	Verifier       auth.Verifier
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

func authenticateUpgrade(r *http.Request, verifier auth.Verifier) (string, error) {
	if verifier == nil {
		return "", nil
	}
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenStr = authHeader[7:]
		}
	}
	if tokenStr == "" {
		return "", fmt.Errorf("missing token")
	}

	identity, err := verifier.Verify(r.Context(), tokenStr)
	if err != nil {
		return "", err
	}
	return identity.UserID, nil
}

// WSMessage is the envelope broadcast to WebSocket clients.
type WSMessage struct {
	Topic   string          `json:"topic"`
	UserID  string          `json:"user_id,omitempty"`
	Payload json.RawMessage `json:"payload"`
}

// SubscribeMessage is sent by a client to subscribe to topics.
type SubscribeMessage struct {
	Action string   `json:"action"`
	Topics []string `json:"topics"`
}

// NewHub creates a melody-backed WebSocket hub.
func NewHub() *melody.Melody {
	m := melody.New()
	m.Upgrader.ReadBufferSize = 1024
	m.Upgrader.WriteBufferSize = 1024

	m.HandleMessage(func(s *melody.Session, msg []byte) {
		var sub SubscribeMessage
		if err := json.Unmarshal(msg, &sub); err != nil {
			return
		}
		if sub.Action != "subscribe" {
			return
		}
		topics, _ := s.Get("topics")
		set, ok := topics.(map[string]bool)
		if !ok {
			set = make(map[string]bool)
		}
		for _, t := range sub.Topics {
			set[t] = true
		}
		s.Set("topics", set)
	})

	return m
}

// ServeWs handles WebSocket requests from clients.
func ServeWs(m *melody.Melody, w http.ResponseWriter, r *http.Request, cfg Config) {
	m.Upgrader.CheckOrigin = func(r *http.Request) bool {
		return originAllowed(r, cfg.AllowedOrigins)
	}

	userID, err := authenticateUpgrade(r, cfg.Verifier)
	if err != nil {
		log.Printf("websocket auth error: %v", err)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if err := m.HandleRequestWithKeys(w, r, map[string]any{
		"userID": userID,
		"topics": make(map[string]bool),
	}); err != nil {
		log.Printf("websocket upgrade error: %v", err)
	}
}

// Broadcast sends a message to all subscribed clients, filtering by topic and userID.
func Broadcast(m *melody.Melody, message []byte) error {
	var wsMsg WSMessage
	if err := json.Unmarshal(message, &wsMsg); err != nil {
		return m.Broadcast(message)
	}

	return m.BroadcastFilter(message, func(s *melody.Session) bool {
		userID, _ := s.Get("userID")
		uid, _ := userID.(string)
		if wsMsg.UserID != "" && uid != "" && uid != wsMsg.UserID {
			return false
		}
		topics, _ := s.Get("topics")
		set, _ := topics.(map[string]bool)
		return set[wsMsg.Topic]
	})
}
