package models

import (
	"time"

	"github.com/google/uuid"
)

// Instrument represents a tradeable asset
type Instrument struct {
	ID                uuid.UUID `json:"id"`
	Symbol            string    `json:"symbol"`
	Name              string    `json:"name"`
	BaseAsset         string    `json:"base_asset"`
	QuoteAsset        string    `json:"quote_asset"`
	MinOrderSize      float64   `json:"min_order_size"`
	PricePrecision    int       `json:"price_precision"`
	QuantityPrecision int       `json:"quantity_precision"`
	IsActive          bool      `json:"is_active"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`

	// Runtime data from Market Data service
	CurrentPrice float64 `json:"current_price,omitempty"`
	Change24H    float64 `json:"change_24h,omitempty"`
	Volume24H    float64 `json:"volume_24h,omitempty"`
	High24H      float64 `json:"high_24h,omitempty"`
	Low24H       float64 `json:"low_24h,omitempty"`
}
