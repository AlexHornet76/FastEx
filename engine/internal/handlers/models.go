package handlers

import (
	"fmt"
	"time"

	"github.com/AlexHornet76/FastEx/engine/internal/models"
	"github.com/google/uuid"
)

// SubmitOrderRequest represents order submission payload
type SubmitOrderRequest struct {
	UserID     string `json:"user_id"`    // UUID string
	Instrument string `json:"instrument"` // e.g., "BTC-USD"
	Side       string `json:"side"`       // "BUY" or "SELL"
	Type       string `json:"type"`       // "LIMIT" or "MARKET"
	Price      int64  `json:"price"`      // Price in smallest unit (0 for MARKET)
	Quantity   int64  `json:"quantity"`   // Quantity in smallest unit
}

// SubmitOrderResponse represents order submission result
type SubmitOrderResponse struct {
	OrderID      string          `json:"order_id"`
	Status       string          `json:"status"` // NEW, OPEN, PARTIAL, FILLED
	FilledQty    int64           `json:"filled_qty"`
	RemainingQty int64           `json:"remaining_qty"`
	Trades       []TradeResponse `json:"trades"`
}

// TradeResponse represents a trade
type TradeResponse struct {
	TradeID      string `json:"trade_id"`
	Instrument   string `json:"instrument"`
	Price        int64  `json:"price"`
	Quantity     int64  `json:"quantity"`
	BuyOrderID   string `json:"buy_order_id"`
	SellOrderID  string `json:"sell_order_id"`
	BuyerUserID  string `json:"buyer_user_id"`
	SellerUserID string `json:"seller_user_id"`
	Timestamp    string `json:"timestamp"`
}

// OrderBookResponse represents order book snapshot
type OrderBookResponse struct {
	Instrument string           `json:"instrument"`
	BestBid    int64            `json:"best_bid"`
	BestAsk    int64            `json:"best_ask"`
	Spread     int64            `json:"spread"`
	BidDepth   int              `json:"bid_depth"`
	AskDepth   int              `json:"ask_depth"`
	Bids       []PriceLevelInfo `json:"bids"`
	Asks       []PriceLevelInfo `json:"asks"`
}

// PriceLevelInfo represents aggregated info at a price level
type PriceLevelInfo struct {
	Price      int64 `json:"price"`
	Quantity   int64 `json:"quantity"`
	OrderCount int   `json:"order_count"`
}

// ErrorResponse represents API error
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}

// HealthResponse represents health check
type HealthResponse struct {
	Status      string            `json:"status"`
	Instruments map[string]string `json:"instruments"` // instrument -> "healthy"
	Timestamp   string            `json:"timestamp"`
}

// Validate validates submit order request
func (r *SubmitOrderRequest) Validate() error {
	if r.UserID == "" {
		return fmt.Errorf("user_id required")
	}
	if _, err := uuid.Parse(r.UserID); err != nil {
		return fmt.Errorf("invalid user_id format")
	}
	if r.Instrument == "" {
		return fmt.Errorf("instrument required")
	}
	if r.Side != "BUY" && r.Side != "SELL" {
		return fmt.Errorf("side must be BUY or SELL")
	}
	if r.Type != "LIMIT" && r.Type != "MARKET" {
		return fmt.Errorf("type must be LIMIT or MARKET")
	}
	if r.Type == "LIMIT" && r.Price <= 0 {
		return fmt.Errorf("price must be positive for LIMIT orders")
	}
	if r.Quantity <= 0 {
		return fmt.Errorf("quantity must be positive")
	}
	return nil
}

// ToOrder converts request to internal order model
func (r *SubmitOrderRequest) ToOrder() (*models.Order, error) {
	userID, err := uuid.Parse(r.UserID)
	if err != nil {
		return nil, err
	}

	order := &models.Order{
		OrderID:    uuid.New(),
		UserID:     userID,
		Instrument: r.Instrument,
		Side:       models.OrderSide(r.Side),
		Type:       models.OrderType(r.Type),
		Price:      r.Price,
		Quantity:   r.Quantity,
		FilledQty:  0,
		Status:     models.New,
		Timestamp:  time.Now(),
	}

	return order, nil
}
