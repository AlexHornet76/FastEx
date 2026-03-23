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

// HandleConnection upgrades HTTP to WebSocket.
func (h *WebSocketHandler) HandleConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("websocket upgrade failed", "error", err)
		return
	}

	ci := &ClientInfo{
		Conn: conn,
		Subs: make(map[string]bool),
	}
	h.clients.Store(conn, ci)

	slog.Info("websocket connection established", "remote_addr", r.RemoteAddr)

	go h.handleMessages(conn, ci)
}

func (h *WebSocketHandler) handleMessages(conn *websocket.Conn, ci *ClientInfo) {
	defer func() {
		h.clients.Delete(conn)
		conn.Close()
		slog.Debug("websocket connection closed")
	}()

	for {
		var msg map[string]any
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

		switch msgType {
		case "auth":
			// Optional auth: sets user_id/username if provided.
			token, ok := msg["token"].(string)
			if !ok || token == "" {
				h.sendError(conn, "missing 'token' field")
				continue
			}

			claims, err := auth.ValidateJWT(token, h.jwtSecret)
			if err != nil {
				slog.Warn("websocket auth failed", "error", err)
				h.sendError(conn, "invalid token")
				continue
			}

			ci.mu.Lock()
			ci.UserID = claims.UserID
			ci.Username = claims.Username
			ci.mu.Unlock()

			slog.Info("websocket client authenticated", "user_id", claims.UserID, "username", claims.Username)
			_ = conn.WriteJSON(map[string]any{
				"type":     "auth_success",
				"user_id":  claims.UserID,
				"username": claims.Username,
			})

		case "subscribe", "unsubscribe":
			// Public subscribe/unsubscribe.
			channel, _ := msg["channel"].(string)
			instrument, _ := msg["instrument"].(string)

			// We support channel="trade" for now.
			if channel == "" || instrument == "" {
				h.sendError(conn, "channel and instrument required")
				continue
			}
			if channel != "trade" {
				h.sendError(conn, "unsupported channel (use 'trade')")
				continue
			}

			key := channel + ":" + instrument
			ci.mu.Lock()
			if msgType == "subscribe" {
				ci.Subs[key] = true
			} else {
				delete(ci.Subs, key)
			}
			ci.mu.Unlock()

			_ = conn.WriteJSON(map[string]any{
				"type":       map[bool]string{true: "subscribed", false: "unsubscribed"}[msgType == "subscribe"],
				"channel":    channel,
				"instrument": instrument,
			})

		default:
			// Keep echo for testing/debugging
			_ = conn.WriteJSON(map[string]any{
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

// BroadcastMarketTrade sends raw trade update to all clients subscribed to trade:<instrument>.
func (h *WebSocketHandler) BroadcastMarketTrade(instrument string, message any) {
	key := "trade:" + instrument

	h.clients.Range(func(_, v any) bool {
		client := v.(*ClientInfo)

		client.mu.Lock()
		subscribed := client.Subs[key]
		client.mu.Unlock()

		if subscribed {
			if err := client.Conn.WriteJSON(message); err != nil {
				slog.Error("market trade broadcast failed", "user_id", client.UserID, "error", err)
			}
		}
		return true
	})
}
