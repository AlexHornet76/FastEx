package wal

import (
	"encoding/json"
	"time"

	"github.com/AlexHornet76/FastEx/engine/internal/models"
	"github.com/google/uuid"
)

// EntryType represents the type of WAL entry
type EntryType string

const (
	TypeOrderPlaced   EntryType = "ORDER_PLACED"
	TypeTradeExecuted EntryType = "TRADE_EXECUTED"
	TypeOrderCanceled EntryType = "ORDER_CANCELED"
)

// Entry represents a single WAL entry
type Entry struct {
	SequenceNum uint64          `json:"sequence_num"` // Monotonic sequence
	Timestamp   time.Time       `json:"timestamp"`    // When logged
	Type        EntryType       `json:"type"`
	Data        json.RawMessage `json:"data"` // Polymorphic payload
}

// OrderPlacedData contains order placement data
type OrderPlacedData struct {
	Order *models.Order `json:"order"`
}

// TradeExecutedData contains trade execution data
type TradeExecutedData struct {
	Trade *models.Trade `json:"trade"`
}

type OrderCanceledData struct {
	OrderID    uuid.UUID `json:"order_id"`
	Instrument string    `json:"instrument"`
	Price      int64     `json:"price"` // for efficient removal
	Timestamp  time.Time `json:"timestamp"`
}

// NewOrderPlacedEntry creates a WAL entry for order placement
func NewOrderPlacedEntry(seqNum uint64, order *models.Order) (*Entry, error) {
	data, err := json.Marshal(OrderPlacedData{Order: order})
	if err != nil {
		return nil, err
	}

	return &Entry{
		SequenceNum: seqNum,
		Timestamp:   time.Now(),
		Type:        TypeOrderPlaced,
		Data:        data,
	}, nil
}

// NewTradeExecutedEntry creates a WAL entry for trade execution
func NewTradeExecutedEntry(seqNum uint64, trade *models.Trade) (*Entry, error) {
	data, err := json.Marshal(TradeExecutedData{Trade: trade})
	if err != nil {
		return nil, err
	}

	return &Entry{
		SequenceNum: seqNum,
		Timestamp:   time.Now(),
		Type:        TypeTradeExecuted,
		Data:        data,
	}, nil
}

// NewOrderCanceledEntry creates a WAL entry for order cancellation
func NewOrderCanceledEntry(seqNum uint64, orderID uuid.UUID, instrument string, price int64) (*Entry, error) {
	data, err := json.Marshal(OrderCanceledData{
		OrderID:    orderID,
		Instrument: instrument,
		Price:      price,
		Timestamp:  time.Now(),
	})
	if err != nil {
		return nil, err
	}

	return &Entry{
		SequenceNum: seqNum,
		Timestamp:   time.Now(),
		Type:        TypeOrderCanceled,
		Data:        data,
	}, nil
}

// ParseOrderPlacedData extracts order from entry
func (e *Entry) ParseOrderPlacedData() (*OrderPlacedData, error) {
	var data OrderPlacedData
	if err := json.Unmarshal(e.Data, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

// ParseTradeExecutedData extracts trade from entry
func (e *Entry) ParseTradeExecutedData() (*TradeExecutedData, error) {
	var data TradeExecutedData
	if err := json.Unmarshal(e.Data, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

// ParseOrderCanceledData extracts cancellation info from entry
func (e *Entry) ParseOrderCanceledData() (*OrderCanceledData, error) {
	var data OrderCanceledData
	if err := json.Unmarshal(e.Data, &data); err != nil {
		return nil, err
	}
	return &data, nil
}
