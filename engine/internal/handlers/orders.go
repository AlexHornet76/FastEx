package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/AlexHornet76/FastEx/engine/internal/engine"
	"github.com/google/uuid"
)

type OrderHandler struct {
	engines map[string]*engine.Engine // instrument -> engine
}

func NewOrderHandler(engines map[string]*engine.Engine) *OrderHandler {
	return &OrderHandler{engines: engines}
}

// SubmitOrder handles POST /orders
func (h *OrderHandler) SubmitOrder(w http.ResponseWriter, r *http.Request) {
	// Parse request
	var req SubmitOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON", "Failed to parse request body")
		return
	}
	// Validate request
	if err := req.Validate(); err != nil {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	// Get engine for the instrument
	eng, exists := h.engines[req.Instrument]
	if !exists {
		respondError(w, http.StatusBadRequest, "UNKNOWN_INSTRUMENT", "Instrument not supported")
		return
	}

	// Convert to order
	order, err := req.ToOrder()
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ORDER", err.Error())
		return
	}

	slog.Info("order submitted",
		"order_id", order.OrderID,
		"user_id", order.UserID,
		"instrument", order.Instrument,
		"side", order.Side,
		"price", order.Price,
		"quantity", order.Quantity)

	// Process order
	result, err := eng.ProcessOrder(order)
	if err != nil {
		slog.Error("failed to process order", "order_id", order.OrderID, "error", err)
		respondError(w, http.StatusInternalServerError, "PROCESSING_ERROR", err.Error())
		return
	}

	// Build response
	response := SubmitOrderResponse{
		OrderID:      order.OrderID.String(),
		Status:       string(order.Status),
		FilledQty:    order.FilledQty,
		RemainingQty: order.RemainingQuantity(),
		Trades:       make([]TradeResponse, len(result.Trades)),
	}

	for i, trade := range result.Trades {
		response.Trades[i] = TradeResponse{
			TradeID:      trade.TradeID.String(),
			Instrument:   trade.Instrument,
			Price:        trade.Price,
			Quantity:     trade.Quantity,
			BuyOrderID:   trade.BuyOrderID.String(),
			SellOrderID:  trade.SellOrderID.String(),
			BuyerUserID:  trade.BuyerUserID.String(),
			SellerUserID: trade.SellerUserID.String(),
			Timestamp:    trade.Timestamp.Format(time.RFC3339),
		}
	}

	slog.Info("order processed",
		"order_id", order.OrderID,
		"status", order.Status,
		"filled_qty", order.FilledQty,
		"trades", len(result.Trades))

	respondJSON(w, http.StatusCreated, response)

}

// CancelOrder handles DELETE /orders/:id
func (h *OrderHandler) CancelOrder(w http.ResponseWriter, r *http.Request) {
	// Extract order ID from URL path
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

	// Get engine
	eng, exists := h.engines[instrument]
	if !exists {
		respondError(w, http.StatusBadRequest, "INVALID_INSTRUMENT", "Instrument not supported")
		return
	}

	// Get order to find price
	order, exists := eng.GetOrderBook().GetOrder(orderID)
	if !exists {
		respondError(w, http.StatusNotFound, "ORDER_NOT_FOUND", "Order not found in book")
		return
	}

	// Cancel order
	if err := eng.CancelOrder(orderID, order.Price); err != nil {
		slog.Error("failed to cancel order", "order_id", orderID, "error", err)
		respondError(w, http.StatusInternalServerError, "CANCELLATION_ERROR", err.Error())
		return
	}

	slog.Info("order cancelled", "order_id", orderID, "instrument", instrument)

	respondJSON(w, http.StatusOK, map[string]string{
		"order_id": orderID.String(),
		"status":   "cancelled",
	})
}

// GetOrderBook handles GET /orderbook/:instrument
func (h *OrderHandler) GetOrderBook(w http.ResponseWriter, r *http.Request) {
	instrument := r.PathValue("instrument")
	if instrument == "" {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "Instrument required")
		return
	}

	eng, exists := h.engines[instrument]
	if !exists {
		respondError(w, http.StatusNotFound, "INVALID_INSTRUMENT", "Instrument not supported")
		return
	}

	ob := eng.GetOrderBook()

	// Build response
	response := OrderBookResponse{
		Instrument: instrument,
		Bids:       make([]PriceLevelInfo, 0),
		Asks:       make([]PriceLevelInfo, 0),
	}

	// Best prices
	if bid, exists := ob.BestBid(); exists {
		response.BestBid = bid
	}
	if ask, exists := ob.BestAsk(); exists {
		response.BestAsk = ask
	}
	if spread, exists := ob.Spread(); exists {
		response.Spread = spread
	}

	// Depth
	bidDepth, askDepth := ob.Depth()
	response.BidDepth = bidDepth
	response.AskDepth = askDepth

	// TODO: top 10 price levels (requires exposing price levels from order book)
	// For now, just aggregated info

	respondJSON(w, http.StatusOK, response)
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
	json.NewEncoder(w).Encode(ErrorResponse{
		Error: message,
		Code:  code,
	})
}
