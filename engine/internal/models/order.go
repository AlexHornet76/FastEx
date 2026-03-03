package models

import (
	"time"

	"github.com/google/uuid"
)

// OrderSide represents buy or sell
type OrderSide string

const (
	Buy  OrderSide = "BUY"
	Sell OrderSide = "SELL"
)

// OrderType represents order type
type OrderType string

const (
	Market OrderType = "MARKET"
	Limit  OrderType = "LIMIT"
)

// OrderStatus represents order lifecycle state
type OrderStatus string

const (
	New      OrderStatus = "NEW"
	Open     OrderStatus = "OPEN"
	Filled   OrderStatus = "FILLED"
	Partial  OrderStatus = "PARTIAL"
	Canceled OrderStatus = "CANCELED"
	Rejected OrderStatus = "REJECTED"
)

// Order represents a trading order
type Order struct {
	OrderID     uuid.UUID   `json:"order_id"`
	UserID      uuid.UUID   `json:"user_id"`
	Instrument  string      `json:"instrument"` // BTC, APPL, etc.
	Side        OrderSide   `json:"side"`       // BUY or SELL
	Type        OrderType   `json:"type"`       // LIMIT or MARKET
	Price       int64       `json:"price"`      // Price in smallest unit (cents, satoshis)
	Quantity    int64       `json:"quantity"`   // Quantity in smallest unit
	FilledQty   int64       `json:"filled_qty"` // How much has been filled
	Status      OrderStatus `json:"status"`
	Timestamp   time.Time   `json:"timestamp"`    // When order was created
	SequenceNum uint64      `json:"sequence_num"` // For deterministic ordering
}

// RemainingQuantity returns unfilled quantity
func (o *Order) RemainingQuantity() int64 {
	return o.Quantity - o.FilledQty
}

// IsFilled returns true if order is completely filled
func (o *Order) IsFilled() bool {
	return o.FilledQty >= o.Quantity
}

// Trade represents a matched trade between two orders
type Trade struct {
	TradeID      uuid.UUID `json:"trade_id"`
	Instrument   string    `json:"instrument"`
	BuyOrderID   uuid.UUID `json:"buy_order_id"`
	SellOrderID  uuid.UUID `json:"sell_order_id"`
	BuyerUserID  uuid.UUID `json:"buyer_user_id"`
	SellerUserID uuid.UUID `json:"seller_user_id"`
	Price        int64     `json:"price"`    // Execution price
	Quantity     int64     `json:"quantity"` // Executed quantity
	Timestamp    time.Time `json:"timestamp"`
	SequenceNum  uint64    `json:"sequence_num"`
}
