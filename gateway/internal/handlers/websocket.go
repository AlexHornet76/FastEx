package handlers

import (
	"log/slog"
	"net/http"
	"sync"

	"github.com/AlexHornet76/FastEx/gateway/internal/auth"
	"github.com/gorilla/websocket"
)

type WebSocketHandler struct {
	upgrader      *websocket.Upgrader
	jwtSecret     string
	clientsByConn sync.Map // map[*websocket.Conn]*ClientInfo
	// user_id -> *sync.Map of conn -> struct{}
	// (sync.Map used to avoid coarse locking)
	connsByUser sync.Map // map[string]*sync.Map
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
		h.removeConn(conn)
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
			h.addConn(&clientInfo)
			authenticated = true

			_ = conn.WriteJSON(map[string]any{
				"type":     "auth_success",
				"user_id":  clientInfo.UserID,
				"username": clientInfo.Username,
			})
			continue
		}

		// /ws/market and /ws/orders are server-push feeds.
		if !authenticated {
			h.sendError(conn, "authentication required")
			continue
		}
		h.sendError(conn, "unsupported message type (server push only)")
	}
}

func (h *WebSocketHandler) sendError(conn *websocket.Conn, message string) {
	_ = conn.WriteJSON(map[string]any{
		"type":  "error",
		"error": message,
	})
}

func (h *WebSocketHandler) addConn(ci *ClientInfo) {
	h.clientsByConn.Store(ci.Conn, ci)

	// get or create per-user conn set
	v, _ := h.connsByUser.LoadOrStore(ci.UserID, &sync.Map{})
	set := v.(*sync.Map)
	set.Store(ci.Conn, struct{}{})
}

func (h *WebSocketHandler) removeConn(conn *websocket.Conn) {
	v, ok := h.clientsByConn.Load(conn)
	if ok {
		ci := v.(*ClientInfo)

		// remove from per-user set
		if sv, ok := h.connsByUser.Load(ci.UserID); ok {
			set := sv.(*sync.Map)
			set.Delete(conn)

			// cleanup empty set
			empty := true
			set.Range(func(_, _ any) bool {
				empty = false
				return false
			})
			if empty {
				h.connsByUser.Delete(ci.UserID)
			}
		}
	}

	h.clientsByConn.Delete(conn)
}

// Broadcast sends a message to ALL authenticated WS clients.
func (h *WebSocketHandler) Broadcast(message any) {
	h.clientsByConn.Range(func(_, v any) bool {
		client := v.(*ClientInfo)
		if err := client.Conn.WriteJSON(message); err != nil {
			slog.Error("broadcast failed", "user_id", client.UserID, "error", err)
		}
		return true
	})
}

// SendToUser sends a message to ALL active WS connections of a specific user.
func (h *WebSocketHandler) SendToUser(userID string, message any) {
	sv, ok := h.connsByUser.Load(userID)
	if !ok {
		return
	}
	set := sv.(*sync.Map)

	set.Range(func(k, _ any) bool {
		conn := k.(*websocket.Conn)
		if err := conn.WriteJSON(message); err != nil {
			slog.Error("send to user failed", "user_id", userID, "error", err)
		}
		return true
	})
}
