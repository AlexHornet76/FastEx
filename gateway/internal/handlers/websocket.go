package handlers

import (
	"log/slog"
	"net/http"
	"sync"

	"github.com/AlexHornet76/FastEx/gateway/internal/auth"
	"github.com/gorilla/websocket"
)

type WebSocketHandler struct {
	upgrader  *websocket.Upgrader
	jwtSecret string
	clients   sync.Map // map[*websocket.Conn]*ClientInfo
}

type ClientInfo struct {
	UserID   string
	Username string
	Conn     *websocket.Conn

	mu   sync.Mutex
	Subs map[string]bool // e.g. "ticker:BTCUSD", "orderbook:ETHUSD"
}

func NewWebSocketHandler(upgrader *websocket.Upgrader, jwtSecret string) *WebSocketHandler {
	return &WebSocketHandler{
		upgrader:  upgrader,
		jwtSecret: jwtSecret,
	}
}

// HandleConnection upgrades HTTP to WebSocket and manages authentication
func (h *WebSocketHandler) HandleConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("websocket upgrade failed", "error", err)
		return
	}

	slog.Info("websocket connection established", "remote_addr", r.RemoteAddr)

	// Handle messages
	go h.handleMessages(conn)
}

func (h *WebSocketHandler) handleMessages(conn *websocket.Conn) {
	defer func() {
		h.clients.Delete(conn)
		conn.Close()
		slog.Debug("websocket connection closed")
	}()

	var authenticated bool
	var clientInfo ClientInfo

	for {
		var msg map[string]interface{}
		if err := conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Warn("websocket read error", "error", err)
			}
			break
		}

		msgType, ok := msg["type"].(string)
		if !ok {
			h.sendError(conn, "missing or invalid 'type' field")
			continue
		}

		// Handle authentication message
		if msgType == "auth" {
			if authenticated {
				h.sendError(conn, "already authenticated")
				continue
			}

			token, ok := msg["token"].(string)
			if !ok {
				h.sendError(conn, "missing 'token' field")
				continue
			}

			// Validate JWT
			claims, err := auth.ValidateJWT(token, h.jwtSecret)
			if err != nil {
				slog.Warn("websocket auth failed", "error", err)
				h.sendError(conn, "invalid token")
				continue
			}

			// Store client info
			clientInfo = ClientInfo{
				UserID:   claims.UserID,
				Username: claims.Username,
				Conn:     conn,
			}
			h.clients.Store(conn, &clientInfo)
			authenticated = true

			slog.Info("websocket client authenticated", "user_id", clientInfo.UserID, "username", clientInfo.Username)

			// Send success response
			response := map[string]interface{}{
				"type":     "auth_success",
				"user_id":  clientInfo.UserID,
				"username": clientInfo.Username,
			}
			conn.WriteJSON(response)
			continue
		}

		// Require authentication for other message types
		if !authenticated {
			h.sendError(conn, "authentication required")
			continue
		}

		// subscribe/unsubscribe
		switch msgType {
		case "subscribe", "unsubscribe":
			channel, _ := msg["channel"].(string)
			instrument, _ := msg["instrument"].(string)
			if channel == "" || instrument == "" {
				h.sendError(conn, "channel and instrument required")
				continue
			}
			key := channel + ":" + instrument
			clientInfo.mu.Lock()
			if msgType == "subscribe" {
				clientInfo.Subs[key] = true
			} else {
				delete(clientInfo.Subs, key)
			}
			clientInfo.mu.Unlock()

			conn.WriteJSON(map[string]any{
				"type":       map[bool]string{true: "subscribed", false: "unsubscribed"}[msgType == "subscribe"],
				"channel":    channel,
				"instrument": instrument,
			})
			continue
		default:
			// keep echo for debugging
			slog.Debug("websocket message received",
				"type", msgType,
				"user_id", clientInfo.UserID,
				"message", msg)

			conn.WriteJSON(map[string]any{
				"type":    "echo",
				"message": msg,
			})
		}
	}
}

func (h *WebSocketHandler) sendError(conn *websocket.Conn, message string) {
	response := map[string]interface{}{
		"type":  "error",
		"error": message,
	}
	if err := conn.WriteJSON(response); err != nil {
		slog.Error("send error failed", "error", err)
	}
}

// BroadcastMarket sends a message to all clients subscribed to an instrument+channel.
// will broadcast "trade" events to channel "ticker" and/or "candles" later.
func (h *WebSocketHandler) BroadcastMarket(instrument string, message any) {
	// infer channel from message.type
	m, ok := message.(map[string]any)
	if !ok {
		return
	}
	t, _ := m["type"].(string)
	channel := ""
	switch t {
	case "trade":
		channel = "ticker"
	case "candle":
		channel = "candles"
	default:
		channel = "ticker"
	}
	key := channel + ":" + instrument

	h.clients.Range(func(_, v any) bool {
		client := v.(*ClientInfo)

		client.mu.Lock()
		subscribed := client.Subs[key]
		client.mu.Unlock()

		if subscribed {
			if err := client.Conn.WriteJSON(message); err != nil {
				slog.Error("market broadcast failed", "user_id", client.UserID, "error", err)
			}
		}
		return true
	})
}
