package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/AlexHornet76/FastEx/gateway/internal/auth"
	"github.com/AlexHornet76/FastEx/gateway/internal/matching"
	"github.com/google/uuid"
)

type OrderHandler struct {
	matchingClient *matching.Client
}

func NewOrderHandler(matchingClient *matching.Client) *OrderHandler {
	return &OrderHandler{
		matchingClient: matchingClient,
	}
}

// SubmitOrderRequest represents client order submission
type SubmitOrderRequest struct {
	Instrument string `json:"instrument"` // e.g., "BTC-USD"
	Side       string `json:"side"`       // "BUY" or "SELL"
	Type       string `json:"type"`       // "LIMIT" or "MARKET"
	Price      int64  `json:"price"`      // Price in smallest unit (0 for MARKET)
	Quantity   int64  `json:"quantity"`   // Quantity in smallest unit
}

// Validate validates the order request
func (r *SubmitOrderRequest) Validate() error {
	if r.Instrument == "" {
		return &ValidationError{"instrument is required"}
	}
	if r.Side != "BUY" && r.Side != "SELL" {
		return &ValidationError{"side must be BUY or SELL"}
	}
	if r.Type != "LIMIT" && r.Type != "MARKET" {
		return &ValidationError{"type must be LIMIT or MARKET"}
	}
	if r.Type == "LIMIT" && r.Price <= 0 {
		return &ValidationError{"price must be positive for LIMIT orders"}
	}
	if r.Quantity <= 0 {
		return &ValidationError{"quantity must be positive"}
	}
	return nil
}

// ValidationError represents a validation error
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// SubmitOrder handles POST /api/orders
func (h *OrderHandler) SubmitOrder(w http.ResponseWriter, r *http.Request) {
	// Extract user from JWT (set by middleware)
	claims := auth.GetUserFromContext(r)
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	// Parse request
	var req SubmitOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON", "Failed to parse request body")
		return
	}

	// Validate
	if err := req.Validate(); err != nil {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	slog.Info("order submission",
		"user_id", claims.UserID,
		"username", claims.Username,
		"instrument", req.Instrument,
		"side", req.Side,
		"price", req.Price,
		"quantity", req.Quantity)

	// Forward to matching engine
	matchingReq := matching.SubmitOrderRequest{
		UserID:     claims.UserID,
		Instrument: req.Instrument,
		Side:       req.Side,
		Type:       req.Type,
		Price:      req.Price,
		Quantity:   req.Quantity,
	}

	result, err := h.matchingClient.SubmitOrder(matchingReq)
	if err != nil {
		slog.Error("matching engine request failed",
			"user_id", claims.UserID,
			"error", err)
		respondError(w, http.StatusInternalServerError, "MATCHING_ENGINE_ERROR", err.Error())
		return
	}

	slog.Info("order processed",
		"user_id", claims.UserID,
		"order_id", result.OrderID,
		"status", result.Status,
		"filled_qty", result.FilledQty,
		"trades", len(result.Trades))

	respondJSON(w, http.StatusCreated, result)
}

// CancelOrder handles DELETE /api/orders/:id
func (h *OrderHandler) CancelOrder(w http.ResponseWriter, r *http.Request) {
	// Extract user from JWT
	claims := auth.GetUserFromContext(r)
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	// Extract order ID from URL
	orderIDStr := r.PathValue("id")
	if orderIDStr == "" {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "Order ID required")
		return
	}

	orderID, err := uuid.Parse(orderIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ORDER_ID", "Invalid UUID format")
		return
	}

	// Extract instrument from query param
	instrument := r.URL.Query().Get("instrument")
	if instrument == "" {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "Instrument parameter required")
		return
	}

	slog.Info("order cancellation",
		"user_id", claims.UserID,
		"order_id", orderID,
		"instrument", instrument)

	// Forward to matching engine
	if err := h.matchingClient.CancelOrder(orderID, instrument); err != nil {
		slog.Error("cancel order failed",
			"user_id", claims.UserID,
			"order_id", orderID,
			"error", err)
		respondError(w, http.StatusInternalServerError, "MATCHING_ENGINE_ERROR", err.Error())
		return
	}

	slog.Info("order cancelled", "user_id", claims.UserID, "order_id", orderID)

	respondJSON(w, http.StatusOK, map[string]string{
		"order_id": orderID.String(),
		"status":   "cancelled",
	})
}

// GetOrderBook handles GET /api/orderbook/:instrument
func (h *OrderHandler) GetOrderBook(w http.ResponseWriter, r *http.Request) {
	instrument := r.PathValue("instrument")
	if instrument == "" {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "Instrument required")
		return
	}

	orderBook, err := h.matchingClient.GetOrderBook(instrument)
	if err != nil {
		slog.Error("get order book failed", "instrument", instrument, "error", err)
		respondError(w, http.StatusInternalServerError, "MATCHING_ENGINE_ERROR", err.Error())
		return
	}

	respondJSON(w, http.StatusOK, orderBook)
}

// Helper functions
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}
