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
}

func NewWebSocketHandler(upgrader *websocket.Upgrader, jwtSecret string) *WebSocketHandler {
	return &WebSocketHandler{
		upgrader:  upgrader,
		jwtSecret: jwtSecret,
	}
}

func (h *WebSocketHandler) HandleConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("websocket upgrade failed", "error", err)
		return
	}

	slog.Info("websocket connection established", "remote_addr", r.RemoteAddr)
	go h.handleMessages(conn)
}

func (h *WebSocketHandler) handleMessages(conn *websocket.Conn) {
	defer func() {
		h.clients.Delete(conn)
		_ = conn.Close()
		slog.Debug("websocket connection closed")
	}()

	var authenticated bool
	var clientInfo ClientInfo

	for {
		var msg map[string]any
		if err := conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Warn("websocket read error", "error", err)
			}
			return
		}

		msgType, ok := msg["type"].(string)
		if !ok || msgType == "" {
			h.sendError(conn, "missing or invalid 'type' field")
			continue
		}

		if msgType == "auth" {
			if authenticated {
				h.sendError(conn, "already authenticated")
				continue
			}

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

			clientInfo = ClientInfo{
				UserID:   claims.UserID,
				Username: claims.Username,
				Conn:     conn,
			}
			h.clients.Store(conn, &clientInfo)
			authenticated = true

			_ = conn.WriteJSON(map[string]any{
				"type":     "auth_success",
				"user_id":  clientInfo.UserID,
				"username": clientInfo.Username,
			})
			continue
		}

		// global feed
		if !authenticated {
			h.sendError(conn, "authentication required")
			continue
		}
		h.sendError(conn, "unsupported message type (global feed)")
	}
}

func (h *WebSocketHandler) sendError(conn *websocket.Conn, message string) {
	_ = conn.WriteJSON(map[string]any{
		"type":  "error",
		"error": message,
	})
}

// Broadcast sends a message to ALL authenticated WS clients.
func (h *WebSocketHandler) Broadcast(message any) {
	h.clients.Range(func(_, v any) bool {
		client := v.(*ClientInfo)
		if err := client.Conn.WriteJSON(message); err != nil {
			slog.Error("broadcast failed", "user_id", client.UserID, "error", err)
		}
		return true
	})
}
