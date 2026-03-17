package kafka

import (
	"time"

	"github.com/google/uuid"
)

const (
	TopicOrderPlaced   = "order.placed"
	TopicTradeExecuted = "trade.executed"
	TopicOrderCanceled = "order.canceled"
)

type OrderPlacedEvent struct {
	EventType  string    `json:"event_type"` // "order.placed.v1"
	EventTime  time.Time `json:"event_time"`
	Instrument string    `json:"instrument"`

	OrderID   uuid.UUID `json:"order_id"`
	UserID    uuid.UUID `json:"user_id"`
	Side      string    `json:"side"`
	Type      string    `json:"type"`
	Price     int64     `json:"price"`
	Quantity  int64     `json:"quantity"`
	FilledQty int64     `json:"filled_qty"`
	Status    string    `json:"status"`
}

type TradeExecutedEvent struct {
	EventType  string    `json:"event_type"` // "trade.executed.v1"
	EventTime  time.Time `json:"event_time"`
	Instrument string    `json:"instrument"`

	TradeID      uuid.UUID `json:"trade_id"`
	BuyOrderID   uuid.UUID `json:"buy_order_id"`
	SellOrderID  uuid.UUID `json:"sell_order_id"`
	BuyerUserID  uuid.UUID `json:"buyer_user_id"`
	SellerUserID uuid.UUID `json:"seller_user_id"`
	Price        int64     `json:"price"`
	Quantity     int64     `json:"quantity"`
}

type OrderCanceledEvent struct {
	EventType  string    `json:"event_type"` // "order.canceled.v1"
	EventTime  time.Time `json:"event_time"`
	Instrument string    `json:"instrument"`

	OrderID uuid.UUID `json:"order_id"`
	Price   int64     `json:"price"`
}
