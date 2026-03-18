package events

import (
	"time"

	"github.com/google/uuid"
)

type TradeExecutedEvent struct {
	EventType  string    `json:"event_type"`
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
